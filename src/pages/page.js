import React from "react";
import Receiver from "../helpers/reciever";
import { DEFAULT_CONFIG } from "../config";
import "../helpers/graph.css";
//This is the base website page, all website components inherit this page.
export default class Page extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      Receiver: new Receiver("CWAPerformanceMetrics"),
      data: [], //CWAdata
      mounted: false,
      config: JSON.parse(localStorage.getItem("config")) || DEFAULT_CONFIG,
    };
  }
  componentDidMount() {
    if (!this.state.mounted) {
      if (localStorage.getItem("config") == null) {
        localStorage.setItem("config", JSON.stringify(this.state.config));
      }
      this.state.Receiver.update().then(() => {
        this.setState({ data: this.state.Receiver.CWAData });
      });
    }
    this.setState({ mounted: true });
  }
  updateConfig() {
    this.setState({
      config: JSON.parse(localStorage.getItem("config")) || DEFAULT_CONFIG,
    });
  }
  render() {
    setGlobalCSSVars(this.state.config);
  }
}

function setGlobalCSSVars(props) {
  document.body.style.setProperty("--fontSize", `${props.textFontSize}px`);
  document.body.style.setProperty(
    "--h3fontSize",
    `${parseInt(props.textFontSize) + 4}px`
  );
  document.body.style.setProperty(
    "--h2fontSize",
    `${parseInt(props.textFontSize) + 8}px`
  );
  document.body.style.setProperty(
    "--tableFontSize",
    `${parseInt(props.tableFontSize)}px`
  );
  document.body.style.setProperty(
    "--headTableFontSize",
    `${parseInt(props.tablefontSize) + 4}px`
  );
  document.body.style.setProperty(
    "--textFontSize",
    `${parseInt(props.textFontSize)}px`
  );
  document.body.style.setProperty(
    "--headTextFontSize",
    `${parseInt(props.textFontSize) + 4}px`
  );
  document.body.style.setProperty(
    "--tablefontSize",
    `${parseInt(props.tableFontSize)}px`
  );
}
