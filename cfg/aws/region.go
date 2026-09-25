// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/config"
)

// SharedConfigRegion returns the region from the environment (AWS_REGION, then
// AWS_DEFAULT_REGION), or else the region of profile in the given shared files. An empty
// profile and nil file lists mean the SDK defaults; a profile that does not exist yields
// "" and no error.
func SharedConfigRegion(ctx context.Context, profile string, credentialsFiles, configFiles []string) (string, error) {
	envCfg, err := config.NewEnvConfig()
	if err != nil {
		return "", err
	}
	if envCfg.Region != "" {
		return envCfg.Region, nil
	}

	// Empty profile: AWS_PROFILE / AWS_DEFAULT_PROFILE, then "default".
	if profile == "" {
		profile = envCfg.SharedConfigProfile
	}
	if profile == "" {
		profile = defaultProfileName
	}
	// Nil file list: AWS_SHARED_CREDENTIALS_FILE / AWS_CONFIG_FILE, then the SDK default
	// location. An explicit list restricts the lookup to those files.
	if credentialsFiles == nil && envCfg.SharedCredentialsFile != "" {
		credentialsFiles = []string{envCfg.SharedCredentialsFile}
	}
	if configFiles == nil && envCfg.SharedConfigFile != "" {
		configFiles = []string{envCfg.SharedConfigFile}
	}
	shared, err := config.LoadSharedConfigProfile(ctx, profile, func(o *config.LoadSharedConfigOptions) {
		o.CredentialsFiles = credentialsFiles
		o.ConfigFiles = configFiles
		o.Logger = SDKLogger{}
	})
	// A missing profile is not an error, so the caller can fall back to other sources such
	// as EC2 metadata. SDK v1 behaved this way; SDK v2's config loader fails hard on a
	// named-but-missing profile, which is why the region is resolved here rather than
	// through config.LoadDefaultConfig.
	var notExist config.SharedConfigProfileNotExistError
	if errors.As(err, &notExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return shared.Region, nil
}
