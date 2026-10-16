// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package version

import "strings"

// UpstreamRelease maps a CloudWatch agent version string to the release used
// for compliance checks (#2049).
//
// S3, SSM, ECR, and Docker Hub names append a build id (b1344). Amazon Linux
// RPMs omit that id and put a dist tag on the release (1.amzn2023, 1.amzn2).
// Neither changes the source. 1.300064.1b1344-1 and 1.300064.1-1.amzn2023 are
// both upstream 1.300064.1.
func UpstreamRelease(nevra string) string {
	s := strings.TrimSpace(nevra)
	s = strings.TrimPrefix(s, "amazon-cloudwatch-agent-")
	s = trimArch(s)

	ver := s
	if i := strings.LastIndex(s, "-"); i > 0 {
		ver = s[:i]
	}
	return stripBuildID(ver)
}

func trimArch(s string) string {
	for _, arch := range []string{".x86_64", ".aarch64", ".arm64", ".i686", ".noarch"} {
		s = strings.TrimSuffix(s, arch)
	}
	return s
}

func stripBuildID(ver string) string {
	ver = strings.ReplaceAll(ver, "+b", "b")
	i := strings.LastIndex(ver, "b")
	if i <= 0 {
		return ver
	}
	if !digits(ver[i+1:]) {
		return ver
	}
	return ver[:i]
}

func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
