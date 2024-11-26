// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

const (
	bjsPartition          = "aws-cn"
	pdtPartition          = "aws-us-gov"
	lckPartition          = "aws-iso-b"
	dcaPartition          = "aws-iso"
	classicFallbackRegion = "us-east-1"
	bjsFallbackRegion     = "cn-north-1"
	pdtFallbackRegion     = "us-gov-west-1"
	lckFallbackRegion     = "us-isob-east-1"
	dcaFallbackRegion     = "us-iso-east-1"
)

type CredentialConfig struct {
	Region    string
	AccessKey string
	SecretKey string
	RoleARN   string
	Profile   string
	Filename  string
	Token     string
}

type stsCredentialProvider struct {
	regional, partitional, fallbackProvider *stscreds.AssumeRoleProvider
}

func (s *stsCredentialProvider) IsExpired() bool {
	if s.fallbackProvider != nil {
		return s.fallbackProvider.IsExpired()
	}
	return s.regional.IsExpired()
}

type RootCredentialsProvider struct {
	Name        func() string
	Credentials func(*CredentialConfig) *credentials.Credentials
}

var credentialsChain = make([]RootCredentialsProvider, 0)

func getRootCredentialsFromChain(c *CredentialConfig) *credentials.Credentials {
	for _, provider := range credentialsChain {
		if creds := provider.Credentials(c); creds != nil {
			return creds
		}
	}
	return nil
}

func GetDefaultCredentialsChain() []RootCredentialsProvider {
	return credentialsChain
}

func OverwriteCredentialsChain(providers ...RootCredentialsProvider) {
	credentialsChain = providers
}

func getSession(config *aws.Config) *session.Session {
	cfgFiles := getFallbackSharedConfigFiles(backwardsCompatibleUserHomeDir)
	log.Printf("D! Fallback shared config file(s): %v", cfgFiles)
	ses, err := session.NewSessionWithOptions(session.Options{
		Config:            *config,
		SharedConfigFiles: cfgFiles,
	})
	if err != nil {
		log.Printf("E! Failed to create credential sessions, retrying in 15s, error was '%s' \n", err)
		time.Sleep(15 * time.Second)
		ses, err = session.NewSessionWithOptions(session.Options{
			Config:            *config,
			SharedConfigFiles: cfgFiles,
		})
		if err != nil {
			log.Printf("E! Retry failed for creating credential sessions, error was '%s' \n", err)
			return ses
		}
	}
	log.Printf("D! Successfully created credential sessions\n")
	cred, err := ses.Config.Credentials.Get()
	if err != nil {
		log.Printf("E! Failed to get credential from session: %v", err)
	} else {
		log.Printf("D! Using credential %s from %s", cred.AccessKeyID, cred.ProviderName)
	}
	if cred.ProviderName == ec2rolecreds.ProviderName {
		var found []string
		cfgFiles = getFallbackSharedConfigFiles(currentUserHomeDir)
		for _, cfgFile := range cfgFiles {
			if _, err = os.Stat(cfgFile); err == nil {
				found = append(found, cfgFile)
			}
		}
		if len(found) > 0 {
			log.Printf("W! Unused shared config file(s) found: %v. If you would like to use them, "+
				"please update your common-config.toml.", found)
			agent.UsageFlags().Set(agent.FlagSharedConfigFallback)
		}
	}
	return ses
}

func (c *CredentialConfig) rootCredentials() client.ConfigProvider {
	config := &aws.Config{
		Region:                        aws.String(c.Region),
		CredentialsChainVerboseErrors: aws.Bool(true),
		HTTPClient:                    &http.Client{Timeout: 1 * time.Minute},
		LogLevel:                      SDKLogLevel(),
		Logger:                        SDKLogger{},
	}
	config.Credentials = getRootCredentialsFromChain(c)
	return getSession(config)
}

func (c *CredentialConfig) assumeCredentials() client.ConfigProvider {
	rootCredentials := c.rootCredentials()
	config := &aws.Config{
		Region:     aws.String(c.Region),
		HTTPClient: &http.Client{Timeout: 1 * time.Minute},
		LogLevel:   SDKLogLevel(),
		Logger:     SDKLogger{},
	}
	config.Credentials = newStsCredentials(rootCredentials, c.RoleARN, c.Region)
	return getSession(config)
}

func (c *CredentialConfig) Credentials() client.ConfigProvider {
	if c.RoleARN != "" {
		return c.assumeCredentials()
	} else {
		return c.rootCredentials()
	}
}

func (s *stsCredentialProvider) Retrieve() (credentials.Value, error) {
	if s.fallbackProvider != nil {
		return s.fallbackProvider.Retrieve()
	}

	v, err := s.regional.Retrieve()

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == sts.ErrCodeRegionDisabledException {
			log.Printf("D! The regional STS endpoint is deactivated and going to fall back to partitional STS endpoint\n")
			s.fallbackProvider = s.partitional
			return s.partitional.Retrieve()
		}
	}

	return v, err
}

func newStsCredentials(c client.ConfigProvider, roleARN string, region string) *credentials.Credentials {
	regional := &stscreds.AssumeRoleProvider{
		Client: sts.New(c, &aws.Config{
			Region:              aws.String(region),
			STSRegionalEndpoint: endpoints.RegionalSTSEndpoint,
			HTTPClient:          &http.Client{Timeout: 1 * time.Minute},
			LogLevel:            SDKLogLevel(),
			Logger:              SDKLogger{},
		}),
		RoleARN:  roleARN,
		Duration: stscreds.DefaultDuration,
	}

	fallbackRegion := getFallbackRegion(region)

	partitional := &stscreds.AssumeRoleProvider{
		Client: sts.New(c, &aws.Config{
			Region:              aws.String(fallbackRegion),
			Endpoint:            aws.String(getFallbackEndpoint(fallbackRegion)),
			STSRegionalEndpoint: endpoints.RegionalSTSEndpoint,
			HTTPClient:          &http.Client{Timeout: 1 * time.Minute},
			LogLevel:            SDKLogLevel(),
			Logger:              SDKLogger{},
		}),
		RoleARN:  roleARN,
		Duration: stscreds.DefaultDuration,
	}

	return credentials.NewCredentials(&stsCredentialProvider{regional: regional, partitional: partitional})
}

// The partitional STS endpoint used to fallback when regional STS endpoint is not activated.
func getFallbackEndpoint(region string) string {
	partition := getPartition(region)
	endpoint, _ := partition.EndpointFor("sts", region)
	log.Printf("D! STS partitional endpoint retrieved: %s", endpoint.URL)
	return endpoint.URL
}

// Get the region in the partition where STS endpoint cannot be deactivated by customers which is used to fallback.
// NOTE: Some Regions are not enabled by default, such as the Asia Pacific Hong Kong Region. In that case, when you
// manually enable the Region, the regional STS endpoints will always be activated and cannot be deactivated.
// Refer to: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_enable-regions.html
func getFallbackRegion(region string) string {
	partition := getPartition(region)
	switch partition.ID() {
	case bjsPartition:
		return bjsFallbackRegion
	case pdtPartition:
		return pdtFallbackRegion
	case dcaPartition:
		return dcaFallbackRegion
	case lckPartition:
		return lckFallbackRegion
	default:
		return classicFallbackRegion
	}
}

// Get the partition information based on the region name
func getPartition(region string) endpoints.Partition {
	partition, _ := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region)
	return partition
}

func init() {
	//Initialize the default root credentials chain
	staticCredentialsProvider := RootCredentialsProvider{
		Name: func() string {
			return "StaticCredentialsProvider"
		},
		Credentials: func(c *CredentialConfig) *credentials.Credentials {
			if c.AccessKey != "" || c.SecretKey != "" {
				return credentials.NewStaticCredentials(c.AccessKey, c.SecretKey, c.Token)
			}
			return nil
		},
	}
	refreshableCredentialsProvider := RootCredentialsProvider{
		Name: func() string {
			return "RefreshableCredentialsProvider"
		},
		Credentials: func(c *CredentialConfig) *credentials.Credentials {
			if c.Profile != "" || c.Filename != "" {
				log.Printf("I! will use file based credentials provider ")
				return credentials.NewCredentials(&Refreshable_shared_credentials_provider{
					sharedCredentialsProvider: &credentials.SharedCredentialsProvider{
						Filename: c.Filename,
						Profile:  c.Profile,
					},
					ExpiryWindow: 10 * time.Minute,
				})
			}
			return nil
		},
	}
	credentialsChain = append(credentialsChain, staticCredentialsProvider, refreshableCredentialsProvider)

	//You can overwrite the default credentials chain by first importing the current file
	//and then calling OverwriteCredentialsChain() with your own credentials chain
}
