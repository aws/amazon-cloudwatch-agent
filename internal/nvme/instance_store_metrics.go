// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unsafe"
)

// EBSMetrics represents the parsed metrics from the EBS NVMe log page 0xD0.
// This structure matches the binary layout with histogram support.
type EBSMetrics struct {
	EBSMagic              uint64    // Magic number for EBS validation (0x3C23B510)
	ReadOps               uint64    // Cumulative read operations counter
	WriteOps              uint64    // Cumulative write operations counter
	ReadBytes             uint64    // Cumulative read bytes counter
	WriteBytes            uint64    // Cumulative write bytes counter
	TotalReadTime         uint64    // Cumulative read time (nanoseconds)
	TotalWriteTime        uint64    // Cumulative write time (nanoseconds)
	EBSIOPSExceeded       uint64    // EBS volume IOPS exceeded counter
	EBSThroughputExceeded uint64    // EBS volume throughput exceeded counter
	EC2IOPSExceeded       uint64    // EC2 instance IOPS exceeded counter
	EC2ThroughputExceeded uint64    // EC2 instance throughput exceeded counter
	QueueLength           uint64    // Current queue length (gauge)
	ReservedArea          [416]byte // Reserved area in log page
	ReadLatency           Histogram // Read latency histogram
	WriteLatency          Histogram // Write latency histogram
}

// InstanceStoreMetrics represents the parsed metrics from the Instance Store NVMe log page 0xC0.
// Similar to EBS but skips EBS-specific fields (EBSIOPSExceeded, EBSThroughputExceeded).
type InstanceStoreMetrics struct {
	Magic                 uint32     // Magic number for Instance Store validation (0xEC2C0D7E)
	Reserved              uint32     // Reserved field
	ReadOps               uint64     // Cumulative read operations counter
	WriteOps              uint64     // Cumulative write operations counter
	ReadBytes             uint64     // Cumulative read bytes counter
	WriteBytes            uint64     // Cumulative write bytes counter
	TotalReadTime         uint64     // Cumulative read time (nanoseconds)
	TotalWriteTime        uint64     // Cumulative write time (nanoseconds)
	EBSIOPSExceeded       uint64     // Skip - not applicable to Instance Store
	EBSThroughputExceeded uint64     // Skip - not applicable to Instance Store
	EC2IOPSExceeded       uint64     // EC2 instance IOPS exceeded counter
	EC2ThroughputExceeded uint64     // EC2 instance throughput exceeded counter
	QueueLength           uint64     // Current queue length (gauge)
	NumHistograms         uint64     // Number of histograms in the log page
	NumBins               uint64     // Number of bins per histogram
	IOSizeRange           uint64     // I/O size range for histograms
	Bounds                [64]uint64 // Histogram bounds
	ReadLatency           Histogram  // Read latency histogram
	WriteLatency          Histogram  // Write latency histogram
	ReservedArea          [416]byte  // Reserved area in log page
}

// Histogram represents latency histogram data from NVMe log pages.
type Histogram struct {
	BinCount uint64           // Number of bins in the histogram
	Bins     [64]HistogramBin // Histogram bins
}

// HistogramBin represents a single bin in a latency histogram.
type HistogramBin struct {
	Lower uint64 // Lower bound of the bin
	Upper uint64 // Upper bound of the bin
	Count uint64 // Count of operations in this bin
}

const (
	// EBSMagicNumber is the magic number that identifies EBS devices
	EBSMagicNumber = 0x3C23B510
	// InstanceStoreMagicNumber is the magic number that identifies Instance Store devices
	InstanceStoreMagicNumber = 0xEC2C0D7E
)

var (
	ErrInvalidEBSMagic           = errors.New("invalid EBS magic number")
	ErrInvalidInstanceStoreMagic = errors.New("invalid Instance Store magic number")
	ErrParseEBSLogPage           = errors.New("failed to parse EBS log page")
	ErrParseInstanceStoreLogPage = errors.New("failed to parse Instance Store log page")
	ErrInsufficientData          = errors.New("insufficient data for parsing")
	ErrDeviceAccess              = errors.New("device access failed")
	ErrIoctlFailed               = errors.New("ioctl operation failed")
	ErrInsufficientPermissions   = errors.New("insufficient permissions for device access")
	ErrDeviceNotFound            = errors.New("device not found")
	ErrBufferOverflow            = errors.New("buffer overflow detected")
)

// SafeUint64ToInt64 converts a uint64 value to int64 with overflow detection.
// Returns an error if the value exceeds the maximum int64 value.
func SafeUint64ToInt64(value uint64) (int64, error) {
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("value %d is too large for int64", value)
	}
	return int64(value), nil
}

// ParseEBSLogPage parses the binary data from EBS log page 0xD0 into EBSMetrics.
// It validates the magic number and uses binary.LittleEndian for parsing with comprehensive bounds checking.
func ParseEBSLogPage(data []byte) (EBSMetrics, error) {
	// Validate input data is not nil
	if data == nil {
		return EBSMetrics{}, fmt.Errorf("%w: input data is nil", ErrInsufficientData)
	}

	// Validate minimum data length with safety margin
	minRequiredSize := int(unsafe.Sizeof(EBSMetrics{}))
	if len(data) < minRequiredSize {
		return EBSMetrics{}, fmt.Errorf("%w: data length %d is insufficient for EBSMetrics structure (minimum required: %d)",
			ErrInsufficientData, len(data), minRequiredSize)
	}

	// Validate maximum data length to prevent potential buffer overflow
	const maxLogPageSize = 8192 // 8KB maximum for safety
	if len(data) > maxLogPageSize {
		return EBSMetrics{}, fmt.Errorf("%w: data length %d exceeds maximum allowed size %d",
			ErrBufferOverflow, len(data), maxLogPageSize)
	}

	// Validate data contains expected log page size (4KB for raw log page)
	// Note: We accept the minimum structure size since log pages can vary
	const expectedLogPageSize = 4096
	if len(data) < expectedLogPageSize {
		return EBSMetrics{}, fmt.Errorf("%w: log page size %d is less than expected minimum %d",
			ErrInsufficientData, len(data), expectedLogPageSize)
	}

	var metrics EBSMetrics
	reader := bytes.NewReader(data)

	if err := binary.Read(reader, binary.LittleEndian, &metrics); err != nil {
		return EBSMetrics{}, fmt.Errorf("%w: binary parsing failed: %w", ErrParseEBSLogPage, err)
	}

	// Validate the magic number to confirm this is an EBS device
	if metrics.EBSMagic != EBSMagicNumber {
		return EBSMetrics{}, fmt.Errorf("%w: expected 0x%X, got 0x%X",
			ErrInvalidEBSMagic, EBSMagicNumber, metrics.EBSMagic)
	}

	// Validate metric values are within reasonable bounds to detect corruption
	if err := validateEBSMetricBounds(&metrics); err != nil {
		return EBSMetrics{}, fmt.Errorf("EBS metric validation failed: %w", err)
	}

	return metrics, nil
}

// ParseInstanceStoreLogPage parses the binary data from Instance Store log page 0xC0 into InstanceStoreMetrics.
// It validates the magic number (0xEC2C0D7E) and uses binary.LittleEndian for parsing with comprehensive bounds checking.
func ParseInstanceStoreLogPage(data []byte) (InstanceStoreMetrics, error) {
	// Validate input data is not nil
	if data == nil {
		return InstanceStoreMetrics{}, fmt.Errorf("%w: input data is nil", ErrInsufficientData)
	}

	// Validate minimum data length with safety margin
	minRequiredSize := int(unsafe.Sizeof(InstanceStoreMetrics{}))
	if len(data) < minRequiredSize {
		return InstanceStoreMetrics{}, fmt.Errorf("%w: data length %d is insufficient for InstanceStoreMetrics structure (minimum required: %d)",
			ErrInsufficientData, len(data), minRequiredSize)
	}

	// Validate maximum data length to prevent potential buffer overflow
	const maxLogPageSize = 8192 // 8KB maximum for safety
	if len(data) > maxLogPageSize {
		return InstanceStoreMetrics{}, fmt.Errorf("%w: data length %d exceeds maximum allowed size %d",
			ErrBufferOverflow, len(data), maxLogPageSize)
	}

	// Validate data contains expected log page size (4KB for raw log page)
	// Note: We accept the minimum structure size since log pages can vary
	const expectedLogPageSize = 4096
	if len(data) < expectedLogPageSize {
		return InstanceStoreMetrics{}, fmt.Errorf("%w: log page size %d is less than expected minimum %d",
			ErrInsufficientData, len(data), expectedLogPageSize)
	}

	var metrics InstanceStoreMetrics
	reader := bytes.NewReader(data)

	if err := binary.Read(reader, binary.LittleEndian, &metrics); err != nil {
		return InstanceStoreMetrics{}, fmt.Errorf("%w: binary parsing failed: %w", ErrParseInstanceStoreLogPage, err)
	}

	// Validate the magic number to confirm this is an Instance Store device
	if metrics.Magic != InstanceStoreMagicNumber {
		return InstanceStoreMetrics{}, fmt.Errorf("%w: expected 0x%X, got 0x%X",
			ErrInvalidInstanceStoreMagic, InstanceStoreMagicNumber, metrics.Magic)
	}

	// Validate metric values are within reasonable bounds to detect corruption
	if err := validateInstanceStoreMetricBounds(&metrics); err != nil {
		return InstanceStoreMetrics{}, fmt.Errorf("Instance Store metric validation failed: %w", err)
	}

	return metrics, nil
}

// validateEBSMetricBounds validates that EBS metric values are within reasonable bounds
// to detect potential data corruption or malicious input
func validateEBSMetricBounds(metrics *EBSMetrics) error {
	// Define reasonable upper bounds for metrics to detect corruption
	const (
		maxReasonableOps      = uint64(1e12) // 1 trillion operations
		maxReasonableBytes    = uint64(1e18) // 1 exabyte
		maxReasonableTime     = uint64(1e18) // ~31 years in nanoseconds
		maxReasonableExceeded = uint64(1e12) // 1 trillion exceeded events
		maxReasonableQueueLen = uint64(1e6)  // 1 million queue length
	)

	// Validate operation counters
	if metrics.ReadOps > maxReasonableOps {
		return fmt.Errorf("ReadOps value %d exceeds reasonable maximum %d", metrics.ReadOps, maxReasonableOps)
	}
	if metrics.WriteOps > maxReasonableOps {
		return fmt.Errorf("WriteOps value %d exceeds reasonable maximum %d", metrics.WriteOps, maxReasonableOps)
	}

	// Validate byte counters
	if metrics.ReadBytes > maxReasonableBytes {
		return fmt.Errorf("ReadBytes value %d exceeds reasonable maximum %d", metrics.ReadBytes, maxReasonableBytes)
	}
	if metrics.WriteBytes > maxReasonableBytes {
		return fmt.Errorf("WriteBytes value %d exceeds reasonable maximum %d", metrics.WriteBytes, maxReasonableBytes)
	}

	// Validate time counters
	if metrics.TotalReadTime > maxReasonableTime {
		return fmt.Errorf("TotalReadTime value %d exceeds reasonable maximum %d", metrics.TotalReadTime, maxReasonableTime)
	}
	if metrics.TotalWriteTime > maxReasonableTime {
		return fmt.Errorf("TotalWriteTime value %d exceeds reasonable maximum %d", metrics.TotalWriteTime, maxReasonableTime)
	}

	// Validate exceeded counters
	if metrics.EBSIOPSExceeded > maxReasonableExceeded {
		return fmt.Errorf("EBSIOPSExceeded value %d exceeds reasonable maximum %d", metrics.EBSIOPSExceeded, maxReasonableExceeded)
	}
	if metrics.EBSThroughputExceeded > maxReasonableExceeded {
		return fmt.Errorf("EBSThroughputExceeded value %d exceeds reasonable maximum %d", metrics.EBSThroughputExceeded, maxReasonableExceeded)
	}
	if metrics.EC2IOPSExceeded > maxReasonableExceeded {
		return fmt.Errorf("EC2IOPSExceeded value %d exceeds reasonable maximum %d", metrics.EC2IOPSExceeded, maxReasonableExceeded)
	}
	if metrics.EC2ThroughputExceeded > maxReasonableExceeded {
		return fmt.Errorf("EC2ThroughputExceeded value %d exceeds reasonable maximum %d", metrics.EC2ThroughputExceeded, maxReasonableExceeded)
	}

	// Validate queue length
	if metrics.QueueLength > maxReasonableQueueLen {
		return fmt.Errorf("QueueLength value %d exceeds reasonable maximum %d", metrics.QueueLength, maxReasonableQueueLen)
	}

	// Validate histogram bounds
	if err := validateHistogramBounds(&metrics.ReadLatency); err != nil {
		return fmt.Errorf("ReadLatency histogram validation failed: %w", err)
	}
	if err := validateHistogramBounds(&metrics.WriteLatency); err != nil {
		return fmt.Errorf("WriteLatency histogram validation failed: %w", err)
	}

	return nil
}

// validateInstanceStoreMetricBounds validates that Instance Store metric values are within reasonable bounds
// to detect potential data corruption or malicious input
func validateInstanceStoreMetricBounds(metrics *InstanceStoreMetrics) error {
	// Define reasonable upper bounds for metrics to detect corruption
	const (
		maxReasonableOps        = uint64(1e12) // 1 trillion operations
		maxReasonableBytes      = uint64(1e18) // 1 exabyte
		maxReasonableTime       = uint64(1e18) // ~31 years in nanoseconds
		maxReasonableExceeded   = uint64(1e12) // 1 trillion exceeded events
		maxReasonableQueueLen   = uint64(1e6)  // 1 million queue length
		maxReasonableHistograms = uint64(10)   // Maximum number of histograms
		maxReasonableBins       = uint64(256)  // Maximum number of bins per histogram
	)

	// Validate operation counters
	if metrics.ReadOps > maxReasonableOps {
		return fmt.Errorf("ReadOps value %d exceeds reasonable maximum %d", metrics.ReadOps, maxReasonableOps)
	}
	if metrics.WriteOps > maxReasonableOps {
		return fmt.Errorf("WriteOps value %d exceeds reasonable maximum %d", metrics.WriteOps, maxReasonableOps)
	}

	// Validate byte counters
	if metrics.ReadBytes > maxReasonableBytes {
		return fmt.Errorf("ReadBytes value %d exceeds reasonable maximum %d", metrics.ReadBytes, maxReasonableBytes)
	}
	if metrics.WriteBytes > maxReasonableBytes {
		return fmt.Errorf("WriteBytes value %d exceeds reasonable maximum %d", metrics.WriteBytes, maxReasonableBytes)
	}

	// Validate time counters
	if metrics.TotalReadTime > maxReasonableTime {
		return fmt.Errorf("TotalReadTime value %d exceeds reasonable maximum %d", metrics.TotalReadTime, maxReasonableTime)
	}
	if metrics.TotalWriteTime > maxReasonableTime {
		return fmt.Errorf("TotalWriteTime value %d exceeds reasonable maximum %d", metrics.TotalWriteTime, maxReasonableTime)
	}

	// Validate exceeded counters (skip EBS-specific fields for Instance Store)
	if metrics.EC2IOPSExceeded > maxReasonableExceeded {
		return fmt.Errorf("EC2IOPSExceeded value %d exceeds reasonable maximum %d", metrics.EC2IOPSExceeded, maxReasonableExceeded)
	}
	if metrics.EC2ThroughputExceeded > maxReasonableExceeded {
		return fmt.Errorf("EC2ThroughputExceeded value %d exceeds reasonable maximum %d", metrics.EC2ThroughputExceeded, maxReasonableExceeded)
	}

	// Validate queue length
	if metrics.QueueLength > maxReasonableQueueLen {
		return fmt.Errorf("QueueLength value %d exceeds reasonable maximum %d", metrics.QueueLength, maxReasonableQueueLen)
	}

	// Validate histogram metadata
	if metrics.NumHistograms > maxReasonableHistograms {
		return fmt.Errorf("NumHistograms value %d exceeds reasonable maximum %d", metrics.NumHistograms, maxReasonableHistograms)
	}
	if metrics.NumBins > maxReasonableBins {
		return fmt.Errorf("NumBins value %d exceeds reasonable maximum %d", metrics.NumBins, maxReasonableBins)
	}

	// Validate histogram bounds
	if err := validateHistogramBounds(&metrics.ReadLatency); err != nil {
		return fmt.Errorf("ReadLatency histogram validation failed: %w", err)
	}
	if err := validateHistogramBounds(&metrics.WriteLatency); err != nil {
		return fmt.Errorf("WriteLatency histogram validation failed: %w", err)
	}

	return nil
}

// validateHistogramBounds validates histogram data for potential corruption or malicious input
func validateHistogramBounds(histogram *Histogram) error {
	const (
		maxReasonableBinCount = uint64(256)  // Maximum reasonable bin count
		maxReasonableBinValue = uint64(1e18) // Maximum reasonable bin value
	)

	// Validate bin count
	if histogram.BinCount > maxReasonableBinCount {
		return fmt.Errorf("BinCount value %d exceeds reasonable maximum %d", histogram.BinCount, maxReasonableBinCount)
	}

	// Validate individual bins up to the declared bin count
	binCount := histogram.BinCount
	if binCount > uint64(len(histogram.Bins)) {
		binCount = uint64(len(histogram.Bins))
	}

	for i := uint64(0); i < binCount; i++ {
		bin := &histogram.Bins[i]

		// Validate bin bounds
		if bin.Lower > maxReasonableBinValue {
			return fmt.Errorf("bin %d Lower value %d exceeds reasonable maximum %d", i, bin.Lower, maxReasonableBinValue)
		}
		if bin.Upper > maxReasonableBinValue {
			return fmt.Errorf("bin %d Upper value %d exceeds reasonable maximum %d", i, bin.Upper, maxReasonableBinValue)
		}
		if bin.Count > maxReasonableBinValue {
			return fmt.Errorf("bin %d Count value %d exceeds reasonable maximum %d", i, bin.Count, maxReasonableBinValue)
		}

		// Validate logical consistency: Lower should be <= Upper
		if bin.Lower > bin.Upper {
			return fmt.Errorf("bin %d has invalid bounds: Lower %d > Upper %d", i, bin.Lower, bin.Upper)
		}
	}

	return nil
}
