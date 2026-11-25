// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidHostname(t *testing.T) {
	testCases := map[string]struct {
		hostname string
		want     bool
	}{
		"ip-hostname":    {hostname: "ip-10-0-0-1.us-west-2.compute.internal", want: true},
		"empty":          {hostname: "", want: false},
		"too long":       {hostname: strings.Repeat("a", maxHostnameLength+1), want: false},
		"max length":     {hostname: strings.Repeat("a", maxHostnameLength), want: true},
		"captive portal": {hostname: "<!DOCTYPE html>\n<html><body>Please log in</body></html>", want: false},
		"whitespace":     {hostname: "my host", want: false},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, isValidHostname(tc.hostname))
		})
	}
}
