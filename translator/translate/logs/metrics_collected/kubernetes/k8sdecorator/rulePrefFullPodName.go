// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sdecorator

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

const (
	SectionKeyPrefFullPodName = "prefer_full_pod_name"
)

type PrefFullPodName struct {
}

func (t *PrefFullPodName) ApplyRule(input interface{}) (string, interface{}) {
	return translator.DefaultCase(SectionKeyPrefFullPodName, false, input)
}

func init() {
	RegisterRule(SectionKeyPrefFullPodName, new(PrefFullPodName))
}
