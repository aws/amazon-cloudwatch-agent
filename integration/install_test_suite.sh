s3Bucket=$(cat ./config_ignore.json | python3 -c "import sys, json; print(json.load(sys.stdin)['s3Bucket'])")
cwaGithubSha=$(cat ./config_ignore.json | python3 -c "import sys, json; print(json.load(sys.stdin)['cwaGithubSha'])")

cd ..
echo "Building agent"
make build package-rpm package-deb package-win
echo "Uploading binary to S3"
aws s3 cp ./build/bin "s3://$s3Bucket/integration-test/binary/$cwaGithubSha" --recursive

