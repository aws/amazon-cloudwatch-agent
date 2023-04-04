// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Directly supported by charset and ianaindex.
// If the mapping here no longer exists, we should add into our override map.
var supportedEncodingByLib = map[string]string{
	"ascii":     "windows-1252",
	"big5":      "big5",
	"cp037":     "IBM037",
	"cp1250":    "windows-1250",
	"cp1251":    "windows-1251",
	"cp1252":    "windows-1252",
	"cp1253":    "windows-1253",
	"cp1254":    "windows-1254",
	"cp1255":    "windows-1255",
	"cp1256":    "windows-1256",
	"cp1257":    "windows-1257",
	"cp1258":    "windows-1258",
	"cp437":     "IBM437",
	"cp850":     "IBM850",
	"cp852":     "IBM852",
	"cp855":     "IBM855",
	"cp860":     "IBM860",
	"cp862":     "IBM862",
	"cp863":     "IBM863",
	"cp865":     "IBM865",
	"cp866":     "ibm866",
	"gb18030":   "gb18030",
	"gb2312":    "gbk",
	"gbk":       "gbk",
	"koi8_r":    "koi8-r",
	"shift_jis": "shift_jis",
}

// the combination of supportedEncodingByLib and our own override map
var completeNameMap = map[string]string{
	"ascii":           "windows-1252",
	"big5":            "big5",
	"big5hkscs":       "big5",
	"cp037":           "IBM037",
	"cp424":           "iso-8859-8",
	"cp437":           "IBM437",
	"cp500":           "",
	"cp720":           "iso-8859-6",
	"cp737":           "iso-8859-7",
	"cp775":           "iso-8859-13",
	"cp850":           "IBM850",
	"cp852":           "IBM852",
	"cp855":           "IBM855",
	"cp856":           "",
	"cp857":           "",
	"cp858":           "",
	"cp860":           "IBM860",
	"cp861":           "",
	"cp862":           "IBM862",
	"cp863":           "IBM863",
	"cp864":           "",
	"cp865":           "IBM865",
	"cp866":           "ibm866",
	"cp869":           "",
	"cp874":           "windows-874",
	"cp875":           "",
	"cp932":           "",
	"cp949":           "",
	"cp950":           "",
	"cp1006":          "",
	"cp1026":          "",
	"cp1140":          "",
	"cp1250":          "windows-1250",
	"cp1251":          "windows-1251",
	"cp1252":          "windows-1252",
	"cp1253":          "windows-1253",
	"cp1254":          "windows-1254",
	"cp1255":          "windows-1255",
	"cp1256":          "windows-1256",
	"cp1257":          "windows-1257",
	"cp1258":          "windows-1258",
	"euc_jp":          "euc-jp",
	"euc_jis_2004":    "",
	"euc_jisx0213":    "",
	"euc_kr":          "euc-kr",
	"gb2312":          "gbk",
	"gbk":             "gbk",
	"gb18030":         "gb18030",
	"hz":              "gbk",
	"iso2022_jp":      "",
	"iso2022_jp_1":    "",
	"iso2022_jp_2":    "",
	"iso2022_jp_2004": "",
	"iso2022_jp_3":    "",
	"iso2022_jp_ext":  "",
	"iso2022_kr":      "",
	"latin_1":         "windows-1252",
	"iso8859_2":       "iso-8859-2",
	"iso8859_3":       "iso-8859-3",
	"iso8859_4":       "iso-8859-4",
	"iso8859_5":       "iso-8859-5",
	"iso8859_6":       "iso-8859-6",
	"iso8859_7":       "iso-8859-7",
	"iso8859_8":       "iso-8859-8",
	"iso8859_9":       "windows-1254",
	"iso8859_10":      "iso-8859-10",
	"iso8859_13":      "iso-8859-13",
	"iso8859_14":      "iso-8859-14",
	"iso8859_15":      "iso-8859-15",
	"iso8859_16":      "iso-8859-16",
	"johab":           "",
	"koi8_r":          "koi8-r",
	"koi8_u":          "koi8-u",
	"mac_cyrillic":    "x-mac-cyrillic",
	"mac_greek":       "",
	"mac_iceland":     "",
	"mac_latin2":      "",
	"mac_roman":       "macintosh",
	"mac_turkish":     "",
	"ptcp154":         "",
	"shift_jis":       "shift_jis",
	"shift_jis_2004":  "",
	"shift_jisx0213":  "",
	"utf_32":          "",
	"utf_32_be":       "",
	"utf_32_le":       "",
	"utf_16":          "utf-16le",
	"utf_16_be":       "utf-16be",
	"utf_16_le":       "utf-16le",
	"utf_7":           "",
	"utf_8":           "utf-8",
	"utf_8_sig":       "",
}

func TestEncoding(t *testing.T) {
	for key, value := range completeNameMap {
		enc := NormalizeEncoding(key)
		assert.Equal(t, value, enc, "For key %s, expected value %s does not equal to normalized value %s", key, value, enc)
	}
	for key, value := range nameMap {
		assert.Equal(t, completeNameMap[key], value, "For key %s, expected map value %s, override map value %s", key, completeNameMap[key], value)
	}
}
