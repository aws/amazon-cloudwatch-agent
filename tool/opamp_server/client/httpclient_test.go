package client

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opamp-go/client/internal"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
)

func TestHTTPPolling(t *testing.T) {
	// Start a Server.
	srv := internal.StartMockServer(t)
	var rcvCounter int64
	srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
		if msg == nil {
			t.Error("unexpected nil msg")
			return nil
		}
		assert.EqualValues(t, rcvCounter, msg.SequenceNum)
		atomic.AddInt64(&rcvCounter, 1)
		return nil
	}

	// Start a client.
	settings := types.StartSettings{}
	settings.OpAMPServerURL = "http://" + srv.Endpoint
	client := NewHTTP(nil)
	prepareClient(t, &settings, client)

	// Shorten the polling interval to speed up the test.
	client.sender.SetPollingInterval(time.Millisecond * 10)

	assert.NoError(t, client.Start(context.Background(), settings))

	// Verify that status report is delivered.
	eventually(t, func() bool { return atomic.LoadInt64(&rcvCounter) == 1 })

	// Verify that status report is delivered again. Polling should ensure this.
	eventually(t, func() bool { return atomic.LoadInt64(&rcvCounter) == 2 })

	// Shutdown the Server.
	srv.Close()

	// Shutdown the client.
	err := client.Stop(context.Background())
	assert.NoError(t, err)
}

func TestHTTPClientCompression(t *testing.T) {
	srv := internal.StartMockServer(t)
	var reqCounter int64

	srv.OnRequest = func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&reqCounter, 1)
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
		reader, err := gzip.NewReader(r.Body)
		assert.NoError(t, err)
		body, err := io.ReadAll(reader)
		assert.NoError(t, err)
		_ = r.Body.Close()
		var response protobufs.AgentToServer
		err = proto.Unmarshal(body, &response)
		assert.NoError(t, err)
		assert.Equal(t, response.AgentDescription.IdentifyingAttributes, []*protobufs.KeyValue{
			{
				Key:   "service.name",
				Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "otelcol"}},
			},
			{
				Key:   "service.namespace",
				Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "default"}},
			},
			{
				Key:   "service.instance.id",
				Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "443e083c-b968-4428-a281-6867bd280e0d"}},
			},
			{
				Key:   "service.version",
				Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "1.0.0"}},
			},
		})
		w.WriteHeader(http.StatusOK)
	}

	settings := types.StartSettings{EnableCompression: true}
	settings.OpAMPServerURL = "http://" + srv.Endpoint
	client := NewHTTP(nil)
	prepareClient(t, &settings, client)

	client.sender.SetPollingInterval(time.Millisecond * 10)

	assert.NoError(t, client.Start(context.Background(), settings))

	eventually(t, func() bool { return atomic.LoadInt64(&reqCounter) == 1 })

	srv.Close()

	err := client.Stop(context.Background())
	assert.NoError(t, err)
}

func TestHTTPClientSetPollingInterval(t *testing.T) {
	// Start a Server.
	srv := internal.StartMockServer(t)
	var rcvCounter int64
	srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
		if msg == nil {
			t.Error("unexpected nil msg")
			return nil
		}
		assert.EqualValues(t, rcvCounter, msg.SequenceNum)
		atomic.AddInt64(&rcvCounter, 1)
		return nil
	}

	// Start a client.
	settings := types.StartSettings{}
	settings.OpAMPServerURL = "http://" + srv.Endpoint
	client := NewHTTP(nil)
	client.SetPollingInterval(100 * time.Millisecond)
	prepareClient(t, &settings, client)

	assert.NoError(t, client.Start(context.Background(), settings))

	// Verify that status report is delivered.
	eventually(t, func() bool { return atomic.LoadInt64(&rcvCounter) == 1 })

	// Verify that status report is delivered again. no call is made for next 100ms
	assert.Eventually(t, func() bool { return atomic.LoadInt64(&rcvCounter) == 2 }, 5*time.Second, 100*time.Millisecond)

	// Shutdown the Server.
	srv.Close()

	// Shutdown the client.
	err := client.Stop(context.Background())
	assert.NoError(t, err)
}

func TestHTTPClientStartWithHeartbeatInterval(t *testing.T) {
	tests := []struct {
		name             string
		enableHeartbeat  bool
		expectHeartbeats bool
	}{
		{"client enable heartbeat", true, true},
		{"client disable heartbeat", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Start a Server.
			srv := internal.StartMockServer(t)
			var rcvCounter int64
			srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
				if msg == nil {
					t.Error("unexpected nil msg")
					return nil
				}
				assert.EqualValues(t, rcvCounter, msg.SequenceNum)
				atomic.AddInt64(&rcvCounter, 1)
				return nil
			}

			// Start a client.
			heartbeat := 10 * time.Millisecond
			settings := types.StartSettings{
				OpAMPServerURL:    "http://" + srv.Endpoint,
				HeartbeatInterval: &heartbeat,
			}
			if tt.enableHeartbeat {
				settings.Capabilities = protobufs.AgentCapabilities_AgentCapabilities_ReportsHeartbeat
			}
			client := NewHTTP(nil)
			prepareClient(t, &settings, client)

			assert.NoError(t, client.Start(context.Background(), settings))

			// Verify that status report is delivered.
			eventually(t, func() bool { return atomic.LoadInt64(&rcvCounter) == 1 })

			if tt.expectHeartbeats {
				assert.Eventually(t, func() bool { return atomic.LoadInt64(&rcvCounter) >= 2 }, 5*time.Second, 10*time.Millisecond)
			} else {
				assert.Never(t, func() bool { return atomic.LoadInt64(&rcvCounter) >= 2 }, 50*time.Millisecond, 10*time.Millisecond)
			}

			// Shutdown the Server.
			srv.Close()

			// Shutdown the client.
			err := client.Stop(context.Background())
			assert.NoError(t, err)
		})
	}
}

func TestHTTPClientStartWithZeroHeartbeatInterval(t *testing.T) {
	srv := internal.StartMockServer(t)

	// Start a client.
	heartbeat := 0 * time.Millisecond
	settings := types.StartSettings{
		OpAMPServerURL:    "http://" + srv.Endpoint,
		HeartbeatInterval: &heartbeat,
		Capabilities:      protobufs.AgentCapabilities_AgentCapabilities_ReportsHeartbeat,
	}
	client := NewHTTP(nil)
	prepareClient(t, &settings, client)

	// Zero heartbeat interval is invalid for http client.
	assert.Error(t, client.Start(context.Background(), settings))

	// Shutdown the Server.
	srv.Close()
}

func mockRedirectHTTP(t testing.TB, viaLen int, err error) *checkRedirectMock {
	m := &checkRedirectMock{
		t:      t,
		viaLen: viaLen,
		http:   true,
	}
	m.On("CheckRedirect", mock.Anything, mock.Anything, mock.Anything).Return(err)
	return m
}

func TestRedirectHTTP(t *testing.T) {
	redirectee := internal.StartMockServer(t)
	tests := []struct {
		Name         string
		Redirector   *httptest.Server
		ExpError     bool
		MockRedirect *checkRedirectMock
	}{
		{
			Name:       "simple redirect",
			Redirector: redirectServer("http://"+redirectee.Endpoint, 302),
		},
		{
			Name:         "check redirect",
			Redirector:   redirectServer("http://"+redirectee.Endpoint, 302),
			MockRedirect: mockRedirectHTTP(t, 1, nil),
		},
		{
			Name:         "check redirect returns error",
			Redirector:   redirectServer("http://"+redirectee.Endpoint, 302),
			MockRedirect: mockRedirectHTTP(t, 1, errors.New("hello")),
			ExpError:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var connectErr atomic.Value
			var connected atomic.Value

			settings := &types.StartSettings{
				Callbacks: types.Callbacks{
					OnConnect: func(ctx context.Context) {
						connected.Store(1)
					},
					OnConnectFailed: func(ctx context.Context, err error) {
						connectErr.Store(err)
					},
				},
			}
			if test.MockRedirect != nil {
				settings.Callbacks = types.Callbacks{
					OnConnect: func(ctx context.Context) {
						connected.Store(1)
					},
					OnConnectFailed: func(ctx context.Context, err error) {
						connectErr.Store(err)
					},
					CheckRedirect: test.MockRedirect.CheckRedirect,
				}
			}
			reURL, _ := url.Parse(test.Redirector.URL) // err can't be non-nil
			settings.OpAMPServerURL = reURL.String()
			client := NewHTTP(nil)
			prepareClient(t, settings, client)

			err := client.Start(context.Background(), *settings)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Stop(context.Background())
			// Wait for connection to be established.
			eventually(t, func() bool {
				return connected.Load() != nil || connectErr.Load() != nil
			})
			if test.ExpError && connectErr.Load() == nil {
				t.Error("expected non-nil error")
			} else if err := connectErr.Load(); !test.ExpError && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHTTPReportsAvailableComponents(t *testing.T) {
	testCases := []struct {
		desc                string
		capabilities        protobufs.AgentCapabilities
		availableComponents *protobufs.AvailableComponents
		startErr            error
	}{
		{
			desc:                "Does not report AvailableComponents",
			availableComponents: nil,
		},
		{
			desc:                "Reports AvailableComponents",
			capabilities:        protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents,
			availableComponents: generateTestAvailableComponents(),
		},
		{
			desc:         "No AvailableComponents on Start() despite capability",
			capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents,
			startErr:     internal.ErrAvailableComponentsMissing,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Start a Server.
			srv := internal.StartMockServer(t)
			var rcvCounter atomic.Uint64
			srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
				assert.EqualValues(t, rcvCounter.Load(), msg.SequenceNum)
				rcvCounter.Add(1)
				time.Sleep(50 * time.Millisecond)
				if rcvCounter.Load() == 1 {
					resp := &protobufs.ServerToAgent{
						InstanceUid: msg.InstanceUid,
					}

					if tc.availableComponents != nil {
						// the first message received should contain just the available component hash
						availableComponents := msg.GetAvailableComponents()
						require.NotNil(t, availableComponents)
						require.Nil(t, availableComponents.GetComponents())
						require.Equal(t, tc.availableComponents.GetHash(), availableComponents.GetHash())

						// add the flag asking for the full available component state to the response
						resp.Flags = uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportAvailableComponents)
					} else {
						require.Nil(t, msg.GetAvailableComponents())
					}

					return resp
				}

				if rcvCounter.Load() == 2 {
					if tc.availableComponents != nil {
						// the second message received should contain the full component state
						availableComponents := msg.GetAvailableComponents()
						require.NotNil(t, availableComponents)
						require.Equal(t, tc.availableComponents.GetComponents(), availableComponents.GetComponents())
						require.Equal(t, tc.availableComponents.GetHash(), availableComponents.GetHash())
					} else {
						require.Nil(t, msg.GetAvailableComponents())
					}

					return nil
				}

				// all subsequent messages should not have any available components
				require.Nil(t, msg.GetAvailableComponents())
				return nil
			}

			// Start a client.
			settings := types.StartSettings{}
			settings.OpAMPServerURL = "http://" + srv.Endpoint
			settings.Capabilities = tc.capabilities

			client := NewHTTP(nil)
			client.SetAvailableComponents(tc.availableComponents)
			prepareClient(t, &settings, client)

			startErr := client.Start(context.Background(), settings)
			if tc.startErr == nil {
				assert.NoError(t, startErr)
			} else {
				assert.ErrorIs(t, startErr, tc.startErr)
				return
			}

			// Verify that status report is delivered.
			eventually(t, func() bool {
				return rcvCounter.Load() == 1
			})

			if tc.availableComponents != nil {
				// Verify that status report is delivered again. Polling should ensure this.
				eventually(t, func() bool {
					return rcvCounter.Load() == 2
				})
			} else {
				// Verify that no second status report is delivered (polling is too infrequent for this to happen in 50ms)
				assert.Never(t, func() bool {
					return rcvCounter.Load() == 2
				}, 50*time.Millisecond, 10*time.Millisecond)
			}

			// Verify that no third status report is delivered (polling is too infrequent for this to happen in 50ms)
			assert.Never(t, func() bool {
				return rcvCounter.Load() == 3
			}, 50*time.Millisecond, 10*time.Millisecond)

			// Shutdown the Server.
			srv.Close()

			// Shutdown the client.
			err := client.Stop(context.Background())
			assert.NoError(t, err)
		})
	}
}
