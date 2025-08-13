package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/internal"
	"github.com/open-telemetry/opamp-go/protobufs"
	serverTypes "github.com/open-telemetry/opamp-go/server/types"
)

var errAlreadyStarted = errors.New("already started")

const (
	defaultOpAMPPath      = "/v1/opamp"
	headerContentType     = "Content-Type"
	headerContentEncoding = "Content-Encoding"
	headerAcceptEncoding  = "Accept-Encoding"
	contentEncodingGzip   = "gzip"
	contentTypeProtobuf   = "application/x-protobuf"
)

type server struct {
	logger   types.Logger
	settings Settings

	// Upgrader to use to upgrade HTTP to WebSocket.
	wsUpgrader websocket.Upgrader

	// The listening HTTP Server after successful Start() call. Nil if Start()
	// is not called or was not successful.
	httpServer        *http.Server
	httpServerServeWg *sync.WaitGroup

	// The network address Server is listening on. Nil if not started.
	addr net.Addr
}

var _ OpAMPServer = (*server)(nil)

// innerHTTPHandler implements the http.Handler interface so it can be used by functions
// that require the type (like Middleware) without exposing ServeHTTP directly on server.
type innerHTTPHander struct {
	httpHandlerFunc http.HandlerFunc
}

func (i innerHTTPHander) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	i.httpHandlerFunc(writer, request)
}

// New creates a new OpAMP Server.
func New(logger types.Logger) *server {
	if logger == nil {
		logger = &internal.NopLogger{}
	}

	return &server{logger: logger}
}

func (s *server) Attach(settings Settings) (HTTPHandlerFunc, ConnContext, error) {
	s.settings = settings
	s.settings.Callbacks.SetDefaults()
	s.wsUpgrader = websocket.Upgrader{
		EnableCompression: settings.EnableCompression,
	}
	return s.httpHandler, contextWithConn, nil
}

func (s *server) Start(settings StartSettings) error {
	if s.httpServer != nil {
		return errAlreadyStarted
	}

	_, _, err := s.Attach(settings.Settings)
	if err != nil {
		return err
	}

	// Prepare handling OpAMP incoming HTTP requests on the requests URL path.
	mux := http.NewServeMux()

	path := settings.ListenPath
	if path == "" {
		path = defaultOpAMPPath
	}

	handler := innerHTTPHander{s.httpHandler}

	if settings.HTTPMiddleware != nil {
		mux.Handle(path, settings.HTTPMiddleware(handler))
	} else {
		mux.Handle(path, handler)
	}

	hs := &http.Server{
		Handler:     mux,
		Addr:        settings.ListenEndpoint,
		TLSConfig:   settings.TLSConfig,
		ConnContext: contextWithConn,
	}
	s.httpServer = hs
	httpServerServeWg := sync.WaitGroup{}
	httpServerServeWg.Add(1)
	s.httpServerServeWg = &httpServerServeWg

	listenAddr := s.httpServer.Addr

	// Start the HTTP Server in background.
	if hs.TLSConfig != nil {
		if listenAddr == "" {
			listenAddr = ":https"
		}
		err = s.startHttpServer(
			listenAddr,
			func(l net.Listener) error {
				defer httpServerServeWg.Done()
				return hs.ServeTLS(l, "", "")
			},
		)
	} else {
		if listenAddr == "" {
			listenAddr = ":http"
		}
		err = s.startHttpServer(
			listenAddr,
			func(l net.Listener) error {
				defer httpServerServeWg.Done()
				return hs.Serve(l)
			},
		)
	}
	return err
}

func (s *server) startHttpServer(listenAddr string, serveFunc func(l net.Listener) error) error {
	// If the listen address is not specified use the default.
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	s.addr = ln.Addr()

	// Begin serving connections in the background.
	go func() {
		err = serveFunc(ln)

		// ErrServerClosed is expected after successful Stop(), so we won't log that
		// particular error.
		if err != nil && err != http.ErrServerClosed {
			s.logger.Errorf(context.Background(), "Error running HTTP Server: %v", err)
		}
	}()

	return nil
}

func (s *server) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		defer func() { s.httpServer = nil }()
		// This stops accepting new connections. TODO: close existing
		// connections and wait them to be terminated.
		err := s.httpServer.Shutdown(ctx)
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			s.httpServerServeWg.Wait()
		}
	}
	return nil
}

func (s *server) Addr() net.Addr {
	return s.addr
}

func (s *server) httpHandler(w http.ResponseWriter, req *http.Request) {
	var connectionCallbacks serverTypes.ConnectionCallbacks
	resp := s.settings.Callbacks.OnConnecting(req)
	if !resp.Accept {
		// HTTP connection is not accepted. Set the response headers.
		for k, v := range resp.HTTPResponseHeader {
			w.Header().Set(k, v)
		}
		// And write the response status code.
		w.WriteHeader(resp.HTTPStatusCode)
		return
	}
	// use connection-specific handler provided by ConnectionResponse
	connectionCallbacks = resp.ConnectionCallbacks
	connectionCallbacks.SetDefaults()

	// HTTP connection is accepted. Check if it is a plain HTTP request.

	if req.Header.Get(headerContentType) == contentTypeProtobuf {
		// Yes, a plain HTTP request.
		s.handlePlainHTTPRequest(req, w, &connectionCallbacks)
		return
	}

	// No, it is a WebSocket. Upgrade it.
	conn, err := s.wsUpgrader.Upgrade(w, req, nil)
	if err != nil {
		s.logger.Errorf(req.Context(), "Cannot upgrade HTTP connection to WebSocket: %v", err)
		return
	}

	// Return from this func to reduce memory usage.
	// Handle the connection on a separate goroutine.
	go s.handleWSConnection(req.Context(), conn, &connectionCallbacks)
}

func (s *server) handleWSConnection(reqCtx context.Context, wsConn *websocket.Conn, connectionCallbacks *serverTypes.ConnectionCallbacks) {
	agentConn := newWSConnection(wsConn)

	defer func() {
		// Close the connection when all is done.
		defer func() {
			err := agentConn.Disconnect()
			if err != nil {
				s.logger.Errorf(context.Background(), "error closing the WebSocket connection: %v", err)
			}
		}()

		connectionCallbacks.OnConnectionClose(agentConn)
	}()

	connectionCallbacks.OnConnected(reqCtx, agentConn)

	sentCustomCapabilities := false

	// Loop until fail to read from the WebSocket connection.
	for {
		msgContext := context.Background()
		request := protobufs.AgentToServer{}

		// Block until the next message can be read.
		mt, msgBytes, err := wsConn.ReadMessage()
		isBreak, err := func() (bool, error) {
			if err != nil {
				if !websocket.IsUnexpectedCloseError(err) {
					s.logger.Errorf(msgContext, "Cannot read a message from WebSocket: %v", err)
					return true, err
				}
				// This is a normal closing of the WebSocket connection.
				s.logger.Debugf(msgContext, "Agent disconnected: %v", err)
				return true, err
			}
			if mt != websocket.BinaryMessage {
				err = fmt.Errorf("unexpected message type: %v, must be binary message", mt)
				s.logger.Errorf(msgContext, "Cannot process a message from WebSocket: %v", err)
				return false, err
			}

			// Decode WebSocket message as a Protobuf message.
			err = internal.DecodeWSMessage(msgBytes, &request)
			if err != nil {
				s.logger.Errorf(msgContext, "Cannot decode message from WebSocket: %v", err)
				return false, err
			}
			return false, nil
		}()
		if err != nil {
			connectionCallbacks.OnReadMessageError(agentConn, mt, msgBytes, err)
			if isBreak {
				break
			}
			continue
		}

		response := connectionCallbacks.OnMessage(msgContext, agentConn, &request)
		if response == nil { // No send message when 'response' is empty
			continue
		}

		if len(response.InstanceUid) == 0 {
			response.InstanceUid = request.InstanceUid
		}
		if !sentCustomCapabilities {
			response.CustomCapabilities = &protobufs.CustomCapabilities{
				Capabilities: s.settings.CustomCapabilities,
			}
			sentCustomCapabilities = true
		}
		err = agentConn.Send(msgContext, response)
		if err != nil {
			s.logger.Errorf(msgContext, "Cannot send message to WebSocket: %v", err)
		}
	}
}

func decompressGzip(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func (s *server) readReqBody(req *http.Request) ([]byte, error) {
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	if req.Header.Get(headerContentEncoding) == contentEncodingGzip {
		data, err = decompressGzip(data)
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

func compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write(data)
	if err != nil {
		return nil, err
	}
	err = w.Close()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *server) handlePlainHTTPRequest(req *http.Request, w http.ResponseWriter, connectionCallbacks *serverTypes.ConnectionCallbacks) {
	bodyBytes, err := s.readReqBody(req)
	if err != nil {
		s.logger.Debugf(req.Context(), "Cannot read HTTP body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Decode the message as a Protobuf message.
	var request protobufs.AgentToServer
	err = proto.Unmarshal(bodyBytes, &request)
	if err != nil {
		s.logger.Debugf(req.Context(), "Cannot decode message from HTTP Body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	agentConn := httpConnection{
		conn: connFromRequest(req),
	}

	connectionCallbacks.OnConnected(req.Context(), agentConn)

	defer func() {
		// Indicate via the callback that the OpAMP Connection is closed. From OpAMP
		// perspective the connection represented by this http request
		// is closed. It is not possible to send or receive more OpAMP messages
		// via this agentConn.
		connectionCallbacks.OnConnectionClose(agentConn)
	}()

	response := connectionCallbacks.OnMessage(req.Context(), agentConn, &request)

	if response == nil {
		response = &protobufs.ServerToAgent{}
	}

	// Set the InstanceUid if it is not set by the callback.
	if len(response.InstanceUid) == 0 {
		response.InstanceUid = request.InstanceUid
	}

	// Return the CustomCapabilities
	response.CustomCapabilities = &protobufs.CustomCapabilities{
		Capabilities: s.settings.CustomCapabilities,
	}

	// Marshal the response.
	bodyBytes, err = proto.Marshal(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Send the response.
	w.Header().Set(headerContentType, contentTypeProtobuf)
	if req.Header.Get(headerAcceptEncoding) == contentEncodingGzip {
		bodyBytes, err = compressGzip(bodyBytes)
		if err != nil {
			s.logger.Errorf(req.Context(), "Cannot compress response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set(headerContentEncoding, contentEncodingGzip)
	}
	_, err = w.Write(bodyBytes)
	if err != nil {
		s.logger.Debugf(req.Context(), "Cannot send HTTP response: %v", err)
	}
}
