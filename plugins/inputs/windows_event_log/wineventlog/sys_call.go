//go:build windows
// +build windows

// Portions Licensed under the Apache License, Version 2.0, Copyright (c) 2012–2017 Elastic <http://www.elastic.co>

package wineventlog

import (
	"syscall"
	"unsafe"
)

// EvtHandle is a handle to the event log.
type EvtHandle uintptr

// EvtSubscribeFlag defines the possible values that specify when to start subscribing to events.
type EvtSubscribeFlag uint32

const (
	EvtSubscribeStartAfterBookmark EvtSubscribeFlag = 3
)

// EvtRenderFlag defines the values that specify what to render.
type EvtRenderFlag uint32

// EVT_RENDER_FLAGS enumeration
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa385563(v=vs.85).aspx
const (
	EvtRenderEventXml EvtRenderFlag = 1
)

// EvtRenderContextFlag defines the values that specify the type of information
// to access from the event.
type EvtRenderContextFlag uint32

// EvtFormatMessageFlag defines the values that specify the message string from
// the event to format.
type EvtFormatMessageFlag uint32

const (
	EvtFormatMessageXml EvtFormatMessageFlag = 9
)

var (
	// For Windows versions newer than 2003
	modwevtapi                   = syscall.NewLazyDLL("wevtapi.dll")
	procEvtSubscribe             = modwevtapi.NewProc("EvtSubscribe")
	procEvtCreateBookmark        = modwevtapi.NewProc("EvtCreateBookmark")
	procEvtCreateRenderContext   = modwevtapi.NewProc("EvtCreateRenderContext")
	procEvtRender                = modwevtapi.NewProc("EvtRender")
	procEvtClose                 = modwevtapi.NewProc("EvtClose")
	procEvtNext                  = modwevtapi.NewProc("EvtNext")
	procEvtFormatMessage         = modwevtapi.NewProc("EvtFormatMessage")
	procEvtOpenPublisherMetadata = modwevtapi.NewProc("EvtOpenPublisherMetadata")
)

func EvtSubscribe(session EvtHandle, signalEvent uintptr, channelPath *uint16, query *uint16, bookmark EvtHandle, context uintptr, callback syscall.Handle, flags EvtSubscribeFlag) (handle EvtHandle, err error) {
	r0, _, e1 := syscall.Syscall9(procEvtSubscribe.Addr(), 8, uintptr(session), uintptr(signalEvent), uintptr(unsafe.Pointer(channelPath)), uintptr(unsafe.Pointer(query)), uintptr(bookmark), uintptr(context), uintptr(callback), uintptr(flags), 0)
	handle = EvtHandle(r0)
	if handle == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func EvtCreateBookmark(bookmarkXML *uint16) (handle EvtHandle, err error) {
	r0, _, e1 := syscall.Syscall(procEvtCreateBookmark.Addr(), 1, uintptr(unsafe.Pointer(bookmarkXML)), 0, 0)
	handle = EvtHandle(r0)
	if handle == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func EvtCreateRenderContext(ValuePathsCount uint32, valuePaths uintptr, flags EvtRenderContextFlag) (handle EvtHandle, err error) {
	r0, _, e1 := syscall.Syscall(procEvtCreateRenderContext.Addr(), 3, uintptr(ValuePathsCount), uintptr(valuePaths), uintptr(flags))
	handle = EvtHandle(r0)
	if handle == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func EvtRender(context EvtHandle, fragment EvtHandle, flags EvtRenderFlag, bufferSize uint32, buffer *byte, bufferUsed *uint32, propertyCount *uint32) (err error) {
	r1, _, e1 := syscall.Syscall9(procEvtRender.Addr(), 7, uintptr(context), uintptr(fragment), uintptr(flags), uintptr(bufferSize), uintptr(unsafe.Pointer(buffer)), uintptr(unsafe.Pointer(bufferUsed)), uintptr(unsafe.Pointer(propertyCount)), 0, 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func EvtClose(object EvtHandle) (err error) {
	r1, _, e1 := syscall.Syscall(procEvtClose.Addr(), 1, uintptr(object), 0, 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func EvtNext(resultSet EvtHandle, eventArraySize uint32, eventArray *EvtHandle, timeout uint32, flags uint32, numReturned *uint32) (err error) {
	r1, _, e1 := syscall.Syscall6(procEvtNext.Addr(), 6, uintptr(resultSet), uintptr(eventArraySize), uintptr(unsafe.Pointer(eventArray)), uintptr(timeout), uintptr(flags), uintptr(unsafe.Pointer(numReturned)))
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func EvtFormatMessage(publisherMetadata EvtHandle, event EvtHandle, messageID uint32, valueCount uint32, values uintptr, flags EvtFormatMessageFlag, bufferSize uint32, buffer *byte, bufferUsed *uint32) (err error) {
	r1, _, e1 := syscall.Syscall9(procEvtFormatMessage.Addr(), 9, uintptr(publisherMetadata), uintptr(event), uintptr(messageID), uintptr(valueCount), uintptr(values), uintptr(flags), uintptr(bufferSize), uintptr(unsafe.Pointer(buffer)), uintptr(unsafe.Pointer(bufferUsed)))
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func EvtOpenPublisherMetadata(session EvtHandle, publisherIdentity *uint16, logFilePath *uint16, locale uint32, flags uint32) (handle EvtHandle, err error) {
	r0, _, e1 := syscall.Syscall6(procEvtOpenPublisherMetadata.Addr(), 5, uintptr(session), uintptr(unsafe.Pointer(publisherIdentity)), uintptr(unsafe.Pointer(logFilePath)), uintptr(locale), uintptr(flags), 0)
	handle = EvtHandle(r0)
	if handle == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}
