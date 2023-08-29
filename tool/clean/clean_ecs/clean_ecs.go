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

// Clean ecs clusters if they have been open longer than 7 day
func main() {
	err := cleanCluster()
	if err != nil {
		log.Fatalf("errors cleaning %v", err)
	}
}

func cleanCluster() error {
	log.Print("Begin to clean ECS Clusters")

	cxt := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(cxt)
	if err != nil {
		return err
	}
	ecsClient := ecs.NewFromConfig(defaultConfig)

	terminateClusters(cxt, ecsClient)
	return err
}

func terminateClusters(ctx context.Context, client *ecs.Client) {
	// you can only filter ecs by name or arn
	// not regex of tag name like ec2
	// describe cluster input max is 100
	ecsListClusterInput := ecs.ListClustersInput{
		MaxResults: aws.Int32(100),
	}
	for {
		clusterIds := make([]*string, 0)
		expirationDateCluster := time.Now().UTC().Add(clean.KeepDurationOneWeek)
		listClusterOutput, err := client.ListClusters(ctx, &ecsListClusterInput)
		if err != nil || listClusterOutput.ClusterArns == nil || len(listClusterOutput.ClusterArns) == 0 {
			break
		}
		describeClustersInput := ecs.DescribeClustersInput{Clusters: listClusterOutput.ClusterArns}
		describeClustersOutput, err := client.DescribeClusters(ctx, &describeClustersInput)
		if err != nil || describeClustersOutput.Clusters == nil || len(describeClustersOutput.Clusters) == 0 {
			break
		}
		for _, cluster := range describeClustersOutput.Clusters {
			if !strings.HasPrefix(*cluster.ClusterName, "cwagent-integ-test-cluster-") {
				continue
			}
			if cluster.RunningTasksCount == 0 && cluster.PendingTasksCount == 0 {
				clusterIds = append(clusterIds, cluster.ClusterArn)
				continue
			}
			describeTaskInput := ecs.DescribeTasksInput{Cluster: cluster.ClusterArn}
			describeTasks, err := client.DescribeTasks(ctx, &describeTaskInput)
			if err != nil {
				continue
			}
			addCluster := true
			for _, task := range describeTasks.Tasks {
				if expirationDateCluster.After(*task.StartedAt) {
					log.Printf("Task %s launch-date %s", *task.TaskArn, *task.StartedAt)
				} else {
					addCluster = false
					break
				}
			}
			if addCluster {
				clusterIds = append(clusterIds, cluster.ClusterArn)
			}
		}
		if len(clusterIds) == 0 {
			log.Printf("No clusters to terminate")
			return
		}

		for _, clusterId := range clusterIds {
			log.Printf("cluster to temrinate %s", *clusterId)
			listContainerInstanceInput := ecs.ListContainerInstancesInput{Cluster: clusterId}
			listContainerInstances, err := client.ListContainerInstances(ctx, &listContainerInstanceInput)
			if err != nil {
				log.Printf("Error %v getting container instances cluster %s", err, *clusterId)
				continue
			}
			for _, instance := range listContainerInstances.ContainerInstanceArns {
				deregisterContainerInstanceInput := ecs.DeregisterContainerInstanceInput{
					ContainerInstance: aws.String(instance),
					Cluster:           clusterId,
					Force:             aws.Bool(true),
				}
				_, err = client.DeregisterContainerInstance(ctx, &deregisterContainerInstanceInput)
				if err != nil {
					log.Printf("Error %v deregister container instances cluster %s container %v", err, *clusterId, instance)
					continue
				}
			}
			serviceInput := ecs.ListServicesInput{Cluster: clusterId}
			services, err := client.ListServices(ctx, &serviceInput)
			if err != nil {
				log.Printf("Error %v getting services cluster %s", err, *clusterId)
				continue
			}
			for _, service := range services.ServiceArns {
				deleteServiceInput := ecs.DeleteServiceInput{Cluster: clusterId, Service: aws.String(service)}
				_, err := client.DeleteService(ctx, &deleteServiceInput)
				if err != nil {
					log.Printf("Error %v deleteing service %s cluster %s", err, serviceInput, *clusterId)
					continue
				}
			}
			terminateClusterInput := ecs.DeleteClusterInput{Cluster: clusterId}
			_, err = client.DeleteCluster(ctx, &terminateClusterInput)
			if err != nil {
				log.Printf("Error %v terminating cluster %s", err, *clusterId)
			}
		}
		if ecsListClusterInput.NextToken == nil {
			break
		}
		ecsListClusterInput.NextToken = listClusterOutput.NextToken
	}
}
