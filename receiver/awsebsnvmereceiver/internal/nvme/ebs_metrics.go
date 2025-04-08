// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// The following code is based on https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/pkg/metrics/nvme.go

// Copyright 2024 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the 'License');
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an 'AS IS' BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nvme

// EBSMetrics represents the parsed metrics from the NVMe log page.
type EBSMetrics struct {
	EBSMagic              uint64
	ReadOps               uint64
	WriteOps              uint64
	ReadBytes             uint64
	WriteBytes            uint64
	TotalReadTime         uint64
	TotalWriteTime        uint64
	EBSIOPSExceeded       uint64
	EBSThroughputExceeded uint64
	EC2IOPSExceeded       uint64
	EC2ThroughputExceeded uint64
	QueueLength           uint64
	ReservedArea          [416]byte
	ReadLatency           Histogram
	WriteLatency          Histogram
}

type Histogram struct {
	BinCount uint64
	Bins     [64]HistogramBin
}

type HistogramBin struct {
	Lower uint64
	Upper uint64
	Count uint64
}
