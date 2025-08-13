package server

import (
	"context"
	"errors"
	"net"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server/types"
)

// ErrInvalidHTTPConnection represents an event of misuse function for plain HTTP
// connection, such as httpConnection.Send() or httpConnection.Disconnect().
// Usage will not result with change but return this error to indicate current state
// might not be as expected.
var ErrInvalidHTTPConnection = errors.New("cannot operate over HTTP connection")

// httpConnection represents an OpAMP connection over a plain HTTP connection.
// Only one response is possible to send when using plain HTTP connection
// and that response will be sent by OpAMP Server's HTTP request handler after the
// onMessage callback returns.
type httpConnection struct {
	conn net.Conn
}

func (c httpConnection) Connection() net.Conn {
	return c.conn
}

var _ types.Connection = (*httpConnection)(nil)

func (c httpConnection) Send(_ context.Context, _ *protobufs.ServerToAgent) error {
	// Send() should not be called for plain HTTP connection. Instead, the response will
	// be sent after the onMessage callback returns.
	return ErrInvalidHTTPConnection
}

func (c httpConnection) Disconnect() error {
	// Disconnect() should not be called for plain HTTP connection.
	return ErrInvalidHTTPConnection
}
