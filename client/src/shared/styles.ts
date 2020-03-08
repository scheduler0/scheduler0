import {createStyles} from "@material-ui/core";

const styles = theme => createStyles({
    containerHeader: {
        height: "50px",
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
    },
    paper: {
        height: '100vh',
        paddingTop: '60px',
        marginTop: '-60px'
    }
});

export default styles;