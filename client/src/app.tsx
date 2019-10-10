import React, { useEffect } from 'react'
import { hot } from 'react-hot-loader';
import {createStyles, makeStyles, WithStyles, withStyles} from '@material-ui/core/styles';
import AppBar from '@material-ui/core/AppBar';
import Toolbar from '@material-ui/core/Toolbar';
import Typography from '@material-ui/core/Typography';
import IconButton from '@material-ui/core/IconButton';
import MenuIcon from '@material-ui/icons/Menu';
import Store from "./models/store";
import CredentialContainer from './containers/credential';

import theme from './theme';
import {inject, observer, Provider} from "mobx-react";
import {ThemeProvider} from "@material-ui/styles";
import {CredentialState, ICredential, ICredentialModel} from "./models/credential";

const styles = theme => ({
    root: {
      flexGrow: 1,
    },
    menuButton: {
      marginRight: theme.spacing(2),
    },
    mainContainer: {
        paddingTop: '60px',
        paddingLeft: '20px',
        paddingRight: '20px'
    }
});


type Props = WithStyles<ReturnType<typeof styles>> & { rootStore: ICredentialModel }

class App extends React.Component<Props> {
    componentDidMount () {
        const jssStyles = document.querySelector('#jss-server-side');
        if (jssStyles) jssStyles.parentNode.removeChild(jssStyles);
    };

    render() {
        const { classes } = this.props;

        console.log(this.props);

        return (
            <section className={classes.root}>
                <AppBar position="fixed">
                    <Toolbar variant="dense">
                        <IconButton edge="start" className={classes.menuButton} color="inherit" aria-label="menu">
                            <MenuIcon />
                        </IconButton>
                        <Typography variant="h6" color="inherit">
                            Cron Server
                        </Typography>
                    </Toolbar>
                </AppBar>
                <section className={classes.mainContainer}>
                    <CredentialContainer rootStore={this.props.rootStore} />
                </section>
            </section>
        );
    }
}

export default hot(module)(withStyles(styles)(observer(App)));
