// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
)

type ServiceDiscovery struct {
	Config *ServiceDiscoveryConfig

	svcEcs *ecs.Client
	svcEc2 *ec2.Client

	stats             ProcessorStats
	clusterProcessors []Processor
	Configurer        *awsmiddleware.Configurer
}

func (sd *ServiceDiscovery) init(ctx context.Context) {
	credentialConfig := &configaws.CredentialsConfig{
		Region: sd.Config.TargetClusterRegion,
	}
	awsConfig, err := credentialConfig.LoadConfig(ctx)
	if err != nil {
		awsConfig = aws.Config{}
	}

	// Configure retry behavior
	awsConfig.RetryMaxAttempts = AwsSdkLevelRetryCount

	// Add middleware if configured
	if sd.Configurer != nil {
		if err = sd.Configurer.Configure(awsmiddleware.SDKv2(&awsConfig)); err != nil {
			log.Printf("ERROR: Failed to configure middleware for ECS and EC2 clients: %v", err)
		}
	}

	sd.svcEcs = ecs.NewFromConfig(awsConfig)
	sd.svcEc2 = ec2.NewFromConfig(awsConfig)

	sd.initClusterProcessorPipeline()
}

func (sd *ServiceDiscovery) initClusterProcessorPipeline() {
	sd.clusterProcessors = append(sd.clusterProcessors, NewTaskProcessor(sd.svcEcs, &sd.stats))
	sd.clusterProcessors = append(sd.clusterProcessors, NewTaskDefinitionProcessor(sd.svcEcs, &sd.stats))
	sd.clusterProcessors = append(sd.clusterProcessors, NewServiceEndpointDiscoveryProcessor(sd.svcEcs, sd.Config.ServiceNamesForTasks, &sd.stats))
	sd.clusterProcessors = append(sd.clusterProcessors, NewDockerLabelDiscoveryProcessor(sd.Config.DockerLabel))
	sd.clusterProcessors = append(sd.clusterProcessors, NewTaskDefinitionDiscoveryProcessor(sd.Config.TaskDefinitions))
	sd.clusterProcessors = append(sd.clusterProcessors, NewTaskFilterProcessor())
	sd.clusterProcessors = append(sd.clusterProcessors, NewContainerInstanceProcessor(sd.svcEcs, sd.svcEc2, &sd.stats))
	sd.clusterProcessors = append(sd.clusterProcessors, NewTargetsExportProcessor(sd.Config, &sd.stats))
}

func StartECSServiceDiscovery(sd *ServiceDiscovery, shutDownChan chan interface{}, wg *sync.WaitGroup) {
	defer wg.Done()

	if !sd.validateConfig() {
		return
	}

	ctx := context.Background()
	frequency, _ := time.ParseDuration(sd.Config.Frequency)
	sd.init(ctx)
	t := time.NewTicker(frequency)
	defer t.Stop()
	for {
		select {
		case <-shutDownChan:
			return
		case <-t.C:
			sd.work(ctx)
		}
	}
}

func (sd *ServiceDiscovery) work(ctx context.Context) {
	sd.stats.ResetStats()
	var clusterTasks []*DecoratedTask
	var err error
	for _, p := range sd.clusterProcessors {
		clusterTasks, err = p.Process(ctx, sd.Config.TargetCluster, clusterTasks)
		// Ignore partial result to avoid overwriting existing targets
		if err != nil {
			log.Printf("E! ECS SD processor: %v got error: %v \n", p.ProcessorName(), err.Error())
			return
		}
	}
	sd.stats.ShowStats()
}

func (sd *ServiceDiscovery) validateConfig() bool {
	if sd.Config == nil {
		return false
	}

	if sd.Config.DockerLabel == nil && len(sd.Config.TaskDefinitions) == 0 && len(sd.Config.ServiceNamesForTasks) == 0 {
		log.Printf("E! Neither docker label based discovery, nor task definition based discovery, nor service name based discovery is enabled.\n")
		return false
	}

	if sd.Config.TargetCluster == "" || sd.Config.TargetClusterRegion == "" {
		log.Printf("E! Target ECS cluster info is not correct.\n")
		return false
	}

	_, err := time.ParseDuration(sd.Config.Frequency)
	if err != nil {
		log.Printf("E! Invalid ECS service discovery frequency: %v.\n", sd.Config.Frequency)
		return false
	}

	return true
}
