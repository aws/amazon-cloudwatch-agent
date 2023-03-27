package migrate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"
)

func getTests() ([]string, error) {
	files, err := os.ReadDir("new/")
	if err != nil {
		return nil, fmt.Errorf("unable to list test case dir: %v", err)
	}
	var tests []string
	for _, file := range files {
		name := file.Name()
		if filepath.Ext(name) == ".conf" {
			tests = append(tests, name)
		}
	}
	return tests, nil
}

func TestIsOldConfig(t *testing.T) {
	tests, err := getTests()
	if err != nil {
		t.Fatalf("Failed to get tests: %v", err)
	}

	for _, test := range tests {
		ofp := fmt.Sprintf("old/%v", test)
		isOld, err := IsOldConfig(ofp)
		if err != nil {
			t.Errorf("Failed to check if config file %v is old: %v", ofp, err)
		}
		if !isOld {
			t.Errorf("Failed to detect old config %v", ofp)
		}

		nfp := fmt.Sprintf("new/%v", test)
		isOld, err = IsOldConfig(nfp)
		if err != nil {
			t.Errorf("Failed to check if config file %v is old: %v", nfp, err)
		}
		if isOld {
			t.Errorf("Failed to detect new config %v", ofp)
		}
	}
}

func TestMigrateFile(t *testing.T) {
	tests, err := getTests()
	if err != nil {
		t.Fatalf("Failed to get tests: %v", err)
	}
	//tests := []string{"advanced_config_linux.conf"}

	for _, test := range tests {
		ofp := fmt.Sprintf("old/%v", test)
		mf, err := MigrateFile(ofp)
		if mf != "" {
			defer os.Remove(mf)
		}
		if err != nil {
			t.Errorf("Failed to migrate file %v: %v", ofp, err)
			continue
		}

		mcb, err := os.ReadFile(mf)
		if err != nil {
			t.Errorf("Failed to read test file '%v' from old folder: %v", test, err)
			continue
		}
		var mc map[string]interface{}

		if err := toml.Unmarshal(mcb, &mc); err != nil {
			t.Errorf("Failed to unmarshal old test file '%v': %v", test, err)
			continue
		}

		ncb, err := os.ReadFile(fmt.Sprintf("new/%v", test))
		if err != nil {
			t.Errorf("Failed to read test file '%v' from new folder: %v", test, err)
			continue
		}
		var nc map[string]interface{}

		if err := toml.Unmarshal(ncb, &nc); err != nil {
			t.Errorf("Failed to unmarshal new test file '%v': %v", test, err)
			continue
		}

		df := diff("", mc, nc)
		if len(df) > 0 {
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "%v : MigrateFile config does not match expectation:\n", test)
			for n, v := range df {
				fmt.Fprintf(&buf, "\n%-40v : %v\n", n, v)
			}
			t.Error(buf.String())
		}
	}
}

func TestMigrateConfigs(t *testing.T) {
	tests, err := getTests()
	if err != nil {
		t.Fatalf("Failed to get tests: %v", err)
	}

	for _, test := range tests {
		ocb, err := os.ReadFile(fmt.Sprintf("old/%v", test))
		if err != nil {
			t.Fatalf("Failed to read test file '%v' from old folder: %v", test, err)
		}
		var oc map[string]interface{}

		if err := toml.Unmarshal(ocb, &oc); err != nil {
			t.Fatalf("Failed to unmarshal old test file '%v': %v", test, err)
		}

		ncb, err := os.ReadFile(fmt.Sprintf("new/%v", test))
		if err != nil {
			t.Fatalf("Failed to read test file '%v' from new folder: %v", test, err)
		}
		var nc map[string]interface{}

		if err := toml.Unmarshal(ncb, &nc); err != nil {
			t.Fatalf("Failed to unmarshal new test file '%v': %v", test, err)
		}

		Migrate(oc)

		df := diff("", oc, nc)
		if len(df) > 0 {
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "%v : Migrated config does not match expectation:\n", test)
			for n, v := range df {
				fmt.Fprintf(&buf, "\n%-40v : %v\n", n, v)
			}
			t.Error(buf.String())
		}
	}
}

func diff(name string, a, b interface{}) map[string]string {
	r := make(map[string]string)

	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		r[name] = fmt.Sprintf("left type %T and right type %T are different", a, b)
		return r
	}

	switch va := a.(type) {
	case int, int32, int64, float32, float64, string:
		if a != b {
			r[name] = fmt.Sprintf("%v != %v", a, b)
		}
	case []map[string]interface{}:
		vb := b.([]map[string]interface{})
		if len(va) != len(vb) {
			r[name] = fmt.Sprintf("array length %v != %v", len(va), len(vb))
			break
		}
		for i, vai := range va {
			vbi := vb[i]
			ni := fmt.Sprintf("%v[%v]", name, i)
			rr := diff(ni, vai, vbi)
			for rrn, rrv := range rr {
				r[rrn] = rrv
			}
		}
	case map[string]interface{}:
		vb := b.(map[string]interface{})
		for vak, vav := range va {
			vbv, ok := vb[vak]
			ni := fmt.Sprintf("%v.%v", name, vak)
			if !ok {
				r[ni] = fmt.Sprintf("right side missing %v = %v", vak, vav)
				continue
			}
			rr := diff(ni, vav, vbv)
			for rrn, rrv := range rr {
				r[rrn] = rrv
			}
		}
		for vbk, vbv := range vb {
			_, ok := va[vbk]
			if !ok {
				ni := fmt.Sprintf("%v.%v", name, vbk)
				r[ni] = fmt.Sprintf("left side missing %v = %v", vbk, vbv)
			}
		}
	case []interface{}:
		vb := b.([]interface{})
		if len(va) != len(vb) {
			r[name] = fmt.Sprintf("array length %v != %v", len(va), len(vb))
			break
		}
		for i, vai := range va {
			vbi := vb[i]
			ni := fmt.Sprintf("%v[%v]", name, i)
			rr := diff(ni, vai, vbi)
			for rrn, rrv := range rr {
				r[rrn] = rrv
			}
		}
	}
	return r
}
