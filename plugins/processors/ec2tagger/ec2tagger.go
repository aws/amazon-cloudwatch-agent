// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"context"
	"hash/fnv"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger/internal/volume"
	translatorCtx "github.com/aws/amazon-cloudwatch-agent/translator/context"
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

type ec2ProviderType func(*configaws.CredentialConfig) ec2iface.EC2API

type Tagger struct {
	*Config

	logger           *zap.Logger
	cancelFunc       context.CancelFunc
	metadataProvider MetadataProvider
	ec2Provider      ec2ProviderType

	shutdownC          chan bool
	ec2TagCache        map[string]string
	started            bool
	ec2MetadataLookup  ec2MetadataLookupType
	ec2MetadataRespond ec2MetadataRespondType
	tagFilters         []*ec2.Filter
	ec2API             ec2iface.EC2API
	volumeSerialCache  volume.Cache

	sync.RWMutex //to protect ec2TagCache
}

// newTagger returns a new EC2 Tagger processor.
func newTagger(config *Config, logger *zap.Logger) *Tagger {
	_, cancel := context.WithCancel(context.Background())
	mdCredentialConfig := &configaws.CredentialConfig{}

	p := &Tagger{
		Config:           config,
		logger:           logger,
		cancelFunc:       cancel,
		metadataProvider: NewMetadataProvider(mdCredentialConfig.Credentials(), config.IMDSRetries),
		ec2Provider: func(ec2CredentialConfig *configaws.CredentialConfig) ec2iface.EC2API {
			return ec2.New(
				ec2CredentialConfig.Credentials(),
				&aws.Config{
					LogLevel: configaws.SDKLogLevel(),
					Logger:   configaws.SDKLogger{},
				})
		},
	}
	return p
}

func getOtelAttributes(m pmetric.Metric) []pcommon.Map {
	attributes := []pcommon.Map{}
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
	}
	return attributes
}

func (t *Tagger) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
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

// updateOtelAttributes adds tags and the requested dimensions to the attributes of each
// DataPoint. We add and remove at the DataPoint level instead of resource level because this is
// where the receiver/adapter does.
func (t *Tagger) updateOtelAttributes(attributes []pcommon.Map) {
	for _, attr := range attributes {
		if t.ec2TagCache != nil {
			for k, v := range t.ec2TagCache {
				attr.PutStr(k, v)
			}
		}
		if t.ec2MetadataLookup.instanceId {
			attr.PutStr(mdKeyInstanceId, t.ec2MetadataRespond.instanceId)
		}
		if t.ec2MetadataLookup.imageId {
			attr.PutStr(mdKeyImageId, t.ec2MetadataRespond.imageId)
		}
		if t.ec2MetadataLookup.instanceType {
			attr.PutStr(mdKeyInstanceType, t.ec2MetadataRespond.instanceType)
		}
		if t.volumeSerialCache != nil {
			if devName, found := attr.Get(t.DiskDeviceTagKey); found {
				serial := t.volumeSerialCache.Serial(devName.Str())
				if serial != "" {
					attr.PutStr(AttributeVolumeId, serial)
				}
			}
		}
		// If append_dimensions are applied, then remove the host dimension.
		attr.Remove("host")
	}
}

// updateTags calls EC2 Describe Tags and replaces the Tagger's tagCache with the newly retrieved values
func (t *Tagger) updateTags() error {
	tags := make(map[string]string)
	input := &ec2.DescribeTagsInput{
		Filters: t.tagFilters,
	}

	for {
		result, err := t.ec2API.DescribeTags(input)
		if err != nil {
			return err
		}
		for _, tag := range result.Tags {
			key := *tag.Key
			if ec2InstanceTagKeyASG == key {
				// rename to match CW dimension as applied by AutoScaling service, not the EC2 tag
				key = cwDimensionASG
			}
			tags[key] = *tag.Value
		}
		if result.NextToken == nil {
			break
		}
		input.SetNextToken(*result.NextToken)
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

// refreshLoop handles the refresh ticks and also responds to shutdown signal
func (t *Tagger) refreshLoop(refreshInterval time.Duration, stopAfterFirstSuccess bool) {
	refreshTicker := time.NewTicker(refreshInterval)
	defer refreshTicker.Stop()
	for {
		select {
		case <-refreshTicker.C:
			t.logger.Debug("ec2tagger refreshing")
			allTagsRetrieved := t.ec2TagsRetrieved()
			allVolumesRetrieved := t.ebsVolumesRetrieved()
			t.logger.Debug("Retrieve status",
				zap.Bool("Ec2AllTagsRetrieved", allTagsRetrieved),
				zap.Bool("EbsAllVolumesRetrieved", allVolumesRetrieved))
			refreshTags := len(t.EC2InstanceTagKeys) > 0
			refreshVolumes := len(t.EBSDeviceKeys) > 0

			if stopAfterFirstSuccess {
				// need refresh tags when it is configured and not all ec2 tags are retrieved
				refreshTags = refreshTags && !allTagsRetrieved
				// need refresh volumes when it is configured and not all volumes are retrieved
				refreshVolumes = refreshVolumes && !allVolumesRetrieved
				if !refreshTags && !refreshVolumes {
					t.logger.Info("ec2tagger: Refresh is no longer needed, stop refreshTicker.")
					return
				}
			}

			if refreshTags {
				if err := t.updateTags(); err != nil {
					t.logger.Warn("ec2tagger: Error refreshing EC2 tags, keeping old values", zap.Error(err))
				}
			}

			if refreshVolumes {
				if err := t.updateVolumes(); err != nil {
					t.logger.Warn("ec2tagger: Error refreshing EBS volumes, keeping old values", zap.Error(err))
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
			if key == ec2InstanceTagKeyASG {
				key = cwDimensionASG
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

// ebsVolumesRetrieved checks if all volumes are successfully retrieved
func (t *Tagger) ebsVolumesRetrieved() bool {
	allVolumesRetrieved := true

	for _, key := range t.EBSDeviceKeys {
		if key == "*" {
			continue
		}
		if volId := t.volumeSerialCache.Serial(key); volId == "" {
			allVolumesRetrieved = false
			break
		}
	}
	return allVolumesRetrieved
}

// Start acts as input validation and serves the purpose of updating ec2 tags and ebs volumes if necessary.
// It will be called when OTel is enabling each processor
func (t *Tagger) Start(ctx context.Context, _ component.Host) error {
	t.shutdownC = make(chan bool)
	t.ec2TagCache = map[string]string{}

	if err := t.deriveEC2MetadataFromIMDS(ctx); err != nil {
		return err
	}

	t.tagFilters = []*ec2.Filter{
		{
			Name:   aws.String("resource-type"),
			Values: aws.StringSlice([]string{"instance"}),
		},
		{
			Name:   aws.String("resource-id"),
			Values: aws.StringSlice([]string{t.ec2MetadataRespond.instanceId}),
		},
	}

	useAllTags := len(t.EC2InstanceTagKeys) == 1 && t.EC2InstanceTagKeys[0] == "*"

	if !useAllTags && len(t.EC2InstanceTagKeys) > 0 {
		// if the customer said 'AutoScalingGroupName' (the CW dimension), do what they mean not what they said
		// and filter for the EC2 tag name called 'aws:autoscaling:groupName'
		for i, key := range t.EC2InstanceTagKeys {
			if cwDimensionASG == key {
				t.EC2InstanceTagKeys[i] = ec2InstanceTagKeyASG
			}
		}

		t.tagFilters = append(t.tagFilters, &ec2.Filter{
			Name:   aws.String("key"),
			Values: aws.StringSlice(t.EC2InstanceTagKeys),
		})
	}

	if len(t.EC2InstanceTagKeys) > 0 || len(t.EBSDeviceKeys) > 0 {
		ec2CredentialConfig := &configaws.CredentialConfig{
			AccessKey: t.AccessKey,
			SecretKey: t.SecretKey,
			RoleARN:   t.RoleARN,
			Profile:   t.Profile,
			Filename:  t.Filename,
			Token:     t.Token,
			Region:    t.ec2MetadataRespond.region,
		}
		t.ec2API = t.ec2Provider(ec2CredentialConfig)
		go func() { //Async start of initial retrieval to prevent block of agent start
			t.initialRetrievalOfTagsAndVolumes()
			t.refreshLoopToUpdateTagsAndVolumes()
		}()
		t.logger.Info("ec2tagger: EC2 tagger has started initialization.")

	} else {
		t.setStarted()
	}

	return nil
}

func (t *Tagger) refreshLoopToUpdateTagsAndVolumes() {
	needRefresh := false
	stopAfterFirstSuccess := false
	refreshInterval := t.RefreshIntervalSeconds

	if t.RefreshIntervalSeconds.Seconds() == 0 {
		//when the refresh interval is 0, this means that customer don't want to
		//update tags/volumes values once they are retrieved successfully. In this case,
		//we still want to do refresh to make sure all the specified keys for tags/volumes
		//are fetched successfully because initial retrieval might not get all of them.
		//When the specified key is "*", there is no way for us to check if all
		//tags/volumes are fetched. So there is no need to do refresh in this case.
		needRefresh = !(len(t.EC2InstanceTagKeys) == 1 && t.EC2InstanceTagKeys[0] == "*") ||
			!(len(t.EBSDeviceKeys) == 1 && t.EBSDeviceKeys[0] == "*")
		stopAfterFirstSuccess = true
		refreshInterval = defaultRefreshInterval
	} else if t.RefreshIntervalSeconds.Seconds() > 0 {
		//customer wants to update the tags/volumes with the given refresh interval
		needRefresh = true
	}

	if needRefresh {
		go func() {
			// randomly stagger the time of the first refresh to mitigate throttling if a whole fleet is
			// restarted at the same time
			sleepUntilHostJitter(refreshInterval)
			t.refreshLoop(refreshInterval, stopAfterFirstSuccess)
		}()
	}
}

// updateVolumes calls EC2 describe volume
func (t *Tagger) updateVolumes() error {
	if t.volumeSerialCache == nil {
		t.volumeSerialCache = volume.NewCache(volume.NewProvider(t.ec2API, t.ec2MetadataRespond.instanceId))
	}

	if err := t.volumeSerialCache.Refresh(); err != nil {
		return err
	}

	t.logger.Debug("Volume Serial Cache", zap.Strings("devices", t.volumeSerialCache.Devices()))
	return nil
}

func (t *Tagger) setStarted() {
	t.Lock()
	t.started = true
	t.Unlock()
	t.logger.Info("ec2tagger: EC2 tagger has started, finished initial retrieval of tags and Volumes")
}

/*
Retrieve metadata from IMDS and use these metadata to:
* Extract InstanceID, ImageID, InstanceType to create custom dimension for collected metrics
* Extract InstanceID to retrieve Instance's Volume and Tags
* Extract Region to create aws session with custom configuration
For more information on IMDS, please follow this document https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html
*/
func (t *Tagger) deriveEC2MetadataFromIMDS(ctx context.Context) error {
	for _, tag := range t.EC2MetadataTags {
		switch tag {
		case mdKeyInstanceId:
			t.ec2MetadataLookup.instanceId = true
		case mdKeyImageId:
			t.ec2MetadataLookup.imageId = true
		case mdKeyInstanceType:
			t.ec2MetadataLookup.instanceType = true
		default:
			t.logger.Error("ec2tagger: Unsupported EC2 Metadata key", zap.String("mdKey", tag))
		}
	}

	t.logger.Info("ec2tagger: Check EC2 Metadata.")
	doc, err := t.metadataProvider.Get(ctx)
	if err != nil {
		t.logger.Error("ec2tagger: Unable to retrieve EC2 Metadata. This plugin must only be used on an EC2 instance.")
		if translatorCtx.CurrentContext().RunInContainer() {
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

// This function never return until calling updateTags() and updateVolumes() succeed or shutdown happen.
func (t *Tagger) initialRetrievalOfTagsAndVolumes() {
	tagsRetrieved := len(t.EC2InstanceTagKeys) == 0
	volsRetrieved := len(t.EBSDeviceKeys) == 0

	retry := 0
	for {
		var waitDuration time.Duration
		if retry < len(backoffSleepArray) {
			waitDuration = backoffSleepArray[retry]
		} else {
			waitDuration = backoffSleepArray[len(backoffSleepArray)-1]
		}

		wait := time.NewTimer(waitDuration)
		select {
		case <-t.shutdownC:
			wait.Stop()
			return
		case <-wait.C:
		}

		if retry > 0 {
			t.logger.Info("ec2tagger: initial retrieval of tags and volumes", zap.Int("retry", retry))
		}

		if !tagsRetrieved {
			if err := t.updateTags(); err != nil {
				t.logger.Warn("ec2tagger: Unable to describe ec2 tags for initial retrieval", zap.Error(err))
			} else {
				tagsRetrieved = true
			}
		}

		if !volsRetrieved {
			if err := t.updateVolumes(); err != nil {
				t.logger.Error("ec2tagger: Unable to describe ec2 volume for initial retrieval", zap.Error(err))
			} else {
				volsRetrieved = true
			}
		}

		if tagsRetrieved { // volsRetrieved is not checked to keep behavior consistency
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
