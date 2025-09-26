// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/stretchr/testify/assert"
)

// SetupMockMetadataForTesting sets up mock metadata service for testing
// This is a public function that can be called from other packages
func SetupMockMetadataForTesting() func() {
	// Store original functions
	originalGetEC2Metadata := getEC2Metadata
	originalGetEC2TagValue := getEC2TagValue
	
	// Setup mock metadata service with test data
	getEC2Metadata = func() (ec2metadata.EC2InstanceIdentityDocument, error) {
		return ec2metadata.EC2InstanceIdentityDocument{
			InstanceID:   "i-1234567890abcdef0",
			InstanceType: "t3.medium",
			ImageID:      "ami-0abcdef1234567890",
			Region:       "us-west-2",
		}, nil
	}

	getEC2TagValue = func(instanceID, region, tagKey string) string {
		mockTags := map[string]string{
			"AutoScalingGroupName": "production-web-asg",
		}
		return mockTags[tagKey]
	}
	
	// Return cleanup function
	return func() {
		getEC2Metadata = originalGetEC2Metadata
		getEC2TagValue = originalGetEC2TagValue
	}
}


func TestFilterReservedKeys_EC2MetadataResolution(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]any
		mockDoc  *ec2metadata.EC2InstanceIdentityDocument
		mockErr  error
	}{
		{
			name: "EC2 metadata variables resolution",
			input: map[string]interface{}{
				"InstanceType": "${aws:InstanceType}",
				"ImageId":      "${aws:ImageId}",
				"InstanceId":   "${aws:InstanceId}",
			},
			expected: map[string]any{
				"InstanceType": "t3.medium",
				"ImageId":      "ami-0abcdef1234567890",
				"InstanceId":   "i-1234567890abcdef0",
			},
			mockDoc: &ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:   "i-1234567890abcdef0",
				InstanceType: "t3.medium",
				ImageID:      "ami-0abcdef1234567890",
			},
		},
		{
			name: "Hardcoded values pass through",
			input: map[string]interface{}{
				"HardcodedName": "HardcodedValue",
				"Environment":   "production",
			},
			expected: map[string]any{
				"HardcodedName": "HardcodedValue",
				"Environment":   "production",
			},
		},
		{
			name: "Mixed AWS variables and hardcoded values",
			input: map[string]interface{}{
				"InstanceType":  "${aws:InstanceType}",
				"HardcodedName": "HardcodedValue",
			},
			expected: map[string]any{
				"InstanceType":  "t3.medium",
				"HardcodedName": "HardcodedValue",
			},
			mockDoc: &ec2metadata.EC2InstanceIdentityDocument{
				InstanceType: "t3.medium",
			},
		},
		{
			name: "Reserved keys are filtered out",
			input: map[string]interface{}{
				"InstanceType":            "${aws:InstanceType}",
				"aws:StorageResolution":   "true",
				"aws:AggregationInterval": "60",
				"VolumeId":                "vol-123",
				"HardcodedName":           "HardcodedValue",
			},
			expected: map[string]any{
				"InstanceType":  "t3.medium",
				"HardcodedName": "HardcodedValue",
			},
			mockDoc: &ec2metadata.EC2InstanceIdentityDocument{
				InstanceType: "t3.medium",
			},
		},
		{
			name: "Failed metadata resolution skips dimension",
			input: map[string]interface{}{
				"InstanceType":  "${aws:InstanceType}",
				"HardcodedName": "HardcodedValue",
			},
			expected: map[string]any{
				"HardcodedName": "HardcodedValue",
			},
			mockErr: errors.New("metadata service unavailable"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the getEC2Metadata function
			originalGetEC2Metadata := getEC2Metadata
			defer func() { getEC2Metadata = originalGetEC2Metadata }()

			getEC2Metadata = func() (ec2metadata.EC2InstanceIdentityDocument, error) {
				if tt.mockErr != nil {
					return ec2metadata.EC2InstanceIdentityDocument{}, tt.mockErr
				}
				if tt.mockDoc != nil {
					return *tt.mockDoc, nil
				}
				return ec2metadata.EC2InstanceIdentityDocument{}, nil
			}

			result := FilterReservedKeys(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterReservedKeys_EC2TagResolution(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		expected    map[string]any
		mockDoc     *ec2metadata.EC2InstanceIdentityDocument
		mockTags    map[string]string
		mockTagsErr error
	}{
		{
			name: "AutoScaling group name from EC2 tag",
			input: map[string]interface{}{
				"AutoScalingGroupName": "${aws:AutoScalingGroupName}",
			},
			expected: map[string]any{
				"AutoScalingGroupName": "my-asg-group",
			},
			mockDoc: &ec2metadata.EC2InstanceIdentityDocument{},
			mockTags: map[string]string{
				"AutoScalingGroupName": "my-asg-group",
			},
		},
		{
			name: "Custom EC2 tag resolution",
			input: map[string]interface{}{
				"Environment": "${aws:Environment}",
				"Team":        "${aws:Team}",
			},
			expected: map[string]any{
				"Environment": "production",
				"Team":        "backend",
			},
			mockDoc: &ec2metadata.EC2InstanceIdentityDocument{},
			mockTags: map[string]string{
				"Environment": "production",
				"Team":        "backend",
			},
		},
		{
			name: "Missing EC2 tag skips dimension",
			input: map[string]interface{}{
				"MissingTag":    "${aws:MissingTag}",
				"HardcodedName": "HardcodedValue",
			},
			expected: map[string]any{
				"HardcodedName": "HardcodedValue",
			},
			mockDoc:  &ec2metadata.EC2InstanceIdentityDocument{},
			mockTags: map[string]string{}, // Empty tags
		},
		{
			name: "EC2 API error skips dimension",
			input: map[string]interface{}{
				"Environment":   "${aws:Environment}",
				"HardcodedName": "HardcodedValue",
			},
			expected: map[string]any{
				"HardcodedName": "HardcodedValue",
			},
			mockDoc:     &ec2metadata.EC2InstanceIdentityDocument{},
			mockTagsErr: errors.New("EC2 API error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the getEC2Metadata function
			originalGetEC2Metadata := getEC2Metadata
			defer func() { getEC2Metadata = originalGetEC2Metadata }()

			getEC2Metadata = func() (ec2metadata.EC2InstanceIdentityDocument, error) {
				if tt.mockDoc != nil {
					return *tt.mockDoc, nil
				}
				return ec2metadata.EC2InstanceIdentityDocument{}, nil
			}

			// Mock the getEC2TagValue function
			originalGetEC2TagValue := getEC2TagValue
			defer func() { getEC2TagValue = originalGetEC2TagValue }()

			getEC2TagValue = func(instanceID, region, tagKey string) string {
				if tt.mockTagsErr != nil {
					return ""
				}
				if value, exists := tt.mockTags[tagKey]; exists {
					return value
				}
				// Handle AutoScaling group name mapping
				if tagKey == "AutoScalingGroupName" {
					if value, exists := tt.mockTags["aws:autoscaling:groupName"]; exists {
						return value
					}
				}
				return ""
			}

			result := FilterReservedKeys(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveAWSMetadata(t *testing.T) {
	tests := []struct {
		name     string
		variable string
		expected string
		mockDoc  *ec2metadata.EC2InstanceIdentityDocument
		mockTags map[string]string
		mockErr  error
	}{
		{
			name:     "Valid InstanceId variable",
			variable: "${aws:InstanceId}",
			expected: "i-1234567890abcdef0",
			mockDoc: &ec2metadata.EC2InstanceIdentityDocument{
				InstanceID: "i-1234567890abcdef0",
			},
		},
		{
			name:     "Valid InstanceType variable",
			variable: "${aws:InstanceType}",
			expected: "t3.medium",
			mockDoc: &ec2metadata.EC2InstanceIdentityDocument{
				InstanceType: "t3.medium",
			},
		},
		{
			name:     "Valid ImageId variable",
			variable: "${aws:ImageId}",
			expected: "ami-0abcdef1234567890",
			mockDoc: &ec2metadata.EC2InstanceIdentityDocument{
				ImageID: "ami-0abcdef1234567890",
			},
		},
		{
			name:     "Invalid variable format",
			variable: "not-aws-variable",
			expected: "",
		},
		{
			name:     "Incomplete variable format",
			variable: "${aws:InstanceType",
			expected: "",
		},
		{
			name:     "Empty variable",
			variable: "",
			expected: "",
		},
		{
			name:     "Custom tag variable",
			variable: "${aws:Environment}",
			expected: "production",
			mockDoc:  &ec2metadata.EC2InstanceIdentityDocument{},
			mockTags: map[string]string{
				"Environment": "production",
			},
		},
		{
			name:     "Metadata service error",
			variable: "${aws:InstanceType}",
			expected: "",
			mockErr:  errors.New("metadata service unavailable"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the getEC2Metadata function
			originalGetEC2Metadata := getEC2Metadata
			defer func() { getEC2Metadata = originalGetEC2Metadata }()

			getEC2Metadata = func() (ec2metadata.EC2InstanceIdentityDocument, error) {
				if tt.mockErr != nil {
					return ec2metadata.EC2InstanceIdentityDocument{}, tt.mockErr
				}
				if tt.mockDoc != nil {
					return *tt.mockDoc, nil
				}
				return ec2metadata.EC2InstanceIdentityDocument{}, nil
			}

			// Mock the getEC2TagValue function
			originalGetEC2TagValue := getEC2TagValue
			defer func() { getEC2TagValue = originalGetEC2TagValue }()

			getEC2TagValue = func(instanceID, region, tagKey string) string {
				if value, exists := tt.mockTags[tagKey]; exists {
					return value
				}
				return ""
			}

			result := resolveAWSMetadata(tt.variable)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Integration test that demonstrates the three main use cases
func TestAppendDimensionsIntegration(t *testing.T) {
	// Mock the getEC2Metadata function
	originalGetEC2Metadata := getEC2Metadata
	defer func() { getEC2Metadata = originalGetEC2Metadata }()

	getEC2Metadata = func() (ec2metadata.EC2InstanceIdentityDocument, error) {
		return ec2metadata.EC2InstanceIdentityDocument{
			InstanceID:   "i-1234567890abcdef0",
			InstanceType: "t3.medium",
			ImageID:      "ami-0abcdef1234567890",
		}, nil
	}

	// Mock the getEC2TagValue function
	originalGetEC2TagValue := getEC2TagValue
	defer func() { getEC2TagValue = originalGetEC2TagValue }()

	getEC2TagValue = func(instanceID, region, tagKey string) string {
		tags := map[string]string{
			"AutoScalingGroupName": "my-asg-group",
			"Environment":          "production",
		}
		return tags[tagKey]
	}

	t.Run("Case 1: EC2 metadata substitution", func(t *testing.T) {
		input := map[string]interface{}{
			"InstanceType": "${aws:InstanceType}",
			"ImageId":      "${aws:ImageId}",
			"InstanceId":   "${aws:InstanceId}",
		}

		expected := map[string]any{
			"InstanceType": "t3.medium",
			"ImageId":      "ami-0abcdef1234567890",
			"InstanceId":   "i-1234567890abcdef0",
		}

		result := FilterReservedKeys(input)
		assert.Equal(t, expected, result)
	})

	t.Run("Case 2: AutoScaling group name from EC2 tag", func(t *testing.T) {
		input := map[string]interface{}{
			"AutoScalingGroupName": "${aws:AutoScalingGroupName}",
		}

		expected := map[string]any{
			"AutoScalingGroupName": "my-asg-group",
		}

		result := FilterReservedKeys(input)
		assert.Equal(t, expected, result)
	})

	t.Run("Case 3: Hardcoded values pass through", func(t *testing.T) {
		input := map[string]interface{}{
			"HardcodedName": "HardcodedValue",
		}

		expected := map[string]any{
			"HardcodedName": "HardcodedValue",
		}

		result := FilterReservedKeys(input)
		assert.Equal(t, expected, result)
	})

	t.Run("Case 4: Mixed scenarios", func(t *testing.T) {
		input := map[string]interface{}{
			"InstanceType":         "${aws:InstanceType}",
			"AutoScalingGroupName": "${aws:AutoScalingGroupName}",
			"HardcodedName":        "HardcodedValue",
			"Environment":          "${aws:Environment}",
		}

		expected := map[string]any{
			"InstanceType":         "t3.medium",
			"AutoScalingGroupName": "my-asg-group",
			"HardcodedName":        "HardcodedValue",
			"Environment":          "production",
		}

		result := FilterReservedKeys(input)
		assert.Equal(t, expected, result)
	})
}
