import React, {useState} from 'react';
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
        margin: theme.spacing(1),
        minWidth: 120,
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

const Executions = (props: Props) => {

    const { executions, classes } = props;

    const TimeFilterEntries = Object.entries(TimeFilters);
    const JobStatusEntries = Object.entries(JobStatus);

    const [filters, setFilters] = useState({ time: TimeFilterEntries[0][0], status: JobStatus.Success });
    const { time, status } = filters;

    const setFilterCallback = (filter: "time" | "status") => (e: React.ChangeEvent<{ value: unknown }>) => {
        setFilters((state) => ({
            ...state,
            [filter]: e.target.value
        }));
    };
    return (
        <Grid container>
            <Grid container className={classes.filterContainer} justify="center" alignItems="center">
                <Grid container className={classes.filterContainer} justify="flex-end" alignItems="center">
                    <FormControl className={classes.formControl}>
                        <InputLabel>Filter By. Time</InputLabel>
                        <Select onChange={setFilterCallback("time")} value={time}>
                            {TimeFilterEntries.map(([k, v], i) => (<MenuItem value={k} key={i}>{v}</MenuItem>))}
                        </Select>
                    </FormControl>
                    <FormControl className={classes.formControl}>
                        <InputLabel>Job Status</InputLabel>
                        <Select onChange={setFilterCallback("status")} value={status}>
                            {JobStatusEntries.map(([k, v], i) => (<MenuItem value={k} key={i}>{v}</MenuItem>))}
                        </Select>
                    </FormControl>
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
                            <Typography className={classes.label} variant={'body1'}>Avg. Timeouts</Typography>
                            <Typography variant={'h4'}>0</Typography>
                        </Paper>
                    </Grid>
                </Grid>
            </Grid>
            <Grid item md={12} lg={12}>
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