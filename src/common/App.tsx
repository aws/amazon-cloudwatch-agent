// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
import { CssBaseline, ThemeProvider, Toolbar } from '@mui/material';
import React from 'react';
import { Route, Routes } from 'react-router-dom';
import { useTheme } from '../core/theme';
import { AppToolbar } from './AppToolbar';
import { ErrorBoundary } from './ErrorBoundary';

import { HomePage, PerformanceReport, PerformanceTrend } from './Routes';

export function App(): JSX.Element {
    const theme = useTheme();
    const [{ password_is_set, password }, setPasswordState] = useStatePassword();
    return (
        <ThemeProvider theme={theme}>
            <CssBaseline />
            <ErrorBoundary>
                <AppToolbar />
                <Toolbar />
                <Routes>
                    <Route path="/trend" element={<React.Suspense children={<PerformanceTrend password={password} password_is_set={password_is_set} set_password_state={setPasswordState} />} />} />
                    <Route path="/report" element={<React.Suspense children={<PerformanceReport password={password} password_is_set={password_is_set} set_password_state={setPasswordState} />} />} />
                    <Route index path="/" element={<React.Suspense children={<HomePage />} />} />
                </Routes>
            </ErrorBoundary>
        </ThemeProvider>
    );
}

function useStatePassword() {
    const [state, setState] = React.useState({
        password: '' as string,
        password_is_set: false as boolean,
    });

    return [state, setState] as const;
}
