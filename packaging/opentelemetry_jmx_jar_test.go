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

func TestOpentelemetryJMXMetricsJarSHA(t *testing.T) {
	var jmxMetricsGathererVersions = map[string]supportedJar{
		"60b2ee1a798c35d10f6e3602ea46f1b1c0298080262636d73b4fc652b7dcd0da": {
			version: "1.35.0-alpha",
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
