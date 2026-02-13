// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultAzureIMDSEndpoint   = "http://169.254.169.254/metadata/identity/oauth2/token"
	defaultAzureIMDSAPIVersion = "2018-02-01"

	// defaultAzureProbeEndpoint is used for detection only — probes the compute metadata
	// path (same as resourcedetectionprocessor's Azure detector) to check "am I on Azure?"
	// without requiring managed identity to be enabled.
	defaultAzureProbeEndpoint   = "http://169.254.169.254/metadata/instance/compute"
	defaultAzureProbeAPIVersion = "2020-09-01"
	defaultAzureProbeTimeout    = 2 * time.Second

	// defaultAzureResource is the audience/resource claim for the token request.
	defaultAzureResource = "https://management.azure.com/"

	// defaultAzureTokenExpiry is the fallback token lifetime (1 hour) when the
	// IMDS response does not include a valid expires_in value.
	defaultAzureTokenExpiry = 3600
)

// AzureProvider fetches OIDC tokens from Azure Instance Metadata Service (IMDS)
// on VMs with managed identity enabled.
type AzureProvider struct {
	client        *http.Client
	endpoint      string // token endpoint, overridable for testing
	probeEndpoint string // compute endpoint for detection, overridable for testing
	resource      string // audience/resource for the token request
}

var _ TokenProvider = (*AzureProvider)(nil)

func NewAzureProvider() TokenProvider {
	return &AzureProvider{
		client:        &http.Client{Timeout: 30 * time.Second},
		endpoint:      defaultAzureIMDSEndpoint,
		probeEndpoint: defaultAzureProbeEndpoint,
		resource:      defaultAzureResource,
	}
}

func (p *AzureProvider) Name() string { return "azure" }

// SetResource overrides the audience/resource claim for the token request.
func (p *AzureProvider) SetResource(r string) { p.resource = r }

func (p *AzureProvider) IsAvailable(ctx context.Context) bool {
	probeCtx, cancel := context.WithTimeout(ctx, defaultAzureProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, p.probeEndpoint, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Metadata", "True")
	q := req.URL.Query()
	q.Set("api-version", defaultAzureProbeAPIVersion)
	q.Set("format", "json")
	req.URL.RawQuery = q.Encode()

	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	// Drain body so the underlying TCP connection can be reused.
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

type azureTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   string `json:"expires_in"`
}

func (p *AzureProvider) GetToken(ctx context.Context) (string, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint, nil)
	if err != nil {
		return "", 0, fmt.Errorf("azure: create request: %w", err)
	}
	req.Header.Set("Metadata", "true")
	q := req.URL.Query()
	q.Set("api-version", defaultAzureIMDSAPIVersion)
	q.Set("resource", p.resource)
	req.URL.RawQuery = q.Encode()

	resp, err := p.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("azure: IMDS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("azure: IMDS returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp azureTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", 0, fmt.Errorf("azure: decode response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", 0, fmt.Errorf("azure: empty access_token in IMDS response")
	}

	expiresIn, _ := strconv.Atoi(tokenResp.ExpiresIn)
	if expiresIn <= 0 {
		expiresIn = defaultAzureTokenExpiry
	}

	return tokenResp.AccessToken, time.Duration(expiresIn) * time.Second, nil
}
