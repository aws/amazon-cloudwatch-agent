// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package testutil

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/tool/stdin"
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

func SetPrometheusRemoteWriteTestingEnv(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "amazing_access_key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "super_secret_key")
	t.Setenv("AWS_REGION", "us-east-1")
}

func Type(inputChan chan<- string, inputString ...string) {
	go func() {
		for _, s := range inputString {
			inputChan <- s
		}
	}()
}
