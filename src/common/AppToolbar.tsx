// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
import { Code, Equalizer, Home, LibraryBooks, TrendingUp } from "@mui/icons-material";
import { AppBar, AppBarProps, Box, Link, Toolbar, Typography } from "@mui/material";
import { CloudWatchIcon } from "../icons/CloudWatchIcon";
import { ThemeButton } from "./ThemeButton";
type AppToolbarProps = AppBarProps;

export function AppToolbar(props: AppToolbarProps): JSX.Element {
  const { sx, ...other } = props;

  return (
    <AppBar
      sx={{
        paddingRight: "unset !important",
        background: "#212121",
        padding: "0.625rem 0",
        display: "flex",
        alignItems: "center",
        justifyContent: "flex-start",
        position: "relative",
      }}
    >
      <Toolbar
        sx={{
          maxWidth: "1140px",
          width: "100%",
          flex: "2",
          display: "flex",
          flexWrap: "nowrap",
          justifyContent: "space-between",
          alignItems: "center",
          minHeight: "50px",
          paddingLeft: "10px",
          marginLeft: "auto",
          marginRight: "auto",
          paddingRight: "15px",
        }}
      >
        {/* App name / logo */}
        <Link href="/" sx={{ padding: "8px 16px 3px" }}>
          <CloudWatchIcon />
        </Link>
        <Box sx={{ display: "flex", alignItems: "center", gap: "25px" }}>
          <Link href="/" sx={{ display: "flex", gap: "10px", color: "#FFF" }}>
            <Home sx={{ color: "#FFF" }} />
            <Typography>Home</Typography>
          </Link>

          <Link href="/report" sx={{ display: "flex", gap: "10px", color: "#FFF" }}>
            <Equalizer sx={{ color: "#FFF" }} />
            <Typography>Performance Report</Typography>
          </Link>
          <Link href="/trend" sx={{ display: "flex", gap: "10px", color: "#FFF" }}>
            <TrendingUp sx={{ color: "#FFF" }} />
            <Typography>Performance Trend</Typography>
          </Link>
          <Link href="/wiki" sx={{ display: "flex", gap: "10px", color: "#FFF" }}>
            <LibraryBooks sx={{ color: "#FFF" }} />
            <Typography>Wikipedia </Typography>
          </Link>
          <Link href="https://github.com/aws/amazon-cloudwatch-agent" sx={{ display: "flex", gap: "10px", color: "#FFF" }}>
            <Code sx={{ color: "#FFF" }} />
            <Typography>GitHub Code </Typography>
          </Link>
          {/* Account related controls (icon buttons) */}

          {<ThemeButton sx={{ mr: 1 }} />}
        </Box>
      </Toolbar>
    </AppBar>
  );
}