// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package xraydaemonmigration

import (
	"flag"
	"fmt"
)

type Flag struct {
	// A set of flags used for cli configuration.
	fs *flag.FlagSet

	// String array used to display flag information on cli.
	cliStrings []string
}

// StringVarF defines 2 string flags for specified name and shortName, default value, and usage string.
// The argument ptr points to a string variable in which to store the value of the flag.
func (f *Flag) StringVarF(ptr *string, name string, shortName string, value string, usage string) {
	f.fs.StringVar(ptr, name, value, usage)
	f.fs.StringVar(ptr, shortName, value, usage)
	var s string
	if len(name) <= 4 {
		s = fmt.Sprintf("\t-%v\t--%v\t\t%v", shortName, name, usage)
	} else {
		s = fmt.Sprintf("\t-%v\t--%v\t%v", shortName, name, usage)
	}
	f.cliStrings = append(f.cliStrings, s)
}

// IntVarF defines 2 int flags for specified name and shortName with default value, and usage string.
// The argument ptr points to an int variable in which to store the value of the flag.
func (f *Flag) IntVarF(ptr *int, name string, shortName string, value int, usage string) {
	f.fs.IntVar(ptr, name, value, usage)
	f.fs.IntVar(ptr, shortName, value, usage)
	s := fmt.Sprintf("\t-%v\t--%v\t%v", shortName, name, usage)
	f.cliStrings = append(f.cliStrings, s)
}

// BoolVarF defines 2 bool flags with specified name and shortName, default value, and usage string.
// The argument ptr points to a bool variable in which to store the value of the flag.
func (f *Flag) BoolVarF(ptr *bool, name string, shortName string, value bool, usage string) {
	f.fs.BoolVar(ptr, name, value, usage)
	f.fs.BoolVar(ptr, shortName, value, usage)
	s := fmt.Sprintf("\t-%v\t--%v\t%v", shortName, name, usage)
	f.cliStrings = append(f.cliStrings, s)
}

// NewFlag returns a new flag with provided flag name.
func NewFlag(name string) *Flag {
	flag := &Flag{
		cliStrings: make([]string, 0, 19),
		fs:         flag.NewFlagSet(name, flag.ContinueOnError),
	}
	return flag
}
