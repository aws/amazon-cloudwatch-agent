// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package security

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// https://docs.microsoft.com/en-us/windows/win32/api/accctrl/ne-accctrl-se_object_type
const (
	SE_UNKNOWN_OBJECT_TYPE = iota
	SE_FILE_OBJECT
)

// https://github.com/mhammond/pywin32/blob/70ddf693927fa1635f15e9ef41eb1aea37fdf32a/win32/Lib/ntsecuritycon.py
const (
	ACCESS_ALLOWED_ACE_TYPE = 0
	ACCESS_DENIED_ACE_TYPE  = 1
	FILE_ALL_ACCESS         = (windows.STANDARD_RIGHTS_ALL | 0x1FF)
)

const (
	AclSizeInformationEnum    = 2
	DACL_SECURITY_INFORMATION = 0x00004
)

var (
	advapi32                 = syscall.NewLazyDLL("advapi32.dll")
	procGetAclInformation    = advapi32.NewProc("GetAclInformation")
	procGetNamedSecurityInfo = advapi32.NewProc("GetNamedSecurityInfoW")
	procGetAce               = advapi32.NewProc("GetAce")
)

// https://docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-acl_size_information
type AclSizeInformation struct {
	AceCount      uint32
	AclBytesInUse uint32
	AclBytesFree  uint32
}

// https://docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-acl
type Acl struct {
	AclRevision uint8
	Sbz1        uint8
	AclSize     uint16
	AceCount    uint16
	Sbz2        uint16
}

// https://docs.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_access_allowed_ace
type AccessAllowedAce struct {
	AceType    uint8
	AceFlags   uint8
	AceSize    uint16
	AccessMask uint32
	SidStart   uint32
}

// Retrieve a copy of security descriptor for an object specified by name (e.g a file)
// For more information: https://docs.microsoft.com/en-us/windows/win32/api/aclapi/nf-aclapi-getnamedsecurityinfoa
func GetNamedSecurityInfo(objectName string, objectType int32, secInfo uint32, owner, group **windows.SID, dacl, sacl **Acl, secDesc *windows.Handle) error {
	ret, _, err := procGetNamedSecurityInfo.Call(
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(objectName))),
		uintptr(objectType),
		uintptr(secInfo),
		uintptr(unsafe.Pointer(owner)),
		uintptr(unsafe.Pointer(group)),
		uintptr(unsafe.Pointer(dacl)),
		uintptr(unsafe.Pointer(sacl)),
		uintptr(unsafe.Pointer(secDesc)),
	)
	if ret != 0 {
		return err
	}
	return nil
}

// Retrieve information about access control list  (e.g a file)
// For more information: https://docs.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-getaclinformation
func GetAclInformation(acl *Acl, info *AclSizeInformation, class uint32) error {
	length := unsafe.Sizeof(*info)
	ret, _, _ := procGetAclInformation.Call(
		uintptr(unsafe.Pointer(acl)),
		uintptr(unsafe.Pointer(info)),
		uintptr(length),
		uintptr(class))

	if int(ret) == 0 {
		return windows.GetLastError()
	}
	return nil
}

// Obtain a pointer to an access control entry (ACE) in an access control list (ACL).
// For more information: https://docs.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-getace
func GetAce(acl *Acl, index uint32, ace **AccessAllowedAce) error {
	ret, _, _ := procGetAce.Call(uintptr(unsafe.Pointer(acl)), uintptr(index), uintptr(unsafe.Pointer(ace)))
	if int(ret) != 0 {
		return windows.GetLastError()
	}
	return nil
}
