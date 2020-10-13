package ecsservicediscovery

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_TaskDefinitionConfig_init(t *testing.T) {
	config := TaskDefinitionConfig{
		JobName: "test_job_1",
		MetricsPorts: "11;12;	 13 ;a;14  ",
		TaskDefArnPattern: "^task.*$",
	}

	config.init()
	assert.True(t, reflect.DeepEqual(config.metricsPortList, []int{11, 12, 13, 14}))
}
