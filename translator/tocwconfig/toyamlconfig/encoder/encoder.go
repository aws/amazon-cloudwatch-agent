// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package encoder

type Encoder interface {
	Encode(in interface{}, out interface{}) error
}
