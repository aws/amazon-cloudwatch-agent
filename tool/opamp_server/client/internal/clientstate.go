package internal

import (
	"errors"
	"sync"

	"github.com/open-telemetry/opamp-go/protobufs"
	"google.golang.org/protobuf/proto"
)

var (
	errRemoteConfigStatusMissing        = errors.New("RemoteConfigStatus is not set")
	errLastRemoteConfigHashNil          = errors.New("LastRemoteConfigHash is nil")
	errPackageStatusesMissing           = errors.New("PackageStatuses is not set")
	errServerProvidedAllPackagesHashNil = errors.New("ServerProvidedAllPackagesHash is nil")
	errCustomCapabilitiesMissing        = errors.New("CustomCapabilities is not set")
	errAvailableComponentsMissing       = errors.New("AvailableComponents is not set")
)

// ClientSyncedState stores the state of the Agent messages that the OpAMP Client needs to
// have access to synchronize to the Server. Seven messages can be stored in this store:
// AgentDescription, ComponentHealth, RemoteConfigStatus, PackageStatuses, CustomCapabilities, AvailableComponents and Flags.
//
// See OpAMP spec for more details on how status reporting works:
// https://github.com/open-telemetry/opamp-spec/blob/main/specification.md#status-reporting
//
// Note that the EffectiveConfig is subject to the same synchronization logic, however
// it is not stored in this struct since it can be large, and we do not want to always
// keep it in memory. To avoid storing it in memory the EffectiveConfig is supposed to be
// stored by the Agent implementation (e.g. it can be stored on disk) and is fetched
// via GetEffectiveConfig callback when it is needed by OpAMP client and then it is
// discarded from memory. See implementation of UpdateEffectiveConfig().
//
// It is safe to call methods of this struct concurrently.
type ClientSyncedState struct {
	mutex sync.Mutex

	agentDescription    *protobufs.AgentDescription
	health              *protobufs.ComponentHealth
	remoteConfigStatus  *protobufs.RemoteConfigStatus
	packageStatuses     *protobufs.PackageStatuses
	customCapabilities  *protobufs.CustomCapabilities
	availableComponents *protobufs.AvailableComponents
	flags               protobufs.AgentToServerFlags
	agentCapabilities   protobufs.AgentCapabilities
}

func (s *ClientSyncedState) AgentDescription() *protobufs.AgentDescription {
	defer s.mutex.Unlock()
	s.mutex.Lock()
	return s.agentDescription
}

func (s *ClientSyncedState) Health() *protobufs.ComponentHealth {
	defer s.mutex.Unlock()
	s.mutex.Lock()
	return s.health
}

func (s *ClientSyncedState) RemoteConfigStatus() *protobufs.RemoteConfigStatus {
	defer s.mutex.Unlock()
	s.mutex.Lock()
	return s.remoteConfigStatus
}

func (s *ClientSyncedState) PackageStatuses() *protobufs.PackageStatuses {
	defer s.mutex.Unlock()
	s.mutex.Lock()
	return s.packageStatuses
}

func (s *ClientSyncedState) CustomCapabilities() *protobufs.CustomCapabilities {
	defer s.mutex.Unlock()
	s.mutex.Lock()
	return s.customCapabilities
}

func (s *ClientSyncedState) AvailableComponents() *protobufs.AvailableComponents {
	defer s.mutex.Unlock()
	s.mutex.Lock()
	return s.availableComponents
}

func (s *ClientSyncedState) Flags() uint64 {
	defer s.mutex.Unlock()
	s.mutex.Lock()
	return uint64(s.flags)
}

func (s *ClientSyncedState) Capabilities() protobufs.AgentCapabilities {
	defer s.mutex.Unlock()
	s.mutex.Lock()
	return s.agentCapabilities
}

// SetAgentDescription sets the AgentDescription in the state.
func (s *ClientSyncedState) SetAgentDescription(descr *protobufs.AgentDescription) error {
	if descr == nil {
		return ErrAgentDescriptionMissing
	}

	if descr.IdentifyingAttributes == nil && descr.NonIdentifyingAttributes == nil {
		return ErrAgentDescriptionNoAttributes
	}

	clone := proto.Clone(descr).(*protobufs.AgentDescription)

	defer s.mutex.Unlock()
	s.mutex.Lock()
	s.agentDescription = clone

	return nil
}

// SetHealth sets the agent health in the state.
func (s *ClientSyncedState) SetHealth(health *protobufs.ComponentHealth) error {
	if health == nil {
		return ErrHealthMissing
	}

	clone := proto.Clone(health).(*protobufs.ComponentHealth)

	defer s.mutex.Unlock()
	s.mutex.Lock()
	s.health = clone

	return nil
}

// SetRemoteConfigStatus sets the RemoteConfigStatus in the state.
func (s *ClientSyncedState) SetRemoteConfigStatus(status *protobufs.RemoteConfigStatus) error {
	if status == nil {
		return errRemoteConfigStatusMissing
	}

	clone := proto.Clone(status).(*protobufs.RemoteConfigStatus)

	defer s.mutex.Unlock()
	s.mutex.Lock()
	s.remoteConfigStatus = clone

	return nil
}

// SetPackageStatuses sets the PackageStatuses in the state.
func (s *ClientSyncedState) SetPackageStatuses(status *protobufs.PackageStatuses) error {
	if status == nil {
		return errPackageStatusesMissing
	}

	clone := proto.Clone(status).(*protobufs.PackageStatuses)

	defer s.mutex.Unlock()
	s.mutex.Lock()
	s.packageStatuses = clone

	return nil
}

// SetCustomCapabilities sets the CustomCapabilities in the state.
func (s *ClientSyncedState) SetCustomCapabilities(capabilities *protobufs.CustomCapabilities) error {
	if capabilities == nil {
		return errCustomCapabilitiesMissing
	}

	clone := proto.Clone(capabilities).(*protobufs.CustomCapabilities)

	defer s.mutex.Unlock()
	s.mutex.Lock()
	s.customCapabilities = clone

	return nil
}

// HasCustomCapability returns true if the provided capability is in the
// CustomCapabilities.
func (s *ClientSyncedState) HasCustomCapability(capability string) bool {
	defer s.mutex.Unlock()
	s.mutex.Lock()

	if s.customCapabilities == nil {
		return false
	}

	for _, c := range s.customCapabilities.Capabilities {
		if c == capability {
			return true
		}
	}

	return false
}

func (s *ClientSyncedState) SetAvailableComponents(components *protobufs.AvailableComponents) error {
	if components == nil {
		return errAvailableComponentsMissing
	}

	clone := proto.Clone(components).(*protobufs.AvailableComponents)

	defer s.mutex.Unlock()
	s.mutex.Lock()
	s.availableComponents = clone

	return nil
}

// SetFlags sets the flags in the state.
func (s *ClientSyncedState) SetFlags(flags protobufs.AgentToServerFlags) {
	defer s.mutex.Unlock()
	s.mutex.Lock()

	s.flags = flags
}

// SetCapabilities sets the Capabilities in the state.
func (s *ClientSyncedState) SetCapabilities(capabilities *protobufs.AgentCapabilities) error {
	if capabilities == nil {
		return ErrCapabilitiesNotSet
	}

	defer s.mutex.Unlock()
	s.mutex.Lock()
	s.agentCapabilities = *capabilities

	return nil
}
