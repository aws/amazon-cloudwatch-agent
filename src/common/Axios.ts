import { Octokit } from "@octokit/rest";
import axios from "axios";

export const AxionConfig = axios.create({
  baseURL:process.env.REACT_APP_DYNAMODB_URL,
  timeout: 3000,
  headers: {
    "x-api-key": process.env.REACT_APP_DYNAMODB_TOKEN,
  },
  responseType: "json",
  maxRedirects: 21,
});

export const OctokitConfig = new Octokit({
  auth: process.env.REACT_APP_GITHUB_SECRET_TOKEN,
});
