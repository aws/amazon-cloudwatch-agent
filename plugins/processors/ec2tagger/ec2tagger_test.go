// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"
	"golang.org/x/exp/maps"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
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
		return nil, errors.New("no tags available now")
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
	device1   = "xvdc"
	volumeId1 = "vol-0303a1cc896c42d28"
)

var (
	device2   = "xvdf"
	volumeId2 = "vol-0c241693efb58734a"
)

var (
	volumeId2Updated = "vol-0459607897eaa8148"
)

type mockMetadataProvider struct {
	InstanceIdentityDocument *ec2metadata.EC2InstanceIdentityDocument
}

func (m *mockMetadataProvider) Get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	if m.InstanceIdentityDocument != nil {
		return *m.InstanceIdentityDocument, nil
	}
	return ec2metadata.EC2InstanceIdentityDocument{}, errors.New("No instance identity document")
}

func (m *mockMetadataProvider) Hostname(ctx context.Context) (string, error) {
	return "MockHostName", nil
}

func (m *mockMetadataProvider) InstanceID(ctx context.Context) (string, error) {
	return "MockInstanceID", nil
}

var mockedInstanceIdentityDoc = &ec2metadata.EC2InstanceIdentityDocument{
	InstanceID:   "i-01d2417c27a396e44",
	Region:       "us-east-1",
	InstanceType: "m5ad.large",
	ImageID:      "ami-09edd32d9b0990d49",
}

type mockVolumeCache struct {
	sync.RWMutex
	cache               map[string]string
	refreshCount        int
	volumesPartialLimit int
	UseUpdatedVolumes   bool
}

func (m *mockVolumeCache) Refresh() error {
	m.Lock()
	defer m.Unlock()
	if m.refreshCount <= m.volumesPartialLimit {
		m.cache[device1] = volumeId1
	} else {
		if m.UseUpdatedVolumes {
			m.cache[device2] = volumeId2Updated
		} else {
			m.cache[device2] = volumeId2
		}
	}
	m.refreshCount++
	return nil
}

func (m *mockVolumeCache) Serial(devName string) string {
	m.RLock()
	defer m.RUnlock()
	return m.cache[devName]
}

func (m *mockVolumeCache) Devices() []string {
	m.RLock()
	defer m.RUnlock()
	return maps.Keys(m.cache)
}

// createTestMetrics create new pmetric.Metrics pm that satisfies:
//
//	pm.ResourceMetrics().Len() == 1
//	pm.ResourceMetrics().At(0).ScopeMetrics().Len() == 1
//	pm.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len() == len(metrics)
//
// and for each metric from metrics it create one single datapoint that appy all tags/attributes from metric
func createTestMetrics(metrics []map[string]string) pmetric.Metrics {
	pm := pmetric.NewMetrics()
	rm := pm.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	for i, metric := range metrics {
		m := sm.Metrics().AppendEmpty()
		var dp pmetric.NumberDataPoint
		if i%2 == 0 {
			m.SetEmptyGauge()
			dp = m.Gauge().DataPoints().AppendEmpty()
		} else {
			m.SetEmptySum()
			dp = m.Sum().DataPoints().AppendEmpty()
		}

		for attrKey, attrValue := range metric {
			dp.Attributes().PutStr(attrKey, attrValue)
		}
	}
	return pm
}

func checkAttributes(t *testing.T, expected, actual pmetric.Metrics) {
	expRMs := expected.ResourceMetrics()
	actualRMs := actual.ResourceMetrics()
	require.Equal(t, expRMs.Len(), actualRMs.Len())
	for i := 0; i < expRMs.Len(); i++ {
		expSMs := expRMs.At(i).ScopeMetrics()
		actualSMs := actualRMs.At(i).ScopeMetrics()
		require.Equal(t, expSMs.Len(), actualSMs.Len())
		for j := 0; j < expSMs.Len(); j++ {
			expMs := expSMs.At(j).Metrics()
			actualMs := actualSMs.At(j).Metrics()
			require.Equal(t, expMs.Len(), actualMs.Len())
			for k := 0; k < expMs.Len(); k++ {
				expM := expMs.At(k)
				actualM := actualMs.At(k)
				require.Equal(t, expM.Type(), actualM.Type())

				expAttrs := getOtelAttributes(expM)
				actualAttrs := getOtelAttributes(actualM)

				require.Equal(t, len(expAttrs), len(actualAttrs))
				for l := 0; l < len(expAttrs); l++ {
					expected := expAttrs[l]
					actual := actualAttrs[l]
					expected.Range(func(k string, v pcommon.Value) bool {
						got, found := actual.Get(k)
						assert.True(t, found)
						assert.Equal(t, v, got)
						return true
					})
				}
			}
		}
	}
}
func TestStartFailWithNoMetadata(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	_, cancel := context.WithCancel(context.Background())
	tagger := &Tagger{
		Config:            cfg,
		logger:            processortest.NewNopCreateSettings().Logger,
		cancelFunc:        cancel,
		metadataProvider:  &mockMetadataProvider{InstanceIdentityDocument: nil},
		volumeSerialCache: &mockVolumeCache{cache: make(map[string]string)},
	}

	err := tagger.Start(context.Background(), componenttest.NewNopHost())
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "No instance identity document")
}

// run Start() and check all tags/volumes are retrieved and saved
func TestStartSuccessWithNoTagsVolumesUpdate(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.RefreshIntervalSeconds = 0 * time.Second
	cfg.EC2MetadataTags = []string{mdKeyInstanceId, mdKeyImageId, mdKeyInstanceType}
	cfg.EC2InstanceTagKeys = []string{tagKey1, tagKey2, "AutoScalingGroupName"}
	cfg.EBSDeviceKeys = []string{device1, device2}
	_, cancel := context.WithCancel(context.Background())
	ec2Client := &mockEC2Client{
		tagsCallCount:    0,
		tagsFailLimit:    0,
		tagsPartialLimit: 1,
		UseUpdatedTags:   false,
	}
	ec2Provider := func(*configaws.CredentialConfig) ec2iface.EC2API {
		return ec2Client
	}
	volumeCache := &mockVolumeCache{cache: make(map[string]string)}

	backoffSleepArray = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	defaultRefreshInterval = 50 * time.Millisecond
	tagger := &Tagger{
		Config:            cfg,
		logger:            processortest.NewNopCreateSettings().Logger,
		cancelFunc:        cancel,
		metadataProvider:  &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc},
		ec2Provider:       ec2Provider,
		volumeSerialCache: volumeCache,
	}
	err := tagger.Start(context.Background(), componenttest.NewNopHost())
	assert.Nil(t, err)
	//assume one second is long enough for the api to be called many times so that all tags/volumes are retrieved
	time.Sleep(time.Second)
	assert.Equal(t, 3, ec2Client.tagsCallCount)
	assert.Equal(t, 2, volumeCache.refreshCount)
	//check tags and volumes
	expectedTags := map[string]string{tagKey1: tagVal1, tagKey2: tagVal2, "AutoScalingGroupName": tagVal3}
	assert.Equal(t, expectedTags, tagger.ec2TagCache)
	assert.Len(t, tagger.volumeSerialCache.Devices(), 2)
	assert.Equal(t, volumeId1, tagger.volumeSerialCache.Serial(device1))
	assert.Equal(t, volumeId2, tagger.volumeSerialCache.Serial(device2))
}

// run Start() and check all tags/volumes are retrieved and saved and then updated
func TestStartSuccessWithTagsVolumesUpdate(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	//use millisecond rather than second to speed up test execution
	cfg.RefreshIntervalSeconds = 20 * time.Millisecond
	cfg.EC2MetadataTags = []string{mdKeyInstanceId, mdKeyImageId, mdKeyInstanceType}
	cfg.EC2InstanceTagKeys = []string{tagKey1, tagKey2, "AutoScalingGroupName"}
	cfg.EBSDeviceKeys = []string{device1, device2}
	_, cancel := context.WithCancel(context.Background())
	ec2Client := &mockEC2Client{
		tagsCallCount:    0,
		tagsFailLimit:    1,
		tagsPartialLimit: 2,
		UseUpdatedTags:   false,
	}
	ec2Provider := func(*configaws.CredentialConfig) ec2iface.EC2API {
		return ec2Client
	}
	volumeCache := &mockVolumeCache{cache: make(map[string]string)}
	backoffSleepArray = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	defaultRefreshInterval = 10 * time.Millisecond

	tagger := &Tagger{
		Config:            cfg,
		logger:            processortest.NewNopCreateSettings().Logger,
		cancelFunc:        cancel,
		metadataProvider:  &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc},
		ec2Provider:       ec2Provider,
		volumeSerialCache: volumeCache,
	}

	err := tagger.Start(context.Background(), componenttest.NewNopHost())
	assert.Nil(t, err)
	//assume one second is long enough for the api to be called many times
	//so that all tags/volumes are retrieved
	time.Sleep(time.Second)
	//check tags and volumes
	expectedTags := map[string]string{tagKey1: tagVal1, tagKey2: tagVal2, "AutoScalingGroupName": tagVal3}
	assert.Equal(t, expectedTags, tagger.ec2TagCache)
	assert.Len(t, tagger.volumeSerialCache.Devices(), 2)
	assert.Equal(t, volumeId1, tagger.volumeSerialCache.Serial(device1))
	assert.Equal(t, volumeId2, tagger.volumeSerialCache.Serial(device2))

	//update the tags and volumes
	ec2Client.UseUpdatedTags = true
	volumeCache.UseUpdatedVolumes = true
	//assume one second is long enough for the api to be called many times
	//so that all tags/volumes are updated
	time.Sleep(time.Second)
	expectedTags = map[string]string{tagKey1: tagVal1, tagKey2: updatedTagVal2, "AutoScalingGroupName": tagVal3}
	assert.Equal(t, expectedTags, tagger.ec2TagCache)
	assert.Len(t, tagger.volumeSerialCache.Devices(), 2)
	assert.Equal(t, volumeId1, tagger.volumeSerialCache.Serial(device1))
	assert.Equal(t, volumeId2Updated, tagger.volumeSerialCache.Serial(device2))
}

// run Start() with ec2_instance_tag_keys = ["*"] and ebs_device_keys = ["*"]
// check there is no attempt to fetch all tags/volumes
func TestStartSuccessWithWildcardTagVolumeKey(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.RefreshIntervalSeconds = 0 * time.Second
	cfg.EC2MetadataTags = []string{mdKeyInstanceId, mdKeyImageId, mdKeyInstanceType}
	cfg.EC2InstanceTagKeys = []string{"*"}
	cfg.EBSDeviceKeys = []string{"*"}
	_, cancel := context.WithCancel(context.Background())
	ec2Client := &mockEC2Client{
		tagsCallCount:    0,
		tagsFailLimit:    0,
		tagsPartialLimit: 1,
		UseUpdatedTags:   false,
	}
	ec2Provider := func(*configaws.CredentialConfig) ec2iface.EC2API {
		return ec2Client
	}
	volumeCache := &mockVolumeCache{cache: make(map[string]string)}
	backoffSleepArray = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	defaultRefreshInterval = 50 * time.Millisecond
	tagger := &Tagger{
		Config:            cfg,
		logger:            processortest.NewNopCreateSettings().Logger,
		cancelFunc:        cancel,
		metadataProvider:  &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc},
		ec2Provider:       ec2Provider,
		volumeSerialCache: volumeCache,
	}

	err := tagger.Start(context.Background(), componenttest.NewNopHost())
	assert.Nil(t, err)
	//assume one second is long enough for the api to be called many times (potentially)
	time.Sleep(time.Second)
	//check only partial tags/volumes are returned
	assert.Equal(t, 2, ec2Client.tagsCallCount)
	assert.Equal(t, 1, volumeCache.refreshCount)
	//check partial tags/volumes are saved
	expectedTags := map[string]string{tagKey1: tagVal1}
	assert.Equal(t, expectedTags, tagger.ec2TagCache)
	assert.Len(t, tagger.volumeSerialCache.Devices(), 1)
	assert.Equal(t, volumeId1, tagger.volumeSerialCache.Serial(device1))
}

// run Start() and then processMetrics and check the output metrics contain expected tags
func TestApplyWithTagsVolumesUpdate(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	//use millisecond rather than second to speed up test execution
	cfg.RefreshIntervalSeconds = 20 * time.Millisecond
	cfg.EC2MetadataTags = []string{mdKeyInstanceId, mdKeyImageId, mdKeyInstanceType}
	cfg.EC2InstanceTagKeys = []string{tagKey1, tagKey2, "AutoScalingGroupName"}
	cfg.EBSDeviceKeys = []string{device1, device2}
	cfg.DiskDeviceTagKey = "device"
	_, cancel := context.WithCancel(context.Background())
	ec2Client := &mockEC2Client{
		tagsCallCount:    0,
		tagsFailLimit:    0,
		tagsPartialLimit: 1,
		UseUpdatedTags:   false,
	}
	ec2Provider := func(*configaws.CredentialConfig) ec2iface.EC2API {
		return ec2Client
	}
	volumeCache := &mockVolumeCache{cache: make(map[string]string)}
	backoffSleepArray = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	defaultRefreshInterval = 50 * time.Millisecond
	tagger := &Tagger{
		Config:            cfg,
		logger:            processortest.NewNopCreateSettings().Logger,
		cancelFunc:        cancel,
		metadataProvider:  &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc},
		ec2Provider:       ec2Provider,
		volumeSerialCache: volumeCache,
	}
	err := tagger.Start(context.Background(), componenttest.NewNopHost())
	assert.Nil(t, err)

	//assume one second is long enough for the api to be called many times
	//so that all tags/volumes are retrieved
	time.Sleep(time.Second)
	md := createTestMetrics([]map[string]string{
		map[string]string{
			"host": "example.org",
		},
		map[string]string{
			"device": device2,
		},
	})
	output, err := tagger.processMetrics(context.Background(), md)
	assert.Nil(t, err)
	expectedOutput := createTestMetrics([]map[string]string{
		map[string]string{
			"AutoScalingGroupName": tagVal3,
			"InstanceId":           "i-01d2417c27a396e44",
			"InstanceType":         "m5ad.large",
			tagKey1:                tagVal1,
			tagKey2:                tagVal2,
		},
		map[string]string{
			"AutoScalingGroupName": tagVal3,
			"VolumeId":             volumeId2,
			"InstanceId":           "i-01d2417c27a396e44",
			"InstanceType":         "m5ad.large",
			tagKey1:                tagVal1,
			tagKey2:                tagVal2,
			"device":               device2,
		},
	})
	checkAttributes(t, expectedOutput, output)

	//update tags and volumes and check metrics are updated as well
	ec2Client.UseUpdatedTags = true
	volumeCache.UseUpdatedVolumes = true
	//assume one second is long enough for the api to be called many times
	//so that all tags/volumes are updated
	time.Sleep(time.Second)
	updatedOutput, err := tagger.processMetrics(context.Background(), md)
	assert.Nil(t, err)
	expectedUpdatedOutput := createTestMetrics([]map[string]string{
		map[string]string{
			"AutoScalingGroupName": tagVal3,
			"InstanceId":           "i-01d2417c27a396e44",
			"InstanceType":         "m5ad.large",
			tagKey1:                tagVal1,
			tagKey2:                updatedTagVal2,
		},
		map[string]string{
			"AutoScalingGroupName": tagVal3,
			"VolumeId":             volumeId2Updated,
			"InstanceId":           "i-01d2417c27a396e44",
			"InstanceType":         "m5ad.large",
			tagKey1:                tagVal1,
			tagKey2:                updatedTagVal2,
			"device":               device2,
		},
	})
	checkAttributes(t, expectedUpdatedOutput, updatedOutput)
}

// Test metrics are dropped before the initial retrieval is done
func TestMetricsDroppedBeforeStarted(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.RefreshIntervalSeconds = 0 * time.Millisecond
	cfg.EC2MetadataTags = []string{mdKeyInstanceId, mdKeyImageId, mdKeyInstanceType}
	cfg.EC2InstanceTagKeys = []string{"*"}
	cfg.EBSDeviceKeys = []string{"*"}
	_, cancel := context.WithCancel(context.Background())
	ec2Client := &mockEC2Client{
		tagsCallCount:    0,
		tagsFailLimit:    0,
		tagsPartialLimit: 1,
		UseUpdatedTags:   false,
	}
	ec2Provider := func(*configaws.CredentialConfig) ec2iface.EC2API {
		return ec2Client
	}
	volumeCache := &mockVolumeCache{cache: make(map[string]string)}
	backoffSleepArray = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	defaultRefreshInterval = 50 * time.Millisecond
	tagger := &Tagger{
		Config:            cfg,
		logger:            processortest.NewNopCreateSettings().Logger,
		cancelFunc:        cancel,
		metadataProvider:  &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc},
		ec2Provider:       ec2Provider,
		volumeSerialCache: volumeCache,
	}

	md := createTestMetrics([]map[string]string{
		map[string]string{
			"host": "example.org",
		},
		map[string]string{
			"device": device1,
		},
		map[string]string{
			"device": device2,
		},
	})
	err := tagger.Start(context.Background(), componenttest.NewNopHost())
	assert.Nil(t, err)
	assert.Equal(t, tagger.started, false)

	output, err := tagger.processMetrics(context.Background(), md)
	assert.Nil(t, err)
	assert.Equal(t, 0, output.ResourceMetrics().Len())

	//assume one second is long enough for the api to be called many times (potentially)
	time.Sleep(time.Second)
	//check only partial tags/volumes are returned
	assert.Equal(t, 2, ec2Client.tagsCallCount)

	//check partial tags/volumes are saved
	expectedTags := map[string]string{tagKey1: tagVal1}
	assert.Equal(t, expectedTags, tagger.ec2TagCache)
	assert.Len(t, tagger.volumeSerialCache.Devices(), 1)
	assert.Equal(t, volumeId1, tagger.volumeSerialCache.Serial(device1))

	assert.Equal(t, tagger.started, true)
	output, err = tagger.processMetrics(context.Background(), md)
	assert.Nil(t, err)
	assert.Equal(t, 3, output.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
}

// Test ec2tagger Start does not block for a long time
func TestTaggerStartDoesNotBlock(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.RefreshIntervalSeconds = 0 * time.Second
	cfg.EC2MetadataTags = []string{mdKeyInstanceId, mdKeyImageId, mdKeyInstanceType}
	cfg.EC2InstanceTagKeys = []string{"*"}
	cfg.EBSDeviceKeys = []string{"*"}
	_, cancel := context.WithCancel(context.Background())
	ec2Client := &mockEC2Client{
		tagsCallCount:    0,
		tagsFailLimit:    0,
		tagsPartialLimit: 1,
		UseUpdatedTags:   false,
	}
	ec2Provider := func(*configaws.CredentialConfig) ec2iface.EC2API {
		return ec2Client
	}
	backoffSleepArray = []time.Duration{1 * time.Minute, 1 * time.Minute, 1 * time.Minute, 3 * time.Minute, 3 * time.Minute, 3 * time.Minute, 10 * time.Minute}
	defaultRefreshInterval = 180 * time.Second
	tagger := &Tagger{
		Config:            cfg,
		logger:            processortest.NewNopCreateSettings().Logger,
		cancelFunc:        cancel,
		metadataProvider:  &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc},
		ec2Provider:       ec2Provider,
		volumeSerialCache: &mockVolumeCache{cache: make(map[string]string)},
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
	err := tagger.Start(context.Background(), componenttest.NewNopHost())
	assert.Nil(t, err)
	assert.Equal(t, tagger.started, false)
	close(inited)
}

// Test ec2tagger Start does not block for a long time
func TestTaggerStartsWithoutTagOrVolume(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.RefreshIntervalSeconds = 0 * time.Second
	cfg.EC2MetadataTags = []string{mdKeyInstanceId, mdKeyImageId, mdKeyInstanceType}
	_, cancel := context.WithCancel(context.Background())

	tagger := &Tagger{
		Config:            cfg,
		logger:            processortest.NewNopCreateSettings().Logger,
		cancelFunc:        cancel,
		metadataProvider:  &mockMetadataProvider{InstanceIdentityDocument: mockedInstanceIdentityDoc},
		volumeSerialCache: &mockVolumeCache{cache: make(map[string]string)},
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
	err := tagger.Start(context.Background(), componenttest.NewNopHost())
	assert.Nil(t, err)
	assert.Equal(t, tagger.started, true)
	close(inited)
}
