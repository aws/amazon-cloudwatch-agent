// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"golang.org/x/sync/errgroup"
)

// This is the main struct that is managing the build process
type RemoteBuildManager struct {
	ssmClient       *ssm.Client
	instanceManager *InstanceManager
	s3Client        *s3.Client
}

var DEFAULT_INSTANCE_GUIDE = map[string]OS{
	"MainBuildEnv":      LINUX,
	"WindowsMSIPacker":  LINUX,
	"MacPkgMaker":       MACOS,
	"WindowsMSIBuilder": WINDOWS,
}
var LINUX_TEST_INSTANCE_GUIDE = map[string]OS{
	"MainBuildEnv": LINUX,
}
var MACOS_TEST_INSTANCE_GUIDE = map[string]OS{
	"MacPkgMaker": MACOS,
}
var WINDOWS_TEST_INSTANCE_GUIDE = map[string]OS{
	"WindowsMSIBuilder": WINDOWS,
}

/*
This function will create EC2 instances as a side effect
*/
func CreateRemoteBuildManager(instanceGuide map[string]OS, accountID string) *RemoteBuildManager {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil
	}
	//instance := *GetInstanceFromID(client, "i-09fc6fdc80cd713a4")
	rbm := RemoteBuildManager{}

	rbm.instanceManager = CreateNewInstanceManager(cfg, instanceGuide)
	fmt.Println("New Instance Manager Created")
	rbm.instanceManager.GetSupportedAMIs(accountID)
	fmt.Println("About to create ec2 instances")
	err = rbm.instanceManager.CreateEC2InstancesBlocking()

	if err != nil {
		panic(err)
	}
	fmt.Println("Starting SSM Client")
	rbm.ssmClient = ssm.NewFromConfig(cfg)
	//RunCmdRemotely(rbm.ssmClient, rbm.instances["linux"], "export PATH=$PATH:/usr/local/go/bin")
	rbm.s3Client = s3.NewFromConfig(cfg)
	return &rbm
}

// This function runs a command on a specific instance
func (rbm *RemoteBuildManager) RunCommand(cmd string, instanceName string, comment string) error {
	if _, ok := rbm.instanceManager.instances[instanceName]; !ok {
		return errors.New("Invalid Instance Name")
	}
	return RunCmdRemotely(rbm.ssmClient, rbm.instanceManager.instances[instanceName], cmd, comment)
}

// This function Builds CWA on a specific instance( it must be a linux instance)
func (rbm *RemoteBuildManager) BuildCWAAgent(gitUrl string, branch string, commitHash string, instanceName string) error {
	err := rbm.instanceManager.insertOSRequirement(instanceName, LINUX)
	if err != nil {
		return err
	}
	if isAlreadyBuilt := rbm.fileExistsInS3(commitHash); isAlreadyBuilt {
		fmt.Println("\033Found cache skipping build")
		return nil
	}
	fmt.Println("Starting CWA Build")
	buildMasterCommand := mergeCommands(
		CloneGitRepo(gitUrl, branch),
		MakeBuild(),
		UploadToS3(commitHash),
	)
	return rbm.RunCommand(buildMasterCommand, instanceName, fmt.Sprintf("building CWA | %s | branch: %s | hash: %s",
		strings.Replace(gitUrl, "https://github.com/", "", 1), branch, commitHash))
}

// Windows
func (rbm *RemoteBuildManager) MakeMsiZip(instanceName string, commitHash string) error {
	err := rbm.instanceManager.insertOSRequirement(instanceName, LINUX)
	if err != nil {
		return err
	}
	if isAlreadyBuilt := rbm.fileExistsInS3(fmt.Sprintf("%s/buildMSI.zip", commitHash)); isAlreadyBuilt {
		fmt.Println("\033Found cache skipping build")
		return nil
	}
	command := mergeCommands(
		CloneGitRepo(TEST_REPO, "main"),
		"cd ccwa",
		CopyBinary(commitHash),
		"ls -a",
		"unzip windows/amd64/amazon-cloudwatch-agent.zip -d windows-agent",
		MakeMSI(),
		"zip buildMSI.zip msi_dep/*",
		UploadMSI(commitHash),
	)
	return rbm.RunCommand(command, instanceName, fmt.Sprintf("Making MSI zip file for %s", commitHash))
}
func (rbm *RemoteBuildManager) BuildMSI(instanceName string, commitHash string) error {
	if err := rbm.instanceManager.insertOSRequirement(instanceName, WINDOWS); err != nil {
		return err
	}
	if isAlreadyBuilt := rbm.fileExistsInS3(fmt.Sprintf("%s/amazon-cloudwatch-agent.msi", commitHash)); isAlreadyBuilt {
		fmt.Println("\033Found cache skipping build")
		return nil
	}
	command := mergeCommandsWin(
		CopyMsi(commitHash),
		"Expand-Archive buildMSI.zip -DestinationPat C:\\buildMSI -Force",
		"cd C:\\buildMSI\\msi_dep",
		fmt.Sprintf(".\\create_msi.ps1 \"nosha\" %s/%s", S3_INTEGRATION_BUCKET, commitHash),
	)

	return rbm.RunCommand(command, instanceName, fmt.Sprintf("Making MSI Build file for %s", commitHash))
}

// / MACOS ------------
func (rbm *RemoteBuildManager) MakeMacPkg(instanceName string, commitHash string) error {
	if err := rbm.instanceManager.insertOSRequirement(instanceName, MACOS); err != nil {
		return err
	}
	if isAlreadyBuilt := rbm.fileExistsInS3(fmt.Sprintf("%s/amd64/amazon-cloudwatch-agent.pkg", commitHash)); isAlreadyBuilt {
		fmt.Println("\033Found cache skipping build")
		return nil
	}
	command := mergeCommands(
		CloneGitRepo(MAIN_REPO, "main"),
		"cd ccwa",
		MakeMacBinary(),
		CopyBinaryMac(),
		CreatePkgCopyDeps(),
		BuildAndUploadMac(commitHash),
	)
	return rbm.RunCommand(command, instanceName, "Making Mac pkg")
}
func (rbm *RemoteBuildManager) Close() error {
	return rbm.instanceManager.Close()
}
func initEnvCmd(os OS) string {
	switch os {
	case MACOS:
		return mergeCommands(
			"source /etc/profile",
			LoadWorkDirectory(os),
			"echo 'ENV SET FOR MACOS'",
		)
	case WINDOWS:
		return mergeCommandsWin(
			"$wixToolsetBinPath = \";C:\\Program Files (x86)\\WiX Toolset v3.11\\bin;\"",
			"$env:PATH = $env:PATH + $wixToolsetBinPath",
			LoadWorkDirectory(os),
		)
	default:
		return mergeCommands(
			"export GOENV=/root/.config/go/env",
			"export GOCACHE=/root/.cache/go-build",
			"export GOMODCACHE=/root/go/pkg/mod",
			"export PATH=$PATH:/usr/local/go/bin",
			LoadWorkDirectory(os),
		)
	}

}

// CACHE COMMANDS
func (rbm *RemoteBuildManager) fileExistsInS3(targetFile string) bool {
	fmt.Printf("Checking for %s cache \n", targetFile)
	input := &s3.HeadObjectInput{
		Bucket: aws.String(S3_INTEGRATION_BUCKET),
		Key:    aws.String(targetFile),
	}
	_, err := rbm.s3Client.HeadObject(context.TODO(), input)
	if err != nil {
		fmt.Printf("Object %s does not exist in bucket %s\n", targetFile, S3_INTEGRATION_BUCKET)
		fmt.Println(err)
		return false
	}
	fmt.Printf("Object %s exists in bucket %s\n", S3_INTEGRATION_BUCKET, targetFile)
	return true

}

func main() {
	var repo string
	var branch string
	var comment string
	var accountID string
	flag.StringVar(&repo, "r", "", "repository")
	flag.StringVar(&repo, "repo", "", "repository")
	flag.StringVar(&branch, "b", "", "branch")
	flag.StringVar(&branch, "branch", "", "branch")
	flag.StringVar(&comment, "c", "", "comment")
	flag.StringVar(&comment, "comment", "", "comment")
	flag.StringVar(&accountID, "a", "", "accountID")
	flag.StringVar(&accountID, "account_id", "", "accountID")
	flag.Parse()
	rbm := CreateRemoteBuildManager(DEFAULT_INSTANCE_GUIDE, accountID)
	//rbm := CreateRemoteBuildManager(WINDOWS_TEST_INSTANCE_GUIDE, accountID)
	//comment = "GHA_DEBUG_RUN"
	var err error
	eg := new(errgroup.Group)
	defer rbm.Close()
	err = rbm.BuildCWAAgent(repo, branch, comment, "MainBuildEnv")
	if err != nil {
		panic(err)
	}
	eg.Go(func() error { // windows
		err = rbm.MakeMsiZip("WindowsMSIPacker", comment)
		if err != nil {
			return err
		}
		err = rbm.BuildMSI("WindowsMSIBuilder", comment)
		if err != nil {
			return err
		}
		return nil
	})
	eg.Go(func() error {
		err = rbm.MakeMacPkg("MacPkgMaker", comment)
		if err != nil {
			return err
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		fmt.Printf("Failed because: %s \n", err)
		return
	}
	fmt.Printf("\033[32mSuccesfully\033[0m built CWA from %s with %s branch, check \033[32m%s \033[0m bucket with \033[1;32m%s\033[0m hash\n",
		repo, branch, S3_INTEGRATION_BUCKET, comment)

}
