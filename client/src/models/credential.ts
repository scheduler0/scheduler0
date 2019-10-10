// import { createActions, handleActions } from "redux-actions";
import {action, observable, runInAction, configure} from 'mobx';
import axios from 'axios';

configure({ enforceActions: "observed" });

export interface ICredential {
    id: string
    api_key: string
    http_referrer_restriction: string
    date_created: string
}

export interface ICredentialModel {
    credentials: ICredential[]
    state: string
    error: string
    fetchCredentials: () => Promise<void>
    setCredentials: (credentials: ICredential[])  => void
    deleteCredential: (credentialId: string) => Promise<void>
}

export class CredentialState {
    @observable credentials: ICredential[] = [];
    @observable state: string =  "pending";
    @observable error: string = "";

    @action
    setCredentials(credentials) {
        this.credentials = credentials;
    }

    @action
    async fetchCredentials() {
        this.credentials = [];
        this.state = "pending";

        try {
            const { data: { success, data } } = await axios.get("/credentials");
            if (success) {
                runInAction(() => {
                    this.state = "done";
                    this.credentials = data;
                    this.error = "";
                });
            } else {
                runInAction(() => {
                    this.state = "done";
                    this.error = "";
                });
            }
        } catch (e) {
            runInAction(() => {
                this.state = "error";
                this.error = e.message;
            });
        }
    }

    @action
    async deleteCredential(credentialId) {
        try {
            const { data: { success } } = await axios.delete("/credential/" + credentialId);
            if (success) {
                runInAction(() => {
                    this.credentials = this.credentials.filter(({id}) => id != credentialId);
                    this.state = "done";
                    this.error = "";
                });
            } else {
                runInAction(() => {
                    this.state = "done";
                    this.error = "";
                });
            }
        } catch (e) {
            this.state = "error";
            this.error = e.message;
        }
    }
};

// const defaultState = {
//     credentials: []
// };
//
// const { setCredentials } = createActions({
//     SET_CREDENTIALS: (credentials = []) => ({ credentials })
// }, defaultState);
//
// export const reducer = handleActions({
//     [setCredentials]: (state, action) => ({
//         credentials: state.credentials
//     })
// }, defaultState);