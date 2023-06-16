// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translator

// Concrete Rule implementation should return a (key string,val interface{})
type Rule interface {
	ApplyRule(interface{}) (string, interface{})
}
