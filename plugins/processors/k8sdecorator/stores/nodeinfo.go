package stores

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/mapWithExpiry"
)

type nodeStats struct {
	podCnt       int
	containerCnt int
	cpuReq       int64
	memReq       int64
}

type nodeInfo struct {
	nodeStats nodeStats
	// ebsIds for persistent volume of pod
	ebsIds *mapWithExpiry.MapWithExpiry
	// mutex for ebsIds
	sync.RWMutex
	*NodeCapacity
}

func (n *nodeInfo) refreshEbsId() {
	// rootfs is mounted with the root dir on host
	file, err := os.Open("/rootfs/proc/mounts")
	if err != nil {
		log.Printf("D! cannot open /rootfs/proc/mounts %v", err)
		return
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	n.Lock()
	defer n.Unlock()
	n.extractEbsId(reader)
}

func (n *nodeInfo) extractEbsId(reader *bufio.Reader) {
	ebsMountPointRegex := regexp.MustCompile(`kubernetes.io/aws-ebs/mounts/aws/(.+)/(vol-\w+)`)

	for {
		line, isPrefix, err := reader.ReadLine()

		// err could be EOF in normal case
		if err != nil {
			break
		}

		// isPrefix is set when a line exceeding 4KB which we treat it as error when reading mount file
		if isPrefix {
			break
		}

		lineStr := string(line)
		if strings.TrimSpace(lineStr) == "" {
			continue
		}

		//example line: /dev/nvme1n1 /var/lib/kubelet/plugins/kubernetes.io/aws-ebs/mounts/aws/us-west-2b/vol-0d9f0816149eb2050 ext4 rw,relatime,data=ordered 0 0
		keys := strings.Split(lineStr, " ")
		if len(keys) < 2 {
			continue
		}
		matches := ebsMountPointRegex.FindStringSubmatch(keys[1])
		if len(matches) > 0 {
			// Set {"/dev/nvme1n1": "aws://us-west-2b/vol-0d9f0816149eb2050"}
			n.ebsIds.Set(keys[0], fmt.Sprintf("aws://%s/%s", matches[1], matches[2]))
		}
	}
}

func (n *nodeInfo) getEbsVolumeId(devName string) string {
	n.RLock()
	defer n.RUnlock()
	if volId, ok := n.ebsIds.Get(devName); ok {
		return volId.(string)
	}
	return ""
}

func (n *nodeInfo) cleanUp(now time.Time) {
	n.ebsIds.CleanUp(now)
}

func newNodeInfo() *nodeInfo {
	nc := &nodeInfo{ebsIds: mapWithExpiry.NewMapWithExpiry(2 * refreshInterval), NodeCapacity: NewNodeCapacity()}
	return nc
}

func (n *nodeInfo) getCPUCapacity() int64 {
	return n.CPUCapacity * 1000
}

func (n *nodeInfo) getMemCapacity() int64 {
	return n.MemCapacity
}
