// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	tests := map[string]struct {
		cfg     Config
		wantErr error
	}{
		"Valid": {
			cfg: Config{RoleARN: "arn:aws:iam::123456789012:role/TestRole"},
		},
		"ValidWithTokenFile": {
			cfg: Config{
				RoleARN:   "arn:aws:iam::123456789012:role/TestRole",
				Region:    "us-east-1",
				TokenFile: "/var/run/oidc/token",
			},
		},
		"MissingRoleARN": {
			cfg:     Config{Region: "us-east-1"},
			wantErr: errMissingRoleARN,
		},
		"Empty": {
			cfg:     Config{},
			wantErr: errMissingRoleARN,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
