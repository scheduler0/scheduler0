package constants

const (
	SqliteDbFileName   = "db.db"
	RecoveryDbFileName = "recover.db"
	SecretsFileName    = ".scheduler0"
	RaftDir            = "raft_data"
	SqliteDir          = "sqlite_data"
	RaftLog            = "logs.dat"
	RaftStableLog      = "stable.dat"
	ConfigFileName     = "config.yml"
)

type Command int32

const (
	CommandTypeDbExecute Command = 0
	CommandTypeJobQueue  Command = 1
	CommandTypeLocalData Command = 2
)

type CommandAction int32

const (
	CommandActionQueueJob                       CommandAction = 0
	CommandActionCleanUncommittedAsyncTasksLogs CommandAction = 1
	CommandActionCleanUncommittedExecutionLogs  CommandAction = 2
)

const (
	DBMaxVariableSize           = 32766
	JobMaxBatchSize             = 5461
	JobExecutionLogMaxBatchSize = 4095
)

const (
	CreateJobAsyncTaskService   = "create_job"
	JobExecutorAsyncTaskService = "job_executor"
)

const ConfigProtocolHTTP = "http"
const ConfigProtocolHTTPS = "https"
const ConfigProtocolTCP = "tcp"
const ConfigProtocolTLS = "tls"

const (
	JobsTableName = "jobs"
)

const (
	JobsIdColumn             = "id"
	JobsProjectIdColumn      = "project_id"
	JobsSpecColumn           = "spec"
	JobsDataColumn           = "data"
	JobsExecutorIdColumn     = "executor_id"
	JobsStartDateColumn      = "start_date"
	JobsEndDateColumn        = "end_date"
	JobsTimezoneColumn       = "timezone"
	JobsTimezoneOffsetColumn = "timezone_offset"
	JobsRetryMaxColumn       = "retry_max"
	JobsDateCreatedColumn    = "date_created"
	JobsAccountIdColumn      = "account_id"
	JobsDateModifiedColumn   = "date_modified"
	JobsCreatedByColumn      = "created_by"
	JobsModifiedByColumn     = "updated_by"
	JobsDeletedByColumn      = "deleted_by"
	JobsStatusColumn         = "status"
)

const (
	ProjectsTableName          = "projects"
	ProjectsIdColumn           = "id"
	ProjectsNameColumn         = "name"
	ProjectsDescriptionColumn  = "description"
	ProjectsDateCreatedColumn  = "date_created"
	ProjectsAccountIdColumn    = "account_id"
	ProjectsDateModifiedColumn = "date_modified"
	ProjectsCreatedByColumn    = "created_by"
	ProjectsModifiedByColumn   = "updated_by"
	ProjectsDeletedByColumn    = "deleted_by"
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
	ExecutionsAccountIdColumn         = "account_id"
	ExecutionsDateModifiedColumn      = "date_modified"
)

const (
	CommittedAsyncTableName   = "async_tasks_committed"
	UnCommittedAsyncTableName = "async_tasks_uncommitted"
)

const (
	AsyncTasksIdColumn           = "id"
	AsyncTasksRequestIdColumn    = "request_id"
	AsyncTasksInputColumn        = "input"
	AsyncTasksOutputColumn       = "output"
	AsyncTasksStateColumn        = "state"
	AsyncTasksServiceColumn      = "service"
	AsyncTasksDateCreatedColumn  = "date_created"
	AsyncTasksAccountIdColumn    = "account_id"
	AsyncTasksDateModifiedColumn = "date_modified"
	AsyncTasksCreatedByColumn    = "created_by"
	AsyncTasksModifiedByColumn   = "updated_by"
	AsyncTasksDeletedByColumn    = "deleted_by"
)

const (
	CredentialTableName = "credentials"
)

const (
	CredentialsIdColumn           = "id"
	CredentialsArchivedColumn     = "archived"
	CredentialsArchivedByColumn   = "archived_by"
	CredentialsApiKeyColumn       = "api_key"
	CredentialsApiSecretColumn    = "api_secret"
	CredentialsDateCreatedColumn  = "date_created"
	CredentialsAccountIdColumn    = "account_id"
	CredentialsDateModifiedColumn = "date_modified"
	CredentialsCreatedByColumn    = "created_by"
	CredentialsModifiedByColumn   = "updated_by"
	CredentialsDeletedByColumn    = "deleted_by"
	CredentialsExpiresAtColumn    = "expires_at"
	CredentialsScopesColumn       = "scopes"
)

const (
	CredentialScopeRead    = "read"
	CredentialScopeWrite   = "write"
	CredentialScopeExecute = "execute"
	CredentialScopeAdmin   = "admin"
)

const CredentialExpiryDays = 90

const CredentialMinExpirySeconds = 300

const (
	DefaultRetryMaxConfig      = 30
	DefaultRetryIntervalConfig = 3
	DefaultMaxConnectedPeers   = 4
)

const (
	JobExecutorTableName                = "job_executors"
	JobExecutorIdColumn                 = "id"
	JobExecutorAccountIdColumn          = "account_id"
	JobExecutorNameColumn               = "name"
	JobExecutorDescriptionColumn        = "description"
	JobExecutorTagsColumn               = "tags"
	JobExecutorTypeColumn               = "type"
	JobExecutorCloudProviderColumn      = "cloud_provider"
	JobExecutorRegionColumn             = "region"
	JobExecutorCloudResourceUrlColumn   = "cloud_resource_url"
	JobExecutorCloudApiKey              = "cloud_api_key"
	JobExecutorCloudApiSecret           = "cloud_api_secret"
	JobExecutorWebhookUrlColumn         = "webhook_url"
	JobExecutorWebhookSecretColumn      = "webhook_secret"
	JobExecutorWebhookMethodColumn      = "webhook_method"
	JobExecutorDateCreatedColumn        = "date_created"
	JobExecutorDateModifiedColumn       = "date_modified"
	JobExecutorCreatedByColumn          = "created_by"
	JobExecutorModifiedByColumn         = "updated_by"
	JobExecutorDeletedByColumn          = "deleted_by"
	JobExecutorPayloadAggregationColumn = "payload_aggregation"
	JobExecutorCommandColumn            = "command"
	JobExecutorWorkingDirColumn         = "working_dir"
)

const (
	AccountsTableName          = "accounts"
	AccountsIdColumn           = "id"
	AccountsNameColumn         = "name"
	AccountsDateCreatedColumn  = "date_created"
	AccountsDateModifiedColumn = "date_modified"

	AccountFeaturesTableName          = "account_features"
	AccountFeaturesAccountIdColumn    = "account_id"
	AccountFeaturesFeatureIdColumn    = "feature_id"
	AccountFeaturesDateCreatedColumn  = "date_created"
	AccountFeaturesDateModifiedColumn = "date_modified"
	AccountFeaturesCreatedByColumn    = "created_by"
	AccountFeaturesModifiedByColumn   = "updated_by"

	FeaturesTableName          = "features"
	FeaturesIdColumn           = "id"
	FeaturesNameColumn         = "name"
	FeaturesDateCreatedColumn  = "date_created"
	FeaturesDateModifiedColumn = "date_modified"
	FeaturesCreatedByColumn    = "created_by"
	FeaturesModifiedByColumn   = "updated_by"
)

const APIV1Base = "/api/v1"

const (
	SendTimePolicyID           = "default_send_time"
	SendTimePolicyVersion      = "1.0.0"
	SendTimeEngineVersion      = "1.0.0"
	SendTimeDefaultHorizonDays = 7
	SendTimeMaxHorizonDays     = 30
	SendTimeMaxRecipients      = 100
	SendTimeDefaultInterval    = 30
	SendTimeDefaultSuggestions = 3
	SendTimeMaxSuggestions     = 10
	SendTimeMaxBusyIntervals   = 1000
	SendTimeMaxMessageText     = 10000
)

const (
	IncreasedRetryMaxByFiveFeature                       = "increased_retry_max_by_five"
	IncreasedJobPayloadSizeTo1MBFeature                  = "increased_job_payload_size_to_1mb"
	IncreasedNumberOfJobExecutions100KPerMonthFeature    = "increased_number_of_job_executions_100_k_per_month"
	IncreasedExecutionLogs90DaysRetentionFeature         = "increased_execution_logs_90_days_retention"
	IncreasedNumberOfClassifyRequests100KPerMonthFeature = "increased_number_of_classify_requests_100_k_per_month"
	IncreasedNumberOfPromptRequests100KPerMonthFeature   = "increased_number_of_prompt_requests_100_k_per_month"
	TeamMembersFeature                                   = "team_members"
)

const (
	DefaultNumberOfJobExecutions10KPerMonth  = 1000
	DefaultNumberOfJobExecutions100KPerMonth = 100000
)

const (
	DefaultNumberOfClassifyRequests1KPerMonth   = 1000
	DefaultNumberOfClassifyRequests100KPerMonth = 100000
)

const (
	DefaultNumberOfPromptRequests1KPerMonth   = 1000
	DefaultNumberOfPromptRequests100KPerMonth = 100000
)

const (
	DefaultListLimit              = 10
	DefaultExecutionLogsListLimit = 50
	MaxListLimit                  = 100
)

const (
	DefaultJobPayloadMaxBytes   = 3072
	IncreasedJobPayloadMaxBytes = 1024 * 1024
	DefaultJobRetryMax          = 3
	IncreasedJobRetryMax        = 15
)

const (
	PromptMaxLength         = 160
	PromptListMaxItems      = 5
	PromptListItemMaxLength = 36
)

const (
	DefaultExecutionLogsRetentionDays  = 30
	ExtendedExecutionLogsRetentionDays = 90
)

const DefaultWebhookTimeoutSeconds = 30

const SystemActorName = "system"

const (
	OrderDirectionAsc  = "ASC"
	OrderDirectionDesc = "DESC"
)

const (
	AIProviderOpenAI       = "openai"
	AIProviderAnthropic    = "anthropic"
	AIProviderBedrock      = "bedrock"
	AIProviderOpenRouter   = "openrouter"
	AIProviderPlatform     = "platform"
	AIDefaultBedrockRegion = "us-east-1"
	AIDefaultPlatformModel = "global.anthropic.claude-sonnet-4-5-20250929-v1:0"
)
