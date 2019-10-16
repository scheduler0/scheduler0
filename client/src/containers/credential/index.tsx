// @ts-ignore
import React from "react";
import { connect } from "react-redux";
import Grid from '@material-ui/core/Grid';
import ModifyCredentialForm from "./modifyCredentialForm"
import CredentialList from './credentialLlist';
import {createStyles} from "@material-ui/core";
import {WithStyles, withStyles} from "@material-ui/core/styles"
import {setCurrentCredentialId, DeleteCredential} from '../../redux/credential'

export enum FormMode {
    Edit = "Edit",
    Create = "Create",
    None = "None"
}

interface IState {
    formMode: FormMode
}

const styles = theme => createStyles({
    containerHeader: {
        height: "50px",
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
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
        }, () => {
            const { currentCredentialId, setCurrentCredentialId } = this.props;
            if (this.state.formMode == FormMode.None && currentCredentialId) {
                setCurrentCredentialId(null);
            }
        });
    };

    render() {
        const {formMode} = this.state;
        const {credentials, currentCredentialId, deleteCredential, setCurrentCredentialId} = this.props;

        const currentCredential = (formMode == FormMode.Edit)
            ? credentials.find(({ id }) => id == currentCredentialId)
            : null;

        return (
            <Grid container>
                <Grid item md={8} lg={8}>
                    <CredentialList
                        credentials={credentials}
                        formMode={formMode}
                        setMode={this.setMode}
                        deleteCredential={deleteCredential}
                        setCurrentCredentialId={setCurrentCredentialId}
                    />
                </Grid>
                <Grid item md={4} lg={4}>
                    <ModifyCredentialForm
                        formMode={formMode}
                        currentCredentialId={currentCredentialId}
                        setCurrentCredentialId={setCurrentCredentialId}
                        setMode={this.setMode}
                        currentCredential={currentCredential}
                    />
                </Grid>
            </Grid>
        );
    }
}

const mapStateToProps = (state) => ({
    credentials: state.CredentialsReducer.credentials,
    currentCredentialId: state.CredentialsReducer.currentCredentialId
});

const mapDispatchToProps = (dispatch) => ({
    setCurrentCredentialId: (id: string) => dispatch(setCurrentCredentialId(id)),
    deleteCredential: (id: string) => dispatch(DeleteCredential(id))
});

export default connect(mapStateToProps, mapDispatchToProps)
    (withStyles(styles)(CredentialContainer)) as any as React.ComponentType;