package ecsservicediscovery

import (
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
	"testing"
)

func buildTestingTask() []*DecoratedTask {

	return []*DecoratedTask{
		{
			TaskDefinition: &ecs.TaskDefinition{},
		},
		{
			TaskDefinition: &ecs.TaskDefinition{},
		},
		{},
	}
}

func Test_filterNilTaskDefinitionTasks(t *testing.T) {
	tasks := filterNilTaskDefinitionTasks(buildTestingTask())
	assert.Equal(t, 2, len(tasks))
}
