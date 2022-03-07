# get the version
$version=$args[0]

# create msi
candle.exe -ext WixUtilExtension.dll ./amazon-cloudwatch-agent.wxs
light.exe -ext WixUtilExtension.dll ./amazon-cloudwatch-agent.wixobj

# upload to s3
aws s3 cp ./amazon-cloudwatch-agent.msi "s3://cloudwatch-agent-integration-bucket/integration-test/packaging/$version/amazon-cloudwatch-agent.msi"
Write-Host "s3 for msi is s3://cloudwatch-agent-integration-bucket/integration-test/packaging/$version/amazon-cloudwatch-agent.msi"