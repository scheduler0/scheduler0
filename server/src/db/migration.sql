SET TIME ZONE 'UTC';

-- Job schema db
ALTER TABLE jobs DROP COLUMN IF EXISTS service_name;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
        WHERE TABLE_NAME='jobs' AND COLUMN_NAME='project_id')
        THEN
        ELSE
            ALTER TABLE jobs ADD project_id varchar(255);
    END IF;

    IF EXISTS (
        SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
        WHERE TABLE_NAME='jobs' AND COLUMN_NAME='missed_execs')
        THEN
        ELSE
            ALTER TABLE jobs ADD missed_execs int;
    END IF;

    IF EXISTS (
            SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
            WHERE TABLE_NAME='jobs' AND COLUMN_NAME='callback_url')
    THEN
    ELSE
        ALTER TABLE jobs ADD callback_url varchar(255);
    END IF;

    IF EXISTS (
            SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
            WHERE TABLE_NAME='jobs' AND COLUMN_NAME='last_status_code')
    THEN
    ELSE
        ALTER TABLE jobs ADD last_status_code int;
    END IF;

    IF EXISTS (
            SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
            WHERE TABLE_NAME='jobs' AND COLUMN_NAME='description')
    THEN
    ELSE
        ALTER TABLE jobs ADD description varchar(255);
    END IF;

    IF EXISTS (
            SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
            WHERE TABLE_NAME='jobs' AND COLUMN_NAME='date_created')
    THEN
    ELSE
        ALTER TABLE jobs ADD date_created timestamp;
    END IF;

    IF EXISTS (
            SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
            WHERE TABLE_NAME='jobs' AND COLUMN_NAME='missed_execs')
    THEN
        ALTER TABLE jobs DROP COLUMN missed_execs;
    END IF;

    IF EXISTS (
            SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
            WHERE TABLE_NAME='jobs' AND COLUMN_NAME='timezone')
    THEN
    ELSE
        ALTER TABLE jobs ADD timezone varchar(255) NOT NULL default 'UTC';
    END IF;
END $$;

-- Project schema db
DO $$
BEGIN
    IF EXISTS (
            SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
            WHERE TABLE_NAME='projects' AND COLUMN_NAME='date_created')
    THEN
    ELSE
        ALTER TABLE projects ADD date_created timestamp;
    END IF;

    ALTER TABLE projects DROP COLUMN  IF EXISTS user_id;
END $$;

-- Executions schema db
DO $$
    BEGIN
        IF EXISTS (
                SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                WHERE TABLE_NAME='executions' AND COLUMN_NAME='date_created')
        THEN
        ELSE
            ALTER TABLE executions ADD date_created timestamp;
        END IF;

        ALTER TABLE executions DROP COLUMN  IF EXISTS user_id;

        IF EXISTS (
                SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
                WHERE TABLE_NAME='executions' AND COLUMN_NAME='token')
        THEN
        ELSE
            ALTER TABLE executions ADD token varchar;
        END IF;
    END $$;
