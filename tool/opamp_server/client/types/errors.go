package types

import "errors"

var (
	// ErrCustomMessageMissing is returned by SendCustomMessage when called with a nil message.
	ErrCustomMessageMissing = errors.New("CustomMessage is nil")

	// ErrCustomCapabilityNotSupported is returned by SendCustomMessage when called with
	// message that has a capability that is not specified as supported by the client.
	ErrCustomCapabilityNotSupported = errors.New("CustomCapability of CustomMessage is not supported")

	// ErrCustomMessagePending is returned by SendCustomMessage when called before the previous
	// message has been sent.
	ErrCustomMessagePending = errors.New("custom message already set")

	// ErrReportsAvailableComponentsNotSet is returned by SetAvailableComponents without the ReportsAvailableComponents capability set
	ErrReportsAvailableComponentsNotSet = errors.New("ReportsAvailableComponents capability is not set")

	// ErrAvailableComponentsMissing is returned by SetAvailableComponents when called with a nil message
	ErrAvailableComponentsMissing = errors.New("AvailableComponents is nil")

	// ErrNoAvailableComponentHash is returned by SetAvailableComponents when called with a message with an empty hash
	ErrNoAvailableComponentHash = errors.New("AvailableComponents.Hash is empty")
)
