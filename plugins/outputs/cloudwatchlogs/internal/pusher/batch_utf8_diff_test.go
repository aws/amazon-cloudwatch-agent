// SPDX-License-Identifier: MIT

package pusher

import (
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"
)

// utf8EncodedLengthRef is a verbatim copy of the pre-SWAR utf8EncodedLength implementation (the byte-at-a-time ASCII
// skip from review comment C2). It is kept here as an oracle so differential tests and the fuzz target can prove the
// SWAR variant returns exactly the same count for every input, including every invalid-UTF-8 shape.
func utf8EncodedLengthRef(message string) int {
	// Fast path: valid UTF-8 (including all ASCII) is counted as is.
	if utf8.ValidString(message) {
		return len(message)
	}
	// Slow path: walk the message, skip ASCII runs byte-by-byte, decode non-ASCII with decodeRuneSize.
	size := 0
	for i := 0; i < len(message); {
		j := i
		for j < len(message) && message[j] < 0x80 {
			j++
		}
		if j > i {
			size += j - i
			i = j
			continue
		}
		n, counted := decodeRuneSize(message[i:])
		size += counted
		i += n
	}
	return size
}

// randomUTF8Fragment returns a byte slice that mixes ASCII, valid 2/3/4-byte UTF-8 runes, lone continuation bytes,
// truncated sequences, illegal lead bytes (0xC0, 0xC1, 0xF5..0xFF), encoded surrogates (ED A0 80), and overlong
// forms. It aims to stress every branch of decodeRuneSize while keeping the generator deterministic for a given
// rng seed. All random values are masked to prove their range to gosec: IntN(2^k) ∈ [0, 2^k) and the mask keeps that
// shape intact, so the int→byte / int→rune conversion cannot overflow.
func randomUTF8Fragment(rng *rand.Rand, size int) []byte {
	out := make([]byte, 0, size)
	for len(out) < size {
		switch rng.IntN(14) {
		case 0, 1, 2, 3, 4, 5:
			// Plain ASCII — stress the word-at-a-time skip.
			out = append(out, byte(rng.IntN(0x80)&0x7F))
		case 6:
			// Valid 2-byte rune (U+0080..U+07FF).
			r := rune(0x80 + rng.IntN(0x7FF-0x80+1)&0x7FF)
			var buf [4]byte
			n := utf8.EncodeRune(buf[:], r)
			out = append(out, buf[:n]...)
		case 7:
			// Valid 3-byte rune (U+0800..U+FFFF), skipping surrogates.
			r := rune(0x0800 + rng.IntN(0xFFFF-0x0800+1)&0xFFFF)
			if r >= 0xD800 && r <= 0xDFFF {
				r = 0xE000
			}
			var buf [4]byte
			n := utf8.EncodeRune(buf[:], r)
			out = append(out, buf[:n]...)
		case 8:
			// Valid 4-byte rune (U+10000..U+10FFFF).
			//nolint:gosec // G115: IntN(0x10FFFF-0x10000+1) < 2^21 fits in rune (int32); checked by construction.
			r := rune(0x10000 + rng.IntN(0x10FFFF-0x10000+1))
			var buf [4]byte
			n := utf8.EncodeRune(buf[:], r)
			out = append(out, buf[:n]...)
		case 9:
			// Lone continuation byte (0x80..0xBF).
			out = append(out, byte(rng.IntN(0x40)&0x3F)|0x80)
		case 10:
			// Illegal lead byte (0xC0, 0xC1, 0xF5..0xFF).
			choices := []byte{0xC0, 0xC1, 0xF5, 0xF6, 0xF7, 0xF8, 0xFE, 0xFF}
			out = append(out, choices[rng.IntN(len(choices))])
		case 11:
			// Truncated multi-byte sequence (lead with wrong number of continuations).
			leads := []byte{0xC2, 0xE6, 0xF0}
			lead := leads[rng.IntN(len(leads))]
			out = append(out, lead)
			// Append 0 or 1 continuation bytes — fewer than the lead requires.
			cnt := rng.IntN(2)
			for k := 0; k < cnt; k++ {
				out = append(out, byte(rng.IntN(0x40)&0x3F)|0x80)
			}
		case 12:
			// Encoded surrogate (ED A0 80 .. ED BF BF): decodes to invalid.
			out = append(out,
				0xED,
				byte(rng.IntN(0x20)&0x1F)|0xA0,
				byte(rng.IntN(0x40)&0x3F)|0x80,
			)
		case 13:
			// Overlong encodings — valid shape, invalid for the codepoint.
			overlongs := [][]byte{
				{0xC0, 0xAF},             // overlong '/'
				{0xE0, 0x80, 0xAF},       // overlong '/'
				{0xF0, 0x80, 0x80, 0xAF}, // overlong '/'
			}
			out = append(out, overlongs[rng.IntN(len(overlongs))]...)
		}
	}
	if len(out) > size {
		out = out[:size]
	}
	return out
}

// TestUTF8EncodedLength_Differential runs the SWAR implementation and the byte-at-a-time reference against the same
// random inputs. Covers lengths 0..~2000, boundary alignments around 8 bytes (invalid bytes planted at offsets 0..15,
// string lengths 0..40), and at least 20,000 total cases. Any mismatch means the SWAR skip undercounted or
// overcounted a non-ASCII run compared with the oracle.
func TestUTF8EncodedLength_Differential(t *testing.T) {
	//nolint:gosec // G404: deterministic test inputs, not security-sensitive.
	rng := rand.New(rand.NewPCG(0xC0FFEE, 42))

	assertEqual := func(t *testing.T, msg string, label string) {
		t.Helper()
		got := utf8EncodedLength(msg)
		want := utf8EncodedLengthRef(msg)
		if got != want {
			t.Fatalf("%s: utf8EncodedLength(%q)=%d, ref=%d", label, msg, got, want)
		}
	}

	// Boundary: every string length 0..17 under a few invalid shapes.
	invalidBytes := []byte{0x80, 0xC0, 0xC1, 0xED, 0xF5, 0xFF}
	for strLen := 0; strLen <= 17; strLen++ {
		base := make([]byte, strLen)
		for k := range base {
			base[k] = 'a'
		}
		// All ASCII baseline.
		assertEqual(t, string(base), "ascii-len-only")
		// Non-ASCII invalid byte planted at every offset 0..min(15, strLen-1).
		maxOff := strLen - 1
		if maxOff > 15 {
			maxOff = 15
		}
		for off := 0; off <= maxOff; off++ {
			for _, b := range invalidBytes {
				cp := make([]byte, strLen)
				copy(cp, base)
				cp[off] = b
				assertEqual(t, string(cp), "boundary-off")
			}
		}
	}

	// Boundary: strings of length 0..40 with an invalid byte planted at offsets 0..15 to stress the word scan's
	// alignment, step, and tail-byte handoff.
	for strLen := 0; strLen <= 40; strLen++ {
		for off := 0; off < strLen && off < 16; off++ {
			for _, b := range invalidBytes {
				buf := make([]byte, strLen)
				for k := range buf {
					buf[k] = 'A'
				}
				buf[off] = b
				assertEqual(t, string(buf), "wordscan-align")
			}
		}
	}

	// Random lengths 0..2000, random content from the full generator.
	const randCases = 20000
	for n := 0; n < randCases; n++ {
		size := rng.IntN(2001)
		buf := randomUTF8Fragment(rng, size)
		assertEqual(t, string(buf), "random")
	}
}

// FuzzUTF8EncodedLength keeps the SWAR and reference implementations in lockstep for every byte string the fuzzer
// can produce. Seed corpus covers each decodeRuneSize branch and a few 8-byte alignment edges. Run interactively
// with:
//
//	go test -run '^$' -fuzz FuzzUTF8EncodedLength -fuzztime 60s ./plugins/outputs/cloudwatchlogs/internal/pusher/
func FuzzUTF8EncodedLength(f *testing.F) {
	seeds := [][]byte{
		nil,
		{},
		[]byte("hello world"),
		[]byte("héllo 日本語 😀"),
		{0xFF},
		{0xC0, 0xAF},
		{0xED, 0xA0, 0x80},
		{0xF4, 0x90, 0x80, 0x80},
		{'a', 'a', 'a', 'a', 'a', 'a', 'a', 'a'}, // exactly 8 ASCII bytes
		{'a', 'a', 'a', 'a', 'a', 'a', 'a', 0xC0},                     // invalid at offset 7
		{0xC0, 'a', 'a', 'a', 'a', 'a', 'a', 'a'},                     // invalid at offset 0
		{'a', 'a', 'a', 'a', 0xC0, 'a', 'a', 'a', 'a', 'a', 'a', 'a'}, // mid-word invalid
		append(bytesRepeat('a', 15), 0x80),                            // 15 ASCII + invalid
		append(bytesRepeat('a', 16), 0x80),                            // 16 ASCII + invalid (word-aligned)
		append(bytesRepeat('a', 17), 0x80),                            // off-by-one past a word
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		msg := string(data)
		got := utf8EncodedLength(msg)
		want := utf8EncodedLengthRef(msg)
		if got != want {
			t.Fatalf("mismatch: utf8EncodedLength=%d ref=%d data=%x", got, want, data)
		}
	})
}

func bytesRepeat(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

// TestFirstInvalidUTF8 pins the firstInvalidUTF8 helper: it must return len(s) exactly when s is valid UTF-8,
// and the byte offset of the first byte not part of a valid UTF-8 sequence otherwise. Covers the offsets the
// chunked ValidString + byte-level fallback must agree on (position 0, mid-word, last byte, truncated rune
// at EOS, boundaries of the 8-byte word scan, and across the 16 KiB chunk boundary firstInvalidUTF8 uses).
func TestFirstInvalidUTF8(t *testing.T) {
	// Chunk size that firstInvalidUTF8 uses internally — not exported, so compute it locally so the test
	// stays explicit about which boundaries it is probing.
	const chunk = 16 * 1024

	tests := []struct {
		name string
		s    string
		want int
	}{
		{name: "Empty", s: "", want: 0},
		{name: "ValidASCII", s: "hello world", want: len("hello world")},
		{name: "ValidMultibyte", s: "héllo 日本語 😀", want: len("héllo 日本語 😀")},
		{name: "InvalidAtZero", s: "\xffhello", want: 0},
		// Mid-word invalid: 4 ASCII bytes then an invalid byte at offset 4 (inside the first 8-byte word).
		{name: "InvalidMidWord", s: "abcd\xff" + strings.Repeat("x", 20), want: 4},
		{name: "InvalidAtLastByte", s: strings.Repeat("a", 20) + "\xff", want: 20},
		// Truncated multi-byte sequence at end of string: lead byte 0xC2 expects one continuation byte.
		{name: "TruncatedRuneAtEnd", s: strings.Repeat("a", 10) + "\xc2", want: 10},
		// Encoded surrogate: valid shape, invalid for the codepoint.
		{name: "EncodedSurrogate", s: "ok" + "\xed\xa0\x80", want: 2},
		// Overlong encoding of '/': valid shape, invalid codepoint.
		{name: "Overlong", s: "ok" + "\xc0\xaf", want: 2},
		// Lone continuation byte with no lead.
		{name: "LoneContinuation", s: "ok\x80rest", want: 2},
		// Invalid byte exactly at the 8-byte word boundary (offset 8) in a word-aligned 16-byte string.
		{name: "InvalidAtWordBoundary", s: "aaaaaaaa\xffaaaaaaa", want: 8},
		// Just before the first word boundary.
		{name: "InvalidJustBeforeWord", s: "aaaaaaa\xffaaaaaaaa", want: 7},
		// Just after a word boundary (byte 9 — first byte of the second word).
		{name: "InvalidJustAfterWord", s: "aaaaaaaaa\xffaaaaaa", want: 9},
		// Across the 16 KiB chunk boundary: invalid byte at the first offset of chunk 1 (chunk 0 is pure
		// ASCII, chunk 1's lead byte is invalid). firstInvalidUTF8 must advance past chunk 0 and locate
		// the offset at the start of chunk 1 — this is the case rune-boundary extension at chunk edges
		// must preserve.
		{
			name: "InvalidAtChunkBoundary",
			s:    strings.Repeat("a", chunk) + "\xff" + strings.Repeat("a", 100),
			want: chunk,
		},
		// Valid 4-byte rune straddling the chunk boundary (first byte of the rune at chunk-2, so bytes
		// chunk-2..chunk+1 form one rune). firstInvalidUTF8 must extend the chunk forward to the next
		// rune start instead of flagging the straddling rune as truncated.
		{
			name: "ValidRuneStraddlesChunkBoundary",
			s:    strings.Repeat("a", chunk-2) + "\xf0\x9f\x98\x80" + strings.Repeat("a", 100),
			want: chunk - 2 + 4 + 100, // whole string is valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstInvalidUTF8(tt.s)
			if got != tt.want {
				t.Fatalf("firstInvalidUTF8(%q) = %d, want %d", tt.s, got, tt.want)
			}
			// Cross-check against utf8.ValidString: when the whole string is valid, firstInvalidUTF8 must
			// return len(s); otherwise, the prefix up to the returned offset must itself be valid UTF-8.
			if got == len(tt.s) {
				if !utf8.ValidString(tt.s) {
					t.Fatalf("firstInvalidUTF8(%q) returned len(s) but ValidString says invalid", tt.s)
				}
			} else {
				if !utf8.ValidString(tt.s[:got]) {
					t.Fatalf("firstInvalidUTF8(%q) = %d but prefix is not valid UTF-8", tt.s, got)
				}
			}
		})
	}
}
