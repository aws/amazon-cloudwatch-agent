// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"errors"
	"testing"
	"time"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
)

type mockEC2Client struct {
	ec2iface.EC2API
	//The following fields are used to control how the mocked DescribeTags api behave:
	//tagsCallCount records how many times DescribeTags has been called
	//if tagsCallCount <= tagsFailLimit, DescribeTags call fails
	//if tagsFailLimit < tagsCallCount <= tagsPartialLimit, DescribeTags returns partial tags
	//if tagsCallCount > tagsPartialLimit, DescribeTags returns all tags
	//DescribeTags returns updated tags if UseUpdatedTags is true
	tagsCallCount    int
	tagsFailLimit    int
	tagsPartialLimit int
	UseUpdatedTags   bool

	//The following fields are used to control how the mocked DescribeVolumes api behave:
	//volumesCallCount records how many times DescribeVolumes has been called
	//if volumesCallCount <= volumesFailLimit, DescribeVolumes call fails
	//if volumesFailLimit < tagsCallCount <= volumesPartialLimit, DescribeVolumes returns partial volumes
	//if volumesCallCount > volumesPartialLimit, DescribeVolumes returns all volumes
	//DescribeVolumes returns update volumes if UseUpdatedVolumes is true
	volumesCallCount    int
	volumesFailLimit    int
	volumesPartialLimit int
	UseUpdatedVolumes   bool
}

// construct the return results for the mocked DescribeTags api
var (
	tagKey1 = "tagKey1"
	tagVal1 = "tagVal1"
	tagDes1 = ec2.TagDescription{Key: &tagKey1, Value: &tagVal1}
)

var (
	tagKey2 = "tagKey2"
	tagVal2 = "tagVal2"
	tagDes2 = ec2.TagDescription{Key: &tagKey2, Value: &tagVal2}
)

var (
	tagKey3 = "aws:autoscaling:groupName"
	tagVal3 = "ASG-1"
	tagDes3 = ec2.TagDescription{Key: &tagKey3, Value: &tagVal3}
)

var (
	updatedTagVal2 = "updated-tagVal2"
	updatedTagDes2 = ec2.TagDescription{Key: &tagKey2, Value: &updatedTagVal2}
)

func (m *mockEC2Client) DescribeTags(*ec2.DescribeTagsInput) (*ec2.DescribeTagsOutput, error) {
	//partial tags returned when the DescribeTags api are called initially
	//some tags are not returned because customer just attach them to the ec2 instance
	//and the api doesn't know about them yet
	partialTags := ec2.DescribeTagsOutput{
		NextToken: nil,
		Tags:      []*ec2.TagDescription{&tagDes1},
	}

	//all tags are returned when the ec2 metadata service knows about all tags
	allTags := ec2.DescribeTagsOutput{
		NextToken: nil,
		Tags:      []*ec2.TagDescription{&tagDes1, &tagDes2, &tagDes3},
	}

	//later customer changes the value of the second tag and DescribeTags api returns updated tags
	allTagsUpdated := ec2.DescribeTagsOutput{
		NextToken: nil,
		Tags:      []*ec2.TagDescription{&tagDes1, &updatedTagDes2, &tagDes3},
	}

	//return error initially to simulate the case
	//when tags are not ready or customer doesn't have permission to call the api
	if m.tagsCallCount <= m.tagsFailLimit {
		m.tagsCallCount++
		return nil, errors.New("No tags available now")
	}

	//return partial tags to simulate the case
	//when the api knows about some but not all tags at early stage
	if m.tagsCallCount <= m.tagsPartialLimit {
		m.tagsCallCount++
		return &partialTags, nil
	}

	//return all tags to simulate the case
	//when the api knows about all tags at later stage
	if m.tagsCallCount >= m.tagsPartialLimit {
		m.tagsCallCount++
		//return updated result after customer edits tags
		if m.UseUpdatedTags {
			return &allTagsUpdated, nil
		}
		return &allTags, nil
	}
	return nil, nil
}

// construct the return results for the mocked DescribeTags api
var (
	device1             = "/dev/xvdc"
	volumeId1           = "vol-0303a1cc896c42d28"
	volumeAttachmentId1 = "aws://us-east-1a/vol-0303a1cc896c42d28"
	volumeAttachment1   = ec2.VolumeAttachment{Device: &device1, VolumeId: &volumeId1}
	availabilityZone    = "us-east-1a"
	volume1             = ec2.Volume{
		Attachments:      []*ec2.VolumeAttachment{&volumeAttachment1},
		AvailabilityZone: &availabilityZone,
	}
)

var (
	device2             = "/dev/xvdf"
	volumeId2           = "vol-0c241693efb58734a"
	volumeAttachmentId2 = "aws://us-east-1a/vol-0c241693efb58734a"
	volumeAttachment2   = ec2.VolumeAttachment{Device: &device2, VolumeId: &volumeId2}
	volume2             = ec2.Volume{
		Attachments:      []*ec2.VolumeAttachment{&volumeAttachment2},
		AvailabilityZone: &availabilityZone,
	}
)

var (
	volumeId2Updated           = "vol-0459607897eaa8148"
	volumeAttachmentUpdatedId2 = "aws://us-east-1a/vol-0459607897eaa8148"
	volumeAttachment2Updated   = ec2.VolumeAttachment{Device: &device2, VolumeId: &volumeId2Updated}
	volume2Updated             = ec2.Volume{
		Attachments:      []*ec2.VolumeAttachment{&volumeAttachment2Updated},
		AvailabilityZone: &availabilityZone,
	}
)

func (m *mockEC2Client) DescribeVolumes(*ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error) {
	//volume1 is the initial disk assigned to an ec2 instance when started
	partialVolumes := ec2.DescribeVolumesOutput{
		NextToken: nil,
		Volumes:   []*ec2.Volume{&volume1},
	}

	//later customer attached volume2 to the running ec2 instance
	//but this volume might not be known to the api immediately
	allVolumes := ec2.DescribeVolumesOutput{
		NextToken: nil,
		Volumes:   []*ec2.Volume{&volume1, &volume2},
	}

	//later customer updates by attaching a different ebs volume to the same device name
	allVolumesUpdated := ec2.DescribeVolumesOutput{
		NextToken: nil,
		Volumes:   []*ec2.Volume{&volume1, &volume2Updated},
	}

	//return error initially to simulate the case
	//when the volumes are not ready or customer doesn't have permission to call the api
	if m.volumesCallCount <= m.volumesFailLimit {
		m.volumesCallCount++
		return nil, errors.New("No volumes available now")
	}

	//return partial volumes to simulate the case
	//when the api knows about some but not all volumes at early stage
	if m.volumesCallCount <= m.volumesPartialLimit {
		m.volumesCallCount++
		return &partialVolumes, nil
	}

	//return all volumes to simulate the case
	//when the api knows about all volumes at later stage
	if m.volumesCallCount > m.volumesPartialLimit {
		m.volumesCallCount++
		//return updated result after customer edits volumes
		if m.UseUpdatedVolumes {
			return &allVolumesUpdated, nil
		}
		return &allVolumes, nil
	}
	return nil, nil
}

type mockEC2Metadata struct {
	EC2MetadataAPI
	InstanceIdentityDocument *ec2metadata.EC2InstanceIdentityDocument
}

var mockedInstanceIdentityDoc = &ec2metadata.EC2InstanceIdentityDocument{
	InstanceID:   "i-01d2417c27a396e44",
	Region:       "us-east-1",
	InstanceType: "m5ad.large",
	ImageID:      "ami-09edd32d9b0990d49",
}

func (m *mockEC2Metadata) GetInstanceIdentityDocument() (ec2metadata.EC2InstanceIdentityDocument, error) {
	if m.InstanceIdentityDocument != nil {
		return *m.InstanceIdentityDocument, nil
	}
	return ec2metadata.EC2InstanceIdentityDocument{}, errors.New("No instance identity document")
}

func TestInitFailWithNoMetadata(t *testing.T) {
	assert := assert.New(t)

	metadataClient := &mockEC2Metadata{
		InstanceIdentityDocument: nil,
	}
	ec2MetadataProvider := func() EC2MetadataAPI {
		return metadataClient
	}

	tagger := Tagger{
		Log:                 testutil.Logger{},
		ec2MetadataProvider: ec2MetadataProvider,
	}
	err := tagger.Init()

	assert.NotNil(err)
	assert.Contains(err.Error(), "No instance identity document")
}

//run Init() and check all tags/volumes are retrieved and saved
func TestInitSuccessWithNoTagsVolumesUpdate(t *testing.T) {
	assert := assert.New(t)

	metadataClient := &mockEC2Metadata{
		InstanceIdentityDocument: mockedInstanceIdentityDoc,
	}
	ec2MetadataProvider := func() EC2MetadataAPI {
		return metadataClient
	}

	ec2Client := &mockEC2Client{
		tagsCallCount:       0,
		tagsFailLimit:       0,
		tagsPartialLimit:    1,
		UseUpdatedTags:      false,
		volumesCallCount:    0,
		volumesFailLimit:    -1,
		volumesPartialLimit: 0,
		UseUpdatedVolumes:   false,
	}
	ec2Provider := func(*configaws.CredentialConfig) ec2iface.EC2API {
		return ec2Client
	}
	backoffSleepArray = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	defaultRefreshInterval = 50 * time.Millisecond
	tagger := Tagger{
		Log:                    testutil.Logger{},
		RefreshIntervalSeconds: internal.Duration{Duration: 0},
		ec2Provider:            ec2Provider,
		ec2API:                 ec2Client,
		ec2MetadataProvider:    ec2MetadataProvider,
		EC2MetadataTags:        []string{mdKeyInstanceId, mdKeyImageId, mdKeyInstanceType},
		EC2InstanceTagKeys:     []string{tagKey1, tagKey2, "AutoScalingGroupName"},
		EBSDeviceKeys:          []string{device1, device2},
	}
	err := tagger.Init()
	assert.Nil(err)
	//assume one second is long enough for the api to be called many times so that all tags/volumes are retrieved
	time.Sleep(time.Second)
	assert.Equal(3, ec2Client.tagsCallCount)
	assert.Equal(2, ec2Client.volumesCallCount)
	//check tags and volumes
	expectedTags := map[string]string{tagKey1: tagVal1, tagKey2: tagVal2, "AutoScalingGroupName": tagVal3}
	assert.Equal(expectedTags, tagger.ec2TagCache)
	expectedVolumes := map[string]string{device1: volumeAttachmentId1, device2: volumeAttachmentId2}
	assert.Equal(expectedVolumes, tagger.ebsVolume.dev2Vol)
}

//run Init() and check all tags/volumes are retrieved and saved and then updated
func TestInitSuccessWithTagsVolumesUpdate(t *testing.T) {
	assert := assert.New(t)
	metadataClient := &mockEC2Metadata{
		InstanceIdentityDocument: mockedInstanceIdentityDoc,
	}
	ec2MetadataProvider := func() EC2MetadataAPI {
		return metadataClient
	}
	ec2Client := &mockEC2Client{
		tagsCallCount:       0,
		tagsFailLimit:       1,
		tagsPartialLimit:    2,
		UseUpdatedTags:      false,
		volumesCallCount:    0,
		volumesFailLimit:    -1,
		volumesPartialLimit: 0,
		UseUpdatedVolumes:   false,
	}
	ec2Provider := func(*configaws.CredentialConfig) ec2iface.EC2API {
		return ec2Client
	}
	backoffSleepArray = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	defaultRefreshInterval = 10 * time.Millisecond
	tagger := Tagger{
		Log: testutil.Logger{},
		//use millisecond rather than second to speed up test execution
		RefreshIntervalSeconds: internal.Duration{Duration: 20 * time.Millisecond},
		ec2Provider:            ec2Provider,
		ec2API:                 ec2Client,
		ec2MetadataProvider:    ec2MetadataProvider,
		EC2MetadataTags:        []string{mdKeyInstanceId, mdKeyImageId, mdKeyInstanceType},
		EC2InstanceTagKeys:     []string{tagKey1, tagKey2, "AutoScalingGroupName"},
		EBSDeviceKeys:          []string{device1, device2},
	}
	err := tagger.Init()
	assert.Nil(err)
	//assume one second is long enough for the api to be called many times
	//so that all tags/volumes are retrieved
	time.Sleep(time.Second)
	//check tags and volumes
	expectedTags := map[string]string{tagKey1: tagVal1, tagKey2: tagVal2, "AutoScalingGroupName": tagVal3}
	assert.Equal(expectedTags, tagger.ec2TagCache)
	expectedVolumes := map[string]string{device1: volumeAttachmentId1, device2: volumeAttachmentId2}
	assert.Equal(expectedVolumes, tagger.ebsVolume.dev2Vol)

	//update the tags and volumes
	ec2Client.UseUpdatedTags = true
	ec2Client.UseUpdatedVolumes = true
	//assume one second is long enough for the api to be called many times
	//so that all tags/volumes are updated
	time.Sleep(time.Second)
	expectedTags = map[string]string{tagKey1: tagVal1, tagKey2: updatedTagVal2, "AutoScalingGroupName": tagVal3}
	assert.Equal(expectedTags, tagger.ec2TagCache)
	expectedVolumes = map[string]string{device1: volumeAttachmentId1, device2: volumeAttachmentUpdatedId2}
	assert.Equal(expectedVolumes, tagger.ebsVolume.dev2Vol)
}

//run Init() with ec2_instance_tag_keys = ["*"] and ebs_device_keys = ["*"]
//check there is no attempt to fetch all tags/volumes
func TestInitSuccessWithWildcardTagVolumeKey(t *testing.T) {
	assert := assert.New(t)

	metadataClient := &mockEC2Metadata{
		InstanceIdentityDocument: mockedInstanceIdentityDoc,
	}
	ec2MetadataProvider := func() EC2MetadataAPI {
		return metadataClient
	}

	ec2Client := &mockEC2Client{
		tagsCallCount:       0,
		tagsFailLimit:       0,
		tagsPartialLimit:    1,
		UseUpdatedTags:      false,
		volumesCallCount:    0,
		volumesFailLimit:    -1,
		volumesPartialLimit: 0,
		UseUpdatedVolumes:   false,
	}
	ec2Provider := func(*configaws.CredentialConfig) ec2iface.EC2API {
		return ec2Client
	}
	backoffSleepArray = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	defaultRefreshInterval = 50 * time.Millisecond
	tagger := Tagger{
		Log:                    testutil.Logger{},
		RefreshIntervalSeconds: internal.Duration{Duration: 0},
		ec2Provider:            ec2Provider,
		ec2API:                 ec2Client,
		ec2MetadataProvider:    ec2MetadataProvider,
		EC2MetadataTags:        []string{mdKeyInstanceId, mdKeyImageId, mdKeyInstanceType},
		EC2InstanceTagKeys:     []string{"*"},
		EBSDeviceKeys:          []string{"*"},
	}
	err := tagger.Init()
	assert.Nil(err)
	//assume one second is long enough for the api to be called many times (potentially)
	time.Sleep(time.Second)
	//check only partial tags/volumes are returned
	assert.Equal(2, ec2Client.tagsCallCount)
	assert.Equal(1, ec2Client.volumesCallCount)
	//check partial tags/volumes are saved
	expectedTags := map[string]string{tagKey1: tagVal1}
	assert.Equal(expectedTags, tagger.ec2TagCache)
	expectedVolumes := map[string]string{device1: volumeAttachmentId1}
	assert.Equal(expectedVolumes, tagger.ebsVolume.dev2Vol)
}

//run Init() and then Apply() and check the output metrics contain expected tags
func TestApplyWithTagsVolumesUpdate(t *testing.T) {
	assert := assert.New(t)

	metadataClient := &mockEC2Metadata{
		InstanceIdentityDocument: mockedInstanceIdentityDoc,
	}
	ec2MetadataProvider := func() EC2MetadataAPI {
		return metadataClient
	}

	ec2Client := &mockEC2Client{
		tagsCallCount:       0,
		tagsFailLimit:       0,
		tagsPartialLimit:    1,
		UseUpdatedTags:      false,
		volumesCallCount:    0,
		volumesFailLimit:    -1,
		volumesPartialLimit: 0,
		UseUpdatedVolumes:   false,
	}
	ec2Provider := func(*configaws.CredentialConfig) ec2iface.EC2API {
		return ec2Client
	}
	backoffSleepArray = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	defaultRefreshInterval = 50 * time.Millisecond
	tagger := Tagger{
		Log: testutil.Logger{},
		//use millisecond rather than second to speed up test execution
		RefreshIntervalSeconds: internal.Duration{Duration: 20 * time.Millisecond},
		ec2Provider:            ec2Provider,
		ec2API:                 ec2Client,
		ec2MetadataProvider:    ec2MetadataProvider,
		EC2MetadataTags:        []string{mdKeyInstanceId, mdKeyInstanceType},
		EC2InstanceTagKeys:     []string{tagKey1, tagKey2, "AutoScalingGroupName"},
		EBSDeviceKeys:          []string{device1, device2},
		DiskDeviceTagKey:       "device",
	}
	err := tagger.Init()
	assert.Nil(err)
	//assume one second is long enough for the api to be called many times
	//so that all tags/volumes are retrieved
	time.Sleep(time.Second)
	input := []telegraf.Metric{
		testutil.MustMetric(
			"cpu",
			map[string]string{
				"host": "example.org",
			},
			map[string]interface{}{
				"cpu": 0.11,
			},
			time.Unix(0, 0),
		),
		testutil.MustMetric(
			"disk",
			map[string]string{
				"device": device2,
			},
			map[string]interface{}{
				"write_bytes": 135,
			},
			time.Unix(0, 0)),
	}
	output := tagger.Apply(input...)
	expectedOutput := []telegraf.Metric{
		testutil.MustMetric(
			"cpu",
			map[string]string{
				"host":                 "example.org",
				"AutoScalingGroupName": "ASG-1",
				"InstanceId":           "i-01d2417c27a396e44",
				"InstanceType":         "m5ad.large",
				tagKey1:                tagVal1,
				tagKey2:                tagVal2,
			},
			map[string]interface{}{
				"cpu": 0.11,
			},
			time.Unix(0, 0),
		),
		testutil.MustMetric(
			"disk",
			map[string]string{
				"AutoScalingGroupName": "ASG-1",
				"EBSVolumeId":          "aws://us-east-1a/vol-0c241693efb58734a",
				"InstanceId":           "i-01d2417c27a396e44",
				"InstanceType":         "m5ad.large",
				tagKey1:                tagVal1,
				tagKey2:                tagVal2,
				"device":               device2,
			},
			map[string]interface{}{
				"write_bytes": 135,
			},
			time.Unix(0, 0),
		),
	}
	testutil.RequireMetricsEqual(t, expectedOutput, output)

	//update tags and volumes and check metrics are updated as well
	ec2Client.UseUpdatedTags = true
	ec2Client.UseUpdatedVolumes = true
	//assume one second is long enough for the api to be called many times
	//so that all tags/volumes are updated
	time.Sleep(time.Second)
	outputUpdated := tagger.Apply(input...)
	expectedOutputUpdated := []telegraf.Metric{
		testutil.MustMetric(
			"cpu",
			map[string]string{
				"host":                 "example.org",
				"AutoScalingGroupName": "ASG-1",
				"InstanceId":           "i-01d2417c27a396e44",
				"InstanceType":         "m5ad.large",
				tagKey1:                tagVal1,
				tagKey2:                updatedTagVal2,
			},
			map[string]interface{}{
				"cpu": 0.11,
			},
			time.Unix(0, 0),
		),
		testutil.MustMetric(
			"disk",
			map[string]string{
				"AutoScalingGroupName": "ASG-1",
				"EBSVolumeId":          "aws://us-east-1a/vol-0459607897eaa8148",
				"InstanceId":           "i-01d2417c27a396e44",
				"InstanceType":         "m5ad.large",
				tagKey1:                tagVal1,
				tagKey2:                updatedTagVal2,
				"device":               device2,
			},
			map[string]interface{}{
				"write_bytes": 135,
			},
			time.Unix(0, 0),
		),
	}
	testutil.RequireMetricsEqual(t, expectedOutputUpdated, outputUpdated)
}

// Test metrics are dropped before the initial retrieval is done
func TestMetricsDroppedBeforeStarted(t *testing.T) {
	assert := assert.New(t)

	metadataClient := &mockEC2Metadata{
		InstanceIdentityDocument: mockedInstanceIdentityDoc,
	}
	ec2MetadataProvider := func() EC2MetadataAPI {
		return metadataClient
	}

	ec2Client := &mockEC2Client{
		tagsCallCount:       0,
		tagsFailLimit:       0,
		tagsPartialLimit:    1,
		UseUpdatedTags:      false,
		volumesCallCount:    0,
		volumesFailLimit:    -1,
		volumesPartialLimit: 0,
		UseUpdatedVolumes:   false,
	}
	ec2Provider := func(*configaws.CredentialConfig) ec2iface.EC2API {
		return ec2Client
	}
	backoffSleepArray = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	defaultRefreshInterval = 50 * time.Millisecond
	tagger := Tagger{
		Log:                    testutil.Logger{},
		RefreshIntervalSeconds: internal.Duration{Duration: 0},
		ec2Provider:            ec2Provider,
		ec2API:                 ec2Client,
		ec2MetadataProvider:    ec2MetadataProvider,
		EC2MetadataTags:        []string{mdKeyInstanceId, mdKeyImageId, mdKeyInstanceType},
		EC2InstanceTagKeys:     []string{"*"},
		EBSDeviceKeys:          []string{"*"},
	}

	input := []telegraf.Metric{
		testutil.MustMetric(
			"cpu",
			map[string]string{
				"host": "example.org",
			},
			map[string]interface{}{
				"cpu": 0.11,
			},
			time.Unix(0, 0),
		),
		testutil.MustMetric(
			"disk",
			map[string]string{
				"device": device1,
			},
			map[string]interface{}{
				"write_bytes": 200,
			},
			time.Unix(0, 0),
		),
		testutil.MustMetric(
			"disk",
			map[string]string{
				"device": device2,
			},
			map[string]interface{}{
				"write_bytes": 135,
			},
			time.Unix(0, 0),
		),
	}

	err := tagger.Init()
	assert.Nil(err)
	assert.Equal(tagger.started, false)

	results := tagger.Apply(input...)
	assert.Equal(len(results), 0)

	//assume one second is long enough for the api to be called many times (potentially)
	time.Sleep(time.Second)
	//check only partial tags/volumes are returned
	assert.Equal(2, ec2Client.tagsCallCount)
	assert.Equal(1, ec2Client.volumesCallCount)

	//check partial tags/volumes are saved
	expectedTags := map[string]string{tagKey1: tagVal1}
	assert.Equal(expectedTags, tagger.ec2TagCache)
	expectedVolumes := map[string]string{device1: volumeAttachmentId1}
	assert.Equal(expectedVolumes, tagger.ebsVolume.dev2Vol)

	assert.Equal(tagger.started, true)
	results = tagger.Apply(input...)

	assert.Equal(len(results), 3)
}

// Test ec2tagger init does not block for a long time
func TestTaggerInitDoesNotBlock(t *testing.T) {
	assert := assert.New(t)

	metadataClient := &mockEC2Metadata{
		InstanceIdentityDocument: mockedInstanceIdentityDoc,
	}
	ec2MetadataProvider := func() EC2MetadataAPI {
		return metadataClient
	}

	ec2Client := &mockEC2Client{
		tagsCallCount:       0,
		tagsFailLimit:       0,
		tagsPartialLimit:    1,
		UseUpdatedTags:      false,
		volumesCallCount:    0,
		volumesFailLimit:    -1,
		volumesPartialLimit: 0,
		UseUpdatedVolumes:   false,
	}
	ec2Provider := func(*configaws.CredentialConfig) ec2iface.EC2API {
		return ec2Client
	}
	backoffSleepArray = []time.Duration{1 * time.Minute, 1 * time.Minute, 1 * time.Minute, 3 * time.Minute, 3 * time.Minute, 3 * time.Minute, 10 * time.Minute}
	defaultRefreshInterval = 180 * time.Second
	tagger := Tagger{
		Log:                    testutil.Logger{},
		RefreshIntervalSeconds: internal.Duration{Duration: 0},
		ec2Provider:            ec2Provider,
		ec2API:                 ec2Client,
		ec2MetadataProvider:    ec2MetadataProvider,
		EC2MetadataTags:        []string{mdKeyInstanceId, mdKeyImageId, mdKeyInstanceType},
		EC2InstanceTagKeys:     []string{"*"},
		EBSDeviceKeys:          []string{"*"},
	}

	deadline := time.NewTimer(1 * time.Second)
	inited := make(chan struct{})
	go func() {
		select {
		case <-deadline.C:
			t.Errorf("Tagger Init took too long to finish")
		case <-inited:
		}
	}()
	err := tagger.Init()
	assert.Nil(err)
	assert.Equal(tagger.started, false)
	close(inited)

}

// Test ec2tagger init does not block for a long time
func TestTaggerStartsWithoutTagOrVolume(t *testing.T) {
	assert := assert.New(t)

	metadataClient := &mockEC2Metadata{
		InstanceIdentityDocument: mockedInstanceIdentityDoc,
	}
	ec2MetadataProvider := func() EC2MetadataAPI {
		return metadataClient
	}

	tagger := Tagger{
		Log:                    testutil.Logger{},
		RefreshIntervalSeconds: internal.Duration{Duration: 0},
		ec2MetadataProvider:    ec2MetadataProvider,
		EC2MetadataTags:        []string{mdKeyInstanceId, mdKeyImageId, mdKeyInstanceType},
	}

	deadline := time.NewTimer(1 * time.Second)
	inited := make(chan struct{})
	go func() {
		select {
		case <-deadline.C:
			t.Errorf("Tagger Init took too long to finish")
		case <-inited:
		}
	}()
	err := tagger.Init()
	assert.Nil(err)
	assert.Equal(tagger.started, true)
	close(inited)
}
