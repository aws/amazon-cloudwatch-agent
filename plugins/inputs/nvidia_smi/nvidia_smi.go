// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:generate ../../../tools/readme_config_includer/generator
package nvidia_smi

import (
	"bytes"
	_ "embed"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/plugins/inputs"

	"github.com/aws/amazon-cloudwatch-agent/internal"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/nvidia_smi/schema_v11"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/nvidia_smi/schema_v12"
)

//go:embed sample.conf
var sampleConfig string

// NvidiaSMI holds the methods for this plugin
type NvidiaSMI struct {
	BinPath              string          `toml:"bin_path"`
	Timeout              config.Duration `toml:"timeout"`
	StartupErrorBehavior string          `toml:"startup_error_behavior"`
	Log                  telegraf.Logger `toml:"-"`

	ignorePlugin bool
	once         sync.Once
}

// Description returns the description of the NvidiaSMI plugin
func (smi *NvidiaSMI) Description() string {
	return "Pulls statistics from nvidia GPUs attached to the host"
}

func (*NvidiaSMI) SampleConfig() string {
	return sampleConfig
}

func (smi *NvidiaSMI) Init() error {
	if _, err := os.Stat(smi.BinPath); os.IsNotExist(err) {
		binPath, err := exec.LookPath("nvidia-smi")
		if err != nil {
			switch smi.StartupErrorBehavior {
			case "ignore":
				smi.ignorePlugin = true
				smi.Log.Warnf("nvidia-smi not found on the system, ignoring: %s", err)
				return nil
			case "", "error":
				return fmt.Errorf("nvidia-smi not found in %q and not in PATH; please make sure nvidia-smi is installed and/or is in PATH", smi.BinPath)
			default:
				return fmt.Errorf("unknown startup behavior setting: %s", smi.StartupErrorBehavior)
			}
		}
		smi.BinPath = binPath
	}

	return nil
}

// Gather implements the telegraf interface
func (smi *NvidiaSMI) Gather(acc telegraf.Accumulator) error {
	if smi.ignorePlugin {
		return nil
	}

	// Construct and execute metrics query
	data, err := internal.CombinedOutputTimeout(exec.Command(smi.BinPath, "-q", "-x"), time.Duration(smi.Timeout))
	if err != nil {
		return fmt.Errorf("calling %q failed: %w", smi.BinPath, err)
	}

	// Parse the output
	return smi.parse(acc, data)
}

func (smi *NvidiaSMI) parse(acc telegraf.Accumulator, data []byte) error {
	schema := "v11"

	buf := bytes.NewBuffer(data)
	decoder := xml.NewDecoder(buf)
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("reading token failed: %w", err)
		}
		d, ok := token.(xml.Directive)
		if !ok {
			continue
		}
		directive := string(d)
		if !strings.HasPrefix(directive, "DOCTYPE") {
			continue
		}
		parts := strings.Split(directive, " ")
		s := strings.Trim(parts[len(parts)-1], "\" ")
		if strings.HasPrefix(s, "nvsmi_device_") && strings.HasSuffix(s, ".dtd") {
			schema = strings.TrimSuffix(strings.TrimPrefix(s, "nvsmi_device_"), ".dtd")
		} else {
			smi.Log.Debugf("Cannot find schema version in %q", directive)
		}
		break
	}
	smi.Log.Debugf("Using schema version in %s", schema)

	switch schema {
	case "v10", "v11":
		return schema_v11.Parse(acc, data)
	case "v12":
		return schema_v12.Parse(acc, data)
	}

	smi.once.Do(func() {
		smi.Log.Warnf(`Unknown schema version %q, using latest know schema for parsing.
		Please report this as an issue to https://github.com/influxdata/telegraf together
		with a sample output of 'nvidia_smi -q -x'!`, schema)
	})
	return schema_v12.Parse(acc, data)
}

func init() {
	inputs.Add("nvidia_smi", func() telegraf.Input {
		return &NvidiaSMI{
			BinPath: "/usr/bin/nvidia-smi",
			Timeout: config.Duration(5 * time.Second),
		}
	})
}
