import uuidv1 from "uuid/v1";
import {createActions, handleActions} from "redux-actions";

export interface INotification {
    id: string,
    message: string
}

export enum NotificationActions {
    ADD = "AddNotification",
    REMOVE = "RemoveNotification"
}

export const { addNotification, removeNotification } = createActions({
    [NotificationActions.ADD]: (notification: string) => {
        return {
            notification: {
                message: notification,
                id: uuidv1()
            }
        }
    },
    [NotificationActions.REMOVE]: (notificationId: string) => ({ notificationId })
});

export const notificationReducer = handleActions({
    [NotificationActions.ADD]: (state, { payload: { notification } }) => {
        return {
            ...state,
            notifications: [ ...state.notifications, notification]
        };
    },
    [NotificationActions.REMOVE]: (state, { payload: { notificationId } }) => {
        return {
            ...state,
            notifications: state.notifications.filter(({ id }) => (notificationId != id))
        };
    }
}, { notifications: [] });