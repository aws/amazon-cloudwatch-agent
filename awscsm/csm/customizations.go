package csm

import (
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"
)

func init() {
	initClient = defaultInitClientFn
}

func defaultInitClientFn(c *client.Client) {
	c.Handlers.Build.PushBack(overrideAppJSONContentType)
}

func overrideAppJSONContentType(r *request.Request) {
	origCT := r.HTTPRequest.Header.Get("Content-Type")
	if origCT != "application/json" {
		return
	}

	r.HTTPRequest.Header.Set("Content-Type", "application/x-amz-json-1.1")
}
