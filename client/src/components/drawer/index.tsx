import React from 'react';
import clsx from 'clsx';
import {createStyles, useTheme, withStyles, WithStyles} from '@material-ui/core/styles';
import Drawer from '@material-ui/core/Drawer';
import AppBar from '@material-ui/core/AppBar';
import Toolbar from '@material-ui/core/Toolbar';
import List from '@material-ui/core/List';
import Typography from '@material-ui/core/Typography';
import Divider from '@material-ui/core/Divider';
import IconButton from '@material-ui/core/IconButton';
import MenuIcon from '@material-ui/icons/Menu';
import ChevronLeftIcon from '@material-ui/icons/ChevronLeft';
import ChevronRightIcon from '@material-ui/icons/ChevronRight';
import ListItem from '@material-ui/core/ListItem';
import ListItemIcon from '@material-ui/core/ListItemIcon';
import ListItemText from '@material-ui/core/ListItemText';
import AccountTreeIcon from '@material-ui/icons/AccountTree';
import WorkIcon from '@material-ui/icons/Work';
import AccessTimeIcon from '@material-ui/icons/AccessTime';
import VpnKeyIcon from '@material-ui/icons/VpnKey';

import { RouteComponentProps, withRouter } from 'react-router-dom';
import {Grid} from "@material-ui/core";

const drawerWidth = 240;

const styles = (theme) => createStyles({
    root: {
        display: 'flex',
        width: '100vw'
    },
    appBar: {
        zIndex: theme.zIndex.drawer + 1,
        transition: theme.transitions.create(['width', 'margin'], {
            easing: theme.transitions.easing.sharp,
            duration: theme.transitions.duration.leavingScreen,
        }),
    },
    appBarShift: {
        marginLeft: drawerWidth,
        width: `calc(100% - ${drawerWidth}px)`,
        transition: theme.transitions.create(['width', 'margin'], {
            easing: theme.transitions.easing.sharp,
            duration: theme.transitions.duration.enteringScreen,
        }),
    },
    menuButton: {
        marginRight: 36,
    },
    hide: {
        display: 'none',
    },
    drawer: {
        width: drawerWidth,
        flexShrink: 0,
        whiteSpace: 'nowrap',
    },
    drawerOpen: {
        width: drawerWidth,
        transition: theme.transitions.create('width', {
            easing: theme.transitions.easing.sharp,
            duration: theme.transitions.duration.enteringScreen,
        }),
    },
    drawerClose: {
        transition: theme.transitions.create('width', {
            easing: theme.transitions.easing.sharp,
            duration: theme.transitions.duration.leavingScreen,
        }),
        overflowX: 'hidden',
        width: theme.spacing(7) + 1,
        [theme.breakpoints.up('sm')]: {
            width: theme.spacing(9) + 1,
        },
    },
    toolbar: {
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: theme.spacing(0, 1),
        ...theme.mixins.toolbar,
    },
    content: {
        width: '100%',
        flexGrow: 1
    },
    sideMenuHeader: {
        marginLeft: '20px'
    },
    appName: {
        color: 'rgba(0,0,0, 0.6)',
        fontSize: '14px'
    },
    versionText: {
        color: 'rgba(0,0,0, 0.3)',
        fontSize: '12px'
    }
});

interface IProps {
    children: React.ReactNode
}

type Props = IProps & WithStyles<typeof styles> & RouteComponentProps;

function MiniDrawerWrapper(props: Props) {
    const { children, classes, history } = props;

    const theme = useTheme();
    const [open, setOpen] = React.useState(false);
    const [currentPageTitle, setCurrentPageTitle] = React.useState("Executions");
    const pages = ['Executions', 'Credentials', 'Projects', 'Jobs'];
    const handleDrawerOpen = () => {
        setOpen(true);
    };

    const handleDrawerClose = () => {
        setOpen(false);
    };

    const handleMainLinkClick = (page: number) => () => {
        const mainLinks = ['/', '/credentials', '/projects', '/jobs'];
        history.push(mainLinks[page]);
        setCurrentPageTitle(pages[page]);
    };

    return (
        <div className={classes.root}>
            <AppBar
                position="fixed"
                className={clsx(classes.appBar, {
                    [classes.appBarShift]: open,
                })}
            >
                <Toolbar>
                    <IconButton
                        color="inherit"
                        aria-label="open drawer"
                        onClick={handleDrawerOpen}
                        edge="start"
                        className={clsx(classes.menuButton, {
                            [classes.hide]: open,
                        })}
                    >
                        <MenuIcon />
                    </IconButton>
                    <Typography variant="h6" noWrap>
                        {currentPageTitle}
                    </Typography>
                </Toolbar>
            </AppBar>
            <Drawer
                variant="permanent"
                className={clsx(classes.drawer, {
                    [classes.drawerOpen]: open,
                    [classes.drawerClose]: !open,
                })}
                classes={{
                    paper: clsx({
                        [classes.drawerOpen]: open,
                        [classes.drawerClose]: !open,
                    }),
                }}
                open={open}
            >
                <div className={classes.toolbar}>
                    <Grid className={classes.sideMenuHeader}>
                        <Typography className={classes.appName}>CronServer</Typography>
                        <Typography variant="body1" className={classes.versionText}>0.0.1</Typography>
                    </Grid>
                    <IconButton onClick={handleDrawerClose}>
                        {theme.direction === 'rtl' ? <ChevronRightIcon /> : <ChevronLeftIcon />}
                    </IconButton>
                </div>
                <Divider />
                <List>
                    {pages.map((text, index) => (
                        <ListItem button key={text} onClick={handleMainLinkClick(index)}>
                            {index === 0 && <ListItemIcon>
                                <AccessTimeIcon />
                            </ListItemIcon>}
                            {index === 1 && <ListItemIcon>
                                <VpnKeyIcon />
                            </ListItemIcon>}
                            {index === 2 && <ListItemIcon>
                                <AccountTreeIcon />
                            </ListItemIcon>}
                            {index === 3 && <ListItemIcon>
                                <WorkIcon />
                            </ListItemIcon>}
                            <ListItemText primary={text} />
                        </ListItem>
                    ))}
                </List>
            </Drawer>
            <main className={classes.content}>
                <div className={classes.toolbar} />
                <div style={{ width: '100%' }}>
                    {children}
                </div>
            </main>
        </div>
    );
}

export default withStyles(styles)(withRouter(MiniDrawerWrapper)) as React.ComponentType<IProps>;
