//go:build windows
// +build windows

// Portions Licensed under the Apache License, Version 2.0, Copyright (c) 2012–2017 Elastic <http://www.elastic.co>

package wineventlog

import (
	"syscall"
	"unsafe"
)

// WindowsEventAPI defines the interface for Windows Event Log API operations
// This allows us to mock the Windows API calls for testing
type WindowsEventAPI interface {
	EvtSubscribe(session EvtHandle, signalEvent uintptr, channelPath *uint16, query *uint16, bookmark EvtHandle, context uintptr, callback syscall.Handle, flags EvtSubscribeFlag) (handle EvtHandle, err error)
	EvtQuery(session EvtHandle, path *uint16, query *uint16, flags EvtQueryFlag) (EvtHandle, error)
	EvtNext(resultSet EvtHandle, eventArraySize uint32, eventArray *EvtHandle, timeout uint32, flags uint32, numReturned *uint32) error
	EvtClose(handle EvtHandle) error
	EvtRender(context EvtHandle, fragment EvtHandle, flags EvtRenderFlag, bufferSize uint32, buffer *byte, bufferUsed *uint32, propertyCount *uint32) error
	EvtCreateBookmark(bookmarkXML *uint16) (handle EvtHandle, err error)
	EvtFormatMessage(publisherMetadata EvtHandle, event EvtHandle, messageID uint32, valueCount uint32, values uintptr, flags EvtFormatMessageFlag, bufferSize uint32, buffer *byte, bufferUsed *uint32) error
	EvtOpenPublisherMetadata(session EvtHandle, publisherId *uint16, logFilePath *uint16, locale uint32, flags uint32) (EvtHandle, error)
}

// EvtHandle is a handle to the event log.
type EvtHandle uintptr

// EvtSubscribeFlag defines the possible values that specify when to start subscribing to events.
type EvtSubscribeFlag uint32

const (
	EvtSubscribeStartAfterBookmark EvtSubscribeFlag = 3
)

// EvtQueryFlag defines the values that specify how to return the query results and whether you are query against a channel or log file.
// https://learn.microsoft.com/en-us/windows/win32/api/winevt/nf-winevt-evtquery
type EvtQueryFlag uint32

const (
	EvtQueryChannelPath EvtQueryFlag = 1
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
	procEvtQuery                 = modwevtapi.NewProc("EvtQuery")
	procEvtCreateBookmark        = modwevtapi.NewProc("EvtCreateBookmark")
	procEvtCreateRenderContext   = modwevtapi.NewProc("EvtCreateRenderContext")
	procEvtRender                = modwevtapi.NewProc("EvtRender")
	procEvtClose                 = modwevtapi.NewProc("EvtClose")
	procEvtNext                  = modwevtapi.NewProc("EvtNext")
	procEvtFormatMessage         = modwevtapi.NewProc("EvtFormatMessage")
	procEvtOpenPublisherMetadata = modwevtapi.NewProc("EvtOpenPublisherMetadata")
)

// windowsEventAPI implements the interface using actual Windows API calls
type windowsEventAPI struct{}

func NewWindowsEventAPI() WindowsEventAPI {
	return &windowsEventAPI{}
}

func (w *windowsEventAPI) EvtSubscribe(session EvtHandle, signalEvent uintptr, channelPath *uint16, query *uint16, bookmark EvtHandle, context uintptr, callback syscall.Handle, flags EvtSubscribeFlag) (handle EvtHandle, err error) {
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

func (w *windowsEventAPI) EvtQuery(session EvtHandle, path *uint16, query *uint16, flags EvtQueryFlag) (handle EvtHandle, err error) {
	r0, _, e1 := syscall.Syscall6(procEvtQuery.Addr(), 4, uintptr(session), uintptr(unsafe.Pointer(path)), uintptr(unsafe.Pointer(query)), uintptr(flags), 0, 0)
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

func (w *windowsEventAPI) EvtCreateBookmark(bookmarkXML *uint16) (handle EvtHandle, err error) {
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

func (w *windowsEventAPI) EvtCreateRenderContext(ValuePathsCount uint32, valuePaths uintptr, flags EvtRenderContextFlag) (handle EvtHandle, err error) {
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

func (w *windowsEventAPI) EvtRender(context EvtHandle, fragment EvtHandle, flags EvtRenderFlag, bufferSize uint32, buffer *byte, bufferUsed *uint32, propertyCount *uint32) (err error) {
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

func (w *windowsEventAPI) EvtClose(object EvtHandle) (err error) {
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

func (w *windowsEventAPI) EvtNext(resultSet EvtHandle, eventArraySize uint32, eventArray *EvtHandle, timeout uint32, flags uint32, numReturned *uint32) (err error) {
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

func (w *windowsEventAPI) EvtFormatMessage(publisherMetadata EvtHandle, event EvtHandle, messageID uint32, valueCount uint32, values uintptr, flags EvtFormatMessageFlag, bufferSize uint32, buffer *byte, bufferUsed *uint32) (err error) {
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

func (w *windowsEventAPI) EvtOpenPublisherMetadata(session EvtHandle, publisherIdentity *uint16, logFilePath *uint16, locale uint32, flags uint32) (handle EvtHandle, err error) {
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
