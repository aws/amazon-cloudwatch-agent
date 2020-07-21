// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emf

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
)

func TestParseValidValues_V1Only(t *testing.T) {
	parser := EMFParser{
		MetricName: "emf_test",
	}
	expectedValueEntry := `{"AutoScalingGroupName":"eksctl-integ-test-eks-1-12-nodegroup-standard-workers-NodeGroup-4Z1TNI9GQUCP","ClusterName":"integ-test-eks-1-12","EBSVolumeId":"aws://us-west-2b/vol-061b736580fd714f9","InstanceId":"i-075266a887d9d714b","InstanceType":"t3.medium","Namespace":"kube-system","NodeName":"ip-192-168-66-210.us-west-2.compute.internal","PodName":"aws-node","Sources":["cadvisor","calculated"],"Timestamp":"1566164466581","Type":"ContainerFS","Version":"0","container_filesystem_available":0,"container_filesystem_capacity":21462233088,"container_filesystem_usage":24576,"container_filesystem_utilization":0.00011450812177480718,"device":"/dev/nvme0n1p1","fstype":"vfs","kubernetes":{"container_name":"aws-node","docker":{"container_id":"81da96463b99796f3b4988e046030c3e880e732504b0721afc845bc703363733"},"host":"ip-192-168-66-210.us-west-2.compute.internal","labels":{"controller-revision-hash":"555dcdcf79","k8s-app":"aws-node","pod-template-generation":"1"},"namespace_name":"kube-system","pod_id":"95559685-9d02-11e9-b3db-0ecc21a80a76","pod_name":"aws-node-szxhd","pod_owners":[{"owner_kind":"DaemonSet","owner_name":"aws-node"}]},"_aws":{"LogGroupName":"test-log-group-name-v1","LogStreamName":"test-log-stream-name-v1"}}`
	metrics, err := parser.Parse([]byte(expectedValueEntry))
	assert.NoError(t, err)
	assert.Len(t, metrics, 1)
	assert.Equal(t, "emf_test", metrics[0].Name())
	assert.Equal(t, map[string]interface{}{
		"value": expectedValueEntry,
	}, metrics[0].Fields())
	assert.Equal(t, map[string]string{logscommon.LogGroupNameTag: "test-log-group-name-v1", logscommon.LogStreamNameTag: "test-log-stream-name-v1"}, metrics[0].Tags())
}

func TestParseValidValues_V1V0Mixed(t *testing.T) {
	parser := EMFParser{
		MetricName: "emf_test",
	}
	expectedValueEntry := `{"AutoScalingGroupName":"eksctl-integ-test-eks-1-12-nodegroup-standard-workers-NodeGroup-4Z1TNI9GQUCP","ClusterName":"integ-test-eks-1-12","EBSVolumeId":"aws://us-west-2b/vol-061b736580fd714f9","InstanceId":"i-075266a887d9d714b","InstanceType":"t3.medium","Namespace":"kube-system","NodeName":"ip-192-168-66-210.us-west-2.compute.internal","PodName":"aws-node","Sources":["cadvisor","calculated"],"Timestamp":"1566164466581","Type":"ContainerFS","Version":"0","container_filesystem_available":0,"container_filesystem_capacity":21462233088,"container_filesystem_usage":24576,"container_filesystem_utilization":0.00011450812177480718,"device":"/dev/nvme0n1p1","fstype":"vfs","kubernetes":{"container_name":"aws-node","docker":{"container_id":"81da96463b99796f3b4988e046030c3e880e732504b0721afc845bc703363733"},"host":"ip-192-168-66-210.us-west-2.compute.internal","labels":{"controller-revision-hash":"555dcdcf79","k8s-app":"aws-node","pod-template-generation":"1"},"namespace_name":"kube-system","pod_id":"95559685-9d02-11e9-b3db-0ecc21a80a76","pod_name":"aws-node-szxhd","pod_owners":[{"owner_kind":"DaemonSet","owner_name":"aws-node"}]},"log_group_name":"test-log-group-name","log_stream_name":"test-log-stream-name","_aws":{"LogGroupName":"test-log-group-name-v1","LogStreamName":"test-log-stream-name-v1"}}`
	metrics, err := parser.Parse([]byte(expectedValueEntry))
	assert.NoError(t, err)
	assert.Len(t, metrics, 1)
	assert.Equal(t, "emf_test", metrics[0].Name())
	assert.Equal(t, map[string]interface{}{
		"value": expectedValueEntry,
	}, metrics[0].Fields())
	assert.Equal(t, map[string]string{logscommon.LogGroupNameTag: "test-log-group-name-v1", logscommon.LogStreamNameTag: "test-log-stream-name-v1"}, metrics[0].Tags())
}

func TestParseValidValues_OneMessage(t *testing.T) {
	parser := EMFParser{
		MetricName: "emf_test",
	}
	expectedValueEntry := `{"AutoScalingGroupName":"eksctl-integ-test-eks-1-12-nodegroup-standard-workers-NodeGroup-4Z1TNI9GQUCP","ClusterName":"integ-test-eks-1-12","EBSVolumeId":"aws://us-west-2b/vol-061b736580fd714f9","InstanceId":"i-075266a887d9d714b","InstanceType":"t3.medium","Namespace":"kube-system","NodeName":"ip-192-168-66-210.us-west-2.compute.internal","PodName":"aws-node","Sources":["cadvisor","calculated"],"Timestamp":"1566164466581","Type":"ContainerFS","Version":"0","container_filesystem_available":0,"container_filesystem_capacity":21462233088,"container_filesystem_usage":24576,"container_filesystem_utilization":0.00011450812177480718,"device":"/dev/nvme0n1p1","fstype":"vfs","kubernetes":{"container_name":"aws-node","docker":{"container_id":"81da96463b99796f3b4988e046030c3e880e732504b0721afc845bc703363733"},"host":"ip-192-168-66-210.us-west-2.compute.internal","labels":{"controller-revision-hash":"555dcdcf79","k8s-app":"aws-node","pod-template-generation":"1"},"namespace_name":"kube-system","pod_id":"95559685-9d02-11e9-b3db-0ecc21a80a76","pod_name":"aws-node-szxhd","pod_owners":[{"owner_kind":"DaemonSet","owner_name":"aws-node"}]},"log_group_name":"test-log-group-name","log_stream_name":"test-log-stream-name"}`
	metrics, err := parser.Parse([]byte(expectedValueEntry))
	assert.NoError(t, err)
	assert.Len(t, metrics, 1)
	assert.Equal(t, "emf_test", metrics[0].Name())
	assert.Equal(t, map[string]interface{}{
		"value": expectedValueEntry,
	}, metrics[0].Fields())
	assert.Equal(t, map[string]string{logscommon.LogGroupNameTag: "test-log-group-name", logscommon.LogStreamNameTag: "test-log-stream-name"}, metrics[0].Tags())
}

func TestParseValidValues_MultiMessages(t *testing.T) {
	parser := EMFParser{
		MetricName: "emf_test",
	}
	expectedValueEntry := `{"AutoScalingGroupName":"eksctl-integ-test-eks-1-12-nodegroup-standard-workers-NodeGroup-4Z1TNI9GQUCP","ClusterName":"integ-test-eks-1-12","EBSVolumeId":"aws://us-west-2b/vol-061b736580fd714f9","InstanceId":"i-075266a887d9d714b","InstanceType":"t3.medium","Namespace":"kube-system","NodeName":"ip-192-168-66-210.us-west-2.compute.internal","PodName":"aws-node","Sources":["cadvisor","calculated"],"Timestamp":"1566164466581","Type":"ContainerFS","Version":"0","container_filesystem_available":0,"container_filesystem_capacity":21462233088,"container_filesystem_usage":24576,"container_filesystem_utilization":0.00011450812177480718,"device":"/dev/nvme0n1p1","fstype":"vfs","kubernetes":{"container_name":"aws-node","docker":{"container_id":"81da96463b99796f3b4988e046030c3e880e732504b0721afc845bc703363733"},"host":"ip-192-168-66-210.us-west-2.compute.internal","labels":{"controller-revision-hash":"555dcdcf79","k8s-app":"aws-node","pod-template-generation":"1"},"namespace_name":"kube-system","pod_id":"95559685-9d02-11e9-b3db-0ecc21a80a76","pod_name":"aws-node-szxhd","pod_owners":[{"owner_kind":"DaemonSet","owner_name":"aws-node"}]},"log_group_name":"test-log-group-name","log_stream_name":"test-log-stream-name"}`
	inputValueEntry := fmt.Sprintf("%s\n%s", expectedValueEntry, expectedValueEntry)
	metrics, err := parser.Parse([]byte(inputValueEntry))
	assert.NoError(t, err)
	assert.Len(t, metrics, 2)
	for i := 0; i < len(metrics); i++ {
		assert.Equal(t, "emf_test", metrics[0].Name())
		assert.Equal(t, map[string]interface{}{
			"value": expectedValueEntry,
		}, metrics[0].Fields())
		assert.Equal(t, map[string]string{logscommon.LogGroupNameTag: "test-log-group-name", logscommon.LogStreamNameTag: "test-log-stream-name"}, metrics[0].Tags())
	}

	inputValueEntry = fmt.Sprintf(" \n \t %s \t\t\t \n %s\n\n\n\n \t \n ", expectedValueEntry, expectedValueEntry)
	metrics, err = parser.Parse([]byte(inputValueEntry))
	assert.NoError(t, err)
	assert.Len(t, metrics, 2)
	for i := 0; i < len(metrics); i++ {
		assert.Equal(t, "emf_test", metrics[0].Name())
		assert.Equal(t, map[string]interface{}{
			"value": expectedValueEntry,
		}, metrics[0].Fields())
		assert.Equal(t, map[string]string{logscommon.LogGroupNameTag: "test-log-group-name", logscommon.LogStreamNameTag: "test-log-stream-name"}, metrics[0].Tags())
	}
}

func TestParseInvalidValues_OneValidMessageAndOneInvalidMessage(t *testing.T) {
	parser := EMFParser{
		MetricName: "emf_test",
	}
	expectedValueEntry := `{"AutoScalingGroupName":"eksctl-integ-test-eks-1-12-nodegroup-standard-workers-NodeGroup-4Z1TNI9GQUCP","ClusterName":"integ-test-eks-1-12","EBSVolumeId":"aws://us-west-2b/vol-061b736580fd714f9","InstanceId":"i-075266a887d9d714b","InstanceType":"t3.medium","Namespace":"kube-system","NodeName":"ip-192-168-66-210.us-west-2.compute.internal","PodName":"aws-node","Sources":["cadvisor","calculated"],"Timestamp":"1566164466581","Type":"ContainerFS","Version":"0","container_filesystem_available":0,"container_filesystem_capacity":21462233088,"container_filesystem_usage":24576,"container_filesystem_utilization":0.00011450812177480718,"device":"/dev/nvme0n1p1","fstype":"vfs","kubernetes":{"container_name":"aws-node","docker":{"container_id":"81da96463b99796f3b4988e046030c3e880e732504b0721afc845bc703363733"},"host":"ip-192-168-66-210.us-west-2.compute.internal","labels":{"controller-revision-hash":"555dcdcf79","k8s-app":"aws-node","pod-template-generation":"1"},"namespace_name":"kube-system","pod_id":"95559685-9d02-11e9-b3db-0ecc21a80a76","pod_name":"aws-node-szxhd","pod_owners":[{"owner_kind":"DaemonSet","owner_name":"aws-node"}]},"log_group_name":"test-log-group-name","log_stream_name":"test-log-stream-name"}`
	multiLineOfExpectedValueEntry := `
{
    "AutoScalingGroupName": "eksctl-integ-test-eks-1-12-nodegroup-standard-workers-NodeGroup-4Z1TNI9GQUCP",
    "ClusterName": "integ-test-eks-1-12",
    "EBSVolumeId": "aws://us-west-2b/vol-061b736580fd714f9",
    "InstanceId": "i-075266a887d9d714b",
    "InstanceType": "t3.medium",
    "Namespace": "kube-system",
    "NodeName": "ip-192-168-66-210.us-west-2.compute.internal",
    "PodName": "aws-node",
    "Sources": [
        "cadvisor",
        "calculated"
    ],
    "Timestamp": "1566164466581",
    "Type": "ContainerFS",
    "Version": "0",
    "container_filesystem_available": 0,
    "container_filesystem_capacity": 21462233088,
    "container_filesystem_usage": 24576,
    "container_filesystem_utilization": 0.00011450812177480718,
    "device": "/dev/nvme0n1p1",
    "fstype": "vfs",
    "kubernetes": {
        "container_name": "aws-node",
        "docker": {
            "container_id": "81da96463b99796f3b4988e046030c3e880e732504b0721afc845bc703363733"
        },
        "host": "ip-192-168-66-210.us-west-2.compute.internal",
        "labels": {
            "controller-revision-hash": "555dcdcf79",
            "k8s-app": "aws-node",
            "pod-template-generation": "1"
        },
        "namespace_name": "kube-system",
        "pod_id": "95559685-9d02-11e9-b3db-0ecc21a80a76",
        "pod_name": "aws-node-szxhd",
        "pod_owners": [
            {"owner_kind": "DaemonSet","owner_name": "aws-node"}
        ]
    },
    "log_group_name": "test-log-group-name",
    "log_stream_name": "test-log-stream-name"
}
	`
	inputValueEntry := fmt.Sprintf("%s\n%s", expectedValueEntry, multiLineOfExpectedValueEntry)
	metrics, err := parser.Parse([]byte(inputValueEntry))
	assert.NoError(t, err)
	assert.Len(t, metrics, 1)
	for i := 0; i < len(metrics); i++ {
		assert.Equal(t, "emf_test", metrics[0].Name())
		assert.Equal(t, map[string]interface{}{
			"value": expectedValueEntry,
		}, metrics[0].Fields())
		assert.Equal(t, map[string]string{logscommon.LogGroupNameTag: "test-log-group-name", logscommon.LogStreamNameTag: "test-log-stream-name"}, metrics[0].Tags())
	}
}

func TestParseValidValues_NoLogStreamName(t *testing.T) {
	parser := EMFParser{
		MetricName: "emf_test",
	}
	expectedValueEntry := `{
		"log_group_name":"test-log-group-name"
	}`
	metric, err := parser.ParseLine(expectedValueEntry)
	assert.NoError(t, err)
	assert.NotNil(t, metric)
	assert.Equal(t, "emf_test", metric.Name())
	assert.Equal(t, map[string]interface{}{
		"value": expectedValueEntry,
	}, metric.Fields())
	assert.Equal(t, map[string]string{logscommon.LogGroupNameTag: "test-log-group-name"}, metric.Tags())
}

func TestParseInvalidValues_NoLogGroupName(t *testing.T) {
	parser := EMFParser{
		MetricName: "emf_test",
	}
	expectedValueEntry := `{
		"x":"y"
	}`
	metric, err := parser.ParseLine(expectedValueEntry)
	assert.Equal(t, nil, metric)
	assert.Equal(t, fmt.Sprintf("log group name is required to send as structured log: %s", expectedValueEntry), err.Error())
}

func TestParseInvalidValues_InvalidJson(t *testing.T) {
	parser := EMFParser{
		MetricName: "emf_test",
	}
	expectedValueEntry := `"x":"y"`
	metric, err := parser.ParseLine(expectedValueEntry)
	assert.Equal(t, nil, metric)
	assert.Equal(t, fmt.Sprintf("cannot serialize %s to json: invalid character ':' after top-level value", expectedValueEntry), err.Error())
}
