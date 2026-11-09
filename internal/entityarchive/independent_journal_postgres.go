package entityarchive

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type postgresJournal struct {
	conn         *pgx.Conn
	installation string
}

func (j postgresJournal) writerHead(ctx context.Context) (journalHead, error) {
	head := journalHead{Installation: j.installation}
	err := j.conn.QueryRow(ctx, `
		SELECT generation, sequence, fenced, pending FROM restore_journal.writer_head
		WHERE installation = $1`, j.installation).Scan(&head.Generation, &head.Sequence, &head.Fenced, &head.Pending)
	if err != nil {
		return journalHead{}, fmt.Errorf("read destruction journal head: %w", err)
	}
	if head.Fenced {
		return journalHead{}, errors.New("destruction journal writer is fenced")
	}
	return head, nil
}

// append must receive the original sealed bytes when retrying an uncertain write.
func (j postgresJournal) append(ctx context.Context, event journalEvent, record sealedJournalEvent) error {
	var sequence uint64
	err := j.conn.QueryRow(ctx, `
		SELECT restore_journal.append_event($1, $2, $3, $4, $5, $6)`,
		j.installation, record.Generation, record.Sequence, event.OperationID,
		string(event.Phase), record.Ciphertext).Scan(&sequence)
	if err != nil {
		return fmt.Errorf("append destruction journal event: %w", err)
	}
	if sequence != record.Sequence {
		return errors.New("unexpected destruction journal sequence")
	}
	return nil
}

// fenceAndRead commits the fence before reading the stream. A failed read must
// leave the previous writer fenced so the operator can retry the same recovery.
func (j postgresJournal) fenceAndRead(ctx context.Context) (journalHead, []sealedJournalEvent, error) {
	head := journalHead{Installation: j.installation, Fenced: true}
	err := j.conn.QueryRow(ctx, `
		SELECT generation, sequence, pending FROM restore_journal.fence_writer($1)`,
		j.installation).Scan(&head.Generation, &head.Sequence, &head.Pending)
	if err != nil {
		return journalHead{}, nil, fmt.Errorf("fence destruction journal writer: %w", err)
	}

	rows, err := j.conn.Query(ctx, `
		SELECT sequence, generation, ciphertext FROM restore_journal.events
		WHERE installation = $1 AND sequence <= $2 ORDER BY sequence`,
		j.installation, head.Sequence)
	if err != nil {
		return journalHead{}, nil, fmt.Errorf("read fenced destruction journal: %w", err)
	}
	records, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (sealedJournalEvent, error) {
		var record sealedJournalEvent
		err := row.Scan(&record.Sequence, &record.Generation, &record.Ciphertext)
		return record, err
	})
	if err != nil {
		return journalHead{}, nil, fmt.Errorf("read fenced destruction journal: %w", err)
	}
	return head, records, nil
}
