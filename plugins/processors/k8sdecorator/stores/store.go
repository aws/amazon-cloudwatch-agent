package stores

import (
	"github.com/influxdata/telegraf"
)

type K8sStore interface {
	Decorate(metric telegraf.Metric, kubernetesBlob map[string]interface{}) bool
	RefreshTick()
}
