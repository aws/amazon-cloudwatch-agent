// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

import { Container, Typography } from "@mui/material";
import { usePageEffect } from "../../core/page";

export default function HomePage(): JSX.Element {
  usePageEffect({ title: "Amazon CloudWatch Agent" });

  return (
    <Container sx={{ py: "5vh", border: "1px solid" }} maxWidth="lg">
      <Container sx={{ mb: 4 }}>
        <Typography sx={{ mb: 2, fontWeight: "bold" }} variant="h2">
          Wikipedia
        </Typography>
        <hr />
      </Container>
    </Container>
  );
}