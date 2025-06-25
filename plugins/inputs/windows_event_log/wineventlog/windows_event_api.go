//go:build windows
// +build windows

// Portions Licensed under the Apache License, Version 2.0, Copyright (c) 2012–2017 Elastic <http://www.elastic.co>

package wineventlog

import "syscall"

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

// windowsEventAPI implements the interface using actual Windows API calls
type windowsEventAPI struct{}

func NewWindowsEventAPI() WindowsEventAPI {
	return &windowsEventAPI{}
}

func (r *windowsEventAPI) EvtSubscribe(session EvtHandle, signalEvent uintptr, channelPath *uint16, query *uint16, bookmark EvtHandle, context uintptr, callback syscall.Handle, flags EvtSubscribeFlag) (handle EvtHandle, err error) {
	return EvtSubscribe(session, signalEvent, channelPath, query, bookmark, context, callback, flags)
}

func (r *windowsEventAPI) EvtQuery(session EvtHandle, path *uint16, query *uint16, flags EvtQueryFlag) (handle EvtHandle, err error) {
	return EvtQuery(session, path, query, flags)
}

func (r *windowsEventAPI) EvtNext(resultSet EvtHandle, eventArraySize uint32, eventArray *EvtHandle, timeout uint32, flags uint32, numReturned *uint32) error {
	return EvtNext(resultSet, eventArraySize, eventArray, timeout, flags, numReturned)
}

func (r *windowsEventAPI) EvtClose(handle EvtHandle) error {
	return EvtClose(handle)
}

func (r *windowsEventAPI) EvtRender(context EvtHandle, fragment EvtHandle, flags EvtRenderFlag, bufferSize uint32, buffer *byte, bufferUsed *uint32, propertyCount *uint32) error {
	return EvtRender(context, fragment, flags, bufferSize, buffer, bufferUsed, propertyCount)
}

func (r *windowsEventAPI) EvtCreateBookmark(bookmarkXML *uint16) (handle EvtHandle, err error) {
	return EvtCreateBookmark(bookmarkXML)
}

func (r *windowsEventAPI) EvtFormatMessage(publisherMetadata EvtHandle, event EvtHandle, messageId uint32, valueCount uint32, values uintptr, flags EvtFormatMessageFlag, bufferSize uint32, buffer *byte, bufferUsed *uint32) error {
	return EvtFormatMessage(publisherMetadata, event, messageId, valueCount, values, flags, bufferSize, buffer, bufferUsed)
}

func (r *windowsEventAPI) EvtOpenPublisherMetadata(session EvtHandle, publisherId *uint16, logFilePath *uint16, locale uint32, flags uint32) (EvtHandle, error) {
	return EvtOpenPublisherMetadata(session, publisherId, logFilePath, locale, flags)
}
