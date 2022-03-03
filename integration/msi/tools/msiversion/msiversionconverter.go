// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

/* We can't use the standard cw agent version for the windows msi due to limitations in the wix tools builder for msi
   msi version is different from the agent original version because of the msi limit Product version must have a major version less than 256,
   a minor version less than 256, and a build version less than 65536
*/
func main() {
	log.Printf("Input %v", os.Args)
	agentVersion := os.Args[1]
	replaceFilePath := os.Args[2]
	msiVersionKey := os.Args[3]
	split := strings.Split(agentVersion, ".")
	major := split[0]
	minor, err := strconv.ParseInt(split[1], 10, 64)
	if err != nil {
		log.Fatalf("Failed to parse agentVersion %v", err)
	}
	minor = minor / 65536
	patch, err := strconv.ParseInt(split[1], 10, 64)
	if err != nil {
		log.Fatalf("Failed to parse agentVersion %v", err)
	}
	patch = patch % 65536
	msiVersion := major + "." + strconv.FormatInt(minor, 10) + "." + strconv.FormatInt(patch, 10)
	log.Printf("Msi version is %v", msiVersion)
	replaceValue(replaceFilePath, msiVersionKey, msiVersion)
}

func replaceValue(pathIn string, key string, value string) {
	out, err := exec.Command("bash", "-c", "sed -i 's/"+key+"/'"+value+"'/g' "+pathIn).Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + string(out))
	}
}
