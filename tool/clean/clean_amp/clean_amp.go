// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/amp"
	"github.com/aws/aws-sdk-go-v2/service/amp/types"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

// WorkspacesToClean lists the alias prefixes of AMP (Amazon Managed Service for
// Prometheus) workspaces created by the integration-test harness. The harness sets
// workspace_alias to "cwagent-integ-test-<testing_id>" in
// terraform/ec2/linux/main.tf (module "amp").
var WorkspacesToClean = []string{
	"cwagent-integ-test-",
}

var dryRun bool

// Clean AMP workspaces created by the integration tests if they have been open
// longer than one day. A test run creates a workspace, uses it for a few minutes,
// then destroys it via terraform. Workspaces left behind by cancelled or
// hard-killed jobs (whose terraform state is lost with the ephemeral runner)
// accumulate to the per-region 75-workspace quota; once at the cap every new run
// fails at CreateWorkspace with ServiceQuotaExceededException. The existing
// terraform destroy cleanup step cannot reclaim these because their state is gone,
// so this cleaner deletes them out-of-band via the AMP API.
func main() {
	flag.BoolVar(&dryRun, "dry-run", false, "Enable dry-run mode (no actual deletion)")
	flag.Parse()

	if err := cleanWorkspaces(); err != nil {
		log.Fatalf("errors cleaning %v", err)
	}
}

func cleanWorkspaces() error {
	log.Print("Begin to clean AMP workspaces")
	ctx := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	client := amp.NewFromConfig(defaultConfig)
	return terminateWorkspaces(ctx, client)
}

func aliasMatchesWorkspacesToClean(alias string, workspacesToClean []string) bool {
	for _, workspaceToClean := range workspacesToClean {
		if strings.HasPrefix(alias, workspaceToClean) {
			return true
		}
	}
	return false
}

func terminateWorkspaces(ctx context.Context, client *amp.Client) error {
	expirationDate := time.Now().UTC().Add(clean.KeepDurationOneDay)

	paginator := amp.NewListWorkspacesPaginator(client, &amp.ListWorkspacesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		for _, workspace := range page.Workspaces {
			workspaceID := aws.ToString(workspace.WorkspaceId)
			alias := aws.ToString(workspace.Alias)

			if workspace.Status == nil || workspace.Status.StatusCode != types.WorkspaceStatusCodeActive {
				log.Printf("Ignoring workspace %s (alias %q) since it is not ACTIVE", workspaceID, alias)
				continue
			}
			if workspace.CreatedAt == nil || !expirationDate.After(*workspace.CreatedAt) {
				log.Printf("Ignoring workspace %s (alias %q) with create-date %v since it was created in the last %s",
					workspaceID, alias, aws.ToTime(workspace.CreatedAt), clean.KeepDurationOneDay)
				continue
			}
			if !aliasMatchesWorkspacesToClean(alias, WorkspacesToClean) {
				log.Printf("Ignoring workspace %s since alias %q does not match any clean prefix", workspaceID, alias)
				continue
			}
			if dryRun {
				log.Printf("Dry-Run: would delete workspace %s (alias %q, create-date %v)",
					workspaceID, alias, aws.ToTime(workspace.CreatedAt))
				continue
			}
			log.Printf("Try to delete workspace %s (alias %q, create-date %v)",
				workspaceID, alias, aws.ToTime(workspace.CreatedAt))
			if _, err := client.DeleteWorkspace(ctx, &amp.DeleteWorkspaceInput{WorkspaceId: workspace.WorkspaceId}); err != nil {
				log.Printf("could not delete workspace %s err %v", workspaceID, err)
			}
		}
	}
	return nil
}
