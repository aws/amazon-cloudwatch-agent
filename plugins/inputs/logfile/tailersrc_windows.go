//go:build windows
// +build windows

package logfile

import "os"

type inodeInfo struct {
	Inode uint64
	Dev   uint64
}

// getInodeInfo is not supported on Windows
func getInodeInfo(fi os.FileInfo) *inodeInfo {
	return nil
}

// findFileByInode is not supported on Windows
func findFileByInode(originalPath string, inode, dev uint64) string {
	return ""
}
