// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package linux

import (
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/ianaindex"
)

// This conversion map is based on
// key: https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/AgentReference.html#agent-configuration-file
// value:https://github.com/golang/text/blob/master/encoding/htmlindex/tables.go#L52-L91
// mapping: https://docs.python.org/3/library/codecs.html#standard-encodings
//          https://github.com/golang/text/blob/master/encoding/htmlindex/tables.go#L95-L312
// Entries are removed if charset.Lookup works

var nameMap = map[string]string{
	"big5hkscs":       "big5",
	"cp424":           "iso-8859-8",
	"cp500":           "", //Western Europe, https://en.wikipedia.org/wiki/EBCDIC_500
	"cp720":           "iso-8859-6",
	"cp737":           "iso-8859-7",
	"cp775":           "iso-8859-13",
	"cp856":           "", //Hebrew, https://en.wikipedia.org/wiki/Code_page_856
	"cp857":           "", //Turkish, https://en.wikipedia.org/wiki/Code_page_857
	"cp858":           "", //Western Europe, https://en.wikipedia.org/wiki/Code_page_858
	"cp861":           "", //Icelandic, https://en.wikipedia.org/wiki/Code_page_861
	"cp864":           "", //Arabic, https://en.wikipedia.org/wiki/Code_page_864
	"cp869":           "", //Greek, https://en.wikipedia.org/wiki/Code_page_869
	"cp874":           "windows-874",
	"cp875":           "", //Greek,https://en.wikipedia.org/wiki/Code_page_875
	"cp932":           "", //Japanese, https://en.wikipedia.org/wiki/Code_page_932_(Microsoft_Windows)
	"cp949":           "", //Korean, https://en.wikipedia.org/wiki/Unified_Hangul_Code
	"cp950":           "", //Traditional Chinese ,https://en.wikipedia.org/wiki/Code_page_950
	"cp1006":          "", //Urdu
	"cp1026":          "", //Turkish
	"cp1140":          "", //Western Europe
	"euc_jp":          "euc-jp",
	"euc_jis_2004":    "", //Japanese, https://en.wikipedia.org/wiki/Extended_Unix_Code#EUC-JP
	"euc_jisx0213":    "", //Japanese, https://en.wikipedia.org/wiki/Extended_Unix_Code#EUC-JP
	"euc_kr":          "euc-kr",
	"hz":              "gbk",
	"iso2022_jp":      "", //Japanese
	"iso2022_jp_1":    "", //Japanese
	"iso2022_jp_2":    "", //Japanese, Korean, Simplified Chinese, Western Europe, Greek
	"iso2022_jp_2004": "", //Japanese
	"iso2022_jp_3":    "", //Japanese
	"iso2022_jp_ext":  "", //Japanese
	"iso2022_kr":      "", //Korean
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
	"johab":           "", //Korean
	"koi8_u":          "koi8-u",
	"mac_cyrillic":    "x-mac-cyrillic",
	"mac_greek":       "", //Greek
	"mac_iceland":     "", //Icelandic
	"mac_latin2":      "", //Central and Eastern Europe
	"mac_roman":       "macintosh",
	"mac_turkish":     "", //Turkish
	"ptcp154":         "", //Kazakh
	"shift_jis_2004":  "", //Japanese, https://en.wikipedia.org/wiki/Shift_JIS#Shift_JISx0213_and_Shift_JIS-2004
	"shift_jisx0213":  "", //Japanese, https://en.wikipedia.org/wiki/Shift_JIS#Shift_JISx0213_and_Shift_JIS-2004
	"utf_32":          "", //not support
	"utf_32_be":       "", //not support
	"utf_32_le":       "", //not support
	"utf_16":          "utf-16le",
	"utf_16_be":       "utf-16be",
	"utf_16_le":       "utf-16le",
	"utf_7":           "", //not support
	"utf_8":           "utf-8",
	"utf_8_sig":       "", //not support
}

func NormalizeEncoding(encoding string) string {
	if name := normalizeByLib(encoding); name != "" {
		return name
	}
	if name, ok := nameMap[encoding]; ok {
		//do another check in case the value in our override map not support by lib any more.
		if name = normalizeByLib(name); name != "" {
			return name
		}
	}
	return ""
}

func normalizeByLib(encoding string) string {
	_, name := charset.Lookup(encoding)
	if name != "" {
		return name
	}
	if enc, err := ianaindex.IANA.Encoding(encoding); err == nil {
		if name, err = ianaindex.IANA.Name(enc); err == nil {
			return name
		}
	}
	return ""
}
