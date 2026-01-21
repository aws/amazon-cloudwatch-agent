// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyrequestcompression "github.com/aws/smithy-go/private/requestcompression"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/outputs"
	"go.uber.org/zap"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws/v2"
	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/useragent"
	"github.com/aws/amazon-cloudwatch-agent/internal"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer/v2"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/middleware"
	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/cloudwatchlogs/internal/pusher"
)

const (
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
	RetentionInDays int32 `toml:"retention_in_days"`
	Concurrency     int   `toml:"concurrency"`

	ForceFlushInterval internal.Duration `toml:"force_flush_interval"` // unit is second

	Log telegraf.Logger `toml:"-"`

	pusherWaitGroup sync.WaitGroup
	cwDests         sync.Map
	workerPool      pusher.WorkerPool
	targetManager   pusher.TargetManager
	once            sync.Once
	middleware      awsmiddleware.Middleware
	configurer      *awsmiddleware.Configurer
	configurerOnce  sync.Once
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

	return nil
}

func (c *CloudWatchLogs) Write([]telegraf.Metric) error {
	// we no longer expect this to be used. We now use the OTel awsemfexporter for sending EMF metrics to CloudWatch Logs
	return fmt.Errorf("unexpected call to Write")
}

func (c *CloudWatchLogs) CreateDest(group, stream string, retention int32, logGroupClass string, logSrc logs.LogSrc) logs.LogDest {
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
	cwd := &cwDest{
		retryer:  logThrottleRetryer,
		refCount: 1,
		onStopFunc: func() {
			c.cwDests.Delete(t)
		},
	}
	client, err := c.createClient(context.Background(), logThrottleRetryer, cwd)
	if err != nil {
		c.Log.Errorf("Failed to create CloudWatch Logs client: %v", err)
		return nil
	}
	agent.UsageFlags().SetValue(agent.FlagRegionType, c.RegionType)
	agent.UsageFlags().SetValue(agent.FlagMode, c.Mode)
	if containerInsightsRegexp.MatchString(t.Group) {
		useragent.Get().SetContainerInsightsFlag()
	}
	c.once.Do(func() {
		if c.Concurrency > 1 {
			c.workerPool = pusher.NewWorkerPool(c.Concurrency)
		}
		c.targetManager = pusher.NewTargetManager(c.Log, client)
	})
	cwd.pusher = pusher.NewPusher(c.Log, t, client, c.targetManager, logSrc, c.workerPool, c.ForceFlushInterval.Duration, maxRetryTimeout, &c.pusherWaitGroup)
	c.cwDests.Store(t, cwd)
	return cwd
}

func (c *CloudWatchLogs) createClient(ctx context.Context, retryer aws.Retryer, cwd *cwDest) (*cloudwatchlogs.Client, error) {
	credentialConfig := &configaws.CredentialsConfig{
		Region:    c.Region,
		AccessKey: c.AccessKey,
		SecretKey: c.SecretKey,
		RoleARN:   c.RoleARN,
		Profile:   c.Profile,
		Filename:  c.Filename,
		Token:     c.Token,
	}

	awsConfig, err := credentialConfig.LoadConfig(ctx)
	if err != nil {
		return nil, err
	}

	if c.middleware != nil {
		c.configurerOnce.Do(func() {
			c.configurer = awsmiddleware.NewConfigurer(c.middleware.Handlers())
		})
		if err = c.configurer.Configure(awsmiddleware.SDKv2(&awsConfig)); err != nil {
			c.Log.Errorf("Unable to configure middleware on cloudwatch logs client: %v", err)
		} else {
			c.Log.Debug("Configured middleware on AWS client")
		}
	}

	if cwd != nil {
		awsConfig.APIOptions = append(awsConfig.APIOptions, func(stack *smithymiddleware.Stack) error {
			return stack.Build.Add(&middleware.CustomHeaderMiddleware{
				MiddlewareID: "EmfHeader",
				Fn:           cwd.emfHeader,
			}, smithymiddleware.After)
		})
	}

	// follows PutMetricData compression setup (https://github.com/aws/aws-sdk-go-v2/blob/main/service/cloudwatch/api_op_PutMetricData.go#L269-L274)
	awsConfig.APIOptions = append(awsConfig.APIOptions, func(stack *smithymiddleware.Stack) error {
		return smithyrequestcompression.AddRequestCompression(stack, awsConfig.DisableRequestCompression, awsConfig.RequestMinCompressSizeBytes, []string{"gzip"})
	})

	client := cloudwatchlogs.NewFromConfig(awsConfig, func(o *cloudwatchlogs.Options) {
		if c.EndpointOverride != "" {
			o.BaseEndpoint = aws.String(c.EndpointOverride)
		}
		if retryer != nil {
			o.Retryer = retryer
		}
	})

	return client, nil
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
	isEMF   atomic.Bool
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
		if !cd.isEMF.Load() {
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
	if cd.isEMF.Load() {
		cd.pusher.AddEventNonBlocking(e)
	} else {
		cd.pusher.AddEvent(e)
	}
}

func (cd *cwDest) switchToEMF() {
	if !cd.isEMF.Load() {
		cd.isEMF.Store(true)
	}
}

func (cd *cwDest) emfHeader() map[string]string {
	if cd.isEMF.Load() {
		return map[string]string{
			"x-amzn-logs-format": "json/emf",
		}
	}
	return nil
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
