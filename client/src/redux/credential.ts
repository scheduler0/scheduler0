import { createActions, handleActions } from "redux-actions";
import {addNotification, NotificationVariant} from './notification';
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

export const {setCredentials, setCurrentCredentialId} = createActions({
    [CredentialActions.SET_CREDENTIALS]: (credentials = []) => ({ credentials }),
    [CredentialActions.SET_CURRENT_CREDENTIAL_ID]: (id: string) => ({ id })
});

export const credentialsReducer = handleActions({
    [CredentialActions.SET_CREDENTIALS]: (state,{ payload: { credentials } }) => {
        return {...state, credentials };
    },
    [CredentialActions.SET_CURRENT_CREDENTIAL_ID]: (state, { payload: { id } }) => {
        return { ...state, currentCredentialId: id };
    }
}, defaultState);

export const FetchCredentials = () => async (dispatch) => {
    try {
        const { data: { data: credentials, success } } = await axios.get('/credentials');
        if (success) {
            dispatch(setCredentials(credentials));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};

export const CreateCredential = (credential: Partial<ICredential>) => async (dispatch, getState) => {
    const state = getState();
    const { CredentialsReducer: { credentials } } = state;
    try {
        const { data: { data: newCredentialId = null, success = false} } = await axios.post('/credentials', credential);
        if (success) {
            const { data: { data: newCredential = null } } = await axios.get(`/credentials/${newCredentialId}`);
            dispatch(setCredentials(credentials.concat(newCredential)));
            dispatch(addNotification("Successfully credential created!"));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};

export const UpdateCredential = (credential: Partial<Credential>) => async (dispatch, getState) => {
    const state = getState();
    const { CredentialsReducer: { credentials, currentCredentialId } } = state;
    const credentialIndex = credentials.findIndex(({ id: credentialId }) => credentialId == currentCredentialId);

    try {
        const { data: { data: updatedCredential = null, success = false} } = await axios.put(`/credentials/${currentCredentialId}`, credential);
        const updatedCredentials = [...credentials];
        updatedCredentials[credentialIndex] = updatedCredential;
        if (success) {
            dispatch(setCredentials(updatedCredentials));
            dispatch(addNotification("Successfully updated credential!"));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};

export const DeleteCredential = (id: string) => async (dispatch, getState) => {
    const state = getState();
    const { CredentialsReducer: { credentials } } = state;
    try {
        const { data: {success = false} } = await axios.delete(`/credentials/${id}`);
        if (success) {
            dispatch(setCredentials(credentials.filter(({ id: credentialId }) => credentialId != id )));
            dispatch(addNotification("Successfully deleted credential!"));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};