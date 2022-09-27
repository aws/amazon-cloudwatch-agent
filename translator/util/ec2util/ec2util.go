// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2util

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

// this is a singleton struct
type ec2Util struct {
	Region     string
	PrivateIP  string
	InstanceID string
	Hostname   string
	AccountID  string
}

var (
	ec2UtilInstance *ec2Util
	once            sync.Once
)

const allowedRetries = 5

func GetEC2UtilSingleton() *ec2Util {
	once.Do(func() {
		ec2UtilInstance = initEC2UtilSingleton()
	})
	return ec2UtilInstance
}

func initEC2UtilSingleton() (newInstance *ec2Util) {
	newInstance = &ec2Util{Region: "", PrivateIP: ""}

	if (context.CurrentContext().Mode() == config.ModeOnPrem) || (context.CurrentContext().Mode() == config.ModeOnPremise){
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

	err := newInstance.deriveEC2MetadataFromIMDS()

	if err != nil {
		fmt.Println("E! [EC2] Cannot get EC2 Metadata from IMDS:", err)
	}

	return
}

func (e *ec2Util) deriveEC2MetadataFromIMDS() error {
	ses, err := session.NewSession()

	if err != nil {
		return err
	}

	md := ec2metadata.New(ses)

	if !md.Available() {
		return errors.New("EC2 metadata is not available.")
	}

	// More information on API: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html#instance-metadata-ex-2
	if hostname, err := md.GetMetadata("hostname"); err == nil {
		e.Hostname = hostname
	} else {
		fmt.Println("E! [EC2] Fetch hostname from EC2 metadata fail:", err)
	}

	// More information on API: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
	if instanceIdentityDocument, err := md.GetInstanceIdentityDocument(); err == nil {
		e.Region = instanceIdentityDocument.Region
		e.AccountID = instanceIdentityDocument.AccountID
		e.PrivateIP = instanceIdentityDocument.PrivateIP
		e.InstanceID = instanceIdentityDocument.InstanceID
	} else {
		fmt.Println("E! [EC2] Fetch identity document from EC2 metadata fail:", err)
	}

	return nil
}
