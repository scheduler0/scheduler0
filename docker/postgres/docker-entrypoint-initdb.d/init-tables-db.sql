DROP DATABASE IF EXISTS scheduler0_test;
CREATE DATABASE scheduler0_test ENCODING = 'UTF8' LOCALE = 'en_US.utf8';

ALTER DATABASE scheduler0_test OWNER TO core;

SET statement_timeout = 0;
SET lock_timeout = 0;
SET log_min_messages = DEBUG5;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

CREATE TABLE public.credentials
(
    id                               SERIAL PRIMARY KEY                     NOT NULL,
    uuid                             uuid UNIQUE              DEFAULT gen_random_uuid(),
    archived                         boolean                                NOT NULL,
    platform                         text                                   NOT NULL,
    api_key                          text,
    api_secret                       text,
    ip_restriction                   text,
    http_referrer_restriction        text,
    ios_bundle_id_restriction        text,
    android_package_name_restriction text,
    date_created                     timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE public.credentials OWNER TO core;

CREATE TABLE public.executions
(
    id             SERIAL PRIMARY KEY                     NOT NULL,
    uuid           uuid UNIQUE              DEFAULT gen_random_uuid(),
    job_uuid       uuid                                   NOT NULL,
    time_added     timestamp with time zone               NOT NULL,
    time_executed  timestamp with time zone,
    execution_time bigint,
    status_code    text,
    date_created   timestamp with time zone DEFAULT now() NOT NULL
);


CREATE TABLE public.jobs
(
    id             SERIAL PRIMARY KEY                                 NOT NULL,
    uuid           uuid UNIQUE              DEFAULT gen_random_uuid() NOT NULL,
    project_uuid   uuid                                               NOT NULL,
    spec           text                                               NOT NULL,
    callback_url   text                                               NOT NULL,
    execution_type text                                               NOT NULL,
    date_created   timestamp with time zone DEFAULT now()             NOT NULL
);


CREATE TABLE public.projects
(
    id           SERIAL PRIMARY KEY                                 NOT NULL,
    uuid         uuid UNIQUE              DEFAULT gen_random_uuid() NOT NULL,
    name         text UNIQUE                                        NOT NULL,
    description  text                                               NOT NULL,
    date_created timestamp with time zone DEFAULT now()             NOT NULL
);


ALTER TABLE ONLY public.executions
    ADD CONSTRAINT executions_job_uuid_fkey FOREIGN KEY (job_uuid) REFERENCES public.jobs (uuid);


ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_project_uuid_fkey FOREIGN KEY (project_uuid) REFERENCES public.projects (uuid);
