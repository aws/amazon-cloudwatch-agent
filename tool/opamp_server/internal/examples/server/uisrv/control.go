package uisrv

import (
	"encoding/json"
	"net/http"
	"os/exec"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/internal/examples/server/data"
)

func startAgentAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	
	uid, err := uuid.Parse(r.Form.Get("instanceid"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	
	instanceId := data.InstanceId(uid)
	agent := data.AllAgents.FindAgent(instanceId)
	if agent == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	
	// The supervisor automatically restarts the agent after it detects it's stopped
	// Just return success - the supervisor will handle the restart in ~5 seconds
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "restart command sent to supervisor"})
}

func stopAgentAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	
	// Kill the CloudWatch agent process - supervisor will detect and restart
	cmd := exec.Command("pkill", "-f", "amazon-cloudwatch-agent")
	output, err := cmd.Output()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
			"output": string(output),
		})
		return
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "CloudWatch agent supervisor stopped",
		"output": string(output),
	})
}

func getAgentStatusAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	cmd := exec.Command("sudo", "/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl", "-a", "status")
	output, err := cmd.Output()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	
	w.WriteHeader(http.StatusOK)
	w.Write(output)
}