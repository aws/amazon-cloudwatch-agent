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
	// Condition is an optional function that determines if this question should be asked.
	// It receives a map of all previously answered questions and their responses.
	// Return true to ask the question, false to skip it.
	// Example: func(answers map[string]string) bool { return answers["prev_question"] == "yes" }
	Condition func(answers map[string]string) bool
}

func TriageCustomerIssue() string {
	questions, order := initializeTriageQuestions()
	answers := runTriageQuestions(questions, order)
	return formatReport(questions, order, answers)
}

func (q *Question) AskQuestion(reader *bufio.Reader) string {
	fmt.Printf("\n%s ", q.Text)
	if len(q.AnswerOptions) > 0 {
		answerOptions := []string{}
		for answerOption := range q.AnswerOptions {
			answerOptions = append(answerOptions, answerOption)
		}
		fmt.Print("(", strings.Join(answerOptions, "/"), "): ")
	}
	input, _ := reader.ReadString('\n')
	answer := strings.TrimSpace(strings.ToLower(input))
	// If the user inputs a given option, return the string that is mapped to that option
	if formatted, ok := q.AnswerOptions[answer]; ok {
		return formatted
	}
	// If not, return the raw answer
	return answer
}

func initializeTriageQuestions() (map[string]Question, []string) {
	questions := map[string]Question{
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
				return strings.ToLower(answers["env_change"]) == "y"
			},
		},
		"additional_info": {
			Text: "Is there any additional information you would like to add?",
		},
	}
	order := []string{"occurrence", "env_change", "env_desc", "additional_info"}
	return questions, order
}

func runTriageQuestions(questions map[string]Question, order []string) map[string]string {
	reader := bufio.NewReader(os.Stdin)
	answers := make(map[string]string)
	fmt.Println("Please answer these questions to better assist with your issue:")

	for _, id := range order {
		question := questions[id]
		// Skip questions whose conditions aren't met
		if question.Condition != nil && !question.Condition(answers) {
			continue
		}
		answers[id] = question.AskQuestion(reader)
	}
	return answers
}

func formatReport(questions map[string]Question, order []string, answers map[string]string) string {
	var report strings.Builder
	report.WriteString("CloudWatch Agent Debugging Information\n")
	report.WriteString("===================================\n\n")

	for _, id := range order {
		question := questions[id]
		report.WriteString("Q: " + question.Text + "\n")
		if answer, ok := answers[id]; ok && answer != "" {
			report.WriteString("A: " + answer + "\n\n")
		} else {
			report.WriteString("A: N/A\n\n")
		}
	}
	return report.String()
}
