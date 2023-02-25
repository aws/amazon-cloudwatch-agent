// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
import React from 'react';
import { Link, Dialog, DialogTitle, TextField, DialogContent, DialogContentText } from '@mui/material';
import LoadingButton from '@mui/lab/LoadingButton';
import { styled } from '@mui/material/styles';
import { ArrowRightAlt } from '@mui/icons-material';
import { AxiosConfig } from './Axios';

const CssTextField = styled(TextField)(({ theme }) => ({
    '& label.Mui-focused': {
        color: '#FFF',
    },
    '& .MuiInput-input': {
        color: '#FFF',
    },
    '& .MuiInput-underline:after': {
        borderBottomColor: '#FFF',
    },
    '& .MuiFormHelperText-root': {
        color: '#000',
    },
    '& .MuiOutlinedInput-root': {
        '& fieldset': {
            borderColor: '#FFF',
            color: '#FFF',
        },
        '&:hover fieldset': {
            borderColor: '#FFF',
            color: '#FFF',
        },
        '&.Mui-focused fieldset': {
            borderColor: '#FFF',
            color: '#FFF',
        },
    },
}));

export function PasswordDialog(props: { password: string; password_is_set: boolean; set_password_state: any }): JSX.Element {
    const [{ error, loading }, setStateLoading] = useStateLoading();
    const { password, password_is_set, set_password_state } = props;
    const setUserPassword = async (e: { target: { value: string } }) => {
        set_password_state({
            password_is_set: false,
            password: e.target.value,
        });
    };

    const onUserEnterPassword = async (e: { key: string }) => {
        switch (e.key) {
            case 'Enter':
                isValidateSuccess();
                break;
            default:
                return; // Quit when this doesn't handle the key event.
        }
    };

    const isValidateSuccess = async () => {
        setStateLoading({ loading: true, error: false });
        const success: { data: { success: Boolean } } = await AxiosConfig.post('/', {
            Action: 'Validate',
            Params: {
                SecretKey: password,
            },
        });

        setStateLoading({ loading: false, error: !success.data.success });
        set_password_state({
            password: password,
            password_is_set: success.data.success,
        });
    };
    return (
        <Dialog
            fullWidth
            open={!password_is_set}
            sx={{
                backdropFilter: 'blur(2px)',
            }}
            PaperProps={{
                style: {
                    overflow: "hidden",
                    height: '320px',
                    width: '400px',
                    padding: '20px 0px 0px 25px',
                    backgroundImage: 'unset',
                    backgroundColor: '#121212',
                    borderRadius: '20px',
                    border: '1px solid #fff',
                },
            }}
        >
            <DialogTitle sx={{ fontSize: '2em', color: '#fff' }}> Welcome back.</DialogTitle>
            <DialogContent sx={{ mt: '-20px' }}>
                <DialogContentText sx={{ mb: 4, color: 'rgba(255, 255, 255, 0.5)' }}>
                    Log in to your account or{' '}
                    <Link sx={{ color: 'rgba(255, 255, 255, 0.9)' }} href="https://github.com/aws/amazon-cloudwatch-agent/issues/new/choose">
                        contact us
                    </Link>
                </DialogContentText>
                <CssTextField
                    sx={{
                        mb: 1,
                        borderRadius: '10px',
                        width: '86%',
                        color: '#fff',
                    }}
                    autoFocus
                    error={error}
                    margin="dense"
                    id="name"
                    size="small"
                    label="Password"
                    type="password"
                    color="primary"
                    focused
                    placeholder="********************************"
                    helperText="Incorrect password"
                    variant="standard"
                    onChange={setUserPassword}
                    onKeyDown={onUserEnterPassword}
                />
                <LoadingButton
                    loading={loading}
                    variant="outlined"
                    sx={{
                        mb: 1,
                        width: '86%',
                        color: '#fff',
                        borderColor: '#fff',
                    }}
                    onClick={isValidateSuccess}
                >
                    Log in with Password <ArrowRightAlt />
                </LoadingButton>
            </DialogContent>
        </Dialog>
    );
}

function useStateLoading() {
    const [state, setState] = React.useState({
        error: false as boolean,
        loading: false as boolean,
    });

    return [state, setState] as const;
}
