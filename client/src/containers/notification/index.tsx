// @ts-ignore:
import React, {useEffect, useState} from 'react';
import {useSnackbar} from 'notistack';
import {connect} from 'react-redux';
import {removeNotification, INotification} from '../../redux/notification'

type Props = ReturnType<typeof mapStateToProps> & ReturnType<typeof mapDispatchToProps>

const Notification = (props: Props) => {
    const { notifications, removeNotification } = props;
    const { enqueueSnackbar, closeSnackbar } = useSnackbar();
    const [notificationsMap, setNotifications] = useState(new Map<string, INotification>());

    useEffect(() => {
        notifications.forEach((notification) => {
            if (!notificationsMap.has(notification.id)) {
                enqueueSnackbar(notification.message, {
                    key: notification.id,
                    variant: notification.variant,
                    onClose: ((N) => () => {
                        closeSnackbar(notification.id);
                        notificationsMap.delete(N.id);
                        removeNotification(N.id);
                        setNotifications(notificationsMap);
                    })(notification)
                });
                notificationsMap.set(notification.id, notification);
            }
        });

    }, [notifications]);


    return null;
};

const mapStateToProps = (state) => ({
    notifications: state.NotificationReducer.notifications,
});

const mapDispatchToProps = (dispatch) => ({
   removeNotification: (id: string) => dispatch(removeNotification(id))
});

export default connect(mapStateToProps, mapDispatchToProps)(Notification);
