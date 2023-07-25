// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build clean
// +build clean

package main

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/efs"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

func main() {
	log.Printf("Begin to clean EFS resources")
	expirationDate := time.Now().UTC().Add(clean.KeepDurationOneDay)
	cxt := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(cxt)
	if err != nil {
		log.Fatalf("Error getting default config %v", err)
	}
	efsclient := efs.NewFromConfig(defaultConfig)

	//get efs to delete
	var nextToken *string
	fileSystemIdSlice := make([]*string, 0)
	for {
		describeFileSystemsInput := efs.DescribeFileSystemsInput{Marker: nextToken}
		describeFileSystemsOutput, err := efsclient.DescribeFileSystems(cxt, &describeFileSystemsInput)
		if err != nil {
			log.Fatalf("Err %v", err)
		}
		for _, fileSystem := range describeFileSystemsOutput.FileSystems {
			if expirationDate.After(*fileSystem.CreationTime) {
				log.Printf("Trying to delete file system %s launch-date %v", *fileSystem.FileSystemId, fileSystem.CreationTime)
				if fileSystem.NumberOfMountTargets > 0 {
					err = deleteMountTargets(cxt, efsclient, fileSystem.FileSystemId)
				}

				if err == nil {
					fileSystemIdSlice = append(fileSystemIdSlice, fileSystem.FileSystemId)
				} else {
					log.Printf("Unable to delete all the mount targets for %s due to %v", *fileSystem.FileSystemId, err)
				}
			}
		}
		if describeFileSystemsOutput.NextMarker == nil {
			break
		}
		nextToken = describeFileSystemsOutput.NextMarker
	}
	for _, fileSystemId := range fileSystemIdSlice {
		terminateFileSystemsInput := efs.DeleteFileSystemInput{FileSystemId: fileSystemId}
		if _, err = efsclient.DeleteFileSystem(cxt, &terminateFileSystemsInput); err != nil {
			log.Printf("Unable to delete file system %s due to %v", *fileSystemId, err)
		} else {
			log.Printf("Deleted file system %s successfully", *fileSystemId)
		}
	}
}

func deleteMountTargets(cxt context.Context, client *efs.Client, fileSystemId *string) error {
	var marker *string
	for {
		dmti := &efs.DescribeMountTargetsInput{Marker: marker, FileSystemId: fileSystemId}
		dmto, err := client.DescribeMountTargets(cxt, dmti)
		if err != nil {
			return err
		}
		for _, mountTarget := range dmto.MountTargets {
			dlmti := &efs.DeleteMountTargetInput{MountTargetId: mountTarget.MountTargetId}
			if _, err = client.DeleteMountTarget(cxt, dlmti); err != nil {
				return err
			}
			log.Printf("Deleted mount target %s for %s successfully", *mountTarget.MountTargetId, *fileSystemId)
		}
		if dmto.Marker == nil {
			break
		}
		marker = dmto.Marker
	}
	return nil
}
