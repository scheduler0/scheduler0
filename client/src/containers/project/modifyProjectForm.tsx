import React, {useCallback, useEffect, useState} from "react";
import {connect} from "react-redux";
import {CreateProject, IProject, UpdateProject} from "../../redux/projects";
import {makeStyles, Typography} from "@material-ui/core";
import Button from "@material-ui/core/Button";
import TextField from "@material-ui/core/TextField";
import theme from "../../theme";
import {FormMode} from "./index";
import Box from "@material-ui/core/Box";

interface IProps {
    formMode?: FormMode
    currentProjectId?: string
    setCurrentProjectId?: (id: string) => void
    setMode: (mode: FormMode) => void
    currentProject?: IProject
}

const useStyles = makeStyles((theme) => ({
    container: {
        marginTop: '50px',
        backgroundColor: '#f1f1f1',
        height: 'calc(100vh - 50px)',
        padding: '30px',
        position: "sticky",
        top: 50,
        left: 0,
    },
    innerContainer: {
        marginTop: '20px',
        marginBottom: '30px',
    }
}));

const ProjectsForm = (props: IProps & ReturnType<typeof mapDispatchToProps>) => {
    const classes = useStyles(theme);
    const [state, setState] = useState({
        name: "",
        description: "",
        id: "",
        date_created: "",
    });
    const { formMode, currentProject = null, setMode, setCurrentProjectId } = props;
    const { name, description } = state;

    useEffect(() => {
        setState((state) => ({
            ...state,
            name: currentProject ? currentProject.name : "",
            description: currentProject ? currentProject.description : ""
        }));
    }, [currentProject]);

    const handleSubmit = useCallback((e: React.FormEvent<HTMLFormElement>) => {
        async function submitProject({ description, name }) {
            const project: Partial<IProject> = {};

            project.description = description;
            project.name = name;

            const { formMode } = props;

            try {
                if (formMode == FormMode.Create) {
                    await props.createProject(project);
                } else {
                    await props.updateProject(project);
                }
            } catch (e) {
                console.error(e);
            }
        }

        e.preventDefault();

        const form = e.currentTarget;
        const formValidity = form.checkValidity();

        if (formValidity) {
            submitProject({
                name: e.currentTarget.project_name.value,
                description: e.currentTarget.project_description.value
            });

            setState((state) => ({
                ...state,
                name: "",
                description: ""
            }));
        }
    }, [formMode]);
    const cancelCallback = useCallback(() => {
        if (formMode == FormMode.Edit) {
            setCurrentProjectId(null);
        }

        setMode(FormMode.None);
    }, [formMode]);
    const setFormValue = useCallback((formName, value) => {
        if (formName === 'project_name') {
            setState((state) => ({ ...state, "name": value }));
        }

        if (formName === 'project_description') {
            setState((state) => ({ ...state, "description": value }));
        }
    }, []);

    if (formMode == FormMode.None) {
        return null
    }

    return (
        <div className={classes.container}>
            {(formMode == FormMode.Create) ?
                <Typography variant="h6">New Project</Typography>:
                <Typography variant="h6">Edit Project</Typography>}
            <div>
                <form onSubmit={handleSubmit} name="new-project">
                    <div className={classes.innerContainer}>
                        <TextField
                            fullWidth
                            required
                            value={name}
                            onChange={(e) => {
                                setFormValue(e.currentTarget.name, e.currentTarget.value);
                                e.persist();
                            }}
                            type="text"
                            name="project_name"
                            label="Name"
                        />
                        <br />
                        <br />
                        <TextField
                            fullWidth
                            required
                            value={description}
                            onChange={(e) => {
                                setFormValue(e.currentTarget.name, e.currentTarget.value);
                                e.persist();
                            }}
                            type="text"
                            name="project_description"
                            label="Description"
                        />
                    </div>
                    <Box display="flex" flexDirection="row" justifyContent="flex-start" alignItems="center">
                        <Button type="submit" variant="contained">Submit</Button>
                        &nbsp;&nbsp;&nbsp;&nbsp;
                        <Button component="span" onClick={cancelCallback}>Cancel</Button>
                    </Box>
                </form>
            </div>
        </div>
    );
};

const mapDispatchToProps = (dispatch) => ({
    createProject: (project: Partial<IProject>) => dispatch(CreateProject(project)),
    updateProject: (project: Partial<IProject>) => dispatch(UpdateProject(project)),
});

const mapStateToProps = (state) => ({});

export default connect(mapStateToProps, mapDispatchToProps)(ProjectsForm);