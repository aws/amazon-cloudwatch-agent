// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package win_perf_counters

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"unsafe"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

var sampleConfig = `
  ## By default this plugin returns basic CPU and Disk statistics.
  ## See the README file for more examples.
  ## Uncomment examples below or write your own as you see fit. If the system
  ## being polled for data does not have the Object at startup of the Telegraf
  ## agent, it will not be gathered.
  ## Settings:
  # PrintValid = false # Print All matching performance counters
  # DisableReplacer = false # Disable the name replacer

  [[inputs.win_perf_counters.object]]
    # Processor usage, alternative to native, reports on a per core.
    ObjectName = "Processor"
    Instances = ["*"]
    Counters = [
      "%% Idle Time", "%% Interrupt Time",
      "%% Privileged Time", "%% User Time",
      "%% Processor Time"
    ]
    Measurement = "win_cpu"
    # Set to true to include _Total instance when querying for all (*).
    # IncludeTotal=false
    # Print out when the performance counter is missing from object, counter or instance.
    # WarnOnMissing = false

  [[inputs.win_perf_counters.object]]
    # Disk times and queues
    ObjectName = "LogicalDisk"
    Instances = ["*"]
    Counters = [
      "%% Idle Time", "%% Disk Time","%% Disk Read Time",
      "%% Disk Write Time", "%% User Time", "Current Disk Queue Length"
    ]
    Measurement = "win_disk"

  [[inputs.win_perf_counters.object]]
    ObjectName = "System"
    Counters = ["Context Switches/sec","System Calls/sec"]
    Instances = ["------"]
    Measurement = "win_system"

  [[inputs.win_perf_counters.object]]
    # Example query where the Instance portion must be removed to get data back,
    # such as from the Memory object.
    ObjectName = "Memory"
    Counters = [
      "Available Bytes", "Cache Faults/sec", "Demand Zero Faults/sec",
      "Page Faults/sec", "Pages/sec", "Transition Faults/sec",
      "Pool Nonpaged Bytes", "Pool Paged Bytes"
    ]
    Instances = ["------"] # Use 6 x - to remove the Instance bit from the query.
    Measurement = "win_mem"
`

type Win_PerfCounters struct {
	configParsed    bool
	PrintValid      bool
	DisableReplacer bool
	TestName        string
	PreVistaSupport bool
	Object          []perfobject
	// Valid queries end up in this map.
	gItemList        map[int]*item
	testConfigParsed bool
	testObject       string
}

type perfobject struct {
	ObjectName    string
	Counters      []string
	Instances     []string
	Measurement   string
	WarnOnMissing bool
	FailOnMissing bool
	IncludeTotal  bool
}

// Parsed configuration ends up here after it has been validated for valid
// Performance Counter paths
type itemList struct {
	items map[int]*item
}

type item struct {
	query         string
	objectName    string
	counter       string
	instance      string
	measurement   string
	include_total bool
	initialized   bool
	handle        PDH_HQUERY
	counterHandle PDH_HCOUNTER
}

func (item *item) init() error {
	if item.initialized {
		return nil
	}

	var handle PDH_HQUERY
	var counterHandle PDH_HCOUNTER
	ret := PdhOpenQuery(0, 0, &handle)
	if ret != ERROR_SUCCESS {
		return errors.New(PdhFormatError(ret))
	}
	ret = PdhAddEnglishCounter(handle, item.query, 0, &counterHandle)
	if ret != ERROR_SUCCESS {
		PdhCloseQuery(handle)
		return errors.New(PdhFormatError(ret))
	}

	item.handle = handle
	item.counterHandle = counterHandle
	item.initialized = true

	return nil
}

var sanitizedChars = strings.NewReplacer("/sec", "_persec", "/Sec", "_persec",
	" ", "_", "%", "Percent", `\`, "")

func (m *Win_PerfCounters) convertName(input string) string {
	if m.DisableReplacer {
		return input
	}
	return sanitizedChars.Replace(input)
}

func (m *Win_PerfCounters) AddItem(metrics *itemList, query string, objectName string, counter string, instance string,
	measurement string, include_total bool) error {

	var handle PDH_HQUERY
	var counterHandle PDH_HCOUNTER

	temp := &item{query, objectName, counter, instance, measurement,
		include_total, false, handle, counterHandle}
	index := len(m.gItemList)
	m.gItemList[index] = temp

	if metrics.items == nil {
		metrics.items = make(map[int]*item)
	}
	metrics.items[index] = temp
	return temp.init()
}

func (m *Win_PerfCounters) Description() string {
	return "Input plugin to query Performance Counters on Windows operating systems"
}

func (m *Win_PerfCounters) SampleConfig() string {
	return sampleConfig
}

func (m *Win_PerfCounters) ParseConfig(metrics *itemList) error {
	var query string

	m.configParsed = true
	m.gItemList = make(map[int]*item)

	if len(m.Object) > 0 {
		for _, PerfObject := range m.Object {
			for _, counter := range PerfObject.Counters {
				for _, instance := range PerfObject.Instances {
					objectname := PerfObject.ObjectName

					if instance == "------" {
						query = "\\" + objectname + "\\" + counter
					} else {
						query = "\\" + objectname + "(" + instance + ")\\" + counter
					}

					err := m.AddItem(metrics, query, objectname, counter, instance,
						PerfObject.Measurement, PerfObject.IncludeTotal)

					if err == nil {
						if m.PrintValid {
							fmt.Printf("Valid: %s\n", query)
						}
					} else {
						if PerfObject.FailOnMissing || PerfObject.WarnOnMissing {
							fmt.Printf("Invalid query: '%s'. Error: %s", query, err.Error())
						}
						if PerfObject.FailOnMissing {
							return err
						}
					}
				}
			}
		}

		return nil
	} else {
		err := errors.New("No performance objects configured!")
		return err
	}
}

func (m *Win_PerfCounters) Cleanup(metrics *itemList) {
	// Cleanup

	for _, metric := range metrics.items {
		ret := PdhCloseQuery(metric.handle)
		_ = ret
	}
}

func (m *Win_PerfCounters) CleanupTestMode() {
	// Cleanup for the testmode.

	for _, metric := range m.gItemList {
		ret := PdhCloseQuery(metric.handle)
		_ = ret
	}
}

func (m *Win_PerfCounters) Gather(acc telegraf.Accumulator) error {
	metrics := itemList{}

	// Both values are empty in normal use.
	if m.TestName != m.testObject {
		// Cleanup any handles before emptying the global variable containing valid queries.
		m.CleanupTestMode()
		m.gItemList = make(map[int]*item)
		m.testObject = m.TestName
		m.testConfigParsed = true
		m.configParsed = false
	}

	// We only need to parse the config during the init, it uses the global variable after.
	if m.configParsed == false {

		err := m.ParseConfig(&metrics)
		if err != nil {
			return err
		}
	}

	var bufSize uint32
	var bufCount uint32
	var size uint32 = uint32(unsafe.Sizeof(PDH_FMT_COUNTERVALUE_ITEM_DOUBLE{}))
	var emptyBuf [1]PDH_FMT_COUNTERVALUE_ITEM_DOUBLE // need at least 1 addressable null ptr.

	// For iterate over the known metrics and get the samples.
	for _, metric := range m.gItemList {
		if !metric.initialized {
			if err := metric.init(); err != nil {
				log.Printf("D! metric init has error: %v", err)
				continue
			}
		}
		// collect
		ret := PdhCollectQueryData(metric.handle)
		if ret == ERROR_SUCCESS {
			ret = PdhGetFormattedCounterArrayDouble(metric.counterHandle, &bufSize,
				&bufCount, &emptyBuf[0]) // uses null ptr here according to MSDN.
			if ret == PDH_MORE_DATA {
				filledBuf := make([]PDH_FMT_COUNTERVALUE_ITEM_DOUBLE, bufCount*size)
				if len(filledBuf) == 0 {
					continue
				}
				ret = PdhGetFormattedCounterArrayDouble(metric.counterHandle,
					&bufSize, &bufCount, &filledBuf[0])
				for i := 0; i < int(bufCount); i++ {
					c := filledBuf[i]
					var s string = UTF16PtrToString(c.SzName)

					var add bool

					if metric.include_total {
						// If IncludeTotal is set, include all.
						add = true
					} else if metric.instance == "*" && !strings.Contains(s, "_Total") {
						// Catch if set to * and that it is not a '*_Total*' instance.
						add = true
					} else if metric.instance == s {
						// Catch if we set it to total or some form of it
						add = true
					} else if strings.Contains(metric.instance, "#") && strings.HasPrefix(metric.instance, s) {
						// If you are using a multiple instance identifier such as "w3wp#1"
						// phd.dll returns only the first 2 characters of the identifier.
						add = true
						s = metric.instance
					} else if metric.instance == "------" {
						add = true
					}

					if add {
						fields := make(map[string]interface{})
						tags := make(map[string]string)
						if s != "" {
							tags["instance"] = s
						}
						tags["objectname"] = metric.objectName
						fields[m.convertName(metric.counter)] =
							float32(c.FmtValue.DoubleValue)

						measurement := m.convertName(metric.measurement)
						if measurement == "" {
							measurement = "win_perf_counters"
						}
						acc.AddFields(measurement, fields, tags)
					}
				}

				filledBuf = nil
				// Need to at least set bufSize to zero, because if not, the function will not
				// return PDH_MORE_DATA and will not set the bufSize.
				bufCount = 0
				bufSize = 0
			}
		}
	}

	return nil
}

func init() {
	inputs.Add("win_perf_counters", func() telegraf.Input { return &Win_PerfCounters{} })
}
