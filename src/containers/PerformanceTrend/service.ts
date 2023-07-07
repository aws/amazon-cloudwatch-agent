// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

import moment from 'moment';
import { AxiosConfig } from '../../common/Axios';
import { OWNER_REPOSITORY, SERVICE_NAME, USE_CASE } from '../../common/Constant';
import { PerformanceTrendData, PerformanceTrendDataParams, ServiceCommitInformation } from './data';
export async function GetPerformanceTrendData(password: string): Promise<PerformanceTrendData[]> {
    const currentUnixTime = moment().unix();
    return GetPerformanceTrend(password, {
        TableName: process.env.REACT_APP_DYNAMODB_NAME,
        Limit: USE_CASE.length * 25,
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
                        N: currentUnixTime.toString(),
                    },
                ],
            },
        },
        ScanIndexForward: false,
    });
}

async function GetPerformanceTrend(password: string, params: PerformanceTrendDataParams): Promise<PerformanceTrendData[]> {
    AxiosConfig.defaults.headers['x-api-key'] = password;
    return AxiosConfig.post('/', {
        Action: 'Query',
        Params: params,
    })
        .then(function (body: { data: { Items: any[] } }) {
            return body?.data?.Items;
        })
        .catch(function (error: unknown) {
            return Promise.reject(error);
        });
}

export async function GetServiceCommitInformation(password: string, commit_sha: string): Promise<ServiceCommitInformation> {
    AxiosConfig.defaults.headers['x-api-key'] = password;
    return AxiosConfig.post('/', {
        Action: 'Github',
        URL: 'GET /repos/{owner}/{repo}/commits/{ref}',
        Params: {
            owner: OWNER_REPOSITORY,
            repo: process.env.REACT_APP_GITHUB_REPOSITORY,
            ref: commit_sha,
        },
    })
        .then(function (value: { data: any }) {
            return Promise.resolve(value?.data);
        })
        .catch(function (error: unknown) {
            return Promise.reject(error);
        });
}
