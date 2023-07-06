// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
import moment from 'moment';
import { AxiosConfig } from '../../common/Axios';
import { OWNER_REPOSITORY, SERVICE_NAME, USE_CASE } from '../../common/Constant';
import { PerformanceMetricReport, PerformanceMetricReportParams, ServiceLatestVersion, ServicePRInformation } from './data.js';
export async function GetLatestPerformanceReports(password: string): Promise<PerformanceMetricReport[]> {
    return GetPerformanceReports(password, {
        TableName: process.env.REACT_APP_DYNAMODB_NAME,
        Limit: USE_CASE.length,
        IndexName: 'ServiceDate',
        KeyConditions: {
            Service: {
                ComparisonOperator: 'EQ',
                AttributeValueList: [
                    {
                        S: SERVICE_NAME,
                    },
                ],
            },
            CommitDate: {
                ComparisonOperator: 'LE',
                AttributeValueList: [
                    {
                        N: moment().unix().toString(),
                    },
                ],
            },
        },
        ScanIndexForward: false,
    });
}

async function GetPerformanceReports(password: string, params: PerformanceMetricReportParams): Promise<PerformanceMetricReport[]> {
    AxiosConfig.defaults.headers['x-api-key'] = password;
    return AxiosConfig.post('/', {
        Action: 'Query',
        Params: params,
    })
        .then(function (body: { data: { Items: PerformanceMetricReport[] } }) {
            return body?.data?.Items;
        })
        .catch(function (error: unknown) {
            return Promise.reject(error);
        });
}

export async function GetServiceLatestVersion(password: string): Promise<ServiceLatestVersion> {
    AxiosConfig.defaults.headers['x-api-key'] = password;
    return AxiosConfig.post('/', {
        Action: 'Github',
        URL: 'GET /repos/{owner}/{repo}/releases/latest',
        Params: {
            owner: OWNER_REPOSITORY,
            repo: process.env.REACT_APP_GITHUB_REPOSITORY,
        },
    })
        .then(function (body: { data: { data:  any }}) {
            return body?.data?.data;
        })
        .catch(function (error: unknown) {
            return Promise.reject(error);
        });
}

export async function GetServicePRInformation(password: string, commit_sha: string): Promise<ServicePRInformation> {
    AxiosConfig.defaults.headers['x-api-key'] = password;
    return AxiosConfig.post('/', {
        Action: 'Github',
        URL: 'GET /repos/{owner}/{repo}/commits/{commit_sha}/pulls',
        Params: {
            owner: OWNER_REPOSITORY,
            repo: process.env.REACT_APP_GITHUB_REPOSITORY,
            commit_sha: commit_sha,
        },
    })
        .then(function (body: { data: any[] }) {
            return Promise.resolve(body.data.at(0));
        })
        .catch(function (error: unknown) {
            return Promise.reject(error);
        });
}
