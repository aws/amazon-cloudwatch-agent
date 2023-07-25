// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package security

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CheckFileRights check that the given filename has access controls and system permission for Administrator, Local System
func CheckFileRights(filePath string) error {
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("Cannot get file's stat %s: %v", filePath, err)
	}

	var fileDacl *Acl
	err := GetNamedSecurityInfo(filePath,
		SE_FILE_OBJECT,
		DACL_SECURITY_INFORMATION,
		nil,
		nil,
		&fileDacl,
		nil,
		nil)

	if err != nil {
		return fmt.Errorf("Cannot get file security info %s: %s", filePath, err)
	}

	var aclSizeInfo AclSizeInformation
	err = GetAclInformation(fileDacl, &aclSizeInfo, AclSizeInformationEnum)
	if err != nil {
		return fmt.Errorf("Cannot query file's ACLs %s: %s", filePath, err)
	}

	// create the sids that are acceptable to us (local system account and administrators group)
	// For more information on account type: https://stackoverflow.com/a/510225
	var localSystem *windows.SID

	err = windows.AllocateAndInitializeSid(&windows.SECURITY_NT_AUTHORITY,
		1, // local system has 1 valid subauth
		windows.SECURITY_LOCAL_SYSTEM_RID,
		0, 0, 0, 0, 0, 0, 0,
		&localSystem)

	if err != nil {
		return fmt.Errorf("Cannot initialize Local System SID: %v", err)
	}

	defer windows.FreeSid(localSystem)

	var administrators *windows.SID

	err = windows.AllocateAndInitializeSid(&windows.SECURITY_NT_AUTHORITY,
		2, // administrators group has 2 valid subauths
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&administrators)

	if err != nil {
		return fmt.Errorf("Cannot initialize  Administrator SID: %s", err)
	}

	defer windows.FreeSid(administrators)

	hasFileAllAccessLocalSystem := false
	hasFileAllAccessAdministrators := false

	for i := uint32(0); i < aclSizeInfo.AceCount; i++ {
		var pAce *AccessAllowedAce
		if err := GetAce(fileDacl, i, &pAce); err != nil {
			return fmt.Errorf("Could not query a ACE on %s with: %s", filePath, err)
		}

		compareSid := (*windows.SID)(unsafe.Pointer(&pAce.SidStart))
		compareIsLocalSystem := windows.EqualSid(compareSid, localSystem)
		compareIsAdministrators := windows.EqualSid(compareSid, administrators)

		if pAce.AceType == ACCESS_DENIED_ACE_TYPE {
			// if the file has denied access to local system or administrators, then it cannot be protected by those accounts
			if compareIsLocalSystem || compareIsAdministrators {
				return fmt.Errorf("File %s has deny access for Administrators and Local System", filePath)
			}
		}

		if pAce.AccessMask == FILE_ALL_ACCESS {
			if compareIsLocalSystem {
				hasFileAllAccessLocalSystem = true
			}
			if compareIsAdministrators {
				hasFileAllAccessAdministrators = true
			}
		}

	}

	if !hasFileAllAccessLocalSystem || !hasFileAllAccessAdministrators {
		return fmt.Errorf("No highest file access for Administrators and Local System with %s", filePath)
	}

	return nil
}
