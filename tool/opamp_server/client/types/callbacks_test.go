package types

import (
	"context"
	"net/http"
	"testing"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallbacksDefaults(t *testing.T) {
	c := Callbacks{}

	c.SetDefaults()

	assert.NotNil(t, c.OnConnect)
	assert.NotNil(t, c.OnConnectFailed)
	assert.NotNil(t, c.OnError)
	assert.NotNil(t, c.OnMessage)
	assert.NotNil(t, c.OnOpampConnectionSettings)
	assert.NotNil(t, c.OnCommand)
	assert.NotNil(t, c.GetEffectiveConfig)
	assert.NotNil(t, c.SaveRemoteConfigStatus)

	// Test default DownloadHTTPClient
	require.NotNil(t, c.DownloadHTTPClient)
	client, err := c.DownloadHTTPClient(context.Background(), &protobufs.DownloadableFile{})
	require.NoError(t, err)
	require.NotNil(t, client)

	// ensure transport was set to *http.Transport
	_, ok := client.Transport.(*http.Transport)
	require.True(t, ok, "Expected the transport to be of type *http.Transport")

	// ensure it returns the same client on subsequent calls
	client2, err := c.DownloadHTTPClient(context.Background(), &protobufs.DownloadableFile{})
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, client, client2)
}
