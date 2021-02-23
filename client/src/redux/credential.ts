import { createActions, handleActions } from "redux-actions";
import {addNotification, NotificationVariant} from './notification';
import axios from "axios";
import {Paginated} from "./projects";

export enum CredentialActions {
    SET_CREDENTIALS = "SET_CREDENTIALS",
    SET_CURRENT_CREDENTIAL_ID = "SET_CURRENT_CREDENTIAL_ID"
}

export interface ICredential {
    uuid: string
    api_key: string
    http_referrer_restriction: string
    date_created: string
}

const defaultState = {
    credentials: [],
    offset: 0,
    limit: 100,
    total: 0,
    currentCredentialId: null
};

export const {setCredentials, setCurrentCredentialId} = createActions({
    [CredentialActions.SET_CREDENTIALS]: (data: Paginated<ICredential>) => (data),
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
        const { data: { data: credentials, success } } = await axios.get('/api/credentials');
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
        const { data: { data, success } } = await axios.post('/api/credentials', credential);
        console.log(data)
        if (success) {
            dispatch(setCredentials({ credentials: credentials.concat(data) }));
            dispatch(addNotification("Successfully credential created!"));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};

export const UpdateCredential = (credential: Partial<Credential>) => async (dispatch, getState) => {
    const state = getState();
    const { CredentialsReducer: { credentials, currentCredentialId } } = state;
    const credentialIndex = credentials.findIndex(({ uuid: credentialId }) => credentialId == currentCredentialId);

    try {
        const { data: { data: updatedCredential = null, success = false} } = await axios.put(`/api/credentials/${currentCredentialId}`, credential);
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
        await axios.delete(`/api/credentials/${id}`);
        dispatch(setCredentials({
            credentials: credentials.filter(({ uuid: credentialId }) => credentialId != id )
        }));
        dispatch(addNotification("Successfully deleted credential!"));
    } catch (e) {
        dispatch(addNotification("unknown error occurred", NotificationVariant.Error));
    }
};
