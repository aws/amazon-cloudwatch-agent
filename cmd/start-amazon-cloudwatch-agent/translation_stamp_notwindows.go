// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package main

// The translation stamp is only used on Windows, where the service start must
// connect to the Service Control Manager within ServicesPipeTimeout. On other
// platforms the config is translated on every start, as before.

type translationInputs struct{}

func translationIsCurrent() bool {
	return false
}

func captureTranslationInputs() translationInputs { return translationInputs{} }

func writeTranslationStamp(translationInputs) {}

func removeTranslationStamp() {}
