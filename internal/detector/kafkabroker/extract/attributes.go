// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extract

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/util"
)

const (
	extProperties = ".properties"
	flagOverride  = "--override"

	// propertyLogDirs is the comma-separated paths to the Kafka logs.
	propertyLogDirs = "log.dirs"
	// propertyBrokerID is the broker ID for older versions of Kafka (Zookeeper)
	propertyBrokerID = "broker.id"
	// propertyNodeID is the equivalent of the propertyBrokerID in newer versions of Kafka (KRaft)
	propertyNodeID = "node.id"
	// propertyClusterID is the cluster ID.
	propertyClusterID = "cluster.id"

	logDirsSeparator    = ","
	propertiesSeparator = '='
	// fileNameMetaProperties is the generated properties file in the Kafka logs directory containing the cluster ID
	// and is the source of truth for the broker ID.
	fileNameMetaProperties = "meta.properties"

	// brokerClassName is the main Java class name for the Kafka broker. This is what is normally run when starting a
	// Kafka broker.
	brokerClassName = "kafka.Kafka"
)

// brokerInfo holds identifying Kafka broker configuration fields extracted from command line arguments and properties
// files.
type brokerInfo struct {
	serverProperties string
	logDirs          string
	brokerID         string
}

// ShouldParseProperties returns true if the properties file should be read to extract the log directories or broker ID.
func (i *brokerInfo) ShouldParseProperties() bool {
	return len(i.serverProperties) != 0 && !(i.HasLogDirs() && i.HasBrokerID())
}

func (i *brokerInfo) HasLogDirs() bool {
	return len(i.logDirs) > 0
}

func (i *brokerInfo) HasBrokerID() bool {
	return len(i.brokerID) > 0
}

type attributesExtractor struct {
	logger *slog.Logger
}

func NewAttributesExtractor(logger *slog.Logger) detector.Extractor[map[string]string] {
	return &attributesExtractor{
		logger: logger,
	}
}

// Extract Kafka broker attributes (broker.id and cluster.id) from a process. It parses the command line arguments to
// identify Kafka brokers, reads configuration from the properties file if needed, and reads meta.properties files in
// Kafka log directories for cluster information. Returns a map containing non-empty attribute values.
func (e *attributesExtractor) Extract(ctx context.Context, process detector.Process) (map[string]string, error) {
	args, err := process.CmdlineSliceWithContext(ctx)
	if err != nil {
		return nil, err
	}

	info, err := e.parseArgs(args)
	if err != nil {
		return nil, err
	}

	// only parse the server properties if needed
	if info.ShouldParseProperties() {
		if err = e.parseServerPropertiesFile(ctx, process, info); err != nil {
			e.logger.Debug("Failed to parse Kafka server properties", "pid", process.PID(), "err", err)
		}
	}

	attributes := e.extractAttributesFromMetaProperties(ctx, process, info)
	if _, ok := attributes[propertyBrokerID]; !ok && info.HasBrokerID() {
		attributes[propertyBrokerID] = info.brokerID
	}
	for key, value := range attributes {
		if value == "" {
			delete(attributes, key)
		}
	}
	return attributes, nil
}

// extractAttributesFromMetaProperties reads the generated meta.properties files in Kafka log directories to find
// broker.id and cluster.id values.
func (e *attributesExtractor) extractAttributesFromMetaProperties(ctx context.Context, process detector.Process, info *brokerInfo) map[string]string {
	attributes := make(map[string]string)
	if !info.HasLogDirs() {
		return attributes
	}
	logDirs := strings.Split(info.logDirs, logDirsSeparator)
	for _, logDir := range logDirs {
		logDir = strings.TrimSpace(logDir)
		if logDir == "" {
			continue
		}
		dir, err := util.AbsPath(ctx, process, logDir)
		if err != nil {
			continue
		}
		err = e.parsePropertiesFile(filepath.Join(dir, fileNameMetaProperties), func(key, value string) bool {
			switch key {
			case propertyBrokerID, propertyNodeID:
				attributes[propertyBrokerID] = value
			case propertyClusterID:
				attributes[key] = value
			}
			// stop scanning the properties once both attributes have been set
			return len(attributes) < 2
		})
		if err != nil {
			continue
		}
		break
	}
	return attributes
}

// parseArgs extracts Kafka broker information from command line arguments. It looks for the kafka.Kafka class name,
// *.properties file path, and --override flags that can be used to override the broker.id and log.dirs properties.
func (e *attributesExtractor) parseArgs(args []string) (*brokerInfo, error) {
	var info *brokerInfo
	var expectOverride bool
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		if info == nil {
			if arg == brokerClassName {
				info = &brokerInfo{}
			}
			continue
		}
		if expectOverride {
			expectOverride = false
			e.parseOverrideArg(info, arg)
			continue
		}

		if arg == flagOverride {
			expectOverride = true
			continue
		}

		if info.serverProperties == "" && strings.HasSuffix(arg, extProperties) {
			info.serverProperties = arg
		}
	}

	if info == nil {
		return nil, fmt.Errorf("%w: Class (%s) not found in command-line", detector.ErrIncompatibleExtractor, brokerClassName)
	}
	return info, nil
}

// parseOverrideArg parses a single --override argument in the format key=value and updates the broker info if the key
// is broker.id or log.dirs.
func (e *attributesExtractor) parseOverrideArg(info *brokerInfo, arg string) {
	parts := strings.SplitN(arg, "=", 2)
	if len(parts) != 2 {
		return
	}
	key, val := parts[0], parts[1]
	switch key {
	case propertyBrokerID, propertyNodeID:
		info.brokerID = val
	case propertyLogDirs:
		info.logDirs = val
	}
}

// parseServerPropertiesFile reads the server properties file to extract missing broker.id and log.dirs values.
func (e *attributesExtractor) parseServerPropertiesFile(ctx context.Context, process detector.Process, info *brokerInfo) error {
	var err error
	info.serverProperties, err = util.AbsPath(ctx, process, info.serverProperties)
	if err != nil {
		return err
	}
	return e.parsePropertiesFile(info.serverProperties, func(key, value string) bool {
		if (key == propertyBrokerID || key == propertyNodeID) && !info.HasBrokerID() {
			info.brokerID = value
		} else if key == propertyLogDirs && !info.HasLogDirs() {
			info.logDirs = value
		}
		return !(info.HasBrokerID() && info.HasLogDirs())
	})
}

func (e *attributesExtractor) parsePropertiesFile(path string, fn func(key, value string) bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	err = util.ScanProperties(file, propertiesSeparator, fn)
	if err != nil && !errors.Is(err, util.ErrLineLimitExceeded) {
		return err
	}
	return nil
}
