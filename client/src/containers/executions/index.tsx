import React from 'react';
import { connect } from "react-redux";
import {createStyles, Typography, WithStyles, withStyles} from "@material-ui/core";
import Grid from "@material-ui/core/Grid";
import Table from "@material-ui/core/Table";
import TableHead from "@material-ui/core/TableHead";
import TableRow from "@material-ui/core/TableRow";
import TableCell from "@material-ui/core/TableCell";
import TableBody from "@material-ui/core/TableBody";

const styles = theme => createStyles({
    containerHeader: {
        height: "50px",
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        marginTop: '50px',
        padding: '20px'
    }
});

type Props = ReturnType<typeof mapStateToProps>
    & WithStyles<ReturnType<typeof styles>>


const Executions = (props: Props) => {
    const { executions } = props;
    return (
        <Grid container>
            <Grid item md={8} lg={8}>
                <div className={props.classes.containerHeader}>
                    <Typography variant="h5">Executions</Typography>
                </div>
                <Table>
                    <TableHead>
                        <TableRow>
                            <TableCell>Status Code</TableCell>
                            <TableCell>Response</TableCell>
                            <TableCell>Job Id</TableCell>
                            <TableCell>Timeout</TableCell>
                        </TableRow>
                    </TableHead>
                    <TableBody>
                        {executions.map((execution, i) => (
                            <TableRow key={`execution-${i}`}>
                                <TableCell>{execution.status_code}</TableCell>
                                <TableCell>{execution.response}</TableCell>
                                <TableCell>{execution.job_id}</TableCell>
                                <TableCell>{execution.timeout}</TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </Grid>
        </Grid>
    )
};

const mapStateToProps = (state) => ({
    executions: state.ExecutionsReducer.executions
});

const mapDispatchToProps = (_) => ({

});

export default connect(mapStateToProps, mapDispatchToProps)(withStyles(styles)(Executions)) as any as React.ComponentType;