import streamlit as st
import requests
import pandas as pd
from datetime import datetime, timedelta
import json
import plotly.graph_objects as go
import plotly.express as px

st.set_page_config(page_title="X-Ray Traces", layout="wide")

st.title("🔍 X-Ray Traces")

# OpAMP server API
try:
    # Time range selection
    col1, col2 = st.columns(2)
    with col1:
        time_filter = st.selectbox("Time Range", ["1h", "6h", "12h", "24h"], index=0)
    with col2:
        if st.button("Refresh Traces"):
            st.rerun()
    
    # Get traces from OpAMP server (all agents)
    response = requests.get(f"http://localhost:4321/api/agent/traces?time_filter={time_filter}")
    traces_data = response.json()
    
    traces = traces_data.get('traces', [])
    
    if traces:
        st.header(f"Found {len(traces)} traces")
        
        # Create service map visualization
        st.subheader("📊 Service Map")
        
        # Extract services and their metrics
        services = {}
        for trace in traces:
            service_ids = trace.get('ServiceIds', [])
            for service in service_ids:
                service_name = service.get('Name', 'Unknown')
                if service_name not in services:
                    services[service_name] = {
                        'trace_count': 0,
                        'total_duration': 0,
                        'error_count': 0
                    }
                services[service_name]['trace_count'] += 1
                services[service_name]['total_duration'] += trace.get('Duration', 0)
                if trace.get('HasError'):
                    services[service_name]['error_count'] += 1
        
        if services:
            # Create service metrics chart
            service_names = list(services.keys())
            trace_counts = [services[s]['trace_count'] for s in service_names]
            avg_durations = [services[s]['total_duration'] / services[s]['trace_count'] if services[s]['trace_count'] > 0 else 0 for s in service_names]
            
            col1, col2 = st.columns(2)
            
            with col1:
                # Trace count by service
                fig1 = px.bar(x=service_names, y=trace_counts, 
                             title="Traces by Service",
                             labels={'x': 'Service', 'y': 'Trace Count'})
                st.plotly_chart(fig1, use_container_width=True)
            
            with col2:
                # Average duration by service
                fig2 = px.bar(x=service_names, y=avg_durations,
                             title="Average Duration by Service", 
                             labels={'x': 'Service', 'y': 'Duration (s)'})
                st.plotly_chart(fig2, use_container_width=True)
        
        st.subheader("📋 Trace Details")
        # Display traces table
        trace_data = []
        for trace in traces:
            # Handle different date formats
            start_time = trace.get('StartTime', '')
            if isinstance(start_time, str):
                start_time_str = start_time
            else:
                start_time_str = str(start_time)
            
            trace_data.append({
                "Trace ID": trace.get('Id', 'N/A'),
                "Duration": f"{trace.get('Duration', 0):.3f}s",
                "Response Time": f"{trace.get('ResponseTime', 0):.3f}s", 
                "Start Time": start_time_str,
                "Has Error": "❌" if trace.get('HasError') == True else "✅",
                "Span Name": trace.get('Id', 'N/A').split('-')[-1] if trace.get('Id') else 'N/A',
                "Service": trace.get('ServiceIds', [{}])[0].get('Name', 'Unknown') if trace.get('ServiceIds') else 'Unknown'
            })
        
        df = pd.DataFrame(trace_data)
        
        # Select trace for details
        selected_trace = st.selectbox(
            "Select trace for details:",
            options=[f"{t['Trace ID']} - {t['Service']}" for t in trace_data]
        )
        
        if selected_trace:
            trace_id = selected_trace.split(" - ")[0]
            
            # Get trace details from OpAMP server
            detail_response = requests.get(f"http://localhost:4321/api/agent/trace-details?trace_id={trace_id}")
            trace_detail = detail_response.json()
            
            if trace_detail['Traces']:
                trace = trace_detail['Traces'][0]
                
                st.subheader(f"Trace Details: {trace_id}")
                
                # Display segments
                for segment in trace['Segments']:
                    segment_doc = json.loads(segment['Document'])
                    
                    with st.expander(f"📊 {segment_doc.get('name', 'Unknown Service')}", expanded=True):
                        col1, col2 = st.columns(2)
                        
                        with col1:
                            st.write(f"**Service:** {segment_doc.get('name')}")
                            st.write(f"**Start Time:** {datetime.fromtimestamp(segment_doc.get('start_time', 0))}")
                            st.write(f"**Duration:** {segment_doc.get('end_time', 0) - segment_doc.get('start_time', 0):.3f}s")
                        
                        with col2:
                            if 'aws' in segment_doc:
                                st.write("**AWS Metadata:**")
                                st.json(segment_doc['aws'])
                        
                        # Show raw segment
                        if st.checkbox(f"Show raw segment for {segment_doc.get('name')}", key=segment_doc.get('id')):
                            st.json(segment_doc)
        
        st.dataframe(df, use_container_width=True)
    else:
        st.info("No traces found in the selected time range")
        st.write("Try:")
        st.write("- Expanding the time range")
        st.write("- Sending test traces to your OTLP endpoint")

except Exception as e:
    st.error(f"Failed to connect to OpAMP server: {e}")
    st.info("Make sure the Go server is running on localhost:4321")