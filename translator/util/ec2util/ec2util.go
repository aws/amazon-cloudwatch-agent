// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2util

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"

	ec2metadata "github.com/aws/amazon-cloudwatch-agent/internal/metadata/ec2"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
)

var (
	ec2UtilInstance *ec2metadata.Metadata
	once            sync.Once
)

const allowedRetries = 5

func GetEC2UtilSingleton() *ec2metadata.Metadata {
	once.Do(func() {
		ec2UtilInstance = initEC2UtilSingleton()
	})
	return ec2UtilInstance
}

func initEC2UtilSingleton() (newInstance *ec2metadata.Metadata) {
	newInstance = &ec2metadata.Metadata{Region: "", PrivateIP: ""}

	if (translatorcontext.CurrentContext().Mode() == config.ModeOnPrem) || (translatorcontext.CurrentContext().Mode() == config.ModeOnPremise) {
		return
	}

	// Need to account for the scenario where a user running the CloudWatch agent on-premises,
	// and doesn't require connectivity with the EC2 instance metadata service, while still
	// gracefully waiting for network access on EC2 instances.
	networkUp := false
	for retry := 0; !networkUp && retry < allowedRetries; retry++ {
		ifs, err := net.Interfaces()

		if err != nil {
			fmt.Println("E! [EC2] An error occurred while fetching network interfaces: ", err)
		}

		for _, in := range ifs {
			if (in.Flags&net.FlagUp) != 0 && (in.Flags&net.FlagLoopback) == 0 {
				networkUp = true
				break
			}
		}
		if networkUp {
			fmt.Println("D! [EC2] Found active network interface")
			break
		}

		fmt.Println("W! [EC2] Sleep until network is up")
		time.Sleep(1 * time.Second)
	}

	if !networkUp {
		fmt.Println("E! [EC2] No available network interface")
	}

	if err := populateEC2Metadata(newInstance); err != nil {
		fmt.Println("E! [EC2] Cannot get EC2 Metadata", err)
	}

	return
}

func populateEC2Metadata(metadata *ec2metadata.Metadata) error {
	ses, err := session.NewSession()
	if err != nil {
		return err
	}

	ctx := context.Background()
	metadataProvider := ec2metadata.NewMetadataProvider(ses)

	if hostname, err := metadataProvider.Hostname(ctx); err != nil {
		fmt.Println("E! [EC2] Fetch hostname from EC2 metadata fail:", err)
	} else {
		metadata.Hostname = hostname
	}

	if md, err := metadataProvider.Get(ctx); err != nil {
		fmt.Println("E! [EC2] Fetch identity document from EC2 metadata fail:", err)
	} else {
		metadata.AccountID = md.AccountID
		metadata.InstanceID = md.InstanceID
		metadata.PrivateIP = md.PrivateIP
		metadata.Region = md.Region
	}
	return nil
}
