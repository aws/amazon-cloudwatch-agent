// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

/*
The reference time used in the layouts in Golang is the specific time.
For example : "Mon Jan 2 15:04:05 MST 2006"

So the TimeFormatMap records time_format code and its corresponding Golang specific reference time.
And the TimeFormatRexMap records time_format code and its corresponding regax expression.
When process the user's input, the translator will translate the timestamp_format into the Golang reference time layout and the regax expression based on those two maps.

Based on https://golang.org/src/time/format.go and http://strftime.org/, here is the mapping below:
stdLongMonth                                   // "January"                                                         //%B
stdMonth                                       // "Jan"                                                             //%b
stdNumMonth                                    // "1"                                                               //%-m
stdZeroMonth                                   // "01"                                                              //%m
stdLongWeekDay                                 // "Monday"                                                          //%A
stdWeekDay                                     // "Mon"                                                             //%a
stdDay                                         // "2"                                                               //%-d
stdUnderDay                                    // "_2"                                                              //
stdZeroDay                                     // "02"                                                              //%d
stdHour                                        // "15"                                                              //%H
stdHour12                                      // "3"                                                               //%-I
stdZeroHour12                                  // "03"                                                              //%I
stdMinute                                      // "4"                                                               //%-M
stdZeroMinute                                  // "04"                                                              //%M
stdSecond                                      // "5"                                                               //%-S
stdZeroSecond                                  // "05"                                                              //%S
stdLongYear                                    // "2006"                                                            //%Y
stdYear                                        // "06"                                                              //%y
stdPM                                          // "PM"                                                              //%p
stdpm                                          // "pm"                                                              //
stdTZ                                          // "MST"                                                             //%Z
stdISO8601TZ                                   // "Z0700"  // prints Z for UTC
stdISO8601SecondsTZ                            // "Z070000"
stdISO8601ShortTZ                              // "Z07"
stdISO8601ColonTZ                              // "Z07:00" // prints Z for UTC
stdISO8601ColonSecondsTZ                       // "Z07:00:00"
stdNumTZ                                       // "-0700"  // always numeric                                        //%z
stdNumSecondsTz                                // "-070000"
stdNumShortTZ                                  // "-07"    // always numeric
stdNumColonTZ                                  // "-07:00" // always numeric
stdNumColonSecondsTZ                           // "-07:00:00"
stdFracSecond0                                 // ".0", ".00", ... , trailing zeros included
stdFracSecond9                                 // ".9", ".99", ..., trailing zeros omitted
*/
var TimeFormatMap = map[string]string{
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

var TimeFormatRexMap = map[string]string{
	"%B":  "\\w{7}",
	"%b":  "\\w{3}",
	"%-m": "\\s{0,1}\\d{1,2}",
	"%m":  "\\s{0,1}\\d{1,2}",
	"%A":  "\\w{6,9}",
	"%a":  "\\w{3}",
	"%-d": "\\s{0,1}\\d{1,2}",
	"%d":  "\\s{0,1}\\d{1,2}",
	"%H":  "\\d{2}",
	"%-I": "\\d{1,2}",
	"%I":  "\\d{2}",
	"%-M": "\\d{1,2}",
	"%M":  "\\d{2}",
	"%-S": "\\d{1,2}",
	"%S":  "\\d{2}",
	"%Y":  "\\d{4}",
	"%y":  "\\d{2}",
	"%p":  "\\w{2}",
	"%Z":  "\\w{3}",
	"%z":  "[\\+-]\\d{4}",
	"%f":  "(\\d{1,9})",
}

// The characters required to be escaped are these characters special in regex, but normal in json.
// Characters are special in regex:
// ^ . * ? + - \ | [ ] ( ) { } $
// + is already part of the timestamp format
// - is not required to be escaped when not inside [].
// \ is already required to be escaped in json too.
// The remaining characters are:
// ^ . * ? | [ ] ( ) { } $
var TimeFormatRegexEscapeMap = map[string]string{
	"^": "\\^",
	".": "\\.",
	"*": "\\*",
	"?": "\\?",
	"|": "\\|",
	"[": "\\[",
	"]": "\\]",
	"(": "\\(",
	")": "\\)",
	"{": "\\{",
	"}": "\\}",
	"$": "\\$",
}

func checkAndReplace(input string, timestampFormatMap map[string]string) string {
	res := input
	for k, v := range timestampFormatMap {
		if strings.Contains(input, k) {
			res = strings.Replace(res, k, v, -1)
		}
	}
	return res
}

type TimestampRegex struct {
}

// ApplyRule add timestamp regex
// do not add timestamp check when viewing cwa logfile
func (t *TimestampRegex) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	//Convert the input string into []rune and iterate the map and build the output []rune
	m := input.(map[string]interface{})
	//If user not specify the timestamp_format, then no config entry for "timestamp_layout" in TOML
	if val, ok := m["timestamp_format"]; !ok {
		return "", ""
	} else if m["file_path"] == context.CurrentContext().GetAgentLogFile() {
		fmt.Printf("timestamp_format set file_path : %s is the same as agent log file %s thus do not use timestamp_regex \n", m["file_path"], context.CurrentContext().GetAgentLogFile())
		return "", ""
	} else {
		//If user provide with the specific timestamp_format, use the one that user provide
		res := checkAndReplace(val.(string), TimeFormatRegexEscapeMap)
		res = checkAndReplace(res, TimeFormatRexMap)
		// remove the prefix, if the format startswith "%-m" or "%-d", there is an "\\s{0,1}" at the beginning.
		// like "timestamp_format": "%-m %-d %H:%M:%S" will be converted into following layout and regex
		//      timestamp_layout = ["1 _2 15:04:05"]
		//      timestamp_regex = "(\\s{0,1}\\d{1,2} \\s{0,1}\\d{1,2} \\d{2}:\\d{2}:\\d{2})"
		// following timestamp string " 2 1 07:10:06" matches the regex, but it can not match the layout.
		// After the prefix "\\s{0,1}", it can match both the regex and layout.
		res = strings.TrimPrefix(res, "\\s{0,1}")
		res = "(" + res + ")"
		returnKey = "timestamp_regex"
		if _, err := regexp.Compile(res); err != nil {
			translator.AddErrorMessages(GetCurPath()+"timestamp_format", fmt.Sprintf("Timestamp format %s is invalid", val))
			return
		}
		returnVal = res
	}
	return
}

type TimestampLayout struct {
}

// ApplyRule add timestamp layout
// do not add timestamp check when viewing cwa logfile
func (t *TimestampLayout) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	//Convert the input string into []rune and iterate the map and build the output []rune
	m := input.(map[string]interface{})
	//If user not specify the timestamp_format, then no config entry for "timestamp_layout" in TOML
	if val, ok := m["timestamp_format"]; !ok {
		return "", ""
	} else if m["file_path"] == context.CurrentContext().GetAgentLogFile() {
		fmt.Printf("timestamp_format set file_path : %s is the same as agent log file %s thus do not use timestamp_layout \n", m["file_path"], context.CurrentContext().GetAgentLogFile())
		return "", ""
	} else {
		res := checkAndReplace(val.(string), TimeFormatMap)
		//If user provide with the specific timestamp_format, use the one that user provide
		returnKey = "timestamp_layout"
		timestampInput := val.(string)
		// Go doesn't support _2 option for month in day as a result need to set
		// timestamp_layout with 2 strings which support %m and %-m
		if strings.Contains(timestampInput, "%m") {
			timestampInput := strings.Replace(timestampInput, "%m", "%-m", -1)
			alternativeLayout := checkAndReplace(timestampInput, TimeFormatMap)
			returnVal = []string{res, alternativeLayout}
		} else if strings.Contains(timestampInput, "%-m") {
			timestampInput = strings.Replace(timestampInput, "%-m", "%m", -1)
			alternativeLayout := checkAndReplace(timestampInput, TimeFormatMap)
			returnVal = []string{res, alternativeLayout}
		} else {
			returnVal = []string{res}
		}
	}
	return
}

type Timezone struct {
}

func (t *Timezone) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	if val, ok := m["timezone"]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		//If user provide with the specific timestamp_format, use the one that user provide
		returnKey = "timezone"
		if val == "UTC" {
			returnVal = "UTC"
		} else {
			returnVal = "LOCAL"
		}
	}
	return
}
func init() {
	t1 := new(TimestampLayout)
	t2 := new(TimestampRegex)
	t3 := new(Timezone)
	r := []Rule{t1, t2, t3}
	RegisterRule("timestamp_format", r)
}
