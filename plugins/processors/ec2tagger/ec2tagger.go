// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"hash/fnv"
	"net/http"
	"os"
	"sync"
	"time"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/processors"
)

type EC2MetadataAPI interface {
	GetInstanceIdentityDocument() (ec2metadata.EC2InstanceIdentityDocument, error)
}

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
type ec2MetadataProviderType func() EC2MetadataAPI

type Tagger struct {
	Log                    telegraf.Logger   `toml:"-"`
	RefreshIntervalSeconds internal.Duration `toml:"refresh_interval_seconds"`
	EC2MetadataTags        []string          `toml:"ec2_metadata_tags"`
	EC2InstanceTagKeys     []string          `toml:"ec2_instance_tag_keys"`
	EBSDeviceKeys          []string          `toml:"ebs_device_keys"`
	//The tag key in the metrics for disk device
	DiskDeviceTagKey string `toml:"disk_device_tag_key"`

	// unlike other AWS plugins, this one determines the region from ec2 metadata not user configuration
	AccessKey string `toml:"access_key"`
	SecretKey string `toml:"secret_key"`
	RoleARN   string `toml:"role_arn"`
	Profile   string `toml:"profile"`
	Filename  string `toml:"shared_credential_file"`
	Token     string `toml:"token"`

	ec2TagCache         map[string]string
	started             bool
	ec2Provider         ec2ProviderType
	ec2API              ec2iface.EC2API
	ec2MetadataProvider ec2MetadataProviderType
	ec2MetadataRespond  ec2MetadataRespondType
	ec2MetadataLookup   ec2MetadataLookupType
	refreshTicker       *time.Ticker
	shutdownC           chan bool
	tagFilters          []*ec2.Filter
	ebsVolume           *EbsVolume

	sync.RWMutex //to protect ec2TagCache
}

func (t *Tagger) SampleConfig() string {
	return sampleConfig
}

func (t *Tagger) Description() string {
	return "Configuration for adding EC2 Metadata and Instance Tags and EBS volumes to metrics."
}

// Apply adds the configured EC2 Metadata and Instance Tags to metrics.
// This is called serially for ALL metrics (that pass the plugin's tag filters) so keep it fast.
func (t *Tagger) Apply(in ...telegraf.Metric) []telegraf.Metric {
	// grab the pointer to the map in case it gets refreshed while we're applying this round of metrics. At least
	// this batch then will all get the same tags.
	t.RLock()
	defer t.RUnlock()

	if !t.started {
		return []telegraf.Metric{}
	}

	for _, metric := range in {
		if t.ec2TagCache != nil {
			for k, v := range t.ec2TagCache {
				metric.AddTag(k, v)
			}
		}
		if t.ec2MetadataLookup.instanceId {
			metric.AddTag(mdKeyInstanceId, t.ec2MetadataRespond.instanceId)
		}
		if t.ec2MetadataLookup.imageId {
			metric.AddTag(mdKeyImageId, t.ec2MetadataRespond.imageId)
		}
		if t.ec2MetadataLookup.instanceType {
			metric.AddTag(mdKeyInstanceType, t.ec2MetadataRespond.instanceType)
		}
		if t.ebsVolume != nil && metric.HasTag(t.DiskDeviceTagKey) {
			devName := metric.Tags()[t.DiskDeviceTagKey]
			ebsVolId := t.ebsVolume.getEbsVolumeId(devName)
			if ebsVolId != "" {
				metric.AddTag(ebsVolumeId, ebsVolId)
			}
		}
	}
	return in
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

// Shutdown currently does not get called, as telegraf does not have a cleanup hook for Filter plugins
func (t *Tagger) Shutdown() {
	close(t.shutdownC)
}

// refreshLoop handles the refresh ticks and also responds to shutdown signal
func (t *Tagger) refreshLoop(refreshInterval time.Duration, stopAfterFirstSuccess bool) {
	refreshTicker := time.NewTicker(refreshInterval)
	defer refreshTicker.Stop()
	for {
		select {
		case <-refreshTicker.C:
			t.Log.Debugf("ec2tagger refreshing: EC2InstanceTags needed %v, retrieved: %v, ebs device needed %v, retrieved: %v", len(t.EC2InstanceTagKeys), t.ec2TagsRetrieved(), len(t.EBSDeviceKeys), t.ebsVolumesRetrieved())
			refreshTags := len(t.EC2InstanceTagKeys) > 0
			refreshVolumes := len(t.EBSDeviceKeys) > 0

			if stopAfterFirstSuccess {
				// need refresh tags when it is configured and not all ec2 tags are retrieved
				refreshTags = refreshTags && !t.ec2TagsRetrieved()
				// need refresh volumes when it is configured and not all volumes are retrieved
				refreshVolumes = refreshVolumes && !t.ebsVolumesRetrieved()
				if !refreshTags && !refreshVolumes {
					t.Log.Info("ec2tagger: Refresh is no longer needed, stop refreshTicker.")
					return
				}
			}

			if refreshTags {
				if err := t.updateTags(); err != nil {
					t.Log.Warnf("ec2tagger: Error refreshing EC2 tags, keeping old values : %+v", err.Error())
				}
			}

			if refreshVolumes {
				if err := t.updateVolumes(); err != nil {
					t.Log.Warnf("ec2tagger: Error refreshing EC2 volumes, keeping old values : %+v", err.Error())
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

//ebsVolumesRetrieved checks if all volumes are successfully retrieved
func (t *Tagger) ebsVolumesRetrieved() bool {
	allVolumesRetrieved := true

	for _, key := range t.EBSDeviceKeys {
		if key == "*" {
			continue
		}
		if volId := t.ebsVolume.getEbsVolumeId(key); volId == "" {
			allVolumesRetrieved = false
			break
		}
	}
	return allVolumesRetrieved
}

//Init() acts as input validation and serves the purpose of updating ec2 tags and ebs volumes if necessary.
//It will be called when Telegraf is enabling each processor plugin
func (t *Tagger) Init() error {
	t.shutdownC = make(chan bool)
	t.ec2TagCache = map[string]string{}

	if err := t.deriveEC2MetadataFromIMDS(); err != nil {
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
		t.Log.Info("ec2tagger: EC2 tagger has started initialization.")

	} else {
		t.setStarted()
	}

	return nil
}

func (t *Tagger) refreshLoopToUpdateTagsAndVolumes() {
	needRefresh := false
	stopAfterFirstSuccess := false
	refreshInterval := t.RefreshIntervalSeconds.Duration

	if t.RefreshIntervalSeconds.Duration.Seconds() == 0 {
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
	} else if t.RefreshIntervalSeconds.Duration.Seconds() > 0 {
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
	if t.ebsVolume == nil {
		t.ebsVolume = NewEbsVolume()
	}

	input := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("attachment.instance-id"),
				Values: aws.StringSlice([]string{t.ec2MetadataRespond.instanceId}),
			},
		},
	}

	for {
		result, err := t.ec2API.DescribeVolumes(input)
		if err != nil {
			return err
		}
		for _, volume := range result.Volumes {
			for _, attachment := range volume.Attachments {
				t.ebsVolume.addEbsVolumeMapping(volume.AvailabilityZone, attachment)
			}
		}
		if result.NextToken == nil {
			break
		}
		input.SetNextToken(*result.NextToken)
	}
	return nil
}

func (t *Tagger) setStarted() {
	t.Lock()
	t.started = true
	t.Unlock()
	t.Log.Info("ec2tagger: EC2 tagger has started, finished initial retrieval of tags and Volumes")
}

/*
	Retrieve metadata from IMDS and use these metadata to:
	* Extract InstanceID, ImageID, InstanceType to create custom dimension for collected metrics
	* Extract InstanceID to retrieve Instance's Volume and Tags
	* Extract Region to create aws session with custom configuration
	For more information on IMDS, please follow this document https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html
*/
func (t *Tagger) deriveEC2MetadataFromIMDS() error {
	for _, tag := range t.EC2MetadataTags {
		switch tag {
		case mdKeyInstanceId:
			t.ec2MetadataLookup.instanceId = true
		case mdKeyImageId:
			t.ec2MetadataLookup.imageId = true
		case mdKeyInstanceType:
			t.ec2MetadataLookup.instanceType = true
		default:
			t.Log.Errorf("ec2tagger: Unsupported EC2 Metadata key: %s.", tag)
		}
	}

	t.Log.Infof("ec2tagger: Check EC2 Metadata.")
	doc, err := t.ec2MetadataProvider().GetInstanceIdentityDocument()
	if err != nil {
		t.Log.Error("ec2tagger: Unable to retrieve EC2 Metadata. This plugin must only be used on an EC2 instance.")
		if context.CurrentContext().RunInContainer() {
			t.Log.Warn("ec2tagger: Timeout may have occurred because hop limit is too small. Please increase hop limit to 2 by following this document https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-options.html#configuring-IMDS-existing-instances.")
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
			t.Log.Infof("ec2tagger: %v retry for initial retrieval of tags and volumes", retry)
		}

		if !tagsRetrieved {
			if err := t.updateTags(); err != nil {
				t.Log.Warnf("ec2tagger: Unable to describe ec2 tags for initial retrieval: %v", err)
			} else {
				tagsRetrieved = true
			}
		}

		if !volsRetrieved {
			if err := t.updateVolumes(); err != nil {
				t.Log.Errorf("ec2tagger: Unable to describe ec2 volume for initial retrieval: %v", err)
			} else {
				volsRetrieved = true
			}
		}

		if tagsRetrieved { // volsRetrieved is not checked to keep behavior consistency
			t.Log.Infof("ec2tagger: Initial retrieval of tags succeeded")
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

// init adds this plugin to the framework's "processors" registry
func init() {
	processors.Add("ec2tagger", func() telegraf.Processor {
		ec2MetadataProvider := func() EC2MetadataAPI {
			mdCredentialConfig := &configaws.CredentialConfig{}
			return ec2metadata.New(
				mdCredentialConfig.Credentials(),
				&aws.Config{
					HTTPClient: &http.Client{Timeout: defaultIMDSTimeout},
					LogLevel:   configaws.SDKLogLevel(),
					Logger:     configaws.SDKLogger{},
					Retryer:    client.DefaultRetryer{NumMaxRetries: allowedIMDSRetries},
				})
		}
		ec2Provider := func(ec2CredentialConfig *configaws.CredentialConfig) ec2iface.EC2API {
			return ec2.New(
				ec2CredentialConfig.Credentials(),
				&aws.Config{
					LogLevel: configaws.SDKLogLevel(),
					Logger:   configaws.SDKLogger{},
				})
		}
		return &Tagger{
			ec2MetadataProvider: ec2MetadataProvider,
			ec2Provider:         ec2Provider,
		}
	})
}
