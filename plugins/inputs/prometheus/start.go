// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// The following code is based on Prometheus project https://github.com/prometheus/prometheus/blob/master/cmd/prometheus/main.go
// and we did modification to remove the logic related to flag handling, Rule manager, TSDB, Web handler, and Notifier.

// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheus

import (
	"fmt"
	"github.com/prometheus/common/promslog"
	"log/slog"
	"os"
	"runtime"
	"sync"

	"github.com/go-kit/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	v "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/version"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	_ "github.com/prometheus/prometheus/discovery/install"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	promRuntime "github.com/prometheus/prometheus/util/runtime"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
)

var (
	configSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prometheus_config_last_reload_successful",
		Help: "Whether the last configuration reload attempt was successful.",
	})
	configSuccessTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prometheus_config_last_reload_success_timestamp_seconds",
		Help: "Timestamp of the last successful configuration reload.",
	})
)

var (
	// Save name before re-label since customers can relabel the prometheus metric name
	// https://github.com/aws/amazon-cloudwatch-agent/issues/190
	// and we would not able to get the metric type for the metric
	// and result in dropping the metrics if it is unknown
	// https://github.com/aws/amazon-cloudwatch-agent/blob/main/plugins/inputs/prometheus_scraper/metrics_filter.go#L23
	metricNameRelabelConfigs = []*relabel.Config{
		{
			Action:       relabel.Replace,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "$1",
			TargetLabel:  savedScrapeNameLabel,
			SourceLabels: model.LabelNames{"__name__"},
		},
	}
)

func init() {
	prometheus.MustRegister(v.NewCollector("prometheus"))
}

// Add this if you haven't already
type slogAdapter struct {
	logger *slog.Logger
}

func (a *slogAdapter) Log(keyvals ...interface{}) error {
	// Convert key-value pairs to attributes
	attrs := make([]any, 0, len(keyvals))
	var msg string

	for i := 0; i < len(keyvals); i += 2 {
		k := keyvals[i]
		var v interface{} = "missing value"
		if i+1 < len(keyvals) {
			v = keyvals[i+1]
		}

		if ks, ok := k.(string); ok && ks == "msg" {
			msg = fmt.Sprint(v)
			continue
		}

		attrs = append(attrs, fmt.Sprint(k), v)
	}

	if msg == "" {
		msg = "log event"
	}

	a.logger.Info(msg, attrs...)
	return nil
}

func (a *slogAdapter) With(keyvals ...interface{}) log.Logger {
	return &slogAdapter{logger: a.logger.With(keyvals...)}
}

func newSlogAdapter(logger *slog.Logger) log.Logger {
	return &slogAdapter{logger: logger}
}

func Start(configFilePath string, receiver storage.Appendable, shutDownChan chan interface{}, wg *sync.WaitGroup, mth *metricsTypeHandler) {
	// ... previous code ...

	// Create slog logger

	if os.Getenv("DEBUG") != "" {
		runtime.SetBlockProfileRate(20)
		runtime.SetMutexProfileFraction(20)
	}

	cfg := struct {
		configFile     string
		promslogConfig promslog.Config
	}{
		promslogConfig: promslog.Config{},
	}

	cfg.configFile = configFilePath

	logger := promslog.New(&cfg.promslogConfig)

	klog.SetLogger(klogr.New().WithName("k8s_client_runtime").V(6))

	logger.Info("Starting Prometheus", "version", version.Info())
	logger.Info("Build Context", "context", version.BuildContext())
	logger.Info("Host Details", "uname", promRuntime.Uname())
	logger.Info("File Descriptor Limits", "limits", promRuntime.FdLimits())
	logger.Info("Virtual Memory Limits", "limits", promRuntime.VMLimits())

	var (
		sdMetrics, _           = discovery.CreateAndRegisterSDMetrics(prometheus.DefaultRegisterer)
		discoveryManagerScrape = discovery.NewManager(
			nil,
			nil,
			prometheus.DefaultRegisterer,
			sdMetrics,
			discovery.Name("scrape"),
		)

		scrapeManager, _ = scrape.NewManager(
			&scrape.Options{},
			nil,
			nil,
			receiver,
			prometheus.DefaultRegisterer,
		)

		taManager = createTargetAllocatorManager(
			configFilePath,
			nil,
			nil,
			scrapeManager,
			discoveryManagerScrape,
		)
	)

	logger.Info("Target Allocator status", "enabled", taManager.enabled)

	// ... rest of the code, updating log calls to use appropriate logger ...

	// ... continue updating other logging calls ...
}

const (
	savedScrapeJobLabel      = "cwagent_saved_scrape_job"
	savedScrapeInstanceLabel = "cwagent_saved_scrape_instance"
	scrapeInstanceLabel      = "__address__"
	savedScrapeNameLabel     = "cwagent_saved_scrape_name" // just arbitrary name that end user won't override in relabel config
)

func relabelScrapeConfigs(prometheusConfig *config.Config, logger log.Logger) {
	// For saving name before relabel
	// - __name__ https://github.com/aws/amazon-cloudwatch-agent/issues/190
	// - job and instance https://github.com/aws/amazon-cloudwatch-agent/issues/193
	for _, sc := range prometheusConfig.ScrapeConfigs {
		relabelConfigs := []*relabel.Config{
			// job
			{
				Action:       relabel.Replace,
				Regex:        relabel.MustNewRegexp(".*"), // __address__ is always there, so we will find a match for every job
				Replacement:  sc.JobName,                  // value is hard coded job name
				SourceLabels: model.LabelNames{"__address__"},
				TargetLabel:  savedScrapeJobLabel, // creates a new magic label
			},
			// instance
			{
				Action:       relabel.Replace,
				Regex:        relabel.MustNewRegexp("(.*)"),
				Replacement:  "$1", // value is actual __address__, i.e. instance if you don't relabel it.
				SourceLabels: model.LabelNames{"__address__"},
				TargetLabel:  savedScrapeInstanceLabel, // creates a new magic label
			},
		}

		sc.RelabelConfigs = append(relabelConfigs, sc.RelabelConfigs...)
		sc.MetricRelabelConfigs = append(metricNameRelabelConfigs, sc.MetricRelabelConfigs...)
	}
}
func reloadConfig(filename string, logger log.Logger, taManager *TargetAllocatorManager, rls ...func(*config.Config) error) (err error) {
	defer func() {
		if err == nil {
			configSuccess.Set(1)
			configSuccessTime.SetToCurrentTime()
		} else {
			configSuccess.Set(0)
		}
	}()
	// Check for TA
	var conf *config.Config
	if taManager.enabled {
		conf = (*config.Config)(taManager.config.PrometheusConfig)
	} else {
		conf, err = config.LoadFile(filename, false, nil)
		if err != nil {
			return errors.Wrapf(err, "couldn't load configuration (--config.file=%q)", filename)
		}
	}
	relabelScrapeConfigs(conf, logger)
	failed := false
	for _, rl := range rls {
		if err := rl(conf); err != nil {
			failed = true
		}
	}
	if failed {
		return errors.Errorf("one or more errors occurred while applying the new configuration (--config.file=%q)", filename)
	}

	return nil
}
