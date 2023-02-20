// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sdecorator

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

const (
	SectionKeyTargetService = "tag_service"
)

type TargetService struct {
}

func (t *TargetService) ApplyRule(input interface{}) (string, interface{}) {
	return translator.DefaultCase(SectionKeyTargetService, true, input)
}

func init() {
	RegisterRule(SectionKeyTargetService, new(TargetService))
}
