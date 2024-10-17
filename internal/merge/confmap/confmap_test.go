// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package confmap

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator/tocwconfig/toyamlconfig"
)

func TestMerge(t *testing.T) {
	testCases := map[string]struct {
		fileNames [2]string
		wantErr   error
		wantConf  *Conf
	}{
		"WithConflicts": {
			fileNames: [2]string{
				filepath.Join("testdata", "base.yaml"),
				filepath.Join("testdata", "conflicts.yaml"),
			},
			wantErr: &MergeConflictError{
				conflicts: []mergeConflict{
					{section: "receivers", keys: []string{"otlp"}},
					{section: "extensions", keys: []string{"health_check"}},
					{section: "service::pipelines", keys: []string{"traces"}},
				},
			},
		},
		"WithNoConflicts": {
			fileNames: [2]string{
				filepath.Join("testdata", "base.yaml"),
				filepath.Join("testdata", "merge.yaml"),
			},
			wantConf: mustLoadFromFile(t, filepath.Join("testdata", "base+merge.yaml")),
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			base := mustLoadFromFile(t, testCase.fileNames[0])
			conf := mustLoadFromFile(t, testCase.fileNames[1])
			assert.Equal(t, testCase.wantErr, base.Merge(conf))
			if testCase.wantConf != nil {
				got := toyamlconfig.ToYamlConfig(base.ToStringMap())
				want := toyamlconfig.ToYamlConfig(testCase.wantConf.ToStringMap())
				assert.Equal(t, want, got)
			}
		})
	}
	conf := New()
	assert.NoError(t, conf.Merge(nil))
}

func mustLoadFromFile(t *testing.T, path string) *Conf {
	conf, err := NewFileLoader(path).Load()
	require.NoError(t, err)
	return conf
}
