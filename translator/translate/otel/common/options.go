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

type IndexSetter interface {
	SetIndex(int)
}

func WithIndex(index int) TranslatorOption {
	return func(target any) {
		if setter, ok := target.(IndexSetter); ok {
			setter.SetIndex(index)
		}
	}
}

type IndexProvider struct {
	index int
}

func (p *IndexProvider) Index() int {
	return p.index
}

func (p *IndexProvider) SetIndex(index int) {
	p.index = index
}

type DestinationSetter interface {
	SetDestination(string)
}

func WithDestination(name string) TranslatorOption {
	return func(target any) {
		if setter, ok := target.(DestinationSetter); ok {
			setter.SetDestination(name)
		}
	}
}

type DestinationProvider struct {
	destination string
}

func (p *DestinationProvider) Destination() string {
	return p.destination
}

func (p *DestinationProvider) SetDestination(destination string) {
	p.destination = destination
}
