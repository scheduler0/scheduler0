enum JobState {
    InActive = 1,
    Active = 2,
    Stale = -1,
}

interface IJob {
    id: string
    project_id: string
    description: string
    cron_spec: string
    total_execs: number
    data: string
    callback_url: string
    last_status_code: number
    state: JobState
    start_date: string
    end_date: string
    next_time: string
    date_created: string
}