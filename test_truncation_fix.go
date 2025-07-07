// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// Test the truncation fix by simulating the calculations
func main() {
	log.SetOutput(os.Stdout)
	
	fmt.Println("=== CloudWatch Agent Truncation Fix Test ===")
	fmt.Println()
	
	// Current settings (before fix)
	fmt.Println("BEFORE FIX:")
	testTruncationLogic(262144, 8192, "[Truncated...]", 0)
	
	fmt.Println()
	fmt.Println("AFTER FIX:")
	testTruncationLogic(262144, 1024, "[Truncated...]", 100)
	
	fmt.Println()
	fmt.Println("=== Test Cases ===")
	testSpecificCases()
}

func testTruncationLogic(maxEventSize, headerReserve int, truncateSuffix string, minContentSize int) {
	fmt.Printf("  Max Event Size: %d bytes (%.1f KB)\n", maxEventSize, float64(maxEventSize)/1024)
	fmt.Printf("  Header Reserve: %d bytes (%.1f KB)\n", headerReserve, float64(headerReserve)/1024)
	fmt.Printf("  Truncation Suffix: '%s' (%d bytes)\n", truncateSuffix, len(truncateSuffix))
	fmt.Printf("  Min Content Size: %d bytes\n", minContentSize)
	
	baseReserve := headerReserve + len(truncateSuffix)
	effectiveMaxSize := maxEventSize - baseReserve
	
	// Apply minimum content size check
	if minContentSize > 0 && effectiveMaxSize < minContentSize {
		fmt.Printf("  ⚠️  Effective size (%d) < Min content (%d), adjusting...\n", effectiveMaxSize, minContentSize)
		effectiveMaxSize = minContentSize
		if effectiveMaxSize + len(truncateSuffix) > maxEventSize {
			effectiveMaxSize = maxEventSize - len(truncateSuffix)
		}
	}
	
	fmt.Printf("  Base Reserve: %d bytes\n", baseReserve)
	fmt.Printf("  Effective Max Size: %d bytes (%.1f KB)\n", effectiveMaxSize, float64(effectiveMaxSize)/1024)
	fmt.Printf("  Content Utilization: %.1f%%\n", float64(effectiveMaxSize)/float64(maxEventSize)*100)
}

func testSpecificCases() {
	testCases := []struct {
		name        string
		messageSize int
		content     string
	}{
		{"Small log", 200, "Regular log message"},
		{"240 KB log", 240 * 1024, strings.Repeat("D", 240*1024)},
		{"256 KB log", 256 * 1024, strings.Repeat("X", 256*1024)},
		{"260 KB log", 260 * 1024, strings.Repeat("Y", 260*1024)},
		{"300 KB log", 300 * 1024, strings.Repeat("Z", 300*1024)},
	}
	
	for _, tc := range testCases {
		fmt.Printf("\n--- %s (%d bytes) ---\n", tc.name, tc.messageSize)
		
		// Test with old settings
		fmt.Println("OLD LOGIC:")
		result := simulateTruncation(tc.messageSize, tc.content, 262144, 8192, "[Truncated...]", 0)
		fmt.Printf("  Result: %d bytes, Content: '%.50s...'\n", len(result), result)
		
		// Test with new settings
		fmt.Println("NEW LOGIC:")
		result = simulateTruncation(tc.messageSize, tc.content, 262144, 1024, "[Truncated...]", 100)
		fmt.Printf("  Result: %d bytes, Content: '%.50s...'\n", len(result), result)
	}
}

func simulateTruncation(messageSize int, content string, maxEventSize, headerReserve int, truncateSuffix string, minContentSize int) string {
	if messageSize <= maxEventSize {
		return content // No truncation needed
	}
	
	baseReserve := headerReserve + len(truncateSuffix)
	effectiveMaxSize := maxEventSize - baseReserve
	
	// Apply minimum content size check
	if minContentSize > 0 && effectiveMaxSize < minContentSize {
		effectiveMaxSize = minContentSize
		if effectiveMaxSize + len(truncateSuffix) > maxEventSize {
			effectiveMaxSize = maxEventSize - len(truncateSuffix)
		}
	}
	
	if effectiveMaxSize <= 0 {
		return truncateSuffix // Edge case
	}
	
	if len(content) > effectiveMaxSize {
		return content[:effectiveMaxSize] + truncateSuffix
	}
	
	return content + truncateSuffix
}
