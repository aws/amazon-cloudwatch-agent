package agent

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"

	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/internal"
	"github.com/open-telemetry/opamp-go/protobufs"
)

const localConfig = `
exporters:
  otlp:
    endpoint: localhost:1111

receivers:
  otlp:
    protocols:
      grpc: {}
      http: {}

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: []
      exporters: [otlp]
`

type Agent struct {
	logger types.Logger

	agentType    string
	agentVersion string

	effectiveConfig string

	instanceId uuid.UUID

	agentDescription *protobufs.AgentDescription

	opampClient client.OpAMPClient

	remoteConfigStatus *protobufs.RemoteConfigStatus

	metricReporter *MetricReporter

	// The TLS certificate used for the OpAMP connection. Can be nil, meaning no client-side
	// certificate is used.
	opampClientCert *tls.Certificate

	tlsConfig *tls.Config

	certRequested       bool
	clientPrivateKeyPEM []byte
}

func NewAgent(logger types.Logger, agentType string, agentVersion string) *Agent {
	agent := &Agent{
		effectiveConfig: localConfig,
		logger:          logger,
		agentType:       agentType,
		agentVersion:    agentVersion,
	}

	agent.createAgentIdentity()
	agent.logger.Debugf(context.Background(), "Agent starting, id=%v, type=%s, version=%s.",
		agent.instanceId, agentType, agentVersion)

	agent.loadLocalConfig()
	tlsConfig, err := internal.CreateClientTLSConfig(
		agent.opampClientCert,
		"../../certs/certs/ca.cert.pem",
	)
	if err != nil {
		agent.logger.Errorf(context.Background(), "Cannot load client TLS config: %v", err)
		return nil
	}
	agent.tlsConfig = tlsConfig
	if err := agent.connect(agent.tlsConfig); err != nil {
		agent.logger.Errorf(context.Background(), "Cannot connect OpAMP client: %v", err)
		return nil
	}

	return agent
}

func (agent *Agent) connect(tlsConfig *tls.Config) error {
	agent.opampClient = client.NewWebSocket(agent.logger)

	agent.tlsConfig = tlsConfig
	settings := types.StartSettings{
		OpAMPServerURL: "wss://127.0.0.1:4320/v1/opamp",
		TLSConfig:      agent.tlsConfig,
		InstanceUid:    types.InstanceUid(agent.instanceId),
		Callbacks: types.Callbacks{
			OnConnect: func(ctx context.Context) {
				agent.logger.Debugf(ctx, "Connected to the server.")
			},
			OnConnectFailed: func(ctx context.Context, err error) {
				agent.logger.Errorf(ctx, "Failed to connect to the server: %v", err)
			},
			OnError: func(ctx context.Context, err *protobufs.ServerErrorResponse) {
				agent.logger.Errorf(ctx, "Server returned an error response: %v", err.ErrorMessage)
			},
			SaveRemoteConfigStatus: func(_ context.Context, status *protobufs.RemoteConfigStatus) {
				agent.remoteConfigStatus = status
			},
			GetEffectiveConfig: func(ctx context.Context) (*protobufs.EffectiveConfig, error) {
				return agent.composeEffectiveConfig(), nil
			},
			OnMessage:                 agent.onMessage,
			OnOpampConnectionSettings: agent.onOpampConnectionSettings,
		},
		RemoteConfigStatus: agent.remoteConfigStatus,
		Capabilities: protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnMetrics |
			protobufs.AgentCapabilities_AgentCapabilities_AcceptsOpAMPConnectionSettings,
	}

	err := agent.opampClient.SetAgentDescription(agent.agentDescription)
	if err != nil {
		return err
	}

	// This sets the request to create a client certificate before the OpAMP client
	// is started, before the connection is established. However, this assumes the
	// server supports "AcceptsConnectionRequest" capability.
	// Alternatively the agent can perform this request after receiving the first
	// message from the server (in onMessage), i.e. after the server capabilities
	// become known and can be checked.
	agent.requestClientCertificate()

	agent.logger.Debugf(context.Background(), "Starting OpAMP client...")

	err = agent.opampClient.Start(context.Background(), settings)
	if err != nil {
		return err
	}

	agent.logger.Debugf(context.Background(), "OpAMP Client started.")

	return nil
}

func (agent *Agent) disconnect(ctx context.Context) {
	agent.logger.Debugf(ctx, "Disconnecting from server...")
	agent.opampClient.Stop(ctx)
}

func (agent *Agent) createAgentIdentity() {
	// Generate instance id.
	uid, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	agent.instanceId = uid

	hostname, _ := os.Hostname()

	// Create Agent description.
	agent.agentDescription = &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key: "service.name",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{StringValue: agent.agentType},
				},
			},
			{
				Key: "service.version",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{StringValue: agent.agentVersion},
				},
			},
		},
		NonIdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key: "os.type",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: runtime.GOOS,
					},
				},
			},
			{
				Key: "host.name",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: hostname,
					},
				},
			},
		},
	}
}

func (agent *Agent) updateAgentIdentity(ctx context.Context, instanceId uuid.UUID) {
	agent.logger.Debugf(ctx, "Agent identify is being changed from id=%v to id=%v",
		agent.instanceId,
		instanceId)
	agent.instanceId = instanceId

	if agent.metricReporter != nil {
		// TODO: reinit or update meter (possibly using a single function to update all own connection settings
		// or with having a common resource factory or so)
	}
}

func (agent *Agent) loadLocalConfig() {
	k := koanf.New(".")
	_ = k.Load(rawbytes.Provider([]byte(localConfig)), yaml.Parser())

	effectiveConfigBytes, err := k.Marshal(yaml.Parser())
	if err != nil {
		panic(err)
	}

	agent.effectiveConfig = string(effectiveConfigBytes)
}

func (agent *Agent) composeEffectiveConfig() *protobufs.EffectiveConfig {
	return &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"": {Body: []byte(agent.effectiveConfig)},
			},
		},
	}
}

func (agent *Agent) initMeter(settings *protobufs.TelemetryConnectionSettings) {
	reporter, err := NewMetricReporter(agent.logger, settings, agent.agentType, agent.agentVersion, agent.instanceId)
	if err != nil {
		agent.logger.Errorf(context.Background(), "Cannot collect metrics: %v", err)
		return
	}

	prevReporter := agent.metricReporter

	agent.metricReporter = reporter

	if prevReporter != nil {
		prevReporter.Shutdown()
	}

	return
}

type agentConfigFileItem struct {
	name string
	file *protobufs.AgentConfigFile
}

type agentConfigFileSlice []agentConfigFileItem

func (a agentConfigFileSlice) Less(i, j int) bool {
	return a[i].name < a[j].name
}

func (a agentConfigFileSlice) Swap(i, j int) {
	t := a[i]
	a[i] = a[j]
	a[j] = t
}

func (a agentConfigFileSlice) Len() int {
	return len(a)
}

func (agent *Agent) applyRemoteConfig(config *protobufs.AgentRemoteConfig) (configChanged bool, err error) {
	if config == nil {
		return false, nil
	}

	agent.logger.Debugf(context.Background(), "Received remote config from server, hash=%x.", config.ConfigHash)

	// Begin with local config. We will later merge received configs on top of it.
	k := koanf.New(".")
	if err := k.Load(rawbytes.Provider([]byte(localConfig)), yaml.Parser()); err != nil {
		return false, err
	}

	orderedConfigs := agentConfigFileSlice{}
	for name, file := range config.Config.ConfigMap {
		if name == "" {
			// skip instance config
			continue
		}
		orderedConfigs = append(orderedConfigs, agentConfigFileItem{
			name: name,
			file: file,
		})
	}

	// Sort to make sure the order of merging is stable.
	sort.Sort(orderedConfigs)

	// Append instance config as the last item.
	instanceConfig := config.Config.ConfigMap[""]
	if instanceConfig != nil {
		orderedConfigs = append(orderedConfigs, agentConfigFileItem{
			name: "",
			file: instanceConfig,
		})
	}

	// Merge received configs.
	for _, item := range orderedConfigs {
		k2 := koanf.New(".")
		err := k2.Load(rawbytes.Provider(item.file.Body), yaml.Parser())
		if err != nil {
			return false, fmt.Errorf("cannot parse config named %s: %v", item.name, err)
		}
		err = k.Merge(k2)
		if err != nil {
			return false, fmt.Errorf("cannot merge config named %s: %v", item.name, err)
		}
	}

	// The merged final result is our effective config.
	effectiveConfigBytes, err := k.Marshal(yaml.Parser())
	if err != nil {
		panic(err)
	}

	newEffectiveConfig := string(effectiveConfigBytes)
	configChanged = false
	if agent.effectiveConfig != newEffectiveConfig {
		agent.logger.Debugf(context.Background(), "Effective config changed. Need to report to server.")
		agent.effectiveConfig = newEffectiveConfig
		configChanged = true
	}

	return configChanged, nil
}

func (agent *Agent) Shutdown() {
	agent.logger.Debugf(context.Background(), "Agent shutting down...")
	if agent.opampClient != nil {
		_ = agent.opampClient.Stop(context.Background())
	}
}

// requestClientCertificate sets a request to be sent to the Server to create
// a client certificate that the Agent can use in subsequent OpAMP connections.
// This is the initiating step of the Client Signing Request (CSR) flow.
func (agent *Agent) requestClientCertificate() {
	if agent.certRequested {
		// Request only once, for bootstrapping.
		// TODO: the Agent may also for example check that the current certificate
		// is approaching expiration date and re-requests a new certificate.
		return
	}

	// Generate a keypair for new client cert.
	clientCertKeyPair, err := rsa.GenerateKey(cryptorand.Reader, 4096)
	if err != nil {
		agent.logger.Errorf(context.Background(), "Cannot generate keypair: %v", err)
		return
	}

	// Encode the private key of the keypair as DER.
	privateKeyDER := x509.MarshalPKCS1PrivateKey(clientCertKeyPair)

	// Convert private key from DER to PEM.
	privateKeyPEM := new(bytes.Buffer)
	pem.Encode(
		privateKeyPEM, &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: privateKeyDER,
		},
	)
	// Keep it. We will need it in later steps of the flow.
	agent.clientPrivateKeyPEM = privateKeyPEM.Bytes()

	// Create the CSR.
	template := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   "OpAMP Example Client",
			Organization: []string{"OpenTelemetry OpAMP Workgroup"},
			Locality:     []string{"Agent-initiated"},
			// Where do we put instance_uid?
		},
		SignatureAlgorithm: x509.SHA256WithRSA,
	}

	derBytes, err := x509.CreateCertificateRequest(cryptorand.Reader, &template, clientCertKeyPair)
	if err != nil {
		agent.logger.Errorf(context.Background(), "Failed to create certificate request: %s", err)
		return
	}

	// Convert CSR from DER to PEM format.
	csrPEM := new(bytes.Buffer)
	pem.Encode(
		csrPEM, &pem.Block{
			Type:  "CERTIFICATE REQUEST",
			Bytes: derBytes,
		},
	)

	// Send the request to the Server (immediately if already connected
	// or upon next successful connection).
	err = agent.opampClient.RequestConnectionSettings(
		&protobufs.ConnectionSettingsRequest{
			Opamp: &protobufs.OpAMPConnectionSettingsRequest{
				CertificateRequest: &protobufs.CertificateRequest{
					Csr: csrPEM.Bytes(),
				},
			},
		},
	)
	if err != nil {
		agent.logger.Errorf(context.Background(), "Failed to send CSR to server: %s", err)
		return
	}

	agent.certRequested = true
}

func (agent *Agent) onMessage(ctx context.Context, msg *types.MessageData) {
	configChanged := false
	if msg.RemoteConfig != nil {
		var err error
		configChanged, err = agent.applyRemoteConfig(msg.RemoteConfig)
		if err != nil {
			agent.opampClient.SetRemoteConfigStatus(
				&protobufs.RemoteConfigStatus{
					LastRemoteConfigHash: msg.RemoteConfig.ConfigHash,
					Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED,
					ErrorMessage:         err.Error(),
				},
			)
		} else {
			agent.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
				LastRemoteConfigHash: msg.RemoteConfig.ConfigHash,
				Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
			})
		}
	}

	if msg.OwnMetricsConnSettings != nil {
		agent.initMeter(msg.OwnMetricsConnSettings)
	}

	if msg.AgentIdentification != nil {
		uid, err := uuid.FromBytes(msg.AgentIdentification.NewInstanceUid)
		if err != nil {
			agent.logger.Errorf(ctx, "invalid NewInstanceUid: %v", err)
			return
		}
		agent.updateAgentIdentity(ctx, uid)
	}

	if configChanged {
		err := agent.opampClient.UpdateEffectiveConfig(ctx)
		if err != nil {
			agent.logger.Errorf(ctx, err.Error())
		}
	}

	// TODO: check that the Server has AcceptsConnectionSettingsRequest capability before
	// requesting a certificate.
	// This is actually a no-op since we already made the request when connecting
	// (see connect()). However we keep this call here to demonstrate that requesting it
	// in onMessage callback is also an option. This approach should be used if it is
	// necessary to check for AcceptsConnectionSettingsRequest (if the Agent is
	// not certain that the Server has this capability).
	agent.requestClientCertificate()
}

func (agent *Agent) tryChangeOpAMP(ctx context.Context, cert *tls.Certificate, tlsConfig *tls.Config) {
	agent.logger.Debugf(ctx, "Reconnecting to verify new OpAMP settings.\n")
	agent.disconnect(ctx)

	oldCfg := agent.tlsConfig
	if tlsConfig == nil {
		tlsConfig = oldCfg.Clone()
	}
	if cert != nil {
		agent.logger.Debugf(ctx, "Using new certificate\n")
		tlsConfig.Certificates = []tls.Certificate{*cert}
	}

	if err := agent.connect(tlsConfig); err != nil {
		agent.logger.Errorf(ctx, "Cannot connect after using new tls config: %s. Ignoring the offer\n", err)
		if err := agent.connect(oldCfg); err != nil {
			agent.logger.Errorf(ctx, "Unable to reconnect after restoring tls config: %s\n", err)
		}
		return
	}

	agent.logger.Debugf(ctx, "Successfully connected to server. Accepting new tls config.\n")
	// TODO: we can also persist the successfully accepted settigns and use it when the
	// agent connects to the server after the restart.
}

func (agent *Agent) onOpampConnectionSettings(ctx context.Context, settings *protobufs.OpAMPConnectionSettings) error {
	if settings == nil {
		agent.logger.Debugf(ctx, "Received nil settings, ignoring.\n")
		return nil
	}

	var cert *tls.Certificate
	var err error
	if settings.Certificate != nil {
		cert, err = agent.getCertFromSettings(settings.Certificate)
		if err != nil {
			return err
		}
	}

	var tlsConfig *tls.Config
	if settings.Tls != nil {
		tlsMin, err := getTLSVersionNumber(settings.Tls.MinVersion)
		if err != nil {
			return fmt.Errorf("unable to convert settings.tls.min_version: %w", err)
		}
		tlsMax, err := getTLSVersionNumber(settings.Tls.MaxVersion)
		if err != nil {
			return fmt.Errorf("unable to convert settings.tls.max_version: %w", err)
		}

		tlsConfig = &tls.Config{
			InsecureSkipVerify: settings.Tls.InsecureSkipVerify,
			MinVersion:         tlsMin,
			MaxVersion:         tlsMax,
			RootCAs:            x509.NewCertPool(),
			// TODO support cipher_suites values
		}

		if settings.Tls.IncludeSystemCaCertsPool {
			tlsConfig.RootCAs, err = x509.SystemCertPool()
			if err != nil {
				return fmt.Errorf("unable to use system cert pool: %w", err)
			}
		}

		if settings.Tls.CaPemContents != "" {
			ok := tlsConfig.RootCAs.AppendCertsFromPEM([]byte(settings.Tls.CaPemContents))
			if !ok {
				return fmt.Errorf("unable to add PEM CA")
			}
			agent.logger.Debugf(ctx, "CA in offered settings.\n")
		}
	}
	// TODO: also use settings.DestinationEndpoint and settings.Headers for future connections.
	go agent.tryChangeOpAMP(ctx, cert, tlsConfig)

	return nil
}

func getTLSVersionNumber(input string) (uint16, error) {
	switch strings.ToUpper(input) {
	case "1.0", "TLSV1", "TLSV1.0":
		return tls.VersionTLS10, nil
	case "1.1", "TLSV1.1":
		return tls.VersionTLS11, nil
	case "1.2", "TLSV1.2":
		return tls.VersionTLS12, nil
	case "1.3", "TLSV1.3":
		return tls.VersionTLS13, nil
	case "":
		// Do nothing if no value is set
		return 0, nil
	default:
		return 0, fmt.Errorf("unsupported value: %s", input)
	}
}

func (agent *Agent) getCertFromSettings(certificate *protobufs.TLSCertificate) (*tls.Certificate, error) {
	// Parse the key pair to a TLS certificate that can be used for network connections.

	// There are 2 types of certificate creation flows in OpAMP: client-initiated CSR
	// and server-initiated. In this example we demonstrate both flows.
	// Real-world Agent implementations will probably choose and use only one of these flows.

	var cert tls.Certificate
	var err error
	if certificate.PrivateKey == nil && agent.clientPrivateKeyPEM != nil {
		// Client-initiated CSR flow. This is currently initiated when connecting
		// to the Server for the first time (see requestClientCertificate()).
		cert, err = tls.X509KeyPair(
			certificate.Cert,          // We received the certificate from the Server.
			agent.clientPrivateKeyPEM, // Private key was earlier locally generated.
		)
	} else {
		// Server-initiated flow. This is currently initiated by user clicking a button in
		// the Server UI.
		// Both certificate and private key are from the Server.
		cert, err = tls.X509KeyPair(
			certificate.Cert,
			certificate.PrivateKey,
		)
	}

	if err != nil {
		agent.logger.Errorf(context.Background(), "Received invalid certificate offer: %s\n", err.Error())
		return nil, err
	}

	if len(certificate.CaCert) != 0 {
		caCertPB, _ := pem.Decode(certificate.CaCert)
		caCert, err := x509.ParseCertificate(caCertPB.Bytes)
		if err != nil {
			agent.logger.Errorf(context.Background(), "Cannot parse CA cert: %v", err)
			return nil, err
		}
		agent.logger.Debugf(context.Background(), "Received offer signed by CA: %v", caCert.Subject)
		// TODO: we can verify the CA's identity here (to match our CA as we know it).
	}

	return &cert, nil
}
