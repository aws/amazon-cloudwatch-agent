package k8sdecorator

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
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
