// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package commonconfig

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoConfig(t *testing.T) {
	contents := ""
	config := New()
	config.Parse(strings.NewReader(contents))
	assert.Nil(t, config.Credentials)
	assert.Nil(t, config.Proxy)
	assert.Nil(t, config.SSL)
}

func TestCredentialOnly(t *testing.T) {
	contents := `
				[credentials]
					shared_credential_profile = "{profile_name}"
					shared_credential_file = "{file_name}"
					role_arn = "{role_arn}"
				[proxy]
				`
	config := New()
	config.Parse(strings.NewReader(contents))
	assert.Equal(t, "{profile_name}", *config.Credentials.CredentialProfile)
	assert.Equal(t, "{file_name}", *config.Credentials.CredentialFile)
	assert.Nil(t, config.Proxy.HttpProxy)
	assert.Nil(t, config.Proxy.HttpsProxy)
	assert.Nil(t, config.Proxy.NoProxy)
	assert.Nil(t, config.SSL)
}

func TestConfig(t *testing.T) {
	contents := `
				[credentials]
					shared_credential_profile = "{profile_name}"
				[proxy]
					http_proxy = "{http_url}"
					https_proxy = "{https_url}"
					no_proxy = "{domain}"
				`
	config := New()
	config.Parse(strings.NewReader(contents))
	assert.Equal(t, "{profile_name}", *config.Credentials.CredentialProfile)
	assert.Equal(t, "{http_url}", *config.Proxy.HttpProxy)
	assert.Equal(t, "{https_url}", *config.Proxy.HttpsProxy)
	assert.Equal(t, "{domain}", *config.Proxy.NoProxy)
}

func TestSSLOnly(t *testing.T) {
	contents := `
				[ssl]
					 ca_bundle_path = "{ca_bundle_file_path}"
				`
	config := New()
	config.Parse(strings.NewReader(contents))
	assert.Equal(t, "{ca_bundle_file_path}", *config.SSL.CABundlePath)
}
