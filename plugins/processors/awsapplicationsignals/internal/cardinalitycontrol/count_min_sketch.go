// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cardinalitycontrol

import (
	"hash/adler32"
	"hash/crc32"
	"hash/fnv"
)

type CountMinSketchHashFunc func(hashKey string) int64

type CountMinSketchEntry interface {
	HashKey() string
	Frequency() int
}

type CountMinSketch struct {
	depth     int
	maxDepth  int
	width     int
	matrix    [][]int
	hashFuncs []CountMinSketchHashFunc
}

func (cms *CountMinSketch) Insert(obj CountMinSketchEntry) {
	for i := 0; i < cms.depth; i++ {
		hashFunc := cms.hashFuncs[i]
		hashValue := hashFunc(obj.HashKey())
		pos := int(hashValue % int64(cms.width))

		cms.matrix[i][pos] += obj.Frequency()
	}
}

func NewCountMinSketch(depth, width int, hashFuncs ...CountMinSketchHashFunc) *CountMinSketch {
	matrix := make([][]int, depth)
	for i := range matrix {
		matrix[i] = make([]int, width)
	}
	cms := &CountMinSketch{
		depth:    0,
		maxDepth: depth,
		width:    width,
		matrix:   matrix,
	}
	if hashFuncs != nil {
		cms.RegisterHashFunc(hashFuncs...)
	} else {
		RegisterDefaultHashFuncs(cms)
	}
	return cms
}

func RegisterDefaultHashFuncs(cms *CountMinSketch) {
	hashFunc1 := func(hashKey string) int64 {
		h := fnv.New32a()
		h.Write([]byte(hashKey))
		return int64(h.Sum32())
	}
	hashFunc2 := func(hashKey string) int64 {
		hash := crc32.ChecksumIEEE([]byte(hashKey))
		return int64(hash)
	}
	hashFunc3 := func(hashKey string) int64 {
		hash := adler32.Checksum([]byte(hashKey))
		return int64(hash)
	}
	cms.RegisterHashFunc(hashFunc1, hashFunc2, hashFunc3)
}

func (cms *CountMinSketch) RegisterHashFunc(hashFuncs ...CountMinSketchHashFunc) {
	if cms.hashFuncs == nil {
		cms.hashFuncs = hashFuncs
	} else {
		cms.hashFuncs = append(cms.hashFuncs, hashFuncs...)
	}
	if cms.maxDepth < len(cms.hashFuncs) {
		cms.depth = cms.maxDepth
	} else {
		cms.depth = len(cms.hashFuncs)
	}
}

func (cms *CountMinSketch) Get(obj CountMinSketchEntry) int {
	minCount := int(^uint(0) >> 1) // Initialize with the maximum possible integer value
	for i := 0; i < cms.depth; i++ {
		hashFunc := cms.hashFuncs[i]
		hashValue := hashFunc(obj.HashKey())
		pos := int(hashValue % int64(cms.width))

		if cms.matrix[i][pos] < minCount {
			minCount = cms.matrix[i][pos]
		}
	}
	return minCount
}
