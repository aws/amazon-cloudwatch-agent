import AWS from "aws-sdk";
import axios from "axios";
import { GENERAL_ATTRIBUTES, BATCH_SIZE } from "../config";
const LATEST_ITEM = "LatestHash";
const CWAData = "CWAData";
const RELEASE_LIST = "ReleaseList";
const LINK = "Link";
const HASH = "Hash";
const IS_RELEASE = "isRelease";
const RESULTS = "Results";
const COMMIT_DATE = "CommitDate";
const REPO_LINK = "https://github.com/aws/amazon-cloudwatch-agent";
const GATEWAY_LINK = process.env.REACT_APP_GATEWAY;
//This class handles the entire frontend from pulling to formatting data
class Receiver {
  constructor(DataBaseName) {
    this.DataBaseName = DataBaseName;
    this.CWAData = null;
    this.ReleaseMap = {}; //hash map
    this.latestItem = null;
    var date = new Date();
    this.year = date.getFullYear().toString();
    let cacheLatestItem = this.cacheGetLatestItem();
    if (cacheLatestItem !== undefined) {
      this.CWAData = this.cacheGetAllData();
      this.latestItem = cacheLatestItem;
    }
    var tempReleaseList = this.cacheGetReleaseList();
    if (tempReleaseList !== null) {
      this.ReleaseMap = tempReleaseList;
    }
  }

  /*update()
  Desc: This function async. updates the local storage by comparing latest 
  item in dynamo and the latest item in local storage. If found a difference
  it updates them by calling getBatchItem to retrieve new data from dynamo
  Returns:  a list where [sync,error msg]; sync represent if cache is synced with dynamo
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
        return [true, ""];
      } else {
        cacheLatestHash = cacheLatestItem[HASH];
        this.CWAData = this.cacheGetAllData();
      }

      if (DynamoHash === cacheLatestHash) {
        return this.updateReleases(); // already synced
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
        return this.updateReleases(); // now synced
      }
      // website is ahead of dynamo
      //clear cache and retry
      this.cacheClear();
      this.update();
    } catch (err) {
      if (this.cacheGetLatestItem === undefined) {
        return [false, err];
      }
      this.CWAData = this.cacheGetAllData();
      return [false, err]; //couldnt sync
    }
  }
  /*updateReleases()
  Desc: This function async. queries all releases and checks
  if they are in cache, if not updates the data and saves them to cache.
  Returns:  a list where [sync,error msg]; sync represent if cache is synced with dynamo
  */
  async updateReleases() {
    //release backtracking
    try {
      var allReleases = await this.getAllReleases();
      allReleases.forEach((item) => {
        if (this.ReleaseMap[item[HASH].S]) {
          //old
          return;
        }
        //new
        Object.keys(item[RESULTS].M).forEach((testCase) => {
          var testCaseValue = item[RESULTS].M[testCase].M;
          Object.keys(testCaseValue).forEach((metric) => {
            //update link and release tag
            this.CWAData[testCase][metric].forEach((_, idx) => {
              if (
                this.CWAData[testCase][metric][idx][HASH] ===
                item[HASH].S.substring(0, 7)
              ) {
                this.CWAData[testCase][metric][idx][IS_RELEASE] = true;
              }
            });
            // this.CWAData[testCase][metric][LINK] = ""
            // this.CWAData[testCase][metric][HASH] =
            console.log("Updated", item[HASH].S);
          });
        });
        this.ReleaseMap[item[HASH].S] = true;
      });
      this.cacheSaveData();
      return [true, ""];
    } catch (err) {
      return [false, err];
    }
  }
  async getAllReleases() {
    const params = {
      // Set the projection expression, which are the attributes that you want.
      TableName: this.DataBaseName,
      KeyConditions: {
        Year: {
          ComparisonOperator: "EQ",
          AttributeValueList: [{ N: this.year }],
        },
      },
      FilterExpression: "#isRelease = :value",
      ExpressionAttributeNames: { "#isRelease": "isRelease" },
      ExpressionAttributeValues: { ":value": { BOOL: true } },

      ScanIndexForward: false,
    };
    var retData = await this.callGateway(params);
    // var retData = await this.dyanamoClient.query(params).promise();
    return retData.Items;
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
    // var data  = await this.callGateway("LATEST_ITEM",params)
    // console.log(data)
    var retData = await this.callGateway(params); //await this.dyanamoClient.query(params).promise();
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
    var retData = await this.callGateway(params); //await this.dyanamoClient.query(params).promise();
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
      var packet = (await this.callGateway(params)).Items;
      retData = retData.concat(packet);
      i++;
    }
    var cleanData = this.formatData(retData);
    return cleanData;
  }
  async callGateway(param) {
    var data = JSON.stringify({
      Params: param,
    });
    var config = {
      method: "POST",
      url: GATEWAY_LINK,
      headers: {
        "x-api-key": process.env.REACT_APP_GATEWAY_API_KEY,
        "Content-Type": "application/json",
        // "Access-Control-Allow-Headers": "Content-Type",
        // "Access-Control-Allow-Origin": "*",
        // "Access-Control-Allow-Methods": "OPTIONS,POST,GET"
      },
      data: data,
    };
    var out = axios(config)
      .then(function (response) {
        return response.data.body;
      })
      .catch(function (error) {
        console.log(error);
        return "error";
      });
    return out;
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
  //CACHE FUNCTIONS
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
  cacheGetReleaseList() {
    return JSON.parse(localStorage.getItem(RELEASE_LIST));
  }
  cacheSaveData() {
    if (this.latestItem == null || this.CWAData == null) {
      console.warn("Items are null");
      return;
    }
    localStorage.setItem(LATEST_ITEM, JSON.stringify(this.latestItem));
    localStorage.setItem(CWAData, JSON.stringify(this.CWAData));
    localStorage.setItem(RELEASE_LIST, JSON.stringify(this.ReleaseMap));
  }
}

export default Receiver;
