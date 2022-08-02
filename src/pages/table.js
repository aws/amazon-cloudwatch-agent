import Page from "./page"
import Navbar from '../helpers/navbar'
import TableGroup from '../helpers/BasicTable'
import Setting from './settings'
import '../helpers/table.css'


//This the webpage that contains the metric tables.
export default class TablePage extends Page {

    render() {
        document.body.style.setProperty("--tableFontSize", (parseInt(this.state.config.tableFontSize)).toString() + "px")
        document.body.style.setProperty("--headTableFontSize", (parseInt(this.state.config.tablefontSize) + 4).toString() + "px")
        document.body.style.setProperty("--textFontSize", (parseInt(this.state.config.textFontSize)).toString() + "px")
        document.body.style.setProperty("--headTextFontSize", (parseInt(this.state.config.textFontSize) + 4).toString() + "px")


        return (
            <div className="table_page">
                <Navbar />
                <div class="page_container">
                    <div class="table_content">
                        <div class="header">
                            <h2>Table Page</h2>
                            <p>
                                In here you can see metrics and their statistics in a table format.
                                Official agent releases are highlighted by coloring the release commit row purple.
                                In this webpage we have multiple settings that can be configured.
                                These settings can be configured from the right hand-side. Supported settings are as following:
                                <br />
                                <br />
                                <ul>
                                    <li>Significant Figure: Adjusts number of significant figures for the data.</li>
                                    <li>Text Font Size: Changes the font size of text on the screen such as this one.</li>
                                    <li>Table Font Size: Changes the font size of the text located inside the graph.</li>
                                </ul>
                            </p>
                        </div>
                        <TableGroup data={this.state.data} config={this.state.config} />
                    </div>
                    <div class="table_settings">
                        <div class="settings_page">
                            <div class="title">
                                <h2>Settings</h2>
                            </div>
                            <br></br>
                            <div class="setting_box">
                                <Setting title="Significant Figure" settingKey="sigfig" type="select" range={[2, 8, 1, '']} page={this} />
                                <Setting title="Text Font Size" settingKey="textFontSize" type="select" range={[8, 32, 4, 'px']} page={this} />
                                <Setting title="Table Font Size" settingKey="tableFontSize" type="select" range={[8, 32, 4, 'px']} page={this} />
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        );
    }
}
