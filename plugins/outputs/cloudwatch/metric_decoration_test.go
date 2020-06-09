package cloudwatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMetricDecorations(t *testing.T) {
	expected := make([]MetricDecorationConfig, 0)

	mdc := MetricDecorationConfig{
		Category: "cpu",
		Metric:   "cpu",
		Rename:   "CPU",
		Unit:     "Percent",
	}
	expected = append(expected, mdc)

	mdc = MetricDecorationConfig{
		Category: "mem",
		Metric:   "mem",
		Unit:     "Megabytes",
	}
	expected = append(expected, mdc)

	mdc = MetricDecorationConfig{
		Category: "disk",
		Metric:   "disk",
		Rename:   "DISK",
	}
	expected = append(expected, mdc)

	m, err := NewMetricDecorations(expected)
	assert.True(t, err == nil)

	assert.Equal(t, "CPU", m.getRename("cpu", "cpu"))
	assert.Equal(t, "Percent", m.getUnit("cpu", "cpu"))
	assert.Equal(t, "Megabytes", m.getUnit("mem", "mem"))
	assert.Equal(t, "DISK", m.getRename("disk", "disk"))
}

func TestNewMetricDecorationsAbnormal(t *testing.T) {
	expected := make([]MetricDecorationConfig, 0)

	mdc := MetricDecorationConfig{
		Category: "cpu",
		Metric:   "cpu",
		Rename:   "CPU",
		Unit:     "InvalidUnit",
	}
	expected = append(expected, mdc)

	_, err := NewMetricDecorations(expected)
	assert.True(t, err != nil)

	_, err = NewMetricDecorations(nil)
	assert.True(t, err == nil)
}

func TestNewMetricDecorationsSpecialCharacter(t *testing.T) {
	expected := make([]MetricDecorationConfig, 0)

	mdc := MetricDecorationConfig{
		Category: "/cpu",
		Metric:   "% cpu",
		Rename:   "\\CPU",
	}
	expected = append(expected, mdc)

	m, err := NewMetricDecorations(expected)
	assert.True(t, err == nil)
	assert.Equal(t, "\\CPU", m.getRename("/cpu", "% cpu"))
}

func TestOverrideDefaultUnit(t *testing.T) {
	m, err := NewMetricDecorations(nil)

	assert.Equal(t, "Percent", m.getUnit("cpu", "usage_idle"))
	expected := make([]MetricDecorationConfig, 0)

	mdc := MetricDecorationConfig{
		Category: "cpu",
		Metric:   "usage_idle",
		Unit:     "Bytes",
	}

	expected = append(expected, mdc)
	mdc = MetricDecorationConfig{
		Category: "Network Interface",
		Metric:   "Packets Sent/sec",
		Unit:     "Bytes",
	}

	expected = append(expected, mdc)

	m, err = NewMetricDecorations(expected)
	assert.True(t, err == nil)
	assert.Equal(t, "Bytes", m.getUnit("cpu", "usage_idle"))
}

func TestProcstatDefaultUnit(t *testing.T) {
	m, err := NewMetricDecorations(nil)
	assert.True(t, err == nil)

	assert.Equal(t, "Percent", m.getUnit("procstat", "cpu_usage"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "memory_data"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "memory_locked"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "memory_rss"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "memory_stack"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "memory_swap"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "memory_vms"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "read_bytes"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "write_bytes"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "rlimit_memory_data_hard"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "rlimit_memory_data_soft"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "rlimit_memory_locked_hard"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "rlimit_memory_locked_soft"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "rlimit_memory_rss_hard"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "rlimit_memory_rss_soft"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "rlimit_memory_stack_hard"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "rlimit_memory_stack_soft"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "rlimit_memory_vms_hard"))
	assert.Equal(t, "Bytes", m.getUnit("procstat", "rlimit_memory_vms_soft"))
}
