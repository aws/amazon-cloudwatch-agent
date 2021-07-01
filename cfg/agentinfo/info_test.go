// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agentinfo

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

func TestVersionUseInjectedIfAvailable(t *testing.T) {
	injected := "VersionString"
	VersionStr = injected

	v := Version()
	if v != injected {
		t.Errorf("Wrong version returned %v, expecting %v", v, injected)
	}
}

func TestFallbackVersion(t *testing.T) {
	VersionStr = ""

	v := Version()
	if v != fallbackVersion {
		t.Errorf("Wrong version returned %v, expecting %v", v, fallbackVersion)
	}
}

func TestReadVersionFile(t *testing.T) {
	VersionStr = ""
	ex, err := os.Executable()
	if err != nil {
		t.Fatalf("cannot get the path for current executable binary: %v", err)
	}
	vfp := filepath.Join(filepath.Dir(ex), versionFilename)
	expectedVersion := "TEST_VERSION"

	if err = ioutil.WriteFile(vfp, []byte(expectedVersion), 0644); err != nil {
		t.Fatalf("failed to write version file at %v: %v", vfp, err)
	}
	defer os.Remove(vfp)

	v := Version()
	if v != expectedVersion {
		t.Errorf("Wrong version returned %v, expecting %v", v, expectedVersion)
	}
}

func TestBuild(t *testing.T) {
	bstr := "SOME BUILD STR"
	BuildStr = bstr

	b := Build()
	if b != bstr {
		t.Errorf("wrong build string returne %v, expecting %v", b, bstr)
	}
}

func TestFullVersion(t *testing.T) {
	VersionStr = "VSTR"
	BuildStr = "BSTR"

	fv := FullVersion()
	fvp := strings.Split(fv, " ")
	if fvp[0] != "CWAgent/VSTR" || fvp[4] != "BSTR" {
		t.Errorf("wrong FullVersion returned '%v' VSTR and BSTR not found", fv)
	}
}

func TestPlugins(t *testing.T) {
	InputPlugins = []string{"a", "b", "c"}
	OutputPlugins = []string{"x", "y", "z"}

	isRunningAsRoot = func() bool { return true }
	plugins := Plugins()
	expected := fmt.Sprintf("inputs:(a b c) outputs:(x y z)")
	if plugins != expected {
		t.Errorf("wrong plugins string constructed '%v', expecting '%v'", plugins, expected)
	}

	isRunningAsRoot = func() bool { return false }
	plugins = Plugins()
	expected = fmt.Sprintf("inputs:(a b c run_as_user) outputs:(x y z)")
	if plugins != expected {
		t.Errorf("wrong plugins string constructed '%v', expecting '%v'", plugins, expected)
	}
}

func TestUserAgent(t *testing.T) {
	userAgent = ""
	VersionStr = "VSTR"
	BuildStr = "BSTR"
	InputPlugins = []string{"a", "b", "c"}
	OutputPlugins = []string{"x", "y", "z"}

	isRunningAsRoot = func() bool { return true }

	ua := UserAgent()
	expected := fmt.Sprintf("CWAgent/VSTR (%v; %v; %v) BSTR inputs:(a b c) outputs:(x y z)", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if ua != expected {
		t.Errorf("wrong UserAgent string constructed '%v', expecting '%v'", ua, expected)
	}
}

func TestUserAgentEnvOverride(t *testing.T) {
	userAgent = ""
	os.Setenv(envconfig.CWAGENT_USER_AGENT, "CUSTOM CWAGENT USER AGENT")
	expected := "CUSTOM CWAGENT USER AGENT"

	ua := UserAgent()
	if ua != expected {
		t.Errorf("UserAgent should use value configured in environment variable CWAGENT_USER_AGENT, but '%v' found, expecting '%v'", ua, expected)
	}
}
