// @ts-ignore
import React from 'react'
import { hot } from 'react-hot-loader';
import CssBaseline from '@material-ui/core/CssBaseline';
import {WithStyles, withStyles} from '@material-ui/core/styles';
import AppBar from '@material-ui/core/AppBar';
import Toolbar from '@material-ui/core/Toolbar';
import Typography from '@material-ui/core/Typography';
import ExecutionsContainer from './containers/executions';
import CredentialContainer from './containers/credential';
import ProjectsContainer from './containers/project';
import JobContainer from './containers/job';
import NotificationContainer from './containers/notification';
import Drawer from '@material-ui/core/Drawer';
import {List} from "@material-ui/core";
import ListItem from "@material-ui/core/ListItem";
import ListItemText from "@material-ui/core/ListItemText";
import NoSsr from '@material-ui/core/NoSsr';
import { withRouter, Switch, Route } from 'react-router-dom';

const drawerWidth = 240;

const styles = theme => ({
    root: {
        display: 'flex',
        flexGrow: 1,
    },
    appBar: {
        width: `calc(100% - ${drawerWidth}px)`,
        marginLeft: drawerWidth,
    },
    menuButton: {
      marginRight: theme.spacing(2),
    },
    mainContainer: {
        flexGrow: 1
    },
    drawer: {
        width: drawerWidth,
        flexShrink: 0,
    },
    drawerPaper: {
        width: drawerWidth,
    },
});


type Props = WithStyles<ReturnType<typeof styles>>

class App extends React.Component<Props> {
    componentDidMount () {
        const jssStyles = document.querySelector('#jss-server-side');
        if (jssStyles) jssStyles.parentNode.removeChild(jssStyles);
    };

    render() {
        // @ts-ignore:
        const { classes, history } = this.props;

        return (
            <>
                <CssBaseline />
                <NoSsr>
                    <NotificationContainer />
                </NoSsr>
                <section className={classes.root}>
                    <AppBar position="fixed" className={classes.appBar}>
                        <Toolbar variant="dense">
                            <Typography variant="h6" color="inherit">
                                Cron Server
                            </Typography>
                        </Toolbar>
                    </AppBar>
                    <Drawer
                        className={classes.drawer}
                        variant="permanent"
                        classes={{ paper: classes.drawerPaper }}
                        anchor="left"
                    >
                        <List>
                            <ListItem button key={"Executions"} onClick={() => history.push('/')}>
                                <ListItemText primary={"Executions"} />
                            </ListItem>
                            <ListItem button key={"Credentials"} onClick={() => history.push('/credentials')}>
                                <ListItemText primary={"Credentials"} />
                            </ListItem>
                            <ListItem button key={"Projects"}  onClick={() => history.push('/projects')}>
                                <ListItemText primary={"Projects"} />
                            </ListItem>
                            <ListItem button key={"Jobs"} onClick={() => history.push('/jobs')}>
                                <ListItemText primary={"Jobs"} />
                            </ListItem>
                        </List>
                    </Drawer>
                    <main className={classes.mainContainer}>
                        <Switch>
                            <Route path="/" exact render={() => <ExecutionsContainer />} />
                            <Route path="/credentials" render={() => <CredentialContainer />} />
                            <Route path="/projects" render={() => <ProjectsContainer />} />
                            <Route path="/jobs" render={() => <JobContainer />} />
                        </Switch>
                    </main>
                </section>
            </>
        );
    }
}

export default hot(module)(
    withStyles(styles)(withRouter((App as any as React.ComponentType)))
);
