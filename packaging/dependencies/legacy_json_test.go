// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package dependencies_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchConfigKeepsLegacyJSONSource(t *testing.T) {
	ran := 0
	if sh, err := exec.LookPath("sh"); err == nil {
		ran++
		t.Run("linux", func(t *testing.T) {
			runCtlScript(t, sh, "amazon-cloudwatch-agent-ctl_test.sh", "amazon-cloudwatch-agent-ctl")
		})
	}
	if bash, err := exec.LookPath("bash"); err == nil {
		ran++
		t.Run("darwin", func(t *testing.T) {
			runCtlScript(t, bash, "amazon-cloudwatch-agent-ctl_test.sh", filepath.Join("..", "darwin", "amazon-cloudwatch-agent-ctl"))
		})
	}
	if pwsh := powershellBin(); pwsh != "" {
		ran++
		t.Run("windows", func(t *testing.T) {
			cmd := exec.Command(pwsh, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.Join("..", "windows", "amazon-cloudwatch-agent-ctl_test.ps1"))
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
		})
	}
	if ran == 0 {
		t.Fatal("no sh, bash, or powershell available to run the ctl regression test")
	}
}

func TestLinuxAndDarwinJSONHelpersMatch(t *testing.T) {
	linux := ctlHelper(t, "amazon-cloudwatch-agent-ctl")
	darwin := ctlHelper(t, filepath.Join("..", "darwin", "amazon-cloudwatch-agent-ctl"))
	if linux != darwin {
		t.Fatal("linux and darwin legacy-json helpers diverged")
	}
}

func ctlHelper(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	start := strings.Index(text, "# file_identity prints")
	end := strings.Index(text, "cwa_config() {")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("legacy-json helper not found in %s", path)
	}
	return text[start:end]
}

func runCtlScript(t *testing.T, shell, script, ctl string) {
	t.Helper()
	cmd := exec.Command(shell, script, ctl)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}

func powershellBin() string {
	if path, err := exec.LookPath("pwsh"); err == nil {
		return path
	}
	if path, err := exec.LookPath("powershell"); err == nil {
		return path
	}
	return ""
}
