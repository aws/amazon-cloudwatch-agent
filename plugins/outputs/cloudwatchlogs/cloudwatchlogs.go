// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/outputs"
	"go.uber.org/zap"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/useragent"
	"github.com/aws/amazon-cloudwatch-agent/handlers"
	"github.com/aws/amazon-cloudwatch-agent/internal"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/cloudwatchlogs/internal/pusher"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

const (
	LogGroupNameTag   = "log_group_name"
	LogStreamNameTag  = "log_stream_name"
	LogGroupClassTag  = "log_group_class"
	LogTimestampField = "log_timestamp"
	LogEntryField     = "value"

	defaultFlushTimeout = 5 * time.Second

	maxRetryTimeout = 14*24*time.Hour + 10*time.Minute
)

var (
	containerInsightsRegexp = regexp.MustCompile("^/aws/.*containerinsights/.*/(performance|prometheus)$")
)

type CloudWatchLogs struct {
	Region           string `toml:"region"`
	RegionType       string `toml:"region_type"`
	Mode             string `toml:"mode"`
	EndpointOverride string `toml:"endpoint_override"`
	AccessKey        string `toml:"access_key"`
	SecretKey        string `toml:"secret_key"`
	RoleARN          string `toml:"role_arn"`
	Profile          string `toml:"profile"`
	Filename         string `toml:"shared_credential_file"`
	Token            string `toml:"token"`

	//log group and stream names
	LogStreamName string `toml:"log_stream_name"`
	LogGroupName  string `toml:"log_group_name"`

	// Retention for log group
	RetentionInDays int `toml:"retention_in_days"`
	Concurrency     int `toml:"concurrency"`

	ForceFlushInterval internal.Duration `toml:"force_flush_interval"` // unit is second

	Log telegraf.Logger `toml:"-"`

	pusherWaitGroup    sync.WaitGroup
	cwDests            sync.Map
	workerPool         pusher.WorkerPool
	retryHeap          pusher.RetryHeap
	retryHeapProcessor *pusher.RetryHeapProcessor
	targetManager      pusher.TargetManager
	once               sync.Once
	middleware         awsmiddleware.Middleware
	configurer         *awsmiddleware.Configurer
	configurerOnce     sync.Once
}

var _ logs.LogBackend = (*CloudWatchLogs)(nil)
var _ telegraf.Output = (*CloudWatchLogs)(nil)

func (c *CloudWatchLogs) Connect() error {
	return nil
}

func (c *CloudWatchLogs) Close() error {

	c.cwDests.Range(func(_, value interface{}) bool {
		if d, ok := value.(*cwDest); ok {
			d.Stop()
		}
		return true
	})

	c.pusherWaitGroup.Wait()

	if c.workerPool != nil {
		c.workerPool.Stop()
	}

	if c.retryHeapProcessor != nil {
		c.retryHeapProcessor.Stop()
	}

	if c.retryHeap != nil {
		c.retryHeap.Stop()
	}

	return nil
}

func (c *CloudWatchLogs) Write(metrics []telegraf.Metric) error {
	// we no longer expect this to be used. We now use the OTel awsemfexporter for sending EMF metrics to CloudWatch Logs
	return fmt.Errorf("unexpected call to Write")
}

func (c *CloudWatchLogs) CreateDest(group, stream string, retention int, logGroupClass string, logSrc logs.LogSrc) logs.LogDest {
	if group == "" {
		group = c.LogGroupName
	}
	if stream == "" {
		stream = c.LogStreamName
	}
	if retention <= 0 {
		retention = -1
	}

	t := pusher.Target{
		Group:     group,
		Stream:    stream,
		Retention: retention,
		Class:     logGroupClass,
	}
	return c.getDest(t, logSrc)
}

func (c *CloudWatchLogs) getDest(t pusher.Target, logSrc logs.LogSrc) *cwDest {
	if cwd, ok := c.cwDests.Load(t); ok {
		d := cwd.(*cwDest)
		d.Lock()
		defer d.Unlock()
		if !d.stopped {
			d.refCount++
			return d
		}
	}

	logThrottleRetryer := retryer.NewLogThrottleRetryer(c.Log)
	client := c.createClient(logThrottleRetryer)
	agent.UsageFlags().SetValue(agent.FlagRegionType, c.RegionType)
	agent.UsageFlags().SetValue(agent.FlagMode, c.Mode)
	if containerInsightsRegexp.MatchString(t.Group) {
		useragent.Get().SetContainerInsightsFlag()
	}
	c.once.Do(func() {
		if c.Concurrency > 1 {
			c.workerPool = pusher.NewWorkerPool(c.Concurrency)
			c.retryHeap = pusher.NewRetryHeap(c.Concurrency)

			retryHeapProcessorRetryer := retryer.NewLogThrottleRetryer(c.Log)
			retryHeapProcessorClient := c.createClient(retryHeapProcessorRetryer)
			c.retryHeapProcessor = pusher.NewRetryHeapProcessor(c.retryHeap, c.workerPool, retryHeapProcessorClient, c.targetManager, c.Log)
			c.retryHeapProcessor.Start()
		}
		c.targetManager = pusher.NewTargetManager(c.Log, client)
	})
	p := pusher.NewPusher(c.Log, t, client, c.targetManager, logSrc, c.workerPool, c.ForceFlushInterval.Duration, maxRetryTimeout, &c.pusherWaitGroup, c.Concurrency, c.retryHeap)
	cwd := &cwDest{
		pusher:   p,
		retryer:  logThrottleRetryer,
		refCount: 1,
		onStopFunc: func() {
			c.cwDests.Delete(t)
		},
	}
	c.cwDests.Store(t, cwd)
	return cwd
}

func (c *CloudWatchLogs) createClient(retryer aws.RequestRetryer) *cloudwatchlogs.CloudWatchLogs {
	credentialConfig := &configaws.CredentialConfig{
		Region:    c.Region,
		AccessKey: c.AccessKey,
		SecretKey: c.SecretKey,
		RoleARN:   c.RoleARN,
		Profile:   c.Profile,
		Filename:  c.Filename,
		Token:     c.Token,
	}
	client := cloudwatchlogs.New(
		credentialConfig.Credentials(),
		&aws.Config{
			Endpoint: aws.String(c.EndpointOverride),
			Retryer:  retryer,
			LogLevel: configaws.SDKLogLevel(),
			Logger:   configaws.SDKLogger{},
		},
	)
	client.Handlers.Build.PushBackNamed(handlers.NewRequestCompressionHandler([]string{"PutLogEvents"}))
	if c.middleware != nil {
		c.configurerOnce.Do(func() {
			c.configurer = awsmiddleware.NewConfigurer(c.middleware.Handlers())
		})
		if err := c.configurer.Configure(awsmiddleware.SDKv1(&client.Handlers)); err != nil {
			c.Log.Errorf("Unable to configure middleware on cloudwatch logs client: %v", err)
		} else {
			c.Log.Debug("Configured middleware on AWS client")
		}
	}
	return client
}

// Description returns a one-sentence description on the Output
func (c *CloudWatchLogs) Description() string {
	return "Configuration for AWS CloudWatchLogs output."
}

var sampleConfig = `
  ## Amazon REGION
  region = "us-east-1"

  ## Amazon Credentials
  ## Credentials are loaded in the following order
  ## 1) Assumed credentials via STS if role_arn is specified
  ## 2) explicit credentials from 'access_key' and 'secret_key'
  ## 3) shared profile from 'profile'
  ## 4) environment variables
  ## 5) shared credentials file
  ## 6) EC2 Instance Profile
  #access_key = ""
  #secret_key = ""
  #token = ""
  #role_arn = ""
  #profile = ""
  #shared_credential_file = ""

  # The log stream name.
  log_stream_name = "<log_stream_name>"
`

// SampleConfig returns the default configuration of the Output
func (c *CloudWatchLogs) SampleConfig() string {
	return sampleConfig
}

// cwDest is responsible for publishing logs from log files to a log group + log stream.
// Logs from more than one log file may be published to the same destination. cwDest closes
// itself when all log file tailers which referenced this cwDest are closed.
// All exported functions should practice thread-safety by acquiring lock the cwDest
// and not calling any other function which requires the lock.
type cwDest struct {
	pusher *pusher.Pusher
	sync.Mutex
	isEMF   bool
	retryer *retryer.LogThrottleRetryer

	// refCount keeps track of how many LogSrc objects are referencing
	// this cwDest object at any given time. Once there are no more
	// references, the cwDest object stops itself, closing all goroutines,
	// and it can no longer be used
	refCount   int
	stopped    bool
	onStopFunc func()
}

var _ logs.LogDest = (*cwDest)(nil)

func (cd *cwDest) Publish(events []logs.LogEvent) error {
	cd.Lock()
	defer cd.Unlock()
	if cd.stopped {
		return logs.ErrOutputStopped
	}
	for _, e := range events {
		if !cd.isEMF {
			msg := e.Message()
			if strings.HasPrefix(msg, "{") && strings.HasSuffix(msg, "}") && strings.Contains(msg, "\"CloudWatchMetrics\"") {
				cd.switchToEMF()
			}
		}
		cd.addEvent(e)
	}
	return nil
}

func (cd *cwDest) NotifySourceStopped() {
	cd.Lock()
	defer cd.Unlock()
	cd.refCount--
	if cd.refCount <= 0 {
		cd.stop()
	}

	if cd.refCount < 0 {
		fmt.Printf("E! Negative refCount on cwDest detected. refCount: %d, logGroup: %s, logStream: %s", cd.refCount, cd.pusher.Group, cd.pusher.Stream)
	}
}

func (cd *cwDest) Stop() {
	cd.Lock()
	defer cd.Unlock()
	cd.stop()
}

func (cd *cwDest) stop() {
	if cd.stopped {
		return
	}
	cd.retryer.Stop()
	cd.pusher.Stop()
	cd.stopped = true
	if cd.onStopFunc != nil {
		cd.onStopFunc()
	}
}

func (cd *cwDest) addEvent(e logs.LogEvent) {
	// Drop events for metric path logs when queue is full
	if cd.isEMF {
		cd.pusher.AddEventNonBlocking(e)
	} else {
		cd.pusher.AddEvent(e)
	}
}

func (cd *cwDest) switchToEMF() {
	if !cd.isEMF {
		cd.isEMF = true
		cwl, ok := cd.pusher.Service.(*cloudwatchlogs.CloudWatchLogs)
		if ok {
			cwl.Handlers.Build.PushBackNamed(handlers.NewCustomHeaderHandler("x-amzn-logs-format", "json/emf"))
		}
	}
}

func init() {
	outputs.Add("cloudwatchlogs", func() telegraf.Output {
		return &CloudWatchLogs{
			ForceFlushInterval: internal.Duration{Duration: defaultFlushTimeout},
			cwDests:            sync.Map{},
			middleware: agenthealth.NewAgentHealth(
				zap.NewNop(),
				&agenthealth.Config{
					IsUsageDataEnabled:  envconfig.IsUsageDataEnabled(),
					Stats:               &agent.StatsConfig{Operations: []string{"PutLogEvents"}},
					IsStatusCodeEnabled: true,
				},
			),
		}
	})
}
