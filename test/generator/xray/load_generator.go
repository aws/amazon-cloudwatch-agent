// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package xray

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
)

var testErr = errors.New("test error")

type Config struct {
	Interval    time.Duration
	Annotations map[string]interface{}
	Metadata    map[string]map[string]interface{}
}

type Generator struct {
	cfg  *Config
	done chan struct{}
}

func NewLoadGenerator(cfg *Config) *Generator {
	return &Generator{
		cfg:  cfg,
		done: make(chan struct{}),
	}
}

func (g *Generator) Generate(ctx context.Context) error {
	rootCtx, root := xray.BeginSegment(ctx, "load-generator")
	defer root.Close(nil)

	for key, value := range g.cfg.Annotations {
		if err := root.AddAnnotation(key, value); err != nil {
			return err
		}
	}

	for namespace, metadata := range g.cfg.Metadata {
		for key, value := range metadata {
			if err := root.AddMetadataToNamespace(namespace, key, value); err != nil {
				return err
			}
		}
	}

	_, subSeg := xray.BeginSubsegment(rootCtx, "with-error")
	defer subSeg.Close(nil)

	if err := subSeg.AddError(testErr); err != nil {
		return err
	}

	return nil
}

func (g *Generator) Start(ctx context.Context) error {
	ticker := time.NewTicker(g.cfg.Interval)
	for {
		select {
		case <-g.done:
			ticker.Stop()
			return nil
		case <-ticker.C:
			if err := g.Generate(ctx); err != nil {
				return err
			}
		}
	}
}

func (g *Generator) Stop() {
	close(g.done)
}
