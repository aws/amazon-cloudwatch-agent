// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metric_value_benchmark

func (suite *MetricBenchmarkTestSuite) TestDummy() {
	suite.Assert().Equal(true, true, "This is a placeholder test to show how you can add more tests as part of the benchmark test suite")
}
