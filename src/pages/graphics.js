import Page from "./page";
import Grapher from "../helpers/grapher";
import Navbar from "../helpers/navbar";
import Setting, { MetricSettingsBox } from "./settings";
import "../helpers/graph.css";
import { BsFillCircleFill, BsSuitDiamondFill } from "react-icons/bs";
//This webpage displays metrics graphs relative to the hashes
export default class GraphicsPage extends Page {
  render() {
    document.body.style.setProperty(
      "--textFontSize",
      parseInt(this.state.config.textFontSize).toString() + "px"
    );
    document.body.style.setProperty(
      "--headTextFontSize",
      (parseInt(this.state.config.textFontSize) + 4).toString() + "px"
    );

    return (
      <div className="GraphicsPage">
        <Navbar />
        <div class="page_container">
          <div class="graph_content">
            <div class="header">
              <h2>Graphs</h2>
              <p>
                In here you can see metrics vs commit hashes on a line graph.
                You can turn off some statistics by clicking on them on the
                legend. If a statistic is off it will be show with a{" "}
                {<BsSuitDiamondFill />}(OFF) while if it is on it will be shown
                with a {<BsFillCircleFill />}(ON). Next, if you hover over a
                point on the graph a tooltip will be show with more statistics;
                this tooltip will disappear when you move away. In addition,{" "}
                <strong style={{ color: "#8B008B" }}>
                  vertical purple coloured lines{" "}
                </strong>{" "}
                represent releases of cloudwatch agent. Lastly,in this webpage
                we have multiple settings that can be configured. These settings
                can be configured from the right hand-side. Supported settings
                are as following:
                <br />
                <br />
                <ul>
                  <li>
                    Significant Figure: Adjusts number of significant figures
                    for the data.
                  </li>
                  <li>
                    Text Font Size: Changes the font size of text on the screen
                    such as this one.
                  </li>
                  <li>
                    Graph Font Size: Changes the font size of the text located
                    inside the graph.
                  </li>
                  <li>
                    Graph Size: Adjusts the dimensions of the graph increasing
                    width and height.
                  </li>
                  <li>
                    Number of Commits: This lets you adjust last number of
                    commits you can see
                  </li>
                </ul>
              </p>
            </div>
            <Grapher data={this.state.data} config={this.state.config} />
          </div>
          <div class="graph_settings">
            <div class="settings_page">
              <div class="title">
                <h2>Settings</h2>
              </div>
              <br></br>
              <div class="setting_box">
                <Setting
                  title="Significant Figure"
                  settingKey="sigfig"
                  type="select"
                  range={[2, 8, 1, ""]}
                  page={this}
                />
                <Setting
                  title="Text Font Size"
                  settingKey="textFontSize"
                  type="select"
                  range={[8, 32, 4, "px"]}
                  page={this}
                />
                <Setting
                  title="Graph Font Size"
                  settingKey="graphFontSize"
                  type="select"
                  range={[8, 32, 4, "px"]}
                  page={this}
                />
                <Setting
                  title="Graph Size"
                  settingKey="graphSize"
                  type="select"
                  range={[1, 6, 1, ""]}
                  page={this}
                />
                <Setting
                  title="Number of Last Commits"
                  settingKey="nLastCommits"
                  type="select"
                  range={[10, 50, 5, " commits"]}
                  page={this}
                />
              </div>
              <br></br>
              <MetricSettingsBox data={this.state.data} page={this} />
            </div>
          </div>
        </div>
      </div>
    );
  }
}
