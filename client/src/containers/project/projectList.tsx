import React, {useCallback} from "react";
import { makeStyles } from "@material-ui/core/styles";
import theme from '../../theme';
import {IProject} from "../../redux/projects";
import ProjectListItem from "./projectListItem";
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
    projects: IProject[]
    setCurrentProjectId: (id: string) => void
    formMode: FormMode
    deleteProject: (id: string) => Promise<void>
    setMode: (mode: FormMode) => void
}

const useStyles = makeStyles(theme => ({
    root: {
        padding: theme.spacing(3, 2),
    },
    container: {
        marginTop: '50px',
        padding: '20px',
    },
    header: {
        height: '50px',
        display: 'flex',
        flexDirection: 'row',
        justifyContent: 'space-between',
        alignItems: 'center'
    }
}));

function ProjectList(props: IProps) {
    const classes = useStyles(theme);
    const { projects, deleteProject, setCurrentProjectId, setMode } = props;

    const handleDelete = useCallback((projectId) => {
        deleteProject(projectId);
    }, []);

    return (
        <div className={classes.container}>
            <Box display="flex"
                 component="div"
                 flexDirection="row"
                 alignItems="center"
                 justifyContent="space-between"
                 style={{ paddingLeft: '20px', paddingRight: '20px', marginBottom: '10px' }}>
                <Typography variant="h5">Projects</Typography>
                <Button component="span" onClick={() => {
                    setCurrentProjectId(null);
                    setMode(FormMode.Create);
                }}>Create New Project</Button>
            </Box>
            <Table>
                <TableHead>
                    <TableRow>
                        <TableCell>Name</TableCell>
                        <TableCell align="right">Description</TableCell>
                        <TableCell align="right"></TableCell>
                        <TableCell align="right"></TableCell>
                    </TableRow>
                </TableHead>
                <TableBody>
                    {projects && projects.map((project, index) => (
                        <ProjectListItem
                            key={`project-${index}`}
                            project={project}
                            onDelete={handleDelete}
                            setCurrentProjectId={() => {
                                setCurrentProjectId(project.id);
                                setMode(FormMode.Edit);
                            }}
                        />
                    ))}
                </TableBody>
            </Table>
        </div>
    );
}

export default ProjectList;