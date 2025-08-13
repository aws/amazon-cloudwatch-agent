package internal

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/assert"
)

func TestNewPackagesSyncer(t *testing.T) {
	tests := []struct {
		name          string
		clientFactory func(context.Context, *protobufs.DownloadableFile) (*http.Client, error)
		err           string
	}{
		{
			name:          "nil client factory",
			clientFactory: nil,
			err:           "httpClientFactory must not be nil",
		},
		{
			name: "non-nil client factory",
			clientFactory: func(context.Context, *protobufs.DownloadableFile) (*http.Client, error) {
				return &http.Client{}, nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewPackagesSyncer(
				nil,
				&protobufs.PackagesAvailable{},
				nil,
				&ClientSyncedState{},
				&InMemPackagesStore{},
				&sync.Mutex{},
				time.Second,
				tt.clientFactory,
			)
			if tt.err != "" {
				assert.EqualError(t, err, tt.err)
				assert.Nil(t, s)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, s)
			}
		})
	}
}
