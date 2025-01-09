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
        .then(function (body: { data: { data: any } }) {
            return body?.data?.data;
        })
        .catch(function (error: unknown) {
            return Promise.reject(error);
        });
}

export async function GetServicePRInformation(password: string, commitHashes: string[]): Promise<ServicePRInformation[]> {
    try {
        AxiosConfig.defaults.headers['x-api-key'] = password;
        const prInformation = commitHashes.map(async (commitHash) => {
            const result = await AxiosConfig.post('/', {
                Action: 'Github',
                URL: 'GET /repos/{owner}/{repo}/commits/{commit_sha}/pulls',
                Params: {
                    owner: OWNER_REPOSITORY,
                    repo: process.env.REACT_APP_GITHUB_REPOSITORY,
                    commit_sha: commitHash,
                },
            });
            if (result.data?.data?.length === undefined) {
                console.log('PR Info not found for: ' + commitHash);
                return undefined;
            }

            return result.data?.data?.at(0);
        });

        return Promise.all(prInformation);
    } catch (error) {
        return Promise.reject(error);
    }
}

export function createDefaultServicePRInformation(): ServicePRInformation {
    return {
        title: 'PR data unavailable',
        html_url: 'https://github.com/aws/amazon-cloudwatch-agent/pulls',
        number: 0,
        sha: 'default-sha',
    };
}
