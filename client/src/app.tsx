import React from 'react'
import { hot } from 'react-hot-loader';
import {WithStyles, withStyles} from '@material-ui/core/styles';
import AppBar from '@material-ui/core/AppBar';
import Toolbar from '@material-ui/core/Toolbar';
import Typography from '@material-ui/core/Typography';
import CredentialContainer from './containers/credential';
import Drawer from '@material-ui/core/Drawer';
import {List} from "@material-ui/core";
import ListItem from "@material-ui/core/ListItem";
import ListItemText from "@material-ui/core/ListItemText";

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
        flexGrow: 1,
        paddingTop: '60px'
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
        const { classes } = this.props;
        return (
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
                        <ListItem button key={"API Keys"}>
                            <ListItemText primary={"API Keys"} />
                        </ListItem>
                        <ListItem button key={"Projects"}>
                            <ListItemText primary={"Projects"} />
                        </ListItem>
                        <ListItem button key={"Jobs"}>
                            <ListItemText primary={"Jobs"} />
                        </ListItem>
                    </List>
                </Drawer>
                <main className={classes.mainContainer}>
                    <CredentialContainer />
                </main>
            </section>
        );
    }
}

export default hot(module)(withStyles(styles)(App));
