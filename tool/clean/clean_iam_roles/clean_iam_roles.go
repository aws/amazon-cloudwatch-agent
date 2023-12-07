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

const retentionPeriod = clean.KeepDurationOneWeek

var (
	roleNamePrefixes = []string{
		"cwa-integ-assume-role",
		"cwagent-eks-Worker-Role",
		"cwagent-integ-test-task-role",
		"cwagent-integ-test-task-execution-role",
		"cwagent-operator-eks-Worker-Role",
		"cwagent-operator-helm-integ-Worker-Role",
	}
)

type iamClient interface {
	ListRoles(ctx context.Context, input *iam.ListRolesInput, optFns ...func(*iam.Options)) (*iam.ListRolesOutput, error)
	GetRole(ctx context.Context, input *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error)
	DeleteRole(ctx context.Context, input *iam.DeleteRoleInput, optFns ...func(*iam.Options)) (*iam.DeleteRoleOutput, error)
	ListAttachedRolePolicies(ctx context.Context, input *iam.ListAttachedRolePoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error)
	DetachRolePolicy(ctx context.Context, input *iam.DetachRolePolicyInput, optFns ...func(*iam.Options)) (*iam.DetachRolePolicyOutput, error)
	ListInstanceProfilesForRole(ctx context.Context, input *iam.ListInstanceProfilesForRoleInput, optFns ...func(*iam.Options)) (*iam.ListInstanceProfilesForRoleOutput, error)
	RemoveRoleFromInstanceProfile(ctx context.Context, input *iam.RemoveRoleFromInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.RemoveRoleFromInstanceProfileOutput, error)
	DeleteInstanceProfile(ctx context.Context, input *iam.DeleteInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.DeleteInstanceProfileOutput, error)
}

func main() {
	log.Print("Begin to clean IAM Roles")
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load AWS config: %v", err)
	}
	client := iam.NewFromConfig(cfg)
	if err = deleteRoles(ctx, client, getExpirationDate()); err != nil {
		log.Fatalf("errors cleaning: %v", err)
	}
}

func getExpirationDate() time.Time {
	return time.Now().UTC().Add(retentionPeriod)
}

func deleteRoles(ctx context.Context, client iamClient, expirationDate time.Time) error {
	var errs error
	var marker *string
	for {
		output, err := client.ListRoles(ctx, &iam.ListRolesInput{Marker: marker})
		if err != nil {
			return err
		}
		for _, role := range output.Roles {
			if hasPrefix(*role.RoleName) && expirationDate.After(*role.CreateDate) {
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

func deleteRole(ctx context.Context, client iamClient, role types.Role) error {
	lastUsed := "never"
	if role.RoleLastUsed != nil && role.RoleLastUsed.LastUsedDate != nil {
		lastUsed = fmt.Sprintf("%d days ago", int(time.Since(*role.RoleLastUsed.LastUsedDate).Hours()/24))
	}
	log.Printf("Trying to delete role (%q) last used %s", *role.RoleName, lastUsed)
	if err := detachPolicies(ctx, client, role); err != nil {
		return err
	}
	if err := deleteProfiles(ctx, client, role); err != nil {
		return err
	}
	if _, err := client.DeleteRole(ctx, &iam.DeleteRoleInput{RoleName: role.RoleName}); err != nil {
		log.Printf("Failed to delete role (%q): %v", *role.RoleName, err)
		return err
	}
	log.Printf("Deleted role (%q) successfully", *role.RoleName)
	return nil
}

func detachPolicies(ctx context.Context, client iamClient, role types.Role) error {
	var marker *string
	for {
		output, err := client.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{RoleName: role.RoleName, Marker: marker})
		if err != nil {
			return err
		}
		for _, policy := range output.AttachedPolicies {
			log.Printf("Trying to detach policy (%q) from role (%q)", *policy.PolicyName, *role.RoleName)
			if _, err = client.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{PolicyArn: policy.PolicyArn, RoleName: role.RoleName}); err != nil {
				log.Printf("Failed to detach policy (%q): %v", *policy.PolicyName, err)
				return err
			}
			log.Printf("Detached policy (%q) from role (%q) successfully", *policy.PolicyName, *role.RoleName)
		}
		if output.Marker == nil {
			break
		}
		marker = output.Marker
	}
	return nil
}

func deleteProfiles(ctx context.Context, client iamClient, role types.Role) error {
	var marker *string
	for {
		output, err := client.ListInstanceProfilesForRole(ctx, &iam.ListInstanceProfilesForRoleInput{RoleName: role.RoleName, Marker: marker})
		if err != nil {
			return err
		}
		for _, profile := range output.InstanceProfiles {
			if err = deleteProfile(ctx, client, role, profile); err != nil {
				return err
			}
		}
		if output.Marker == nil {
			break
		}
		marker = output.Marker
	}
	return nil
}

func deleteProfile(ctx context.Context, client iamClient, role types.Role, profile types.InstanceProfile) error {
	log.Printf("Trying to remove role (%q) from instance profile (%q)", *role.RoleName, *profile.InstanceProfileName)
	_, err := client.RemoveRoleFromInstanceProfile(ctx, &iam.RemoveRoleFromInstanceProfileInput{
		InstanceProfileName: profile.InstanceProfileName,
		RoleName:            role.RoleName,
	})
	if err != nil {
		return err
	}
	log.Printf("Removed role (%q) from instance profile (%q) successfully", *role.RoleName, *profile.InstanceProfileName)
	// profile's only role is about to be deleted, so delete it too
	if len(profile.Roles) == 1 {
		log.Printf("Trying to delete instance profile (%q) attached to role (%q)", *profile.InstanceProfileName, *role.RoleName)
		if _, err = client.DeleteInstanceProfile(ctx, &iam.DeleteInstanceProfileInput{InstanceProfileName: profile.InstanceProfileName}); err != nil {
			log.Printf("Failed to delete instance profile (%q): %v", *profile.InstanceProfileName, err)
			return err
		}
		log.Printf("Deleted instance profile (%q) successfully", *profile.InstanceProfileName)
	}
	return nil
}

func hasPrefix(roleName string) bool {
	for _, prefix := range roleNamePrefixes {
		if strings.HasPrefix(roleName, prefix) {
			return true
		}
	}
	return false
}
