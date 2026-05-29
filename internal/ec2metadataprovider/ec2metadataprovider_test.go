// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2metadataprovider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

func mockIMDSServer(t *testing.T, v2Enabled bool, responses map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if r.URL.Path == "/latest/api/token" {
			if !v2Enabled {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test-token"))
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/latest/meta-data/")
		if strings.HasPrefix(r.URL.Path, "/latest/dynamic/instance-identity/document") {
			path = "instance-identity/document"
		}

		if response, ok := responses[path]; ok {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
}

func createTestProvider(serverURL string, retries int) MetadataProvider {
	return newMetadataProvider(aws.Config{}, retries, func(o *imds.Options) {
		o.Endpoint = serverURL
	})
}

func TestMetadataProvider_Get(t *testing.T) {
	instanceDoc := `{
		"instanceId": "i-1234567890abcdef0",
		"region": "us-west-2",
		"availabilityZone": "us-west-2a",
		"instanceType": "t3.micro"
	}`

	testCases := map[string]struct {
		v2Enabled    bool
		wantFallback bool
	}{
		"v2_enabled": {
			v2Enabled:    true,
			wantFallback: false,
		},
		"v2_disabled_fallback_to_v1": {
			v2Enabled:    false,
			wantFallback: true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			server := mockIMDSServer(t, testCase.v2Enabled, map[string]string{
				"instance-identity/document": instanceDoc,
			})
			defer server.Close()

			provider := createTestProvider(server.URL, 1)
			doc, err := provider.Get(t.Context())

			require.NoError(t, err)
			assert.Equal(t, "i-1234567890abcdef0", doc.InstanceID)
			assert.Equal(t, "us-west-2", doc.Region)
			assert.Equal(t, "us-west-2a", doc.AvailabilityZone)
			assert.Equal(t, "t3.micro", doc.InstanceType)

			if testCase.wantFallback {
				assert.True(t, agent.UsageFlags().IsSet(agent.FlagIMDSFallbackSuccess))
			}
		})
	}
}

func TestMetadataProvider_InstanceID(t *testing.T) {
	testCases := map[string]struct {
		v2Enabled    bool
		instanceID   string
		wantFallback bool
	}{
		"v2_enabled": {
			v2Enabled:    true,
			instanceID:   "i-1234567890abcdef0",
			wantFallback: false,
		},
		"v2_disabled_fallback_to_v1": {
			v2Enabled:    false,
			instanceID:   "i-0987654321fedcba0",
			wantFallback: true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			server := mockIMDSServer(t, testCase.v2Enabled, map[string]string{
				"instance-id": testCase.instanceID,
			})
			defer server.Close()

			provider := createTestProvider(server.URL, 1)
			instanceID, err := provider.InstanceID(t.Context())

			require.NoError(t, err)
			assert.Equal(t, testCase.instanceID, instanceID)

			if testCase.wantFallback {
				assert.True(t, agent.UsageFlags().IsSet(agent.FlagIMDSFallbackSuccess))
			}
		})
	}
}

func TestMetadataProvider_Hostname(t *testing.T) {
	testCases := map[string]struct {
		v2Enabled bool
		hostname  string
	}{
		"v2_enabled": {
			v2Enabled: true,
			hostname:  "ip-10-0-0-1.us-west-2.compute.internal",
		},
		"v2_disabled_fallback_to_v1": {
			v2Enabled: false,
			hostname:  "ip-10-0-0-2.us-west-2.compute.internal",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			server := mockIMDSServer(t, testCase.v2Enabled, map[string]string{
				"hostname": testCase.hostname,
			})
			defer server.Close()

			provider := createTestProvider(server.URL, 1)
			hostname, err := provider.Hostname(t.Context())

			require.NoError(t, err)
			assert.Equal(t, testCase.hostname, hostname)
		})
	}
}

func TestMetadataProvider_InstanceTags(t *testing.T) {
	testCases := map[string]struct {
		v2Enabled  bool
		tagsString string
		wantTags   []string
	}{
		"v2_enabled_multiple_tags": {
			v2Enabled:  true,
			tagsString: "Name\nEnvironment\nApplication",
			wantTags:   []string{"Name", "Environment", "Application"},
		},
		"v2_disabled_fallback_to_v1": {
			v2Enabled:  false,
			tagsString: "Tag1\nTag2",
			wantTags:   []string{"Tag1", "Tag2"},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			server := mockIMDSServer(t, testCase.v2Enabled, map[string]string{
				"tags/instance": testCase.tagsString,
			})
			defer server.Close()

			provider := createTestProvider(server.URL, 1)
			tags, err := provider.InstanceTags(t.Context())

			require.NoError(t, err)
			assert.Equal(t, testCase.wantTags, tags)
		})
	}
}

func TestMetadataProvider_ClientIAMRole(t *testing.T) {
	testCases := map[string]struct {
		v2Enabled bool
		roleName  string
	}{
		"v2_enabled": {
			v2Enabled: true,
			roleName:  "MyInstanceRole",
		},
		"v2_disabled_fallback_to_v1": {
			v2Enabled: false,
			roleName:  "AnotherRole",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			server := mockIMDSServer(t, testCase.v2Enabled, map[string]string{
				"iam/security-credentials": testCase.roleName,
			})
			defer server.Close()

			provider := createTestProvider(server.URL, 1)
			roleName, err := provider.ClientIAMRole(t.Context())

			require.NoError(t, err)
			assert.Equal(t, testCase.roleName, roleName)
		})
	}
}

func TestMetadataProvider_InstanceTagValue(t *testing.T) {
	testCases := map[string]struct {
		v2Enabled bool
		tagKey    string
		tagValue  string
	}{
		"v2_enabled": {
			v2Enabled: true,
			tagKey:    "Name",
			tagValue:  "my-instance",
		},
		"v2_disabled_fallback_to_v1": {
			v2Enabled: false,
			tagKey:    "Environment",
			tagValue:  "production",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			server := mockIMDSServer(t, testCase.v2Enabled, map[string]string{
				fmt.Sprintf("tags/instance/%s", testCase.tagKey): testCase.tagValue,
			})
			defer server.Close()

			provider := createTestProvider(server.URL, 1)
			tagValue, err := provider.InstanceTagValue(t.Context(), testCase.tagKey)

			require.NoError(t, err)
			assert.Equal(t, testCase.tagValue, tagValue)
		})
	}
}

func TestMetadataProvider_ErrorHandling(t *testing.T) {
	t.Run("both_v2_and_v1_fail", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		provider := createTestProvider(server.URL, 0)
		_, err := provider.InstanceID(t.Context())

		assert.Error(t, err)
	})
}
