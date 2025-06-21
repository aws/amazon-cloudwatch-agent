package debugger

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
)

type InstanceInfo struct {
	InstanceID string `json:"instanceId"`
	AccountID string `json:"accountId"`
	Region string `json:"region"`
	InstanceType string `json:"InstanceType"`
	ImageID string `json:"imageId"`
	AvailabilityZone string `json:"availabilityZone"`
	Architecture string `json:"architecture"`
}

func GetInstanceInfo(ctx context.Context) (*InstanceInfo, error) {
	sess := session.Must(session.NewSession())
	provider := ec2metadataprovider.NewMetadataProvider(sess, 0)

	doc, err := provider.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to get instance identity document: %w", err)
	}

	return &InstanceInfo{
		InstanceID: doc.InstanceID,
		AccountID: doc.AccountID,
		Region: doc.Region,
		InstanceType: doc.InstanceType,
		ImageID: doc.ImageID,
		AvailabilityZone: doc.AvailabilityZone,
		Architecture: doc.Architecture,
	}, nil
}