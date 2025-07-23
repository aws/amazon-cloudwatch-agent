// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/debugger/utils"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

type Answers struct {
	Occurence             string
	EnvironmentChange     string
	EnvironmentChangeDesc string
	AddInfo               string
}

const (
	// Specifies the amount of time to wait for debug level logging to appear.
	// 90 seconds is chosen to stay consistent with the current Mechanic implementation.
	logWaitTime = 90

	// Specifies how many lines are taken from the logs.
	// A ticket has a maximum file capacity of 5MB.
	// 50,000 lines compressed equates to ~2MB of data, allowing for a large buffer for other files.
	logLinesTaken = 50000

	// Specifies the size of logs we process at a time.
	// Chunking by 4KB can improve I/O efficiency because Linux's disk block size is 4096 bytes.
	chunkSize = 4096
)

func CreateTarball(ssm bool) {

	if ssm {
		// For SSM automatically enable debug logging and wait
		enableDebugLogging()
		showProgressBar(logWaitTime)
	} else {
		fmt.Println("\nTo capture more detailed logs, DEBUG level logging will be temporarily enabled")
		fmt.Println("for the CloudWatch Agent and LogDebug level logging for the AWS SDK.")
		fmt.Println("This will help diagnose issues more effectively.")
		fmt.Print("\nIs this okay? (y/n, default: y): ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "" || response == "y" {
			if err := enableDebugLogging(); err != nil {
				fmt.Printf("Warning: Unable to modify log levels: %v\n", err)

				// Check for permission denied using regex
				permissionDenied, _ := regexp.MatchString(`(?i)permission\s+denied`, err.Error())
				if permissionDenied {
					fmt.Println("\nThis appears to be a permission issue. Please run the debugger with sudo permissions.")
					fmt.Print("Would you like to terminate the process so you can rerun with sudo? (y/n): ")

					sudoResponse, _ := reader.ReadString('\n')
					sudoResponse = strings.TrimSpace(strings.ToLower(sudoResponse))

					if sudoResponse == "y" {
						fmt.Println("Terminating process")
						os.Exit(1)
					}
				}

				fmt.Println("Continuing with current log levels...")
			} else {
				fmt.Println("Debug logging enabled successfully.")
			}

			fmt.Println("\nWaiting 90 seconds to collect debug logs...")
			showProgressBar(logWaitTime)
		} else {
			fmt.Println("Skipping debug logging enhancement.")
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current directory: ", err)
		return
	}

	outputPath := filepath.Join(cwd, "cwagent-debug.tar.gz")
	fmt.Println("Creating tarball at:", outputPath)

	file, err := os.Create(outputPath)
	if err != nil {
		fmt.Println("Error creating tarball file:", err)
		return
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	if err := addFileToTarball(tarWriter, paths.AgentLogFilePath, "logs/amazon-cloudwatch-agent.log", 50000); err != nil {
		fmt.Println("Error, unable to add log file to tarball: ", err)
	} else {
		fmt.Println("Warning: Only the last 50,000 lines of the log file are included.")
	}

	etcPath := "/opt/aws/amazon-cloudwatch-agent/etc"
	if err := addDirectoryToTarball(tarWriter, etcPath, "etc"); err != nil {
		fmt.Println("Error adding etc directory to tarball:", err)
	}

	// We remove triaging if called through SSM since it does not support stream inputs.
	if !ssm {
		answersContent := Triage()

		if err := addTriageToTarball(tarWriter, answersContent, "debug-info.txt"); err != nil {
			fmt.Println("Error adding answers to tarball:", err)
		}
	}

	fmt.Println("Tarball created successfully at:", outputPath)
}

func Triage() string {

	answers := utils.RunTriage()
	formattedAnswers := utils.FormatReport(answers)
	return formattedAnswers

}

func addTriageToTarball(tarWriter *tar.Writer, content, tarPath string) error {
	header := &tar.Header{
		Name:    tarPath,
		Size:    int64(len(content)),
		Mode:    0644,
		ModTime: time.Now(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header for %s: %v", tarPath, err)
	}

	if _, err := io.WriteString(tarWriter, content); err != nil {
		return fmt.Errorf("failed to write content to %s: %v", tarPath, err)
	}

	return nil
}

func addFileToTarball(tarWriter *tar.Writer, filePath, tarPath string, maxLength ...int) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", filePath, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info for %s: %v", filePath, err)
	}

	tailLines := 0
	if len(maxLength) > 0 {
		tailLines = maxLength[0]
	}

	// If maxLength is 0 or negative, copy the entire file
	if tailLines <= 0 {
		header := &tar.Header{
			Name:    tarPath,
			Size:    stat.Size(),
			Mode:    int64(stat.Mode()),
			ModTime: stat.ModTime(),
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for %s: %v", filePath, err)
		}

		if _, err := io.Copy(tarWriter, file); err != nil {
			return fmt.Errorf("failed to copy file content for %s: %v", filePath, err)
		}

		return nil
	}

	// For maxLength > 0, implement tailing logic
	_, tailContent, err := findTailContent(file, tailLines)
	if err != nil {
		return fmt.Errorf("failed to find tail content in %s: %v", filePath, err)
	}

	// Create header with the size of the tail content
	header := &tar.Header{
		Name:    tarPath,
		Size:    int64(len(tailContent)),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("Failed to write tar header for %s: %v", filePath, err)
	}

	if _, err := tarWriter.Write(tailContent); err != nil {
		return fmt.Errorf("Failed to write tail content for %s: %v", filePath, err)
	}

	return nil
}

func findTailContent(file *os.File, maxLines int) (int64, []byte, error) {
	stat, err := file.Stat()
	if err != nil {
		return 0, nil, err
	}
	fileSize := stat.Size()

	if fileSize == 0 {
		return 0, []byte{}, nil
	}

	pos := fileSize
	lineCount := 0
	chunkSize := int64(chunkSize)

	var startPos int64 = 0
	buf := make([]byte, chunkSize)

	for pos > 0 && lineCount < maxLines {
		readSize := chunkSize
		if pos < chunkSize {
			readSize = pos
		}
		readPos := pos - readSize

		if _, err := file.Seek(readPos, io.SeekStart); err != nil {
			return 0, nil, err
		}

		n, err := file.Read(buf[:readSize])
		if err != nil && err != io.EOF {
			return 0, nil, err
		}

		// Count newlines in the chunk (backwards)
		for i := n - 1; i >= 0; i-- {
			if buf[i] == '\n' {
				lineCount++
				if lineCount >= maxLines {
					// Found enough lines, return position after this newline
					startPos = readPos + int64(i) + 1
					break
				}
			}
		}

		if lineCount >= maxLines {
			break
		}

		pos = readPos
	}

	if _, err := file.Seek(startPos, io.SeekStart); err != nil {
		return 0, nil, err
	}

	tailContent, err := io.ReadAll(file)
	if err != nil {
		return 0, nil, err
	}

	return startPos, tailContent, nil
}

func enableDebugLogging() error {
	envConfigPath := filepath.Join(filepath.Dir(paths.AgentLogFilePath), "env-config.json")

	envVars := make(map[string]string)
	if data, err := os.ReadFile(envConfigPath); err == nil {
		json.Unmarshal(data, &envVars)
	}

	envVars["CWAGENT_LOG_LEVEL"] = "DEBUG"
	envVars["AWS_SDK_LOG_LEVEL"] = "LogDebug"

	data, err := json.MarshalIndent(envVars, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal env config: %v", err)
	}

	return os.WriteFile(envConfigPath, data, 0644)
}

func showProgressBar(seconds int) {
	barWidth := 50
	for i := 0; i <= seconds; i++ {
		progress := float64(i) / float64(seconds)
		filledWidth := int(progress * float64(barWidth))

		bar := "["
		for j := 0; j < barWidth; j++ {
			if j < filledWidth {
				bar += "█"
			} else {
				bar += "░"
			}
		}
		bar += "]"

		percentage := int(progress * 100)
		remaining := seconds - i
		fmt.Printf("\r%s %d%% (%ds remaining)", bar, percentage, remaining)

		if i < seconds {
			time.Sleep(1 * time.Second)
		}
	}
	fmt.Println()
}

// Recursively add directory to tarball
func addDirectoryToTarball(tarWriter *tar.Writer, dirPath, tarPath string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %v", path, err)
		}

		relPath = strings.ReplaceAll(relPath, "\\", "/")

		// base case
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("failed to create tar header for %s: %v", path, err)
		}

		header.Name = filepath.Join(tarPath, relPath)
		if info.IsDir() {
			header.Name += "/"
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for %s: %v", path, err)
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file %s: %v", path, err)
			}

			if _, err := io.Copy(tarWriter, file); err != nil {
				return fmt.Errorf("failed to copy file content for %s: %v", path, err)
			}

			file.Close()
		}

		return nil
	})
}
