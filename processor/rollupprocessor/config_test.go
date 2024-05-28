// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rollupprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	testCases := []struct {
		id   component.ID
		want component.Config
	}{
		{
			id:   component.NewID(component.MustNewType(typeStr)),
			want: NewFactory().CreateDefaultConfig(),
		},
		{
			id:   component.NewIDWithName(component.MustNewType(typeStr), "1"),
			want: &Config{DropOriginal: []string{"MetricName"}, CacheSize: defaultCacheSize},
		},
		{
			id:   component.NewIDWithName(component.MustNewType(typeStr), "2"),
			want: &Config{AttributeGroups: [][]string{{"Attr1"}, {"Attr1", "Attr2"}, {"Attr3"}, {}}, CacheSize: defaultCacheSize},
		},
		{
			id:   component.NewIDWithName(component.MustNewType(typeStr), "3"),
			want: &Config{DropOriginal: []string{"MetricName"}, AttributeGroups: [][]string{{"Attr1", "Attr2"}}, CacheSize: 10},
		},
		{
			id:   component.NewIDWithName(component.MustNewType(typeStr), "4"),
			want: &Config{CacheSize: -1},
		},
	}
	for _, testCase := range testCases {
		conf, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
		require.NoError(t, err)
		cfg := NewFactory().CreateDefaultConfig()
		sub, err := conf.Sub(testCase.id.String())
		require.NoError(t, err)
		require.NoError(t, component.UnmarshalConfig(sub, cfg))

		assert.NoError(t, component.ValidateConfig(cfg))
		assert.Equal(t, testCase.want, cfg)
	}
}
