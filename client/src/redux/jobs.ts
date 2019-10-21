import { createActions, handleActions } from "redux-actions";
import {addNotification, NotificationVariant} from './notification';
import axios from "axios";

export enum JobState {
    InActive = 1,
    Active = 2,
    Stale = -1,
}

export interface IJob {
    id: string
    project_id: string
    description: string
    cron_spec: string
    total_execs: number
    data: string
    timezone: string
    callback_url: string
    last_status_code: number
    state: JobState
    start_date: string
    end_date: string
    next_time: string
    date_created: string
}


export enum JobActions {
    SET_JOBS = "SET_JOBS",
    SET_CURRENT_JOB_ID = "SET_CURRENT_JOB_ID"
}

const defaultState = {
    jobs: [],
    currentJobId: null
};

export const {setJobs, setCurrentJobId} = createActions({
    [JobActions.SET_JOBS]: (jobs = []) => ({ jobs }),
    [JobActions.SET_CURRENT_JOB_ID]: (id: string) => ({ id })
});

export const jobsReducer = handleActions({
    [JobActions.SET_JOBS]: (state,{ payload: { jobs } }) => {
        return {...state, jobs };
    },
    [JobActions.SET_CURRENT_JOB_ID]: (state, { payload: { id } }) => {
        return { ...state, currentJobId: id };
    }
}, defaultState);

export const FetchJobs = () => async (dispatch) => {
    try {
        const { data: { data: jobs, success } } = await axios.get('/api/jobs');
        if (success) {
            dispatch(setJobs(jobs));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};

export const CreateJob = (job: Partial<IJob>) => async (dispatch, getState) => {
    const state = getState();
    const { JobsReducer: { jobs } } = state;
    debugger;
    try {
        const { data: { data: newJobId = null, success = false} } = await axios.post('/api/jobs', job);
        if (success) {
            const { data: { data: newJob = null } } = await axios.get(`/api/jobs/${newJobId}`);
            dispatch(setJobs(jobs.concat(newJob)));
            dispatch(addNotification("Successfully job created!"));
        }
    } catch (e) {
        console.log(e);
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};

export const UpdateJob = (job: Partial<IJob>) => async (dispatch, getState) => {
    const state = getState();
    const { JobsReducer: { jobs, currentJobId } } = state;
    const jobIndex = jobs.findIndex(({ id: jobId }) => jobId == currentJobId);

    try {
        const { data: { data: updatedJob = null, success = false} } = await axios.put(`/api/jobs/${currentJobId}`, job);
        const updatedJobs = [...jobs];
        updatedJobs[jobIndex] = updatedJob;
        if (success) {
            dispatch(setJobs(updatedJobs));
            dispatch(addNotification("Successfully updated job!"));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};

export const DeleteJob = (id: string) => async (dispatch, getState) => {
    const state = getState();
    const { JobsReducer: { jobs } } = state;
    try {
        const { data: {success = false} } = await axios.delete(`/api/jobs/${id}`);
        if (success) {
            dispatch(setJobs(jobs.filter(({ id: jobId }) => jobId != id )));
            dispatch(addNotification("Successfully deleted job!"));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};