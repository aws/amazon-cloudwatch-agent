package uisrv

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"time"
)

func getAgentTracesAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	// Get parameters
	timeFilter := r.URL.Query().Get("time_filter")
	agentId := r.URL.Query().Get("agent_id")
	
	// Calculate time range
	endTime := time.Now()
	var startTime time.Time
	
	if timeFilter != "" {
		switch timeFilter {
		case "1h":
			startTime = endTime.Add(-1 * time.Hour)
		case "6h":
			startTime = endTime.Add(-6 * time.Hour)
		case "12h":
			startTime = endTime.Add(-12 * time.Hour)
		case "24h":
			startTime = endTime.Add(-24 * time.Hour)
		default:
			startTime = endTime.Add(-1 * time.Hour)
		}
	} else {
		startTime = endTime.Add(-1 * time.Hour)
	}
	
	// Get traces from X-Ray using AWS CLI
	cmd := exec.Command("aws", "xray", "get-trace-summaries",
		"--start-time", startTime.Format(time.RFC3339),
		"--end-time", endTime.Format(time.RFC3339),
		"--region", "us-east-2",
		"--output", "json")
	
	output, err := cmd.Output()
	if err != nil {
		response := map[string]interface{}{
			"error": "Failed to fetch traces: " + err.Error(),
			"traces": []interface{}{},
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	
	var awsResponse struct {
		TraceSummaries []map[string]interface{} `json:"TraceSummaries"`
	}
	
	if err := json.Unmarshal(output, &awsResponse); err != nil {
		response := map[string]interface{}{
			"error": "Failed to parse X-Ray response: " + err.Error(),
			"traces": []interface{}{},
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Add agent context to traces if agent_id provided
	traces := awsResponse.TraceSummaries
	if agentId != "" {
		// Add agent info to each trace
		for i := range traces {
			traces[i]["AgentId"] = agentId
		}
	}
	
	response := map[string]interface{}{
		"traces": traces,
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		"count": len(traces),
		"agent_id": agentId,
	}
	
	json.NewEncoder(w).Encode(response)
}

func getTraceDetailsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	traceId := r.URL.Query().Get("trace_id")
	if traceId == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "trace_id parameter required"})
		return
	}
	
	// Get trace details from X-Ray
	cmd := exec.Command("aws", "xray", "batch-get-traces",
		"--trace-ids", traceId,
		"--region", "us-east-2",
		"--output", "json")
	
	output, err := cmd.Output()
	if err != nil {
		response := map[string]interface{}{
			"error": "Failed to fetch trace details: " + err.Error(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	
	var awsResponse map[string]interface{}
	if err := json.Unmarshal(output, &awsResponse); err != nil {
		response := map[string]interface{}{
			"error": "Failed to parse trace details: " + err.Error(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	
	json.NewEncoder(w).Encode(awsResponse)
}