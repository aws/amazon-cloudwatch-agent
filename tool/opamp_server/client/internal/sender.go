package internal

import (
	"errors"
	"time"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
)

// Sender is an interface of the sending portion of OpAMP protocol that stores
// the NextMessage to be sent and can be ordered to send the message.
type Sender interface {
	// NextMessage gives access to the next message that will be sent by this Sender.
	// Can be called concurrently with any other method.
	NextMessage() *NextMessage

	// ScheduleSend signals to Sender that the message in NextMessage struct
	// is now ready to be sent.  The Sender should send the NextMessage as soon as possible.
	// If there is no pending message (e.g. the NextMessage was already sent and
	// "pending" flag is reset) then no message will be sent.
	ScheduleSend()

	// SetInstanceUid sets a new instanceUid to be used for all subsequent messages to be sent.
	SetInstanceUid(instanceUid types.InstanceUid) error

	// SetHeartbeatInterval sets the interval for the agent heartbeats.
	SetHeartbeatInterval(duration time.Duration) error
}

// SenderCommon is partial Sender implementation that is common between WebSocket and plain
// HTTP transports. This struct is intended to be embedded in the WebSocket and
// HTTP Sender implementations.
type SenderCommon struct {
	// Indicates that there is a pending message to send.
	hasPendingMessage chan struct{}

	// The next message to send.
	nextMessage NextMessage
}

// NewSenderCommon creates a new SenderCommon. This is intended to be used by
// the WebSocket and HTTP Sender implementations.
func NewSenderCommon() SenderCommon {
	return SenderCommon{
		hasPendingMessage: make(chan struct{}, 1),
		nextMessage:       NewNextMessage(),
	}
}

// ScheduleSend signals to HTTPSender that the message in NextMessage struct
// is now ready to be sent. If there is no pending message (e.g. the NextMessage was
// already sent and "pending" flag is reset) then no message will be sent.
func (h *SenderCommon) ScheduleSend() {
	// Set pending flag. Don't block on writing to channel.
	select {
	case h.hasPendingMessage <- struct{}{}:
	default:
		break
	}
}

// NextMessage gives access to the next message that will be sent by this looper.
// Can be called concurrently with any other method.
func (h *SenderCommon) NextMessage() *NextMessage {
	return &h.nextMessage
}

// SetInstanceUid sets a new instanceUid to be used for all subsequent messages to be sent.
// Can be called concurrently, normally is called when a message is received from the
// Server that instructs us to change our instance UID.
func (h *SenderCommon) SetInstanceUid(instanceUid types.InstanceUid) error {
	var emptyUid types.InstanceUid
	if instanceUid == emptyUid {
		return errors.New("cannot set instance uid to empty value")
	}

	h.nextMessage.Update(
		func(msg *protobufs.AgentToServer) {
			msg.InstanceUid = instanceUid[:]
		})

	return nil
}
