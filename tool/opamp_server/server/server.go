package server

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"github.com/open-telemetry/opamp-go/server/types"
)

// Settings contains the settings for attaching an OpAMP Server.
type Settings struct {
	// Callbacks that the Server will call after successful Attach/Start.
	Callbacks types.Callbacks

	// EnableCompression can be set to true to enable the compression. Note that for WebSocket transport
	// the compression is only effectively enabled if the client also supports compression.
	// The data will be compressed in both directions.
	EnableCompression bool

	// Defines the custom capabilities of the Server. Each capability is a reverse FQDN with
	// optional version information that uniquely identifies the custom capability and
	// should match a capability specified in a supported CustomMessage.
	//
	// See
	// https://github.com/open-telemetry/opamp-spec/blob/main/specification.md#customcapabilities
	// for more details.
	CustomCapabilities []string
}

// StartSettings contains the settings for starting an OpAMP Server.
type StartSettings struct {
	Settings

	// ListenEndpoint specifies the endpoint to listen on, e.g. "127.0.0.1:4320"
	ListenEndpoint string

	// ListenPath specifies the URL path on which to accept the OpAMP connections
	// If this is empty string then Start() will use the default "/v1/opamp" path.
	ListenPath string

	// Server's TLS configuration.
	TLSConfig *tls.Config

	// HTTPMiddleware specifies middleware for HTTP messages received by the server.
	// Note that the function will be called once for websockets upon connecting and will
	// be called for every HTTP request. This function is optional to set.
	HTTPMiddleware func(handler http.Handler) http.Handler
}

type HTTPHandlerFunc func(http.ResponseWriter, *http.Request)

type ConnContext func(ctx context.Context, c net.Conn) context.Context

// OpAMPServer is an interface representing the server side of the OpAMP protocol.
type OpAMPServer interface {
	// Attach prepares the OpAMP Server to begin handling requests from an existing
	// http.Server. The returned HTTPHandlerFunc and ConnContext should be added as a
	// handler and ConnContext respectively to the desired http.Server by the caller
	// and the http.Server should be started by the caller after that. The ConnContext
	// is only used for plain http connections.
	// For example:
	//   handler, connContext, _ := Server.Attach()
	//   mux := http.NewServeMux()
	//   mux.HandleFunc("/opamp", handler)
	//   httpSrv := &http.Server{Handler:mux,Addr:"127.0.0.1:4320", ConnContext: connContext}
	//   httpSrv.ListenAndServe()
	Attach(settings Settings) (HTTPHandlerFunc, ConnContext, error)

	// Start an OpAMP Server and begin accepting connections. Starts its own http.Server
	// using provided settings. This should block until the http.Server is ready to
	// accept connections.
	Start(settings StartSettings) error

	// Stop accepting new connections and close all current connections.
	// This operation should block until both the server socket and all
	// connections have been closed.
	Stop(ctx context.Context) error

	// Addr returns the network address Server is listening on. Nil if not started.
	// Typically used to fetch the port when ListenEndpoint's port is specified as 0 to
	// allocate an ephemeral port.
	Addr() net.Addr
}
