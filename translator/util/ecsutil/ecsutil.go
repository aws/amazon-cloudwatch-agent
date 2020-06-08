package ecsutil

import (
	"encoding/json"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/httpclient"
	"log"
	"os"
	"strings"
	"sync"
)

const (
	v2MetadataEndpoint    = "http://169.254.170.2/v2/metadata"
	v3MetadataEndpointEnv = "ECS_CONTAINER_METADATA_URI"
)

type ecsMetadataResponse struct {
	Cluster string
	TaskARN string
}

type ecsUtil struct {
	Cluster    string
	Region     string
	TaskARN    string
	httpClient *httpclient.HttpClient
}

var ecsUtilInstance *ecsUtil
var ecsUtilOnce sync.Once

func GetECSUtilSingleton() *ecsUtil {
	ecsUtilOnce.Do(func() {
		ecsUtilInstance = initECSUtilSingleton()
	})
	return ecsUtilInstance
}

func initECSUtilSingleton() (newInstance *ecsUtil) {
	newInstance = &ecsUtil{httpClient: httpclient.New()}
	if os.Getenv(config.RUN_IN_CONTAINER) != config.RUN_IN_CONTAINER_TRUE {
		return
	}
	ecsMetadataResponse, err := newInstance.getECSMetadata()
	if err != nil {
		log.Println("E! getting information from ECS task metadata fail: ", err)
		return
	}

	newInstance.parseRegion(ecsMetadataResponse)
	newInstance.Cluster = ecsMetadataResponse.Cluster
	newInstance.TaskARN = ecsMetadataResponse.TaskARN
	return

}

func (e *ecsUtil) IsECS() bool {
	return e.Region != ""
}

func (e *ecsUtil) getECSMetadata() (em *ecsMetadataResponse, err error) {
	// choose available endpoint
	if v3MetadataEndpoint, ok := os.LookupEnv(v3MetadataEndpointEnv); !ok {
		em, err = e.getMetadataResponse(v2MetadataEndpoint)
	} else {
		em, err = e.getMetadataResponse(v3MetadataEndpoint + "/task")
	}
	return
}

func (e *ecsUtil) getMetadataResponse(endpoint string) (em *ecsMetadataResponse, err error) {
	em = &ecsMetadataResponse{}
	resp, err := e.httpClient.Request(endpoint)
	if err != nil {
		return
	}

	err = json.Unmarshal(resp, em)
	if err != nil {
		log.Printf("E! unable to parse resp from ecsmetadata endpoint, error: %v", err)
		log.Printf("D! resp content is %s", string(resp))
	}
	return
}

// There are two formats of Task ARN (https://docs.aws.amazon.com/AmazonECS/latest/userguide/ecs-account-settings.html#ecs-resource-ids)
// arn:aws:ecs:region:aws_account_id:task/task-id
// arn:aws:ecs:region:aws_account_id:task/cluster-name/task-id
// This function will return region extracted from Task ARN
func (e *ecsUtil) parseRegion(em *ecsMetadataResponse) {
	splitedContent := strings.Split(em.TaskARN, ":")
	// When splitting the ARN with ":", the 4th segment is the region
	if len(splitedContent) < 4 {
		log.Printf("E! invalid ecs task arn: %s", em.TaskARN)
	}
	e.Region = splitedContent[3]
}
