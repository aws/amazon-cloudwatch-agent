package util

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

func TestDetectAgentModeAuto(t *testing.T) {
	testCases := map[string]struct {
		runInAws  string
		ec2Region string
		ecsRegion string
		wantMode  string
	}{
		"WithRunInAWS":  {runInAws: config.RUN_IN_AWS_TRUE, wantMode: config.ModeEC2},
		"WithEC2Region": {ec2Region: "us-east-1", wantMode: config.ModeEC2},
		"WithECSRegion": {ecsRegion: "us-east-1", wantMode: config.ModeEC2},
		"WithNone":      {wantMode: config.ModeOnPrem},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			runInAws = testCase.runInAws
			DefaultEC2Region = func() string { return testCase.ec2Region }
			DefaultECSRegion = func() string { return testCase.ecsRegion }
			require.Equal(t, testCase.wantMode, DetectAgentMode("auto"))
		})
	}
}
