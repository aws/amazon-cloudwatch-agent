import AWS from "aws-sdk";
import { DEBUG, GENERAL_ATTRIBUTES, BATCH_SIZE } from "../config";
AWS.config.update({
  region: "us-west-2",
  secretAccessKey: process.env.REACT_APP_TERRAFORM_AWS_SECRET_ACCESS_KEY,
  accessKeyId: process.env.REACT_APP_TERRAFORM_AWS_ACCESS_KEY_ID,
});
const LATEST_ITEM = "LatestHash";
const CWAData = "CWAData";
const LINK = "Link";
const HASH = "Hash";
const RESULTS = "Results";
const COMMIT_DATE = "CommitDate";
const REPO_LINK = "https://github.com/aws/amazon-cloudwatch-agent";
//This class handles the entire frontend from pulling to formatting data
class Receiver {
  constructor(DataBaseName) {
    this.dyanamoClient = new AWS.DynamoDB({ apiVersion: "2012-08-10" });
    this.DataBaseName = DataBaseName;
    this.CWAData = null;
    this.latestItem = null;
    var date = new Date();
    this.year = date.getFullYear().toString();
    let cacheLatestItem = this.cacheGetLatestItem();
    if (cacheLatestItem !== undefined) {
      this.CWAData = this.cacheGetAllData();
      this.latestItem = cacheLatestItem;
    }
  }

  /*update()
  Desc: This function async. updates the local storage by comparing latest 
  item in dynamo and the latest item in local storage. If found a difference
  it updates them by calling getBatchItem to retrieve new data from dynamo
  Return: if the sync is successful or not
  */
  async update() {
    // check the latest hash from cache
    try {
      let dynamoLatestItem = await this.getLatestItem();
      let DynamoHash = dynamoLatestItem[HASH];
      // ask dynamo what is the lastest hash it received
      let cacheLatestItem = this.cacheGetLatestItem(); //[HASH].S //rename to lastest hash
      let cacheLatestHash = "";
      if (cacheLatestItem === null) {
        // no cache found, pull every thing and set
        this.CWAData = await this.getAllItems();
        this.latestItem = dynamoLatestItem;
        this.cacheSaveData();
        return true;
      } else {
        cacheLatestHash = cacheLatestItem[HASH];
        this.CWAData = this.cacheGetAllData();
      }

      if (DynamoHash === cacheLatestHash) {
        return true // already synced
      } else if (
        parseInt(dynamoLatestItem[COMMIT_DATE]) >=
        parseInt(cacheLatestItem[COMMIT_DATE])
      ) {
        /// if hashes dont match call getBatchItem with local hash and update local hash with new data
        var newItems = await this.getBatchItem(
          cacheLatestItem[COMMIT_DATE],
          dynamoLatestItem[COMMIT_DATE]
        );
        Object.keys(newItems).forEach((key) => {
          if (this.CWAData[key] === undefined) {
            // new testCase
            this.CWAData[key] = {};
          }
          //already have this testCase
          Object.keys(this.CWAData[key]).forEach((metric) => {
            if (this.CWAData[key][metric] === undefined) {
              // new metric
              this.CWAData[key][metric] = [];
            }
            //already have this metric
            this.CWAData[key][metric].push(...newItems[key][metric]);
          });
        });
        this.latestItem = dynamoLatestItem;
        this.cacheSaveData();
        return true // now synced
      }
      // website is ahead of dynamo
      //clear cache and retry 
      this.cacheClear()
      this.update()
    } catch (err) {
      console.log(`ERROR:${err}`);
      alert(`ERROR:${err}`);
      if (this.cacheGetLatestItem === undefined) {
        return false
      }
      this.CWAData = this.cacheGetAllData()
      // return this.cacheGetAllData();
      return false //couldnt sync
    }
  }

  /*getLatestItem()
  Desc: This pulls the item with HIGHEST CommitDate using global secondary index query
  Return: the most recently added item
  */
  async getLatestItem() {
    //add secondary index
    const params = {
      // Set the projection expression, which are the attributes that you want.
      TableName: this.DataBaseName,
      Limit: 1,
      KeyConditions: {
        Year: {
          ComparisonOperator: "EQ",
          AttributeValueList: [{ N: this.year }],
        },
      },
      ScanIndexForward: false,
    };
    var retData = await this.dyanamoClient.query(params).promise();
    var cleanData = AWS.DynamoDB.Converter.unmarshall(retData.Items[0]);
    cleanData[HASH] = cleanData[HASH].substring(0, 7);
    return cleanData;
  }
  /*getAllItems()
  Desc: This function pulls the entire from dynamo
  Note: This function is called only if cache doesn't exist
  Return: Entire Table
  */
  async getAllItems() {
    const params = {
      // Set the projection expression, which are the attributes that you want.
      TableName: this.DataBaseName,
      KeyConditions: {
        Year: {
          ComparisonOperator: "EQ",
          AttributeValueList: [{ N: this.year }],
        },
      },
    };
    var retData = await this.dyanamoClient.query(params).promise();

    var cleanData = this.formatData(retData.Items);
    return cleanData;
  }
  /* getBatchItem()
  Desc: This function pulls data from dynamo in batches. The batch configurable from
  config.js. By default it is designed to pull data with 1MB batches 
  Return: Updated Data stored in local storage
  */
  async getBatchItem(cacheHashDate, dynamoHashDate) {
    // will use scan because getBatchItem requires me to know both hash and
    var params = {
      TableName: this.DataBaseName,
      KeyConditions: {
        Year: {
          ComparisonOperator: "EQ",
          AttributeValueList: [{ N: this.year }],
        },
        CommitDate: {
          ComparisonOperator: "BETWEEN",
          AttributeValueList: [{ N: cacheHashDate }, { N: dynamoHashDate }],
        },
      },
    };
    var retData = [];
    var dynamoHashDateInt = parseInt(dynamoHashDate);
    var cacheHashDateInt = parseInt(cacheHashDate);
    var upperBound = 0;
    var i = 1;
    while (upperBound < dynamoHashDateInt) {
      //get data in 1mb packets
      var lowerBound = cacheHashDateInt + i * BATCH_SIZE;
      upperBound = cacheHashDateInt + (i + 1) * BATCH_SIZE;
      params.KeyConditions.CommitDate.AttributeValueList[0].N =
        lowerBound.toString(); //0 idx is start, 1 is end
      params.KeyConditions.CommitDate.AttributeValueList[1].N =
        upperBound.toString();
      var packet = (await this.dyanamoClient.query(params).promise()).Items;
      retData = retData.concat(packet);
      i++;
    }
    var cleanData = this.formatData(retData);
    return cleanData;
  }
  /* formatData
  Desc: Converts Dynamo formatted items to {...testCases:{...metrics:{...stats,...attributes}} format.
  Param: data: Object, raw data from dynamo
  Return: Clean Data
  */
  formatData(data) {
    var formattedData = {};
    data.forEach((item) => {
      var cleanData = AWS.DynamoDB.Converter.unmarshall(item);
      Object.keys(cleanData[RESULTS]).forEach((testCase) => {
        if (formattedData[testCase] === undefined) {
          formattedData[testCase] = {};
        }
        Object.keys(cleanData[RESULTS][testCase]).forEach((metric) => {
          if (formattedData[testCase][metric] === undefined) {
            formattedData[testCase][metric] = [];
          }
          var newStructure = cleanData[RESULTS][testCase][metric];
          GENERAL_ATTRIBUTES.forEach((generalAttribute) => {
            if (generalAttribute === HASH) {
              if (cleanData[generalAttribute].length > 7) {
                //@Todo: remove in the long since this is for SHA support
                newStructure[generalAttribute] = cleanData[
                  generalAttribute
                ].substring(0, 7);
                newStructure[
                  LINK
                ] = `${REPO_LINK}/commit/${cleanData[generalAttribute]}`;
                return;
              }
            }
            newStructure[generalAttribute] = cleanData[generalAttribute];
          });
          formattedData[testCase][metric].push(newStructure);
        });
      });
    });
    return formattedData;
  }
  cacheClear() {
    localStorage.clear();
    document.location.reload();
  }
  cacheGetAllData() {
    return JSON.parse(localStorage.getItem(CWAData));
  }
  cacheGetLatestItem() {
    return JSON.parse(localStorage.getItem(LATEST_ITEM));
  }
  cacheSaveData() {
    if (this.latestItem == null || this.CWAData == null) {
      console.warn("Items are null");
      return;
    }
    localStorage.setItem(LATEST_ITEM, JSON.stringify(this.latestItem));
    localStorage.setItem(CWAData, JSON.stringify(this.CWAData));
  }
}

export default Receiver;
