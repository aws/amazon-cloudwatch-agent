// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tail

import (
	"log"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/fdlimit"
)

/*
	Centralize all the tailers which are scheduled for each file
*/

type TailerEnqueue struct {
	Queue chan struct{}
}

func NewTailerFifoQueue() *TailerEnqueue {
	allowedOpenFileLimit, err := fdlimit.CurrentOpenFileLimit()
	if err != nil {
		log.Panic("E! CloudWatchAgent does not able to get the allowed file descriptors")
	}

	return &TailerEnqueue{Queue: make(chan struct{}, allowedOpenFileLimit)}
}

func (t *TailerEnqueue) Enqueue() {
	t.Queue <- struct{}{}
}

func (t *TailerEnqueue) Dequeue() struct{} {
	select {
	case v := <-t.Queue:
		return v
	default:
		return struct{}{}
	}
}

func (t *TailerEnqueue) Size() int {
	return len(t.Queue)
}

func (t *TailerEnqueue) Capacity() int {
	return cap(t.Queue)
}
