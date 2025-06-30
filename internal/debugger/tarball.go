// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

type Answers struct {
	Occurence             string
	EnvironmentChange     string
	EnvironmentChangeDesc string
	AddInfo               string
}

func CreateTarball(ssm bool) {
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

	if err := addFileToTarball(tarWriter, paths.AgentLogFilePath, "logs/amazon-cloudwatch-agent.log", 50, 000); err != nil {
		fmt.Println("Error, unable to add log file to tarball: ", err)
	} else {
		fmt.Println("Warning: Only the last 50,000 lines of the log file are included.")
	}

	etcPath := "/opt/aws/amazon-cloudwatch-agent/etc"
	if err := addDirectoryToTarball(tarWriter, etcPath, "etc"); err != nil {
		fmt.Println("Error adding etc directory to tarball:", err)
	}

	// We remove triaging if called through SSM since it does not support stdin.
	if !ssm {

		answersContent := writeTriage()

		if err := addStringToTarball(tarWriter, answersContent, "debug-info.txt"); err != nil {
			fmt.Println("Error adding answers to tarball:", err)
		}
	}

	fmt.Println("Tarball created successfully at:", outputPath)

}

func Triage() Answers {
	answers := Answers{}
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Please answer these questions to better assist with your issue:")

	fmt.Println("Is this issue once off, intermittent, or consistently happening right now? (o/i/c): ")
	occurence, err := reader.ReadString('\n')
	if err == nil {
		answers.Occurence = strings.TrimSpace(occurence)
	}

	fmt.Print("Has anything changed in the environment recently? (y/n): ")
	envChange, err := reader.ReadString('\n')

	if err == nil {
		answers.EnvironmentChange = strings.TrimSpace(envChange)

		if strings.ToLower(answers.EnvironmentChange) == "y" {
			fmt.Print("Please describe what changed and when: ")
			envChangeDesc, err := reader.ReadString('\n')
			if err == nil {
				answers.EnvironmentChangeDesc = strings.TrimSpace(envChangeDesc)
			}
		}
	}

	fmt.Print("Is there any additional information you would like to add? ")
	addInfo, err := reader.ReadString('\n')

	if err == nil {
		answers.AddInfo = strings.TrimSpace(addInfo)
	}

	return answers
}

func writeTriage() string {

	answers := Triage()

	// Create answers content
	answersContent := "CloudWatch Agent Debugging Information\n"
	answersContent += "===================================\n\n"
	answersContent += "Q: Is this issue once off, intermittent, or consistently happening right now?\n"

	// Formatting answers
	occurrenceAnswer := ""
	switch strings.ToLower(answers.Occurence) {
	case "o":
		occurrenceAnswer = "Once off"
	case "i":
		occurrenceAnswer = "Intermittent"
	case "c":
		occurrenceAnswer = "Consistently happening"
	default:
		occurrenceAnswer = answers.Occurence
	}
	answersContent += "A: " + occurrenceAnswer + "\n\n"

	answersContent += "Q: Has anything changed in the environment recently?\n"
	envChangeAnswer := ""
	if strings.ToLower(answers.EnvironmentChange) == "y" {
		envChangeAnswer = "Yes"
		answersContent += "A: " + envChangeAnswer + "\n\n"
		answersContent += "Q: Please describe what has changed and when:\n"
		answersContent += "A: " + answers.EnvironmentChangeDesc + "\n\n"
	} else {
		envChangeAnswer = "No"
		answersContent += "A: " + envChangeAnswer + "\n\n"
		answersContent += "Q: Please describe what has changed and when:\n"
		answersContent += "A: N/A\n\n"
	}

	answersContent += "Q: Is there any additional information you would like to add?"
	answersContent += "A: " + answers.AddInfo + "\n\n"

	return answersContent

}

func addStringToTarball(tarWriter *tar.Writer, content, tarPath string) error {
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

// finds the last N lines of a file
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
	chunkSize := int64(4096) // 4KB chunks

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
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return fmt.Errorf("failed to copy file content for %s: %v", path, err)
			}
		}

		return nil
	})
}
