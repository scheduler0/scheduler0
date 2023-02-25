package constants

const SqliteDbFileName = "db.db"
const RecoveryDbFileName = "recover.db"
const SecretsFileName = ".scheduler0"
const RaftDir = "raft_data"
const RaftLog = "logs.dat"
const RaftStableLog = "stable.dat"
const ConfigFileName = "config.yml"

type Command int32

const (
	CommandTypeDbExecute   Command = 0
	CommandTypeJobQueue    Command = 1
	CommandTypeLocalData   Command = 2
	CommandTypeStopJobs    Command = 3
	CommandTypeRecoverJobs Command = 4
)

const DBMaxVariableSize = 32766
const JobMaxBatchSize = 5461
const JobExecutionLogMaxBatchSize = 4095
const CreateJobAsyncTaskService = "create_job"
const JobExecutorAsyncTaskService = "job_executor"
