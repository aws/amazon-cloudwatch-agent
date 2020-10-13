package ecsservicediscovery

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
)

// Tag the Tasks that match the Task Definition ARN based Service Discovery
type TaskDefinitionDiscoveryProcessor struct {
	taskDefsConfig []*TaskDefinitionConfig
}

func NewTaskDefinitionDiscoveryProcessor(taskDefinitions []*TaskDefinitionConfig) *TaskDefinitionDiscoveryProcessor {
	for _, v := range taskDefinitions {
		v.init()
	}

	return &TaskDefinitionDiscoveryProcessor{taskDefsConfig: taskDefinitions}
}

func checkContainerNamePattern(containers []*ecs.ContainerDefinition, config *TaskDefinitionConfig) bool {
	for _, c := range containers {
		if config.containerNameRegex.MatchString(aws.StringValue(c.Name)) {
			return true
		}
	}
	return false
}

func (p *TaskDefinitionDiscoveryProcessor) Process(cluster string, taskList []*DecoratedTask) ([]*DecoratedTask, error) {
	if len(p.taskDefsConfig) == 0 {
		return taskList, nil
	}

	for _, v := range taskList {
		if v.TaskDefinition.TaskDefinitionArn == nil {
			continue
		}
		for _, t := range p.taskDefsConfig {
			if t.taskDefRegex.MatchString(aws.StringValue(v.TaskDefinition.TaskDefinitionArn)) {
				if t.ContainerNamePattern == "" || checkContainerNamePattern(v.TaskDefinition.ContainerDefinitions, t) {
					v.TaskDefinitionBased = true
					break
				}
			}
		}
	}

	return taskList, nil
}

func (p *TaskDefinitionDiscoveryProcessor) ProcessorName() string {
	return "TaskDefinitionDiscoveryProcessor"
}
