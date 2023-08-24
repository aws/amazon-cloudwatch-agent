// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package commonconfig

import (
	"fmt"
	"io"

	"github.com/BurntSushi/toml"
)

const (
	CredentialSection = "credentials"
	CredentialProfile = "shared_credential_profile"
	CredentialFile    = "shared_credential_file"
	ProxySection      = "proxy"
	HttpProxy         = "http_proxy"
	HttpsProxy        = "https_proxy"
	NoProxy           = "no_proxy"
	SSLSection        = "ssl"
	CABundlePath      = "ca_bundle_path"
)

type CommonConfig struct {
	Credentials *Credentials
	Proxy       *Proxy
	SSL         *SSL
	IMDS        *IMDS
}

type Credentials struct {
	CredentialProfile *string `toml:"shared_credential_profile"`
	CredentialFile    *string `toml:"shared_credential_file"`
}

type Proxy struct {
	HttpProxy  *string `toml:"http_proxy"`
	HttpsProxy *string `toml:"https_proxy"`
	NoProxy    *string `toml:"no_proxy"`
}

type SSL struct {
	CABundlePath *string `toml:"ca_bundle_path"`
}

// IMDS is in common config because it happens before agent config translation
type IMDS struct {
	ImdsRetries *int `toml:"imds_retries"`
}

func New() *CommonConfig {
	return &CommonConfig{}
}

func Parse(r io.Reader) (*CommonConfig, error) {
	cc := New()
	err := cc.Parse(r)
	if err != nil {
		return nil, err
	}
	return cc, nil
}

func (c *CommonConfig) Parse(r io.Reader) error {
	if _, err := toml.DecodeReader(r, c); err != nil {
		return fmt.Errorf("unable to decode toml: %v", err)
	}
	return nil
}

// Temporary functions to enable build
// TODO: To be removed when all uses of commonconfig stop using map[string]string
func (c CommonConfig) CredentialsMap() map[string]string {
	result := make(map[string]string)
	if c.Credentials == nil {
		return result
	}

	if c.Credentials.CredentialProfile != nil {
		result[CredentialProfile] = *c.Credentials.CredentialProfile
	}

	if c.Credentials.CredentialFile != nil {
		result[CredentialFile] = *c.Credentials.CredentialFile
	}

	return result
}

// Temporary functions to enable build
// TODO: To be removed when all uses of commonconfig stop using map[string]string
func (c CommonConfig) ProxyMap() map[string]string {
	result := make(map[string]string)
	if c.Proxy == nil {
		return result
	}

	if c.Proxy.HttpProxy != nil {
		result[HttpProxy] = *c.Proxy.HttpProxy
	}

	if c.Proxy.HttpsProxy != nil {
		result[HttpsProxy] = *c.Proxy.HttpsProxy
	}

	if c.Proxy.NoProxy != nil {
		result[NoProxy] = *c.Proxy.NoProxy
	}

	return result
}

// Temporary functions to enable build
// TODO: To be removed when all uses of commonconfig stop using map[string]string
func (c CommonConfig) SSLMap() map[string]string {
	result := make(map[string]string)
	if c.SSL == nil {
		return result
	}

	if c.SSL.CABundlePath != nil {
		result[CABundlePath] = *c.SSL.CABundlePath
	}

	return result
}
