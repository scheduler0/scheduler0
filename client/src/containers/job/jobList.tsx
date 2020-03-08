import React, {useCallback} from "react";
import { makeStyles } from "@material-ui/core/styles";
import theme from '../../theme';
import {IJob} from "../../redux/jobs";
import JobListItem from "./jobListItem";
import Box from "@material-ui/core/Box";
import {Typography} from "@material-ui/core";
import Button from "@material-ui/core/Button";
import {FormMode} from "./index";
import Table from "@material-ui/core/Table";
import TableHead from "@material-ui/core/TableHead";
import TableRow from "@material-ui/core/TableRow";
import TableCell from "@material-ui/core/TableCell";
import TableBody from "@material-ui/core/TableBody";

interface IProps {
    jobs: IJob[]
    setCurrentJobId: (id: string) => void
    formMode: FormMode
    deleteJob: (id: string) => Promise<void>
    setMode: (mode: FormMode) => void
}

const useStyles = makeStyles(theme => ({
    root: {},
    container: {},
    header: {
        height: '50px',
        display: 'flex',
        flexDirection: 'row',
        justifyContent: 'space-between',
        alignItems: 'center'
    }
}));

function JobList(props: IProps) {
    const classes = useStyles(theme);
    const { jobs, deleteJob, setCurrentJobId, setMode } = props;

    const handleDelete = useCallback((jobId) => {
        deleteJob(jobId);
    }, []);

    return (
        <div className={classes.container}>
            <Box display="flex"
                 component="div"
                 flexDirection="row"
                 alignItems="center"
                 justifyContent="flex-end"
                 style={{ margin: '25px 0px', padding: '0px 20px' }}>
                <Button component="span" onClick={() => {
                    setCurrentJobId(null);
                    setMode(FormMode.Create);
                }}>Create New Job</Button>
            </Box>
            <Table>
                <TableHead>
                    <TableRow>
                        <TableCell>Description</TableCell>
                        <TableCell align="right">Cron Spec</TableCell>
                        <TableCell align="right">Start Date</TableCell>
                        <TableCell align="right">End Date</TableCell>
                        <TableCell align="right">Next Time</TableCell>
                        <TableCell align="right">Callback Url</TableCell>
                        <TableCell align="right"></TableCell>
                        <TableCell align="right"></TableCell>
                    </TableRow>
                </TableHead>
                <TableBody>
                    {jobs.map((job, index) => (
                        <JobListItem
                            key={`job-${index}`}
                            job={job}
                            onDelete={handleDelete}
                            setCurrentJobId={() => {
                                setCurrentJobId(job.id);
                                setMode(FormMode.Edit);
                            }}
                        />
                    ))}
                </TableBody>
            </Table>
        </div>
    );
}

export default JobList;
