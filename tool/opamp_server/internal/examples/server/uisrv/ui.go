package uisrv

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"os/exec"
	"path"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"

	"github.com/open-telemetry/opamp-go/internal"
	"github.com/open-telemetry/opamp-go/internal/examples/server/data"
	"github.com/open-telemetry/opamp-go/protobufs"
)

var (
	htmlDir string
	srv     *http.Server
	opampCA = sync.OnceValue(func() string {
		p, err := os.ReadFile("../../certs/certs/ca.cert.pem")
		if err != nil {
			panic(err)
		}
		return string(p)
	})
)

var logger = log.New(log.Default().Writer(), "[UI] ", log.Default().Flags()|log.Lmsgprefix|log.Lmicroseconds)

func Start(rootDir string) {
	htmlDir = path.Join(rootDir, "uisrv/html")

	mux := http.NewServeMux()
	mux.HandleFunc("/", renderRoot)
	mux.HandleFunc("/agent", renderAgent)
	mux.HandleFunc("/save_config", saveCustomConfigForInstance)
	mux.HandleFunc("/rotate_client_cert", rotateInstanceClientCert)
	mux.HandleFunc("/opamp_connection_settings", opampConnectionSettings)
	// API endpoints
	mux.HandleFunc("/api/agent/start", startAgentAPI)
	mux.HandleFunc("/api/agent/stop", stopAgentAPI)
	mux.HandleFunc("/api/agent/status", getAgentStatusAPI)
	mux.HandleFunc("/api/agent/logs", getAgentLogsAPI)
	mux.HandleFunc("/api/agent/streams", getLogStreamsAPI)
	mux.HandleFunc("/api/agent/metrics", getAgentMetricsAPI)
	mux.HandleFunc("/api/agent/available-metrics", getAvailableMetricsAPI)
	mux.HandleFunc("/api/agent/traces", getAgentTracesAPI)
	mux.HandleFunc("/api/agent/trace-details", getTraceDetailsAPI)
	mux.HandleFunc("/api/test-metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		args := []string{"cloudwatch", "get-metric-statistics", "--namespace", "CWAgent", "--metric-name", "mem_used_percent", "--start-time", time.Now().Add(-1*time.Hour).Format(time.RFC3339), "--end-time", time.Now().Format(time.RFC3339), "--period", "300", "--statistics", "Average", "--region", "us-east-2", "--dimensions", "Name=InstanceId,Value=i-096f7d43c15f79fb4", "Name=ImageId,Value=ami-0ae9f87d24d606be4", "Name=InstanceType,Value=t2.small", "--output", "json"}
		cmd := exec.Command("aws", args...)
		output, err := cmd.Output()
		response := map[string]interface{}{"command": args, "output": string(output), "error": ""}
		if err != nil { response["error"] = err.Error() }
		json.NewEncoder(w).Encode(response)
	})
	mux.HandleFunc("/api/debug/files", debugFilesAPI)
	mux.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		agents := data.AllAgents.GetAllAgentsReadonlyClone()
		response := make(map[string]interface{})
		for _, agent := range agents {
			healthy := false
			lastError := ""
			attributes := make(map[string]string)
			if agent.Status != nil {
				if agent.Status.Health != nil {
					healthy = agent.Status.Health.Healthy
					lastError = agent.Status.Health.LastError
				}
				if agent.Status.AgentDescription != nil {
					for _, attr := range agent.Status.AgentDescription.IdentifyingAttributes {
						if attr.Value != nil {
							attributes[attr.Key] = attr.Value.GetStringValue()
						}
					}
					for _, attr := range agent.Status.AgentDescription.NonIdentifyingAttributes {
						if attr.Value != nil {
							attributes[attr.Key] = attr.Value.GetStringValue()
						}
					}
				}
			}
			startedAt := ""
			if !agent.StartedAt.IsZero() {
				startedAt = agent.StartedAt.Format("2006-01-02 15:04:05")
			}
			response[agent.InstanceIdStr] = map[string]interface{}{
				"InstanceIdStr": agent.InstanceIdStr,
				"Status": map[string]interface{}{
					"Health": map[string]interface{}{
						"Healthy": healthy,
						"LastError": lastError,
					},
				},
				"StartedAt": startedAt,
				"CustomInstanceConfig": agent.CustomInstanceConfig,
				"EffectiveConfig": agent.EffectiveConfig,
				"Attributes": attributes,
				"Components": extractComponents(agent),
				"Pipelines": extractPipelines(agent),
			}
		}
		json.NewEncoder(w).Encode(response)
	})
	srv = &http.Server{
		Addr:    "0.0.0.0:4321",
		Handler: mux,
	}
	go srv.ListenAndServe()
}

func Shutdown() {
	srv.Shutdown(context.Background())
}

func renderTemplate(w http.ResponseWriter, htmlTemplateFile string, data interface{}) {
	t, err := template.ParseFiles(
		path.Join(htmlDir, "header.html"),
		path.Join(htmlDir, htmlTemplateFile),
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Printf("Error parsing html template %s: %v", htmlTemplateFile, err)
		return
	}

	err = t.Lookup(htmlTemplateFile).Execute(w, data)
	if err != nil {
		// It is too late to send an HTTP status code since content is already written.
		// We can just log the error.
		logger.Printf("Error writing html content %s: %v", htmlTemplateFile, err)
		return
	}
}

func renderRoot(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "root.html", data.AllAgents.GetAllAgentsReadonlyClone())
}

func renderAgent(w http.ResponseWriter, r *http.Request) {
	uid, err := uuid.Parse(r.URL.Query().Get("instanceid"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	agent := data.AllAgents.GetAgentReadonlyClone(data.InstanceId(uid))
	if agent == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	renderTemplate(w, "agent.html", agent)
}

func saveCustomConfigForInstance(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	uid, err := uuid.Parse(r.Form.Get("instanceid"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	instanceId := data.InstanceId(uid)
	agent := data.AllAgents.GetAgentReadonlyClone(instanceId)
	if agent == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	configStr := r.PostForm.Get("config")
	config := &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigFile{
			"": {Body: []byte(configStr)},
		},
	}

	notifyNextStatusUpdate := make(chan struct{}, 1)
	data.AllAgents.SetCustomConfigForAgent(instanceId, config, notifyNextStatusUpdate)

	// Wait for up to 5 seconds for a Status update, which is expected
	// to be reported by the Agent after we set the remote config.
	timer := time.NewTicker(time.Second * 5)

	select {
	case <-notifyNextStatusUpdate:
	case <-timer.C:
	}

	http.Redirect(w, r, "/agent?instanceid="+uid.String(), http.StatusSeeOther)
}

func rotateInstanceClientCert(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Find the agent instance.
	uid, err := uuid.Parse(r.Form.Get("instanceid"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	instanceId := data.InstanceId(uid)
	agent := data.AllAgents.GetAgentReadonlyClone(instanceId)
	if agent == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Create a new certificate for the agent.
	certificate, err := internal.CreateTLSCert("../../certs/certs/ca.cert.pem", "../../certs/private/ca.key.pem")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Println(err)
		return
	}

	// Create an offer for the agent.
	offers := &protobufs.ConnectionSettingsOffers{
		Opamp: &protobufs.OpAMPConnectionSettings{
			Certificate: certificate,
		},
	}

	// Send the offer to the agent.
	data.AllAgents.OfferAgentConnectionSettings(instanceId, offers)

	logger.Printf("Waiting for agent %s to reconnect\n", instanceId)

	// Wait for up to 5 seconds for a Status update, which is expected
	// to be reported by the agent after we set the remote config.
	timer := time.NewTicker(time.Second * 5)

	// TODO: wait for agent to reconnect instead of waiting full 5 seconds.

	select {
	case <-timer.C:
		logger.Printf("Time out waiting for agent %s to reconnect\n", instanceId)
	}

	http.Redirect(w, r, "/agent?instanceid="+uid.String(), http.StatusSeeOther)
}

func opampConnectionSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Find the agent instance.
	uid, err := uuid.Parse(r.Form.Get("instanceid"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	instanceId := data.InstanceId(uid)
	agent := data.AllAgents.GetAgentReadonlyClone(instanceId)
	if agent == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// parse tls_min
	tlsMinVal := r.Form.Get("tls_min")
	var tlsMin string
	switch tlsMinVal {
	case "TLSv1.0":
		tlsMin = "1.0"
	case "TLSv1.1":
		tlsMin = "1.1"
	case "TLSv1.2":
		tlsMin = "1.2"
	case "TLSv1.3":
		tlsMin = "1.3"
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	offers := &protobufs.ConnectionSettingsOffers{
		Opamp: &protobufs.OpAMPConnectionSettings{
			Tls: &protobufs.TLSConnectionSettings{
				CaPemContents: opampCA(),
				MinVersion:    tlsMin,
				MaxVersion:    "1.3",
			},
		},
	}

	data.AllAgents.OfferAgentConnectionSettings(instanceId, offers)

	logger.Printf("Waiting for agent %s to reconnect\n", instanceId)

	// Wait for up to 5 seconds for a Status update, which is expected
	// to be reported by the agent after we set the remote config.
	timer := time.NewTicker(time.Second * 5)

	// TODO: wait for agent to reconnect instead of waiting full 5 seconds.

	select {
	case <-timer.C:
		logger.Printf("Time out waiting for agent %s to reconnect\n", instanceId)
	}

	http.Redirect(w, r, "/agent?instanceid="+uid.String(), http.StatusSeeOther)
}

func getAgentLogsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	// Get parameters
	logStream := r.URL.Query().Get("stream")
	agentId := r.URL.Query().Get("agent_id")
	
	
	// Get log group and stream from agent config or use defaults
	logGroup := "Opamp_supervisor_log"
	agentLogStream := ""
	
	// If agent_id provided, try to get log config from agent
	if agentId != "" {
		uid, err := uuid.Parse(agentId)
		if err == nil {
			instanceId := data.InstanceId(uid)
			agent := data.AllAgents.GetAgentReadonlyClone(instanceId)
			if agent != nil {
				// Try to extract log group/stream from agent's effective config
				if effectiveConfig := agent.EffectiveConfig; effectiveConfig != "" {
					lines := strings.Split(effectiveConfig, "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if strings.Contains(line, "log_group_name:") {
							parts := strings.SplitN(line, ":", 2)
							if len(parts) > 1 {
								group := strings.Trim(strings.TrimSpace(parts[1]), `" `)
								if group != "" {
									logGroup = group

								}
							}
						}
						if strings.Contains(line, "log_stream_name:") {
							parts := strings.SplitN(line, ":", 2)
							if len(parts) > 1 {
								stream := strings.Trim(strings.TrimSpace(parts[1]), `" `)
								if stream != "" {
									agentLogStream = stream

								}
							}
						}
					}
				}
			}
		}
	}
	
	// Use provided stream parameter, or fall back to agent's stream
	if logStream == "" {
		logStream = agentLogStream
	}
	

	
	// Read logs from CloudWatch Logs
	cmd := exec.Command("aws", "logs", "get-log-events", 
		"--log-group-name", logGroup,
		"--log-stream-name", logStream,
		"--limit", "100",
		"--region", "us-east-2",
		"--output", "json")
	
	output, err := cmd.Output()
	if err != nil {
		logger.Printf("[ERROR] AWS CLI failed: %v", err)
		response := map[string]interface{}{
			"logs": []string{"Failed to read CloudWatch logs:", err.Error(), "Command: aws logs get-log-events --log-group-name " + logGroup + " --log-stream-name " + logStream},
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
			"error": "AWS CLI failed",
			"source": "CloudWatch Logs: " + logGroup + "/" + logStream,
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	
	var awsResponse struct {
		Events []struct {
			Message   string `json:"message"`
			Timestamp int64  `json:"timestamp"`
		} `json:"events"`
	}
	
	if err := json.Unmarshal(output, &awsResponse); err != nil {
		response := map[string]interface{}{
			"logs": []string{"Failed to parse CloudWatch response:", err.Error()},
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
			"error": "JSON parse failed",
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	
	var logs []string
	for _, event := range awsResponse.Events {
		logs = append(logs, event.Message)
	}
	

	response := map[string]interface{}{
		"logs": logs,
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		"source": "CloudWatch Logs: " + logGroup + "/" + logStream,
	}
	
	json.NewEncoder(w).Encode(response)
}

func getLogStreamsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	// Get streams from connected agents' configurations
	agents := data.AllAgents.GetAllAgentsReadonlyClone()
	streamsFromAgents := make(map[string]bool)
	
	for _, agent := range agents {
		// Include streams from all agents with effective config (healthy or not)
		if effectiveConfig := agent.EffectiveConfig; effectiveConfig != "" {
			lines := strings.Split(effectiveConfig, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.Contains(line, "log_stream_name:") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) > 1 {
						stream := strings.Trim(strings.TrimSpace(parts[1]), `" `)
						if stream != "" {
							streamsFromAgents[stream] = true
						}
					}
				}
			}
		}
	}
	
	var streams []string
	for stream := range streamsFromAgents {
		streams = append(streams, stream)
	}
	
	response := map[string]interface{}{
		"streams": streams,
	}
	
	json.NewEncoder(w).Encode(response)
}

func debugFilesAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	// Check what files exist in common directories
	dirs := []string{
		"/opt/aws/amazon-cloudwatch-agent/logs",
		"/var/log/amazon",
		"/var/log",
		"./storage",
	}
	
	result := make(map[string]interface{})
	for _, dir := range dirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			result[dir] = "Directory not accessible: " + err.Error()
			continue
		}
		
		var fileNames []string
		for _, file := range files {
			fileNames = append(fileNames, file.Name())
		}
		result[dir] = fileNames
	}
	
	json.NewEncoder(w).Encode(result)
}
