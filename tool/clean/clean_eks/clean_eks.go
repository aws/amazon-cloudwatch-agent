// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

// make this configurable or construct based on some configs?
const betaEksEndpoint = "https://api.beta.us-west-2.wesley.amazonaws.com"

// Bound how long we wait for a cluster's node groups to finish deleting before
// attempting to delete the cluster itself. A cluster cannot be deleted while it
// still has node groups, so if we do not wait the DeleteCluster call fails and
// the cluster (and its VPC/ELB/EBS/ENI dependents) lingers until a later run.
const nodegroupDeleteTimeout = 20 * time.Minute

var (
	ClustersToClean = []string{
		"cwagent-eks-integ-",
		"cwagent-operator-helm-integ-",
		"cwagent-helm-chart-integ-",
		"cwagent-operator-eks-integ-",
		"cwagent-monitoring-config-e2e-eks-",
		"cwagent-addon-eks-integ-",
	}
)

// Clean eks clusters if they have been open longer than 1 day
func main() {
	err := cleanCluster()
	if err != nil {
		log.Fatalf("errors cleaning %v", err)
	}
}

func cleanCluster() error {
	log.Print("Begin to clean EKS Clusters")
	ctx := context.Background()
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

// shouldCleanCluster decides whether a cluster is eligible for deletion: it must
// be older than the expiration cutoff and its name must match one of the known
// test-cluster prefixes. Kept free of AWS SDK types so it is unit testable.
func shouldCleanCluster(clusterName string, createdAt *time.Time, expirationDate time.Time, clustersToClean []string) bool {
	if createdAt == nil {
		return false
	}
	if !expirationDate.After(*createdAt) {
		return false
	}
	return clusterNameMatchesClustersToClean(clusterName, clustersToClean)
}

func terminateClusters(ctx context.Context, client *eks.Client) {
	listClusterInput := eks.ListClustersInput{}
	expirationDateCluster := time.Now().UTC().Add(clean.KeepDurationOneDay)

	clusters, err := client.ListClusters(ctx, &listClusterInput)
	if err != nil {
		log.Fatalf("could not get cluster list")
	}
	for _, cluster := range clusters.Clusters {
		describeClusterInput := eks.DescribeClusterInput{Name: aws.String(cluster)}
		describeClusterOutput, err := client.DescribeCluster(ctx, &describeClusterInput)
		if err != nil {
			return
		}
		if !shouldCleanCluster(*describeClusterOutput.Cluster.Name, describeClusterOutput.Cluster.CreatedAt, expirationDateCluster, ClustersToClean) {
			log.Printf("Ignoring cluster %s with a launch-date %s (too new or name does not match a cleanable prefix)", cluster, *describeClusterOutput.Cluster.CreatedAt)
			continue
		}
		log.Printf("Try to delete cluster %s launch-date %s", cluster, *describeClusterOutput.Cluster.CreatedAt)

		// A cluster cannot be deleted while it still has node groups, so delete
		// them and wait for the deletions to finish before deleting the cluster.
		// Waiting (rather than relying on a later run) is what makes teardown
		// complete in a single pass and stops the cluster's dependents from
		// orphaning.
		deleteNodegroupsAndWait(ctx, client, cluster)

		deleteClusterInput := eks.DeleteClusterInput{Name: aws.String(cluster)}
		if _, err = client.DeleteCluster(ctx, &deleteClusterInput); err != nil {
			// Left in place for the next run to retry (for example if a node
			// group deletion did not finish within the timeout above).
			log.Printf("could not delete cluster %s err %v", cluster, err)
			continue
		}
		log.Printf("Deleted cluster %s", cluster)
	}
}

// deleteNodegroupsAndWait issues a delete for every node group on the cluster and
// then waits for all of them to finish deleting, bounded by nodegroupDeleteTimeout.
// Deletes are issued for all node groups first so they drain in parallel.
func deleteNodegroupsAndWait(ctx context.Context, client *eks.Client, cluster string) {
	nodeGroupOutput, err := client.ListNodegroups(ctx, &eks.ListNodegroupsInput{ClusterName: aws.String(cluster)})
	if err != nil {
		log.Printf("could not query node groups cluster %s err %v", cluster, err)
		return
	}

	for _, nodegroup := range nodeGroupOutput.Nodegroups {
		deleteNodegroupInput := eks.DeleteNodegroupInput{
			ClusterName:   aws.String(cluster),
			NodegroupName: aws.String(nodegroup),
		}
		if _, err := client.DeleteNodegroup(ctx, &deleteNodegroupInput); err != nil {
			log.Printf("could not delete node group %s cluster %s err %v", nodegroup, cluster, err)
		}
	}

	waiter := eks.NewNodegroupDeletedWaiter(client)
	for _, nodegroup := range nodeGroupOutput.Nodegroups {
		describeNodegroupInput := eks.DescribeNodegroupInput{
			ClusterName:   aws.String(cluster),
			NodegroupName: aws.String(nodegroup),
		}
		if err := waiter.Wait(ctx, &describeNodegroupInput, nodegroupDeleteTimeout); err != nil {
			log.Printf("timed out or errored waiting for node group %s cluster %s to delete err %v", nodegroup, cluster, err)
		}
	}
}
