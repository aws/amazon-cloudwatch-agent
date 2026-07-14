// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

// make this configurable or construct based on some configs?
const betaEksEndpoint = "https://api.beta.us-west-2.wesley.amazonaws.com"

// reapingTagKey marks a VPC the reaper has begun tearing down. It lets a later
// run resume teardown of a VPC whose NAT gateways/endpoints (the age signals)
// were already deleted, instead of skipping it as "age cannot be determined".
const reapingTagKey = "cleaner:reaping"

var (
	// clustersToClean is the allowlist of EKS cluster name prefixes the cleaner
	// is permitted to delete.
	clustersToClean = []string{
		"cwagent-eks-integ-",
		"cwagent-operator-helm-integ-",
		"cwagent-helm-chart-integ-",
		"cwagent-operator-eks-integ-",
		"cwagent-monitoring-config-e2e-eks-",
		"cwagent-addon-eks-integ-",
	}

	// testVpcNamePrefixes are the tag:Name prefixes of the dedicated VPCs that
	// some tests stand up (private subnets + NAT gateway + interface/gateway VPC
	// endpoints). A partially-failed `terraform destroy` strands the whole stack
	// — leaking the VPC, NAT gateway, Elastic IP and the quota-limited S3 gateway
	// endpoint — so the reaper below reclaims the entire orphaned stack.
	testVpcNamePrefixes = []string{
		"efa-test-vpc-",
	}

	// Poll budgets for resources that delete asynchronously and whose ENIs must
	// clear before dependents (subnets, security groups, VPC) can be removed.
	endpointDeleteTimeout = 3 * time.Minute
	natDeleteTimeout      = 6 * time.Minute
	pollInterval          = 15 * time.Second
)

// Clean eks clusters if they have been open longer than the keep-duration, then
// reap any orphaned test VPCs left behind by failed terraform destroys.
func main() {
	clean.RegisterCommonFlags()
	flag.Parse()

	ctx := context.Background()
	if err := cleanCluster(ctx); err != nil {
		log.Fatalf("errors cleaning %v", err)
	}
	// Orphaned-VPC reaping is best-effort: a failure here must not mask a
	// successful cluster clean, and the next daily run will retry.
	if err := cleanOrphanedVpcResources(ctx); err != nil {
		log.Printf("errors cleaning orphaned vpc resources %v", err)
	}
}

func cleanCluster(ctx context.Context) error {
	log.Print("Begin to clean EKS Clusters")
	defaultConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	eksClient := eks.NewFromConfig(defaultConfig)
	terminateClusters(ctx, eksClient)

	// delete beta clusters
	betaConfig, err := config.LoadDefaultConfig(ctx, config.WithEndpointResolverWithOptions(eksBetaEndpointResolver()))
	if err != nil {
		return err
	}
	betaClient := eks.NewFromConfig(betaConfig)
	terminateClusters(ctx, betaClient)

	return nil
}

func eksBetaEndpointResolver() aws.EndpointResolverWithOptionsFunc {
	return func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		endpoint, err := eks.NewDefaultEndpointResolver().ResolveEndpoint(region, eks.EndpointResolverOptions{})
		if err != nil {
			return aws.Endpoint{}, err
		}
		endpoint.URL = betaEksEndpoint
		return endpoint, nil
	}
}

func clusterNameMatchesClustersToClean(clusterName string, clustersToClean []string) bool {
	for _, clusterToClean := range clustersToClean {
		if strings.HasPrefix(clusterName, clusterToClean) {
			return true
		}
	}
	return false
}

func terminateClusters(ctx context.Context, client *eks.Client) {
	expirationDateCluster := time.Now().UTC().Add(clean.KeepDurationOneDay)

	var nextToken *string
	for {
		clusters, err := client.ListClusters(ctx, &eks.ListClustersInput{NextToken: nextToken})
		if err != nil {
			// Non-fatal: a beta-endpoint outage should not abort the whole run.
			log.Printf("could not get cluster list: %v", err)
			return
		}
		for _, cluster := range clusters.Clusters {
			describeClusterOutput, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(cluster)})
			if err != nil {
				log.Printf("could not describe cluster %s err %v", cluster, err)
				continue
			}
			if describeClusterOutput.Cluster == nil || describeClusterOutput.Cluster.CreatedAt == nil {
				log.Printf("Ignoring cluster %s: missing cluster metadata", cluster)
				continue
			}
			if !expirationDateCluster.After(*describeClusterOutput.Cluster.CreatedAt) {
				log.Printf("Ignoring cluster %s with a launch-date %s since it was created in the last %s", cluster, *describeClusterOutput.Cluster.CreatedAt, clean.KeepDurationOneDay)
				continue
			}
			if !clusterNameMatchesClustersToClean(*describeClusterOutput.Cluster.Name, clustersToClean) {
				log.Printf("Ignoring cluster %s since it doesnt match any of the clean regexes", cluster)
				continue
			}
			if clean.Skip("delete cluster %s launch-date %s", cluster, *describeClusterOutput.Cluster.CreatedAt) {
				continue
			}
			log.Printf("Try to delete cluster %s launch-date %s", cluster, *describeClusterOutput.Cluster.CreatedAt)
			nodeGroupOutput, err := client.ListNodegroups(ctx, &eks.ListNodegroupsInput{ClusterName: aws.String(cluster)})
			if err != nil {
				log.Printf("could not query node groups cluster %s err %v", cluster, err)
			} else {
				// it takes about 5 minutes to delete node groups
				// it will fail to delete cluster if we need to delete node groups
				// this will delete the cluster on next run the next day
				// I do not want to wait for node groups to be deleted
				// as it will increase the runtime of this cleaner
				for _, nodegroup := range nodeGroupOutput.Nodegroups {
					deleteNodegroupInput := eks.DeleteNodegroupInput{
						ClusterName:   aws.String(cluster),
						NodegroupName: aws.String(nodegroup),
					}
					if _, err := client.DeleteNodegroup(ctx, &deleteNodegroupInput); err != nil {
						log.Printf("could not delete node groups %s cluster %s err %v", nodegroup, cluster, err)
					}
				}
			}
			deleteClusterInput := eks.DeleteClusterInput{Name: aws.String(cluster)}
			if _, err := client.DeleteCluster(ctx, &deleteClusterInput); err != nil {
				log.Printf("could not delete cluster %s err %v", cluster, err)
			}
		}
		if clusters.NextToken == nil {
			return
		}
		nextToken = clusters.NextToken
	}
}

// cleanOrphanedVpcResources finds the dedicated test VPCs named by
// testVpcNamePrefixes that are no longer backing any live EKS cluster and tears
// the entire VPC stack down: VPC endpoints, NAT gateways, Elastic IPs, internet
// gateways, subnets, route tables, security groups and finally the VPC itself.
func cleanOrphanedVpcResources(ctx context.Context) error {
	log.Print("Begin to clean orphaned test VPCs")
	defaultConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	ec2Client := ec2.NewFromConfig(defaultConfig)
	eksClient := eks.NewFromConfig(defaultConfig)

	betaConfig, err := config.LoadDefaultConfig(ctx, config.WithEndpointResolverWithOptions(eksBetaEndpointResolver()))
	if err != nil {
		return err
	}
	betaEksClient := eks.NewFromConfig(betaConfig)

	// A VPC that still backs a live EKS cluster must be left alone. Build the
	// in-use set from BOTH the prod and beta EKS endpoints (mirroring
	// terminateClusters). If we cannot build a complete in-use set (e.g. a
	// DescribeCluster call fails), we abort the VPC pass rather than risk
	// deleting a VPC that a cluster we failed to inspect still depends on.
	// Fail closed: the next daily run retries.
	inUse, err := vpcsInUseByClusters(ctx, eksClient)
	if err != nil {
		return fmt.Errorf("determining in-use VPCs (prod): %w", err)
	}
	// Beta enumeration is best-effort: the beta endpoint is internal and may be
	// unreachable from the runner. A failure here must not disable the whole
	// (prod) VPC pass. EFA test VPCs back prod clusters, so the residual risk of
	// skipping beta is negligible; prod enumeration above stays fail-closed.
	if betaInUse, err := vpcsInUseByClusters(ctx, betaEksClient); err != nil {
		log.Printf("could not determine beta in-use VPCs (continuing with prod only): %v", err)
	} else {
		for vpcID := range betaInUse {
			inUse[vpcID] = true
		}
	}

	nameFilters := make([]string, 0, len(testVpcNamePrefixes))
	for _, p := range testVpcNamePrefixes {
		nameFilters = append(nameFilters, p+"*")
	}
	var vpcs []ec2types.Vpc
	var vpcNextToken *string
	for {
		vpcsOut, err := ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
			Filters:   []ec2types.Filter{{Name: aws.String("tag:Name"), Values: nameFilters}},
			NextToken: vpcNextToken,
		})
		if err != nil {
			return fmt.Errorf("describing test VPCs: %w", err)
		}
		vpcs = append(vpcs, vpcsOut.Vpcs...)
		if vpcsOut.NextToken == nil {
			break
		}
		vpcNextToken = vpcsOut.NextToken
	}

	for _, vpc := range vpcs {
		vpcID := aws.ToString(vpc.VpcId)
		name := vpcNameTag(vpc)

		// Gate 1: never touch a VPC backing a live cluster.
		if inUse[vpcID] {
			log.Printf("Ignoring VPC %s (%s): still backs a live EKS cluster", vpcID, name)
			continue
		}
		// Gate 2: never touch a VPC that still has instances in any
		// non-terminated state (running/pending/stopping/stopped/shutting-down).
		active, err := vpcHasActiveInstances(ctx, ec2Client, vpcID)
		if err != nil {
			log.Printf("could not check instances for VPC %s: %v", vpcID, err)
			continue
		}
		if active {
			log.Printf("Ignoring VPC %s (%s): has active (non-terminated) instances", vpcID, name)
			continue
		}
		// Gate 3: only reap a VPC we can prove is old enough. VPCs carry no
		// creation timestamp, so age is derived from the newest NAT gateway or
		// VPC endpoint in the VPC. If neither exists we cannot determine age and
		// fail safe (skip) — this prevents deleting a stack mid-terraform-apply
		// whose NAT gateway/endpoints have not been created yet.
		//
		// Exception: if the VPC already carries a reaping marker WITH a valid
		// timestamp (written by this reaper at teardown start), a prior run
		// approved it and began teardown — which deletes the NAT/endpoint age
		// signals — so we resume rather than strand a partially-torn-down VPC.
		// A malformed value is ignored so an unrelated/hand-set tag cannot
		// silently disable the age gate.
		reaping := false
		if ts := vpcTag(vpc, reapingTagKey); ts != "" {
			if _, perr := time.Parse(time.RFC3339, ts); perr == nil {
				reaping = true
			} else {
				log.Printf("VPC %s has malformed %s tag %q; ignoring it", vpcID, reapingTagKey, ts)
			}
		}
		if !reaping {
			eligible, err := vpcReapEligibleByAge(ctx, ec2Client, vpcID)
			if err != nil {
				log.Printf("could not determine age for VPC %s: %v", vpcID, err)
				continue
			}
			if !eligible {
				log.Printf("Ignoring VPC %s (%s): too new or age cannot be determined", vpcID, name)
				continue
			}
		}

		if err := deleteVpcAndDependencies(ctx, ec2Client, vpcID, name); err != nil {
			log.Printf("could not fully tear down VPC %s (%s): %v", vpcID, name, err)
		}
	}
	return nil
}

// deleteVpcAndDependencies removes an orphaned VPC's resources in dependency
// order. Each step is best-effort: a failure is logged and the teardown
// proceeds, so the next daily run can retry whatever remains.
func deleteVpcAndDependencies(ctx context.Context, ec2Client *ec2.Client, vpcID, name string) error {
	if clean.DryRun {
		return dryRunDescribeVpc(ctx, ec2Client, vpcID, name)
	}
	log.Printf("Tearing down orphaned VPC %s (%s)", vpcID, name)

	// 0. Mark the VPC as being reaped. If a later step fails and the VPC
	//    survives, the next run resumes teardown (via the reaping-tag gate)
	//    even though the NAT gateways/endpoints it derives age from are gone.
	//    Best-effort.
	if _, err := ec2Client.CreateTags(ctx, &ec2.CreateTagsInput{
		Resources: []string{vpcID},
		Tags:      []ec2types.Tag{{Key: aws.String(reapingTagKey), Value: aws.String(time.Now().UTC().Format(time.RFC3339))}},
	}); err != nil {
		log.Printf("could not tag VPC %s as reaping: %v", vpcID, err)
	}

	// 1. VPC endpoints (interface endpoints own ENIs that block subnet/SG/VPC
	//    deletion; the S3 gateway endpoint is the quota-limited resource).
	deleteVpcEndpoints(ctx, ec2Client, vpcID)
	waitVpcEndpointsGone(ctx, ec2Client, vpcID)

	// 2. NAT gateways (own ENIs + hold the Elastic IPs). Collect the allocation
	//    IDs so we can release the EIPs once the gateways are gone.
	allocationIDs := deleteNatGateways(ctx, ec2Client, vpcID)
	waitNatGatewaysGone(ctx, ec2Client, vpcID)

	// 3. Release the Elastic IPs the NAT gateways were using.
	releaseAddresses(ctx, ec2Client, allocationIDs)

	// 4. Internet gateways must be detached before deletion.
	deleteInternetGateways(ctx, ec2Client, vpcID)

	// 5. Delete leftover 'available' (detached) ENIs. These are the usual reason
	//    a terraform destroy stalls on these VPCs, and they block subnet
	//    deletion. Only detached interfaces are removed.
	deleteAvailableNetworkInterfaces(ctx, ec2Client, vpcID)

	// Re-check for active instances before deleting subnets — the gates ran
	// before the multi-minute endpoint/NAT waits above, so this closes the
	// TOCTOU window where a workload began launching into the VPC meanwhile.
	// The VPC keeps its reaping tag, so a later run resumes if we abort here.
	if active, err := vpcHasActiveInstances(ctx, ec2Client, vpcID); err != nil {
		return fmt.Errorf("re-checking instances for VPC %s: %w", vpcID, err)
	} else if active {
		return fmt.Errorf("aborting teardown of VPC %s: active instances appeared during teardown", vpcID)
	}

	// 6. Subnets (deleting a subnet also drops its route-table associations).
	deleteSubnets(ctx, ec2Client, vpcID)

	// 7. Non-main route tables.
	deleteRouteTables(ctx, ec2Client, vpcID)

	// 8. Non-default security groups.
	deleteSecurityGroups(ctx, ec2Client, vpcID)

	// 9. Finally the VPC itself.
	if _, err := ec2Client.DeleteVpc(ctx, &ec2.DeleteVpcInput{VpcId: aws.String(vpcID)}); err != nil {
		return fmt.Errorf("deleting VPC %s: %w", vpcID, err)
	}
	log.Printf("Deleted orphaned VPC %s (%s)", vpcID, name)
	return nil
}

// The per-VPC delete helpers below each issue a single (unpaginated) Describe.
// Orphaned test VPCs hold only a handful of each resource, so one page always
// suffices; add pagination if that assumption ever changes.
func deleteVpcEndpoints(ctx context.Context, ec2Client *ec2.Client, vpcID string) {
	out, err := ec2Client.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{
		Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
	})
	if err != nil {
		log.Printf("could not describe endpoints for VPC %s: %v", vpcID, err)
		return
	}
	ids := make([]string, 0, len(out.VpcEndpoints))
	for _, ep := range out.VpcEndpoints {
		if ep.State == ec2types.StateDeleted || ep.State == ec2types.StateDeleting {
			continue
		}
		ids = append(ids, aws.ToString(ep.VpcEndpointId))
	}
	if len(ids) == 0 {
		return
	}
	log.Printf("Deleting %d endpoint(s) %v in VPC %s", len(ids), ids, vpcID)
	res, err := ec2Client.DeleteVpcEndpoints(ctx, &ec2.DeleteVpcEndpointsInput{VpcEndpointIds: ids})
	if err != nil {
		log.Printf("could not delete endpoints in VPC %s: %v", vpcID, err)
		return
	}
	for _, un := range res.Unsuccessful {
		msg := ""
		if un.Error != nil {
			msg = aws.ToString(un.Error.Message)
		}
		log.Printf("failed to delete endpoint %s in VPC %s: %s", aws.ToString(un.ResourceId), vpcID, msg)
	}
}

func deleteNatGateways(ctx context.Context, ec2Client *ec2.Client, vpcID string) []string {
	out, err := ec2Client.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{
		Filter: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
	})
	if err != nil {
		log.Printf("could not describe NAT gateways for VPC %s: %v", vpcID, err)
		return nil
	}
	var allocationIDs []string
	for _, gw := range out.NatGateways {
		// Collect EIP allocation IDs from every NAT gateway that still reports
		// them — including ones already deleting/deleted (AWS lists a deleted
		// NAT for ~1h). This lets a later run still release the EIP when NAT
		// deletion outlasted this run's wait budget.
		for _, addr := range gw.NatGatewayAddresses {
			if id := aws.ToString(addr.AllocationId); id != "" {
				allocationIDs = append(allocationIDs, id)
			}
		}
		if gw.State == ec2types.NatGatewayStateDeleted || gw.State == ec2types.NatGatewayStateDeleting {
			continue
		}
		id := aws.ToString(gw.NatGatewayId)
		log.Printf("Deleting NAT gateway %s in VPC %s", id, vpcID)
		if _, err := ec2Client.DeleteNatGateway(ctx, &ec2.DeleteNatGatewayInput{NatGatewayId: aws.String(id)}); err != nil {
			log.Printf("could not delete NAT gateway %s: %v", id, err)
		}
	}
	return allocationIDs
}

func releaseAddresses(ctx context.Context, ec2Client *ec2.Client, allocationIDs []string) {
	for _, id := range allocationIDs {
		log.Printf("Releasing Elastic IP %s", id)
		if _, err := ec2Client.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{AllocationId: aws.String(id)}); err != nil {
			log.Printf("could not release Elastic IP %s: %v", id, err)
		}
	}
}

func deleteInternetGateways(ctx context.Context, ec2Client *ec2.Client, vpcID string) {
	out, err := ec2Client.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{
		Filters: []ec2types.Filter{{Name: aws.String("attachment.vpc-id"), Values: []string{vpcID}}},
	})
	if err != nil {
		log.Printf("could not describe internet gateways for VPC %s: %v", vpcID, err)
		return
	}
	for _, igw := range out.InternetGateways {
		id := aws.ToString(igw.InternetGatewayId)
		if _, err := ec2Client.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
			InternetGatewayId: aws.String(id),
			VpcId:             aws.String(vpcID),
		}); err != nil {
			log.Printf("could not detach internet gateway %s from VPC %s: %v", id, vpcID, err)
		}
		log.Printf("Deleting internet gateway %s", id)
		if _, err := ec2Client.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{InternetGatewayId: aws.String(id)}); err != nil {
			log.Printf("could not delete internet gateway %s: %v", id, err)
		}
	}
}

func deleteAvailableNetworkInterfaces(ctx context.Context, ec2Client *ec2.Client, vpcID string) {
	out, err := ec2Client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("vpc-id"), Values: []string{vpcID}},
			{Name: aws.String("status"), Values: []string{"available"}},
		},
	})
	if err != nil {
		log.Printf("could not describe network interfaces for VPC %s: %v", vpcID, err)
		return
	}
	for _, eni := range out.NetworkInterfaces {
		id := aws.ToString(eni.NetworkInterfaceId)
		log.Printf("Deleting leftover network interface %s", id)
		if _, err := ec2Client.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{NetworkInterfaceId: aws.String(id)}); err != nil {
			log.Printf("could not delete network interface %s: %v", id, err)
		}
	}
}

func deleteSubnets(ctx context.Context, ec2Client *ec2.Client, vpcID string) {
	out, err := ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
	})
	if err != nil {
		log.Printf("could not describe subnets for VPC %s: %v", vpcID, err)
		return
	}
	for _, sn := range out.Subnets {
		id := aws.ToString(sn.SubnetId)
		log.Printf("Deleting subnet %s", id)
		if _, err := ec2Client.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{SubnetId: aws.String(id)}); err != nil {
			log.Printf("could not delete subnet %s: %v", id, err)
		}
	}
}

func deleteRouteTables(ctx context.Context, ec2Client *ec2.Client, vpcID string) {
	out, err := ec2Client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
	})
	if err != nil {
		log.Printf("could not describe route tables for VPC %s: %v", vpcID, err)
		return
	}
	for _, rt := range out.RouteTables {
		id := aws.ToString(rt.RouteTableId)
		main := false
		for _, assoc := range rt.Associations {
			if aws.ToBool(assoc.Main) {
				main = true
				continue
			}
			if aid := aws.ToString(assoc.RouteTableAssociationId); aid != "" {
				if _, err := ec2Client.DisassociateRouteTable(ctx, &ec2.DisassociateRouteTableInput{AssociationId: aws.String(aid)}); err != nil {
					log.Printf("could not disassociate route table %s (assoc %s): %v", id, aid, err)
				}
			}
		}
		// The main route table is deleted implicitly with the VPC.
		if main {
			continue
		}
		log.Printf("Deleting route table %s", id)
		if _, err := ec2Client.DeleteRouteTable(ctx, &ec2.DeleteRouteTableInput{RouteTableId: aws.String(id)}); err != nil {
			log.Printf("could not delete route table %s: %v", id, err)
		}
	}
}

func deleteSecurityGroups(ctx context.Context, ec2Client *ec2.Client, vpcID string) {
	out, err := ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
	})
	if err != nil {
		log.Printf("could not describe security groups for VPC %s: %v", vpcID, err)
		return
	}

	// Two passes: first revoke all ingress/egress rules on every non-default SG,
	// then delete them. A single pass fails with DependencyViolation when SGs
	// reference each other — an SG cannot be deleted while another SG's rule
	// still references it — which would leave the VPC un-reapable.
	var deletable []ec2types.SecurityGroup
	for _, sg := range out.SecurityGroups {
		// The default security group cannot be deleted; it goes with the VPC.
		if aws.ToString(sg.GroupName) == "default" {
			continue
		}
		id := aws.ToString(sg.GroupId)
		if len(sg.IpPermissions) > 0 {
			if _, err := ec2Client.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{
				GroupId:       aws.String(id),
				IpPermissions: sg.IpPermissions,
			}); err != nil {
				log.Printf("could not revoke ingress on security group %s: %v", id, err)
			}
		}
		if len(sg.IpPermissionsEgress) > 0 {
			if _, err := ec2Client.RevokeSecurityGroupEgress(ctx, &ec2.RevokeSecurityGroupEgressInput{
				GroupId:       aws.String(id),
				IpPermissions: sg.IpPermissionsEgress,
			}); err != nil {
				log.Printf("could not revoke egress on security group %s: %v", id, err)
			}
		}
		deletable = append(deletable, sg)
	}
	for _, sg := range deletable {
		id := aws.ToString(sg.GroupId)
		log.Printf("Deleting security group %s (%s)", id, aws.ToString(sg.GroupName))
		if _, err := ec2Client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{GroupId: aws.String(id)}); err != nil {
			log.Printf("could not delete security group %s: %v", id, err)
		}
	}
}

func waitVpcEndpointsGone(ctx context.Context, ec2Client *ec2.Client, vpcID string) {
	deadline := time.Now().Add(endpointDeleteTimeout)
	for {
		out, err := ec2Client.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{
			Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
		})
		if err != nil {
			log.Printf("could not poll endpoints for VPC %s: %v", vpcID, err)
			return
		}
		remaining := 0
		for _, ep := range out.VpcEndpoints {
			if ep.State != ec2types.StateDeleted {
				remaining++
			}
		}
		if remaining == 0 {
			return
		}
		if time.Now().After(deadline) {
			log.Printf("timed out waiting for %d endpoint(s) in VPC %s to delete", remaining, vpcID)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

func waitNatGatewaysGone(ctx context.Context, ec2Client *ec2.Client, vpcID string) {
	deadline := time.Now().Add(natDeleteTimeout)
	for {
		out, err := ec2Client.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{
			Filter: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
		})
		if err != nil {
			log.Printf("could not poll NAT gateways for VPC %s: %v", vpcID, err)
			return
		}
		remaining := 0
		for _, gw := range out.NatGateways {
			if gw.State != ec2types.NatGatewayStateDeleted {
				remaining++
			}
		}
		if remaining == 0 {
			return
		}
		if time.Now().After(deadline) {
			log.Printf("timed out waiting for %d NAT gateway(s) in VPC %s to delete", remaining, vpcID)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

func dryRunDescribeVpc(ctx context.Context, ec2Client *ec2.Client, vpcID, name string) error {
	log.Printf("Dry-Run: would tear down orphaned VPC %s (%s):", vpcID, name)

	if out, err := ec2Client.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{
		Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
	}); err == nil {
		for _, ep := range out.VpcEndpoints {
			log.Printf("  - endpoint %s (%s)", aws.ToString(ep.VpcEndpointId), aws.ToString(ep.ServiceName))
		}
	}
	if out, err := ec2Client.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{
		Filter: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
	}); err == nil {
		for _, gw := range out.NatGateways {
			if gw.State == ec2types.NatGatewayStateDeleted {
				continue
			}
			for _, addr := range gw.NatGatewayAddresses {
				if id := aws.ToString(addr.AllocationId); id != "" {
					log.Printf("  - Elastic IP %s (via NAT %s)", id, aws.ToString(gw.NatGatewayId))
				}
			}
			log.Printf("  - NAT gateway %s", aws.ToString(gw.NatGatewayId))
		}
	}
	if out, err := ec2Client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("vpc-id"), Values: []string{vpcID}},
			{Name: aws.String("status"), Values: []string{"available"}},
		},
	}); err == nil {
		for _, eni := range out.NetworkInterfaces {
			log.Printf("  - network interface %s", aws.ToString(eni.NetworkInterfaceId))
		}
	}
	if out, err := ec2Client.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{
		Filters: []ec2types.Filter{{Name: aws.String("attachment.vpc-id"), Values: []string{vpcID}}},
	}); err == nil {
		for _, igw := range out.InternetGateways {
			log.Printf("  - internet gateway %s", aws.ToString(igw.InternetGatewayId))
		}
	}
	if out, err := ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
	}); err == nil {
		for _, sn := range out.Subnets {
			log.Printf("  - subnet %s", aws.ToString(sn.SubnetId))
		}
	}
	if out, err := ec2Client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
	}); err == nil {
		for _, rt := range out.RouteTables {
			isMain := false
			for _, assoc := range rt.Associations {
				if aws.ToBool(assoc.Main) {
					isMain = true
				}
			}
			if !isMain {
				log.Printf("  - route table %s", aws.ToString(rt.RouteTableId))
			}
		}
	}
	if out, err := ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
	}); err == nil {
		for _, sg := range out.SecurityGroups {
			if aws.ToString(sg.GroupName) != "default" {
				log.Printf("  - security group %s (%s)", aws.ToString(sg.GroupId), aws.ToString(sg.GroupName))
			}
		}
	}
	log.Printf("  - VPC %s", vpcID)
	return nil
}

func vpcsInUseByClusters(ctx context.Context, client *eks.Client) (map[string]bool, error) {
	inUse := make(map[string]bool)
	var nextToken *string
	for {
		out, err := client.ListClusters(ctx, &eks.ListClustersInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, name := range out.Clusters {
			desc, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(name)})
			if err != nil {
				var notFound *ekstypes.ResourceNotFoundException
				if errors.As(err, &notFound) {
					// Cluster is being deleted (e.g. by terminateClusters, which ran just
					// before) — it no longer holds a VPC, so skip it rather than aborting
					// the whole reap pass. Genuine errors still fail closed below.
					continue
				}
				return nil, fmt.Errorf("describing cluster %s: %w", name, err)
			}
			if desc.Cluster != nil && desc.Cluster.ResourcesVpcConfig != nil {
				if vpcID := aws.ToString(desc.Cluster.ResourcesVpcConfig.VpcId); vpcID != "" {
					inUse[vpcID] = true
				}
			}
		}
		if out.NextToken == nil {
			return inUse, nil
		}
		nextToken = out.NextToken
	}
}

// vpcHasActiveInstances reports whether the VPC contains any instance that is
// not fully terminated. Stopped/stopping/shutting-down instances still occupy
// the VPC (ENIs, subnets) and signal the stack is not truly orphaned.
func vpcHasActiveInstances(ctx context.Context, client *ec2.Client, vpcID string) (bool, error) {
	var nextToken *string
	for {
		out, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			Filters: []ec2types.Filter{
				{Name: aws.String("vpc-id"), Values: []string{vpcID}},
				{Name: aws.String("instance-state-name"), Values: []string{"running", "pending", "stopping", "stopped", "shutting-down"}},
			},
			NextToken: nextToken,
		})
		if err != nil {
			return false, err
		}
		for _, r := range out.Reservations {
			if len(r.Instances) > 0 {
				return true, nil
			}
		}
		if out.NextToken == nil {
			return false, nil
		}
		nextToken = out.NextToken
	}
}

// vpcReapEligibleByAge reports whether the VPC is old enough to reap. VPCs have
// no creation timestamp, so age is derived from the newest non-deleted NAT
// gateway or VPC endpoint in the VPC. The VPC is eligible only if that newest
// signal is older than the keep-duration. When no age signal exists the
// function returns false (fail safe): a VPC we cannot age — e.g. one still
// mid-`terraform apply` before its NAT gateway/endpoints exist — is never
// reaped.
func vpcReapEligibleByAge(ctx context.Context, client *ec2.Client, vpcID string) (bool, error) {
	// KeepDurationOneDay is negative, so expiration is "now minus one day".
	expiration := time.Now().UTC().Add(clean.KeepDurationOneDay)
	var newest *time.Time
	consider := func(t *time.Time) {
		if t == nil {
			return
		}
		if newest == nil || t.After(*newest) {
			newest = t
		}
	}

	natOut, err := client.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{
		Filter: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
	})
	if err != nil {
		return false, err
	}
	for _, gw := range natOut.NatGateways {
		if gw.State == ec2types.NatGatewayStateDeleted {
			continue
		}
		consider(gw.CreateTime)
	}

	epOut, err := client.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{
		Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
	})
	if err != nil {
		return false, err
	}
	for _, ep := range epOut.VpcEndpoints {
		if ep.State == ec2types.StateDeleted {
			continue
		}
		consider(ep.CreationTimestamp)
	}

	if newest == nil {
		return false, nil // no age signal — fail safe
	}
	return !newest.After(expiration), nil
}

func vpcTag(vpc ec2types.Vpc, key string) string {
	for _, tag := range vpc.Tags {
		if aws.ToString(tag.Key) == key {
			return aws.ToString(tag.Value)
		}
	}
	return ""
}

func vpcNameTag(vpc ec2types.Vpc) string {
	return vpcTag(vpc, "Name")
}
