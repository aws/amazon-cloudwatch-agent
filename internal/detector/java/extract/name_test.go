// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extract

import (
	"archive/zip"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/detectortest"
)

type mockArgNameExtractor struct {
	mock.Mock
}

var _ argNameExtractor = (*mockArgNameExtractor)(nil)

func (m *mockArgNameExtractor) Extract(ctx context.Context, process detector.Process, arg string) (string, error) {
	args := m.Called(ctx, process, arg)
	return args.String(0), args.Error(1)
}

func TestNameExtractor(t *testing.T) {
	type mocks struct {
		process      *detectortest.MockProcess
		nameFilter   *detectortest.MockNameFilter
		subExtractor *mockArgNameExtractor
	}

	ctx := context.Background()
	testCases := map[string]struct {
		setup   func(*mocks)
		want    string
		wantErr error
	}{
		"WithProcessError": {
			setup: func(m *mocks) {
				m.process.On("CmdlineSliceWithContext", ctx).Return(nil, assert.AnError)
			},
			wantErr: assert.AnError,
		},
		"WithNoArgs": {
			setup: func(m *mocks) {
				m.process.On("CmdlineSliceWithContext", ctx).Return([]string{"java"}, nil)
			},
			wantErr: detector.ErrExtractName,
		},
		"WithNameFilter/BeforeSubExtractor": {
			setup: func(m *mocks) {
				m.process.On("CmdlineSliceWithContext", ctx).Return([]string{"java", "skip.jar"}, nil)
				m.nameFilter.On("ShouldInclude", "skip.jar").Return(false)
			},
			wantErr: detector.ErrSkipProcess,
		},
		"WithNameFilter/AfterSubExtractor": {
			setup: func(m *mocks) {
				m.process.On("CmdlineSliceWithContext", ctx).Return([]string{"java", "test.jar"}, nil)
				m.nameFilter.On("ShouldInclude", "test.jar").Return(true).Once()
				m.subExtractor.On("Extract", ctx, m.process, "test.jar").Return("com.example.test.Test", nil)
				m.nameFilter.On("ShouldInclude", "com.example.test.Test").Return(false).Once()
			},
			wantErr: detector.ErrSkipProcess,
		},
		"WithSimpleClass": {
			setup: func(m *mocks) {
				m.process.On("CmdlineSliceWithContext", ctx).Return([]string{"java", "com.example.Main"}, nil)
				m.nameFilter.On("ShouldInclude", "com.example.Main").Return(true)
				m.subExtractor.On("Extract", ctx, m.process, "com.example.Main").Return("", assert.AnError)
			},
			want: "com.example.Main",
		},
		"WithClassPath": {
			setup: func(m *mocks) {
				m.process.On("CmdlineSliceWithContext", ctx).Return([]string{"java", "-cp", "lib/*", "com.example.Main"}, nil)
				m.nameFilter.On("ShouldInclude", "com.example.Main").Return(true)
				m.subExtractor.On("Extract", ctx, m.process, "com.example.Main").Return("", assert.AnError)
			},
			want: "com.example.Main",
		},
		"WithApplicationArgs": {
			setup: func(m *mocks) {
				m.process.On("CmdlineSliceWithContext", ctx).Return([]string{"java", "com.example.Main", "--version"}, nil)
				m.nameFilter.On("ShouldInclude", "com.example.Main").Return(true)
				m.subExtractor.On("Extract", ctx, m.process, "com.example.Main").Return("", assert.AnError)
			},
			want: "com.example.Main",
		},
		"WithSubExtractor": {
			setup: func(m *mocks) {
				m.process.On("CmdlineSliceWithContext", ctx).Return([]string{"java", "-Dcom.example.test.value=test", "test.jar"}, nil)
				m.nameFilter.On("ShouldInclude", "test.jar").Return(true).Once()
				m.subExtractor.On("Extract", ctx, m.process, "test.jar").Return("com.example.test.Test", nil)
				m.nameFilter.On("ShouldInclude", "com.example.test.Test").Return(true).Once()
			},
			want: "com.example.test.Test",
		},
		"WithComplexArgs": {
			setup: func(m *mocks) {
				m.process.On("CmdlineSliceWithContext", ctx).Return(detectortest.CmdlineArgsFromFile(t, filepath.Join("..", "testdata", "java_cmdline_1")), nil)
				m.nameFilter.On("ShouldInclude", "spring-boot-web-starter-tomcat.jar").Return(true).Once()
				m.subExtractor.On("Extract", ctx, m.process, "./spring-boot-web-starter-tomcat.jar").Return("spring-boot-web-starter-tomcat", nil)
				m.nameFilter.On("ShouldInclude", "spring-boot-web-starter-tomcat").Return(true).Once()
			},
			want: "spring-boot-web-starter-tomcat",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			m := &mocks{
				process:      new(detectortest.MockProcess),
				nameFilter:   new(detectortest.MockNameFilter),
				subExtractor: new(mockArgNameExtractor),
			}
			testCase.setup(m)

			extractor := NewNameExtractor(slog.Default(), m.nameFilter)
			ne, ok := extractor.(*nameExtractor)
			require.True(t, ok)
			ne.subExtractors = []argNameExtractor{m.subExtractor}
			got, err := extractor.Extract(ctx, m.process)
			if testCase.wantErr != nil {
				assert.ErrorIs(t, err, testCase.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCase.want, got)
			}
			m.process.AssertExpectations(t)
			m.subExtractor.AssertExpectations(t)
		})
	}
}

func TestArchiveManifestNameExtractor(t *testing.T) {
	extractor := newArchiveManifestNameExtractor(slog.Default())
	testCases := map[string]struct {
		setup   func(*testing.T, *detectortest.MockProcess)
		arg     string
		want    string
		wantErr error
	}{
		"WithApplicationName": {
			setup: func(t *testing.T, mp *detectortest.MockProcess) {
				dir := t.TempDir()
				jar := filepath.Join(dir, "app.jar")
				createTestArchive(t, jar, map[string]string{
					"Implementation-Title":   "Implementation Title",
					"Implementation-Version": "1.0.0",
					"Implementation-Vendor":  "Company",
					"Application-Name":       "Application Name",
					"Start-Class":            "com.example.Application",
					"Main-Class":             "org.springframework.boot.loader.JarLauncher",
					"Specification-Title":    "Specification Title",
					"Specification-Version":  "1.1.1",
					"Specification-Vendor":   "Other Company",
				})
				mp.On("CwdWithContext", context.Background()).Return(dir, nil)
			},
			arg:  "app.jar",
			want: "Application Name",
		},
		"WithImplementationTitle": {
			setup: func(t *testing.T, mp *detectortest.MockProcess) {
				dir := t.TempDir()
				jar := filepath.Join(dir, "app.jar")
				createTestArchive(t, jar, map[string]string{
					"Implementation-Title": "Implementation Title",
					"Start-Class":          "com.example.Application",
					"Main-Class":           "org.springframework.boot.loader.JarLauncher",
				})
				mp.On("CwdWithContext", context.Background()).Return(dir, nil)
			},
			arg:  "app.jar",
			want: "Implementation Title",
		},
		"WithStartClass": {
			setup: func(t *testing.T, mp *detectortest.MockProcess) {
				dir := t.TempDir()
				jar := filepath.Join(dir, "app.jar")
				createTestArchive(t, jar, map[string]string{
					"Start-Class": "com.example.Application",
					"Main-Class":  "org.springframework.boot.loader.JarLauncher",
				})
				mp.On("CwdWithContext", context.Background()).Return(dir, nil)
			},
			arg:  "app.jar",
			want: "com.example.Application",
		},
		"WithMainClass": {
			setup: func(t *testing.T, mp *detectortest.MockProcess) {
				dir := t.TempDir()
				jar := filepath.Join(dir, "app.jar")
				createTestArchive(t, jar, map[string]string{
					"Main-Class": "com.example.Main",
				})
				mp.On("CwdWithContext", context.Background()).Return(dir, nil)
			},
			arg:  "app.jar",
			want: "com.example.Main",
		},
		"WithNoFields": {
			setup: func(t *testing.T, mp *detectortest.MockProcess) {
				dir := t.TempDir()
				jar := filepath.Join(dir, "app.jar")
				createTestArchive(t, jar, map[string]string{
					"Other": "Value",
				})
				mp.On("CwdWithContext", context.Background()).Return(dir, nil)
			},
			arg:  "app.jar",
			want: "app",
		},
		"WithNoManifest": {
			setup: func(t *testing.T, mp *detectortest.MockProcess) {
				dir := t.TempDir()
				war := filepath.Join(dir, "app.war")
				createTestArchive(t, war, nil)
				mp.On("CwdWithContext", context.Background()).Return(dir, nil)
			},
			arg:  "app.war",
			want: "app",
		},
		"WithNonArchive": {
			setup: func(*testing.T, *detectortest.MockProcess) {
			},
			arg:     "app.txt",
			wantErr: detector.ErrIncompatibleExtractor,
		},
		"WithAbsolutePath": {
			setup: func(t *testing.T, _ *detectortest.MockProcess) {
				dir := t.TempDir()
				jar := filepath.Join(dir, "app.jar")
				createTestArchive(t, jar, map[string]string{
					"Start-Class": "com.example.Application",
				})
			},
			arg:  filepath.Join(t.TempDir(), "app.jar"),
			want: "app",
		},
		"WithWARFile": {
			setup: func(t *testing.T, mp *detectortest.MockProcess) {
				dir := t.TempDir()
				war := filepath.Join(dir, "app.war")
				createTestArchive(t, war, map[string]string{
					"Start-Class": "com.example.WebApplication",
				})
				mp.On("CwdWithContext", context.Background()).Return(dir, nil)
			},
			arg:  "app.war",
			want: "com.example.WebApplication",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mp := new(detectortest.MockProcess)
			testCase.setup(t, mp)

			got, err := extractor.Extract(context.Background(), mp, testCase.arg)
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

func createTestArchive(t *testing.T, path string, manifest map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	z := zip.NewWriter(f)
	defer z.Close()

	if manifest != nil {
		var manifestFile io.Writer
		manifestFile, err = z.Create("META-INF/MANIFEST.MF")
		require.NoError(t, err)

		var content strings.Builder
		for k, v := range manifest {
			content.WriteString(k + ": " + v + "\n")
		}
		_, err = manifestFile.Write([]byte(content.String()))
		require.NoError(t, err)
	}
}
