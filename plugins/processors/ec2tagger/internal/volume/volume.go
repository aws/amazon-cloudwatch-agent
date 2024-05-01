// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package volume

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"golang.org/x/exp/maps"
)

var (
	errNoProviders = errors.New("no available volume info providers")
)

type Provider interface {
	// DeviceToSerialMap provides a map with device name keys and serial number values.
	DeviceToSerialMap() (map[string]string, error)
}

func NewProvider(ec2Client ec2iface.EC2API, instanceID string) Provider {
	return newMergeProvider([]Provider{
		NewHostProvider(),
		NewDescribeVolumesProvider(ec2Client, instanceID),
	})
}

type Cache struct {
	sync.RWMutex
	// device name to volumeId mapping
	cache    map[string]string
	provider Provider
}

func NewCache(provider Provider) *Cache {
	return &Cache{
		cache:    make(map[string]string),
		provider: provider,
	}
}

func (c *Cache) Add(devName, serial string) {
	// *attachment.Device is sth like: /dev/xvda
	devPath := findNvmeBlockNameIfPresent(devName)
	if devPath == "" {
		devPath = devName
	}

	// to match the disk device tag
	devPath = strings.ReplaceAll(devPath, "/dev/", "")

	c.Lock()
	defer c.Unlock()
	c.cache[devPath] = serial
}

func (c *Cache) Reset() {
	c.Lock()
	defer c.Unlock()
	maps.Clear(c.cache)
}

func (c *Cache) Refresh() error {
	if c.provider == nil {
		return errNoProviders
	}
	result, err := c.provider.DeviceToSerialMap()
	if err != nil {
		return fmt.Errorf("unable to refresh volume cache: %w", err)
	}
	c.Reset()
	for deviceName, serial := range result {
		c.Add(deviceName, serial)
	}
	return nil
}

func (c *Cache) Serial(devName string) string {
	c.RLock()
	defer c.RUnlock()

	// check exact match first
	if v, ok := c.cache[devName]; ok {
		return v
	}

	for k, v := range c.cache {
		// The key of cache is device name like nvme0n1, while the input devName could be a partition name like nvme0n1p1
		if strings.HasPrefix(devName, k) {
			return v
		}
	}
	return ""
}

func (c *Cache) Devices() []string {
	c.RLock()
	defer c.RUnlock()
	return maps.Keys(c.cache)
}

func (c *Cache) Map() map[string]string {
	return c.cache
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
