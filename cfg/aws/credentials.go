package aws

import (
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
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

type RootCredentialsProvider struct {
	Name        func() string
	Credentials func(*CredentialConfig) *credentials.Credentials
}

var credentialsChain = make([]RootCredentialsProvider, 0)

func getRootCredentialsFromChain(c *CredentialConfig) *credentials.Credentials {
	for _, provider := range credentialsChain {
		credentials := provider.Credentials(c)
		if credentials != nil {
			return credentials
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
	ses, err := session.NewSession(config)
	if err != nil {
		log.Printf("E! Failed to create credential sessions, retrying in 15s, error was '%s' \n", err)
		time.Sleep(15 * time.Second)
		ses, err = session.NewSession(config)
		if err != nil {
			log.Printf("E! Retry failed for creating credential sessions, error was '%s' \n", err)
			return ses
		}
	}
	log.Printf("D! Successfully created credential sessions\n")
	return ses
}

func (c *CredentialConfig) rootCredentials() client.ConfigProvider {
	config := &aws.Config{
		Region:                        aws.String(c.Region),
		CredentialsChainVerboseErrors: aws.Bool(true),
	}
	config.Credentials = getRootCredentialsFromChain(c)
	return getSession(config)
}

func (c *CredentialConfig) assumeCredentials() client.ConfigProvider {
	rootCredentials := c.rootCredentials()
	config := &aws.Config{
		Region: aws.String(c.Region),
	}
	config.Credentials = stscreds.NewCredentials(rootCredentials, c.RoleARN)
	return getSession(config)
}

func (c *CredentialConfig) Credentials() client.ConfigProvider {
	if c.RoleARN != "" {
		return c.assumeCredentials()
	} else {
		return c.rootCredentials()
	}
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
