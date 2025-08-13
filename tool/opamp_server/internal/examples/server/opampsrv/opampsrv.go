package opampsrv

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/oklog/ulid/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/open-telemetry/opamp-go/internal"
	"github.com/open-telemetry/opamp-go/internal/examples/server/data"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	"github.com/open-telemetry/opamp-go/server/types"
)

type Server struct {
	opampSrv server.OpAMPServer
	agents   *data.Agents
	logger   *Logger
}

func NewServer(agents *data.Agents) *Server {
	logger := &Logger{
		log.New(
			log.Default().Writer(),
			"[OPAMP] ",
			log.Default().Flags()|log.Lmsgprefix|log.Lmicroseconds,
		),
	}

	srv := &Server{
		agents: agents,
		logger: logger,
	}

	srv.opampSrv = server.New(logger)

	return srv
}

func (srv *Server) Start() {
	settings := server.StartSettings{
		Settings: server.Settings{
			Callbacks: types.Callbacks{
				OnConnecting: func(request *http.Request) types.ConnectionResponse {
					return types.ConnectionResponse{
						Accept: true,
						ConnectionCallbacks: types.ConnectionCallbacks{
							OnMessage:         srv.onMessage,
							OnConnectionClose: srv.onDisconnect,
						},
					}
				},
			},
		},
		ListenEndpoint: "127.0.0.1:4320",
		HTTPMiddleware: otelhttp.NewMiddleware("/v1/opamp"),
	}
	tlsConfig, err := internal.CreateServerTLSConfig(
		"../../certs/certs/ca.cert.pem",
		"../../certs/server_certs/server.cert.pem",
		"../../certs/server_certs/server.key.pem",
	)
	if err != nil {
		srv.logger.Debugf(context.Background(), "Could not load TLS config, working without TLS: %v", err.Error())
	}
	settings.TLSConfig = tlsConfig

	if err := srv.opampSrv.Start(settings); err != nil {
		srv.logger.Errorf(context.Background(), "OpAMP server start fail: %v", err.Error())
		os.Exit(1)
	}
}

func (srv *Server) Stop() {
	srv.opampSrv.Stop(context.Background())
}

func (srv *Server) onDisconnect(conn types.Connection) {
	srv.agents.RemoveConnection(conn)
}

func (srv *Server) onMessage(ctx context.Context, conn types.Connection, msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
	// Start building the response.
	response := &protobufs.ServerToAgent{}

	var instanceId data.InstanceId
	if len(msg.InstanceUid) == 26 {
		// This is an old-style ULID.
		u, err := ulid.Parse(string(msg.InstanceUid))
		if err != nil {
			srv.logger.Errorf(ctx, "Cannot parse ULID %s: %v", string(msg.InstanceUid), err)
			return response
		}
		instanceId = data.InstanceId(u.Bytes())
	} else if len(msg.InstanceUid) == 16 {
		// This is a 16 byte, new style UID.
		instanceId = data.InstanceId(msg.InstanceUid)
	} else {
		srv.logger.Errorf(ctx, "Invalid length of msg.InstanceUid")
		return response
	}

	agent := srv.agents.FindOrCreateAgent(instanceId, conn)

	// Process the status report and continue building the response.
	agent.UpdateStatus(msg, response)

	// Send the response back to the Agent.
	return response
}
