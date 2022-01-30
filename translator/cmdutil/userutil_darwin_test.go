// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build darwin
// +build darwin

package cmdutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDSOutput(t *testing.T) {
	u := `name: darcy
password: ********
uid: 123456
gid: 78910
dir: /Users/darcy
shell: /bin/zsh
gecos: Fitzwilliam, Darcy
`
	m, err := parseDSOutput(u)
	require.Nil(t, err)
	assert.Equal(t, "darcy", m["name"])

	g := `name: PrideAndPrejudice
password: *
gid: 7788
`
	m, err = parseDSOutput(g)
	require.Nil(t, err)
	assert.Equal(t, "7788", m["gid"])
}
