package nvme

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"syscall"
	"unsafe"
)

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

type Histogram struct {
	BinCount uint64
	Bins     [64]HistogramBin
}

type HistogramBin struct {
	Lower uint64
	Upper uint64
	Count uint64
}

// As defined in <linux/nvme_ioctl.h>.
type nvmePassthruCommand struct {
	opcode      uint8
	flags       uint8
	rsvd1       uint16
	nsid        uint32
	cdw2        uint32
	cdw3        uint32
	metadata    uint64
	addr        uint64
	metadataLen uint32
	dataLen     uint32
	cdw10       uint32
	cdw11       uint32
	cdw12       uint32
	cdw13       uint32
	cdw14       uint32
	cdw15       uint32
	timeoutMs   uint32
	result      uint32
}

var (
	ErrInvalidEBSMagic = errors.New("invalid EBS magic number")
	ErrParseLogPage    = errors.New("failed to parse log page")
)

func GetMetrics(devicePath string) (EBSMetrics, error) {
	data, err := getNVMEMetrics(devicePath)
	if err != nil {
		return EBSMetrics{}, err
	}

	return parseLogPage(data)
}

// TODO: normalize some of these names
// getNVMEMetrics retrieves NVMe metrics by reading the log page from the NVMe device at the given path.
func getNVMEMetrics(devicePath string) ([]byte, error) {
	f, err := os.OpenFile(devicePath, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("getNVMEMetrics: error opening device: %w", err)
	}
	defer f.Close()

	data, err := nvmeReadLogPage(f.Fd(), 0xD0)
	if err != nil {
		return nil, fmt.Errorf("getNVMEMetrics: error reading log page %w", err)
	}

	return data, nil
}

// nvmeReadLogPage reads an NVMe log page via an ioctl system call.
func nvmeReadLogPage(fd uintptr, logID uint8) ([]byte, error) {
	data := make([]byte, 4096) // 4096 bytes is the length of the log page.
	bufferLen := len(data)

	if bufferLen > math.MaxUint32 {
		return nil, errors.New("nvmeReadLogPage: bufferLen exceeds MaxUint32")
	}

	cmd := nvmePassthruCommand{
		opcode:  0x02,
		addr:    uint64(uintptr(unsafe.Pointer(&data[0]))),
		nsid:    1,
		dataLen: uint32(bufferLen),
		cdw10:   uint32(logID) | (1024 << 16),
	}

	status, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, 0xC0484E41, uintptr(unsafe.Pointer(&cmd)))
	if errno != 0 {
		return nil, fmt.Errorf("nvmeReadLogPage: ioctl error %w", errno)
	}
	if status != 0 {
		return nil, fmt.Errorf("nvmeReadLogPage: ioctl command failed with status %d", status)
	}
	return data, nil
}

// parseLogPage parses the binary data from an EBS log page into EBSMetrics.
func parseLogPage(data []byte) (EBSMetrics, error) {
	var metrics EBSMetrics
	reader := bytes.NewReader(data)

	if err := binary.Read(reader, binary.LittleEndian, &metrics); err != nil {
		return EBSMetrics{}, fmt.Errorf("%w: %w", ErrParseLogPage, err)
	}

	if metrics.EBSMagic != 0x3C23B510 {
		return EBSMetrics{}, fmt.Errorf("%w: %x", ErrInvalidEBSMagic, metrics.EBSMagic)
	}

	return metrics, nil
}
