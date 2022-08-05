import { useState } from "react";
import {
  LineChart,
  Line,
  CartesianGrid,
  XAxis,
  YAxis,
  Tooltip,
  Legend,
  ErrorBar,
  ReferenceLine,
} from "recharts";
import "./graph.css";
import { IGNORE_ATTRIBUTES, TEST_VARIABLES, N_STATS,UNITS } from "../config";

const IGNORE_ATTRIBUTES_GRAPH = IGNORE_ATTRIBUTES + ["Period", "Std"];
const MAX_COLOUR = 0xff;
const MIN_COLOUR = 0x11;
const COLOUR_DIFF_CONST = Math.floor((MAX_COLOUR - MIN_COLOUR) / N_STATS);


/*This function is given a seed and a index 
and return a hex colour in string format. 

seed:(0-2) determine if it is red,blue, or green based
idx:  represent the statistic we are drawing and assigns a colour for that stat.

As idx increases the colour goes closer to red,green, or blue depending on seed.
*/
function getRandomColour(seed, idx) {
  // at least 1 FF to make bright colours
  let coloursOptions = [MIN_COLOUR, MIN_COLOUR, MIN_COLOUR];
  coloursOptions[seed] += COLOUR_DIFF_CONST * idx;
  var colour = "#";
  for (var i = 0; i < 3; i++) {
    colour += coloursOptions[i].toString(16).slice(-2);
  }

  return colour;
}
const CustomToolTip = (props) => {
  var { active, payload, label } = props;

  if (active && payload && payload.length) {
    var payloades = [];
    for (var i = 0; i < payload.length; i++) {
      payloades.push(
        <p style={{ color: payload[i].stroke }}>
          {`${payload[i].name} : ${payload[i].value.toPrecision(
            props.config.sigfig
          )}`}
        </p>
      );
    }
    var date = new Date(payload[0].payload.CommitDate * 1000);
    payloades.push(
      <p>{`Commit Date: ${date.getUTCHours()}:${date.getUTCMinutes()}:${date.getUTCSeconds()}|${date.getUTCDate()}/${
        date.getUTCMonth() + 1
      }/${date.getUTCFullYear()}`}</p>
    );
    return (
      <div className="custom-tooltip">
        <p className="label">{label}</p>
        {payloades}
      </div>
    );
  }

  return null;
};
//This component graphs statistics for each metric
export function Graph(props) {
  var testCases = Object.keys(props.data);
  var testVariables = Array(testCases[0].split("-").length); //map[string]Set
  testCases.forEach((test) => {
    test.split("-").forEach((value, idx) => {
      if (testVariables[idx] === undefined) {
        testVariables[idx] = [];
      }

      if (!testVariables[idx].includes(value)) {
        testVariables[idx].push(value);
        testVariables[idx].sort();
      }
    });
  });
  const [currentTestCase, setCurrentTest] = useState(testCases[0]);
  var buttons = [];
  var metricLines = []; // list of react components
  var metricStatsNames = Object.keys(
    props.data[currentTestCase][props.metric][0]
  ); // names of stats
  var mainColour = props.idx % 2; // main colour for that metric., red , green ,blue
  var i = 0;
  var basicVisibility = {};
  metricStatsNames.forEach((name) => {
    basicVisibility[name] = true;
  }); //init  stats visibility
  const [visibility, setVisibility] = useState(basicVisibility);

  metricStatsNames.forEach((name) => {
    if (!IGNORE_ATTRIBUTES_GRAPH.includes(name)) {
      metricLines.push(
        <Line
          type="monotone"
          dataKey={name}
          stroke={getRandomColour(mainColour, i)}
          legendType={visibility[name] ? "circle" : "diamond"}
          style={{ display: visibility[name] ? "block" : "none" }}
          activeDot={{ r: visibility[name] ? 7 : 0 }}
        >
          {name.includes("Average") ? (
            <ErrorBar
              dataKey="Std"
              direction="y"
              style={{ display: visibility[name] ? "block" : "none" }}
            />
          ) : (
            <></>
          )}
        </Line>
      );
      //Draw each stats

      i++;
    }
  });
  if (props.config["metricConfig"][props.metric].thresholds !== undefined) {
    metricLines.push(
      <ReferenceLine
        y={props.config["metricConfig"][props.metric].thresholds}
        label="Threshold"
        stroke="blue"
        strokeDasharray="10 10"
      />
    ); //Add a threshold line
  }
  testVariables.forEach((varSet, i) => {
    var options = [];
    varSet.forEach((value) => {
      options.push(<option>{value}</option>);
    });

    buttons.push(
      <div class="select_box">
        <label>{TEST_VARIABLES[i]}</label>
        <select
          id={`testCase-${props.title}-${i}`}
          onChange={() => {
            var testCase = "";
            var n_variables = testVariables.length;
            for (var j = 0; j < n_variables; j++) {
              testCase += document.getElementById(
                `testCase${props.title}-${j}`
              ).value;
              if (j < n_variables - 1) {
                testCase += "-";
              }
            }
            setCurrentTest(testCase);
            
          }}
        >
          {options}
        </select>
      </div>
    );
  });

  props.data[currentTestCase][props.metric].forEach((item) => {
    if (item["isRelease"]) {
      metricLines.push(
        <ReferenceLine
          x={item["Hash"]}
          label=""
          stroke="#8B008B"
          strokeWidth={3}
        />
      );
    }
  });
  var size = parseInt(props.config.graphSize);

  return (
    <div class="graph">
      <div class="button_container">{buttons}</div>
      <h2>{`${props.title}-${currentTestCase}`}</h2>
      <LineChart
        width={487.5 + 162.5 * size}
        height={300 + 75 * size}
        data={props.data[currentTestCase][props.metric].slice(
          -props.config.nLastCommits,
          props.data.length
        )}
        margin={{ top: 5, right: 30 }}
        style={{ overflowY: "hidden"}}
      >
        {metricLines}
        <Tooltip content={<CustomToolTip config={props.config} />} />
        <Legend
          verticalAlign="top"
          onClick={(data) => {
            var lastVisibility = visibility[data["dataKey"]];
            setVisibility({
              ...visibility,
              [data["dataKey"]]: !lastVisibility,
            });
          }}
        />
        <CartesianGrid stroke="#ccc" strokeDasharray="3 3" />
        <XAxis
          dataKey="Hash"
          label="Commit Hash"
          height={100}
          tickCount={props.config.nLastCommits}
          style={{ fontSize: props.config.graphFontSize }}
        />

        <YAxis
          tickCount={6}
          width={100}
          label={UNITS[props.title]}
          domain={[(dataMin) => dataMin * 0.95, "auto"]}
          style={{ fontSize: props.config.graphFontSize }}
        />
      </LineChart>
    </div>
  );
}
//This component creates a graph for each metric
export default function Grapher(props) {
  if (props.data === undefined) {
    return;
  }

  var testCase = props.data[Object.keys(props.data)[0]];
  if (testCase === undefined) {
    return;
  }
  var metrics = Object.keys(testCase);
  var graphs = [];
  var j = 0;
  metrics.forEach((element) => {
    graphs.push(
      <Graph
        metric={element}
        data={props.data}
        idx={j}
        title={element}
        config={props.config}
      />
    ); // Add a graph
    j++;
  });
  return (
    <div id="graphArea" class="grapher">
      {graphs}
    </div>
  );
}
