// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
import {
    CircularProgress,
    Container,
    Link,
    MenuItem,
    Paper,
    Select,
    Table,
    TableBody,
    TableCell,
    TableContainer,
    TableRow,
    Typography,
  } from "@mui/material";
  import moment from "moment";
  import * as React from "react";
  import { TRANSACTION_PER_MINUTE } from "../../common/Constant";
  import { usePageEffect } from "../../core/page";
  import { PerformanceTable } from "../../core/table";
  import { UseCaseData } from "./data";
  import { GetLatestPerformanceReports, GetServiceLatestVersion, GetServicePRInformation } from "./service";
  
  export default function PerformanceReport(): JSX.Element {
    usePageEffect({ title: "Amazon CloudWatch Agent" });
  
    const [{ version, commit_date, commit_title, commit_url, use_cases, ami_id, collection_period }, ] = useStatePerformanceReport();
    const [{ data_type }, setDataTypeState] = useStateDataType();
  
    return !version || !commit_title ? (
      <Container sx={{ display: "flex", justifyContent: "center", alignItems: "center", height: "100vh" }}>
        <CircularProgress color="inherit" />
      </Container>
    ) : (
      <Container sx={{ py: "5vh", border: "1px solid" }} maxWidth="lg">
        <Container sx={{ mb: 4 }}>
          <Typography sx={{ mb: 2, fontWeight: "bold" }} variant="h2">
            Performance Report
            <hr />
          </Typography>
        </Container>
        <Container sx={{ mb: 4 }}>
          <TableContainer sx={{ mb: 4, display: "flex", justifyContent: "center", boxShadow: "unset" }} component={Paper}>
            <Table sx={{ borderStyle: "solid", width: "fit-content" }} size="small" aria-label="a dense table">
              <TableBody>
                {["Version", "Architectural", "Collection Period", "Testing AMI", "Commit Name", "Commit Date", "Data type"]?.map((name) => (
                  <TableRow key={name}>
                    <TableCell sx={{ border: "1px solid #000", fontWeight: "bold" }}>{name}</TableCell>
                    <TableCell sx={{ border: "1px solid #000", textAlign: "center" }}>
                      {name === "Version" ? (
                        <Link href={`https://github.com/aws/amazon-cloudwatch-agent/releases/tag/${version}`}>{version}</Link>
                      ) : name === "Architectural" ? (
                        <Typography variant="h4">EC2</Typography>
                      ) : name === "Collection Period" ? (
                        <Typography variant="h4">{collection_period}s</Typography>
                      ) : name === "Testing AMI" ? (
                        <Typography variant="h4">{ami_id}</Typography>
                      ) : name === "Commit Name" ? (
                        <Link href={commit_url} variant="h4">
                          {commit_title}
                        </Link>
                      ) : name === "Commit Date" ? (
                        <Typography variant="h4">{commit_date}</Typography>
                      ) : (
                        <Select
                          sx={{ height: "41px" }}
                          value={data_type}
                          onChange={(e: { target: { value: string } }) => setDataTypeState({ data_type: e.target.value })}
                        >
                          <MenuItem value={"Metrics"}>Metric</MenuItem>
                          <MenuItem value={"Traces"}>Trace</MenuItem>
                          <MenuItem value={"Logs"}>Logs</MenuItem>
                        </Select>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
          <hr />
        </Container>
        {TRANSACTION_PER_MINUTE.map((tpm) => (
          <Container key={tpm}>
            <Typography sx={{ mb: 2, fontWeight: "bold" }} variant="h3">
              {data_type} (TPM: {tpm}){" "}
            </Typography>
            <PerformanceTable
              data_rate={String(tpm)}
              use_cases={use_cases.filter((use_case: UseCaseData) => use_case?.data_type === data_type.toLowerCase())}
            />
          </Container>
        ))}
      </Container>
    );
  }
  
  function useStatePerformanceReport() {
    const [state, setState] = React.useState({
      version: undefined as string | undefined,
      commit_url: undefined as string | undefined,
      commit_date: undefined as string | undefined,
      commit_title: undefined as string | undefined,
      use_cases: [] as UseCaseData[] ,
      ami_id: undefined as string | undefined,
      collection_period: undefined as string | undefined,
      error: undefined as string | undefined,
    });
  
    React.useEffect(() => {
      (async () => {
        var [service_info, performance_reports] = await Promise.all([GetServiceLatestVersion(), GetLatestPerformanceReports()]);
        if (service_info == null || performance_reports == null || performance_reports.length === 0) {
          return;
        }
  
        let use_cases: UseCaseData[] = [];
        // We only get the latest commit ID; therefore, only use case are different; however, general metadata
        // information (e.g Commit_Hash, Commit_Date of the PR) would be the same for all datas.
        const commit_hash = performance_reports.at(0)?.CommitHash.S || "";
        const commit_date = performance_reports.at(0)?.CommitDate.N;
        const collection_period = performance_reports.at(0)?.CollectionPeriod.S;
        const ami_id = performance_reports.at(0)?.InstanceAMI.S;
        for (let pReport of performance_reports) {
          // Instead of using Max, Min, Std, P99, we would use Avg for every collected metrics
          use_cases.push({
            name: pReport?.UseCase.S,
            data_type: pReport?.DataType.S,
            instance_type: pReport?.InstanceType.S,
            data: TRANSACTION_PER_MINUTE.reduce(
              (accu, tpm) => ({
                ...accu,
                [tpm]: {
                  procstat_cpu_usage: pReport?.Results.M[tpm]?.M?.procstat_cpu_usage?.M?.Average?.N,
                  procstat_memory_rss: pReport?.Results.M[tpm]?.M?.procstat_memory_rss?.M?.Average?.N,
                  procstat_memory_swap: pReport?.Results.M[tpm]?.M?.procstat_memory_swap?.M?.Average?.N,
                  procstat_memory_vms: pReport?.Results.M[tpm]?.M?.procstat_memory_vms?.M?.Average?.N,
                  procstat_memory_data: pReport?.Results.M[tpm]?.M?.procstat_memory_data?.M?.Average?.N,
                  procstat_write_bytes: pReport?.Results.M[tpm]?.M?.procstat_write_bytes?.M?.Average?.N,
                  procstat_num_fds: pReport?.Results.M[tpm]?.M?.procstat_num_fds?.M?.Average?.N,
                  net_bytes_sent: pReport?.Results.M[tpm]?.M?.net_bytes_sent?.M?.Average?.N,
                  net_packets_sent: pReport?.Results.M[tpm]?.M?.net_packets_sent?.M?.Average?.N,
                  mem_total: pReport?.Results.M[tpm]?.M?.mem_total?.M?.Average?.N,
                },
              }),
              {}
            ),
          });
        }
  
        var commit_info = await GetServicePRInformation(commit_hash);
  
        setState((prev: any) => ({
          ...prev,
          version: service_info.tag_name,
          ami_id: ami_id,
          collection_period: collection_period,
          use_cases: use_cases,
          commit_title: `${commit_info.title} (#${commit_info.number})`,
          commit_url: commit_info.html_url,
          commit_date: moment.unix(Number(commit_date)).format("dddd, MMMM Do, YYYY h:mm:ss A"),
        }));
      })();
    }, [setState]);
  
    return [state, setState] as const;
  }
  
  function useStateDataType() {
    const [state, setState] = React.useState({
      data_type: "Metrics" as "Metrics" | "Traces" | "Logs" | string,
    });
  
    return [state, setState] as const;
  }