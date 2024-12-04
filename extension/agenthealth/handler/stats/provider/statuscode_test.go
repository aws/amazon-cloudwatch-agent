package provider

import (
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"reflect"
	"testing"
)

// MockOperationsFilter is a mock implementation of OperationsFilter.
type MockOperationsFilter struct {
	allowedOperations map[string]bool
}

func (m *MockOperationsFilter) IsAllowed(operation string) bool {
	return m.allowedOperations[operation]
}

func TestSingletonStatsProvider_Stats(t *testing.T) {
	provider := &SingletonStatsProvider{
		statusCodeStats: map[string][5]int{
			"operation1": {1, 2, 3, 4, 5},
		},
	}

	stats := provider.Stats("operation1")

	expected := agent.Stats{
		StatusCodes: map[string][5]int{
			"operation1": {1, 2, 3, 4, 5},
		},
	}

	if !reflect.DeepEqual(stats, expected) {
		t.Errorf("Stats() failed. Got %+v, expected %+v", stats, expected)
	}
}

func TestSingletonStatsProvider_UpdateStats(t *testing.T) {
	provider := &SingletonStatsProvider{
		statusCodeStats: make(map[string][5]int),
	}

	provider.UpdateStats("operation1", [5]int{1, 0, 0, 0, 0})

	expected := map[string][5]int{
		"operation1": {1, 0, 0, 0, 0},
	}

	if !reflect.DeepEqual(provider.statusCodeStats, expected) {
		t.Errorf("UpdateStats() failed. Got %+v, expected %+v", provider.statusCodeStats, expected)
	}
}

func TestGetShortOperationName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"PutRetentionPolicy", "prp"},
		{"DescribeInstances", "di"},
		{"UnknownOperation", ""},
	}

	for _, test := range tests {
		result := GetShortOperationName(test.input)
		if result != test.expected {
			t.Errorf("GetShortOperationName(%q) = %q; want %q", test.input, result, test.expected)
		}
	}
}
