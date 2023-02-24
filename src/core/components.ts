// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

import { type Palette, type ThemeOptions } from "@mui/material/styles";

/**
 * Style overrides for Material UI components.
 */
/* eslint-disable-next-line @typescript-eslint/no-unused-vars */
export const components = (palette: Palette): ThemeOptions["components"] => ({
  MuiButton: {
    styleOverrides: {
      root: {
        textTransform: "unset",
      },
      contained: {
        boxShadow: "none",
        "&:hover": {
          boxShadow: "none",
        },
      },
    },
  },

  MuiButtonGroup: {
    styleOverrides: {
      root: {
        boxShadow: "none",
      },
    },
  },
});