// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package generator

import "context"

type Generator interface {
	Start(ctx context.Context) error
	Stop()
}
