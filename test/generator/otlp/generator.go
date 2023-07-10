// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"context"
	"time"

	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/maps"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/test/generator"
)

const (
	serviceName                    = "load-generator"
	attributeKeyAwsXrayAnnotations = "aws.xray.annotations"
)

type Generator struct {
	cfg  *generator.Config
	done chan struct{}
}

func NewLoadGenerator(cfg *generator.Config) generator.Generator {
	return &Generator{
		cfg:  cfg,
		done: make(chan struct{}),
	}
}

func (g *Generator) generate(ctx context.Context) error {
	tracer := otel.Tracer("tracer")
	_, span := tracer.Start(ctx, "example-span", trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()

	if len(g.cfg.Annotations) > 0 {
		span.SetAttributes(attribute.StringSlice(attributeKeyAwsXrayAnnotations, maps.Keys(g.cfg.Annotations)))
	}
	span.SetAttributes(g.cfg.Attributes...)
	return nil
}

func (g *Generator) Start(ctx context.Context) error {
	client, shutdown, err := setupClient(ctx)
	if err != nil {
		return err
	}
	defer shutdown(ctx)
	ticker := time.NewTicker(g.cfg.Interval)
	for {
		select {
		case <-g.done:
			ticker.Stop()
			return client.ForceFlush(ctx)
		case <-ticker.C:
			if err = g.generate(ctx); err != nil {
				return err
			}
		}
	}
}

func (g *Generator) Stop() {
	close(g.done)
}

func setupClient(ctx context.Context) (*sdktrace.TracerProvider, func(context.Context) error, error) {
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
	)

	tp, err := setupTraceProvider(ctx, res)
	if err != nil {
		return nil, nil, err
	}

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(xray.Propagator{})

	return tp, func(context.Context) (err error) {
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		err = tp.Shutdown(timeoutCtx)
		if err != nil {
			return err
		}
		return nil
	}, nil
}

func setupTraceProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithIDGenerator(xray.NewIDGenerator()),
	), nil
}
