// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

type TranslatorOption func(any)

type NameSetter interface {
	SetName(string)
}

func WithName(name string) TranslatorOption {
	return func(target any) {
		if setter, ok := target.(NameSetter); ok {
			setter.SetName(name)
		}
	}
}

type NameProvider struct {
	name string
}

func (p *NameProvider) Name() string {
	return p.name
}

func (p *NameProvider) SetName(name string) {
	p.name = name
}
