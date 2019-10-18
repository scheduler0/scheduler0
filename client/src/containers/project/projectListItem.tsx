import React from "react";
import {IProject} from "../../redux/projects";
import ListItemText from "@material-ui/core/ListItemText";
import IconButton from "@material-ui/core/IconButton";
import Tooltip from "@material-ui/core/Tooltip"
import DeleteIcon from '@material-ui/icons/Delete';
import EditIcon from "@material-ui/icons/Edit";
import {ListItem} from "@material-ui/core";


interface IProps {
    project: IProject
    onDelete: (id) => void
    setCurrentProjectId: () => void
}

const ProjectListItem = (props: IProps) => {
    const { project, onDelete, setCurrentProjectId } = props;

    return (
        <div style={{ borderTop: '1px solid rgba(0,0,0, 0.1)' }}>
            <ListItem>
                <ListItemText
                    primary={project.name}
                    secondary={project.description}
                />
                <Tooltip title="Edit">
                    <IconButton onClick={setCurrentProjectId}>
                        <EditIcon/>
                    </IconButton>
                </Tooltip>
                <Tooltip title="Delete">
                    <IconButton onClick={() => onDelete(project.id)}>
                        <DeleteIcon />
                    </IconButton>
                </Tooltip>
            </ListItem>
        </div>
    );
};

export default ProjectListItem;