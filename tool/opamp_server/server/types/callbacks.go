package types

import (
	"context"
	"net/http"

	"github.com/open-telemetry/opamp-go/protobufs"
)

// ConnectionResponse is the return type of the OnConnecting callback.
type ConnectionResponse struct {
	Accept              bool
	HTTPStatusCode      int
	HTTPResponseHeader  map[string]string
	ConnectionCallbacks ConnectionCallbacks
}

type Callbacks struct {
	// OnConnecting is called when there is a new incoming connection.
	// The handler can examine the request and either accept or reject the connection.
	// To accept:
	//   Return ConnectionResponse with Accept=true. ConnectionCallbacks MUST be set to an
	//   instance of the ConnectionCallbacks struct to handle the connection callbacks.
	//   HTTPStatusCode and HTTPResponseHeader are ignored.
	//
	// To reject:
	//   Return ConnectionResponse with Accept=false. HTTPStatusCode MUST be set to
	//   non-zero value to indicate the rejection reason (typically 401, 429 or 503).
	//   HTTPResponseHeader may be optionally set (e.g. "Retry-After: 30").
	//   ConnectionCallbacks is ignored.
	OnConnecting func(request *http.Request) ConnectionResponse
}

func defaultOnConnecting(r *http.Request) ConnectionResponse {
	return ConnectionResponse{Accept: true}
}

func (c *Callbacks) SetDefaults() {
	if c.OnConnecting == nil {
		c.OnConnecting = defaultOnConnecting
	}
}

// ConnectionCallbacks specifies callbacks for a specific connection. An instance of
// this struct MUST be set on the ConnectionResponse returned by the OnConnecting
// callback if Accept=true. The instance can be shared by all connections or can be
// unique for each connection.
type ConnectionCallbacks struct {
	// The following callbacks will never be called concurrently for the same
	// connection. They may be called concurrently for different connections.

	// OnConnected is called when an incoming OpAMP connection is successfully
	// established after OnConnecting() returns.
	OnConnected func(ctx context.Context, conn Connection)

	// OnMessage is called when a message is received from the connection. Can happen
	// only after OnConnected().
	// When the returned ServerToAgent message is nil, WebSocket will not send a
	// message to the Agent, and the HTTP request will respond to an empty message.
	// If the return is not nil it will be sent as a response to the Agent.
	// For plain HTTP requests once OnMessage returns and the response is sent
	// to the Agent the OnConnectionClose message will be called immediately.
	OnMessage func(ctx context.Context, conn Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent

	// OnConnectionClose is called when the OpAMP connection is closed.
	OnConnectionClose func(conn Connection)

	// OnReadMessageError is called when an error occurs while reading or deserializing a message.
	OnReadMessageError func(conn Connection, mt int, msgByte []byte, err error)
}

func defaultOnConnected(ctx context.Context, conn Connection) {}

func defaultOnMessage(
	ctx context.Context, conn Connection, message *protobufs.AgentToServer,
) *protobufs.ServerToAgent {
	// We will send an empty response since there is no user-defined callback to handle it.
	return &protobufs.ServerToAgent{
		InstanceUid: message.InstanceUid,
	}
}

func defaultOnConnectionClose(conn Connection) {}

func defaultOnReadMessageError(conn Connection, mt int, msgByte []byte, err error) {}

func (c *ConnectionCallbacks) SetDefaults() {
	if c.OnConnected == nil {
		c.OnConnected = defaultOnConnected
	}

	if c.OnMessage == nil {
		c.OnMessage = defaultOnMessage
	}

	if c.OnConnectionClose == nil {
		c.OnConnectionClose = defaultOnConnectionClose
	}

	if c.OnReadMessageError == nil {
		c.OnReadMessageError = defaultOnReadMessageError
	}
}
