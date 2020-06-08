package processors

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

var StartProcessor Processor

type Processor interface {
	Process(ctx *runtime.Context, config *data.Config)
	NextProcessor(ctx *runtime.Context, config *data.Config) interface{}
}
