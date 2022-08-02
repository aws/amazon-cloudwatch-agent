import Navbar from "../helpers/navbar";
import Page from "./page"
import "./wiki.css"
import {
    ScrollingProvider,
    useScrollSections,
    Section,
} from 'react-scroll-section';

export const DynamicMenu = () => {
    const Sections = useScrollSections();

    return (
        <ul>
            {Sections.map(({ id, onClick, selected }) => (
                <li key={id} onClick={onClick} selected={selected}>
                    {id.toUpperCase()}
                </li>
            ))}
        </ul>
    );
};
export default class WikiPage extends Page {
    render() {
        document.body.style.setProperty("--fontSize", parseInt(this.state.config.textFontSize).toString() + "px")
        document.body.style.setProperty("--h3fontSize", (parseInt(this.state.config.textFontSize) + 4).toString() + "px")
        document.body.style.setProperty("--h2fontSize", (parseInt(this.state.config.textFontSize) + 8).toString() + "px")
        return (
            <div class="wiki_page">
                <Navbar />
                <h2>WikiPage</h2>
                <ScrollingProvider>
                    <div class="container">
                        <div class="index static">
                            <div class="static">
                            <h3>Index</h3>
                            <DynamicMenu />
                            </div>
                        </div>
                        <div class="content">
                            <Section id="Webpages Explained">
                                <h3>Webpages Explained</h3>
                                <h4>Home/Landing Page</h4>
                                <p>
                                    As the entry point to the website, this page gives an overview of the information
                                    the website provides. It also contains some troubleshooting and a way to contact
                                    our team should there be a problem with the website.
                                </p>
                                <h4>Table</h4>
                                <p>
                                    The table webpage contains statistics on the data gathered during the performance test.
                                    Tables are produced for every metric collected on the agent during the benchmark.
                                    Rows are sorted by commit date, meaning that the most recent agent commits are at the top.
                                    Full agent releases are highlighted for convenience. The table also has options to change
                                    the test load so all available data can be explored.
                                </p>
                                <p>
                                    Settings: <br />
                                    <ul>
                                        <li>Significant Figure: change the number of significant digits shown to the specified number</li>
                                        <li>Text Font Size: change the font size of all the non-table text elements on the webpage</li>
                                        <li>Table Font Size: change the font size of all the table text elements</li>
                                    </ul>
                                </p>
                                <h4>Graph</h4>
                                <p>
                                    The graph webage provides visualizations of the collected data on the CloudWatch Agent
                                    benchmark. Graphs are produced for every metric collected on the agent during the benchmark.
                                    The x-axis shows commit hashes, and the hashes are sorted by time so the most recent hash is
                                    furthest right.
                                </p>
                                <p>
                                    Settings: <br />
                                    <ul>
                                        <li>Significant Figure: change the number of significant digits shown to the specified number</li>
                                        <li>Text Font Size: change the font size of all the non-graph text elements on the webpage</li>
                                        <li>Graph Font Size: change the font size of all the graph text elements</li>
                                        <li>Number of Last Commits: change the number of (most recent) commits shown on the graph</li>
                                        <div><br></br></div>
                                        <li>Metric Settings: change the value of the threshold bar on each graph</li>
                                    </ul>
                                </p>
                            </Section>
                            <Section id="Data Addition">
                                <h3>How to add data</h3>
                                <p>
                                    Data is automatically generated for a commit when the commit is merged into the CloudWatch Agent Github repository.
                                    The repository has an integration test that runs on any merge to master that handles benchmarking the agent.
                                    When the test is ran, an EC2 instance is started and the CloudWatch Agent is installed to that host.
                                    Different config files are generated with different amounts of log files monitored for the agent to use during tests.
                                    The agent is run with several different loads, and the metrics collected on it are pushed to a database.
                                    This website fetches that data and displays it via tables and graphs.
                                </p>
                                <ul>

                                </ul>
                                <p>

                                </p>
                            </Section>
                            <Section id="Test Enviorment">
                                <h3>Test Environment</h3>
                                <p>
                                    The performance test is run on an AL2 EC2 instance and the agent resources are monitored via the procstat plugin for
                                    CloudWatch Agent. When the agent is installed on the EC2 instance, a config file is generated for the agent to use. The config
                                    file contains an entry for procstat that the agent uses to gather metrics on itself. The config also contains a 
                                    dynamically generated number of log files that the agent will monitor. When the agent starts, lines are written to each log
                                    file at the rate specified for the given test (TPS).
                                    <br></br>
                                    The resource usage data is automatically uploaded to CloudWatch by the agent where it is pulled
                                    via an API request from the integration test. The integration test processes the data and then stores it in the database.
                                </p>
                            </Section>
                            <Section id="Troubleshooting">
                                <h3>Troubleshooting</h3>
                                <p>
                                    <ul>
                                        <li>Refresh the page</li>
                                        <li>Click the "Clear Cache" button at the bottom of the home page in the FAQ</li>
                                        <li>Clear your browser's cache</li>
                                    </ul>
                                </p>

                            </Section>

                        </div>
                    </div>
                </ScrollingProvider>
            </div>
        );
    }
}