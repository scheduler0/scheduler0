package constants

const SqliteDbFileName = "db.db"
const CredentialsFileName = ".scheduler0"
const RaftDir = "raft_data"
const RaftLog = "logs.dat"
const RaftStableLog = "stable.dat"
const ConfigFileName = "config.yml"
const ExecutionLogsDir = "logs"

type Command int32

const (
	CommandTypeDbExecute            Command = 0
	CommandTypeJobQueue                     = 1
	CommandTypePrepareJobExecutions         = 2
	CommandTypeCommitJobExecutions          = 3
	CommandTypeErrorJobExecutions           = 4
)
