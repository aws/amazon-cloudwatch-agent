// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rollupprocessor

import (
	"sort"
	"strings"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type rollupCache interface {
	Get(key string, opts ...ttlcache.Option[string, []pcommon.Map]) *ttlcache.Item[string, []pcommon.Map]
	Set(key string, value []pcommon.Map, ttl time.Duration) *ttlcache.Item[string, []pcommon.Map]
	Key(attrs pcommon.Map) string
	Start()
	Stop()
}

// ttlRollupCache is a wrapper for the ttlcache.Cache that implements the
// rollupCache interface.
type ttlRollupCache struct {
	*ttlcache.Cache[string, []pcommon.Map]
}

var _ rollupCache = (*ttlRollupCache)(nil)

func (c *ttlRollupCache) Key(attrs pcommon.Map) string {
	pairs := make([]string, 0, attrs.Len())
	attrs.Range(func(k string, v pcommon.Value) bool {
		pairs = append(pairs, k+":"+v.AsString())
		return true
	})
	sort.Strings(pairs)
	return strings.Join(pairs, "|")
}

// nopRollupCache used when the rollup cache is disabled.
type nopRollupCache struct {
}

var _ rollupCache = (*nopRollupCache)(nil)

func (c *nopRollupCache) Get(string, ...ttlcache.Option[string, []pcommon.Map]) *ttlcache.Item[string, []pcommon.Map] {
	return nil
}

func (c *nopRollupCache) Set(string, []pcommon.Map, time.Duration) *ttlcache.Item[string, []pcommon.Map] {
	return nil
}

func (c *nopRollupCache) Key(pcommon.Map) string {
	return ""
}

func (c *nopRollupCache) Start() {
}

func (c *nopRollupCache) Stop() {
}

func buildRollupCache(cacheSize int) rollupCache {
	if cacheSize <= 0 {
		return &nopRollupCache{}
	}
	return &ttlRollupCache{
		Cache: ttlcache.New[string, []pcommon.Map](
			ttlcache.WithCapacity[string, []pcommon.Map](uint64(cacheSize)),
		),
	}
}
