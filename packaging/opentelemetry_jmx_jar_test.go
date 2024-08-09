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
		"14f28b1c45e6ad91faa7f25462bfd96e6ab3b6980afe5534f92b8a4973895cbb": {
			version: "1.37.0-fix",
			jar:     "JMX metrics gatherer w/ Tomcat metrics fix",
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
