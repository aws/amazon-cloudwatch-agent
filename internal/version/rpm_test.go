// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package version

import "testing"

func TestUpstreamRelease(t *testing.T) {
	const want = "1.300064.1"
	cases := []string{
		"1.300064.1",
		"1.300064.1b1344",
		"1.300064.1+b1344",
		"1.300064.1b1344-1",
		"1.300064.1-1.amzn2023",
		"1.300064.1-1.amzn2",
		"amazon-cloudwatch-agent-1.300064.1b1344-1.x86_64",
		"amazon-cloudwatch-agent-1.300064.1-1.amzn2023.x86_64",
		"amazon-cloudwatch-agent-1.300064.1-1.amzn2.aarch64",
	}
	for _, in := range cases {
		if got := UpstreamRelease(in); got != want {
			t.Errorf("UpstreamRelease(%q) = %q, want %q", in, got, want)
		}
	}
}
