// SPDX-License-Identifier: MIT

package pusher

import (
	"math/rand/v2"
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
