// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"

	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/tail"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

var ErrOutputStopped = errors.New("Output plugin stopped")

// A LogCollection is a collection of LogSrc, a plugin which can provide many LogSrc
type LogCollection interface {
	FindLogSrc() []LogSrc
	Start(acc telegraf.Accumulator) error
}

type LogEvent interface {
	Message() string
	Time() time.Time
	Done()
}

type LogEntityProvider interface {
	Entity() *cloudwatchlogs.Entity
}

// A LogSrc is a single source where log events are generated
// e.g. a single log file
type LogSrc interface {
	LogEntityProvider
	SetOutput(func(LogEvent))
	Group() string
	Stream() string
	Destination() string
	Description() string
	Retention() int
	Class() string
	Stop()
}

// A LogBackend is able to return a LogDest of a given name.
// The same name should always return the same LogDest.
type LogBackend interface {
	CreateDest(string, string, int, string, LogSrc) LogDest
}

// A LogDest represents a final endpoint where log events are published to.
// e.g. a particular log stream in cloudwatchlogs.
type LogDest interface {
	Publish(events []LogEvent) error
}

// LogAgent is the agent handles pure log pipelines
type LogAgent struct {
	Config                    *config.Config
	backends                  map[string]LogBackend
	destNames                 map[LogDest]string
	collections               []LogCollection
	retentionAlreadyAttempted map[string]bool
}

func NewLogAgent(c *config.Config) *LogAgent {
	return &LogAgent{
		Config:                    c,
		backends:                  make(map[string]LogBackend),
		destNames:                 make(map[LogDest]string),
		retentionAlreadyAttempted: make(map[string]bool),
	}
}

// Run LogAgent will scan all input and output plugins for LogCollection and LogBackend.
// And connect all the LogSrc from the LogCollection found to the respective LogDest
// based on the configured "destination", and "name"
func (l *LogAgent) Run(ctx, monitoringCtx context.Context) {
	log.Printf("I! [logagent] starting")

	// Initialize backends and collections
	for _, output := range l.Config.Outputs {
		backend, ok := output.Output.(LogBackend)
		if !ok {
			continue
		}
		log.Printf("I! [logagent] found plugin %v is a log backend", output.Config.Name)
		name := output.Config.Alias
		if name == "" {
			name = output.Config.Name
		}
		l.backends[name] = backend
	}

	var lastFileCount int64 = -1
	zeroCountDuration := time.Duration(0)

	// Start file monitoring in a separate goroutine with monitoring context
	go func() {
		monitorTicker := time.NewTicker(time.Second)
		defer monitorTicker.Stop()

		for {
			select {
			case <-monitorTicker.C:
				currentCount := tail.OpenFileCount.Load()
				log.Printf("D! [logagent] New333---open file count, %v", currentCount)

				// If count drops to 0, track how long it's been 0
				if currentCount == 0 {
					if lastFileCount > 0 {
						// File count just dropped to 0
						log.Printf("I! [logagent] File count dropped to 0, initiating recovery")
						l.restartCollections()
					}
					zeroCountDuration += time.Second
					// If count has been 0 for more than 5 seconds, try to recover
					if zeroCountDuration >= 5*time.Second {
						log.Printf("I! [logagent] Attempting to recover file monitoring")
						l.restartCollections()
						zeroCountDuration = 0
					}
				} else {
					zeroCountDuration = 0
				}
				lastFileCount = currentCount
			case <-monitoringCtx.Done():
				log.Printf("I! [logagent] Stopping file monitoring")
				return
			}
		}
	}()

	// Start initial collections
	l.startCollections()

	// Main processing loop
	processTicker := time.NewTicker(time.Second)
	defer processTicker.Stop()

	for {
		select {
		case <-processTicker.C:
			l.processCollections()
		case <-ctx.Done():
			log.Printf("I! [logagent] Shutting down log processing")
			return
		}
	}
}

func (l *LogAgent) restartCollections() {
	log.Printf("I! [logagent] Restarting collections")
	// Stop existing collections
	for _, collection := range l.collections {
		if stopper, ok := collection.(interface{ Stop() }); ok {
			stopper.Stop()
		}
	}

	// Clear existing collections
	l.collections = nil

	// Start new collections
	for _, input := range l.Config.Inputs {
		if collection, ok := input.Input.(LogCollection); ok {
			log.Printf("I! [logagent] Restarting collection for plugin %v", input.Config.Name)
			err := collection.Start(nil)
			if err != nil {
				log.Printf("E! could not restart log collection %v err %v", input.Config.Name, err)
				continue
			}
			l.collections = append(l.collections, collection)
		}
	}
}

// Add these helper methods to LogAgent
func (l *LogAgent) startCollections() {
	for _, input := range l.Config.Inputs {
		if collection, ok := input.Input.(LogCollection); ok {
			log.Printf("I! [logagent] Starting collection for plugin %v", input.Config.Name)
			err := collection.Start(nil)
			if err != nil {
				log.Printf("E! could not start log collection %v err %v", input.Config.Name, err)
				continue
			}
			l.collections = append(l.collections, collection)
		}
	}
}

func (l *LogAgent) processCollections() {
	for _, c := range l.collections {
		srcs := c.FindLogSrc()
		for _, src := range srcs {
			dname := src.Destination()
			logGroup := src.Group()
			logStream := src.Stream()
			description := src.Description()
			retention := src.Retention()
			logGroupClass := src.Class()
			backend, ok := l.backends[dname]
			if !ok {
				log.Printf("E! [logagent] Failed to find destination %s for log source %s/%s(%s) ", dname, logGroup, logStream, description)
				continue
			}
			retention = l.checkRetentionAlreadyAttempted(retention, logGroup)
			dest := backend.CreateDest(logGroup, logStream, retention, logGroupClass, src)
			l.destNames[dest] = dname
			log.Printf("I! [logagent] piping log from %s/%s(%s) to %s with retention %d", logGroup, logStream, description, dname, retention)
			go l.runSrcToDest(src, dest)
		}
	}
}

func (l *LogAgent) runSrcToDest(src LogSrc, dest LogDest) {
	eventsCh := make(chan LogEvent)
	defer src.Stop()

	closed := false
	src.SetOutput(func(e LogEvent) {
		if closed {
			return
		}
		if e == nil {
			close(eventsCh)
			closed = true
			log.Printf("I! [logagent] Log src has stopped for %v/%v(%v)", src.Group(), src.Stream(), src.Description())
			return
		}
		eventsCh <- e
	})

	for e := range eventsCh {
		err := dest.Publish([]LogEvent{e})
		if err == ErrOutputStopped {
			log.Printf("I! [logagent] Log destination %v has stopped, finalizing %v/%v", l.destNames[dest], src.Group(), src.Stream())
			return
		}
		if err != nil {
			log.Printf("E! [logagent] Failed to publish log to %v, error: %v", l.destNames[dest], err)
			return
		}
	}
}

func (l *LogAgent) checkRetentionAlreadyAttempted(retention int, logGroup string) int {
	if retention > 0 && l.retentionAlreadyAttempted[logGroup] {
		log.Printf("D! [logagent] Retention already set for log group %s, current retention %d", logGroup, retention)
		retention = -1
	} else if retention > 0 {
		log.Printf("I! First time setting retention for log group %s, update map to avoid setting twice", logGroup)
		l.retentionAlreadyAttempted[logGroup] = true
	}
	return retention
}
