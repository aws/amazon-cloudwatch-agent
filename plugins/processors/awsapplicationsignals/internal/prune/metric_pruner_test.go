// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prune

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
)

func TestMetricPrunerWithIndexableAttribute(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want bool
	}{
		{
			"testShouldDropChineseChar",
			"漢",
			true,
		}, {
			"testShouldDropSymbolChar",
			"€, £, µ",
			true,
		}, {
			"testShouldDropAllBlackSpace",
			"   ",
			true,
		},
		{
			"testShouldDropAllTab",
			"		",
			true,
		}, {
			"testShouldKeepEnglishWord",
			"abcdefg-",
			false,
		},
	}

	p := &MetricPruner{}
	for _, tt := range tests {
		attributes := pcommon.NewMap()
		attributes.PutStr(common.MetricAttributeLocalService, tt.val)
		t.Run(tt.name, func(t *testing.T) {
			got, _ := p.ShouldBeDropped(attributes)
			if got != tt.want {
				t.Errorf("ShouldBeDropped() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetricPrunerWithNonIndexableAttribute(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want bool
	}{
		{
			"testShouldKeepChineseChar",
			"漢",
			false,
		}, {
			"testShouldKeepEnglishWord",
			"abcdefg-",
			false,
		},
	}

	p := &MetricPruner{}
	for _, tt := range tests {
		attributes := pcommon.NewMap()
		attributes.PutStr(common.AttributeEC2InstanceId, tt.val)
		t.Run(tt.name, func(t *testing.T) {
			got, _ := p.ShouldBeDropped(attributes)
			if got != tt.want {
				t.Errorf("ShouldBeDropped() got = %v, want %v", got, tt.want)
			}
		})
	}
}
