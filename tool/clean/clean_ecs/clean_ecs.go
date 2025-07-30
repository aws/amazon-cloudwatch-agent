// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build clean
// +build clean

package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

// Clean ECS clusters if they have been running longer than 7 days

var expirationTimeOneWeek = time.Now().UTC().Add(-clean.KeepDurationOneWeek)

const cwaIntegTestClusterPrefix = "cwagent-integ-test-cluster-"

func main() {
	ctx := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Error loading AWS config for ECS cleanup: %v", err)
	}

	ecsClient := ecs.NewFromConfig(defaultConfig)
	terminateClusters(ctx, ecsClient)
}

func terminateClusters(ctx context.Context, client *ecs.Client) {
	// you can only filter ecs by name or arn
	// not regex of tag name like ec2
	// describe cluster input max is 100

	log.Print("Begin to clean ECS Clusters")

	ecsListClusterInput := ecs.ListClustersInput{
		MaxResults: aws.Int32(100),
	}
	clusterIds := make([]*string, 0)

	for {
		listClusterOutput, err := client.ListClusters(ctx, &ecsListClusterInput)
		if err != nil || listClusterOutput.ClusterArns == nil || len(listClusterOutput.ClusterArns) == 0 {
			break
		}
		describeClustersInput := ecs.DescribeClustersInput{Clusters: listClusterOutput.ClusterArns}
		describeClustersOutput, err := client.DescribeClusters(ctx, &describeClustersInput)
		if err != nil || describeClustersOutput.Clusters == nil || len(describeClustersOutput.Clusters) == 0 {
			break
		}

		/* Cluster should meet all criteria to be deleted:
		1. Prefix should match: 'cwagent-integ-test-cluster-'
		2. No running or pending tasks OR Task started more than 1 week ago (ie expired)
		*/

		for _, cluster := range describeClustersOutput.Clusters {
			if !strings.HasPrefix(*cluster.ClusterName, cwaIntegTestClusterPrefix) {
				continue
			}
			if cluster.RunningTasksCount == 0 && cluster.PendingTasksCount == 0 {
				clusterIds = append(clusterIds, cluster.ClusterArn)
				continue
			}

			if isClusterTasksExpired(ctx, client, cluster.ClusterArn) {
				clusterIds = append(clusterIds, cluster.ClusterArn)
				continue
			}
		}

		// Pagination to break loop
		if listClusterOutput.NextToken == nil {
			break
		}
		ecsListClusterInput.NextToken = listClusterOutput.NextToken
	}

	// Deletion Logic
	for _, clusterId := range clusterIds {
		log.Printf("Cluster to terminate: %s", *clusterId)

		// Delete cluster services
		serviceInput := ecs.ListServicesInput{Cluster: clusterId}
		services, err := client.ListServices(ctx, &serviceInput)
		if err != nil {
			log.Printf("Error getting services cluster %s: %v", *clusterId, err)
			continue
		}

		for _, service := range services.ServiceArns {
			// Scale Down Service
			updateServiceInput := ecs.UpdateServiceInput{Cluster: clusterId, Service: aws.String(service), DesiredCount: aws.Int32(0)}
			_, err := client.UpdateService(ctx, &updateServiceInput)
			if err != nil {
				log.Printf("Error scaling down service %s in cluster %s: %v", service, *clusterId, err)
				log.Print("Trying service deletion anyways...")
			}

			// Delete Service
			deleteServiceInput := ecs.DeleteServiceInput{Cluster: clusterId, Service: aws.String(service)}
			_, err = client.DeleteService(ctx, &deleteServiceInput)
			if err != nil {
				log.Printf("Error deleting service %s in cluster %s: %v", service, *clusterId, err)
				continue
			}
		}

		// Delete Container Instances
		listContainerInstanceInput := ecs.ListContainerInstancesInput{Cluster: clusterId}
		listContainerInstances, err := client.ListContainerInstances(ctx, &listContainerInstanceInput)
		if err != nil {
			log.Printf("Error getting container instances cluster %s: %v", *clusterId, err)
		}
		for _, instance := range listContainerInstances.ContainerInstanceArns {
			deregisterContainerInstanceInput := ecs.DeregisterContainerInstanceInput{
				ContainerInstance: aws.String(instance),
				Cluster:           clusterId,
				Force:             aws.Bool(true),
			}
			_, err = client.DeregisterContainerInstance(ctx, &deregisterContainerInstanceInput)
			if err != nil {
				log.Printf("Error deregister container instances cluster %s container %s: %v", *clusterId, instance, err)
			}
		}

		// Delete Cluster
		terminateClusterInput := ecs.DeleteClusterInput{Cluster: clusterId}
		_, err = client.DeleteCluster(ctx, &terminateClusterInput)
		if err != nil {
			log.Printf("Error terminating cluster %s: %v", *clusterId, err)
		}
		log.Printf("Cluster deleted")
	}
}

func isClusterTasksExpired(ctx context.Context, client *ecs.Client, clusterArn *string) bool {
	listTasksInput := ecs.ListTasksInput{Cluster: clusterArn}
	listTasksOutput, err := client.ListTasks(ctx, &listTasksInput)
	if err != nil {
		log.Printf("Failed to listTasks for cluster %s: %v", *clusterArn, err)
		return false
	}
	describeTaskInput := ecs.DescribeTasksInput{
		Cluster: clusterArn,
		Tasks:   listTasksOutput.TaskArns,
	}
	describeTasks, err := client.DescribeTasks(ctx, &describeTaskInput)
	if err != nil {
		log.Printf("Failed to describeTasks for cluster %s: %v", *clusterArn, err)
		return false
	}
	for _, task := range describeTasks.Tasks {
		if task.StartedAt != nil && expirationTimeOneWeek.Before(*task.StartedAt) {
			log.Printf("Task %s launched too recently on launch-date %s.", *task.TaskArn, *task.StartedAt)
			return false
		}
	}
	return true
}
