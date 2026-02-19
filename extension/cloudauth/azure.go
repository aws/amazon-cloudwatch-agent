// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	defaultAzureIMDSEndpoint = "http://169.254.169.254/metadata/identity/oauth2/token"
	// azureComputeEndpoint is used for detection only — probes the compute metadata
	// path (same as resourcedetectionprocessor's Azure detector) to check "am I on Azure?"
	// without requiring managed identity to be enabled.
	azureComputeEndpoint = "http://169.254.169.254/metadata/instance/compute"
	azureAPIVersion      = "2018-02-01"
	azureComputeAPI      = "2020-09-01"
	// The default resource/audience for the token.
	azureDefaultResource = "https://management.azure.com/"
	azureProbeTimeout = 2 * time.Second
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

func NewAzureProvider() *AzureProvider {
	return &AzureProvider{
		client:        &http.Client{Timeout: 30 * time.Second},
		endpoint:      defaultAzureIMDSEndpoint,
		probeEndpoint: azureComputeEndpoint,
		resource:      azureDefaultResource,
	}
}

func (p *AzureProvider) Name() string { return "azure" }

func (p *AzureProvider) IsAvailable(ctx context.Context) bool {
	probeCtx, cancel := context.WithTimeout(ctx, azureProbeTimeout)
	defer cancel()

	// Probe the compute metadata endpoint (not the token endpoint) to detect Azure.
	// This matches the resourcedetectionprocessor's Azure detector approach and works
	// even if managed identity is not enabled on the VM.
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet,
		fmt.Sprintf("%s?api-version=%s&format=json", p.probeEndpoint, azureComputeAPI), nil)
	if err != nil {
		return false
	}
	req.Header.Set("Metadata", "True")

	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

type azureTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   string `json:"expires_in"`
}

func (p *AzureProvider) GetToken(ctx context.Context) (string, time.Duration, error) {
	reqURL := fmt.Sprintf("%s?api-version=%s&resource=%s",
		p.endpoint, azureAPIVersion, url.QueryEscape(p.resource))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", 0, fmt.Errorf("azure: create request: %w", err)
	}
	req.Header.Set("Metadata", "true")

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
		expiresIn = 3600
	}

	return tokenResp.AccessToken, time.Duration(expiresIn) * time.Second, nil
}
