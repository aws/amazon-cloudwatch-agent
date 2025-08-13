package supervisor

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"

	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/internal/examples/supervisor/supervisor/commander"
	"github.com/open-telemetry/opamp-go/internal/examples/supervisor/supervisor/config"
	"github.com/open-telemetry/opamp-go/internal/examples/supervisor/supervisor/healthchecker"
	"github.com/open-telemetry/opamp-go/protobufs"
)

// This Supervisor is developed specifically for OpenTelemetry Collector.
const agentType = "io.opentelemetry.collector"

// TODO: fetch agent version from Collector executable or by some other means.
const agentVersion = "1.0.0"

// Supervisor implements supervising of OpenTelemetry Collector and uses OpAMPClient
// to work with an OpAMP Server.
type Supervisor struct {
	logger types.Logger

	// Commander that starts/stops the Agent process.
	commander *commander.Commander

	startedAt time.Time

	healthCheckTicker  *backoff.Ticker
	healthChecker      *healthchecker.HttpHealthChecker
	lastHealthCheckErr error

	// Supervisor's own config.
	config config.Supervisor

	// The version of the Agent being Supervised.
	agentVersion string

	// Agent's instance id.
	instanceId uuid.UUID

	// A config section to be added to the Collector's config to fetch its own metrics.
	// TODO: store this persistently so that when starting we can compose the effective
	// config correctly.
	agentConfigOwnMetricsSection atomic.Value

	// Final effective config of the Collector.
	effectiveConfig atomic.Value

	// Location of the effective config file.
	effectiveConfigFilePath string

	// Last received remote config.
	remoteConfig *protobufs.AgentRemoteConfig

	// A channel to indicate there is a new config to apply.
	hasNewConfig chan struct{}

	// The OpAMP client to connect to the OpAMP Server.
	opampClient client.OpAMPClient
}

func NewSupervisor(logger types.Logger) (*Supervisor, error) {
	s := &Supervisor{
		logger:                  logger,
		agentVersion:            agentVersion,
		hasNewConfig:            make(chan struct{}, 1),
		effectiveConfigFilePath: "effective.yaml",
	}

	if err := s.loadConfig(); err != nil {
		return nil, fmt.Errorf("Error loading config: %v", err)
	}

	s.createInstanceId()
	logger.Debugf(context.Background(), "Supervisor starting, id=%v, type=%s, version=%s.",
		s.instanceId, agentType, agentVersion)

	s.loadAgentEffectiveConfig()

	if err := s.startOpAMP(); err != nil {
		return nil, fmt.Errorf("Cannot start OpAMP client: %v", err)
	}

	var err error
	s.commander, err = commander.NewCommander(
		s.logger,
		s.config.Agent,
		"--config", s.effectiveConfigFilePath,
	)
	if err != nil {
		return nil, err
	}

	go s.runAgentProcess()

	return s, nil
}

func (s *Supervisor) loadConfig() error {
	const configFile = "supervisor.yaml"

	k := koanf.New("::")
	if err := k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
		return err
	}

	if err := k.Unmarshal("", &s.config); err != nil {
		return fmt.Errorf("cannot parse %v: %w", configFile, err)
	}

	return nil
}

func (s *Supervisor) startOpAMP() error {
	s.opampClient = client.NewWebSocket(s.logger)

	settings := types.StartSettings{
		OpAMPServerURL: s.config.Server.Endpoint,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		InstanceUid: types.InstanceUid(s.instanceId),
		Callbacks: types.Callbacks{
			OnConnect: func(ctx context.Context) {
				s.logger.Debugf(ctx, "Connected to the server.")
			},
			OnConnectFailed: func(ctx context.Context, err error) {
				s.logger.Errorf(ctx, "Failed to connect to the server: %v", err)
			},
			OnError: func(ctx context.Context, err *protobufs.ServerErrorResponse) {
				s.logger.Errorf(ctx, "Server returned an error response: %v", err.ErrorMessage)
			},
			GetEffectiveConfig: func(ctx context.Context) (*protobufs.EffectiveConfig, error) {
				return s.createEffectiveConfigMsg(), nil
			},
			OnMessage: s.onMessage,
		},
		Capabilities: protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnMetrics |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth,
	}
	err := s.opampClient.SetAgentDescription(s.createAgentDescription())
	if err != nil {
		return err
	}

	err = s.opampClient.SetHealth(&protobufs.ComponentHealth{Healthy: false})
	if err != nil {
		return err
	}

	s.logger.Debugf(context.Background(), "Starting OpAMP client...")

	err = s.opampClient.Start(context.Background(), settings)
	if err != nil {
		return err
	}

	s.logger.Debugf(context.Background(), "OpAMP Client started.")

	return nil
}

func (s *Supervisor) createInstanceId() {
	// Generate instance id.

	uid, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	s.instanceId = uid

	// TODO: set instanceId in the Collector config.
}

func keyVal(key, val string) *protobufs.KeyValue {
	return &protobufs.KeyValue{
		Key: key,
		Value: &protobufs.AnyValue{
			Value: &protobufs.AnyValue_StringValue{StringValue: val},
		},
	}
}

func (s *Supervisor) createAgentDescription() *protobufs.AgentDescription {
	hostname, _ := os.Hostname()

	// Create Agent description.
	return &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			keyVal("service.name", agentType),
			keyVal("service.version", s.agentVersion),
		},
		NonIdentifyingAttributes: []*protobufs.KeyValue{
			keyVal("os.type", runtime.GOOS),
			keyVal("host.name", hostname),
		},
	}
}

func (s *Supervisor) composeExtraLocalConfig() string {
	return fmt.Sprintf(`
service:
  telemetry:
    logs:
      # Enables JSON log output for the Agent.
      encoding: json
    resource:
      # Set resource attributes required by OpAMP spec.
      # See https://github.com/open-telemetry/opamp-spec/blob/main/specification.md#agentdescriptionidentifying_attributes
      service.name: %s
      service.version: %s
      service.instance.id: %s

  # Enable extension to allow the Supervisor to check health.
  extensions: [health_check]

extensions:
  health_check:
    # TODO: choose the endpoint dynamically.
`,
		agentType,
		s.agentVersion,
		s.instanceId.String(),
	)
}

func (s *Supervisor) loadAgentEffectiveConfig() error {
	var effectiveConfigBytes []byte

	effFromFile, err := os.ReadFile(s.effectiveConfigFilePath)
	if err == nil {
		// We have an effective config file.
		effectiveConfigBytes = effFromFile
	} else {
		// No effective config file, just use the initial config.
		effectiveConfigBytes = []byte(s.composeExtraLocalConfig())
	}

	s.effectiveConfig.Store(string(effectiveConfigBytes))

	return nil
}

// createEffectiveConfigMsg create an EffectiveConfig with the content of the
// current effective config.
func (s *Supervisor) createEffectiveConfigMsg() *protobufs.EffectiveConfig {
	cfgStr, ok := s.effectiveConfig.Load().(string)
	if !ok {
		cfgStr = ""
	}

	cfg := &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"": {Body: []byte(cfgStr)},
			},
		},
	}

	return cfg
}

func (s *Supervisor) setupOwnMetrics(ctx context.Context, settings *protobufs.TelemetryConnectionSettings) (configChanged bool) {
	var cfg string
	if settings.DestinationEndpoint == "" {
		// No destination. Disable metric collection.
		s.logger.Debugf(ctx, "Disabling own metrics pipeline in the config")
		cfg = ""
	} else {
		s.logger.Debugf(ctx, "Enabling own metrics pipeline in the config")

		// TODO: choose the scraping port dynamically instead of hard-coding to 8888.
		cfg = fmt.Sprintf(
			`
receivers:
  # Collect own metrics
  prometheus/own_metrics:
    config:
      scrape_configs:
        - job_name: 'otel-collector'
          scrape_interval: 10s
          static_configs:
            - targets: ['0.0.0.0:8888']  
exporters:
  otlphttp/own_metrics:
    metrics_endpoint: %s

service:
  pipelines:
    metrics/own_metrics:
      receivers: [prometheus/own_metrics]
      exporters: [otlphttp/own_metrics]
`, settings.DestinationEndpoint,
		)
	}

	s.agentConfigOwnMetricsSection.Store(cfg)

	// Need to recalculate the Agent config so that the metric config is included in it.
	configChanged, err := s.recalcEffectiveConfig(ctx)
	if err != nil {
		return
	}

	return configChanged
}

// composeEffectiveConfig composes the effective config from multiple sources:
// 1) the remote config from OpAMP Server, 2) the own metrics config section,
// 3) the local override config that is hard-coded in the Supervisor.
func (s *Supervisor) composeEffectiveConfig(ctx context.Context, config *protobufs.AgentRemoteConfig) (configChanged bool, err error) {
	k := koanf.New(".")

	// Begin with empty config. We will merge received configs on top of it.
	if err := k.Load(rawbytes.Provider([]byte{}), yaml.Parser()); err != nil {
		return false, err
	}

	// Sort to make sure the order of merging is stable.
	var names []string
	for name := range config.Config.ConfigMap {
		if name == "" {
			// skip instance config
			continue
		}
		names = append(names, name)
	}

	sort.Strings(names)

	// Append instance config as the last item.
	names = append(names, "")

	// Merge received configs.
	for _, name := range names {
		item := config.Config.ConfigMap[name]
		k2 := koanf.New(".")
		err := k2.Load(rawbytes.Provider(item.Body), yaml.Parser())
		if err != nil {
			return false, fmt.Errorf("cannot parse config named %s: %v", name, err)
		}
		err = k.Merge(k2)
		if err != nil {
			return false, fmt.Errorf("cannot merge config named %s: %v", name, err)
		}
	}

	// Merge own metrics config.
	ownMetricsCfg, ok := s.agentConfigOwnMetricsSection.Load().(string)
	if ok {
		if err := k.Load(rawbytes.Provider([]byte(ownMetricsCfg)), yaml.Parser()); err != nil {
			return false, err
		}
	}

	// Merge local config last since it has the highest precedence.
	if err := k.Load(rawbytes.Provider([]byte(s.composeExtraLocalConfig())), yaml.Parser()); err != nil {
		return false, err
	}

	// The merged final result is our effective config.
	effectiveConfigBytes, err := k.Marshal(yaml.Parser())
	if err != nil {
		return false, err
	}

	// Check if effective config is changed.
	newEffectiveConfig := string(effectiveConfigBytes)
	configChanged = false
	if s.effectiveConfig.Load().(string) != newEffectiveConfig {
		s.logger.Debugf(ctx, "Effective config changed.")
		s.effectiveConfig.Store(newEffectiveConfig)
		configChanged = true
	}

	return configChanged, nil
}

// Recalculate the Agent's effective config and if the config changes signal to the
// background goroutine that the config needs to be applied to the Agent.
func (s *Supervisor) recalcEffectiveConfig(ctx context.Context) (configChanged bool, err error) {
	configChanged, err = s.composeEffectiveConfig(ctx, s.remoteConfig)
	if err != nil {
		s.logger.Errorf(ctx, "Error composing effective config. Ignoring received config: %v", err)
		return configChanged, err
	}

	return configChanged, nil
}

func (s *Supervisor) startAgent() {
	err := s.commander.Start(context.Background())
	if err != nil {
		errMsg := fmt.Sprintf("Cannot start the agent: %v", err)
		s.logger.Errorf(context.Background(), errMsg)
		s.opampClient.SetHealth(&protobufs.ComponentHealth{Healthy: false, LastError: errMsg})
		return
	}
	s.startedAt = time.Now()

	// Prepare health checker
	healthCheckBackoff := backoff.NewExponentialBackOff()
	healthCheckBackoff.MaxInterval = 60 * time.Second
	healthCheckBackoff.MaxElapsedTime = 0 // Never stop
	if s.healthCheckTicker != nil {
		s.healthCheckTicker.Stop()
	}
	s.healthCheckTicker = backoff.NewTicker(healthCheckBackoff)

	// TODO: choose the port dynamically.
	healthEndpoint := "http://localhost:13133"
	s.healthChecker = healthchecker.NewHttpHealthChecker(healthEndpoint)
}

func (s *Supervisor) healthCheck() {
	if !s.commander.IsRunning() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)

	err := s.healthChecker.Check(ctx)
	cancel()

	if errors.Is(err, s.lastHealthCheckErr) {
		// No difference from last check. Nothing new to report.
		return
	}

	// Prepare OpAMP health report.
	health := &protobufs.ComponentHealth{
		StartTimeUnixNano: uint64(s.startedAt.UnixNano()),
	}

	if err != nil {
		health.Healthy = false
		health.LastError = err.Error()
		s.logger.Errorf(ctx, "Agent is not healthy: %s", health.LastError)
	} else {
		health.Healthy = true
		s.logger.Debugf(ctx, "Agent is healthy.")
	}

	// Report via OpAMP.
	if err2 := s.opampClient.SetHealth(health); err2 != nil {
		s.logger.Errorf(ctx, "Could not report health. SetHealth returned: %v", err2)
		return
	}

	s.lastHealthCheckErr = err
}

func (s *Supervisor) runAgentProcess() {
	if _, err := os.Stat(s.effectiveConfigFilePath); err == nil {
		// We have an effective config file saved previously. Use it to start the agent.
		s.startAgent()
	}

	restartTimer := time.NewTimer(0)
	restartTimer.Stop()

	for {
		var healthCheckTickerCh <-chan time.Time
		if s.healthCheckTicker != nil {
			healthCheckTickerCh = s.healthCheckTicker.C
		}

		select {
		case <-s.hasNewConfig:
			restartTimer.Stop()
			s.stopAgentApplyConfig()
			s.startAgent()

		case <-s.commander.Done():
			errMsg := fmt.Sprintf(
				"Agent process PID=%d exited unexpectedly, exit code=%d. Will restart in a bit...",
				s.commander.Pid(), s.commander.ExitCode(),
			)
			s.logger.Debugf(context.Background(), errMsg)
			s.opampClient.SetHealth(&protobufs.ComponentHealth{Healthy: false, LastError: errMsg})

			// TODO: decide why the agent stopped. If it was due to bad config, report it to server.

			// Wait 5 seconds before starting again.
			restartTimer.Stop()
			restartTimer.Reset(5 * time.Second)

		case <-restartTimer.C:
			s.startAgent()

		case <-healthCheckTickerCh:
			s.healthCheck()
		}
	}
}

func (s *Supervisor) stopAgentApplyConfig() {
	s.logger.Debugf(context.Background(), "Stopping the agent to apply new config.")
	cfg := s.effectiveConfig.Load().(string)
	s.commander.Stop(context.Background())
	s.writeEffectiveConfigToFile(cfg, s.effectiveConfigFilePath)
}

func (s *Supervisor) writeEffectiveConfigToFile(cfg string, filePath string) {
	f, err := os.Create(filePath)
	if err != nil {
		s.logger.Errorf(context.Background(), "Cannot write effective config file: %v", err)
	}
	defer f.Close()

	f.WriteString(cfg)
}

func (s *Supervisor) Shutdown() {
	s.logger.Debugf(context.Background(), "Supervisor shutting down...")
	if s.commander != nil {
		s.commander.Stop(context.Background())
	}
	if s.opampClient != nil {
		s.opampClient.SetHealth(
			&protobufs.ComponentHealth{
				Healthy: false, LastError: "Supervisor is shutdown",
			},
		)
		_ = s.opampClient.Stop(context.Background())
	}
}

func (s *Supervisor) onMessage(ctx context.Context, msg *types.MessageData) {
	configChanged := false
	if msg.RemoteConfig != nil {
		s.remoteConfig = msg.RemoteConfig
		s.logger.Debugf(ctx, "Received remote config from server, hash=%x.", s.remoteConfig.ConfigHash)

		var err error
		configChanged, err = s.recalcEffectiveConfig(ctx)
		if err != nil {
			s.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
				LastRemoteConfigHash: msg.RemoteConfig.ConfigHash,
				Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED,
				ErrorMessage:         err.Error(),
			})
		} else {
			s.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
				LastRemoteConfigHash: msg.RemoteConfig.ConfigHash,
				Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
			})
		}
	}

	if msg.OwnMetricsConnSettings != nil {
		configChanged = s.setupOwnMetrics(ctx, msg.OwnMetricsConnSettings) || configChanged
	}

	if msg.AgentIdentification != nil {
		newInstanceId, err := uuid.FromBytes(msg.AgentIdentification.NewInstanceUid)
		if err != nil {
			s.logger.Errorf(ctx, "invalid NewInstanceUid: %v", err)
			return
		}

		s.logger.Debugf(ctx, "Agent identify is being changed from id=%v to id=%v",
			s.instanceId.String(),
			newInstanceId.String())
		s.instanceId = newInstanceId

		// TODO: update metrics pipeline by altering configuration and setting
		// the instance id when Collector implements https://github.com/open-telemetry/opentelemetry-collector/pull/5402.
	}

	if configChanged {
		err := s.opampClient.UpdateEffectiveConfig(ctx)
		if err != nil {
			s.logger.Errorf(ctx, err.Error())
		}

		s.logger.Debugf(ctx, "Config is changed. Signal to restart the agent.")
		// Signal that there is a new config.
		select {
		case s.hasNewConfig <- struct{}{}:
		default:
		}
	}
}
