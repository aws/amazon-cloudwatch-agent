// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2util

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
)

// EC2Util is a singleton struct
type EC2Util struct {
	Region       string
	PrivateIP    string
	InstanceID   string
	Hostname     string
	AccountID    string
	InstanceType string
	ImageID      string
}

var (
	ec2UtilInstance *EC2Util
	once            sync.Once
)

const allowedRetries = 5

func GetEC2UtilSingleton() *EC2Util {
	once.Do(func() {
		ec2UtilInstance = initEC2UtilSingleton()
	})
	return ec2UtilInstance
}

func initEC2UtilSingleton() *EC2Util {
	newInstance := &EC2Util{Region: "", PrivateIP: ""}

	if (translatorcontext.CurrentContext().Mode() == translatorconfig.ModeOnPrem) || (translatorcontext.CurrentContext().Mode() == translatorconfig.ModeOnPremise) {
		return newInstance
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

	if err := newInstance.deriveEC2MetadataFromIMDS(context.Background()); err != nil {
		fmt.Println("E! [EC2] Cannot get EC2 Metadata from IMDS:", err)
	}

	return newInstance
}

func (e *EC2Util) deriveEC2MetadataFromIMDS(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}

	mdProvider := ec2metadataprovider.NewMetadataProvider(cfg, retryer.GetDefaultRetryNumber())
	// ec2 and ecs treats retries for getting host name differently
	// More information on API: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html#instance-metadata-ex-2
	if hostname, err := mdProvider.Hostname(ctx); err != nil {
		fmt.Println("E! [EC2] Fetch hostname from EC2 metadata fail:", err)
	} else {
		e.Hostname = hostname
	}

	// More information on API: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
	if instanceIdentityDocument, err := mdProvider.Get(ctx); err != nil {
		fmt.Println("E! [EC2] Fetch identity document from EC2 metadata fail:", err)
	} else {
		e.Region = instanceIdentityDocument.Region
		e.AccountID = instanceIdentityDocument.AccountID
		e.PrivateIP = instanceIdentityDocument.PrivateIP
		e.InstanceID = instanceIdentityDocument.InstanceID
		e.InstanceType = instanceIdentityDocument.InstanceType
		e.ImageID = instanceIdentityDocument.ImageID
	}

	return nil
}
