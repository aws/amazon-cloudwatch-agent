// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"uniformBuild/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

var (
	INVALID_INSTANCE = errors.New("Invalid Instance")
	INVALID_OS       = errors.New("That OS is not in supported AMIs")
)

type Instance struct {
	types.Instance
	Name string
	Os   common.OS
}
type InstanceManager struct {
	Ec2Client     *ec2.Client
	Instances     map[string]*Instance
	Amis          map[common.OS]*types.Image
	instanceGuide map[string]common.OS
}

func CreateNewInstanceManager(cfg aws.Config, instanceGuide map[string]common.OS) *InstanceManager {
	return &InstanceManager{
		Ec2Client:     ec2.NewFromConfig(cfg),
		Instances:     make(map[string]*Instance),
		Amis:          make(map[common.OS]*types.Image),
		instanceGuide: instanceGuide,
	}
}

func parseTime(value string) *time.Time {
	t, err := time.Parse("2006-01-02T15:04:05.999999999Z", value)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return &t
}
func getPlatformDetails(img *types.Image) string {
	for _, tag := range img.Tags {
		if *tag.Key == common.PLATFORM_KEY {
			return *tag.Value
		}
	}
	return ""
}
func (imng *InstanceManager) GetSupportedAMIs(accountID string) {
	//this populates the amis map
	latestAmis := GetAllAMIVersions(imng.Ec2Client) //this is sorted by date
	fmt.Printf("Found %d possible AMIs \n", len(latestAmis))
	for _, os := range common.SUPPORTED_OS {
		for _, ami := range latestAmis {
			platform := getPlatformDetails(&ami)
			if strings.Contains(strings.ToLower(platform), string(os)) {
				fmt.Printf("Using: \033[1m %s \033[0m with \033[1;34m %s \033[0m \n", *ami.ImageId, platform)
				imng.Amis[os] = &ami
				break
			}
		}
	}

}
func (imng *InstanceManager) CreateEC2InstancesBlocking() error {
	//check if all OSes are valid
	for _, osType := range imng.instanceGuide {
		if _, ok := imng.Amis[osType]; !ok {
			return INVALID_OS
		}
	}
	//create instances
	for instanceName, osType := range imng.instanceGuide {
		image := imng.Amis[osType]
		instance := CreateInstanceCmd(imng.Ec2Client, image, instanceName, osType)
		imng.Instances[instanceName] = &instance
	}
	time.Sleep(1 * time.Minute) // on average an ec2 launches in 60-90 seconds
	var wg sync.WaitGroup
	for _, instance := range imng.Instances {
		wg.Add(1)
		go func(targetInstance *Instance) {
			defer wg.Done()
			WaitUntilAgentIsOn(imng.Ec2Client, targetInstance)
			err := AssignInstanceProfile(imng.Ec2Client, targetInstance)
			if err != nil {
				fmt.Println(err)
				return
			}
			time.Sleep(30 * time.Second)
		}(instance)
	}
	wg.Wait()
	return nil
}
func (imng *InstanceManager) Close() error {
	var wg sync.WaitGroup
	for instanceName, instance := range imng.Instances {
		fmt.Printf("\033[1;35mClosed instance: %s - %s \n \033[0m", instanceName, *instance.InstanceId)
		wg.Add(1)
		go func(client *ec2.Client, instanceID string) {
			defer wg.Done()
			err := TerminateInstanceCmd(client, instanceID)
			time.Sleep(10 * time.Second)
			if err != nil {
				return
			}
		}(imng.Ec2Client, *instance.InstanceId)
	}
	wg.Wait()
	return nil
}
func (imng *InstanceManager) InsertOSRequirement(instanceName string, targetOS common.OS) error {
	instanceOS, ok := imng.instanceGuide[instanceName]
	if !ok {
		b, _ := json.MarshalIndent(imng.instanceGuide, "", "  ")
		fmt.Printf("%s is not in %s \n", instanceName, b)
		return INVALID_INSTANCE
	}
	if instanceOS == targetOS {
		return nil
	}
	return errors.New(fmt.Sprintf("This Instance is not the required OS, got: %s, requied: %s ", instanceOS, targetOS))

}
