package performancetest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
	"strconv"
	"strings"
	"math"
	"log"
	"sort"
	"math/rand"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	METRIC_PERIOD = 5 * 60 // this const is in seconds , 5 mins
	PARTITION_KEY ="Year"
	HASH = "Hash"
	COMMIT_DATE= "CommitDate"
	SHA_ENV  = "SHA"
	SHA_DATE_ENV = "SHA_DATE"
	NUMBER_OF_LOGS_MONITORED = "NumberOfLogsMonitored"
	TPS = "TPS"
	UPDATE_DELAY_THRESHOLD = 60
)
type TransmitterAPI struct {
	dynamoDbClient *dynamodb.Client
	DataBaseName   string // this is the name of the table when test is run
}

// this is the packet that will be sent converted to DynamoItem
type Metric struct {
	Average float64
	P99     float64 //99% percent process
	Max     float64
	Min     float64
	Period  int //in seconds
	Std 	float64
	Data    []float64
}

type collectorData []struct { // this is the struct data collector passes in
	Id         string    `json:"Id"`
	Label      string    `json:Label`
	Messages   string    `json:Messages`
	StatusCode string    `json:StatusCode`
	Timestamps []string  `json:Timestamps`
	Values     []float64 `json:Values`
}

/*
InitializeTransmitterAPI
Desc: Initializes the transmitter class
Side effects: Creates a dynamodb table if it doesn't already exist
*/
func InitializeTransmitterAPI(DataBaseName string) *TransmitterAPI {
	//setup aws session
	cfg, err := config.LoadDefaultConfig(context.TODO(),config.WithRegion("us-west-2"))
	if err != nil {
		fmt.Printf("Error: Loading in config %s\n", err)
	}
	transmitter := TransmitterAPI{
		dynamoDbClient: dynamodb.NewFromConfig(cfg),
		DataBaseName:   DataBaseName,
	}
	// check if the dynamo table exist if not create it
	tableExist, err := transmitter.TableExist()
	if err != nil {
		return nil
	}
	if !tableExist {
		fmt.Println("Table doesn't exist")
		err := transmitter.CreateTable()
		if err != nil {
			return nil
		}
	}
	fmt.Println("API ready")
	return &transmitter

}

/*
CreateTable()
Desc: Will create a DynamoDB Table with given param. and config
*/
 //add secondary index space vs time  
func (transmitter *TransmitterAPI) CreateTable() error {
	_, err := transmitter.dynamoDbClient.CreateTable(
		context.TODO(), &dynamodb.CreateTableInput{
			AttributeDefinitions: []types.AttributeDefinition{
				{
					AttributeName: aws.String(PARTITION_KEY),
					AttributeType: types.ScalarAttributeTypeN,
				},
				{
					AttributeName: aws.String("CommitDate"),
					AttributeType: types.ScalarAttributeTypeN,
				},
				{
					AttributeName: aws.String("Hash"),
					AttributeType: types.ScalarAttributeTypeS,
				},
			},
			KeySchema: []types.KeySchemaElement{
				{
					AttributeName: aws.String(PARTITION_KEY),
					KeyType:       types.KeyTypeHash,
				},
				{
					AttributeName: aws.String("CommitDate"),
					KeyType:	   types.KeyTypeRange,
				},
			},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("Hash-index"),
					KeySchema: []types.KeySchemaElement{
						{
							AttributeName: aws.String("Hash"),
							KeyType:       types.KeyTypeHash,
						},
						{
							AttributeName: aws.String("CommitDate"),
							KeyType:	   types.KeyTypeRange,
						},
					},
					Projection: &types.Projection{
						ProjectionType : "ALL",
					},
					ProvisionedThroughput: &types.ProvisionedThroughput{
						ReadCapacityUnits:  aws.Int64(10),
						WriteCapacityUnits: aws.Int64(10),
					},
				},
			},
			ProvisionedThroughput: &types.ProvisionedThroughput{
				ReadCapacityUnits:  aws.Int64(10),
				WriteCapacityUnits: aws.Int64(10),
			},
			TableName: aws.String(transmitter.DataBaseName),
		}) // this is the config for the new table)
	if err != nil {
		fmt.Printf("Error calling CreateTable: %s", err)
		return err
	}
	//https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/GettingStarted.CreateTable.html
	waiter := dynamodb.NewTableExistsWaiter(transmitter.dynamoDbClient)
		err = waiter.Wait(context.TODO(), &dynamodb.DescribeTableInput{
			TableName: aws.String(transmitter.DataBaseName)}, 5* time.Minute)
		if err != nil {
			log.Printf("Wait for table exists failed. Here's why: %v\n", err)
	}
	fmt.Println("Created the table", transmitter.DataBaseName)
	return nil
}

/*
AddItem()
Desc: Takes in a packet and
will convert to dynamodb format  and upload to dynamodb table.
Param:
	packet * map[string]interface{}:  is a map with data collection data
Side effects:
	Adds an item to dynamodb table
*/
func (transmitter *TransmitterAPI) AddItem(packet map[string]interface{}) (string, error) {
	item, err := attributevalue.MarshalMap(packet)
	if err != nil {
		panic(err)
	}
	_, err = transmitter.dynamoDbClient.PutItem(context.TODO(),
		&dynamodb.PutItemInput{
			Item:      item,
			TableName: aws.String(transmitter.DataBaseName),
		})
	if err != nil {
		fmt.Printf("Error adding item to table.  %v\n", err)
	}
	return fmt.Sprintf("%v", item), err
}

/*
TableExist()
Desc: Checks if the the table exist and returns the value
//https://github.com/awsdocs/aws-doc-sdk-examples/blob/05a89da8c2f2e40781429a7c34cf2f2b9ae35f89/gov2/dynamodb/actions/table_basics.go
*/
func (transmitter *TransmitterAPI) TableExist() (bool, error) {
	exists := true
	_, err := transmitter.dynamoDbClient.DescribeTable(
		context.TODO(), &dynamodb.DescribeTableInput{TableName: aws.String(transmitter.DataBaseName)},
	)
	if err != nil {
		var notFoundEx *types.ResourceNotFoundException
		if errors.As(err, &notFoundEx) {
			fmt.Printf("Table %v does not exist.\n", transmitter.DataBaseName)
			err = nil
		} else {
			fmt.Printf("Couldn't determine existence of table %v. Error: %v\n", transmitter.DataBaseName, err)
		}
		exists = false
	}
	return exists, err
}


/*
SendItem()
Desc: Parses the input data and adds it to the dynamo table
Param: data []byte is the data collected by data collector
*/
func (transmitter *TransmitterAPI) SendItem(data []byte) (string, error) {
	// return nil
	packet, err := transmitter.Parser(data)
	var sentItem string
	if err != nil {
		return "", err
	}
	// check if hash exists
	currentItemList,err := transmitter.Query(packet[HASH].(string))
	if err!=nil {
		return "",err
	}
	if len(currentItemList)==0{ // if doesnt  exit addItem
		sentItem, err = transmitter.AddItem(packet)
		return sentItem, err
	}
	// item already exist so update
	for {//concurrency retry
		newAttributes,err := transmitter.TestCasePackager(packet,currentItemList[0])
		if(err!=nil){
			return "",err
		}
		itemList,err := transmitter.Query(packet[HASH].(string))
		if len(currentItemList[0]["Results"].(map[string]interface{})) != len(itemList[0]["Results"].(map[string]interface{})){ 
			// 0 bcs hash values are unique, len bcs im checking if a new test case is added
			fmt.Println("Retrying...")
			time.Sleep(time.Duration(rand.Intn(UPDATE_DELAY_THRESHOLD))*time.Second)
			continue
		}
		if transmitter.UpdateItem(packet[HASH].(string),newAttributes) ==nil{
			fmt.Println("Update completed")
			break
		}
	}
	return sentItem, err
}
func (transmitter * TransmitterAPI) TestCasePackager(newPacket map[string]interface{}, currentPacket map[string]interface{} )(map[string]types.AttributeValue,error){
	testSettings := fmt.Sprintf("%s-%s",os.Getenv("PERFORMANCE_NUMBER_OF_LOGS"),"10")
	fmt.Println("The test is",testSettings)
	item := currentPacket["Results"].(map[string]interface{})
	_,isPresent := item[testSettings] // check if we already had this test
	if isPresent{ // no diff
		return nil,errors.New("Nothing to update")
	}
	testSettingValue, err := attributevalue.MarshalMap(currentPacket["Results"].(map[string]interface{})[testSettings])
	fmt.Println("test value",testSettingValue)
	if err !=nil{
		fmt.Println(err)
	}
	tempResults := make(map[string]map[string]interface{})
	tempResults["Results"] = make(map[string]interface{})
	for attribute,value := range item{
		_, isPresent := newPacket["Results"].(map[string]map[string]Metric)[attribute]
		if(isPresent){continue}
		tempResults["Results"][attribute] = value
		
	}
	
	tempResults["Results"][testSettings] = testSettingValue
	newAttributes, _ := attributevalue.MarshalMap(tempResults)
	return newAttributes, nil
}
func (transmitter *TransmitterAPI) Parser(data []byte) (map[string]interface{}, error) {
	dataHolder := collectorData{}
	err := json.Unmarshal(data, &dataHolder)
	if err != nil {
		return nil, err
	}
	packet := make(map[string]interface{})
	packet[PARTITION_KEY] = time.Now().Year()
	packet[HASH] =  os.Getenv(SHA_ENV) //fmt.Sprintf("%d", time.Now().UnixNano())
	packet[COMMIT_DATE],_ = strconv.Atoi(os.Getenv(SHA_DATE_ENV))
	packet["isRelease"] = false
	testSettings := fmt.Sprintf("%s-%s",os.Getenv("PERFORMANCE_NUMBER_OF_LOGS"),"10")
	testMetricResults := make(map[string]Metric)
	for _, rawMetricData := range dataHolder {

		metric := CalcStats(rawMetricData.Values)
		testMetricResults[rawMetricData.Label] = metric
	}
	packet["Results"] = map[string]map[string]Metric{ testSettings: testMetricResults}
	return packet, nil
}
func (transmitter * TransmitterAPI) UpdateItem(hash string,targetAttributes map[string]types.AttributeValue) error{
	var err error
	fmt.Println("Updating:",hash)
	item,err := transmitter.Query(hash) // O(1) bcs of global sec. idx.
	if len(item) ==0{
		return errors.New("ERROR: Hash is not found in dynamo")
	}
	commitDate := fmt.Sprintf("%d",int(item[0]["CommitDate"].(float64)))
	year := fmt.Sprintf("%d",int(item[0]["Year"].(float64)))
	expressionAttributeValues := make(map[string]types.AttributeValue)
	expression := ""
	n_expression := len(targetAttributes)
	i :=0
	for attribute, value := range targetAttributes{
		expressionName := ":" +strings.ToLower(attribute)
		expression = fmt.Sprintf("set %s = %s",attribute,expressionName)
		expressionAttributeValues[expressionName] = value
		if(n_expression -1 >i){
			expression += "and"
		}
		i++
	}
	fmt.Println(expression)
	_, err = transmitter.dynamoDbClient.UpdateItem(context.TODO(), &dynamodb.UpdateItemInput{
        TableName: aws.String(transmitter.DataBaseName),
        Key: map[string]types.AttributeValue{
            "Year": &types.AttributeValueMemberN{Value: year},
			"CommitDate": &types.AttributeValueMemberN{Value: commitDate },
        },
        UpdateExpression: aws.String(expression),
        ExpressionAttributeValues: expressionAttributeValues,
    })

    if err != nil {
        panic(err)
    }
	return err
}
/*
UpdateReleaseTag()
Desc: This function takes in a commit hash and updates the release value to true
Param: commit hash in terms of string 
*/
func (transmitter * TransmitterAPI) UpdateReleaseTag(hash string) error{
	attributes := map[string]types.AttributeValue{
		"isRelease":&types.AttributeValueMemberBOOL{Value: true},
	}
	err := transmitter.UpdateItem(hash,attributes)
	return err
}


func (transmitter* TransmitterAPI) Query(hash string) ([]map[string]interface{}, error) {
	var err error
	var packets []map[string]interface{}
    out, err := transmitter.dynamoDbClient.Query(context.TODO(), &dynamodb.QueryInput{
        TableName:              aws.String(transmitter.DataBaseName),
		IndexName:				aws.String("Hash-index"),
        KeyConditionExpression: aws.String("#hash = :hash"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":hash": &types.AttributeValueMemberS{Value: hash},
        },
        ExpressionAttributeNames: map[string]string{
            "#hash": "Hash",
        },
        ScanIndexForward: aws.Bool(true), // true or false to sort by "date" Sort/Range key ascending or descending
    })
	if err != nil {
        panic(err)
    }
	// fmt.Println(out.Items)
	attributevalue.UnmarshalListOfMaps(out.Items,&packets)
	return packets, err
}




//CalcStats takes in an array of data and returns the average, min, max, p99, and stdev of the data in a Metric struct
func CalcStats(data []float64) Metric {
	length := len(data)
	if length == 0 {
		return Metric{}
	}

	//make a copy so we aren't modifying original
	dataCopy := make([]float64, length)
	copy(dataCopy, data)
	sort.Float64s(dataCopy)

	min := dataCopy[0]
	max := dataCopy[length - 1]

	sum := 0.0
	for _, value := range dataCopy {
		sum += value
	}

	avg := sum / float64(length)

	if length < 99 {
		log.Println("Note: less than 99 values given, p99 value will be equal the max value")
	}
	p99Index := int(float64(length) * .99) - 1
	p99Val := dataCopy[p99Index]

	stdDevSum := 0.0
	for _, value := range dataCopy {
		stdDevSum += math.Pow(avg - value, 2)
	}

	stdDev := math.Sqrt(stdDevSum / float64(length))

	metrics := Metric{
		Average: avg,
		Max:     max,
		Min:     min,
		P99:     p99Val,
		Std:     stdDev,
		Period:  int(METRIC_PERIOD / float64(length)),
		Data:    data,
	}

	return metrics
}
