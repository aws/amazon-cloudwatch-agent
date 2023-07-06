// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
import { Box, CircularProgress, Container, MenuItem, Paper, Select, Table, TableBody, TableCell, TableContainer, TableRow, Typography } from '@mui/material';
import { SelectChangeEvent } from '@mui/material/Select';
import { useTheme } from '@mui/material/styles';
import merge from 'lodash/merge';
import moment from 'moment';
import * as React from 'react';
import Chart from 'react-apexcharts';
import { CONVERT_REPORTED_METRICS_NAME, REPORTED_METRICS, TRANSACTION_PER_MINUTE, USE_CASE } from '../../common/Constant';
import { usePageEffect } from '../../core/page';
import { CommitInformation, PerformanceTrendData, ServiceCommitInformation, TrendData } from './data';
import { GetPerformanceTrendData, GetServiceCommitInformation } from './service';
import { PasswordDialog } from '../../common/Dialog';
import { BasedOptionChart } from './styles';

export default function PerformanceTrend(props: { password: string; password_is_set: boolean; set_password_state: any }): JSX.Element {
    usePageEffect({ title: 'Amazon CloudWatch Agent' });
    const theme = useTheme();
    const { password, password_is_set, set_password_state } = props;
    const [{ last_update, hash_categories, trend_data, commits_information }] = useStatePerformanceTrend(password);
    const [{ data_type }, setDataTypeState] = useStateDataType();
    const [{ selected_metrics }, setSelectedMetricsState] = useStateSelectedMetrics();
    const colors = hash_categories.map(() => (theme.palette.mode === 'light' ? '#212121' : '#FFF'));
    return (
        <Container>
            <PasswordDialog password={password} password_is_set={password_is_set} set_password_state={set_password_state} />
            {!last_update ? (
                <Container
                    sx={{
                        display: 'flex',
                        justifyContent: 'center',
                        alignItems: 'center',
                        height: '100vh',
                    }}
                >
                    <CircularProgress color="inherit" />
                </Container>
            ) : (
                <Container sx={{ py: '5vh', border: '1px solid' }} maxWidth="lg">
                    <Container sx={{ mb: 4 }}>
                        <Typography sx={{ mb: 2, fontWeight: 'bold' }} variant="h2">
                            Performance Trend
                            <hr />
                        </Typography>
                    </Container>
                    <Container sx={{ mb: 4 }}>
                        <TableContainer
                            sx={{
                                position: 'relative',
                                mb: 4,
                                display: 'flex',
                                justifyContent: 'center',
                                boxShadow: 'unset',
                            }}
                            component={Paper}
                        >
                            <Table
                                sx={{
                                    borderStyle: 'solid',
                                    width: 'fit-content',
                                    overflow: 'hidden',
                                }}
                                size="small"
                                aria-label="a dense table"
                            >
                                <TableBody>
                                    {['Last Updated', 'Data Type', 'Selected Metric']?.map((name) => (
                                        <TableRow key={name}>
                                            <TableCell
                                                sx={{
                                                    border: '1px solid #000',
                                                    fontWeight: 'bold',
                                                }}
                                            >
                                                {name}
                                            </TableCell>
                                            <TableCell
                                                sx={{
                                                    border: '1px solid #000',
                                                    textAlign: 'center',
                                                }}
                                            >
                                                {name === 'Last Updated' ? (
                                                    <Typography variant="h4">{last_update}</Typography>
                                                ) : name === 'Data Type' ? (
                                                    <Select
                                                        sx={{ height: '38px' }}
                                                        value={data_type}
                                                        onChange={(e: {
                                                            target: {
                                                                value: string;
                                                            };
                                                        }) =>
                                                            setDataTypeState({
                                                                data_type: e.target.value,
                                                            })
                                                        }
                                                    >
                                                        <MenuItem value={'Metrics'}>Metric</MenuItem>
                                                        <MenuItem value={'Logs'}>Logs</MenuItem>
                                                    </Select>
                                                ) : (
                                                    <Select
                                                        multiple
                                                        sx={{
                                                            width: '17vw',
                                                            height: '38px',
                                                        }}
                                                        value={selected_metrics}
                                                        onChange={(event: SelectChangeEvent<string[]>) => {
                                                            const {
                                                                target: { value },
                                                            } = event;
                                                            setSelectedMetricsState({
                                                                selected_metrics: typeof value === 'string' ? value.split(',') : value,
                                                            });
                                                        }}
                                                    >
                                                        {' '}
                                                        {REPORTED_METRICS.map((metric) => (
                                                            <MenuItem key={metric} value={metric}>
                                                                {CONVERT_REPORTED_METRICS_NAME[metric]}
                                                            </MenuItem>
                                                        ))}
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
                    <Container
                        sx={{
                            display: 'flex',
                            flexDirection: 'column',
                            justifyContent: 'center',
                            alignItems: 'center',
                        }}
                    >
                        {REPORTED_METRICS.map((metric) =>
                            TRANSACTION_PER_MINUTE.map(
                                (tpm) =>
                                    selected_metrics.includes(metric) && (
                                        <Box key={`${tpm}${metric}`} sx={{ mb: 4 }}>
                                            <Chart
                                                options={merge(BasedOptionChart(), {
                                                    xaxis: {
                                                        categories: hash_categories,
                                                        labels: {
                                                            style: {
                                                                colors: colors,
                                                            },
                                                        },
                                                    },
                                                    yaxis: {
                                                        title: {
                                                            text: metric === 'procstat_cpu_usage' ? 'Percent' : metric === 'procstat_num_fds' ? '' : 'MB',
                                                        },
                                                    },
                                                    title: {
                                                        text: `${data_type} (TPM: ${tpm}) - Avg ${CONVERT_REPORTED_METRICS_NAME[metric]}`,
                                                    },
                                                    tooltip: {
                                                        custom: function (event: {
                                                            ctx: {
                                                                opts: {
                                                                    colors: string[];
                                                                    series: {
                                                                        name: string;
                                                                    }[];
                                                                };
                                                            };
                                                            series: number[][];
                                                            seriesIndex: number;
                                                            dataPointIndex: number;
                                                            w: {
                                                                globals: {
                                                                    categoryLabels: string[];
                                                                };
                                                            };
                                                        }) {
                                                            const { ctx, series, seriesIndex, dataPointIndex, w } = event;
                                                            const use_case_color = ctx.opts.colors.at(seriesIndex) || '#000';
                                                            const use_case = ctx.opts.series.at(seriesIndex)?.name;
                                                            const selected_data = series[seriesIndex][dataPointIndex];
                                                            const selected_hash = w.globals.categoryLabels[dataPointIndex];
                                                            const selected_hash_information = commits_information.filter((c: CommitInformation) => c.sha === selected_hash).at(0);

                                                            const commit_history = selected_hash_information?.commit_message.replace(/\n\r*\n*/g, '<br />');
                                                            const commited_by = selected_hash_information?.commit_date + ' commited by @' + selected_hash_information?.commiter_name;
                                                            const commit_data = `<b>${use_case}</b>: ${selected_data}`;

                                                            return (
                                                                '<div class="commit_box"><div class="mb"><b>' +
                                                                selected_hash_information?.sha +
                                                                '</b></div><div class="mb bold"><b>' +
                                                                commit_history +
                                                                '</b></div><div class="mb bold"><b>' +
                                                                commited_by +
                                                                '</b></div><div class="f">' +
                                                                `<div style="width: 25px; height: 10px; border: solid #fff 1px; background: ${use_case_color}"><div/>` +
                                                                `<div class="ml">${commit_data}</div>` +
                                                                '</div></div>'
                                                            );
                                                        },
                                                    },
                                                })}
                                                series={
                                                    trend_data.filter((t: TrendData) => t.name === metric && t.data_type === data_type.toLowerCase() && t.data_tpm === tpm)?.at(0)?.data_series || []
                                                }
                                                type="line"
                                                width="800"
                                            />
                                        </Box>
                                    )
                            )
                        )}
                    </Container>
                </Container>
            )}
        </Container>
    );
}

function useStatePerformanceTrend(password: string) {
    const [state, setState] = React.useState({
        last_update: undefined as string | undefined,
        hash_categories: [] as number[],
        trend_data: [] as TrendData[],
        commits_information: [] as CommitInformation[],
    });

    React.useEffect(() => {
        (async () => {
            if (password === '') {
                return;
            }

            const performances: PerformanceTrendData[] = await GetPerformanceTrendData(password);
            if (performances == null || performances.length === 0) {
                return;
            }

            const trend_data: TrendData[] = [];
            // With ScanIndexForward being set to true, the trend data are being sorted descending based on the CommitDate.
            // Therefore, the first data that has commit date is the latest commit.
            const commit_date = performances.at(0)?.CommitDate.N || '';
            const hash_categories = Array.from(new Set(performances.map((p) => p.CommitHash.S.substring(0, 6)))).reverse();
            // Get all the information for the hash categories in order to get the commiter name, the commit message, and the releveant information
            // const commits_information = await Promise.all(hash_categories.map((hash) => GetServiceCommitInformation(hash)));
            const commits_information: ServiceCommitInformation[] = hash_categories.map((hash) => {
                return {
                    author: { login: 'Login' },
                    commit: { message: 'Message', committer: { date: '1/1/99' } },
                    sha: hash,
                };
            });
            const final_commits_information: CommitInformation[] = commits_information.map((c) => {
                return {
                    commiter_name: c.author.login,
                    commit_message: c.commit.message,
                    commit_date: c.commit.committer.date,
                    sha: c.sha.substring(0, 7),
                };
            });

            /* Generate series of data that has the following format:
            data_rate: transaction per minute
            data_series: [{…}]
            data_type: metrics or traces or logs
            name: metric_name
            */
            for (const metric of REPORTED_METRICS) {
                for (const tpm of TRANSACTION_PER_MINUTE) {
                    for (const data_type of ['metrics', 'traces', 'logs']) {
                        const typeGrouping = performances.filter((p) => p.DataType.S === data_type);
                        if (typeGrouping.length === 0) {
                            continue;
                        }
                        const data_series: { name: string; data: number[] }[] = [];
                        for (const use_case of USE_CASE) {
                            const data = typeGrouping
                                .reverse()
                                .filter((d) => d.UseCase.S === use_case)
                                .map((p) => {
                                    try {
                                        return Number(Number(p.Results.M[tpm].M[metric].M.Average?.N).toFixed(2));
                                    } catch (e) {
                                        return -1;
                                    }
                                });
                            if (data.length === 0) {
                                continue;
                            }
                            data_series.push({
                                name: use_case,
                                data: data,
                            });
                        }
                        trend_data.push({
                            name: metric,
                            data_type: data_type,
                            data_tpm: tpm,
                            data_series: data_series.reverse(),
                        });
                    }
                }
            }
            setState((prev: any) => ({
                ...prev,
                trend_data: trend_data,
                hash_categories: hash_categories,
                commits_information: final_commits_information,
                last_update: moment.unix(Number(commit_date)).format('dddd, MMMM Do, YYYY h:mm:ss A'),
            }));
        })();
    }, [password, setState]);

    return [state, setState] as const;
}

function useStateDataType() {
    const [state, setState] = React.useState({
        data_type: 'Metrics' as 'Metrics' | 'Traces' | 'Logs' | string,
    });

    return [state, setState] as const;
}

function useStateSelectedMetrics() {
    const [state, setState] = React.useState({
        selected_metrics: ['procstat_cpu_usage', 'procstat_memory_rss'] as string[],
    });

    return [state, setState] as const;
}
