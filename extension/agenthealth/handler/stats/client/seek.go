// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package client

import (
	"io"
)

// Taken from:
// - AWS SDK Go v2: https://github.com/aws/aws-sdk-go-v2/blob/d2074554db4cd82d3a4b9e6d949c8eecb336477b/feature/s3/manager/types.go#L20

// readSeekCloser wraps an io.Reader returning a readerSeekerCloser.
func readSeekCloser(r io.Reader) *readerSeekerCloser {
	return &readerSeekerCloser{r}
}

// readerSeekerCloser represents a reader that can also delegate io.Seeker and
// io.Closer interfaces to the underlying object if they are available.
type readerSeekerCloser struct {
	r io.Reader
}

// seekerLen attempts to get the number of bytes remaining at the seeker's
// current position.  Returns the number of bytes remaining or error.
func seekerLen(s io.Seeker) (int64, error) {
	// Determine if the seeker is actually seekable. readerSeekerCloser
	// hides the fact that io.Readers might not actually be seekable.
	switch v := s.(type) {
	case *readerSeekerCloser:
		return v.GetLen()
	}

	return computeSeekerLength(s)
}

// GetLen returns the length of the bytes remaining in the underlying reader.
// Checks first for Len(), then io.Seeker to determine the size of the
// underlying reader.
//
// Will return -1 if the length cannot be determined.
func (r *readerSeekerCloser) GetLen() (int64, error) {
	if l, ok := r.HasLen(); ok {
		return int64(l), nil
	}

	if s, ok := r.r.(io.Seeker); ok {
		return computeSeekerLength(s)
	}

	return -1, nil
}

func computeSeekerLength(s io.Seeker) (int64, error) {
	curOffset, err := s.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	endOffset, err := s.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	_, err = s.Seek(curOffset, io.SeekStart)
	if err != nil {
		return 0, err
	}

	return endOffset - curOffset, nil
}

// HasLen returns the length of the underlying reader if the value implements
// the Len() int method.
func (r *readerSeekerCloser) HasLen() (int, bool) {
	type lenner interface {
		Len() int
	}

	if lr, ok := r.r.(lenner); ok {
		return lr.Len(), true
	}

	return 0, false
}

// Read reads from the reader up to size of p. The number of bytes read, and
// error if it occurred will be returned.
//
// If the reader is not an io.Reader zero bytes read, and nil error will be
// returned.
//
// Performs the same functionality as io.Reader Read
func (r *readerSeekerCloser) Read(p []byte) (int, error) {
	switch t := r.r.(type) {
	case io.Reader:
		return t.Read(p)
	}
	return 0, nil
}

// Seek sets the offset for the next Read to offset, interpreted according to
// whence: 0 means relative to the origin of the file, 1 means relative to the
// current offset, and 2 means relative to the end. Seek returns the new offset
// and an error, if any.
//
// If the readerSeekerCloser is not an io.Seeker nothing will be done.
func (r *readerSeekerCloser) Seek(offset int64, whence int) (int64, error) {
	switch t := r.r.(type) {
	case io.Seeker:
		return t.Seek(offset, whence)
	}
	return int64(0), nil
}

// Close closes the readerSeekerCloser.
//
// If the readerSeekerCloser is not an io.Closer nothing will be done.
func (r *readerSeekerCloser) Close() error {
	switch t := r.r.(type) {
	case io.Closer:
		return t.Close()
	}
	return nil
}
