package main

import (
	"fmt"
	"unsafe"

	"github.com/aws/amazon-cloudwatch-agent/internal/nvme"
)

func main() {
	fmt.Println("InstanceStoreMetrics size:", unsafe.Sizeof(nvme.InstanceStoreMetrics{}))
	fmt.Println("EBSMetrics size:", unsafe.Sizeof(nvme.EBSMetrics{}))
}
