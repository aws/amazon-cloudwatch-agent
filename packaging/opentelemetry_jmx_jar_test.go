// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package packaging

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"testing"
)

type supportedJar struct {
	jar     string
	version string
}

func TestOpenTelemetryJmxMetricsJarSHA(t *testing.T) {
	var jmxMetricsGathererVersions = map[string]supportedJar{
		"0ef4abb0da557fc424867bcd55d73459cf9f6374842775fa2e64a9fcc0fe232c": {
			version: "1.50.0-alpha",
			jar:     "JMX metrics gatherer",
		},
	}
	hash, _ := hashFile("opentelemetry-jmx-metrics.jar")
	_, ok := jmxMetricsGathererVersions[hash]
	if !ok {
		t.Fatalf("jar hash does not match known versions")
	}
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
