package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
)

type testTranslator struct {
	cfgType config.Type
	result  int
}

var _ Translator[int] = (*testTranslator)(nil)

func (t testTranslator) Translate(_ *confmap.Conf) (int, error) {
	return t.result, nil
}

func (t testTranslator) Type() config.Type {
	return t.cfgType
}

func TestConfigKeys(t *testing.T) {
	require.Equal(t, "1::2", ConfigKey("1", "2"))
}

func TestGetString(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{"int": 10, "string": "test"})
	got, ok := GetString(conf, "int")
	require.True(t, ok)
	// converts int to string
	require.Equal(t, "10", got)
	got, ok = GetString(conf, "string")
	require.True(t, ok)
	require.Equal(t, "test", got)
	got, ok = GetString(conf, "invalid_key")
	require.False(t, ok)
	require.Equal(t, "", got)
}

func TestGetDuration(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{"invalid": "invalid", "valid": 1})
	got, ok := GetDuration(conf, "invalid")
	require.False(t, ok)
	require.Equal(t, time.Duration(0), got)
	got, ok = GetDuration(conf, "valid")
	require.True(t, ok)
	require.Equal(t, time.Second, got)
}

func TestParseDuration(t *testing.T) {
	testCases := map[string]struct {
		input   interface{}
		want    time.Duration
		wantErr bool
	}{
		"WithString/Duration": {input: "60s", want: time.Minute},
		"WithString/Int":      {input: "60", want: time.Minute},
		"WithString/Float":    {input: "60.7", want: time.Minute},
		"WithString/Invalid":  {input: "test", wantErr: true},
		"WithInt":             {input: 60, want: time.Minute},
		"WithInt64":           {input: int64(60), want: time.Minute},
		"WithFloat64":         {input: 59.7, want: 59 * time.Second},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseDuration(testCase.input)
			if testCase.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, testCase.want, got)
		})
	}
}

func TestTranslatorMap(t *testing.T) {
	got := NewTranslatorMap[int](&testTranslator{"test", 0}, &testTranslator{"other", 1})
	require.Len(t, got, 2)
	translator, ok := got.Get("test")
	require.True(t, ok)
	result, err := translator.Translate(nil)
	require.NoError(t, err)
	require.Equal(t, 0, result)
	got.Add(&testTranslator{"test", 2})
	require.Len(t, got, 2)
	translator, ok = got.Get("test")
	require.True(t, ok)
	result, err = translator.Translate(nil)
	require.NoError(t, err)
	require.Equal(t, 2, result)
}

func TestMissingKeyError(t *testing.T) {
	err := &MissingKeyError{Type: "type", JsonKey: "key"}
	require.Equal(t, "\"type\" missing key in JSON: \"key\"", err.Error())
}
