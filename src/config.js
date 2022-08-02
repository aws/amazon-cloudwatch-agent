
export const DEBUG = false
export const GENERAL_ATTRIBUTES = ["Hash", "Year", "CommitDate", "isRelease"]
export const TEST_VARIABLES = ["Number of Logs","TPS"]
export const IGNORE_ATTRIBUTES  = [...GENERAL_ATTRIBUTES,"Link","Data"]
//Metric Specific Configs
export const UNITS = {
    procstat_cpu_usage : "%",
    procstat_memory_rss: "B"
}
export const MetricConfigs = [
    "thresholds",
]
//@TODO: make this auto update
const N_METRIC = 2 //number of metrics
const N_TIMESTAMPS = 3 // number of timestamps per metric
export const N_STATS = 4
//CALCULATED CONST
// 1 commit is # kb, so to make max 1MB batches our batches should be 
const ONE_MB = 1000 //KB
const BASE_PACKET_SIZE = 0.057 // KB per packet with no timstamp or metric
const TIMESTAMP_SIZE = 0.026 // KB per timestamp
const METRIC_SIZE = 0.154 //KB per metric with no timestamps
const PACKET_SIZE = (BASE_PACKET_SIZE + N_METRIC * (METRIC_SIZE + N_TIMESTAMPS * TIMESTAMP_SIZE))
export const BATCH_SIZE = parseInt(ONE_MB / PACKET_SIZE)


export const DEFAULT_CONFIG = {
    "sigfig": "3",
    "textFontSize": "20",
    "graphFontSize": "16",
    "graphSize": "2",
    "tableFontSize": "15",
    "nLastCommits": "10",
    //Add logs to here 
    "metricConfig": {
        "procstat_cpu_usage": {
            "thresholds": "0.0",
        },
        "procstat_memory_rss": {
            "thresholds": "0.0",
        }
    },

}
