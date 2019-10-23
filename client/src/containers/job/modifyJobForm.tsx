import React, {useCallback, useEffect, useState} from "react";
import {connect} from "react-redux";
import {CreateJob, IJob, UpdateJob} from "../../redux/jobs";
import {makeStyles, Typography} from "@material-ui/core";
import Button from "@material-ui/core/Button";
import TextField from "@material-ui/core/TextField";
import theme from "../../theme";
import {FormMode} from "./index";
import Box from "@material-ui/core/Box";
import Select from "@material-ui/core/Select";
import MenuItem from "@material-ui/core/MenuItem";
import {IProject} from "../../redux/projects";
import FormControl from "@material-ui/core/FormControl";
import InputLabel from "@material-ui/core/InputLabel";
import NoSsr from "@material-ui/core/NoSsr";
import {format} from "date-fns-tz";
import {isAfter} from "date-fns";

interface IProps {
    formMode?: FormMode
    currentJobId?: string
    setCurrentJobId?: (id: string) => void
    setMode: (mode: FormMode) => void
    currentJob?: IJob
    projects: IProject[]
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

declare var window: {
    schedule: any
};

const JobsForm = (props: IProps & ReturnType<typeof mapDispatchToProps>) => {
    const { formMode, currentJob = null, setMode, setCurrentJobId, projects } = props;

    const classes = useStyles(theme);
    const [state, setState] = useState({
        description: "",
        cron_spec: "",
        start_date: "",
        end_date: "",
        data: "",
        callback_url: "",
        timezone: "UTC",
        project_id: ""
    });

    const {
        description,
        cron_spec,
        start_date,
        end_date,
        data,
        callback_url,
        project_id,
        timezone
    } = state;

    useEffect(() => {
        if (currentJob) {
            setState((state) => ({
                ...state,
                description: currentJob.description,
                cron_spec: currentJob.cron_spec,
                start_date: currentJob.start_date,
                end_date: currentJob.end_date ?  currentJob.end_date : '',
                callback_url: currentJob.callback_url,
                data: currentJob.data,
                project_id: currentJob.project_id
            }));
        }

       return () => {
            setState((state) => ({
                ...state,
                description: "",
                cron_spec: "",
                data: "",
                start_date: "",
                end_date: "",
                callback_url: "",
                project_id: ""
            }));
       }
    }, [currentJob]);

    const handleSubmit = useCallback((e: React.FormEvent<HTMLFormElement>) => {
        async function submitJob(job) {
            const { formMode } = props;

            try {
                if (formMode == FormMode.Create) {
                    await props.createJob(job);
                } else {
                    await props.updateJob(job);
                }
            } catch (e) {
                console.error(e);
            }
        }

        e.preventDefault();

        const form = e.currentTarget;
        const formValidity = form.checkValidity();
        if (formValidity) {
            const body = {
                cron_spec: cron_spec,
                data: data,
                start_date: format(new Date(start_date), "eee, ii MMM yyyy HH:mm:ss zz"),
                end_date: isAfter(new Date(end_date), new Date(start_date)) ?
                    format(new Date(end_date), "eee, ii MMM yyyy HH:mm:ss zz") :
                    formMode == FormMode.Edit ?
                        null :
                        format(new Date(end_date), "eee, ii MMM yyyy HH:mm:ss zz"),
                description: description,
                callback_url: callback_url,
                project_id: project_id,
                timezone
            };

            submitJob(body)
                .then(() => {
                    setState((state) => ({
                    ...state,
                    cron_spec: "",
                    start_date: "",
                    end_date: "",
                    data: "",
                    description: "",
                    callback_url: "",
                    project_id: ""
                }));
            });
        }
    }, [formMode, state]);

    const cancelCallback = useCallback(() => {
        if (formMode == FormMode.Edit) {
            setCurrentJobId(null);
        }

        setMode(FormMode.None);
    }, [formMode]);

    const setFormValue = useCallback((formName, value) => {
        if (formName === 'description') {
            setState((state) => ({ ...state, "description": value }));
        }

        if (formName === 'cron_spec') {
            setState((state) => ({ ...state, "cron_spec": value }));
        }

        if (formName === 'project_id') {
            setState((state) => ({ ...state, "project_id": value }));
        }

        if (formName === 'start_date') {
            setState((state) => ({ ...state, "start_date": value }));
        }

        if (formName === 'end_date') {
            setState((state) => ({ ...state, "end_date": value }));
        }

        if (formName === 'callback_url') {
            setState((state) => ({ ...state, "callback_url": value }));
        }

        if (formName === 'data') {
            setState((state) => ({ ...state, "data": value }));
        }
    }, []);

    if (formMode == FormMode.None) {
        return null
    }

    return (
        <div className={classes.container}>
            {(formMode == FormMode.Create) ?
                <Typography variant="h6">New Job</Typography>:
                <Typography variant="h6">Edit Job</Typography>}
            <div>
                <form onSubmit={handleSubmit} name="new-job">
                    <div className={classes.innerContainer}>
                        <TextField
                            fullWidth
                            required
                            value={description}
                            onChange={(e) => {
                                setFormValue(e.currentTarget.name, e.currentTarget.value);
                                e.persist()
                            }}
                            type="text"
                            name="description"
                            label="Description"
                        />
                        <br /><br />
                        <TextField
                            fullWidth
                            required
                            value={cron_spec}
                            disabled={formMode == FormMode.Edit}
                            onChange={(e) => {
                                setFormValue(e.currentTarget.name, e.currentTarget.value);
                                e.persist()
                            }}
                            type="text"
                            name="cron_spec"
                            label="Cron Spec"
                        />
                        <p>Schedules are specified using unix-cron format. E.g. every minute: "* * * * *", every 3 hours: "0 */3 * * *"", every monday at 9:00: "0 9 * * 1"</p>
                        <NoSsr>
                            {(cron_spec.trim().length > 1) && <p style={{ color: "#0984e3" }}>Next execution time from now: {window.schedule(cron_spec)}</p>}
                        </NoSsr>
                        <FormControl fullWidth required>
                            <InputLabel htmlFor="project_id">Project</InputLabel>
                            <Select
                                name="project_id"
                                value={project_id}
                                disabled={formMode == FormMode.Edit}
                                onChange={(e) => {setFormValue("project_id",  e.target.value)}}
                            >
                                {projects && projects.map(({ id, name }) => {
                                    return (
                                        <MenuItem key={id} value={id}>{name}</MenuItem>
                                    );
                                })}
                            </Select>
                        </FormControl>
                        <br /><br />
                        <TextField
                            fullWidth
                            required
                            value={start_date}
                            disabled={formMode == FormMode.Edit}
                            onChange={(e) => {
                                setFormValue(e.currentTarget.name, e.currentTarget.value);
                                e.persist()
                            }}
                            type="datetime-local"
                            name="start_date"
                            label="Start Date"
                        />
                        <br /><br />
                        <TextField
                            fullWidth
                            value={end_date}
                            onChange={(e) => {
                                setFormValue(e.currentTarget.name, e.currentTarget.value);
                                e.persist()
                            }}
                            type="datetime-local"
                            name="end_date"
                            label="End Date"
                        />
                        <br /><br />
                        <TextField
                            fullWidth
                            required
                            value={callback_url}
                            onChange={(e) => {
                                setFormValue(e.currentTarget.name, e.currentTarget.value);
                                e.persist()
                            }}
                            type="url"
                            name="callback_url"
                            label="Callback Url"
                        />
                        <br /><br />
                        <TextField
                            fullWidth
                            value={data}
                            onChange={(e) => {
                                setFormValue(e.currentTarget.name, e.currentTarget.value);
                                e.persist()
                            }}
                            type="text"
                            name="data"
                            label="Data"
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
    createJob: (job: Partial<IJob>) => dispatch(CreateJob(job)),
    updateJob: (job: Partial<IJob>) => dispatch(UpdateJob(job)),
});

const mapStateToProps = (state) => ({});

export default connect(mapStateToProps, mapDispatchToProps)(JobsForm);