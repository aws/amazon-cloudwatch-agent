// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2util

import (
	"log"
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

const allowedRetries = 5

var e *ec2Util
var once sync.Once

func GetEC2UtilSingleton() *ec2Util {
	once.Do(func() {
		e = initEC2UtilSingleton()
	})
	return e
}

func initEC2UtilSingleton() (newInstance *ec2Util) {
	newInstance = &ec2Util{Region: "", PrivateIP: ""}
	if context.CurrentContext().Mode() == config.ModeOnPrem {
		return
	}

	// Need to account for the scenario where a user running the CloudWatch agent on-premises,
	// and doesn't require connectivity with the EC2 instance metadata service, while still
	// gracefully waiting for network access on EC2 instances.
	networkUp := false
	for retry := 0; !networkUp && retry < allowedRetries; retry++ {
		ifs, err := net.Interfaces()

		if err != nil {
			log.Println("E! [EC2] An error occurred while fetching network interfaces: ", err)
		}

		for _, in := range ifs {
			if (in.Flags&net.FlagUp) != 0 && (in.Flags&net.FlagLoopback) == 0 {
				networkUp = true
				break
			}
		}
		if networkUp {
			log.Println("D! [EC2] Found active network interface")
			break
		}

		log.Println("W! [EC2] Sleep until network is up")
		time.Sleep(1 * time.Second)
	}
	if !networkUp {
		log.Println("E! [EC2] No available network interface")
	}

	ses, err := session.NewSession()
	if err != nil {
		log.Println("E! [EC2] getting new session info: ", err)
		return
	}
	md := ec2metadata.New(ses)

	if !md.Available() {
		log.Println("E! ec2metadata is not available")
		return
	}

	if info, err := md.GetMetadata("instance-id"); err == nil {
		newInstance.InstanceID = info
	} else {
		log.Println("E! getting instance-id from EC2 metadata fail: ", err)
	}

	if info, err := md.GetMetadata("hostname"); err == nil {
		newInstance.Hostname = info
	} else {
		log.Println("E! getting hostname from EC2 metadata fail: ", err)
	}

	if info, err := md.GetMetadata("local-ipv4"); err == nil {
		newInstance.PrivateIP = info
	} else {
		log.Println("E! getting local-ipv4 from EC2 metadata fail: ", err)
	}

	if info, err := md.GetInstanceIdentityDocument(); err == nil {
		newInstance.Region = info.Region
		newInstance.AccountID = info.AccountID
	} else {
		log.Println("E! fetching identity document from EC2 metadata fail: ", err)
	}

	return
}
