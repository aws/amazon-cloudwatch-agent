// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/metadata/host"
)

const (
	// The ID of the instance.
	filterKeyInstanceID = "instance-id"
	// The private IPv4 address of the instance.
	filterKeyPrivateIpAddress = "private-ip-address"
	prefixInstanceID          = "i-"
	prefixPrivateIpAddress    = "ip-"
	suffixDefault             = ".ec2.internal"
	suffixRegional            = ".compute.internal"
)

var (
	errUnsupportedHostname = errors.New("unable to parse non-fixed format hostname")
	errUnsupportedFilter   = errors.New("unable to determine EC2 filter")
	errReservationCount    = errors.New("invalid number of reservations found")
	errInstanceCount       = errors.New("invalid number of instances found")
)

type ec2ClientProvider func(client.ConfigProvider, ...*aws.Config) ec2iface.EC2API

type describeInstancesMetadataProvider struct {
	configProvider       client.ConfigProvider
	hostMetadataProvider host.MetadataProvider
	newEC2Client         ec2ClientProvider
}

var _ MetadataProvider = (*describeInstancesMetadataProvider)(nil)

func newDescribeInstancesMetadataProvider(configProvider client.ConfigProvider) *describeInstancesMetadataProvider {
	return &describeInstancesMetadataProvider{
		configProvider:       configProvider,
		hostMetadataProvider: host.NewMetadataProvider(),
		newEC2Client: func(provider client.ConfigProvider, configs ...*aws.Config) ec2iface.EC2API {
			return ec2.New(provider, configs...)
		},
	}
}

func (p *describeInstancesMetadataProvider) ID() string {
	return "DescribeInstances"
}

func (p *describeInstancesMetadataProvider) Get(ctx context.Context) (*Metadata, error) {
	filter, region, hostnameErr := p.filterFromHostname(ctx)
	if hostnameErr != nil {
		var ipErr error
		filter, ipErr = p.filterFromHostIP()
		if ipErr != nil {
			return nil, errors.Join(
				fmt.Errorf("%w from hostname: %w", errUnsupportedFilter, hostnameErr),
				fmt.Errorf("%w from host IP: %w", errUnsupportedFilter, ipErr),
			)
		}
	}
	input := &ec2.DescribeInstancesInput{Filters: []*ec2.Filter{filter}}
	cfg := &aws.Config{
		LogLevel:                      configaws.SDKLogLevel(),
		Logger:                        configaws.SDKLogger{},
		CredentialsChainVerboseErrors: aws.Bool(true),
	}
	if region != "" {
		cfg = cfg.WithRegion(region)
	}
	svc := p.newEC2Client(p.configProvider, cfg)
	output, err := svc.DescribeInstances(input)
	if err != nil {
		return nil, err
	}
	reservationCount := len(output.Reservations)
	if reservationCount == 0 || reservationCount > 1 {
		return nil, fmt.Errorf("%w: %v", errReservationCount, reservationCount)
	}
	metadata, err := fromReservation(*output.Reservations[0])
	if err != nil {
		return nil, err
	}
	metadata.Region = region
	if metadata.Region == "" {
		metadata.Region = getRegionFromAZ(metadata.AvailabilityZone)
	}
	return metadata, nil
}

func (p *describeInstancesMetadataProvider) Hostname(context.Context) (string, error) {
	return p.hostMetadataProvider.Hostname()
}

func (p *describeInstancesMetadataProvider) filterFromHostname(ctx context.Context) (*ec2.Filter, string, error) {
	hostname, err := p.Hostname(ctx)
	if err != nil {
		return nil, "", err
	}
	prefix, region, err := splitHostname(hostname)
	if region == "" {
		return nil, "", err
	}
	filter, err := filterFromHostnamePrefix(prefix)
	if err != nil {
		return nil, "", err
	}
	return filter, region, nil
}

func (p *describeInstancesMetadataProvider) filterFromHostIP() (*ec2.Filter, error) {
	hostIP, err := p.hostMetadataProvider.HostIP()
	if err != nil {
		return nil, err
	}
	return &ec2.Filter{
		Name:   aws.String(filterKeyPrivateIpAddress),
		Values: aws.StringSlice([]string{hostIP}),
	}, nil
}

func filterFromHostnamePrefix(prefix string) (*ec2.Filter, error) {
	// i-0123456789abcdef
	if strings.HasPrefix(prefix, prefixInstanceID) {
		return &ec2.Filter{
			Name:   aws.String(filterKeyInstanceID),
			Values: aws.StringSlice([]string{prefix}),
		}, nil
	}
	// ip-10-24-34-0 -> 10.24.34.0
	if ipAddress, ok := strings.CutPrefix(prefix, prefixPrivateIpAddress); ok {
		return &ec2.Filter{
			Name:   aws.String(filterKeyPrivateIpAddress),
			Values: aws.StringSlice([]string{strings.ReplaceAll(ipAddress, "-", ".")}),
		}, nil
	}
	return nil, fmt.Errorf("%w from hostname prefix: %s", errUnsupportedFilter, prefix)
}

// splitHostname extracts the prefix and region based on https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-naming.html
func splitHostname(hostname string) (prefix string, region string, err error) {
	before, ok := strings.CutSuffix(hostname, suffixRegional)
	if ok {
		parts := strings.Split(before, ".")
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}
	before, ok = strings.CutSuffix(hostname, suffixDefault)
	if ok {
		return before, "us-east-1", nil
	}
	return hostname, "", fmt.Errorf("%w: %s", errUnsupportedHostname, hostname)
}

func fromReservation(reservation ec2.Reservation) (*Metadata, error) {
	instanceCount := len(reservation.Instances)
	if instanceCount == 0 || instanceCount > 1 {
		return nil, fmt.Errorf("%w: %v", errInstanceCount, instanceCount)
	}
	instance := reservation.Instances[0]
	metadata := &Metadata{
		AccountID:    aws.StringValue(reservation.OwnerId),
		ImageID:      aws.StringValue(instance.ImageId),
		InstanceID:   aws.StringValue(instance.InstanceId),
		InstanceType: aws.StringValue(instance.InstanceType),
		PrivateIP:    aws.StringValue(instance.PrivateIpAddress),
	}
	if instance.Placement != nil {
		metadata.AvailabilityZone = aws.StringValue(instance.Placement.AvailabilityZone)
	}
	return metadata, nil
}

func getRegionFromAZ(az string) string {
	if az == "" {
		return ""
	}
	return az[:len(az)-1]
}
