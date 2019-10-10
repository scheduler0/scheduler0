import React, { useEffect, useState} from "react";
import { makeStyles } from "@material-ui/core/styles";
import theme from '../../theme';
import {Typography} from "@material-ui/core";
import {ICredential} from "../../models/credential";
import CredentialListItem from "./list-item";

interface IProps {
    credentials: ICredential[]
    onDelete: (id) => Promise<void>
}

const useStyles = makeStyles(theme => ({
    root: {
        padding: theme.spacing(3, 2),
    },
    container: {
        marginTop: '10px'
    },
}));

function Credential(props: IProps) {
    const classes = useStyles(theme);
    const { credentials, onDelete } = props;

    return (
        <>
            <Typography variant="h5">
                API Keys
            </Typography>
            <div className={classes.container}>
                {credentials && credentials.map((credential) => (
                    <CredentialListItem
                        key={credential.api_key}
                        credential={credential}
                        onDelete={onDelete}
                    />)
                )}
            </div>
        </>
    );
}

export default Credential;