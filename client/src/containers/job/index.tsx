// @ts-ignore
import React from "react";
import { connect } from "react-redux";
import Grid from '@material-ui/core/Grid';
import JobList from './jobList';
import {createStyles} from "@material-ui/core";
import {WithStyles, withStyles} from "@material-ui/core/styles"
import {setCurrentJobId, DeleteJob} from '../../redux/jobs'
import ModifyJobForm from "../job/modifyJobForm";

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

class JobContainer extends React.Component<Props> {

    state: IState = {
        formMode: FormMode.None
    };

    setMode = (mode: FormMode) => {
        this.setState({
            formMode: mode
        }, () => {
            const { currentJobId, setCurrentJobId } = this.props;
            if (this.state.formMode == FormMode.None && currentJobId) {
                setCurrentJobId(null);
            }
        });
    };

    render() {
        const {formMode} = this.state;
        const {jobs, projects, currentJobId, deleteJob, setCurrentJobId} = this.props;

        const currentJob = (formMode == FormMode.Edit)
            ? jobs.find(({ id }) => id == currentJobId)
            : null;

        return (
            <Grid container>
                <Grid item md={8} lg={8}>
                    <JobList
                        jobs={jobs}
                        formMode={formMode}
                        setMode={this.setMode}
                        deleteJob={deleteJob}
                        setCurrentJobId={setCurrentJobId}
                    />
                </Grid>
                <Grid item md={4} lg={4}>
                    <ModifyJobForm
                        projects={projects}
                        formMode={formMode}
                        currentJobId={currentJobId}
                        setCurrentJobId={setCurrentJobId}
                        setMode={this.setMode}
                        currentJob={currentJob}
                    />
                </Grid>
            </Grid>
        );
    }
}

const mapStateToProps = (state) => ({
    jobs: state.JobsReducer.jobs,
    projects: state.ProjectsReducer.projects,
    currentJobId: state.JobsReducer.currentJobId
});

const mapDispatchToProps = (dispatch) => ({
    setCurrentJobId: (id: string) => dispatch(setCurrentJobId(id)),
    deleteJob: (id: string) => dispatch(DeleteJob(id))
});

export default connect(mapStateToProps, mapDispatchToProps)
(withStyles(styles)(JobContainer)) as any as React.ComponentType;