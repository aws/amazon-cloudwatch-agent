import requests
import json
from typing import Dict, Any, Optional

class OpAMPClient:
    def __init__(self, base_url: str = "http://localhost:4321"):
        self.base_url = base_url
    
    def get_agents(self) -> Dict[str, Any]:
        """Get all agents from the OpAMP server"""
        try:
            response = requests.get(f"{self.base_url}/api/agents", timeout=5)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            raise Exception(f"Failed to fetch agents: {e}")
    
    def get_agent(self, instance_id: str) -> Optional[Dict[str, Any]]:
        """Get specific agent by instance ID"""
        try:
            response = requests.get(f"{self.base_url}/api/agent/{instance_id}", timeout=5)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            raise Exception(f"Failed to fetch agent {instance_id}: {e}")
    
    def update_agent_config(self, instance_id: str, config: str) -> bool:
        """Update agent configuration"""
        try:
            data = {"instanceid": instance_id, "config": config}
            response = requests.post(f"{self.base_url}/save_config", data=data, timeout=10)
            response.raise_for_status()
            return True
        except requests.exceptions.RequestException as e:
            raise Exception(f"Failed to update config for agent {instance_id}: {e}")
    
    def start_agent(self, instance_id: str) -> bool:
        """Start agent"""
        try:
            data = {"instanceid": instance_id}
            response = requests.post(f"{self.base_url}/api/agent/start", data=data, timeout=30)
            response.raise_for_status()
            return True
        except requests.exceptions.RequestException as e:
            raise Exception(f"Failed to start agent {instance_id}: {e}")
    
    def stop_agent(self, instance_id: str) -> bool:
        """Stop agent"""
        try:
            data = {"instanceid": instance_id}
            response = requests.post(f"{self.base_url}/api/agent/stop", data=data, timeout=30)
            response.raise_for_status()
            return True
        except requests.exceptions.RequestException as e:
            raise Exception(f"Failed to stop agent {instance_id}: {e}")
    
    def get_agent_status(self) -> dict:
        """Get CloudWatch agent status"""
        try:
            response = requests.get(f"{self.base_url}/api/agent/status", timeout=10)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            return {"error": str(e)}
    
    def get_agent_logs(self, agent_id: str = None, stream: str = None) -> dict:
        """Get CloudWatch agent logs"""
        try:
            params = {}
            if agent_id:
                params['agent_id'] = agent_id
            if stream:
                params['stream'] = stream
            import time
            params['_t'] = str(int(time.time()))  # Cache buster
            url = f"{self.base_url}/api/agent/logs"
            response = requests.get(url, params=params, timeout=10)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            return {"error": str(e)}
    
    def get_log_streams(self) -> dict:
        """Get available log streams"""
        try:
            response = requests.get(f"{self.base_url}/api/agent/streams", timeout=10)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            return {"streams": ["Log_pipeline", "Log_testing"], "error": str(e)}
    
    def get_agent_metrics(self, agent_id: str = None, metric_name: str = None, time_filter: str = None, namespace: str = None) -> dict:
        """Get CloudWatch agent metrics"""
        try:
            params = {}
            if agent_id:
                params['agent_id'] = agent_id
            if metric_name:
                params['metric_name'] = metric_name
            if time_filter:
                params['time_filter'] = time_filter
            if namespace:
                params['namespace'] = namespace
            import time
            params['_t'] = str(int(time.time()))  # Cache buster
            url = f"{self.base_url}/api/agent/metrics"
            response = requests.get(url, params=params, timeout=10)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            return {"error": str(e)}
    
    def get_available_metrics(self, namespace: str = None) -> dict:
        """Get available metrics from CloudWatch"""
        try:
            params = {}
            if namespace:
                params['namespace'] = namespace
            response = requests.get(f"{self.base_url}/api/agent/available-metrics", params=params, timeout=10)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            return {"metrics": ["mem_used_percent", "disk_used_percent", "swap_used_percent"], "error": str(e)}