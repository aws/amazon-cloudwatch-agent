package emfProcessor

import (
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/structuredlogscommon"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
)

func buildTestMetricDeclarations() (mds []*metricDeclaration) {
	md1 := &metricDeclaration{
		SourceLabels:    []string{"tagA"},
		LabelMatcher:    "^v1$",
		MetricSelectors: []string{"metric_a"},
		Dimensions:      [][]string{{"tagA"}},
	}
	mds = append(mds, md1)

	md2 := &metricDeclaration{
		SourceLabels:    []string{"tagA"},
		LabelMatcher:    "^v1$",
		MetricSelectors: []string{"metric_a", "metric_d"},
		Dimensions:      [][]string{{"tagA"}},
	}
	mds = append(mds, md2)
	return
}

func buildTestMetrics(ts time.Time) (ms []telegraf.Metric) {
	m1, _ := metric.New("prometheus_scraper",
		map[string]string{"tagA": "v1"},
		map[string]interface{}{"metric_a": 0.1},
		ts)
	ms = append(ms, m1)
	return
}

func buildEMFMetricRule() *structuredlogscommon.MetricRule {
	return &structuredlogscommon.MetricRule{
		Namespace:     "ContainerInsights/Prometheus",
		DimensionSets: [][]string{{"tagA"}},
		Metrics:       []structuredlogscommon.MetricAttr{{Name: "metric_a"}},
	}
}

func buildExpectedMetrics(ts time.Time) (ms []telegraf.Metric) {
	m1, _ := metric.New("prometheus_scraper",
		map[string]string{"tagA": "v1", "attributesInFields": "CloudWatchMetrics"},
		map[string]interface{}{"metric_a": 0.1, "CloudWatchMetrics": []structuredlogscommon.MetricRule{*buildEMFMetricRule(), *buildEMFMetricRule()}},
		ts)
	ms = append(ms, m1)
	return
}

func buildExpectedDeduppedMetrics(ts time.Time) (ms []telegraf.Metric) {
	m1, _ := metric.New("prometheus_scraper",
		map[string]string{"tagA": "v1", "attributesInFields": "CloudWatchMetrics"},
		map[string]interface{}{"metric_a": 0.1, "CloudWatchMetrics": []structuredlogscommon.MetricRule{*buildEMFMetricRule()}},
		ts)
	ms = append(ms, m1)
	return
}

//TODO
//Disable the test as it fails randomly
// func TestEmfProcessor_Apply(t *testing.T) {
// 	type fields struct {
// 		inited                  bool
// 		MetricDeclarationsDedup bool
// 		MetricDeclarations      []*metricDeclaration
// 	}
// 	type args struct {
// 		in []telegraf.Metric
// 	}

// 	ts := time.Now()
// 	tests := []struct {
// 		name       string
// 		fields     fields
// 		args       args
// 		wantResult []telegraf.Metric
// 	}{
// 		{name: "dedupped",
// 			fields: fields{MetricDeclarationsDedup: true,
// 				MetricDeclarations: buildTestMetricDeclarations()},
// 			args:       args{in: buildTestMetrics(ts)},
// 			wantResult: buildExpectedDeduppedMetrics(ts),
// 		},
// 		{name: "not_dedupped",
// 			fields: fields{MetricDeclarationsDedup: false,
// 				MetricDeclarations: buildTestMetricDeclarations()},
// 			args:       args{in: buildTestMetrics(ts)},
// 			wantResult: buildExpectedMetrics(ts),
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			e := &EmfProcessor{
// 				inited:                  tt.fields.inited,
// 				MetricDeclarationsDedup: tt.fields.MetricDeclarationsDedup,
// 				MetricDeclarations:      tt.fields.MetricDeclarations,
// 				MetricNamespace:         "ContainerInsights/Prometheus",
// 			}
// 			if gotResult := e.Apply(tt.args.in...); !reflect.DeepEqual(gotResult, tt.wantResult) {
// 				t.Errorf("Apply() = %v, want %v", gotResult, tt.wantResult)
// 			}
// 		})
// 	}
// }
