import streamlit as st
import pandas as pd
from utils.api_client import OpAMPClient

st.set_page_config(page_title="Agent Logs", layout="wide")

client = OpAMPClient()

st.title("📋 Agent Logs")

try:
    agents = client.get_agents()
    
    if agents:
        # Agent selection
        agent_data = []
        for agent_id, agent in agents.items():
            status = agent.get("Status", {})
            health = status.get("Health", {})
            
            agent_data.append({
                "Instance ID": agent.get("InstanceIdStr", "N/A"),
                "Healthy": "✅" if health.get("Healthy") else "❌",
                "Started At": agent.get("StartedAt", "N/A"),
                "Last Error": health.get("LastError", "None")
            })
        
        # Agent selection dropdown
        agent_options = []
        for i, agent_data_item in enumerate(agent_data):
            instance_id = agent_data_item["Instance ID"]
            started_at = agent_data_item["Started At"]
            agent_options.append(f"{instance_id} (Started: {started_at})")
        
        selected_agent_display = st.selectbox(
            "Select agent for logs:",
            options=agent_options
        )
        
        selected_agent = selected_agent_display.split(" (")[0] if selected_agent_display else None
        
        if selected_agent:
            # Find the selected agent data
            selected_agent_data = None
            for agent_id, agent in agents.items():
                if agent.get("InstanceIdStr") == selected_agent:
                    selected_agent_data = agent
                    break
            
            if selected_agent_data:
                st.header(f"Logs for: {selected_agent_display}")
                
                # Show agent's log configuration
                effective_config = selected_agent_data.get("EffectiveConfig", "")
                agent_log_group = "Opamp_supervisor_log"  # default
                agent_log_stream = ""
                
                if effective_config:
                    lines = effective_config.split("\n")
                    for line in lines:
                        line = line.strip()
                        if "log_group_name:" in line:
                            parts = line.split(":")
                            if len(parts) > 1:
                                agent_log_group = parts[1].strip().strip('"')
                        elif "log_stream_name:" in line:
                            parts = line.split(":")
                            if len(parts) > 1:
                                agent_log_stream = parts[1].strip().strip('"')
                
                if agent_log_stream:
                    st.info(f"🔗 **This agent sends logs to:** `{agent_log_group}/{agent_log_stream}`")
                else:
                    st.info(f"🔗 **This agent's log configuration:** Not found in config")
                
                # Log stream selection
                col_stream, col_refresh = st.columns([3, 1])
                with col_stream:
                    # Get streams from all connected agents with config
                    available_streams = []
                    for agent_id, agent in agents.items():
                        # Check if agent has effective config (regardless of health)
                        has_config = bool(agent.get("EffectiveConfig", ""))
                        
                        if has_config:
                            effective_config = agent.get("EffectiveConfig", "")
                            lines = effective_config.split("\n")
                            for line in lines:
                                if "log_stream_name:" in line:
                                    parts = line.split(":")
                                    if len(parts) > 1:
                                        stream_name = parts[1].strip().strip('"')
                                        if stream_name and stream_name not in available_streams:
                                            available_streams.append(stream_name)
                    
                    # If no streams found from agents, show message
                    if not available_streams:
                        st.info("ℹ️ No log streams configured for connected agents")
                        available_streams = []
                    
                    if available_streams:
                        stream_options = available_streams + ["Custom..."]
                        
                        # Get agent's configured stream as default
                        agent_configured_stream = ""
                        if effective_config:
                            lines = effective_config.split("\n")
                            for line in lines:
                                if "log_stream_name:" in line:
                                    parts = line.split(":")
                                    if len(parts) > 1:
                                        agent_configured_stream = parts[1].strip().strip('"')
                                        break
                        
                        # Use session state to track stream changes
                        stream_key = f"stream_select_{selected_agent}"
                        if stream_key not in st.session_state:
                            # Default to first available stream
                            st.session_state[stream_key] = available_streams[0]
                        
                        # Show agent's natural stream first in options if it exists
                        if agent_configured_stream and agent_configured_stream not in stream_options:
                            stream_options = [agent_configured_stream] + stream_options
                        
                        selected_stream = st.selectbox(
                            "Select log stream:",
                            options=stream_options,
                            index=stream_options.index(st.session_state[stream_key]) if st.session_state[stream_key] in stream_options else 0,
                            key=stream_key,
                            help=f"This agent's natural stream: {agent_configured_stream}" if agent_configured_stream else "Select any available stream"
                        )
                        
                        if selected_stream == "Custom...":
                            custom_stream = st.text_input(
                                "Enter stream name:",
                                key=f"custom_stream_{selected_agent}"
                            )
                            if custom_stream:
                                selected_stream = custom_stream
                            else:
                                selected_stream = available_streams[0] if available_streams else ""
                    else:
                        st.warning("⚠️ No log streams available from connected agents")
                        selected_stream = None
                
                with col_refresh:
                    st.write("")
                    if st.button("Refresh Logs", key=f"refresh_logs_{selected_agent}"):
                        st.rerun()
                
                if selected_stream:
                    try:
                        logs_data = client.get_agent_logs(selected_agent, stream=selected_stream)
                        if logs_data and 'logs' in logs_data:
                            st.write(f"**Source:** {logs_data.get('source', 'Unknown')}")
                        
                            # Find which agent actually owns this log stream
                            stream_owner = "Unknown"
                            for agent_id, agent in agents.items():
                                if effective_config := agent.get("EffectiveConfig", ""):
                                    lines = effective_config.split("\n")
                                    for line in lines:
                                        if "log_stream_name:" in line:
                                            parts = line.split(":")
                                            if len(parts) > 1:
                                                agent_stream = parts[1].strip().strip('"')
                                                if agent_stream == selected_stream:
                                                    stream_owner = agent.get("InstanceIdStr", "Unknown")
                                                    break
                                    if stream_owner != "Unknown":
                                        break
                            
                            if stream_owner != "Unknown":
                                if stream_owner == selected_agent:
                                    st.info(f"📋 **Stream Owner:** This stream belongs to the selected agent (`{stream_owner}`)") 
                                else:
                                    st.warning(f"⚠️ **Stream Owner:** This stream actually belongs to agent `{stream_owner}`, not the selected agent (`{selected_agent}`)") 
                            else:
                                st.info(f"📋 **Stream Info:** Viewing stream `{selected_stream}` (owner unknown)")
                            
                            st.write(f"**Last Updated:** {logs_data.get('timestamp', 'Unknown')}")
                            log_lines = logs_data['logs']
                            # Filter out empty lines
                            log_lines = [line for line in log_lines if line.strip()]
                            if log_lines:
                                # Show logs in a scrollable text area
                                logs_text = '\n'.join(log_lines)  # Show all lines
                                st.text_area("Recent Logs:", value=logs_text, height=400, disabled=True)
                            else:
                                st.info("No log entries found")
                        else:
                            st.warning("Could not retrieve logs")
                    except Exception as e:
                        st.error(f"Failed to fetch logs: {e}")
                else:
                    st.info("📋 No log streams to display")
    
    else:
        st.info("No agents connected")

except Exception as e:
    st.error(f"Failed to connect to OpAMP server: {e}")
    st.info("Make sure the Go server is running on localhost:4321")