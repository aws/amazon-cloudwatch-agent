// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import "fmt"

// ScopeStatementsForSolution returns OTTL scope-context statements that set cloudwatch.source and cloudwatch.solution attributes.
func ScopeStatementsForSolution(solution string) []string {
	return []string{
		`set(scope.attributes["cloudwatch.source"], "cloudwatch-agent")`,
		fmt.Sprintf(`set(scope.attributes["cloudwatch.solution"], "%s")`, solution),
	}
}
