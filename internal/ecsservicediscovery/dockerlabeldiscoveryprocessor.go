// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

// Tag the Tasks that matched the Docker Label based SD Discovery
type DockerLabelDiscoveryProcessor struct {
	label string
}

func NewDockerLabelDiscoveryProcessor(d *DockerLabelConfig) *DockerLabelDiscoveryProcessor {
	if d == nil {
		return &DockerLabelDiscoveryProcessor{label: ""}
	}
	return &DockerLabelDiscoveryProcessor{label: d.PortLabel}
}

func (p *DockerLabelDiscoveryProcessor) Process(cluster string, taskList []*DecoratedTask) ([]*DecoratedTask, error) {
	if p.label == "" {
		return taskList, nil
	}

	for _, v := range taskList {
		for _, d := range v.TaskDefinition.ContainerDefinitions {
			if _, ok := d.DockerLabels[p.label]; ok {
				v.DockerLabelBased = true
				break
			}
		}
	}
	return taskList, nil
}

func (p *DockerLabelDiscoveryProcessor) ProcessorName() string {
	return "DockerLabelDiscoveryProcessor"
}
