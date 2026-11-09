package entityarchive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestPostgresJournal(t *testing.T) {
	dsn := os.Getenv("TEST_INDEPENDENT_JOURNAL_DSN")
	if dsn == "" {
		t.Skip("requires a disposable PostgreSQL with the independent journal schema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "INSERT INTO restore_journal.installations (installation, writer_login) VALUES ('go-integration', session_user)"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, "SET ROLE restore_journal_writer"); err != nil {
		t.Fatal(err)
	}
	journal := postgresJournal{conn: conn, installation: "go-integration"}
	cipher, err := newJournalCipher(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	id := 42
	target := journalTarget{EntityType: "application", EntityID: &id, ApplicationNumber: "synthetic", Action: "purge", Basis: "test"}
	for _, event := range []journalEvent{
		{OperationID: "op", Phase: journalIntent, Target: &target},
		{OperationID: "op", Phase: journalCompleted},
	} {
		head, err := journal.writerHead(ctx)
		if err != nil {
			t.Fatal(err)
		}
		record, err := cipher.seal(head.Installation, head.Generation, head.Sequence+1, event)
		if err != nil {
			t.Fatal(err)
		}
		if err := journal.append(ctx, event, record); err != nil {
			t.Fatal(err)
		}
		if err := journal.append(ctx, event, record); err != nil {
			t.Fatalf("retry: %v", err)
		}
	}
	backupHead, err := journal.writerHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	digests := journalBackupDigests{
		Database: sha256.Sum256([]byte("database")),
		Uploads:  sha256.Sum256([]byte("uploads")),
		Archive:  sha256.Sum256([]byte("archive")),
	}
	t.Setenv("DESTRUCTION_JOURNAL_WRITER_DSN", dsn)
	t.Setenv("DESTRUCTION_JOURNAL_INSTALLATION", "go-integration")
	t.Setenv("DESTRUCTION_JOURNAL_KEY", hex.EncodeToString(make([]byte, 32)))
	position, err := BackupJournalCheckpoint(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := BackupJournalCheckpoint(ctx, []string{position,
		hex.EncodeToString(digests.Database[:]), hex.EncodeToString(digests.Uploads[:]), hex.EncodeToString(digests.Archive[:]),
	})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := hex.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	pending := backupHead
	pending.Pending = true
	if _, err := cipher.sealBackupCheckpoint(pending, pending, digests); err == nil {
		t.Fatal("sealed checkpoint during a pending destruction")
	}
	if _, err := cipher.sealBackupCheckpoint(backupHead, pending, digests); err == nil {
		t.Fatal("sealed checkpoint while journal changed")
	}
	for _, event := range []journalEvent{
		{OperationID: "after-backup", Phase: journalIntent, Target: &target},
		{OperationID: "after-backup", Phase: journalCompleted, Receipt: &journalReceipt{
			ID: 9, ActorNote: "synthetic", CreatedAt: time.Unix(1700000000, 0).UTC(),
		}},
	} {
		head, err := journal.writerHead(ctx)
		if err != nil {
			t.Fatal(err)
		}
		record, err := cipher.seal(head.Installation, head.Generation, head.Sequence+1, event)
		if err != nil {
			t.Fatal(err)
		}
		if err := journal.append(ctx, event, record); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("DESTRUCTION_JOURNAL_RECOVERY_DSN", dsn)
	var exported bytes.Buffer
	if err := ExportIndependentRestoreJournal(ctx, []string{encoded,
		hex.EncodeToString(digests.Database[:]), hex.EncodeToString(digests.Uploads[:]), hex.EncodeToString(digests.Archive[:]),
	}, &exported); err != nil {
		t.Fatal(err)
	}
	if want := "9,\"application\",42,\"purge\",\"test\",\"synthetic\",\"\",0,0,,\"synthetic\",\"2023-11-14T22:13:20Z\",0,\n"; exported.String() != want {
		t.Fatalf("unexpected exported row: %q", exported.String())
	}
	if _, err := conn.Exec(ctx, "SET ROLE restore_journal_recovery"); err != nil {
		t.Fatal(err)
	}
	head, records, err := journal.fenceAndRead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cipher.verifyBackupCheckpoint(checkpoint, digests, head); err != nil {
		t.Fatal(err)
	}
	digests.Archive = sha256.Sum256([]byte("archive from a different backup"))
	if _, err := cipher.verifyBackupCheckpoint(checkpoint, digests, head); err == nil {
		t.Fatal("accepted mismatched archive")
	}
	targets, err := cipher.completedJournalTargets(head, records)
	if err != nil || len(targets) != 2 || targets[0].ApplicationNumber != target.ApplicationNumber {
		t.Fatalf("unexpected recovery result: %v, %v", targets, err)
	}
}
