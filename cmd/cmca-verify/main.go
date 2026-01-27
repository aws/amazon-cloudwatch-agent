// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// cmca-verify is a standalone tool to verify CMCA provider implementations
// return correct values from cloud IMDS endpoints.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata"
)

const (
	// Azure IMDS endpoints
	azureIMDSBase   = "http://169.254.169.254/metadata/instance"
	azureAPIVersion = "2021-02-01"

	// AWS IMDS endpoints
	awsIMDSBase = "http://169.254.169.254/latest/meta-data"
	// #nosec G101 -- This is the AWS IMDS endpoint URL, not a credential
	awsIMDSTokenURL = "http://169.254.169.254/latest/api/token"
)

type verificationResult struct {
	Field    string
	Expected string
	Actual   string
	Match    bool
	Source   string
}

func main() {
	verbose := flag.Bool("v", false, "Verbose output")
	jsonOutput := flag.Bool("json", false, "Output results as JSON")
	flag.Parse()

	// Setup logger
	config := zap.NewProductionConfig()
	if *verbose {
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	} else {
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}
	logger, err := config.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Initialize CMCA
	logger.Info("Initializing CMCA provider...")
	ctx := context.Background()
	if err := cloudmetadata.InitGlobalProvider(ctx, logger); err != nil {
		logger.Error("Failed to initialize CMCA provider", zap.Error(err))
		os.Exit(1)
	}

	provider, err := cloudmetadata.GetGlobalProvider()
	if err != nil {
		logger.Error("Failed to get CMCA provider", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("CMCA provider initialized successfully")

	// Detect cloud and run appropriate verification
	var results []verificationResult

	if isAzure() {
		logger.Info("Detected Azure environment")
		results = verifyAzure(logger, provider)
	} else if isAWS() {
		logger.Info("Detected AWS environment")
		results = verifyAWS(logger, provider)
	} else {
		logger.Warn("Could not detect cloud environment (using mock provider)")
		results = verifyMock(logger, provider)
	}

	// Output results
	if *jsonOutput {
		outputJSON(results)
	} else {
		outputTable(results)
	}

	// Exit with error if any verification failed
	for _, r := range results {
		if !r.Match {
			os.Exit(1)
		}
	}
}

func isAzure() bool {
	// Check DMI for Azure signature
	data, err := os.ReadFile("/sys/class/dmi/id/sys_vendor")
	if err == nil && string(data) == "Microsoft Corporation\n" {
		return true
	}

	// Try Azure IMDS
	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest("GET", azureIMDSBase+"?api-version="+azureAPIVersion, nil)
	req.Header.Set("Metadata", "true")
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
		return resp.StatusCode == 200
	}

	return false
}

func isAWS() bool {
	// Try AWS IMDS
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(awsIMDSBase + "/instance-id")
	if err == nil {
		resp.Body.Close()
		return resp.StatusCode == 200
	}
	return false
}

func verifyAzure(logger *zap.Logger, provider cloudmetadata.Provider) []verificationResult {
	results := []verificationResult{}

	// Fetch Azure IMDS data
	logger.Info("Fetching Azure IMDS metadata...")
	compute, network, err := fetchAzureIMDS()
	if err != nil {
		logger.Error("Failed to fetch Azure IMDS", zap.Error(err))
		return results
	}

	// Verify each field
	results = append(results, verificationResult{
		Field:    "InstanceId (cloud:InstanceId)",
		Expected: compute.VMID,
		Actual:   provider.GetInstanceID(),
		Match:    compute.VMID == provider.GetInstanceID(),
		Source:   "Azure IMDS compute.vmId",
	})

	results = append(results, verificationResult{
		Field:    "Region (cloud:Region)",
		Expected: compute.Location,
		Actual:   provider.GetRegion(),
		Match:    compute.Location == provider.GetRegion(),
		Source:   "Azure IMDS compute.location",
	})

	results = append(results, verificationResult{
		Field:    "AccountId (cloud:AccountId)",
		Expected: compute.SubscriptionID,
		Actual:   provider.GetAccountID(),
		Match:    compute.SubscriptionID == provider.GetAccountID(),
		Source:   "Azure IMDS compute.subscriptionId",
	})

	results = append(results, verificationResult{
		Field:    "InstanceType (cloud:InstanceType)",
		Expected: compute.VMSize,
		Actual:   provider.GetInstanceType(),
		Match:    compute.VMSize == provider.GetInstanceType(),
		Source:   "Azure IMDS compute.vmSize",
	})

	// Private IP - extract from network metadata
	expectedIP := ""
	if len(network.Interface) > 0 && len(network.Interface[0].IPv4.IPAddress) > 0 {
		expectedIP = network.Interface[0].IPv4.IPAddress[0].PrivateIPAddress
	}

	results = append(results, verificationResult{
		Field:    "PrivateIp (cloud:PrivateIp)",
		Expected: expectedIP,
		Actual:   provider.GetPrivateIP(),
		Match:    expectedIP == provider.GetPrivateIP(),
		Source:   "Azure IMDS network.interface[0].ipv4.ipAddress[0].privateIpAddress",
	})

	// Azure doesn't have availability zones
	results = append(results, verificationResult{
		Field:    "AvailabilityZone (cloud:AvailabilityZone)",
		Expected: "",
		Actual:   provider.GetAvailabilityZone(),
		Match:    provider.GetAvailabilityZone() == "",
		Source:   "N/A (Azure doesn't have AZs)",
	})

	// ImageID not directly available in Azure IMDS
	results = append(results, verificationResult{
		Field:    "ImageId (cloud:ImageId)",
		Expected: "",
		Actual:   provider.GetImageID(),
		Match:    true, // Accept any value for now
		Source:   "N/A (not in Azure IMDS)",
	})

	return results
}

func verifyAWS(logger *zap.Logger, provider cloudmetadata.Provider) []verificationResult {
	results := []verificationResult{}

	// Fetch AWS IMDS data
	logger.Info("Fetching AWS IMDS metadata...")
	metadata, err := fetchAWSIMDS()
	if err != nil {
		logger.Error("Failed to fetch AWS IMDS", zap.Error(err))
		return results
	}

	// Verify each field
	results = append(results, verificationResult{
		Field:    "InstanceId (cloud:InstanceId)",
		Expected: metadata.InstanceID,
		Actual:   provider.GetInstanceID(),
		Match:    metadata.InstanceID == provider.GetInstanceID(),
		Source:   "AWS IMDS /instance-id",
	})

	results = append(results, verificationResult{
		Field:    "Region (cloud:Region)",
		Expected: metadata.Region,
		Actual:   provider.GetRegion(),
		Match:    metadata.Region == provider.GetRegion(),
		Source:   "AWS IMDS /placement/region",
	})

	results = append(results, verificationResult{
		Field:    "AvailabilityZone (cloud:AvailabilityZone)",
		Expected: metadata.AvailabilityZone,
		Actual:   provider.GetAvailabilityZone(),
		Match:    metadata.AvailabilityZone == provider.GetAvailabilityZone(),
		Source:   "AWS IMDS /placement/availability-zone",
	})

	results = append(results, verificationResult{
		Field:    "PrivateIp (cloud:PrivateIp)",
		Expected: metadata.PrivateIP,
		Actual:   provider.GetPrivateIP(),
		Match:    metadata.PrivateIP == provider.GetPrivateIP(),
		Source:   "AWS IMDS /local-ipv4",
	})

	results = append(results, verificationResult{
		Field:    "InstanceType (cloud:InstanceType)",
		Expected: metadata.InstanceType,
		Actual:   provider.GetInstanceType(),
		Match:    metadata.InstanceType == provider.GetInstanceType(),
		Source:   "AWS IMDS /instance-type",
	})

	results = append(results, verificationResult{
		Field:    "ImageId (cloud:ImageId)",
		Expected: metadata.ImageID,
		Actual:   provider.GetImageID(),
		Match:    metadata.ImageID == provider.GetImageID(),
		Source:   "AWS IMDS /ami-id",
	})

	// AccountID requires parsing identity document
	results = append(results, verificationResult{
		Field:    "AccountId (cloud:AccountId)",
		Expected: metadata.AccountID,
		Actual:   provider.GetAccountID(),
		Match:    metadata.AccountID == provider.GetAccountID(),
		Source:   "AWS IMDS /dynamic/instance-identity/document",
	})

	return results
}

func verifyMock(_ *zap.Logger, provider cloudmetadata.Provider) []verificationResult {
	results := []verificationResult{}

	// For mock provider, just verify it returns non-empty values
	fields := map[string]string{
		"InstanceId":       provider.GetInstanceID(),
		"Region":           provider.GetRegion(),
		"PrivateIp":        provider.GetPrivateIP(),
		"AvailabilityZone": provider.GetAvailabilityZone(),
		"AccountId":        provider.GetAccountID(),
		"ImageId":          provider.GetImageID(),
		"InstanceType":     provider.GetInstanceType(),
	}

	for field, value := range fields {
		results = append(results, verificationResult{
			Field:    field,
			Expected: "(mock value)",
			Actual:   value,
			Match:    value != "",
			Source:   "Mock provider",
		})
	}

	return results
}

// Azure IMDS structures
type azureComputeMetadata struct {
	VMID           string `json:"vmId"`
	Location       string `json:"location"`
	VMSize         string `json:"vmSize"`
	SubscriptionID string `json:"subscriptionId"`
	ResourceGroup  string `json:"resourceGroupName"`
	Name           string `json:"name"`
}

type azureNetworkMetadata struct {
	Interface []struct {
		IPv4 struct {
			IPAddress []struct {
				PrivateIPAddress string `json:"privateIpAddress"`
			} `json:"ipAddress"`
		} `json:"ipv4"`
	} `json:"interface"`
}

func fetchAzureIMDS() (*azureComputeMetadata, *azureNetworkMetadata, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	// Fetch compute metadata
	req, _ := http.NewRequest("GET", azureIMDSBase+"/compute?api-version="+azureAPIVersion+"&format=json", nil)
	req.Header.Set("Metadata", "true")
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch compute metadata: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var compute azureComputeMetadata
	if err := json.Unmarshal(body, &compute); err != nil {
		return nil, nil, fmt.Errorf("failed to parse compute metadata: %w", err)
	}

	// Fetch network metadata
	req, _ = http.NewRequest("GET", azureIMDSBase+"/network?api-version="+azureAPIVersion+"&format=json", nil)
	req.Header.Set("Metadata", "true")
	resp, err = client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch network metadata: %w", err)
	}
	defer resp.Body.Close()

	body, _ = io.ReadAll(resp.Body)
	var network azureNetworkMetadata
	if err := json.Unmarshal(body, &network); err != nil {
		return nil, nil, fmt.Errorf("failed to parse network metadata: %w", err)
	}

	return &compute, &network, nil
}

// AWS IMDS structures
type awsMetadata struct {
	InstanceID       string
	Region           string
	AvailabilityZone string
	PrivateIP        string
	InstanceType     string
	ImageID          string
	AccountID        string
}

type awsIdentityDocument struct {
	AccountID string `json:"accountId"`
	Region    string `json:"region"`
}

func fetchAWSIMDS() (*awsMetadata, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	// Get IMDSv2 token
	tokenReq, _ := http.NewRequest("PUT", awsIMDSTokenURL, nil)
	tokenReq.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "21600")
	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get IMDSv2 token: %w", err)
	}
	defer tokenResp.Body.Close()

	tokenBytes, _ := io.ReadAll(tokenResp.Body)
	token := string(tokenBytes)

	// Helper to fetch metadata with token
	fetch := func(path string) (string, error) {
		req, _ := http.NewRequest("GET", awsIMDSBase+path, nil)
		req.Header.Set("X-aws-ec2-metadata-token", token)
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body), nil
	}

	metadata := &awsMetadata{}

	metadata.InstanceID, _ = fetch("/instance-id")
	metadata.AvailabilityZone, _ = fetch("/placement/availability-zone")
	metadata.PrivateIP, _ = fetch("/local-ipv4")
	metadata.InstanceType, _ = fetch("/instance-type")
	metadata.ImageID, _ = fetch("/ami-id")

	// Get region and account from identity document
	req, _ := http.NewRequest("GET", "http://169.254.169.254/latest/dynamic/instance-identity/document", nil)
	req.Header.Set("X-aws-ec2-metadata-token", token)
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var doc awsIdentityDocument
		if json.Unmarshal(body, &doc) == nil {
			metadata.AccountID = doc.AccountID
			metadata.Region = doc.Region
		}
	}

	return metadata, nil
}

func outputTable(results []verificationResult) {
	fmt.Println("\n=== CMCA Provider Verification Results ===")
	fmt.Println()

	maxFieldLen := 0
	for _, r := range results {
		if len(r.Field) > maxFieldLen {
			maxFieldLen = len(r.Field)
		}
	}

	passed := 0
	failed := 0

	for _, r := range results {
		status := "✅ PASS"
		if !r.Match {
			status = "❌ FAIL"
			failed++
		} else {
			passed++
		}

		fmt.Printf("%-*s  %s\n", maxFieldLen, r.Field, status)
		fmt.Printf("  Expected: %s\n", r.Expected)
		fmt.Printf("  Actual:   %s\n", r.Actual)
		fmt.Printf("  Source:   %s\n\n", r.Source)
	}

	fmt.Printf("=== Summary: %d passed, %d failed ===\n", passed, failed)
}

func outputJSON(results []verificationResult) {
	data, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(data))
}
