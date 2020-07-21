// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package providers

type loopState int

const (
	loopStateNone = loopState(iota)
	loopStateContinue
	loopStateBreak
)

// ContinueError will allow for us to see if we need to continue
// in a loop
type ContinueError interface {
	Continue() bool
}

// BreakError ...
type BreakError interface {
	Break() bool
}

type loopControlError struct {
	message string
	state   loopState
}

// newLoopControlError ...
func newLoopControlError(message string, state loopState) loopControlError {
	return loopControlError{
		message,
		state,
	}
}

func (err loopControlError) Error() string {
	return err.message
}

func (err loopControlError) Continue() bool {
	return err.state == loopStateContinue
}

func (err loopControlError) Break() bool {
	return err.state == loopStateBreak
}
