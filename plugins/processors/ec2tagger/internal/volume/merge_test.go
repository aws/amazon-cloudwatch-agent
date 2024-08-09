// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package volume

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockProvider struct {
	serialMap map[string]string
	err       error
}

func (m *mockProvider) DeviceToSerialMap() (map[string]string, error) {
	return m.serialMap, m.err
}

func TestMergeProvider(t *testing.T) {
	errFirstTest := errors.New("skip first")
	errSecondTest := errors.New("skip second")
	testCases := map[string]struct {
		providers     []Provider
		wantSerialMap map[string]string
		wantErr       error
	}{
		"WithErrors": {
			providers: []Provider{
				&mockProvider{err: errFirstTest},
				&mockProvider{err: errSecondTest},
			},
			wantErr: errSecondTest,
		},
		"WithPartialError": {
			providers: []Provider{
				&mockProvider{err: errFirstTest},
				&mockProvider{serialMap: map[string]string{
					"key": "value",
				}},
			},
			wantSerialMap: map[string]string{
				"key": "value",
			},
		},
		"WithMerge": {
			providers: []Provider{
				&mockProvider{serialMap: map[string]string{
					"foo": "bar",
					"key": "first",
				}},
				&mockProvider{serialMap: map[string]string{
					"key":   "second",
					"hello": "world",
				}},
			},
			wantSerialMap: map[string]string{
				"foo":   "bar",
				"key":   "second",
				"hello": "world",
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			p := newMergeProvider(testCase.providers)
			got, err := p.DeviceToSerialMap()
			assert.ErrorIs(t, err, testCase.wantErr)
			assert.Equal(t, testCase.wantSerialMap, got)
		})
	}
}
