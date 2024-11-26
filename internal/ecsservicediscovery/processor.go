// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

type Processor interface {
	Process(cluster string, taskList []*DecoratedTask) ([]*DecoratedTask, error)
	ProcessorName() string
}
