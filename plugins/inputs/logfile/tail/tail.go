// Copyright (c) 2015 HPE Software Inc. All rights reserved.
// Copyright (c) 2013 ActiveState Software Inc. All rights reserved.

package tail

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/models"
	"gopkg.in/tomb.v1"

	"github.com/aws/amazon-cloudwatch-agent/internal/state"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/constants"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/tail/watch"
)

var (
	ErrStop                     = errors.New("Tail should now stop")
	ErrDeletedNotReOpen         = errors.New("File was deleted, tail should now stop")
	exitOnDeletionCheckDuration = time.Minute
	exitOnDeletionWaitDuration  = 5 * time.Minute
	OpenFileCount               atomic.Int64
)

type Line struct {
	Text   string
	Time   time.Time
	Err    error // Error from tail
	Offset int64 // offset of current reader
}

// SeekInfo represents arguments to `os.Seek`
type SeekInfo struct {
	Offset int64
	Whence int // os.SEEK_*
}

type limiter interface {
	Pour(uint16) bool
}

// Config is used to specify how a file must be tailed.
type Config struct {
	// File-specifc
	Location    *SeekInfo       // Seek to this location before tailing
	GapsToRead  state.RangeList // Seek to these ranges before we seek to Location
	ReOpen      bool            // Reopen recreated files (tail -F)
	MustExist   bool            // Fail early if the file does not exist
	Poll        bool            // Poll for file changes instead of using inotify
	Pipe        bool            // Is a named pipe (mkfifo)
	RateLimiter limiter

	// Generic IO
	Follow      bool // Continue looking for new lines (tail -f)
	MaxLineSize int  // If non-zero, split longer lines into multiple lines

	Logger telegraf.Logger

	// Special handling for utf16
	IsUTF16 bool

	// IsUTF8 enables rune-boundary backoff at MaxLineSize cuts so a UTF-8
	// multi-byte character that straddles the raw byte boundary is not split
	// into invalid bytes. When false (declared non-UTF-8 encoding handled by
	// tailersrc, or UTF-16 handled by readlineUtf16), raw byte cuts are kept.
	IsUTF8 bool
}

type Tail struct {
	Filename string
	Lines    chan *Line
	Config

	file           *os.File
	reader         *bufio.Reader
	useLargeBuffer bool

	watcher watch.FileWatcher
	changes *watch.FileChanges

	curOffset int64
	tomb.Tomb // provides: Done, Kill, Dying
	dropCnt   int

	lk sync.Mutex

	FileDeletedCh chan struct{}

	linePool sync.Pool

	// pending holds UTF-8 continuation bytes carried over from a previous
	// readLine ErrBufferFull-driven rune-boundary backoff. The next readSlice
	// prepends them to the fresh read so the two chunks still concatenate to
	// the original line byte-for-byte. bufio.Reader.UnreadByte only
	// guarantees one byte, so we keep a small (at most utf8.UTFMax-1 = 3
	// byte) buffer here instead.
	pending []byte

	// utf8BoundaryBackoffs counts how many times readLine or partitionString
	// held bytes back from a cut to align to a rune boundary. Exposed via
	// UTF8BoundaryBackoffs() for test and future metric emission.
	utf8BoundaryBackoffs atomic.Int64
}

// TailFile begins tailing the file. Output stream is made available
// via the `Tail.Lines` channel. To handle errors during tailing,
// invoke the `Wait` or `Err` method after finishing reading from the
// `Lines` channel.
func TailFile(filename string, config Config) (*Tail, error) {
	if config.ReOpen && !config.Follow {
		return nil, errors.New("cannot set ReOpen without Follow.")
	}

	t := &Tail{
		Filename:      filename,
		Lines:         make(chan *Line),
		Config:        config,
		FileDeletedCh: make(chan struct{}),
		linePool: sync.Pool{
			New: func() any { return &Line{} },
		},
	}

	// when Logger was not specified in config, create new one
	if t.Logger == nil {
		t.Logger = models.NewLogger("inputs", "tail", "")
	}

	if t.Poll {
		t.watcher = watch.NewPollingFileWatcher(filename)
	} else {
		t.watcher = watch.NewInotifyFileWatcher(filename)
	}

	if t.MustExist {
		var err error
		t.file, err = OpenFile(t.Filename)
		if err != nil {
			return nil, err
		}
		OpenFileCount.Add(1)
	}

	if !config.ReOpen {
		go t.exitOnDeletion()
	}

	go t.tailFileSync()

	return t, nil
}

// Return the file's current position, like stdio's ftell().
// But this value is not very accurate.
// it may readed one line in the chan(tail.Lines),
// so it may lost one line.
func (tail *Tail) Tell() (offset int64, err error) {
	if tail.file == nil {
		return
	}
	offset, err = tail.file.Seek(0, os.SEEK_CUR)
	if err != nil {
		return
	}

	tail.lk.Lock()
	defer tail.lk.Unlock()
	if tail.reader == nil {
		return
	}

	offset -= int64(tail.reader.Buffered())
	return
}

// Stop stops the tailing activity.
func (tail *Tail) Stop() error {
	tail.Kill(nil)
	return tail.Wait()
}

// StopAtEOF stops tailing as soon as the end of the file is reached.
// Does not wait until tailer is dead.
func (tail *Tail) StopAtEOF() {
	tail.Kill(errStopAtEOF)
}

var errStopAtEOF = errors.New("tail: stop at eof")

func (tail *Tail) close() {
	if tail.dropCnt > 0 {
		tail.Logger.Errorf("Dropped %v lines for stopped tail for file %v", tail.dropCnt, tail.Filename)
	}
	close(tail.Lines)
	tail.CloseFile()
}

func (tail *Tail) IsFileClosed() bool {
	return tail.file == nil
}

func (tail *Tail) CloseFile() {
	if tail.file != nil {
		tail.file.Close()
		tail.file = nil
		OpenFileCount.Add(-1)
	}
}

func (tail *Tail) Reopen(resetOffset bool) error {
	tail.CloseFile()
	for {
		var err error
		tail.file, err = OpenFile(tail.Filename)
		if resetOffset {
			tail.curOffset = 0
		}
		if err != nil {
			if os.IsNotExist(err) {
				tail.Logger.Debugf("Waiting for %s to appear...", tail.Filename)
				if err := tail.watcher.BlockUntilExists(&tail.Tomb); err != nil {
					if err == tomb.ErrDying {
						return err
					}
					return fmt.Errorf("Failed to detect creation of %s: %s", tail.Filename, err)
				}
				continue
			}
			return fmt.Errorf("Unable to open file %s: %s", tail.Filename, err)
		}
		break
	}
	OpenFileCount.Add(1)

	tail.openReader()
	if !resetOffset && tail.curOffset > 0 {
		err := tail.seekTo(SeekInfo{Offset: tail.curOffset, Whence: io.SeekStart})
		if err != nil {
			return fmt.Errorf("unable to restore offset on reopen: %w", err)
		}
	}

	return nil
}

// readLine() tries to return a single line, not including the end-of-line bytes.
// If the line is too long for the buffer then partial line will be returned.
// The rest of the line will be returned from future calls. If error is encountered
// before finding the end-of-line bytes(often io.EOF), it returns the data read
// before the error and the error itself.
func (tail *Tail) readLine() (string, error) {
	if tail.Config.IsUTF16 {
		return tail.readlineUtf16()
	}
	tail.lk.Lock()
	defer tail.lk.Unlock()

	line, err := tail.readSlice('\n')
	if err == bufio.ErrBufferFull {
		// Handle the case where "\r\n" straddles the buffer.
		if len(line) > 0 && line[len(line)-1] == '\r' {
			tail.unreadByte()
			line = line[:len(line)-1]
		}
		// UTF-8 rune-boundary backoff. The raw cut lands at MaxLineSize
		// bytes with no rune awareness, so a multi-byte sequence that
		// straddles the boundary would be split into invalid bytes
		// downstream (every orphan byte then becomes U+FFFD in the SDK's
		// JSON encoder). When the file is UTF-8, inspect the last up-to-3
		// bytes for an incomplete-but-valid UTF-8 prefix; if found, back
		// the cut off to that rune start and park the trailing 1-3 bytes
		// in tail.pending so the next readSlice prepends them. Invalid
		// UTF-8 input (orphan continuations, lone start bytes whose
		// FullRune is already true as RuneError) keeps today's behavior
		// exactly.
		if tail.Config.IsUTF8 && len(line) >= utf8.UTFMax {
			runeStart := -1
			for j := 1; j < utf8.UTFMax; j++ {
				if utf8.RuneStart(line[len(line)-j]) {
					runeStart = len(line) - j
					break
				}
			}
			if runeStart > 0 && !utf8.FullRune(line[runeStart:]) {
				carry := len(line) - runeStart
				// Stash the carried bytes for the next readSlice. Decrement
				// curOffset so the (shorter) line's reported offset still
				// equals the file offset of its last byte + 1. readSlice
				// restores the offset when it prepends tail.pending to the
				// next word.
				tail.pending = append(tail.pending[:0], line[runeStart:]...)
				tail.curOffset -= int64(carry)
				line = line[:runeStart]
				tail.utf8BoundaryBackoffs.Add(1)
				if tail.Logger != nil {
					tail.Logger.Debugf("UTF-8 rune-boundary backoff on %s: %d byte(s) carried to next chunk at offset %d",
						tail.Filename, carry, tail.curOffset)
				}
			}
		}
		return string(line), nil
	}

	if len(line) > 0 && line[len(line)-1] == '\n' {
		drop := 1
		if len(line) > 1 && line[len(line)-2] == '\r' {
			drop = 2
		}
		line = line[:len(line)-drop]
	}
	return string(line), err
}

func (tail *Tail) readlineUtf16() (string, error) {
	tail.lk.Lock()
	defer tail.lk.Unlock()

	var cur []byte
	var err error
	var res [][]byte
	var resSize int

	for {
		// Check LF
		cur, err = tail.readSlice('\n')
		// buffer size is even
		if err == bufio.ErrBufferFull {
			// Handle the case where "\r\n" straddles the buffer.
			if len(cur) > 1 && cur[len(cur)-1] == '\x00' && cur[len(cur)-2] == '\r' {
				tail.unreadByte()
				tail.unreadByte()
				cur = cur[:len(cur)-2]
			}
			err = nil
			break
		}
		if err != nil {
			break
		}
		// We only care about 0a00
		if len(cur)%2 != 0 {
			var nextByte byte
			nextByte, err = tail.readByte()
			if err != nil {
				break
			}
			if nextByte == '\x00' {
				// confirmed it's LF, check for Carriage Return
				if len(cur) >= 3 && cur[len(cur)-2] == '\x00' && cur[len(cur)-3] == '\r' {
					cur = cur[:len(cur)-3]
				} else {
					cur = cur[:len(cur)-1]
				}
				break
			}
			cur = append(cur, nextByte)
		}
		// Use MaxLineSize if configured, otherwise use DefaultMaxEventSize
		maxSize := constants.DefaultMaxEventSize // Use the constant defined in constants package
		if tail.MaxLineSize > 0 {
			maxSize = tail.MaxLineSize
		}
		if resSize+len(cur) >= maxSize {
			break
		}
		buf := make([]byte, len(cur))
		copy(buf, cur)
		res = append(res, buf)
		resSize += len(buf)
	}

	resSize += len(cur)
	res = append(res, cur)

	finalRes := make([]byte, resSize)
	n := 0
	for i := range res {
		n += copy(finalRes[n:], res[i])
	}

	return string(finalRes), err
}

func (tail *Tail) tailFileSync() {
	defer tail.Done()
	defer tail.close()

	if !tail.MustExist {
		// deferred first open.
		err := tail.Reopen(true)
		if err != nil {
			if err != tomb.ErrDying {
				tail.Kill(err)
			}
			return
		}
	}
	// openReader should be invoked before seekTo
	tail.openReader()

	// Send the lines for the gaps found in the state file's ranges
	if len(tail.GapsToRead) > 0 {
		for _, seekRange := range tail.GapsToRead {
			if seekRange.IsEndOffsetUnbounded() {
				continue
			}

			err := tail.seekTo(SeekInfo{Offset: seekRange.StartOffsetInt64(), Whence: io.SeekStart})
			if err != nil {
				tail.Logger.Debugf("Failed to seek %s to %d for gaps: %s", tail.Filename, seekRange.StartOffsetInt64(), err)
				continue
			}

			for tail.curOffset < seekRange.EndOffsetInt64() {
				line, err := tail.readLine()

				if err == nil {
					tail.sendLine(line, tail.curOffset)
				} else if err == io.EOF {
					tail.Logger.Debugf("Failed read line due to EOF on %s for gaps: %s", tail.Filename, err)
					break
				} else {
					tail.Logger.Debugf("Failed read line on %s for gaps: %s", tail.Filename, err)
				}

				select {
				case <-tail.Dying():
					if tail.Err() == errStopAtEOF {
						continue
					}
					return
				default:
				}
			}
		}
	}

	// Seek to requested location on first open of the file.
	if tail.Location != nil {
		err := tail.seekTo(*tail.Location)
		tail.Logger.Debugf("Seeked %s - %+v\n", tail.Filename, tail.Location)
		if err != nil {
			tail.Killf("Seek error on %s: %s", tail.Filename, err)
			return
		}
	}

	// Only set up file watchers if we're following the file
	if tail.Follow {
		if err := tail.watchChanges(); err != nil {
			tail.Killf("Error watching for changes on %s: %s", tail.Filename, err)
			return
		}
	}

	var backupOffset int64
	// Read line by line.
	for {
		// do not set backupOffset in named pipes
		if !tail.Pipe {
			backupOffset = tail.curOffset
		}
		line, err := tail.readLine()

		// Process `line` even if err is EOF.
		if err == nil {
			cooloff := !tail.sendLine(line, tail.curOffset)
			if cooloff {
				// Wait a second before seeking till the end of
				// file when rate limit is reached.
				msg := "too much log activity; waiting a second before resuming tailing"
				// Warning: Make sure to release line once done!
				lineObject := tail.linePool.Get().(*Line)
				lineObject.Text = msg
				lineObject.Time = time.Now()
				lineObject.Err = errors.New(msg)
				lineObject.Offset = tail.curOffset
				tail.Lines <- lineObject
				select {
				case <-time.After(time.Second):
				case <-tail.Dying():
					return
				}
				if err := tail.seekEnd(); err != nil {
					tail.Kill(err)
					return
				}
			}
		} else if err == io.EOF {
			if !tail.Follow {
				if line != "" {
					tail.sendLine(line, tail.curOffset)
				}
				return
			}

			if tail.Follow && line != "" {
				// this has the potential to never return the last line if
				// it's not followed by a newline; seems a fair trade here
				err := tail.seekTo(SeekInfo{Offset: backupOffset, Whence: 0})
				if err != nil {
					tail.Kill(err)
					return
				}
			}

			// When EOF is reached, wait for more data to become
			// available. Wait strategy is based on the `tail.watcher`
			// implementation (inotify or polling).
			err := tail.waitForChanges()
			if err != nil {
				if err == ErrDeletedNotReOpen {
					close(tail.FileDeletedCh)
					for {
						line, errReadLine := tail.readLine()
						if errReadLine == nil {
							tail.sendLine(line, tail.curOffset)
						} else {
							return
						}
					}
				} else if err != ErrStop {
					tail.Kill(err)
				}
				return
			}
		} else {
			// non-EOF error
			tail.Killf("Error reading %s: %s", tail.Filename, err)
			return
		}

		select {
		case <-tail.Dying():
			if tail.Err() == errStopAtEOF {
				continue
			}
			return
		default:
		}
	}
}

// watchChanges ensures the watcher is running.
func (tail *Tail) watchChanges() error {
	if tail.changes != nil {
		return nil
	}
	pos, err := tail.file.Seek(0, os.SEEK_CUR)
	if err != nil {
		return err
	}
	tail.changes, err = tail.watcher.ChangeEvents(&tail.Tomb, pos)
	return err
}

// waitForChanges waits until the file has been appended, deleted,
// moved or truncated. When moved or deleted - the file will be
// reopened if ReOpen is true. Truncated files are always reopened.
func (tail *Tail) waitForChanges() error {
	if err := tail.watchChanges(); err != nil {
		return err
	}

	select {
	case <-tail.changes.Modified:
		return nil
	case <-tail.changes.Deleted:
		tail.changes = nil
		if tail.ReOpen {
			tail.Logger.Infof("Re-opening moved/deleted file %s ...", tail.Filename)
			if err := tail.Reopen(true); err != nil {
				return err
			}
			tail.Logger.Debugf("Successfully reopened %s", tail.Filename)
			tail.openReader()
			return nil
		} else {
			tail.Logger.Warnf("Stopping tail as file no longer exists: %s", tail.Filename)
			return ErrDeletedNotReOpen
		}
	case <-tail.changes.Truncated:
		// Always reopen truncated files (Follow is true)
		tail.Logger.Infof("Re-opening truncated file %s ...", tail.Filename)
		if err := tail.Reopen(true); err != nil {
			return err
		}
		tail.Logger.Debugf("Successfully reopened truncated %s", tail.Filename)
		tail.openReader()
		return nil
	case <-tail.Dying():
		return ErrStop
	}
}

func (tail *Tail) openReader() {
	tail.lk.Lock()
	if tail.useLargeBuffer {
		tail.reader = bufio.NewReaderSize(tail.file, tail.MaxLineSize)
	} else {
		tail.reader = bufio.NewReaderSize(tail.file, constants.DefaultReaderBufferSize)
	}
	// Drop any UTF-8 bytes carried from the previous reader. On rotation,
	// truncation, or any other reopen, the new file is discontinuous with
	// those bytes — prepending them would corrupt the first line of the
	// new file. The curOffset decrement that was paired with the carry is
	// implicitly undone by callers: Reopen(true) resets curOffset to 0,
	// and Reopen(false) immediately follows with seekTo() which recomputes
	// curOffset via Tell().
	tail.pending = tail.pending[:0]
	tail.lk.Unlock()
}

func (tail *Tail) readSlice(delim byte) ([]byte, error) {
	// Pull any UTF-8 bytes that a previous ErrBufferFull-driven rune-boundary
	// backoff parked here; they are prepended to this read's result so the
	// returned bytes concatenate to the original file content. curOffset was
	// decremented by len(carried) when the carry was created, so adding it
	// back after the read keeps offsets consistent.
	var carried []byte
	if len(tail.pending) > 0 {
		carried = append([]byte(nil), tail.pending...)
		tail.pending = tail.pending[:0]
	}

	// First try: normal ReadSlice
	word, err := tail.reader.ReadSlice(delim)
	if err != bufio.ErrBufferFull {
		tail.curOffset += int64(len(word))
		if len(carried) > 0 {
			tail.curOffset += int64(len(carried))
			word = append(carried, word...)
		}
		return word, err // fast path when no carry: no allocation
	}

	// Check if buffer already upgraded
	if tail.reader.Size() == tail.MaxLineSize {
		tail.curOffset += int64(len(word))
		if len(carried) > 0 {
			tail.curOffset += int64(len(carried))
			word = append(carried, word...)
		}
		return word, bufio.ErrBufferFull
	}

	// Buffer was too small → allocate ONCE
	buf := append([]byte(nil), word...)

	// Copy any unread buffered data
	unread := tail.reader.Buffered()
	if unread > 0 {
		peek, _ := tail.reader.Peek(unread)
		buf = append(buf, peek...)
		tail.reader.Discard(unread)
	}

	// Switch to a bigger buffer
	tail.reader = bufio.NewReaderSize(tail.file, tail.MaxLineSize)
	// In the event that the tail is re-opened, we don't want to have to do this
	// re-sizing of the buffer again. The reader should just re-open with the larger buffer
	tail.useLargeBuffer = true

	word, err = tail.reader.ReadSlice(delim)
	buf = append(buf, word...)
	tail.curOffset += int64(len(buf))
	if len(carried) > 0 {
		tail.curOffset += int64(len(carried))
		buf = append(carried, buf...)
	}

	return buf, err
}

func (tail *Tail) seekEnd() error {
	return tail.seekTo(SeekInfo{Offset: 0, Whence: os.SEEK_END})
}

func (tail *Tail) seekTo(pos SeekInfo) error {
	_, err := tail.file.Seek(pos.Offset, pos.Whence)
	if err != nil {
		return fmt.Errorf("Seek error on %s: %s", tail.Filename, err)
	}
	// Reset the read buffer whenever the file is re-seek'ed, and drop any
	// UTF-8 bytes carried from the pre-seek position: they belong to a line
	// at the old offset and are discontinuous with the new one. The paired
	// curOffset decrement is implicitly undone by the Tell() call below,
	// which recomputes curOffset from the file's new position.
	tail.lk.Lock()
	tail.reader.Reset(tail.file)
	tail.pending = tail.pending[:0]
	tail.lk.Unlock()
	tail.curOffset, err = tail.Tell()
	return err
}

// sendLine sends the line(s) to Lines channel, splitting longer lines
// if necessary. Return false if rate limit is reached.
func (tail *Tail) sendLine(line string, offset int64) bool {
	now := time.Now()
	lines := []string{line}

	// Split longer lines
	if tail.MaxLineSize > 0 && len(line) > tail.MaxLineSize {
		lines = partitionString(line, tail.MaxLineSize, tail.Config.IsUTF8)
		if tail.Config.IsUTF8 {
			// A non-final chunk shorter than MaxLineSize is only possible
			// because partitionString backed its cut off to a rune start.
			// Count each so operators can observe the fix at work via
			// UTF8BoundaryBackoffs(); bounded by one event per cut.
			for i := 0; i < len(lines)-1; i++ {
				if len(lines[i]) < tail.MaxLineSize {
					tail.utf8BoundaryBackoffs.Add(1)
				}
			}
		}
	}

	for i, line := range lines {
		// This select is to avoid blockage on the tail.Lines chan
		// Warning: Make sure to release line once done!
		lineObject := tail.linePool.Get().(*Line)
		lineObject.Text = line
		lineObject.Time = now
		lineObject.Err = nil
		lineObject.Offset = offset
		select {
		case tail.Lines <- lineObject:
		case <-tail.Dying():
			if tail.Err() == errStopAtEOF {
				// Try sending, even if it blocks.
				tail.Lines <- lineObject
			} else {
				tail.dropCnt += len(lines) - i
				return true
			}
		}
	}

	if tail.Config.RateLimiter != nil {
		ok := tail.Config.RateLimiter.Pour(uint16(len(lines)))
		if !ok {
			tail.Logger.Debugf("Leaky bucket full (%v); entering 1s cooloff period.\n",
				tail.Filename)
			return false
		}
	}

	return true
}

// Cleanup removes inotify watches added by the tail package. This function is
// meant to be invoked from a process's exit handler. Linux kernel may not
// automatically remove inotify watches after the process exits.
func (tail *Tail) Cleanup() {
	watch.Cleanup(tail.Filename)
}

// A wrapper of bufio ReadByte
func (tail *Tail) readByte() (b byte, err error) {
	b, err = tail.reader.ReadByte()
	tail.curOffset += 1
	return
}

// A wrapper of bufio UnreadByte
func (tail *Tail) unreadByte() (err error) {
	err = tail.reader.UnreadByte()
	tail.curOffset -= 1
	return
}

func (tail *Tail) ReleaseLine(line *Line) {
	*line = Line{}
	tail.linePool.Put(line)
}

// A wrapper of tomb Err()
func (tail *Tail) UnexpectedError() (err error) {
	err = tail.Err()
	// ignore the error ErrStillAlive and errStopAtEOF
	if err == tomb.ErrStillAlive || err == errStopAtEOF {
		err = nil
	}
	return
}

func (tail *Tail) exitOnDeletion() {
	ticker := time.NewTicker(exitOnDeletionCheckDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if tail.isFileDeleted() {
				select {
				case <-tail.Dying():
					return
				case <-time.After(exitOnDeletionWaitDuration):
					// wait for some time in case tail can catch up with the EOF
					msg := fmt.Sprintf("File %s was deleted, but file content is not tailed completely.", tail.Filename)
					tail.Logger.Error(msg)
					tail.Kill(errors.New(msg))
					return
				}
			}
		case <-tail.Dying():
			return
		}
	}
}

// partitionString partitions the string into chunks of given size,
// with the last chunk of variable size.
//
// When utf8Aware is true, each non-final chunk is right-trimmed by at most
// utf8.UTFMax-1 bytes if the trailing bytes form an incomplete-but-valid
// UTF-8 prefix. The trimmed bytes are prepended to the next chunk, so the
// concatenation of all chunks equals the input byte-for-byte. The last
// chunk is never trimmed. If chunkSize < utf8.UTFMax or the backoff would
// land at the chunk start, the raw cut is emitted and backoff is skipped
// — invalid-UTF-8 input also keeps today's behavior exactly.
func partitionString(s string, chunkSize int, utf8Aware bool) []string {
	if chunkSize <= 0 {
		panic("Invalid chunkSize")
	}
	length := len(s)
	chunks := 1 + length/chunkSize
	start := 0
	end := chunkSize
	parts := make([]string, 0, chunks)
	for {
		if end > length {
			end = length
		}
		// Rune-boundary backoff for non-final chunks only. The raw cut
		// at s[start:end] has no rune awareness, so a multi-byte sequence
		// straddling `end` would end up as invalid trailing bytes in this
		// chunk and leading bytes in the next — both invalid UTF-8. Walk
		// back up to utf8.UTFMax-1 bytes to find the last RuneStart and,
		// if the resulting [i:end] is an incomplete rune, cut at i and
		// let the next iteration pick up the carried bytes.
		if utf8Aware && end < length && (end-start) >= utf8.UTFMax {
			runeStart := -1
			for j := 1; j < utf8.UTFMax; j++ {
				if utf8.RuneStart(s[end-j]) {
					runeStart = end - j
					break
				}
			}
			if runeStart > start && !utf8.FullRuneInString(s[runeStart:end]) {
				end = runeStart
			}
		}
		parts = append(parts, s[start:end])
		if end == length {
			break
		}
		start, end = end, end+chunkSize
	}
	return parts
}

// UTF8BoundaryBackoffs returns the monotonic count of times readLine or
// partitionString backed a cut off to align it to a UTF-8 rune boundary.
// Exposed for tests and future metric emission.
func (tail *Tail) UTF8BoundaryBackoffs() int64 {
	return tail.utf8BoundaryBackoffs.Load()
}
