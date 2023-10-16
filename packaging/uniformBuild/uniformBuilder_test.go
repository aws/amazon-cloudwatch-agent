// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

//@TODO UPDATE THE TESTS
import (
	//"github.com/stretchr/testify/require"
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"
	"uniformBuild/commands"
	"uniformBuild/common"
	"uniformBuild/utils"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-sdk-go-v2/config"
)

var accountID string

func TestMain(m *testing.M) {
	flag.StringVar(&accountID, "a", "", "accountID")
	flag.StringVar(&accountID, "account_id", "", "accountID")
	code := m.Run()
	os.Exit(code)
}
func TestAmiLatest(t *testing.T) {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	imng := utils.CreateNewInstanceManager(cfg, DEFAULT_INSTANCE_GUIDE)
	// is it consistent
	previous := *imng.GetLatestAMIVersion(accountID).ImageId
	for i := 0; i < 5; i++ {
		current := *imng.GetLatestAMIVersion(accountID).ImageId
		require.Equalf(t, current, previous, "AMI is inconsistent %s | %s", current, previous)
	}
	fmt.Println(imng.GetLatestAMIVersion(accountID).ImageId)

}
func TestSupportedAmis(t *testing.T) {
	cfg, _ := config.LoadDefaultConfig(context.TODO())

	imng := utils.CreateNewInstanceManager(cfg, DEFAULT_INSTANCE_GUIDE)
	imng.GetSupportedAMIs(accountID)
	for _, os := range common.SUPPORTED_OS {
		_, ok := imng.amis[os]
		require.Truef(t, ok, "It does not contain", os)
	}
	//fmt.Println(imng.amis)
}

func TestEc2Generation(t *testing.T) {
	rbm := CreateRemoteBuildManager(LINUX_TEST_INSTANCE_GUIDE, accountID)
	fmt.Println(rbm.ssmClient)
	defer rbm.Close()
}
func TestS3Cache(t *testing.T) {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	rbm := RemoteBuildManager{}
	rbm.s3Client = s3.NewFromConfig(cfg)
	require.False(t, rbm.fileExistsInS3("FileThatDoestExist"), "Should fail")
	require.True(t, rbm.fileExistsInS3("checkS3"))

}
func TestOnSpecificInstance(t *testing.T) {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	imng := utils.CreateNewInstanceManager(cfg, DEFAULT_INSTANCE_GUIDE)
	testInstance := &utils.Instance{
		*utils.GetInstanceFromID(imng.ec2Client, "i-0dd926b8dcf5884b6"),
		"_",
		common.LINUX,
	}
	ssmClient := ssm.NewFromConfig(cfg)
	utils.RunCmdRemotely(ssmClient, testInstance, commands.mergeCommands(
		"aws --version",
	),
		"Manual Testing")
}
func TestEnviorment(t *testing.T) {
	guide := map[string]common.OS{
		"MainBuildEnv": common.LINUX,
	}
	rbm := CreateRemoteBuildManager(guide, accountID)
	defer rbm.Close()
	func() {
		require.NoError(t,
			rbm.RunCommand(commands.mergeCommands(
				"go version",
				"go env",
			),
				"MainBuildEnv",
				"test env go version"),
		)
	}()
	require.NoError(t,
		rbm.RunCommand(commands.mergeCommands(
			"aws --version",
		),
			"MainBuildEnv",
			"test env aws"),
	)
	require.NoError(t,
		rbm.RunCommand(commands.mergeCommands(
			"make --version",
		),
			"MainBuildEnv",

			"make"),
	)
}
func TestOSMixUp(t *testing.T) {
	guide := map[string]common.OS{
		"linux": common.LINUX,
		"win":   common.WINDOWS,
	}
	rbm := CreateRemoteBuildManager(guide, accountID)
	defer rbm.Close()
	require.NoErrorf(t, rbm.instanceManager.insertOSRequirement("linux", common.LINUX), "")
	require.Errorf(t, rbm.instanceManager.insertOSRequirement("linux", common.WINDOWS),
		"You should be getting an error for mixing OSes")

}
func TestPublicRepoBuild(t *testing.T) {
	REPO_NAME := "https://github.com/aws/amazon-cloudwatch-agent.git"
	BRANCH_NAME := "main"
	rbm := CreateRemoteBuildManager(DEFAULT_INSTANCE_GUIDE, accountID)
	defer rbm.Close()
	err := rbm.BuildCWAAgent(REPO_NAME, BRANCH_NAME, fmt.Sprintf("PUBLIC_REPO_TEST-%d", time.Now().Unix()), "MainBuildEnv")
	require.NoError(t, err)
	//rbm.RunCommand(RemoveFolder("ccwa"), "clean the repo folder")
}

func TestPrivateFork(t *testing.T) {
	//REPO_NAME := "https://github.com/aws/amazon-cloudwatch-agent.git"
	//BRANCH_NAME := "main"
	//rbm := CreateRemoteBuildManager()
	//rbm.CloneGitRepo(REPO_NAME, BRANCH_NAME)
}
func TestMakeMsiZip(t *testing.T) {
	//TestPublicRepoBuild(t)
	guide := map[string]common.OS{
		"WindowsMSIPacker": common.LINUX,
	}
	rbm := CreateRemoteBuildManager(guide, accountID)
	defer rbm.Close()
	require.NoError(t, rbm.MakeMsiZip("WindowsMSIPacker", "PUBLIC_REPO_TEST-1695063642"))
}
func TestBuildMsi(t *testing.T) {

}
func TestMakeMacPkg(t *testing.T) {

}
