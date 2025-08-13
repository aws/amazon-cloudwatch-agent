package internal

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
)

var (
	ErrAgentDescriptionMissing      = errors.New("AgentDescription is nil")
	ErrAgentDescriptionNoAttributes = errors.New("AgentDescription has no attributes defined")
	ErrHealthMissing                = errors.New("health is nil")
	ErrReportsEffectiveConfigNotSet = errors.New("ReportsEffectiveConfig capability is not set")
	ErrReportsRemoteConfigNotSet    = errors.New("ReportsRemoteConfig capability is not set")
	ErrPackagesStateProviderNotSet  = errors.New("PackagesStateProvider must be set")
	ErrCapabilitiesNotSet           = errors.New("Capabilities is not set")
	ErrAcceptsPackagesNotSet        = errors.New("AcceptsPackages and ReportsPackageStatuses must be set")
	ErrAvailableComponentsMissing   = errors.New("AvailableComponents is nil")

	errAlreadyStarted               = errors.New("already started")
	errCannotStopNotStarted         = errors.New("cannot stop because not started")
	errReportsPackageStatusesNotSet = errors.New("ReportsPackageStatuses capability is not set")
)

// ClientCommon contains the OpAMP logic that is common between WebSocket and
// plain HTTP transports.
type ClientCommon struct {
	Logger    types.Logger
	Callbacks types.Callbacks

	// Client state storage. This is needed if the Server asks to report the state.
	ClientSyncedState ClientSyncedState

	// PackagesStateProvider provides access to the local state of packages.
	PackagesStateProvider types.PackagesStateProvider

	// PackageSyncMutex makes sure only one package syncing operation happens at a time.
	PackageSyncMutex sync.Mutex

	// The transport-specific sender.
	sender Sender

	// True if Start() is successful.
	isStarted bool

	// Cancellation func for background go routines.
	runCancel context.CancelFunc

	// True when stopping is in progress.
	isStoppingFlag  bool
	isStoppingMutex sync.RWMutex

	// Indicates that the Client is fully stopped.
	stoppedSignal chan struct{}

	// DownloadReporterInterval is the interval used to update a package's status while it is downloading.
	// It is set to 10s by default, a min value of 1s is forced.
	DownloadReporterInterval time.Duration
}

// NewClientCommon creates a new ClientCommon.
func NewClientCommon(logger types.Logger, sender Sender) ClientCommon {
	return ClientCommon{Logger: logger, sender: sender, stoppedSignal: make(chan struct{}, 1)}
}

func (c *ClientCommon) hasCapability(capability protobufs.AgentCapabilities) bool {
	return c.ClientSyncedState.Capabilities()&capability != 0
}

func (c *ClientCommon) validateCapabilities(capabilities protobufs.AgentCapabilities) error {
	if capabilities&protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth != 0 && c.ClientSyncedState.Health() == nil {
		return ErrHealthMissing
	}
	if capabilities&protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents != 0 && c.ClientSyncedState.AvailableComponents() == nil {
		return ErrAvailableComponentsMissing
	}
	if c.PackagesStateProvider != nil {
		if (capabilities&protobufs.AgentCapabilities_AgentCapabilities_AcceptsPackages == 0) ||
			(capabilities&protobufs.AgentCapabilities_AgentCapabilities_ReportsPackageStatuses == 0) {
			return ErrAcceptsPackagesNotSet
		}
	} else {
		if capabilities&protobufs.AgentCapabilities_AgentCapabilities_AcceptsPackages != 0 ||
			capabilities&protobufs.AgentCapabilities_AgentCapabilities_ReportsPackageStatuses != 0 {
			return ErrPackagesStateProviderNotSet
		}
	}
	return nil
}

// PrepareStart prepares the client state for the next Start() call.
// It returns an error if the client is already started, or if the settings are invalid.
func (c *ClientCommon) PrepareStart(
	ctx context.Context, settings types.StartSettings,
) error {
	if c.isStarted {
		return errAlreadyStarted
	}
	// Deprecated: Use client.SetCapabilities() instead.
	if settings.Capabilities != 0 {
		c.Logger.Errorf(ctx, "settings.Capabilities is deprecated, use client.SetCapabilities() instead")
		capabilities := settings.Capabilities
		// According to OpAMP spec this capability MUST be set, since all Agents MUST report status.
		capabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus
		if err := c.ClientSyncedState.SetCapabilities(&capabilities); err != nil {
			return err
		}
	}
	if c.ClientSyncedState.Capabilities() == 0 {
		c.Logger.Errorf(ctx, "you must call client.SetCapabilities before start.")
		capabilities := protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus
		if err := c.ClientSyncedState.SetCapabilities(&capabilities); err != nil {
			return err
		}
		// We allow this to succeed for now, but later this will become an error.
		// TODO: https://github.com/open-telemetry/opamp-go/issues/407
		// return ErrCapabilitiesNotSet
	}

	if c.ClientSyncedState.AgentDescription() == nil {
		return ErrAgentDescriptionMissing
	}

	// Prepare package statuses.
	c.PackagesStateProvider = settings.PackagesStateProvider
	if err := c.validateCapabilities(c.ClientSyncedState.Capabilities()); err != nil {
		return err
	}

	// Prepare remote config status.
	if settings.RemoteConfigStatus == nil {
		// RemoteConfigStatus is not provided. Start with empty.
		settings.RemoteConfigStatus = &protobufs.RemoteConfigStatus{
			Status: protobufs.RemoteConfigStatuses_RemoteConfigStatuses_UNSET,
		}
	}

	if err := c.ClientSyncedState.SetRemoteConfigStatus(settings.RemoteConfigStatus); err != nil {
		return err
	}

	var packageStatuses *protobufs.PackageStatuses
	if c.PackagesStateProvider != nil &&
		c.hasCapability(protobufs.AgentCapabilities_AgentCapabilities_AcceptsPackages) &&
		c.hasCapability(protobufs.AgentCapabilities_AgentCapabilities_ReportsPackageStatuses) {
		// Set package status from the value previously saved in the PackagesStateProvider.
		var err error
		packageStatuses, err = settings.PackagesStateProvider.LastReportedStatuses()
		if err != nil {
			return err
		}
	}

	if packageStatuses == nil {
		// PackageStatuses is not provided. Start with empty.
		packageStatuses = &protobufs.PackageStatuses{}
	}
	if err := c.ClientSyncedState.SetPackageStatuses(packageStatuses); err != nil {
		return err
	}

	// Prepare callbacks.
	c.Callbacks = settings.Callbacks
	c.Callbacks.SetDefaults()

	if c.hasCapability(protobufs.AgentCapabilities_AgentCapabilities_ReportsHeartbeat) && settings.HeartbeatInterval != nil {
		if err := c.sender.SetHeartbeatInterval(*settings.HeartbeatInterval); err != nil {
			return err
		}
	}

	if err := c.sender.SetInstanceUid(settings.InstanceUid); err != nil {
		return err
	}

	if settings.DownloadReporterInterval != nil && *settings.DownloadReporterInterval < time.Second {
		c.DownloadReporterInterval = time.Second
	} else if settings.DownloadReporterInterval != nil {
		c.DownloadReporterInterval = *settings.DownloadReporterInterval
	}

	return nil
}

// Stop stops the client. It returns an error if the client is not started.
func (c *ClientCommon) Stop(ctx context.Context) error {
	if !c.isStarted {
		return errCannotStopNotStarted
	}

	c.isStoppingMutex.Lock()
	cancelFunc := c.runCancel
	c.isStoppingFlag = true
	c.isStoppingMutex.Unlock()

	cancelFunc()

	// Wait until stopping is finished.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.stoppedSignal:
	}
	return nil
}

// IsStopping returns true if Stop() was called.
func (c *ClientCommon) IsStopping() bool {
	c.isStoppingMutex.RLock()
	defer c.isStoppingMutex.RUnlock()
	return c.isStoppingFlag
}

// StartConnectAndRun initiates the connection with the Server and starts the
// background goroutine that handles the communication unitl client is stopped.
func (c *ClientCommon) StartConnectAndRun(runner func(ctx context.Context)) {
	// Create a cancellable context.
	runCtx, runCancel := context.WithCancel(context.Background())

	c.isStoppingMutex.Lock()
	defer c.isStoppingMutex.Unlock()

	if c.isStoppingFlag {
		// Stop() was called. Don't connect.
		runCancel()
		return
	}
	c.runCancel = runCancel
	c.isStarted = true

	go func() {
		defer func() {
			// We only return from runner() when we are instructed to stop.
			// When returning signal that we stopped.
			c.stoppedSignal <- struct{}{}
		}()

		runner(runCtx)
	}()
}

// PrepareFirstMessage prepares the initial state of NextMessage struct that client
// sends when it first establishes a connection with the Server.
func (c *ClientCommon) PrepareFirstMessage(ctx context.Context) error {
	cfg, err := c.Callbacks.GetEffectiveConfig(ctx)
	if err != nil {
		return err
	}

	// initially, do not send the full component state - just send the hash.
	// full state is available on request from the server using the corresponding ServerToAgent flag
	var availableComponents *protobufs.AvailableComponents
	if c.hasCapability(protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents) {
		availableComponents = &protobufs.AvailableComponents{
			Hash: c.ClientSyncedState.AvailableComponents().GetHash(),
		}
	}

	c.sender.NextMessage().Update(
		func(msg *protobufs.AgentToServer) {
			msg.AgentDescription = c.ClientSyncedState.AgentDescription()
			msg.EffectiveConfig = cfg
			msg.RemoteConfigStatus = c.ClientSyncedState.RemoteConfigStatus()
			msg.PackageStatuses = c.ClientSyncedState.PackageStatuses()
			msg.Capabilities = uint64(c.ClientSyncedState.Capabilities())
			msg.CustomCapabilities = c.ClientSyncedState.CustomCapabilities()
			msg.Flags = c.ClientSyncedState.Flags()
			msg.AvailableComponents = availableComponents
		},
	)
	return nil
}

// AgentDescription returns the current state of the AgentDescription.
func (c *ClientCommon) AgentDescription() *protobufs.AgentDescription {
	// Return a cloned copy to allow caller to do whatever they want with the result.
	return proto.Clone(c.ClientSyncedState.AgentDescription()).(*protobufs.AgentDescription)
}

// SetAgentDescription sends a status update to the Server with the new AgentDescription
// and remembers the AgentDescription in the client state so that it can be sent
// to the Server when the Server asks for it.
func (c *ClientCommon) SetAgentDescription(descr *protobufs.AgentDescription) error {
	// store the Agent description to send on reconnect
	if err := c.ClientSyncedState.SetAgentDescription(descr); err != nil {
		return err
	}
	c.sender.NextMessage().Update(
		func(msg *protobufs.AgentToServer) {
			msg.AgentDescription = c.ClientSyncedState.AgentDescription()
		},
	)
	c.sender.ScheduleSend()
	return nil
}

func (c *ClientCommon) RequestConnectionSettings(request *protobufs.ConnectionSettingsRequest) error {
	c.sender.NextMessage().Update(
		func(msg *protobufs.AgentToServer) {
			msg.ConnectionSettingsRequest = request
		},
	)
	c.sender.ScheduleSend()
	return nil
}

// SetHealth sends a status update to the Server with the new agent health
// and remembers the health in the client state so that it can be sent
// to the Server when the Server asks for it.
func (c *ClientCommon) SetHealth(health *protobufs.ComponentHealth) error {
	// store the health to send on reconnect
	if err := c.ClientSyncedState.SetHealth(health); err != nil {
		return err
	}
	c.sender.NextMessage().Update(
		func(msg *protobufs.AgentToServer) {
			msg.Health = c.ClientSyncedState.Health()
		},
	)
	c.sender.ScheduleSend()
	return nil
}

// UpdateEffectiveConfig fetches the current local effective config using
// GetEffectiveConfig callback and sends it to the Server using provided Sender.
func (c *ClientCommon) UpdateEffectiveConfig(ctx context.Context) error {
	if !c.hasCapability(protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig) {
		return ErrReportsEffectiveConfigNotSet
	}

	// Fetch the locally stored config.
	cfg, err := c.Callbacks.GetEffectiveConfig(ctx)
	if err != nil {
		return fmt.Errorf("GetEffectiveConfig failed: %w", err)
	}

	// Send it to the Server.
	c.sender.NextMessage().Update(
		func(msg *protobufs.AgentToServer) {
			msg.EffectiveConfig = cfg
		},
	)
	// TODO: if this call is coming from OnMessage callback don't schedule the send
	// immediately, wait until the end of OnMessage to send one message only.
	c.sender.ScheduleSend()

	// Note that we do not store the EffectiveConfig anywhere else. It will be deleted
	// from NextMessage when the message is sent. This avoids storing EffectiveConfig
	// in memory for longer than it is needed.
	return nil
}

// SetRemoteConfigStatus sends a status update to the Server if the new RemoteConfigStatus
// is different from the status we already have in the state.
// It also remembers the new RemoteConfigStatus in the client state so that it can be
// sent to the Server when the Server asks for it.
func (c *ClientCommon) SetRemoteConfigStatus(status *protobufs.RemoteConfigStatus) error {
	if !c.hasCapability(protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig) {
		return ErrReportsRemoteConfigNotSet
	}

	if status.LastRemoteConfigHash == nil {
		return errLastRemoteConfigHashNil
	}

	statusChanged := !proto.Equal(c.ClientSyncedState.RemoteConfigStatus(), status)

	// Remember the new status.
	if err := c.ClientSyncedState.SetRemoteConfigStatus(status); err != nil {
		return err
	}

	if statusChanged {
		// Let the Server know about the new status.
		c.sender.NextMessage().Update(
			func(msg *protobufs.AgentToServer) {
				msg.RemoteConfigStatus = c.ClientSyncedState.RemoteConfigStatus()
			},
		)
		// TODO: if this call is coming from OnMessage callback don't schedule the send
		// immediately, wait until the end of OnMessage to send one message only.
		c.sender.ScheduleSend()
	}

	return nil
}

// SetPackageStatuses sends a status update to the Server if the new PackageStatuses
// are different from the ones we already have in the state.
// It also remembers the new PackageStatuses in the client state so that it can be
// sent to the Server when the Server asks for it.
func (c *ClientCommon) SetPackageStatuses(statuses *protobufs.PackageStatuses) error {
	if !c.hasCapability(protobufs.AgentCapabilities_AgentCapabilities_ReportsPackageStatuses) {
		return errReportsPackageStatusesNotSet
	}

	if statuses.ServerProvidedAllPackagesHash == nil {
		return errServerProvidedAllPackagesHashNil
	}

	statusChanged := !proto.Equal(c.ClientSyncedState.PackageStatuses(), statuses)

	if err := c.ClientSyncedState.SetPackageStatuses(statuses); err != nil {
		return err
	}

	// Check if the new status is different from the previous.
	if statusChanged {
		// Let the Server know about the new status.

		c.sender.NextMessage().Update(
			func(msg *protobufs.AgentToServer) {
				msg.PackageStatuses = c.ClientSyncedState.PackageStatuses()
			},
		)
		// TODO: if this call is coming from OnMessage callback don't schedule the send
		// immediately, wait until the end of OnMessage to send one message only.
		c.sender.ScheduleSend()
	}

	return nil
}

// SetCustomCapabilities sends a message to the Server with the new custom capabilities.
func (c *ClientCommon) SetCustomCapabilities(customCapabilities *protobufs.CustomCapabilities) error {
	// store the customCapabilities to send
	if err := c.ClientSyncedState.SetCustomCapabilities(customCapabilities); err != nil {
		return err
	}
	// send the new customCapabilities to the Server
	c.sender.NextMessage().Update(
		func(msg *protobufs.AgentToServer) {
			msg.CustomCapabilities = c.ClientSyncedState.CustomCapabilities()
		},
	)
	c.sender.ScheduleSend()
	return nil
}

func (c *ClientCommon) SetFlags(flags protobufs.AgentToServerFlags) {
	// store the flags to send
	c.ClientSyncedState.SetFlags(flags)

	// send the new flags to the Server
	c.sender.NextMessage().Update(
		func(msg *protobufs.AgentToServer) {
			msg.Flags = uint64(flags)
		},
	)
	c.sender.ScheduleSend()
}

// SendCustomMessage sends the specified custom message to the server.
func (c *ClientCommon) SendCustomMessage(message *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error) {
	if message == nil {
		return nil, types.ErrCustomMessageMissing
	}
	if !c.ClientSyncedState.HasCustomCapability(message.Capability) {
		return nil, types.ErrCustomCapabilityNotSupported
	}

	hasCustomMessage := false
	sendingChan := c.sender.NextMessage().Update(
		func(msg *protobufs.AgentToServer) {
			if msg.CustomMessage != nil {
				hasCustomMessage = true
			} else {
				msg.CustomMessage = message
			}
		},
	)

	if hasCustomMessage {
		return sendingChan, types.ErrCustomMessagePending
	}

	c.sender.ScheduleSend()

	return sendingChan, nil
}

// SetAvailableComponents sends a message to the server with the available components for the agent
func (c *ClientCommon) SetAvailableComponents(components *protobufs.AvailableComponents) error {
	if !c.isStarted {
		return c.ClientSyncedState.SetAvailableComponents(components)
	}

	if c.ClientSyncedState.Capabilities()&protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents == 0 {
		return types.ErrReportsAvailableComponentsNotSet
	}

	if components == nil {
		return types.ErrAvailableComponentsMissing
	}

	if len(components.Hash) == 0 {
		return types.ErrNoAvailableComponentHash
	}

	// implement agent status compression, don't send the message if it hasn't changed from the previous message
	availableComponentsChanged := !proto.Equal(c.ClientSyncedState.AvailableComponents(), components)

	if availableComponentsChanged {
		if err := c.ClientSyncedState.SetAvailableComponents(components); err != nil {
			return err
		}

		// initially, do not send the full component state - just send the hash.
		// full state is available on request from the server using the corresponding ServerToAgent flag
		availableComponents := &protobufs.AvailableComponents{
			Hash: c.ClientSyncedState.AvailableComponents().GetHash(),
		}

		c.sender.NextMessage().Update(
			func(msg *protobufs.AgentToServer) {
				msg.AvailableComponents = availableComponents
			},
		)

		c.sender.ScheduleSend()
	}

	return nil
}

// SetCapabilities sends a message to the Server with the new capabilities.
func (c *ClientCommon) SetCapabilities(capabilities *protobufs.AgentCapabilities) error {
	if capabilities == nil {
		return nil
	}
	// According to OpAMP spec this capability MUST be set, since all Agents MUST report status.
	*capabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus
	if validateErr := c.validateCapabilities(*capabilities); validateErr != nil {
		return validateErr
	}
	// store the capabilities to send
	if err := c.ClientSyncedState.SetCapabilities(capabilities); err != nil {
		return err
	}
	// send the new customCapabilities to the Server
	c.sender.NextMessage().Update(
		func(msg *protobufs.AgentToServer) {
			msg.Capabilities = uint64(c.ClientSyncedState.Capabilities())
		},
	)
	c.sender.ScheduleSend()
	return nil
}
