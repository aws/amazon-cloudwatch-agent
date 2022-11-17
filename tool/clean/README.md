**What does the cleaner do?**

###Cleaner cleans out old ami (ami older than 60 days)

The cleaner first searches for ami names (these are the ami created by the pipeline for use int he integration tests)
1. cloudwatch-agent-integration-test*

Then checks to see if the creation date is greater than 60 days. (The aws sdk v2 gives creation date as a pointer to string. To convert to golang time we use the aws smithy go time. This allows us to compare to 60 days in past time)

If the ami is older than 60 days old then we delete the ami

###Cleans dedicated hosts for mac

The cleaner first searches for dedicated host tag Name:IntegrationTestMacDedicatedHost

Then checks to see if the creation date is greater than 26 hours and host status is available

Delete is true