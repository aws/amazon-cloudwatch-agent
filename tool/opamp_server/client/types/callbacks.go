package types

import (
	"context"
	"net/http"

	"github.com/open-telemetry/opamp-go/client/internal/utils"
	"github.com/open-telemetry/opamp-go/protobufs"
)

// MessageData represents a message received from the server and handled by Callbacks.
type MessageData struct {
	// RemoteConfig is offered by the Server. The Agent must process it and call
	// OpAMPClient.SetRemoteConfigStatus to indicate success or failure. If the
	// effective config has changed as a result of processing the Agent must also call
	// OpAMPClient.UpdateEffectiveConfig. SetRemoteConfigStatus and UpdateEffectiveConfig
	// may be called from OnMessage handler or after OnMessage returns.
	RemoteConfig *protobufs.AgentRemoteConfig

	// Connection settings are offered by the Server. These fields should be processed
	// as described in the ConnectionSettingsOffers message.
	OwnMetricsConnSettings *protobufs.TelemetryConnectionSettings
	OwnTracesConnSettings  *protobufs.TelemetryConnectionSettings
	OwnLogsConnSettings    *protobufs.TelemetryConnectionSettings
	OtherConnSettings      map[string]*protobufs.OtherConnectionSettings

	// PackagesAvailable offered by the Server. The Agent must process the offer.
	// The typical way to process is to call PackageSyncer.Sync() function, which will
	// take care of reporting the status to the Server as processing happens.
	//
	// If PackageSyncer.Sync() function is not called then it is the responsibility of
	// OnMessage handler to do the processing and call OpAMPClient.SetPackageStatuses to
	// reflect the processing status. SetPackageStatuses may be called from OnMessage
	// handler or after OnMessage returns.
	PackagesAvailable *protobufs.PackagesAvailable
	PackageSyncer     PackagesSyncer

	// AgentIdentification indicates a new identification received from the Server.
	// The Agent must save this identification and use it in the future instantiations
	// of OpAMPClient.
	AgentIdentification *protobufs.AgentIdentification

	// CustomCapabilities contains a list of custom capabilities that are supported by the
	// server.
	CustomCapabilities *protobufs.CustomCapabilities

	// CustomMessage contains a custom message sent by the server.
	CustomMessage *protobufs.CustomMessage
}

// Callbacks contains functions that are executed when the client encounters
// particular events.
//
// In most cases, defaults will be set when library users
// opt not to provide one. See SetDefaults for more information.
//
// Callbacks are expected to honour the context passed to them, meaning they
// should be aware of cancellations.
type Callbacks struct {
	// OnConnect is called when the connection is successfully established to the Server.
	// May be called after Start() is called and every time a connection is established to the Server.
	// For WebSocket clients this is called after the handshake is completed without any error.
	// For HTTP clients this is called for any request if the response status is OK.
	OnConnect func(ctx context.Context)

	// OnConnectFailed is called when the connection to the Server cannot be established.
	// May be called after Start() is called and tries to connect to the Server.
	// May also be called if the connection is lost and reconnection attempt fails.
	OnConnectFailed func(ctx context.Context, err error)

	// OnError is called when the Server reports an error in response to some previously
	// sent request. Useful for logging purposes. The Agent should not attempt to process
	// the error by reconnecting or retrying previous operations. The client handles the
	// ErrorResponse_UNAVAILABLE case internally by performing retries as necessary.
	OnError func(ctx context.Context, err *protobufs.ServerErrorResponse)

	// OnMessage is called when the Agent receives a message that needs processing.
	// See MessageData definition for the data that may be available for processing.
	// During OnMessage execution the OpAMPClient functions that change the status
	// of the client may be called, e.g. if RemoteConfig is processed then
	// SetRemoteConfigStatus should be called to reflect the processing result.
	// These functions may also be called after OnMessage returns. This is advisable
	// if processing can take a long time. In that case returning quickly is preferable
	// to avoid blocking the OpAMPClient.
	OnMessage func(ctx context.Context, msg *MessageData)

	// OnOpampConnectionSettings is called when the Agent receives an OpAMP
	// connection settings offer from the Server. Typically, the settings can specify
	// authorization headers or TLS certificate, potentially also a different
	// OpAMP destination to work with.
	//
	// The Agent should process the offer by reconnecting the client using the new
	// settings or return an error if the Agent does not want to accept the settings
	// (e.g. if the TSL certificate in the settings cannot be verified).
	//
	// Only one OnOpampConnectionSettings call can be active at any time.
	// See OnRemoteConfig for the behavior.
	OnOpampConnectionSettings func(
		ctx context.Context,
		settings *protobufs.OpAMPConnectionSettings,
	) error

	// For all methods that accept a context parameter the caller may cancel the
	// context if processing takes too long. In that case the method should return
	// as soon as possible with an error.

	// SaveRemoteConfigStatus is called after OnRemoteConfig returns. The status
	// will be set either as APPLIED or FAILED depending on whether OnRemoteConfig
	// returned a success or error.
	// The Agent must remember this RemoteConfigStatus and supply in the future
	// calls to Start() in StartSettings.RemoteConfigStatus.
	SaveRemoteConfigStatus func(ctx context.Context, status *protobufs.RemoteConfigStatus)

	// GetEffectiveConfig returns the current effective config. Only one
	// GetEffectiveConfig call can be active at any time. Until GetEffectiveConfig
	// returns it will not be called again.
	GetEffectiveConfig func(ctx context.Context) (*protobufs.EffectiveConfig, error)

	// OnCommand is called when the Server requests that the connected Agent perform a command.
	OnCommand func(ctx context.Context, command *protobufs.ServerToAgentCommand) error

	// CheckRedirect is called before following a redirect, allowing the client
	// the opportunity to observe the redirect chain, and optionally terminate
	// following redirects early.
	//
	// CheckRedirect is intended to be similar, although not exactly equivalent,
	// to net/http.Client's CheckRedirect feature. Unlike in net/http, the via
	// parameter is a slice of HTTP responses, instead of requests. This gives
	// an opportunity to users to know what the exact response headers and
	// status were. The request itself can be obtained from the response.
	//
	// The responses in the via parameter are passed with their bodies closed.
	CheckRedirect func(req *http.Request, viaReq []*http.Request, via []*http.Response) error

	// DownloadHTTPClient is called to create an HTTP client that is used to download files by the package syncer.
	// If the callback is not set, a default HTTP client will be created with the default transport settings.
	// The callback must return a non-nil HTTP client or an error.
	DownloadHTTPClient func(ctx context.Context, file *protobufs.DownloadableFile) (*http.Client, error)
}

func (c *Callbacks) SetDefaults() {
	if c.OnConnect == nil {
		c.OnConnect = func(ctx context.Context) {}
	}
	if c.OnConnectFailed == nil {
		c.OnConnectFailed = func(ctx context.Context, err error) {}
	}
	if c.OnError == nil {
		c.OnError = func(ctx context.Context, err *protobufs.ServerErrorResponse) {}
	}
	if c.OnMessage == nil {
		c.OnMessage = func(ctx context.Context, msg *MessageData) {}
	}
	if c.OnOpampConnectionSettings == nil {
		c.OnOpampConnectionSettings = func(ctx context.Context, settings *protobufs.OpAMPConnectionSettings) error { return nil }
	}
	if c.OnCommand == nil {
		c.OnCommand = func(ctx context.Context, command *protobufs.ServerToAgentCommand) error { return nil }
	}
	if c.GetEffectiveConfig == nil {
		c.GetEffectiveConfig = func(ctx context.Context) (*protobufs.EffectiveConfig, error) { return nil, nil }
	}
	if c.SaveRemoteConfigStatus == nil {
		c.SaveRemoteConfigStatus = func(ctx context.Context, status *protobufs.RemoteConfigStatus) {}
	}
	if c.DownloadHTTPClient == nil {
		defaultHttpClient := utils.NewHttpClient()
		c.DownloadHTTPClient = func(ctx context.Context, file *protobufs.DownloadableFile) (*http.Client, error) {
			return defaultHttpClient, nil
		}
	}
}
