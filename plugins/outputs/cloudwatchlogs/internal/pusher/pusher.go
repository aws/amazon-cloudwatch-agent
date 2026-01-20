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
	wg *sync.WaitGroup,
	_ int,
	retryHeap RetryHeap,
) *Pusher {
	s := createSender(logger, service, targetManager, workerPool, retryHeap)

	q := newQueue(logger, target, flushTimeout, entityProvider, s, wg)
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

func (p *Pusher) Stop() {
	p.Queue.Stop()
	p.Sender.Stop()
}

// createSender initializes a Sender. Wraps it in a senderPool if a WorkerPool is provided.
func createSender(
	logger telegraf.Logger,
	service cloudWatchLogsService,
	targetManager TargetManager,
	workerPool WorkerPool,
	retryHeap RetryHeap,
) Sender {
	s := newSender(logger, service, targetManager, retryHeap)
	if workerPool == nil {
		return s
	}
	return newSenderPool(workerPool, s)
}
