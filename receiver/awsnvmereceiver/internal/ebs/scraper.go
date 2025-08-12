// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ebs

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

const EbsMagic uint64 = 0x3C23B510

var ErrInvalidEbsMagic = errors.New("invalid EBS magic number")

// EBSMetrics represents the parsed metrics from the NVMe log page.
type EBSMetrics struct {
	EBSMagic              uint64
	ReadOps               uint64
	WriteOps              uint64
	ReadBytes             uint64
	WriteBytes            uint64
	TotalReadTime         uint64
	TotalWriteTime        uint64
	EBSIOPSExceeded       uint64
	EBSThroughputExceeded uint64
	EC2IOPSExceeded       uint64
	EC2ThroughputExceeded uint64
	QueueLength           uint64
	ReservedArea          [416]byte
	ReadLatency           Histogram
	WriteLatency          Histogram
}

func (EBSMetrics) IsNVMeMetrics() {}

// Histogram holds latency distribution bins.
type Histogram struct {
	BinCount uint64
	Bins     [64]HistogramBin
}

type HistogramBin struct {
	Lower uint64
	Upper uint64
	Count uint64
}

// scraper implements DeviceTypeScraper for EBS devices.
type scraper struct{}

func NewScraper() *scraper {
	return &scraper{}
}

func (s *scraper) Model() string {
	return "Amazon Elastic Block Store"
}

func (s *scraper) DeviceType() string {
	return "ebs"
}

func (s *scraper) Identifier(serial string) (string, error) {
	if !strings.HasPrefix(serial, "vol") || len(serial) < 4 {
		return "", fmt.Errorf("invalid EBS serial: %s", serial)
	}
	return fmt.Sprintf("vol-%s", serial[3:]), nil
}

func (s *scraper) SetResourceAttribute(rb *metadata.ResourceBuilder, identifier string) {
	rb.SetVolumeID(identifier)
}

func (s *scraper) ParseRawData(data []byte) (nvme.NVMeMetrics, error) {
	log.Println("Parsing Raw Data EBS2")
	if len(data) < 8 {
		return nil, fmt.Errorf("input too short: %w", ErrInvalidEbsMagic)
	}

	magic64 := binary.LittleEndian.Uint64(data[0:8])
	if magic64 != EbsMagic {
		return nil, ErrInvalidEbsMagic
	}

	var metrics EBSMetrics
	reader := bytes.NewReader(data)
	if err := binary.Read(reader, binary.LittleEndian, &metrics); err != nil {
		return nil, fmt.Errorf("failed to parse log page: %w", err)
	}
	if metrics.EBSMagic != EbsMagic {
		return nil, ErrInvalidEbsMagic
	}
	return metrics, nil
}

func (s *scraper) RecordMetrics(recordMetric nvme.RecordMetricFunc, mb *metadata.MetricsBuilder, ts pcommon.Timestamp, metrics nvme.NVMeMetrics) {
	m, ok := metrics.(EBSMetrics)
	if !ok {
		return
	}
	recordMetric(mb.RecordDiskioEbsTotalReadOpsDataPoint, ts, m.ReadOps)
	recordMetric(mb.RecordDiskioEbsTotalWriteOpsDataPoint, ts, m.WriteOps)
	recordMetric(mb.RecordDiskioEbsTotalReadBytesDataPoint, ts, m.ReadBytes)
	recordMetric(mb.RecordDiskioEbsTotalWriteBytesDataPoint, ts, m.WriteBytes)
	recordMetric(mb.RecordDiskioEbsTotalReadTimeDataPoint, ts, m.TotalReadTime)
	recordMetric(mb.RecordDiskioEbsTotalWriteTimeDataPoint, ts, m.TotalWriteTime)
	recordMetric(mb.RecordDiskioEbsVolumeQueueLengthDataPoint, ts, m.QueueLength)
	recordMetric(mb.RecordDiskioEbsVolumePerformanceExceededIopsDataPoint, ts, m.EBSIOPSExceeded)
	recordMetric(mb.RecordDiskioEbsVolumePerformanceExceededTpDataPoint, ts, m.EBSThroughputExceeded)
	recordMetric(mb.RecordDiskioEbsEc2InstancePerformanceExceededIopsDataPoint, ts, m.EC2IOPSExceeded)
	recordMetric(mb.RecordDiskioEbsEc2InstancePerformanceExceededTpDataPoint, ts, m.EC2ThroughputExceeded)
}

func (s *scraper) IsEnabled(m *metadata.MetricsConfig) bool {
	return m.DiskioEbsTotalReadOps.Enabled ||
		m.DiskioEbsTotalWriteOps.Enabled ||
		m.DiskioEbsTotalReadBytes.Enabled ||
		m.DiskioEbsTotalWriteBytes.Enabled ||
		m.DiskioEbsTotalReadTime.Enabled ||
		m.DiskioEbsTotalWriteTime.Enabled ||
		m.DiskioEbsVolumePerformanceExceededIops.Enabled ||
		m.DiskioEbsVolumePerformanceExceededTp.Enabled ||
		m.DiskioEbsEc2InstancePerformanceExceededIops.Enabled ||
		m.DiskioEbsEc2InstancePerformanceExceededTp.Enabled ||
		m.DiskioEbsVolumeQueueLength.Enabled
}
