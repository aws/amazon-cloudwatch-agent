// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2util

import (
	"log"
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
}

const allowedRetries = 3

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

	ses, e := session.NewSession()
	if e != nil {
		log.Println("E! [EC2] getting new session info: ", e)
		return
	}
	md := ec2metadata.New(ses)
	for i := 0; i < allowedRetries; i++ {
		if md.Available() {
			continue
		}
		log.Println("W! [EC2] network not available yet. Sleeping for 1 second")
		time.Sleep(1 * time.Second)
	}

	if !md.Available() {
		log.Println("E! ec2metadata is not available")
		return
	}

	if info, e := md.GetMetadata("instance-id"); e == nil {
		newInstance.InstanceID = info
	} else {
		log.Println("E! getting instance-id from EC2 metadata fail: ", e)
	}

	if info, e := md.GetMetadata("hostname"); e == nil {
		newInstance.Hostname = info
	} else {
		log.Println("E! getting hostname from EC2 metadata fail: ", e)
	}

	if info, e := md.GetMetadata("local-ipv4"); e == nil {
		newInstance.PrivateIP = info
	} else {
		log.Println("E! getting local-ipv4 from EC2 metadata fail: ", e)
	}

	if info, e := md.GetInstanceIdentityDocument(); e == nil {
		newInstance.Region = info.Region
	} else {
		log.Println("E! getting region from EC2 metadata fail: ", e)
	}

	return
}
