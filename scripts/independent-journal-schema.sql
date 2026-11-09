-- Provision once, in a dedicated database. Assign group membership to separate
-- login roles after provisioning. The application must never own these objects.
BEGIN;

CREATE ROLE restore_journal_writer NOLOGIN;
CREATE ROLE restore_journal_recovery NOLOGIN;
CREATE SCHEMA restore_journal;
REVOKE ALL ON SCHEMA restore_journal FROM PUBLIC;
GRANT USAGE ON SCHEMA restore_journal TO restore_journal_writer, restore_journal_recovery;

CREATE TABLE restore_journal.installations (
    installation text PRIMARY KEY CHECK (installation <> ''),
    writer_login name NOT NULL,
    generation bigint NOT NULL DEFAULT 1 CHECK (generation > 0),
    sequence bigint NOT NULL DEFAULT 0 CHECK (sequence >= 0),
    pending_operation text,
    fenced boolean NOT NULL DEFAULT false
);

CREATE TABLE restore_journal.events (
    installation text NOT NULL REFERENCES restore_journal.installations (installation),
    sequence bigint NOT NULL CHECK (sequence > 0),
    generation bigint NOT NULL CHECK (generation > 0),
    operation_id text NOT NULL CHECK (operation_id <> ''),
    phase text NOT NULL CHECK (phase IN ('intent', 'completed', 'cancelled')),
    ciphertext bytea NOT NULL CHECK (octet_length(ciphertext) >= 28),
    PRIMARY KEY (installation, sequence),
    UNIQUE (installation, operation_id, phase)
);

CREATE VIEW restore_journal.writer_head AS
    SELECT installation, generation, sequence, fenced, pending_operation IS NOT NULL AS pending
    FROM restore_journal.installations
    WHERE writer_login = session_user;

CREATE FUNCTION restore_journal.append_event(
    installation_id text, expected_generation bigint, next_sequence bigint,
    event_operation text, event_phase text, event_ciphertext bytea
) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $$
DECLARE
    head restore_journal.installations%ROWTYPE;
    existing restore_journal.events%ROWTYPE;
BEGIN
    SELECT * INTO STRICT head FROM restore_journal.installations
        WHERE installation = installation_id FOR UPDATE;

    IF head.fenced OR head.writer_login <> session_user OR expected_generation IS DISTINCT FROM head.generation THEN
        RAISE EXCEPTION 'journal writer is fenced' USING ERRCODE = '55000';
    END IF;

    SELECT * INTO existing FROM restore_journal.events
        WHERE installation = installation_id AND operation_id = event_operation AND phase = event_phase;
    IF FOUND THEN
        IF (existing.generation, existing.sequence, existing.ciphertext)
            IS DISTINCT FROM (expected_generation, next_sequence, event_ciphertext) THEN
            RAISE EXCEPTION 'journal retry differs from stored event' USING ERRCODE = '23505';
        END IF;
        RETURN existing.sequence;
    END IF;

    IF next_sequence IS DISTINCT FROM head.sequence + 1 THEN
        RAISE EXCEPTION 'journal head advanced' USING ERRCODE = '40001';
    END IF;

    IF event_phase = 'intent' THEN
        IF head.pending_operation IS NOT NULL THEN
            RAISE EXCEPTION 'journal operation still pending' USING ERRCODE = '55000';
        END IF;
        head.pending_operation := event_operation;
    ELSE
        IF head.pending_operation IS NULL OR head.pending_operation IS DISTINCT FROM event_operation THEN
            RAISE EXCEPTION 'journal operation does not match pending intent' USING ERRCODE = '55000';
        END IF;
        head.pending_operation := NULL;
    END IF;

    INSERT INTO restore_journal.events VALUES (
        installation_id, next_sequence, expected_generation,
        event_operation, event_phase, event_ciphertext
    );
    UPDATE restore_journal.installations
        SET sequence = next_sequence, pending_operation = head.pending_operation
        WHERE installation = installation_id;
    RETURN next_sequence;
END;
$$;

CREATE FUNCTION restore_journal.fence_writer(installation_id text)
RETURNS TABLE (generation bigint, sequence bigint, pending boolean)
LANGUAGE sql SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $$
    UPDATE restore_journal.installations AS head
    SET fenced = true, generation = head.generation + CASE WHEN head.fenced THEN 0 ELSE 1 END
    WHERE head.installation = installation_id
    RETURNING head.generation, head.sequence, head.pending_operation IS NOT NULL;
$$;

-- Called by the database owner after the restored installation has been checked.
-- Use a newly provisioned login, never recycle credentials from a prior writer.
CREATE FUNCTION restore_journal.replace_writer(
    installation_id text, fenced_generation bigint, fenced_sequence bigint, new_login name
) RETURNS boolean
LANGUAGE sql
SET search_path = pg_catalog, pg_temp
AS $$
    WITH changed AS (
        UPDATE restore_journal.installations AS head
        SET writer_login = new_login, fenced = false
        WHERE head.installation = installation_id AND head.fenced AND head.pending_operation IS NULL
          AND head.generation = fenced_generation AND head.sequence = fenced_sequence
          AND head.writer_login <> new_login
          AND EXISTS (SELECT FROM pg_roles WHERE rolname = new_login AND rolcanlogin
                      AND pg_has_role(oid, 'restore_journal_writer', 'USAGE'))
        RETURNING 1
    )
    SELECT EXISTS (SELECT FROM changed);
$$;

REVOKE ALL ON FUNCTION restore_journal.append_event(text, bigint, bigint, text, text, bytea) FROM PUBLIC;
REVOKE ALL ON FUNCTION restore_journal.fence_writer(text) FROM PUBLIC;
REVOKE ALL ON FUNCTION restore_journal.replace_writer(text, bigint, bigint, name) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION restore_journal.append_event(text, bigint, bigint, text, text, bytea)
    TO restore_journal_writer;
GRANT SELECT ON restore_journal.writer_head TO restore_journal_writer;
GRANT EXECUTE ON FUNCTION restore_journal.fence_writer(text) TO restore_journal_recovery;
GRANT SELECT ON restore_journal.events TO restore_journal_recovery;

COMMIT;
