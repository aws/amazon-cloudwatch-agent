// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package extract

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/util"
)

const (
	instanceNameFlag     = "-s"
	defaultInstanceName  = "MSSQLSERVER"
	sqlServerRegBasePath = `SOFTWARE\Microsoft\Microsoft SQL Server`
)

// platformPortExtractors returns Windows-specific port extractors.
// The registry extractor correctly resolves ports for named instances.
func platformPortExtractors() []detector.PortExtractor {
	return []detector.PortExtractor{&registryPortExtractor{}}
}

// registryPortExtractor reads the TCP port from the Windows registry for a SQL Server instance.
// Each instance stores its port in:
// HKLM\SOFTWARE\Microsoft\Microsoft SQL Server\<InstanceID>\MSSQLServer\SuperSocketNetLib\Tcp\IPAll
type registryPortExtractor struct{}

func (e *registryPortExtractor) Extract(ctx context.Context, process detector.Process) (int, error) {
	instanceName := extractInstanceName(ctx, process)

	instanceID, err := resolveInstanceID(instanceName)
	if err != nil {
		return 0, err
	}

	return readPortFromRegistry(instanceID)
}

// extractInstanceName gets the SQL Server instance name from the process command line.
// Named instances are started with -s INSTANCENAME flag. Default instance uses MSSQLSERVER.
func extractInstanceName(ctx context.Context, process detector.Process) string {
	args, err := process.CmdlineSliceWithContext(ctx)
	if err != nil {
		return defaultInstanceName
	}

	for i, arg := range args {
		lower := strings.ToLower(arg)
		if lower == instanceNameFlag && i+1 < len(args) {
			return strings.ToUpper(args[i+1])
		}
		if strings.HasPrefix(lower, instanceNameFlag) && len(arg) > len(instanceNameFlag) {
			return strings.ToUpper(arg[len(instanceNameFlag):])
		}
	}

	return defaultInstanceName
}

// resolveInstanceID maps an instance name (e.g., "MSSQLSERVER" or "YOURDBINSTANCE2")
// to its registry ID (e.g., "MSSQL17.MSSQLSERVER") by reading
// HKLM\SOFTWARE\Microsoft\Microsoft SQL Server\Instance Names\SQL
func resolveInstanceID(instanceName string) (string, error) {
	keyPath := sqlServerRegBasePath + `\Instance Names\SQL`
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("failed to open registry key %s: %w", keyPath, err)
	}
	defer key.Close()

	instanceID, _, err := key.GetStringValue(instanceName)
	if err != nil {
		return "", fmt.Errorf("failed to read instance %s from registry: %w", instanceName, err)
	}

	return instanceID, nil
}

// readPortFromRegistry reads the TCP port for a given instance ID.
// Checks TcpPort (static) first, then TcpDynamicPorts (dynamic).
func readPortFromRegistry(instanceID string) (int, error) {
	keyPath := fmt.Sprintf(`%s\%s\MSSQLServer\SuperSocketNetLib\Tcp\IPAll`, sqlServerRegBasePath, instanceID)
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return 0, fmt.Errorf("failed to open registry key %s: %w", keyPath, err)
	}
	defer key.Close()

	if port, err := readPortValue(key, "TcpPort"); err == nil {
		return port, nil
	}

	if port, err := readPortValue(key, "TcpDynamicPorts"); err == nil {
		return port, nil
	}

	return 0, detector.ErrExtractPort
}

func readPortValue(key registry.Key, valueName string) (int, error) {
	val, _, err := key.GetStringValue(valueName)
	if err != nil {
		return 0, err
	}
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, detector.ErrExtractPort
	}
	port, err := strconv.Atoi(val)
	if err != nil {
		return 0, err
	}
	if !util.IsValidPort(port) {
		return 0, detector.ErrInvalidPort
	}
	return port, nil
}
