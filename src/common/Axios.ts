// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

import axios from 'axios';

export const AxiosConfig = axios.create({
    baseURL: process.env.REACT_APP_LAMBDA_URL,
    // timeout in milliseconds; increased from 3000ms due to large number of commit data requests
    timeout: 4000,
    headers: {
        'Content-Type': 'application/json',
    },
    responseType: 'json',
    maxRedirects: 21,
});
