// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

type mockNetInterface struct {
	addrs []net.Addr
	err   error
}

func (m *mockNetInterface) Addrs() ([]net.Addr, error) {
	return m.addrs, m.err
}

func TestHostname(t *testing.T) {
	testErr := errors.New("test")
	p := NewMetadataProvider().(*hostMetadataProvider)
	p.osHostname = func() (string, error) {
		return "", testErr
	}
	t.Setenv(envconfig.HostName, "hostname")
	got, err := p.Hostname()
	assert.NoError(t, err)
	assert.Equal(t, "hostname", got)
	t.Setenv(envconfig.HostName, "")
	got, err = p.Hostname()
	assert.ErrorIs(t, err, testErr)
	assert.Equal(t, "", got)
}

func TestHostIP(t *testing.T) {
	testErr := errors.New("test")
	testCases := map[string]struct {
		envHostIP     string
		netInterfaces []netInterface
		netErr        error
		wantHostIP    string
		wantErr       error
	}{
		"WithEnvironmentVariable": {
			envHostIP:  "host-ip",
			wantHostIP: "host-ip",
		},
		"WithNetInterfaces/Error": {
			netErr:  testErr,
			wantErr: testErr,
		},
		"WithNetInterfaces/None": {
			wantErr: errNoPrivateIPv4,
		},
		"WithNetInterfaces/Skipped": {
			netInterfaces: []netInterface{
				&mockNetInterface{
					addrs: []net.Addr{
						&net.IPAddr{IP: net.IPv4(127, 0, 0, 1)},
						&net.IPNet{IP: net.IPv4(224, 0, 0, 0)},
					},
				},
				&mockNetInterface{
					addrs: []net.Addr{
						&net.IPAddr{IP: net.IPv4(10, 24, 34, 0)},
					},
					err: testErr,
				},
				&mockNetInterface{
					addrs: []net.Addr{
						&net.IPAddr{IP: net.IPv4(0, 0, 0, 0)},
						&net.IPNet{IP: net.IPv4(169, 254, 0, 0)},
					},
				},
			},
			wantErr: errNoPrivateIPv4,
		},
		"WithNetInterfaces/Found": {
			netInterfaces: []netInterface{
				&mockNetInterface{
					addrs: []net.Addr{
						&net.IPAddr{IP: net.IPv4(10, 24, 34, 0)},
					},
				},
			},
			wantHostIP: "10.24.34.0",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(envconfig.HostIP, testCase.envHostIP)
			p := NewMetadataProvider().(*hostMetadataProvider)
			p.netInterfaces = func() ([]netInterface, error) {
				return testCase.netInterfaces, testCase.netErr
			}
			got, err := p.HostIP()
			assert.ErrorIs(t, err, testCase.wantErr)
			assert.Equal(t, testCase.wantHostIP, got)
		})
	}
}

func TestFilterInterfaces(t *testing.T) {
	first := net.Interface{
		Flags: net.FlagUp,
		Index: 1,
	}
	skip := net.Interface{
		Flags: net.FlagUp | net.FlagPointToPoint,
		Index: 2,
	}
	third := net.Interface{
		Flags: net.FlagUp | net.FlagRunning,
		Index: 3,
	}
	got := filterInterfaces([]net.Interface{
		third,
		skip,
		first,
	})
	assert.Equal(t, []netInterface{
		&first,
		&third,
	}, got)
}
