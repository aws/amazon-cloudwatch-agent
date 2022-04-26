// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"
	"context"

	"github.com/aws/amazon-cloudwatch-agent/integration/clean/ami"
	"github.com/aws/amazon-cloudwatch-agent/integration/clean/dedicated_host"
	"github.com/aws/amazon-cloudwatch-agent/integration/clean/ssm"
)

const (
	defaultDaysToKeep = 5
	cleanerTypeAll    = "all"
	delimiter         = ","
)

var (
	// flags
	daysToKeep    int
	cleanersToRun string

	cleanerTypes   = []string{ssm.Type, ami.Type, dedicated_host.Type }
	cleanerOptions = strings.Join(cleanerTypes, delimiter)
)

func init() {
	flag.IntVar(&daysToKeep, "keep", defaultDaysToKeep, "Days to keep a resource before cleaning it.")
	flag.StringVar(&cleanersToRun, "clean", cleanerTypeAll,
		fmt.Sprintf("Determines which cleaners to run. (e.g. -clean ami%sssm)\nOptions: %s,%s", delimiter, cleanerTypeAll, cleanerOptions))
}

func main() {
	flag.Parse()

	ctx := context.Background()

	keepDuration := -1 * time.Hour * 24 * time.Duration(daysToKeep)

	if strings.Contains(cleanersToRun, cleanerTypeAll) {
		cleanersToRun = cleanerOptions
	}

	// The date used to determine if a resource can be cleaned.
	// Anything older than the date will be removed.
	expirationDate := time.Now().UTC().Add(keepDuration)
	log.Printf("Expiration date set to %v", expirationDate)

	for _, cleaner := range strings.Split(cleanersToRun, delimiter) {
		switch cleaner {
		case ami.Type:
			if err := ami.Clean(ctx, expirationDate); err != nil {
				log.Printf("%v", err)
			}
		case dedicated_host.Type:
			if err := dedicated_host.Clean(ctx, expirationDate); err != nil {
				log.Printf("%v", err)
			}
		case ssm.Type:
			if err := ssm.Clean(ctx, expirationDate); err != nil {
				log.Printf("%v", err)
			}
		default:
			log.Printf("Skipping invalid cleaner '%s'. Please see -h for options.", cleaner)
		}
	}

	log.Printf("Finished cleaning AWS resources")
}