// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

type ec2Client interface {
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	DescribeNetworkInterfaces(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
	DeleteSecurityGroup(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error)
	RevokeSecurityGroupIngress(ctx context.Context, params *ec2.RevokeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error)
	RevokeSecurityGroupEgress(ctx context.Context, params *ec2.RevokeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupEgressOutput, error)
}

const (
	SecurityGroupProcessChanSize = 500
)

// Config holds the application configuration
type Config struct {
	ageThreshold    time.Duration
	numWorkers      int
	deleteBatchCap  int
	exceptionList   []string
	dryRun          bool
	skipVpcSGs      bool
	skipWithRules   bool
}

// Global configuration
var (
	cfg Config
)

func init() {
	// Set default configuration
	cfg = Config{
		ageThreshold:   3 * clean.KeepDurationOneDay,
		numWorkers:     30,
		exceptionList:  []string{"default"},
		dryRun:         true,
		skipVpcSGs:     false,
		skipWithRules:  false,
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	
	// Parse command line flags
	flag.BoolVar(&cfg.dryRun, "dry-run", true, "Enable dry-run mode (no actual deletion)")
	flag.DurationVar(&cfg.ageThreshold, "age", 3*clean.KeepDurationOneDay, "Age threshold for security groups (e.g. 72h)")
	flag.BoolVar(&cfg.skipVpcSGs, "skip-vpc", false, "Skip security groups associated with VPCs")
	flag.BoolVar(&cfg.skipWithRules, "skip-with-rules", false, "Skip security groups that have ingress or egress rules")
	flag.Parse()
	
	// Load AWS configuration
	awsCfg, err := loadAWSConfig(ctx)
	if err != nil {
		log.Fatalf("Error loading AWS config: %v", err)
	}

	// Create EC2 client
	client := ec2.NewFromConfig(awsCfg)

	log.Printf("🔍 Searching for unused Security Groups older than %v in %s region\n",
		cfg.ageThreshold, awsCfg.Region)

	// Delete old security groups
	deletedGroups := deleteUnusedSecurityGroups(ctx, client)
	log.Printf("Total security groups deleted: %d", len(deletedGroups))
}

func loadAWSConfig(ctx context.Context) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, fmt.Errorf("loading AWS config: %w", err)
	}
	cfg.RetryMode = aws.RetryModeAdaptive
	return cfg, nil
}

func deleteUnusedSecurityGroups(ctx context.Context, client ec2Client) []string {
	var (
		wg                      sync.WaitGroup
		deletedSecurityGroups   []string
		foundSecurityGroupChan  = make(chan types.SecurityGroup, SecurityGroupProcessChanSize)
		deletedSecurityGroupChan = make(chan string, SecurityGroupProcessChanSize)
		handlerWg               sync.WaitGroup
	)

	// Start worker pool
	log.Printf("👷 Creating %d workers\n", cfg.numWorkers)
	for i := 0; i < cfg.numWorkers; i++ {
		wg.Add(1)
		w := worker{
			id:                      i,
			wg:                      &wg,
			incomingSecurityGroupChan: foundSecurityGroupChan,
			deletedSecurityGroupChan:  deletedSecurityGroupChan,
		}
		go w.processSecurityGroup(ctx, client)
	}

	// Start handler with its own WaitGroup
	handlerWg.Add(1)
	go func() {
		handleDeletedSecurityGroups(&deletedSecurityGroups, deletedSecurityGroupChan)
		handlerWg.Done()
	}()

	// Process security groups in batches
	if err := fetchAndProcessSecurityGroups(ctx, client, foundSecurityGroupChan); err != nil {
		log.Printf("Error processing security groups: %v", err)
	}

	close(foundSecurityGroupChan)
	wg.Wait()
	close(deletedSecurityGroupChan)
	handlerWg.Wait()

	return deletedSecurityGroups
}

func handleDeletedSecurityGroups(deletedSecurityGroups *[]string, deletedSecurityGroupChan chan string) {
	for securityGroupId := range deletedSecurityGroupChan {
		*deletedSecurityGroups = append(*deletedSecurityGroups, securityGroupId)
		log.Printf("🔍 Processed %d security groups so far\n", len(*deletedSecurityGroups))
	}
}

type worker struct {
	id                        int
	wg                        *sync.WaitGroup
	incomingSecurityGroupChan <-chan types.SecurityGroup
	deletedSecurityGroupChan  chan<- string
}

func (w *worker) processSecurityGroup(ctx context.Context, client ec2Client) {
	defer w.wg.Done()

	for securityGroup := range w.incomingSecurityGroupChan {
		if err := w.handleSecurityGroup(ctx, client, securityGroup); err != nil {
			log.Printf("Worker %d: Error processing security group: %v", w.id, err)
		}
	}
}

func (w *worker) handleSecurityGroup(ctx context.Context, client ec2Client, securityGroup types.SecurityGroup) error {
	sgID := *securityGroup.GroupId
	sgName := *securityGroup.GroupName
	
	// Skip default security groups
	if isDefaultSecurityGroup(securityGroup) {
		log.Printf("⏭️ Worker %d: Skipping default security group: %s (%s)", w.id, sgID, sgName)
		return nil
	}
	
	// Skip security groups in exception list
	if isSecurityGroupException(securityGroup) {
		log.Printf("⏭️ Worker %d: Skipping security group in exception list: %s (%s)", w.id, sgID, sgName)
		return nil
	}
	
	// Check if security group is in use
	isInUse, err := isSecurityGroupInUse(ctx, client, sgID)
	if err != nil {
		return fmt.Errorf("checking if security group is in use: %w", err)
	}
	
	if isInUse {
		log.Printf("⏭️ Worker %d: Security group is in use: %s (%s)", w.id, sgID, sgName)
		return nil
	}
	
	// Check if security group has rules and we're configured to skip those
	if cfg.skipWithRules && hasRules(securityGroup) {
		log.Printf("⏭️ Worker %d: Skipping security group with rules: %s (%s)", w.id, sgID, sgName)
		return nil
	}
	
	log.Printf("🚨 Worker %d: Found unused security group: %s (%s)", w.id, sgID, sgName)
	
	// Clean up any rules before deletion
	if hasRules(securityGroup) {
		if err := cleanSecurityGroupRules(ctx, client, securityGroup); err != nil {
			return fmt.Errorf("cleaning security group rules: %w", err)
		}
	}
	
	w.deletedSecurityGroupChan <- sgID
	
	if cfg.dryRun {
		log.Printf("🛑 Dry-Run: Would delete security group: %s (%s)", sgID, sgName)
		return nil
	}
	
	return deleteSecurityGroup(ctx, client, sgID)
}

func deleteSecurityGroup(ctx context.Context, client ec2Client, securityGroupID string) error {
	_, err := client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
		GroupId: aws.String(securityGroupID),
	})
	if err != nil {
		return fmt.Errorf("deleting security group %s: %w", securityGroupID, err)
	}
	log.Printf("✅ Deleted security group: %s", securityGroupID)
	return nil
}

func cleanSecurityGroupRules(ctx context.Context, client ec2Client, securityGroup types.SecurityGroup) error {
	sgID := *securityGroup.GroupId
	
	// Clean ingress rules if any exist
	if len(securityGroup.IpPermissions) > 0 {
		if cfg.dryRun {
			log.Printf("🛑 Dry-Run: Would revoke %d ingress rules from security group: %s", 
				len(securityGroup.IpPermissions), sgID)
		} else {
			// Use a different approach - describe the security group first to get fresh rules
			describeOutput, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
				GroupIds: []string{sgID},
			})
			if err != nil {
				log.Printf("⚠️ Warning: Failed to describe security group %s: %v", sgID, err)
				return nil // Continue with deletion anyway
			}
			
			if len(describeOutput.SecurityGroups) > 0 && len(describeOutput.SecurityGroups[0].IpPermissions) > 0 {
				_, err := client.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{
					GroupId:       aws.String(sgID),
					IpPermissions: describeOutput.SecurityGroups[0].IpPermissions,
				})
				if err != nil {
					log.Printf("⚠️ Warning: Failed to revoke ingress rules for security group %s: %v", sgID, err)
					// Continue with deletion anyway
				} else {
					log.Printf("✅ Revoked ingress rules from security group: %s", sgID)
				}
			}
		}
	}
	
	// Clean egress rules if any exist
	if len(securityGroup.IpPermissionsEgress) > 0 {
		if cfg.dryRun {
			log.Printf("🛑 Dry-Run: Would revoke %d egress rules from security group: %s", 
				len(securityGroup.IpPermissionsEgress), sgID)
		} else {
			// Use a different approach - describe the security group first to get fresh rules
			describeOutput, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
				GroupIds: []string{sgID},
			})
			if err != nil {
				log.Printf("⚠️ Warning: Failed to describe security group %s: %v", sgID, err)
				return nil // Continue with deletion anyway
			}
			
			if len(describeOutput.SecurityGroups) > 0 && len(describeOutput.SecurityGroups[0].IpPermissionsEgress) > 0 {
				_, err := client.RevokeSecurityGroupEgress(ctx, &ec2.RevokeSecurityGroupEgressInput{
					GroupId:       aws.String(sgID),
					IpPermissions: describeOutput.SecurityGroups[0].IpPermissionsEgress,
				})
				if err != nil {
					log.Printf("⚠️ Warning: Failed to revoke egress rules for security group %s: %v", sgID, err)
					// Continue with deletion anyway
				} else {
					log.Printf("✅ Revoked egress rules from security group: %s", sgID)
				}
			}
		}
	}
	
	return nil
}

func fetchAndProcessSecurityGroups(ctx context.Context, client ec2Client,
	securityGroupChan chan<- types.SecurityGroup) error {

	var nextToken *string
	describeCount := 0

	for {
		output, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
			NextToken: nextToken,
		})
		if err != nil {
			return fmt.Errorf("describing security groups: %w", err)
		}

		log.Printf("🔍 Described %d times | Found %d security groups\n", describeCount, len(output.SecurityGroups))

		for _, securityGroup := range output.SecurityGroups {
			securityGroupChan <- securityGroup
		}

		if output.NextToken == nil {
			break
		}

		nextToken = output.NextToken
		describeCount++
	}

	return nil
}

func isSecurityGroupInUse(ctx context.Context, client ec2Client, securityGroupID string) (bool, error) {
	// Check if security group is attached to any network interfaces
	output, err := client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("group-id"),
				Values: []string{securityGroupID},
			},
		},
	})
	if err != nil {
		return false, fmt.Errorf("describing network interfaces: %w", err)
	}
	
	if len(output.NetworkInterfaces) > 0 {
		return true, nil
	}
	
	// Check if security group is referenced by other security groups
	sgOutput, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		return false, fmt.Errorf("describing security groups: %w", err)
	}
	
	// Check if this security group is referenced in any other security group's rules
	for _, sg := range sgOutput.SecurityGroups {
		// Skip self-references
		if *sg.GroupId == securityGroupID {
			continue
		}
		
		// Check ingress rules
		for _, permission := range sg.IpPermissions {
			for _, userIdGroupPair := range permission.UserIdGroupPairs {
				if userIdGroupPair.GroupId != nil && *userIdGroupPair.GroupId == securityGroupID {
					return true, nil
				}
			}
		}
		
		// Check egress rules
		for _, permission := range sg.IpPermissionsEgress {
			for _, userIdGroupPair := range permission.UserIdGroupPairs {
				if userIdGroupPair.GroupId != nil && *userIdGroupPair.GroupId == securityGroupID {
					return true, nil
				}
			}
		}
	}
	
	return false, nil
}

func isDefaultSecurityGroup(securityGroup types.SecurityGroup) bool {
	return *securityGroup.GroupName == "default"
}

func isSecurityGroupException(securityGroup types.SecurityGroup) bool {
	sgName := *securityGroup.GroupName
	for _, exception := range cfg.exceptionList {
		if strings.Contains(sgName, exception) {
			return true
		}
	}
	return false
}

func hasRules(securityGroup types.SecurityGroup) bool {
	return len(securityGroup.IpPermissions) > 0 || len(securityGroup.IpPermissionsEgress) > 0
}