package migrations

import (
	"database/sql"
	"scheduler0/pkg/models"
)

func MainTables() models.Migration {
	return models.Migration{
		Name: "main_tables",
		Up:   mainTablesUp,
		Down: mainTablesDown,
	}
}

func mainTablesUp(db *sql.Tx) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS accounts
(
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	name          TEXT NOT NULL,
	date_created  datetime NOT NULL,
	date_modified datetime
);

CREATE TABLE IF NOT EXISTS features
(
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	name          TEXT NOT NULL,
	date_created  datetime NOT NULL,
	date_modified datetime
);

CREATE TABLE IF NOT EXISTS account_features
(
	account_id    INTEGER REFERENCES accounts (id) ON DELETE CASCADE NOT NULL,
	feature_id    INTEGER REFERENCES features (id) ON DELETE CASCADE NOT NULL,
	date_created  datetime NOT NULL,
	date_modified datetime,
	UNIQUE(account_id, feature_id)
);

CREATE INDEX IF NOT EXISTS idx_account_features_account_id_feature_id ON account_features(account_id, feature_id);

CREATE TABLE IF NOT EXISTS credentials
(
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	archived      boolean NOT NULL,
	api_key       TEXT,
	account_id    INTEGER REFERENCES accounts (id) ON DELETE CASCADE NOT NULL,
	api_secret    TEXT,
	date_created  datetime NOT NULL,
	date_modified datetime,
	created_by    TEXT,
	updated_by    TEXT,
	deleted_by    TEXT,
	archived_by   TEXT,
	expires_at    datetime,
	scopes        TEXT NOT NULL DEFAULT 'read,write,execute'
);

CREATE INDEX IF NOT EXISTS idx_credentials_expires_at ON credentials(expires_at);
CREATE INDEX IF NOT EXISTS idx_credentials_account_id ON credentials(account_id);

CREATE TABLE IF NOT EXISTS projects
(
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id    INTEGER REFERENCES accounts (id) ON DELETE CASCADE NOT NULL,
	name          TEXT NOT NULL,
	description   TEXT NOT NULL,
	date_created  datetime NOT NULL,
	date_modified datetime,
	created_by    TEXT,
	updated_by    TEXT,
	deleted_by    TEXT
);

CREATE INDEX IF NOT EXISTS idx_projects_account_id ON projects(account_id);

CREATE TABLE IF NOT EXISTS job_executors
(
	id                  INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id          INTEGER NOT NULL,
	name                TEXT NOT NULL,
	type                TEXT NOT NULL,
	cloud_provider      TEXT,
	region              TEXT,
	cloud_resource_url  TEXT,
	cloud_api_key       TEXT,
	cloud_api_secret    TEXT,
	date_created        datetime NOT NULL,
	webhook_url         TEXT,
	webhook_secret      TEXT,
	webhook_method      TEXT,
	payload_aggregation BOOLEAN NOT NULL DEFAULT 0,
	date_modified       datetime,
	created_by          TEXT,
	updated_by          TEXT,
	deleted_by          TEXT,
	command             TEXT NOT NULL DEFAULT '',
	working_dir         TEXT NOT NULL DEFAULT '',
	description         TEXT NOT NULL DEFAULT '',
	tags                TEXT NOT NULL DEFAULT '',
	FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_job_executors_account_id ON job_executors(account_id);

CREATE TABLE IF NOT EXISTS jobs
(
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id      INTEGER REFERENCES accounts (id) ON DELETE CASCADE NOT NULL,
	project_id      INTEGER NOT NULL,
	spec            TEXT NOT NULL,
	data            TEXT,
	start_date      datetime NOT NULL,
	end_date        datetime,
	retry_max       INTEGER NOT NULL DEFAULT 3,
	date_created    datetime NOT NULL,
	timezone        TEXT NOT NULL,
	timezone_offset INTEGER NOT NULL,
	date_modified   datetime,
	status          TEXT NOT NULL DEFAULT 'active',
	created_by      TEXT,
	updated_by      TEXT,
	deleted_by      TEXT,
	executor_id     INTEGER REFERENCES job_executors (id)
);

CREATE INDEX IF NOT EXISTS idx_jobs_account_id ON jobs(account_id);

CREATE TABLE IF NOT EXISTS job_executions_committed
(
	id                  INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id          INTEGER REFERENCES accounts (id) ON DELETE CASCADE NOT NULL,
	unique_id           TEXT,
	state               INTEGER NOT NULL,
	node_id             INTEGER NOT NULL,
	last_execution_time datetime NOT NULL,
	next_execution_time datetime NOT NULL,
	job_id              INTEGER NOT NULL,
	date_created        datetime NOT NULL,
	job_queue_version   INTEGER NOT NULL,
	execution_version   INTEGER NOT NULL,
	date_modified       datetime
);

CREATE TABLE IF NOT EXISTS job_executions_uncommitted
(
	id                  INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id          INTEGER REFERENCES accounts (id) ON DELETE CASCADE NOT NULL,
	unique_id           TEXT,
	state               INTEGER NOT NULL,
	node_id             INTEGER NOT NULL,
	last_execution_time datetime NOT NULL,
	next_execution_time datetime NOT NULL,
	job_id              INTEGER NOT NULL,
	date_created        datetime NOT NULL,
	job_queue_version   INTEGER NOT NULL,
	execution_version   INTEGER NOT NULL,
	date_modified       datetime
);

CREATE INDEX IF NOT EXISTS idx_job_executions_committed_account_id ON job_executions_committed(account_id);
CREATE INDEX IF NOT EXISTS idx_job_executions_uncommitted_account_id ON job_executions_uncommitted(account_id);
CREATE INDEX IF NOT EXISTS idx_exec_committed_account_date ON job_executions_committed(account_id, date_created);
CREATE INDEX IF NOT EXISTS idx_exec_uncommitted_account_date ON job_executions_uncommitted(account_id, date_created);
CREATE INDEX IF NOT EXISTS idx_exec_committed_account_job_date ON job_executions_committed(account_id, job_id, date_created);
CREATE INDEX IF NOT EXISTS idx_exec_uncommitted_account_job_date ON job_executions_uncommitted(account_id, job_id, date_created);
CREATE INDEX IF NOT EXISTS idx_exec_committed_account_state_date ON job_executions_committed(account_id, state, date_created);
CREATE INDEX IF NOT EXISTS idx_exec_uncommitted_account_state_date ON job_executions_uncommitted(account_id, state, date_created);

CREATE TABLE IF NOT EXISTS job_queues
(
	id                 INTEGER PRIMARY KEY AUTOINCREMENT,
	node_id            INTEGER NOT NULL,
	lower_bound_job_id INTEGER NOT NULL,
	upper_bound_job_id INTEGER NOT NULL,
	version            INTEGER NOT NULL,
	date_created       datetime NOT NULL,
	date_modified      datetime
);

CREATE TABLE IF NOT EXISTS job_queue_versions
(
	id                     INTEGER PRIMARY KEY AUTOINCREMENT,
	version                INTEGER NOT NULL,
	number_of_active_nodes INTEGER NOT NULL,
	date_created           datetime NOT NULL,
	date_modified          datetime
);

CREATE TABLE IF NOT EXISTS async_tasks_committed
(
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id    INTEGER REFERENCES accounts (id) ON DELETE CASCADE NOT NULL,
	request_id    TEXT NOT NULL,
	input         TEXT NOT NULL,
	output        TEXT,
	state         INTEGER NOT NULL,
	service       TEXT NOT NULL,
	date_created  datetime NOT NULL,
	date_modified datetime
);

CREATE TABLE IF NOT EXISTS async_tasks_uncommitted
(
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id    INTEGER REFERENCES accounts (id) ON DELETE CASCADE NOT NULL,
	request_id    TEXT NOT NULL,
	input         TEXT NOT NULL,
	output        TEXT,
	state         INTEGER NOT NULL,
	service       TEXT NOT NULL,
	date_created  datetime NOT NULL,
	date_modified datetime
);

CREATE INDEX IF NOT EXISTS idx_async_tasks_committed_account_id ON async_tasks_committed(account_id);
CREATE INDEX IF NOT EXISTS idx_async_tasks_uncommitted_account_id ON async_tasks_uncommitted(account_id);

CREATE TABLE IF NOT EXISTS account_job_executions_count
(
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id      INTEGER NOT NULL,
	execution_count INTEGER NOT NULL,
	tokens          INTEGER,
	date_created    DATETIME NOT NULL,
	date_modified   DATETIME NOT NULL,
	next_reset_date DATETIME NOT NULL,
	FOREIGN KEY (account_id) REFERENCES accounts (id)
);

CREATE TABLE IF NOT EXISTS account_ai_settings
(
	account_id            INTEGER PRIMARY KEY,
	openai_api_key        TEXT NOT NULL DEFAULT '',
	anthropic_api_key     TEXT NOT NULL DEFAULT '',
	bedrock_access_key_id TEXT NOT NULL DEFAULT '',
	bedrock_secret_key    TEXT NOT NULL DEFAULT '',
	bedrock_region        TEXT NOT NULL DEFAULT '',
	date_created          DATETIME NOT NULL,
	date_modified         DATETIME,
	openrouter_api_key    TEXT NOT NULL DEFAULT '',
	active_models         TEXT NOT NULL DEFAULT '',
	FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS account_prompt_requests
(
	id                 INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id         INTEGER NOT NULL,
	prompt             TEXT NOT NULL DEFAULT '',
	provider           TEXT NOT NULL DEFAULT '',
	model              TEXT NOT NULL DEFAULT '',
	output             TEXT NOT NULL DEFAULT '',
	input_tokens       INTEGER NOT NULL DEFAULT 0,
	output_tokens      INTEGER NOT NULL DEFAULT 0,
	total_tokens       INTEGER NOT NULL DEFAULT 0,
	duration_ms        INTEGER NOT NULL DEFAULT 0,
	estimated_cost_usd REAL NOT NULL DEFAULT 0,
	status             TEXT NOT NULL DEFAULT '',
	error              TEXT NOT NULL DEFAULT '',
	date_created       DATETIME NOT NULL,
	FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS account_classify_requests
(
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id   INTEGER NOT NULL,
	kind         TEXT NOT NULL DEFAULT '',
	prompt       TEXT NOT NULL DEFAULT '',
	decision     TEXT NOT NULL DEFAULT '',
	status       TEXT NOT NULL DEFAULT '',
	error        TEXT NOT NULL DEFAULT '',
	date_created DATETIME NOT NULL,
	FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_account_prompt_requests_account_id_date ON account_prompt_requests (account_id, date_created);
CREATE INDEX IF NOT EXISTS idx_account_classify_requests_account_id_date ON account_classify_requests (account_id, date_created);

CREATE TABLE IF NOT EXISTS account_ai_quota_period
(
	account_id      INTEGER PRIMARY KEY,
	period_start    DATETIME NOT NULL,
	next_reset_date DATETIME NOT NULL,
	date_created    DATETIME NOT NULL,
	date_modified   DATETIME NOT NULL,
	FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE
);
`)
	return err
}

func mainTablesDown(db *sql.Tx) error {
	_, err := db.Exec(`
	DROP TABLE IF EXISTS account_ai_quota_period;
	DROP TABLE IF EXISTS account_classify_requests;
	DROP TABLE IF EXISTS account_prompt_requests;
	DROP TABLE IF EXISTS account_ai_settings;
	DROP TABLE IF EXISTS account_job_executions_count;
	DROP TABLE IF EXISTS async_tasks_uncommitted;
	DROP TABLE IF EXISTS async_tasks_committed;
	DROP TABLE IF EXISTS job_queue_versions;
	DROP TABLE IF EXISTS job_queues;
	DROP TABLE IF EXISTS job_executions_uncommitted;
	DROP TABLE IF EXISTS job_executions_committed;
	DROP TABLE IF EXISTS jobs;
	DROP TABLE IF EXISTS job_executors;
	DROP TABLE IF EXISTS projects;
	DROP TABLE IF EXISTS credentials;
	DROP TABLE IF EXISTS account_features;
	DROP TABLE IF EXISTS features;
	DROP TABLE IF EXISTS accounts;
	`)
	return err
}
