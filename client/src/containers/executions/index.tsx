import React, {useCallback, useState} from 'react';
import { connect } from "react-redux";
import { createStyles, WithStyles, withStyles } from "@material-ui/core/styles";
import Grid from "@material-ui/core/Grid";
import FormControl from "@material-ui/core/FormControl";
import Typography from "@material-ui/core/Typography";
import Table from "@material-ui/core/Table";
import TableHead from "@material-ui/core/TableHead";
import TableRow from "@material-ui/core/TableRow";
import TableCell from "@material-ui/core/TableCell";
import TableBody from "@material-ui/core/TableBody";
import Paper from "@material-ui/core/Paper";
import Select from "@material-ui/core/Select";
import MenuItem from '@material-ui/core/MenuItem';
import InputLabel from "@material-ui/core/InputLabel";
import RefreshIcon from '@material-ui/icons/Refresh';
import Button  from "@material-ui/core/Button";
import {TablePagination} from "@material-ui/core";
import { compose } from "recompose";

const styles = theme => createStyles({
    containerHeader: {
        height: "50px",
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        padding: '20px'
    },
    label: {
        fontSize: '14px',
        color: 'rgba(0,0,0, 0.4)',
        textTransform: 'uppercase'
    },
    paper: {
        padding: '20px',
        width: '80%',
        height: '100px',
        boxSizing: 'border-box'
    },
    formControl: {
        minWidth: 120
    },
    filterContainer: {
        margin: '25px'
    },
    tagsContainer: {
        marginBottom: '25px'
    },
});

type Props = ReturnType<typeof mapStateToProps>
    & WithStyles<ReturnType<typeof styles>>


enum TimeFilters {
    PastHour = "Past Hour",
    PastDay = "Past Day",
    PastWeek = "Past Week",
    PastMonth = "Past Month",
    PastYear = "Past Year"
}


enum JobStatus {
    Success = "Success",
    Fail = "Fail"
}

const ResultsPerPage = [10, 50, 100, 150, 200, "ALL"];

const Executions = (props: Props) => {

    const { executions, classes } = props;

    const TimeFilterEntries = Object.entries(TimeFilters);
    const JobStatusEntries = Object.entries(JobStatus);
    const ResultsPerPageEntries = Object.entries(ResultsPerPage);

    const [filters, setFilters] = useState({
        results: ResultsPerPageEntries[0][0],
        time: TimeFilterEntries[0][0],
        status: JobStatus.Success,
    });
    const { results, time, status } = filters;

    const setFilterCallback = (filter: "time" | "status" | "results") => (e: React.ChangeEvent<{ value: unknown }>) => {
        setFilters((state) => ({
            ...state,
            [filter]: e.target.value
        }));
    };

    const handleChangePage = useCallback(() => {}, []);
    const handleChangeRowsPerPage = useCallback(() => {}, []);

    return (
        <Grid container>
            <Grid container className={classes.filterContainer} justify="center" alignItems="center">
                <Grid container direction="row" className={classes.filterContainer} justify="space-between" alignItems="center">
                    <Grid item md={3} lg={3}>
                        <Button>
                            <Typography>Refresh</Typography>
                            <RefreshIcon />
                        </Button>
                    </Grid>
                    <Grid item md={3} lg={3}>
                        <FormControl className={classes.formControl} fullWidth>
                            <InputLabel>Filter By. Time</InputLabel>
                            <Select onChange={setFilterCallback("time")} value={time}>
                                {TimeFilterEntries.map(([k, v], i) => (<MenuItem value={k} key={i}>{v}</MenuItem>))}
                            </Select>
                        </FormControl>
                    </Grid>
                    <Grid item md={3} lg={3}>
                        <FormControl className={classes.formControl} fullWidth>
                            <InputLabel>Job Status</InputLabel>
                            <Select onChange={setFilterCallback("status")} value={status}>
                                {JobStatusEntries.map(([k, v], i) => (<MenuItem value={k} key={i}>{v}</MenuItem>))}
                            </Select>
                        </FormControl>
                    </Grid>
                </Grid>
            </Grid>
            <Grid container className={classes.tagsContainer}>
                <Grid item md={4} lg={4}>
                    <Grid container justify={"center"}>
                        <Paper className={classes.paper}>
                            <Typography className={classes.label} variant={'body1'}>No. Successful Jobs</Typography>
                            <Typography variant={'h4'}>0</Typography>
                        </Paper>
                    </Grid>
                </Grid>
                <Grid item md={4} lg={4}>
                    <Grid container justify={"center"}>
                        <Paper className={classes.paper}>
                            <Typography className={classes.label} variant={'body1'}>No. Failed Jobs</Typography>
                            <Typography variant={'h4'}>0</Typography>
                        </Paper>
                    </Grid>
                </Grid>
                <Grid item md={4} lg={4}>
                    <Grid container justify={"center"}>
                        <Paper className={classes.paper}>
                            <Typography className={classes.label} variant={'body1'}>Avg. Timeouts(ms)</Typography>
                            <Typography variant={'h4'}>0</Typography>
                        </Paper>
                    </Grid>
                </Grid>
            </Grid>
            <Grid item md={12} lg={12}>
                <Paper>
                    <TablePagination
                        rowsPerPageOptions={[5, 10, 25]}
                        component="div"
                        count={0}
                        rowsPerPage={10}
                        page={0}
                        onChangePage={handleChangePage}
                        onChangeRowsPerPage={handleChangeRowsPerPage}
                    />
                    <Table>
                        <TableHead>
                            <TableRow>
                                <TableCell>Status Code</TableCell>
                                <TableCell>Job ID</TableCell>
                                <TableCell>Timeouts</TableCell>
                            </TableRow>
                        </TableHead>
                        <TableBody>
                            {executions.map((execution, i) => (
                                <TableRow key={`execution-${i}`}>
                                    <TableCell>{execution.status_code}</TableCell>
                                    <TableCell>{execution.job_id}</TableCell>
                                    <TableCell>{execution.timeout}</TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                </Paper>
            </Grid>
        </Grid>
    )
};

const mapStateToProps = (state) => ({
    executions: state.ExecutionsReducer.executions
});

const mapDispatchToProps = (_) => ({});

export default compose(
    connect(mapStateToProps, mapDispatchToProps),
    withStyles(styles)
)(Executions) as any as React.ComponentType;
