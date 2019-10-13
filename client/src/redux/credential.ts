import { createActions, handleActions } from "redux-actions";
import axios from "axios";

export enum CredentialActions {
    SET_CREDENTIALS = "SET_CREDENTIALS",
    SET_CURRENT_CREDENTIAL_ID = "SET_CURRENT_CREDENTIAL_ID"
}

export interface ICredential {
    id: string
    api_key: string
    http_referrer_restriction: string
    date_created: string
}

const defaultState = {
    credentials: [],
    currentCredentialId: null
};

const { SET_CREDENTIALS, SET_CURRENT_CREDENTIAL_ID } = createActions({
    "SET_CREDENTIALS": (credentials = []) => ({ credentials }),
    "SET_CURRENT_CREDENTIAL_ID": (id: string) => ({ currentCredentialId: id })
});

export const credentialsReducer = handleActions({
    SET_CREDENTIALS: (state,{ payload: { credentials } }) => {
        console.log('Setting credentials');
        return {...state, credentials };
    },
    SET_CURRENT_CREDENTIAL_ID: (state, { payload: { id  } }) => {
        return { ...state, currentCredentialId: id };
    }
}, defaultState);

export const FetchCredentials = () => async (state, action) => {
    const data = await axios.get('/credentials')
};

export const CreateCredential = (credential: Partial<ICredential>) => async (dispatch, getState) => {
    const state = getState();
    const { CredentialsReducer: { credentials } } = state;
    debugger;

    try {
        const { data: { data = null, success = false} } = await axios.post('/credentials', credential);
        if (success) {

            debugger

            dispatch({ type: CredentialActions.SET_CREDENTIALS, payload: [data].concat(credentials) });
        } else {
            throw new Error(data);
        }
    } catch (e) {
        throw e;
    }
};

export const UpdateCredential = (credential: Partial<Credential>) => async (state, action) => {

};

export const DeleteCredential = (id: string) => async (state, action) => {};