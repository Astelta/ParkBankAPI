--
-- PostgreSQL database dump
--

\restrict s55IyJYttx4nTIK3YMzwTYNJeuG0XH2szT1Kw62nz2oXqEpbmg9NzuBc5HYj0hT

-- Dumped from database version 17.11
-- Dumped by pg_dump version 17.12 (Ubuntu 17.12-1.pgdg24.04+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: revoke_tokens_on_password_change(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.revoke_tokens_on_password_change() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    -- Usuwa z tabeli tokens wszystkie wiersze, gdzie user_id odpowiada identyfikatorowi modyfikowanego konta
    DELETE FROM tokens WHERE user_id = NEW.id;
    
    -- Funkcja wyzwalacza przy operacji UPDATE musi zwrócić NEW, aby baza mogła poprawnie zakończyć zapisywanie wiersza
    RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tokens (
    id integer NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    token character varying(255) NOT NULL,
    user_id integer NOT NULL,
    expiration timestamp without time zone NOT NULL
);


--
-- Name: tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tokens_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tokens_id_seq OWNED BY public.tokens.id;


--
-- Name: transactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.transactions (
    id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    sender_id integer NOT NULL,
    recipient_id integer NOT NULL,
    amount bigint NOT NULL
);


--
-- Name: transactions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.transactions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: transactions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.transactions_id_seq OWNED BY public.transactions.id;


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id integer NOT NULL,
    discord_id character varying(255),
    balance bigint DEFAULT 0 NOT NULL,
    login character varying(255),
    password character varying(255)
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: tokens id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tokens ALTER COLUMN id SET DEFAULT nextval('public.tokens_id_seq'::regclass);


--
-- Name: transactions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.transactions ALTER COLUMN id SET DEFAULT nextval('public.transactions_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Name: tokens tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tokens
    ADD CONSTRAINT tokens_pkey PRIMARY KEY (id);


--
-- Name: transactions transactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT transactions_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: login; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX login ON public.users USING btree (login);


--
-- Name: token_unique_string; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX token_unique_string ON public.tokens USING btree (token);


--
-- Name: users trigger_password_change; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_password_change AFTER UPDATE ON public.users FOR EACH ROW WHEN (((old.password)::text IS DISTINCT FROM (new.password)::text)) EXECUTE FUNCTION public.revoke_tokens_on_password_change();


--
-- Name: transactions recipient_id; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT recipient_id FOREIGN KEY (recipient_id) REFERENCES public.users(id);


--
-- Name: transactions sender_id; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT sender_id FOREIGN KEY (sender_id) REFERENCES public.users(id) ON DELETE SET DEFAULT;


--
-- Name: tokens user_relation; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tokens
    ADD CONSTRAINT user_relation FOREIGN KEY (user_id) REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict s55IyJYttx4nTIK3YMzwTYNJeuG0XH2szT1Kw62nz2oXqEpbmg9NzuBc5HYj0hT

