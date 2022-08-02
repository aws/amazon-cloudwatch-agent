import Table from "@material-ui/core/Table";
import TableBody from "@material-ui/core/TableBody";
import TableCell from "@material-ui/core/TableCell";
import TableContainer from "@material-ui/core/TableContainer";
import TableHead from "@material-ui/core/TableHead";
import TableRow from "@material-ui/core/TableRow";
import Paper from "@material-ui/core/Paper";
import Container from "@material-ui/core/Container";
import * as React from 'react';
import PropTypes from 'prop-types';
import { useTheme } from '@mui/material/styles';
import Box from '@mui/material/Box';
import TableFooter from '@mui/material/TableFooter';
import TablePagination from '@mui/material/TablePagination';
import IconButton from '@mui/material/IconButton';
import FirstPageIcon from '@mui/icons-material/FirstPage';
import KeyboardArrowLeft from '@mui/icons-material/KeyboardArrowLeft';
import KeyboardArrowRight from '@mui/icons-material/KeyboardArrowRight';
import LastPageIcon from '@mui/icons-material/LastPage';
import "./table.css"
import { IGNORE_ATTRIBUTES, UNITS, TEST_VARIABLES } from "../config"
//@TODO ADD UNITS TO TABLE HEADERS
const MIN_COLOUR = 0x50


function getRandomColour(idx) {
    var colours = [MIN_COLOUR, MIN_COLOUR, MIN_COLOUR]
    colours[idx % 2] = 0xCC
    colours[idx % 3] = 0xCC
    var stringColour = "#"
    for (var i = 0; i < colours.length; i++) {
        stringColour += colours[i].toString(16)
    }

    return stringColour
}
function TablePaginationActions(props) {
    const theme = useTheme();
    const { count, page, rowsPerPage, onPageChange } = props;

    const handleFirstPageButtonClick = (event) => {
        onPageChange(event, 0);
    };

    const handleBackButtonClick = (event) => {
        onPageChange(event, page - 1);
    };

    const handleNextButtonClick = (event) => {
        onPageChange(event, page + 1);
    };

    const handleLastPageButtonClick = (event) => {
        onPageChange(event, Math.max(0, Math.ceil(count / rowsPerPage) - 1));
    };



    return (
        <Box sx={{ flexShrink: 0, ml: 2.5 }}>
            <IconButton
                onClick={handleFirstPageButtonClick}
                disabled={page === 0}
                aria-label="first page"
            >
                {theme.direction === 'rtl' ? <LastPageIcon /> : <FirstPageIcon />}
            </IconButton>
            <IconButton
                onClick={handleBackButtonClick}
                disabled={page === 0}
                aria-label="previous page"
            >
                {theme.direction === 'rtl' ? <KeyboardArrowRight /> : <KeyboardArrowLeft />}
            </IconButton>
            <IconButton
                onClick={handleNextButtonClick}
                disabled={page >= Math.ceil(count / rowsPerPage) - 1}
                aria-label="next page"
            >
                {theme.direction === 'rtl' ? <KeyboardArrowLeft /> : <KeyboardArrowRight />}
            </IconButton>
            <IconButton
                onClick={handleLastPageButtonClick}
                disabled={page >= Math.ceil(count / rowsPerPage) - 1}
                aria-label="last page"
            >
                {theme.direction === 'rtl' ? <FirstPageIcon /> : <LastPageIcon />}
            </IconButton>
        </Box>
    );
}

TablePaginationActions.propTypes = {
    count: PropTypes.number.isRequired,
    onPageChange: PropTypes.func.isRequired,
    page: PropTypes.number.isRequired,
    rowsPerPage: PropTypes.number.isRequired,
};


function CreateRow(props, sigfig, currentTestCase, index, rowLength) {

    var line = []
    var testMetrics = props.data[Object.keys(props.data)[0]]
    var currMetric = testMetrics[props.metric]
    if (index >= currMetric.length) {
        return []
    }

    var currEntry = currMetric[index]

    line.push(<TableCell class="cell_text"><a href={currEntry["Link"]} target="_blank" rel="noreferrer">
        {currEntry["Hash"]}</a></TableCell>);

    var date = new Date(currEntry["CommitDate"] * 1000)
    line.push(<TableCell class="cell_text">{date.toUTCString()}</TableCell>);

    if (currentTestCase.split("-")[1] === "all") {
        let testCaseList = Object.keys(props.data)
        let selectedLogNum = currentTestCase.split("-")[0]
        for (let i = 0; i < testCaseList.length; i++) {
            let testCaseOption = testCaseList[i].split("-")
            if (testCaseOption[0] === selectedLogNum) {
                let testMetric = props.data[testCaseList[i]][props.metric]
                for (let metric in testMetric[index]) {
                    if (IGNORE_ATTRIBUTES.includes(metric) || metric === "Period") {
                        continue;
                    }
                    line.push(<TableCell class="cell_text">{testMetric[index][metric].toPrecision(sigfig)}</TableCell>)
                }
            }
        }
    } else {
        let testMetric = props.data[currentTestCase][props.metric]
        //add line with contained data
        for (let metric in testMetric[index]) {
            if (IGNORE_ATTRIBUTES.includes(metric) || metric === "Period") {
                continue;
            }
            line.push(<TableCell class="cell_text">{testMetric[index][metric].toPrecision(sigfig)}</TableCell>)
        }
    }

    //fill in empty cells to ensure line is same length as table
    while (line.length < rowLength) {
        line.push(<TableCell class="cell_text"></TableCell>)
    }
    return line;
}

export function BasicTable(props) {

    document.body.style.setProperty("--tablefontSize", parseInt(props.config.tableFontSize).toString() + "px")
    var metricNames = []
    var sigfig = parseInt(props.config.sigfig)
    metricNames.push(<TableCell class="cell_text head">{"Hash"}</TableCell>)
    metricNames.push(<TableCell class="cell_text head">{"CommitDate"}</TableCell>)


    const [page, setPage] = React.useState(0);
    const [rowsPerPage, setRowsPerPage] = React.useState(5);

    var testCases = Object.keys(props.data)
    var testVariables = Array((testCases[0].split("-")).length)
    
    testCases.forEach((test) => {
        test.split("-").forEach((value, idx) => {
            if (testVariables[idx] === undefined) {
                testVariables[idx] = []
            }

            if (!testVariables[idx].includes(value)) {
                testVariables[idx].push(value)
                testVariables[idx].sort()
            }
        })
    })
    var buttons = []
    const [currentTestCase, setCurrentTest] = React.useState(testCases[0])
    

    const handleChangePage = (event, newPage) => {
        setPage(newPage);
    };

    const handleChangeRowsPerPage = (event) => {
        setRowsPerPage(parseInt(event.target.value, 10));
        setPage(0);
    };


    var testMetrics = props.data[Object.keys(props.data)[0]]
    var currMetric = testMetrics[props.metric]
    
    // Avoid a layout jump when reaching the last page with empty rows.
    const emptyRows = 
        page > 0 ? Math.max(0, (1 + page) * rowsPerPage - currMetric.length) : 0;
    

    testVariables.forEach((varSet, i) => {

        var options = []

        varSet.forEach((value) => {
            options.push(<option>{value}</option>)
        })

        if (TEST_VARIABLES[i] === "TPS") {
            options.push(<option>all</option>)
        }

        buttons.push(
            <div class="select_box">
                <label>{TEST_VARIABLES[i]}</label>
                <select id={`testCase${props.title}-${i}`}
                    onChange={() => {
                        var testCase = ""
                        var n_variables = testVariables.length
                        for (var j = 0; j < n_variables; j++) {
                            testCase += document.getElementById(`testCase${props.title}-${j}`).value
                            if (j < n_variables - 1) {
                                testCase += "-"
                            }
                        }
                        setCurrentTest(testCase)                
                    }}
                >{options}</select>
            </div>)
    })

    var columnHeaders = currentTestCase.split("-")[1] === "all" ? 3 : 1
    for (let i = 0; i < columnHeaders; i++) {
        for (var metric in currMetric[0]) {

            if (IGNORE_ATTRIBUTES.includes(metric) || metric === "Period") {
                continue;
            }

            metricNames.push(<TableCell class="cell_text head">{`${metric} (${UNITS[props.metric]})`}</TableCell>)
        }
    }

    var labelHeaderClass = columnHeaders === 3 ? "table_labels" : "table_labels_hidden"

    var rows = []

    for (let i = page * rowsPerPage; i < page * rowsPerPage + rowsPerPage; i++) {
        if (i >= currMetric.length) {
            break
        }

        let line = CreateRow(props, sigfig, currentTestCase, i, metricNames.length)

        if (currMetric[i].isRelease) {
            rows.push(<TableRow style={{ background: "#896799" }}>{line}</TableRow>)
        } else {
            rows.push(<TableRow style={i % 2 ? { background: "#dddddd" } : { background: "white" }}>{line}</TableRow>)
        }
    }

    return (
        <Container class="table_container">
            <h2>{props.title}</h2>
            <div class="button_container">
                {buttons}
            </div>
            <TableContainer component={Paper}>
                <Table aria-label="data table" class="table">

                    <TableHead>
                        <TableRow class={labelHeaderClass}>
                            <TableCell align="center" colSpan={2}>
                                Agent Info
                            </TableCell>
                            <TableCell align="center" colSpan={5}>
                                Low
                            </TableCell>
                            <TableCell align="center" colSpan={5}>
                                Medium
                            </TableCell>
                            <TableCell align="center" colSpan={5}>
                                High
                            </TableCell>
                        </TableRow>
                        <TableRow style={{ background: getRandomColour(props.idx), "textAlign": "center" }}>
                            {metricNames}
                        </TableRow>
                    </TableHead>
                    <TableBody>
                        {rows}
                        {emptyRows > 0 && (
                            <TableRow style={{ height: 53 * emptyRows }}>
                                <TableCell colSpan={6} />
                            </TableRow>
                        )}
                    </TableBody>
                    <TableFooter class="table_footer">
                        <TableRow>
                            <TablePagination
                                rowsPerPageOptions={[5, 10, 25]}
                                colSpan={metricNames.length}
                                count={currMetric.length}
                                rowsPerPage={rowsPerPage}
                                page={page}
                                SelectProps={{
                                    inputProps: {
                                        'aria-label': 'rows per page',
                                    },
                                    native: true,
                                }}
                                onPageChange={handleChangePage}
                                onRowsPerPageChange={handleChangeRowsPerPage}
                                ActionsComponent={TablePaginationActions}
                            />
                        </TableRow>
                    </TableFooter>
                </Table>
            </TableContainer>
        </Container>
    )
}


export default function TableGroup(props) {

    if (props.data === undefined) {
        return
    }

    var testCase = props.data[Object.keys(props.data)[0]]
    if (testCase === undefined) {
        return

    }
    var metrics = Object.keys(testCase)
    var tables = []
    var j = 0;
    metrics.forEach(element => {
        tables.push(<BasicTable metric={element} data={props.data} idx={j} title={element}
            config={props.config} />)

        j++;
    })
    return (
        <div id="TableArea" class="table_group">
            {tables}
        </div>
    )

}
