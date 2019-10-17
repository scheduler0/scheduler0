import React, {useCallback, useEffect, useState} from "react";
import {connect} from "react-redux";
import {CreateCredential, ICredential, UpdateCredential} from "../../redux/credential";
import {makeStyles, Typography} from "@material-ui/core";
import Button from "@material-ui/core/Button";
import TextField from "@material-ui/core/TextField";
import theme from "../../theme";
import {FormMode} from "./index";
import Box from "@material-ui/core/Box";

interface IProps {
    formMode?: FormMode
    currentCredentialId?: string
    http_referrer_restriction?: string
    setCurrentCredentialId?: (id: string) => void
    setMode: (mode: FormMode) => void
    currentCredential?: ICredential
}

const useStyles = makeStyles((theme) => ({
    container: {
        marginTop: '50px',
        backgroundColor: '#f1f1f1',
        height: 'calc(100vh - 50px)',
        padding: '30px',
        position: "sticky",
        top: 50,
        left: 0,
    },
    innerContainer: {
        marginTop: '20px',
        marginBottom: '30px',
    }
}));

const CredentialsForm = (props: IProps & ReturnType<typeof mapDispatchToProps>) => {
    const classes = useStyles(theme);
    const [state, setState] = useState({ http_referrer_restriction: "" });
    const { formMode, currentCredential = null, setMode, setCurrentCredentialId } = props;
    const { http_referrer_restriction } = state;

    useEffect(() => {
        setState({
            http_referrer_restriction: currentCredential
                ? currentCredential.http_referrer_restriction
                : ""
        });
    }, [currentCredential]);

    const handleSubmit = useCallback((e: React.FormEvent<HTMLFormElement>) => {
        async function submitCredential(httpReferralUrl) {
            const credential: Partial<ICredential> = {};
            credential.http_referrer_restriction = httpReferralUrl;
            const { formMode } = props;

            try {
                if (formMode == FormMode.Create) {
                    await props.createCredential(credential);
                } else {
                    await props.updateCredential(credential);
                }
            } catch (e) {
                console.error(e);
            }
        }

        e.preventDefault();

        const form = e.currentTarget;
        const formValidity = form.checkValidity();
        if (formValidity) {
            submitCredential(e.currentTarget.url.value);
            setState({ http_referrer_restriction: "" });
        }
    }, [formMode]);
    const cancelCallback = useCallback(() => {
        if (formMode == FormMode.Edit) {
            setCurrentCredentialId(null);
        }

        setMode(FormMode.None);
    }, [formMode]);

    if (formMode == FormMode.None) {
        return null
    }

    return (
        <div className={classes.container}>
            {(formMode == FormMode.Create) ?
                <Typography variant="h6">New API Key</Typography>:
                <Typography variant="h6">Edit API Key</Typography>}
            <div>
                <form onSubmit={handleSubmit} name="new-credential">
                    <div className={classes.innerContainer}>
                        <TextField
                            fullWidth
                            required
                            value={http_referrer_restriction}
                            onChange={(e) => setState({ http_referrer_restriction: e.target.value })}
                            type="url"
                            name="url"
                            label="http referrer restriction"
                        />
                    </div>
                    <Box display="flex" flexDirection="row" justifyContent="flex-start" alignItems="center">
                        <Button type="submit" variant="contained">Submit</Button>
                        &nbsp;&nbsp;&nbsp;&nbsp;
                        <Button component="span" onClick={cancelCallback}>Cancel</Button>
                    </Box>
                </form>
            </div>
        </div>
    );
};

const mapDispatchToProps = (dispatch) => ({
    createCredential: (credential: Partial<ICredential>) => dispatch(CreateCredential(credential)),
    updateCredential: (credential: Partial<ICredential>) => dispatch(UpdateCredential(credential)),
});

const mapStateToProps = (state) => ({});

export default connect(mapStateToProps, mapDispatchToProps)(CredentialsForm);