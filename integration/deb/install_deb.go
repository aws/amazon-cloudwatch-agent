// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"time"
)

const (
	numberRetry = 5
	waitTime    = time.Minute
)

// Try to install amazon cloud watch agent deb. If you can't install wait 1 minute
func main() {
	for i := 0; i < numberRetry; i++ {
		log.Print("Attempt to install cloud watch agent deb number " + strconv.Itoa(i+1))
		out, err := exec.Command("bash", "-c", "sudo apt install -y ./amazon-cloudwatch-agent.deb").Output()
		if err != nil {
			log.Print(fmt.Sprint(err) + string(out))
		} else {
			log.Print("Agent Installed")
			return
		}
		time.Sleep(waitTime)
	}
	log.Fatal("Could not install amazon cloud watch agent deb")
}
