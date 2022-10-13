// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package status

import (
	"fmt"
	"log"
	"text/tabwriter"
)

type TestSuiteResult struct {
	Name             string
	TestGroupResults []TestGroupResult
}

func (r TestSuiteResult) GetStatus() TestStatus {
	for _, result := range r.TestGroupResults {
		if result.GetStatus() == FAILED {
			return FAILED
		}
	}
	return SUCCESSFUL
}

func (r TestSuiteResult) Print() {
	log.Printf(">>>>>>>>>>>>>>%v<<<<<<<<<<<<<<", r.Name)
	log.Printf(">>>>>>>>>>>>>>%v<<<<<<<<<<<<<<", string(r.GetStatus()))
	for _, result := range r.TestGroupResults {
		result.Print()
	}
	log.Printf(">>>>>>>>>>>>>>><<<<<<<<<<<<<<<")
}

type TestGroupResult struct {
	Name        string
	TestResults []TestResult
}

func (r TestGroupResult) GetStatus() TestStatus {
	for _, result := range r.TestResults {
		if result.Status == FAILED {
			return FAILED
		}
	}
	return SUCCESSFUL
}

func (r TestGroupResult) Print() {
	log.Printf("==============%v==============", r.Name)
	log.Printf("==============%v==============", string(r.GetStatus()))
	w := tabwriter.NewWriter(log.Writer(), 1, 1, 1, ' ', 0)
	for _, result := range r.TestResults {
		fmt.Fprintln(w, result.Name, "\t", result.Status, "\t")
	}
	w.Flush()
	log.Printf("==============================")
}

type TestResult struct {
	Name   string
	Status TestStatus
}
