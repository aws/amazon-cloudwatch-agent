import Page from "./page"
import './homepage.css'
import Navbar from '../helpers/navbar';
//This webpage provides information about the project
export default class Home extends Page {
    render() {
        document.body.style.setProperty("--fontSize",parseInt(this.state.config.textFontSize).toString()+"px")
        document.body.style.setProperty("--h3fontSize",(parseInt(this.state.config.textFontSize)+4).toString()+"px")
        document.body.style.setProperty("--h2fontSize",(parseInt(this.state.config.textFontSize)+8).toString()+"px")
        return (
            <div class="homepage">
                <Navbar/>
                <h2>CloudWatch Agent Performance Metrics</h2>
                <section>
                    <h3>About CloudWatch Agent Performance Tracking</h3>
                    <p>
                        The <a href="https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Install-CloudWatch-Agent.html"
                            target="_blank" rel="noreferrer">CloudWatch Agent</a> performance tracker provides
                        data on the resource usage of the CloudWatch Agent.
                        It is currently designed and configured to be run on
                        Amazon Linux on an EC2 instance. The aim of this tracker is to
                        provide information on expected resource usage of
                        the CloudWatch Agent so resources can be accurately predicted/allocated
                        to the CloudWatch Agent.
                        <div><br></br></div>
                        To obtain this benchmarking data, an EC2 instance is started and CloudWatch Agent is 
                        installed on the host. A configuration file is generated for the agent to use with a specified 
                        number of logs monitored. When the test begins, the agent is started and lines are written to 
                        each log file monitored in the config. The lines are written at a specified rate for the 
                        given test. While the agent is running, it is also monitoring it's own resource usage so it can 
                        report this data. These metrics are pulled from CloudWatch and saved as the benchmarking data used 
                        on this website. 
                    </p>
                </section>
                <section>
                    <h3>Need Assistance?</h3>
                    <p>
                        Read our FAQ below
                        <div><br></br></div>
                        Need further assistance? Let us know about your issue 
                        by opening an issue on github: <a href="https://github.com/aws/amazon-cloudwatch-agent/issues/new/choose" target="_blank" rel="noreferrer">here</a>
                        <div>
                            <br></br>
                        </div>
                        Please include "CloudWatch Agent Performance Tracker" in the title of the issue.
                    </p>
                </section>
                <section>
                    <h3>Intended Use</h3>
                    <p>
                        With this data, CloudWatch agent customers can get a better idea of how the CloudWatch Agent performs under different loads.
                        Since several tests are run with different load cases, customers can select a test that best represents a use
                        case similar to theirs, and view the benchmarked resources used by CloudWatch Agent with that simulated 
                        configuration and load. This data should provide insights into how efficiently the CloudWatch Agent can be expected 
                        to run under a given load.
                        <div><br></br></div>
                        Currently, the performance benchmarking collects the CloudWatch Agent's CPU usage 
                        and <a href="https://en.wikipedia.org/wiki/Resident_set_size" target="_blank" rel="noreferrer">Resident Set Size (RSS) memory</a>.
                    </p>
                </section>
                <section style={{"margin-bottom": "3%"}}>
                    <h3> FAQ</h3>
                    <p>
                        Q: Graphs or Table are not updated when I refresh? 
                        <br/>A: Hit this button <button
                        title="Click"
                        style={{width:"15%",height:"24px"}}
                         onClick={()=>{
                             this.state.Receiver.cacheClear()
                         }}>Clear Cache</button>
                    </p>
                    <p>
                        Q: How can I update table and graph data and recieve most recent commits
                        <br/>A: Refresh the page, if it still doesn't work revert to previous FAQ.
                    </p>
                </section>
            </div>

        );
    }
}