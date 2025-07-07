// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"log"
)

// HeaderReserveValidator provides validation and debugging for CloudWatch header reserve calculations
type HeaderReserveValidator struct {
	MaxEventSize       int
	HeaderReserve      int
	TruncationSuffix   string
	MinContentSize     int
	ActualHeaderBytes  int // From CloudWatch API specification
}

// NewHeaderReserveValidator creates a new validator with current settings
func NewHeaderReserveValidator() *HeaderReserveValidator {
	return &HeaderReserveValidator{
		MaxEventSize:       262144, // 256 KB
		HeaderReserve:      cloudWatchHeaderReserve,
		TruncationSuffix:   "[Truncated...]",
		MinContentSize:     minContentSize,
		ActualHeaderBytes:  52, // Per CloudWatch Logs API specification
	}
}

// ValidateConfiguration checks if the current header reserve settings are appropriate
func (v *HeaderReserveValidator) ValidateConfiguration() {
	log.Printf("[HEADER VALIDATION] CloudWatch Agent Header Reserve Configuration:")
	log.Printf("  ========================================")
	log.Printf("  Max Event Size: %d bytes (%.2f KB)", v.MaxEventSize, float64(v.MaxEventSize)/1024)
	log.Printf("  Current Header Reserve: %d bytes (%.2f KB)", v.HeaderReserve, float64(v.HeaderReserve)/1024)
	log.Printf("  Actual CloudWatch Header: %d bytes", v.ActualHeaderBytes)
	log.Printf("  Truncation Suffix: '%s' (%d bytes)", v.TruncationSuffix, len(v.TruncationSuffix))
	log.Printf("  Minimum Content Size: %d bytes", v.MinContentSize)
	log.Printf("  ========================================")
	
	// Calculate effective sizes
	totalReserve := v.HeaderReserve + len(v.TruncationSuffix)
	effectiveMaxSize := v.MaxEventSize - totalReserve
	actualReserveNeeded := v.ActualHeaderBytes + len(v.TruncationSuffix)
	optimalEffectiveSize := v.MaxEventSize - actualReserveNeeded
	
	log.Printf("  Current Calculation:")
	log.Printf("    - Total Reserve: %d bytes (header: %d + suffix: %d)", totalReserve, v.HeaderReserve, len(v.TruncationSuffix))
	log.Printf("    - Effective Max Size: %d bytes (%.2f KB)", effectiveMaxSize, float64(effectiveMaxSize)/1024)
	log.Printf("    - Content Utilization: %.1f%%", float64(effectiveMaxSize)/float64(v.MaxEventSize)*100)
	
	log.Printf("  Optimal Calculation (based on API spec):")
	log.Printf("    - Actual Reserve Needed: %d bytes (header: %d + suffix: %d)", actualReserveNeeded, v.ActualHeaderBytes, len(v.TruncationSuffix))
	log.Printf("    - Optimal Effective Size: %d bytes (%.2f KB)", optimalEffectiveSize, float64(optimalEffectiveSize)/1024)
	log.Printf("    - Optimal Content Utilization: %.1f%%", float64(optimalEffectiveSize)/float64(v.MaxEventSize)*100)
	
	// Analysis and recommendations
	log.Printf("  ========================================")
	log.Printf("  ANALYSIS:")
	
	if v.HeaderReserve > v.ActualHeaderBytes*10 {
		log.Printf("  ⚠️  WARNING: Header reserve is %.1fx larger than actual CloudWatch header size", float64(v.HeaderReserve)/float64(v.ActualHeaderBytes))
		log.Printf("      This may cause unnecessary truncation of log content")
	}
	
	if effectiveMaxSize < v.MinContentSize {
		log.Printf("  ❌ ERROR: Effective max size (%d) is less than minimum content size (%d)", effectiveMaxSize, v.MinContentSize)
		log.Printf("      This will cause over-truncation and potentially create very short log entries")
	}
	
	wastedSpace := v.HeaderReserve - v.ActualHeaderBytes
	if wastedSpace > 1024 {
		log.Printf("  💡 RECOMMENDATION: Consider reducing header reserve by %d bytes (%.2f KB)", wastedSpace, float64(wastedSpace)/1024)
		log.Printf("      This would increase available content space from %d to %d bytes", effectiveMaxSize, optimalEffectiveSize)
	}
	
	log.Printf("  ========================================")
}

// CalculateOptimalReserve suggests an optimal header reserve size
func (v *HeaderReserveValidator) CalculateOptimalReserve() int {
	// CloudWatch API header (52 bytes) + safety margin (200 bytes) + truncation suffix
	safetyMargin := 200 // Conservative safety margin for log group/stream names
	return v.ActualHeaderBytes + safetyMargin
}

// TestTruncationScenarios tests various message sizes to predict truncation behavior
func (v *HeaderReserveValidator) TestTruncationScenarios() {
	log.Printf("[TRUNCATION SCENARIOS] Testing various message sizes:")
	log.Printf("  ========================================")
	
	testSizes := []int{
		100 * 1024,    // 100 KB
		200 * 1024,    // 200 KB
		240 * 1024,    // 240 KB
		256 * 1024,    // 256 KB (CloudWatch limit)
		260 * 1024,    // 260 KB
		300 * 1024,    // 300 KB
		500 * 1024,    // 500 KB
	}
	
	for _, size := range testSizes {
		v.testSingleSize(size)
	}
	
	log.Printf("  ========================================")
}

func (v *HeaderReserveValidator) testSingleSize(messageSize int) {
	totalReserve := v.HeaderReserve + len(v.TruncationSuffix)
	effectiveMaxSize := v.MaxEventSize - totalReserve
	
	willTruncate := messageSize > v.MaxEventSize
	finalSize := messageSize
	if willTruncate {
		finalSize = effectiveMaxSize + len(v.TruncationSuffix)
	}
	
	status := "✅ PASS"
	if willTruncate {
		if effectiveMaxSize < v.MinContentSize {
			status = "❌ OVER-TRUNCATED"
		} else {
			status = "⚠️  TRUNCATED"
		}
	}
	
	log.Printf("  Message Size: %6.1f KB → Final: %6.1f KB | %s", 
		float64(messageSize)/1024, 
		float64(finalSize)/1024, 
		status)
	
	if willTruncate {
		contentRemaining := float64(effectiveMaxSize) / float64(messageSize) * 100
		log.Printf("    Content Remaining: %.1f%% (%d bytes)", contentRemaining, effectiveMaxSize)
	}
}
