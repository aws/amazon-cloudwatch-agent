// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import "testing"

func TestEscapeDollarDigit(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"no dollar", "hello world", "hello world"},
		{"dollar at end", "regex$", "regex$"},
		{"dollar letter", "$FOO", "$FOO"},
		{"dollar brace", "${FOO}", "${FOO}"},
		{"bare $1", "replacement: $1", "replacement: $$1"},
		{"bare $0", "$0", "$$0"},
		{"bare $9", "$9", "$$9"},
		{"already escaped $$1", "$$1", "$$$1"},
		{"triple $$$1", "$$$1", "$$$$1"},
		{"consecutive $1$2", "$1$2", "$$1$$2"},
		{"multi-digit $10", "$10", "$$10"},
		{"mixed text", "tag: k8s.label.$1 and $FOO", "tag: k8s.label.$$1 and $FOO"},
		{"dollar dollar no digit", "$$FOO", "$$FOO"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeDollarDigit(tt.in)
			if got != tt.want {
				t.Errorf("escapeDollarDigit(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
