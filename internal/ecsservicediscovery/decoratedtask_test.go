// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"log"
	"reflect"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
)

func TestAddExporterLabels(t *testing.T) {
	labels := make(map[string]string)
	var labelValueB string
	labelValueC := "label_value_c"
	addExporterLabels(labels, "Key_A", nil)
	addExporterLabels(labels, "Key_B", &labelValueB)
	addExporterLabels(labels, "Key_C", &labelValueC)
	expected := map[string]string{"Key_C": "label_value_c"}
	assert.True(t, reflect.DeepEqual(labels, expected))
}

// ARN formats: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-account-settings.html#ecs-resource-ids
func TestGeneratePrometheusTargetOldARNFormat(t *testing.T) {
	fullTask := buildWorkloadFargateAWSVPC(false, true, false, "")
	assert.Equal(t, "10.0.0.129", fullTask.getPrivateIp())

	config := &ServiceDiscoveryConfig{
		DockerLabel: &DockerLabelConfig{
			JobNameLabel:     "FARGATE_PROMETHEUS_JOB_NAME",
			PortLabel:        "FARGATE_PROMETHEUS_EXPORTER_PORT",
			MetricsPathLabel: "ECS_PROMETHEUS_METRICS_PATH",
		},
	}

	targets := make(map[string]*PrometheusTarget)
	dockerLabelRegex := regexp.MustCompile(prometheusLabelNamePattern)
	fullTask.ExporterInformation(config, dockerLabelRegex, targets)

	target, ok := targets["10.0.0.129:9406/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9406/metrics")
	assert.Equal(t, "", target.Labels["TaskClusterName"])
	assert.Equal(t, "1234567890123456789", target.Labels["TaskId"])
}

func buildWorkloadFargateAWSVPC(useNewTaskArnFormat bool, dockerLabel bool, taskDef bool, serviceName string) *DecoratedTask {
	networkMode := types.NetworkModeAwsvpc
	taskAttachmentId := "775c6c63-b5f7-4a5b-8a60-8f8295a04cda"
	taskAttachmentType := "ElasticNetworkInterface"
	taskAttachmentStatus := "ATTACHING"
	taskAttachmentDetailsKey1 := "networkInterfaceId"
	taskAttachmentDetailsKey2 := "privateIPv4Address"
	taskAttachmentDetailsValue1 := "eni-03de9d47faaa2e5ec"
	taskAttachmentDetailsValue2 := "10.0.0.129"

	taskArn := "arn:aws:ecs:us-east-2:211220956907:task/1234567890123456789"
	if useNewTaskArnFormat {
		taskArn = "arn:aws:ecs:us-east-2:211220956907:task/ExampleCluster/1234567890123456789"
	}

	taskDefinitionArn := "arn:aws:ecs:us-east-2:211220956907:task-definition/prometheus-java-tomcat-fargate-awsvpc:1"
	var taskRevision int32 = 4
	port9404String := "9404"
	port9406String := "9406"
	var port9404Int32 int32 = 9404
	var port9406Int32 int32 = 9406
	containerNameTomcat := "bugbash-tomcat-fargate-awsvpc-with-docker-label"
	containerNameJar := "bugbash-jar-fargate-awsvpc-with-dockerlabel"

	jobNameLabel := "java-tomcat-fargate-awsvpc"
	metricsPathLabel := "/metrics"

	return &DecoratedTask{
		DockerLabelBased:    dockerLabel,
		TaskDefinitionBased: taskDef,
		ServiceName:         serviceName,
		Task: &types.Task{
			TaskArn:           aws.String(taskArn),
			TaskDefinitionArn: aws.String(taskDefinitionArn),
			Attachments: []types.Attachment{
				{
					Id:     aws.String(taskAttachmentId),
					Type:   aws.String(taskAttachmentType),
					Status: aws.String(taskAttachmentStatus),
					Details: []types.KeyValuePair{
						{
							Name:  aws.String(taskAttachmentDetailsKey1),
							Value: aws.String(taskAttachmentDetailsValue1),
						},
						{
							Name:  aws.String(taskAttachmentDetailsKey2),
							Value: aws.String(taskAttachmentDetailsValue2),
						},
					},
				},
			},
		},
		TaskDefinition: &types.TaskDefinition{
			NetworkMode:       networkMode,
			TaskDefinitionArn: aws.String(taskDefinitionArn),
			Revision:          taskRevision,
			ContainerDefinitions: []types.ContainerDefinition{
				{
					Name: aws.String(containerNameTomcat),
					DockerLabels: map[string]string{
						"FARGATE_PROMETHEUS_EXPORTER_PORT": port9404String,
						"FARGATE_PROMETHEUS_JOB_NAME":      jobNameLabel,
					},
					PortMappings: []types.PortMapping{
						{
							ContainerPort: aws.Int32(port9404Int32),
							HostPort:      aws.Int32(port9404Int32),
						},
					},
				},
				{
					Name: aws.String(containerNameJar),
					DockerLabels: map[string]string{
						"FARGATE_PROMETHEUS_EXPORTER_PORT": port9406String,
						"ECS_PROMETHEUS_METRICS_PATH":      metricsPathLabel,
					},
					PortMappings: []types.PortMapping{
						{
							ContainerPort: aws.Int32(port9406Int32),
							HostPort:      aws.Int32(port9406Int32),
						},
					},
				},
			},
		},
	}
}

func Test_ExportDockerLabelBasedTarget_Fargate_AWSVPC(t *testing.T) {
	fullTask := buildWorkloadFargateAWSVPC(true, true, false, "")
	assert.Equal(t, "10.0.0.129", fullTask.getPrivateIp())

	config := &ServiceDiscoveryConfig{
		DockerLabel: &DockerLabelConfig{
			JobNameLabel:     "FARGATE_PROMETHEUS_JOB_NAME",
			PortLabel:        "FARGATE_PROMETHEUS_EXPORTER_PORT",
			MetricsPathLabel: "ECS_PROMETHEUS_METRICS_PATH",
		},
	}

	targets := make(map[string]*PrometheusTarget)
	dockerLabelRegex := regexp.MustCompile(prometheusLabelNamePattern)
	fullTask.ExporterInformation(config, dockerLabelRegex, targets)

	assert.Equal(t, 2, len(targets))
	target, ok := targets["10.0.0.129:9404/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9404/metrics")

	assert.Equal(t, 7, len(target.Labels))
	assert.Equal(t, "java-tomcat-fargate-awsvpc", target.Labels["job"])
	assert.Equal(t, "bugbash-tomcat-fargate-awsvpc-with-docker-label", target.Labels["container_name"])
	assert.Equal(t, "4", target.Labels["TaskRevision"])
	assert.Equal(t, "ExampleCluster", target.Labels["TaskClusterName"])
	assert.Equal(t, "1234567890123456789", target.Labels["TaskId"])
	assert.Equal(t, "9404", target.Labels["FARGATE_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "java-tomcat-fargate-awsvpc", target.Labels["FARGATE_PROMETHEUS_JOB_NAME"])

	target, ok = targets["10.0.0.129:9406/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9406/metrics")
	assert.Equal(t, 7, len(target.Labels))
	assert.Equal(t, "ExampleCluster", target.Labels["TaskClusterName"])
	assert.Equal(t, "1234567890123456789", target.Labels["TaskId"])
	assert.Equal(t, "4", target.Labels["TaskRevision"])
	assert.Equal(t, "bugbash-jar-fargate-awsvpc-with-dockerlabel", target.Labels["container_name"])
	assert.Equal(t, "9406", target.Labels["FARGATE_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "/metrics", target.Labels["__metrics_path__"])
	assert.Equal(t, "/metrics", target.Labels["ECS_PROMETHEUS_METRICS_PATH"])

}

func Test_ExportTaskDefBasedTarget_Fargate_AWSVPC(t *testing.T) {
	fullTask := buildWorkloadFargateAWSVPC(true, false, true, "")
	assert.Equal(t, "10.0.0.129", fullTask.getPrivateIp())
	config := &ServiceDiscoveryConfig{
		TaskDefinitions: []*TaskDefinitionConfig{
			{
				JobName:           "",
				MetricsPorts:      "9404;9406",
				TaskDefArnPattern: ".*:task-definition/prometheus-java-tomcat-fargate-awsvpc:[0-9]+",
				MetricsPath:       "/stats/metrics",
			},
		},
	}
	config.TaskDefinitions[0].init()
	assert.Equal(t, []int32{9404, 9406}, config.TaskDefinitions[0].metricsPortList)

	targets := make(map[string]*PrometheusTarget)
	dockerLabelRegex := regexp.MustCompile(prometheusLabelNamePattern)
	fullTask.ExporterInformation(config, dockerLabelRegex, targets)

	assert.Equal(t, 2, len(targets))
	target, ok := targets["10.0.0.129:9404/stats/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9404/stats/metrics")

	assert.Equal(t, 7, len(target.Labels))
	assert.Equal(t, "java-tomcat-fargate-awsvpc", target.Labels["FARGATE_PROMETHEUS_JOB_NAME"])
	assert.Equal(t, "4", target.Labels["TaskRevision"])
	assert.Equal(t, "bugbash-tomcat-fargate-awsvpc-with-docker-label", target.Labels["container_name"])
	assert.Equal(t, "ExampleCluster", target.Labels["TaskClusterName"])
	assert.Equal(t, "1234567890123456789", target.Labels["TaskId"])
	assert.Equal(t, "9404", target.Labels["FARGATE_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "/stats/metrics", target.Labels["__metrics_path__"])

	target, ok = targets["10.0.0.129:9406/stats/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9406/stats/metrics")
	assert.Equal(t, 7, len(target.Labels))
	assert.Equal(t, "ExampleCluster", target.Labels["TaskClusterName"])
	assert.Equal(t, "1234567890123456789", target.Labels["TaskId"])
	assert.Equal(t, "4", target.Labels["TaskRevision"])
	assert.Equal(t, "bugbash-jar-fargate-awsvpc-with-dockerlabel", target.Labels["container_name"])
	assert.Equal(t, "9406", target.Labels["FARGATE_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "/stats/metrics", target.Labels["__metrics_path__"])
	assert.Equal(t, "/metrics", target.Labels["ECS_PROMETHEUS_METRICS_PATH"])
}

func Test_exportServiceEndpointBasedTarget_Fargate_AWSVPC(t *testing.T) {
	fullTask := buildWorkloadFargateAWSVPC(true, false, false, "true")
	assert.Equal(t, "10.0.0.129", fullTask.getPrivateIp())
	config := &ServiceDiscoveryConfig{
		ServiceNamesForTasks: []*ServiceNameForTasksConfig{
			{
				ServiceNamePattern: "true",
				JobName:            "",
				MetricsPorts:       "9404;9406",
				MetricsPath:        "/stats/metrics",
				taskArnList: []string{
					"arn:aws:ecs:us-east-2:211220956907:task/ExampleCluster/1234567890123456789",
				},
			},
		},
	}
	config.ServiceNamesForTasks[0].init()
	assert.Equal(t, []int32{9404, 9406}, config.ServiceNamesForTasks[0].metricsPortList)

	targets := make(map[string]*PrometheusTarget)
	dockerLabelRegex := regexp.MustCompile(prometheusLabelNamePattern)
	fullTask.ExporterInformation(config, dockerLabelRegex, targets)

	assert.Equal(t, 2, len(targets))
	target, ok := targets["10.0.0.129:9404/stats/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9404/stats/metrics")

	assert.Equal(t, 8, len(target.Labels))
	assert.Equal(t, "java-tomcat-fargate-awsvpc", target.Labels["FARGATE_PROMETHEUS_JOB_NAME"])
	assert.Equal(t, "4", target.Labels["TaskRevision"])
	assert.Equal(t, "bugbash-tomcat-fargate-awsvpc-with-docker-label", target.Labels["container_name"])
	assert.Equal(t, "ExampleCluster", target.Labels["TaskClusterName"])
	assert.Equal(t, "1234567890123456789", target.Labels["TaskId"])
	assert.Equal(t, "9404", target.Labels["FARGATE_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "/stats/metrics", target.Labels["__metrics_path__"])

	target, ok = targets["10.0.0.129:9406/stats/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9406/stats/metrics")
	assert.Equal(t, 8, len(target.Labels))
	assert.Equal(t, "ExampleCluster", target.Labels["TaskClusterName"])
	assert.Equal(t, "1234567890123456789", target.Labels["TaskId"])
	assert.Equal(t, "4", target.Labels["TaskRevision"])
	assert.Equal(t, "bugbash-jar-fargate-awsvpc-with-dockerlabel", target.Labels["container_name"])
	assert.Equal(t, "9406", target.Labels["FARGATE_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "/stats/metrics", target.Labels["__metrics_path__"])
	assert.Equal(t, "/metrics", target.Labels["ECS_PROMETHEUS_METRICS_PATH"])
}

func Test_ExportMixedSDTarget_Fargate_AWSVPC(t *testing.T) {
	fullTask := buildWorkloadFargateAWSVPC(true, true, true, "")
	log.Print(fullTask)
	assert.Equal(t, "10.0.0.129", fullTask.getPrivateIp())
	config := &ServiceDiscoveryConfig{
		DockerLabel: &DockerLabelConfig{
			JobNameLabel:     "FARGATE_PROMETHEUS_JOB_NAME",
			PortLabel:        "FARGATE_PROMETHEUS_EXPORTER_PORT",
			MetricsPathLabel: "ECS_PROMETHEUS_METRICS_PATH",
		},
		TaskDefinitions: []*TaskDefinitionConfig{
			{
				JobName:           "",
				MetricsPorts:      "9404;9406",
				TaskDefArnPattern: ".*:task-definition/prometheus-java-tomcat-fargate-awsvpc:[0-9]+",
				MetricsPath:       "/stats/metrics",
			},
		},
	}
	config.TaskDefinitions[0].init()
	assert.Equal(t, []int32{9404, 9406}, config.TaskDefinitions[0].metricsPortList)

	targets := make(map[string]*PrometheusTarget)
	dockerLabelRegex := regexp.MustCompile(prometheusLabelNamePattern)
	fullTask.ExporterInformation(config, dockerLabelRegex, targets)
	assert.Equal(t, 4, len(targets))
	_, ok := targets["10.0.0.129:9404/stats/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9404/stats/metrics")
	_, ok = targets["10.0.0.129:9406/stats/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9406/stats/metrics")
	_, ok = targets["10.0.0.129:9404/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9404/metrics")
	_, ok = targets["10.0.0.129:9406/metrics"]
	assert.True(t, ok, "Missing target: 10.0.0.129:9406/metrics")
}

func buildWorkloadEC2BridgeDynamicPort(dockerLabel bool, taskDef bool, serviceName string, networkMode types.NetworkMode) *DecoratedTask {
	taskContainersArn := "arn:aws:ecs:us-east-2:211220956907:container/3b288961-eb2c-4de5-a4c5-682c0a7cc625"
	var taskContainersDynamicHostPort int32 = 32774
	var taskContainersMappedHostPort int32 = 9494

	taskArn := "arn:aws:ecs:us-east-2:211220956907:task/ExampleCluster/1234567890123456789"
	taskDefinitionArn := "arn:aws:ecs:us-east-2:211220956907:task-definition/prometheus-java-tomcat-ec2-awsvpc:1"
	var taskRevision int32 = 5
	port9404String := "9404"
	port9406String := "9406"
	var port9404Int32 int32 = 9404
	var port9406Int32 int32 = 9406
	var port0Int32 int32

	containerNameTomcat := "bugbash-tomcat-prometheus-workload-java-ec2-bridge-mapped-port"
	containerNameJar := "bugbash-jar-prometheus-workload-java-ec2-bridge"

	jobNameLabelTomcat := "bugbash-tomcat-ec2-bridge-mapped-port"
	metricsPathLabel := "/metrics"

	return &DecoratedTask{
		DockerLabelBased:    dockerLabel,
		TaskDefinitionBased: taskDef,
		ServiceName:         serviceName,
		EC2Info: &EC2MetaData{
			ContainerInstanceId: "arn:aws:ecs:us-east-2:211220956907:container-instance/7b0a9662-ee0b-4cf6-9391-03f50ca501a5",
			ECInstanceId:        "i-02aa8e82e91b2c30e",
			PrivateIP:           "10.4.0.205",
			InstanceType:        "t3.medium",
			VpcId:               "vpc-03e9f55a92516a5e4",
			SubnetId:            "subnet-0d0b0212d14b70250",
		},
		Task: &types.Task{
			TaskArn:           aws.String(taskArn),
			TaskDefinitionArn: aws.String(taskDefinitionArn),
			Attachments:       []types.Attachment{},
			Containers: []types.Container{
				{
					ContainerArn: aws.String(taskContainersArn),
					Name:         aws.String(containerNameTomcat),
					NetworkBindings: []types.NetworkBinding{
						{
							ContainerPort: aws.Int32(port9404Int32),
							HostPort:      aws.Int32(taskContainersMappedHostPort),
						},
					},
				},
				{
					ContainerArn: aws.String(taskContainersArn),
					Name:         aws.String(containerNameJar),
					NetworkBindings: []types.NetworkBinding{
						{
							ContainerPort: aws.Int32(port9404Int32),
							HostPort:      aws.Int32(taskContainersDynamicHostPort),
						},
					},
				},
			},
		},
		TaskDefinition: &types.TaskDefinition{
			NetworkMode:       networkMode,
			TaskDefinitionArn: aws.String(taskDefinitionArn),
			Revision:          taskRevision,
			ContainerDefinitions: []types.ContainerDefinition{
				{
					Name: aws.String(containerNameTomcat),
					DockerLabels: map[string]string{
						"EC2_PROMETHEUS_EXPORTER_PORT": port9404String,
						"EC2_PROMETHEUS_JOB_NAME":      jobNameLabelTomcat,
					},
					PortMappings: []types.PortMapping{
						{
							ContainerPort: aws.Int32(port9404Int32),
							HostPort:      aws.Int32(port9404Int32),
						},
					},
				},
				{
					Name: aws.String(containerNameJar),
					DockerLabels: map[string]string{
						"EC2_PROMETHEUS_EXPORTER_PORT": port9406String,
						"EC2_PROMETHEUS_METRICS_PATH":  metricsPathLabel,
					},
					PortMappings: []types.PortMapping{
						{
							ContainerPort: aws.Int32(port9406Int32),
							HostPort:      aws.Int32(port0Int32),
						},
					},
				},
			},
		},
	}
}

func Test_ExportMixedSDTarget_EC2_Bridge_DynamicPort(t *testing.T) {
	testExportMixedSDTargetEC2BridgeDynamicPort(t, types.NetworkModeBridge, 2)
}

func Test_ExportMixedSDTarget_EC2_Bridge_DynamicPort_With_Implicit_NetworkMode(t *testing.T) {
	testExportMixedSDTargetEC2BridgeDynamicPort(t, "", 2)
}

func Test_ExportMixedSDTarget_EC2_Bridge_DynamicPort_With_NetworkModeNone(t *testing.T) {
	testExportMixedSDTargetEC2BridgeDynamicPort(t, types.NetworkModeNone, 0)
}

func testExportMixedSDTargetEC2BridgeDynamicPort(t *testing.T, networkMode types.NetworkMode, expectedTargets int) {
	t.Helper()
	fullTask := buildWorkloadEC2BridgeDynamicPort(true, true, "", networkMode)
	if expectedTargets == 0 {
		assert.Equal(t, "", fullTask.getPrivateIp())
	} else {
		assert.Equal(t, "10.4.0.205", fullTask.getPrivateIp())
	}

	config := &ServiceDiscoveryConfig{
		DockerLabel: &DockerLabelConfig{
			JobNameLabel:     "EC2_PROMETHEUS_JOB_NAME",
			PortLabel:        "EC2_PROMETHEUS_EXPORTER_PORT",
			MetricsPathLabel: "ECS_PROMETHEUS_METRICS_PATH",
		},
		TaskDefinitions: []*TaskDefinitionConfig{
			{
				JobName:           "",
				MetricsPorts:      "9404;9406",
				TaskDefArnPattern: ".*:task-definition/prometheus-java-tomcat-ec2-awsvpc:[0-9]+",
				MetricsPath:       "/metrics",
			},
		},
	}
	config.TaskDefinitions[0].init()
	assert.Equal(t, []int32{9404, 9406}, config.TaskDefinitions[0].metricsPortList)

	targets := make(map[string]*PrometheusTarget)
	dockerLabelRegex := regexp.MustCompile(prometheusLabelNamePattern)
	fullTask.ExporterInformation(config, dockerLabelRegex, targets)

	assert.Equal(t, expectedTargets, len(targets))
	if expectedTargets == 0 {
		return
	}

	target, ok := targets["10.4.0.205:32774/metrics"]
	assert.True(t, ok, "Missing target: 10.4.0.205:32774/metrics")

	assert.Equal(t, 10, len(target.Labels))
	assert.Equal(t, "/metrics", target.Labels["EC2_PROMETHEUS_METRICS_PATH"])
	assert.Equal(t, "9406", target.Labels["EC2_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "t3.medium", target.Labels["InstanceType"])
	assert.Equal(t, "subnet-0d0b0212d14b70250", target.Labels["SubnetId"])
	assert.Equal(t, "5", target.Labels["TaskRevision"])
	assert.Equal(t, "vpc-03e9f55a92516a5e4", target.Labels["VpcId"])
	assert.Equal(t, "/metrics", target.Labels["__metrics_path__"])
	assert.Equal(t, "bugbash-jar-prometheus-workload-java-ec2-bridge", target.Labels["container_name"])
	assert.Equal(t, "ExampleCluster", target.Labels["TaskClusterName"])
	assert.Equal(t, "1234567890123456789", target.Labels["TaskId"])

	target, ok = targets["10.4.0.205:9494/metrics"]
	assert.True(t, ok, "Missing target: 10.4.0.205:9494/metrics")
	assert.Equal(t, 10, len(target.Labels))
	assert.Equal(t, "9404", target.Labels["EC2_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "bugbash-tomcat-ec2-bridge-mapped-port", target.Labels["EC2_PROMETHEUS_JOB_NAME"])
	assert.Equal(t, "t3.medium", target.Labels["InstanceType"])
	assert.Equal(t, "subnet-0d0b0212d14b70250", target.Labels["SubnetId"])
	assert.Equal(t, "5", target.Labels["TaskRevision"])
	assert.Equal(t, "vpc-03e9f55a92516a5e4", target.Labels["VpcId"])
	assert.Equal(t, "bugbash-tomcat-prometheus-workload-java-ec2-bridge-mapped-port", target.Labels["container_name"])
	assert.Equal(t, "ExampleCluster", target.Labels["TaskClusterName"])
	assert.Equal(t, "1234567890123456789", target.Labels["TaskId"])
	assert.Equal(t, "bugbash-tomcat-ec2-bridge-mapped-port", target.Labels["job"])
}

func Test_ExportContainerNameSDTarget_EC2_Bridge_DynamicPort(t *testing.T) {
	testExportContainerNameSDTargetEC2BridgeDynamicPort(t, types.NetworkModeBridge, 1)
}

func Test_ExportContainerNameSDTarget_EC2_Bridge_DynamicPort_With_Implicit_NetworkMode(t *testing.T) {
	testExportContainerNameSDTargetEC2BridgeDynamicPort(t, "", 1)
}

func Test_ExportContainerNameSDTarget_EC2_Bridge_DynamicPort_With_NetworkModeNone(t *testing.T) {
	testExportContainerNameSDTargetEC2BridgeDynamicPort(t, types.NetworkModeNone, 0)
}

func testExportContainerNameSDTargetEC2BridgeDynamicPort(t *testing.T, networkMode types.NetworkMode, expectedTargets int) {
	t.Helper()
	fullTask := buildWorkloadEC2BridgeDynamicPort(false, true, "", networkMode)
	log.Print(fullTask)
	if expectedTargets == 0 {
		assert.Equal(t, "", fullTask.getPrivateIp())
	} else {
		assert.Equal(t, "10.4.0.205", fullTask.getPrivateIp())
	}
	config := &ServiceDiscoveryConfig{
		TaskDefinitions: []*TaskDefinitionConfig{
			{
				JobName:              "",
				MetricsPorts:         "9404;9406",
				TaskDefArnPattern:    ".*:task-definition/prometheus-java-tomcat-ec2-awsvpc:[0-9]+",
				MetricsPath:          "/metrics",
				ContainerNamePattern: ".*tomcat-prometheus-workload-java-ec2.*",
			},
		},
	}
	config.TaskDefinitions[0].init()
	assert.Equal(t, []int32{9404, 9406}, config.TaskDefinitions[0].metricsPortList)

	targets := make(map[string]*PrometheusTarget)
	dockerLabelRegex := regexp.MustCompile(prometheusLabelNamePattern)
	fullTask.ExporterInformation(config, dockerLabelRegex, targets)

	assert.Equal(t, expectedTargets, len(targets))
	if expectedTargets == 0 {
		return
	}

	target, ok := targets["10.4.0.205:9494/metrics"]
	log.Print(target)
	assert.True(t, ok, "Missing target: 10.4.0.205:9494/metrics")
	assert.Equal(t, 10, len(target.Labels))
	assert.Equal(t, "9404", target.Labels["EC2_PROMETHEUS_EXPORTER_PORT"])
	assert.Equal(t, "bugbash-tomcat-ec2-bridge-mapped-port", target.Labels["EC2_PROMETHEUS_JOB_NAME"])
	assert.Equal(t, "t3.medium", target.Labels["InstanceType"])
	assert.Equal(t, "subnet-0d0b0212d14b70250", target.Labels["SubnetId"])
	assert.Equal(t, "5", target.Labels["TaskRevision"])
	assert.Equal(t, "vpc-03e9f55a92516a5e4", target.Labels["VpcId"])
	assert.Equal(t, "/metrics", target.Labels["__metrics_path__"])
	assert.Equal(t, "bugbash-tomcat-prometheus-workload-java-ec2-bridge-mapped-port", target.Labels["container_name"])
	assert.Equal(t, "ExampleCluster", target.Labels["TaskClusterName"])
	assert.Equal(t, "1234567890123456789", target.Labels["TaskId"])
}
