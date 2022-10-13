// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package status

type TestSuiteResult struct {
	Name             string
	TestGroupResults []TestGroupResult
}

type TestGroupResult struct {
	Name        string
	TestResults []TestResult
}

//TODO TestSuiteResult.IsSuccessful()
//TODO TestSuiteResult.Print()

type TestResult struct {
	Name    string
	Status  TestStatus
	Message string
}
