//@ts-ignore:
import React from "react";
import {IJob} from "../../redux/jobs";
import IconButton from "@material-ui/core/IconButton";
import Tooltip from "@material-ui/core/Tooltip"
import DeleteIcon from '@material-ui/icons/Delete';
import EditIcon from "@material-ui/icons/Edit";
import TableRow from "@material-ui/core/TableRow";
import TableCell from "@material-ui/core/TableCell";
import { formatDistanceToNow } from "date-fns";
import {createStyles, WithStyles} from "@material-ui/core";
import withStyles from "@material-ui/core/styles/withStyles";

interface IProps {
    job: IJob
    onDelete: (id) => void
    setCurrentJobId: () => void
}


const styles = (theme) => createStyles({
   date: {
       color: 'rgba(0,0,0, 0.3)',
       fontSize: '10px'
   }
});

const JobListItem = (props: IProps & WithStyles<ReturnType<typeof styles>>) => {
    const {classes, job, onDelete, setCurrentJobId} = props;

    const { description, cron_spec, start_date, end_date, next_time, callback_url } = job;


    return (
        <TableRow>
            <TableCell>
                {description}
            </TableCell>
            <TableCell align="right">
                {cron_spec}
            </TableCell>
            <TableCell align="right">
                {start_date}
                <br />
                <br />
                {formatDistanceToNow(new Date(start_date), {addSuffix: true, includeSeconds: true})}
            </TableCell>
            <TableCell align="right">
                {end_date}
                <br />
                <br />
                {formatDistanceToNow(new Date(end_date), {addSuffix: true, includeSeconds: true})}
            </TableCell>
            <TableCell align="right">
                <span className={classes.date}>{next_time}</span>
                <br />
                <br />
                <span className={classes.date}>
                    {formatDistanceToNow(new Date(next_time), {addSuffix: true, includeSeconds: true})}
                </span>
            </TableCell>
            <TableCell align="right">
                {callback_url}
            </TableCell >
            <TableCell align="right">
                <Tooltip title="Edit">
                    <IconButton onClick={setCurrentJobId}>
                        <EditIcon/>
                    </IconButton>
                </Tooltip>
            </TableCell>
            <TableCell align="right">
                <Tooltip title="Delete">
                    <IconButton onClick={() => onDelete(job.id)}>
                        <DeleteIcon />
                    </IconButton>
                </Tooltip>
            </TableCell>
        </TableRow>
    );
};

export default withStyles(styles)(JobListItem);