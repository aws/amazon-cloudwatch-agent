// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extract

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/detectortest"
)

func TestAttributesExtractor(t *testing.T) {
	ctx := context.Background()
	testCases := map[string]struct {
		setup   func(*testing.T, *detectortest.MockProcess)
		want    map[string]string
		wantErr error
	}{
		"Process/Error": {
			setup: func(_ *testing.T, mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return(nil, assert.AnError)
			},
			wantErr: assert.AnError,
		},
		"Process/NotKafkaBroker": {
			setup: func(t *testing.T, mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return(
					detectortest.CmdlineArgsFromFile(t, filepath.Join("..", "testdata", "zookeeper_cmdline")), nil)
			},
			wantErr: detector.ErrIncompatibleExtractor,
		},
		"Process/KafkaBroker/Error": {
			setup: func(_ *testing.T, mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{
					"java",
					"kafka.Kafka",
					"server.properties",
					"--override", "log.dirs=override",
				}, nil)
				mp.On("CwdWithContext", ctx).Return("", assert.AnError)
			},
			want: map[string]string{},
		},
		"Process/KafkaBroker/SimpleCmdline": {
			setup: func(t *testing.T, mp *detectortest.MockProcess) {
				dir := t.TempDir()
				createTestPropertiesFile(t, filepath.Join(dir, "config"), "server.properties", map[string]string{
					"broker.id": "123",
					"log.dirs":  "some/log/path",
				})
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{
					"java",
					"kafka.Kafka",
					"config/server.properties",
				}, nil)
				mp.On("CwdWithContext", ctx).Return(dir, nil)
			},
			want: map[string]string{
				"broker.id": "123",
			},
		},
		"Process/KafkaBroker/WithOverrides/LogDirs": {
			setup: func(t *testing.T, mp *detectortest.MockProcess) {
				dir := t.TempDir()
				createTestPropertiesFile(t, filepath.Join(dir, "somewhere"), "else.properties", map[string]string{
					"broker.id": "123",
					"log.dirs":  "custom/log/path",
				})
				createTestPropertiesFile(t, filepath.Join(dir, "custom/log/path"), "meta.properties", map[string]string{
					"cluster.id": "from-server-properties",
				})
				createTestPropertiesFile(t, filepath.Join(dir, "override/path"), "meta.properties", map[string]string{
					"cluster.id": "from-override",
				})
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{
					"java",
					"kafka.Kafka",
					"somewhere/else.properties",
					"--override", "log.dirs=override,override/path",
				}, nil)
				mp.On("CwdWithContext", ctx).Return(dir, nil)
			},
			want: map[string]string{
				"cluster.id": "from-override",
				"broker.id":  "123",
			},
		},
		"Process/KafkaBroker/WithOverrides": {
			setup: func(t *testing.T, mp *detectortest.MockProcess) {
				dir := t.TempDir()
				createTestPropertiesFile(t, filepath.Join(dir, "override/path"), "meta.properties", map[string]string{
					"cluster.id": "from-override",
				})
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{
					"java",
					"kafka.Kafka",
					"server.properties",
					"--override", "log.dirs=override/path",
					"--override", "broker.id=234",
				}, nil)
				mp.On("CwdWithContext", ctx).Return(dir, nil)
			},
			want: map[string]string{
				"cluster.id": "from-override",
				"broker.id":  "234",
			},
		},
		"Process/KafkaBroker/ComplexCmdline": {
			setup: func(t *testing.T, mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return(
					detectortest.CmdlineArgsFromFile(t, filepath.Join("..", "testdata", "kafka_broker_cmdline")), nil)
				mp.On("CwdWithContext", ctx).Return(filepath.Join("..", "testdata"), nil)
			},
			want: map[string]string{
				"cluster.id": "WQSzAfd_RvO0TocjqhQoaA",
				"broker.id":  "0",
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mp := new(detectortest.MockProcess)
			testCase.setup(t, mp)

			extractor := NewAttributesExtractor(slog.Default())
			got, err := extractor.Extract(ctx, mp)
			if testCase.wantErr != nil {
				assert.ErrorIs(t, err, testCase.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCase.want, got)
			}
			mp.AssertExpectations(t)
		})
	}
}

func createTestPropertiesFile(t *testing.T, dir, filename string, properties map[string]string) string {
	t.Helper()

	filePath := filepath.Join(dir, filename)
	err := os.MkdirAll(filepath.Dir(filePath), 0755)
	require.NoError(t, err)

	var content strings.Builder
	content.WriteString("# Generated test properties file\n")
	for key, value := range properties {
		content.WriteString(fmt.Sprintf("%s=%s\n", key, value))
	}

	err = os.WriteFile(filePath, []byte(content.String()), 0600)
	require.NoError(t, err)
	return filePath
}
