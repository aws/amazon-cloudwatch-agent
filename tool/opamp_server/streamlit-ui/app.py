import streamlit as st
import pandas as pd
from utils.api_client import OpAMPClient
from datetime import datetime

st.set_page_config(page_title="OpAMP Dashboard", layout="wide")

client = OpAMPClient()

st.title("OpAMP Server Dashboard")

# Add navigation info
st.sidebar.info("📋 Visit the **Logs** page for detailed agent log viewing!")

# Auto-refresh
if st.button("Refresh"):
    st.rerun()

try:
    agents = client.get_agents()
    
    if agents:
        st.header(f"Connected Agents ({len(agents)})")
        
        # Create DataFrame for display
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
        
        df = pd.DataFrame(agent_data)
        st.dataframe(df, use_container_width=True)
        
        # Agent selection for details - show more info to distinguish agents
        agent_options = []
        for i, agent_data_item in enumerate(agent_data):
            instance_id = agent_data_item["Instance ID"]
            started_at = agent_data_item["Started At"]
            agent_options.append(f"{instance_id} (Started: {started_at})")
        
        # Preserve selected agent across refreshes
        if "selected_agent_id" not in st.session_state:
            st.session_state.selected_agent_id = None
        
        # Find current option for previously selected agent
        current_index = 0
        if st.session_state.selected_agent_id:
            for i, option in enumerate(agent_options):
                if option.startswith(st.session_state.selected_agent_id):
                    current_index = i
                    break
        
        selected_agent_display = st.selectbox(
            "Select agent for details:",
            options=agent_options,
            index=current_index
        )
        
        # Update session state with current selection
        if selected_agent_display:
            st.session_state.selected_agent_id = selected_agent_display.split(" (")[0]
        
        # Extract just the instance ID from the selection
        selected_agent = selected_agent_display.split(" (")[0] if selected_agent_display else None
        
        if selected_agent:
            # Find the selected agent
            selected_agent_data = None
            for agent_id, agent in agents.items():
                if agent.get("InstanceIdStr") == selected_agent:
                    selected_agent_data = agent
                    break
            
            if selected_agent_data:
                st.header(f"Agent Details: {selected_agent_display}")
                
                col1, col2 = st.columns(2)
                
                with col1:
                    st.subheader("Status")
                    status = selected_agent_data.get("Status", {})
                    health = status.get("Health", {})
                    is_healthy = health.get("Healthy", False)
                    st.write(f"**Healthy:** {'Yes' if is_healthy else 'No'}")
                    if health.get("LastError"):
                        st.error(f"Last Error: {health.get('LastError')}")
                    
                    # Control buttons
                    st.subheader("Control")
                    col_start, col_stop = st.columns(2)
                    
                    with col_start:
                        if st.button("🟢 Start", use_container_width=True):
                            st.rerun()
                    
                    with col_stop:
                        if st.button("🔴 Stop", use_container_width=True):
                            try:
                                client.stop_agent(selected_agent)
                                st.success("Stop command sent!")
                                st.rerun()
                            except Exception as e:
                                st.error(f"Failed to stop: {e}")
                
                with col2:
                    st.subheader("Attributes")
                    attributes = selected_agent_data.get("Attributes", {})
                    if attributes:
                        for key, value in attributes.items():
                            st.write(f"**{key}:** {value}")
                    else:
                        st.write("No attributes available")
                
                # Components and Pipelines section
                st.subheader("Components & Pipelines")
                
                effective_config = selected_agent_data.get("EffectiveConfig", "")
                components = set()
                pipelines = {}
                
                # Parse only YAML section from effective config
                if effective_config:
                    yaml_start = effective_config.find("Config: effective.yaml")
                    if yaml_start != -1:
                        yaml_content_start = effective_config.find("\n", yaml_start + effective_config[yaml_start:].find("\n") + 1)
                        if yaml_content_start != -1:
                            next_separator = effective_config.find("\n" + "=" * 80, yaml_content_start)
                            if next_separator != -1:
                                yaml_section = effective_config[yaml_content_start:next_separator]
                            else:
                                yaml_section = effective_config[yaml_content_start:]
                            
                            try:
                                import yaml
                                config_data = yaml.safe_load(yaml_section)
                                if config_data:
                                    for comp_type in ['receivers', 'processors', 'exporters', 'extensions']:
                                        if comp_type in config_data:
                                            for comp_name in config_data[comp_type].keys():
                                                components.add(f"{comp_type[:-1]}: {comp_name}")
                                    
                                    if 'service' in config_data and 'pipelines' in config_data['service']:
                                        for pipeline_name, pipeline_config in config_data['service']['pipelines'].items():
                                            pipelines[pipeline_name] = {
                                                'receivers': pipeline_config.get('receivers', []),
                                                'processors': pipeline_config.get('processors', []),
                                                'exporters': pipeline_config.get('exporters', [])
                                            }
                            except:
                                pass
                
                if st.checkbox("Show Components", value=False):
                    if components:
                        st.write("**Available Components:**")
                        # Convert to DataFrame format like original
                        comp_data = []
                        for comp in sorted(components):
                            comp_type, comp_name = comp.split(": ", 1)
                            comp_data.append({
                                "Type": comp_type.title(),
                                "Name": comp_name,
                                "Used": "✅"
                            })
                        components_df = pd.DataFrame(comp_data)
                        st.dataframe(components_df, use_container_width=True)
                    else:
                        st.write("No components found in YAML config")
                
                if st.checkbox("Show Pipelines", value=False):
                    if pipelines:
                        col_metrics, col_traces, col_logs = st.columns(3)
                        
                        with col_metrics:
                            metrics_count = len([p for p in pipelines.keys() if p.startswith('metrics')])
                            st.metric("Metrics Pipelines", metrics_count, border=True)
                        with col_traces:
                            traces_count = len([p for p in pipelines.keys() if p.startswith('traces')])
                            st.metric("Traces Pipelines", traces_count, border=True)
                        with col_logs:
                            logs_count = len([p for p in pipelines.keys() if p.startswith('logs')])
                            st.metric("Logs Pipelines", logs_count, border=True)
                        
                        for pipeline_name, pipeline_data in pipelines.items():
                            with st.expander(f"Pipeline: {pipeline_name}", expanded=True):
                                st.write(f"**Receivers:** {', '.join(pipeline_data.get('receivers', []))}")
                                st.write(f"**Processors:** {', '.join(pipeline_data.get('processors', []))}")
                                st.write(f"**Exporters:** {', '.join(pipeline_data.get('exporters', []))}")
                    else:
                        st.write("No pipelines found in YAML config")
                
                st.subheader("Configuration")
                col3, col4 = st.columns(2)
                
                with col3:
                    st.write("**Effective Config:**")
                    effective_config = selected_agent_data.get("EffectiveConfig", "")

                    if effective_config:
                        # Parse different config sections
                        config_sections = effective_config.split("\n" + "=" * 80 + "\n")
                        
                        has_sections = False
                        for section in config_sections:
                            if section.strip():
                                lines = section.strip().split("\n")
                                if lines and lines[0].startswith("Config: "):
                                    has_sections = True
                                    config_name = lines[0].replace("Config: ", "")
                                    config_content = "\n".join(lines[2:])  # Skip header and separator
                                    
                                    with st.expander(f"📄 {config_name}", expanded=False):
                                        if config_name.endswith(".json"):
                                            st.code(config_content, language="json")
                                        elif config_name.endswith(".yaml"):
                                            st.code(config_content, language="yaml")
                                        elif config_name.endswith(".toml"):
                                            st.code(config_content, language="toml")
                                        else:
                                            st.code(config_content)
                        
                        # If no sections found, display as raw YAML
                        if not has_sections:
                            with st.expander("📄 effective.yaml", expanded=False):
                                st.code(effective_config, language="yaml")
                    else:
                        st.write("No effective config")
                
                with col4:
                    st.write("**Custom Config:**")
                    config = selected_agent_data.get("CustomInstanceConfig", "")
                    new_config = st.text_area("Agent Config:", value=config, height=200)
                    
                    if st.button("Update Config"):
                        try:
                            client.update_agent_config(selected_agent, new_config)
                            st.success("Configuration updated!")
                        except Exception as e:
                            st.error(f"Failed to update config: {e}")
                
                # Logs section
                st.subheader("Agent Logs")
                st.info("📋 **View detailed logs on the dedicated Logs page (use sidebar navigation)**")
                
                # Quick log preview
                if st.button("Show Quick Log Preview", key=f"preview_{selected_agent}"):
                    try:
                        logs_data = client.get_agent_logs(selected_agent)
                        if logs_data and 'logs' in logs_data:
                            log_lines = logs_data['logs']
                            log_lines = [line for line in log_lines if line.strip()]
                            if log_lines:
                                st.text_area("Last 10 log entries:", value='\n'.join(log_lines[-10:]), height=200, disabled=True)
                            else:
                                st.info("No recent log entries")
                    except Exception as e:
                        st.error(f"Failed to fetch logs: {e}")
    else:
        st.info("No agents connected")

except Exception as e:
    st.error(f"Failed to connect to OpAMP server: {e}")
    st.info("Make sure the Go server is running on localhost:4321")