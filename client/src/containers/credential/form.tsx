import React, { useCallback, useState } from "react";
import { ICredential } from "../../models/credential";
import {makeStyles, Typography} from "@material-ui/core";
import Button from "@material-ui/core/Button";
import TextField from "@material-ui/core/TextField";
import theme from "../../theme";

interface IProps {
    http_referrer_restriction?: string
}

const useStyles = makeStyles(theme => ({
    container: {
        paddingLeft: '20px',
        paddingRight: '20px'
    },
    innerContainer: {
        marginTop: '20px',
        marginBottom: '30px',
    }
}));

const CredentialsForm = (props: IProps) => {
    const classes = useStyles(theme);

    return (
        <>
            <Typography variant="h5">
                New API Key
            </Typography>
            <div>
                <form>
                    <div className={classes.innerContainer}>
                        <TextField type="url" fullWidth label="http referrer restriction" />
                    </div>
                    <Button variant="contained">Submit</Button>
                </form>
            </div>
        </>
    );
};

export default CredentialsForm;