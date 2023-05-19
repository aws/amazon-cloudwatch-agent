// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package testutil

import (
	"fmt"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/stdin"
)

func SetUpTestInputStream() chan<- string {
	inputChan := make(chan string)
	stdin.Scanln = func(answer ...interface{}) (int, error) {
		inputString := <-inputChan
		fmt.Println(inputString)
		*(answer[0].(*string)) = inputString
		return len(inputString), nil
	}
	return inputChan
}

func Type(inputChan chan<- string, inputString ...string) {
	go func() {
		for _, s := range inputString {
			inputChan <- s
		}
	}()
}
