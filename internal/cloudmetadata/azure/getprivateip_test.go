// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azure

import (
	"testing"

	"go.uber.org/zap"
)

func TestGetPrivateIP_WithNetworkMetadata(t *testing.T) {
	logger := zap.NewNop()
	p := &Provider{
		logger: logger,
		networkMetadata: &NetworkMetadata{
			Interface: []NetworkInterface{
				{
					IPv4: NetworkIPv4{
						IPAddress: []NetworkIPAddress{
							{
								PrivateIPAddress: "172.16.0.4",
								PublicIPAddress:  "20.1.2.3",
							},
						},
					},
				},
			},
		},
	}

	result := p.GetPrivateIP()
	expected := "172.16.0.4"

	if result != expected {
		t.Errorf("GetPrivateIP() = %q, want %q", result, expected)
	}
}

func TestGetPrivateIP_NoNetworkMetadata(t *testing.T) {
	logger := zap.NewNop()
	p := &Provider{
		logger:          logger,
		networkMetadata: nil,
	}

	result := p.GetPrivateIP()
	expected := ""

	if result != expected {
		t.Errorf("GetPrivateIP() with no network metadata = %q, want %q", result, expected)
	}
}

func TestGetPrivateIP_NoInterfaces(t *testing.T) {
	logger := zap.NewNop()
	p := &Provider{
		logger: logger,
		networkMetadata: &NetworkMetadata{
			Interface: []NetworkInterface{},
		},
	}

	result := p.GetPrivateIP()
	expected := ""

	if result != expected {
		t.Errorf("GetPrivateIP() with no interfaces = %q, want %q", result, expected)
	}
}

func TestGetPrivateIP_NoIPAddresses(t *testing.T) {
	logger := zap.NewNop()
	p := &Provider{
		logger: logger,
		networkMetadata: &NetworkMetadata{
			Interface: []NetworkInterface{
				{
					IPv4: NetworkIPv4{
						IPAddress: []NetworkIPAddress{},
					},
				},
			},
		},
	}

	result := p.GetPrivateIP()
	expected := ""

	if result != expected {
		t.Errorf("GetPrivateIP() with no IP addresses = %q, want %q", result, expected)
	}
}

func TestGetPrivateIP_MultipleInterfaces(t *testing.T) {
	logger := zap.NewNop()
	p := &Provider{
		logger: logger,
		networkMetadata: &NetworkMetadata{
			Interface: []NetworkInterface{
				{
					IPv4: NetworkIPv4{
						IPAddress: []NetworkIPAddress{
							{
								PrivateIPAddress: "172.16.0.4",
								PublicIPAddress:  "20.1.2.3",
							},
						},
					},
				},
				{
					IPv4: NetworkIPv4{
						IPAddress: []NetworkIPAddress{
							{
								PrivateIPAddress: "172.16.0.5",
								PublicIPAddress:  "20.1.2.4",
							},
						},
					},
				},
			},
		},
	}

	result := p.GetPrivateIP()
	expected := "172.16.0.4" // Should return first interface

	if result != expected {
		t.Errorf("GetPrivateIP() with multiple interfaces = %q, want %q", result, expected)
	}
}

func TestGetPrivateIP_MultipleIPsPerInterface(t *testing.T) {
	logger := zap.NewNop()
	p := &Provider{
		logger: logger,
		networkMetadata: &NetworkMetadata{
			Interface: []NetworkInterface{
				{
					IPv4: NetworkIPv4{
						IPAddress: []NetworkIPAddress{
							{
								PrivateIPAddress: "172.16.0.4",
								PublicIPAddress:  "20.1.2.3",
							},
							{
								PrivateIPAddress: "172.16.0.10",
								PublicIPAddress:  "20.1.2.10",
							},
						},
					},
				},
			},
		},
	}

	result := p.GetPrivateIP()
	expected := "172.16.0.4" // Should return first IP

	if result != expected {
		t.Errorf("GetPrivateIP() with multiple IPs = %q, want %q", result, expected)
	}
}
