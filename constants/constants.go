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
	CommandTypeDbExecute        Command = 0
	CommandTypeJobQueue         Command = 1
	CommandTypeJobExecutionLogs Command = 2
	CommandTypeStopJobs         Command = 3
	CommandTypeRecoverJobs      Command = 4
)

// JobMaxBatchSize exceed this and sql-lite won't be happy max variable is 32766
const DBMaxVariableSize = 32766
const JobMaxBatchSize = 5461
const JobExecutionLogMaxBatchSize = 4095
