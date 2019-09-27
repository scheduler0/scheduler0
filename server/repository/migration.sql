SET TIME ZONE 'UTC';

-- Job schema migrations
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
END $$;

-- Project schema migrations
DO $$
BEGIN
    IF EXISTS (
            SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
            WHERE TABLE_NAME='projects' AND COLUMN_NAME='date_created')
    THEN
    ELSE
        ALTER TABLE projects ADD date_created timestamp;
    END IF;
END $$;
