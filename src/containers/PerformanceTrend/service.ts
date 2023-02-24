import moment from "moment";
import { AxionConfig, OctokitConfig } from "../../common/Axios";
import { OWNER_REPOSITORY, SERVICE_NAME, USE_CASE } from "../../common/Constant";
import { PerformanceTrendData, PerformanceTrendDataParams, ServiceCommitInformation } from "./data";
export async function GetPerformanceTrendData(): Promise<PerformanceTrendData[]> {
  const currentUnixTime = moment().unix();
  return GetPerformanceTrend({
    TableName: process.env.REACT_APP_DYNAMODB_NAME || "",
    Limit: USE_CASE.length * 25,
    IndexName: "ServiceDate",
    KeyConditions: {
      Service: {
        ComparisonOperator: "EQ",
        AttributeValueList: [
          {
            S: SERVICE_NAME,
          },
        ],
      },
      CommitDate: {
        ComparisonOperator: "LE",
        AttributeValueList: [
          {
            N: currentUnixTime.toString(),
          },
        ],
      },
    },
    ScanIndexForward: false,
  });
}

async function GetPerformanceTrend(params: PerformanceTrendDataParams): Promise<PerformanceTrendData[]> {
  return AxionConfig.post("/", { Action: "Query", Params: params })
    .then(function (body: { data: { Items: any[] } }) {
      return body?.data?.Items;
    })
    .catch(function (error: unknown) {
      return Promise.reject(error);
    });
}

export async function GetServiceCommitInformation(commit_sha: string): Promise<ServiceCommitInformation> {
  return OctokitConfig.request("GET /repos/{owner}/{repo}/commits/{ref}", {
    owner: OWNER_REPOSITORY,
    repo: process.env.REACT_APP_GITHUB_REPOSITORY || "",
    ref: commit_sha,
  })
    .then(function (value: { data: any }) {
      return Promise.resolve(value?.data);
    })
    .catch(function (error: unknown) {
      return Promise.reject(error);
    });
}