// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package instancestore

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/nvme"
)

const InstanceStoreMagic uint32 = 0xEC2C0D7E

var ErrInvalidInstanceStoreMagic = errors.New("invalid Instance Store magic number")

// InstanceStoreMetrics represents the parsed metrics from the NVMe log page.
type InstanceStoreMetrics struct {
	Magic                 uint32
	Reserved              uint32
	ReadOps               uint64
	WriteOps              uint64
	ReadBytes             uint64
	WriteBytes            uint64
	TotalReadTime         uint64
	TotalWriteTime        uint64
	EBSIOPSExceeded       uint64 // Not applicable
	EBSThroughputExceeded uint64 // Not applicable
	EC2IOPSExceeded       uint64
	EC2ThroughputExceeded uint64
	QueueLength           uint64
	NumHistograms         uint32
	NumBins               uint32
	IOSizeRange           [8]uint32
	Bounds                [32]struct {
		Lower uint64
		Upper uint64
	}
	Histograms   [5]HistogramPair
	ReservedArea [888]byte
}

type HistogramPair struct {
	Read  [32]uint64
	Write [32]uint64
}

func (InstanceStoreMetrics) IsNVMeMetrics() {}

// scraper implements DeviceTypeScraper for Instance Store devices.
type scraper struct{}

func NewScraper() *scraper {
	return &scraper{}
}

func (s *scraper) Model() string {
	return "Amazon EC2 NVMe Instance Storage"
}

func (s *scraper) DeviceType() string {
	return "instance_store"
}

func (s *scraper) Identifier(serial string) (string, error) {
	if strings.TrimSpace(serial) == "" {
		return "", fmt.Errorf("empty serial for instance store")
	}
	return serial, nil
}

func (s *scraper) SetResourceAttribute(rb *metadata.ResourceBuilder, identifier string) {
	rb.SetSerialID(identifier)
}

func (s *scraper) ParseRawData(data []byte) (nvme.NVMeMetrics, error) {
	log.Println("Parsing Raw Data Instance Store2")
	if len(data) < 8 {
		return nil, fmt.Errorf("input too short: %w", ErrInvalidInstanceStoreMagic)
	}

	magic32 := binary.LittleEndian.Uint32(data[0:4])
	if magic32 != InstanceStoreMagic {
		return nil, ErrInvalidInstanceStoreMagic
	}

	var metrics InstanceStoreMetrics
	reader := bytes.NewReader(data)
	if err := binary.Read(reader, binary.LittleEndian, &metrics); err != nil {
		return nil, fmt.Errorf("failed to parse log page: %w", err)
	}
	if metrics.Magic != InstanceStoreMagic {
		return nil, ErrInvalidInstanceStoreMagic
	}
	return metrics, nil
}

func (s *scraper) RecordMetrics(recordMetric nvme.RecordMetricFunc, mb *metadata.MetricsBuilder, ts pcommon.Timestamp, metrics nvme.NVMeMetrics) {
	m, ok := metrics.(InstanceStoreMetrics)
	if !ok {
		return
	}
	recordMetric(mb.RecordDiskioInstanceStoreTotalReadOpsDataPoint, ts, m.ReadOps)
	recordMetric(mb.RecordDiskioInstanceStoreTotalWriteOpsDataPoint, ts, m.WriteOps)
	recordMetric(mb.RecordDiskioInstanceStoreTotalReadBytesDataPoint, ts, m.ReadBytes)
	recordMetric(mb.RecordDiskioInstanceStoreTotalWriteBytesDataPoint, ts, m.WriteBytes)
	recordMetric(mb.RecordDiskioInstanceStoreTotalReadTimeDataPoint, ts, m.TotalReadTime)
	recordMetric(mb.RecordDiskioInstanceStoreTotalWriteTimeDataPoint, ts, m.TotalWriteTime)
	recordMetric(mb.RecordDiskioInstanceStoreVolumeQueueLengthDataPoint, ts, m.QueueLength)
	recordMetric(mb.RecordDiskioInstanceStorePerformanceExceededIopsDataPoint, ts, m.EC2IOPSExceeded)
	recordMetric(mb.RecordDiskioInstanceStorePerformanceExceededTpDataPoint, ts, m.EC2ThroughputExceeded)
}

func (s *scraper) IsEnabled(m *metadata.MetricsConfig) bool {
	return m.DiskioInstanceStoreTotalReadOps.Enabled ||
		m.DiskioInstanceStoreTotalWriteOps.Enabled ||
		m.DiskioInstanceStoreTotalReadBytes.Enabled ||
		m.DiskioInstanceStoreTotalWriteBytes.Enabled ||
		m.DiskioInstanceStoreTotalReadTime.Enabled ||
		m.DiskioInstanceStoreTotalWriteTime.Enabled ||
		m.DiskioInstanceStorePerformanceExceededIops.Enabled ||
		m.DiskioInstanceStorePerformanceExceededTp.Enabled ||
		m.DiskioInstanceStoreVolumeQueueLength.Enabled
}
