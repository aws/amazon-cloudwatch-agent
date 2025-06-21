package main

import (
	"context"
	"fmt"
	"log"
	"github.com/aws/amazon-cloudwatch-agent/internal/debugger"
)

func main() {
	ctx := context.Background()

	printHeader()
	info, err := debugger.GetInstanceInfo(ctx)
	if err != nil {
		log.Fatalf("Failed to get instance info: %v", err)
	}
	printInstanceInfo(info)

	debugger.CheckConfigFiles()
}

func printHeader() {
	fmt.Println("=== AWS EC2 Instance Information ===")
	fmt.Println()
}

func printInstanceInfo(info *debugger.InstanceInfo) {
	fmt.Println("")
	fmt.Printf("Instance ID:       %s\n", info.InstanceID)
	fmt.Printf("Account ID:        %s\n", info.AccountID)
	fmt.Printf("Region:            %s\n", info.Region)
	fmt.Printf("Instance Type:     %s\n", info.InstanceType)
	fmt.Printf("Image ID:          %s\n", info.ImageID)
	fmt.Printf("Availability Zone: %s\n", info.AvailabilityZone)
	fmt.Printf("Architecture:      %s\n", info.Architecture)
}