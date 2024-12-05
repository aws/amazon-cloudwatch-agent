// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

import axios from 'axios';

export const AxiosConfig = axios.create({
    baseURL: process.env.REACT_APP_LAMBDA_URL,
    timeout: 5000,
    headers: {
        'Content-Type': 'application/json',
    },
    responseType: 'json',
    maxRedirects: 21,
});
