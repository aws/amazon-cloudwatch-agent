import streamlit as st
import pandas as pd
import plotly.express as px
import plotly.graph_objects as go
from utils.api_client import OpAMPClient
from datetime import datetime

st.set_page_config(page_title="Agent Metrics", layout="wide")

client = OpAMPClient()

st.title("📊 Agent Metrics")

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
        

        
        # Agent selection dropdown with instance ID support
        agent_options = {}
        for agent_data_item in agent_data:
            instance_id = agent_data_item["Instance ID"]
            started_at = agent_data_item["Started At"]
            display_name = f"{instance_id} (Started: {started_at})"
            agent_options[display_name] = instance_id
        
        selected_agent_display = st.selectbox(
            "Select agent for metrics:",
            options=list(agent_options.keys()),
            key="agent_selector"
        )
        
        # Get the actual agent UUID from the agents data
        selected_agent_id = None
        for agent_id, agent in agents.items():
            instance_id = agent.get("InstanceIdStr", "N/A")
            started_at = agent.get("StartedAt", "N/A")
            display_name = f"{instance_id} (Started: {started_at})"
            if selected_agent_display == display_name:
                selected_agent_id = agent_id

                break
        

        # Clear metrics when agent changes
        if "previous_agent" not in st.session_state:
            st.session_state.previous_agent = selected_agent_display
        elif st.session_state.previous_agent != selected_agent_display:
            st.session_state.previous_agent = selected_agent_display
            st.session_state.metrics_selector = []
        
        selected_agent = selected_agent_id is not None
        
        if selected_agent:
            
            # Time filter selection
            col1, col2, col3, col4, col5 = st.columns([1, 1, 1, 1, 2])
            
            with col1:
                if st.button("30m"):
                    st.session_state.time_filter = "30m"
            with col2:
                if st.button("1h"):
                    st.session_state.time_filter = "1h"
            with col3:
                if st.button("6h"):
                    st.session_state.time_filter = "6h"
            with col4:
                if st.button("24h"):
                    st.session_state.time_filter = "24h"
            with col5:
                custom_time = st.text_input("Custom (e.g., 2h, 45m):", key="custom_time")
                if custom_time:
                    st.session_state.time_filter = custom_time
            
            time_filter = st.session_state.get("time_filter", "1h")
            st.info(f"📅 **Time Range:** Last {time_filter}")
            
            # Get available metrics
            available_metrics_data = client.get_available_metrics(namespace="CWAgent")
            available_metrics = sorted(available_metrics_data.get("metrics", []))
            
            # Metric selection
            selected_metrics = st.multiselect(
                "Select metrics to display:",
                options=available_metrics,
                key="metrics_selector"
            )
            
            if selected_metrics:
                # Create tabs for different metric views
                tab1, tab2 = st.tabs(["📈 Time Series", "📋 Current Values"])
                
                with tab1:
                    # Display metrics charts
                    for metric_name in selected_metrics:
                        try:
                            metrics_data = client.get_agent_metrics(
                                agent_id=selected_agent_id,
                                metric_name=metric_name,
                                time_filter=time_filter,
                                namespace="CWAgent"
                            )
                            
                            if "error" in metrics_data:
                                st.error(f"Failed to fetch {metric_name}: {metrics_data['error']}")
                                continue
                            
                            datapoints = metrics_data.get("metrics", [])
                            if datapoints:
                                # Convert to DataFrame for plotting
                                df = pd.DataFrame(datapoints)
                                df['Timestamp'] = pd.to_datetime(df['Timestamp'])
                                df = df.sort_values('Timestamp')
                                
                                # Create time series chart
                                fig = px.line(
                                    df, 
                                    x='Timestamp', 
                                    y='Average',
                                    title=f"{metric_name}",
                                    labels={'Average': 'Value', 'Timestamp': 'Time'}
                                )
                                fig.update_layout(height=400)
                                st.plotly_chart(fig, use_container_width=True)
                            else:
                                st.warning(f"No data available for {metric_name}")
                        
                        except Exception as e:
                            st.error(f"Error displaying {metric_name}: {e}")
                
                with tab2:
                    # Display current metric values
                    st.subheader("Current Metric Values")
                    
                    metric_cols = st.columns(min(len(selected_metrics), 3))
                    
                    for i, metric_name in enumerate(selected_metrics):
                        try:
                            metrics_data = client.get_agent_metrics(
                                agent_id=selected_agent_id,
                                metric_name=metric_name,
                                time_filter="30m",
                                namespace="CWAgent"
                            )
                            
                            datapoints = metrics_data.get("metrics", [])
                            if datapoints:
                                # Get the most recent value
                                latest_point = max(datapoints, key=lambda x: x['Timestamp'])
                                current_value = latest_point['Average']
                                timestamp = datetime.fromisoformat(latest_point['Timestamp'].replace('Z', '+00:00'))
                                
                                with metric_cols[i % 3]:
                                    st.metric(
                                        label=metric_name,
                                        value=f"{current_value:.2f}",
                                        help=f"Last updated: {timestamp.strftime('%H:%M:%S')}"
                                    )
                            else:
                                with metric_cols[i % 3]:
                                    st.metric(
                                        label=metric_name,
                                        value="No data",
                                        help="No recent data available"
                                    )
                        
                        except Exception as e:
                            with metric_cols[i % 3]:
                                st.metric(
                                    label=metric_name,
                                    value="Error",
                                    help=f"Error: {e}"
                                )
            else:
                st.info("Please select at least one metric to display")
    
    else:
        st.info("No agents connected")

except Exception as e:
    st.error(f"Failed to connect to OpAMP server: {e}")
    st.info("Make sure the Go server is running on localhost:4321")