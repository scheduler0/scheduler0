package constants

const SqliteDbFileName = "db.db"
const CredentialsFileName = ".scheduler0"
const RaftDir = "raft_data"
const RaftLog = "logs.dat"
const RaftStableLog = "stable.dat"
const ConfigFileName = "config.yml"
const ExecutionLogsDir = "logs"

const (
	CommandTypeDbExecute int32 = 0
	CommandTypeJobQueue  int32 = 1
	CommandTypeDbQuery   int32 = 2
)
