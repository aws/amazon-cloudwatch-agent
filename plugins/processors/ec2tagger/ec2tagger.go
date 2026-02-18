// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"context"
	"hash/fnv"
	"os"
	"sync"
	"time"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
)

type ec2MetadataLookupType struct {
	instanceId   bool
	imageId      bool
	instanceType bool
}

type ec2MetadataRespondType struct {
	instanceId   string
	imageId      string // aka AMI
	instanceType string
	region       string
}

// EC2APIClient defines the interface for EC2 API operations needed by the tagger
type EC2APIClient interface {
	ec2.DescribeTagsAPIClient
}

type ec2ProviderType func(ctx context.Context, host component.Host, credentialConfig *configaws.CredentialsConfig) EC2APIClient

type Tagger struct {
	*Config

	logger           *zap.Logger
	cancelFunc       context.CancelFunc
	metadataProvider ec2metadataprovider.MetadataProvider
	ec2Provider      ec2ProviderType

	shutdownC          chan bool
	ec2TagCache        map[string]string
	started            bool
	ec2MetadataLookup  ec2MetadataLookupType
	ec2MetadataRespond ec2MetadataRespondType
	tagFilters         []types.Filter
	ec2API             EC2APIClient

	Configurer   *awsmiddleware.Configurer
	sync.RWMutex //to protect ec2TagCache
}

// newTagger returns a new EC2 Tagger processor.
func newTagger(config *Config, logger *zap.Logger) *Tagger {
	_, cancel := context.WithCancel(context.Background())
	mdCredentialConfig := &configaws.CredentialsConfig{}

	mdCfg, err := mdCredentialConfig.LoadConfig(context.Background())
	if err != nil {
		logger.Error("ec2tagger: Failed to load AWS config for metadata provider", zap.Error(err))
	}

	p := &Tagger{
		Config:           config,
		logger:           logger,
		cancelFunc:       cancel,
		metadataProvider: ec2metadataprovider.NewMetadataProvider(mdCfg, config.IMDSRetries),
	}
	p.ec2Provider = p.createEC2Client
	return p
}

func (t *Tagger) createEC2Client(ctx context.Context, host component.Host, credentialConfig *configaws.CredentialsConfig) EC2APIClient {
	cfg, err := credentialConfig.LoadConfig(ctx)
	if err != nil {
		cfg = aws.Config{}
	}

	if t.MiddlewareID != nil {
		awsmiddleware.TryConfigure(t.logger, host, *t.MiddlewareID, awsmiddleware.SDKv2(&cfg))
	}

	return ec2.NewFromConfig(cfg)
}

func (t *Tagger) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	// grab the pointer to the map in case it gets refreshed while we're applying this round of metrics. At least
	// this batch then will all get the same tags.
	t.RLock()
	defer t.RUnlock()

	if !t.started {
		return pmetric.NewMetrics(), nil
	}

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			metrics := sms.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				attributes := getOtelAttributes(metrics.At(k))
				t.updateOtelAttributes(attributes)
			}
		}
	}
	return md, nil
}

func getOtelAttributes(m pmetric.Metric) []pcommon.Map {
	var attributes []pcommon.Map
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			attributes = append(attributes, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			attributes = append(attributes, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeHistogram:
		dps := m.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			attributes = append(attributes, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := m.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			attributes = append(attributes, dps.At(i).Attributes())
		}
	default:
	}
	return attributes
}

// updateOtelAttributes adds tags and the requested dimensions to the attributes of each
// DataPoint. We add and remove at the DataPoint level instead of resource level because this is
// where the receiver/adapter does.
func (t *Tagger) updateOtelAttributes(attributes []pcommon.Map) {
	for _, attr := range attributes {
		if t.ec2TagCache != nil {
			for k, v := range t.ec2TagCache {
				if _, exists := attr.Get(k); !exists {
					attr.PutStr(k, v)
				}
			}
		}
		if t.ec2MetadataLookup.instanceId {
			if _, exists := attr.Get(MdKeyInstanceID); !exists {
				attr.PutStr(MdKeyInstanceID, t.ec2MetadataRespond.instanceId)
			}
		}
		if t.ec2MetadataLookup.imageId {
			if _, exists := attr.Get(MdKeyImageID); !exists {
				attr.PutStr(MdKeyImageID, t.ec2MetadataRespond.imageId)
			}
		}
		if t.ec2MetadataLookup.instanceType {
			if _, exists := attr.Get(MdKeyInstanceType); !exists {
				attr.PutStr(MdKeyInstanceType, t.ec2MetadataRespond.instanceType)
			}
		}
		attr.Remove("host")
	}
}

// updateTags calls EC2 Describe Tags and replaces the Tagger's tagCache with the newly retrieved values
func (t *Tagger) updateTags(ctx context.Context) error {
	tags := make(map[string]string)

	paginator := ec2.NewDescribeTagsPaginator(t.ec2API, &ec2.DescribeTagsInput{
		Filters: t.tagFilters,
	})

	for paginator.HasMorePages() {
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		for _, tag := range result.Tags {
			key := *tag.Key
			if Ec2InstanceTagKeyASG == key {
				// rename to match CW dimension as applied by AutoScaling service, not the EC2 tag
				key = CWDimensionASG
			}
			tags[key] = *tag.Value
		}
	}

	t.Lock()
	defer t.Unlock()
	t.ec2TagCache = tags
	return nil
}

func (t *Tagger) Shutdown(context.Context) error {
	close(t.shutdownC)
	t.cancelFunc()
	return nil
}

// refreshLoopTags handles the refresh ticks for describe tags and also responds to shutdown signal
func (t *Tagger) refreshLoopTags(refreshInterval time.Duration, stopAfterFirstSuccess bool) {
	refreshTicker := time.NewTicker(refreshInterval)
	defer refreshTicker.Stop()
	for {
		select {
		case <-refreshTicker.C:
			t.logger.Debug("ec2tagger refreshing tags")
			allTagsRetrieved := t.ec2TagsRetrieved()
			t.logger.Debug("Retrieve status",
				zap.Bool("Ec2AllTagsRetrieved", allTagsRetrieved))
			refreshTags := len(t.EC2InstanceTagKeys) > 0

			if stopAfterFirstSuccess {
				// need refresh tags when it is configured and not all ec2 tags are retrieved
				refreshTags = refreshTags && !allTagsRetrieved
				if !refreshTags {
					t.logger.Info("ec2tagger: Refresh for tags is no longer needed, stop refreshTicker.")
					return
				}
			}

			if refreshTags {
				if err := t.updateTags(context.Background()); err != nil {
					t.logger.Warn("ec2tagger: Error refreshing EC2 tags, keeping old values", zap.Error(err))
				}
			}

		case <-t.shutdownC:
			return
		}
	}
}

func (t *Tagger) ec2TagsRetrieved() bool {
	allTagsRetrieved := true
	t.RLock()
	defer t.RUnlock()
	if t.ec2TagCache != nil {
		for _, key := range t.EC2InstanceTagKeys {
			if key == Ec2InstanceTagKeyASG {
				key = CWDimensionASG
			}
			if key == "*" {
				continue
			}
			if _, ok := t.ec2TagCache[key]; !ok {
				allTagsRetrieved = false
				break
			}
		}
	}
	return allTagsRetrieved
}

// Start acts as input validation and serves the purpose of updating ec2 tags.
// It will be called when OTel is enabling each processor
func (t *Tagger) Start(ctx context.Context, host component.Host) error {
	t.shutdownC = make(chan bool)
	t.ec2TagCache = map[string]string{}
	if err := t.deriveEC2MetadataFromIMDS(ctx); err != nil {
		return err
	}
	t.tagFilters = []types.Filter{
		{
			Name:   aws.String("resource-type"),
			Values: []string{"instance"},
		},
		{
			Name:   aws.String("resource-id"),
			Values: []string{t.ec2MetadataRespond.instanceId},
		},
	}
	// if the customer said 'AutoScalingGroupName' (the CW dimension), do what they mean not what they said
	// and filter for the EC2 tag name called 'aws:autoscaling:groupName'
	useAllTags := len(t.EC2InstanceTagKeys) == 1 && t.EC2InstanceTagKeys[0] == "*"
	if !useAllTags && len(t.EC2InstanceTagKeys) > 0 {
		// if the customer said 'AutoScalingGroupName' (the CW dimension), do what they mean not what they said
		for i, key := range t.EC2InstanceTagKeys {
			if CWDimensionASG == key {
				t.EC2InstanceTagKeys[i] = Ec2InstanceTagKeyASG
			}
		}

		t.tagFilters = append(t.tagFilters, types.Filter{
			Name:   aws.String("key"),
			Values: t.EC2InstanceTagKeys,
		})
	}
	if len(t.EC2InstanceTagKeys) > 0 {
		ec2CredentialConfig := &configaws.CredentialsConfig{
			AccessKey: t.AccessKey,
			SecretKey: t.SecretKey,
			RoleARN:   t.RoleARN,
			Profile:   t.Profile,
			Filename:  t.Filename,
			Token:     t.Token,
			Region:    t.ec2MetadataRespond.region,
		}

		t.ec2API = t.ec2Provider(ctx, host, ec2CredentialConfig)

		go func() { //Async start of initial retrieval to prevent block of agent start
			t.initialRetrievalOfTags()
			t.refreshLoopToUpdateTags()
		}()
		t.logger.Info("ec2tagger: EC2 tagger has started initialization.")

	} else {
		t.setStarted()
	}
	return nil
}

func (t *Tagger) refreshLoopToUpdateTags() {
	needRefresh := false
	stopAfterFirstSuccess := false

	refreshInterval := t.RefreshTagsInterval
	if refreshInterval.Seconds() == 0 {
		//when the refresh interval is 0, this means that customer don't want to
		//update tags values once they are retrieved successfully. In this case,
		//we still want to do refresh to make sure all the specified keys for tags
		//are fetched successfully because initial retrieval might not get all of them.
		//When the specified key is "*", there is no way for us to check if all
		//tags are fetched. So there is no need to do refresh in this case.
		needRefresh = len(t.EC2InstanceTagKeys) != 1 || t.EC2InstanceTagKeys[0] != "*"

		stopAfterFirstSuccess = true
		refreshInterval = defaultRefreshInterval
	} else if refreshInterval.Seconds() > 0 {
		//customer wants to update the tags with the given refresh interval
		needRefresh = true
	}

	if needRefresh {
		go func() {
			// randomly stagger the time of the first refresh to mitigate throttling if a whole fleet is
			// restarted at the same time
			sleepUntilHostJitter(refreshInterval)
			t.refreshLoopTags(refreshInterval, stopAfterFirstSuccess)
		}()
	}
}

func (t *Tagger) setStarted() {
	t.Lock()
	t.started = true
	t.Unlock()
	t.logger.Info("ec2tagger: EC2 tagger has started, finished initial retrieval of tags")
}

/*
Retrieve metadata from IMDS and use these metadata to:
* Extract InstanceID, ImageID, InstanceType to create custom dimension for collected metrics
* Extract InstanceID to retrieve Instance's Tags
* Extract Region to create aws session with custom configuration
For more information on IMDS, please follow this document https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html
*/
func (t *Tagger) deriveEC2MetadataFromIMDS(ctx context.Context) error {
	for _, tag := range t.EC2MetadataTags {
		switch tag {
		case MdKeyInstanceID:
			t.ec2MetadataLookup.instanceId = true
		case MdKeyImageID:
			t.ec2MetadataLookup.imageId = true
		case MdKeyInstanceType:
			t.ec2MetadataLookup.instanceType = true
		default:
			t.logger.Error("ec2tagger: Unsupported EC2 Metadata key", zap.String("mdKey", tag))
		}
	}

	t.logger.Info("ec2tagger: Check EC2 Metadata.")
	doc, err := t.metadataProvider.Get(ctx)
	if err != nil {
		t.logger.Error("ec2tagger: Unable to retrieve EC2 Metadata. This plugin must only be used on an EC2 instance.")
		if translatorcontext.CurrentContext().RunInContainer() {
			t.logger.Warn("ec2tagger: Timeout may have occurred because hop limit is too small. Please increase hop limit to 2 by following this document https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-options.html#configuring-IMDS-existing-instances.")
		}
		return err
	}

	t.ec2MetadataRespond.region = doc.Region
	t.ec2MetadataRespond.instanceId = doc.InstanceID
	if t.ec2MetadataLookup.imageId {
		t.ec2MetadataRespond.imageId = doc.ImageID
	}
	if t.ec2MetadataLookup.instanceType {
		t.ec2MetadataRespond.instanceType = doc.InstanceType
	}

	return nil
}

// This function never return until calling updateTags() succeeds or shutdown happen.
func (t *Tagger) initialRetrievalOfTags() {
	tagsRetrieved := len(t.EC2InstanceTagKeys) == 0

	retry := 0
	for {
		var waitDuration time.Duration
		if retry < len(BackoffSleepArray) {
			waitDuration = BackoffSleepArray[retry]
		} else {
			waitDuration = BackoffSleepArray[len(BackoffSleepArray)-1]
		}

		wait := time.NewTimer(waitDuration)
		select {
		case <-t.shutdownC:
			wait.Stop()
			return
		case <-wait.C:
		}

		if retry > 0 {
			t.logger.Info("ec2tagger: initial retrieval of tags", zap.Int("retry", retry))
		}

		if !tagsRetrieved {
			if err := t.updateTags(context.Background()); err != nil {
				t.logger.Warn("ec2tagger: Unable to describe ec2 tags for initial retrieval", zap.Error(err))
			} else {
				tagsRetrieved = true
			}
		}

		if tagsRetrieved {
			t.logger.Info("ec2tagger: Initial retrieval of tags succeeded")
			t.setStarted()
			return
		}

		retry++
	}

}

func sleepUntilHostJitter(max time.Duration) {
	time.Sleep(hostJitter(max))
}

func hostJitter(max time.Duration) time.Duration {
	hostName, err := os.Hostname()
	if err != nil {
		hostName = "Unknown"
	}
	hash := fnv.New64()
	hash.Write([]byte(hostName))
	// Right shift the uint64 hash by one to make sure the jitter duration is always positive
	hostSleepJitter := time.Duration(int64(hash.Sum64()>>1)) % max
	return hostSleepJitter
}
