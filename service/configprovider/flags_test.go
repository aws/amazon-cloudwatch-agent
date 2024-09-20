// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetAndString(t *testing.T) {
	var fOtelConfigs OtelConfigFlags
	flag.Var(&fOtelConfigs, OtelConfigFlag, "YAML configuration files to run OTel pipeline")
	err := fOtelConfigs.Set("otelconfig1.yaml")
	assert.NoError(t, err)
	err = fOtelConfigs.Set("otelconfig2.yaml")
	assert.NoError(t, err)
	assert.Equal(t, "[otelconfig1.yaml otelconfig2.yaml]", fOtelConfigs.String())
}
