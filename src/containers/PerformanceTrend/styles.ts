import { useTheme } from "@mui/material/styles";
import { ApexOptions } from "apexcharts";
import { OWNER_REPOSITORY } from "../../common/Constant";
import "./styles.css";

export function BasedOptionChart(): ApexOptions {
  const theme = useTheme();
  return {
    chart: {
      type: "line",
      toolbar: {
        show: true,
        offsetX: -100,
        offsetY: 5,
        tools: {
          selection: false,
          zoom: false,
          zoomin: false,
          zoomout: false,
          pan: false,
        },
      },
      events: {
        xAxisLabelClick: function (event: any, context: any, config: { globals: { categoryLabels: number[] }; labelIndex: number }) {
          const commit_sha = config.globals.categoryLabels.at(config.labelIndex);
          window.location.assign(`https://github.com/${OWNER_REPOSITORY}/${process.env.REACT_APP_GITHUB_REPOSITORY}/commit/${commit_sha}`);
        },
      },
    },
    xaxis: {
      labels: {
        rotateAlways: true,
        rotate: -45,
        style: {
          colors: [theme.palette.mode === "light" ? "#212121" : "#FFFFFF"],
          fontSize: "12px",
        },
        offsetX: 10,
        offsetY: 5,
      },
      tooltip: {
        enabled: false,
      },
      title: {
        text: "Commit Sha",
        style: {
          color: theme.palette.mode === "light" ? "#212121" : "#FFF",
          fontSize: "14px",
        },
        offsetY: -20,
      },
    },
    colors: ["#FF6384", "#FF9F40", "#FFCD56", "#0ED87C", "#4BC0C0", "#36A2EB", "#9965FF", "#996255", "#DF358D", "#DF358D"],
    yaxis: {
      min: 0,
      max: 300,
      labels: {
        style: {
          colors: [theme.palette.mode === "light" ? "#212121" : "#FFFFFF"],
        },
      },
      title: {
        style: {
          color: theme.palette.mode === "light" ? "#212121" : "#FFF",
          fontSize: "14px",
        },
      },
    },
    tooltip: {
      intersect: true,
      shared: false,
      followCursor: true,
      onDatasetHover: {
        highlightDataSeries: true,
      },
      x: {
        show: false,
      },
    },
    grid: {
      show: true,
      xaxis: {
        lines: {
          show: true,
        },
      },
      yaxis: {
        lines: {
          show: true,
        },
      },
    },
    legend: {
      position: "right",
      showForSingleSeries: true,
      markers: {
        width: 20,
        radius: 2,
      },
      offsetX: -40,
      offsetY: 40,
      itemMargin: {
        horizontal: 5,
        vertical: 0,
      },
      labels: {
        colors: [theme.palette.mode === "light" ? "#212121" : "#FFFFFF"],
      },
    },
    markers: {
      size: 5,
    },
    title: {
      align: "center",
      offsetX: -30,
      style: {
        color: theme.palette.mode === "light" ? "#212121" : "#FFF",
        fontSize: "20px",
      },
    },
  };
}