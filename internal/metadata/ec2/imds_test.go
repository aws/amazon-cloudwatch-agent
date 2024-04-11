// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/stretchr/testify/assert"
)

type mockIMDSClient struct {
	metadata map[string]string
	document ec2metadata.EC2InstanceIdentityDocument
	err      error
}

func (m *mockIMDSClient) GetMetadataWithContext(_ aws.Context, key string) (string, error) {
	if m.metadata == nil {
		return "", m.err
	}
	return m.metadata[key], m.err
}

func (m *mockIMDSClient) GetInstanceIdentityDocumentWithContext(aws.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	return m.document, m.err
}

func TestIMDSProvider(t *testing.T) {
	testErr := errors.New("test")
	testCases := map[string]struct {
		provider     *imdsMetadataProvider
		metadata     map[string]string
		document     ec2metadata.EC2InstanceIdentityDocument
		clientErr    error
		wantID       string
		wantHostname string
		wantMetadata *Metadata
		wantErr      error
	}{
		"WithSuccess": {
			provider: newIMDSv1MetadataProvider(mock.Session),
			metadata: map[string]string{
				metadataKeyHostname: "test.hostname",
			},
			document: ec2metadata.EC2InstanceIdentityDocument{
				AccountID:    "account-id",
				ImageID:      "image-id",
				InstanceID:   "instance-id",
				InstanceType: "instance-type",
				PrivateIP:    "private-ip",
				Region:       "region",
			},
			wantID:       string(IMDSv1),
			wantHostname: "test.hostname",
			wantMetadata: &Metadata{
				AccountID:    "account-id",
				ImageID:      "image-id",
				InstanceID:   "instance-id",
				InstanceType: "instance-type",
				PrivateIP:    "private-ip",
				Region:       "region",
			},
		},
		"WithError": {
			provider:  newIMDSv2MetadataProvider(mock.Session, 0),
			clientErr: testErr,
			wantID:    string(IMDSv2),
			wantErr:   testErr,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			p := testCase.provider
			p.svc = &mockIMDSClient{
				metadata: testCase.metadata,
				document: testCase.document,
				err:      testCase.clientErr,
			}
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
