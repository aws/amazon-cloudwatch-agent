# What does the cleaner do?

###Cleaner cleans out old ami (ami older than 60 days)

The cleaner first searches for ami names (these are the ami created by the pipeline for use int he integration tests)
1. cloudwatch-agent-integration-test*

Then checks to see if the creation date is greater than 60 days. (The aws sdk v2 gives creation date as a pointer to string. To convert to golang time we use the aws smithy go time. This allows us to compare to 60 days in past time)

If the ami is older than 30 days old as default (can be configured) then we delete the ami

###Cleans dedicated hosts for mac

The cleaner first searches for dedicated host tag Name:IntegrationTestMacDedicatedHost

Then checks to see if the creation date is greater than 30 days as default (can be configured) and host status is available

Delete is true

# How to use the script?
By running the below command **with or without replacing these variables**:
* **keep:** days to keep your resources (i.e 5 which is deleting resources after existing for 5 days)
* **clean:** resources need to be cleaned (only support dedicated_host, ssm, ami)
```
go run ./integration/clean/main.go \
    -keep {{ days to keep the resources }} \
    -clean {{ resources cleaning }}
```