// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import "context"

type Processor interface {
	Process(ctx context.Context, cluster string, taskList []*DecoratedTask) ([]*DecoratedTask, error)
	ProcessorName() string
}
