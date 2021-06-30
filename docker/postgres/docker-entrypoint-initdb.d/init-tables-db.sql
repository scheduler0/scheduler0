--
-- PostgreSQL database dump
--

-- Dumped from database version 13.2
-- Dumped by pg_dump version 13.2

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

DROP DATABASE IF EXISTS scheduler0_test;
--
-- Name: scheduler0_test; Type: DATABASE; Schema: -; Owner: core
--

CREATE DATABASE scheduler0_test WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE = 'en_US.utf8';


ALTER DATABASE scheduler0_test OWNER TO core;

\connect scheduler0_test

SET statement_timeout = 0;
SET lock_timeout = 0;
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

--
-- Name: credentials; Type: TABLE; Schema: public; Owner: core
--

CREATE TABLE public.credentials (
                                    id bigint NOT NULL,
                                    uuid uuid DEFAULT gen_random_uuid(),
                                    archived boolean NOT NULL,
                                    platform text NOT NULL,
                                    api_key text,
                                    api_secret text,
                                    ip_restriction text,
                                    http_referrer_restriction text,
                                    ios_bundle_id_restriction text,
                                    android_package_name_restriction text,
                                    date_created timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.credentials OWNER TO core;

--
-- Name: credentials_id_seq; Type: SEQUENCE; Schema: public; Owner: core
--

CREATE SEQUENCE public.credentials_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.credentials_id_seq OWNER TO core;

--
-- Name: credentials_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: core
--

ALTER SEQUENCE public.credentials_id_seq OWNED BY public.credentials.id;


--
-- Name: executions; Type: TABLE; Schema: public; Owner: core
--

CREATE TABLE public.executions (
                                   id bigint NOT NULL,
                                   uuid uuid DEFAULT gen_random_uuid(),
                                   job_id bigint NOT NULL,
                                   job_uuid uuid NOT NULL,
                                   time_added timestamp with time zone NOT NULL,
                                   time_executed timestamp with time zone,
                                   execution_time bigint,
                                   status_code text,
                                   date_created timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.executions OWNER TO core;

--
-- Name: executions_id_seq; Type: SEQUENCE; Schema: public; Owner: core
--

CREATE SEQUENCE public.executions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.executions_id_seq OWNER TO core;

--
-- Name: executions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: core
--

ALTER SEQUENCE public.executions_id_seq OWNED BY public.executions.id;


--
-- Name: jobs; Type: TABLE; Schema: public; Owner: core
--

CREATE TABLE public.jobs (
                             id bigint NOT NULL,
                             uuid uuid DEFAULT gen_random_uuid() NOT NULL,
                             project_id bigint NOT NULL,
                             project_uuid uuid NOT NULL,
                             spec text NOT NULL,
                             callback_url text NOT NULL,
                             execution_type text NOT NULL,
                             date_created timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.jobs OWNER TO core;

--
-- Name: jobs_id_seq; Type: SEQUENCE; Schema: public; Owner: core
--

CREATE SEQUENCE public.jobs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.jobs_id_seq OWNER TO core;

--
-- Name: jobs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: core
--

ALTER SEQUENCE public.jobs_id_seq OWNED BY public.jobs.id;


--
-- Name: projects; Type: TABLE; Schema: public; Owner: core
--

CREATE TABLE public.projects (
                                 id bigint NOT NULL,
                                 uuid uuid DEFAULT gen_random_uuid() NOT NULL,
                                 name text NOT NULL,
                                 description text NOT NULL,
                                 date_created timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.projects OWNER TO core;

--
-- Name: projects_id_seq; Type: SEQUENCE; Schema: public; Owner: core
--

CREATE SEQUENCE public.projects_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.projects_id_seq OWNER TO core;

--
-- Name: projects_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: core
--

ALTER SEQUENCE public.projects_id_seq OWNED BY public.projects.id;


--
-- Name: credentials id; Type: DEFAULT; Schema: public; Owner: core
--

ALTER TABLE ONLY public.credentials ALTER COLUMN id SET DEFAULT nextval('public.credentials_id_seq'::regclass);


--
-- Name: executions id; Type: DEFAULT; Schema: public; Owner: core
--

ALTER TABLE ONLY public.executions ALTER COLUMN id SET DEFAULT nextval('public.executions_id_seq'::regclass);


--
-- Name: jobs id; Type: DEFAULT; Schema: public; Owner: core
--

ALTER TABLE ONLY public.jobs ALTER COLUMN id SET DEFAULT nextval('public.jobs_id_seq'::regclass);


--
-- Name: projects id; Type: DEFAULT; Schema: public; Owner: core
--

ALTER TABLE ONLY public.projects ALTER COLUMN id SET DEFAULT nextval('public.projects_id_seq'::regclass);


--
-- Data for Name: credentials; Type: TABLE DATA; Schema: public; Owner: core
--

COPY public.credentials (id, uuid, archived, platform, api_key, api_secret, ip_restriction, http_referrer_restriction, ios_bundle_id_restriction, android_package_name_restriction, date_created) FROM stdin;
\.


--
-- Data for Name: executions; Type: TABLE DATA; Schema: public; Owner: core
--

COPY public.executions (id, uuid, job_id, job_uuid, time_added, time_executed, execution_time, status_code, date_created) FROM stdin;
\.


--
-- Data for Name: jobs; Type: TABLE DATA; Schema: public; Owner: core
--

COPY public.jobs (id, uuid, project_id, project_uuid, spec, callback_url, execution_type, date_created) FROM stdin;
\.


--
-- Data for Name: projects; Type: TABLE DATA; Schema: public; Owner: core
--

COPY public.projects (id, uuid, name, description, date_created) FROM stdin;
\.


--
-- Name: credentials_id_seq; Type: SEQUENCE SET; Schema: public; Owner: core
--

SELECT pg_catalog.setval('public.credentials_id_seq', 1, false);


--
-- Name: executions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: core
--

SELECT pg_catalog.setval('public.executions_id_seq', 1, false);


--
-- Name: jobs_id_seq; Type: SEQUENCE SET; Schema: public; Owner: core
--

SELECT pg_catalog.setval('public.jobs_id_seq', 1, false);


--
-- Name: projects_id_seq; Type: SEQUENCE SET; Schema: public; Owner: core
--

SELECT pg_catalog.setval('public.projects_id_seq', 1, false);


--
-- Name: credentials credentials_pkey; Type: CONSTRAINT; Schema: public; Owner: core
--

ALTER TABLE ONLY public.credentials
    ADD CONSTRAINT credentials_pkey PRIMARY KEY (id);


--
-- Name: credentials credentials_uuid_key; Type: CONSTRAINT; Schema: public; Owner: core
--

ALTER TABLE ONLY public.credentials
    ADD CONSTRAINT credentials_uuid_key UNIQUE (uuid);


--
-- Name: executions executions_pkey; Type: CONSTRAINT; Schema: public; Owner: core
--

ALTER TABLE ONLY public.executions
    ADD CONSTRAINT executions_pkey PRIMARY KEY (id);


--
-- Name: executions executions_uuid_key; Type: CONSTRAINT; Schema: public; Owner: core
--

ALTER TABLE ONLY public.executions
    ADD CONSTRAINT executions_uuid_key UNIQUE (uuid);


--
-- Name: jobs jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: core
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_pkey PRIMARY KEY (id, uuid);


--
-- Name: jobs jobs_uuid_key; Type: CONSTRAINT; Schema: public; Owner: core
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_uuid_key UNIQUE (uuid);


--
-- Name: projects projects_name_key; Type: CONSTRAINT; Schema: public; Owner: core
--

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_name_key UNIQUE (name);


--
-- Name: projects projects_pkey; Type: CONSTRAINT; Schema: public; Owner: core
--

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_pkey PRIMARY KEY (id, uuid);


--
-- Name: projects projects_uuid_key; Type: CONSTRAINT; Schema: public; Owner: core
--

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_uuid_key UNIQUE (uuid);


--
-- Name: executions executions_job_id_job_uuid_fkey; Type: FK CONSTRAINT; Schema: public; Owner: core
--

ALTER TABLE ONLY public.executions
    ADD CONSTRAINT executions_job_uuid_fkey FOREIGN KEY (job_uuid) REFERENCES public.jobs(uuid);


--
-- Name: jobs jobs_project_id_project_uuid_fkey; Type: FK CONSTRAINT; Schema: public; Owner: core
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_project_uuid_fkey FOREIGN KEY (project_uuid) REFERENCES public.projects(uuid);


--
-- PostgreSQL database dump complete
--

