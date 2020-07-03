package agentinfo

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	plugins := Plugins()
	expected := "inputs:(a b c) outputs:(x y z)"
	if plugins != expected {
		t.Errorf("wrong plugins string constructed '%v', expecting '%v'", plugins, expected)
	}
}
