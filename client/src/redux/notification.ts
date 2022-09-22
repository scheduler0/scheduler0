import uuidv1 from "uuid/v1";
import {createActions, handleActions} from "redux-actions";

export enum NotificationVariant {
    Default = "default",
    Success = "success",
    Error = "error",
    Info = "info"
}

export interface INotification {
    id: string,
    message: string
    variant: NotificationVariant,
}

export enum NotificationActions {
    ADD = "AddNotification",
    REMOVE = "RemoveNotification"
}

export const { addNotification, removeNotification } = createActions({
    [NotificationActions.ADD]: (message: string, variant: NotificationVariant = NotificationVariant.Success) => {
        return {
            notification: {
                id: uuidv1(),
                message,
                variant,
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