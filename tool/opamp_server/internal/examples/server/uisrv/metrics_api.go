package uisrv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/internal/examples/server/data"
)

func getAgentMetricsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	// Get parameters
	metricName := r.URL.Query().Get("metric_name")
	timeFilter := r.URL.Query().Get("time_filter")
	namespace := r.URL.Query().Get("namespace")
	
	if namespace == "" {
		namespace = "CWAgent"
	}
	
	// Calculate time range
	endTime := time.Now()
	var startTime time.Time
	
	if timeFilter != "" {
		var duration time.Duration
		if strings.HasSuffix(timeFilter, "m") {
			if minutes, err := strconv.Atoi(strings.TrimSuffix(timeFilter, "m")); err == nil {
				duration = time.Duration(minutes) * time.Minute
			}
		} else if strings.HasSuffix(timeFilter, "h") {
			if hours, err := strconv.Atoi(strings.TrimSuffix(timeFilter, "h")); err == nil {
				duration = time.Duration(hours) * time.Hour
			}
		}
		if duration > 0 {
			startTime = endTime.Add(-duration)
		}
	} else {
		startTime = endTime.Add(-1 * time.Hour)
	}
	
	// Get agent_id parameter if provided
	agentId := r.URL.Query().Get("agent_id")

	
	// Get real metrics from CloudWatch
	realData, err := tryGetRealMetrics(metricName, startTime, endTime, agentId)
	if err != nil {
		response := map[string]interface{}{
			"error": "Failed to fetch metrics: " + err.Error(),
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	
	response := map[string]interface{}{
		"metrics":     realData,
		"metric_name": metricName,
		"namespace":   namespace,
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
	}
	
	json.NewEncoder(w).Encode(response)
}

func tryGetRealMetrics(metricName string, startTime, endTime time.Time, agentId string) ([]map[string]interface{}, error) {
	// Create AWS session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-2"),
	})
	if err != nil {
		return nil, err
	}
	
	svc := cloudwatch.New(sess)
	
	// Get instance ID from specific agent or any connected agent
	agents := data.AllAgents.GetAllAgentsReadonlyClone()
	var instanceId string
	
	if agentId != "" {
		// Look for specific agent by UUID
		for agentUUID, agent := range agents {
			uuidStr := uuid.UUID(agentUUID).String()
			if uuidStr == agentId {
				if effectiveConfig := agent.EffectiveConfig; effectiveConfig != "" {
					lines := strings.Split(effectiveConfig, "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if strings.Contains(line, "log_stream_name") && strings.Contains(line, "i-") {
							start := strings.Index(line, "i-")
							if start != -1 {
								end := start + 19
								if end <= len(line) {
									instanceId = line[start:end]
									break
								}
							}
						}
					}
				}
				break
			}
		}
	} else {
		// Fallback to any connected agent

		for _, agent := range agents {
			if effectiveConfig := agent.EffectiveConfig; effectiveConfig != "" {
				lines := strings.Split(effectiveConfig, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.Contains(line, "log_stream_name") && strings.Contains(line, "i-") {
						start := strings.Index(line, "i-")
						if start != -1 {
							end := start + 19
							if end <= len(line) {
								instanceId = line[start:end]
								break
							}
						}
					}
				}
				if instanceId != "" {
					break
				}
			}
		}
	}
	
	// Get EC2 metadata dynamically
	var dimensions []*cloudwatch.Dimension
	if instanceId != "" {
		// Get ImageId and InstanceType dynamically from EC2 API
		ec2Svc := ec2.New(sess)
		input := &ec2.DescribeInstancesInput{
			InstanceIds: []*string{aws.String(instanceId)},
		}
		
		result, err := ec2Svc.DescribeInstances(input)
		if err != nil {
			return nil, err
		}
		if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
			return nil, fmt.Errorf("instance %s not found", instanceId)
		}
		
		instance := result.Reservations[0].Instances[0]
		dimensions = append(dimensions,
			&cloudwatch.Dimension{Name: aws.String("InstanceId"), Value: aws.String(instanceId)},
			&cloudwatch.Dimension{Name: aws.String("ImageId"), Value: instance.ImageId},
			&cloudwatch.Dimension{Name: aws.String("InstanceType"), Value: instance.InstanceType},
		)
	}
	
	// Query CloudWatch
	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String("CWAgent"),
		MetricName: aws.String(metricName),
		StartTime:  aws.Time(startTime),
		EndTime:    aws.Time(endTime),
		Period:     aws.Int64(300),
		Statistics: []*string{aws.String("Average")},
		Dimensions: dimensions,
	}
	
	result, err := svc.GetMetricStatistics(input)
	if err != nil {
		return nil, err
	}
	
	var metrics []map[string]interface{}
	for _, dp := range result.Datapoints {
		metrics = append(metrics, map[string]interface{}{
			"Timestamp": dp.Timestamp.Format(time.RFC3339),
			"Average":   *dp.Average,
		})
	}
	
	return metrics, nil
}

func getAvailableMetricsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	// Get metrics from connected agents' configurations
	agents := data.AllAgents.GetAllAgentsReadonlyClone()
	metricsFromAgents := make(map[string]bool)
	
	for _, agent := range agents {
		if effectiveConfig := agent.EffectiveConfig; effectiveConfig != "" {
			config := strings.ToLower(effectiveConfig)
			
			// Parse based on what receivers are actually configured
			if strings.Contains(config, "telegraf_cpu") {
				metricsFromAgents["cpu_usage_idle"] = true
				metricsFromAgents["cpu_usage_user"] = true
				metricsFromAgents["cpu_usage_system"] = true
				metricsFromAgents["cpu_usage_iowait"] = true
				metricsFromAgents["cpu_usage_nice"] = true
				metricsFromAgents["cpu_usage_softirq"] = true
				metricsFromAgents["cpu_usage_steal"] = true
				metricsFromAgents["cpu_usage_guest"] = true
				metricsFromAgents["cpu_usage_guest_nice"] = true
			}
			if strings.Contains(config, "telegraf_mem") {
				metricsFromAgents["mem_used_percent"] = true
			}
			if strings.Contains(config, "telegraf_disk") {
				metricsFromAgents["disk_used_percent"] = true
				metricsFromAgents["disk_inodes_free"] = true
			}
			if strings.Contains(config, "telegraf_diskio") {
				metricsFromAgents["diskio_reads"] = true
				metricsFromAgents["diskio_writes"] = true
				metricsFromAgents["diskio_read_bytes"] = true
				metricsFromAgents["diskio_write_bytes"] = true
				metricsFromAgents["diskio_io_time"] = true
			}
			if strings.Contains(config, "telegraf_netstat") {
				metricsFromAgents["netstat_tcp_established"] = true
				metricsFromAgents["netstat_tcp_time_wait"] = true
			}
			if strings.Contains(config, "telegraf_swap") {
				metricsFromAgents["swap_used_percent"] = true
			}
			if strings.Contains(config, "telegraf_net") {
				metricsFromAgents["net_bytes_sent"] = true
				metricsFromAgents["net_bytes_recv"] = true
				metricsFromAgents["net_packets_sent"] = true
				metricsFromAgents["net_packets_recv"] = true
			}
			if strings.Contains(config, "diskio") {
				metricsFromAgents["diskio_reads"] = true
				metricsFromAgents["diskio_writes"] = true
				metricsFromAgents["diskio_read_bytes"] = true
				metricsFromAgents["diskio_write_bytes"] = true
				metricsFromAgents["diskio_read_time"] = true
				metricsFromAgents["diskio_write_time"] = true
				metricsFromAgents["diskio_io_time"] = true
				metricsFromAgents["diskio_weighted_io_time"] = true
				metricsFromAgents["diskio_iops_in_progress"] = true
			}
			if strings.Contains(config, "net") && !strings.Contains(config, "netstat") {
				metricsFromAgents["net_bytes_sent"] = true
				metricsFromAgents["net_bytes_recv"] = true
				metricsFromAgents["net_packets_sent"] = true
				metricsFromAgents["net_packets_recv"] = true
				metricsFromAgents["net_err_in"] = true
				metricsFromAgents["net_err_out"] = true
				metricsFromAgents["net_drop_in"] = true
				metricsFromAgents["net_drop_out"] = true
			}
			if strings.Contains(config, "netstat") {
				metricsFromAgents["netstat_tcp_established"] = true
				metricsFromAgents["netstat_tcp_time_wait"] = true
				metricsFromAgents["netstat_tcp_close"] = true
				metricsFromAgents["netstat_tcp_close_wait"] = true
				metricsFromAgents["netstat_tcp_closing"] = true
				metricsFromAgents["netstat_tcp_fin_wait1"] = true
				metricsFromAgents["netstat_tcp_fin_wait2"] = true
				metricsFromAgents["netstat_tcp_last_ack"] = true
				metricsFromAgents["netstat_tcp_listen"] = true
				metricsFromAgents["netstat_tcp_syn_sent"] = true
				metricsFromAgents["netstat_tcp_syn_recv"] = true
				metricsFromAgents["netstat_udp_socket"] = true
			}
			if strings.Contains(config, "swap") {
				metricsFromAgents["swap_used_percent"] = true
				metricsFromAgents["swap_used"] = true
				metricsFromAgents["swap_free"] = true
				metricsFromAgents["swap_total"] = true
			}
			if strings.Contains(config, "processes") {
				metricsFromAgents["processes_running"] = true
				metricsFromAgents["processes_sleeping"] = true
				metricsFromAgents["processes_stopped"] = true
				metricsFromAgents["processes_total"] = true
				metricsFromAgents["processes_zombie"] = true
				metricsFromAgents["processes_blocked"] = true
				metricsFromAgents["processes_idle"] = true
				metricsFromAgents["processes_wait"] = true
			}
		}
	}
	
	var metrics []string
	for metric := range metricsFromAgents {
		metrics = append(metrics, metric)
	}
	
	// Fallback if no agents connected
	if len(metrics) == 0 {
		metrics = []string{"mem_used_percent", "disk_used_percent", "swap_used_percent"}
	}
	
	response := map[string]interface{}{
		"metrics": metrics,
	}
	
	json.NewEncoder(w).Encode(response)
}