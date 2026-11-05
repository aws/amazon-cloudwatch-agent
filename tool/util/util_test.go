// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	internaltestutil "github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

var expectResult = `{
	"agent": {
		"collect_interval": "10s"
	},
	"metrics": {
		"cpu": {
			"percore": true
		}
	}
}`

func TestCurOS(t *testing.T) {
	assert.Equal(t, runtime.GOOS, CurOS())
}

func TestReadConfigFromJsonFile(t *testing.T) {
	err := os.WriteFile(ConfigFilePath(), []byte(expectResult), os.ModePerm)
	assert.NoError(t, err)

	actualResult := ReadConfigFromJsonFile()
	assert.Equal(t, expectResult, actualResult)
}

func TestSerializeResultMapToJsonByteArray(t *testing.T) {
	resultMap := make(map[string]interface{})

	agentMap := make(map[string]interface{})
	resultMap["agent"] = agentMap
	agentMap["collect_interval"] = "10s"

	metricsMap := make(map[string]interface{})
	resultMap["metrics"] = metricsMap
	cpuMap := make(map[string]interface{})
	metricsMap["cpu"] = cpuMap
	cpuMap["percore"] = true

	bytes := SerializeResultMapToJsonByteArray(resultMap)
	assert.Equal(t, expectResult, string(bytes))

}

func TestSaveResultByteArrayToJsonFile(t *testing.T) {
	filePath := SaveResultByteArrayToJsonFile([]byte(expectResult), ConfigFilePath())
	bytes, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	actualResult := string(bytes)
	assert.Equal(t, expectResult, actualResult)
}

func TestYes(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	testutil.Type(inputChan, "")
	assert.True(t, Yes("Some question"))

	testutil.Type(inputChan, "2")
	assert.False(t, Yes("Some question"))
}

func TestNo(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	testutil.Type(inputChan, "")
	assert.False(t, No("Some question"))

	testutil.Type(inputChan, "1")
	assert.True(t, No("Some question"))
}

func TestAskWithDefault(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	testutil.Type(inputChan, "")

	parsedAnswer := AskWithDefault("Question", "DefaultAnswer")

	assert.Equal(t, "DefaultAnswer", parsedAnswer)

	testutil.Type(inputChan, "Answer")

	parsedAnswer = AskWithDefault("Question", "DefaultAnswer")

	assert.Equal(t, "Answer", parsedAnswer)
}

func TestAsk(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	testutil.Type(inputChan, "Answer")

	parsedAnswer := Ask("Question")

	assert.Equal(t, "Answer", parsedAnswer)
}

func TestChoice(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	testutil.Type(inputChan, "")

	parsedAnswer := Choice("Question", 1, []string{"validValue1", "validValue2"})

	assert.Equal(t, "validValue1", parsedAnswer)

	testutil.Type(inputChan, "InvalidAnswer", "2")

	parsedAnswer = Choice("Question", 1, []string{"validValue1", "validValue2"})

	assert.Equal(t, "validValue2", parsedAnswer)
}

func TestChoiceIndex(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	testutil.Type(inputChan, "")

	parsedAnswer := ChoiceIndex("Question", 1, []string{"validValue1", "validValue2"})

	assert.Equal(t, 0, parsedAnswer)

	testutil.Type(inputChan, "InvalidAnswer", "2")

	parsedAnswer = ChoiceIndex("Question", 1, []string{"validValue1", "validValue2"})

	assert.Equal(t, 1, parsedAnswer)
}

func TestBackupConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFilePath := filepath.Join(tmpDir, "testConfig.json")
	err := os.WriteFile(configFilePath, []byte(`{"key":"value"}`), 0644)
	assert.Nil(t, err)

	backupDirPath := filepath.Join(tmpDir, "backup")
	for i := 0; i < 16; i++ {
		err = backupConfigFile(configFilePath, backupDirPath)
		assert.Nil(t, err)

		files, err := os.ReadDir(backupDirPath)
		assert.Nil(t, err)

		backupFileContents, err := os.ReadFile(filepath.Join(backupDirPath, files[0].Name()))
		assert.Nil(t, err)
		assert.Equal(t, `{"key":"value"}`, string(backupFileContents))
		time.Sleep(time.Second)
	}
	files, err := os.ReadDir(backupDirPath)
	assert.Nil(t, err)
	assert.Equal(t, 10, len(files))

}

func TestSDKRegionWithProfile(t *testing.T) {
	t.Run("EnvRegion", func(t *testing.T) {
		internaltestutil.IsolateAWSSharedConfigEnv(t)
		t.Setenv("AWS_REGION", "eu-west-1")
		assert.Equal(t, "eu-west-1", SDKRegionWithProfile(t.Context(), "AmazonCloudWatchAgent"))
	})
	t.Run("ErrorIsEmpty", func(t *testing.T) {
		internaltestutil.IsolateAWSSharedConfigEnv(t)
		t.Setenv("AWS_MAX_ATTEMPTS", "not-a-number")
		assert.Empty(t, SDKRegionWithProfile(t.Context(), "AmazonCloudWatchAgent"))
	})
}

// TestDefaultEC2Region serves an IMDS endpoint locally and sets a stale AWS_PROFILE. The lookup
// must return the served region without loading credentials: with the credential-backed client it
// used, a stale profile caused a 15s sleep and then no region.
func TestDefaultEC2Region(t *testing.T) {
	internaltestutil.IsolateAWSSharedConfigEnv(t)
	t.Setenv("AWS_PROFILE", "does-not-exist")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "false")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/latest/api/token":
			_, _ = w.Write([]byte("test-token"))
		case r.Method == http.MethodGet && r.URL.Path == "/latest/dynamic/instance-identity/document":
			_, _ = w.Write([]byte(`{"region":"eu-north-1","instanceId":"i-0123456789abcdef0"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("AWS_EC2_METADATA_SERVICE_ENDPOINT", server.URL)

	start := time.Now()
	region := DefaultEC2Region(t.Context())
	assert.Equal(t, "eu-north-1", region)
	assert.Less(t, time.Since(start), 5*time.Second, "region lookup must not wait on credential loading")
}
