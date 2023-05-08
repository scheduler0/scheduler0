package constants

// These constants define file and directory names used in the application.
const (
	SqliteDbFileName   = "db.db"       // The name of the SQLite database file
	RecoveryDbFileName = "recover.db"  // The name of the recovery database file
	SecretsFileName    = ".scheduler0" // The name of the secrets file
	RaftDir            = "raft_data"   // The name of the raft data directory
	SqliteDir          = "sqlite_data" // The name of the SQLite data directory
	RaftLog            = "logs.dat"    // The name of the raft log file
	RaftStableLog      = "stable.dat"  // The name of the raft stable log file
	ConfigFileName     = "config.yml"  // The name of the configuration file
)

// Command represents a command used in the application.
type Command int32

// These constants represent different types of commands.
const (
	CommandTypeDbExecute   Command = 0 // Execute a database command
	CommandTypeJobQueue    Command = 1 // Enqueue a job
	CommandTypeLocalData   Command = 2 // Store data locally
	CommandTypeStopJobs    Command = 3 // Stop a job
	CommandTypeRecoverJobs Command = 4 // Recover jobs
)

// These constants define the maximum size of certain data structures used in the application.
const (
	DBMaxVariableSize           = 32766 // The maximum size of a variable in the database
	JobMaxBatchSize             = 5461  // The maximum number of jobs that can be batched
	JobExecutionLogMaxBatchSize = 4095  // The maximum number of job executions that can be batched
)

// These constants represent the names of asynchronous task services used in the application.
const (
	CreateJobAsyncTaskService   = "create_job"   // The name of the asynchronous task service for creating jobs
	JobExecutorAsyncTaskService = "job_executor" // The name of the asynchronous task service for executing jobs
)
