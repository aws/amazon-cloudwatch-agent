package types

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
)

// StartSettings defines the parameters for starting the OpAMP Client.
type StartSettings struct {
	// Connection parameters.

	// Server URL. MUST be set.
	OpAMPServerURL string

	// Optional additional HTTP headers to send with all HTTP requests.
	Header http.Header

	// Optional function that can be used to modify the HTTP headers
	// before each HTTP request.
	// Can modify and return the argument or return the argument without modifying.
	HeaderFunc func(http.Header) http.Header

	// Optional TLS config for HTTP connection.
	TLSConfig *tls.Config

	// Agent information.
	InstanceUid InstanceUid

	// Callbacks that the client will call after Start() returns nil.
	Callbacks Callbacks

	// Previously saved state. These will be reported to the Server immediately
	// after the connection is established.

	// The remote config status. If nil is passed it will force
	// the Server to send a remote config back.
	RemoteConfigStatus *protobufs.RemoteConfigStatus

	LastConnectionSettingsHash []byte

	// PackagesStateProvider provides access to the local state of packages.
	// If nil then ReportsPackageStatuses and AcceptsPackages capabilities will be disabled,
	// i.e. package status reporting and syncing from the Server will be disabled.
	PackagesStateProvider PackagesStateProvider

	// Defines the capabilities of the Agent. AgentCapabilities_ReportsStatus bit does not need to
	// be set in this field, it will be set automatically since it is required by OpAMP protocol.
	// Deprecated: Use client.SetCapabilities() instead.
	Capabilities protobufs.AgentCapabilities

	// EnableCompression can be set to true to enable the compression. Note that for WebSocket transport
	// the compression is only effectively enabled if the Server also supports compression.
	// The data will be compressed in both directions.
	EnableCompression bool

	// Optional HeartbeatInterval to configure the heartbeat interval for client.
	// If nil, the default heartbeat interval (30s) will be used.
	// If zero, heartbeat will be disabled for a Websocket-based client.
	//
	// Note that an HTTP-based client will use the heartbeat interval as its polling interval
	// and zero is invalid for an HTTP-based client.
	//
	// If the ReportsHeartbeat capability is disabled, this option has no effect.
	HeartbeatInterval *time.Duration

	// Optional DownloadReporterInterval to configure how often a client reports the status of a package that is being downloaded.
	// If nil, the default reporter interval (10s) will be used.
	// If specified a minimum value of 1s will be enforced.
	DownloadReporterInterval *time.Duration
}
