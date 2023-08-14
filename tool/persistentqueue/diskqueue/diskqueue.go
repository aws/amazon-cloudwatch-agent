// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package diskqueue

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/tool"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/nsqio/go-diskqueue"

	"github.com/aws/amazon-cloudwatch-agent/tool/persistentqueue"
)

type diskQueue struct {
	marshal   persistentqueue.Marshaler
	unmarshal persistentqueue.Unmarshaler
	queue     diskqueue.Interface
	size      int64
	logger    telegraf.Logger
}

func NewPersistentQueue(
	name string,
	directory string,
	size int64,
	maxBytesPerFile int64,
	minMsgSize int32,
	maxMsgSize int32,
	syncEvery int64,
	syncTimeout time.Duration,
	marshal persistentqueue.Marshaler,
	unmarshal persistentqueue.Unmarshaler,
	logger telegraf.Logger,
) persistentqueue.PersistentQueue {
	return &diskQueue{
		marshal:   marshal,
		unmarshal: unmarshal,
		size:      size,
		queue: diskqueue.New(
			name,
			directory,
			maxBytesPerFile,
			minMsgSize,
			maxMsgSize,
			syncEvery,
			syncTimeout,
			getDiskQueueLogger(logger),
		),
		logger: logger,
	}
}

func (dq *diskQueue) Enqueue(obj interface{}) error {
	marshaledObj, err := dq.marshal(obj)
	if err != nil {
		dq.logger.Debugf("errors happen when marshal")
		return err
	}

	compressedObj, err := tool.Compress(marshaledObj)
	if err != nil {

		return err
	}
	for dq.queue.Depth() >= dq.size {
		<-dq.queue.ReadChan()
	}
	dq.logger.Debugf("put function start work")
	//return dq.queue.Put(marshaledObj)
	return dq.queue.Put(compressedObj)
}

func (dq *diskQueue) Dequeue() (interface{}, error) {
	//obj := <-dq.queue.ReadChan()
	obj, err := tool.Uncompress(<-dq.queue.ReadChan())
	if err != nil {
		return nil, err
	}
	return dq.unmarshal(obj)
}

func (dq *diskQueue) Depth() int64 {
	return dq.queue.Depth()
}

func (dq *diskQueue) Close() error {
	return dq.queue.Close()
}

func getDiskQueueLogger(logger telegraf.Logger) func(level diskqueue.LogLevel, f string, args ...interface{}) {
	return func(level diskqueue.LogLevel, f string, args ...interface{}) {
		logFn := logger.Debugf
		switch level {
		case diskqueue.DEBUG:
			logFn = logger.Debugf
		case diskqueue.INFO:
			logFn = logger.Debugf
		case diskqueue.WARN:
			logFn = logger.Debugf
		case diskqueue.ERROR:
			logFn = logger.Debugf
		case diskqueue.FATAL:
			logFn = logger.Debugf
		}

		logFn(fmt.Sprintf(f, args))
	}
}
