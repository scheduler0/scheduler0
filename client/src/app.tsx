// @ts-ignore
import React from 'react'
import { hot } from 'react-hot-loader';
import CssBaseline from '@material-ui/core/CssBaseline';
import {WithStyles, withStyles} from '@material-ui/core/styles';
import ExecutionsContainer from './containers/executions';
import CredentialContainer from './containers/credential';
import ProjectsContainer from './containers/project';
import JobContainer from './containers/job';
import NotificationContainer from './containers/notification';
import NoSsr from '@material-ui/core/NoSsr';
import { withRouter, Switch, Route } from 'react-router-dom';

import MiniDrawerWrapper from './components/drawer'

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
                    <MiniDrawerWrapper>
                        <Switch>
                            <Route path="/" exact render={() => <ExecutionsContainer />} />
                            <Route path="/credentials" render={() => <CredentialContainer />} />
                            <Route path="/projects" render={() => <ProjectsContainer />} />
                            <Route path="/jobs" render={() => <JobContainer />} />
                        </Switch>
                    </MiniDrawerWrapper>
                </section>
            </>
        );
    }
}

export default hot(module)(
    withStyles(styles)(withRouter((App as any as React.ComponentType)))
);
