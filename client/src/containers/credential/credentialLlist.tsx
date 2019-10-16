import React, {useCallback, useEffect, useState} from "react";
import { makeStyles } from "@material-ui/core/styles";
import theme from '../../theme';
import {ICredential} from "../../redux/credential";
import CredentialListItem from "./creadentialListItem";
import Box from "@material-ui/core/Box";
import {Typography} from "@material-ui/core";
import Button from "@material-ui/core/Button";
import {FormMode} from "./index";

interface IProps {
    credentials: ICredential[]
    setCurrentCredentialId: (id: string) => void
    formMode: FormMode
    deleteCredential: (id: string) => Promise<void>
    setMode: (mode: FormMode) => () => void
}

const useStyles = makeStyles(theme => ({
    root: {
        padding: theme.spacing(3, 2),
    },
    container: {
        marginTop: '50px',
        padding: '20px',
    },
    header: {
        height: '50px',
        display: 'flex',
        flexDirection: 'row',
        justifyContent: 'space-between',
        alignItems: 'center'
    }
}));

function CredentialList(props: IProps) {
    const classes = useStyles(theme);
    const { credentials, deleteCredential, setCurrentCredentialId, setMode } = props;

    const handleDelete = useCallback((credentialId) => {
        deleteCredential(credentialId);
    }, []);

    return (
        <div className={classes.container}>
            <Box display="flex"
                 component="div"
                 flexDirection="row"
                 alignItems="center"
                 justifyContent="space-between"
                 style={{ paddingLeft: '20px', paddingRight: '20px', marginBottom: '10px' }}>
                <Typography variant="h5">Credentials</Typography>
                <Button component="span" onClick={() => {
                    setCurrentCredentialId(null);
                    setMode(FormMode.Create)();
                }}>Create New Key</Button>
            </Box>
            <div>
                {credentials && credentials.map((credential, index) => (
                    <CredentialListItem
                        key={`credential-${index}`}
                        credential={credential}
                        onDelete={handleDelete}
                        setCurrentCredentialId={() => {
                            setCurrentCredentialId(credential.id);
                            setMode(FormMode.Edit)();
                        }}
                    />
                ))}
            </div>
        </div>
    );
}

export default CredentialList;