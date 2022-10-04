package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"log"
	"time"
)

// delete ssm parameter older than 14 days
const keepDurationSSMParameter = -1 * time.Hour * 24 * 14
const parameterSearchKey = "Name"
const parameterSearchValue = "AmazonCloudWatch"
const parameterSearchOption = "Contains"

func main() {
	err := cleanSSMParameter()
	if err != nil {
		log.Fatalf("errors cleaning %v", err)
	}
}

func cleanSSMParameter() error {
	log.Print("Begin to clean ssm parm")

	expirationSSMParameter := time.Now().UTC().Add(keepDurationSSMParameter)

	cxt := context.Background()
	ssmClient, err := getSSMClient(cxt)
	if err != nil {
		return err
	}

	ssmParameter, err := getSSMParameter(cxt, ssmClient, expirationSSMParameter)
	if err != nil || len(ssmParameter) == 0 {
		return err
	}

	err = deleteSSMParameter(cxt, ssmClient, ssmParameter)

	return err
}

func getSSMClient(cxt context.Context) (*ssm.Client, error) {
	defaultConfig, err := config.LoadDefaultConfig(cxt)
	if err != nil {
		return nil, err
	}
	return ssm.NewFromConfig(defaultConfig), nil
}

func getSSMParameter(ctx context.Context, ssmClient *ssm.Client, expirationSSMParameter time.Time) ([]string, error) {
	nameFilter := types.ParameterStringFilter{Key: aws.String(parameterSearchKey),
		Option: aws.String(parameterSearchOption),
		Values: []string{
			parameterSearchValue,
		}}

	nextToken := aws.String("")
	names := make([]string, 0)
	parameterInput := ssm.DescribeParametersInput{ParameterFilters: []types.ParameterStringFilter{nameFilter}, MaxResults: aws.Int32(50)}
	// we are getting throttled for too many calls to ssm thus put a max on the amount of names to delete per run
	for nextToken != nil && len(names) < 250 {
		// see https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_DescribeParameters.html for max results
		parameterOutput, err := ssmClient.DescribeParameters(ctx, &parameterInput)
		if err != nil {
			return nil, err
		}
		for i := range parameterOutput.Parameters {
			log.Printf("Parameter : %v was last modified %v", *parameterOutput.Parameters[i].Name, *parameterOutput.Parameters[i].LastModifiedDate)
			if expirationSSMParameter.After(*parameterOutput.Parameters[i].LastModifiedDate) {
				log.Printf("Parameter : %v was last modified : %v which is before 14 days : %v thus try to delete", *parameterOutput.Parameters[i].Name, *parameterOutput.Parameters[i].LastModifiedDate, expirationSSMParameter)
				names = append(names, *parameterOutput.Parameters[i].Name)
			}
		}
		nextToken = parameterOutput.NextToken
		parameterInput = ssm.DescribeParametersInput{ParameterFilters: []types.ParameterStringFilter{nameFilter}, MaxResults: aws.Int32(50), NextToken: nextToken}
	}
	log.Printf("Parameter names to remove %v", names)
	return names, nil
}

func deleteSSMParameter(ctx context.Context, ssmClient *ssm.Client, names []string) error {
	// must batch into size of 10 due to error
	// "at 'names' failed to satisfy constraint: Member must have length less than or equal to 10"
	for i := 0; i < len(names); i = i + 10 {
		batch := make([]string, 0, 10)
		for j := 0; i+j < i+10 && i+j < len(names); j++ {
			batch = append(batch, names[i+j])
		}
		deleteParameterInput := ssm.DeleteParametersInput{Names: batch}
		parameters, err := ssmClient.DeleteParameters(ctx, &deleteParameterInput)
		if err != nil {
			return err
		}
		log.Printf("Valid Parameter Deleted : %v Invalid Parameter Not Deleted %v", parameters.DeletedParameters, parameters.InvalidParameters)
	}
	return nil
}
