// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/service/ec2"
)

type EbsVolume struct {
	// device name to volumeId mapping
	dev2Vol map[string]string
	sync.RWMutex
}

func NewEbsVolume() *EbsVolume {
	return &EbsVolume{dev2Vol: make(map[string]string)}
}

func (e *EbsVolume) addEbsVolumeMapping(attachment *ec2.VolumeAttachment) {
	// *attachment.Device is sth like: /dev/xvda
	devPath := findNvmeBlockNameIfPresent(*attachment.Device)
	if devPath == "" {
		devPath = *attachment.Device
	}

	// to match the disk device tag
	devPath = strings.ReplaceAll(devPath, "/dev/", "")

	e.Lock()
	defer e.Unlock()
	e.dev2Vol[devPath] = *attachment.VolumeId
}

// find nvme block name by symlink, if symlink doesn't exist, return ""
func findNvmeBlockNameIfPresent(devName string) string {
	// for nvme(ssd), there is a symlink from devName to nvme block name, i.e. /dev/xvda -> /dev/nvme0n1
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/nvme-ebs-volumes.html
	hasRootFs := true
	if _, err := os.Lstat("/rootfs/proc"); os.IsNotExist(err) {
		hasRootFs = false
	}
	nvmeName := ""

	if hasRootFs {
		devName = "/rootfs" + devName
	}

	if info, err := os.Lstat(devName); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if path, err := filepath.EvalSymlinks(devName); err == nil {
				nvmeName = path
			}
		}
	}

	if nvmeName != "" && hasRootFs {
		nvmeName = strings.TrimPrefix(nvmeName, "/rootfs")
	}
	return nvmeName
}

func (e *EbsVolume) getEbsVolumeId(devName string) string {
	e.RLock()
	defer e.RUnlock()

	for k, v := range e.dev2Vol {
		// The key of dev2Vol is device name like nvme0n1, while the input devName could be a partition name like nvme0n1p1
		if strings.HasPrefix(devName, k) {
			return v
		}
	}

	return ""
}
