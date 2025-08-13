# OpAMP Streamlit UI

## Setup

1. Install dependencies:
```bash
pip install -r requirements.txt
```

2. Start the Go OpAMP server:
```bash
cd ../internal/examples/server
go run main.go
```

3. Run Streamlit app:
```bash
streamlit run app.py
```

The Streamlit UI will be available at http://localhost:8501
The Go server API is at http://localhost:4321