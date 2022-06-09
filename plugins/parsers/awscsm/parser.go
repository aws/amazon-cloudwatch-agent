// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
	pluginjson "github.com/influxdata/telegraf/plugins/parsers/json"
)

const (
	maxPacketSize = 8192
)

// JSONParser is a parser used to parse JSON and store tags associated with the
// metrics it is parsing.
type JSONParser struct {
	MetricName  string
	TagKeys     []string
	DefaultTags map[string]string
}

func (p *JSONParser) parseArray(buf []byte) ([]telegraf.Metric, error) {
	metrics := make([]telegraf.Metric, 0)

	var jsonOut []map[string]interface{}
	err := json.Unmarshal(buf, &jsonOut)
	if err != nil {
		err = fmt.Errorf("unable to parse out as JSON Array, %s", err)
		return nil, err
	}
	for _, item := range jsonOut {
		metrics, err = p.parseObject(metrics, item)
	}
	return metrics, nil
}

func (p *JSONParser) parseObject(metrics []telegraf.Metric, jsonOut map[string]interface{}) ([]telegraf.Metric, error) {

	tags := make(map[string]string)
	for k, v := range p.DefaultTags {
		tags[k] = v
	}

	for _, tag := range p.TagKeys {
		switch v := jsonOut[tag].(type) {
		case string:
			tags[tag] = v
		case bool:
			tags[tag] = strconv.FormatBool(v)
		case float64:
			tags[tag] = strconv.FormatFloat(v, 'f', -1, 64)
		}
		delete(jsonOut, tag)
	}

	f := pluginjson.JSONFlattener{}
	err := f.FullFlattenJSON("", jsonOut, true, true)
	if err != nil {
		return nil, err
	}

	tags["awscsm"] = "enabled"
	metric := metric.New(p.MetricName, tags, f.Fields, time.Now().UTC())

	return append(metrics, metric), nil
}

// Parse will parse a JSON object or array  and return any error associated with
// the JSON.
func (p *JSONParser) Parse(buf []byte) ([]telegraf.Metric, error) {

	if len(buf) > maxPacketSize {
		return nil, fmt.Errorf("Csm datagram exceeded maximum size %d(%d)", len(buf), maxPacketSize)
	}

	if !isarray(buf) {
		metrics := make([]telegraf.Metric, 0)
		var jsonOut map[string]interface{}
		err := json.Unmarshal(buf, &jsonOut)
		if err != nil {
			err = fmt.Errorf("unable to parse out as JSON, %s", err)
			return nil, err
		}
		return p.parseObject(metrics, jsonOut)
	}
	return p.parseArray(buf)
}

// ParseLine will parse an individual line of JSON
func (p *JSONParser) ParseLine(line string) (telegraf.Metric, error) {
	metrics, err := p.Parse([]byte(line + "\n"))

	if err != nil {
		return nil, err
	}

	if len(metrics) < 1 {
		return nil, fmt.Errorf("Can not parse the line: %s, for data format: influx ", line)
	}

	return metrics[0], nil
}

// SetDefaultTags will add awscsm tag to allow filtering of metrics that are not
// of awscsm format.
func (p *JSONParser) SetDefaultTags(tags map[string]string) {
	p.DefaultTags = tags
}

func isarray(buf []byte) bool {
	ia := bytes.IndexByte(buf, '[')
	ib := bytes.IndexByte(buf, '{')
	return ia > -1 && ia < ib
}
