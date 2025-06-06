// Code generated by private/model/cli/gen-api/main.go. DO NOT EDIT.

package cloudwatchlogs

import (
	"github.com/aws/aws-sdk-go/private/protocol"
)

const (

	// ErrCodeAccessDeniedException for service response error code
	// "AccessDeniedException".
	//
	// You don't have sufficient permissions to perform this action.
	ErrCodeAccessDeniedException = "AccessDeniedException"

	// ErrCodeConflictException for service response error code
	// "ConflictException".
	//
	// This operation attempted to create a resource that already exists.
	ErrCodeConflictException = "ConflictException"

	// ErrCodeDataAlreadyAcceptedException for service response error code
	// "DataAlreadyAcceptedException".
	//
	// The event was already logged.
	//
	// PutLogEvents actions are now always accepted and never return DataAlreadyAcceptedException
	// regardless of whether a given batch of log events has already been accepted.
	ErrCodeDataAlreadyAcceptedException = "DataAlreadyAcceptedException"

	// ErrCodeInvalidOperationException for service response error code
	// "InvalidOperationException".
	//
	// The operation is not valid on the specified resource.
	ErrCodeInvalidOperationException = "InvalidOperationException"

	// ErrCodeInvalidParameterException for service response error code
	// "InvalidParameterException".
	//
	// A parameter is specified incorrectly.
	ErrCodeInvalidParameterException = "InvalidParameterException"

	// ErrCodeInvalidSequenceTokenException for service response error code
	// "InvalidSequenceTokenException".
	//
	// The sequence token is not valid. You can get the correct sequence token in
	// the expectedSequenceToken field in the InvalidSequenceTokenException message.
	//
	// PutLogEvents actions are now always accepted and never return InvalidSequenceTokenException
	// regardless of receiving an invalid sequence token.
	ErrCodeInvalidSequenceTokenException = "InvalidSequenceTokenException"

	// ErrCodeLimitExceededException for service response error code
	// "LimitExceededException".
	//
	// You have reached the maximum number of resources that can be created.
	ErrCodeLimitExceededException = "LimitExceededException"

	// ErrCodeMalformedQueryException for service response error code
	// "MalformedQueryException".
	//
	// The query string is not valid. Details about this error are displayed in
	// a QueryCompileError object. For more information, see QueryCompileError (https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_QueryCompileError.html).
	//
	// For more information about valid query syntax, see CloudWatch Logs Insights
	// Query Syntax (https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/CWL_QuerySyntax.html).
	ErrCodeMalformedQueryException = "MalformedQueryException"

	// ErrCodeOperationAbortedException for service response error code
	// "OperationAbortedException".
	//
	// Multiple concurrent requests to update the same resource were in conflict.
	ErrCodeOperationAbortedException = "OperationAbortedException"

	// ErrCodeResourceAlreadyExistsException for service response error code
	// "ResourceAlreadyExistsException".
	//
	// The specified resource already exists.
	ErrCodeResourceAlreadyExistsException = "ResourceAlreadyExistsException"

	// ErrCodeResourceNotFoundException for service response error code
	// "ResourceNotFoundException".
	//
	// The specified resource does not exist.
	ErrCodeResourceNotFoundException = "ResourceNotFoundException"

	// ErrCodeServiceQuotaExceededException for service response error code
	// "ServiceQuotaExceededException".
	//
	// This request exceeds a service quota.
	ErrCodeServiceQuotaExceededException = "ServiceQuotaExceededException"

	// ErrCodeServiceUnavailableException for service response error code
	// "ServiceUnavailableException".
	//
	// The service cannot complete the request.
	ErrCodeServiceUnavailableException = "ServiceUnavailableException"

	// ErrCodeSessionStreamingException for service response error code
	// "SessionStreamingException".
	//
	// This exception is returned if an unknown error occurs during a Live Tail
	// session.
	ErrCodeSessionStreamingException = "SessionStreamingException"

	// ErrCodeSessionTimeoutException for service response error code
	// "SessionTimeoutException".
	//
	// This exception is returned in a Live Tail stream when the Live Tail session
	// times out. Live Tail sessions time out after three hours.
	ErrCodeSessionTimeoutException = "SessionTimeoutException"

	// ErrCodeThrottlingException for service response error code
	// "ThrottlingException".
	//
	// The request was throttled because of quota limits.
	ErrCodeThrottlingException = "ThrottlingException"

	// ErrCodeTooManyTagsException for service response error code
	// "TooManyTagsException".
	//
	// A resource can have no more than 50 tags.
	ErrCodeTooManyTagsException = "TooManyTagsException"

	// ErrCodeUnrecognizedClientException for service response error code
	// "UnrecognizedClientException".
	//
	// The most likely cause is an Amazon Web Services access key ID or secret key
	// that's not valid.
	ErrCodeUnrecognizedClientException = "UnrecognizedClientException"

	// ErrCodeValidationException for service response error code
	// "ValidationException".
	//
	// One of the parameters for the request is not valid.
	ErrCodeValidationException = "ValidationException"
)

var exceptionFromCode = map[string]func(protocol.ResponseMetadata) error{
	"AccessDeniedException":          newErrorAccessDeniedException,
	"ConflictException":              newErrorConflictException,
	"DataAlreadyAcceptedException":   newErrorDataAlreadyAcceptedException,
	"InvalidOperationException":      newErrorInvalidOperationException,
	"InvalidParameterException":      newErrorInvalidParameterException,
	"InvalidSequenceTokenException":  newErrorInvalidSequenceTokenException,
	"LimitExceededException":         newErrorLimitExceededException,
	"MalformedQueryException":        newErrorMalformedQueryException,
	"OperationAbortedException":      newErrorOperationAbortedException,
	"ResourceAlreadyExistsException": newErrorResourceAlreadyExistsException,
	"ResourceNotFoundException":      newErrorResourceNotFoundException,
	"ServiceQuotaExceededException":  newErrorServiceQuotaExceededException,
	"ServiceUnavailableException":    newErrorServiceUnavailableException,
	"SessionStreamingException":      newErrorSessionStreamingException,
	"SessionTimeoutException":        newErrorSessionTimeoutException,
	"ThrottlingException":            newErrorThrottlingException,
	"TooManyTagsException":           newErrorTooManyTagsException,
	"UnrecognizedClientException":    newErrorUnrecognizedClientException,
	"ValidationException":            newErrorValidationException,
}
