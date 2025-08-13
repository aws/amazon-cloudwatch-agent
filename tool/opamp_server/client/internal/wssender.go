package internal

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/internal"
	"github.com/open-telemetry/opamp-go/protobufs"
)

const (
	defaultSendCloseMessageTimeout = 5 * time.Second
	defaultHeartbeatIntervalMs     = 30 * 1000
)

// WSSender implements the WebSocket client's sending portion of OpAMP protocol.
type WSSender struct {
	SenderCommon
	conn   *websocket.Conn
	logger types.Logger

	// Indicates that the sender has fully stopped.
	stopped chan struct{}
	err     error

	heartbeatIntervalUpdated chan struct{}
	heartbeatIntervalMs      atomic.Int64
	heartbeatTimer           *time.Timer
}

// NewSender creates a new Sender that uses WebSocket to send
// messages to the server.
func NewSender(logger types.Logger) *WSSender {
	s := &WSSender{
		logger:                   logger,
		heartbeatIntervalUpdated: make(chan struct{}, 1),
		heartbeatTimer:           time.NewTimer(0),
		SenderCommon:             NewSenderCommon(),
	}
	s.heartbeatIntervalMs.Store(defaultHeartbeatIntervalMs)

	return s
}

// Start the sender and send the first message that was set via NextMessage().Update()
// earlier. To stop the WSSender cancel the ctx.
func (s *WSSender) Start(ctx context.Context, conn *websocket.Conn) error {
	s.conn = conn
	err := s.sendNextMessage(ctx)

	// Run the sender in the background.
	s.stopped = make(chan struct{})
	s.err = nil
	go s.run(ctx)

	return err
}

// IsStopped returns a channel that's closed when the sender is stopped.
func (s *WSSender) IsStopped() <-chan struct{} {
	return s.stopped
}

// StoppingErr returns an error if there was a problem with stopping the sender.
// If stopping was successful will return nil.
// StoppingErr() can be called only after IsStopped() is signalled.
func (s *WSSender) StoppingErr() error {
	return s.err
}

// SetHeartbeatInterval sets the heartbeat interval and triggers timer reset.
func (s *WSSender) SetHeartbeatInterval(d time.Duration) error {
	if d < 0 {
		return errors.New("heartbeat interval for wsclient must be non-negative")
	}

	s.heartbeatIntervalMs.Store(int64(d.Milliseconds()))
	select {
	case s.heartbeatIntervalUpdated <- struct{}{}:
	default:
	}
	return nil
}

func (s *WSSender) shouldSendHeartbeat() <-chan time.Time {
	t := s.heartbeatTimer

	// Handle both GODEBUG=asynctimerchan=[0|1] properly.
	// ref: https://pkg.go.dev/time#Timer.Reset
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}

	if d := time.Duration(s.heartbeatIntervalMs.Load()) * time.Millisecond; d != 0 {
		t.Reset(d)
		return t.C
	}

	// Heartbeat interval is set to Zero, disable heartbeat.
	return nil
}

func (s *WSSender) run(ctx context.Context) {
out:
	for {
		select {
		case <-s.shouldSendHeartbeat():
			s.NextMessage().Update(func(msg *protobufs.AgentToServer) {})
			s.ScheduleSend()
		case <-s.heartbeatIntervalUpdated:
			// trigger heartbeat timer reset
		case <-s.hasPendingMessage:
			s.sendNextMessage(ctx)

		case <-ctx.Done():
			if err := s.sendCloseMessage(); err != nil && err != websocket.ErrCloseSent {
				s.err = err
			}
			break out
		}
	}

	s.heartbeatTimer.Stop()
	close(s.stopped)
}

func (s *WSSender) sendCloseMessage() error {
	return s.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Normal closure"),
		time.Now().Add(defaultSendCloseMessageTimeout),
	)
}

func (s *WSSender) sendNextMessage(ctx context.Context) error {
	msgToSend := s.nextMessage.PopPending()
	if msgToSend != nil && !proto.Equal(msgToSend, &protobufs.AgentToServer{}) {
		// There is a pending message and the message has some fields populated.
		return s.sendMessage(ctx, msgToSend)
	}
	return nil
}

func (s *WSSender) sendMessage(ctx context.Context, msg *protobufs.AgentToServer) error {
	if err := internal.WriteWSMessage(s.conn, msg); err != nil {
		s.logger.Errorf(ctx, "Cannot write WS message: %v", err)
		// TODO: check if it is a connection error then propagate error back to Client and reconnect.
		return err
	}
	return nil
}
