// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package constants

const (
	FileSuffixTmp  = ".tmp"
	FileSuffixYAML = ".yaml"

	ExitCodeNoJSONFile = 99

	// AdapterReceiverAttribute is used to mark histogram datapoints from the adapter receiver
	//
	// The adapter receiver slightly misuses the histogram datapoint type so histogram metrics from the adapter receiver
	// need to be handled slightly differently.
	AdapterReceiverAttribute = "__cwa_adapterreceiver"
)
