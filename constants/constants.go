package constants

const SqliteDbFileName = "db.db"
const CredentialsFileName = ".scheduler0"
const RaftDir = "raft_data"
const RaftLog = "logs.dat"
const RaftStableLog = "stable.dat"
const ConfigFileName = "config.yml"
const ExecutionLogsDir = "logs"

const (
	COMMAND_TYPE_UNKNOWN     int32 = 0
	COMMAND_TYPE_DB_QUERY    int32 = 1
	COMMAND_TYPE_DB_EXECUTE  int32 = 2
	COMMAND_TYPE_JOB_EXECUTE int32 = 3
	COMMAND_TYPE_NOOP        int32 = 4
	COMMAND_TYPE_LOAD        int32 = 5
)
