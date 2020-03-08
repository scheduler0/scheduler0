import { createActions, handleActions } from "redux-actions";
import {addNotification, NotificationVariant} from './notification';
import axios from "axios";

export enum ExecutionActions {
    SET_EXECUTIONS = "SET_EXECUTIONS",
    SET_CURRENT_EXECUTION_ID = "SET_CURRENT_EXECUTION_ID"
}

export interface IExecution {
    id: string
    job_id: string
    status_code: string
    timeout: number
    response: string
    date_created: string
}

const defaultState = {
    executions: [],
    currentExecutionId: null
};

export const {setExecutions, setCurrentExecutionId} = createActions({
    [ExecutionActions.SET_EXECUTIONS]: (executions = []) => ({ executions }),
    [ExecutionActions.SET_CURRENT_EXECUTION_ID]: (id: string) => ({ id })
});

export const executionsReducer = handleActions({
    [ExecutionActions.SET_EXECUTIONS]: (state,{ payload: { executions } }) => {
        return {...state, executions };
    },
    [ExecutionActions.SET_CURRENT_EXECUTION_ID]: (state, { payload: { id } }) => {
        return { ...state, currentExecutionId: id };
    }
}, defaultState);

export const FetchExecutions = () => async (dispatch) => {
    try {
        const { data: { data: executions, success } } = await axios.get('/api/executions');
        if (success) {
            dispatch(setExecutions(executions));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};

export const CreateExecution = (execution: Partial<IExecution>) => async (dispatch, getState) => {
    const state = getState();
    const { ExecutionsReducer: { executions } } = state;
    try {
        const { data: { data: newExecutionId = null, success = false} } = await axios.post('/api/executions', execution);
        if (success) {
            const { data: { data: newExecution = null } } = await axios.get(`/api/executions/${newExecutionId}`);
            dispatch(setExecutions(executions.concat(newExecution)));
            dispatch(addNotification("Successfully execution created!"));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};

export const UpdateExecution = (execution: Partial<IExecution>) => async (dispatch, getState) => {
    const state = getState();
    const { ExecutionsReducer: { executions, currentExecutionId } } = state;
    const executionIndex = executions.findIndex(({ id: executionId }) => executionId == currentExecutionId);

    try {
        const { data: { data: updatedExecution = null, success = false} } = await axios.put(`/api/executions/${currentExecutionId}`, execution);
        const updatedExecutions = [...executions];
        updatedExecutions[executionIndex] = updatedExecution;
        if (success) {
            dispatch(setExecutions(updatedExecutions));
            dispatch(addNotification("Successfully updated execution!"));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};

export const DeleteExecution = (id: string) => async (dispatch, getState) => {
    const state = getState();
    const { ExecutionsReducer: { executions } } = state;
    try {
        const { data: {success = false} } = await axios.delete(`/api/executions/${id}`);
        if (success) {
            dispatch(setExecutions(executions.filter(({ id: executionId }) => executionId != id )));
            dispatch(addNotification("Successfully deleted execution!"));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};