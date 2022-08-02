import React from 'react';
import Receiver from "../helpers/reciever"
import { DEFAULT_CONFIG } from "../config"
import "../helpers/graph.css"
//This is the base website page, all website components inherit this page.
export default class Page extends React.Component {
  constructor(props) {
    super(props)
    this.state = {
      Receiver: new Receiver("CWAPerformanceMetrics"),
      data: [], //CWAdata
      mounted: false,
      config: JSON.parse(localStorage.getItem("config")) || DEFAULT_CONFIG,
    }
  }
  componentDidMount() {
    console.log("mounted")
    if (!this.state.mounted) {
      // debugger;
      if (localStorage.getItem("config") == null) {
        localStorage.setItem("config", JSON.stringify(this.state.config))
      }
      this.state.Receiver.update().then(() => {
        this.setState({ data: this.state.Receiver.CWAData })

      })
    }
    this.setState({ mounted: true })
  }
  updateConfig() {
    this.setState({ config: JSON.parse(localStorage.getItem("config")) || DEFAULT_CONFIG })
  }
}

