import Grid from '@material-ui/core/Grid';
import React, {useContext, useEffect, useState} from "react";
import Form from "./form"
import List from './list';
import {inject, observer} from "mobx-react";
import {ICredentialModel} from "../../models/credential";

class CredentialContainer extends React.Component<{ rootStore: ICredentialModel }> {
    render() {
        console.log(this.props);
        return (
            <Grid container>
                <Grid item md={6}>
                    <Form />
                </Grid>
                <Grid item md={6}>
                    <p>{this.props.rootStore.error}</p>
                    <p>{this.props.rootStore.state}</p>
                    <List
                        credentials={this.props.rootStore.credentials}
                        onDelete={(id) => this.props.rootStore.deleteCredential(id)}
                    />
                </Grid>
            </Grid>
        );
    }
}

export default CredentialContainer;