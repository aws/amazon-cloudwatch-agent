// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

var (
	roleNamePrefix = "cwagent-"
)

func main() {
	log.Print("Begin to clean IAM Roles")
	expirationDate := time.Now().UTC().Add(clean.KeepDurationSixtyDay)
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load AWS config: %v", err)
	}
	client := iam.NewFromConfig(cfg)
	if err = deleteRoles(ctx, client, expirationDate); err != nil {
		log.Fatalf("errors cleaning: %v", err)
	}
}

func deleteRoles(ctx context.Context, client *iam.Client, expirationDate time.Time) error {
	var errs error
	var marker *string
	for {
		output, err := client.ListRoles(ctx, &iam.ListRolesInput{Marker: marker})
		if err != nil {
			return err
		}
		for _, role := range output.Roles {
			if strings.HasPrefix(*role.RoleName, roleNamePrefix) && expirationDate.After(*role.CreateDate) {
				var getRoleOutput *iam.GetRoleOutput
				getRoleOutput, err = client.GetRole(ctx, &iam.GetRoleInput{RoleName: role.RoleName})
				if err != nil {
					return err
				}
				role = *getRoleOutput.Role
				if role.RoleLastUsed == nil || role.RoleLastUsed.LastUsedDate == nil || expirationDate.After(*role.RoleLastUsed.LastUsedDate) {
					errs = errors.Join(errs, deleteRole(ctx, client, role))
				}
			}
		}
		if output.Marker == nil {
			break
		}
		marker = output.Marker
	}
	return errs
}

func deleteRole(ctx context.Context, client *iam.Client, role types.Role) error {
	lastUsed := "never"
	if role.RoleLastUsed != nil && role.RoleLastUsed.LastUsedDate != nil {
		lastUsed = fmt.Sprintf("%d days ago", int(time.Since(*role.RoleLastUsed.LastUsedDate).Hours()/24))
	}
	log.Printf("Trying to delete role (%q) last used %s", *role.RoleName, lastUsed)
	if err := detachPolicies(ctx, client, role); err != nil {
		return err
	}
	if _, err := client.DeleteRole(ctx, &iam.DeleteRoleInput{RoleName: role.RoleName}); err != nil {
		return err
	}
	log.Printf("Deleted role (%q) successfully", *role.RoleName)
	return nil
}

func detachPolicies(ctx context.Context, client *iam.Client, role types.Role) error {
	var marker *string
	for {
		output, err := client.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{RoleName: role.RoleName, Marker: marker})
		if err != nil {
			return err
		}
		for _, policy := range output.AttachedPolicies {
			if _, err = client.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{PolicyArn: policy.PolicyArn, RoleName: role.RoleName}); err != nil {
				return fmt.Errorf("unable to detach policy (%q) from role (%q): %w", *policy.PolicyName, *role.RoleName, err)
			}
		}
		if output.Marker == nil {
			break
		}
		marker = output.Marker
	}
	return nil
}
