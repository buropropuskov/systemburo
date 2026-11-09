package entityarchive

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"gorm.io/gorm"
	"systemburo/internal/models"
)

const journalTimeout = 5 * time.Second

type destructionJournalWrite struct {
	journal postgresJournal
	cipher  journalCipher
	id      string
	target  journalTarget
	receipt *journalReceipt
}

func beginDestructionJournal(ctx context.Context, opt DestructionOptions, rec models.DestructionRecord) (*destructionJournalWrite, error) {
	if opt.ReplayOf != nil {
		return nil, nil
	}
	dsn := os.Getenv("DESTRUCTION_JOURNAL_WRITER_DSN")
	installation := os.Getenv("DESTRUCTION_JOURNAL_INSTALLATION")
	key := os.Getenv("DESTRUCTION_JOURNAL_KEY")
	if dsn == "" && installation == "" && key == "" {
		return nil, nil
	}
	if dsn == "" || installation == "" || key == "" {
		return nil, errors.New("incomplete destruction journal configuration")
	}
	decoded, err := hex.DecodeString(key)
	if err != nil {
		return nil, errors.New("invalid destruction journal key")
	}
	cipher, err := newJournalCipher(decoded)
	if err != nil {
		return nil, err
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid destruction journal connection configuration")
	}
	config.RuntimeParams["synchronous_commit"] = "on"
	ctx, cancel := context.WithTimeout(ctx, journalTimeout)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return nil, errors.New("connect to destruction journal")
	}
	write := &destructionJournalWrite{
		journal: postgresJournal{conn: conn, installation: installation},
		cipher:  cipher,
		id:      uuid.NewString(),
	}
	write.target = journalTarget{
		EntityType: rec.EntityType, EntityID: rec.EntityID, ApplicationNumber: rec.ApplicationNumber,
		SubjectAnchors: rec.SubjectAnchors, Action: rec.Action, Basis: rec.Basis,
	}
	if err := write.append(ctx, journalEvent{OperationID: write.id, Phase: journalIntent, Target: &write.target}); err != nil {
		write.close()
		return nil, err
	}
	return write, nil
}

func (w *destructionJournalWrite) record(ctx context.Context, tx *gorm.DB, opt DestructionOptions, rec *models.DestructionRecord) error {
	if w != nil && (rec.EntityType != w.target.EntityType || rec.Action != w.target.Action ||
		rec.ApplicationNumber != w.target.ApplicationNumber || rec.SubjectAnchors != w.target.SubjectAnchors) {
		return errors.New("destruction journal target changed")
	}
	if err := writeDestruction(ctx, tx, opt, rec); err != nil {
		return err
	}
	if w != nil {
		w.receipt = &journalReceipt{ID: rec.ID, Rows: rec.Rows, Files: rec.Files,
			ActorID: rec.ActorID, ActorNote: rec.ActorNote, CreatedAt: rec.CreatedAt.Truncate(time.Microsecond)}
	}
	return nil
}

func (w *destructionJournalWrite) append(ctx context.Context, event journalEvent) error {
	head, err := w.journal.writerHead(ctx)
	if err != nil {
		return err
	}
	record, err := w.cipher.seal(head.Installation, head.Generation, head.Sequence+1, event)
	if err != nil {
		return err
	}
	return w.journal.append(ctx, event, record)
}

func (w *destructionJournalWrite) complete(ctx context.Context) error {
	if w == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), journalTimeout)
	defer cancel()
	if w.receipt == nil {
		return errors.New("missing destruction journal receipt")
	}
	if err := w.append(ctx, journalEvent{OperationID: w.id, Phase: journalCompleted, Receipt: w.receipt}); err != nil {
		return fmt.Errorf("destruction finished but journal completion is unconfirmed: %w", err)
	}
	return nil
}

func (w *destructionJournalWrite) close() {
	if w == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), journalTimeout)
	defer cancel()
	_ = w.journal.conn.Close(ctx)
}
