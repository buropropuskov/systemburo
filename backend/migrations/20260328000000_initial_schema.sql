--
-- PostgreSQL database dump
--

-- Dumped from database version 16.2
-- Dumped by pg_dump version 16.2

-- Started on 2026-03-28 21:33:32


--
-- TOC entry 4 (class 2615 OID 2200)
--


--
-- TOC entry 5635 (class 0 OID 0)
-- Dependencies: 4
--


--
-- TOC entry 330 (class 1255 OID 78134)
-- Name: add_all_responsibles_on_application_create(); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.add_all_responsibles_on_application_create() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    resp_record RECORD;
BEGIN
    FOR resp_record IN 
        SELECT user_id, required_approval, is_primary 
        FROM application_responsible_users 
        WHERE application_id = NEW.id
    LOOP
        INSERT INTO application_history (
            application_id, 
            user_id, 
            action_type,
            metadata
        ) VALUES (
            NEW.id,
            resp_record.user_id,
            'assigned_responsible',
            jsonb_build_object(
                'required_approval', resp_record.required_approval,
                'is_primary', resp_record.is_primary
            )
        );
    END LOOP;
    RETURN NEW;
END;
$$;


--
-- TOC entry 318 (class 1255 OID 78105)
-- Name: add_application_history_on_create(); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.add_application_history_on_create() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    INSERT INTO application_history (
        application_id, 
        user_id, 
        action_type, 
        action_status,
        new_value,
        metadata
    ) VALUES (
        NEW.id,
        NEW.sender_user_id,
        'create',
        NEW.status,
        NEW.application_number,
        jsonb_build_object(
            'timestamp', NOW(),
            'confirmation', NEW.confirmation,
            'organization_id', NEW.organization_id,
            'company_id', NEW.company_id
        )
    );
    RETURN NEW;
END;
$$;


--
-- TOC entry 333 (class 1255 OID 78489)
-- Name: check_expired_cars_trigger(); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.check_expired_cars_trigger() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    -- Обновляем статус для машин с истекшим сроком
    UPDATE cars
    SET status = 0,
        updated_at = NOW()
    WHERE status = 1
    AND (
        entry_date_to < CURRENT_DATE
        OR (entry_date_to = CURRENT_DATE AND entry_time_to <= CURRENT_TIME)
    );
    RETURN NULL;
END;
$$;


--
-- TOC entry 316 (class 1255 OID 69669)
-- Name: cleanup_old_requests(integer); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.cleanup_old_requests(days_to_keep integer DEFAULT 30) RETURNS integer
    LANGUAGE plpgsql
    AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM request_log 
    WHERE timestamp < CURRENT_TIMESTAMP - (days_to_keep || ' days')::INTERVAL;
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$;


--
-- TOC entry 315 (class 1255 OID 69621)
-- Name: decrypt_sensitive_data(text); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.decrypt_sensitive_data(text) RETURNS text
    LANGUAGE plpgsql
    AS $_$
BEGIN
    -- „ҐЄ®¤Ёа®ў ­ЁҐ Ё§ base64
    RETURN convert_from(decode($1, 'base64'), 'UTF8');
END;
$_$;


--
-- TOC entry 314 (class 1255 OID 69620)
-- Name: encrypt_sensitive_data(text); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.encrypt_sensitive_data(text) RETURNS text
    LANGUAGE plpgsql
    AS $_$
BEGIN
    -- ‡¤Ґбм ¬®¦­® ЁбЇ®«м§®ў вм pgcrypto Ё«Ё ¤агЈ®© ¬Ґв®¤ иЁда®ў ­Ёп
    -- „«п Їа®бв®вл ЁбЇ®«м§гҐ¬ base64 ў ¤Ґ¬®-жҐ«пе
    -- ‚ аҐ «м­®¬ Їа®ҐЄвҐ ЁбЇ®«м§г©вҐ pgp_sym_encrypt Ё§ pgcrypto
    RETURN encode($1::bytea, 'base64');
END;
$_$;


--
-- TOC entry 312 (class 1255 OID 69250)
-- Name: generate_application_number(); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.generate_application_number() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    today_date VARCHAR(8);
    daily_counter INTEGER;
    padded_counter VARCHAR(3);
BEGIN
    -- Получаем текущую дату в формате ГГГГММДД
    today_date := TO_CHAR(NEW.sending_datetime, 'YYYYMMDD');
    
    -- Находим количество заявок за сегодня
    SELECT COUNT(*) + 1 INTO daily_counter 
    FROM applications 
    WHERE DATE(sending_datetime) = DATE(NEW.sending_datetime);
    
    -- Форматируем счетчик
    padded_counter := LPAD(daily_counter::TEXT, 3, '0');
    
    -- Генерируем номер заявки
    NEW.application_number := '№ ' || today_date || '/' || padded_counter;
    
    RETURN NEW;
END;
$$;


--
-- TOC entry 313 (class 1255 OID 69274)
-- Name: log_application_status_change(); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.log_application_status_change() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF OLD.status IS DISTINCT FROM NEW.status THEN
        INSERT INTO application_status_history (application_id, old_status, new_status, changed_by_user_id)
        VALUES (NEW.id, OLD.status, NEW.status, NEW.responsible_user_id);
    END IF;
    RETURN NEW;
END;
$$;


--
-- TOC entry 331 (class 1255 OID 78138)
-- Name: normalize_history_order(); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.normalize_history_order() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    -- Обновляем created_at для assigned_responsible, чтобы они были после создания
    UPDATE application_history 
    SET created_at = (
        SELECT created_at + INTERVAL '1 second' * row_number() OVER (ORDER BY id)
        FROM application_history h2
        WHERE h2.application_id = NEW.application_id
        AND h2.action_type = 'assigned_responsible'
        AND h2.id = application_history.id
    )
    WHERE application_id = NEW.application_id 
    AND action_type = 'assigned_responsible';
    
    RETURN NEW;
END;
$$;


--
-- TOC entry 332 (class 1255 OID 78488)
-- Name: update_expired_cars_status(); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.update_expired_cars_status() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    UPDATE cars
    SET status = 0,
        updated_at = NOW()
    WHERE status = 1
    AND (
        entry_date_to < CURRENT_DATE
        OR (entry_date_to = CURRENT_DATE AND entry_time_to <= CURRENT_TIME)
    );
    
    -- Логируем количество обновленных машин
    RAISE NOTICE 'Updated expired cars at %', NOW();
END;
$$;


--
-- TOC entry 317 (class 1255 OID 69767)
-- Name: update_resolved_by_username(); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.update_resolved_by_username() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NEW.resolved_by IS NOT NULL AND NEW.resolved_by_username IS NULL THEN
        SELECT username INTO NEW.resolved_by_username 
        FROM users WHERE id = NEW.resolved_by;
    END IF;
    RETURN NEW;
END;
$$;


--
-- TOC entry 311 (class 1255 OID 68838)
-- Name: update_updated_at_column(); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.update_updated_at_column() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- TOC entry 308 (class 1259 OID 86749)
-- Name: announcements; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.announcements (
    id integer NOT NULL,
    title character varying(255) NOT NULL,
    description text NOT NULL,
    full_text text,
    is_important boolean DEFAULT false NOT NULL,
    is_active boolean DEFAULT false NOT NULL,
    created_by integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone,
    updated_by integer,
    activated_at timestamp without time zone,
    activated_by integer
);


--
-- TOC entry 307 (class 1259 OID 86748)
-- Name: announcements_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.announcements_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5636 (class 0 OID 0)
-- Dependencies: 307
-- Name: announcements_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.announcements_id_seq OWNED BY public.announcements.id;


--
-- TOC entry 286 (class 1259 OID 78058)
-- Name: application_approvers; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.application_approvers (
    id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    created_by integer
);


--
-- TOC entry 285 (class 1259 OID 78057)
-- Name: application_approvers_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.application_approvers_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5637 (class 0 OID 0)
-- Dependencies: 285
-- Name: application_approvers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.application_approvers_id_seq OWNED BY public.application_approvers.id;


--
-- TOC entry 256 (class 1259 OID 69032)
-- Name: application_employees; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.application_employees (
    id integer NOT NULL,
    attachment_id integer NOT NULL,
    last_name character varying(100) NOT NULL,
    first_name character varying(100) NOT NULL,
    middle_name character varying(100),
    "position" character varying(200) NOT NULL,
    citizenship_id integer,
    passport_series_number character varying(50) NOT NULL,
    patent_number character varying(100),
    other_permission character varying(200),
    order_index integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- TOC entry 255 (class 1259 OID 69031)
-- Name: application_employees_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.application_employees_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5638 (class 0 OID 0)
-- Dependencies: 255
-- Name: application_employees_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.application_employees_id_seq OWNED BY public.application_employees.id;


--
-- TOC entry 288 (class 1259 OID 78081)
-- Name: application_history; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.application_history (
    id integer NOT NULL,
    application_id integer NOT NULL,
    user_id integer NOT NULL,
    action_type character varying(50) NOT NULL,
    action_status character varying(50),
    old_value text,
    new_value text,
    comment text,
    created_at timestamp with time zone DEFAULT now(),
    metadata jsonb,
    action_user_id integer
);


--
-- TOC entry 5639 (class 0 OID 0)
-- Dependencies: 288
-- Name: TABLE application_history; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.application_history IS 'История действий с заявками';


--
-- TOC entry 5640 (class 0 OID 0)
-- Dependencies: 288
-- Name: COLUMN application_history.action_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.application_history.action_type IS 'Тип действия: approve, reject_approval, revoke_approval, take_to_work, revoke_from_work, restore_to_work, reject, forward, comment';


--
-- TOC entry 5641 (class 0 OID 0)
-- Dependencies: 288
-- Name: COLUMN application_history.action_status; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.application_history.action_status IS 'Статус действия';


--
-- TOC entry 5642 (class 0 OID 0)
-- Dependencies: 288
-- Name: COLUMN application_history.metadata; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.application_history.metadata IS 'Дополнительные данные в JSON формате';


--
-- TOC entry 287 (class 1259 OID 78080)
-- Name: application_history_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.application_history_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5643 (class 0 OID 0)
-- Dependencies: 287
-- Name: application_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.application_history_id_seq OWNED BY public.application_history.id;


--
-- TOC entry 260 (class 1259 OID 69122)
-- Name: application_items; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.application_items (
    id integer NOT NULL,
    attachment_id integer NOT NULL,
    item_name character varying(200) NOT NULL,
    quantity integer DEFAULT 1 NOT NULL,
    description text,
    order_index integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- TOC entry 259 (class 1259 OID 69121)
-- Name: application_items_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.application_items_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5644 (class 0 OID 0)
-- Dependencies: 259
-- Name: application_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.application_items_id_seq OWNED BY public.application_items.id;


--
-- TOC entry 278 (class 1259 OID 69532)
-- Name: application_responsible_users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.application_responsible_users (
    id integer NOT NULL,
    application_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    is_primary boolean DEFAULT false NOT NULL,
    required_approval boolean DEFAULT false NOT NULL,
    approval_status character varying(20) DEFAULT 'pending'::character varying,
    approval_comment text,
    approval_datetime timestamp with time zone,
    created_by integer,
    CONSTRAINT application_responsible_users_approval_status_check CHECK (((approval_status)::text = ANY ((ARRAY['pending'::character varying, 'approved'::character varying, 'rejected'::character varying])::text[])))
);


--
-- TOC entry 5645 (class 0 OID 0)
-- Dependencies: 278
-- Name: TABLE application_responsible_users; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.application_responsible_users IS 'Таблица для хранения связи заявка - ответственный пользователь';


--
-- TOC entry 5646 (class 0 OID 0)
-- Dependencies: 278
-- Name: COLUMN application_responsible_users.is_primary; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.application_responsible_users.is_primary IS 'Флаг главного ответственного (берется из organization_users или companies_users)';


--
-- TOC entry 277 (class 1259 OID 69531)
-- Name: application_responsible_users_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.application_responsible_users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5647 (class 0 OID 0)
-- Dependencies: 277
-- Name: application_responsible_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.application_responsible_users_id_seq OWNED BY public.application_responsible_users.id;


--
-- TOC entry 266 (class 1259 OID 69253)
-- Name: application_status_history; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.application_status_history (
    id integer NOT NULL,
    application_id integer NOT NULL,
    old_status character varying(20),
    new_status character varying(20) NOT NULL,
    changed_by_user_id integer,
    comment text,
    changed_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- TOC entry 265 (class 1259 OID 69252)
-- Name: application_status_history_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.application_status_history_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5648 (class 0 OID 0)
-- Dependencies: 265
-- Name: application_status_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.application_status_history_id_seq OWNED BY public.application_status_history.id;


--
-- TOC entry 290 (class 1259 OID 78269)
-- Name: application_viewers; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.application_viewers (
    id integer NOT NULL,
    application_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    created_by integer
);


--
-- TOC entry 289 (class 1259 OID 78268)
-- Name: application_viewers_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.application_viewers_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5649 (class 0 OID 0)
-- Dependencies: 289
-- Name: application_viewers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.application_viewers_id_seq OWNED BY public.application_viewers.id;


--
-- TOC entry 268 (class 1259 OID 69317)
-- Name: applications; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.applications (
    id integer NOT NULL,
    application_number character varying(50) NOT NULL,
    confirmation character varying(20) DEFAULT 'Согласование'::character varying NOT NULL,
    sending_datetime timestamp with time zone DEFAULT now() NOT NULL,
    reading_datetime timestamp with time zone,
    confirmation_datetime timestamp with time zone,
    organization_id integer NOT NULL,
    sender_user_id integer NOT NULL,
    message text,
    status character varying(20) DEFAULT 'Непрочитано'::character varying NOT NULL,
    responsible_user_id integer,
    responsible_comment text,
    data_approval boolean DEFAULT false NOT NULL,
    company_id integer
);


--
-- TOC entry 267 (class 1259 OID 69316)
-- Name: applications_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.applications_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5650 (class 0 OID 0)
-- Dependencies: 267
-- Name: applications_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.applications_id_seq OWNED BY public.applications.id;


--
-- TOC entry 264 (class 1259 OID 69213)
-- Name: attachments; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.attachments (
    id integer NOT NULL,
    application_id integer NOT NULL,
    attachment_type character varying(20) NOT NULL,
    attachment_name character varying(255) NOT NULL,
    attachment_display_name character varying(255),
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    entry_date_from date,
    entry_date_to date,
    entry_time_from time without time zone,
    entry_time_to time without time zone,
    unique_attachment_id integer,
    status integer DEFAULT 1,
    CONSTRAINT attachments_attachment_type_check CHECK (((attachment_type)::text = ANY ((ARRAY['cars'::character varying, 'people'::character varying, 'items'::character varying])::text[])))
);


--
-- TOC entry 263 (class 1259 OID 69212)
-- Name: attachments_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.attachments_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5651 (class 0 OID 0)
-- Dependencies: 263
-- Name: attachments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.attachments_id_seq OWNED BY public.attachments.id;


--
-- TOC entry 226 (class 1259 OID 60430)
-- Name: car_unload_places; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.car_unload_places (
    id integer NOT NULL,
    car_id integer NOT NULL,
    unload_place_id integer NOT NULL,
    order_index integer DEFAULT 1 NOT NULL,
    planned_time time without time zone,
    notes text
);


--
-- TOC entry 225 (class 1259 OID 60429)
-- Name: car_unload_places_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.car_unload_places_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5652 (class 0 OID 0)
-- Dependencies: 225
-- Name: car_unload_places_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.car_unload_places_id_seq OWNED BY public.car_unload_places.id;


--
-- TOC entry 270 (class 1259 OID 69358)
-- Name: cars; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.cars (
    id integer NOT NULL,
    attachment_id integer NOT NULL,
    car_number character varying(15) NOT NULL,
    car_brand character varying(50) NOT NULL,
    unload_place character varying(50),
    entry_date_from date NOT NULL,
    entry_time_from time without time zone NOT NULL,
    entry_date_to date NOT NULL,
    entry_time_to time without time zone NOT NULL,
    territory_entry_time timestamp without time zone,
    territory_status integer DEFAULT 0,
    status integer DEFAULT 0,
    date_added date DEFAULT CURRENT_DATE,
    date_removed date,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    territory_exit_time timestamp without time zone
);


--
-- TOC entry 5653 (class 0 OID 0)
-- Dependencies: 270
-- Name: COLUMN cars.territory_entry_time; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cars.territory_entry_time IS 'время въезда на территорию';


--
-- TOC entry 5654 (class 0 OID 0)
-- Dependencies: 270
-- Name: COLUMN cars.territory_status; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cars.territory_status IS '0 - не на территории, 1 - на территории, 2 - выехал';


--
-- TOC entry 300 (class 1259 OID 78463)
-- Name: cars_history; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.cars_history (
    id integer NOT NULL,
    car_id integer NOT NULL,
    user_id integer,
    action_type character varying(50) NOT NULL,
    field_name character varying(100),
    old_value text,
    new_value text,
    comment text,
    metadata jsonb,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    table_id integer
);


--
-- TOC entry 5655 (class 0 OID 0)
-- Dependencies: 300
-- Name: TABLE cars_history; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.cars_history IS 'История изменений автомобилей';


--
-- TOC entry 5656 (class 0 OID 0)
-- Dependencies: 300
-- Name: COLUMN cars_history.action_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cars_history.action_type IS 'Тип действия: create, entry, exit, update, delete, activate, deactivate';


--
-- TOC entry 299 (class 1259 OID 78462)
-- Name: cars_history_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.cars_history_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5657 (class 0 OID 0)
-- Dependencies: 299
-- Name: cars_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.cars_history_id_seq OWNED BY public.cars_history.id;


--
-- TOC entry 269 (class 1259 OID 69357)
-- Name: cars_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.cars_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5658 (class 0 OID 0)
-- Dependencies: 269
-- Name: cars_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.cars_id_seq OWNED BY public.cars.id;


--
-- TOC entry 250 (class 1259 OID 68957)
-- Name: citizenships; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.citizenships (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    icon character varying(255),
    is_active boolean DEFAULT true,
    is_default boolean DEFAULT false,
    patent_required boolean DEFAULT false,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- TOC entry 249 (class 1259 OID 68956)
-- Name: citizenships_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.citizenships_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5659 (class 0 OID 0)
-- Dependencies: 249
-- Name: citizenships_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.citizenships_id_seq OWNED BY public.citizenships.id;


--
-- TOC entry 218 (class 1259 OID 60323)
-- Name: companies; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.companies (
    id integer NOT NULL,
    name character varying(100) NOT NULL
);


--
-- TOC entry 217 (class 1259 OID 60322)
-- Name: companies_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.companies_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5660 (class 0 OID 0)
-- Dependencies: 217
-- Name: companies_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.companies_id_seq OWNED BY public.companies.id;


--
-- TOC entry 248 (class 1259 OID 68935)
-- Name: companies_tables; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.companies_tables (
    id integer NOT NULL,
    company_id integer NOT NULL,
    table_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now()
);


--
-- TOC entry 247 (class 1259 OID 68934)
-- Name: companies_tables_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.companies_tables_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5661 (class 0 OID 0)
-- Dependencies: 247
-- Name: companies_tables_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.companies_tables_id_seq OWNED BY public.companies_tables.id;


--
-- TOC entry 232 (class 1259 OID 68655)
-- Name: companies_unload_places; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.companies_unload_places (
    id integer NOT NULL,
    company_id integer NOT NULL,
    unload_place_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now()
);


--
-- TOC entry 231 (class 1259 OID 68654)
-- Name: companies_unload_places_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.companies_unload_places_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5662 (class 0 OID 0)
-- Dependencies: 231
-- Name: companies_unload_places_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.companies_unload_places_id_seq OWNED BY public.companies_unload_places.id;


--
-- TOC entry 234 (class 1259 OID 68677)
-- Name: companies_users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.companies_users (
    id integer NOT NULL,
    company_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    is_primary boolean DEFAULT false,
    required_approval boolean DEFAULT false NOT NULL
);


--
-- TOC entry 233 (class 1259 OID 68676)
-- Name: companies_users_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.companies_users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5663 (class 0 OID 0)
-- Dependencies: 233
-- Name: companies_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.companies_users_id_seq OWNED BY public.companies_users.id;


--
-- TOC entry 254 (class 1259 OID 69015)
-- Name: employee_files; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.employee_files (
    id integer NOT NULL,
    employee_id integer,
    file_path character varying(500) NOT NULL,
    file_type character varying(50) NOT NULL,
    file_name character varying(200),
    uploaded_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- TOC entry 253 (class 1259 OID 69014)
-- Name: employee_files_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.employee_files_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5664 (class 0 OID 0)
-- Dependencies: 253
-- Name: employee_files_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.employee_files_id_seq OWNED BY public.employee_files.id;


--
-- TOC entry 276 (class 1259 OID 69420)
-- Name: employee_target_tables; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.employee_target_tables (
    id integer NOT NULL,
    employee_id integer NOT NULL,
    table_id integer NOT NULL,
    order_index integer DEFAULT 1 NOT NULL
);


--
-- TOC entry 275 (class 1259 OID 69419)
-- Name: employee_target_tables_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.employee_target_tables_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5665 (class 0 OID 0)
-- Dependencies: 275
-- Name: employee_target_tables_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.employee_target_tables_id_seq OWNED BY public.employee_target_tables.id;


--
-- TOC entry 272 (class 1259 OID 69378)
-- Name: employees; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.employees (
    id integer NOT NULL,
    attachment_id integer NOT NULL,
    last_name character varying(100) NOT NULL,
    first_name character varying(100) NOT NULL,
    middle_name character varying(100),
    citizenship_id integer,
    "position" character varying(100),
    passport_series_number character varying(30),
    patent_number character varying(50),
    other_permission text,
    territory_entry_time timestamp without time zone,
    territory_status integer DEFAULT 0,
    status integer DEFAULT 0,
    date_created date DEFAULT CURRENT_DATE,
    date_deleted date,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- TOC entry 302 (class 1259 OID 78497)
-- Name: employees_history; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.employees_history (
    id integer NOT NULL,
    employee_id integer NOT NULL,
    user_id integer,
    action_type character varying(50) NOT NULL,
    field_name character varying(100),
    old_value text,
    new_value text,
    comment text,
    metadata jsonb,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    table_id integer
);


--
-- TOC entry 301 (class 1259 OID 78496)
-- Name: employees_history_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.employees_history_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5666 (class 0 OID 0)
-- Dependencies: 301
-- Name: employees_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.employees_history_id_seq OWNED BY public.employees_history.id;


--
-- TOC entry 271 (class 1259 OID 69377)
-- Name: employees_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.employees_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5667 (class 0 OID 0)
-- Dependencies: 271
-- Name: employees_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.employees_id_seq OWNED BY public.employees.id;


--
-- TOC entry 284 (class 1259 OID 69831)
-- Name: feedback; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.feedback (
    id integer NOT NULL,
    user_id integer NOT NULL,
    message text NOT NULL,
    status character varying(20) DEFAULT 'Не решено'::character varying NOT NULL,
    is_read boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    resolution_comment text,
    resolved_at timestamp with time zone
);


--
-- TOC entry 283 (class 1259 OID 69830)
-- Name: feedback_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.feedback_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5668 (class 0 OID 0)
-- Dependencies: 283
-- Name: feedback_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.feedback_id_seq OWNED BY public.feedback.id;


--
-- TOC entry 282 (class 1259 OID 69770)
-- Name: feedback_messages; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.feedback_messages (
    id integer NOT NULL,
    user_id integer NOT NULL,
    username character varying(100) NOT NULL,
    message text NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    resolved_at timestamp with time zone,
    resolved_by integer,
    resolved_by_username character varying(100),
    CONSTRAINT feedback_messages_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'in_progress'::character varying, 'resolved'::character varying])::text[])))
);


--
-- TOC entry 281 (class 1259 OID 69769)
-- Name: feedback_messages_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.feedback_messages_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5669 (class 0 OID 0)
-- Dependencies: 281
-- Name: feedback_messages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.feedback_messages_id_seq OWNED BY public.feedback_messages.id;


--
-- TOC entry 274 (class 1259 OID 69401)
-- Name: items; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.items (
    id integer NOT NULL,
    attachment_id integer NOT NULL,
    name character varying(255) NOT NULL,
    count integer DEFAULT 1 NOT NULL,
    date_created date DEFAULT CURRENT_DATE,
    date_deleted date,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- TOC entry 273 (class 1259 OID 69400)
-- Name: items_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.items_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5670 (class 0 OID 0)
-- Dependencies: 273
-- Name: items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.items_id_seq OWNED BY public.items.id;


--
-- TOC entry 238 (class 1259 OID 68708)
-- Name: license_plate_format_cells; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.license_plate_format_cells (
    id integer NOT NULL,
    format_id integer NOT NULL,
    cell_order integer NOT NULL,
    cell_type character varying(20) NOT NULL,
    min_length integer NOT NULL,
    max_length integer NOT NULL,
    allowed_letters character varying(50),
    alphabet_type character varying(20),
    language character varying(10),
    padding_char character(1) DEFAULT '0'::bpchar,
    padding_side character varying(10) DEFAULT 'left'::character varying,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT license_plate_format_cells_alphabet_type_check CHECK (((alphabet_type)::text = ANY ((ARRAY['cyrillic'::character varying, 'latin'::character varying, 'both'::character varying])::text[]))),
    CONSTRAINT license_plate_format_cells_cell_type_check CHECK (((cell_type)::text = ANY ((ARRAY['letters'::character varying, 'numbers'::character varying, 'mixed'::character varying])::text[]))),
    CONSTRAINT license_plate_format_cells_padding_side_check CHECK (((padding_side)::text = ANY ((ARRAY['left'::character varying, 'right'::character varying])::text[])))
);


--
-- TOC entry 237 (class 1259 OID 68707)
-- Name: license_plate_format_cells_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.license_plate_format_cells_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5671 (class 0 OID 0)
-- Dependencies: 237
-- Name: license_plate_format_cells_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.license_plate_format_cells_id_seq OWNED BY public.license_plate_format_cells.id;


--
-- TOC entry 236 (class 1259 OID 68699)
-- Name: license_plate_formats; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.license_plate_formats (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    country_code character varying(3),
    icon character varying(255),
    is_active boolean DEFAULT true,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    is_default boolean DEFAULT false
);


--
-- TOC entry 235 (class 1259 OID 68698)
-- Name: license_plate_formats_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.license_plate_formats_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5672 (class 0 OID 0)
-- Dependencies: 235
-- Name: license_plate_formats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.license_plate_formats_id_seq OWNED BY public.license_plate_formats.id;


--
-- TOC entry 306 (class 1259 OID 86725)
-- Name: news; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.news (
    id integer NOT NULL,
    title character varying(255) NOT NULL,
    description text NOT NULL,
    full_text text,
    created_by integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone,
    updated_by integer,
    is_active boolean DEFAULT true NOT NULL
);


--
-- TOC entry 305 (class 1259 OID 86724)
-- Name: news_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.news_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5673 (class 0 OID 0)
-- Dependencies: 305
-- Name: news_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.news_id_seq OWNED BY public.news.id;


--
-- TOC entry 304 (class 1259 OID 86686)
-- Name: notifications; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.notifications (
    id integer NOT NULL,
    user_id integer NOT NULL,
    type character varying(50) NOT NULL,
    title character varying(255) NOT NULL,
    message text NOT NULL,
    data jsonb,
    is_read boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- TOC entry 303 (class 1259 OID 86685)
-- Name: notifications_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.notifications_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5674 (class 0 OID 0)
-- Dependencies: 303
-- Name: notifications_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.notifications_id_seq OWNED BY public.notifications.id;


--
-- TOC entry 246 (class 1259 OID 68913)
-- Name: organization_tables; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.organization_tables (
    id integer NOT NULL,
    organization_id integer NOT NULL,
    table_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now()
);


--
-- TOC entry 245 (class 1259 OID 68912)
-- Name: organization_tables_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.organization_tables_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5675 (class 0 OID 0)
-- Dependencies: 245
-- Name: organization_tables_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.organization_tables_id_seq OWNED BY public.organization_tables.id;


--
-- TOC entry 228 (class 1259 OID 68611)
-- Name: organization_unload_places; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.organization_unload_places (
    id integer NOT NULL,
    organization_id integer NOT NULL,
    unload_place_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now()
);


--
-- TOC entry 227 (class 1259 OID 68610)
-- Name: organization_unload_places_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.organization_unload_places_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5676 (class 0 OID 0)
-- Dependencies: 227
-- Name: organization_unload_places_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.organization_unload_places_id_seq OWNED BY public.organization_unload_places.id;


--
-- TOC entry 230 (class 1259 OID 68633)
-- Name: organization_users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.organization_users (
    id integer NOT NULL,
    organization_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    is_primary boolean DEFAULT false,
    required_approval boolean DEFAULT false NOT NULL
);


--
-- TOC entry 229 (class 1259 OID 68632)
-- Name: organization_users_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.organization_users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5677 (class 0 OID 0)
-- Dependencies: 229
-- Name: organization_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.organization_users_id_seq OWNED BY public.organization_users.id;


--
-- TOC entry 216 (class 1259 OID 60314)
-- Name: organizations; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.organizations (
    id integer NOT NULL,
    name character varying(100) NOT NULL
);


--
-- TOC entry 215 (class 1259 OID 60313)
-- Name: organizations_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.organizations_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5678 (class 0 OID 0)
-- Dependencies: 215
-- Name: organizations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.organizations_id_seq OWNED BY public.organizations.id;


--
-- TOC entry 258 (class 1259 OID 69101)
-- Name: refresh_tokens; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.refresh_tokens (
    id integer NOT NULL,
    user_id integer NOT NULL,
    token_hash text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    is_revoked boolean DEFAULT false,
    ip_address character varying(45),
    user_agent text
);


--
-- TOC entry 257 (class 1259 OID 69100)
-- Name: refresh_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.refresh_tokens_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5679 (class 0 OID 0)
-- Dependencies: 257
-- Name: refresh_tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.refresh_tokens_id_seq OWNED BY public.refresh_tokens.id;


--
-- TOC entry 280 (class 1259 OID 69650)
-- Name: request_log; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.request_log (
    id integer NOT NULL,
    "timestamp" timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    method character varying(10) NOT NULL,
    path character varying(500) NOT NULL,
    query_params text,
    user_id integer,
    username character varying(100),
    ip_address character varying(45),
    user_agent text,
    request_body text,
    request_headers text,
    response_status integer,
    response_body text,
    response_headers text,
    duration_ms integer,
    error_message text
);


--
-- TOC entry 279 (class 1259 OID 69649)
-- Name: request_log_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.request_log_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5680 (class 0 OID 0)
-- Dependencies: 279
-- Name: request_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.request_log_id_seq OWNED BY public.request_log.id;


--
-- TOC entry 310 (class 1259 OID 86780)
-- Name: request_logs; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.request_logs (
    id bigint NOT NULL,
    user_id integer,
    username character varying(255),
    method character varying(10) NOT NULL,
    url text NOT NULL,
    headers jsonb,
    request_body text,
    response_status integer,
    response_body text,
    duration_ms integer,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- TOC entry 309 (class 1259 OID 86779)
-- Name: request_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.request_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5681 (class 0 OID 0)
-- Dependencies: 309
-- Name: request_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.request_logs_id_seq OWNED BY public.request_logs.id;


--
-- TOC entry 298 (class 1259 OID 78380)
-- Name: system_table_photos; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.system_table_photos (
    id integer NOT NULL,
    table_id integer NOT NULL,
    photo_url text NOT NULL,
    file_name character varying(255) NOT NULL,
    file_size integer,
    mime_type character varying(100),
    is_main boolean DEFAULT false,
    uploaded_at timestamp without time zone DEFAULT now(),
    uploaded_by integer
);


--
-- TOC entry 5682 (class 0 OID 0)
-- Dependencies: 298
-- Name: TABLE system_table_photos; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.system_table_photos IS 'Фотографии постов (таблиц)';


--
-- TOC entry 297 (class 1259 OID 78379)
-- Name: system_table_photos_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.system_table_photos_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5683 (class 0 OID 0)
-- Dependencies: 297
-- Name: system_table_photos_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.system_table_photos_id_seq OWNED BY public.system_table_photos.id;


--
-- TOC entry 296 (class 1259 OID 78362)
-- Name: system_table_time_slots; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.system_table_time_slots (
    id integer NOT NULL,
    table_id integer NOT NULL,
    day_of_week integer NOT NULL,
    open_time time without time zone NOT NULL,
    close_time time without time zone NOT NULL,
    is_next_day boolean DEFAULT false,
    is_active boolean DEFAULT true,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    CONSTRAINT system_table_time_slots_day_of_week_check CHECK (((day_of_week >= 0) AND (day_of_week <= 6)))
);


--
-- TOC entry 5684 (class 0 OID 0)
-- Dependencies: 296
-- Name: TABLE system_table_time_slots; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.system_table_time_slots IS 'Временные окна работы постов (таблиц)';


--
-- TOC entry 5685 (class 0 OID 0)
-- Dependencies: 296
-- Name: COLUMN system_table_time_slots.is_next_day; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.system_table_time_slots.is_next_day IS 'true если время закрытия на следующий день';


--
-- TOC entry 295 (class 1259 OID 78361)
-- Name: system_table_time_slots_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.system_table_time_slots_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5686 (class 0 OID 0)
-- Dependencies: 295
-- Name: system_table_time_slots_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.system_table_time_slots_id_seq OWNED BY public.system_table_time_slots.id;


--
-- TOC entry 242 (class 1259 OID 68878)
-- Name: system_tables; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.system_tables (
    id integer NOT NULL,
    name character varying(255) NOT NULL,
    display_name character varying(255) NOT NULL,
    table_type character varying(50) NOT NULL,
    show_fact_table boolean DEFAULT false,
    fact_table_hint text,
    is_active boolean DEFAULT true,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    instruction text,
    map_link text,
    status character varying(20) DEFAULT 'active'::character varying,
    status_comment text,
    location_description text,
    CONSTRAINT system_tables_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'inactive'::character varying, 'maintenance'::character varying])::text[]))),
    CONSTRAINT system_tables_table_type_check CHECK (((table_type)::text = ANY ((ARRAY['cars'::character varying, 'people'::character varying])::text[])))
);


--
-- TOC entry 5687 (class 0 OID 0)
-- Dependencies: 242
-- Name: COLUMN system_tables.map_link; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.system_tables.map_link IS 'Ссылка на карту для навигации';


--
-- TOC entry 5688 (class 0 OID 0)
-- Dependencies: 242
-- Name: COLUMN system_tables.status; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.system_tables.status IS 'Статус: active - активно, inactive - неактивно, maintenance - на обслуживании';


--
-- TOC entry 5689 (class 0 OID 0)
-- Dependencies: 242
-- Name: COLUMN system_tables.status_comment; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.system_tables.status_comment IS 'Комментарий к статусу (причина неактивности)';


--
-- TOC entry 5690 (class 0 OID 0)
-- Dependencies: 242
-- Name: COLUMN system_tables.location_description; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.system_tables.location_description IS 'Описание местоположения';


--
-- TOC entry 241 (class 1259 OID 68877)
-- Name: system_tables_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.system_tables_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5691 (class 0 OID 0)
-- Dependencies: 241
-- Name: system_tables_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.system_tables_id_seq OWNED BY public.system_tables.id;


--
-- TOC entry 244 (class 1259 OID 68894)
-- Name: table_fields; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.table_fields (
    id integer NOT NULL,
    table_id integer,
    field_name character varying(255) NOT NULL,
    field_type character varying(50) NOT NULL,
    display_order integer,
    is_visible boolean DEFAULT true,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- TOC entry 243 (class 1259 OID 68893)
-- Name: table_fields_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.table_fields_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5692 (class 0 OID 0)
-- Dependencies: 243
-- Name: table_fields_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.table_fields_id_seq OWNED BY public.table_fields.id;


--
-- TOC entry 262 (class 1259 OID 69155)
-- Name: unique_attachments; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.unique_attachments (
    id integer NOT NULL,
    attachment_type character varying(50) NOT NULL,
    name character varying(255) NOT NULL,
    display_name character varying(255) NOT NULL,
    title character varying(255) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    instruction text,
    is_active boolean DEFAULT true,
    CONSTRAINT unique_attachments_attachment_type_check CHECK (((attachment_type)::text = ANY ((ARRAY['cars'::character varying, 'people'::character varying, 'items'::character varying])::text[])))
);


--
-- TOC entry 261 (class 1259 OID 69154)
-- Name: unique_attachments_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.unique_attachments_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5693 (class 0 OID 0)
-- Dependencies: 261
-- Name: unique_attachments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.unique_attachments_id_seq OWNED BY public.unique_attachments.id;


--
-- TOC entry 240 (class 1259 OID 68841)
-- Name: unique_cars; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.unique_cars (
    id integer NOT NULL,
    number character varying(20) NOT NULL,
    mark character varying(100) NOT NULL,
    organization_id integer,
    company_id integer,
    format_id integer,
    user_id integer,
    status boolean DEFAULT false,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- TOC entry 239 (class 1259 OID 68840)
-- Name: unique_cars_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.unique_cars_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5694 (class 0 OID 0)
-- Dependencies: 239
-- Name: unique_cars_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.unique_cars_id_seq OWNED BY public.unique_cars.id;


--
-- TOC entry 252 (class 1259 OID 68974)
-- Name: unique_employees; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.unique_employees (
    id integer NOT NULL,
    last_name character varying(100),
    first_name character varying(100),
    middle_name character varying(100),
    citizenship_id integer,
    "position" character varying(200),
    passport_series_number character varying(50),
    patent_number character varying(100),
    other_permission character varying(200),
    organization_id integer,
    company_id integer,
    user_id integer,
    status boolean DEFAULT false,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- TOC entry 251 (class 1259 OID 68973)
-- Name: unique_employees_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.unique_employees_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5695 (class 0 OID 0)
-- Dependencies: 251
-- Name: unique_employees_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.unique_employees_id_seq OWNED BY public.unique_employees.id;


--
-- TOC entry 292 (class 1259 OID 78314)
-- Name: unload_place_photos; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.unload_place_photos (
    id integer NOT NULL,
    unload_place_id integer NOT NULL,
    photo_url text NOT NULL,
    file_name character varying(255) NOT NULL,
    file_size integer,
    mime_type character varying(100),
    is_main boolean DEFAULT false,
    uploaded_at timestamp without time zone DEFAULT now(),
    uploaded_by integer
);


--
-- TOC entry 5696 (class 0 OID 0)
-- Dependencies: 292
-- Name: TABLE unload_place_photos; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.unload_place_photos IS '”®в®Ја дЁЁ ¬Ґбв а §Јаг§ЄЁ';


--
-- TOC entry 291 (class 1259 OID 78313)
-- Name: unload_place_photos_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.unload_place_photos_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5697 (class 0 OID 0)
-- Dependencies: 291
-- Name: unload_place_photos_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.unload_place_photos_id_seq OWNED BY public.unload_place_photos.id;


--
-- TOC entry 294 (class 1259 OID 78340)
-- Name: unload_place_time_slots; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.unload_place_time_slots (
    id integer NOT NULL,
    unload_place_id integer NOT NULL,
    day_of_week integer NOT NULL,
    open_time time without time zone NOT NULL,
    close_time time without time zone NOT NULL,
    is_next_day boolean DEFAULT false,
    is_active boolean DEFAULT true,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    CONSTRAINT unload_place_time_slots_day_of_week_check CHECK (((day_of_week >= 0) AND (day_of_week <= 6)))
);


--
-- TOC entry 5698 (class 0 OID 0)
-- Dependencies: 294
-- Name: TABLE unload_place_time_slots; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.unload_place_time_slots IS '‚аҐ¬Ґ­­лҐ ®Є­  ¬Ґбв а §Јаг§ЄЁ';


--
-- TOC entry 5699 (class 0 OID 0)
-- Dependencies: 294
-- Name: COLUMN unload_place_time_slots.is_next_day; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.unload_place_time_slots.is_next_day IS 'true Ґб«Ё ўаҐ¬п § ЄалвЁп ­  б«Ґ¤гойЁ© ¤Ґ­м';


--
-- TOC entry 293 (class 1259 OID 78339)
-- Name: unload_place_time_slots_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.unload_place_time_slots_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5700 (class 0 OID 0)
-- Dependencies: 293
-- Name: unload_place_time_slots_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.unload_place_time_slots_id_seq OWNED BY public.unload_place_time_slots.id;


--
-- TOC entry 224 (class 1259 OID 60417)
-- Name: unload_places; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.unload_places (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    description text,
    is_active boolean DEFAULT true,
    created_at timestamp without time zone DEFAULT now(),
    map_link text,
    status_comment text,
    status character varying(20) DEFAULT 'active'::character varying,
    updated_at timestamp without time zone DEFAULT now(),
    CONSTRAINT unload_places_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'inactive'::character varying, 'maintenance'::character varying])::text[])))
);


--
-- TOC entry 5701 (class 0 OID 0)
-- Dependencies: 224
-- Name: COLUMN unload_places.map_link; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.unload_places.map_link IS '‘бл«Є  ­  Є авг ¤«п ­ ўЁЈ жЁЁ';


--
-- TOC entry 5702 (class 0 OID 0)
-- Dependencies: 224
-- Name: COLUMN unload_places.status_comment; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.unload_places.status_comment IS 'Љ®¬¬Ґ­в аЁ© Є бв вгбг (ЇаЁзЁ­  ­Ґ ЄвЁў­®бвЁ)';


--
-- TOC entry 5703 (class 0 OID 0)
-- Dependencies: 224
-- Name: COLUMN unload_places.status; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.unload_places.status IS '‘в вгб: active -  ЄвЁў­®, inactive - ­Ґ ЄвЁў­®, maintenance - ­  ®Ўб«г¦Ёў ­ЁЁ';


--
-- TOC entry 223 (class 1259 OID 60416)
-- Name: unload_places_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.unload_places_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5704 (class 0 OID 0)
-- Dependencies: 223
-- Name: unload_places_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.unload_places_id_seq OWNED BY public.unload_places.id;


--
-- TOC entry 222 (class 1259 OID 60375)
-- Name: user_types; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.user_types (
    id integer NOT NULL,
    name character varying(50) NOT NULL,
    code character varying(20) NOT NULL
);


--
-- TOC entry 221 (class 1259 OID 60374)
-- Name: user_types_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.user_types_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5705 (class 0 OID 0)
-- Dependencies: 221
-- Name: user_types_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.user_types_id_seq OWNED BY public.user_types.id;


--
-- TOC entry 220 (class 1259 OID 60332)
-- Name: users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.users (
    id integer NOT NULL,
    username character varying(100) NOT NULL,
    password character varying(255) NOT NULL,
    organization_id integer NOT NULL,
    company_id integer NOT NULL,
    type_id integer DEFAULT 1 NOT NULL,
    last_name character varying(100),
    first_name character varying(100),
    middle_name character varying(100),
    "position" character varying(100),
    email character varying(100),
    phone character varying(20),
    last_login_at timestamp without time zone
);


--
-- TOC entry 219 (class 1259 OID 60331)
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 5706 (class 0 OID 0)
-- Dependencies: 219
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- TOC entry 5030 (class 2604 OID 86752)
-- Name: announcements id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.announcements ALTER COLUMN id SET DEFAULT nextval('public.announcements_id_seq'::regclass);


--
-- TOC entry 4998 (class 2604 OID 78061)
-- Name: application_approvers id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_approvers ALTER COLUMN id SET DEFAULT nextval('public.application_approvers_id_seq'::regclass);


--
-- TOC entry 4941 (class 2604 OID 69035)
-- Name: application_employees id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_employees ALTER COLUMN id SET DEFAULT nextval('public.application_employees_id_seq'::regclass);


--
-- TOC entry 5000 (class 2604 OID 78084)
-- Name: application_history id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_history ALTER COLUMN id SET DEFAULT nextval('public.application_history_id_seq'::regclass);


--
-- TOC entry 4946 (class 2604 OID 69125)
-- Name: application_items id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_items ALTER COLUMN id SET DEFAULT nextval('public.application_items_id_seq'::regclass);


--
-- TOC entry 4983 (class 2604 OID 69535)
-- Name: application_responsible_users id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_responsible_users ALTER COLUMN id SET DEFAULT nextval('public.application_responsible_users_id_seq'::regclass);


--
-- TOC entry 4957 (class 2604 OID 69256)
-- Name: application_status_history id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_status_history ALTER COLUMN id SET DEFAULT nextval('public.application_status_history_id_seq'::regclass);


--
-- TOC entry 5002 (class 2604 OID 78272)
-- Name: application_viewers id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_viewers ALTER COLUMN id SET DEFAULT nextval('public.application_viewers_id_seq'::regclass);


--
-- TOC entry 4959 (class 2604 OID 69320)
-- Name: applications id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.applications ALTER COLUMN id SET DEFAULT nextval('public.applications_id_seq'::regclass);


--
-- TOC entry 4953 (class 2604 OID 69216)
-- Name: attachments id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.attachments ALTER COLUMN id SET DEFAULT nextval('public.attachments_id_seq'::regclass);


--
-- TOC entry 4891 (class 2604 OID 60433)
-- Name: car_unload_places id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.car_unload_places ALTER COLUMN id SET DEFAULT nextval('public.car_unload_places_id_seq'::regclass);


--
-- TOC entry 4964 (class 2604 OID 69361)
-- Name: cars id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.cars ALTER COLUMN id SET DEFAULT nextval('public.cars_id_seq'::regclass);


--
-- TOC entry 5020 (class 2604 OID 78466)
-- Name: cars_history id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.cars_history ALTER COLUMN id SET DEFAULT nextval('public.cars_history_id_seq'::regclass);


--
-- TOC entry 4929 (class 2604 OID 68960)
-- Name: citizenships id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.citizenships ALTER COLUMN id SET DEFAULT nextval('public.citizenships_id_seq'::regclass);


--
-- TOC entry 4882 (class 2604 OID 60326)
-- Name: companies id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies ALTER COLUMN id SET DEFAULT nextval('public.companies_id_seq'::regclass);


--
-- TOC entry 4927 (class 2604 OID 68938)
-- Name: companies_tables id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies_tables ALTER COLUMN id SET DEFAULT nextval('public.companies_tables_id_seq'::regclass);


--
-- TOC entry 4899 (class 2604 OID 68658)
-- Name: companies_unload_places id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies_unload_places ALTER COLUMN id SET DEFAULT nextval('public.companies_unload_places_id_seq'::regclass);


--
-- TOC entry 4901 (class 2604 OID 68680)
-- Name: companies_users id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies_users ALTER COLUMN id SET DEFAULT nextval('public.companies_users_id_seq'::regclass);


--
-- TOC entry 4939 (class 2604 OID 69018)
-- Name: employee_files id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employee_files ALTER COLUMN id SET DEFAULT nextval('public.employee_files_id_seq'::regclass);


--
-- TOC entry 4981 (class 2604 OID 69423)
-- Name: employee_target_tables id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employee_target_tables ALTER COLUMN id SET DEFAULT nextval('public.employee_target_tables_id_seq'::regclass);


--
-- TOC entry 4970 (class 2604 OID 69381)
-- Name: employees id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employees ALTER COLUMN id SET DEFAULT nextval('public.employees_id_seq'::regclass);


--
-- TOC entry 5022 (class 2604 OID 78500)
-- Name: employees_history id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employees_history ALTER COLUMN id SET DEFAULT nextval('public.employees_history_id_seq'::regclass);


--
-- TOC entry 4993 (class 2604 OID 69834)
-- Name: feedback id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.feedback ALTER COLUMN id SET DEFAULT nextval('public.feedback_id_seq'::regclass);


--
-- TOC entry 4990 (class 2604 OID 69773)
-- Name: feedback_messages id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.feedback_messages ALTER COLUMN id SET DEFAULT nextval('public.feedback_messages_id_seq'::regclass);


--
-- TOC entry 4976 (class 2604 OID 69404)
-- Name: items id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.items ALTER COLUMN id SET DEFAULT nextval('public.items_id_seq'::regclass);


--
-- TOC entry 4909 (class 2604 OID 68711)
-- Name: license_plate_format_cells id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.license_plate_format_cells ALTER COLUMN id SET DEFAULT nextval('public.license_plate_format_cells_id_seq'::regclass);


--
-- TOC entry 4905 (class 2604 OID 68702)
-- Name: license_plate_formats id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.license_plate_formats ALTER COLUMN id SET DEFAULT nextval('public.license_plate_formats_id_seq'::regclass);


--
-- TOC entry 5027 (class 2604 OID 86728)
-- Name: news id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.news ALTER COLUMN id SET DEFAULT nextval('public.news_id_seq'::regclass);


--
-- TOC entry 5024 (class 2604 OID 86689)
-- Name: notifications id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notifications ALTER COLUMN id SET DEFAULT nextval('public.notifications_id_seq'::regclass);


--
-- TOC entry 4925 (class 2604 OID 68916)
-- Name: organization_tables id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization_tables ALTER COLUMN id SET DEFAULT nextval('public.organization_tables_id_seq'::regclass);


--
-- TOC entry 4893 (class 2604 OID 68614)
-- Name: organization_unload_places id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization_unload_places ALTER COLUMN id SET DEFAULT nextval('public.organization_unload_places_id_seq'::regclass);


--
-- TOC entry 4895 (class 2604 OID 68636)
-- Name: organization_users id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization_users ALTER COLUMN id SET DEFAULT nextval('public.organization_users_id_seq'::regclass);


--
-- TOC entry 4881 (class 2604 OID 60317)
-- Name: organizations id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organizations ALTER COLUMN id SET DEFAULT nextval('public.organizations_id_seq'::regclass);


--
-- TOC entry 4943 (class 2604 OID 69104)
-- Name: refresh_tokens id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.refresh_tokens ALTER COLUMN id SET DEFAULT nextval('public.refresh_tokens_id_seq'::regclass);


--
-- TOC entry 4988 (class 2604 OID 69653)
-- Name: request_log id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.request_log ALTER COLUMN id SET DEFAULT nextval('public.request_log_id_seq'::regclass);


--
-- TOC entry 5034 (class 2604 OID 86783)
-- Name: request_logs id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.request_logs ALTER COLUMN id SET DEFAULT nextval('public.request_logs_id_seq'::regclass);


--
-- TOC entry 5017 (class 2604 OID 78383)
-- Name: system_table_photos id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.system_table_photos ALTER COLUMN id SET DEFAULT nextval('public.system_table_photos_id_seq'::regclass);


--
-- TOC entry 5012 (class 2604 OID 78365)
-- Name: system_table_time_slots id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.system_table_time_slots ALTER COLUMN id SET DEFAULT nextval('public.system_table_time_slots_id_seq'::regclass);


--
-- TOC entry 4916 (class 2604 OID 68881)
-- Name: system_tables id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.system_tables ALTER COLUMN id SET DEFAULT nextval('public.system_tables_id_seq'::regclass);


--
-- TOC entry 4922 (class 2604 OID 68897)
-- Name: table_fields id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.table_fields ALTER COLUMN id SET DEFAULT nextval('public.table_fields_id_seq'::regclass);


--
-- TOC entry 4949 (class 2604 OID 69158)
-- Name: unique_attachments id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unique_attachments ALTER COLUMN id SET DEFAULT nextval('public.unique_attachments_id_seq'::regclass);


--
-- TOC entry 4913 (class 2604 OID 68844)
-- Name: unique_cars id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unique_cars ALTER COLUMN id SET DEFAULT nextval('public.unique_cars_id_seq'::regclass);


--
-- TOC entry 4935 (class 2604 OID 68977)
-- Name: unique_employees id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unique_employees ALTER COLUMN id SET DEFAULT nextval('public.unique_employees_id_seq'::regclass);


--
-- TOC entry 5004 (class 2604 OID 78317)
-- Name: unload_place_photos id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unload_place_photos ALTER COLUMN id SET DEFAULT nextval('public.unload_place_photos_id_seq'::regclass);


--
-- TOC entry 5007 (class 2604 OID 78343)
-- Name: unload_place_time_slots id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unload_place_time_slots ALTER COLUMN id SET DEFAULT nextval('public.unload_place_time_slots_id_seq'::regclass);


--
-- TOC entry 4886 (class 2604 OID 60420)
-- Name: unload_places id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unload_places ALTER COLUMN id SET DEFAULT nextval('public.unload_places_id_seq'::regclass);


--
-- TOC entry 4885 (class 2604 OID 60378)
-- Name: user_types id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_types ALTER COLUMN id SET DEFAULT nextval('public.user_types_id_seq'::regclass);


--
-- TOC entry 4883 (class 2604 OID 60335)
-- Name: users id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- TOC entry 5627 (class 0 OID 86749)
-- Dependencies: 308
-- Data for Name: announcements; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5605 (class 0 OID 78058)
-- Dependencies: 286
-- Data for Name: application_approvers; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5575 (class 0 OID 69032)
-- Dependencies: 256
-- Data for Name: application_employees; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5607 (class 0 OID 78081)
-- Dependencies: 288
-- Data for Name: application_history; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5579 (class 0 OID 69122)
-- Dependencies: 260
-- Data for Name: application_items; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5597 (class 0 OID 69532)
-- Dependencies: 278
-- Data for Name: application_responsible_users; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5585 (class 0 OID 69253)
-- Dependencies: 266
-- Data for Name: application_status_history; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5609 (class 0 OID 78269)
-- Dependencies: 290
-- Data for Name: application_viewers; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5587 (class 0 OID 69317)
-- Dependencies: 268
-- Data for Name: applications; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5583 (class 0 OID 69213)
-- Dependencies: 264
-- Data for Name: attachments; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5545 (class 0 OID 60430)
-- Dependencies: 226
-- Data for Name: car_unload_places; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5589 (class 0 OID 69358)
-- Dependencies: 270
-- Data for Name: cars; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5619 (class 0 OID 78463)
-- Dependencies: 300
-- Data for Name: cars_history; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5569 (class 0 OID 68957)
-- Dependencies: 250
-- Data for Name: citizenships; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5537 (class 0 OID 60323)
-- Dependencies: 218
-- Data for Name: companies; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5567 (class 0 OID 68935)
-- Dependencies: 248
-- Data for Name: companies_tables; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5551 (class 0 OID 68655)
-- Dependencies: 232
-- Data for Name: companies_unload_places; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5553 (class 0 OID 68677)
-- Dependencies: 234
-- Data for Name: companies_users; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5573 (class 0 OID 69015)
-- Dependencies: 254
-- Data for Name: employee_files; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5595 (class 0 OID 69420)
-- Dependencies: 276
-- Data for Name: employee_target_tables; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5591 (class 0 OID 69378)
-- Dependencies: 272
-- Data for Name: employees; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5621 (class 0 OID 78497)
-- Dependencies: 302
-- Data for Name: employees_history; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5603 (class 0 OID 69831)
-- Dependencies: 284
-- Data for Name: feedback; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5601 (class 0 OID 69770)
-- Dependencies: 282
-- Data for Name: feedback_messages; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5593 (class 0 OID 69401)
-- Dependencies: 274
-- Data for Name: items; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5557 (class 0 OID 68708)
-- Dependencies: 238
-- Data for Name: license_plate_format_cells; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5555 (class 0 OID 68699)
-- Dependencies: 236
-- Data for Name: license_plate_formats; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5625 (class 0 OID 86725)
-- Dependencies: 306
-- Data for Name: news; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5623 (class 0 OID 86686)
-- Dependencies: 304
-- Data for Name: notifications; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5565 (class 0 OID 68913)
-- Dependencies: 246
-- Data for Name: organization_tables; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5547 (class 0 OID 68611)
-- Dependencies: 228
-- Data for Name: organization_unload_places; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5549 (class 0 OID 68633)
-- Dependencies: 230
-- Data for Name: organization_users; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5535 (class 0 OID 60314)
-- Dependencies: 216
-- Data for Name: organizations; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5577 (class 0 OID 69101)
-- Dependencies: 258
-- Data for Name: refresh_tokens; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5599 (class 0 OID 69650)
-- Dependencies: 280
-- Data for Name: request_log; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5629 (class 0 OID 86780)
-- Dependencies: 310
-- Data for Name: request_logs; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5617 (class 0 OID 78380)
-- Dependencies: 298
-- Data for Name: system_table_photos; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5615 (class 0 OID 78362)
-- Dependencies: 296
-- Data for Name: system_table_time_slots; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5561 (class 0 OID 68878)
-- Dependencies: 242
-- Data for Name: system_tables; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5563 (class 0 OID 68894)
-- Dependencies: 244
-- Data for Name: table_fields; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5581 (class 0 OID 69155)
-- Dependencies: 262
-- Data for Name: unique_attachments; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5559 (class 0 OID 68841)
-- Dependencies: 240
-- Data for Name: unique_cars; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5571 (class 0 OID 68974)
-- Dependencies: 252
-- Data for Name: unique_employees; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5611 (class 0 OID 78314)
-- Dependencies: 292
-- Data for Name: unload_place_photos; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5613 (class 0 OID 78340)
-- Dependencies: 294
-- Data for Name: unload_place_time_slots; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5543 (class 0 OID 60417)
-- Dependencies: 224
-- Data for Name: unload_places; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5541 (class 0 OID 60375)
-- Dependencies: 222
-- Data for Name: user_types; Type: TABLE DATA; Schema: public; Owner: postgres
--

INSERT INTO public.user_types (id, name, code) VALUES
    (1, 'Пользователь', 'user'),
    (2, 'Арендатор', 'renter'),
    (3, 'Подрядчик', 'contractor'),
    (4, 'Охранник', 'security'),
    (5, 'Руководитель', 'manager'),
    (6, 'Бюро пропусков', 'buropropuskov');
SELECT setval('public.user_types_id_seq', 6);


--
-- TOC entry 5539 (class 0 OID 60332)
-- Dependencies: 220
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: postgres
--


--
-- TOC entry 5707 (class 0 OID 0)
-- Dependencies: 307
-- Name: announcements_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5708 (class 0 OID 0)
-- Dependencies: 285
-- Name: application_approvers_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5709 (class 0 OID 0)
-- Dependencies: 255
-- Name: application_employees_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5710 (class 0 OID 0)
-- Dependencies: 287
-- Name: application_history_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5711 (class 0 OID 0)
-- Dependencies: 259
-- Name: application_items_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5712 (class 0 OID 0)
-- Dependencies: 277
-- Name: application_responsible_users_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5713 (class 0 OID 0)
-- Dependencies: 265
-- Name: application_status_history_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5714 (class 0 OID 0)
-- Dependencies: 289
-- Name: application_viewers_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5715 (class 0 OID 0)
-- Dependencies: 267
-- Name: applications_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5716 (class 0 OID 0)
-- Dependencies: 263
-- Name: attachments_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5717 (class 0 OID 0)
-- Dependencies: 225
-- Name: car_unload_places_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5718 (class 0 OID 0)
-- Dependencies: 299
-- Name: cars_history_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5719 (class 0 OID 0)
-- Dependencies: 269
-- Name: cars_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5720 (class 0 OID 0)
-- Dependencies: 249
-- Name: citizenships_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5721 (class 0 OID 0)
-- Dependencies: 217
-- Name: companies_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5722 (class 0 OID 0)
-- Dependencies: 247
-- Name: companies_tables_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5723 (class 0 OID 0)
-- Dependencies: 231
-- Name: companies_unload_places_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5724 (class 0 OID 0)
-- Dependencies: 233
-- Name: companies_users_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5725 (class 0 OID 0)
-- Dependencies: 253
-- Name: employee_files_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5726 (class 0 OID 0)
-- Dependencies: 275
-- Name: employee_target_tables_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5727 (class 0 OID 0)
-- Dependencies: 301
-- Name: employees_history_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5728 (class 0 OID 0)
-- Dependencies: 271
-- Name: employees_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5729 (class 0 OID 0)
-- Dependencies: 283
-- Name: feedback_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5730 (class 0 OID 0)
-- Dependencies: 281
-- Name: feedback_messages_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5731 (class 0 OID 0)
-- Dependencies: 273
-- Name: items_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5732 (class 0 OID 0)
-- Dependencies: 237
-- Name: license_plate_format_cells_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5733 (class 0 OID 0)
-- Dependencies: 235
-- Name: license_plate_formats_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5734 (class 0 OID 0)
-- Dependencies: 305
-- Name: news_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5735 (class 0 OID 0)
-- Dependencies: 303
-- Name: notifications_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5736 (class 0 OID 0)
-- Dependencies: 245
-- Name: organization_tables_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5737 (class 0 OID 0)
-- Dependencies: 227
-- Name: organization_unload_places_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5738 (class 0 OID 0)
-- Dependencies: 229
-- Name: organization_users_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5739 (class 0 OID 0)
-- Dependencies: 215
-- Name: organizations_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5740 (class 0 OID 0)
-- Dependencies: 257
-- Name: refresh_tokens_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5741 (class 0 OID 0)
-- Dependencies: 279
-- Name: request_log_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5742 (class 0 OID 0)
-- Dependencies: 309
-- Name: request_logs_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5743 (class 0 OID 0)
-- Dependencies: 297
-- Name: system_table_photos_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5744 (class 0 OID 0)
-- Dependencies: 295
-- Name: system_table_time_slots_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5745 (class 0 OID 0)
-- Dependencies: 241
-- Name: system_tables_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5746 (class 0 OID 0)
-- Dependencies: 243
-- Name: table_fields_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5747 (class 0 OID 0)
-- Dependencies: 261
-- Name: unique_attachments_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5748 (class 0 OID 0)
-- Dependencies: 239
-- Name: unique_cars_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5749 (class 0 OID 0)
-- Dependencies: 251
-- Name: unique_employees_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5750 (class 0 OID 0)
-- Dependencies: 291
-- Name: unload_place_photos_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5751 (class 0 OID 0)
-- Dependencies: 293
-- Name: unload_place_time_slots_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5752 (class 0 OID 0)
-- Dependencies: 223
-- Name: unload_places_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5753 (class 0 OID 0)
-- Dependencies: 221
-- Name: user_types_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5754 (class 0 OID 0)
-- Dependencies: 219
-- Name: users_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--


--
-- TOC entry 5299 (class 2606 OID 86759)
-- Name: announcements announcements_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.announcements
    ADD CONSTRAINT announcements_pkey PRIMARY KEY (id);


--
-- TOC entry 5246 (class 2606 OID 78064)
-- Name: application_approvers application_approvers_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_approvers
    ADD CONSTRAINT application_approvers_pkey PRIMARY KEY (id);


--
-- TOC entry 5248 (class 2606 OID 78066)
-- Name: application_approvers application_approvers_user_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_approvers
    ADD CONSTRAINT application_approvers_user_id_key UNIQUE (user_id);


--
-- TOC entry 5158 (class 2606 OID 69040)
-- Name: application_employees application_employees_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_employees
    ADD CONSTRAINT application_employees_pkey PRIMARY KEY (id);


--
-- TOC entry 5251 (class 2606 OID 78089)
-- Name: application_history application_history_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_history
    ADD CONSTRAINT application_history_pkey PRIMARY KEY (id);


--
-- TOC entry 5167 (class 2606 OID 69131)
-- Name: application_items application_items_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_items
    ADD CONSTRAINT application_items_pkey PRIMARY KEY (id);


--
-- TOC entry 5223 (class 2606 OID 69541)
-- Name: application_responsible_users application_responsible_users_application_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_responsible_users
    ADD CONSTRAINT application_responsible_users_application_id_user_id_key UNIQUE (application_id, user_id);


--
-- TOC entry 5225 (class 2606 OID 69539)
-- Name: application_responsible_users application_responsible_users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_responsible_users
    ADD CONSTRAINT application_responsible_users_pkey PRIMARY KEY (id);


--
-- TOC entry 5183 (class 2606 OID 69261)
-- Name: application_status_history application_status_history_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_status_history
    ADD CONSTRAINT application_status_history_pkey PRIMARY KEY (id);


--
-- TOC entry 5256 (class 2606 OID 78277)
-- Name: application_viewers application_viewers_application_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_viewers
    ADD CONSTRAINT application_viewers_application_id_user_id_key UNIQUE (application_id, user_id);


--
-- TOC entry 5258 (class 2606 OID 78275)
-- Name: application_viewers application_viewers_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_viewers
    ADD CONSTRAINT application_viewers_pkey PRIMARY KEY (id);


--
-- TOC entry 5187 (class 2606 OID 69330)
-- Name: applications applications_application_number_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.applications
    ADD CONSTRAINT applications_application_number_key UNIQUE (application_number);


--
-- TOC entry 5189 (class 2606 OID 69328)
-- Name: applications applications_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.applications
    ADD CONSTRAINT applications_pkey PRIMARY KEY (id);


--
-- TOC entry 5178 (class 2606 OID 69223)
-- Name: attachments attachments_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.attachments
    ADD CONSTRAINT attachments_pkey PRIMARY KEY (id);


--
-- TOC entry 5069 (class 2606 OID 60440)
-- Name: car_unload_places car_unload_places_car_id_unload_place_id_order_index_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.car_unload_places
    ADD CONSTRAINT car_unload_places_car_id_unload_place_id_order_index_key UNIQUE (car_id, unload_place_id, order_index);


--
-- TOC entry 5071 (class 2606 OID 60438)
-- Name: car_unload_places car_unload_places_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.car_unload_places
    ADD CONSTRAINT car_unload_places_pkey PRIMARY KEY (id);


--
-- TOC entry 5274 (class 2606 OID 78471)
-- Name: cars_history cars_history_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.cars_history
    ADD CONSTRAINT cars_history_pkey PRIMARY KEY (id);


--
-- TOC entry 5201 (class 2606 OID 69368)
-- Name: cars cars_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.cars
    ADD CONSTRAINT cars_pkey PRIMARY KEY (id);


--
-- TOC entry 5138 (class 2606 OID 68967)
-- Name: citizenships citizenships_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.citizenships
    ADD CONSTRAINT citizenships_pkey PRIMARY KEY (id);


--
-- TOC entry 5053 (class 2606 OID 60330)
-- Name: companies companies_name_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies
    ADD CONSTRAINT companies_name_key UNIQUE (name);


--
-- TOC entry 5055 (class 2606 OID 60328)
-- Name: companies companies_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies
    ADD CONSTRAINT companies_pkey PRIMARY KEY (id);


--
-- TOC entry 5132 (class 2606 OID 68943)
-- Name: companies_tables companies_tables_company_id_table_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies_tables
    ADD CONSTRAINT companies_tables_company_id_table_id_key UNIQUE (company_id, table_id);


--
-- TOC entry 5134 (class 2606 OID 68941)
-- Name: companies_tables companies_tables_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies_tables
    ADD CONSTRAINT companies_tables_pkey PRIMARY KEY (id);


--
-- TOC entry 5088 (class 2606 OID 68663)
-- Name: companies_unload_places companies_unload_places_company_id_unload_place_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies_unload_places
    ADD CONSTRAINT companies_unload_places_company_id_unload_place_id_key UNIQUE (company_id, unload_place_id);


--
-- TOC entry 5090 (class 2606 OID 68661)
-- Name: companies_unload_places companies_unload_places_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies_unload_places
    ADD CONSTRAINT companies_unload_places_pkey PRIMARY KEY (id);


--
-- TOC entry 5094 (class 2606 OID 68685)
-- Name: companies_users companies_users_company_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies_users
    ADD CONSTRAINT companies_users_company_id_user_id_key UNIQUE (company_id, user_id);


--
-- TOC entry 5096 (class 2606 OID 68683)
-- Name: companies_users companies_users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies_users
    ADD CONSTRAINT companies_users_pkey PRIMARY KEY (id);


--
-- TOC entry 5154 (class 2606 OID 69023)
-- Name: employee_files employee_files_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employee_files
    ADD CONSTRAINT employee_files_pkey PRIMARY KEY (id);


--
-- TOC entry 5217 (class 2606 OID 69428)
-- Name: employee_target_tables employee_target_tables_employee_id_table_id_order_index_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employee_target_tables
    ADD CONSTRAINT employee_target_tables_employee_id_table_id_order_index_key UNIQUE (employee_id, table_id, order_index);


--
-- TOC entry 5219 (class 2606 OID 69426)
-- Name: employee_target_tables employee_target_tables_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employee_target_tables
    ADD CONSTRAINT employee_target_tables_pkey PRIMARY KEY (id);


--
-- TOC entry 5282 (class 2606 OID 78505)
-- Name: employees_history employees_history_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employees_history
    ADD CONSTRAINT employees_history_pkey PRIMARY KEY (id);


--
-- TOC entry 5208 (class 2606 OID 69390)
-- Name: employees employees_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employees
    ADD CONSTRAINT employees_pkey PRIMARY KEY (id);


--
-- TOC entry 5238 (class 2606 OID 69780)
-- Name: feedback_messages feedback_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.feedback_messages
    ADD CONSTRAINT feedback_messages_pkey PRIMARY KEY (id);


--
-- TOC entry 5242 (class 2606 OID 69842)
-- Name: feedback feedback_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.feedback
    ADD CONSTRAINT feedback_pkey PRIMARY KEY (id);


--
-- TOC entry 5215 (class 2606 OID 69410)
-- Name: items items_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.items
    ADD CONSTRAINT items_pkey PRIMARY KEY (id);


--
-- TOC entry 5105 (class 2606 OID 68719)
-- Name: license_plate_format_cells license_plate_format_cells_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.license_plate_format_cells
    ADD CONSTRAINT license_plate_format_cells_pkey PRIMARY KEY (id);


--
-- TOC entry 5101 (class 2606 OID 68706)
-- Name: license_plate_formats license_plate_formats_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.license_plate_formats
    ADD CONSTRAINT license_plate_formats_pkey PRIMARY KEY (id);


--
-- TOC entry 5297 (class 2606 OID 86734)
-- Name: news news_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.news
    ADD CONSTRAINT news_pkey PRIMARY KEY (id);


--
-- TOC entry 5292 (class 2606 OID 86695)
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);


--
-- TOC entry 5128 (class 2606 OID 68921)
-- Name: organization_tables organization_tables_organization_id_table_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization_tables
    ADD CONSTRAINT organization_tables_organization_id_table_id_key UNIQUE (organization_id, table_id);


--
-- TOC entry 5130 (class 2606 OID 68919)
-- Name: organization_tables organization_tables_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization_tables
    ADD CONSTRAINT organization_tables_pkey PRIMARY KEY (id);


--
-- TOC entry 5077 (class 2606 OID 68619)
-- Name: organization_unload_places organization_unload_places_organization_id_unload_place_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization_unload_places
    ADD CONSTRAINT organization_unload_places_organization_id_unload_place_id_key UNIQUE (organization_id, unload_place_id);


--
-- TOC entry 5079 (class 2606 OID 68617)
-- Name: organization_unload_places organization_unload_places_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization_unload_places
    ADD CONSTRAINT organization_unload_places_pkey PRIMARY KEY (id);


--
-- TOC entry 5084 (class 2606 OID 68641)
-- Name: organization_users organization_users_organization_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization_users
    ADD CONSTRAINT organization_users_organization_id_user_id_key UNIQUE (organization_id, user_id);


--
-- TOC entry 5086 (class 2606 OID 68639)
-- Name: organization_users organization_users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization_users
    ADD CONSTRAINT organization_users_pkey PRIMARY KEY (id);


--
-- TOC entry 5049 (class 2606 OID 60321)
-- Name: organizations organizations_name_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organizations
    ADD CONSTRAINT organizations_name_key UNIQUE (name);


--
-- TOC entry 5051 (class 2606 OID 60319)
-- Name: organizations organizations_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organizations
    ADD CONSTRAINT organizations_pkey PRIMARY KEY (id);


--
-- TOC entry 5163 (class 2606 OID 69110)
-- Name: refresh_tokens refresh_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_pkey PRIMARY KEY (id);


--
-- TOC entry 5165 (class 2606 OID 69112)
-- Name: refresh_tokens refresh_tokens_token_hash_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_token_hash_key UNIQUE (token_hash);


--
-- TOC entry 5236 (class 2606 OID 69658)
-- Name: request_log request_log_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.request_log
    ADD CONSTRAINT request_log_pkey PRIMARY KEY (id);


--
-- TOC entry 5309 (class 2606 OID 86788)
-- Name: request_logs request_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.request_logs
    ADD CONSTRAINT request_logs_pkey PRIMARY KEY (id);


--
-- TOC entry 5272 (class 2606 OID 78389)
-- Name: system_table_photos system_table_photos_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.system_table_photos
    ADD CONSTRAINT system_table_photos_pkey PRIMARY KEY (id);


--
-- TOC entry 5269 (class 2606 OID 78372)
-- Name: system_table_time_slots system_table_time_slots_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.system_table_time_slots
    ADD CONSTRAINT system_table_time_slots_pkey PRIMARY KEY (id);


--
-- TOC entry 5118 (class 2606 OID 68892)
-- Name: system_tables system_tables_name_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.system_tables
    ADD CONSTRAINT system_tables_name_key UNIQUE (name);


--
-- TOC entry 5120 (class 2606 OID 68890)
-- Name: system_tables system_tables_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.system_tables
    ADD CONSTRAINT system_tables_pkey PRIMARY KEY (id);


--
-- TOC entry 5124 (class 2606 OID 68901)
-- Name: table_fields table_fields_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.table_fields
    ADD CONSTRAINT table_fields_pkey PRIMARY KEY (id);


--
-- TOC entry 5174 (class 2606 OID 69168)
-- Name: unique_attachments unique_attachments_name_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unique_attachments
    ADD CONSTRAINT unique_attachments_name_key UNIQUE (name);


--
-- TOC entry 5176 (class 2606 OID 69166)
-- Name: unique_attachments unique_attachments_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unique_attachments
    ADD CONSTRAINT unique_attachments_pkey PRIMARY KEY (id);


--
-- TOC entry 5114 (class 2606 OID 68848)
-- Name: unique_cars unique_cars_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unique_cars
    ADD CONSTRAINT unique_cars_pkey PRIMARY KEY (id);


--
-- TOC entry 5152 (class 2606 OID 68984)
-- Name: unique_employees unique_employees_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unique_employees
    ADD CONSTRAINT unique_employees_pkey PRIMARY KEY (id);


--
-- TOC entry 5263 (class 2606 OID 78323)
-- Name: unload_place_photos unload_place_photos_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unload_place_photos
    ADD CONSTRAINT unload_place_photos_pkey PRIMARY KEY (id);


--
-- TOC entry 5266 (class 2606 OID 78350)
-- Name: unload_place_time_slots unload_place_time_slots_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unload_place_time_slots
    ADD CONSTRAINT unload_place_time_slots_pkey PRIMARY KEY (id);


--
-- TOC entry 5065 (class 2606 OID 60428)
-- Name: unload_places unload_places_name_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unload_places
    ADD CONSTRAINT unload_places_name_key UNIQUE (name);


--
-- TOC entry 5067 (class 2606 OID 60426)
-- Name: unload_places unload_places_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unload_places
    ADD CONSTRAINT unload_places_pkey PRIMARY KEY (id);


--
-- TOC entry 5061 (class 2606 OID 60382)
-- Name: user_types user_types_code_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_types
    ADD CONSTRAINT user_types_code_key UNIQUE (code);


--
-- TOC entry 5063 (class 2606 OID 60380)
-- Name: user_types user_types_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_types
    ADD CONSTRAINT user_types_pkey PRIMARY KEY (id);


--
-- TOC entry 5057 (class 2606 OID 60339)
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- TOC entry 5059 (class 2606 OID 60341)
-- Name: users users_username_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_username_key UNIQUE (username);


--
-- TOC entry 5300 (class 1259 OID 86778)
-- Name: idx_announcements_activated_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_announcements_activated_at ON public.announcements USING btree (activated_at DESC);


--
-- TOC entry 5301 (class 1259 OID 86777)
-- Name: idx_announcements_created_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_announcements_created_at ON public.announcements USING btree (created_at DESC);


--
-- TOC entry 5302 (class 1259 OID 86775)
-- Name: idx_announcements_is_active; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_announcements_is_active ON public.announcements USING btree (is_active);


--
-- TOC entry 5303 (class 1259 OID 86776)
-- Name: idx_announcements_is_important; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_announcements_is_important ON public.announcements USING btree (is_important);


--
-- TOC entry 5226 (class 1259 OID 69552)
-- Name: idx_app_resp_users_application_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_app_resp_users_application_id ON public.application_responsible_users USING btree (application_id);


--
-- TOC entry 5227 (class 1259 OID 78054)
-- Name: idx_app_resp_users_approval_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_app_resp_users_approval_status ON public.application_responsible_users USING btree (approval_status);


--
-- TOC entry 5228 (class 1259 OID 78053)
-- Name: idx_app_resp_users_required_approval; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_app_resp_users_required_approval ON public.application_responsible_users USING btree (required_approval);


--
-- TOC entry 5229 (class 1259 OID 69553)
-- Name: idx_app_resp_users_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_app_resp_users_user_id ON public.application_responsible_users USING btree (user_id);


--
-- TOC entry 5249 (class 1259 OID 78077)
-- Name: idx_application_approvers_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_application_approvers_user_id ON public.application_approvers USING btree (user_id);


--
-- TOC entry 5159 (class 1259 OID 69069)
-- Name: idx_application_employees_application_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_application_employees_application_id ON public.application_employees USING btree (attachment_id);


--
-- TOC entry 5252 (class 1259 OID 78100)
-- Name: idx_application_history_application_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_application_history_application_id ON public.application_history USING btree (application_id);


--
-- TOC entry 5253 (class 1259 OID 78102)
-- Name: idx_application_history_created_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_application_history_created_at ON public.application_history USING btree (created_at DESC);


--
-- TOC entry 5254 (class 1259 OID 78101)
-- Name: idx_application_history_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_application_history_user_id ON public.application_history USING btree (user_id);


--
-- TOC entry 5168 (class 1259 OID 69137)
-- Name: idx_application_items_application_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_application_items_application_id ON public.application_items USING btree (attachment_id);


--
-- TOC entry 5169 (class 1259 OID 69247)
-- Name: idx_application_items_attachment_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_application_items_attachment_id ON public.application_items USING btree (attachment_id);


--
-- TOC entry 5170 (class 1259 OID 69138)
-- Name: idx_application_items_item_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_application_items_item_name ON public.application_items USING btree (item_name);


--
-- TOC entry 5184 (class 1259 OID 69272)
-- Name: idx_application_status_history_application_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_application_status_history_application_id ON public.application_status_history USING btree (application_id);


--
-- TOC entry 5185 (class 1259 OID 69273)
-- Name: idx_application_status_history_changed_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_application_status_history_changed_at ON public.application_status_history USING btree (changed_at);


--
-- TOC entry 5259 (class 1259 OID 78293)
-- Name: idx_application_viewers_application_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_application_viewers_application_id ON public.application_viewers USING btree (application_id);


--
-- TOC entry 5260 (class 1259 OID 78294)
-- Name: idx_application_viewers_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_application_viewers_user_id ON public.application_viewers USING btree (user_id);


--
-- TOC entry 5190 (class 1259 OID 69350)
-- Name: idx_applications_confirmation; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_applications_confirmation ON public.applications USING btree (confirmation);


--
-- TOC entry 5191 (class 1259 OID 69578)
-- Name: idx_applications_dates; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_applications_dates ON public.applications USING btree (sending_datetime DESC) WHERE ((status)::text <> 'Удалено'::text);


--
-- TOC entry 5192 (class 1259 OID 69589)
-- Name: idx_applications_fts_simple; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_applications_fts_simple ON public.applications USING gin (to_tsvector('russian'::regconfig, (((((((COALESCE(application_number, ''::character varying))::text || ' '::text) || COALESCE(message, ''::text)) || ' '::text) || (COALESCE(status, ''::character varying))::text) || ' '::text) || (COALESCE(confirmation, ''::character varying))::text)));


--
-- TOC entry 5193 (class 1259 OID 78487)
-- Name: idx_applications_org_company; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_applications_org_company ON public.applications USING btree (organization_id, company_id);


--
-- TOC entry 5194 (class 1259 OID 69346)
-- Name: idx_applications_organization; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_applications_organization ON public.applications USING btree (organization_id);


--
-- TOC entry 5195 (class 1259 OID 69348)
-- Name: idx_applications_responsible; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_applications_responsible ON public.applications USING btree (responsible_user_id);


--
-- TOC entry 5196 (class 1259 OID 69347)
-- Name: idx_applications_sender; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_applications_sender ON public.applications USING btree (sender_user_id);


--
-- TOC entry 5197 (class 1259 OID 69577)
-- Name: idx_applications_sender_org; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_applications_sender_org ON public.applications USING btree (sender_user_id, organization_id);


--
-- TOC entry 5198 (class 1259 OID 69488)
-- Name: idx_applications_sending_datetime; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_applications_sending_datetime ON public.applications USING btree (sending_datetime DESC);


--
-- TOC entry 5199 (class 1259 OID 69349)
-- Name: idx_applications_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_applications_status ON public.applications USING btree (status);


--
-- TOC entry 5179 (class 1259 OID 69229)
-- Name: idx_attachments_application_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_attachments_application_id ON public.attachments USING btree (application_id);


--
-- TOC entry 5180 (class 1259 OID 78493)
-- Name: idx_attachments_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_attachments_status ON public.attachments USING btree (status);


--
-- TOC entry 5181 (class 1259 OID 69230)
-- Name: idx_attachments_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_attachments_type ON public.attachments USING btree (attachment_type);


--
-- TOC entry 5072 (class 1259 OID 60451)
-- Name: idx_car_unload_places_car_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_car_unload_places_car_id ON public.car_unload_places USING btree (car_id);


--
-- TOC entry 5073 (class 1259 OID 60452)
-- Name: idx_car_unload_places_place_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_car_unload_places_place_id ON public.car_unload_places USING btree (unload_place_id);


--
-- TOC entry 5202 (class 1259 OID 69374)
-- Name: idx_cars_attachment_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_cars_attachment_id ON public.cars USING btree (attachment_id);


--
-- TOC entry 5203 (class 1259 OID 69590)
-- Name: idx_cars_fts_simple; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_cars_fts_simple ON public.cars USING gin (to_tsvector('russian'::regconfig, (((COALESCE(car_number, ''::character varying))::text || ' '::text) || (COALESCE(car_brand, ''::character varying))::text)));


--
-- TOC entry 5275 (class 1259 OID 78484)
-- Name: idx_cars_history_action_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_cars_history_action_type ON public.cars_history USING btree (action_type);


--
-- TOC entry 5276 (class 1259 OID 78482)
-- Name: idx_cars_history_car_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_cars_history_car_id ON public.cars_history USING btree (car_id);


--
-- TOC entry 5277 (class 1259 OID 78485)
-- Name: idx_cars_history_created_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_cars_history_created_at ON public.cars_history USING btree (created_at);


--
-- TOC entry 5278 (class 1259 OID 78532)
-- Name: idx_cars_history_table_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_cars_history_table_id ON public.cars_history USING btree (table_id);


--
-- TOC entry 5279 (class 1259 OID 78483)
-- Name: idx_cars_history_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_cars_history_user_id ON public.cars_history USING btree (user_id);


--
-- TOC entry 5280 (class 1259 OID 78495)
-- Name: idx_cars_history_user_id_null; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_cars_history_user_id_null ON public.cars_history USING btree (user_id) WHERE (user_id IS NULL);


--
-- TOC entry 5204 (class 1259 OID 78486)
-- Name: idx_cars_number_brand; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_cars_number_brand ON public.cars USING btree (car_number, car_brand);


--
-- TOC entry 5205 (class 1259 OID 69375)
-- Name: idx_cars_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_cars_status ON public.cars USING btree (status);


--
-- TOC entry 5206 (class 1259 OID 78491)
-- Name: idx_cars_status_dates; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_cars_status_dates ON public.cars USING btree (status, entry_date_to, entry_time_to);


--
-- TOC entry 5139 (class 1259 OID 68969)
-- Name: idx_citizenships_is_active; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_citizenships_is_active ON public.citizenships USING btree (is_active);


--
-- TOC entry 5140 (class 1259 OID 68970)
-- Name: idx_citizenships_is_default; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_citizenships_is_default ON public.citizenships USING btree (is_default);


--
-- TOC entry 5141 (class 1259 OID 68968)
-- Name: idx_citizenships_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_citizenships_name ON public.citizenships USING btree (name);


--
-- TOC entry 5097 (class 1259 OID 78056)
-- Name: idx_comp_users_required_approval; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_comp_users_required_approval ON public.companies_users USING btree (required_approval);


--
-- TOC entry 5135 (class 1259 OID 68954)
-- Name: idx_companies_tables_company_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_companies_tables_company_id ON public.companies_tables USING btree (company_id);


--
-- TOC entry 5136 (class 1259 OID 68955)
-- Name: idx_companies_tables_table_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_companies_tables_table_id ON public.companies_tables USING btree (table_id);


--
-- TOC entry 5091 (class 1259 OID 68674)
-- Name: idx_companies_unload_places_company_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_companies_unload_places_company_id ON public.companies_unload_places USING btree (company_id);


--
-- TOC entry 5092 (class 1259 OID 68675)
-- Name: idx_companies_unload_places_place_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_companies_unload_places_place_id ON public.companies_unload_places USING btree (unload_place_id);


--
-- TOC entry 5098 (class 1259 OID 68696)
-- Name: idx_companies_users_company_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_companies_users_company_id ON public.companies_users USING btree (company_id);


--
-- TOC entry 5099 (class 1259 OID 68697)
-- Name: idx_companies_users_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_companies_users_user_id ON public.companies_users USING btree (user_id);


--
-- TOC entry 5155 (class 1259 OID 69029)
-- Name: idx_employee_files_employee_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_employee_files_employee_id ON public.employee_files USING btree (employee_id);


--
-- TOC entry 5156 (class 1259 OID 69030)
-- Name: idx_employee_files_file_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_employee_files_file_type ON public.employee_files USING btree (file_type);


--
-- TOC entry 5220 (class 1259 OID 69439)
-- Name: idx_employee_target_tables_employee_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_employee_target_tables_employee_id ON public.employee_target_tables USING btree (employee_id);


--
-- TOC entry 5221 (class 1259 OID 69440)
-- Name: idx_employee_target_tables_table_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_employee_target_tables_table_id ON public.employee_target_tables USING btree (table_id);


--
-- TOC entry 5209 (class 1259 OID 69396)
-- Name: idx_employees_attachment_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_employees_attachment_id ON public.employees USING btree (attachment_id);


--
-- TOC entry 5283 (class 1259 OID 78518)
-- Name: idx_employees_history_action_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_employees_history_action_type ON public.employees_history USING btree (action_type);


--
-- TOC entry 5284 (class 1259 OID 78519)
-- Name: idx_employees_history_created_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_employees_history_created_at ON public.employees_history USING btree (created_at);


--
-- TOC entry 5285 (class 1259 OID 78516)
-- Name: idx_employees_history_employee_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_employees_history_employee_id ON public.employees_history USING btree (employee_id);


--
-- TOC entry 5286 (class 1259 OID 78526)
-- Name: idx_employees_history_table_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_employees_history_table_id ON public.employees_history USING btree (table_id);


--
-- TOC entry 5287 (class 1259 OID 78517)
-- Name: idx_employees_history_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_employees_history_user_id ON public.employees_history USING btree (user_id);


--
-- TOC entry 5288 (class 1259 OID 78520)
-- Name: idx_employees_history_user_id_null; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_employees_history_user_id_null ON public.employees_history USING btree (user_id) WHERE (user_id IS NULL);


--
-- TOC entry 5210 (class 1259 OID 69398)
-- Name: idx_employees_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_employees_name ON public.employees USING btree (last_name, first_name, middle_name);


--
-- TOC entry 5211 (class 1259 OID 69397)
-- Name: idx_employees_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_employees_status ON public.employees USING btree (status);


--
-- TOC entry 5243 (class 1259 OID 86705)
-- Name: idx_feedback_created_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_feedback_created_at ON public.feedback USING btree (created_at DESC);


--
-- TOC entry 5244 (class 1259 OID 69848)
-- Name: idx_feedback_is_read; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_feedback_is_read ON public.feedback USING btree (is_read);


--
-- TOC entry 5239 (class 1259 OID 69792)
-- Name: idx_feedback_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_feedback_status ON public.feedback_messages USING btree (status);


--
-- TOC entry 5240 (class 1259 OID 69791)
-- Name: idx_feedback_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_feedback_user_id ON public.feedback_messages USING btree (user_id);


--
-- TOC entry 5102 (class 1259 OID 68725)
-- Name: idx_format_cells_format_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_format_cells_format_id ON public.license_plate_format_cells USING btree (format_id);


--
-- TOC entry 5103 (class 1259 OID 68726)
-- Name: idx_format_cells_order; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_format_cells_order ON public.license_plate_format_cells USING btree (format_id, cell_order);


--
-- TOC entry 5212 (class 1259 OID 69416)
-- Name: idx_items_attachment_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_items_attachment_id ON public.items USING btree (attachment_id);


--
-- TOC entry 5213 (class 1259 OID 69417)
-- Name: idx_items_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_items_name ON public.items USING btree (name);


--
-- TOC entry 5293 (class 1259 OID 86745)
-- Name: idx_news_created_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_news_created_at ON public.news USING btree (created_at DESC);


--
-- TOC entry 5294 (class 1259 OID 86747)
-- Name: idx_news_created_by; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_news_created_by ON public.news USING btree (created_by);


--
-- TOC entry 5295 (class 1259 OID 86746)
-- Name: idx_news_is_active; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_news_is_active ON public.news USING btree (is_active);


--
-- TOC entry 5289 (class 1259 OID 86701)
-- Name: idx_notifications_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_notifications_user_id ON public.notifications USING btree (user_id);


--
-- TOC entry 5290 (class 1259 OID 86702)
-- Name: idx_notifications_user_unread; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_notifications_user_unread ON public.notifications USING btree (user_id, is_read);


--
-- TOC entry 5080 (class 1259 OID 78055)
-- Name: idx_org_users_required_approval; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_org_users_required_approval ON public.organization_users USING btree (required_approval);


--
-- TOC entry 5125 (class 1259 OID 68932)
-- Name: idx_organization_tables_org_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_organization_tables_org_id ON public.organization_tables USING btree (organization_id);


--
-- TOC entry 5126 (class 1259 OID 68933)
-- Name: idx_organization_tables_table_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_organization_tables_table_id ON public.organization_tables USING btree (table_id);


--
-- TOC entry 5074 (class 1259 OID 68630)
-- Name: idx_organization_unload_places_org_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_organization_unload_places_org_id ON public.organization_unload_places USING btree (organization_id);


--
-- TOC entry 5075 (class 1259 OID 68631)
-- Name: idx_organization_unload_places_place_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_organization_unload_places_place_id ON public.organization_unload_places USING btree (unload_place_id);


--
-- TOC entry 5081 (class 1259 OID 68652)
-- Name: idx_organization_users_org_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_organization_users_org_id ON public.organization_users USING btree (organization_id);


--
-- TOC entry 5082 (class 1259 OID 68653)
-- Name: idx_organization_users_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_organization_users_user_id ON public.organization_users USING btree (user_id);


--
-- TOC entry 5160 (class 1259 OID 69119)
-- Name: idx_refresh_tokens_expires_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_refresh_tokens_expires_at ON public.refresh_tokens USING btree (expires_at);


--
-- TOC entry 5161 (class 1259 OID 69118)
-- Name: idx_refresh_tokens_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_refresh_tokens_user_id ON public.refresh_tokens USING btree (user_id);


--
-- TOC entry 5230 (class 1259 OID 69667)
-- Name: idx_request_log_method; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_request_log_method ON public.request_log USING btree (method);


--
-- TOC entry 5231 (class 1259 OID 69666)
-- Name: idx_request_log_path; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_request_log_path ON public.request_log USING btree (path);


--
-- TOC entry 5232 (class 1259 OID 69668)
-- Name: idx_request_log_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_request_log_status ON public.request_log USING btree (response_status);


--
-- TOC entry 5233 (class 1259 OID 69664)
-- Name: idx_request_log_timestamp; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_request_log_timestamp ON public.request_log USING btree ("timestamp" DESC);


--
-- TOC entry 5234 (class 1259 OID 69665)
-- Name: idx_request_log_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_request_log_user_id ON public.request_log USING btree (user_id);


--
-- TOC entry 5304 (class 1259 OID 86795)
-- Name: idx_request_logs_created_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_request_logs_created_at ON public.request_logs USING btree (created_at);


--
-- TOC entry 5305 (class 1259 OID 86796)
-- Name: idx_request_logs_method; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_request_logs_method ON public.request_logs USING btree (method);


--
-- TOC entry 5306 (class 1259 OID 86797)
-- Name: idx_request_logs_response_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_request_logs_response_status ON public.request_logs USING btree (response_status);


--
-- TOC entry 5307 (class 1259 OID 86794)
-- Name: idx_request_logs_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_request_logs_user_id ON public.request_logs USING btree (user_id);


--
-- TOC entry 5270 (class 1259 OID 78400)
-- Name: idx_system_table_photos_table_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_system_table_photos_table_id ON public.system_table_photos USING btree (table_id);


--
-- TOC entry 5267 (class 1259 OID 78378)
-- Name: idx_system_table_time_slots_table_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_system_table_time_slots_table_id ON public.system_table_time_slots USING btree (table_id);


--
-- TOC entry 5115 (class 1259 OID 68907)
-- Name: idx_system_tables_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_system_tables_name ON public.system_tables USING btree (name);


--
-- TOC entry 5116 (class 1259 OID 68908)
-- Name: idx_system_tables_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_system_tables_type ON public.system_tables USING btree (table_type);


--
-- TOC entry 5121 (class 1259 OID 68910)
-- Name: idx_table_fields_order; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_table_fields_order ON public.table_fields USING btree (display_order);


--
-- TOC entry 5122 (class 1259 OID 68909)
-- Name: idx_table_fields_table_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_table_fields_table_id ON public.table_fields USING btree (table_id);


--
-- TOC entry 5171 (class 1259 OID 69170)
-- Name: idx_unique_attachments_active; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_unique_attachments_active ON public.unique_attachments USING btree (is_active);


--
-- TOC entry 5172 (class 1259 OID 69169)
-- Name: idx_unique_attachments_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_unique_attachments_type ON public.unique_attachments USING btree (attachment_type);


--
-- TOC entry 5106 (class 1259 OID 68872)
-- Name: idx_unique_cars_company_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_unique_cars_company_id ON public.unique_cars USING btree (company_id);


--
-- TOC entry 5107 (class 1259 OID 68870)
-- Name: idx_unique_cars_company_number_mark; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX idx_unique_cars_company_number_mark ON public.unique_cars USING btree (company_id, number, mark) WHERE (company_id IS NOT NULL);


--
-- TOC entry 5108 (class 1259 OID 68875)
-- Name: idx_unique_cars_mark; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_unique_cars_mark ON public.unique_cars USING btree (mark);


--
-- TOC entry 5109 (class 1259 OID 68874)
-- Name: idx_unique_cars_number; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_unique_cars_number ON public.unique_cars USING btree (number);


--
-- TOC entry 5110 (class 1259 OID 68869)
-- Name: idx_unique_cars_org_number_mark; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX idx_unique_cars_org_number_mark ON public.unique_cars USING btree (organization_id, number, mark) WHERE (organization_id IS NOT NULL);


--
-- TOC entry 5111 (class 1259 OID 68871)
-- Name: idx_unique_cars_organization_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_unique_cars_organization_id ON public.unique_cars USING btree (organization_id);


--
-- TOC entry 5112 (class 1259 OID 68873)
-- Name: idx_unique_cars_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_unique_cars_user_id ON public.unique_cars USING btree (user_id);


--
-- TOC entry 5142 (class 1259 OID 69008)
-- Name: idx_unique_employees_citizenship_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_unique_employees_citizenship_id ON public.unique_employees USING btree (citizenship_id);


--
-- TOC entry 5143 (class 1259 OID 69006)
-- Name: idx_unique_employees_company_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_unique_employees_company_id ON public.unique_employees USING btree (company_id);


--
-- TOC entry 5144 (class 1259 OID 69012)
-- Name: idx_unique_employees_company_passport; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX idx_unique_employees_company_passport ON public.unique_employees USING btree (company_id, passport_series_number) WHERE ((company_id IS NOT NULL) AND (passport_series_number IS NOT NULL));


--
-- TOC entry 5145 (class 1259 OID 69010)
-- Name: idx_unique_employees_first_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_unique_employees_first_name ON public.unique_employees USING btree (first_name);


--
-- TOC entry 5146 (class 1259 OID 69009)
-- Name: idx_unique_employees_last_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_unique_employees_last_name ON public.unique_employees USING btree (last_name);


--
-- TOC entry 5147 (class 1259 OID 69011)
-- Name: idx_unique_employees_org_passport; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX idx_unique_employees_org_passport ON public.unique_employees USING btree (organization_id, passport_series_number) WHERE ((organization_id IS NOT NULL) AND (passport_series_number IS NOT NULL));


--
-- TOC entry 5148 (class 1259 OID 69005)
-- Name: idx_unique_employees_organization_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_unique_employees_organization_id ON public.unique_employees USING btree (organization_id);


--
-- TOC entry 5149 (class 1259 OID 69007)
-- Name: idx_unique_employees_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_unique_employees_user_id ON public.unique_employees USING btree (user_id);


--
-- TOC entry 5150 (class 1259 OID 69013)
-- Name: idx_unique_employees_user_passport; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX idx_unique_employees_user_passport ON public.unique_employees USING btree (user_id, passport_series_number) WHERE ((user_id IS NOT NULL) AND (passport_series_number IS NOT NULL));


--
-- TOC entry 5261 (class 1259 OID 78334)
-- Name: idx_unload_place_photos_place_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_unload_place_photos_place_id ON public.unload_place_photos USING btree (unload_place_id);


--
-- TOC entry 5264 (class 1259 OID 78356)
-- Name: idx_unload_place_time_slots_place_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_unload_place_time_slots_place_id ON public.unload_place_time_slots USING btree (unload_place_id);


--
-- TOC entry 5387 (class 2620 OID 69249)
-- Name: attachments update_attachments_updated_at; Type: TRIGGER; Schema: public; Owner: postgres
--

CREATE TRIGGER update_attachments_updated_at BEFORE UPDATE ON public.attachments FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- TOC entry 5388 (class 2620 OID 69376)
-- Name: cars update_cars_updated_at; Type: TRIGGER; Schema: public; Owner: postgres
--

CREATE TRIGGER update_cars_updated_at BEFORE UPDATE ON public.cars FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- TOC entry 5385 (class 2620 OID 68972)
-- Name: citizenships update_citizenships_updated_at; Type: TRIGGER; Schema: public; Owner: postgres
--

CREATE TRIGGER update_citizenships_updated_at BEFORE UPDATE ON public.citizenships FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- TOC entry 5389 (class 2620 OID 69399)
-- Name: employees update_employees_updated_at; Type: TRIGGER; Schema: public; Owner: postgres
--

CREATE TRIGGER update_employees_updated_at BEFORE UPDATE ON public.employees FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- TOC entry 5390 (class 2620 OID 69418)
-- Name: items update_items_updated_at; Type: TRIGGER; Schema: public; Owner: postgres
--

CREATE TRIGGER update_items_updated_at BEFORE UPDATE ON public.items FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- TOC entry 5384 (class 2620 OID 68911)
-- Name: system_tables update_system_tables_updated_at; Type: TRIGGER; Schema: public; Owner: postgres
--

CREATE TRIGGER update_system_tables_updated_at BEFORE UPDATE ON public.system_tables FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- TOC entry 5386 (class 2620 OID 69171)
-- Name: unique_attachments update_unique_attachments_updated_at; Type: TRIGGER; Schema: public; Owner: postgres
--

CREATE TRIGGER update_unique_attachments_updated_at BEFORE UPDATE ON public.unique_attachments FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- TOC entry 5380 (class 2606 OID 86770)
-- Name: announcements announcements_activated_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.announcements
    ADD CONSTRAINT announcements_activated_by_fkey FOREIGN KEY (activated_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5381 (class 2606 OID 86760)
-- Name: announcements announcements_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.announcements
    ADD CONSTRAINT announcements_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- TOC entry 5382 (class 2606 OID 86765)
-- Name: announcements announcements_updated_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.announcements
    ADD CONSTRAINT announcements_updated_by_fkey FOREIGN KEY (updated_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5357 (class 2606 OID 78072)
-- Name: application_approvers application_approvers_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_approvers
    ADD CONSTRAINT application_approvers_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5358 (class 2606 OID 78067)
-- Name: application_approvers application_approvers_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_approvers
    ADD CONSTRAINT application_approvers_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- TOC entry 5337 (class 2606 OID 69046)
-- Name: application_employees application_employees_citizenship_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_employees
    ADD CONSTRAINT application_employees_citizenship_id_fkey FOREIGN KEY (citizenship_id) REFERENCES public.citizenships(id);


--
-- TOC entry 5359 (class 2606 OID 78230)
-- Name: application_history application_history_action_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_history
    ADD CONSTRAINT application_history_action_user_id_fkey FOREIGN KEY (action_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5360 (class 2606 OID 78090)
-- Name: application_history application_history_application_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_history
    ADD CONSTRAINT application_history_application_id_fkey FOREIGN KEY (application_id) REFERENCES public.applications(id) ON DELETE CASCADE;


--
-- TOC entry 5361 (class 2606 OID 78095)
-- Name: application_history application_history_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_history
    ADD CONSTRAINT application_history_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- TOC entry 5350 (class 2606 OID 69542)
-- Name: application_responsible_users application_responsible_users_application_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_responsible_users
    ADD CONSTRAINT application_responsible_users_application_id_fkey FOREIGN KEY (application_id) REFERENCES public.applications(id) ON DELETE CASCADE;


--
-- TOC entry 5351 (class 2606 OID 78140)
-- Name: application_responsible_users application_responsible_users_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_responsible_users
    ADD CONSTRAINT application_responsible_users_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5352 (class 2606 OID 69547)
-- Name: application_responsible_users application_responsible_users_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_responsible_users
    ADD CONSTRAINT application_responsible_users_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- TOC entry 5340 (class 2606 OID 69267)
-- Name: application_status_history application_status_history_changed_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_status_history
    ADD CONSTRAINT application_status_history_changed_by_user_id_fkey FOREIGN KEY (changed_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5362 (class 2606 OID 78278)
-- Name: application_viewers application_viewers_application_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_viewers
    ADD CONSTRAINT application_viewers_application_id_fkey FOREIGN KEY (application_id) REFERENCES public.applications(id) ON DELETE CASCADE;


--
-- TOC entry 5363 (class 2606 OID 78288)
-- Name: application_viewers application_viewers_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_viewers
    ADD CONSTRAINT application_viewers_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5364 (class 2606 OID 78283)
-- Name: application_viewers application_viewers_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.application_viewers
    ADD CONSTRAINT application_viewers_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- TOC entry 5341 (class 2606 OID 69352)
-- Name: applications applications_company_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.applications
    ADD CONSTRAINT applications_company_id_fkey FOREIGN KEY (company_id) REFERENCES public.companies(id) ON DELETE SET NULL;


--
-- TOC entry 5342 (class 2606 OID 69331)
-- Name: applications applications_organization_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.applications
    ADD CONSTRAINT applications_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- TOC entry 5343 (class 2606 OID 69341)
-- Name: applications applications_responsible_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.applications
    ADD CONSTRAINT applications_responsible_user_id_fkey FOREIGN KEY (responsible_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5344 (class 2606 OID 69336)
-- Name: applications applications_sender_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.applications
    ADD CONSTRAINT applications_sender_user_id_fkey FOREIGN KEY (sender_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- TOC entry 5339 (class 2606 OID 69556)
-- Name: attachments attachments_unique_attachment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.attachments
    ADD CONSTRAINT attachments_unique_attachment_id_fkey FOREIGN KEY (unique_attachment_id) REFERENCES public.unique_attachments(id);


--
-- TOC entry 5313 (class 2606 OID 60446)
-- Name: car_unload_places car_unload_places_unload_place_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.car_unload_places
    ADD CONSTRAINT car_unload_places_unload_place_id_fkey FOREIGN KEY (unload_place_id) REFERENCES public.unload_places(id);


--
-- TOC entry 5345 (class 2606 OID 69369)
-- Name: cars cars_attachment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.cars
    ADD CONSTRAINT cars_attachment_id_fkey FOREIGN KEY (attachment_id) REFERENCES public.attachments(id) ON DELETE CASCADE;


--
-- TOC entry 5371 (class 2606 OID 78472)
-- Name: cars_history cars_history_car_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.cars_history
    ADD CONSTRAINT cars_history_car_id_fkey FOREIGN KEY (car_id) REFERENCES public.cars(id) ON DELETE CASCADE;


--
-- TOC entry 5372 (class 2606 OID 78527)
-- Name: cars_history cars_history_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.cars_history
    ADD CONSTRAINT cars_history_table_id_fkey FOREIGN KEY (table_id) REFERENCES public.system_tables(id) ON DELETE SET NULL;


--
-- TOC entry 5373 (class 2606 OID 78477)
-- Name: cars_history cars_history_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.cars_history
    ADD CONSTRAINT cars_history_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- TOC entry 5330 (class 2606 OID 68944)
-- Name: companies_tables companies_tables_company_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies_tables
    ADD CONSTRAINT companies_tables_company_id_fkey FOREIGN KEY (company_id) REFERENCES public.companies(id) ON DELETE CASCADE;


--
-- TOC entry 5331 (class 2606 OID 68949)
-- Name: companies_tables companies_tables_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies_tables
    ADD CONSTRAINT companies_tables_table_id_fkey FOREIGN KEY (table_id) REFERENCES public.system_tables(id) ON DELETE CASCADE;


--
-- TOC entry 5318 (class 2606 OID 68664)
-- Name: companies_unload_places companies_unload_places_company_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies_unload_places
    ADD CONSTRAINT companies_unload_places_company_id_fkey FOREIGN KEY (company_id) REFERENCES public.companies(id) ON DELETE CASCADE;


--
-- TOC entry 5319 (class 2606 OID 68669)
-- Name: companies_unload_places companies_unload_places_unload_place_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies_unload_places
    ADD CONSTRAINT companies_unload_places_unload_place_id_fkey FOREIGN KEY (unload_place_id) REFERENCES public.unload_places(id) ON DELETE CASCADE;


--
-- TOC entry 5320 (class 2606 OID 68686)
-- Name: companies_users companies_users_company_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies_users
    ADD CONSTRAINT companies_users_company_id_fkey FOREIGN KEY (company_id) REFERENCES public.companies(id) ON DELETE CASCADE;


--
-- TOC entry 5321 (class 2606 OID 68691)
-- Name: companies_users companies_users_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.companies_users
    ADD CONSTRAINT companies_users_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- TOC entry 5336 (class 2606 OID 69024)
-- Name: employee_files employee_files_employee_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employee_files
    ADD CONSTRAINT employee_files_employee_id_fkey FOREIGN KEY (employee_id) REFERENCES public.unique_employees(id) ON DELETE CASCADE;


--
-- TOC entry 5348 (class 2606 OID 69429)
-- Name: employee_target_tables employee_target_tables_employee_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employee_target_tables
    ADD CONSTRAINT employee_target_tables_employee_id_fkey FOREIGN KEY (employee_id) REFERENCES public.employees(id) ON DELETE CASCADE;


--
-- TOC entry 5349 (class 2606 OID 69434)
-- Name: employee_target_tables employee_target_tables_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employee_target_tables
    ADD CONSTRAINT employee_target_tables_table_id_fkey FOREIGN KEY (table_id) REFERENCES public.system_tables(id) ON DELETE CASCADE;


--
-- TOC entry 5346 (class 2606 OID 69391)
-- Name: employees employees_attachment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employees
    ADD CONSTRAINT employees_attachment_id_fkey FOREIGN KEY (attachment_id) REFERENCES public.attachments(id) ON DELETE CASCADE;


--
-- TOC entry 5374 (class 2606 OID 78506)
-- Name: employees_history employees_history_employee_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employees_history
    ADD CONSTRAINT employees_history_employee_id_fkey FOREIGN KEY (employee_id) REFERENCES public.employees(id) ON DELETE CASCADE;


--
-- TOC entry 5375 (class 2606 OID 78521)
-- Name: employees_history employees_history_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employees_history
    ADD CONSTRAINT employees_history_table_id_fkey FOREIGN KEY (table_id) REFERENCES public.system_tables(id) ON DELETE SET NULL;


--
-- TOC entry 5376 (class 2606 OID 78511)
-- Name: employees_history employees_history_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.employees_history
    ADD CONSTRAINT employees_history_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5354 (class 2606 OID 69786)
-- Name: feedback_messages feedback_messages_resolved_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.feedback_messages
    ADD CONSTRAINT feedback_messages_resolved_by_fkey FOREIGN KEY (resolved_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5355 (class 2606 OID 69781)
-- Name: feedback_messages feedback_messages_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.feedback_messages
    ADD CONSTRAINT feedback_messages_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- TOC entry 5356 (class 2606 OID 69843)
-- Name: feedback feedback_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.feedback
    ADD CONSTRAINT feedback_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- TOC entry 5347 (class 2606 OID 69411)
-- Name: items items_attachment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.items
    ADD CONSTRAINT items_attachment_id_fkey FOREIGN KEY (attachment_id) REFERENCES public.attachments(id) ON DELETE CASCADE;


--
-- TOC entry 5322 (class 2606 OID 68720)
-- Name: license_plate_format_cells license_plate_format_cells_format_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.license_plate_format_cells
    ADD CONSTRAINT license_plate_format_cells_format_id_fkey FOREIGN KEY (format_id) REFERENCES public.license_plate_formats(id) ON DELETE CASCADE;


--
-- TOC entry 5378 (class 2606 OID 86735)
-- Name: news news_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.news
    ADD CONSTRAINT news_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- TOC entry 5379 (class 2606 OID 86740)
-- Name: news news_updated_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.news
    ADD CONSTRAINT news_updated_by_fkey FOREIGN KEY (updated_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5377 (class 2606 OID 86696)
-- Name: notifications notifications_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- TOC entry 5328 (class 2606 OID 68922)
-- Name: organization_tables organization_tables_organization_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization_tables
    ADD CONSTRAINT organization_tables_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- TOC entry 5329 (class 2606 OID 68927)
-- Name: organization_tables organization_tables_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization_tables
    ADD CONSTRAINT organization_tables_table_id_fkey FOREIGN KEY (table_id) REFERENCES public.system_tables(id) ON DELETE CASCADE;


--
-- TOC entry 5314 (class 2606 OID 68620)
-- Name: organization_unload_places organization_unload_places_organization_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization_unload_places
    ADD CONSTRAINT organization_unload_places_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- TOC entry 5315 (class 2606 OID 68625)
-- Name: organization_unload_places organization_unload_places_unload_place_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization_unload_places
    ADD CONSTRAINT organization_unload_places_unload_place_id_fkey FOREIGN KEY (unload_place_id) REFERENCES public.unload_places(id) ON DELETE CASCADE;


--
-- TOC entry 5316 (class 2606 OID 68642)
-- Name: organization_users organization_users_organization_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization_users
    ADD CONSTRAINT organization_users_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- TOC entry 5317 (class 2606 OID 68647)
-- Name: organization_users organization_users_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization_users
    ADD CONSTRAINT organization_users_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- TOC entry 5338 (class 2606 OID 69113)
-- Name: refresh_tokens refresh_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- TOC entry 5353 (class 2606 OID 69659)
-- Name: request_log request_log_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.request_log
    ADD CONSTRAINT request_log_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5383 (class 2606 OID 86789)
-- Name: request_logs request_logs_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.request_logs
    ADD CONSTRAINT request_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5369 (class 2606 OID 78390)
-- Name: system_table_photos system_table_photos_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.system_table_photos
    ADD CONSTRAINT system_table_photos_table_id_fkey FOREIGN KEY (table_id) REFERENCES public.system_tables(id) ON DELETE CASCADE;


--
-- TOC entry 5370 (class 2606 OID 78395)
-- Name: system_table_photos system_table_photos_uploaded_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.system_table_photos
    ADD CONSTRAINT system_table_photos_uploaded_by_fkey FOREIGN KEY (uploaded_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5368 (class 2606 OID 78373)
-- Name: system_table_time_slots system_table_time_slots_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.system_table_time_slots
    ADD CONSTRAINT system_table_time_slots_table_id_fkey FOREIGN KEY (table_id) REFERENCES public.system_tables(id) ON DELETE CASCADE;


--
-- TOC entry 5327 (class 2606 OID 68902)
-- Name: table_fields table_fields_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.table_fields
    ADD CONSTRAINT table_fields_table_id_fkey FOREIGN KEY (table_id) REFERENCES public.system_tables(id) ON DELETE CASCADE;


--
-- TOC entry 5323 (class 2606 OID 68854)
-- Name: unique_cars unique_cars_company_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unique_cars
    ADD CONSTRAINT unique_cars_company_id_fkey FOREIGN KEY (company_id) REFERENCES public.companies(id) ON DELETE SET NULL;


--
-- TOC entry 5324 (class 2606 OID 68859)
-- Name: unique_cars unique_cars_format_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unique_cars
    ADD CONSTRAINT unique_cars_format_id_fkey FOREIGN KEY (format_id) REFERENCES public.license_plate_formats(id) ON DELETE SET NULL;


--
-- TOC entry 5325 (class 2606 OID 68849)
-- Name: unique_cars unique_cars_organization_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unique_cars
    ADD CONSTRAINT unique_cars_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE SET NULL;


--
-- TOC entry 5326 (class 2606 OID 68864)
-- Name: unique_cars unique_cars_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unique_cars
    ADD CONSTRAINT unique_cars_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5332 (class 2606 OID 68985)
-- Name: unique_employees unique_employees_citizenship_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unique_employees
    ADD CONSTRAINT unique_employees_citizenship_id_fkey FOREIGN KEY (citizenship_id) REFERENCES public.citizenships(id) ON DELETE SET NULL;


--
-- TOC entry 5333 (class 2606 OID 68995)
-- Name: unique_employees unique_employees_company_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unique_employees
    ADD CONSTRAINT unique_employees_company_id_fkey FOREIGN KEY (company_id) REFERENCES public.companies(id) ON DELETE SET NULL;


--
-- TOC entry 5334 (class 2606 OID 68990)
-- Name: unique_employees unique_employees_organization_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unique_employees
    ADD CONSTRAINT unique_employees_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE SET NULL;


--
-- TOC entry 5335 (class 2606 OID 69000)
-- Name: unique_employees unique_employees_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unique_employees
    ADD CONSTRAINT unique_employees_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5365 (class 2606 OID 78324)
-- Name: unload_place_photos unload_place_photos_unload_place_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unload_place_photos
    ADD CONSTRAINT unload_place_photos_unload_place_id_fkey FOREIGN KEY (unload_place_id) REFERENCES public.unload_places(id) ON DELETE CASCADE;


--
-- TOC entry 5366 (class 2606 OID 78329)
-- Name: unload_place_photos unload_place_photos_uploaded_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unload_place_photos
    ADD CONSTRAINT unload_place_photos_uploaded_by_fkey FOREIGN KEY (uploaded_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- TOC entry 5367 (class 2606 OID 78351)
-- Name: unload_place_time_slots unload_place_time_slots_unload_place_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.unload_place_time_slots
    ADD CONSTRAINT unload_place_time_slots_unload_place_id_fkey FOREIGN KEY (unload_place_id) REFERENCES public.unload_places(id) ON DELETE CASCADE;


--
-- TOC entry 5310 (class 2606 OID 60347)
-- Name: users users_company_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_company_id_fkey FOREIGN KEY (company_id) REFERENCES public.companies(id);


--
-- TOC entry 5311 (class 2606 OID 60342)
-- Name: users users_organization_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES public.organizations(id);


--
-- TOC entry 5312 (class 2606 OID 60384)
-- Name: users users_type_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_type_id_fkey FOREIGN KEY (type_id) REFERENCES public.user_types(id);


-- Completed on 2026-03-28 21:33:33

--
-- PostgreSQL database dump complete
--


-- Seed: test organization and company for FK in users
INSERT INTO organizations (name) VALUES ('Тестовая организация');
INSERT INTO companies (name) VALUES ('Тестовая компания');
