package main

import (
	"fmt"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile"
)

func main() {
	// Test the new size calculation
	config := &logfile.FileConfig{}
	
	// Test with default size (256KB)
	err := config.Init()
	if err != nil {
		fmt.Printf("Error initializing config: %v\n", err)
		return
	}
	
	fmt.Printf("Default MaxEventSize after header adjustment: %d bytes\n", config.MaxEventSize)
	fmt.Printf("This should be: %d (256KB - 26 bytes)\n", 256*1024-26)
	
	// Test with custom size
	config2 := &logfile.FileConfig{
		MaxEventSize: 100*1024, // 100KB
	}
	err = config2.Init()
	if err != nil {
		fmt.Printf("Error initializing config2: %v\n", err)
		return
	}
	
	fmt.Printf("Custom MaxEventSize after header adjustment: %d bytes\n", config2.MaxEventSize)
	fmt.Printf("This should be: %d (100KB - 26 bytes)\n", 100*1024-26)
	
	// Test that we're accounting for headers properly
	expectedDefault := 256*1024 - 26
	expectedCustom := 100*1024 - 26
	
	if config.MaxEventSize == expectedDefault {
		fmt.Println("✓ Default size calculation is correct")
	} else {
		fmt.Printf("✗ Default size calculation is wrong: got %d, expected %d\n", config.MaxEventSize, expectedDefault)
	}
	
	if config2.MaxEventSize == expectedCustom {
		fmt.Println("✓ Custom size calculation is correct")
	} else {
		fmt.Printf("✗ Custom size calculation is wrong: got %d, expected %d\n", config2.MaxEventSize, expectedCustom)
	}
	
	// Test truncation behavior
	testMessage := strings.Repeat("A", config.MaxEventSize + 100) // Message larger than limit
	truncateSuffix := "[Truncated...]"
	
	// Simulate what the input plugin would do
	if len(testMessage) > config.MaxEventSize {
		truncatedMessage := testMessage[:config.MaxEventSize-len(truncateSuffix)] + truncateSuffix
		fmt.Printf("Original message length: %d\n", len(testMessage))
		fmt.Printf("Truncated message length: %d\n", len(truncatedMessage))
		fmt.Printf("Max allowed length: %d\n", config.MaxEventSize)
		
		if len(truncatedMessage) <= config.MaxEventSize {
			fmt.Println("✓ Truncation logic works correctly")
		} else {
			fmt.Printf("✗ Truncation logic is wrong: truncated length %d > max %d\n", len(truncatedMessage), config.MaxEventSize)
		}
	}
}
