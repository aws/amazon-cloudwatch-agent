package main

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/internal/nvme"
)

func main() {
	err := nvme.ValidateMetricBounds("volume_performance_exceeded_iops", 2000000000000, "/dev/nvme0n1")
	fmt.Printf("Error: %v\n", err)
	if err != nil {
		fmt.Printf("Error message: %s\n", err.Error())
	}

	// Test the limit
	fmt.Printf("Limit check: 2000000000000 > 1000000000000 = %v\n", 2000000000000 > 1000000000000)
}
