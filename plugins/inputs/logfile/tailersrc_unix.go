//go:build linux || darwin || freebsd || netbsd || openbsd
// +build linux darwin freebsd netbsd openbsd

package logfile

import (
	"os"
	"path/filepath"
	"syscall"
)

type inodeInfo struct {
	Inode uint64
	Dev   uint64
}

// getInodeInfo extracts inode and device from FileInfo
func getInodeInfo(fi os.FileInfo) *inodeInfo {
	if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
		return &inodeInfo{
			Inode: stat.Ino,
			Dev:   uint64(stat.Dev),
		}
	}
	return nil
}

// findFileByInode searches for a file with the given inode in the same directory
func findFileByInode(originalPath string, inode, dev uint64) string {
	dir := filepath.Dir(originalPath)

	// Check files in the same directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		if fi, err := os.Stat(fullPath); err == nil {
			if info := getInodeInfo(fi); info != nil {
				if info.Inode == inode && info.Dev == dev {
					return fullPath
				}
			}
		}
	}

	return ""
}
