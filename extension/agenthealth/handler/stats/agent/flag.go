// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"encoding"
	"errors"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
)

var (
	errUnsupportedFlag = errors.New("unsupported usage flag")
)

const (
	FlagIMDSFallbackSuccess Flag = iota
	FlagSharedConfigFallback
	FlagAppSignal
	FlagEnhancedContainerInsights
	FlagRunningInContainer
	FlagMode
	FlagRegionType

	flagIMDSFallbackSuccessStr       = "imds_fallback_success"
	flagSharedConfigFallbackStr      = "shared_config_fallback"
	flagAppSignalsStr                = "application_signals"
	flagEnhancedContainerInsightsStr = "enhanced_container_insights"
	flagRunningInContainerStr        = "running_in_container"
	flagModeStr                      = "mode"
	flagRegionTypeStr                = "region_type"
)

type Flag int

var _ encoding.TextMarshaler = (*Flag)(nil)
var _ encoding.TextUnmarshaler = (*Flag)(nil)

func (f Flag) String() string {
	switch f {
	case FlagAppSignal:
		return flagAppSignalsStr
	case FlagEnhancedContainerInsights:
		return flagEnhancedContainerInsightsStr
	case FlagIMDSFallbackSuccess:
		return flagIMDSFallbackSuccessStr
	case FlagMode:
		return flagModeStr
	case FlagRegionType:
		return flagRegionTypeStr
	case FlagRunningInContainer:
		return flagRunningInContainerStr
	case FlagSharedConfigFallback:
		return flagSharedConfigFallbackStr
	}
	return ""
}

func (f Flag) MarshalText() (text []byte, err error) {
	s := f.String()
	if s == "" {
		return nil, fmt.Errorf("%w: %[2]T(%[2]d)", errUnsupportedFlag, f)
	}
	return []byte(s), nil
}

func (f *Flag) UnmarshalText(text []byte) error {
	switch s := string(text); s {
	case flagAppSignalsStr:
		*f = FlagAppSignal
	case flagEnhancedContainerInsightsStr:
		*f = FlagEnhancedContainerInsights
	case flagIMDSFallbackSuccessStr:
		*f = FlagIMDSFallbackSuccess
	case flagModeStr:
		*f = FlagMode
	case flagRegionTypeStr:
		*f = FlagRegionType
	case flagRunningInContainerStr:
		*f = FlagRunningInContainer
	case flagSharedConfigFallbackStr:
		*f = FlagSharedConfigFallback
	default:
		return fmt.Errorf("%w: %s", errUnsupportedFlag, s)
	}
	return nil
}

var (
	flagSingleton FlagSet
	flagOnce      sync.Once
)

// FlagSet is a getter/setter for flag/value pairs. Once a flag key is set, its value is immutable.
type FlagSet interface {
	// IsSet returns if the flag is present in the backing map.
	IsSet(flag Flag) bool
	// GetString if the value stored with the flag is a string. If not, returns nil.
	GetString(flag Flag) *string
	// Set adds the Flag with an unused value.
	Set(flag Flag)
	// SetValue adds the Flag with a value.
	SetValue(flag Flag, value any)
	// SetValues adds each Flag/value pair.
	SetValues(flags map[Flag]any)
	// OnChange registers a callback that triggers on flag sets.
	OnChange(callback func())
}

type flagSet struct {
	m         sync.Map
	mu        sync.RWMutex
	callbacks []func()
}

var _ FlagSet = (*flagSet)(nil)

func (p *flagSet) IsSet(flag Flag) bool {
	_, ok := p.m.Load(flag)
	return ok
}

func (p *flagSet) GetString(flag Flag) *string {
	value, ok := p.m.Load(flag)
	if !ok {
		return nil
	}
	var str string
	str, ok = value.(string)
	if !ok || str == "" {
		return nil
	}
	return aws.String(str)
}

func (p *flagSet) Set(flag Flag) {
	p.SetValue(flag, 1)
}

func (p *flagSet) SetValue(flag Flag, value any) {
	if p.setWithValue(flag, value) {
		p.notify()
	}
}

func (p *flagSet) SetValues(m map[Flag]any) {
	var changed bool
	for flag, value := range m {
		if p.setWithValue(flag, value) {
			changed = true
		}
	}
	if changed {
		p.notify()
	}
}

func (p *flagSet) setWithValue(flag Flag, value any) bool {
	if !p.IsSet(flag) {
		p.m.Store(flag, value)
		return true
	}
	return false
}

func (p *flagSet) OnChange(f func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks = append(p.callbacks, f)
}

func (p *flagSet) notify() {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, callback := range p.callbacks {
		callback()
	}
}

func UsageFlags() FlagSet {
	flagOnce.Do(func() {
		flagSingleton = &flagSet{}
	})
	return flagSingleton
}
