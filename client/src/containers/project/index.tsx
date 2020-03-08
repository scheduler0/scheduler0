import React from "react";
import { connect } from "react-redux";
import Grid from '@material-ui/core/Grid';
import ModifyProjectForm from "./modifyProjectForm"
import ProjectList from './projectList';
import {WithStyles, withStyles} from "@material-ui/core/styles"
import {setCurrentProjectId, DeleteProject} from '../../redux/projects'
import { compose } from "recompose";

import SharedStyles from "../../shared/styles";
import Paper from "@material-ui/core/Paper";

export enum FormMode {
    Edit = "Edit",
    Create = "Create",
    None = "None"
}

interface IState {
    formMode: FormMode
}

type Props = ReturnType<typeof mapStateToProps>
    & WithStyles<ReturnType<typeof SharedStyles>>
    & ReturnType<typeof mapDispatchToProps>

class ProjectContainer extends React.Component<Props> {

    state: IState = {
        formMode: FormMode.None
    };

    setMode = (mode: FormMode) => {
        this.setState({
            formMode: mode
        }, () => {
            const { currentProjectId, setCurrentProjectId } = this.props;
            if (this.state.formMode == FormMode.None && currentProjectId) {
                setCurrentProjectId(null);
            }
        });
    };

    render() {
        const {formMode} = this.state;
        const {classes, projects, currentProjectId, deleteProject, setCurrentProjectId} = this.props;

        const currentProject = (formMode == FormMode.Edit)
            ? projects.find(({ id }) => id == currentProjectId)
            : null;

        return (
            <Grid container>
                <Grid item md={8} lg={8}>
                    <Paper className={classes.paper}>
                        <ProjectList
                            projects={projects}
                            formMode={formMode}
                            setMode={this.setMode}
                            deleteProject={deleteProject}
                            setCurrentProjectId={setCurrentProjectId}
                        />
                    </Paper>
                </Grid>
                <Grid item md={4} lg={4}>
                    <ModifyProjectForm
                        formMode={formMode}
                        currentProjectId={currentProjectId}
                        setCurrentProjectId={setCurrentProjectId}
                        setMode={this.setMode}
                        currentProject={currentProject}
                    />
                </Grid>
            </Grid>
        );
    }
}

const mapStateToProps = (state) => ({
    projects: state.ProjectsReducer.projects,
    currentProjectId: state.ProjectsReducer.currentProjectId
});

const mapDispatchToProps = (dispatch) => ({
    setCurrentProjectId: (id: string) => dispatch(setCurrentProjectId(id)),
    deleteProject: (id: string) => dispatch(DeleteProject(id))
});

export default compose(
    connect(mapStateToProps, mapDispatchToProps),
    withStyles(SharedStyles)
)(ProjectContainer) as any as React.ComponentType;