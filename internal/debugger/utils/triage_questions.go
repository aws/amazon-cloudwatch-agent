// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Question struct {
	Text          string
	AnswerOptions map[string]string
	Condition     func(answers map[string]string) bool
}

var Questions = map[string]Question{
	"occurrence": {
		Text: "Is this issue once off, intermittent, or consistently happening right now?",
		AnswerOptions: map[string]string{
			"o": "Once off",
			"i": "Intermittent",
			"c": "Consistently happening",
		},
	},
	"env_change": {
		Text: "Has anything changed in the environment recently?",
		AnswerOptions: map[string]string{
			"y": "Yes",
			"n": "No",
		},
	},
	"env_desc": {
		Text: "Please describe what changed and when:",
		Condition: func(answers map[string]string) bool {
			return strings.ToLower(answers["env_change"]) == "yes" || answers["env_change"] == "y"
		},
	},
	"additional_info": {
		Text: "Is there any additional information you would like to add?",
	},
}

var QuestionOrder = []string{"occurrence", "env_change", "env_desc", "additional_info"}

func (question *Question) AskQuestion(reader *bufio.Reader) string {
	fmt.Print("\n", question.Text, " ")

	if len(question.AnswerOptions) > 0 {
		AnswerOptions := []string{}

		for shortcut := range question.AnswerOptions {
			AnswerOptions = append(AnswerOptions, shortcut)
		}

		fmt.Print("(", strings.Join(AnswerOptions, "/"), "): ")
	}

	input, _ := reader.ReadString('\n')
	answer := strings.TrimSpace(strings.ToLower(input))

	if formatted, ok := question.AnswerOptions[answer]; ok {
		return formatted
	}

	return answer
}

func RunTriage() map[string]string {
	reader := bufio.NewReader(os.Stdin)
	answers := make(map[string]string)

	fmt.Println("Please answer these questions to better assist with your issue:")

	for _, id := range QuestionOrder {
		question := Questions[id]

		// Skip questions whose conditions aren't met
		if question.Condition != nil && !question.Condition(answers) {
			continue
		}

		answers[id] = question.AskQuestion(reader)
	}

	return answers
}

func FormatReport(answers map[string]string) string {
	var report strings.Builder

	report.WriteString("CloudWatch Agent Debugging Information\n")
	report.WriteString("===================================\n\n")

	for _, id := range QuestionOrder {
		question := Questions[id]

		report.WriteString("Q: " + question.Text + "\n")
		if answer, ok := answers[id]; ok && answer != "" {
			report.WriteString("A: " + answer + "\n\n")
		} else {
			report.WriteString("A: N/A\n\n")
		}
	}

	return report.String()
}
