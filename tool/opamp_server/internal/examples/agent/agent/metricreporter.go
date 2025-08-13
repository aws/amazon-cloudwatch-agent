package agent

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/process"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	otelresource "go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
)

// MetricReporter is a metric reporter that collects Agent metrics and sends them to an
// OTLP/HTTP destination.
type MetricReporter struct {
	logger types.Logger

	meter           metric.Meter
	meterShutdowner func(ctx context.Context) error
	done            chan struct{}

	// The Agent's process.
	process *process.Process

	// Some example metrics to report.
	processMemoryPhysical metric.Int64ObservableGauge
	counter               metric.Int64Counter
	processCpuTime        metric.Float64ObservableCounter
}

func NewMetricReporter(
	logger types.Logger,
	dest *protobufs.TelemetryConnectionSettings,
	agentType string,
	agentVersion string,
	instanceId uuid.UUID,
) (*MetricReporter, error) {
	// Check the destination credentials to make sure they look like a valid OTLP/HTTP
	// destination.

	if dest.DestinationEndpoint == "" {
		err := fmt.Errorf("metric destination must specify DestinationEndpoint")
		return nil, err
	}
	u, err := url.Parse(dest.DestinationEndpoint)
	if err != nil {
		err := fmt.Errorf("invalid DestinationEndpoint: %v", err)
		return nil, err
	}

	// Create OTLP/HTTP metric exporter.
	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(u.Host),
		otlpmetrichttp.WithURLPath(u.Path),
	}

	if u.Scheme == "http" {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}

	metricExporter, err := otlpmetrichttp.New(context.Background(), opts...)
	if err != nil {
		err := fmt.Errorf("failed to initialize stdoutmetric export pipeline: %v", err)
		return nil, err
	}

	// Define the Resource to be exported with all metrics. Use OpenTelemetry semantic
	// conventions as the OpAMP spec requires:
	// https://github.com/open-telemetry/opamp-spec/blob/main/specification.md#own-telemetry-reporting
	resource, err := otelresource.New(context.Background(),
		otelresource.WithAttributes(
			semconv.ServiceNameKey.String(agentType),
			semconv.ServiceVersionKey.String(agentVersion),
			semconv.ServiceInstanceIDKey.String(instanceId.String()),
		),
	)

	// Wire up the Resource and the exporter together into a meter provider
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(5*time.Second)),
		))

	otel.SetMeterProvider(meterProvider)

	reporter := &MetricReporter{
		logger: logger,
	}

	reporter.done = make(chan struct{})

	reporter.meter = otel.Meter("opamp")

	reporter.process, err = process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return nil, fmt.Errorf("cannot query own process: %v", err)
	}

	// Create some metrics that will be reported according to OpenTelemetry semantic
	// conventions for process metrics (conventions are TBD for now).
	reporter.processCpuTime, err = reporter.meter.Float64ObservableCounter(
		"process.cpu.time",
		metric.WithFloat64Callback(reporter.processCpuTimeFunc),
	)
	if err != nil {
		return nil, fmt.Errorf("could not initiatilize 'process.cpu.time' instrument: %v", err)
	}

	reporter.processMemoryPhysical, err = reporter.meter.Int64ObservableGauge(
		"process.memory.physical_usage",
		metric.WithInt64Callback(reporter.processMemoryPhysicalFunc),
	)
	if err != nil {
		return nil, fmt.Errorf("could not initiatilize 'process.memory.physical_usage' instrument: %v", err)
	}

	reporter.counter, err = reporter.meter.Int64Counter("custom_metric_ticks")
	if err != nil {
		return nil, fmt.Errorf("could not initiatilize 'custom_metric_ticks' instrument: %v", err)
	}

	reporter.meterShutdowner = meterProvider.Shutdown

	go reporter.sendMetrics()

	return reporter, nil
}

func (reporter *MetricReporter) processCpuTimeFunc(ctx context.Context, result metric.Float64Observer) error {
	times, err := reporter.process.Times()
	if err != nil {
		reporter.logger.Errorf(ctx, "Cannot get process CPU times: %v", err)
		return err
	}

	// Report process CPU times, but also add some randomness to make it interesting for demo.
	result.Observe(math.Min(times.User+rand.Float64(), 1), metric.WithAttributes(attribute.String("state", "user")))
	result.Observe(math.Min(times.System+rand.Float64(), 1), metric.WithAttributes(attribute.String("state", "system")))
	result.Observe(math.Min(times.Iowait+rand.Float64(), 1), metric.WithAttributes(attribute.String("state", "wait")))
	return nil
}

func (reporter *MetricReporter) processMemoryPhysicalFunc(ctx context.Context, result metric.Int64Observer) error {
	memory, err := reporter.process.MemoryInfo()
	if err != nil {
		reporter.logger.Errorf(ctx, "Cannot get process memory information: %v", err)
		return err
	}

	// Report the RSS, but also add some randomness to make it interesting for demo.
	result.Observe(int64(memory.RSS) + rand.Int63n(10000000))
	return nil
}

func (reporter *MetricReporter) sendMetrics() {
	// Collect metrics every 5 seconds.
	t := time.NewTicker(time.Second * 5)
	ticks := int64(0)

	for {
		select {
		case <-reporter.done:
			return

		case <-t.C:
			ctx := context.Background()
			reporter.counter.Add(ctx, ticks)
			ticks++
		}
	}
}

func (reporter *MetricReporter) Shutdown() {
	if reporter.done != nil {
		close(reporter.done)
	}

	if reporter.meterShutdowner != nil {
		reporter.meterShutdowner(context.Background())
	}
}
