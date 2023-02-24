export interface PerformanceTrendDataParams {
    // DynamoDB holds metrics
    TableName: string;
  
    //
    IndexName: string;
  
    // Number of items going to take from DynamoDB
    Limit: number;
  
    // Getting metrics based on certain conditions
    KeyConditions: {
      Service: { ComparisonOperator: "EQ"; AttributeValueList: { S: string }[] };
      CommitDate: { ComparisonOperator: "EQ" | "LE"; AttributeValueList: { N: string }[] };
    };
  
    ScanIndexForward: boolean;
  }
  
  export interface PerformanceTrendData {
    CollectionPeriod: { S: string };
  
    CommitDate: { N: string };
  
    CommitHash: { S: string };
  
    DataType: { S: string };
  
    InstanceAMI: { S: string };
    InstanceType: { S: string };
  
    Results: {
      M: { [data_rate: string]: { M: { [metric_name: string]: { M: PerformanceMetricStatistic } } } };
    };
  
    UseCase: { S: string };
  }
  
  export interface TrendData {
    name: string;
    data_type: string;
    data_tpm: number;
    data_series: {
      name: string;
      data: number[];
    }[];
  }
  
  export interface PerformanceMetricStatistic {
    Average?: { N: string };
    Period?: { N: string };
    P99?: { N: string };
    Std?: { N: string };
    Min?: { N: string };
    Max?: { N: string };
  }
  
  export interface ServiceCommitInformation {
    // Release version for the service
    author: { login: string };
    commit: { message: string; committer: { date: string } };
    sha: string;
  }
  
  export interface CommitInformation {
    commiter_name: string;
    commit_message: string;
    commit_date: string;
    sha: string;
  }