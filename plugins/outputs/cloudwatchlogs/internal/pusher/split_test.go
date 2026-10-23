// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"bytes"
	"math/rand/v2"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/state"
)

// noChunkCap is a chunk cap that no test message reaches, for checking the split without truncation.
const noChunkCap = 1 << 30

// checkSplit splits message with splitMessage and checks the invariants that hold for any input:
//   - there are between 1 and maxChunks chunks, and each but possibly a truncated last one is non-empty;
//   - each chunk's counted size, measured alone, is the size returned for it and is within budget;
//   - no chunk splits a rune: decoding the whole message, every rune lies within one chunk, and the counted sizes of
//     the runes in a chunk add up to the chunk's counted size measured alone;
//   - without truncation, the chunks concatenate to message byte for byte, their sizes add up to its counted size, and
//     there is more than one chunk exactly when message is over budget;
//   - with truncation, there are maxChunks chunks, the last one ends with defaultTruncationSuffix, and the chunks
//     without the suffix are a prefix of message.
func checkSplit(t *testing.T, message string, budget, maxChunks int) ([]string, []int, bool) {
	t.Helper()
	size := utf8EncodedLength(message)
	chunks, sizes, truncated := splitMessage(message, size, budget, maxChunks)
	require.Len(t, sizes, len(chunks))
	require.NotEmpty(t, chunks)
	require.LessOrEqual(t, len(chunks), maxChunks)

	sum := 0
	for i, chunk := range chunks {
		require.Equal(t, utf8EncodedLength(chunk), sizes[i], "chunk %d", i)
		require.LessOrEqual(t, sizes[i], budget, "chunk %d", i)
		if message != "" {
			require.NotEmpty(t, chunk, "chunk %d", i)
		}
		sum += sizes[i]
	}

	// The chunks as cut from message, without the suffix of a truncated last chunk.
	parts := chunks
	if truncated {
		require.Len(t, chunks, maxChunks)
		last := chunks[len(chunks)-1]
		require.True(t, strings.HasSuffix(last, defaultTruncationSuffix))
		parts = append(append([]string{}, chunks[:len(chunks)-1]...), strings.TrimSuffix(last, defaultTruncationSuffix))
	}
	joined := strings.Join(parts, "")
	if truncated {
		require.True(t, strings.HasPrefix(message, joined), "the kept parts are a prefix of the message")
		require.Less(t, len(joined), len(message))
	} else {
		require.True(t, joined == message, "the chunks concatenate to the message")
		require.Equal(t, size, sum)
		require.Equal(t, size > budget, len(chunks) > 1)
	}

	// Walk the runes of the whole message, as the service decodes it, and attribute each to the part it starts in.
	ends := make([]int, len(parts))
	end := 0
	for i, part := range parts {
		end += len(part)
		ends[i] = end
	}
	contribution := make([]int, len(parts))
	part := 0
	for i := 0; i < len(joined); {
		for i >= ends[part] {
			part++
		}
		n, counted := decodeRuneSize(message[i:])
		// Checked without testify, as this runs once per rune.
		if i+n > ends[part] {
			t.Fatalf("the rune at %d crosses the end of chunk %d", i, part)
		}
		contribution[part] += counted
		i += n
	}
	for i, p := range parts {
		require.Equal(t, utf8EncodedLength(p), contribution[i], "chunk %d is counted alone as in the whole message", i)
	}
	return chunks, sizes, truncated
}

// randomMessage returns a message of n pieces drawn from ASCII, separators, valid multi-byte UTF-8 and the invalid
// sequences in utf8EncodedLengthTestCases.
func randomMessage(r *rand.Rand, n int) string {
	pieces := []string{
		"a", "Z", "0", "{", " ", "\t", "\n", "é", "日", "😀", "\u2028", "\uFFFD",
		"\xff", "\x80", "\xe9", "\xe6\x97", "\xc0\xaf", "\xed\xa0\x80", "\xf4\x90\x80\x80",
	}
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString(pieces[r.IntN(len(pieces))])
	}
	return sb.String()
}

// TestSplitMessage_Random checks the splitMessage invariants on random messages with small budgets, so that the
// messages are split into many chunks and the look-back for a natural cut spans whole chunks.
func TestSplitMessage_Random(t *testing.T) {
	//nolint:gosec // G404: deterministic test inputs, not security-sensitive
	r := rand.New(rand.NewPCG(2336, 1))
	for i := 0; i < 2000; i++ {
		message := randomMessage(r, r.IntN(3000))
		// A truncated last chunk holds the suffix, so the budget must leave room for it and one rune.
		budget := len(defaultTruncationSuffix) + utf8.UTFMax + r.IntN(300)
		checkSplit(t, message, budget, noChunkCap)
		// The same message with a cap, which it may or may not reach.
		checkSplit(t, message, budget, 1+r.IntN(10))
	}
}

// TestSplitMessage_Adversarial checks the splitMessage invariants with the real budget, for messages of each kind at
// and around the budget and the multiples where a cut lands.
func TestSplitMessage_Adversarial(t *testing.T) {
	const b = maxEventMessageBytes
	// fill returns unit repeated to a counted size of target, padded with ASCII.
	fill := func(unit string, target int) string {
		unitSize := utf8EncodedLength(unit)
		return strings.Repeat(unit, target/unitSize) + strings.Repeat("a", target%unitSize)
	}
	kinds := map[string]string{
		"ASCII":            "a",
		"ASCIIWithSpaces":  "lorem ipsum\tdolor\nsit ",
		"ThreeByteUTF8":    "日",
		"FourByteUTF8":     "😀",
		"AllInvalid":       "\xff",
		"Mixed":            "a\xffé\xe6\x97日 😀\n\xc0\xaf",
		"InvalidAndSpaces": "\xff\xff\xff\xff \xe9\xe9\n",
	}
	sizes := map[string]int{"B-1": b - 1, "B": b, "B+1": b + 1, "3B+1": 3*b + 1}
	for kind, unit := range kinds {
		for name, target := range sizes {
			t.Run(kind+"/"+name, func(t *testing.T) {
				message := fill(unit, target)
				require.Equal(t, target, utf8EncodedLength(message))
				chunks, _, truncated := checkSplit(t, message, b, maxSplitChunks)
				assert.False(t, truncated)
				switch name {
				case "B-1", "B":
					assert.Len(t, chunks, 1)
				case "B+1":
					assert.Len(t, chunks, 2)
				}
			})
		}
	}
}

// TestSplitMessage_ChunkCounts checks the number of chunks for the inputs in the design's worst-case table, with the
// default max_event_size (1,048,376 raw bytes per tailer line).
func TestSplitMessage_ChunkCounts(t *testing.T) {
	const tailCap = maxEventMessageBytes
	invalid := func(n int) string { return string(bytes.Repeat([]byte{0xff}, n)) }
	tests := []struct {
		name     string
		message  string
		expected int
	}{
		{name: "ValidASCIIAtTailCap", message: strings.Repeat("a", tailCap), expected: 1},
		{name: "AllInvalidAtTailCap", message: invalid(tailCap), expected: 4},
		// ISO-8859-1 bytes 0x80-0xFF decoded to UTF-8 are 2 bytes each.
		{name: "TwoByteUTF8AtTailCap", message: strings.Repeat("é", tailCap), expected: 2},
		// The largest multi-line group the tailer could build before the multi-line fix: a buffer of tailCap-1 bytes,
		// "\n" and a whitespace-led continuation line of tailCap bytes (2,096,752 bytes).
		{name: "MultiLineMaximumASCII", message: strings.Repeat("a", tailCap-1) + "\n " + strings.Repeat("a", tailCap-1), expected: 2},
		{name: "MultiLineMaximumAllInvalid", message: invalid(tailCap-1) + "\n " + invalid(tailCap-1), expected: 7},
		{name: "MultiLineMaximumAllInvalidCustomPattern", message: invalid(tailCap-1) + "\n" + invalid(tailCap), expected: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if strings.HasPrefix(tt.name, "MultiLineMaximum") {
				require.Len(t, tt.message, 2_096_752)
			}
			chunks, _, truncated := checkSplit(t, tt.message, maxEventMessageBytes, maxSplitChunks)
			assert.False(t, truncated)
			assert.Len(t, chunks, tt.expected)
		})
	}
}

// TestSplitMessage_Cap checks that a message that needs more than maxSplitChunks chunks keeps maxSplitChunks chunks,
// the last one cut to the budget less the suffix and ending with it, and drops the rest.
func TestSplitMessage_Cap(t *testing.T) {
	const b = maxEventMessageBytes
	// Counted as 9 budgets, so 9 chunks would be needed.
	message := string(bytes.Repeat([]byte{0xff}, 3*b))
	chunks, sizes, truncated := checkSplit(t, message, b, maxSplitChunks)
	require.True(t, truncated)
	require.Len(t, chunks, maxSplitChunks)
	// Each full chunk of invalid bytes holds b/3 of them.
	for i := 0; i < maxSplitChunks-1; i++ {
		assert.Equal(t, b/invalidUTF8ByteSize, len(chunks[i]), "chunk %d", i)
	}
	last := chunks[maxSplitChunks-1]
	assert.Equal(t, strings.Repeat("\xff", (b-len(defaultTruncationSuffix))/invalidUTF8ByteSize)+defaultTruncationSuffix, last)
	assert.LessOrEqual(t, sizes[maxSplitChunks-1], b)

	// The cap of 1 is the truncation of validateAndTruncateMessage.
	chunks, sizes, truncated = splitMessage(message, utf8EncodedLength(message), b, 1)
	expected, expectedSize, expectedTruncated := validateAndTruncateMessage(message)
	assert.Equal(t, []string{expected}, chunks)
	assert.Equal(t, []int{expectedSize}, sizes)
	assert.Equal(t, expectedTruncated, truncated)
}

// TestSplitMessage_BudgetTooSmall checks that a budget smaller than one rune, which callers never pass, keeps what
// fits in one chunk and reports the rest as dropped rather than looping forever.
func TestSplitMessage_BudgetTooSmall(t *testing.T) {
	chunks, sizes, truncated := splitMessage("ab日", 5, 2, maxSplitChunks)
	assert.Equal(t, []string{"ab"}, chunks)
	assert.Equal(t, []int{2}, sizes)
	assert.True(t, truncated)
}

// TestSplitMessage_NaturalCut checks where the cut between two chunks lands: after the last newline within the final
// naturalCutWindow counted bytes of the chunk, else after the last space or tab within them, else at the rune boundary
// closest to the budget.
func TestSplitMessage_NaturalCut(t *testing.T) {
	const b = maxEventMessageBytes
	a := func(n int) string { return strings.Repeat("a", n) }
	tests := []struct {
		name string
		// The message, which is over the budget.
		message string
		// The expected length of the first chunk.
		expectedLen int
	}{
		{name: "Newline", message: a(b-100) + "\n" + a(1000), expectedLen: b - 99},
		{name: "Space", message: a(b-100) + " " + a(1000), expectedLen: b - 99},
		{name: "Tab", message: a(b-100) + "\t" + a(1000), expectedLen: b - 99},
		{name: "NewlinePreferredOverLaterSpace", message: a(b-2000) + "\n" + a(1000) + " " + a(2000), expectedLen: b - 1999},
		{name: "LastNewline", message: a(b-3000) + "\n" + a(1000) + "\n" + a(2000), expectedLen: b - 1998},
		{name: "NewlineOutsideWindowSpaceInside", message: a(b-5000) + "\n" + a(4000) + " " + a(2000), expectedLen: b - 998},
		{name: "NoneInWindow", message: a(b-5000) + " \n" + a(6000), expectedLen: b},
		{name: "NewlineAtEndOfWindow", message: a(b-naturalCutWindow) + "\n" + a(naturalCutWindow+10), expectedLen: b - naturalCutWindow + 1},
		{name: "NewlineJustOutsideWindow", message: a(b-naturalCutWindow-1) + "\n" + a(naturalCutWindow+10), expectedLen: b},
		{name: "ChunkEndsWithNewline", message: a(b-1) + "\n" + a(10), expectedLen: b},
		// The window is in counted bytes: 1,365 invalid bytes after the space count 4,095, so the space is the first
		// of the final 4,096 counted bytes. One more invalid byte puts it outside, although it is only 1,367 raw
		// bytes from the end.
		{name: "InvalidBytesWithinCountedWindow", message: a(b-4096) + " " + strings.Repeat("\xff", 2000), expectedLen: b - 4095},
		{name: "InvalidBytesOutsideCountedWindow", message: a(b-4099) + " " + strings.Repeat("\xff", 2000), expectedLen: b - 4098 + 1366},
		{name: "MultiByteOnly", message: strings.Repeat("日", b/3) + "日日", expectedLen: b / 3 * 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, _, truncated := checkSplit(t, tt.message, b, maxSplitChunks)
			require.False(t, truncated)
			require.Len(t, chunks, 2)
			assert.Equal(t, tt.expectedLen, len(chunks[0]))
		})
	}
}

// TestSplitMessage_MatchesSerializedMessage checks the chunks as the service receives them: each is within the
// per-event limit as the service counts it, and the decoded chunks concatenate to the decoded message, so the service
// stores the same text as it would for the whole message.
func TestSplitMessage_MatchesSerializedMessage(t *testing.T) {
	//nolint:gosec // G404: deterministic test inputs, not security-sensitive
	r := rand.New(rand.NewPCG(2336, 2))
	var sb strings.Builder
	for sb.Len() < 3*maxEventMessageBytes/2 {
		sb.WriteString(randomMessage(r, 1000))
	}
	message := sb.String()
	chunks, sizes, truncated := checkSplit(t, message, maxEventMessageBytes, maxSplitChunks)
	require.False(t, truncated)
	require.Greater(t, len(chunks), 1)
	var decoded strings.Builder
	for i, chunk := range chunks {
		d := decodeSerializedMessage(t, chunk)
		assert.Equal(t, sizes[i], len(d), "chunk %d", i)
		assert.LessOrEqual(t, len(d)+serviceEventOverheadBytes, maxEventPayloadBytes, "chunk %d", i)
		decoded.WriteString(d)
	}
	assert.True(t, decoded.String() == decodeSerializedMessage(t, message))
}

func TestIsEMFMessage(t *testing.T) {
	assert.True(t, IsEMFMessage(`{"_aws":{"CloudWatchMetrics":[]}}`))
	assert.False(t, IsEMFMessage(`{"_aws":{}}`))
	assert.False(t, IsEMFMessage(`"CloudWatchMetrics"`))
	assert.False(t, IsEMFMessage(`{"CloudWatchMetrics": 1} trailing`))
	assert.False(t, IsEMFMessage(""))
}

// TestNewStatefulLogEvents checks the events for one source message: one for a message that fits, several with the
// same timestamp for one over the limit with the state and done callback on the last only, and one truncated event
// for an EMF message over the limit.
func TestNewStatefulLogEvents(t *testing.T) {
	now := time.Now()
	r := state.NewRange(10, 20)

	t.Run("Fits", func(t *testing.T) {
		mrq := &mockRangeQueue{}
		single, split := newStatefulLogEvents(now, "small", func() {}, &logEventState{r: r, queue: mrq})
		require.NotNil(t, single)
		require.Nil(t, split)
		assert.Equal(t, "small", single.message)
		assert.Equal(t, len("small")+perEventHeaderBytes, single.eventBytes)
		assert.NotNil(t, single.doneCallback)
		assert.NotNil(t, single.state)
		assert.False(t, single.truncated)
	})

	t.Run("Split", func(t *testing.T) {
		mrq := &mockRangeQueue{}
		var doneCalls int
		message := strings.Repeat("x", 3*maxEventMessageBytes+1)
		single, events := newStatefulLogEvents(now, message, func() { doneCalls++ }, &logEventState{r: r, queue: mrq})
		require.Nil(t, single)
		require.Len(t, events, 4)
		for i, event := range events {
			assert.Equal(t, now, event.timestamp, "event %d", i)
			assert.False(t, event.truncated, "event %d", i)
			if i < len(events)-1 {
				assert.Nil(t, event.doneCallback, "event %d", i)
				assert.Nil(t, event.state, "event %d", i)
				assert.Equal(t, maxEventPayloadBytes, event.eventBytes, "a full chunk fills a request")
			}
		}
		last := events[len(events)-1]
		assert.Equal(t, "x", last.message)
		require.NotNil(t, last.state)
		assert.Equal(t, r, last.state.r)
		last.doneCallback()
		assert.Equal(t, 1, doneCalls)
	})

	t.Run("PastCap", func(t *testing.T) {
		message := string(bytes.Repeat([]byte{0xff}, 3*maxEventMessageBytes))
		single, events := newStatefulLogEvents(now, message, nil, nil)
		require.Nil(t, single)
		require.Len(t, events, maxSplitChunks)
		for i, event := range events[:len(events)-1] {
			assert.False(t, event.truncated, "event %d", i)
		}
		last := events[len(events)-1]
		assert.True(t, last.truncated)
		assert.True(t, strings.HasSuffix(last.message, defaultTruncationSuffix))
		assert.LessOrEqual(t, last.eventBytes, maxEventPayloadBytes)
	})

	t.Run("EMFOverLimitIsTruncated", func(t *testing.T) {
		message := `{"_aws":{"CloudWatchMetrics":[]},"data":"` + strings.Repeat("x", maxEventMessageBytes) + `"}`
		require.True(t, IsEMFMessage(message))
		single, split := newStatefulLogEvents(now, message, func() {}, nil)
		require.NotNil(t, single)
		require.Nil(t, split)
		expected, size, truncated := validateAndTruncateMessage(message)
		require.True(t, truncated)
		assert.Equal(t, expected, single.message)
		assert.Equal(t, size+perEventHeaderBytes, single.eventBytes)
		assert.True(t, single.truncated)
		assert.NotNil(t, single.doneCallback)
	})

	t.Run("JSONWithoutCloudWatchMetricsIsSplit", func(t *testing.T) {
		message := `{"data":"` + strings.Repeat("x", maxEventMessageBytes) + `"}`
		require.False(t, IsEMFMessage(message))
		events := assertSplitsLosslessly(t, message)
		assert.Len(t, events, 2)
	})
}

// TestLogEventBatch_SplitEventsAckOnce checks that the events of a split message, each in its own batch as the queue
// sends them, enqueue the source range once, from the batch of the last event, and run the done callback once.
func TestLogEventBatch_SplitEventsAckOnce(t *testing.T) {
	r := state.NewRange(10, 20)
	mrq := &mockRangeQueue{}
	mrq.On("ID").Return("file")
	mrq.On("Enqueue", r).Return().Once()
	var doneCalls int
	events := flattenStatefulLogEvents(newStatefulLogEvents(time.Now(), strings.Repeat("x", 2*maxEventMessageBytes+1), func() { doneCalls++ }, &logEventState{r: r, queue: mrq}))
	require.Len(t, events, 3)
	for i, event := range events {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		batch.append(event)
		batch.done()
		if i < len(events)-1 {
			mrq.AssertNotCalled(t, "Enqueue", mock.Anything)
		}
	}
	mrq.AssertExpectations(t)
	// A stateful event is acknowledged through the range queue, not the done callback.
	assert.Equal(t, 0, doneCalls)
}

// TestLogEventBatch_SortKeepsSplitEventsInOrder checks that sorting a batch keeps events with the same timestamp, such
// as the events split from one message, in the order they were added.
func TestLogEventBatch_SortKeepsSplitEventsInOrder(t *testing.T) {
	now := time.Now()
	batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
	batch.append(newLogEvent(now.Add(time.Second), "later", nil))
	split := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15"}
	for _, message := range split {
		batch.append(newLogEvent(now, message, nil))
	}
	batch.append(newLogEvent(now.Add(-time.Second), "earlier", nil))
	require.True(t, batch.needSort)

	input := batch.build()
	require.Len(t, input.LogEvents, len(split)+2)
	assert.Equal(t, "earlier", *input.LogEvents[0].Message)
	for i, message := range split {
		assert.Equal(t, message, *input.LogEvents[i+1].Message)
	}
	assert.Equal(t, "later", *input.LogEvents[len(split)+1].Message)
}
