package structuredlogscommon

import (
	"testing"
	"time"

	"github.com/influxdata/telegraf/metric"
	"github.com/stretchr/testify/assert"
)

func TestAppendAttributesInFields(t *testing.T) {
	m, _ := metric.New("test", map[string]string{}, map[string]interface{}{}, time.Now())
	AppendAttributesInFields("testFieldName", "testFieldValue", m)
	assert.Equal(t, "testFieldName", m.Tags()[attributesInFields])
	assert.Equal(t, "testFieldValue", m.Fields()["testFieldName"].(string))

	AppendAttributesInFields("testFieldName2", "testFieldValue2", m)
	assert.Equal(t, "testFieldName,testFieldName2", m.Tags()[attributesInFields])
	assert.Equal(t, "testFieldValue2", m.Fields()["testFieldName2"].(string))
}

func TestBuildAttributes(t *testing.T) {
	m, _ := metric.New("test", map[string]string{}, map[string]interface{}{}, time.Now())
	AppendAttributesInFields("testFieldName", "testFieldValue", m)
	structuredlogs := map[string]interface{}{}
	BuildAttributes(m, structuredlogs)
	assert.Equal(t, map[string]interface{}{"testFieldName": "testFieldValue"}, structuredlogs)
}

func TestBuildValidMeasurements(t *testing.T) {
	m, _ := metric.New("test", map[string]string{}, map[string]interface{}{"testFieldString": "value", "testFieldInt": 0, "testFieldFloat": 0.0, "testFieldBool": true}, time.Now())
	structuredlogs := map[string]interface{}{}
	err := BuildMeasurements(m, structuredlogs)
	assert.Equal(t, nil, err)
	assert.Equal(t, map[string]interface{}{"testFieldString": "value", "testFieldInt": 0.0, "testFieldFloat": 0.0, "testFieldBool": true}, structuredlogs)
}

func TestBuildInvalidMeasurements(t *testing.T) {
	m, _ := metric.New("test", map[string]string{}, map[string]interface{}{"testFieldMap": map[string]string{}}, time.Now())
	structuredlogs := map[string]interface{}{}
	err := BuildMeasurements(m, structuredlogs)
	assert.True(t, nil != err)
	assert.Equal(t, map[string]interface{}{}, structuredlogs)
}
