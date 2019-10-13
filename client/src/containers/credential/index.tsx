import React from "react";
import { connect } from "react-redux";
import Grid from '@material-ui/core/Grid';
import ModifyCredentialForm from "./modifyCredentialForm"
import CredentialList from './credentialLlist';
import {Typography} from "@material-ui/core";
import { WithStyles, withStyles } from "@material-ui/core/styles"
import Button from "@material-ui/core/Button";
import Box from "@material-ui/core/Box";
import { CredentialActions } from '../../redux/credential'

export enum FormMode {
    Edit = "Edit",
    Create = "Create",
    None = "None"
}

interface IState {
    formMode: FormMode
}

const styles = theme => ({
    containerHeader: {
        height: "50px",
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center"
    }
});

type Props = ReturnType<typeof mapStateToProps>
    & WithStyles<ReturnType<typeof styles>>
    & ReturnType<typeof mapDispatchToProps>

class CredentialContainer extends React.Component<Props> {
    state: IState = {
        formMode: FormMode.None
    };

    setMode = (mode: FormMode) => () => {
        this.setState({
            formMode: mode
        });
    };

    setCurrentCredentialId = (id: string) => () => {

    };

    render() {
        const { formMode } = this.state;
        const { classes } = this.props;

        return (
            <Grid container>
                <Grid item md={8} lg={8}>
                    <Box display="flex"
                        component="div"
                        flexDirection="row"
                        alignItems="center"
                        justifyContent="space-between"
                        style={{ paddingLeft: '20px', paddingRight: '20px' }}>
                        <Typography variant="h5">
                            API Keys
                        </Typography>
                        <Button component="span" onClick={this.setMode(FormMode.Create)}>Create New Key</Button>
                    </Box>
                    <CredentialList
                        credentials={this.props.credentials}
                        onDelete={(id) => Promise.resolve()}
                        setCurrentCredentialId={this.setCurrentCredentialId}
                    />
                </Grid>
                <Grid item md={4} lg={4}>
                    <ModifyCredentialForm mode={formMode} />
                </Grid>
            </Grid>
        );
    }
}

const mapStateToProps = (state) => {
    return {
        credentials: state.CredentialsReducer.credentials,
        currentCredentialId: state.CredentialsReducer.currentCredentialId
    }
};

const mapDispatchToProps = (dispatch) => {
    return {
        setCurrentCredentialId: (id: string) => dispatch({ type: CredentialActions.SET_CREDENTIALS, payload: { id } })
    }
};

export default connect(mapStateToProps, mapDispatchToProps)
    (withStyles(styles)(CredentialContainer)) as any as React.ComponentType;