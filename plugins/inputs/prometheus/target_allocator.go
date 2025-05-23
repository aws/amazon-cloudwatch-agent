// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	otelpromreceiver "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	tamanager "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/targetallocator"
	"github.com/prometheus/common/promslog"
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

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

var DEFAULT_TLS_CA_FILE_PATH = filepath.Join("/etc", "amazon-cloudwatch-observability-agent-cert", "tls-ca.crt")

const DEFAULT_TLS_RELOAD_INTERVAL_SECONDS = 10 * time.Second

type TargetAllocatorManager struct {
	enabled             bool
	host                component.Host
	shutdownCh          chan struct{}
	taReadyCh           chan struct{}
	reloadConfigHandler func(config *promconfig.Config)
	manager             *tamanager.Manager
	config              *otelpromreceiver.Config
	sm                  *scrape.Manager
	dm                  *discovery.Manager
	logger              *slog.Logger
}

func isPodNameAvailable() bool {
	podName := os.Getenv(envconfig.PodName)
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
func createLogger(level *promslog.AllowedLevel) (*slog.Logger, error) {
	var logLevel slog.Level
	switch level.String() {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
		return nil, fmt.Errorf("invalid log level: %s, defaulting to info", level.String())
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	return logger, nil
}

func createTargetAllocatorManager(filename string, logger *slog.Logger, logLevel *promslog.AllowedLevel, sm *scrape.Manager, dm *discovery.Manager) *TargetAllocatorManager {
	tam := TargetAllocatorManager{
		enabled:             false,
		manager:             nil,
		config:              nil,
		host:                nil,
		sm:                  sm,
		dm:                  dm,
		shutdownCh:          make(chan struct{}, 1),
		taReadyCh:           make(chan struct{}, 1),
		reloadConfigHandler: nil,
		logger:              logger,
	}
	err := tam.loadConfig(filename)
	if err != nil {
		logger.Error("Could not load config for target allocator from file", "filename", filename, "err", err)
		return &tam
	}
	if tam.config == nil {
		return &tam
	}
	tam.enabled = (tam.config.TargetAllocator != nil) && isPodNameAvailable()
	if tam.enabled {
		tam.loadManager(logLevel)
	}
	return &tam
}

func (tam *TargetAllocatorManager) loadManager(logLevel *promslog.AllowedLevel) {
	zapLogger, err := createZapLogger(logLevel)
	if err != nil {
		tam.logger.Error("Error creating zap logger", "error", err)
	}
	receiverSettings := receiver.Settings{
		ID: component.MustNewID(strings.ReplaceAll(tam.config.TargetAllocator.CollectorID, "-", "_")),
		TelemetrySettings: component.TelemetrySettings{
			Logger:         zapLogger,
			TracerProvider: nil,
			MeterProvider:  nil,
			Resource:       pcommon.Resource{},
		},
	}

	tam.manager = tamanager.NewManager(receiverSettings, tam.config.TargetAllocator, (*promconfig.Config)(tam.config.PrometheusConfig), false)
}

func createZapLogger(level *promslog.AllowedLevel) (*zap.Logger, error) {
	var zapLevel zapcore.Level
	switch level.String() {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
		return nil, fmt.Errorf("invalid log level: %s, defaulting to info", level.String())
	}

	zapCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(os.Stdout),
		zapLevel,
	)

	zapLogger := zap.New(zapCore)
	return zapLogger, nil
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
	tam.config.TargetAllocator.TLSSetting.ReloadInterval = DEFAULT_TLS_RELOAD_INTERVAL_SECONDS
	return nil
}
func (tam *TargetAllocatorManager) Run() error {
	err := tam.manager.Start(context.Background(), tam.host, tam.sm, tam.dm)
	if err != nil {
		return err
	}
	err = tam.reloadConfigTicker()
	if err != nil {
		tam.manager.Shutdown()
		return err
	}
	// go ahead and let dependencies know TA is ready
	close(tam.taReadyCh)
	//don't stop until shutdown
	<-tam.shutdownCh
	return nil
}
func (tam *TargetAllocatorManager) Shutdown() {
	tam.manager.Shutdown()
	close(tam.shutdownCh)
}
func (tam *TargetAllocatorManager) AttachReloadConfigHandler(handler func(config *promconfig.Config)) {
	tam.reloadConfigHandler = handler
}

func (tam *TargetAllocatorManager) reloadConfigTicker() error {
	if tam.config.TargetAllocator == nil {
		tam.logger.Error("target Allocator is not configured properly")
		return fmt.Errorf("target Allocator is not configured properly")
	}
	if tam.reloadConfigHandler == nil {
		tam.logger.Error("target allocator reload config handler is not configured properly")
		return fmt.Errorf("target allocator reload config handler is not configured properly")
	}

	tam.logger.Info("Starting Target Allocator Reload Config Ticker",
		"interval", tam.config.TargetAllocator.Interval.Seconds())

	ticker := time.NewTicker(tam.config.TargetAllocator.Interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				tam.reloadConfigHandler((*promconfig.Config)(tam.config.PrometheusConfig))
			case <-tam.shutdownCh:
				ticker.Stop()
				// Stop the ticker and exit when stop is signaled
				tam.logger.Info("Stopping Target Allocator Reload Config Ticker")
				return
			}
		}
	}()
	return nil
}
