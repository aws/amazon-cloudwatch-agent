// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeDollarDigit(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "bare $1", in: "replacement: $1", want: "replacement: $$1"},
		{name: "bare $0", in: "$0", want: "$$0"},
		{name: "bare $9", in: "value=$9", want: "value=$$9"},
		{name: "multiple refs", in: "$1 and $2 and $3", want: "$$1 and $$2 and $$3"},
		{name: "already escaped $$1", in: "$$1", want: "$$$1"},
		{name: "dollar non-digit", in: "$HOME and $PATH", want: "$HOME and $PATH"},
		{name: "no dollar", in: "no dollars here", want: "no dollars here"},
		{name: "empty string", in: "", want: ""},
		{name: "dollar at end", in: "trailing$", want: "trailing$"},
		{name: "multi-digit $10", in: "$10", want: "$$10"},
		{name: "mixed text", in: "tag: k8s.label.$1 and $FOO", want: "tag: k8s.label.$$1 and $FOO"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeDollarDigit(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}
