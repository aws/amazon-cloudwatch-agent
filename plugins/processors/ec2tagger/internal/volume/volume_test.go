// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package volume

import (
	"errors"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvider(t *testing.T) {
	p := NewProvider(nil, "")
	mp, ok := p.(*mergeProvider)
	assert.True(t, ok)
	assert.Len(t, mp.providers, 2)
	_, ok = mp.providers[0].(*hostProvider)
	assert.True(t, ok)
	_, ok = mp.providers[1].(*describeVolumesProvider)
	assert.True(t, ok)
}

func TestCache(t *testing.T) {
	testErr := errors.New("test")
	p := &mockProvider{
		serialMap: map[string]string{
			"/dev/xvdf": "foo",
			"xvdc":      "bar",
			"xvdc1":     "baz",
		},
		err: testErr,
	}
	c := NewCache(nil).(*cache)
	c.fetchBlockName = func(s string) string {
		return ""
	}
	assert.ErrorIs(t, c.Refresh(), errNoProviders)
	c.provider = p
	assert.ErrorIs(t, c.Refresh(), testErr)
	p.err = nil
	assert.NoError(t, c.Refresh())
	assert.Equal(t, "foo", c.Serial("xvdf"))
	assert.Equal(t, "bar", c.Serial("xvdc"))
	assert.Equal(t, "baz", c.Serial("xvdc1"))
	assert.Equal(t, "bar", c.Serial("xvdc2"))
	assert.Equal(t, "", c.Serial("xvde"))
	got := c.Devices()
	sort.Strings(got)
	assert.Equal(t, []string{"xvdc", "xvdc1", "xvdf"}, got)
}
