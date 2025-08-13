package client

import (
	"context"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
)

// OpAMPClient is an interface representing the client side of the OpAMP protocol.
type OpAMPClient interface {
	// Start the client and begin attempts to connect to the Server. Once connection
	// is established the client will attempt to maintain it by reconnecting if
	// the connection is lost. All failed connection attempts will be reported via
	// OnConnectFailed callback.
	//
	// SetAgentDescription() MUST be called before Start().
	//
	// Start may immediately return an error if the settings are incorrect (e.g. the
	// serverURL is not a valid URL).
	//
	// Start does not wait until the connection to the Server is established and will
	// likely return before the connection attempts are even made.
	//
	// It is guaranteed that after the Start() call returns without error one of the
	// following callbacks will be called eventually (unless Stop() is called earlier):
	//  - OnConnectFailed
	//  - OnError
	//  - OnRemoteConfig
	//
	// Start should be called only once. It should not be called concurrently with
	// any other OpAMPClient methods.
	Start(ctx context.Context, settings types.StartSettings) error

	// Stop the client. May be called only after Start() returns successfully.
	// May be called only once.
	// After this call returns successfully it is guaranteed that no
	// callbacks will be called. Stop() will cancel context of any in-fly
	// callbacks, but will wait until such in-fly callbacks are returned before
	// Stop returns, so make sure the callbacks don't block infinitely and react
	// promptly to context cancellations.
	// Once stopped OpAMPClient cannot be started again.
	Stop(ctx context.Context) error

	// SetAgentDescription sets attributes of the Agent. The attributes will be included
	// in the next status report sent to the Server. MUST be called before Start().
	// May be also called after Start(), in which case the attributes will be included
	// in the next outgoing status report. This is typically used by Agents which allow
	// their AgentDescription to change dynamically while the OpAMPClient is started.
	// May be also called from OnMessage handler.
	//
	// nil values are not allowed and will return an error.
	SetAgentDescription(descr *protobufs.AgentDescription) error

	// AgentDescription returns the last value successfully set by SetAgentDescription().
	AgentDescription() *protobufs.AgentDescription

	// SetHealth sets the health status of the Agent. The health will be included
	// in the next status report sent to the Server. MAY be called before or after Start().
	// May be also called after Start().
	// May be also called from OnMessage handler.
	//
	// nil health parameter is not allowed and will return an error.
	SetHealth(health *protobufs.ComponentHealth) error

	// UpdateEffectiveConfig fetches the current local effective config using
	// GetEffectiveConfig callback and sends it to the Server.
	// May be called anytime after Start(), including from OnMessage handler.
	UpdateEffectiveConfig(ctx context.Context) error

	// SetRemoteConfigStatus sets the current RemoteConfigStatus.
	// LastRemoteConfigHash field must be non-nil.
	// May be called anytime after Start(), including from OnMessage handler.
	// nil values are not allowed and will return an error.
	SetRemoteConfigStatus(status *protobufs.RemoteConfigStatus) error

	// SetPackageStatuses sets the current PackageStatuses.
	// ServerProvidedAllPackagesHash must be non-nil.
	// May be called anytime after Start(), including from OnMessage handler.
	// nil values are not allowed and will return an error.
	SetPackageStatuses(statuses *protobufs.PackageStatuses) error

	// RequestConnectionSettings sets a ConnectionSettingsRequest. The ConnectionSettingsRequest
	// will be included in the next AgentToServer message sent to the Server.
	// Used for client-initiated connection setting acquisition flows.
	// It is the responsibility of the caller to ensure that the Server supports
	// AcceptsConnectionSettingsRequest capability.
	// May be called before or after Start().
	// May be also called from OnMessage handler.
	RequestConnectionSettings(request *protobufs.ConnectionSettingsRequest) error

	// SetCustomCapabilities modifies the set of customCapabilities supported by the client.
	// The new customCapabilities will be sent with the next message to the server. If
	// custom capabilities are used SHOULD be called before Start(). If not called before
	// Start(), the set of supported custom capabilities will be empty. May also be called
	// anytime after Start(), including from OnMessage handler, to modify the set of
	// supported custom capabilities. nil values are not allowed and will return an error.
	//
	// Each capability is a reverse FQDN with optional version information that uniquely
	// identifies the custom capability and should match a capability specified in a
	// supported CustomMessage. The client will automatically ignore any CustomMessage that
	// contains a custom capability that is not specified in this field.
	//
	// See
	// https://github.com/open-telemetry/opamp-spec/blob/main/specification.md#customcapabilities
	// for more details.
	SetCustomCapabilities(customCapabilities *protobufs.CustomCapabilities) error

	// SetFlags modifies the set of flags supported by the client.
	// May be called before or after Start(), including from OnMessage handler.
	// The zero value of protobufs.AgentToServerFlags corresponds to FlagsUnspecified
	// and is safe to use.
	//
	// See
	// https://github.com/open-telemetry/opamp-spec/blob/main/specification.md#agenttoserverflags
	// for more details.
	SetFlags(flags protobufs.AgentToServerFlags)

	// SendCustomMessage sends the custom message to the Server. May be called anytime after
	// Start(), including from OnMessage handler.
	//
	// If the CustomMessage is nil, ErrCustomMessageMissing will be returned. If the message
	// specifies a capability that is not listed in the CustomCapabilities provided in the
	// StartSettings for the client, ErrCustomCapabilityNotSupported will be returned.
	//
	// Only one message can be sent at a time. If SendCustomMessage has been already called
	// and the message is still pending (in progress) then subsequent calls to
	// SendCustomMessage will return ErrCustomMessagePending and a channel that will be
	// closed when the pending message is sent. To ensure that the previous send is complete
	// and it is safe to send another CustomMessage, the caller should wait for the returned
	// channel to be closed before attempting to send another custom message.
	//
	// If no error is returned, the channel returned will be closed after the specified
	// message is sent.
	SendCustomMessage(message *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error)

	// SetAvailableComponents modifies the set of components that are available for configuration
	// on the agent.
	// If called before Start(), initializes the client state that will be sent to the server upon
	// Start() if the ReportsAvailableComponents capability is set.
	// Must be called before Start() if the ReportsAvailableComponents capability is set.
	//
	// May be called any time after Start(), including from the OnMessage handler.
	// The new components will be sent with the next message to the server.
	//
	// When called after Start():
	// If components is nil, types.ErrAvailableComponentsMissing will be returned.
	// If components.Hash is nil or an empty []byte, types.ErrNoAvailableComponentHash will be returned.
	// If the ReportsAvailableComponents capability is not set in StartSettings.Capabilities during Start(),
	// types.ErrReportsAvailableComponentsNotSet will be returned.
	//
	// This method is subject to agent status compression - if components is not
	// different from the cached agent state, this method is a no-op.
	SetAvailableComponents(components *protobufs.AvailableComponents) error

	// SetCapabilities updates the set of capabilities that the client supports.
	// These capabilities will be communicated to the server in the next message.
	//
	// This method can be called at any time before or after Start(), including from within
	// an OnMessage handler, to dynamically update the set of supported capabilities.
	// The updated capabilities will be sent to the server in the next outgoing message.
	//
	// The capabilities parameter must not be nil; passing a nil value will result in an error.
	//
	// For more details, refer to the OpAMP specification:
	// https://github.com/open-telemetry/opamp-spec/blob/main/specification.md#agenttoservercapabilities
	SetCapabilities(capabilities *protobufs.AgentCapabilities) error
}
