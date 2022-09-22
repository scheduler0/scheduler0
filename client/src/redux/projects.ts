import { createActions, handleActions } from "redux-actions";
import {addNotification, NotificationVariant} from './notification';
import axios from "axios";

export interface IProject {
    name: string
    description: string
    id: string
    date_created: string
}

export enum ProjectActions {
    SET_PROJECTS = "SET_PROJECTS",
    SET_CURRENT_PROJECT_ID = "SET_CURRENT_PROJECT_ID"
}

const defaultState = {
    projects: [],
    offset: 0,
    limit: 100,
    total: 0,
    currentProjectId: null
};


export type Paginated<T> = {
    total: number,
    offset: number,
    limit: number,
    data: T[]
}

export const {setProjects, setCurrentProjectId} = createActions({
    [ProjectActions.SET_PROJECTS]: ( data: Paginated<IProject>) => (data),
    [ProjectActions.SET_CURRENT_PROJECT_ID]: (id: string) => ({ id })
});

export const projectsReducer = handleActions({
    [ProjectActions.SET_PROJECTS]: (state,{ payload: { projects } }) => {
        return {...state, projects };
    },
    [ProjectActions.SET_CURRENT_PROJECT_ID]: (state, { payload: { id } }) => {
        return { ...state, currentProjectId: id };
    }
}, defaultState);

export const FetchProjects = () => async (dispatch, getState) => {
    const { ProjectsReducer: { offset, limit } } = getState()

    try {
        const { data: { data: projects, success } } = await axios.get(`/projects?offset=${offset}&limit=${limit}`);
        if (success) {
            dispatch(setProjects(projects));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};

export const CreateProject = (project: Partial<IProject>) => async (dispatch, getState) => {
    const state = getState();
    const { ProjectsReducer: { projects } } = state;
    try {
        const { data: { data: newProjectId = null, success = false} } = await axios.post('/api/projects', project);
        if (success) {
            const { data: { data: newProject = null } } = await axios.get(`/api/projects/${newProjectId}`);
            dispatch(setProjects(projects.concat(newProject)));
            dispatch(addNotification("Successfully project created!"));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};

export const UpdateProject = (project: Partial<IProject>) => async (dispatch, getState) => {
    const state = getState();
    const { ProjectsReducer: { projects, currentProjectId } } = state;
    const projectIndex = projects.findIndex(({ id: projectId }) => projectId == currentProjectId);

    try {
        const { data: { data: updatedProject = null, success = false} } = await axios.put(`/api/projects/${currentProjectId}`, project);
        const updatedProjects = [...projects];
        updatedProjects[projectIndex] = updatedProject;
        if (success) {
            dispatch(setProjects(updatedProjects));
            dispatch(addNotification("Successfully updated project!"));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};

export const DeleteProject = (id: string) => async (dispatch, getState) => {
    const state = getState();
    const { ProjectsReducer: { projects } } = state;
    try {
        const { data: {success = false} } = await axios.delete(`/api/projects/${id}`);
        if (success) {
            dispatch(setProjects(projects.filter(({ id: projectId }) => projectId != id )));
            dispatch(addNotification("Successfully deleted project!"));
        }
    } catch (e) {
        dispatch(addNotification(e.response.data.data, NotificationVariant.Error));
    }
};
