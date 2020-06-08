package translator

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

func GetTargetPlatform() string {
	return context.CurrentContext().Os()
}

func SetTargetPlatform(targetPlatform string) {
	context.CurrentContext().SetOs(targetPlatform)
}
