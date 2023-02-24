export interface PerformanceMetricReportParams {
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
  
  export interface PerformanceMetricReport {
    CollectionPeriod: { S: string };
  
    CommitDate: { N: string };
  
    CommitHash: { S: string };
  
    DataType: { S: string };
  
    InstanceAMI: { S: string };
    InstanceType: { S: string };
  
    Results: {
      M: { [data_rate: string]: { M: PerformanceMetric } };
    };
  
    UseCase: { S: string };
  }
  
  // PerformanceMetric shows all collected metrics when running performance metrics
  export interface PerformanceMetric {
    procstat_cpu_usage?: { M: PerformanceMetricStatistic };
    procstat_memory_rss?: { M: PerformanceMetricStatistic };
    procstat_memory_swap?: { M: PerformanceMetricStatistic };
    procstat_memory_vms?: { M: PerformanceMetricStatistic };
    procstat_memory_data?: { M: PerformanceMetricStatistic };
    procstat_num_fds?: { M: PerformanceMetricStatistic };
    procstat_write_bytes?: { M: PerformanceMetricStatistic };
    net_bytes_sent?: { M: PerformanceMetricStatistic };
    net_packets_sent?: { M: PerformanceMetricStatistic };
    mem_total?: { M: PerformanceMetricStatistic };
  }
  
  export interface PerformanceMetricStatistic {
    Average?: { N: string };
    Period?: { N: string };
    P99?: { N: string };
    Std?: { N: string };
    Min?: { N: string };
    Max?: { N: string };
  }
  
  export interface ServiceLatestVersion {
    // Release version for the service
    tag_name: string;
  }
  
  export interface ServicePRInformation {
    // Release version for the service
    title: string;
    html_url: string;
    number: number;
  }
  
  export interface UseCaseData {
    name?: string;
    data_type?: string;
    instance_type?: string;
    data: {
      [data_rate: string]: {
        procstat_cpu_usage?: string;
        procstat_memory_rss?: string;
        procstat_memory_swap?: string;
        procstat_memory_vms?: string;
        procstat_memory_data?: string;
        procstat_num_fds?: string;
        procstat_write_bytes?: string;
        net_bytes_sent?: string;
        net_packets_sent?: string;
        mem_total?: string;
      };
    };
  }
  