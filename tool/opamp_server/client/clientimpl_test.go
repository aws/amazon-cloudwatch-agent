package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opamp-go/client/internal"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/internal/testhelpers"
	"github.com/open-telemetry/opamp-go/protobufs"
)

const retryAfterHTTPHeader = "Retry-After"

var coreCapabilities = protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus

func createAgentDescr() *protobufs.AgentDescription {
	agentDescr := &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
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
		},
		NonIdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key:   "host.name",
				Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "somehost"}},
			},
		},
	}
	return agentDescr
}

func testClients(t *testing.T, f func(t *testing.T, client OpAMPClient)) {
	// Run the test defined by f() for WebSocket and HTTP clients.
	tests := []struct {
		name   string
		client OpAMPClient
	}{
		{
			name:   "http",
			client: NewHTTP(nil),
		},
		{
			name:   "ws",
			client: NewWebSocket(nil),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f(t, test.client)
		})
	}
}

func TestConnectInvalidURL(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		settings := types.StartSettings{
			OpAMPServerURL: ":not a url",
		}

		err := client.Start(context.Background(), settings)
		assert.Error(t, err)
	})
}

func eventually(t *testing.T, f func() bool) {
	assert.Eventually(t, f, 5*time.Second, 10*time.Millisecond)
}

func genNewInstanceUid(t *testing.T) types.InstanceUid {
	uid, err := uuid.NewV7()
	require.NoError(t, err)
	b, err := uid.MarshalBinary()
	require.NoError(t, err)
	return types.InstanceUid(b)
}

func prepareSettings(t *testing.T, settings *types.StartSettings, c OpAMPClient) {
	// Autogenerate instance id.
	settings.InstanceUid = genNewInstanceUid(t)

	// Make sure correct URL scheme is used, based on the type of the OpAMP client.
	u, err := url.Parse(settings.OpAMPServerURL)
	require.NoError(t, err)
	switch c.(type) {
	case *httpClient:
		if settings.TLSConfig != nil {
			u.Scheme = "https"
		} else {
			u.Scheme = "http"
		}
	case *wsClient:
		if settings.TLSConfig != nil {
			u.Scheme = "wss"
		} else {
			u.Scheme = "ws"
		}
	}
	settings.OpAMPServerURL = u.String()
}

func prepareClient(t *testing.T, settings *types.StartSettings, c OpAMPClient) {
	prepareSettings(t, settings, c)
	err := c.SetAgentDescription(createAgentDescr())
	assert.NoError(t, err)
	if settings.Capabilities != 0 {
		c.SetCapabilities(&settings.Capabilities)
	} else {
		c.SetCapabilities(&coreCapabilities)
	}
}

func startClient(t *testing.T, settings types.StartSettings, client OpAMPClient) {
	prepareClient(t, &settings, client)
	err := client.Start(context.Background(), settings)
	assert.NoError(t, err)
}

// Create start settings that point to a non-existing Server.
func createNoServerSettings() types.StartSettings {
	return types.StartSettings{
		OpAMPServerURL: "ws://" + testhelpers.GetAvailableLocalAddress(),
	}
}

func TestConnectNoServer(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		startClient(t, createNoServerSettings(), client)
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestInvalidInstanceId(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		settings := createNoServerSettings()
		prepareClient(t, &settings, client)
		settings.InstanceUid = types.InstanceUid{}

		err := client.Start(context.Background(), settings)
		assert.Error(t, err)
	})
}

func TestOnConnectFail(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		var connectErr atomic.Value
		settings := createNoServerSettings()
		settings.Callbacks = types.Callbacks{
			OnConnectFailed: func(ctx context.Context, err error) {
				connectErr.Store(err)
			},
		}

		startClient(t, settings, client)

		eventually(t, func() bool { return connectErr.Load() != nil })

		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestStartStarted(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		settings := createNoServerSettings()
		startClient(t, settings, client)

		// Try to start again.
		err := client.Start(context.Background(), settings)
		assert.Error(t, err)

		err = client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestStopWithoutStart(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		err := client.Stop(context.Background())
		assert.Error(t, err)
	})
}

func TestStopCancellation(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		startClient(t, createNoServerSettings(), client)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := client.Stop(ctx)
		if err != nil {
			assert.ErrorIs(t, err, context.Canceled)
		}
	})
}

func TestStartNoDescription(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		setErr := client.SetCapabilities(&coreCapabilities)
		assert.NoError(t, setErr)
		settings := createNoServerSettings()
		prepareSettings(t, &settings, client)
		err := client.Start(context.Background(), settings)
		assert.EqualValues(t, err, internal.ErrAgentDescriptionMissing)
	})
}

func TestStartNoCapabilities(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		setErr := client.SetAgentDescription(createAgentDescr())
		require.NoError(t, setErr)
		settings := createNoServerSettings()
		prepareSettings(t, &settings, client)
		err := client.Start(context.Background(), settings)
		assert.NoError(t, err, "no error should be found until the client rejects empty capabilities")
	})
}

func TestSetCapabilitiesErrorsBeforeStart(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		capabilities := coreCapabilities | protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents
		setCapabilityErr := client.SetCapabilities(&capabilities)
		assert.Error(t, setCapabilityErr)
		assert.Contains(t, setCapabilityErr.Error(), "AvailableComponents is nil")
	})
}

func TestSetCapabilitiesErrorsDuringStart(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		capabilities := coreCapabilities | protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents
		settings := types.StartSettings{
			Capabilities: capabilities,
		}
		prepareClient(t, &settings, client)

		// Client --->
		startErr := client.Start(context.Background(), settings)
		assert.Error(t, startErr)
		assert.Contains(t, startErr.Error(), "AvailableComponents is nil")
	})
}

func TestSetInvalidAgentDescription(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		settings := createNoServerSettings()
		prepareSettings(t, &settings, client)
		err := client.SetAgentDescription(nil)
		assert.EqualValues(t, err, internal.ErrAgentDescriptionMissing)
		err = client.SetAgentDescription(&protobufs.AgentDescription{})
		assert.EqualValues(t, err, internal.ErrAgentDescriptionNoAttributes)
	})
}

func TestConnectWithServer(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a server.
		srv := internal.StartMockServer(t)

		// Start a client.
		var connected int64
		settings := types.StartSettings{
			Callbacks: types.Callbacks{
				OnConnect: func(ctx context.Context) {
					atomic.StoreInt64(&connected, 1)
				},
			},
		}
		settings.OpAMPServerURL = "ws://" + srv.Endpoint
		startClient(t, settings, client)

		// Wait for connection to be established.
		eventually(t, func() bool { return atomic.LoadInt64(&connected) != 0 })

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestConnectWithServer503(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a server.
		var connectionAttempts int64
		srv := internal.StartMockServer(t)
		srv.OnRequest = func(w http.ResponseWriter, r *http.Request) {
			atomic.StoreInt64(&connectionAttempts, 1)

			// Always respond with an error to the client.
			w.Header().Set(retryAfterHTTPHeader, "30")
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		// Start a client.
		var clientConnected int64
		var connectErr atomic.Value
		settings := types.StartSettings{
			Callbacks: types.Callbacks{
				OnConnect: func(ctx context.Context) {
					atomic.StoreInt64(&clientConnected, 1)
					assert.Fail(t, "Client should not be able to connect")
				},
				OnConnectFailed: func(ctx context.Context, err error) {
					connectErr.Store(err)
				},
			},
		}
		settings.OpAMPServerURL = "ws://" + srv.Endpoint
		startClient(t, settings, client)

		// Wait for connection to fail.
		eventually(t, func() bool { return connectErr.Load() != nil })

		assert.EqualValues(t, 1, atomic.LoadInt64(&connectionAttempts))
		assert.EqualValues(t, 0, atomic.LoadInt64(&clientConnected))

		// Shutdown the Server.
		srv.Close()
		_ = client.Stop(context.Background())
	})
}

func TestConnectWithHeader(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a server.
		srv := internal.StartMockServer(t)
		var conn atomic.Value
		srv.OnConnect = func(r *http.Request) {
			authHdr := r.Header.Get("Authorization")
			assert.EqualValues(t, "Bearer 12345678", authHdr)
			userAgentHdr := r.Header.Get("User-Agent")
			assert.EqualValues(t, "custom-agent/1.0", userAgentHdr)
			conn.Store(true)
		}

		header := http.Header{}
		header.Set("Authorization", "Bearer 12345678")
		header.Set("User-Agent", "custom-agent/1.0")

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
			Header:         header,
		}
		startClient(t, settings, client)

		// Wait for connection to be established.
		eventually(t, func() bool { return conn.Load() != nil })

		// Shutdown the Server and the client.
		srv.Close()
		_ = client.Stop(context.Background())
	})
}

func TestConnectWithHeaderFunc(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a server.
		srv := internal.StartMockServer(t)
		var conn atomic.Value
		srv.OnConnect = func(r *http.Request) {
			authHdr := r.Header.Get("Authorization")
			assert.EqualValues(t, "Bearer 12345678", authHdr)
			userAgentHdr := r.Header.Get("User-Agent")
			assert.EqualValues(t, "custom-agent/1.0", userAgentHdr)
			conn.Store(true)
		}

		hf := func(header http.Header) http.Header {
			header.Set("Authorization", "Bearer 12345678")
			header.Set("User-Agent", "custom-agent/1.0")
			return header
		}

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
			HeaderFunc:     hf,
		}
		startClient(t, settings, client)

		// Wait for connection to be established.
		eventually(t, func() bool { return conn.Load() != nil })

		// Shutdown the Server and the client.
		srv.Close()
		_ = client.Stop(context.Background())
	})
}

func TestConnectWithHeaderAndHeaderFunc(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a server.
		srv := internal.StartMockServer(t)
		var conn atomic.Value
		srv.OnConnect = func(r *http.Request) {
			authHdr := r.Header.Get("Authorization")
			assert.EqualValues(t, "Bearer 12345678", authHdr)
			userAgentHdr := r.Header.Get("User-Agent")
			assert.EqualValues(t, "custom-agent/1.0", userAgentHdr)
			conn.Store(true)
		}

		baseHeader := http.Header{}
		baseHeader.Set("User-Agent", "custom-agent/1.0")

		hf := func(header http.Header) http.Header {
			header.Set("Authorization", "Bearer 12345678")
			return header
		}

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
			Header:         baseHeader,
			HeaderFunc:     hf,
		}
		startClient(t, settings, client)

		// Wait for connection to be established.
		eventually(t, func() bool { return conn.Load() != nil })

		// Shutdown the Server and the client.
		srv.Close()
		_ = client.Stop(context.Background())
	})
}

func TestConnectWithTLS(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a server.
		srv := internal.StartTLSMockServer(t)
		var conn atomic.Value
		srv.OnConnect = func(r *http.Request) {
			conn.Store(true)
		}

		certs := rootCAs(t, srv.GetHTTPTestServer())

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "wss://" + srv.Endpoint,
			TLSConfig: &tls.Config{
				RootCAs: certs,
			},
		}

		startClient(t, settings, client)

		// Wait for connection to be established.
		eventually(t, func() bool { return conn.Load() != nil })

		// Shutdown the Server and the client.
		srv.Close()
		_ = client.Stop(context.Background())
	})
}

func rootCAs(t *testing.T, s *httptest.Server) *x509.CertPool {
	certs := x509.NewCertPool()
	for _, c := range s.TLS.Certificates {
		roots, err := x509.ParseCertificates(c.Certificate[len(c.Certificate)-1])
		if err != nil {
			t.Fatalf("error parsing server's root cert: %v", err)
		}
		for _, root := range roots {
			certs.AddCert(root)
		}
	}
	return certs
}

func createRemoteConfig() *protobufs.AgentRemoteConfig {
	return &protobufs.AgentRemoteConfig{
		Config: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{},
		},
		ConfigHash: []byte{1, 2, 3, 4},
	}
}

func TestFirstStatusReport(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		remoteConfig := createRemoteConfig()

		// Start a Server.
		srv := internal.StartMockServer(t)
		srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			assert.EqualValues(t, 0, msg.SequenceNum)
			return &protobufs.ServerToAgent{
				InstanceUid:  msg.InstanceUid,
				RemoteConfig: remoteConfig,
			}
		}

		// Start a client.
		var connected, remoteConfigReceived int64
		settings := types.StartSettings{
			Callbacks: types.Callbacks{
				OnConnect: func(ctx context.Context) {
					atomic.AddInt64(&connected, 1)
				},
				OnMessage: func(ctx context.Context, msg *types.MessageData) {
					// Verify that the client received exactly the remote config that
					// the Server sent.
					assert.True(t, proto.Equal(remoteConfig, msg.RemoteConfig))
					atomic.AddInt64(&remoteConfigReceived, 1)
				},
			},
			Capabilities: protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig,
		}
		settings.OpAMPServerURL = "ws://" + srv.Endpoint
		startClient(t, settings, client)

		// Wait for connection to be established.
		eventually(t, func() bool { return atomic.LoadInt64(&connected) != 0 })

		// Wait to receive remote config.
		eventually(t, func() bool { return atomic.LoadInt64(&remoteConfigReceived) != 0 })

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestIncludesDetailsOnReconnect(t *testing.T) {
	srv := internal.StartMockServer(t)

	seqNum := 0

	var receivedDetails int64
	srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
		assert.EqualValues(t, seqNum, msg.SequenceNum)
		seqNum++

		// Track when we receive AgentDescription
		if msg.AgentDescription != nil {
			atomic.AddInt64(&receivedDetails, 1)
		}

		return &protobufs.ServerToAgent{
			InstanceUid: msg.InstanceUid,
		}
	}

	var connected int64
	settings := types.StartSettings{
		Callbacks: types.Callbacks{
			OnConnect: func(ctx context.Context) {
				atomic.AddInt64(&connected, 1)
			},
		},
	}

	settings.OpAMPServerURL = "ws://" + srv.Endpoint
	client := NewWebSocket(nil)
	startClient(t, settings, client)

	eventually(t, func() bool { return atomic.LoadInt64(&connected) == 1 })
	eventually(t, func() bool { return atomic.LoadInt64(&receivedDetails) == 1 })

	// close the Agent connection. expect it to reconnect and send details again.
	require.NotNil(t, client.conn)
	err := client.conn.Close()
	assert.NoError(t, err)

	eventually(t, func() bool { return atomic.LoadInt64(&connected) == 2 })
	eventually(t, func() bool { return atomic.LoadInt64(&receivedDetails) == 2 })

	err = client.Stop(context.Background())
	assert.NoError(t, err)
}

func createEffectiveConfig() *protobufs.EffectiveConfig {
	cfg := &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"key": {},
			},
		},
	}
	return cfg
}

func TestSetEffectiveConfig(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a server.
		srv := internal.StartMockServer(t)
		var rcvConfig atomic.Value
		srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			if msg.EffectiveConfig != nil {
				rcvConfig.Store(msg.EffectiveConfig)
			}
			return nil
		}

		// Start a client.
		sendConfig := createEffectiveConfig()
		settings := types.StartSettings{
			Callbacks: types.Callbacks{
				GetEffectiveConfig: func(ctx context.Context) (*protobufs.EffectiveConfig, error) {
					return sendConfig, nil
				},
			},
			Capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig,
		}
		settings.OpAMPServerURL = "ws://" + srv.Endpoint
		prepareClient(t, &settings, client)

		require.NoError(t, client.Start(context.Background(), settings))

		// Verify config is delivered.
		eventually(
			t,
			func() bool {
				return rcvConfig.Load() != nil &&
					proto.Equal(sendConfig, rcvConfig.Load().(*protobufs.EffectiveConfig))
			},
		)

		// Now change the config.
		sendConfig.ConfigMap.ConfigMap["key2"] = &protobufs.AgentConfigFile{}
		updateErr := client.UpdateEffectiveConfig(context.Background())
		require.NoError(t, updateErr)

		// Verify change is delivered.
		eventually(
			t,
			func() bool {
				return rcvConfig.Load() != nil &&
					proto.Equal(sendConfig, rcvConfig.Load().(*protobufs.EffectiveConfig))
			},
		)

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestSetAgentDescription(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a Server.
		srv := internal.StartMockServer(t)
		var rcvAgentDescr atomic.Value
		srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			if msg.AgentDescription != nil {
				rcvAgentDescr.Store(msg.AgentDescription)
			}
			return nil
		}

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
		}
		prepareClient(t, &settings, client)

		clientAgentDescr := createAgentDescr()
		assert.NoError(t, client.SetAgentDescription(clientAgentDescr))

		assert.NoError(t, client.Start(context.Background(), settings))

		// Verify it is delivered.
		eventually(
			t,
			func() bool {
				agentDescr, ok := rcvAgentDescr.Load().(*protobufs.AgentDescription)
				if !ok || agentDescr == nil {
					return false
				}
				return proto.Equal(clientAgentDescr, agentDescr)
			},
		)

		// Now change again.
		clientAgentDescr.NonIdentifyingAttributes = []*protobufs.KeyValue{
			{
				Key:   "os.name",
				Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "linux"}},
			},
		}
		assert.NoError(t, client.SetAgentDescription(clientAgentDescr))

		// Verify change is delivered.
		eventually(
			t,
			func() bool {
				agentDescr := rcvAgentDescr.Load().(*protobufs.AgentDescription)
				if agentDescr == nil {
					return false
				}
				return proto.Equal(clientAgentDescr, agentDescr)
			},
		)

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestAgentIdentification(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a server.
		srv := internal.StartMockServer(t)
		newInstanceUid := genNewInstanceUid(t)
		var rcvAgentInstanceUid atomic.Value
		srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			if msg.Flags&uint64(protobufs.AgentToServerFlags_AgentToServerFlags_RequestInstanceUid) == 1 {
				newInstanceUid = genNewInstanceUid(t)
				rcvAgentInstanceUid.Store(newInstanceUid[:])
				return &protobufs.ServerToAgent{
					InstanceUid: msg.InstanceUid,
					AgentIdentification: &protobufs.AgentIdentification{
						// If the RequestInstanceUid flag was set, populate this field.
						NewInstanceUid: newInstanceUid[:],
					},
				}
			}
			rcvAgentInstanceUid.Store(msg.InstanceUid)
			// Start by sending just the old instance ID.
			return &protobufs.ServerToAgent{
				InstanceUid: msg.InstanceUid,
			}
		}

		// Start a client.
		settings := types.StartSettings{}
		settings.OpAMPServerURL = "ws://" + srv.Endpoint
		prepareClient(t, &settings, client)

		oldInstanceUid := settings.InstanceUid
		assert.NoError(t, client.Start(context.Background(), settings))

		// First, Server gets the original instanceId
		eventually(
			t,
			func() bool {
				instanceUid, ok := rcvAgentInstanceUid.Load().([]byte)
				if !ok {
					return false
				}
				return types.InstanceUid(instanceUid) == oldInstanceUid
			},
		)

		// Send a dummy message
		_ = client.SetAgentDescription(createAgentDescr())

		// Verify that the old instance id was not overridden
		eventually(
			t,
			func() bool {
				instanceUid, ok := rcvAgentInstanceUid.Load().([]byte)
				if !ok {
					return false
				}
				return types.InstanceUid(instanceUid) == oldInstanceUid
			},
		)

		// Set the flags to request a new ID.
		client.SetFlags(protobufs.AgentToServerFlags_AgentToServerFlags_RequestInstanceUid)

		// When it was sent, the new instance uid should have been used, which should
		// have been observed by the Server
		eventually(
			t,
			func() bool {
				instanceUid, ok := rcvAgentInstanceUid.Load().([]byte)
				if !ok {
					return false
				}
				return types.InstanceUid(instanceUid) == newInstanceUid
			},
		)

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestServerOfferConnectionSettings(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		hash := []byte{1, 2, 3}
		opampSettings := &protobufs.OpAMPConnectionSettings{DestinationEndpoint: "http://opamp.com"}
		metricsSettings := &protobufs.TelemetryConnectionSettings{DestinationEndpoint: "http://metrics.com"}
		tracesSettings := &protobufs.TelemetryConnectionSettings{DestinationEndpoint: "http://traces.com"}
		logsSettings := &protobufs.TelemetryConnectionSettings{DestinationEndpoint: "http://logs.com"}
		otherSettings := &protobufs.OtherConnectionSettings{DestinationEndpoint: "http://other.com"}

		var rcvStatus int64
		// Start a Server.
		srv := internal.StartMockServer(t)
		srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			if msg != nil {
				atomic.AddInt64(&rcvStatus, 1)

				return &protobufs.ServerToAgent{
					ConnectionSettings: &protobufs.ConnectionSettingsOffers{
						Hash:       hash,
						Opamp:      opampSettings,
						OwnMetrics: metricsSettings,
						OwnTraces:  tracesSettings,
						OwnLogs:    logsSettings,
						OtherConnections: map[string]*protobufs.OtherConnectionSettings{
							"other": otherSettings,
						},
					},
				}
			}
			return nil
		}

		var gotOpampSettings int64
		var gotOwnSettings int64
		var gotOtherSettings int64

		// Start a client.
		settings := types.StartSettings{
			Callbacks: types.Callbacks{
				OnMessage: func(ctx context.Context, msg *types.MessageData) {
					assert.True(t, proto.Equal(metricsSettings, msg.OwnMetricsConnSettings))
					assert.True(t, proto.Equal(tracesSettings, msg.OwnTracesConnSettings))
					assert.True(t, proto.Equal(logsSettings, msg.OwnLogsConnSettings))
					atomic.AddInt64(&gotOwnSettings, 1)

					assert.Len(t, msg.OtherConnSettings, 1)
					assert.True(t, proto.Equal(otherSettings, msg.OtherConnSettings["other"]))
					atomic.AddInt64(&gotOtherSettings, 1)
				},

				OnOpampConnectionSettings: func(
					ctx context.Context, settings *protobufs.OpAMPConnectionSettings,
				) error {
					assert.True(t, proto.Equal(opampSettings, settings))
					atomic.AddInt64(&gotOpampSettings, 1)
					return nil
				},
			},
			Capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnTraces |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnMetrics |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnLogs |
				protobufs.AgentCapabilities_AgentCapabilities_AcceptsOtherConnectionSettings |
				protobufs.AgentCapabilities_AgentCapabilities_AcceptsOpAMPConnectionSettings,
		}
		settings.OpAMPServerURL = "ws://" + srv.Endpoint
		prepareClient(t, &settings, client)

		assert.NoError(t, client.Start(context.Background(), settings))

		eventually(t, func() bool { return atomic.LoadInt64(&gotOpampSettings) == 1 })
		eventually(t, func() bool { return atomic.LoadInt64(&gotOwnSettings) == 1 })
		eventually(t, func() bool { return atomic.LoadInt64(&gotOtherSettings) == 1 })
		eventually(t, func() bool { return atomic.LoadInt64(&rcvStatus) == 1 })

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestClientRequestConnectionSettings(t *testing.T) {
	testClients(
		t, func(t *testing.T, client OpAMPClient) {
			opampSettings := &protobufs.OpAMPConnectionSettings{DestinationEndpoint: "http://opamp.com"}

			var srvReceivedRequest int64
			// Start a Server.
			srv := internal.StartMockServer(t)
			srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
				if msg != nil && msg.ConnectionSettingsRequest != nil {
					atomic.AddInt64(&srvReceivedRequest, 1)
					return &protobufs.ServerToAgent{
						ConnectionSettings: &protobufs.ConnectionSettingsOffers{
							Opamp: opampSettings,
						},
					}
				}
				return nil
			}

			var clientGotOpampSettings int64

			// Start a client.
			settings := types.StartSettings{
				Callbacks: types.Callbacks{
					OnOpampConnectionSettings: func(
						ctx context.Context, settings *protobufs.OpAMPConnectionSettings,
					) error {
						assert.True(t, proto.Equal(opampSettings, settings))
						atomic.AddInt64(&clientGotOpampSettings, 1)
						return nil
					},
				},
				Capabilities: protobufs.AgentCapabilities_AgentCapabilities_AcceptsOpAMPConnectionSettings,
			}
			settings.OpAMPServerURL = "ws://" + srv.Endpoint
			prepareClient(t, &settings, client)

			assert.NoError(t, client.Start(context.Background(), settings))

			client.RequestConnectionSettings(&protobufs.ConnectionSettingsRequest{})

			// Wait until server receives the request.
			eventually(t, func() bool { return atomic.LoadInt64(&srvReceivedRequest) == 1 })

			// Wait until client receives the server's response.
			eventually(t, func() bool { return atomic.LoadInt64(&clientGotOpampSettings) == 1 })

			// Shutdown the Server.
			srv.Close()

			// Shutdown the client.
			err := client.Stop(context.Background())
			assert.NoError(t, err)
		},
	)
}

func TestReportAgentDescription(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a Server.
		srv := internal.StartMockServer(t)
		srv.EnableExpectMode()

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
			Capabilities:   protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig,
		}
		prepareClient(t, &settings, client)

		// Client --->
		assert.NoError(t, client.Start(context.Background(), settings))

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			assert.EqualValues(t, 0, msg.SequenceNum)
			// The first status report after Start must have full AgentDescription.
			assert.True(t, proto.Equal(client.AgentDescription(), msg.AgentDescription))
			return &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid}
		})

		// Client --->
		// Trigger a status report.
		_ = client.UpdateEffectiveConfig(context.Background())

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			// The status report must have compressed AgentDescription.
			assert.Nil(t, msg.AgentDescription)

			assert.EqualValues(t, 1, msg.SequenceNum)

			// Ask client for full AgentDescription.
			return &protobufs.ServerToAgent{
				InstanceUid: msg.InstanceUid,
				Flags:       uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState),
			}
		})

		// Server has requested the client to report, so there will be another message
		// coming to the Server.
		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			assert.EqualValues(t, 2, msg.SequenceNum)
			// The status report must again have full AgentDescription
			// because the Server asked for it.
			assert.True(t, proto.Equal(client.AgentDescription(), msg.AgentDescription))
			return &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid}
		})

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestReportAgentHealth(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a Server.
		srv := internal.StartMockServer(t)
		srv.EnableExpectMode()

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
			Capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth,
		}
		prepareClient(t, &settings, client)

		assert.Error(t, client.SetHealth(nil))

		sendHealth := &protobufs.ComponentHealth{
			Healthy:           true,
			StartTimeUnixNano: 123,
			LastError:         "bad error",
		}
		assert.NoError(t, client.SetHealth(sendHealth))

		// Client --->
		assert.NoError(t, client.Start(context.Background(), settings))

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			assert.EqualValues(t, 0, msg.SequenceNum)
			// The first status report after Start must have the Health.
			assert.True(t, proto.Equal(sendHealth, msg.Health))
			return &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid}
		})

		// Client --->
		// Trigger a status report.
		_ = client.UpdateEffectiveConfig(context.Background())

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			// The status report must have compressed Health.
			assert.Nil(t, msg.Health)

			assert.EqualValues(t, 1, msg.SequenceNum)

			// Ask client for full AgentDescription.
			return &protobufs.ServerToAgent{
				InstanceUid: msg.InstanceUid,
				Flags:       uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState),
			}
		})

		// Server has requested the client to report, so there will be another message
		// coming to the Server.
		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			assert.EqualValues(t, 2, msg.SequenceNum)
			// The status report must again have full Health
			// because the Server asked for it.
			assert.True(t, proto.Equal(sendHealth, msg.Health))
			return &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid}
		})

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestReportEffectiveConfig(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a Server.
		srv := internal.StartMockServer(t)
		srv.EnableExpectMode()

		clientEffectiveConfig := createEffectiveConfig()

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
			Callbacks: types.Callbacks{
				GetEffectiveConfig: func(ctx context.Context) (*protobufs.EffectiveConfig, error) {
					return clientEffectiveConfig, nil
				},
			},
		}
		prepareClient(t, &settings, client)

		// Client --->
		assert.NoError(t, client.Start(context.Background(), settings))

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			assert.EqualValues(t, 0, msg.SequenceNum)
			// The first status report after Start must have full EffectiveConfig.
			assert.True(t, proto.Equal(clientEffectiveConfig, msg.EffectiveConfig))
			return &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid}
		})

		// Client --->
		// Trigger another status report for example by setting AgentDescription.
		_ = client.SetAgentDescription(client.AgentDescription())

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			// The status report must have compressed EffectiveConfig.
			assert.Nil(t, msg.EffectiveConfig)

			assert.EqualValues(t, 1, msg.SequenceNum)

			// Ask client for full AgentDescription.
			return &protobufs.ServerToAgent{
				InstanceUid: msg.InstanceUid,
				Flags:       uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState),
			}
		})

		// Server has requested the client to report, so there will be another message.
		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			assert.EqualValues(t, 2, msg.SequenceNum)
			// The status report must again have full EffectiveConfig
			// because Server asked for it.
			assert.True(t, proto.Equal(clientEffectiveConfig, msg.EffectiveConfig))
			return &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid}
		})

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func verifyRemoteConfigUpdate(t *testing.T, successCase bool, expectStatus *protobufs.RemoteConfigStatus) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a Server.
		srv := internal.StartMockServer(t)
		srv.EnableExpectMode()

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
			Callbacks: types.Callbacks{
				OnMessage: func(ctx context.Context, msg *types.MessageData) {
					if msg.RemoteConfig != nil {
						if successCase {
							client.SetRemoteConfigStatus(
								&protobufs.RemoteConfigStatus{
									LastRemoteConfigHash: msg.RemoteConfig.ConfigHash,
									Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
								})
						} else {
							client.SetRemoteConfigStatus(
								&protobufs.RemoteConfigStatus{
									LastRemoteConfigHash: msg.RemoteConfig.ConfigHash,
									Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED,
									ErrorMessage:         "cannot update remote config",
								})
						}
					}
				},
			},
			Capabilities: protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig,
		}
		prepareClient(t, &settings, client)

		// Client --->
		assert.NoError(t, client.Start(context.Background(), settings))

		remoteCfg := createRemoteConfig()
		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			assert.EqualValues(t, 0, msg.SequenceNum)
			// Send the remote config to the Agent.
			return &protobufs.ServerToAgent{
				InstanceUid:  msg.InstanceUid,
				RemoteConfig: remoteCfg,
			}
		})

		// The Agent will try to apply the remote config and will send the status
		// report about it back to the Server.

		var firstConfigStatus *protobufs.RemoteConfigStatus

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			assert.EqualValues(t, 1, msg.SequenceNum)
			// Verify that the remote config status is as expected.
			status := msg.RemoteConfigStatus
			assert.EqualValues(t, expectStatus.Status, status.Status)
			assert.Equal(t, expectStatus.ErrorMessage, status.ErrorMessage)
			assert.EqualValues(t, remoteCfg.ConfigHash, status.LastRemoteConfigHash)

			firstConfigStatus = proto.Clone(status).(*protobufs.RemoteConfigStatus)

			return &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid}
		})

		// Client --->
		// Trigger another status report by setting AgentDescription.
		_ = client.SetAgentDescription(client.AgentDescription())

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			// This time the RemoteConfigStatus field must be unset. This is expected
			// as compression in OpAMP.
			require.Nil(t, msg.RemoteConfigStatus)
			assert.EqualValues(t, 2, msg.SequenceNum)

			return &protobufs.ServerToAgent{
				InstanceUid: msg.InstanceUid,
				// Ask client to report full status.
				Flags: uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState),
			}
		})

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			assert.EqualValues(t, 3, msg.SequenceNum)
			// Exact same full status must be present again.
			status := msg.RemoteConfigStatus
			assert.True(t, proto.Equal(status, firstConfigStatus))

			return &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid}
		})

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestRemoteConfigUpdate(t *testing.T) {
	tests := []struct {
		name           string
		success        bool
		expectedStatus *protobufs.RemoteConfigStatus
	}{
		{
			name:    "success",
			success: true,
			expectedStatus: &protobufs.RemoteConfigStatus{
				Status:       protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
				ErrorMessage: "",
			},
		},
		{
			name:    "fail",
			success: false,
			expectedStatus: &protobufs.RemoteConfigStatus{
				Status:       protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED,
				ErrorMessage: "cannot update remote config",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifyRemoteConfigUpdate(t, test.success, test.expectedStatus)
		})
	}
}

type packageTestCase struct {
	name                string
	errorOnCallback     bool
	available           *protobufs.PackagesAvailable
	expectedStatus      *protobufs.PackageStatuses
	expectedFileContent map[string][]byte
	expectedSignature   map[string][]byte
	expectedError       string

	// expectedTemporaryStatuses is a slice used by a test case to check if a non-final package status occurs.
	// When a PackageStatuses message is added to the slice only the Packages name and Status will be checked
	// If they are successfully observed then the corresponding entry in observedTemporaryStatuses will be marked as true
	expectedTemporaryStatuses []*protobufs.PackageStatuses
	observedTemporaryStatuses []bool
}

const packageUpdateErrorMsg = "cannot update packages"

func assertPackageStatus(t *testing.T,
	testCase packageTestCase,
	msg *protobufs.AgentToServer,
) (*protobufs.ServerToAgent, bool) {
	expectedStatusReceived := false
	testCase.observedTemporaryStatuses = make([]bool, len(testCase.expectedTemporaryStatuses))

	status := msg.PackageStatuses
	if status == nil {
		// PackageStatuses is not yet reported, keep waiting.
		return nil, false
	}
	assert.EqualValues(t, testCase.expectedStatus.ServerProvidedAllPackagesHash, status.ServerProvidedAllPackagesHash)

	if testCase.expectedError != "" {
		assert.EqualValues(t, testCase.expectedError, status.ErrorMessage)
		return &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid}, true
	}

	// Verify individual package statuses.
	for name, pkgExpected := range testCase.expectedStatus.Packages {
		pkgStatus := status.Packages[name]
		if pkgStatus == nil {
			// Package status not yet included in the report.
			continue
		}
		switch pkgStatus.Status {
		case protobufs.PackageStatusEnum_PackageStatusEnum_InstallFailed:
			assert.Contains(t, pkgStatus.ErrorMessage, pkgExpected.ErrorMessage)

		case protobufs.PackageStatusEnum_PackageStatusEnum_Installed:
			assert.EqualValues(t, pkgExpected.AgentHasHash, pkgStatus.AgentHasHash)
			assert.EqualValues(t, pkgExpected.AgentHasVersion, pkgStatus.AgentHasVersion)
			assert.Empty(t, pkgStatus.ErrorMessage)
		default:
			assert.Empty(t, pkgStatus.ErrorMessage)
		}
		assert.EqualValues(t, pkgExpected.ServerOfferedHash, pkgStatus.ServerOfferedHash)
		assert.EqualValues(t, pkgExpected.ServerOfferedVersion, pkgStatus.ServerOfferedVersion)

		if pkgStatus.Status == pkgExpected.Status {
			expectedStatusReceived = true
			assert.Len(t, status.Packages, len(testCase.available.Packages))
		}
	}

	for i, tempStatus := range testCase.expectedTemporaryStatuses {
		for name, pack := range tempStatus.Packages {
			obsPackage, ok := status.Packages[name]
			if !ok {
				// name does not match
				continue
			}
			if pack.Status == obsPackage.Status {
				testCase.observedTemporaryStatuses[i] = true
			}
		}
	}

	return &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid}, expectedStatusReceived
}

func verifyUpdatePackages(t *testing.T, testCase packageTestCase) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a Server.
		srv := internal.StartMockServer(t)
		srv.EnableExpectMode()

		localPackageState := internal.NewInMemPackagesStore()

		var syncerDoneCh <-chan struct{}

		// Prepare a callback that returns either success or failure.
		onMessageFunc := func(ctx context.Context, msg *types.MessageData) {
			if msg.PackageSyncer != nil {
				if testCase.errorOnCallback {
					client.SetPackageStatuses(&protobufs.PackageStatuses{
						ServerProvidedAllPackagesHash: msg.PackagesAvailable.AllPackagesHash,
						ErrorMessage:                  packageUpdateErrorMsg,
					})
				} else {
					syncerDoneCh = msg.PackageSyncer.Done()
					err := msg.PackageSyncer.Sync(ctx)
					require.NoError(t, err)
				}
			}
		}

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
			Callbacks: types.Callbacks{
				OnMessage: onMessageFunc,
			},
			PackagesStateProvider: localPackageState,
			Capabilities: protobufs.AgentCapabilities_AgentCapabilities_AcceptsPackages |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsPackageStatuses,
		}
		prepareClient(t, &settings, client)

		// Client --->
		assert.NoError(t, client.Start(context.Background(), settings))

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			assert.EqualValues(t, 0, msg.SequenceNum)
			// Send the packages to the Agent.
			return &protobufs.ServerToAgent{
				InstanceUid:       msg.InstanceUid,
				PackagesAvailable: testCase.available,
			}
		})

		// The Agent will try to install the packages and will send the status
		// report about it back to the Server.

		// ---> Server
		// Wait for the expected package statuses to be received.
		srv.EventuallyExpect("full PackageStatuses", func(msg *protobufs.AgentToServer) (*protobufs.ServerToAgent,
			bool,
		) {
			return assertPackageStatus(t, testCase, msg)
		})

		if syncerDoneCh != nil {
			// Wait until all syncing is done.
			<-syncerDoneCh

			for pkgName, receivedContent := range localPackageState.GetContent() {
				expectedContent := testCase.expectedFileContent[pkgName]
				assert.EqualValues(t, expectedContent, receivedContent)

				actualSignature := localPackageState.GetSignature()[pkgName]
				expectedSignature := testCase.expectedSignature[pkgName]
				assert.EqualValues(t, expectedSignature, actualSignature)
			}

			for i, ok := range testCase.observedTemporaryStatuses {
				assert.Truef(t, ok, "expected to observe temporary status %#v", testCase.expectedTemporaryStatuses[i])
			}
		}

		// Client --->
		// Trigger another status report by setting AgentDescription.
		_ = client.SetAgentDescription(client.AgentDescription())

		// ---> Server
		srv.EventuallyExpect("compressed PackageStatuses",
			func(msg *protobufs.AgentToServer) (*protobufs.ServerToAgent, bool) {
				// Ensure that compressed status is received.
				compressedReceived := msg.PackageStatuses == nil

				response := &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid}

				if compressedReceived {
					// Ask for full report again.
					response.Flags = uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState)
				} else {
					// Keep triggering status report by setting AgentDescription
					// until the compressed PackageStatuses arrives.
					_ = client.SetAgentDescription(client.AgentDescription())
				}

				return response, compressedReceived
			})

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

// Downloadable package file constants.
const packageFileURL = "/validfile.pkg"

var packageFileContent = []byte("Package File Content")

var optionalAuthHeaders = protobufs.Header{Key: "Authorization", Value: "Basic YWxhZGRpbjpvcGVuc2VzYW1l"}

func createDownloadSrv(t *testing.T) *httptest.Server {
	m := http.NewServeMux()
	m.HandleFunc(packageFileURL,
		func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(optionalAuthHeaders.GetKey()) != "" {
				assert.Equal(t, r.Header.Get(optionalAuthHeaders.GetKey()), optionalAuthHeaders.GetValue())
			}

			w.WriteHeader(http.StatusOK)
			_, err := w.Write(packageFileContent)
			assert.NoError(t, err)
		})

	srv := httptest.NewServer(m)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	endpoint := u.Host
	testhelpers.WaitForEndpoint(endpoint)

	return srv
}

func createPackageTestCase(name string, downloadSrv *httptest.Server) packageTestCase {
	return packageTestCase{
		name:            name,
		errorOnCallback: false,
		available: &protobufs.PackagesAvailable{
			Packages: map[string]*protobufs.PackageAvailable{
				"package1": {
					Type:    protobufs.PackageType_PackageType_TopLevel,
					Version: "1.0.0",
					File: &protobufs.DownloadableFile{
						DownloadUrl: downloadSrv.URL + packageFileURL,
						ContentHash: []byte{4, 5},
						Signature:   []byte{6, 7},
					},
					Hash: []byte{1, 2, 3},
				},
			},
			AllPackagesHash: []byte{1, 2, 3, 4, 5},
		},

		expectedStatus: &protobufs.PackageStatuses{
			Packages: map[string]*protobufs.PackageStatus{
				"package1": {
					Name:                 "package1",
					AgentHasVersion:      "1.0.0",
					AgentHasHash:         []byte{1, 2, 3},
					ServerOfferedVersion: "1.0.0",
					ServerOfferedHash:    []byte{1, 2, 3},
					Status:               protobufs.PackageStatusEnum_PackageStatusEnum_Installed,
					ErrorMessage:         "",
				},
			},
			ServerProvidedAllPackagesHash: []byte{1, 2, 3, 4, 5},
		},

		expectedFileContent: map[string][]byte{
			"package1": packageFileContent,
		},

		expectedSignature: map[string][]byte{
			"package1": {6, 7},
		},
	}
}

func TestUpdatePackages(t *testing.T) {
	downloadSrv := createDownloadSrv(t)
	defer downloadSrv.Close()

	// A success case.
	var tests []packageTestCase
	tests = append(tests, createPackageTestCase("success", downloadSrv))

	// A case when downloading the file fails because the URL is incorrect.
	notFound := createPackageTestCase("downloadable file not found", downloadSrv)
	notFound.available.Packages["package1"].File.DownloadUrl = downloadSrv.URL + "/notfound"
	notFound.expectedStatus.Packages["package1"].Status = protobufs.PackageStatusEnum_PackageStatusEnum_InstallFailed
	notFound.expectedStatus.Packages["package1"].ErrorMessage = "cannot download"
	tests = append(tests, notFound)

	// A case when OnPackagesAvailable callback returns an error.
	errorOnCallback := createPackageTestCase("error on callback", downloadSrv)
	errorOnCallback.expectedError = packageUpdateErrorMsg
	errorOnCallback.errorOnCallback = true
	tests = append(tests, errorOnCallback)

	// Check that the downloading status is sent
	downloading := createPackageTestCase("download status set", downloadSrv)
	downloading.expectedTemporaryStatuses = append(downloading.expectedTemporaryStatuses, &protobufs.PackageStatuses{
		Packages: map[string]*protobufs.PackageStatus{
			"package1": {
				Name:   "package1",
				Status: protobufs.PackageStatusEnum_PackageStatusEnum_Downloading,
			},
		},
	})
	tests = append(tests, downloading)

	// A case where we send optional headers
	withHeaders := createPackageTestCase("with optional HTTP headers", downloadSrv)
	withHeaders.available.Packages["package1"].File.Headers = &protobufs.Headers{Headers: []*protobufs.Header{&optionalAuthHeaders}}
	tests = append(tests, withHeaders)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifyUpdatePackages(t, test)
		})
	}
}

func TestMissingCapabilities(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a server.
		srv := internal.StartMockServer(t)
		srv.EnableExpectMode()

		// Start a client.
		settings := types.StartSettings{
			Callbacks: types.Callbacks{
				OnMessage: func(ctx context.Context, msg *types.MessageData) {
					// These fields must not be set since we did not define the capabilities to accept them.
					assert.Nil(t, msg.RemoteConfig)
					assert.Nil(t, msg.OwnLogsConnSettings)
					assert.Nil(t, msg.OwnMetricsConnSettings)
					assert.Nil(t, msg.OwnTracesConnSettings)
					assert.Nil(t, msg.OtherConnSettings)
					assert.Nil(t, msg.PackagesAvailable)
				},
				OnOpampConnectionSettings: func(
					ctx context.Context, settings *protobufs.OpAMPConnectionSettings,
				) error {
					assert.Fail(t, "should not be called since capability is not set to accept it")
					return nil
				},
			},
		}
		settings.OpAMPServerURL = "ws://" + srv.Endpoint
		prepareClient(t, &settings, client)

		require.NoError(t, client.Start(context.Background(), settings))

		// Change the config.
		err := client.UpdateEffectiveConfig(context.Background())

		assert.ErrorIs(t, err, internal.ErrReportsEffectiveConfigNotSet)

		remoteCfg := createRemoteConfig()
		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			opampSettings := &protobufs.OpAMPConnectionSettings{DestinationEndpoint: "http://opamp.com"}
			metricsSettings := &protobufs.TelemetryConnectionSettings{DestinationEndpoint: "http://metrics.com"}
			tracesSettings := &protobufs.TelemetryConnectionSettings{DestinationEndpoint: "http://traces.com"}
			logsSettings := &protobufs.TelemetryConnectionSettings{DestinationEndpoint: "http://logs.com"}
			otherSettings := &protobufs.OtherConnectionSettings{DestinationEndpoint: "http://other.com"}
			hash := []byte{1, 2, 3}

			return &protobufs.ServerToAgent{
				InstanceUid:  msg.InstanceUid,
				RemoteConfig: remoteCfg,
				ConnectionSettings: &protobufs.ConnectionSettingsOffers{
					Hash:       hash,
					Opamp:      opampSettings,
					OwnMetrics: metricsSettings,
					OwnTraces:  tracesSettings,
					OwnLogs:    logsSettings,
					OtherConnections: map[string]*protobufs.OtherConnectionSettings{
						"other": otherSettings,
					},
				},
				PackagesAvailable: &protobufs.PackagesAvailable{
					Packages:        map[string]*protobufs.PackageAvailable{},
					AllPackagesHash: []byte{1, 2, 3, 4, 5},
				},
			}
		})

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err = client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestMissingPackagesStateProvider(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a client.
		settings := types.StartSettings{
			Callbacks: types.Callbacks{},
			Capabilities: protobufs.AgentCapabilities_AgentCapabilities_AcceptsPackages |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsPackageStatuses,
		}
		prepareClient(t, &settings, client)
		assert.ErrorIs(t, client.Start(context.Background(), settings), internal.ErrPackagesStateProviderNotSet)

		// Start a client.
		localPackageState := internal.NewInMemPackagesStore()
		settings = types.StartSettings{
			Callbacks:             types.Callbacks{},
			PackagesStateProvider: localPackageState,
			Capabilities:          protobufs.AgentCapabilities_AgentCapabilities_AcceptsPackages,
		}
		prepareClient(t, &settings, client)

		assert.ErrorIs(t, client.Start(context.Background(), settings), internal.ErrAcceptsPackagesNotSet)

		// Start a client.
		settings = types.StartSettings{
			Callbacks:             types.Callbacks{},
			PackagesStateProvider: localPackageState,
			Capabilities:          protobufs.AgentCapabilities_AgentCapabilities_ReportsPackageStatuses,
		}
		prepareClient(t, &settings, client)

		assert.ErrorIs(t, client.Start(context.Background(), settings), internal.ErrAcceptsPackagesNotSet)
	})
}

func TestOfferUpdatedVersion(t *testing.T) {
	downloadSrv := createDownloadSrv(t)
	defer downloadSrv.Close()

	testCase := createPackageTestCase("offer new version", downloadSrv)

	testClients(t, func(t *testing.T, client OpAMPClient) {
		localPackageState := internal.NewInMemPackagesStore()
		srv := internal.StartMockServer(t)
		srv.EnableExpectMode()

		onMessageFunc := func(ctx context.Context, msg *types.MessageData) {
			if msg.PackageSyncer != nil {
				msg.PackageSyncer.Done()
				err := msg.PackageSyncer.Sync(ctx)
				require.NoError(t, err)
			}
		}

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
			Callbacks: types.Callbacks{
				OnMessage: onMessageFunc,
			},
			PackagesStateProvider: localPackageState,
			Capabilities: protobufs.AgentCapabilities_AgentCapabilities_AcceptsPackages |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsPackageStatuses,
		}
		prepareClient(t, &settings, client)

		// Client --->
		assert.NoError(t, client.Start(context.Background(), settings))

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			assert.EqualValues(t, 0, msg.SequenceNum)
			// Send the packages to the Agent.
			return &protobufs.ServerToAgent{
				InstanceUid:       msg.InstanceUid,
				PackagesAvailable: testCase.available,
			}
		})

		// The Agent will try to install the packages and will send the status
		// report about it back to the Server.
		// ---> Server
		// Wait for the expected package statuses to be received.
		srv.EventuallyExpect("full PackageStatuses", func(msg *protobufs.AgentToServer) (*protobufs.ServerToAgent,
			bool,
		) {
			return assertPackageStatus(t, testCase, msg)
		})

		newByte := []byte{45}
		testCase.available.Packages["package1"].Version = "1.0.1"
		testCase.available.AllPackagesHash = append(testCase.available.AllPackagesHash, newByte...)
		testCase.expectedStatus.Packages["package1"].AgentHasVersion = "1.0.1"
		testCase.expectedStatus.Packages["package1"].ServerOfferedVersion = "1.0.1"
		testCase.expectedStatus.ServerProvidedAllPackagesHash = append(testCase.expectedStatus.ServerProvidedAllPackagesHash, newByte...)

		_ = client.SetHealth(&protobufs.ComponentHealth{})

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			// Send the packages to the Agent.
			return &protobufs.ServerToAgent{
				InstanceUid:       msg.InstanceUid,
				PackagesAvailable: testCase.available,
			}
		})
		// The Agent will try to install the packages and will send the status
		// report about it back to the Server.

		// ---> Server
		// Wait for the expected package statuses to be received.
		srv.EventuallyExpect("full PackageStatuses updated version", func(msg *protobufs.AgentToServer) (*protobufs.ServerToAgent,
			bool,
		) {
			return assertPackageStatus(t, testCase, msg)
		})

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestSetCapabilities(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a Server.
		srv := internal.StartMockServer(t)
		srv.EnableExpectMode()

		var clientRcvCustomMessage atomic.Value

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
			Callbacks: types.Callbacks{
				OnMessage: func(ctx context.Context, msg *types.MessageData) {
					clientRcvCustomMessage.Store(msg.CustomMessage)
				},
			},
			Capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig,
		}
		prepareClient(t, &settings, client)

		// Client --->
		assert.NoError(t, client.Start(context.Background(), settings))

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			assert.EqualValues(t, 0, msg.SequenceNum)
			// The first status report after Start must have the ReportsStatus.
			assert.True(t, protobufs.AgentCapabilities(msg.Capabilities)&protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus != 0)
			// The first status report after Start must have the ReportsEffectiveConfig.
			assert.True(t, protobufs.AgentCapabilities(msg.Capabilities)&protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig != 0)
			// The first status report after Start must not  have the AcceptsRemoteConfig.
			assert.True(t, protobufs.AgentCapabilities(msg.Capabilities)&protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig == 0)
			return &protobufs.ServerToAgent{
				InstanceUid: msg.InstanceUid,
			}
		})

		newCapabilities := protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig |
			protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus
		err := client.SetCapabilities(&newCapabilities)
		assert.NoError(t, err)

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			// Check ReportsStatus is still true.
			assert.True(t, protobufs.AgentCapabilities(msg.Capabilities)&protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus != 0)
			// ReportsEffectiveConfig should no longer be present.
			assert.True(t, protobufs.AgentCapabilities(msg.Capabilities)&protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig != 0)
			// AcceptsRemoteConfig should now be present
			assert.True(t, protobufs.AgentCapabilities(msg.Capabilities)&protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig != 0)

			// Send a custom message response and ask client for full state again.
			return &protobufs.ServerToAgent{
				InstanceUid: msg.InstanceUid,
				Flags:       uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState),
			}
		})

		newCapabilities = protobufs.AgentCapabilities_AgentCapabilities_AcceptsRestartCommand |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig |
			protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus
		newSetErr := client.SetCapabilities(&newCapabilities)
		assert.NoError(t, newSetErr)

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			// Check ReportsStatus is still true.
			assert.True(t, protobufs.AgentCapabilities(msg.Capabilities)&protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus != 0)
			// ReportsEffectiveConfig should  present.
			assert.True(t, protobufs.AgentCapabilities(msg.Capabilities)&protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig != 0)
			// AcceptsRemoteConfig should now be present
			assert.True(t, protobufs.AgentCapabilities(msg.Capabilities)&protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig != 0)
			// AcceptsRestartCommand should now be present
			assert.True(t, protobufs.AgentCapabilities(msg.Capabilities)&protobufs.AgentCapabilities_AgentCapabilities_AcceptsRestartCommand != 0)
			// Send a custom message response and ask client for full state again.
			return &protobufs.ServerToAgent{
				InstanceUid: msg.InstanceUid,
				Flags:       uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState),
			}
		})

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err = client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestReportCustomCapabilities(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a Server.
		srv := internal.StartMockServer(t)
		srv.EnableExpectMode()

		var clientRcvCustomMessage atomic.Value

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
			Callbacks: types.Callbacks{
				OnMessage: func(ctx context.Context, msg *types.MessageData) {
					clientRcvCustomMessage.Store(msg.CustomMessage)
				},
			},
		}
		prepareClient(t, &settings, client)
		clientCustomCapabilities := &protobufs.CustomCapabilities{
			Capabilities: []string{"local.test.echo"},
		}
		client.SetCustomCapabilities(clientCustomCapabilities)

		// Client --->
		assert.NoError(t, client.Start(context.Background(), settings))

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			assert.EqualValues(t, 0, msg.SequenceNum)
			// The first status report after Start must have the CustomCapabilities.
			assert.True(t, proto.Equal(clientCustomCapabilities, msg.CustomCapabilities))
			return &protobufs.ServerToAgent{
				InstanceUid: msg.InstanceUid,
				CustomCapabilities: &protobufs.CustomCapabilities{
					Capabilities: []string{"local.test.echo"},
				},
			}
		})

		clientEchoRequest := &protobufs.CustomMessage{
			Capability: "local.test.echo",
			Type:       "request",
			Data:       []byte("data"),
		}
		serverEchoResponse := &protobufs.CustomMessage{
			Capability: "local.test.echo",
			Type:       "response",
			Data:       []byte("data"),
		}

		// Client --->
		// Send a custom message to the server
		_, err := client.SendCustomMessage(clientEchoRequest)
		assert.NoError(t, err)

		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			// CustomCapabilities must not be sent again.
			assert.Nil(t, msg.CustomCapabilities)

			assert.EqualValues(t, 1, msg.SequenceNum)

			// Send a custom message response and ask client for full state again.
			return &protobufs.ServerToAgent{
				InstanceUid:   msg.InstanceUid,
				Flags:         uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState),
				CustomMessage: serverEchoResponse,
			}
		})

		// Verify response received
		// Client --->
		eventually(
			t,
			func() bool {
				msg, ok := clientRcvCustomMessage.Load().(*protobufs.CustomMessage)
				if !ok || msg == nil {
					return false
				}
				return proto.Equal(serverEchoResponse, msg)
			},
		)

		// Server has requested the client to report, so there will be another message.
		// ---> Server
		srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			assert.EqualValues(t, 2, msg.SequenceNum)

			// CustomCapabilities should be sent since ReportFullState was requested
			assert.True(t, proto.Equal(clientCustomCapabilities, msg.CustomCapabilities))
			return &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid}
		})

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err = client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

// TestSendCustomMessage tests the SendCustomMessage method to ensure it returns errors
// appropriately.
func TestSendCustomMessage(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		settings := types.StartSettings{
			Callbacks: types.Callbacks{},
		}
		prepareClient(t, &settings, client)
		clientCustomCapabilities := &protobufs.CustomCapabilities{
			Capabilities: []string{"io.opentelemetry.supported"},
		}
		client.SetCustomCapabilities(clientCustomCapabilities)

		// Client --->
		assert.NoError(t, client.Start(context.Background(), settings))

		tests := []struct {
			name          string
			message       *protobufs.CustomMessage
			expectedError error
		}{
			{
				name:          "nil message is error",
				message:       nil,
				expectedError: types.ErrCustomMessageMissing,
			},
			{
				name: "unsupported message is error",
				message: &protobufs.CustomMessage{
					Capability: "io.opentelemetry.not-supported",
				},
				expectedError: types.ErrCustomCapabilityNotSupported,
			},
			{
				name: "supported capability is ok",
				message: &protobufs.CustomMessage{
					Capability: "io.opentelemetry.supported",
				},
				expectedError: nil,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				_, err := client.SendCustomMessage(test.message)
				assert.ErrorIs(t, err, test.expectedError)
			})
		}
	})
}

// TestCustomMessages tests the custom messages functionality.
func TestCustomMessages(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a Server.
		srv := internal.StartMockServer(t)
		var rcvCustomMessage atomic.Value
		srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			if msg.CustomMessage != nil {
				rcvCustomMessage.Store(msg.CustomMessage)
			}
			return nil
		}

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
		}
		prepareClient(t, &settings, client)
		clientCustomCapabilities := &protobufs.CustomCapabilities{
			Capabilities: []string{"local.test.example"},
		}
		client.SetCustomCapabilities(clientCustomCapabilities)

		assert.NoError(t, client.Start(context.Background(), settings))

		// Send message 1
		customMessage1 := &protobufs.CustomMessage{
			Capability: "local.test.example",
			Type:       "hello",
			Data:       []byte("test message 1"),
		}
		_, err := client.SendCustomMessage(customMessage1)
		assert.NoError(t, err)

		// Verify message 1 delivered
		eventually(
			t,
			func() bool {
				msg, ok := rcvCustomMessage.Load().(*protobufs.CustomMessage)
				if !ok || msg == nil {
					return false
				}
				return proto.Equal(customMessage1, msg)
			},
		)

		// Send message 2
		customMessage2 := &protobufs.CustomMessage{
			Capability: "local.test.example",
			Type:       "hello",
			Data:       []byte("test message 2"),
		}
		_, err = client.SendCustomMessage(customMessage2)
		assert.NoError(t, err)

		// Verify message 2 delivered
		eventually(
			t,
			func() bool {
				msg, ok := rcvCustomMessage.Load().(*protobufs.CustomMessage)
				if !ok || msg == nil {
					return false
				}
				return proto.Equal(customMessage2, msg)
			},
		)

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err = client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestSendCustomMessagePendingError(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a Server.
		srv := internal.StartMockServer(t)
		var rcvCustomMessage atomic.Value
		srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			if msg.CustomMessage != nil {
				rcvCustomMessage.Store(msg.CustomMessage)
			}
			return nil
		}

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
		}
		prepareClient(t, &settings, client)
		clientCustomCapabilities := &protobufs.CustomCapabilities{
			Capabilities: []string{"local.test.example"},
		}
		client.SetCustomCapabilities(clientCustomCapabilities)

		customMessage1 := &protobufs.CustomMessage{
			Capability: "local.test.example",
			Type:       "hello",
			Data:       []byte("test message 1"),
		}
		customMessage2 := &protobufs.CustomMessage{
			Capability: "local.test.example",
			Type:       "hello",
			Data:       []byte("test message 2"),
		}

		// Send a message to the unstarted client.
		_, err := client.SendCustomMessage(customMessage1)
		assert.NoError(t, err)

		// Sending another message immediately should fail with ErrCustomMessagePending.
		sendingChan, err := client.SendCustomMessage(customMessage2)
		assert.ErrorIs(t, err, types.ErrCustomMessagePending)
		assert.NotNil(t, sendingChan)

		// Start the client so we can start processing messages properly.
		assert.NoError(t, client.Start(context.Background(), settings))

		// Receive the first custom message
		eventually(
			t,
			func() bool {
				msg, ok := rcvCustomMessage.Load().(*protobufs.CustomMessage)
				if !ok || msg == nil {
					return false
				}
				return proto.Equal(customMessage1, msg)
			},
		)

		// Wait for the sending channel to be closed.
		<-sendingChan

		// Now sending the second message should work.
		_, err = client.SendCustomMessage(customMessage2)
		assert.NoError(t, err)

		// Receive the second custom message
		eventually(
			t,
			func() bool {
				msg, ok := rcvCustomMessage.Load().(*protobufs.CustomMessage)
				if !ok || msg == nil {
					return false
				}
				return proto.Equal(customMessage2, msg)
			},
		)
	})
}

// TestCustomMessagesSendAndWait tests the ability to send many custom messages in succession.
func TestCustomMessagesSendAndWait(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Start a Server.
		srv := internal.StartMockServer(t)

		numTestMessages := 10
		makeTestCustomMessage := func(i int) *protobufs.CustomMessage {
			return &protobufs.CustomMessage{
				Capability: "local.test.example",
				Type:       "hello",
				Data:       []byte(fmt.Sprintf("test message %d", i)),
			}
		}

		// The OnMessage callback puts CustomMessages on a channel to be verified
		rcvCustomMessages := make(chan *protobufs.CustomMessage)
		srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			if msg.CustomMessage != nil {
				select {
				case rcvCustomMessages <- msg.CustomMessage:
				case <-ctx.Done():
					assert.NoError(t, ctx.Err())
				}
			}
			return nil
		}

		// Start a client.
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
		}
		prepareClient(t, &settings, client)
		clientCustomCapabilities := &protobufs.CustomCapabilities{
			Capabilities: []string{"local.test.example"},
		}
		client.SetCustomCapabilities(clientCustomCapabilities)

		assert.NoError(t, client.Start(context.Background(), settings))

		// Send the test messages from a separate goroutine
		go func() {
			for i := 0; i < numTestMessages; i++ {
				messageSendingChannel, err := client.SendCustomMessage(makeTestCustomMessage(i))
				assert.NoError(t, err)

				// Wait for the sending channel to be closed before sending another message.
				// Without this the next SendCustomMessage will fail with ErrCustomMessagePending.
				select {
				case <-messageSendingChannel:
				case <-ctx.Done():
					assert.NoError(t, ctx.Err())
				}
			}
		}()

		// Verify messages received by the server
		for i := 0; i < numTestMessages; i++ {
			select {
			case msg := <-rcvCustomMessages:
				assert.True(t, proto.Equal(makeTestCustomMessage(i), msg))
			case <-ctx.Done():
				assert.NoError(t, ctx.Err())
			}
		}

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

// TestSetCustomCapabilities tests the ability for the client to change the set of custom capabilities that it supports.
func TestSetCustomCapabilities(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a Server.
		srv := internal.StartMockServer(t)
		var rcvCustomCapabilities atomic.Value
		var rcvCustomMessage atomic.Value
		srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			if msg.CustomMessage != nil {
				rcvCustomMessage.Store(msg.CustomMessage)
			}
			if msg.CustomCapabilities != nil {
				rcvCustomCapabilities.Store(msg.CustomCapabilities)
			}
			return nil
		}

		// Start a client with no support for CustomCapabilities
		settings := types.StartSettings{
			OpAMPServerURL: "ws://" + srv.Endpoint,
		}
		prepareClient(t, &settings, client)

		assert.NoError(t, client.Start(context.Background(), settings))

		// Send message 1 (should fail because capability not supported)
		customMessage1 := &protobufs.CustomMessage{
			Capability: "local.test.example",
			Type:       "hello",
			Data:       []byte("test message 1"),
		}
		_, err := client.SendCustomMessage(customMessage1)
		assert.Error(t, err)

		// SetCustomCapabilities fails when protobufs.CustomCapabilities is missing
		err = client.SetCustomCapabilities(nil)
		assert.Error(t, err)

		// Update CustomCapabilities
		newCustomCapabilities := &protobufs.CustomCapabilities{
			Capabilities: []string{"local.test.example"},
		}
		err = client.SetCustomCapabilities(newCustomCapabilities)
		assert.NoError(t, err)

		// Verify custom capabilities delivered to the server
		eventually(
			t,
			func() bool {
				msg, ok := rcvCustomCapabilities.Load().(*protobufs.CustomCapabilities)
				if !ok || msg == nil {
					return false
				}
				return proto.Equal(newCustomCapabilities, msg)
			},
		)

		// Send message 1 again (should succeed now)
		_, err = client.SendCustomMessage(customMessage1)
		assert.NoError(t, err)

		// Verify message 1 delivered
		eventually(
			t,
			func() bool {
				msg, ok := rcvCustomMessage.Load().(*protobufs.CustomMessage)
				if !ok || msg == nil {
					return false
				}
				return proto.Equal(customMessage1, msg)
			},
		)

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err = client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

// TestSetFlags tests the ability for the client to change the set of flags it sends.
func TestSetFlags(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a Server.
		srv := internal.StartMockServer(t)
		var rcvCustomFlags atomic.Value
		var flags protobufs.AgentToServerFlags

		srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			if msg.Flags != 0 {
				rcvCustomFlags.Store(msg.Flags)
			}
			return nil
		}

		settings := types.StartSettings{}
		settings.OpAMPServerURL = "ws://" + srv.Endpoint
		prepareClient(t, &settings, client)

		assert.NoError(t, client.Start(context.Background(), settings))

		// The zero value of AgentToServerFlags is ready to use
		client.SetFlags(flags)

		// Update flags to send
		flags |= protobufs.AgentToServerFlags_AgentToServerFlags_RequestInstanceUid
		client.SetFlags(flags)

		// Verify new flags were delivered to the server
		eventually(
			t,
			func() bool {
				msg, ok := rcvCustomFlags.Load().(uint64)
				if !ok || msg == 0 {
					return false
				}
				return uint64(flags) == msg
			},
		)

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

// TestSetFlags tests the ability for the client to set its flags before starting up.
func TestSetFlagsBeforeStart(t *testing.T) {
	testClients(t, func(t *testing.T, client OpAMPClient) {
		// Start a Server.
		flags := protobufs.AgentToServerFlags_AgentToServerFlags_RequestInstanceUid
		srv := internal.StartMockServer(t)
		var rcvCustomFlags atomic.Value
		var isFirstMessage atomic.Bool
		isFirstMessage.Store(true)

		// Make sure we only record flags from the very first message.
		srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
			if isFirstMessage.Load() {
				rcvCustomFlags.Store(msg.Flags)
			}
			isFirstMessage.Store(false)
			return nil
		}

		settings := types.StartSettings{}
		settings.OpAMPServerURL = "ws://" + srv.Endpoint
		prepareClient(t, &settings, client)

		// Set up the flags _before_ calling Start to verify that they're
		// handled correctly in PrepareFirstMessage.
		client.SetFlags(flags)

		// Start the client.
		assert.NoError(t, client.Start(context.Background(), settings))

		// Verify the flags were delivered to the server during the first message.
		eventually(
			t,
			func() bool {
				msg, ok := rcvCustomFlags.Load().(uint64)
				if !ok || msg == 0 {
					return false
				}
				return uint64(flags) == msg
			},
		)

		// Shutdown the Server.
		srv.Close()

		// Shutdown the client.
		err := client.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestSetAvailableComponents(t *testing.T) {
	testCases := []struct {
		desc         string
		capabilities protobufs.AgentCapabilities
		testFunc     func(t *testing.T, client OpAMPClient, srv *internal.MockServer)
	}{
		{
			desc:         "apply nil AvailableComponents",
			capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents,
			testFunc: func(t *testing.T, client OpAMPClient, _ *internal.MockServer) {
				require.ErrorIs(t, client.SetAvailableComponents(nil), types.ErrAvailableComponentsMissing)
			},
		},
		{
			desc:         "apply AvailableComponents with empty hash",
			capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents,
			testFunc: func(t *testing.T, client OpAMPClient, _ *internal.MockServer) {
				require.ErrorIs(t, client.SetAvailableComponents(&protobufs.AvailableComponents{}), types.ErrNoAvailableComponentHash)
			},
		},
		{
			desc: "apply AvailableComponents without required capability",
			testFunc: func(t *testing.T, client OpAMPClient, _ *internal.MockServer) {
				require.ErrorIs(t, client.SetAvailableComponents(generateTestAvailableComponents()), types.ErrReportsAvailableComponentsNotSet)
			},
		},
		{
			desc:         "apply AvailableComponents with cached AvailableComponents",
			capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents,
			testFunc: func(t *testing.T, client OpAMPClient, _ *internal.MockServer) {
				require.NoError(t, client.SetAvailableComponents(generateTestAvailableComponents()))
			},
		},
		{
			desc:         "apply AvailableComponents with new AvailableComponents",
			capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents,
			testFunc: func(t *testing.T, client OpAMPClient, srv *internal.MockServer) {
				availableComponents := generateTestAvailableComponents()
				availableComponents.Hash = []byte("different")
				require.NoError(t, client.SetAvailableComponents(availableComponents))
				srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
					assert.EqualValues(t, 1, msg.SequenceNum)
					msgAvailableComponents := msg.GetAvailableComponents()
					require.NotNil(t, msgAvailableComponents)
					require.Equal(t, msgAvailableComponents.GetHash(), availableComponents.GetHash())
					require.Nil(t, msgAvailableComponents.GetComponents())
					return nil
				})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			testClients(t, func(t *testing.T, client OpAMPClient) {
				// Start a Server.
				srv := internal.StartMockServer(t)
				srv.EnableExpectMode()

				availableComponents := generateTestAvailableComponents()
				client.SetAvailableComponents(availableComponents)

				// Start a client.
				settings := types.StartSettings{
					OpAMPServerURL: "ws://" + srv.Endpoint,
					Callbacks: types.Callbacks{
						OnMessage: func(ctx context.Context, msg *types.MessageData) {},
					},
					Capabilities: tc.capabilities,
				}
				prepareClient(t, &settings, client)

				// Client --->
				assert.NoError(t, client.Start(context.Background(), settings))

				// ---> Server
				srv.Expect(func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
					assert.EqualValues(t, 0, msg.SequenceNum)
					msgAvailableComponents := msg.GetAvailableComponents()
					if tc.capabilities&protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents != 0 {
						require.NotNil(t, msgAvailableComponents)
						require.Equal(t, msgAvailableComponents.GetHash(), availableComponents.GetHash())
						require.Nil(t, msgAvailableComponents.GetComponents())
					} else {
						require.Nil(t, msgAvailableComponents)
					}
					return nil
				})

				tc.testFunc(t, client, srv)

				// Shutdown the Server.
				srv.Close()

				// Shutdown the client.
				err := client.Stop(context.Background())
				assert.NoError(t, err)
			})
		})
	}
}

func TestValidateCapabilities(t *testing.T) {
	testCases := []struct {
		name          string
		capabilities  protobufs.AgentCapabilities
		setupFunc     func(t *testing.T, client OpAMPClient)
		expectedError error
	}{
		{
			name:         "ReportsHealth capability without health",
			capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth,
			setupFunc: func(t *testing.T, client OpAMPClient) {
				// Do not set health
			},
			expectedError: internal.ErrHealthMissing,
		},
		{
			name:         "ReportsHealth capability with health",
			capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth,
			setupFunc: func(t *testing.T, client OpAMPClient) {
				err := client.SetHealth(&protobufs.ComponentHealth{})
				require.NoError(t, err)
			},
			expectedError: nil,
		},
		{
			name:         "ReportsAvailableComponents capability without available components",
			capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents,
			setupFunc: func(t *testing.T, client OpAMPClient) {
				// Do not set available components
			},
			expectedError: internal.ErrAvailableComponentsMissing,
		},
		{
			name:         "ReportsAvailableComponents capability with available components",
			capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents,
			setupFunc: func(t *testing.T, client OpAMPClient) {
				err := client.SetAvailableComponents(generateTestAvailableComponents())
				require.NoError(t, err)
			},
			expectedError: nil,
		},
		{
			name:         "AcceptsPackages capability without PackagesStateProvider",
			capabilities: protobufs.AgentCapabilities_AgentCapabilities_AcceptsPackages,
			setupFunc: func(t *testing.T, client OpAMPClient) {
				// Do not set PackagesStateProvider
			},
			expectedError: internal.ErrPackagesStateProviderNotSet,
		},
		{
			name:         "ReportsPackageStatuses capability without PackagesStateProvider",
			capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsPackageStatuses,
			setupFunc: func(t *testing.T, client OpAMPClient) {
				// Do not set PackagesStateProvider
			},
			expectedError: internal.ErrPackagesStateProviderNotSet,
		},
		{
			name:         "AcceptsPackages and ReportsPackageStatuses capabilities without PackagesStateProvider",
			capabilities: protobufs.AgentCapabilities_AgentCapabilities_AcceptsPackages | protobufs.AgentCapabilities_AgentCapabilities_ReportsPackageStatuses,
			setupFunc: func(t *testing.T, client OpAMPClient) {
				// Do not set PackagesStateProvider
			},
			expectedError: internal.ErrPackagesStateProviderNotSet,
		},
		{
			name:         "No capabilities set",
			capabilities: protobufs.AgentCapabilities_AgentCapabilities_Unspecified,
			setupFunc: func(t *testing.T, client OpAMPClient) {
				// No setup needed
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testClients(t, func(t *testing.T, client OpAMPClient) {
				// Setup the client state as per the test case
				tc.setupFunc(t, client)

				// Validate capabilities
				err := client.SetCapabilities(&tc.capabilities)
				assert.Equal(t, tc.expectedError, err)
			})
		})
	}
}

func generateTestAvailableComponents() *protobufs.AvailableComponents {
	return &protobufs.AvailableComponents{
		Hash: []byte("fake-hash"),
		Components: map[string]*protobufs.ComponentDetails{
			"receivers": {
				Metadata: []*protobufs.KeyValue{
					{
						Key: "component",
						Value: &protobufs.AnyValue{
							Value: &protobufs.AnyValue_StringValue{
								StringValue: "filereceiver",
							},
						},
					},
				},
			},
		},
	}
}
