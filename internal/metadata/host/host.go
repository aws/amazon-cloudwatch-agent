// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"errors"
	"fmt"
	"net"
	"os"
	"sort"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

var (
	errNoPrivateIPv4 = errors.New("unable to find private IPv4")
)

type MetadataProvider interface {
	Hostname() (string, error)
	HostIP() (string, error)
}

type hostMetadataProvider struct {
	osHostname    func() (string, error)
	netInterfaces func() ([]netInterface, error)
}

func NewMetadataProvider() MetadataProvider {
	return &hostMetadataProvider{
		osHostname: os.Hostname,
		netInterfaces: func() ([]netInterface, error) {
			interfaces, err := net.Interfaces()
			if err != nil {
				return nil, err
			}
			return filterInterfaces(interfaces), nil
		},
	}
}

func (p *hostMetadataProvider) Hostname() (string, error) {
	hostname := os.Getenv(envconfig.HostName)
	if hostname == "" {
		return p.osHostname()
	}
	return hostname, nil
}

func (p *hostMetadataProvider) HostIP() (string, error) {
	hostIP := os.Getenv(envconfig.HostIP)
	if hostIP == "" {
		interfaces, err := p.netInterfaces()
		if err != nil {
			return "", fmt.Errorf("failed to load network interfaces: %w", err)
		}
		return selectIP(toIPv4s(interfaces))
	}
	return hostIP, nil
}

func filterInterfaces(interfaces []net.Interface) []netInterface {
	sort.Sort(byIndex(interfaces))
	var ifaces []netInterface
	for _, i := range interfaces {
		iface := i
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagPointToPoint != 0 {
			continue
		}
		ifaces = append(ifaces, &iface)
	}
	return ifaces
}

func toIPv4s(interfaces []netInterface) []net.IP {
	var ipv4s []net.IP
	for _, i := range interfaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPAddr:
				ipv4s = append(ipv4s, v.IP.To4())
			case *net.IPNet:
				ipv4s = append(ipv4s, v.IP.To4())
			}
		}
	}
	return ipv4s
}

func selectIP(ips []net.IP) (string, error) {
	var result net.IP
	for _, ip := range ips {
		if ip != nil && !ip.IsUnspecified() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && ip.IsPrivate() {
			result = ip
			break
		}
	}
	if result == nil {
		return "", errNoPrivateIPv4
	}
	return result.String(), nil
}

// netInterface is used to stub out the Addrs function to avoid the syscalls
type netInterface interface {
	Addrs() ([]net.Addr, error)
}

// byIndex implements sorting for net.Interface.
type byIndex []net.Interface

func (b byIndex) Len() int           { return len(b) }
func (b byIndex) Less(i, j int) bool { return b[i].Index < b[j].Index }
func (b byIndex) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
