// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
)

const (
	High_Resolution_Tag_Key      = "aws:StorageResolution"
	Aggregation_Interval_Tag_Key = "aws:AggregationInterval"

	// EC2 metadata keys (matching ec2tagger constants)
	mdKeyInstanceId   = "InstanceId"
	mdKeyImageId      = "ImageId"
	mdKeyInstanceType = "InstanceType"
)

var ReservedTagKeySet = collections.NewSet[string](High_Resolution_Tag_Key, Aggregation_Interval_Tag_Key, ec2tagger.AttributeVolumeId)

func AddHighResolutionTag(tags interface{}) {
	tagMap := tags.(map[string]interface{})
	tagMap[High_Resolution_Tag_Key] = "true"
}

// FilterReservedKeys out reserved tag keys and resolves AWS metadata variables at translation time
func FilterReservedKeys(input any) any {
	result := map[string]any{}
	for k, v := range input.(map[string]interface{}) {
		if !ReservedTagKeySet.Contains(k) {
			// Resolve AWS metadata variables at translation time
			if vStr, ok := v.(string); ok && strings.HasPrefix(vStr, "${aws:") {
				resolvedValue := resolveAWSMetadata(vStr)
				if resolvedValue != "" {
					result[k] = resolvedValue
				}
				// If resolution fails, skip the dimension rather than using the literal value
			} else {
				result[k] = v
			}
		}
	}
	return result
}

// resolveAWSMetadata resolves AWS metadata variables like ${aws:TagKey} to actual values from EC2 tags
func resolveAWSMetadata(variable string) string {
	// Extract the tag key from ${aws:Key}
	if !strings.HasPrefix(variable, "${aws:") || !strings.HasSuffix(variable, "}") {
		return ""
	}

	tagKey := strings.TrimSuffix(strings.TrimPrefix(variable, "${aws:"), "}")

	doc, err := getEC2Metadata()
	if err != nil {
		log.Printf("Failed to get EC2 metadata during translation: %v", err)
		return ""
	}

	// Handle special metadata keys that come from instance metadata (not tags)
	// Use same constants as ec2tagger
	switch tagKey {
	case mdKeyInstanceId:
		return doc.InstanceID
	case mdKeyInstanceType:
		return doc.InstanceType
	case mdKeyImageId:
		return doc.ImageID
	default:
		// For any other key, look it up as an EC2 tag
		return getEC2TagValue(doc.InstanceID, doc.Region, tagKey)
	}
}

// getEC2TagValue retrieves any EC2 tag value by key
var getEC2TagValue = func(instanceID, region, tagKey string) string {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		log.Printf("Failed to create AWS session: %v", err)
		return ""
	}

	ec2Client := ec2.New(sess)

	// Query EC2 tags for the specified key
	input := &ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("resource-type"),
				Values: []*string{aws.String("instance")},
			},
			{
				Name:   aws.String("resource-id"),
				Values: []*string{aws.String(instanceID)},
			},
			{
				Name:   aws.String("key"),
				Values: []*string{aws.String(tagKey)},
			},
		},
	}

	result, err := ec2Client.DescribeTags(input)
	if err != nil {
		log.Printf("Failed to describe EC2 tags for key '%s': %v", tagKey, err)
		return ""
	}

	if len(result.Tags) > 0 && result.Tags[0].Value != nil {
		return *result.Tags[0].Value
	}

	log.Printf("EC2 tag '%s' not found on instance %s", tagKey, instanceID)
	return ""
}

// getEC2Metadata retrieves EC2 metadata using the same approach as ec2tagger
var getEC2Metadata = func() (ec2metadata.EC2InstanceIdentityDocument, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mdCredentialConfig := &configaws.CredentialConfig{}
	metadataProvider := ec2metadataprovider.NewMetadataProvider(mdCredentialConfig.Credentials(), 3)

	return metadataProvider.Get(ctx)
}
