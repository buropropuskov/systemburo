package entityarchive

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

type journalPhase string

const (
	journalIntent    journalPhase = "intent"
	journalCompleted journalPhase = "completed"
	journalCancelled journalPhase = "cancelled"
)

// journalEvent contains only the information required for recovery. In particular,
// it has no free text, personal document numbers, or file contents.
type journalEvent struct {
	OperationID string          `json:"operation_id"`
	Phase       journalPhase    `json:"phase"`
	Target      *journalTarget  `json:"target,omitempty"`
	Receipt     *journalReceipt `json:"receipt,omitempty"`
}

type journalReceipt struct {
	ID        int
	Rows      int
	Files     int
	ActorID   *int
	ActorNote string
	CreatedAt time.Time
}

type completedJournalOperation struct {
	journalTarget
	Receipt  *journalReceipt
	Sequence uint64
}

type journalTarget struct {
	EntityType        string `json:"entity_type"`
	EntityID          *int   `json:"entity_id,omitempty"`
	ApplicationNumber string `json:"application_number,omitempty"`
	SubjectAnchors    string `json:"subject_anchors,omitempty"`
	Action            string `json:"action"`
	Basis             string `json:"basis"`
}

type sealedJournalEvent struct {
	Sequence   uint64
	Generation uint64
	Ciphertext []byte
}

// journalHead must come from the store after fencing the previous writer.
type journalHead struct {
	Installation string
	Generation   uint64
	Sequence     uint64
	Pending      bool
	Fenced       bool
}

type journalCipher struct {
	aead cipher.AEAD
}

func newJournalCipher(key []byte) (journalCipher, error) {
	if len(key) != 32 {
		return journalCipher{}, errors.New("journal key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return journalCipher{}, err
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	return journalCipher{aead: aead}, err
}

func journalAssociatedData(installation string, generation, sequence uint64) []byte {
	return fmt.Appendf(nil, "destruction-journal/v1/%s/%s/%s",
		strconv.Quote(installation), strconv.FormatUint(generation, 10), strconv.FormatUint(sequence, 10))
}

func (c journalCipher) seal(installation string, generation, sequence uint64, event journalEvent) (sealedJournalEvent, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return sealedJournalEvent{}, err
	}
	return sealedJournalEvent{
		Generation: generation,
		Sequence:   sequence,
		Ciphertext: c.aead.Seal(nil, nil, payload, journalAssociatedData(installation, generation, sequence)),
	}, nil
}

// completedJournalTargets returns targets in intent order. A partially verified
// range must never yield work for the restore process.
func (c journalCipher) completedJournalTargets(head journalHead, records []sealedJournalEvent) ([]completedJournalOperation, error) {
	if head.Installation == "" || head.Generation == 0 || uint64(len(records)) != head.Sequence {
		return nil, errors.New("incomplete destruction journal")
	}

	type operation struct {
		completedJournalOperation
		phase journalPhase
	}
	operations := make(map[string]operation)
	order := make([]string, 0)
	var generation uint64

	for i, record := range records {
		if record.Sequence != uint64(i)+1 || record.Generation == 0 ||
			record.Generation < generation || record.Generation > head.Generation {
			return nil, errors.New("invalid destruction journal sequence or generation")
		}
		generation = record.Generation
		payload, err := c.aead.Open(nil, nil, record.Ciphertext,
			journalAssociatedData(head.Installation, record.Generation, record.Sequence))
		if err != nil {
			return nil, fmt.Errorf("authenticate destruction journal event %d: %w", record.Sequence, err)
		}
		var event journalEvent
		if err := json.Unmarshal(payload, &event); err != nil || event.OperationID == "" {
			return nil, errors.New("invalid destruction journal event")
		}
		op, exists := operations[event.OperationID]
		switch event.Phase {
		case journalIntent:
			if exists || event.Target == nil || event.Receipt != nil {
				return nil, errors.New("invalid destruction journal intent")
			}
			operations[event.OperationID] = operation{completedJournalOperation: completedJournalOperation{journalTarget: *event.Target}, phase: journalIntent}
			order = append(order, event.OperationID)
		case journalCompleted, journalCancelled:
			if !exists || op.phase != journalIntent || event.Target != nil || (event.Phase == journalCancelled && event.Receipt != nil) {
				return nil, errors.New("invalid destruction journal outcome")
			}
			op.phase = event.Phase
			op.Receipt = event.Receipt
			op.Sequence = record.Sequence
			operations[event.OperationID] = op
		default:
			return nil, errors.New("unknown destruction journal phase")
		}
	}

	targets := make([]completedJournalOperation, 0, len(order))
	for _, id := range order {
		op := operations[id]
		if op.phase == journalIntent {
			return nil, errors.New("destruction journal contains an unresolved intent")
		}
		if op.phase == journalCompleted {
			targets = append(targets, op.completedJournalOperation)
		}
	}
	return targets, nil
}
