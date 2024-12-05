// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"encoding/json"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

const (
	AllowAllOperations = "*"
)

type Stats struct {
	CPUPercent                *float64          `json:"cpu,omitempty"`
	MemoryBytes               *uint64           `json:"mem,omitempty"`
	FileDescriptorCount       *int32            `json:"fd,omitempty"`
	ThreadCount               *int32            `json:"th,omitempty"`
	LatencyMillis             *int64            `json:"lat,omitempty"`
	PayloadBytes              *int              `json:"load,omitempty"`
	StatusCode                *int              `json:"code,omitempty"`
	SharedConfigFallback      *int              `json:"scfb,omitempty"`
	ImdsFallbackSucceed       *int              `json:"ifs,omitempty"`
	AppSignals                *int              `json:"as,omitempty"`
	EnhancedContainerInsights *int              `json:"eci,omitempty"`
	RunningInContainer        *int              `json:"ric,omitempty"`
	RegionType                *string           `json:"rt,omitempty"`
	Mode                      *string           `json:"m,omitempty"`
	EntityRejected            *int              `json:"ent,omitempty"`
	StatusCodes               map[string][5]int `json:"codes,omitempty"` //represents status codes 200,400,408,413,429,
}

// Merge the other Stats into the current. If the field is not nil,
// then it'll overwrite the existing one.
func (s *Stats) Merge(other Stats) {
	if other.CPUPercent != nil {
		s.CPUPercent = other.CPUPercent
	}
	if other.MemoryBytes != nil {
		s.MemoryBytes = other.MemoryBytes
	}
	if other.FileDescriptorCount != nil {
		s.FileDescriptorCount = other.FileDescriptorCount
	}
	if other.ThreadCount != nil {
		s.ThreadCount = other.ThreadCount
	}
	if other.LatencyMillis != nil {
		s.LatencyMillis = other.LatencyMillis
	}
	if other.PayloadBytes != nil {
		s.PayloadBytes = other.PayloadBytes
	}
	if other.StatusCode != nil {
		s.StatusCode = other.StatusCode
	}
	if other.SharedConfigFallback != nil {
		s.SharedConfigFallback = other.SharedConfigFallback
	}
	if other.ImdsFallbackSucceed != nil {
		s.ImdsFallbackSucceed = other.ImdsFallbackSucceed
	}
	if other.AppSignals != nil {
		s.AppSignals = other.AppSignals
	}
	if other.EnhancedContainerInsights != nil {
		s.EnhancedContainerInsights = other.EnhancedContainerInsights
	}
	if other.RunningInContainer != nil {
		s.RunningInContainer = other.RunningInContainer
	}
	if other.RegionType != nil {
		s.RegionType = other.RegionType
	}
	if other.Mode != nil {
		s.Mode = other.Mode
	}
	if other.EntityRejected != nil {
		s.EntityRejected = other.EntityRejected
	}
	if other.StatusCodes != nil {
		if s.StatusCodes == nil {
			s.StatusCodes = make(map[string][5]int)
		}

		for key, value := range other.StatusCodes {
			if existing, ok := s.StatusCodes[key]; ok {
				s.StatusCodes[key] = [5]int{
					existing[0] + value[0], // 200
					existing[1] + value[1], // 400
					existing[2] + value[2], // 408
					existing[3] + value[3], // 413
					existing[4] + value[4], // 429
				}
			} else {
				s.StatusCodes[key] = value
			}
		}
	}

}

func (s *Stats) Marshal() (string, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	content := strings.TrimPrefix(string(raw), "{")
	return strings.TrimSuffix(content, "}"), nil
}

type StatsProvider interface {
	Stats(operation string) Stats
}

type OperationsFilter struct {
	operations collections.Set[string]
	allowAll   bool
}

func (of OperationsFilter) IsAllowed(operationName string) bool {
	return of.allowAll || of.operations.Contains(operationName)
}

type StatsConfig struct {
	// Operations are the allowed operation names to gather stats for.
	Operations []string `mapstructure:"operations,omitempty"`
	// UsageFlags are the usage flags to set on start up.
	UsageFlags map[Flag]any `mapstructure:"usage_flags,omitempty"`
}

var StatusCodeOperations = []string{ // all the operations that are allowed
	"PutRetentionPolicy",
	"DescribeInstances",
	"DescribeTags",
	"DescribeVolumes",
	"DescribeContainerInstances",
	"DescribeServices",
	"DescribeTaskDefinition",
	"ListServices",
	"ListTasks",
	"DescribeTasks",
	"CreateLogGroup",
	"CreateLogStream",
}

func NewOperationsFilter(operations ...string) OperationsFilter {
	allowed := collections.NewSet[string](operations...)
	return OperationsFilter{
		operations: allowed,
		allowAll:   allowed.Contains(AllowAllOperations),
	}
}

// NewStatusCodeOperationsFilter creates a new filter for allowed operations and status codes.
func NewStatusCodeOperationsFilter() OperationsFilter {
	allowed := collections.NewSet[string](StatusCodeOperations...)

	return OperationsFilter{
		operations: allowed,
		allowAll:   allowed.Contains(AllowAllOperations),
	}
}

// GetShortOperationName maps long operation names to short ones.
func GetShortOperationName(operation string) string {
	switch operation {
	case "PutRetentionPolicy":
		return "prp"
	case "DescribeInstances":
		return "di"
	case "DescribeTags":
		return "dt"
	case "DescribeTasks":
		return "dts"
	case "DescribeVolumes":
		return "dv"
	case "DescribeContainerInstances":
		return "dci"
	case "DescribeServices":
		return "ds"
	case "DescribeTaskDefinition":
		return "dtd"
	case "ListServices":
		return "ls"
	case "ListTasks":
		return "lt"
	case "CreateLogGroup":
		return "clg"
	case "CreateLogStream":
		return "cls"
	default:
		return ""
	}
}
