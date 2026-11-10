// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	override "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	translateutil "github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
)

// The test binary doubles as a fake config-translator: prepareConfig tests
// point paths.TranslatorBinaryPath at it and set fakeTranslatorEnv.
const (
	fakeTranslatorEnv     = "CWA_START_TEST_FAKE_TRANSLATOR"
	fakeTranslatorExitEnv = "CWA_START_TEST_FAKE_TRANSLATOR_EXIT"
	fakeTranslatorTOML    = "[agent]\n  region = \"eu-central-1\"\n"
)

func TestMain(m *testing.M) {
	if os.Getenv(fakeTranslatorEnv) != "" {
		os.Exit(runFakeTranslator())
	}
	os.Exit(m.Run())
}

func runFakeTranslator() int {
	if code := os.Getenv(fakeTranslatorExitEnv); code != "" {
		exitCode, err := strconv.Atoi(code)
		if err != nil {
			return 2
		}
		return exitCode
	}
	var output string
	for i, arg := range os.Args {
		if arg == "--output" && i+1 < len(os.Args) {
			output = os.Args[i+1]
		}
	}
	if output == "" {
		return 3
	}
	// The output path comes from the test that started this process.
	if err := os.WriteFile(output, []byte(fakeTranslatorTOML), 0o600); err != nil { //nolint:gosec // G703: path is set by the test itself
		return 4
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(output), "env-config.json"), []byte("{}"), 0o600); err != nil { //nolint:gosec // G703: path is set by the test itself
		return 5
	}
	return 0
}

type stampTestEnv struct {
	dir      string
	identity hostIdentity
	version  string
	now      time.Time
}

var testIdentity = hostIdentity{
	InstanceID:    "i-0123456789abcdef0",
	IMDSHostname:  "ip-10-0-0-1.eu-central-1.compute.internal",
	LocalHostname: "EC2AMAZ-TEST",
	PrivateIP:     "10.0.0.1",
	Region:        "eu-central-1",
	AccountID:     "123456789012",
	ImageID:       "ami-0123456789abcdef0",
	InstanceType:  "m5.large",
}

// setupStampTest redirects all paths into a temp directory, creates a
// translated config and stubs the version and identity lookups. Everything
// is restored by t.Cleanup.
func setupStampTest(t *testing.T) *stampTestEnv {
	t.Helper()
	dir := t.TempDir()
	env := &stampTestEnv{dir: dir, identity: testIdentity, version: "1.300000.0", now: time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local)}

	origTOML := paths.TomlConfigPath
	origYAML := paths.YamlConfigPath
	origEnvConfig := paths.EnvConfigPath
	origJSON := paths.JsonConfigPath
	origConfigDir := paths.ConfigDirPath
	origCommonConfig := paths.CommonConfigPath
	origAgentBinary := paths.AgentBinaryPath
	origTranslatorBinary := paths.TranslatorBinaryPath
	origAgentVersion := agentVersion
	origLookupHostIdentity := lookupHostIdentity
	origLocalHostname := localHostname
	origNow := now
	origDefaultSharedConfigDir := defaultSharedConfigDir
	t.Cleanup(func() {
		paths.TomlConfigPath = origTOML
		paths.YamlConfigPath = origYAML
		paths.EnvConfigPath = origEnvConfig
		paths.JsonConfigPath = origJSON
		paths.ConfigDirPath = origConfigDir
		paths.CommonConfigPath = origCommonConfig
		paths.AgentBinaryPath = origAgentBinary
		paths.TranslatorBinaryPath = origTranslatorBinary
		agentVersion = origAgentVersion
		lookupHostIdentity = origLookupHostIdentity
		localHostname = origLocalHostname
		now = origNow
		defaultSharedConfigDir = origDefaultSharedConfigDir
		resetHostIdentityCache()
	})

	paths.TomlConfigPath = filepath.Join(dir, "amazon-cloudwatch-agent.toml")
	paths.YamlConfigPath = filepath.Join(dir, "amazon-cloudwatch-agent.yaml")
	paths.EnvConfigPath = filepath.Join(dir, "env-config.json")
	paths.JsonConfigPath = filepath.Join(dir, "amazon-cloudwatch-agent.json")
	paths.ConfigDirPath = filepath.Join(dir, "Configs")
	paths.CommonConfigPath = filepath.Join(dir, "common-config.toml")
	paths.AgentBinaryPath = filepath.Join(dir, "bin", "amazon-cloudwatch-agent.exe")
	paths.TranslatorBinaryPath = filepath.Join(dir, "bin", "config-translator.exe")
	agentVersion = func() string { return env.version }
	lookupHostIdentity = func() (hostIdentity, error) { return env.identity, nil }
	now = func() time.Time { return env.now }
	defaultSharedConfigDir = func() string { return filepath.Join(dir, "home", ".aws") }
	resetHostIdentityCache()

	t.Setenv("RUN_IN_CONTAINER", "")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "")
	t.Setenv("AWS_CONFIG_FILE", "")

	credentialFile := filepath.Join(dir, "creds", "credentials")
	writeTestFile(t, paths.JsonConfigPath, `{"logs":{"logs_collected":{}}}`)
	writeTestFile(t, paths.CommonConfigPath, "[credentials]\n  shared_credential_file = '"+credentialFile+"'\n")
	writeTestFile(t, credentialFile, "[default]\n")
	writeTestFile(t, filepath.Join(paths.ConfigDirPath, "file_a.json"), `{"metrics":{}}`)
	writeTestFile(t, filepath.Join(paths.ConfigDirPath, "sub", "file_b.json"), `{"logs":{}}`)
	writeTestFile(t, paths.AgentBinaryPath, "agent")
	writeTestFile(t, paths.TranslatorBinaryPath, "translator")
	writeTestFile(t, paths.TomlConfigPath, fakeTranslatorTOML)
	writeTestFile(t, paths.EnvConfigPath, "{}")
	return env
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func resetHostIdentityCache() {
	hostIdentityLookedUp = false
	cachedHostIdentity = hostIdentity{}
	cachedHostIdentityErr = nil
}

// isCurrentInNewProcess mimics the next service start: the per-process
// identity cache is empty.
func isCurrentInNewProcess() bool {
	resetHostIdentityCache()
	return translationIsCurrent()
}

func writeStamp(t *testing.T) {
	t.Helper()
	resetHostIdentityCache()
	writeTranslationStamp(captureTranslationInputs())
	require.FileExists(t, translationStampPath())
}

func TestTranslationIsCurrent_NoStamp(t *testing.T) {
	setupStampTest(t)
	assert.False(t, isCurrentInNewProcess())
}

func TestTranslationIsCurrent_UnchangedAfterStamp(t *testing.T) {
	setupStampTest(t)
	writeStamp(t)
	assert.True(t, isCurrentInNewProcess())

	stamp, err := readTranslationStamp()
	require.NoError(t, err)
	assert.Equal(t, translationStampSchemaVersion, stamp.SchemaVersion)
	assert.Equal(t, "1.300000.0", stamp.AgentVersion)
	assert.Equal(t, testIdentity, stamp.Identity)
	assert.Len(t, stamp.Binaries, 3)
	assert.Empty(t, stamp.Date)
	assert.NotContains(t, translationStampPath(), paths.ConfigDirPath, "stamp must not be a translator input")
}

func TestTranslationIsCurrent_MtimeOnlyChangeIsCurrent(t *testing.T) {
	setupStampTest(t)
	writeStamp(t)
	later := time.Now().Add(time.Hour)
	for _, p := range []string{paths.JsonConfigPath, paths.CommonConfigPath, filepath.Join(paths.ConfigDirPath, "file_a.json"), paths.TomlConfigPath} {
		require.NoError(t, os.Chtimes(p, later, later))
	}
	assert.True(t, isCurrentInNewProcess())
}

func TestTranslationIsCurrent_DetectsChanges(t *testing.T) {
	testCases := map[string]func(t *testing.T, env *stampTestEnv){
		"JSONContentChanged": func(t *testing.T, _ *stampTestEnv) {
			writeTestFile(t, paths.JsonConfigPath, `{"logs":{"logs_collected":{"files":{}}}}`)
		},
		"JSONDeleted": func(t *testing.T, _ *stampTestEnv) {
			require.NoError(t, os.Remove(paths.JsonConfigPath))
		},
		"ConfigsFileAdded": func(t *testing.T, _ *stampTestEnv) {
			writeTestFile(t, filepath.Join(paths.ConfigDirPath, "file_c.json"), `{}`)
		},
		"ConfigsFileDeleted": func(t *testing.T, _ *stampTestEnv) {
			require.NoError(t, os.Remove(filepath.Join(paths.ConfigDirPath, "file_a.json")))
		},
		"ConfigsFileRenamed": func(t *testing.T, _ *stampTestEnv) {
			require.NoError(t, os.Rename(filepath.Join(paths.ConfigDirPath, "file_a.json"), filepath.Join(paths.ConfigDirPath, "file_z.json")))
		},
		"NestedConfigsFileChanged": func(t *testing.T, _ *stampTestEnv) {
			writeTestFile(t, filepath.Join(paths.ConfigDirPath, "sub", "file_b.json"), `{"logs":{"force_flush_interval":5}}`)
		},
		"ConfigsTmpFileAdded": func(t *testing.T, _ *stampTestEnv) {
			writeTestFile(t, filepath.Join(paths.ConfigDirPath, "file_d.tmp"), `{}`)
		},
		"ConfigsDirRemoved": func(t *testing.T, _ *stampTestEnv) {
			require.NoError(t, os.RemoveAll(paths.ConfigDirPath))
		},
		"CopiedWithOldTimestamp": func(t *testing.T, _ *stampTestEnv) {
			p := filepath.Join(paths.ConfigDirPath, "file_a.json")
			info, err := os.Stat(p)
			require.NoError(t, err)
			writeTestFile(t, p, `{"metrics":{"x":1}}`)
			old := info.ModTime().Add(-24 * time.Hour)
			require.NoError(t, os.Chtimes(p, old, old))
		},
		"CommonConfigChanged": func(t *testing.T, _ *stampTestEnv) {
			writeTestFile(t, paths.CommonConfigPath, "[credentials]\n  shared_credential_profile = 'other'\n")
		},
		"SharedCredentialFileChanged": func(t *testing.T, env *stampTestEnv) {
			writeTestFile(t, filepath.Join(env.dir, "creds", "credentials"), "[default]\nregion = us-east-1\n")
		},
		"SharedConfigNextToCredentialFileAdded": func(t *testing.T, env *stampTestEnv) {
			writeTestFile(t, filepath.Join(env.dir, "creds", "config"), "[default]\nregion = us-east-1\n")
		},
		"DefaultSharedConfigAdded": func(t *testing.T, env *stampTestEnv) {
			writeTestFile(t, filepath.Join(env.dir, "home", ".aws", "config"), "[profile AmazonCloudWatchAgent]\nregion = us-east-1\n")
		},
		"AgentVersionChanged": func(_ *testing.T, env *stampTestEnv) {
			env.version = "1.300001.0"
		},
		"AgentBinarySizeChanged": func(t *testing.T, _ *stampTestEnv) {
			writeTestFile(t, paths.AgentBinaryPath, "agent-hotfix")
		},
		"TranslatorBinaryTimestampChanged": func(t *testing.T, _ *stampTestEnv) {
			later := time.Now().Add(time.Hour)
			require.NoError(t, os.Chtimes(paths.TranslatorBinaryPath, later, later))
		},
		"TOMLEditedOutOfBand": func(t *testing.T, _ *stampTestEnv) {
			writeTestFile(t, paths.TomlConfigPath, fakeTranslatorTOML+"# edited\n")
		},
		"TOMLDeleted": func(t *testing.T, _ *stampTestEnv) {
			require.NoError(t, os.Remove(paths.TomlConfigPath))
		},
		"YAMLCreated": func(t *testing.T, _ *stampTestEnv) {
			writeTestFile(t, paths.YamlConfigPath, "receivers: {}\n")
		},
		"EnvConfigChanged": func(t *testing.T, _ *stampTestEnv) {
			writeTestFile(t, paths.EnvConfigPath, `{"CWAGENT_LOG_LEVEL":"DEBUG"}`)
		},
		"EnvironmentChanged": func(t *testing.T, _ *stampTestEnv) {
			t.Setenv("CWA_START_TEST_EXTRA_VAR", "1")
		},
	}
	for name, mutate := range testCases {
		t.Run(name, func(t *testing.T) {
			env := setupStampTest(t)
			writeStamp(t)
			require.True(t, isCurrentInNewProcess())
			mutate(t, env)
			assert.False(t, isCurrentInNewProcess())
		})
	}
}

func TestTranslationIsCurrent_DetectsIdentityChanges(t *testing.T) {
	testCases := map[string]func(*hostIdentity){
		"InstanceID":    func(i *hostIdentity) { i.InstanceID = "i-0fedcba9876543210" },
		"IMDSHostname":  func(i *hostIdentity) { i.IMDSHostname = "ip-10-0-0-2.eu-central-1.compute.internal" },
		"LocalHostname": func(i *hostIdentity) { i.LocalHostname = "RENAMED-HOST" },
		"PrivateIP":     func(i *hostIdentity) { i.PrivateIP = "10.0.0.2" },
		"Region":        func(i *hostIdentity) { i.Region = "eu-west-1" },
		"AccountID":     func(i *hostIdentity) { i.AccountID = "210987654321" },
		"ImageID":       func(i *hostIdentity) { i.ImageID = "ami-0fedcba9876543210" },
		"InstanceType":  func(i *hostIdentity) { i.InstanceType = "m5.xlarge" },
	}
	for name, mutate := range testCases {
		t.Run(name, func(t *testing.T) {
			env := setupStampTest(t)
			writeStamp(t)
			require.True(t, isCurrentInNewProcess())
			mutate(&env.identity)
			assert.False(t, isCurrentInNewProcess())
		})
	}
}

func TestTranslationIsCurrent_IdentityUnavailable(t *testing.T) {
	setupStampTest(t)
	writeStamp(t)
	lookupHostIdentity = func() (hostIdentity, error) {
		return hostIdentity{}, errors.New("EC2 instance metadata not available within 5s")
	}
	assert.False(t, isCurrentInNewProcess())
}

func TestTranslationIsCurrent_IdentityLookedUpOnlyWhenLocalStateMatches(t *testing.T) {
	setupStampTest(t)
	writeStamp(t)
	writeTestFile(t, paths.JsonConfigPath, `{}`)
	lookups := 0
	lookupHostIdentity = func() (hostIdentity, error) {
		lookups++
		return testIdentity, nil
	}
	assert.False(t, isCurrentInNewProcess())
	assert.Zero(t, lookups, "a local change must be detected without waiting for IMDS")
}

func TestTranslationIsCurrent_StampUnusable(t *testing.T) {
	testCases := map[string]string{
		"Corrupt":              "{not json",
		"WrongSchemaVersion":   `{"schema_version": 2}`,
		"MissingSchemaVersion": `{}`,
	}
	for name, content := range testCases {
		t.Run(name, func(t *testing.T) {
			setupStampTest(t)
			writeStamp(t)
			writeTestFile(t, translationStampPath(), content)
			assert.False(t, isCurrentInNewProcess())
		})
	}
}

func TestWriteTranslationStamp_NotWritten(t *testing.T) {
	testCases := map[string]func(t *testing.T, env *stampTestEnv){
		"UnknownAgentVersion": func(_ *testing.T, env *stampTestEnv) {
			env.version = "Unknown"
		},
		"EmptyAgentVersion": func(_ *testing.T, env *stampTestEnv) {
			env.version = ""
		},
		"IdentityUnavailable": func(_ *testing.T, _ *stampTestEnv) {
			lookupHostIdentity = func() (hostIdentity, error) { return hostIdentity{}, errors.New("timeout") }
		},
		"AutoScalingGroupPlaceholder": func(t *testing.T, _ *stampTestEnv) {
			writeTestFile(t, filepath.Join(paths.ConfigDirPath, "file_a.json"),
				`{"metrics":{"metrics_collected":{"procstat":{"append_dimensions":{"AutoScalingGroupName":"${aws:AutoScalingGroupName}"}}}}}`)
		},
		"TOMLMissing": func(t *testing.T, _ *stampTestEnv) {
			require.NoError(t, os.Remove(paths.TomlConfigPath))
		},
	}
	for _, value := range unresolvedPlaceholderValues {
		testCases["OutputContains"+value] = func(t *testing.T, _ *stampTestEnv) {
			writeTestFile(t, paths.TomlConfigPath, "log_stream_name = \""+value+"_stream\"\n")
		}
	}
	for name, mutate := range testCases {
		t.Run(name, func(t *testing.T) {
			env := setupStampTest(t)
			mutate(t, env)
			resetHostIdentityCache()
			writeTranslationStamp(captureTranslationInputs())
			assert.NoFileExists(t, translationStampPath())
			assert.False(t, isCurrentInNewProcess())
		})
	}
}

// TestWriteTranslationStamp_InputChangedDuringTranslation covers a config
// change between the start and the end of a translation: the inputs are
// captured before translateConfig, so a later change must stop the stamp from
// being written and the next start must retranslate.
func TestWriteTranslationStamp_InputChangedDuringTranslation(t *testing.T) {
	testCases := map[string]func(t *testing.T){
		"JSONContentChanged": func(t *testing.T) {
			writeTestFile(t, paths.JsonConfigPath, `{"logs":{"logs_collected":{"files":{}}}}`)
		},
		"ConfigsFileAdded": func(t *testing.T) {
			writeTestFile(t, filepath.Join(paths.ConfigDirPath, "file_c.json"), `{}`)
		},
		"EnvironmentChanged": func(t *testing.T) {
			t.Setenv("CWA_START_TEST_EXTRA_VAR", "1")
		},
	}
	for name, changeDuringTranslation := range testCases {
		t.Run(name, func(t *testing.T) {
			setupStampTest(t)
			resetHostIdentityCache()
			captured := captureTranslationInputs()
			changeDuringTranslation(t)
			writeTranslationStamp(captured)
			assert.NoFileExists(t, translationStampPath())
			assert.False(t, isCurrentInNewProcess())
		})
	}
}

// TestWriteTranslationStamp_DateCapturedBeforeTranslation pins the {date}
// handling across a translation. Within the same day the captured date is
// recorded; a translation that crosses midnight is a change and is not cached.
func TestWriteTranslationStamp_DateCapturedBeforeTranslation(t *testing.T) {
	t.Run("SameDay", func(t *testing.T) {
		env := setupStampTest(t)
		writeTestFile(t, paths.JsonConfigPath, `{"logs":{"logs_collected":{"files":{"collect_list":[{"log_stream_name":"{instance_id}_{date}"}]}}}}`)
		resetHostIdentityCache()
		captured := captureTranslationInputs()
		env.now = env.now.Add(10 * time.Hour) // still 2026-11-03 local time
		writeTranslationStamp(captured)
		require.FileExists(t, translationStampPath())
		stamp, err := readTranslationStamp()
		require.NoError(t, err)
		assert.Equal(t, "2026-11-03", stamp.Date)
	})
	t.Run("CrossesMidnight", func(t *testing.T) {
		env := setupStampTest(t)
		writeTestFile(t, paths.JsonConfigPath, `{"logs":{"logs_collected":{"files":{"collect_list":[{"log_stream_name":"{instance_id}_{date}"}]}}}}`)
		resetHostIdentityCache()
		captured := captureTranslationInputs()
		env.now = env.now.Add(24 * time.Hour) // translation crosses midnight
		writeTranslationStamp(captured)
		assert.NoFileExists(t, translationStampPath())
	})
}

func TestTranslationIsCurrent_DatePlaceholder(t *testing.T) {
	env := setupStampTest(t)
	writeTestFile(t, paths.JsonConfigPath, `{"logs":{"logs_collected":{"files":{"collect_list":[{"log_stream_name":"{instance_id}_{date}"}]}}}}`)
	writeStamp(t)
	stamp, err := readTranslationStamp()
	require.NoError(t, err)
	assert.Equal(t, "2026-11-03", stamp.Date)

	env.now = env.now.Add(10 * time.Hour) // still 2026-11-03 local time
	assert.True(t, isCurrentInNewProcess())

	env.now = env.now.Add(24 * time.Hour)
	assert.False(t, isCurrentInNewProcess())
}

func TestTranslationIsCurrent_DateIgnoredWithoutPlaceholder(t *testing.T) {
	env := setupStampTest(t)
	writeStamp(t)
	env.now = env.now.Add(48 * time.Hour)
	assert.True(t, isCurrentInNewProcess())
}

func TestPrepareConfig_SkipsTranslatorWhenCurrent(t *testing.T) {
	setupStampTest(t)
	writeStamp(t)
	resetHostIdentityCache()
	// The fake translator file is not executable, so running it would fail.
	translated, err := prepareConfig()
	require.NoError(t, err)
	assert.False(t, translated)
}

func TestPrepareConfig_TranslatesAndWritesStamp(t *testing.T) {
	setupStampTest(t)
	self, err := os.Executable()
	require.NoError(t, err)
	paths.TranslatorBinaryPath = self
	t.Setenv(fakeTranslatorEnv, "1")
	writeTestFile(t, paths.TomlConfigPath, "stale")

	resetHostIdentityCache()
	translated, err := prepareConfig()
	require.NoError(t, err)
	assert.True(t, translated)
	content, err := os.ReadFile(paths.TomlConfigPath)
	require.NoError(t, err)
	assert.Equal(t, fakeTranslatorTOML, string(content))
	assert.FileExists(t, translationStampPath())
	assert.True(t, isCurrentInNewProcess())
}

func TestPrepareConfig_FailedTranslationLeavesNoStamp(t *testing.T) {
	testCases := map[string]func(t *testing.T){
		"TranslatorExitsWithError": func(t *testing.T) {
			self, err := os.Executable()
			require.NoError(t, err)
			paths.TranslatorBinaryPath = self
			t.Setenv(fakeTranslatorEnv, "1")
			t.Setenv(fakeTranslatorExitEnv, "1")
		},
		"TranslatorMissing": func(_ *testing.T) {
			paths.TranslatorBinaryPath = filepath.Join(filepath.Dir(paths.TranslatorBinaryPath), "does-not-exist.exe")
		},
	}
	for name, breakTranslator := range testCases {
		t.Run(name, func(t *testing.T) {
			setupStampTest(t)
			writeStamp(t)
			breakTranslator(t)
			// A config change forces a translation, which then fails.
			writeTestFile(t, paths.JsonConfigPath, `{"metrics":{}}`)
			resetHostIdentityCache()
			translated, err := prepareConfig()
			require.Error(t, err)
			assert.True(t, translated)
			assert.NoFileExists(t, translationStampPath())
			assert.False(t, isCurrentInNewProcess())
		})
	}
}

// TestPrepareConfig_ContainerModeAlwaysTranslates verifies that a container
// never takes the skip path, even with a matching stamp. The translator is
// missing, so the attempt fails; a nil error would mean the skip was taken.
func TestPrepareConfig_ContainerModeAlwaysTranslates(t *testing.T) {
	setupStampTest(t)
	writeStamp(t)
	paths.TranslatorBinaryPath = filepath.Join(filepath.Dir(paths.TranslatorBinaryPath), "does-not-exist.exe")
	t.Setenv("RUN_IN_CONTAINER", "True") // envconfig.TrueValue
	resetHostIdentityCache()
	translated, err := prepareConfig()
	require.Error(t, err)
	assert.True(t, translated)
}

func TestFetchHostIdentity(t *testing.T) {
	complete := instanceMetadata{
		Region:       testIdentity.Region,
		PrivateIP:    testIdentity.PrivateIP,
		InstanceID:   testIdentity.InstanceID,
		Hostname:     testIdentity.IMDSHostname,
		AccountID:    testIdentity.AccountID,
		InstanceType: testIdentity.InstanceType,
		ImageID:      testIdentity.ImageID,
	}
	origLocalHostname := localHostname
	t.Cleanup(func() { localHostname = origLocalHostname })
	localHostname = func() (string, error) { return testIdentity.LocalHostname, nil }

	t.Run("Complete", func(t *testing.T) {
		identity, err := fetchHostIdentity(func(context.Context) instanceMetadata { return complete }, time.Second)
		require.NoError(t, err)
		assert.Equal(t, testIdentity, identity)
	})

	incomplete := map[string]func(*instanceMetadata){
		"Region":       func(e *instanceMetadata) { e.Region = "" },
		"PrivateIP":    func(e *instanceMetadata) { e.PrivateIP = "" },
		"InstanceID":   func(e *instanceMetadata) { e.InstanceID = "" },
		"Hostname":     func(e *instanceMetadata) { e.Hostname = "" },
		"AccountID":    func(e *instanceMetadata) { e.AccountID = "" },
		"InstanceType": func(e *instanceMetadata) { e.InstanceType = "" },
		"ImageID":      func(e *instanceMetadata) { e.ImageID = "" },
	}
	for name, blank := range incomplete {
		t.Run("Missing"+name, func(t *testing.T) {
			md := complete
			blank(&md)
			_, err := fetchHostIdentity(func(context.Context) instanceMetadata { return md }, time.Second)
			assert.Error(t, err)
		})
	}

	t.Run("Timeout", func(t *testing.T) {
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		start := time.Now()
		_, err := fetchHostIdentity(func(context.Context) instanceMetadata {
			<-release
			return complete
		}, 50*time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not available within")
		assert.Less(t, time.Since(start), 5*time.Second)
	})

	t.Run("LocalHostnameError", func(t *testing.T) {
		localHostname = func() (string, error) { return "", errors.New("no hostname") }
		t.Cleanup(func() { localHostname = func() (string, error) { return testIdentity.LocalHostname, nil } })
		_, err := fetchHostIdentity(func(context.Context) instanceMetadata { return complete }, time.Second)
		assert.Error(t, err)
	})

	t.Run("TimeoutCancelsLookup", func(t *testing.T) {
		canceled := make(chan struct{})
		_, err := fetchHostIdentity(func(ctx context.Context) instanceMetadata {
			<-ctx.Done()
			close(canceled)
			return complete
		}, 50*time.Millisecond)
		require.Error(t, err)
		select {
		case <-canceled:
		case <-time.After(5 * time.Second):
			t.Fatal("lookup context was not canceled after the timeout")
		}
	})
}

const fakeIMDSToken = "fake-token"

var fakeIdentityDocument = `{"region":"eu-central-1","accountId":"123456789012","privateIp":"10.0.0.1",` +
	`"instanceId":"i-0123456789abcdef0","instanceType":"m5.large","imageId":"ami-0123456789abcdef0"}`

// newFakeIMDS serves the two IMDS paths the translator reads. With v2 it
// issues tokens and requires them; without v2 the token request returns 403,
// which makes the SDK fall back to IMDSv1 when fallback is allowed.
func newFakeIMDS(t *testing.T, v2 bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/latest/api/token", func(w http.ResponseWriter, r *http.Request) {
		if !v2 || r.Method != http.MethodPut {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("X-Aws-Ec2-Metadata-Token-Ttl-Seconds", "300")
		_, _ = w.Write([]byte(fakeIMDSToken))
	})
	serve := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if v2 && r.Header.Get("X-Aws-Ec2-Metadata-Token") != fakeIMDSToken {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(body))
		}
	}
	mux.HandleFunc("/latest/meta-data/hostname", serve(testIdentity.IMDSHostname))
	mux.HandleFunc("/latest/dynamic/instance-identity/document", serve(fakeIdentityDocument))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// translatorInstanceMetadata reads IMDS the way translator/util/ec2util does
// (deriveEC2MetadataFromIMDS with ec2metadataprovider and the override/aws
// IMDS client), against the given endpoint.
func translatorInstanceMetadata(ctx context.Context, endpoint string) instanceMetadata {
	var md instanceMetadata
	provider := ec2metadataprovider.NewMetadataProviderWithoutConfig(nil, override.GetDefaultRetryNumber(), func(o *imds.Options) {
		o.Endpoint = endpoint
	})
	if hostname, err := provider.Hostname(ctx); err == nil {
		md.Hostname = hostname
	}
	if doc, err := provider.Get(ctx); err == nil {
		md.Region = doc.Region
		md.AccountID = doc.AccountID
		md.PrivateIP = doc.PrivateIP
		md.InstanceID = doc.InstanceID
		md.InstanceType = doc.InstanceType
		md.ImageID = doc.ImageID
	}
	return md
}

// TestIMDSLookupMatchesTranslator compares the IMDS lookup of this binary
// with the client the translator uses, including the IMDSv1 fallback and its
// opt-out, against the same fake IMDS.
func TestIMDSLookupMatchesTranslator(t *testing.T) {
	complete := instanceMetadata{
		Region:       testIdentity.Region,
		PrivateIP:    testIdentity.PrivateIP,
		InstanceID:   testIdentity.InstanceID,
		Hostname:     testIdentity.IMDSHostname,
		AccountID:    testIdentity.AccountID,
		InstanceType: testIdentity.InstanceType,
		ImageID:      testIdentity.ImageID,
	}
	testCases := map[string]struct {
		v2         bool
		v1Disabled string
		closed     bool
		want       instanceMetadata
	}{
		"IMDSv2":                 {v2: true, want: complete},
		"IMDSv1Fallback":         {v2: false, want: complete},
		"IMDSv1FallbackDisabled": {v2: false, v1Disabled: "true", want: instanceMetadata{}},
		"IMDSv2WithV1Disabled":   {v2: true, v1Disabled: "TRUE", want: complete},
		"Unreachable":            {v2: true, closed: true, want: instanceMetadata{}},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("AWS_EC2_METADATA_V1_DISABLED", tc.v1Disabled)
			t.Setenv("IMDS_NUMBER_RETRY", "0")
			srv := newFakeIMDS(t, tc.v2)
			if tc.closed {
				srv.Close()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			got := imdsInstanceMetadata(ctx, func(o *imds.Options) { o.Endpoint = srv.URL })
			assert.Equal(t, tc.want, got)
			assert.Equal(t, translatorInstanceMetadata(ctx, srv.URL), got, "must match the translator's IMDS client")
		})
	}
}

// TestInstanceMetadataMatchesEC2Util fails when ec2util.EC2Util gains or
// loses a field, so the stamp is reviewed together with the translator.
func TestInstanceMetadataMatchesEC2Util(t *testing.T) {
	fieldNames := func(v any) []string {
		typ := reflect.TypeOf(v)
		names := make([]string, 0, typ.NumField())
		for i := 0; i < typ.NumField(); i++ {
			names = append(names, typ.Field(i).Name)
		}
		return names
	}
	assert.ElementsMatch(t, fieldNames(ec2util.EC2Util{}), fieldNames(instanceMetadata{}))
}

func TestIMDSRetries(t *testing.T) {
	testCases := map[string]int{"": 1, "0": 0, "3": 3, "-1": 1, "x": 1}
	for value, want := range testCases {
		t.Setenv("IMDS_NUMBER_RETRY", value)
		assert.Equal(t, override.GetDefaultRetryNumber(), imdsRetries(), "IMDS_NUMBER_RETRY=%q", value)
		assert.Equal(t, want, imdsRetries(), "IMDS_NUMBER_RETRY=%q", value)
	}
}

func TestWriteFileAtomic_ReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, translationStampFileName)
	require.NoError(t, writeFileAtomic(target, []byte("first")))
	require.NoError(t, writeFileAtomic(target, []byte("second")))

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "second", string(content))

	leftovers, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
	require.NoError(t, err)
	assert.Empty(t, leftovers)
}

// TestTranslatorPlaceholdersCovered fails when the translator gains a
// placeholder or fallback value the stamp does not know about.
func TestTranslatorPlaceholdersCovered(t *testing.T) {
	// Every placeholder is resolved from a field of hostIdentity:
	// {instance_id} InstanceID, {hostname} IMDSHostname/LocalHostname,
	// {local_hostname} LocalHostname, {ip_address} PrivateIP,
	// {aws_region} Region (or the inputs), {account_id} AccountID.
	info := translateutil.GetMetadataInfo(func() *translateutil.Metadata { return &translateutil.Metadata{} })
	keys := make([]string, 0, len(info))
	for k := range info {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, []string{"{instance_id}", "{hostname}", "{local_hostname}", "{ip_address}", "{aws_region}", "{account_id}"}, keys)

	// Fallback values used without metadata must be recognized as unresolved.
	assert.Contains(t, unresolvedPlaceholderValues, translateutil.UnknownInstanceID)
	for k, v := range info {
		if strings.Contains(v, "UNKNOWN") {
			assert.Contains(t, unresolvedPlaceholderValues, v, "fallback for %s", k)
		}
	}

	before := time.Now().Format(dateLayout)
	resolved := translateutil.ResolvePlaceholder(datePlaceholder, nil)
	after := time.Now().Format(dateLayout)
	assert.Contains(t, []string{before, after}, resolved)

	assert.Equal(t, ec2tagger.SupportedAppendDimensions["AutoScalingGroupName"], asgPlaceholder)
}
