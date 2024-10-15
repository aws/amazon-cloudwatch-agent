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
	"context"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/oklog/run"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	v "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/promlog"
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

func Start(configFilePath string, receiver storage.Appendable, shutDownChan chan interface{}, wg *sync.WaitGroup, mth *metricsTypeHandler) {
	logLevel := &promlog.AllowedLevel{}
	logLevel.Set("info")

	if os.Getenv("DEBUG") != "" {
		runtime.SetBlockProfileRate(20)
		runtime.SetMutexProfileFraction(20)
		logLevel.Set("debug")
	}
	logFormat := &promlog.AllowedFormat{}
	_ = logFormat.Set("logfmt")

	cfg := struct {
		configFile    string
		promlogConfig promlog.Config
	}{
		promlogConfig: promlog.Config{Level: logLevel, Format: logFormat},
	}

	cfg.configFile = configFilePath

	logger := promlog.New(&cfg.promlogConfig)
	//stdlog.SetOutput(log.NewStdlibAdapter(logger))
	//stdlog.Println("redirect std log")

	klog.SetLogger(klogr.New().WithName("k8s_client_runtime").V(6))

	level.Info(logger).Log("msg", "Starting Prometheus", "version", version.Info())
	level.Info(logger).Log("build_context", version.BuildContext())
	level.Info(logger).Log("host_details", promRuntime.Uname())
	level.Info(logger).Log("fd_limits", promRuntime.FdLimits())
	level.Info(logger).Log("vm_limits", promRuntime.VMLimits())

	var (
		ctxScrape, cancelScrape = context.WithCancel(context.Background())
		sdMetrics, _            = discovery.CreateAndRegisterSDMetrics(prometheus.DefaultRegisterer)
		discoveryManagerScrape  = discovery.NewManager(ctxScrape, log.With(logger, "component", "discovery manager scrape"), prometheus.DefaultRegisterer, sdMetrics, discovery.Name("scrape"))
		scrapeManager, _        = scrape.NewManager(&scrape.Options{}, log.With(logger, "component", "scrape manager"), receiver, prometheus.DefaultRegisterer)
	)
	mth.SetScrapeManager(scrapeManager)

	var reloaders = []func(cfg *config.Config) error{
		// The Scrape and notifier managers need to reload before the Discovery manager as
		// they need to read the most updated config when receiving the new targets list.
		scrapeManager.ApplyConfig,
		func(cfg *config.Config) error {
			c := make(map[string]discovery.Configs)
			for _, v := range cfg.ScrapeConfigs {
				c[v.JobName] = v.ServiceDiscoveryConfigs
			}
			return discoveryManagerScrape.ApplyConfig(c)
		},
	}

	prometheus.MustRegister(configSuccess)
	prometheus.MustRegister(configSuccessTime)

	// sync.Once is used to make sure we can close the channel at different execution stages(SIGTERM or when the config is loaded).
	type closeOnce struct {
		C     chan struct{}
		once  sync.Once
		Close func()
	}
	// Wait until the server is ready to handle reloading.
	reloadReady := &closeOnce{
		C: make(chan struct{}),
	}
	reloadReady.Close = func() {
		reloadReady.once.Do(func() {
			close(reloadReady.C)
		})
	}

	var g run.Group
	{
		// Termination handler.
		cancel := make(chan struct{})
		g.Add(
			func() error {
				// Don't forget to release the reloadReady channel so that waiting blocks can exit normally.
				select {
				case <-shutDownChan:
					level.Warn(logger).Log("msg", "Received ShutDown, exiting gracefully...")
					reloadReady.Close()

				case <-cancel:
					reloadReady.Close()
					break
				}
				return nil
			},
			func(err error) {
				close(cancel)
			},
		)
	}
	{
		// Scrape discovery manager.
		g.Add(
			func() error {
				err := discoveryManagerScrape.Run()
				level.Info(logger).Log("msg", "Scrape discovery manager stopped")
				return err
			},
			func(err error) {
				level.Info(logger).Log("msg", "Stopping scrape discovery manager...")
				cancelScrape()
			},
		)
	}
	{
		// Scrape manager.
		g.Add(
			func() error {
				// When the scrape manager receives a new targets list
				// it needs to read a valid config for each job.
				// It depends on the config being in sync with the discovery manager so
				// we wait until the config is fully loaded.
				<-reloadReady.C

				level.Info(logger).Log("msg", "start discovery")
				err := scrapeManager.Run(discoveryManagerScrape.SyncCh())
				level.Info(logger).Log("msg", "Scrape manager stopped")
				return err
			},
			func(err error) {
				// Scrape manager needs to be stopped before closing the local TSDB
				// so that it doesn't try to write samples to a closed storage.
				level.Info(logger).Log("msg", "Stopping scrape manager...")
				scrapeManager.Stop()
			},
		)
	}
	{
		// Reload handler.

		// Make sure that sighup handler is registered with a redirect to the channel before the potentially
		// long and synchronous tsdb init.
		hup := make(chan os.Signal, 1)
		signal.Notify(hup, syscall.SIGHUP)
		cancel := make(chan struct{})
		g.Add(
			func() error {
				<-reloadReady.C

				for {
					select {
					case <-hup:
						if err := reloadConfig(cfg.configFile, logger, reloaders...); err != nil {
							level.Error(logger).Log("msg", "Error reloading config", "err", err)
						}

					case <-cancel:
						return nil
					}
				}

			},
			func(err error) {
				// Wait for any in-progress reloads to complete to avoid
				// reloading things after they have been shutdown.
				cancel <- struct{}{}
			},
		)
	}
	{
		// Initial configuration loading.
		cancel := make(chan struct{})
		g.Add(
			func() error {
				select {
				// In case a shutdown is initiated before the dbOpen is released
				case <-cancel:
					reloadReady.Close()
					return nil

				default:
				}

				level.Info(logger).Log("msg", "handling config file")
				if err := reloadConfig(cfg.configFile, logger, reloaders...); err != nil {
					return errors.Wrapf(err, "error loading config from %q", cfg.configFile)
				}
				level.Info(logger).Log("msg", "finish handling config file")

				reloadReady.Close()
				<-cancel
				return nil
			},
			func(err error) {
				close(cancel)
			},
		)
	}

	if err := g.Run(); err != nil {
		level.Error(logger).Log("err", err)
	}
	level.Info(logger).Log("msg", "See you next time!")
	wg.Done()
}

const (
	savedScrapeJobLabel      = "cwagent_saved_scrape_job"
	savedScrapeInstanceLabel = "cwagent_saved_scrape_instance"
	scrapeInstanceLabel      = "__address__"
	savedScrapeNameLabel     = "cwagent_saved_scrape_name" // just arbitrary name that end user won't override in relabel config
)

func reloadConfig(filename string, logger log.Logger, rls ...func(*config.Config) error) (err error) {
	level.Info(logger).Log("msg", "Loading configuration file", "filename", filename)
	content, _ := os.ReadFile(filename)
	text := string(content)
	level.Debug(logger).Log("msg", "Prometheus configuration file", "value", text)

	defer func() {
		if err == nil {
			configSuccess.Set(1)
			configSuccessTime.SetToCurrentTime()
		} else {
			configSuccess.Set(0)
		}
	}()

	conf, err := config.LoadFile(filename, false, false, logger)
	if err != nil {
		return errors.Wrapf(err, "couldn't load configuration (--config.file=%q)", filename)
	}

	// For saving name before relabel
	// - __name__ https://github.com/aws/amazon-cloudwatch-agent/issues/190
	// - job and instance https://github.com/aws/amazon-cloudwatch-agent/issues/193
	for _, sc := range conf.ScrapeConfigs {
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

		level.Info(logger).Log("msg", "Add extra relabel_configs and metric_relabel_configs to save job, instance and __name__ before user relabel")

		sc.RelabelConfigs = append(relabelConfigs, sc.RelabelConfigs...)
		sc.MetricRelabelConfigs = append(metricNameRelabelConfigs, sc.MetricRelabelConfigs...)

	}

	failed := false
	for _, rl := range rls {
		if err := rl(conf); err != nil {
			level.Error(logger).Log("msg", "Failed to apply configuration", "err", err)
			failed = true
		}
	}
	if failed {
		return errors.Errorf("one or more errors occurred while applying the new configuration (--config.file=%q)", filename)
	}

	level.Info(logger).Log("msg", "Completed loading of configuration file", "filename", filename)
	return nil
}
