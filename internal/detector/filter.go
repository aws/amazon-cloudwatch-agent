// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package detector

import "context"

// ProcessFilter determines if a process should be included in the detection results.
type ProcessFilter interface {
	ShouldInclude(ctx context.Context, process Process) bool
}

// NameFilter determines based on the detected name if a resource should be included in the detection results.
type NameFilter interface {
	ShouldInclude(name string) bool
}
