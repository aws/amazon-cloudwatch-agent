import { CssBaseline, ThemeProvider, Toolbar } from "@mui/material";
import  React from "react";
import { Route, Routes } from "react-router-dom";
import { useTheme } from "../core/theme";
import { AppToolbar } from "./AppToolbar";
import { ErrorBoundary } from "./ErrorBoundary";
import { HomePage, PerformanceReport, PerformanceTrend, Wikipedia } from "./Routes";
export function App():JSX.Element {
  const theme = useTheme();
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <ErrorBoundary>
        <AppToolbar />
        <Toolbar />

        <Routes>
          <Route index element={<React.Suspense children={<HomePage />} />} />

          <Route path="/report" element={<React.Suspense children={<PerformanceReport />} />} />

          <Route path="/trend" element={<React.Suspense children={<PerformanceTrend />} />} />

          <Route path="/wiki" element={<React.Suspense children={<Wikipedia />} />} />
        </Routes>
      </ErrorBoundary>
    </ThemeProvider>
  );
}

