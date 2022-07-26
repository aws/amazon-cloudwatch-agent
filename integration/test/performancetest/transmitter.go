package performancetest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
	"strings"
	"log"
	"math/rand"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/google/uuid"
)


const (
	UPDATE_DELAY_THRESHOLD = 60 *1000// this is how long we want to wait for random sleep in milli seconds
	/*
	!Warning: if this value is less than 20 there is a risk of testCases being lost.
	This will only happen if all test threads and at the same time and get the same 
	sleep value after first attempt to add ITEM
	*/
)
type TransmitterAPI struct {
	dynamoDbClient *dynamodb.Client
	DataBaseName   string // this is the name of the table when test is run
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
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{ // this make sure we can query hashes in O(1) time
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
			TableName: aws.String(transmitter.DataBaseName)}, 5* time.Minute) //5 minutes is the timeout value for a table creation
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
	var ae *types.ConditionalCheckFailedException // this exception represent the the atomic check has failed
	item, err := attributevalue.MarshalMap(packet)
	if err != nil {
		panic(err)
	}
	_, err = transmitter.dynamoDbClient.PutItem(context.TODO(),
		&dynamodb.PutItemInput{
			Item:      item,
			TableName: aws.String(transmitter.DataBaseName),
			ConditionExpression: aws.String("attribute_not_exists(#hash)"),
			// ExpressionAttributeValues: map[string]types.AttributeValue{
			// 	":testHash": &types.AttributeValueMemberS{Value: packet[TEST_HASH].(string)},
			// },
			ExpressionAttributeNames: map[string]string{
				// "#testHash": TEST_HASH,
				"#hash" : "Hash",
			},
		})
	
	if err != nil && !errors.As(err,&ae){
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
func (transmitter *TransmitterAPI) SendItem(packet map[string]interface{}) (string, error) {
	// return nil
	var sentItem string
	var ae *types.ConditionalCheckFailedException // this exception represent the the atomic check has failed
	// check if hash exists
	currentItemList,err := transmitter.Query(packet[HASH].(string))
	if err!=nil {
		return "",err
	}
	if len(currentItemList)==0{ // if an item with the same has doesn't exist add it
		sentItem, err = transmitter.AddItem(packet) // this may be overwritten by other test threads, in that case it will return a specific error
	
		if !errors.As(err,&ae){ // check if our add call got overwritten by other threads
			return sentItem,err
		}
		if err!=nil { //any other error dont try again
			return "",err
		}
		// got overwritten now instead of adding go update the item
		rand.Seed(time.Now().UnixNano())
		time.Sleep(time.Duration(rand.Intn(UPDATE_DELAY_THRESHOLD))*time.Millisecond)
		fmt.Println("Item already exist going to update",len(currentItemList))
		
	}
	// item already exist so update the item instead
	for {//concurrency retry
		/*solving parallel updates using an optimistic lock and retry,
		this may result in a livelock;
		however, random sleeps makes this possiblity nearly impossible*/
		currentItemList,err = transmitter.Query(packet[HASH].(string)) //get the item's most up to date version
		newAttributes,err := transmitter.TestCasePackager(packet,currentItemList[0])
		if(err!=nil){
			return "",err
		}
		err = transmitter.UpdateItem(packet[HASH].(string),newAttributes,currentItemList[0][TEST_HASH].(string)) //try to update the item
		//this may be overwritten by other test threads, in that case it will return a specific error
		if errors.As(err,&ae){ //check if our call got overwritten
			// item has changed
			fmt.Println("Retrying...")
			rand.Seed(time.Now().UnixNano())
			time.Sleep(time.Duration(rand.Intn(UPDATE_DELAY_THRESHOLD))*time.Millisecond)
			continue
		}
		fmt.Println("Update Completed")
		break
	}
	return sentItem, err
}
/*
TestCasePackager()
Desc:
	This function updates the currentPacket with the unique parts of newPacket and returns in dynamo format
Params:
	newPacket: this is the agentData collected in this test
	currentPacket: this is the agentData stored in dynamo currently
*/
func (transmitter * TransmitterAPI) TestCasePackager(newPacket map[string]interface{}, currentPacket map[string]interface{} )(map[string]types.AttributeValue,error){
	testSettings := fmt.Sprintf("%s-%s",os.Getenv("PERFORMANCE_NUMBER_OF_LOGS"),os.Getenv("TPS"))
	fmt.Println("The test is",testSettings)
	item := currentPacket["Results"].(map[string]interface{})
	_,isPresent := item[testSettings] // check if we already had this test
	if isPresent{ 
		// we already had this test so ignore it
		return nil,errors.New("Nothing to update")
	}
	testSettingValue:=newPacket["Results"].(map[string]map[string]Stats)[testSettings]
	mergedResults := make(map[string]map[string]interface{})
	mergedResults["Results"] = make(map[string]interface{})
	for attribute,value := range item{
		_, isPresent := newPacket["Results"].(map[string]map[string]Stats)[attribute]
		if(isPresent){continue}
		mergedResults["Results"][attribute] = value
		
	}
	
	mergedResults["Results"][testSettings] = testSettingValue
	newAttributes, _ := attributevalue.MarshalMap(mergedResults)
	return newAttributes, nil
}
/*
UpdateItem()
Desc:
	This function updates the item in dynamo if the atomic condition is true else it will return ConditionalCheckFailedException 
Params:
	hash: this is the commitHash
	targetAttributes: this is the targetAttribute to be added to the dynamo item
	testHash: this is the hash of the last item, used like a version check
*/
func (transmitter * TransmitterAPI) UpdateItem(hash string,targetAttributes map[string]types.AttributeValue, testHash string) error{
	var err error
	var ae *types.ConditionalCheckFailedException // this exception represent the the atomic check has failed
	fmt.Println("Updating:",hash,testHash)
	item,err := transmitter.Query(hash) // get most Up to date item from dynamo | O(1) bcs of global sec. idx.
	if len(item) ==0{ // check if hash is in dynamo
		return errors.New("ERROR: Hash is not found in dynamo")
	}
	commitDate := fmt.Sprintf("%d",int(item[0]["CommitDate"].(float64)))
	year := fmt.Sprintf("%d",int(item[0]["Year"].(float64)))
	//setup the update expression
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
	expressionAttributeValues[":testHash"] = &types.AttributeValueMemberS{Value: testHash}
	//----
	//call update
	_, err = transmitter.dynamoDbClient.UpdateItem(context.TODO(), &dynamodb.UpdateItemInput{
        TableName: aws.String(transmitter.DataBaseName),
        Key: map[string]types.AttributeValue{
            "Year": &types.AttributeValueMemberN{Value: year},
			"CommitDate": &types.AttributeValueMemberN{Value: commitDate },
        },
        UpdateExpression: aws.String(expression),
        ExpressionAttributeValues: expressionAttributeValues,
		ConditionExpression: aws.String("#testHash = :testHash"),
		ExpressionAttributeNames: map[string]string{
			"#testHash": TEST_HASH,
		},
    })

    if err != nil && !errors.As(err,&ae) {
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
	err := transmitter.UpdateItem(hash,attributes,uuid.New().String())
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