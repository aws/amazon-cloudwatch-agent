// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockMetadataProvider struct {
	Index    int
	Metadata *Metadata
	Err      error
}

func (m *mockMetadataProvider) ID() string {
	return fmt.Sprintf("mock/%v", m.Index)
}

func (m *mockMetadataProvider) Get(context.Context) (*Metadata, error) {
	if m.Metadata != nil {
		return m.Metadata, nil
	}
	return nil, m.Err
}

func (m *mockMetadataProvider) Hostname(context.Context) (string, error) {
	if m.Metadata != nil && m.Metadata.Hostname != "" {
		return m.Metadata.Hostname, nil
	}
	return "", m.Err
}

func TestChainProvider(t *testing.T) {
	errFirstTest := errors.New("skip first")
	errSecondTest := errors.New("skip second")
	testCases := map[string]struct {
		providers    []MetadataProvider
		wantID       string
		wantMetadata *Metadata
		wantHostname string
		wantErr      error
	}{
		"WithErrors": {
			providers: []MetadataProvider{
				&mockMetadataProvider{
					Index: 1,
					Err:   errFirstTest,
				},
				&mockMetadataProvider{
					Index: 2,
					Err:   errSecondTest,
				},
			},
			wantID:  "Chain [mock/1,mock/2]",
			wantErr: errSecondTest,
		},
		"WithEarlyChainSuccess": {
			providers: []MetadataProvider{
				&mockMetadataProvider{
					Index: 1,
					Metadata: &Metadata{
						Hostname:   "hostname-1",
						InstanceID: "instance-id-1",
					},
				},
				&mockMetadataProvider{
					Index: 2,
					Metadata: &Metadata{
						Hostname:   "hostname-2",
						InstanceID: "instance-id-2",
					},
				},
			},
			wantID:       "Chain [mock/1,mock/2]",
			wantHostname: "hostname-1",
			wantMetadata: &Metadata{
				Hostname:   "hostname-1",
				InstanceID: "instance-id-1",
			},
		},
		"WithFallback": {
			providers: []MetadataProvider{
				&mockMetadataProvider{
					Index: 1,
					Err:   errFirstTest,
				},
				&mockMetadataProvider{
					Index: 2,
					Metadata: &Metadata{
						Hostname:   "hostname-2",
						InstanceID: "instance-id-2",
					},
				},
			},
			wantID:       "Chain [mock/1,mock/2]",
			wantHostname: "hostname-2",
			wantMetadata: &Metadata{
				Hostname:   "hostname-2",
				InstanceID: "instance-id-2",
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			p := newChainMetadataProvider(testCase.providers)
			assert.Equal(t, testCase.wantID, p.ID())
			hostname, err := p.Hostname(ctx)
			assert.ErrorIs(t, err, testCase.wantErr)
			assert.Equal(t, testCase.wantHostname, hostname)
			metadata, err := p.Get(ctx)
			assert.ErrorIs(t, err, testCase.wantErr)
			assert.Equal(t, testCase.wantMetadata, metadata)
		})
	}
}
