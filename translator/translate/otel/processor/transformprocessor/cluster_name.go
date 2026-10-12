// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package transformprocessor

import (
	"fmt"

	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const setClusterNameProcessorName = "set_cluster_name"

func NewSetClusterNameTranslator(conf *confmap.Conf) (common.ComponentTranslator, error) {
	clusterName := common.GetClusterName(conf, common.OtelClusterNameKey)
	if clusterName == "" {
		return nil, nil
	}
	if err := common.ValidateClusterName(clusterName); err != nil {
		return nil, err
	}
	stmt := fmt.Sprintf(`set(resource.attributes["k8s.cluster.name"], "%s")`, clusterName)
	return NewTranslatorWithName(setClusterNameProcessorName,
		WithMetricResourceStatements([]string{stmt}),
		WithLogResourceStatements([]string{stmt}),
		WithTraceResourceStatements([]string{stmt}),
		WithProfileResourceStatements([]string{stmt}),
	), nil
}
