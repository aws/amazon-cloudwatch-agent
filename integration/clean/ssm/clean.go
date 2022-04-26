// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ssm

import (
	"log"
	"time"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmType "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

const (
	Type = "ssm" 
	containSSMParameterName = "AmazonCloudWatch"
)


func Clean(ctx context.Context, expirationDate time.Time) error {
	defaultConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	
	log.Println("Begin to clean SSM Parameter Store")
	ssmClient := ssm.NewFromConfig(defaultConfig)

	//Allow to load all the SSM parameter store since the default respond is paginated SSM Parameter Stores.
	//Look into the documentations and read the starting-token for more details
	//Documentation: https://docs.aws.amazon.com/cli/latest/reference/ssm/describe-parameters.html
	var nextToken *string

	var parameterStoreNameFilter = ssmType.ParameterStringFilter{
		Key:    aws.String("Name"),
		Option: aws.String("BeginsWith"),
		Values: []string{containSSMParameterName},
	}

	for {
		describeParametersInput := ssm.DescribeParametersInput{
			ParameterFilters: []ssmType.ParameterStringFilter{parameterStoreNameFilter},
			NextToken:        nextToken,
		}
		describeParametersOutput, err := ssmClient.DescribeParameters(ctx, &describeParametersInput)

		if err != nil {
			return err
		}

		for _, parameter := range describeParametersOutput.Parameters {

			if !expirationDate.After(*parameter.LastModifiedDate) {
				continue
			}

			log.Printf("Trying to delete Parameter Store with name %s and creation date %v", *parameter.Name, *parameter.LastModifiedDate)

			deleteParameterInput := ssm.DeleteParameterInput{Name: parameter.Name}

			if _, err := ssmClient.DeleteParameter(ctx, &deleteParameterInput); err != nil {
				return err
			}
		}

		if describeParametersOutput.NextToken == nil {
			break
		}

		nextToken = describeParametersOutput.NextToken
	}
	
	log.Println("End cleaning SSM Parameter Store")
	return nil
}
