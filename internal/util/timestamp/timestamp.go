// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package timestamp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

/*
Strftime-to-regex and strftime-to-Go-layout mappings for timestamp parsing.

Go's reference time is: "Mon Jan 2 15:04:05 MST 2006"
Based on https://golang.org/src/time/format.go and http://strftime.org/

Directive | Go layout   | Regex              | Meaning
----------|-------------|--------------------|---------
%B        | January     | \w{7}              | Full month name
%b        | Jan         | \w{3}              | Abbreviated month name
%-m       | 1           | \s{0,1}\d{1,2}     | Month (no leading zero)
%m        | 01          | \s{0,1}\d{1,2}     | Month (zero-padded)
%A        | Monday      | \w{6,9}            | Full weekday name
%a        | Mon         | \w{3}              | Abbreviated weekday name
%-d       | _2          | \s{0,1}\d{1,2}     | Day (no leading zero)
%d        | _2          | \s{0,1}\d{1,2}     | Day (zero-padded)
%H        | 15          | \d{2}              | Hour (24h, zero-padded)
%-I       | 3           | \d{1,2}            | Hour (12h, no leading zero)
%I        | 03          | \d{2}              | Hour (12h, zero-padded)
%-M       | 4           | \d{1,2}            | Minute (no leading zero)
%M        | 04          | \d{2}              | Minute (zero-padded)
%-S       | 5           | \d{1,2}            | Second (no leading zero)
%S        | 05          | \d{2}              | Second (zero-padded)
%Y        | 2006        | \d{4}              | Year (4 digit)
%y        | 06          | \d{2}              | Year (2 digit)
%p        | PM          | \w{2}              | AM/PM
%Z        | MST         | \w{3}              | Timezone name
%z        | -0700       | [+-]\d{4}          | Timezone offset
%f        | .000        | \d{1,9}            | Fractional seconds
%s        | epoch       | -?\d+(?:\.\d+)?    | Unix epoch (see ParseEpoch)
*/

const (
	// LayoutEpoch is the timestamp_layout value for a Unix epoch timestamp_format (%s).
	// time.Parse cannot read epoch numbers, so callers must use ParseEpoch.
	LayoutEpoch = "epoch"
	// epochNumber matches an integer or fractional Unix timestamp.
	epochNumber = `-?\d+(?:\.\d+)?`
	// Unit boundaries for an integer epoch value. A fractional value is always seconds.
	epochMillis int64 = 100_000_000_000         // 1e11
	epochMicros int64 = 100_000_000_000_000     // 1e14
	epochNanos  int64 = 100_000_000_000_000_000 // 1e17
)

var epochJSONKeys = []string{"timestamp", "time", "@timestamp"}

// IsEpochFormat reports whether format is a Unix epoch timestamp_format.
func IsEpochFormat(format string) bool {
	switch strings.TrimSpace(format) {
	case "%s", "%s.%f", "%s%f":
		return true
	default:
		return false
	}
}

// EpochStanzaLayout maps an epoch timestamp_format to a stanza epoch layout.
// %s is integer seconds. %s.%f and %s%f are seconds with a fractional part.
func EpochStanzaLayout(format string) (string, bool) {
	switch strings.TrimSpace(format) {
	case "%s":
		return "s", true
	case "%s.%f", "%s%f":
		return "s.ns", true
	default:
		return "", false
	}
}

// ParseEpoch converts a Unix epoch string into a UTC time.
// A value containing '.' is seconds plus a fractional part.
// An integer is seconds, milliseconds, microseconds, or nanoseconds,
// chosen from its magnitude.
func ParseEpoch(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty epoch timestamp")
	}
	if dot := strings.IndexByte(value, '.'); dot >= 0 {
		sec, err := strconv.ParseInt(value[:dot], 10, 64)
		if err != nil || dot == len(value)-1 {
			return time.Time{}, fmt.Errorf("invalid epoch timestamp %q", value)
		}
		frac := value[dot+1:]
		if len(frac) > 9 {
			frac = frac[:9]
		}
		nsec, err := strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid epoch timestamp %q", value)
		}
		for i := len(frac); i < 9; i++ {
			nsec *= 10
		}
		return time.Unix(sec, nsec).UTC(), nil
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid epoch timestamp %q", value)
	}
	mag := n
	if mag < 0 {
		mag = -mag
	}
	switch {
	case mag < epochMillis:
		return time.Unix(n, 0).UTC(), nil
	case mag < epochMicros:
		return time.UnixMilli(n).UTC(), nil
	case mag < epochNanos:
		return time.UnixMicro(n).UTC(), nil
	default:
		return time.Unix(0, n).UTC(), nil
	}
}

// ParseEpochJSON reads an epoch timestamp from a JSON object.
// The first usable value among "timestamp", "time", and "@timestamp" wins.
func ParseEpochJSON(line string) (time.Time, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || trimmed[0] != '{' {
		return time.Time{}, false
	}
	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return time.Time{}, false
	}
	for _, key := range epochJSONKeys {
		v, ok := obj[key]
		if !ok {
			continue
		}
		if t, ok := epochValue(v); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

func epochValue(v any) (time.Time, bool) {
	switch n := v.(type) {
	case json.Number:
		t, err := ParseEpoch(n.String())
		return t, err == nil
	case string:
		t, err := ParseEpoch(n)
		return t, err == nil
	default:
		return time.Time{}, false
	}
}

// FormatRegexMap maps strftime directives to regex patterns for timestamp extraction.
var FormatRegexMap = map[string]string{
	"%B":  `\w{7}`,
	"%b":  `\w{3}`,
	"%-m": `\s{0,1}\d{1,2}`,
	"%m":  `\s{0,1}\d{1,2}`,
	"%A":  `\w{6,9}`,
	"%a":  `\w{3}`,
	"%-d": `\s{0,1}\d{1,2}`,
	"%d":  `\s{0,1}\d{1,2}`,
	"%H":  `\d{2}`,
	"%-I": `\d{1,2}`,
	"%I":  `\d{2}`,
	"%-M": `\d{1,2}`,
	"%M":  `\d{2}`,
	"%-S": `\d{1,2}`,
	"%S":  `\d{2}`,
	"%Y":  `\d{4}`,
	"%y":  `\d{2}`,
	"%p":  `\w{2}`,
	"%Z":  `\w{3}`,
	"%z":  `[+-]\d{4}`,
	"%f":  `(\d{1,9})`,
}

// FormatLayoutMap maps strftime directives to Go reference time layout strings.
var FormatLayoutMap = map[string]string{
	"%B":  "January",
	"%b":  "Jan",
	"%-m": "1",
	"%m":  "01",
	"%A":  "Monday",
	"%a":  "Mon",
	"%-d": "_2",
	"%d":  "_2",
	"%H":  "15",
	"%-I": "3",
	"%I":  "03",
	"%-M": "4",
	"%M":  "04",
	"%-S": "5",
	"%S":  "05",
	"%Y":  "2006",
	"%y":  "06",
	"%p":  "PM",
	"%Z":  "MST",
	"%z":  "-0700",
	"%f":  ".000",
}

// RegexEscapeMap escapes characters that are special in regex but normal in the
// user's timestamp format string.
var RegexEscapeMap = map[string]string{
	"^": `\^`,
	".": `\.`,
	"*": `\*`,
	"?": `\?`,
	"+": `\+`,
	"|": `\|`,
	"[": `\[`,
	"]": `\]`,
	"(": `\(`,
	")": `\)`,
	"{": `\{`,
	"}": `\}`,
	"$": `\$`,
}

// BuildRegexWithNamedCaptureGroup converts a strftime format string to a regex with a named capture group (?P<timestamp>...).
func BuildRegexWithNamedCaptureGroup(format string) string {
	return `(?P<timestamp>` + BuildRegex(format) + `)`
}

// BuildRegex converts a strftime format string to a regex pattern.
// If the format starts with "%-m" or "%-d", the leading \s{0,1} is stripped because:
//
//	"%-m %-d %H:%M:%S" would produce regex "(\s{0,1}\d{1,2} \s{0,1}\d{1,2} \d{2}:\d{2}:\d{2})"
//	and layout "1 _2 15:04:05". The timestamp " 2 1 07:10:06" matches the regex but not the
//	layout. Stripping the prefix makes the regex and layout consistent.
func BuildRegex(format string) string {
	if IsEpochFormat(format) {
		return epochNumber
	}
	res := ReplaceAll(format, RegexEscapeMap)
	res = ReplaceAll(res, FormatRegexMap)
	res = strings.TrimPrefix(res, `\s{0,1}`)
	return res
}

// BuildLayout converts a strftime format string to a Go time layout string.
func BuildLayout(format string) string {
	if IsEpochFormat(format) {
		return LayoutEpoch
	}
	res := format
	// %f needs variable-width fractional seconds (".999999999") instead of FormatLayoutMap's
	// fixed ".000". Handle both ".%f" and "%f" since %f includes the dot separator.
	res = strings.ReplaceAll(res, ".%f", ".999999999")
	res = strings.ReplaceAll(res, "%f", ".999999999")
	// Use lenient layout "1" for month (accepts both "1" and "01"), avoiding v1's
	// two-layout approach where both %m ("01") and %-m ("1") variants were emitted.
	// %-m already maps to "1" in FormatLayoutMap so only %m needs overriding.
	res = strings.ReplaceAll(res, "%m", "%-m")
	return ReplaceAll(res, FormatLayoutMap)
}

// ReplaceAll replaces all occurrences of keys in the replacements map with their values.
func ReplaceAll(input string, replacements map[string]string) string {
	res := input
	for k, v := range replacements {
		if strings.Contains(res, k) {
			res = strings.ReplaceAll(res, k, v)
		}
	}
	return res
}
