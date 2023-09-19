package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"strings"
	"time"
)

const TEST_REPO = "https://github.com/aws/amazon-cloudwatch-agent-test"
const BUILD_ARN = "arn:aws:iam::506463145083:instance-profile/Uniform-Build-Env-Instance-Profile"
const COMMAND_TRACKING_TIMEOUT = 20 * time.Minute
const COMMAND_TRACKING_INTERVAL = 1 * time.Second
const COMMAND_TRACKING_COUNT = int(COMMAND_TRACKING_TIMEOUT / COMMAND_TRACKING_INTERVAL)

// This is the main struct that is managing the build process
type RemoteBuildManager struct {
	ssmClient       *ssm.Client
	instanceManager *InstanceManager
	s3Client        *s3.Client
}

var DEFAULT_INSTANCE_GUIDE = map[string]OS{
	"MainBuildEnv":     LINUX,
	"WindowsMSIPacker": LINUX,
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
	b, err := json.MarshalIndent(rbm.instanceManager.amis, "", "  ")
	fmt.Printf("Got Supported Amis: %s  %s\n ", b, err)
	//linuxImage := rbm.instanceManager.GetLatestAMIVersion()
	//rbm.instanceManager.amis["linux"] = linuxImage
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
	if isAlreadyBuilt := rbm.CheckS3(commitHash); isAlreadyBuilt {
		fmt.Println("Found cache skipping build")
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
func (rbm *RemoteBuildManager) MakeMsiZip(instanceName string, commitHash string) error {
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
func (rbm *RemoteBuildManager) BuildMsi() error {
	//@TODO needs windows ami
	//command := mergeCommands(
	//	)
	return nil
}
func (rbm *RemoteBuildManager) MakeMacPkg() error {
	//@TODO needs mac ami
	command := mergeCommands(
		CloneGitRepo(TEST_REPO, "main"),
		"cd ccwa",
		MakeMacBinary(),
		CopyBinaryMac(),
		CreatePkgCopyDeps(),
		BuildAndUploadMac(),
	)
	return rbm.RunCommand(command, "MacBuildEnv", "Making Mac pkg")
}
func (rbm *RemoteBuildManager) Close() error {
	return rbm.instanceManager.Close()
}
func initEnvCmd() string {
	return mergeCommands(
		"export GOENV=/root/.config/go/env",
		"export GOCACHE=/root/.cache/go-build",
		"export GOMODCACHE=/root/go/pkg/mod",
		"export PATH=$PATH:/usr/local/go/bin",
	)
}

// CACHE COMMANDS
func (rbm *RemoteBuildManager) CheckS3(targetFile string) bool {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(S3_INTEGRATION_BUCKET),
		Key:    aws.String(targetFile),
	}
	_, err := rbm.s3Client.HeadObject(context.Background(), input)
	if err != nil {
		if err.Error() == "NotFound: Not Found" {
			fmt.Printf("Object %s does not exist in bucket %s\n", S3_INTEGRATION_BUCKET, targetFile)
		} else {
			panic(err)
		}
	} else {
		fmt.Printf("Object %s exists in bucket %s\n", S3_INTEGRATION_BUCKET, targetFile)
	}
	return false
}

func main() {
	//REPO_NAME := "https://github.com/aws/amazon-cloudwatch-agent.git"
	//BRANCH_NAME := "main"
	//@TODO ADD CACHE
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
	defer rbm.Close()
	err := rbm.BuildCWAAgent(repo, branch, comment, "MainBuildEnv")
	if err != nil {
		panic(err)
	}
	err = rbm.MakeMsiZip("WindowsMSIPacker", comment)
	if err != nil {
		panic(err)
	}

}
