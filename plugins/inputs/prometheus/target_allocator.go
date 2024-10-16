// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	otelpromreceiver "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	tamanager "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/targetallocator"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/scrape"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

const DEFAULT_TLS_CA_FILE_PATH = "/etc/amazon-cloudwatch-observability-agent-cert/tls-ca.crt"

type TargetAllocatorManager struct {
	enabled   bool
	manager   *tamanager.Manager
	config    *otelpromreceiver.Config
	host      component.Host
	sm        *scrape.Manager
	dm        *discovery.Manager
	smLiveCh  chan struct{}
	dmLiveCh  chan struct{}
	taReadyCh chan struct{}
}

func isPodNameAvailable() bool {
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		return false
	}
	return true
}
func loadConfigFromFilename(filename string) (*otelpromreceiver.Config, error) {
	yamlFile, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var stringMap map[string]interface{}
	err = yaml.Unmarshal(yamlFile, &stringMap)
	if err != nil {
		return nil, err
	}
	componentParser := confmap.NewFromStringMap(stringMap)
	if componentParser == nil {
		return nil, fmt.Errorf("unable to parse config from filename %s", filename)
	}
	var cfg otelpromreceiver.Config
	err = componentParser.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Adapter from go-kit/log to zap.Logger
func createLogger(level zapcore.Level) *zap.Logger {
	// Create a base zap logger (you can customize it as needed)
	zapCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), // Use JSON encoder for zap
		zapcore.AddSync(os.Stdout),                               // Output to stdout
		level,                                                    // Set log level to Debug
	)
	// Create the zap logger
	zapLogger := zap.New(zapCore)
	return zapLogger
}

func createTargetAllocatorManager(filename string, logger log.Logger, sm *scrape.Manager, dm *discovery.Manager) *TargetAllocatorManager {
	tam := TargetAllocatorManager{
		enabled: false,
		manager: nil,
		config:  nil,
		host:    nil,
		sm:      sm,
		dm:      dm,
	}
	tam.smLiveCh = make(chan struct{}, 1)
	tam.dmLiveCh = make(chan struct{}, 1)
	tam.taReadyCh = make(chan struct{}, 1)
	err := tam.loadConfig(filename)
	if err != nil {
		level.Error(logger).Log("msg", "Error loading config", "err", err)
		return &tam
	}
	tam.host = nil
	if tam.config == nil {
		return &tam
	}
	tam.enabled = (tam.config.TargetAllocator != nil) && isPodNameAvailable()
	if tam.enabled {
		tam.loadManager()
	}
	return &tam
}
func (tam *TargetAllocatorManager) loadManager() {
	receiverSettings := receiver.Settings{
		ID: component.MustNewID(strings.ReplaceAll(tam.config.TargetAllocator.CollectorID, "-", "_")),
		TelemetrySettings: component.TelemetrySettings{
			Logger:         createLogger(zapcore.DebugLevel),
			TracerProvider: nil,
			MeterProvider:  nil,
			MetricsLevel:   0,
			Resource:       pcommon.Resource{},
			ReportStatus:   nil,
		},
	}

	tam.manager = tamanager.NewManager(receiverSettings, tam.config.TargetAllocator, (*promconfig.Config)(tam.config.PrometheusConfig), false)
}
func (tam *TargetAllocatorManager) loadConfig(filename string) error {
	config, err := loadConfigFromFilename(filename)
	if err != nil {
		return err
	}
	tam.config = config
	if tam.config.TargetAllocator == nil {
		return nil // no target allocator return
	}
	//has target allocator
	tam.config.TargetAllocator.TLSSetting.CAFile = DEFAULT_TLS_CA_FILE_PATH
	return nil
}
func (tam *TargetAllocatorManager) Start() error {
	err := tam.manager.Start(context.Background(), tam.host, tam.sm, tam.dm)
	if err != nil {
		return err
	}
	// go ahead and let dependencies know TA is ready
	close(tam.taReadyCh)
	//don't stop until Scrape and Discovery Manager ends
	<-tam.smLiveCh
	<-tam.dmLiveCh
	return nil
}
func (tam *TargetAllocatorManager) Shutdown() {
	tam.manager.Shutdown()
}
