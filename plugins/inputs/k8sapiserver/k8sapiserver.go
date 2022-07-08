// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sapiserver

import (
	"context"
	"errors"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/k8sCommon/k8sclient"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

const (
	lockName = "cwagent-clusterleader"
)

type K8sAPIServer struct {
	NodeName string `toml:"node_name"`

	cancel  context.CancelFunc
	leading bool
}

var sampleConfig = `
`

func init() {
	inputs.Add("k8sapiserver", func() telegraf.Input {
		return &K8sAPIServer{}
	})
}

//SampleConfig returns a sample config
func (k *K8sAPIServer) SampleConfig() string {
	return sampleConfig
}

//Description returns the description of this plugin
func (k *K8sAPIServer) Description() string {
	return "Calculate cluster level metrics from the k8s api server"
}

func (k *K8sAPIServer) Gather(acc telegraf.Accumulator) error {
	if k.leading {
		log.Printf("D! collect data from K8s API Server...")
		timestamp := strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
		client := k8sclient.Get()
		acc.AddFields("k8sapiserver",
			map[string]interface{}{
				"cluster_failed_node_count": client.Node.ClusterFailedNodeCount(),
				"cluster_node_count":        client.Node.ClusterNodeCount(),
			},
			map[string]string{
				containerinsightscommon.MetricType: containerinsightscommon.TypeCluster,
				"Timestamp":                        timestamp,
			})
		for service, podNum := range client.Ep.ServiceToPodNum() {
			acc.AddFields("k8sapiserver",
				map[string]interface{}{
					"service_number_of_running_pods": podNum,
				},
				map[string]string{
					containerinsightscommon.MetricType:   containerinsightscommon.TypeClusterService,
					"Timestamp":                          timestamp,
					containerinsightscommon.TypeService:  service.ServiceName,
					containerinsightscommon.K8sNamespace: service.Namespace,
				})
		}
		log.Printf("I! number of namespace to running pod num %v", client.Pod.NamespaceToRunningPodNum())
		for namespace, podNum := range client.Pod.NamespaceToRunningPodNum() {
			acc.AddFields("k8sapiserver",
				map[string]interface{}{
					"namespace_number_of_running_pods": podNum,
				},
				map[string]string{
					containerinsightscommon.MetricType:   containerinsightscommon.TypeClusterNamespace,
					"Timestamp":                          timestamp,
					containerinsightscommon.K8sNamespace: namespace,
				})
		}
	}
	return nil
}

func (k *K8sAPIServer) Start(acc telegraf.Accumulator) error {
	ctx := context.Background()
	ctx, k.cancel = context.WithCancel(context.Background())

	lockNamespace := os.Getenv("K8S_NAMESPACE")
	if lockNamespace == "" {
		log.Printf("E! Missing environment variable K8S_NAMESPACE which is required to create lock. Please check your YAML config.")
		return errors.New("missing environment variable K8S_NAMESPACE")
	}

	configMapInterface := k8sclient.Get().ClientSet.CoreV1().ConfigMaps(lockNamespace)
	opts := metav1.CreateOptions{}
	if configMap, err := configMapInterface.Get(ctx, lockName, metav1.GetOptions{}); configMap == nil || err != nil {
		log.Printf("I! Cannot get the leader config map: %v, try to create the config map...", err)
		configMap, err = configMapInterface.Create(ctx, &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Namespace: lockNamespace,
			Name:      lockName,
		},
		},
			opts)
		log.Printf("I! configMap: %v, err: %v", configMap, err)
	}

	lock, err := resourcelock.New(
		resourcelock.ConfigMapsResourceLock,
		lockNamespace, lockName,
		k8sclient.Get().ClientSet.CoreV1(),
		k8sclient.Get().ClientSet.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      k.NodeName,
			EventRecorder: createRecorder(k8sclient.Get().ClientSet, lockName, lockNamespace),
		})
	if err != nil {
		log.Printf("E! Failed to create resource lock: %v", err)
		return err
	}

	go k.startLeaderElection(ctx, lock)

	return nil
}

func (k *K8sAPIServer) startLeaderElection(ctx context.Context, lock resourcelock.Interface) {

	for {
		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock: lock,
			// IMPORTANT: you MUST ensure that any code you have that
			// is protected by the lease must terminate **before**
			// you call cancel. Otherwise, you could have a background
			// loop still running and another process could
			// get elected before your background loop finished, violating
			// the stated goal of the lease.
			LeaseDuration: 60 * time.Second,
			RenewDeadline: 15 * time.Second,
			RetryPeriod:   5 * time.Second,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(ctx context.Context) {
					log.Printf("I! k8sapiserver OnStartedLeading: %s", k.NodeName)
					// we're notified when we start
					k.leading = true
				},
				OnStoppedLeading: func() {
					log.Printf("I! k8sapiserver OnStoppedLeading: %s", k.NodeName)
					// we can do cleanup here, or after the RunOrDie method returns
					k.leading = false
					//node and pod are only used for cluster level metrics, endpoint is used for decorator too.
					k8sclient.Get().Node.Shutdown()
					k8sclient.Get().Pod.Shutdown()
				},
				OnNewLeader: func(identity string) {
					log.Printf("I! k8sapiserver Switch New Leader: %s", identity)
				},
			},
		})

		select {
		case <-ctx.Done(): //when leader election ends, the channel ctx.Done() will be closed
			log.Printf("I! k8sapiserver shutdown Leader Election: %s", k.NodeName)
			return
		default:
		}
	}
}

func (k *K8sAPIServer) Stop() {
	if k.cancel != nil {
		k.cancel()
	}
}

func createRecorder(clientSet kubernetes.Interface, name, namespace string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(clientSet.CoreV1().RESTClient()).Events(namespace)})
	return eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: name})
}
