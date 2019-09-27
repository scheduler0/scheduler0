SET TIME ZONE 'UTC';

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
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
        WHERE TABLE_NAME='jobs' AND COLUMN_NAME='missed_execs')
        THEN
        ELSE
            ALTER TABLE jobs ADD missed_execs int;
    END IF;
END $$;


DO $$
BEGIN
    IF EXISTS (
            SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
            WHERE TABLE_NAME='jobs' AND COLUMN_NAME='callback_url')
    THEN
    ELSE
        ALTER TABLE jobs ADD callback_url varchar(255);
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
            SELECT 1 FROM INFORMATION_SCHEMA.COLUMNS
            WHERE TABLE_NAME='jobs' AND COLUMN_NAME='last_status_code')
    THEN
    ELSE
        ALTER TABLE jobs ADD last_status_code int;
    END IF;
END $$;