/// <reference types="react-scripts" />

declare namespace NodeJS {
    interface ProcessEnv {
        NODE_ENV: 'development' | 'production' | 'test';
        PUBLIC_URL: string;
        REACT_APP_LAMBDA_URL: string;
        REACT_APP_DYNAMODB_NAME: string;
        REACT_APP_GITHUB_REPOSITORY: string;
    }
}
