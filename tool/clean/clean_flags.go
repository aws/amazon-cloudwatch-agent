// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package clean

import (
	"flag"
	"fmt"
	"log"
)

// DryRun reports whether cleaners should only log the resources they would
// delete instead of deleting them. It is populated by RegisterCommonFlags once
// flag.Parse has run.
var DryRun bool

// RegisterCommonFlags registers the flags shared by every cleaner on the
// default flag.CommandLine set. Call it from main before flag.Parse. Callers
// may register additional flags of their own before parsing.
//
//	-dry-run  when true, no resource is deleted; intended deletions are logged.
//	-tags     accepted and ignored. The clean-aws-resources workflow passes
//	          -tags as a go *build* flag (before the package path), so programs
//	          never receive it. This is defensive for manual/legacy invocations
//	          like `clean_x.go --tags=clean` that would otherwise pass it as a
//	          program argument and make flag.Parse reject the unknown flag.
func RegisterCommonFlags() {
	flag.BoolVar(&DryRun, "dry-run", false, "log the resources that would be deleted without deleting them")
	_ = flag.String("tags", "", "ignored; defensive for manual `--tags=clean` invocations")
}

// Skip logs an intended mutating action and reports whether it should be
// skipped because dry-run mode is enabled. Use it to gate every mutating call
// (delete/terminate/release/revoke/detach), for example:
//
//	if clean.Skip("delete volume %s", id) {
//		continue
//	}
//	// ... perform the real deletion ...
func Skip(action string, args ...any) bool {
	if DryRun {
		// Forward action as the format string (rather than concatenating a prefix
		// into it) so `go vet` recognizes Skip as a printf wrapper and validates
		// format/arg mismatches at every call site.
		log.Print("[dry-run] would ", fmt.Sprintf(action, args...))
		return true
	}
	return false
}
