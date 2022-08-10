import React from "react";
import Snackbar from "@mui/material/Snackbar";
import MuiAlert from "@mui/material/Alert";
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
      synced: false,
      error: ["error", ""], //"errorType":["error,"warning,"info","success"],"errormsg"
    };
  }
  componentDidMount() {
    if (!this.state.mounted) {
      if (localStorage.getItem("config") == null) {
        localStorage.setItem("config", JSON.stringify(this.state.config));
      }
      this.state.Receiver.update().then((updateState) => {
        this.setState({
          data: this.state.Receiver.CWAData,
          synced: updateState[0],
          error: ["error", updateState[1]],
        });
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
    return <div></div>;
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
// This component creates a snack bar alert if errorMsg is not ""
export function ErrorHandler(props) {
  var errorType = props.error[0];
  var errorMsg = props.error[1];
  return (
    <div>
      <Snackbar open={props.error !== null && errorMsg !== ""}>
        <MuiAlert severity={errorType}>
          {errorType.toUpperCase()}: {errorMsg}
        </MuiAlert>
      </Snackbar>
    </div>
  );
}
