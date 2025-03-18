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
	ageThreshold  time.Duration
	numWorkers    int
	exceptionList []string
	dryRun        bool
	skipVpcSGs    bool
	skipWithRules bool
}

// Global configuration
var (
	cfg Config
)

func init() {
	// Set default configuration
	cfg = Config{
		ageThreshold:  1 * clean.KeepDurationOneDay,
		numWorkers:    30,
		exceptionList: []string{"default"},
		dryRun:        true,
		skipVpcSGs:    false,
		skipWithRules: false,
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Parse command line flags
	flag.BoolVar(&cfg.dryRun, "dry-run", true, "Enable dry-run mode (no actual deletion)")
	flag.DurationVar(&cfg.ageThreshold, "age", 1*clean.KeepDurationOneDay, "Age threshold for security groups (e.g. 24h)")
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
	deletedGroups, err := deleteUnusedSecurityGroups(ctx, client)
	if err != nil {
		log.Printf("Error deleting security groups: %v", err)
	}
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

func deleteUnusedSecurityGroups(ctx context.Context, client ec2Client) ([]string, error) {
	var (
		wg                       sync.WaitGroup
		deletedSecurityGroups    []string
		foundSecurityGroupChan   = make(chan types.SecurityGroup, SecurityGroupProcessChanSize)
		deletedSecurityGroupChan = make(chan string, SecurityGroupProcessChanSize)
		handlerWg                sync.WaitGroup
	)

	// Start worker pool
	log.Printf("👷 Creating %d workers\n", cfg.numWorkers)
	for i := 0; i < cfg.numWorkers; i++ {
		wg.Add(1)
		w := worker{
			id:                        i,
			wg:                        &wg,
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
		return nil, err
	}

	close(foundSecurityGroupChan)
	wg.Wait()
	close(deletedSecurityGroupChan)
	handlerWg.Wait()

	return deletedSecurityGroups, nil
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

	for {
		select {
		case securityGroup, ok := <-w.incomingSecurityGroupChan:
			if !ok {
				return
			}
			if err := w.handleSecurityGroup(ctx, client, securityGroup); err != nil {
				log.Printf("Worker %d: Error processing security group: %v", w.id, err)
			}
		case <-ctx.Done():
			log.Printf("Worker %d: Stopping due to context cancellation", w.id)
			return
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

	// Get fresh security group data in one call
	describeOutput, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		GroupIds: []string{sgID},
	})
	if err != nil {
		return fmt.Errorf("describing security group %s: %w", sgID, err)
	}

	if len(describeOutput.SecurityGroups) == 0 {
		return fmt.Errorf("security group %s not found", sgID)
	}

	sg := describeOutput.SecurityGroups[0]

	// Handle both ingress and egress rules concurrently
	var wg sync.WaitGroup
	var ingressErr, egressErr error

	if len(sg.IpPermissions) > 0 {
		if cfg.dryRun {
			log.Printf("🛑 Dry-Run: Would revoke %d ingress rules from security group: %s",
				len(sg.IpPermissions), sgID)
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := client.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{
					GroupId:       aws.String(sgID),
					IpPermissions: sg.IpPermissions,
				})
				if err != nil {
					ingressErr = fmt.Errorf("revoking ingress rules: %w", err)
				} else {
					log.Printf("✅ Revoked ingress rules from security group: %s", sgID)
				}
			}()
		}
	}

	if len(sg.IpPermissionsEgress) > 0 {
		if cfg.dryRun {
			log.Printf("🛑 Dry-Run: Would revoke %d egress rules from security group: %s",
				len(sg.IpPermissionsEgress), sgID)
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := client.RevokeSecurityGroupEgress(ctx, &ec2.RevokeSecurityGroupEgressInput{
					GroupId:       aws.String(sgID),
					IpPermissions: sg.IpPermissionsEgress,
				})
				if err != nil {
					egressErr = fmt.Errorf("revoking egress rules: %w", err)
				} else {
					log.Printf("✅ Revoked egress rules from security group: %s", sgID)
				}
			}()
		}
	}

	wg.Wait()

	if ingressErr != nil {
		return ingressErr
	}
	if egressErr != nil {
		return egressErr
	}

	return nil
}

func fetchAndProcessSecurityGroups(ctx context.Context, client ec2Client,
	securityGroupChan chan<- types.SecurityGroup) error {

	maxResults := int32(100) // AWS maximum allowed
	var nextToken *string
	describeCount := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			output, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
				MaxResults: aws.Int32(maxResults),
				NextToken:  nextToken,
			})
			if err != nil {
				return fmt.Errorf("describing security groups: %w", err)
			}

			log.Printf("🔍 Described %d times | Found %d security groups\n", describeCount, len(output.SecurityGroups))

			// Process in batches with context awareness
			for _, securityGroup := range output.SecurityGroups {
				select {
				case securityGroupChan <- securityGroup:
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			if output.NextToken == nil {
				break
			}

			nextToken = output.NextToken
			describeCount++
		}
	}

	return nil
}

func isSecurityGroupInUse(ctx context.Context, client ec2Client, securityGroupID string) (bool, error) {
	// Use a channel to handle concurrent checks
	resultChan := make(chan bool, 2)
	errChan := make(chan error, 2)

	// Check network interfaces concurrently
	go func() {
		output, err := client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("group-id"),
					Values: []string{securityGroupID},
				},
			},
		})
		if err != nil {
			errChan <- fmt.Errorf("describing network interfaces: %w", err)
			return
		}
		resultChan <- len(output.NetworkInterfaces) > 0
	}()

	// Check security group references concurrently
	go func() {
		output, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
		if err != nil {
			errChan <- fmt.Errorf("describing security groups: %w", err)
			return
		}

		for _, sg := range output.SecurityGroups {
			if *sg.GroupId == securityGroupID {
				continue
			}

			// Check both ingress and egress rules
			if isReferencedInRules(sg.IpPermissions, securityGroupID) ||
				isReferencedInRules(sg.IpPermissionsEgress, securityGroupID) {
				resultChan <- true
				return
			}
		}
		resultChan <- false
	}()

	// Wait for both checks
	for i := 0; i < 2; i++ {
		select {
		case err := <-errChan:
			return false, err
		case isUsed := <-resultChan:
			if isUsed {
				return true, nil
			}
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}

	return false, nil
}

func isReferencedInRules(permissions []types.IpPermission, securityGroupID string) bool {
	for _, permission := range permissions {
		for _, userIdGroupPair := range permission.UserIdGroupPairs {
			if userIdGroupPair.GroupId != nil && *userIdGroupPair.GroupId == securityGroupID {
				return true
			}
		}
	}
	return false
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
