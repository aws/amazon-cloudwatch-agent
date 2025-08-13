package client

import (
	"context"
	"time"

	"github.com/open-telemetry/opamp-go/client/internal"
	"github.com/open-telemetry/opamp-go/client/types"
	sharedinternal "github.com/open-telemetry/opamp-go/internal"
	"github.com/open-telemetry/opamp-go/protobufs"
)

var _ OpAMPClient = (*httpClient)(nil)

// httpClient is an OpAMP Client implementation for plain HTTP transport.
// See specification: https://github.com/open-telemetry/opamp-spec/blob/main/specification.md#plain-http-transport
type httpClient struct {
	common internal.ClientCommon

	opAMPServerURL string

	// The sender performs HTTP request/response loop.
	sender *internal.HTTPSender
}

// NewHTTP creates a new OpAMP Client that uses HTTP transport.
func NewHTTP(logger types.Logger) *httpClient {
	if logger == nil {
		logger = &sharedinternal.NopLogger{}
	}

	sender := internal.NewHTTPSender(logger)
	w := &httpClient{
		common: internal.NewClientCommon(logger, sender),
		sender: sender,
	}
	return w
}

// Start implements OpAMPClient.Start.
func (c *httpClient) Start(ctx context.Context, settings types.StartSettings) error {
	if err := c.common.PrepareStart(ctx, settings); err != nil {
		return err
	}

	c.opAMPServerURL = settings.OpAMPServerURL

	// Prepare Server connection settings.
	c.sender.SetRequestHeader(settings.Header, settings.HeaderFunc)

	// Add TLS configuration into httpClient
	c.sender.AddTLSConfig(settings.TLSConfig)

	if settings.EnableCompression {
		c.sender.EnableCompression()
	}

	// Prepare the first message to send.
	err := c.common.PrepareFirstMessage(ctx)
	if err != nil {
		return err
	}

	c.sender.ScheduleSend()

	c.common.StartConnectAndRun(c.runUntilStopped)

	return nil
}

// Stop implements OpAMPClient.Stop.
func (c *httpClient) Stop(ctx context.Context) error {
	return c.common.Stop(ctx)
}

// AgentDescription implements OpAMPClient.AgentDescription.
func (c *httpClient) AgentDescription() *protobufs.AgentDescription {
	return c.common.AgentDescription()
}

// SetAgentDescription implements OpAMPClient.SetAgentDescription.
func (c *httpClient) SetAgentDescription(descr *protobufs.AgentDescription) error {
	return c.common.SetAgentDescription(descr)
}

func (c *httpClient) RequestConnectionSettings(request *protobufs.ConnectionSettingsRequest) error {
	return c.common.RequestConnectionSettings(request)
}

// SetHealth implements OpAMPClient.SetHealth.
func (c *httpClient) SetHealth(health *protobufs.ComponentHealth) error {
	return c.common.SetHealth(health)
}

// UpdateEffectiveConfig implements OpAMPClient.UpdateEffectiveConfig.
func (c *httpClient) UpdateEffectiveConfig(ctx context.Context) error {
	return c.common.UpdateEffectiveConfig(ctx)
}

// SetRemoteConfigStatus implements OpAMPClient.SetRemoteConfigStatus.
func (c *httpClient) SetRemoteConfigStatus(status *protobufs.RemoteConfigStatus) error {
	return c.common.SetRemoteConfigStatus(status)
}

// SetPackageStatuses implements OpAMPClient.SetPackageStatuses.
func (c *httpClient) SetPackageStatuses(statuses *protobufs.PackageStatuses) error {
	return c.common.SetPackageStatuses(statuses)
}

// SendCustomCapabilities implements OpAMPClient.SetCustomCapabilities.
func (c *httpClient) SetCustomCapabilities(customCapabilities *protobufs.CustomCapabilities) error {
	return c.common.SetCustomCapabilities(customCapabilities)
}

// SetFlags implements OpAMPClient.SetFlags.
func (c *httpClient) SetFlags(flags protobufs.AgentToServerFlags) {
	c.common.SetFlags(flags)
}

// SendCustomMessage implements OpAMPClient.SendCustomMessage.
func (c *httpClient) SendCustomMessage(message *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error) {
	return c.common.SendCustomMessage(message)
}

// SetAvailableComponents implements OpAMPClient.SetAvailableComponents
func (c *httpClient) SetAvailableComponents(components *protobufs.AvailableComponents) error {
	return c.common.SetAvailableComponents(components)
}

// SetCapabilities implements OpAMPClient.SetCapabilities
func (c *httpClient) SetCapabilities(capabilities *protobufs.AgentCapabilities) error {
	return c.common.SetCapabilities(capabilities)
}

func (c *httpClient) runUntilStopped(ctx context.Context) {
	// Start the HTTP sender. This will make request/responses with retries for
	// failures and will wait with configured polling interval if there is nothing
	// to send.
	c.sender.Run(
		ctx,
		c.opAMPServerURL,
		c.common.Callbacks,
		&c.common.ClientSyncedState,
		c.common.PackagesStateProvider,
		&c.common.PackageSyncMutex,
		c.common.DownloadReporterInterval,
	)
}

func (c *httpClient) SetPollingInterval(duration time.Duration) {
	c.sender.SetPollingInterval(duration)
}
