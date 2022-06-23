// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"fmt"
	"time"

	awscsmmetrics "github.com/aws/private-amazon-cloudwatch-agent-staging/awscsm"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/models"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/plugins/outputs/awscsm/providers"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/agent"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/influxdata/telegraf/plugins/inputs/socket_listener"
	"github.com/influxdata/telegraf/plugins/parsers"
)

type AwsCsmListener struct {
	ServiceAddresses []string
	Log              telegraf.Logger

	listeners []socket_listener.SocketListener
	parser    parsers.Parser
	shutdown  chan bool
}

func (aws *AwsCsmListener) Description() string {
	return "Socket listener wrapper for AWS client-side monitoring data stream"
}

func (aws *AwsCsmListener) SampleConfig() string {
	return `
  ## URLs to listen on
  # service_addresses = [ "udp4://:8094", ... ]
`
}

func (aws *AwsCsmListener) Gather(_ telegraf.Accumulator) error {
	return nil
}

func (aws *AwsCsmListener) SetParser(parser parsers.Parser) {
	aws.parser = parser
}

func (aws *AwsCsmListener) Start(_ telegraf.Accumulator) error {

	// ignore passed-in global accumulator in favor of private csm data stream
	acc := agent.NewAccumulator(&models.AwsCsmMakeMetric{}, models.AwsCsmInputChannel)

	for _, addr := range aws.ServiceAddresses {
		l := socket_listener.SocketListener{
			Parser:         aws.parser,
			ServiceAddress: addr,
			Log:            aws.Log,
		}

		err := l.Start(acc)
		if err != nil {
			return fmt.Errorf("AwsCsmListener start failure: %s", err.Error())
		}

		aws.listeners = append(aws.listeners, l)
	}

	go aws.aggregate()

	return nil
}

func (aws *AwsCsmListener) Stop() {
	for _, l := range aws.listeners {
		l.Stop()
	}

	aws.shutdown <- true
}

func (aws *AwsCsmListener) aggregate() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	records := &AggregationRecords{}
	inputChannel := models.AwsCsmInputChannel

	for {
		select {
		case m := <-inputChannel:
			if cfg := providers.Config.RetrieveAgentConfig(); !cfg.CanCollect() {
				continue
			}

			records.Add(m.Fields())

		case <-ticker.C:
			ms := records.MapStringInterface(filterPrior{
				cutoff: time.Now(),
			}.Filter)

			aws.pushRecords(ms)

		case <-aws.shutdown:
			ms := records.MapStringInterface()
			aws.pushRecords(ms)
			return
		}
	}
}

func (aws *AwsCsmListener) pushRecords(records map[string]interface{}) {
	for _, v := range records {
		fMetric, ok := v.(awscsmmetrics.Metric)
		if !ok {
			continue
		}

		models.AwsCsmOutputChannel <- fMetric
	}
}

func newAwsCsmListener() *AwsCsmListener {
	return &AwsCsmListener{}
}

func init() {
	inputs.Add("awscsm_listener", func() telegraf.Input { return newAwsCsmListener() })
}
