package handlers

import "github.com/aws/aws-sdk-go/aws/request"

func NewRetryErrorCodeHandler(errorCodes []string) request.NamedHandler {
	return request.NamedHandler{
		Name: "RetryErrorCodeHandler",
		Fn: func(req *request.Request) {
			req.RetryErrorCodes = append(req.RetryErrorCodes, errorCodes...)
		},
	}
}

func NewThrottleErrorCodeHandler(errorCodes []string) request.NamedHandler {
	return request.NamedHandler{
		Name: "ThrottleErrorCodeHandler",
		Fn: func(req *request.Request) {
			req.ThrottleErrorCodes = append(req.ThrottleErrorCodes, errorCodes...)
		},
	}
}
