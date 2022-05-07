// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build integration
// +build integration

package test

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

func CopyFile(pathIn string, pathOut string) {
	log.Printf("Copy File %s to %s", pathIn, pathOut)
	pathInAbs, err := filepath.Abs(pathIn)

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("File %s abs path %s", pathIn, pathInAbs)
	out, err := exec.Command("bash", "-c", "sudo cp "+pathInAbs+" "+pathOut).Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + string(out))
	}

	log.Printf("File : %s copied to : %s", pathIn, pathOut)
}

func StartAgent(configOutputPath string) {
	out, err := exec.
		Command("bash", "-c", "sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -a fetch-config -m ec2 -s -c file:"+configOutputPath).
		Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + string(out))
	}

	log.Printf("Agent has started")
}

func StopAgent() {
	out, err := exec.
		Command("bash", "-c", "sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -a stop").
		Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + string(out))
	}

	log.Printf("Agent is stopped")
}

func ReadAgentOutput(d time.Duration) string {
	out, err := exec.Command("bash", "-c",
		fmt.Sprintf("journalctl -u amazon-cloudwatch-agent.service --since \"%s ago\" --no-pager", d.String())).
		Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + string(out))
	}

	return string(out)
}

func RunShellScript(path string, args ...string) {
	out, err := exec.Command("bash", "-c", "chmod +x "+path).Output()

	if err != nil {
		log.Fatalf("Error occurred when attempting to chmod %s: %s | %s", path, err.Error(), string(out))
	}

	bashArgs := []string{"-c", "sudo ./" + path}
	bashArgs = append(bashArgs, args...)

	//out, err = exec.Command("bash", "-c", "sudo ./"+path, args).Output()
	out, err = exec.Command("bash", bashArgs...).Output()

	if err != nil {
		log.Fatalf("Error occurred when executing %s: %s | %s", path, err.Error(), string(out))
	}
}

func RunCommand(cmd string) {
	out, err := exec.Command("bash", "-c", cmd).Output()

	if err != nil {
		log.Fatalf("Error occurred when executing %s: %s | %s", cmd, err.Error(), string(out))
	}
}

func ReplaceLocalStackHostName(pathIn string) {
	out, err := exec.Command("bash", "-c", "sed -i 's/localhost.localstack.cloud/'\"$LOCAL_STACK_HOST_NAME\"'/g' "+pathIn).Output()

	if err != nil {
		log.Fatal(fmt.Sprint(err) + string(out))
	}
}

func GetInstanceId() string {
	ctx := context.Background()
	c, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		// fail fast so we don't continue the test
		log.Fatalf("Error occurred while creating SDK config: %v", err)
	}

	// TODO: this only works for EC2 based testing
	client := imds.NewFromConfig(c)
	metadata, err := client.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		log.Fatalf("Error occurred while retrieving EC2 instance ID: %v", err)
	}
	return metadata.InstanceID
}

func GetCWClient(cxt context.Context) *cloudwatch.Client {
	defaultConfig, err := config.LoadDefaultConfig(cxt)
	if err != nil {
		log.Fatalf("err occurred while creating config %v", err)
	}
	return cloudwatch.NewFromConfig(defaultConfig)
}

