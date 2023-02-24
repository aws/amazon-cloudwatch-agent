import Paper from "@mui/material/Paper";
import { styled } from "@mui/material/styles";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell, { tableCellClasses } from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import { UseCaseData } from "../containers/PerformanceReport/data.js";

const StyledTableCell = styled(TableCell)(({ theme }) => ({
  [`&.${tableCellClasses.head}`]: {
    backgroundColor: theme.palette.common.black,
    color: theme.palette.common.white,
    border: "1px solid #000",
    textAlign: "center",
  },
  [`&.${tableCellClasses.body}`]: {
    fontSize: 14,
    border: "1px solid #000",
    textAlign: "center",
  },
}));

const StyledTableRow = styled(TableRow)(({ theme }) => ({
  "&:nth-of-type(odd)": {
    backgroundColor: theme.palette.action.hover,
  },
  // hide last border
  "&:last-child td, &:last-child th": {},
}));

export function PerformanceTable(props: { use_cases: UseCaseData[]; data_rate: string }): JSX.Element {
  const { use_cases, data_rate } = props;
  return (
    <TableContainer sx={{ mb: 4 }} component={Paper}>
      <Table sx={{ borderStyle: "solid" }} size="small" aria-label="a dense table">
        <TableHead>
          <TableRow>
            <StyledTableCell width="50vw" align="center" sx={{ fontWeight: "bold", whiteSpace: "nowrap" }}>
              Use Case
            </StyledTableCell>
            <StyledTableCell width="30%" align="center" sx={{ border: "1px solid #000", fontWeight: "bold", whiteSpace: "nowrap" }}>
              Instance Type
            </StyledTableCell>
            <StyledTableCell width="50%" align="center" sx={{ border: "1px solid #000", fontWeight: "bold", whiteSpace: "nowrap" }}>
              Avg CPU Usage (%)
            </StyledTableCell>
            <StyledTableCell width="30%" align="center" sx={{ border: "1px solid #000", fontWeight: "bold", whiteSpace: "nowrap" }}>
              Avg Memory Usage (%)
            </StyledTableCell>
            <StyledTableCell width="30%" align="center" sx={{ border: "1px solid #000", fontWeight: "bold", whiteSpace: "nowrap" }}>
              Avg Memory Swap (%)
            </StyledTableCell>
            <StyledTableCell width="30%" align="center" sx={{ border: "1px solid #000", fontWeight: "bold", whiteSpace: "nowrap" }}>
              Avg Memory Data (%)
            </StyledTableCell>
            <StyledTableCell width="30%" align="center" sx={{ border: "1px solid #000", fontWeight: "bold", whiteSpace: "nowrap" }}>
              Avg Virtual Memory (%)
            </StyledTableCell>
            <StyledTableCell width="30%" align="center" sx={{ border: "1px solid #000", fontWeight: "bold", whiteSpace: "nowrap" }}>
              Avg Write Disk Bytes (MB)
            </StyledTableCell>
            <StyledTableCell width="30%" align="center" sx={{ border: "1px solid #000", fontWeight: "bold", whiteSpace: "nowrap" }}>
              File Descriptors
            </StyledTableCell>
            <StyledTableCell width="30%" sx={{ border: "1px solid #000", fontWeight: "bold", whiteSpace: "nowrap" }}>
              Avg Net Bytes Sent (MB)
            </StyledTableCell>
            <StyledTableCell width="30%" sx={{ border: "1px solid #000", fontWeight: "bold", whiteSpace: "nowrap" }}>
              Avg Net Packages Sent (MB)
            </StyledTableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {use_cases?.map((use_case) => (
            <StyledTableRow key={use_case.name}>
              <StyledTableCell>{use_case.name}</StyledTableCell>
              <StyledTableCell>{use_case.instance_type}</StyledTableCell>
              <StyledTableCell>{Number(use_case.data?.[data_rate]?.procstat_cpu_usage).toFixed(2)}</StyledTableCell>
              <StyledTableCell>
                {(Number(use_case.data?.[data_rate]?.procstat_memory_rss) / Number(use_case.data?.[data_rate]?.mem_total)).toFixed(2)}
              </StyledTableCell>
              <StyledTableCell>
                {(Number(use_case.data?.[data_rate]?.procstat_memory_swap) / Number(use_case.data?.[data_rate]?.mem_total)).toFixed(2)}
              </StyledTableCell>
              <StyledTableCell>
                {(Number(use_case.data?.[data_rate]?.procstat_memory_data) / Number(use_case.data?.[data_rate]?.mem_total)).toFixed(2)}
              </StyledTableCell>
              <StyledTableCell>
                {(Number(use_case.data?.[data_rate]?.procstat_memory_vms) / Number(use_case.data?.[data_rate]?.mem_total)).toFixed(2)}
              </StyledTableCell>
              <StyledTableCell>{Number(use_case.data?.[data_rate]?.procstat_write_bytes).toFixed(2)}</StyledTableCell>
              <StyledTableCell>{Number(use_case.data?.[data_rate]?.procstat_num_fds).toFixed(2)}</StyledTableCell>
              <StyledTableCell>{Number(use_case.data?.[data_rate]?.net_bytes_sent).toFixed(2)}</StyledTableCell>
              <StyledTableCell>{Number(use_case.data?.[data_rate]?.net_packets_sent).toFixed(2)}</StyledTableCell>
            </StyledTableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
}