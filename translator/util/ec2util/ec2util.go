// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2util

import (
	localContext "context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	loggerConfig "github.com/aws/private-amazon-cloudwatch-agent-staging/cfg/aws"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/config"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/context"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// this is a singleton struct
type Ec2Util struct {
	Region           string
	PrivateIP        string
	InstanceID       string
	Hostname         string
	AccountID        string
	InstanceDocument *imds.InstanceIdentityDocument
}

var (
	ec2UtilInstance *Ec2Util
	once            sync.Once
)

const (
	allowedRetries     = 5
	hostname           = "hostname"
	defaultIMDSTimeout = 1 * time.Second
)

func GetEC2UtilSingleton() *Ec2Util {
	once.Do(func() {
		ec2UtilInstance = initEC2UtilSingleton()
	})
	return ec2UtilInstance
}

func initEC2UtilSingleton() (newInstance *Ec2Util) {
	newInstance = &Ec2Util{Region: "", PrivateIP: ""}

	if (context.CurrentContext().Mode() == config.ModeOnPrem) || (context.CurrentContext().Mode() == config.ModeOnPremise) {
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

func (e *Ec2Util) deriveEC2MetadataFromIMDS() error {
	cfg, err := awsConfig.LoadDefaultConfig(localContext.Background())
	if err != nil {
		return err
	}
	cfg.Retryer = func() aws.Retryer {
		return retry.NewStandard(func(options *retry.StandardOptions) {
			options.MaxAttempts = allowedRetries
		})
	}
	cfg.Logger = loggerConfig.SDKLogger{}
	cfg.ClientLogMode = loggerConfig.SDKV2ClientMode()
	cfg.HTTPClient = &http.Client{Timeout: defaultIMDSTimeout}

	optionsIMDSV2Only := func(o *imds.Options) {
		o.EnableFallback = aws.FalseTernary
	}
	optionsIMDSV1Fallback := func(o *imds.Options) {
		o.EnableFallback = aws.TrueTernary
	}

	clientIMDSV2Only := imds.NewFromConfig(cfg, optionsIMDSV2Only)
	clientIMDSV1Fallback := imds.NewFromConfig(cfg, optionsIMDSV1Fallback)
	err = e.callIMDSClient(clientIMDSV2Only)
	if err != nil {
		log.Printf("W! [EC2] Fetch EC2 metadata from imdsv2 fail: %v", err)
		err := e.callIMDSClient(clientIMDSV1Fallback)
		if err != nil {
			log.Printf("W! [EC2] Fetch EC2 metadata from imdsv1 fail: %v", err)
		}
	}

	return nil
}

func (e *Ec2Util) callIMDSClient(client *imds.Client) error {
	getMetadataInput := imds.GetMetadataInput{
		Path: hostname,
	}
	metadata, err := client.GetMetadata(localContext.Background(), &getMetadataInput)
	if err != nil {
		log.Printf("W! [EC2] Fetch hostname from EC2 metadata: %v", err)
		return err
	}
	e.Hostname = fmt.Sprintf("%v", metadata.ResultMetadata.Get(hostname))

	getInstanceDocumentInput := imds.GetInstanceIdentityDocumentInput{}
	instanceDocument, err := client.GetInstanceIdentityDocument(localContext.Background(), &getInstanceDocumentInput)
	if err != nil {
		log.Printf("W! [EC2] Fetch identity document from EC2 metadata fail: %v", err)
		return err
	}
	e.Region = instanceDocument.Region
	e.AccountID = instanceDocument.AccountID
	e.PrivateIP = instanceDocument.PrivateIP
	e.InstanceID = instanceDocument.InstanceID
	e.InstanceDocument = &instanceDocument.InstanceIdentityDocument
	return nil
}
