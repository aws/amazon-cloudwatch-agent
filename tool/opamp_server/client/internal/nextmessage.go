package internal

import (
	"sync"

	"github.com/open-telemetry/opamp-go/protobufs"
	"google.golang.org/protobuf/proto"
)

// NextMessage encapsulates the next message to be sent and provides a
// concurrency-safe interface to work with the message.
type NextMessage struct {
	// The next message to send.
	nextMessage *protobufs.AgentToServer
	// nextMessageSending is a channel that is closed when the message is sent.
	nextMessageSending chan struct{}
	// Indicates that nextMessage is pending to be sent.
	messagePending bool
	// Mutex to protect the above 3 fields.
	messageMutex sync.Mutex
}

// NewNextMessage returns a new empty NextMessage.
func NewNextMessage() NextMessage {
	return NextMessage{
		nextMessage:        &protobufs.AgentToServer{},
		nextMessageSending: make(chan struct{}),
	}
}

// Update applies the specified modifier function to the next message that will be sent
// and marks the message as pending to be sent.
//
// The messageSendingChannel returned by this function is closed when the modified message
// is popped in PopPending before being sent to the server. After this channel is closed,
// additional calls to Update will be applied to the next message and will return a
// channel corresponding to that message.
func (s *NextMessage) Update(modifier func(msg *protobufs.AgentToServer)) (messageSendingChannel chan struct{}) {
	s.messageMutex.Lock()
	modifier(s.nextMessage)
	s.messagePending = true
	sending := s.nextMessageSending
	s.messageMutex.Unlock()
	return sending
}

// PopPending returns the next message to be sent, if it is pending or nil otherwise.
// Clears the "pending" flag.
func (s *NextMessage) PopPending() *protobufs.AgentToServer {
	var msgToSend *protobufs.AgentToServer
	s.messageMutex.Lock()
	if s.messagePending {
		// Clone the message to have a copy for sending and avoid blocking
		// future updates to s.NextMessage field.
		msgToSend = proto.Clone(s.nextMessage).(*protobufs.AgentToServer)
		s.messagePending = false

		// Reset fields that we do not have to send unless they change before the
		// next report after this one.
		msg := &protobufs.AgentToServer{
			InstanceUid: s.nextMessage.InstanceUid,
			// Increment the sequence number.
			SequenceNum:  s.nextMessage.SequenceNum + 1,
			Capabilities: s.nextMessage.Capabilities,
		}

		sending := s.nextMessageSending

		s.nextMessage = msg
		s.nextMessageSending = make(chan struct{})

		// Notify that the message is being sent and a new nextMessage has been created.
		close(sending)
	}
	s.messageMutex.Unlock()
	return msgToSend
}
