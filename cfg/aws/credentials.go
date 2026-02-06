// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

type CredentialsConfig struct {
	Region    string
	AccessKey string
	SecretKey string
	RoleARN   string
	Profile   string
	Filename  string
	Token     string
}

func (c *CredentialsConfig) LoadConfig(ctx context.Context) (aws.Config, error) {
	if c.RoleARN != "" {
		return c.assumeRoleConfig(ctx)
	}
	return c.rootConfig(ctx)
}

func (c *CredentialsConfig) assumeRoleConfig(ctx context.Context) (aws.Config, error) {
	cfg, err := c.rootConfig(ctx)
	if err != nil {
		return aws.Config{}, err
	}
	return c.loadConfig(ctx, aws.NewCredentialsCache(newStsCredentialsProvider(cfg, c.RoleARN, c.Region)))
}

func (c *CredentialsConfig) rootConfig(ctx context.Context) (aws.Config, error) {
	return c.loadConfig(ctx, c.fromChain())
}

func (c *CredentialsConfig) loadConfig(ctx context.Context, provider aws.CredentialsProvider) (aws.Config, error) {
	cfgFiles := getFallbackSharedConfigFiles(backwardsCompatibleUserHomeDir)
	log.Printf("D! Fallback shared config file(s): %v", cfgFiles)
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(c.Region),
		config.WithHTTPClient(getSharedHTTPClient()),
		config.WithClientLogMode(SDKLogLevel()),
		config.WithLogger(SDKLogger{}),
		config.WithSharedCredentialsFiles(cfgFiles),
	}
	if provider != nil {
		opts = append(opts, config.WithCredentialsProvider(provider))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		log.Printf("E! Failed to create credential sessions, retrying in 15s, error was '%s'", err)
		time.Sleep(15 * time.Second)
		cfg, err = config.LoadDefaultConfig(ctx, opts...)
		if err != nil {
			log.Printf("E! Retry failed for creating credential sessions, error was '%s'", err)
			return aws.Config{}, err
		}
	}
	log.Println("D! Successfully created credential sessions")
	cred, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		log.Printf("E! Failed to get credential from session: %v", err)
	} else {
		log.Printf("D! Using credential %s from %s", cred.AccessKeyID, cred.Source)
	}
	if cred.Source == ec2rolecreds.ProviderName {
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
	return cfg, nil
}

func (c *CredentialsConfig) fromChain() aws.CredentialsProvider {
	for _, provider := range CredentialsChain() {
		if p := provider.Provider(c); p != nil {
			return p
		}
	}
	return nil
}

type CredentialsProvider struct {
	Name     func() string
	Provider func(*CredentialsConfig) aws.CredentialsProvider
}

var credentialsChain []CredentialsProvider

func CredentialsChain() []CredentialsProvider {
	return credentialsChain
}

func OverwriteCredentialsChain(providers ...CredentialsProvider) {
	credentialsChain = providers
}

func init() {
	// Initialize the default root credentials chain
	staticCredentialsProvider := CredentialsProvider{
		Name: func() string { return "StaticCredentialsProvider" },
		Provider: func(c *CredentialsConfig) aws.CredentialsProvider {
			if c.AccessKey != "" && c.SecretKey != "" {
				return aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, c.Token))
			}
			return nil
		},
	}
	refreshableCredentialsProvider := CredentialsProvider{
		Name: func() string { return "RefreshableCredentialsProvider" },
		Provider: func(c *CredentialsConfig) aws.CredentialsProvider {
			if c.Profile != "" || c.Filename != "" {
				log.Printf("I! will use file based credentials provider")
				return aws.NewCredentialsCache(RefreshableSharedCredentialsProvider{
					Provider: SharedCredentialsProvider{
						Filename: c.Filename,
						Profile:  c.Profile,
					},
					ExpiryWindow: defaultExpiryWindow,
				})
			}
			return nil
		},
	}
	credentialsChain = append(credentialsChain, staticCredentialsProvider, refreshableCredentialsProvider)
	// You can overwrite the default credentials chain by first importing the current file
	// and then calling OverwriteCredentialsChain() with your own credentials chain
}
