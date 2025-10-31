// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadProperties(t *testing.T) {
	testCases := map[string]struct {
		input     string
		separator rune
		want      map[string]string
		wantErr   error
	}{
		"BasicProperties": {
			input:     "key1=value1\nkey2=value2\n",
			separator: '=',
			want:      map[string]string{"key1": "value1", "key2": "value2"},
		},
		"ManifestFormat": {
			input:     "Main-Class: com.example.App\nImplementation-Title: MyApp\n",
			separator: ':',
			want:      map[string]string{"Main-Class": "com.example.App", "Implementation-Title": "MyApp"},
		},
		"WithComments": {
			input:     "# This is a comment\nkey1=value1\n# Another comment\nkey2=value2\n",
			separator: '=',
			want:      map[string]string{"key1": "value1", "key2": "value2"},
		},
		"WithWhitespace": {
			input:     "  key1  =  value1  \n\t key2\t=\tvalue2\t\n",
			separator: '=',
			want:      map[string]string{"key1": "value1", "key2": "value2"},
		},
		"EmptyLines": {
			input:     "key1=value1\n\n\nkey2=value2\n",
			separator: '=',
			want:      map[string]string{"key1": "value1", "key2": "value2"},
		},
		"NoSeparator": {
			input:     "key1\nkey2=value2\n",
			separator: '=',
			want:      map[string]string{"key2": "value2"},
		},
		"EmptyValue": {
			input:     "key1=\nkey2=value2\n",
			separator: '=',
			want:      map[string]string{"key1": "", "key2": "value2"},
		},
		"EmptyInput": {
			input:     "",
			separator: '=',
			want:      map[string]string{},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			reader := strings.NewReader(testCase.input)
			got := make(map[string]string)

			err := ReadProperties(reader, testCase.separator, func(key, value string) bool {
				got[key] = value
				return true
			})

			if testCase.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, testCase.wantErr, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.want, got)
			}
		})
	}
}

func TestReadProperties_EarlyReturn(t *testing.T) {
	input := "key1=value1\nkey2=value2\nkey3=value3\n"
	reader := strings.NewReader(input)
	got := make(map[string]string)

	err := ReadProperties(reader, '=', func(key, value string) bool {
		got[key] = value
		return key != "key2" // Stop after key2
	})

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"key1": "value1", "key2": "value2"}, got)
}

func TestReadProperties_LineLimit(t *testing.T) {
	testCases := map[string]struct {
		input     string
		separator rune
		lineLimit int
		want      map[string]string
		wantErr   error
	}{
		"WithinLimit": {
			input:     "key1=value1\nkey2=value2\n",
			separator: '=',
			lineLimit: 5,
			want:      map[string]string{"key1": "value1", "key2": "value2"},
		},
		"ExceedsLimit": {
			input:     "key1=value1\nkey2=value2\nkey3=value3\n",
			separator: '=',
			lineLimit: 2,
			want:      map[string]string{"key1": "value1", "key2": "value2"},
			wantErr:   ErrLineLimitExceeded,
		},
		"NoLimit": {
			input:     "key1=value1\nkey2=value2\nkey3=value3\n",
			separator: '=',
			lineLimit: 0,
			want:      map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"},
		},
		"LimitWithComments": {
			input:     "# Comment\nkey1=value1\n# Another comment\nkey2=value2\n",
			separator: '=',
			lineLimit: 3,
			want:      map[string]string{"key1": "value1"},
			wantErr:   ErrLineLimitExceeded,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			reader := strings.NewReader(testCase.input)
			got := make(map[string]string)

			err := readPropertiesWithLimit(reader, testCase.separator, testCase.lineLimit, func(key, value string) bool {
				got[key] = value
				return true
			})

			if testCase.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, testCase.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, testCase.want, got)
		})
	}
}
