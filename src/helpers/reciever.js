import AWS from "aws-sdk"
import {DEBUG,GENERAL_ATTRIBUTES,BATCH_SIZE} from "../config"
AWS.config.update({
  'region': 'us-west-2',
  'secretAccessKey': process.env.REACT_APP_TERRAFORM_AWS_SECRET_ACCESS_KEY,
  'accessKeyId': process.env.REACT_APP_TERRAFORM_AWS_ACCESS_KEY_ID
})
// ENDS HERE
const LATEST_ITEM = "LatestHash"
const CWAData = "CWAData"
const LINK = "Link"
const REPO_LINK = "https://github.com/aws/amazon-cloudwatch-agent"
//This class handles the entire frontend from pulling to formatting data
class Receiver {
  constructor(DataBaseName) {
    // this.cacheClear()

    this.dyanamoClient = new AWS.DynamoDB({ apiVersion: '2012-08-10' });
    this.DataBaseName = DataBaseName
    this.CWAData = null
    this.latestItem = null
    var date = new Date()
    this.year = date.getFullYear()
    let cacheLatestItem = this.cacheGetLatestItem()
    if (cacheLatestItem !== undefined) {
      this.CWAData = this.cacheGetAllData()
      this.latestItem = cacheLatestItem
    }
  }

  //update
  //@TODO Add interface
  async update() {
    console.log("updating")
    // check the latest hash from cache
    try {
      let dynamoLatestItem = (await this.getLatestItem())
      let DynamoHash = dynamoLatestItem["Hash"]
      // ask dynamo what is the lastest hash it received 
      let cacheLatestItem = this.cacheGetLatestItem()//["Hash"].S //rename to lastest hash
      let cacheLatestHash = ""
      if (cacheLatestItem === null) {
        console.log("NO cache found", localStorage.key(0), localStorage.key(1))
        // no cache found, pull every thing and set
        this.CWAData = await this.getAllItems()
        this.latestItem = dynamoLatestItem
        this.cacheSaveData()
        return
      } else {
        cacheLatestHash = cacheLatestItem["Hash"]
        this.CWAData = this.cacheGetAllData()  
      }
      
      if (DynamoHash === cacheLatestHash) {
        console.log("synced") // synced up
      } else if (parseInt(dynamoLatestItem["CommitDate"]) >= parseInt(cacheLatestItem["CommitDate"])) {
        /// if hashes dont match call getBatchItem with local hash and update local hash with new data
        console.log("not synced")
        var newItems = await this.getBatchItem(cacheLatestItem["CommitDate"], dynamoLatestItem["CommitDate"])
        console.log(this.CWAData, newItems)
        Object.keys(newItems).forEach((key)=>{
          if(this.CWAData[key]=== undefined){
            // new testCase
            this.CWAData[key]={}
          }
          //already have this testCase
          Object.keys(this.CWAData[key]).forEach((metric)=>{
            if(this.CWAData[key][metric] === undefined){
              // new metric
              this.CWAData[key][metric] = []
            }
            //already have this metric
            this.CWAData[key][metric].push(...newItems[key][metric])
          })
        })
        this.latestItem = dynamoLatestItem
        this.cacheSaveData()
      }
    }
    catch (err) {
      console.log(`ERROR:${err}`)
      alert(`ERROR:${err}`)
      if (this.cacheGetLatestItem === undefined) {
        return {}
      }
      return this.cacheGetAllData()

    }

  }

  //get latest item
  //@TODO Add interface
  async getLatestItem() {
    //add secondary index 
    const params = {
      // Set the projection expression, which are the attributes that you want.
      TableName: this.DataBaseName,
      Limit: 1,
      KeyConditions: {
        "Year": {
          ComparisonOperator: "EQ",
          AttributeValueList: [{ N: "2022" }]
        }
      },
      ScanIndexForward: false,

    };
    var retData = (await this.dyanamoClient.query(params).promise())
    if (DEBUG) {
      console.log(`getLatestItem: Item: ${retData.Items[0]["Hash"]["S"]}, Count: ${retData.Count}, ScannedCount:${retData.ScannedCount}`)
    }
    var cleanData = AWS.DynamoDB.Converter.unmarshall(retData.Items[0])
    cleanData["Hash"] = cleanData["Hash"].substring(0, 7)
    return cleanData
  }
  // get all
  //@TODO Add interface
  async getAllItems() {
    const params = {
      // Set the projection expression, which are the attributes that you want.
      TableName: this.DataBaseName,
      KeyConditions: {
        "Year": {
          ComparisonOperator: "EQ",
          AttributeValueList: [{ N: "2022" }]
        }
      },

    };
    var retData = (await this.dyanamoClient.query(params).promise())

    if (DEBUG) {
      console.log(`getAllItem: Item: ${retData.Items}, Count: ${retData.Count}, ScannedCount:${retData.ScannedCount}`)
    }
    var cleanData = this.formatData(retData.Items)
    return cleanData
  }
  // get a batch of items
  /*
  lastHash : the last hash i have cached
  */
  //@TODO Add interface
  async getBatchItem(cacheHashDate, dynamoHashDate) {
    console.log("Getting batch item", cacheHashDate, dynamoHashDate, cacheHashDate < dynamoHashDate)
    // will use scan because getBatchItem requires me to know both hash and 
    var params = {
      TableName: this.DataBaseName,
      KeyConditions: {
        "Year": {
          ComparisonOperator: "EQ",
          AttributeValueList: [{ N: "2022" }]
        },
        "CommitDate": {
          ComparisonOperator: "BETWEEN",
          AttributeValueList: [
            { "N": cacheHashDate },
            { "N": dynamoHashDate }
          ]
        }
      },

    }
    var retData = []
    var dynamoHashDateInt = parseInt(dynamoHashDate)
    var cacheHashDateInt = parseInt(cacheHashDate)
    var upperBound = 0
    var i = 1;
    console.log(BATCH_SIZE)
    while (upperBound < dynamoHashDateInt) { //get data in 1mb packets
      var lowerBound = cacheHashDateInt + (i * BATCH_SIZE)
      upperBound = cacheHashDateInt + (i + 1) * (BATCH_SIZE)
      params.KeyConditions.CommitDate.AttributeValueList[0].N = lowerBound.toString()//0 idx is start, 1 is end
      params.KeyConditions.CommitDate.AttributeValueList[1].N = upperBound.toString()
      var packet = (await this.dyanamoClient.query(params).promise()).Items
      if (DEBUG) {
        console.log(`Running Batch: (${lowerBound}->${upperBound}): Size:${retData}, ${packet}`)
      }
      retData = retData.concat(packet)
      i++
    }
    var cleanData = this.formatData(retData)
    return cleanData
  }
  //@TODO Add interface
  formatData(data) {
    var formattedData = {}
    data.forEach((item) => {
      var cleanData = AWS.DynamoDB.Converter.unmarshall(item)
      Object.keys(cleanData["Results"]).forEach(testCase => {
        if (formattedData[testCase] === undefined) {
          formattedData[testCase] = {}
        }
        Object.keys(cleanData["Results"][testCase]).forEach(metric=>{
          if (formattedData[testCase][metric] === undefined) {
            formattedData[testCase][metric] = []
          }
          var newStructure = cleanData["Results"][testCase][metric]
          GENERAL_ATTRIBUTES.forEach((generalAttribute) => {
            if (generalAttribute === "Hash") {
              if (cleanData[generalAttribute].length > 7) {
                newStructure[generalAttribute] = cleanData[generalAttribute].substring(0, 7)
                newStructure[LINK] = `${REPO_LINK}/commit/${cleanData[generalAttribute]}`
                return
              }
            }
            newStructure[generalAttribute] = cleanData[generalAttribute]
          })
          formattedData[testCase][metric].push(newStructure)
          
        })
      })

    })
    return formattedData

  }
//@TODO Add interface
cacheClear() {
  localStorage.clear()
  document.location.reload()
}
cacheGetAllData() {
  return JSON.parse(localStorage.getItem(CWAData))
}
cacheGetLatestItem() {
  return JSON.parse(localStorage.getItem(LATEST_ITEM))
}
cacheSaveData() {
  if (this.latestItem == null || this.CWAData == null) {
    console.warn("Items are null")
    return
  }
  localStorage.setItem(LATEST_ITEM, JSON.stringify(this.latestItem))
  localStorage.setItem(CWAData, JSON.stringify(this.CWAData))
  if (DEBUG) {
    console.log(` CACHE SAVE DATA: \n Latest: ${this.cacheGetLatestItem()}
        \nALL: ${localStorage.getItem(CWAData).length}
        
        `)
  }
}
}


export default Receiver


