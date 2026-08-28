// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScopeStatementsForSolution(t *testing.T) {
	stmts := ScopeStatementsForSolution("otel-test")
	assert.Len(t, stmts, 2)
	assert.Equal(t, `set(scope.attributes["cloudwatch.source"], "cloudwatch-agent")`, stmts[0])
	assert.Equal(t, `set(scope.attributes["cloudwatch.solution"], "otel-test")`, stmts[1])
}
