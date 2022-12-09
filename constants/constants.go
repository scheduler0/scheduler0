package constants

const SqliteDbFileName = "db.db"
const SecretsFileName = ".scheduler0"
const RaftDir = "raft_data"
const RaftLog = "logs.dat"
const RaftStableLog = "stable.dat"
const ConfigFileName = "config.yml"
const ExecutionLogsDir = "logs"
const ExecutionLogsCommitFile = "committed.log"
const ExecutionLogsUnCommitFile = "uncommitted.log"

type Command int32

const (
	CommandTypeDbExecute               Command = 0
	CommandTypeJobQueue                        = 1
	CommandTypeScheduleJobExecutions           = 2
	CommandTypeSuccessfulJobExecutions         = 3
	CommandTypeFailedJobExecutions             = 4
	CommandTypeStopJobs                        = 5
)

// JobMaxBatchSize exceed this and sql-lite won't be happy
const JobMaxBatchSize = 5461

const QueueExecutionLogPrefix = "queue"
const ScheduledExecutionLogPrefix = "schedule"
const SuccessfulExecutionLogPrefix = "success"
const FailedExecutionLogPrefix = "failed"
