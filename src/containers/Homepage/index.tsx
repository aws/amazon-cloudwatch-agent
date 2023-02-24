// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

import { Container, Link, ListItem, Paper, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Typography } from "@mui/material";
import { usePageEffect } from "../../core/page";
import { CIIcon } from "../../icons/CIIcon";
import { CodeCoverageIcon } from "../../icons/CodeCoverageIcon";
import { ReleaseVersionIcon } from "../../icons/ReleaseVersionIcon";
import { SUPPORTED_PLUGINs, SUPPORTED_USE_CASES } from "./constants";

export default function HomePage(): JSX.Element {
  usePageEffect({ title: "Amazon CloudWatch Agent" });
  return (
    <Container sx={{ py: "5vh", border: "1px solid" }} maxWidth="lg">
      <Container sx={{ mb: 4 }}>
        <CIIcon />
        <CodeCoverageIcon />
        <ReleaseVersionIcon />
      </Container>
      <Container sx={{ mb: 4 }}>
        <Typography sx={{ fontWeight: "bold" }} variant="h2">
          Overview
        </Typography>
        <hr />
        <Typography sx={{ mb: 2 }} variant="h4">
          CloudWatchAgent (CWA) is an agent which collects system-level metrics, custom metrics (e.g Prometheus, Statsd, Collectd), monitoring logs
          and publicizes these telemetry data to AWS CloudWatch Metrics, and Logs backends. It is fully compatible with AWS computing platforms
          including EC2, ECS, and EKS and non-AWS environment.
        </Typography>

        <Typography variant="h4">
          See the{" "}
          <Link href="https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Install-CloudWatch-Agent.html">Amazon CloudWatch Agent</Link> for
          more information on supported OS and how to install Amazon CloudWatch Agent.
        </Typography>
      </Container>

      <Container sx={{ mb: 4 }}>
        <Typography sx={{ fontWeight: "bold" }} variant="h2">
          Components and Use Case
        </Typography>
        <hr />
        <Typography sx={{ mb: 3, fontWeight: "bold" }} variant="h4">
          CWA Built-in Components
        </Typography>
        <TableContainer sx={{ mb: 4 }} component={Paper}>
          <Table sx={{ minWidth: 100, borderStyle: "solid" }} size="small" aria-label="a dense table">
            <TableHead>
              <TableRow>
                <TableCell sx={{ border: "1px solid #000", fontWeight: "bold" }}>Input</TableCell>
                <TableCell sx={{ border: "1px solid #000", fontWeight: "bold" }}>Processor</TableCell>
                <TableCell sx={{ border: "1px solid #000", fontWeight: "bold" }}>Output</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {SUPPORTED_PLUGINs.map((plugin) => (
                <TableRow key={plugin.input}>
                  <TableCell sx={{ border: "1px solid #000" }}>{plugin.input}</TableCell>
                  <TableCell sx={{ border: "1px solid #000" }}>{plugin.processor}</TableCell>
                  <TableCell sx={{ border: "1px solid #000" }}>{plugin.output}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
        <Typography sx={{ mb: 1, fontWeight: "bold" }} variant="h4">
          CWA Use Case
        </Typography>
        {SUPPORTED_USE_CASES.map((use_case) => (
          <ListItem key={use_case.name} sx={{ display: "list-item", mb: -2 }}>
            <Link href={use_case.url}>{use_case.name}</Link>
          </ListItem>
        ))}
      </Container>

      <Container sx={{ mb: 4 }}>
        <Typography sx={{ fontWeight: "bold" }} variant="h2">
          Getting Started
        </Typography>
        <hr />
        <Typography sx={{ mb: 3, fontWeight: "bold" }} variant="h4">
          Prerequisites
        </Typography>
        <Typography sx={{ mb: 3 }} variant="h4">
          To build the Amazon CloudWatch Agent locally, you will need to have Golang installed. You can download and install{" "}
          <Link href="https://go.dev/doc/install">Golang</Link>
        </Typography>

        <Typography sx={{ mb: 3, fontWeight: "bold" }} variant="h4">
          CWA configuration
        </Typography>
        <Typography sx={{ mb: 3 }} variant="h4">
          Amazon CloudWatch Agent is built with a{" "}
          <Link href="https://github.com/aws/amazon-cloudwatch-agent/blob/main/translator/config/defaultConfig.go#L6-L176">
            default configuration
          </Link>
          .The Amazon CloudWatch Agent uses the JSON configuration following{" "}
          <Link href="https://github.com/aws/amazon-cloudwatch-agent/blob/main/translator/config/schema.json">this schema design</Link>. For more
          information on how to configure Amazon CloudWatchAgent configuration files when running the agent, please following this{" "}
          <Link href="https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-Configuration-File-Details.html">document</Link>
        </Typography>

        <Typography sx={{ mb: 3, fontWeight: "bold" }} variant="h4">
          Try out CWA
        </Typography>
        <Typography sx={{ mb: 3 }} variant="h4">
          The Amazon CloudWatch Agent supports all AWS computing platforms and Docker/Kubernetes. Here are some examples on how to run the Amazon
          CloudWatch Agent to send telemetry data:
          <ListItem sx={{ display: "list-item", mb: -1 }}>
            <Link href="https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/install-CloudWatch-Agent-on-premise.html">
              Run in with local host
            </Link>
          </ListItem>
          <ListItem sx={{ display: "list-item", mb: -1 }}>
            <Link href="https://docs.amazonaws.cn/en_us/AmazonCloudWatch/latest/monitoring/install-CloudWatch-Agent-on-EC2-Instance-fleet.html">
              Run in with EC2
            </Link>
          </ListItem>
          <ListItem sx={{ display: "list-item", mb: -1 }}>
            <Link href="https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights-Prometheus-Setup-ECS.html">
              Run in with ECS
            </Link>
          </ListItem>
          <ListItem sx={{ display: "list-item", mb: -1 }}>
            <Link href="https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights-Prometheus-install-EKS.html">
              Run in with EKS
            </Link>
          </ListItem>
        </Typography>

        <Typography sx={{ mb: 3 }} variant="h4">
          To build the Amazon CloudWatch Agent locally, you will need to have Golang installed. You can download and install{" "}
          <Link href="https://go.dev/doc/install">Golang</Link>
        </Typography>

        <Typography sx={{ mb: 3, fontWeight: "bold" }} variant="h4">
          Build your own artifacts
        </Typography>
        <Typography sx={{ mb: 3 }} variant="h4">
          Use the following instructions to build your own Amazon Cloudwatch Agent artifacts:
          <ListItem sx={{ display: "list-item", mb: -1 }}>
            <Link href="https://github.com/aws/amazon-cloudwatch-agent/tree/master#building-and-running-from-source">Build RPM/DEB/MSI/TAR</Link>
          </ListItem>
          <ListItem sx={{ display: "list-item", mb: -1 }}>
            <Link href="https://github.com/aws/amazon-cloudwatch-agent/tree/master/amazon-cloudwatch-container-insights/cloudwatch-agent-dockerfile">
              Build Docker image
            </Link>
          </ListItem>
        </Typography>
      </Container>

      <Container sx={{ mb: 4 }}>
        <Typography sx={{ fontWeight: "bold" }} variant="h2">
          Getting Help
        </Typography>
        <hr />
        <Typography>
          Use the community resources below for getting help with the Amazon CloudWatch Agent.
          <ListItem sx={{ display: "list-item", mb: -2 }}>
            Use GitHub issues to{" "}
            <Link href="https://github.com/aws/amazon-cloudwatch-agent/issues/new/choose">report bugs and request features.</Link>
          </ListItem>
          <ListItem sx={{ display: "list-item", mb: -2 }}>
            If you think you may have found a security issues, please following{" "}
            <Link href="https://aws.amazon.com/security/vulnerability-reporting/">this instructions.</Link>
          </ListItem>
          <ListItem sx={{ display: "list-item", mb: -2 }}>
            For contributing guidelines, refer to{" "}
            <Link href="https://github.com/aws/amazon-cloudwatch-agent/blob/main/CONTRIBUTING.md">CONTRIBUTING.md.</Link>
          </ListItem>
        </Typography>
      </Container>

      <Container sx={{ mb: 4 }}>
        <Typography sx={{ fontWeight: "bold" }} variant="h2">
          License
        </Typography>
        <hr />
        <Typography sx={{ mb: 2 }} variant="h4">
          MIT License
        </Typography>
        <Typography sx={{ mb: 4 }} variant="h4">
          Copyright (c) 2015-2019 InfluxData Inc. Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved. Permission is hereby granted,
          free of charge, to any person obtaining a copy of this software and associated documentation files (the Software), to deal in the Software
          without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
          copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions: The above
          copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
        </Typography>
      </Container>
    </Container>
  );
}