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

const (
	JobsTableName = "jobs"
)

const (
	JobsIdColumn             = "id"
	JobsProjectIdColumn      = "project_id"
	JobsSpecColumn           = "spec"
	JobsCallbackURLColumn    = "callback_url"
	JobsDataColumn           = "data"
	JobsExecutionTypeColumn  = "execution_type"
	JobsTimezoneColumn       = "timezone"
	JobsTimezoneOffsetColumn = "timezone_offset"
	JobsDateCreatedColumn    = "date_created"
)

const (
	ProjectsTableName         = "projects"
	ProjectsIdColumn          = "id"
	ProjectsNameColumn        = "name"
	ProjectsDescriptionColumn = "description"
	ProjectsDateCreatedColumn = "date_created"
)

const (
	JobQueuesTableName        = "job_queues"
	JobQueueIdColumn          = "id"
	JobQueueNodeIdColumn      = "node_id"
	JobQueueLowerBoundJobId   = "lower_bound_job_id"
	JobQueueUpperBound        = "upper_bound_job_id"
	JobQueueDateCreatedColumn = "date_created"
	JobQueueVersion           = "version"

	ExecutionsUnCommittedTableName    = "job_executions_uncommitted"
	ExecutionsCommittedTableName      = "job_executions_committed"
	ExecutionsUniqueIdColumn          = "unique_id"
	ExecutionsStateColumn             = "state"
	ExecutionsNodeIdColumn            = "node_id"
	ExecutionsLastExecutionTimeColumn = "last_execution_time"
	ExecutionsNextExecutionTime       = "next_execution_time"
	ExecutionsJobIdColumn             = "job_id"
	ExecutionsDateCreatedColumn       = "date_created"
	ExecutionsJobQueueVersion         = "job_queue_version"
	ExecutionsVersion                 = "execution_version"
)

const (
	CommittedAsyncTableName   = "async_tasks_committed"
	UnCommittedAsyncTableName = "async_tasks_uncommitted"
)

const (
	AsyncTasksIdColumn          = "id"
	AsyncTasksRequestIdColumn   = "request_id"
	AsyncTasksInputColumn       = "input"
	AsyncTasksOutputColumn      = "output"
	AsyncTasksStateColumn       = "state"
	AsyncTasksServiceColumn     = "service"
	AsyncTasksDateCreatedColumn = "date_created"
)

const (
	CredentialTableName = "credentials"
)

const (
	CredentialsIdColumn          = "id"
	CredentialsArchivedColumn    = "archived"
	CredentialsApiKeyColumn      = "api_key"
	CredentialsApiSecretColumn   = "api_secret"
	CredentialsDateCreatedColumn = "date_created"
)

const (
	DefaultRetryMaxConfig      = 30
	DefaultRetryIntervalConfig = 3
	DefaultMaxConnectedPeers   = 4
)
