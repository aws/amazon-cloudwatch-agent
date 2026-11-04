// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tail

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/quick"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/constants"
)

// 2-, 3- and 4-byte UTF-8 runes used across the tests.
var (
	rune2 = []byte("ñ") // 0xC3 0xB1
	rune3 = []byte("中") // 0xE4 0xB8 0xAD
	rune4 = []byte("😀") // 0xF0 0x9F 0x98 0x80
)

// buildStraddleInput places a single multi-byte rune so that `splitAt` of
// its bytes land in the first chunk and the rest spill into the second.
// Padding before and after is pure ASCII, so the only rune boundary in play
// is this one.
func buildStraddleInput(chunkSize int, r []byte, splitAt int) string {
	// pad so that the rune's first `splitAt` bytes end exactly at offset chunkSize
	padBefore := chunkSize - splitAt
	// pad after so there's more than one chunk's worth of content remaining
	padAfter := chunkSize/2 + 7
	return strings.Repeat("a", padBefore) + string(r) + strings.Repeat("b", padAfter)
}

// TestPartitionStringRuneBoundary — table of 2/3/4-byte runes placed so the
// raw cut lands at each in-rune offset. Every emitted part must be valid
// UTF-8 and the parts' concatenation must equal the input.
func TestPartitionStringRuneBoundary(t *testing.T) {
	chunkSize := 32
	cases := []struct {
		name    string
		rune    []byte
		splitAt int // bytes of the rune that end up in the first chunk
	}{
		{"2-byte_cut_1-1", rune2, 1},
		{"3-byte_cut_1-2", rune3, 1},
		{"3-byte_cut_2-1", rune3, 2},
		{"4-byte_cut_1-3", rune4, 1},
		{"4-byte_cut_2-2", rune4, 2},
		{"4-byte_cut_3-1", rune4, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := buildStraddleInput(chunkSize, tc.rune, tc.splitAt)
			parts := partitionString(in, chunkSize, true)
			assert.Equal(t, in, strings.Join(parts, ""), "concat must equal input")
			for i, p := range parts {
				assert.True(t, utf8.ValidString(p),
					"part %d is not valid UTF-8: %q", i, p)
			}
		})
	}
}

// TestPartitionStringInvalidUTF8 — orphan continuation bytes and lone start
// bytes with no continuation. Current (byte-raw) behavior must be preserved
// exactly: concat == input, cuts stay at raw chunkSize boundaries.
func TestPartitionStringInvalidUTF8(t *testing.T) {
	chunkSize := 8
	cases := []struct {
		name string
		in   string
	}{
		// Trailing orphan continuation bytes (0x80-0xBF are not RuneStart):
		// walking back 1-3 finds no RuneStart, no backoff.
		{"orphan_continuations", strings.Repeat("a", 6) + "\x80\x80" + strings.Repeat("b", 8)},
		// Lone 2-byte start byte with an invalid continuation (0x40 is not a
		// continuation byte): FullRune(0xC3 0x40) is true (decodes as RuneError,
		// but the sequence is complete), so no backoff.
		{"invalid_sequence", strings.Repeat("a", 6) + "\xC3\x40" + strings.Repeat("b", 8)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parts := partitionString(tc.in, chunkSize, true)
			assert.Equal(t, tc.in, strings.Join(parts, ""), "concat must equal input")
			// First chunk is still a raw chunkSize-length cut (no backoff).
			assert.Equal(t, chunkSize, len(parts[0]),
				"invalid-UTF-8 input must not trigger backoff")
		})
	}
}

// TestPartitionStringASCII — pure ASCII cuts land at exact chunkSize offsets
// with no carries.
func TestPartitionStringASCII(t *testing.T) {
	chunkSize := 16
	in := strings.Repeat("x", chunkSize*3+5)
	parts := partitionString(in, chunkSize, true)
	assert.Equal(t, in, strings.Join(parts, ""))
	for i := 0; i < len(parts)-1; i++ {
		assert.Equal(t, chunkSize, len(parts[i]),
			"ASCII chunk %d must be exactly chunkSize", i)
	}
}

// TestPartitionStringSmallChunkSize — chunkSize = 1,2,3. Backoff is skipped
// (chunk is below utf8.UTFMax) but progress is guaranteed and concat holds.
func TestPartitionStringSmallChunkSize(t *testing.T) {
	in := "a" + string(rune3) + "bc" + string(rune4) + "zz"
	for _, size := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("chunk=%d", size), func(t *testing.T) {
			parts := partitionString(in, size, true)
			assert.Equal(t, in, strings.Join(parts, ""), "concat must equal input")
			for i, p := range parts {
				assert.NotEmpty(t, p, "chunk %d must be non-empty", i)
			}
		})
	}
}

// TestPartitionStringExactBoundary — rune ends exactly at chunkSize so no
// backoff is needed.
func TestPartitionStringExactBoundary(t *testing.T) {
	chunkSize := 16
	// Pad so rune4's last byte lands at offset chunkSize-1.
	padBefore := chunkSize - len(rune4)
	in := strings.Repeat("a", padBefore) + string(rune4) + strings.Repeat("b", 10)
	parts := partitionString(in, chunkSize, true)
	assert.Equal(t, in, strings.Join(parts, ""))
	assert.Equal(t, chunkSize, len(parts[0]),
		"exact-boundary cut must not trigger backoff")
	assert.True(t, utf8.ValidString(parts[0]))
}

// TestPartitionStringUtf8AwareOff — regression: when utf8Aware is false the
// function keeps its original raw-cut behavior.
func TestPartitionStringUtf8AwareOff(t *testing.T) {
	chunkSize := 8
	in := strings.Repeat("a", chunkSize-1) + string(rune3) + "bb"
	parts := partitionString(in, chunkSize, false)
	assert.Equal(t, in, strings.Join(parts, ""))
	assert.Equal(t, chunkSize, len(parts[0]),
		"utf8Aware=false must behave exactly as before")
}

// TestPartitionStringProperty — property-based: concat equals input; and
// when the input is valid UTF-8 and chunkSize >= utf8.UTFMax every chunk is
// valid UTF-8.
func TestPartitionStringProperty(t *testing.T) {
	f := func(bs []byte) bool {
		if len(bs) == 0 {
			return true
		}
		for _, size := range []int{4, 8, 16, 64} {
			parts := partitionString(string(bs), size, true)
			if strings.Join(parts, "") != string(bs) {
				return false
			}
			if utf8.Valid(bs) && size >= utf8.UTFMax {
				for _, p := range parts {
					if !utf8.ValidString(p) {
						return false
					}
				}
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Error(err)
	}
}

// TestPartitionStringRegressionMultibyte — the exact regression from
// multibyte-finding.md §1: a 1.2 MB emoji line and a 2.5 MB mixed-rune line.
// After the fix, every emitted chunk is valid UTF-8 and the concatenation
// equals the input.
func TestPartitionStringRegressionMultibyte(t *testing.T) {
	chunkSize := 1048376 // DefaultMaxEventSize - PerEventHeaderBytes

	// 1.2 MB line: ASCII pad + emoji exactly straddling the cut (3|1).
	emojiLine := strings.Repeat("a", chunkSize-3) + string(rune4) +
		strings.Repeat("b", 180_000)
	parts := partitionString(emojiLine, chunkSize, true)
	assert.Equal(t, emojiLine, strings.Join(parts, ""))
	for i, p := range parts {
		assert.True(t, utf8.ValidString(p), "emoji chunk %d not valid UTF-8", i)
	}

	// 2.5 MB line: ASCII pad + 中 straddling (1|2) at every chunk boundary.
	n := 2
	mixed := strings.Repeat("a", chunkSize-1) + string(rune3)
	for i := 0; i < n; i++ {
		mixed += strings.Repeat("c", chunkSize-1) + string(rune3)
	}
	mixed += strings.Repeat("d", 1024)
	parts = partitionString(mixed, chunkSize, true)
	assert.Equal(t, mixed, strings.Join(parts, ""))
	for i, p := range parts {
		assert.True(t, utf8.ValidString(p), "mixed chunk %d not valid UTF-8", i)
	}
}

// TestReadLineUtf8Backoff — write a file with lines containing multi-byte
// runes straddling MaxLineSize at each of the six in-rune offsets. Every
// returned chunk must be valid UTF-8, and concat must equal the file content.
func TestReadLineUtf8Backoff(t *testing.T) {
	cases := []struct {
		name    string
		rune    []byte
		splitAt int
	}{
		{"2-byte_cut_1-1", rune2, 1},
		{"3-byte_cut_1-2", rune3, 1},
		{"3-byte_cut_2-1", rune3, 2},
		{"4-byte_cut_1-3", rune4, 1},
		{"4-byte_cut_2-2", rune4, 2},
		{"4-byte_cut_3-1", rune4, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			filename := filepath.Join(tempDir, "utf8_boundary.log")

			maxLineSize := 512 * 1024
			// Place the rune so splitAt of its bytes fall in the first chunk.
			padBefore := maxLineSize - tc.splitAt
			content := strings.Repeat("a", padBefore) + string(tc.rune) +
				strings.Repeat("b", maxLineSize/4) + "\n"
			require.NoError(t, os.WriteFile(filename, []byte(content), 0600))

			tail, err := TailFile(filename, Config{
				Follow:      false,
				MustExist:   true,
				MaxLineSize: maxLineSize,
				IsUTF8:      true,
			})
			require.NoError(t, err)
			defer tail.Stop()

			var collected []string
			readStart := time.Now()
			deadline := 5 * time.Second
			// The file content minus the trailing newline.
			want := strings.TrimRight(content, "\n")
			for time.Since(readStart) < deadline {
				select {
				case line := <-tail.Lines:
					if line == nil {
						break
					}
					collected = append(collected, line.Text)
					tail.ReleaseLine(line)
					if strings.Join(collected, "") == want {
						// Verify every emitted chunk is valid UTF-8.
						for i, c := range collected {
							assert.True(t, utf8.ValidString(c),
								"chunk %d not valid UTF-8", i)
						}
						return
					}
				case <-time.After(500 * time.Millisecond):
					// keep looping until deadline
				}
			}
			t.Fatalf("timed out assembling line; got %d chunks totaling %d bytes (want %d)",
				len(collected), len(strings.Join(collected, "")), len(want))
		})
	}
}

// TestReadLineUtf8AwareOff — when IsUTF8 is false, cuts stay at raw byte
// boundaries. Preserves today's behavior for non-UTF-8 encodings whose raw
// bytes are decoded later in tailersrc.
func TestReadLineUtf8AwareOff(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "utf8_off.log")

	maxLineSize := 512 * 1024
	// Place a 3-byte rune straddling the boundary at (1|2).
	padBefore := maxLineSize - 1
	content := strings.Repeat("a", padBefore) + string(rune3) +
		strings.Repeat("b", 1024) + "\n"
	require.NoError(t, os.WriteFile(filename, []byte(content), 0600))

	tail, err := TailFile(filename, Config{
		Follow:      false,
		MustExist:   true,
		MaxLineSize: maxLineSize,
		IsUTF8:      false,
	})
	require.NoError(t, err)
	defer tail.Stop()

	var collected []string
	deadline := time.Now().Add(5 * time.Second)
	want := strings.TrimRight(content, "\n")
	for time.Now().Before(deadline) {
		select {
		case line := <-tail.Lines:
			collected = append(collected, line.Text)
			tail.ReleaseLine(line)
			if strings.Join(collected, "") == want {
				// With IsUTF8=false the raw chunk is exactly maxLineSize.
				assert.Equal(t, maxLineSize, len(collected[0]),
					"IsUTF8=false must not trigger backoff")
				return
			}
		case <-time.After(500 * time.Millisecond):
		}
	}
	t.Fatalf("timed out; got %d chunks", len(collected))
}

// --- Benchmarks ------------------------------------------------------------

func BenchmarkPartitionStringASCII(b *testing.B) {
	chunkSize := 1048376
	in := strings.Repeat("a", chunkSize*2+37)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = partitionString(in, chunkSize, true)
	}
}

func BenchmarkPartitionStringMultibyte(b *testing.B) {
	chunkSize := 1048376
	// Multi-byte filler so every chunk has runes near its tail.
	fill := strings.Repeat(string(rune3), (chunkSize/3)+1)
	in := fill + fill + "zz"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = partitionString(in, chunkSize, true)
	}
}

// Reference bench — utf8Aware=false, which is the pre-fix behavior.
func BenchmarkPartitionStringRawCut(b *testing.B) {
	chunkSize := 1048376
	in := strings.Repeat(string(rune3), (chunkSize/3)+1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = partitionString(in, chunkSize, false)
	}
}

var _ = constants.DefaultMaxEventSize // keep import until used by tests above

// TestUTF8BoundaryBackoffsCounter — ensure the per-tail counter increments
// when partitionString performs a backoff (via sendLine's split path).
func TestUTF8BoundaryBackoffsCounter(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "counter.log")

	maxLineSize := 512 * 1024
	// Build a 2-chunk line with a 中 straddling the first boundary (1|2).
	content := strings.Repeat("a", maxLineSize-1) + string(rune3) +
		strings.Repeat("b", 1024) + "\n"
	require.NoError(t, os.WriteFile(filename, []byte(content), 0600))

	tail, err := TailFile(filename, Config{
		Follow:      false,
		MustExist:   true,
		MaxLineSize: maxLineSize,
		IsUTF8:      true,
	})
	require.NoError(t, err)
	defer tail.Stop()

	deadline := time.Now().Add(5 * time.Second)
	var collected int
	want := len(strings.TrimRight(content, "\n"))
	for time.Now().Before(deadline) && collected < want {
		select {
		case line := <-tail.Lines:
			if line == nil {
				continue
			}
			collected += len(line.Text)
			tail.ReleaseLine(line)
		case <-time.After(500 * time.Millisecond):
		}
	}
	assert.GreaterOrEqual(t, tail.UTF8BoundaryBackoffs(), int64(1),
		"backoff counter should have ticked at least once")
}

// --- Pending-cleared-on-reopen regression tests ----------------------------
//
// Regression for a bug where tail.pending (1-3 UTF-8 continuation bytes
// carried from a previous ErrBufferFull-driven rune-boundary backoff) was
// not cleared when the underlying file was rotated, truncated, or
// otherwise re-seek'd. Without the fix those stale bytes would be
// prepended by the next readSlice to the first line of the new file.
//
// These tests exercise the state transitions directly: they inject a
// pending carry and then invoke the paths that re-open the reader or
// re-seek it, asserting that pending is empty afterwards. This avoids
// racing the tailFileSync goroutine, which would consume pending on its
// very next readSlice.

// newBareTail builds a Tail bound to filename and opens its reader
// without starting tailFileSync. It mirrors the subset of TailFile setup
// that openReader / Reopen / seekTo require, so these tests can drive
// those paths without a live tailer goroutine racing their state.
func newBareTail(t *testing.T, filename string, maxLineSize int) *Tail {
	t.Helper()
	f, err := OpenFile(filename)
	require.NoError(t, err)
	tail := &Tail{
		Filename: filename,
		Lines:    make(chan *Line, 1),
		Config: Config{
			IsUTF8:      true,
			MaxLineSize: maxLineSize,
		},
		file: f,
	}
	tail.openReader()
	return tail
}

// TestPendingClearedOnReopenRotation — rotation case. The waitForChanges
// Deleted branch calls Reopen(true) after re-creating the file, which in
// turn calls openReader(). Any UTF-8 bytes carried from the previous
// file must be dropped before the first readSlice on the new file.
func TestPendingClearedOnReopenRotation(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "rot.log")
	// Original file content is irrelevant to the state transition: the
	// point is that Reopen reopens the file handle and openReader builds
	// a fresh reader, both of which must drop any carried bytes.
	require.NoError(t, os.WriteFile(filename, []byte("init\n"), 0600))

	tail := newBareTail(t, filename, 1024)
	defer tail.CloseFile()

	// Simulate a prior UTF-8 rune-boundary backoff: pending holds the
	// first 2 bytes of 中 (0xE4 0xB8), and curOffset was decremented to
	// match. Reopen(true) will reset curOffset to 0, so the ghost
	// decrement is clobbered implicitly.
	carry := []byte{0xE4, 0xB8}
	tail.lk.Lock()
	tail.pending = append(tail.pending[:0], carry...)
	tail.curOffset -= int64(len(carry))
	tail.lk.Unlock()

	require.NoError(t, tail.Reopen(true))

	tail.lk.Lock()
	defer tail.lk.Unlock()
	assert.Empty(t, tail.pending,
		"pending must be cleared after Reopen(true); otherwise stale bytes "+
			"would be prepended to the new file's first line")
	assert.Equal(t, int64(0), tail.curOffset,
		"curOffset must be reset to 0 on rotation (resetOffset=true)")
}

// TestPendingClearedOnReopenTruncation — truncation case. The
// waitForChanges Truncated branch also calls Reopen(true) and then
// openReader() a second time. Pending must still be empty afterwards
// (an idempotent clear), and no stale bytes may survive.
func TestPendingClearedOnReopenTruncation(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "trunc.log")
	require.NoError(t, os.WriteFile(filename, []byte(strings.Repeat("x", 32)+"\n"), 0600))

	tail := newBareTail(t, filename, 1024)
	defer tail.CloseFile()

	// Carry 3 bytes (a full-width 3-byte sequence) and decrement
	// curOffset by that amount.
	carry := []byte{0xE4, 0xB8, 0xAD}
	tail.lk.Lock()
	tail.pending = append(tail.pending[:0], carry...)
	tail.curOffset -= int64(len(carry))
	tail.lk.Unlock()

	// Mimic the Truncated path: truncate the file on disk, then Reopen,
	// then call openReader() a second time (waitForChanges does this
	// redundant second call).
	require.NoError(t, os.Truncate(filename, 0))
	require.NoError(t, tail.Reopen(true))
	tail.openReader()

	tail.lk.Lock()
	defer tail.lk.Unlock()
	assert.Empty(t, tail.pending,
		"pending must be cleared when the file is truncated and reopened")
	assert.Equal(t, int64(0), tail.curOffset,
		"curOffset must be reset to 0 after truncation reopen")
}

// TestPendingClearedOnSeekTo — a seek to a position discontinuous with
// the carried bytes must drop pending. All seeks are treated as
// discontinuous because curOffset is unconditionally recomputed via
// Tell() after Seek, so the carried-bytes decrement would otherwise be
// silently lost while the carried bytes themselves were still queued
// to prepend on the next readSlice.
func TestPendingClearedOnSeekTo(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "seek.log")
	require.NoError(t, os.WriteFile(filename, []byte(strings.Repeat("x", 64)+"\n"), 0600))

	tail := newBareTail(t, filename, 1024)
	defer tail.CloseFile()

	carry := []byte{0xE4, 0xB8, 0xAD}
	tail.lk.Lock()
	tail.pending = append(tail.pending[:0], carry...)
	tail.curOffset -= int64(len(carry))
	tail.lk.Unlock()

	require.NoError(t, tail.seekTo(SeekInfo{Offset: 0, Whence: io.SeekStart}))

	tail.lk.Lock()
	defer tail.lk.Unlock()
	assert.Empty(t, tail.pending,
		"pending must be cleared on seekTo; stale bytes from the pre-seek "+
			"line would otherwise be prepended at the new offset")
	assert.Equal(t, int64(0), tail.curOffset,
		"curOffset must be re-derived from the file after seekTo")
}
