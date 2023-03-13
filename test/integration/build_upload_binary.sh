#!/bin/sh
# builds the agent from the current commit hash and uploads to s3

configPath="./config.json"
cwaGithubSha=$( git rev-parse HEAD )
s3Bucket=$(cat $configPath | python3 -c "import sys, json; print(json.load(sys.stdin)['s3Bucket'])")
s3Url="s3://$s3Bucket/integration-test/binary/$cwaGithubSha"

echo "s3Bucket=$s3Bucket"
echo "cwaGithubSha=$cwaGithubSha"
echo "s3Url=$s3Url"

cd ../../
echo "Building agent"
make release
echo "Uploading binary to S3"
aws s3 cp ./build/bin $s3Url --recursive

