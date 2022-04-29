// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsdecorator

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/httpclient"
)

type ecsInfo struct {
	hostIP              string
	clusterName         string
	containerInstanceId string
	cgroup              *cgroupScanner
	runningTaskCount    int64
	cpuReserved         int64
	memReserved         int64
	refreshInterval     time.Duration
	shutdownC           chan bool
	httpClient          *httpclient.HttpClient
	sync.RWMutex
}

const (
	ecsAgentEndpoint         = "http://%s:51678/v1/metadata"
	ecsAgentTaskInfoEndpoint = "http://%s:51678/v1/tasks"
	taskStatusRunning        = "RUNNING"
)

type ContainerInstance struct {
	Cluster              string
	ContainerInstanceArn string
}

type ECSContainer struct {
	DockerId string
}
type ECSTask struct {
	KnownStatus string
	ARN         string
	Containers  []ECSContainer
}

type ECSTasksInfo struct {
	Tasks []ECSTask
}

func (e *ecsInfo) updateRunningTaskCount() {
	ecsTasksInfo := e.getTasksInfo()
	runningTaskCount := int64(0)
	cpuReserved := int64(0)
	memReserved := int64(0)
	for _, task := range ecsTasksInfo.Tasks {
		if task.KnownStatus != taskStatusRunning {
			continue
		}
		taskId, err := getTaskCgroupPathFromARN(task.ARN)
		if err != nil {
			log.Printf("W! failed to get ecs taskid from arn: %v", err)
			continue
		}

		// ignore the one only consume 2 shares which is the default value in cgroup
		if cr := e.cgroup.getCPUReserved(taskId, e.clusterName); cr > 2 {
			cpuReserved += cr
		}
		memReserved += e.cgroup.getMEMReserved(taskId, e.clusterName, task.Containers)

		runningTaskCount += 1
	}

	e.Lock()
	defer e.Unlock()
	e.runningTaskCount = runningTaskCount
	e.cpuReserved = cpuReserved
	e.memReserved = memReserved
}

func (e *ecsInfo) getRunningTaskCount() int64 {
	e.RLock()
	defer e.RUnlock()
	return e.runningTaskCount
}

func (e *ecsInfo) getCpuReserved() int64 {
	e.RLock()
	defer e.RUnlock()
	return e.cpuReserved
}

func (e *ecsInfo) getMemReserved() int64 {
	e.RLock()
	defer e.RUnlock()
	return e.memReserved
}

func newECSInfo(hostIP string) (e *ecsInfo) {
	e = &ecsInfo{hostIP: hostIP, refreshInterval: 1 * time.Minute, shutdownC: make(chan bool), httpClient: httpclient.New()}
	containerInstance := e.getContainerInstanceInfo()
	//Sample Cluster Name: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-agent-introspection.html
	e.clusterName = containerInstance.Cluster
	e.containerInstanceId = e.getContainerInstanceIdFromArn(containerInstance.ContainerInstanceArn)
	e.cgroup = newCGroupScannerForContainer()
	e.updateRunningTaskCount()
	go func() {
		refreshTicker := time.NewTicker(e.refreshInterval)
		defer refreshTicker.Stop()
		for {
			select {
			case <-refreshTicker.C:
				e.updateRunningTaskCount()
			case <-e.shutdownC:
				refreshTicker.Stop()
				return
			}
		}
	}()
	return
}

func (e *ecsInfo) shutdown() {
	close(e.shutdownC)
}

func (e *ecsInfo) getECSAgentEndpoint() string {
	return fmt.Sprintf(ecsAgentEndpoint, e.hostIP)
}

func (e *ecsInfo) getECSAgentTaskInfoEndpoint() string {
	return fmt.Sprintf(ecsAgentTaskInfoEndpoint, e.hostIP)

}

// There are two formats of ContainerInstance ARN (https://docs.aws.amazon.com/AmazonECS/latest/userguide/ecs-account-settings.html#ecs-resource-ids)
// arn:aws:ecs:region:aws_account_id:container-instance/container-instance-id
// arn:aws:ecs:region:aws_account_id:container-instance/cluster-name/container-instance-id
// This function will return "container-instance-id" for both ARN format
func (e *ecsInfo) getContainerInstanceIdFromArn(arn string) (containerInstanceId string) {
	// When splitting the ARN with ":", the 6th segments could be either:
	// container-instance/47c0ab6e-2c2c-475e-9c30-b878fa7a8c3d or
	// container-instance/cluster-name/47c0ab6e-2c2c-475e-9c30-b878fa7a8c3d
	if splitedList := strings.Split(arn, ":"); len(splitedList) >= 6 {
		// Further splitting tmpResult with "/", it could be splitted into either 2 or 3
		// Characters of "cluster-name" is only allowed to be letters, numbers and hyphens
		tmpResult := strings.Split(splitedList[5], "/")
		if len(tmpResult) == 2 {
			containerInstanceId = tmpResult[1]
			return
		} else if len(tmpResult) == 3 {
			containerInstanceId = tmpResult[2]
			return
		}
	}

	log.Printf("E! can't get ecs container instance id from ContainerInstance arn: %v", arn)
	return

}

func (e *ecsInfo) getContainerInstanceInfo() (containerInstance *ContainerInstance) {
	containerInstance = &ContainerInstance{}
	resp, err := e.httpClient.Request(e.getECSAgentEndpoint())
	if err != nil {
		log.Printf("W! failing to call ecsagent endpoint, error: %v", err)
		return containerInstance
	}

	err = json.Unmarshal(resp, containerInstance)
	if err != nil {
		log.Printf("W! unable to parse resp from ecsagent endpoint, error: %v", err)
		log.Printf("D! resp content is %s", string(resp))
	}
	return
}

func (e *ecsInfo) getTasksInfo() (ecsTasksInfo *ECSTasksInfo) {
	ecsTasksInfo = &ECSTasksInfo{}
	resp, err := e.httpClient.Request(e.getECSAgentTaskInfoEndpoint())
	if err != nil {
		log.Printf("W! failing to call ecsagent taskinfo endpoint, error: %v", err)
		return ecsTasksInfo
	}

	err = json.Unmarshal(resp, ecsTasksInfo)
	if err != nil {
		log.Printf("W! unable to parse resp from ecsagent taskinfo endpoint, error: %v", err)
		log.Printf("D! resp content is %s", string(resp))
	}
	return
}

// There are two formats of Task ARN (https://docs.aws.amazon.com/AmazonECS/latest/userguide/ecs-account-settings.html#ecs-resource-ids)
// arn:aws:ecs:region:aws_account_id:task/task-id
// arn:aws:ecs:region:aws_account_id:task/cluster-name/task-id
// we should get "task-id" as result no matter what format the ARN is.
func getTaskCgroupPathFromARN(arn string) (string, error) {
	result := strings.Split(arn, ":")
	if len(result) < 6 {
		return "", fmt.Errorf("invalid ecs task arn: %v", result)
	}

	result = strings.Split(result[5], "/")
	if len(result) == 2 {
		return result[1], nil
	} else if len(result) == 3 {
		return result[2], nil
	} else {
		return "", fmt.Errorf("invalid ecs task arn: %v", result)
	}
}
