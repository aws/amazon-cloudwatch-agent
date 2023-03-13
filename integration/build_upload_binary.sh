#!/bin/sh
# builds the binary and uploads to s3, using the config_ignore.json

configPath="./config_ignore.json"
s3Bucket=$(cat $configPath | python3 -c "import sys, json; print(json.load(sys.stdin)['s3Bucket'])")
cwaGithubSha=$(cat $configPath | python3 -c "import sys, json; print(json.load(sys.stdin)['cwaGithubSha'])")

echo "s3Bucket=$s3Bucket"
echo "cwaGithubSha=$cwaGithubSha"

cd ..
echo "Building agent"
make build package-rpm package-deb package-win
echo "Uploading binary to S3"
aws s3 cp ./build/bin "s3://$s3Bucket/integration-test/binary/$cwaGithubSha" --recursive

