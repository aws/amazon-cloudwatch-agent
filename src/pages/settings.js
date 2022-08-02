import './settings.css'
import {MetricConfigs } from "../config"
//,TEST_VARIABLES
const CONFIG = "config"
const METRIC_CONFIG_KEY = "metricConfig"
//const TABLE_CONFIG_KEY = "tableConfig"

function loadSetting(settingTag) {
    var config = JSON.parse(localStorage.getItem(CONFIG))
    if (config == null) {
        return ""
    }
    return config[settingTag]
}
function saveSetting(settingTag, value) {
    var config = JSON.parse(localStorage.getItem(CONFIG))
    if (config == null) {
        config = {}
    }
    config[settingTag] = value
    // debugger;
    localStorage.setItem(CONFIG, JSON.stringify(config))
}
export default function Setting(props) {
    var key = props.settingKey || props.title
    var defaultValue = props.defaultValue || loadSetting(key)
    // const [defaultValue,setDefaultValue] = useState(prevValue)
    var inputType = <input type="text" />
    switch (props.type) {
        case "select": {
            if (props.range === undefined) { 
                throw Object.assign(
                new Error("Select requires a range like : [start,end,interval,unit]"),
                {code: 400}
                );  
            }
            var options = []
            for (var i = props.range[0]; i < props.range[1]; i = i + props.range[2]) {
                options.push(
                    <option selected={i === defaultValue ? "selected" : ""}
                        value={i}>{`${i} ${props.range[3]}`}</option>
                )
            }
            inputType = <select onChange={event => {
                saveSetting(key, event.target.value)
                if (props.page !== undefined) {
                    props.page.updateConfig()
                }
            }}>{options}</select>
            break
        }
        default: {
            inputType = <input type={props.type} onChange={event => {
                if (props.onSave === undefined) {
                    saveSetting(key, event.target.value)
                } else {
                    props.onSave(key, event.target.value)
                }
            }} placeholder={defaultValue} />
            break;
        }
    }
    return (
        <div class="setting">
            <label class="setting-left">{props.title}</label>
            {inputType}
        </div>
    )
}



export function MetricSettingsBox(props) {
    if (props.data === undefined || props.data == null) {
        return null
    }
    var testCase = props.data[Object.keys(props.data)[0]]
    if (testCase === undefined) {
        return
    }
    var metrics = Object.keys(testCase)
    var metricSpecificSettings = []
    var defaultValue = loadSetting(METRIC_CONFIG_KEY)
    MetricConfigs.forEach((settingKey) => {
        metricSpecificSettings.push(<h4>{settingKey.toUpperCase()}</h4>)
        metrics.forEach((metric) => {
            metricSpecificSettings.push(<Setting
                title={`${metric}`}
                settingKey={`${settingKey}`}
                onSave={(key, value) => {
                    defaultValue[metric][key] = parseFloat(value)
                    saveSetting(METRIC_CONFIG_KEY, defaultValue)
                    if (props.page !== undefined) {
                        props.page.updateConfig()
                    }
                }}
                defaultValue={defaultValue[metric][settingKey]}
            />)
        })
    })

    return (
        <div class="metric_setting_box">
            <h3>Metric Settings</h3>
            <h2><br></br></h2>
            {metricSpecificSettings}
        </div>
    )
}

