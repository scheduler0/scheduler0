//@ts-ignore:
import React from "react";
import {IJob} from "../../redux/jobs";
import IconButton from "@material-ui/core/IconButton";
import Tooltip from "@material-ui/core/Tooltip"
import DeleteIcon from '@material-ui/icons/Delete';
import EditIcon from "@material-ui/icons/Edit";
import TableRow from "@material-ui/core/TableRow";
import TableCell from "@material-ui/core/TableCell";


interface IProps {
    job: IJob
    onDelete: (id) => void
    setCurrentJobId: () => void
}

const JobListItem = (props: IProps) => {
    const {job, onDelete, setCurrentJobId} = props;

    const { description, cron_spec, last_status_code, next_time, callback_url } = job;


    return (
        <TableRow>
            <TableCell>
                {description}
            </TableCell>
            <TableCell align="right">
                {cron_spec}
            </TableCell>
            <TableCell align="right">
                <p>{last_status_code}</p>
            </TableCell>
            <TableCell align="right">
                <p>{next_time}</p>
            </TableCell>
            <TableCell align="right">
                <p>{callback_url}</p>
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

export default JobListItem;