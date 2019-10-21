import React from "react";
import {IProject} from "../../redux/projects";
import ListItemText from "@material-ui/core/ListItemText";
import IconButton from "@material-ui/core/IconButton";
import Tooltip from "@material-ui/core/Tooltip"
import DeleteIcon from '@material-ui/icons/Delete';
import EditIcon from "@material-ui/icons/Edit";
import {ListItem} from "@material-ui/core";
import TableRow from "@material-ui/core/TableRow";
import TableCell from "@material-ui/core/TableCell";


interface IProps {
    project: IProject
    onDelete: (id) => void
    setCurrentProjectId: () => void
}

const ProjectListItem = (props: IProps) => {
    const { project, onDelete, setCurrentProjectId } = props;

    return (
        <TableRow>
            <TableCell>
                {project.name}
            </TableCell>
            <TableCell align="right">
                {project.description}
            </TableCell>
            <TableCell align="right">
                <Tooltip title="Edit">
                    <IconButton onClick={setCurrentProjectId}>
                        <EditIcon/>
                    </IconButton>
                </Tooltip>
            </TableCell>
            <TableCell align="right">
                <Tooltip title="Delete">
                    <IconButton onClick={() => onDelete(project.id)}>
                        <DeleteIcon />
                    </IconButton>
                </Tooltip>
            </TableCell>
        </TableRow>
    );
};

export default ProjectListItem;