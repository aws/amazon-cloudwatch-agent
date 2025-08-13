package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opamp-go/client/internal"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/internal/testhelpers"
	"github.com/open-telemetry/opamp-go/protobufs"
)

func TestWSSenderReportsHeartbeat(t *testing.T) {
	tests := []struct {
		name                  string
		clientEnableHeartbeat bool
		serverEnableHeartbeat bool
		expectHeartbeats      bool
	}{
		{"enable heartbeat", true, true, true},
		{"client disable heartbeat", false, true, false},
		{"server disable heartbeat", true, false, false},
	}

	for _, tt := range tests {
		srv := internal.StartMockServer(t)

		var firstMsg atomic.Bool
		var conn atomic.Value
		srv.OnWSConnect = func(c *websocket.Conn) {
			conn.Store(c)
			firstMsg.Store(true)
		}
		var msgCount atomic.Int64
		srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			if firstMsg.Load() {
				firstMsg.Store(false)
				resp := &protobufs.ServerToAgent{
					InstanceUid: msg.InstanceUid,
					ConnectionSettings: &protobufs.ConnectionSettingsOffers{
						Opamp: &protobufs.OpAMPConnectionSettings{
							HeartbeatIntervalSeconds: 1,
						},
					},
				}
				if !tt.serverEnableHeartbeat {
					resp.ConnectionSettings.Opamp.HeartbeatIntervalSeconds = 0
				}
				return resp
			}
			msgCount.Add(1)
			return nil
		}

		// Start an OpAMP/WebSocket client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
		}
		if tt.clientEnableHeartbeat {
			settings.Capabilities = protobufs.AgentCapabilities_AgentCapabilities_ReportsHeartbeat
		}
		client := NewWebSocket(nil)
		startClient(t, settings, client)

		// Wait for connection to be established.
		eventually(t, func() bool { return conn.Load() != nil })

		if tt.expectHeartbeats {
			assert.Eventually(t, func() bool {
				return msgCount.Load() >= 2
			}, 3*time.Second, 10*time.Millisecond)
		} else {
			assert.Never(t, func() bool {
				return msgCount.Load() >= 2
			}, 50*time.Millisecond, 10*time.Millisecond)
		}

		// Stop the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	}
}

func TestWSClientStartWithHeartbeatInterval(t *testing.T) {
	tests := []struct {
		name                  string
		clientEnableHeartbeat bool
		expectHeartbeats      bool
	}{
		{"client enable heartbeat", true, true},
		{"client disable heartbeat", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := internal.StartMockServer(t)

			var conn atomic.Value
			srv.OnWSConnect = func(c *websocket.Conn) {
				conn.Store(c)
			}
			var msgCount atomic.Int64
			srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
				msgCount.Add(1)
				return nil
			}

			// Start an OpAMP/WebSocket client.
			heartbeat := 10 * time.Millisecond
			settings := types.StartSettings{
				OpAMPServerURL:    "ws://" + srv.Endpoint,
				HeartbeatInterval: &heartbeat,
			}
			if tt.clientEnableHeartbeat {
				settings.Capabilities = protobufs.AgentCapabilities_AgentCapabilities_ReportsHeartbeat
			}
			client := NewWebSocket(nil)
			startClient(t, settings, client)

			// Wait for connection to be established.
			eventually(t, func() bool { return conn.Load() != nil })

			if tt.expectHeartbeats {
				assert.Eventually(t, func() bool {
					return msgCount.Load() >= 2
				}, 5*time.Second, 10*time.Millisecond)
			} else {
				assert.Never(t, func() bool {
					return msgCount.Load() >= 2
				}, 50*time.Millisecond, 10*time.Millisecond)
			}

			// Stop the client.
			err := client.Stop(context.Background())
			assert.NoError(t, err)
		})
	}
}

func TestDisconnectWSByServer(t *testing.T) {
	// Start a Server.
	srv := internal.StartMockServer(t)

	var conn atomic.Value
	srv.OnWSConnect = func(c *websocket.Conn) {
		conn.Store(c)
	}

	// Start an OpAMP/WebSocket client.
	var connected int64
	var connectErr atomic.Value
	settings := types.StartSettings{
		Callbacks: types.Callbacks{
			OnConnect: func(ctx context.Context) {
				atomic.StoreInt64(&connected, 1)
			},
			OnConnectFailed: func(ctx context.Context, err error) {
				connectErr.Store(err)
			},
		},
	}
	settings.OpAMPServerURL = "ws://" + srv.Endpoint
	client := NewWebSocket(nil)
	startClient(t, settings, client)

	// Wait for connection to be established.
	eventually(t, func() bool { return conn.Load() != nil })
	assert.True(t, connectErr.Load() == nil)

	// Close the Server and forcefully disconnect.
	srv.Close()
	_ = conn.Load().(*websocket.Conn).Close()

	// The client must retry and must fail now.
	eventually(t, func() bool { return connectErr.Load() != nil })

	// Stop the client.
	err := client.Stop(context.Background())
	assert.NoError(t, err)
}

func TestVerifyWSCompress(t *testing.T) {
	tests := []bool{false, true}
	for _, withCompression := range tests {
		t.Run(fmt.Sprintf("%v", withCompression), func(t *testing.T) {
			// Start a Server.
			srv := internal.StartMockServer(t)
			srv.EnableExpectMode()
			if withCompression {
				srv.EnableCompression()
			}

			// We use a transparent TCP proxy to be able to count the actual bytes transferred so that
			// we can test the number of actual bytes vs number of expected bytes with and without compression.
			proxy := testhelpers.NewProxy(srv.Endpoint)
			assert.NoError(t, proxy.Start())

			// Start an OpAMP/WebSocket client.
			var clientGotRemoteConfig atomic.Value
			settings := types.StartSettings{
				Callbacks: types.Callbacks{
					OnMessage: func(ctx context.Context, msg *types.MessageData) {
						if msg.RemoteConfig != nil {
							clientGotRemoteConfig.Store(msg.RemoteConfig)
						}
					},
					GetEffectiveConfig: func(ctx context.Context) (*protobufs.EffectiveConfig, error) {
						// If the client already received a remote config offer make sure to report
						// the effective config back to the server.
						var effCfg []byte
						remoteCfg, _ := clientGotRemoteConfig.Load().(*protobufs.AgentRemoteConfig)
						if remoteCfg != nil {
							effCfg = remoteCfg.Config.ConfigMap[""].Body
						}
						return &protobufs.EffectiveConfig{
							ConfigMap: &protobufs.AgentConfigMap{
								ConfigMap: map[string]*protobufs.AgentConfigFile{
									"key": {
										Body: effCfg,
									},
								},
							},
						}, nil
					},
				},
				Capabilities: protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig |
					protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig,
			}
			settings.OpAMPServerURL = "ws://" + proxy.IncomingEndpoint()

			if withCompression {
				settings.EnableCompression = true
			}

			client := NewWebSocket(nil)
			startClient(t, settings, client)

			// Use highly compressible config body.
			uncompressedCfg := []byte(strings.Repeat("test", 10000))

			remoteCfg := &protobufs.AgentRemoteConfig{
				Config: &protobufs.AgentConfigMap{
					ConfigMap: map[string]*protobufs.AgentConfigFile{
						"": {
							Body: uncompressedCfg,
						},
					},
				},
				ConfigHash: []byte{1, 2, 3, 4},
			}

			// ---> Server
			srv.Expect(
				func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
					assert.EqualValues(t, 0, msg.SequenceNum)
					// The first status report after Start must have full AgentDescription.
					assert.True(t, proto.Equal(client.AgentDescription(), msg.AgentDescription))
					return &protobufs.ServerToAgent{
						InstanceUid:  msg.InstanceUid,
						RemoteConfig: remoteCfg,
					}
				},
			)

			// Wait to receive remote config
			eventually(t, func() bool { return clientGotRemoteConfig.Load() != nil })

			_ = client.UpdateEffectiveConfig(context.Background())

			// ---> Server
			srv.Expect(
				func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
					return &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid}
				},
			)

			// Stop the client.
			err := client.Stop(context.Background())
			assert.NoError(t, err)

			proxy.Stop()

			fmt.Printf("sent %d, received %d\n", proxy.ClientToServerBytes(), proxy.ServerToClientBytes())

			if withCompression {
				// With compression the entire bytes exchanged should be less than the config body.
				// This is only possible if there is any compression happening.
				assert.Less(t, proxy.ServerToClientBytes(), len(uncompressedCfg))
				assert.Less(t, proxy.ClientToServerBytes(), len(uncompressedCfg))
			} else {
				// Without compression the entire bytes exchanged should be more than the config body.
				assert.Greater(t, proxy.ServerToClientBytes(), len(uncompressedCfg))
				assert.Greater(t, proxy.ClientToServerBytes(), len(uncompressedCfg))
			}
		})
	}
}

func redirectServer(to string, status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, to, http.StatusSeeOther)
	}))
}

func errServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(302)
	}))
}

type checkRedirectMock struct {
	mock.Mock
	t      testing.TB
	viaLen int
	http   bool
}

func (c *checkRedirectMock) CheckRedirect(req *http.Request, viaReq []*http.Request, via []*http.Response) error {
	if req == nil {
		c.t.Error("nil request in CheckRedirect")
		return errors.New("nil request in CheckRedirect")
	}
	if len(viaReq) > c.viaLen {
		c.t.Error("viaReq should be shorter than viaLen")
	}
	if !c.http {
		// websocket transport
		if len(via) > c.viaLen {
			c.t.Error("via should be shorter than viaLen")
		}
	}
	if !c.http && len(via) > 0 {
		location, err := via[len(via)-1].Location()
		if err != nil {
			c.t.Error(err)
		}
		// the URL of the request should match the location header of the last response
		assert.Equal(c.t, req.URL, location, "request URL should equal the location in the response")
	}
	return c.Called(req, via).Error(0)
}

func mockRedirect(t testing.TB, viaLen int, err error) *checkRedirectMock {
	m := &checkRedirectMock{
		t:      t,
		viaLen: viaLen,
	}
	m.On("CheckRedirect", mock.Anything, mock.Anything, mock.Anything).Return(err)
	return m
}

func TestRedirectWS(t *testing.T) {
	redirectee := internal.StartMockServer(t)
	tests := []struct {
		Name         string
		Redirector   *httptest.Server
		ExpError     bool
		MockRedirect *checkRedirectMock
	}{
		{
			Name:       "redirect ws scheme",
			Redirector: redirectServer("ws://"+redirectee.Endpoint, 302),
		},
		{
			Name:       "redirect http scheme",
			Redirector: redirectServer("http://"+redirectee.Endpoint, 302),
		},
		{
			Name:       "missing location header",
			Redirector: errServer(),
			ExpError:   true,
		},
		{
			Name:         "check redirect",
			Redirector:   redirectServer("ws://"+redirectee.Endpoint, 302),
			MockRedirect: mockRedirect(t, 1, nil),
		},
		{
			Name:         "check redirect returns error",
			Redirector:   redirectServer("ws://"+redirectee.Endpoint, 302),
			MockRedirect: mockRedirect(t, 1, errors.New("hello")),
			ExpError:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var conn atomic.Value
			redirectee.OnWSConnect = func(c *websocket.Conn) {
				conn.Store(c)
			}

			// Start an OpAMP/WebSocket client.
			var connected int64
			var connectErr atomic.Value
			settings := types.StartSettings{
				Callbacks: types.Callbacks{
					OnConnect: func(ctx context.Context) {
						atomic.StoreInt64(&connected, 1)
					},
					OnConnectFailed: func(ctx context.Context, err error) {
						if err != websocket.ErrBadHandshake {
							connectErr.Store(err)
						}
					},
				},
			}
			if test.MockRedirect != nil {
				settings.Callbacks.CheckRedirect = test.MockRedirect.CheckRedirect
			}
			reURL, err := url.Parse(test.Redirector.URL)
			assert.NoError(t, err)
			reURL.Scheme = "ws"
			settings.OpAMPServerURL = reURL.String()
			client := NewWebSocket(nil)
			startClient(t, settings, client)

			// Wait for connection to be established.
			eventually(t, func() bool {
				return conn.Load() != nil || connectErr.Load() != nil || client.lastInternalErr.Load() != nil
			})
			if test.ExpError {
				if connectErr.Load() == nil && client.lastInternalErr.Load() == nil {
					t.Error("expected non-nil error")
				}
			} else {
				assert.True(t, connectErr.Load() == nil)
			}

			// Stop the client.
			err = client.Stop(context.Background())
			assert.NoError(t, err)

			if test.MockRedirect != nil {
				test.MockRedirect.AssertCalled(t, "CheckRedirect", mock.Anything, mock.Anything)
			}
		})
	}
}

func TestRedirectWSFollowChain(t *testing.T) {
	// test that redirect following is recursive
	redirectee := internal.StartMockServer(t)
	middle := redirectServer("http://"+redirectee.Endpoint, 302)
	middleURL, err := url.Parse(middle.URL)
	if err != nil {
		// unlikely
		t.Fatal(err)
	}
	redirector := redirectServer("http://"+middleURL.Host, 302)

	var conn atomic.Value
	redirectee.OnWSConnect = func(c *websocket.Conn) {
		conn.Store(c)
	}

	// Start an OpAMP/WebSocket client.
	var connected int64
	var connectErr atomic.Value
	mr := mockRedirect(t, 2, nil)
	settings := types.StartSettings{
		Callbacks: types.Callbacks{
			OnConnect: func(ctx context.Context) {
				atomic.StoreInt64(&connected, 1)
			},
			OnConnectFailed: func(ctx context.Context, err error) {
				if err != websocket.ErrBadHandshake {
					connectErr.Store(err)
				}
			},
			CheckRedirect: mr.CheckRedirect,
		},
	}
	reURL, err := url.Parse(redirector.URL)
	if err != nil {
		// unlikely
		t.Fatal(err)
	}
	reURL.Scheme = "ws"
	settings.OpAMPServerURL = reURL.String()
	client := NewWebSocket(nil)
	startClient(t, settings, client)

	// Wait for connection to be established.
	eventually(t, func() bool {
		return conn.Load() != nil || connectErr.Load() != nil || client.lastInternalErr.Load() != nil
	})

	assert.True(t, connectErr.Load() == nil)

	// Stop the client.
	err = client.Stop(context.Background())
	assert.NoError(t, err)
}

func TestHandlesStopBeforeStart(t *testing.T) {
	client := NewWebSocket(nil)
	require.Error(t, client.Stop(context.Background()))
}

func TestPerformsClosingHandshake(t *testing.T) {
	srv := internal.StartMockServer(t)
	var wsConn *websocket.Conn
	connected := make(chan struct{})
	closed := make(chan struct{})
	acked := make(chan struct{})

	srv.OnWSConnect = func(conn *websocket.Conn) {
		wsConn = conn
		connected <- struct{}{}
	}

	client := NewWebSocket(nil)
	startClient(t, types.StartSettings{
		OpAMPServerURL: srv.GetHTTPTestServer().URL,
	}, client)

	select {
	case <-connected:
	case <-time.After(2 * time.Second):
		require.Fail(t, "Connection never established")
	}

	eventually(t, func() bool {
		client.connMutex.RLock()
		conn := client.conn
		client.connMutex.RUnlock()
		return conn != nil
	})

	{
		defhandler := client.conn.CloseHandler()
		client.conn.SetCloseHandler(func(code int, msg string) error {
			close(acked)
			return defhandler(code, msg)
		})
	}

	defHandler := wsConn.CloseHandler()

	wsConn.SetCloseHandler(func(code int, _ string) error {
		require.Equal(t, websocket.CloseNormalClosure, code, "Client sent non-normal closing code")

		err := defHandler(code, "")
		closed <- struct{}{}
		return err
	})

	client.Stop(context.Background())

	select {
	case <-closed:
		select {
		case <-acked:
		case <-time.After(2 * time.Second):
			require.Fail(t, "Close connection without waiting for a close message from server")
		}
	case <-time.After(2 * time.Second):
		require.Fail(t, "Connection never closed")
	}
}

func TestHandlesSlowCloseMessageFromServer(t *testing.T) {
	srv := internal.StartMockServer(t)
	var wsConn *websocket.Conn
	connected := make(chan struct{})
	closed := make(chan struct{})

	srv.OnWSConnect = func(conn *websocket.Conn) {
		wsConn = conn
		connected <- struct{}{}
	}

	client := NewWebSocket(nil)
	client.connShutdownTimeout = 100 * time.Millisecond
	startClient(t, types.StartSettings{
		OpAMPServerURL: srv.GetHTTPTestServer().URL,
	}, client)

	select {
	case <-connected:
	case <-time.After(2 * time.Second):
		require.Fail(t, "Connection never established")
	}

	require.Eventually(t, func() bool {
		client.connMutex.RLock()
		conn := client.conn
		client.connMutex.RUnlock()
		return conn != nil
	}, 2*time.Second, 250*time.Millisecond)

	defHandler := wsConn.CloseHandler()

	wsConn.SetCloseHandler(func(code int, _ string) error {
		require.Equal(t, websocket.CloseNormalClosure, code, "Client sent non-normal closing code")

		time.Sleep(200 * time.Millisecond)
		err := defHandler(code, "")
		closed <- struct{}{}
		return err
	})

	client.Stop(context.Background())

	select {
	case <-closed:
	case <-time.After(1 * time.Second):
		require.Fail(t, "Connection never closed")
	}
}

func TestHandlesNoCloseMessageFromServer(t *testing.T) {
	srv := internal.StartMockServer(t)
	var wsConn *websocket.Conn
	connected := make(chan struct{})
	closed := make(chan struct{})

	srv.OnWSConnect = func(conn *websocket.Conn) {
		wsConn = conn
		connected <- struct{}{}
	}

	client := NewWebSocket(nil)
	client.connShutdownTimeout = 100 * time.Millisecond
	startClient(t, types.StartSettings{
		OpAMPServerURL: srv.GetHTTPTestServer().URL,
	}, client)

	select {
	case <-connected:
	case <-time.After(2 * time.Second):
		require.Fail(t, "Connection never established")
	}

	require.Eventually(t, func() bool {
		client.connMutex.RLock()
		conn := client.conn
		client.connMutex.RUnlock()
		return conn != nil
	}, 2*time.Second, 250*time.Millisecond)

	wsConn.SetCloseHandler(func(code int, _ string) error {
		// Don't send close message
		return nil
	})

	go func() {
		client.Stop(context.Background())
		closed <- struct{}{}
	}()

	select {
	case <-closed:
	case <-time.After(1 * time.Second):
		require.Fail(t, "Connection never closed")
	}
}

func TestHandlesConnectionError(t *testing.T) {
	srv := internal.StartMockServer(t)
	var wsConn *websocket.Conn
	connected := make(chan struct{})

	srv.OnWSConnect = func(conn *websocket.Conn) {
		wsConn = conn
		connected <- struct{}{}
	}

	client := NewWebSocket(nil)
	startClient(t, types.StartSettings{
		OpAMPServerURL: srv.GetHTTPTestServer().URL,
	}, client)

	select {
	case <-connected:
	case <-time.After(2 * time.Second):
		require.Fail(t, "Connection never established")
	}

	require.Eventually(t, func() bool {
		client.connMutex.RLock()
		conn := client.conn
		client.connMutex.RUnlock()
		return conn != nil
	}, 2*time.Second, 250*time.Millisecond)

	// Write an invalid message to the connection. The client
	// will take this as an error and reconnect to the server.
	writer, err := wsConn.NextWriter(websocket.BinaryMessage)
	require.NoError(t, err)
	n, err := writer.Write([]byte{99, 1, 2, 3, 4, 5})
	require.NoError(t, err)
	require.Equal(t, 6, n)
	err = writer.Close()
	require.NoError(t, err)

	select {
	case <-connected:
	case <-time.After(2 * time.Second):
		require.Fail(t, "Connection never re-established")
	}

	require.Eventually(t, func() bool {
		client.connMutex.RLock()
		conn := client.conn
		client.connMutex.RUnlock()
		return conn != nil
	}, 2*time.Second, 250*time.Millisecond)

	err = client.Stop(context.Background())
	require.NoError(t, err)
}

func TestWSSenderReportsAvailableComponents(t *testing.T) {
	testCases := []struct {
		desc                string
		availableComponents *protobufs.AvailableComponents
	}{
		{
			desc:                "Does not report AvailableComponents",
			availableComponents: nil,
		},
		{
			desc:                "Reports AvailableComponents",
			availableComponents: generateTestAvailableComponents(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			srv := internal.StartMockServer(t)

			var firstMsg atomic.Bool
			var conn atomic.Value
			srv.OnWSConnect = func(c *websocket.Conn) {
				conn.Store(c)
				firstMsg.Store(true)
			}
			var msgCount atomic.Int64
			srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
				if firstMsg.Load() {
					msgCount.Add(1)
					firstMsg.Store(false)
					resp := &protobufs.ServerToAgent{
						InstanceUid: msg.InstanceUid,
					}

					if tc.availableComponents != nil {
						availableComponents := msg.GetAvailableComponents()
						require.NotNil(t, availableComponents)
						require.Nil(t, availableComponents.GetComponents())
						require.Equal(t, tc.availableComponents.GetHash(), availableComponents.GetHash())

						resp.Flags = uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportAvailableComponents)
					} else {
						require.Nil(t, msg.GetAvailableComponents())
					}

					return resp
				}
				msgCount.Add(1)
				if tc.availableComponents != nil {
					availableComponents := msg.GetAvailableComponents()
					require.NotNil(t, availableComponents)
					require.Equal(t, tc.availableComponents.GetHash(), availableComponents.GetHash())
					require.Equal(t, tc.availableComponents.GetComponents(), availableComponents.GetComponents())
				} else {
					require.Error(t, errors.New("should not receive a second message when ReportsAvailableComponents is disabled"))
				}

				return nil
			}

			// Start an OpAMP/WebSocket client.
			settings := types.StartSettings{
				OpAMPServerURL: "ws://" + srv.Endpoint,
			}
			client := NewWebSocket(nil)

			if tc.availableComponents != nil {
				settings.Capabilities = protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents
				client.SetAvailableComponents(tc.availableComponents)
			}

			startClient(t, settings, client)

			// Wait for connection to be established.
			eventually(t, func() bool { return conn.Load() != nil })

			if tc.availableComponents != nil {
				assert.Eventually(t, func() bool {
					return msgCount.Load() >= 2
				}, 5*time.Second, 10*time.Millisecond)
			} else {
				assert.Never(t, func() bool {
					return msgCount.Load() >= 2
				}, 50*time.Millisecond, 10*time.Millisecond)
			}

			// Stop the client.
			err := client.Stop(context.Background())
			assert.NoError(t, err)
		})
	}
}
