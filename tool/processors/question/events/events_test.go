package events

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/serialization"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"

	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"

	"github.com/aws/amazon-cloudwatch-agent/tool/util"

	"github.com/stretchr/testify/assert"
)

func TestProcessor_Process(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	ctx := new(runtime.Context)
	ctx.OsParameter = util.OsTypeWindows
	conf := new(data.Config)

	testutil.Type(inputChan, "", "", "", "", "", "", "", "", "", "", "2")
	Processor.Process(ctx, conf)
	_, confMap := conf.ToMap(ctx)
	assert.Equal(t, map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"windows_events": map[string]interface{}{
					"collect_list": []map[string]interface{}{
						{"event_name": "System", "event_format": "xml", "event_levels": []string{VERBOSE, INFORMATION, WARNING, ERROR, CRITICAL}, "log_group_name": "System", "log_stream_name": "{instance_id}"}}}}}},
		confMap)
}

func TestProcessor_NextProcessor(t *testing.T) {
	nextProcessor := Processor.NextProcessor(nil, nil)
	assert.Equal(t, serialization.Processor, nextProcessor)
}
