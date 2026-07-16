// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package timestamp

import "strings"

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
*/

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
	res := ReplaceAll(format, RegexEscapeMap)
	res = ReplaceAll(res, FormatRegexMap)
	res = strings.TrimPrefix(res, `\s{0,1}`)
	return res
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
