package entityarchive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ExportIndependentRestoreJournal emits the same columns as the live database
// export. Only operations committed after the authenticated backup are needed.
func ExportIndependentRestoreJournal(ctx context.Context, args []string, out io.Writer) error {
	if len(args) != 4 {
		return errors.New("expected checkpoint and three backup component digests")
	}
	checkpoint, err := hex.DecodeString(strings.TrimSpace(args[0]))
	if err != nil {
		return errors.New("invalid backup checkpoint")
	}
	var digests journalBackupDigests
	for i, dest := range []*[sha256.Size]byte{&digests.Database, &digests.Uploads, &digests.Archive} {
		n, err := hex.Decode(dest[:], []byte(args[i+1]))
		if err != nil || n != sha256.Size {
			return errors.New("invalid backup component digest")
		}
	}
	key, err := hex.DecodeString(os.Getenv("DESTRUCTION_JOURNAL_KEY"))
	if err != nil {
		return errors.New("invalid destruction journal key")
	}
	cipher, err := newJournalCipher(key)
	if err != nil {
		return err
	}
	installation := os.Getenv("DESTRUCTION_JOURNAL_INSTALLATION")
	dsn := os.Getenv("DESTRUCTION_JOURNAL_RECOVERY_DSN")
	if installation == "" || dsn == "" {
		return errors.New("incomplete destruction journal recovery configuration")
	}
	if _, err := cipher.openBackupCheckpoint(checkpoint, digests, installation); err != nil {
		return err
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return errors.New("invalid destruction journal recovery connection configuration")
	}
	cfg.RuntimeParams["synchronous_commit"] = "on"
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return errors.New("connect to destruction journal for recovery")
	}
	defer conn.Close(ctx)
	head, records, err := (postgresJournal{conn: conn, installation: installation}).fenceAndRead(ctx)
	if err != nil {
		return err
	}
	start, err := cipher.verifyBackupCheckpoint(checkpoint, digests, head)
	if err != nil {
		return err
	}
	operations, err := cipher.completedJournalTargets(head, records)
	if err != nil {
		return err
	}
	seen := make(map[int]bool)
	for _, op := range operations {
		if op.Sequence <= start.Sequence {
			continue
		}
		r := op.Receipt
		if r == nil || r.ID <= 0 || r.CreatedAt.IsZero() || seen[r.ID] {
			return errors.New("invalid or duplicate destruction journal receipt")
		}
		seen[r.ID] = true
		if err := writeRestoreJournalRow(out, op); err != nil {
			return err
		}
	}
	return nil
}

func writeRestoreJournalRow(out io.Writer, op completedJournalOperation) error {
	r := op.Receipt
	quote := func(value string) string { return `"` + strings.ReplaceAll(value, `"`, `""`) + `"` }
	nullableID := func(id *int) string {
		if id == nil {
			return ""
		}
		return strconv.Itoa(*id)
	}
	_, err := fmt.Fprintln(out, strings.Join([]string{
		strconv.Itoa(r.ID), quote(op.EntityType), nullableID(op.EntityID), quote(op.Action),
		quote(op.Basis), quote(op.ApplicationNumber), quote(op.SubjectAnchors),
		strconv.Itoa(r.Rows), strconv.Itoa(r.Files), nullableID(r.ActorID), quote(r.ActorNote),
		quote(r.CreatedAt.UTC().Format(time.RFC3339Nano)), "0", "",
	}, ","))
	return err
}
