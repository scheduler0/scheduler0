// @ts-ignore
import React from "react";
import { connect } from "react-redux";
import Grid from '@material-ui/core/Grid';
import ModifyProjectForm from "./modifyProjectForm"
import ProjectList from './projectList';
import {createStyles} from "@material-ui/core";
import {WithStyles, withStyles} from "@material-ui/core/styles"
import {setCurrentProjectId, DeleteProject} from '../../redux/projects'

export enum FormMode {
    Edit = "Edit",
    Create = "Create",
    None = "None"
}

interface IState {
    formMode: FormMode
}

const styles = theme => createStyles({
    containerHeader: {
        height: "50px",
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
    }
});

type Props = ReturnType<typeof mapStateToProps>
    & WithStyles<ReturnType<typeof styles>>
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
        const {projects, currentProjectId, deleteProject, setCurrentProjectId} = this.props;

        const currentProject = (formMode == FormMode.Edit)
            ? projects.find(({ id }) => id == currentProjectId)
            : null;

        return (
            <Grid container>
                <Grid item md={8} lg={8}>
                    <ProjectList
                        projects={projects}
                        formMode={formMode}
                        setMode={this.setMode}
                        deleteProject={deleteProject}
                        setCurrentProjectId={setCurrentProjectId}
                    />
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

export default connect(mapStateToProps, mapDispatchToProps)
(withStyles(styles)(ProjectContainer)) as any as React.ComponentType;