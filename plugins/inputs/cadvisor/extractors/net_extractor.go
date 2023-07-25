// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extractors

import (
	"log"
	"time"

	cinfo "github.com/google/cadvisor/info/v1"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/mapWithExpiry"
)

const (
	oneTerabytes = 1 * 1024 * 1024 * 1024 * 1024
)

type NetMetricExtractor struct {
	preInfos *mapWithExpiry.MapWithExpiry
}

func (n *NetMetricExtractor) recordPreviousInfo(info *cinfo.ContainerInfo) {
	n.preInfos.Set(info.Name, info)
}

func getInterfacesStats(stats *cinfo.ContainerStats) []cinfo.InterfaceStats {
	ifceStats := stats.Network.Interfaces
	if len(ifceStats) == 0 {
		ifceStats = []cinfo.InterfaceStats{stats.Network.InterfaceStats}
	}
	return ifceStats
}

func (n *NetMetricExtractor) HasValue(info *cinfo.ContainerInfo) bool {
	return info.Spec.HasNetwork
}

func (n *NetMetricExtractor) GetValue(info *cinfo.ContainerInfo, containerType string) []*CAdvisorMetric {
	var metrics []*CAdvisorMetric

	// Ignore both pod and container because the network metrics comes from InfraContainer.
	if containerType == TypePod || containerType == TypeContainer {
		return metrics
	}
	// Rename type to pod so the metric name prefix is pod_
	if containerType == TypeInfraContainer {
		containerType = TypePod
	}

	if preInfo, ok := n.preInfos.Get(info.Name); ok {
		curStats := GetStats(info)
		preStats := GetStats(preInfo.(*cinfo.ContainerInfo))
		deltaCTimeInNano := curStats.Timestamp.Sub(preStats.Timestamp).Nanoseconds()
		if deltaCTimeInNano > MinTimeDiff {
			curIfceStats := getInterfacesStats(curStats)
			preIfceStats := getInterfacesStats(preStats)

			// used for aggregation
			var netIfceMetrics []map[string]float64

			for _, cur := range curIfceStats {
				for _, pre := range preIfceStats {
					if cur.Name == pre.Name {
						mType := getNetMetricType(containerType)
						netIfceMetric := make(map[string]float64)

						netIfceMetric[NetRxBytes] = float64(cur.RxBytes-pre.RxBytes) / float64(deltaCTimeInNano) * float64(time.Second)
						netIfceMetric[NetRxPackets] = float64(cur.RxPackets-pre.RxPackets) / float64(deltaCTimeInNano) * float64(time.Second)
						netIfceMetric[NetRxDropped] = float64(cur.RxDropped-pre.RxDropped) / float64(deltaCTimeInNano) * float64(time.Second)
						netIfceMetric[NetRxErrors] = float64(cur.RxErrors-pre.RxErrors) / float64(deltaCTimeInNano) * float64(time.Second)
						netIfceMetric[NetTxBytes] = float64(cur.TxBytes-pre.TxBytes) / float64(deltaCTimeInNano) * float64(time.Second)
						netIfceMetric[NetTxPackets] = float64(cur.TxPackets-pre.TxPackets) / float64(deltaCTimeInNano) * float64(time.Second)
						netIfceMetric[NetTxDropped] = float64(cur.TxDropped-pre.TxDropped) / float64(deltaCTimeInNano) * float64(time.Second)
						netIfceMetric[NetTxErrors] = float64(cur.TxErrors-pre.TxErrors) / float64(deltaCTimeInNano) * float64(time.Second)
						netIfceMetric[NetTotalBytes] = netIfceMetric[NetRxBytes] + netIfceMetric[NetTxBytes]
						if netIfceMetric[NetRxBytes] > oneTerabytes || netIfceMetric[NetTxBytes] > oneTerabytes {
							log.Printf("I! Too Big value for network RX/TX bytes, final Rx:%v, final Tx:%v, curRx:%v, preRx:%v, curTx:%v, preTx:%v, deltaCTimeInNano:%v",
								netIfceMetric[NetRxBytes], netIfceMetric[NetTxBytes],
								cur.RxBytes, pre.RxBytes,
								cur.TxBytes, pre.TxBytes,
								deltaCTimeInNano)
						}

						netIfceMetrics = append(netIfceMetrics, netIfceMetric)

						metric := newCadvisorMetric(mType)
						metric.tags[NetIfce] = cur.Name
						for k, v := range netIfceMetric {
							metric.fields[MetricName(mType, k)] = v
						}

						metric.cgroupPath = info.Name
						metrics = append(metrics, metric)
						break
					}
				}
			}
			aggregatedFields := aggregate(netIfceMetrics)
			if len(aggregatedFields) > 0 {
				metric := newCadvisorMetric(containerType)
				for k, v := range aggregatedFields {
					metric.fields[MetricName(containerType, k)] = v
				}
				metric.cgroupPath = info.Name
				metrics = append(metrics, metric)
			}
		}
	}
	n.recordPreviousInfo(info)

	return metrics
}

func (n *NetMetricExtractor) CleanUp(now time.Time) {
	n.preInfos.CleanUp(now)
}

func NewNetMetricExtractor() *NetMetricExtractor {
	return &NetMetricExtractor{
		preInfos: mapWithExpiry.NewMapWithExpiry(CleanInterval),
	}
}

func getNetMetricType(containerType string) string {
	metricType := ""
	switch containerType {
	case TypeNode:
		metricType = TypeNodeNet
	case TypeInstance:
		metricType = TypeInstanceNet
	case TypePod:
		metricType = TypePodNet
	default:
		log.Printf("W! net_extractor: net metric extractor is parsing unexpected containerType %s", containerType)
	}
	return metricType
}
