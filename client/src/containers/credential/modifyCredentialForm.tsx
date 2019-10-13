import React, {useCallback, useState} from "react";
import {connect} from "react-redux";
import {CreateCredential, ICredential} from "../../redux/credential";
import {makeStyles, Typography} from "@material-ui/core";
import Button from "@material-ui/core/Button";
import TextField from "@material-ui/core/TextField";
import theme from "../../theme";
import {FormMode} from "./index";
import Box from "@material-ui/core/Box";

interface IProps {
    mode?: FormMode
    http_referrer_restriction?: string
}

const useStyles = makeStyles(theme => ({
    container: {
        marginTop: '-10px',
        backgroundColor: '#f1f1f1',
        height: 'calc(100vh - 110px)',
        padding: '30px',
    },
    innerContainer: {
        marginTop: '20px',
        marginBottom: '30px',
    }
}));

const CredentialsForm = (props: IProps & ReturnType<typeof mapDispatchToProps>) => {
    const classes = useStyles(theme);
    const [error, setError] = useState("");
    const { mode, http_referrer_restriction = "" } = props;

    const submit = useCallback((e: React.FormEvent<HTMLFormElement>) => {
        async function submitCredential(httpReferralUrl) {
            const credential: Partial<ICredential> = {};
            credential.http_referrer_restriction = httpReferralUrl;
            try {
                await props.createCredential(credential);
            } catch(err) {
                setError(err.message);
            }
        }
        e.preventDefault();
        const formValidity = e.currentTarget.checkValidity();
        if (formValidity) {
            submitCredential(e.currentTarget.url.value);
        }
    }, []);

    if (mode == FormMode.None) {
        return null
    }

    return (
        <div className={classes.container}>
            <Typography variant="h6">
                New API Key
            </Typography>
            <div>
                <form onSubmit={submit} name="new-credential">
                    <p>{error}</p>
                    <div className={classes.innerContainer}>
                        <TextField
                            fullWidth
                            required
                            defaultValue={http_referrer_restriction}
                            type="url"
                            name="url"
                            label="http referrer restriction"
                        />
                    </div>
                    <Box display="flex" flexDirection="row" justifyContent="flex-start" alignItems="center">
                        <Button type="submit" variant="contained">Submit</Button>
                        &nbsp;&nbsp;&nbsp;&nbsp;
                        <Button component="span">Cancel</Button>
                    </Box>
                </form>
            </div>
        </div>
    );
};

const mapDispatchToProps = (dispatch) => {
    return {
        createCredential: (credential: Partial<ICredential>) => dispatch(CreateCredential(credential))
    };
};
const mapStateToProps = (state) => ({});

export default connect(mapStateToProps, mapDispatchToProps)(CredentialsForm);