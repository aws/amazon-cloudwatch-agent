// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

const (
	TimeFormat = "2006-01-02T15:04:05Z"
	DateRegex  = `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z E!`
	// Finder looks for errors from each agent restart
	StartUp = `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z\s+I!\s+Starting AmazonCloudWatchAgent.*with log file`
)

func GetErrorSuggestions(logFilePath string) []DiagnosticSuggestion {
	err := checkAgentLogPermissions(logFilePath)
	if err != nil {
		fmt.Printf("Permission check for suggestions failed: %v\n", err)
		return nil
	}

	logEntries, err := getLogEntries(logFilePath, StartUp)
	if err != nil {
		fmt.Printf("Error reading log file for suggestions: %v\n", err)
		return nil
	}

	errorEntries := filterErrorEntries(logEntries)

	commonErrors := InitializeCommonErrors()
	suggestions := matchPatterns(errorEntries, commonErrors)
	suggestions = deduplicateSuggestions(suggestions)

	printLogErrorsSummary(suggestions, len(errorEntries))

	return suggestions
}

func checkAgentLogPermissions(logFilePath string) error {
	_, err := os.Stat(logFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("CloudWatch Agent log file not found at %s. Ensure the CloudWatch Agent is running", logFilePath)
		}
		return fmt.Errorf("Cannot access log file %s: %v. Try running as cwagent user or with sudo", logFilePath, err)
	}

	file, err := os.Open(logFilePath)
	if err != nil {
		return fmt.Errorf("Cannot read log file %s: %v. Try running as cwagent user or with sudo", logFilePath, err)
	}
	file.Close()

	return nil
}

func getLogEntries(logFilePath string, startupPattern string) ([]string, error) {
	logFile, err := os.Open(logFilePath)
	if err != nil {
		return nil, err
	}
	defer logFile.Close()

	logFileContent, err := io.ReadAll(logFile)
	if err != nil {
		return nil, err
	}

	entryPattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`)

	// Passing -1 means find all matches
	logLineIndices := entryPattern.FindAllStringIndex(string(logFileContent), -1)

	if len(logLineIndices) == 0 {
		return []string{}, nil
	}

	// Find the most recent startup entry by scanning backwards
	// If a startup entry is not found, use the whole log file for the next section
	startIndex := 0
	if startupPattern != "" {
		startupRegex := regexp.MustCompile(startupPattern)

		for i := len(logLineIndices) - 1; i >= 0; i-- {
			start := logLineIndices[i][0]
			var end int

			if i+1 < len(logLineIndices) {
				end = logLineIndices[i+1][0]
			} else {
				end = len(logFileContent)
			}

			entry := string(logFileContent[start:end])
			if startupRegex.MatchString(entry) {
				startIndex = i
				break
			}

		}
	}

	var allEntries []string
	for i := startIndex; i < len(logLineIndices); i++ {
		start := logLineIndices[i][0]
		var end int

		if i+1 < len(logLineIndices) {
			end = logLineIndices[i+1][0]
		} else {
			end = len(logFileContent)
		}

		entry := strings.TrimSpace(string(logFileContent[start:end]))

		if entry != "" {
			allEntries = append(allEntries, entry)
		}
	}

	return allEntries, nil
}

func filterErrorEntries(entries []string) []string {
	var errorEntries []string
	errorPattern := regexp.MustCompile(DateRegex)

	for _, entry := range entries {
		if !errorPattern.MatchString(entry) {
			continue
		}
		errorEntries = append(errorEntries, entry)
	}

	return errorEntries
}

func matchPatterns(errorEntries []string, patterns []DiagnosticSuggestion) []DiagnosticSuggestion {
	var suggestions []DiagnosticSuggestion

	for _, entry := range errorEntries {
		for _, pattern := range patterns {
			if !pattern.Pattern.MatchString(entry) {
				continue
			}
			suggestions = append(suggestions, pattern)
		}
	}

	return suggestions
}

func deduplicateSuggestions(suggestions []DiagnosticSuggestion) []DiagnosticSuggestion {
	seen := make(map[string]bool)
	var uniqueErrors []DiagnosticSuggestion

	for _, suggestion := range suggestions {
		if !seen[suggestion.Possibility] {
			seen[suggestion.Possibility] = true
			uniqueErrors = append(uniqueErrors, suggestion)
		}
	}

	return uniqueErrors
}

func printLogErrorsSummary(suggestions []DiagnosticSuggestion, numErrorEntries int) {
	fmt.Println("\n=== Log Errors Summary ===")
	fmt.Printf("Found %d error entries since last startup\n", numErrorEntries)
	if len(suggestions) == 0 {
		if numErrorEntries > 0 {
			fmt.Println("Found no matching errors. Check the agent's logs to review the errors.")
		}
		fmt.Println("\nPlease restart agent for log error changes to take effect.")
		return
	}

	fmt.Printf("Issues from logs (%d):\n", len(suggestions))
	for _, suggestion := range suggestions {
		fmt.Printf(". %s\n", suggestion.Issue)
		fmt.Printf("  %s\n", suggestion.Possibility)
	}

	fmt.Println("\nPlease restart agent for log error changes to take effect.")
}
