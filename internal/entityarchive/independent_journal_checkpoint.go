package entityarchive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"

	"github.com/jackc/pgx/v5"
)

const journalCheckpointDomain = "destruction-journal/backup-checkpoint/v1"

// BackupJournalCheckpoint returns the current journal position with no
// arguments, or seals the original position and three component digests.
func BackupJournalCheckpoint(ctx context.Context, args []string) (string, error) {
	if len(args) != 0 && len(args) != 4 {
		return "", errors.New("expected original journal position and three SHA-256 digests")
	}
	dsn := os.Getenv("DESTRUCTION_JOURNAL_WRITER_DSN")
	installation := os.Getenv("DESTRUCTION_JOURNAL_INSTALLATION")
	if dsn == "" || installation == "" {
		return "", errors.New("incomplete destruction journal configuration")
	}
	key, err := hex.DecodeString(os.Getenv("DESTRUCTION_JOURNAL_KEY"))
	if err != nil {
		return "", errors.New("invalid destruction journal key")
	}
	cipher, err := newJournalCipher(key)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, journalTimeout)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return "", errors.New("connect to destruction journal")
	}
	defer conn.Close(ctx)
	head, err := (postgresJournal{conn: conn, installation: installation}).writerHead(ctx)
	if err != nil {
		return "", err
	}
	if head.Pending {
		return "", errors.New("destruction journal has an unfinished operation")
	}
	if len(args) == 0 {
		encoded, err := json.Marshal(head)
		return string(encoded), err
	}
	var before journalHead
	if err := json.Unmarshal([]byte(args[0]), &before); err != nil {
		return "", errors.New("invalid original journal position")
	}
	var digests journalBackupDigests
	for i, dest := range []*[sha256.Size]byte{&digests.Database, &digests.Uploads, &digests.Archive} {
		decoded, err := hex.DecodeString(args[i+1])
		if err != nil || len(decoded) != sha256.Size {
			return "", errors.New("invalid backup component SHA-256 digest")
		}
		copy(dest[:], decoded)
	}
	sealed, err := cipher.sealBackupCheckpoint(before, head, digests)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sealed), nil
}

type journalBackupDigests struct {
	Database [sha256.Size]byte
	Uploads  [sha256.Size]byte
	Archive  [sha256.Size]byte
}

type journalBackupCheckpoint struct {
	Head    journalHead
	Digests journalBackupDigests
}

// Seal only after observing the same idle head before and after the complete
// backup. This excludes snapshots taken during an unfinished destruction.
func (c journalCipher) sealBackupCheckpoint(before, after journalHead, digests journalBackupDigests) ([]byte, error) {
	if before != after || before.Installation == "" || before.Generation == 0 || before.Pending || before.Fenced {
		return nil, errors.New("destruction journal changed or was unavailable during backup")
	}
	if !digests.complete() {
		return nil, errors.New("independent recovery requires all three backup components")
	}
	payload, err := json.Marshal(journalBackupCheckpoint{Head: before, Digests: digests})
	if err != nil {
		return nil, err
	}
	return c.aead.Seal(nil, nil, payload, []byte(journalCheckpointDomain)), nil
}

func (d journalBackupDigests) complete() bool {
	var empty [sha256.Size]byte
	return d.Database != empty && d.Uploads != empty && d.Archive != empty
}

func (c journalCipher) openBackupCheckpoint(sealed []byte, digests journalBackupDigests, installation string) (journalHead, error) {
	payload, err := c.aead.Open(nil, nil, sealed, []byte(journalCheckpointDomain))
	if err != nil {
		return journalHead{}, errors.New("cannot authenticate backup checkpoint")
	}
	var checkpoint journalBackupCheckpoint
	if err := json.Unmarshal(payload, &checkpoint); err != nil {
		return journalHead{}, errors.New("invalid backup checkpoint")
	}
	start := checkpoint.Head
	if !digests.complete() || checkpoint.Digests != digests || start.Installation == "" ||
		start.Installation != installation || start.Generation == 0 || start.Pending || start.Fenced {
		return journalHead{}, errors.New("backup is outside verified destruction journal coverage")
	}
	return start, nil
}

func (c journalCipher) verifyBackupCheckpoint(sealed []byte, digests journalBackupDigests, head journalHead) (journalHead, error) {
	start, err := c.openBackupCheckpoint(sealed, digests, head.Installation)
	if err != nil {
		return journalHead{}, err
	}
	if start.Generation >= head.Generation || start.Sequence > head.Sequence || !head.Fenced || head.Pending {
		return journalHead{}, errors.New("backup is outside verified destruction journal coverage")
	}
	return start, nil
}
