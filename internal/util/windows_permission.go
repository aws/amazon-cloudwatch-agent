package utils

import (
	"fmt"
	"os"
	"unsafe"
	"golang.org/x/sys/windows"
)

// checkFileRights check that the given filename has access controls for Administrator, Local System
func checkFileRights(filename string) error {
	if _, err := os.Stat(filename); err != nil {
		return fmt.Errorf("secretBackendCommand %s does not exist", filename)
	}

	var fileDacl *Acl
	err := GetNamedSecurityInfo(filename,
		SE_FILE_OBJECT,
		DACL_SECURITY_INFORMATION,
		nil,
		nil,
		&fileDacl,
		nil,
		nil)
	if err != nil {
		return fmt.Errorf("could not query ACLs for %s: %s", filename, err)
	}

	var aclSizeInfo AclSizeInformation
	err = GetAclInformation(fileDacl, &aclSizeInfo, AclSizeInformationEnum)
	if err != nil {
		return fmt.Errorf("could not query ACLs for %s: %s", filename, err)
	}

	// create the sids that are acceptable to us (local system account and
	// administrators group)
	var localSystem *windows.SID
	err = windows.AllocateAndInitializeSid(&windows.SECURITY_NT_AUTHORITY,
		1, // local system has 1 valid subauth
		windows.SECURITY_LOCAL_SYSTEM_RID,
		0, 0, 0, 0, 0, 0, 0,
		&localSystem)
	if err != nil {
		return fmt.Errorf("could not query Local System SID: %s", err)
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
		return fmt.Errorf("could not query Administrator SID: %s", err)
	}
	
	defer windows.FreeSid(administrators)
	
	hasGenericAllLocalSystem := false
	hasGenericAllAdministrators := false
	for i := uint32(0); i < aclSizeInfo.AceCount; i++ {
		var pAce *AccessAllowedAce
		if err := GetAce(fileDacl, i, &pAce); err != nil {
			return fmt.Errorf("Could not query a ACE on %s with: %s", filename, err)
		}

		compareSid := (*windows.SID)(unsafe.Pointer(&pAce.SidStart))
		compareIsLocalSystem := windows.EqualSid(compareSid, localSystem)
		compareIsAdministrators := windows.EqualSid(compareSid, administrators)
 
		if pAce.AceType == ACCESS_DENIED_ACE_TYPE {
			// if the file has denied access to local system or administrators, then it cannot be protected by those accounts
			if compareIsLocalSystem || compareIsAdministrators  {
				return fmt.Errorf("Invalid executable '%s': Can't deny access LOCAL_SYSTEM, Administrators", filename)
			}
		}

		if pAce.AccessMask == windows.GENERIC_ALL {
			if compareIsLocalSystem  {
				hasGenericAllLocalSystem = true
			}
			if compareIsLocalSystem  {
				hasGenericAllAdministrators = true
			}
		}
		
	}
	
	if !hasGenericAllLocalSystem || !hasGenericAllAdministrators{
		return fmt.Errorf("No execution for local administrators and local system with %s", filename)
	}
	
	return nil
}