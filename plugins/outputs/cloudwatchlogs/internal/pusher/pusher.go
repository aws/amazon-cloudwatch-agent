// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/aws/amazon-cloudwatch-agent/logs"
)

// Pusher connects the Queue to the Sender.
type Pusher struct {
	Target
	Queue
	Service        cloudWatchLogsService
	TargetManager  TargetManager
	EntityProvider logs.LogEntityProvider
	Sender         Sender
}

// NewPusher creates a new Pusher instance with a new Queue and Sender. Calls PutRetentionPolicy using the
// TargetManager.
func NewPusher(
	logger telegraf.Logger,
	target Target,
	service cloudWatchLogsService,
	targetManager TargetManager,
	entityProvider logs.LogEntityProvider,
	workerPool WorkerPool,
	flushTimeout time.Duration,
	retryDuration time.Duration,
	stop <-chan struct{},
	wg *sync.WaitGroup,
) *Pusher {
	s := createSender(logger, service, targetManager, workerPool, retryDuration, stop)
	q := newQueue(logger, target, flushTimeout, entityProvider, s, stop, wg)
	targetManager.PutRetentionPolicy(target)
	return &Pusher{
		Target:         target,
		Queue:          q,
		Service:        service,
		TargetManager:  targetManager,
		EntityProvider: entityProvider,
		Sender:         s,
	}
}

// createSender initializes a Sender. Wraps it in a senderPool if a WorkerPool is provided.
func createSender(
	logger telegraf.Logger,
	service cloudWatchLogsService,
	targetManager TargetManager,
	workerPool WorkerPool,
	retryDuration time.Duration,
	stop <-chan struct{},
) Sender {
	s := newSender(logger, service, targetManager, retryDuration, stop)
	if workerPool == nil {
		return s
	}
	return newSenderPool(workerPool, s)
}
