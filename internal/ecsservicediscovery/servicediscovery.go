// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
)

type ServiceDiscovery struct {
	Config *ServiceDiscoveryConfig

	svcEcs *ecs.ECS
	svcEc2 *ec2.EC2

	stats             ProcessorStats
	clusterProcessors []Processor
	Configurer        *awsmiddleware.Configurer
}

func (sd *ServiceDiscovery) init() {
	credentialConfig := &configaws.CredentialConfig{
		Region: sd.Config.TargetClusterRegion,
	}
	configProvider := credentialConfig.Credentials()
	sd.svcEcs = ecs.New(configProvider, aws.NewConfig().WithRegion(sd.Config.TargetClusterRegion).WithMaxRetries(AwsSdkLevelRetryCount))
	if err := sd.Configurer.Configure(awsmiddleware.SDKv1(&sd.svcEcs.Handlers)); err != nil {
		log.Println("Failed to configure ecs client")
	} else {
		log.Println("Configured ecs client handlers!")
	}
	sd.svcEc2 = ec2.New(configProvider, aws.NewConfig().WithRegion(sd.Config.TargetClusterRegion).WithMaxRetries(AwsSdkLevelRetryCount))
	if err := sd.Configurer.Configure(awsmiddleware.SDKv1(&sd.svcEc2.Handlers)); err != nil {
		log.Println("Failed to configure ec2 client")
	} else {
		log.Println("Configured ec2 client handlers!")
	}
	sd.initClusterProcessorPipeline()
}

func (sd *ServiceDiscovery) initClusterProcessorPipeline() {
	sd.clusterProcessors = append(sd.clusterProcessors, NewTaskProcessor(sd.svcEcs, &sd.stats, sd.Configurer))
	sd.clusterProcessors = append(sd.clusterProcessors, NewTaskDefinitionProcessor(sd.svcEcs, &sd.stats, sd.Configurer))
	sd.clusterProcessors = append(sd.clusterProcessors, NewServiceEndpointDiscoveryProcessor(sd.svcEcs, sd.Config.ServiceNamesForTasks, &sd.stats, sd.Configurer))
	sd.clusterProcessors = append(sd.clusterProcessors, NewDockerLabelDiscoveryProcessor(sd.Config.DockerLabel))
	sd.clusterProcessors = append(sd.clusterProcessors, NewTaskDefinitionDiscoveryProcessor(sd.Config.TaskDefinitions))
	sd.clusterProcessors = append(sd.clusterProcessors, NewTaskFilterProcessor())
	sd.clusterProcessors = append(sd.clusterProcessors, NewContainerInstanceProcessor(sd.svcEcs, sd.svcEc2, &sd.stats, sd.Configurer))
	sd.clusterProcessors = append(sd.clusterProcessors, NewTargetsExportProcessor(sd.Config, &sd.stats))
}

func StartECSServiceDiscovery(sd *ServiceDiscovery, shutDownChan chan interface{}, wg *sync.WaitGroup) {
	log.Println("StartECSServiceDiscovery handleContainerInstances - - - - - ")

	defer wg.Done()

	if !sd.validateConfig() {
		return
	}

	frequency, _ := time.ParseDuration(sd.Config.Frequency)
	sd.init()
	t := time.NewTicker(frequency)
	defer t.Stop()
	for {
		select {
		case <-shutDownChan:
			return
		case <-t.C:
			sd.work()
		}
	}
}

func (sd *ServiceDiscovery) work() {
	log.Println("Work handleContainerInstances - - - - - ")
	ec2Ids := make([]*string, 0, batchSize)
	ec2input := &ec2.DescribeInstancesInput{InstanceIds: ec2Ids}
	temp, _ := sd.svcEc2.DescribeInstances(ec2input)

	log.Println(temp)

	sd.stats.ResetStats()
	var err error
	var clusterTasks []*DecoratedTask
	for _, p := range sd.clusterProcessors {
		log.Println(p.ProcessorName())
		clusterTasks, err = p.Process(sd.Config.TargetCluster, clusterTasks)

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
