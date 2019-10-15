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

createActions({
    [CredentialActions.SET_CREDENTIALS]: (credentials = []) => ({ credentials }),
    [CredentialActions.SET_CURRENT_CREDENTIAL_ID]: (id: string) => ({ currentCredentialId: id })
});

export const credentialsReducer = handleActions({
    [CredentialActions.SET_CREDENTIALS]: (state,{ payload: { credentials } }) => {
        return {...state, credentials };
    },
    [CredentialActions.SET_CURRENT_CREDENTIAL_ID]: (state, { payload: { id  } }) => {
        return { ...state, currentCredentialId: id };
    }
}, defaultState);

export const FetchCredentials = () => async (dispatch) => {
    try {
        const { data: { data: credentials, success } } = await axios.get('/credentials');
        if (success) {
            dispatch({
                type: CredentialActions.SET_CREDENTIALS,
                payload: { credentials: credentials }
            });
        }
    } catch (e) {
        throw e;
    }
};

export const CreateCredential = (credential: Partial<ICredential>) => async (dispatch, getState) => {
    const state = getState();
    const { CredentialsReducer: { credentials } } = state;

    try {
        const { data: { data: newCredentialId = null, success = false} } = await axios.post('/credentials', credential);
        if (success) {
            const { data: { data: newCredential = null, success = false } } = await axios.get(`/credentials/${newCredentialId}`);
            dispatch({
                type: CredentialActions.SET_CREDENTIALS,
                payload: { credentials: credentials.concat(newCredential) }
            });
        }
    } catch (e) {
        throw e;
    }
};

export const UpdateCredential = (credential: Partial<Credential>) => async (dispatch, getState) => {
    const state = getState();
    const { CredentialsReducer: { credentials, currentCredentialId } } = state;
    const credentialIndex = credentials.findIndex(({ id: credentialId }) => credentialId == currentCredentialId);

    try {
        const { data: { data: updatedCredential = null, success = false} } = await axios.put(`/credentials/${currentCredentialId}`, credential);
        const updatedCredentials = [...credentials, updatedCredential];
        updatedCredentials[credentialIndex] = updatedCredential;

        if (success) {
            dispatch({
                type: CredentialActions.SET_CREDENTIALS,
                payload: { credentials: updatedCredentials }
            });
        }
    } catch (e) {
        throw e;
    }
};

export const DeleteCredential = (id: string) => async (dispatch, getState) => {
    const state = getState();
    const { CredentialsReducer: { credentials } } = state;
    try {
        const { data: { success = false} } = await axios.delete(`/credentials/${id}`);
        if (success) {
            dispatch({
                type: CredentialActions.SET_CREDENTIALS,
                payload: { credentials: credentials.filter(({ id: credentialId }) => credentialId != id )}
            });
        }
    } catch (e) {
        throw e;
    }
};