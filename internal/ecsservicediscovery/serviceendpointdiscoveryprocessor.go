// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
)

// Tag the Tasks that match the Service Name based Service Discovery
type ServiceEndpointDiscoveryProcessor struct {
	serviceNamesForTasksConfig []*ServiceNameForTasksConfig
	svcEcs                     *ecs.ECS
	stats                      *ProcessorStats
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func NewServiceEndpointDiscoveryProcessor(ecs *ecs.ECS, serviceNamesForTasks []*ServiceNameForTasksConfig, s *ProcessorStats) *ServiceEndpointDiscoveryProcessor {
	for _, v := range serviceNamesForTasks {
		v.init()
	}

	return &ServiceEndpointDiscoveryProcessor{
		serviceNamesForTasksConfig: serviceNamesForTasks,
		svcEcs:                     ecs,
		stats:                      s,
	}
}

func (p *ServiceEndpointDiscoveryProcessor) Process(cluster string, taskList []*DecoratedTask) ([]*DecoratedTask, error) {
	if len(p.serviceNamesForTasksConfig) == 0 {
		return taskList, nil
	}
	idToServiceName := make(map[string]string)
	var servicesToDescribe []*string
	req := &ecs.ListServicesInput{Cluster: &cluster}
	for {
		listServiceResp, listServiceErr := p.svcEcs.ListServices(req)
		p.stats.AddStats(AWSCLIListServices)
		if listServiceErr != nil {
			return taskList, newServiceDiscoveryError("Failed to list service ARNs for "+cluster, &listServiceErr)
		}
		servicesToDescribe = p.processServices(listServiceResp, servicesToDescribe)
		if listServiceResp.NextToken == nil {
			break
		}
		req.NextToken = listServiceResp.NextToken
	}
	describeServiceErr := p.mapDeploymentIDs(cluster, servicesToDescribe, idToServiceName)
	if describeServiceErr != nil {
		return taskList, describeServiceErr
	}
	p.processDecoratedTasks(taskList, idToServiceName)
	return taskList, nil
}

func (p *ServiceEndpointDiscoveryProcessor) processServices(listServiceResp *ecs.ListServicesOutput, servicesToDescribe []*string) []*string {
	for _, serviceArn := range listServiceResp.ServiceArns {
		splitArn := strings.Split(*serviceArn, "/")
		serviceNameFromArn := splitArn[len(splitArn)-1]
		for _, serviceName := range p.serviceNamesForTasksConfig {
			if serviceName.serviceNameRegex.MatchString(serviceNameFromArn) {
				servicesToDescribe = append(servicesToDescribe, serviceArn)
				break
			}
		}
	}
	return servicesToDescribe
}

func (p *ServiceEndpointDiscoveryProcessor) mapDeploymentIDs(cluster string, servicesForInput []*string, idToServiceName map[string]string) error {
	for startIndex := 0; startIndex < len(servicesForInput); startIndex += 10 {
		endIndex := minInt(startIndex+10, len(servicesForInput))
		req := &ecs.DescribeServicesInput{Cluster: &cluster, Services: servicesForInput[startIndex:endIndex]}
		describeServiceResp, describeServiceErr := p.svcEcs.DescribeServices(req)
		for _, describedService := range describeServiceResp.Services {
			for _, deployment := range describedService.Deployments {
				if *deployment.Status == "ACTIVE" || *deployment.Status == "PRIMARY" {
					if describedService.ServiceName != nil {
						idToServiceName[*deployment.Id] = *describedService.ServiceName
					}
				}
			}
		}
		if describeServiceErr != nil {
			return newServiceDiscoveryError("Failed to describe service ARNs for "+cluster, &describeServiceErr)
		}
	}
	return nil
}

func (p *ServiceEndpointDiscoveryProcessor) processDecoratedTasks(taskList []*DecoratedTask, idToServiceName map[string]string) {
	for _, v := range taskList {
		if v.Task.StartedBy != nil {
			if val, ok := idToServiceName[*v.Task.StartedBy]; ok {
				if p.validateServiceNameDiscoveredTask(val, v, p.serviceNamesForTasksConfig) {
					v.ServiceName = val
				}
			}
		}
	}
}

func (p *ServiceEndpointDiscoveryProcessor) validateServiceNameDiscoveredTask(serviceName string, task *DecoratedTask, serviceNamesForTasksConfig []*ServiceNameForTasksConfig) bool {
	for _, serviceConfig := range serviceNamesForTasksConfig {
		if serviceConfig.serviceNameRegex.MatchString(serviceName) {
			if serviceConfig.ContainerNamePattern == "" || checkContainerNamePatternService(task.TaskDefinition.ContainerDefinitions, serviceConfig) {
				return true
			}
		}
	}
	return false
}

func checkContainerNamePatternService(containers []*ecs.ContainerDefinition, config *ServiceNameForTasksConfig) bool {
	for _, c := range containers {
		if config.containerNameRegex.MatchString(aws.StringValue(c.Name)) {
			return true
		}
	}
	return false
}

func (p *ServiceEndpointDiscoveryProcessor) ProcessorName() string {
	return "ServiceEndpointDiscoveryProcessor"
}
