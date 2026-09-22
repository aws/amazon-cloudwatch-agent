// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2metadataprovider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func createTestProvider(t *testing.T, serverURL string, retries int) MetadataProvider {
	t.Helper()
	// Keep the tests hermetic: the IMDS client reads these from the environment.
	t.Setenv("AWS_EC2_METADATA_DISABLED", "false")
	t.Setenv("AWS_EC2_METADATA_V1_DISABLED", "false")
	return newMetadataProvider(aws.Config{}, nil, retries, func(o *imds.Options) {
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
		v2Enabled bool
	}{
		"v2_enabled": {
			v2Enabled: true,
		},
		"v2_disabled_fallback_to_v1": {
			v2Enabled: false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			server := mockIMDSServer(t, testCase.v2Enabled, map[string]string{
				"instance-identity/document": instanceDoc,
			})
			defer server.Close()

			provider := createTestProvider(t, server.URL, 1)
			doc, err := provider.Get(t.Context())

			require.NoError(t, err)
			assert.Equal(t, "i-1234567890abcdef0", doc.InstanceID)
			assert.Equal(t, "us-west-2", doc.Region)
			assert.Equal(t, "us-west-2a", doc.AvailabilityZone)
			assert.Equal(t, "t3.micro", doc.InstanceType)
		})
	}
}

func TestMetadataProvider_InstanceID(t *testing.T) {
	testCases := map[string]struct {
		v2Enabled  bool
		instanceID string
	}{
		"v2_enabled": {
			v2Enabled:  true,
			instanceID: "i-1234567890abcdef0",
		},
		"v2_disabled_fallback_to_v1": {
			v2Enabled:  false,
			instanceID: "i-0987654321fedcba0",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			server := mockIMDSServer(t, testCase.v2Enabled, map[string]string{
				"instance-id": testCase.instanceID,
			})
			defer server.Close()

			provider := createTestProvider(t, server.URL, 1)
			instanceID, err := provider.InstanceID(t.Context())

			require.NoError(t, err)
			assert.Equal(t, testCase.instanceID, instanceID)
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

			provider := createTestProvider(t, server.URL, 1)
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

			provider := createTestProvider(t, server.URL, 1)
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

			provider := createTestProvider(t, server.URL, 1)
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

			provider := createTestProvider(t, server.URL, 1)
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

		provider := createTestProvider(t, server.URL, 0)
		_, err := provider.InstanceID(t.Context())

		assert.Error(t, err)
	})
}

// v1OnlyIMDSServer rejects every IMDSv2 token request and serves metadata to
// token-less (IMDSv1) requests, counting both.
func v1OnlyIMDSServer(t *testing.T) (*httptest.Server, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	var tokenPuts, tokenlessGets atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/latest/api/token" {
			tokenPuts.Add(1)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.Header.Get("X-aws-ec2-metadata-token") == "" {
			tokenlessGets.Add(1)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("i-1234567890abcdef0"))
	}))
	t.Cleanup(server.Close)
	return server, &tokenPuts, &tokenlessGets
}

func TestMetadataProvider_IMDSv1OptOut(t *testing.T) {
	testCases := map[string]struct {
		env          string
		sharedConfig string
		wantErr      bool
	}{
		"fallback_allowed":               {env: "false"},
		"env_opt_out":                    {env: "true", wantErr: true},
		"shared_config_opt_out":          {sharedConfig: "[default]\nec2_metadata_v1_disabled = true\n", wantErr: true},
		"env_false_overrides_shared_cfg": {env: "false", sharedConfig: "[default]\nec2_metadata_v1_disabled = true\n"},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			server, tokenPuts, tokenlessGets := v1OnlyIMDSServer(t)

			t.Setenv("AWS_EC2_METADATA_DISABLED", "false")
			t.Setenv("AWS_EC2_METADATA_V1_DISABLED", testCase.env)
			t.Setenv("AWS_PROFILE", "")
			sharedConfigFiles := []string{}
			if testCase.sharedConfig != "" {
				path := filepath.Join(t.TempDir(), "config")
				require.NoError(t, os.WriteFile(path, []byte(testCase.sharedConfig), 0o600))
				sharedConfigFiles = []string{path}
			}
			cfg, err := config.LoadDefaultConfig(t.Context(),
				config.WithSharedConfigFiles(sharedConfigFiles),
				config.WithSharedCredentialsFiles([]string{}))
			require.NoError(t, err)

			provider := newMetadataProvider(cfg, nil, 0, func(o *imds.Options) {
				o.Endpoint = server.URL
			})
			id, err := provider.InstanceID(t.Context())

			assert.Positive(t, tokenPuts.Load(), "IMDSv2 must be attempted first")
			if testCase.wantErr {
				assert.Error(t, err)
				assert.Zero(t, tokenlessGets.Load(), "opted-out client must not send token-less requests")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "i-1234567890abcdef0", id)
			assert.Positive(t, tokenlessGets.Load(), "fallback should have used IMDSv1")
		})
	}
}

// A config without ConfigSources (e.g. a zero aws.Config substituted after a
// failed load) cannot carry the shared-config opt-out, but the environment
// opt-out must still be honored.
func TestMetadataProvider_IMDSv1OptOut_ZeroConfig(t *testing.T) {
	server, tokenPuts, tokenlessGets := v1OnlyIMDSServer(t)
	t.Setenv("AWS_EC2_METADATA_DISABLED", "false")
	t.Setenv("AWS_EC2_METADATA_V1_DISABLED", "true")

	provider := newMetadataProvider(aws.Config{}, nil, 0, func(o *imds.Options) {
		o.Endpoint = server.URL
	})
	_, err := provider.InstanceID(t.Context())

	assert.Positive(t, tokenPuts.Load(), "IMDSv2 must be attempted first")
	assert.Error(t, err)
	assert.Zero(t, tokenlessGets.Load(), "opted-out client must not send token-less requests")
}
