// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rollupprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestNopCache(t *testing.T) {
	cache := &nopRollupCache{}
	key := cache.Key(pcommon.NewMap())
	assert.Equal(t, "", key)
	assert.Nil(t, cache.Get(key))
	assert.Nil(t, cache.Set(key, nil, time.Millisecond))
}

func TestCacheKey(t *testing.T) {
	testCases := []struct {
		attrs map[string]any
		want  string
	}{
		{
			attrs: map[string]any{},
			want:  "",
		},
		{
			attrs: map[string]any{
				"c": "v1",
				"d": "v2",
				"a": "v3",
				"b": "v4",
			},
			want: "a:v3|b:v4|c:v1|d:v2",
		},
		{
			attrs: map[string]any{
				"a": []any{"1", "3", "2"},
				"b": 1,
				"c": 2.5,
				"d": false,
			},
			want: `a:["1","3","2"]|b:1|c:2.5|d:false`,
		},
	}
	for _, testCase := range testCases {
		cache := buildRollupCache(1)
		attrs := pcommon.NewMap()
		require.NoError(t, attrs.FromRaw(testCase.attrs))
		assert.Equal(t, testCase.want, cache.Key(attrs))

		// validate no-op cache
		cache = buildRollupCache(0)
		assert.Equal(t, "", cache.Key(attrs))
	}
}
