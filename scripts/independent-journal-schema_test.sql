\set ON_ERROR_STOP on
BEGIN;
INSERT INTO restore_journal.installations (installation, writer_login) VALUES ('synthetic', session_user);

SET LOCAL ROLE restore_journal_writer;
DO $$
DECLARE
    payload bytea := decode(repeat('34', 28), 'hex');
BEGIN
    IF restore_journal.append_event('synthetic', 1, 1, 'a', 'intent', payload) <> 1
        OR restore_journal.append_event('synthetic', 1, 1, 'a', 'intent', payload) <> 1 THEN
        RAISE EXCEPTION 'append or exact retry failed';
    END IF;
    BEGIN
        PERFORM restore_journal.append_event('synthetic', 1, 1, 'a', 'intent', payload || payload);
        RAISE EXCEPTION 'changed retry accepted';
    EXCEPTION WHEN unique_violation THEN NULL;
    END;
    BEGIN
        PERFORM restore_journal.append_event('synthetic', 1, 2, 'b', 'intent', payload);
        RAISE EXCEPTION 'competing intent accepted';
    EXCEPTION WHEN object_not_in_prerequisite_state THEN NULL;
    END;
    BEGIN
        PERFORM restore_journal.append_event('synthetic', 1, 3, 'a', 'completed', payload);
        RAISE EXCEPTION 'sequence gap accepted';
    EXCEPTION WHEN serialization_failure THEN NULL;
    END;
    IF restore_journal.append_event('synthetic', 1, 2, 'a', 'completed', payload) <> 2 THEN
        RAISE EXCEPTION 'completion failed';
    END IF;
    BEGIN
        DELETE FROM restore_journal.events;
        RAISE EXCEPTION 'writer can delete';
    EXCEPTION WHEN insufficient_privilege THEN NULL;
    END;
    BEGIN
        UPDATE restore_journal.events SET phase = 'cancelled';
        RAISE EXCEPTION 'writer can replace';
    EXCEPTION WHEN insufficient_privilege THEN NULL;
    END;
    BEGIN
        PERFORM restore_journal.fence_writer('synthetic');
        RAISE EXCEPTION 'writer can fence';
    EXCEPTION WHEN insufficient_privilege THEN NULL;
    END;
END;
$$;

SET LOCAL ROLE restore_journal_recovery;
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM restore_journal.fence_writer('synthetic') WHERE generation = 2 AND sequence = 2)
        OR NOT EXISTS (SELECT FROM restore_journal.fence_writer('synthetic') WHERE generation = 2 AND sequence = 2)
        OR (SELECT count(*) FROM restore_journal.events WHERE installation = 'synthetic') <> 2 THEN
        RAISE EXCEPTION 'fenced head or event stream incorrect';
    END IF;
END;
$$;

SET LOCAL ROLE restore_journal_writer;
DO $$
BEGIN
    BEGIN
        PERFORM restore_journal.append_event('synthetic', 1, 3, 'b', 'intent', decode(repeat('34', 28), 'hex'));
        RAISE EXCEPTION 'fenced writer can append';
    EXCEPTION WHEN object_not_in_prerequisite_state THEN NULL;
    END;
END;
$$;
RESET ROLE;
CREATE ROLE restore_journal_test_next LOGIN;
GRANT restore_journal_writer TO restore_journal_test_next;
DO $$
BEGIN
    IF restore_journal.replace_writer('synthetic', 2, 1, 'restore_journal_test_next')
        OR NOT restore_journal.replace_writer('synthetic', 2, 2, 'restore_journal_test_next') THEN
        RAISE EXCEPTION 'incorrect writer replacement';
    END IF;
END;
$$;
SET LOCAL ROLE restore_journal_writer;
DO $$
BEGIN
    BEGIN
        PERFORM restore_journal.append_event('synthetic', 2, 3, 'b', 'intent', decode(repeat('34', 28), 'hex'));
        RAISE EXCEPTION 'previous writer accepted after replacement';
    EXCEPTION WHEN object_not_in_prerequisite_state THEN NULL;
    END;
END;
$$;
RESET ROLE;
SET LOCAL SESSION AUTHORIZATION restore_journal_test_next;
SELECT restore_journal.append_event('synthetic', 2, 3, 'b', 'intent', decode(repeat('34', 28), 'hex')) AS next_sequence;
ROLLBACK;
