CREATE TABLE IF NOT EXISTS credentials
(
    id                               INTEGER PRIMARY KEY AUTOINCREMENT,
    archived                         boolean   NOT NULL,
    platform                         TEXT      NOT NULL,
    api_key                          TEXT,
    api_secret                       TEXT,
    ip_restriction                   TEXT,
    http_referrer_restriction        TEXT,
    ios_bundle_id_restriction        TEXT,
    android_package_name_restriction TEXT,
    date_created                     TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS projects
(
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT      NOT NULL,
    description  TEXT      NOT NULL,
    date_created TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS jobs
(
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id     INTEGER   NOT NULL,
    spec           TEXT      NOT NULL,
    data           TEXT,
    callback_url   TEXT      NOT NULL,
    execution_type TEXT      NOT NULL,
    date_created   TIMESTAMP NOT NULL,
    FOREIGN KEY (project_id)
        REFERENCES projects (id)
        ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS executions
(
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id         INTEGER   NOT NULL,
    time_added     TIMESTAMP NOT NULL,
    time_executed  TIMESTAMP,
    execution_time INTEGER,
    status_code    TEXT,
    date_created   TIMESTAMP NOT NULL,
    FOREIGN KEY (job_id)
        REFERENCES jobs (id)
        ON DELETE CASCADE
);
