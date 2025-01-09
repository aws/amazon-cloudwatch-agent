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

export async function GetServiceCommitInformation(password: string, commitSha: string): Promise<ServiceCommitInformation> {
    try {
        AxiosConfig.defaults.headers['x-api-key'] = password;
        const response = await AxiosConfig.post('/', {
            Action: 'Github',
            URL: 'GET /repos/{owner}/{repo}/commits/{ref}',
            Params: {
                owner: OWNER_REPOSITORY,
                repo: process.env.REACT_APP_GITHUB_REPOSITORY,
                ref: commitSha,
            },
        });

        // Validate response
        if (!response?.data?.data) {
            return createDefaultServiceCommitInformation();
        }

        return response.data.data;
    } catch (error) {
        console.error('Failed to fetch commit information:', error);
        throw error; // Re-throw the error for handling by the caller
    }
}

function createDefaultServiceCommitInformation(): ServiceCommitInformation {
    return {
        author: {
            login: 'default-user',
        },
        commit: {
            message: 'No commit message available',
            author: {
                date: new Date().toISOString(),
            },
        },
        sha: 'default-sha',
    };
}
