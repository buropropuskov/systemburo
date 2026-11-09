package entityarchive

import (
	"bytes"
	"testing"
)

func TestIndependentJournalRejectsUnverifiedRecovery(t *testing.T) {
	codec, err := newJournalCipher(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	head := journalHead{Installation: "training", Generation: 2, Sequence: 2}
	target := journalTarget{EntityType: "application", Action: "purged", ApplicationNumber: "training-001"}
	intent, err := codec.seal(head.Installation, 1, 1, journalEvent{
		OperationID: "op-1", Phase: journalIntent, Target: &target,
	})
	if err != nil {
		t.Fatal(err)
	}
	completed, err := codec.seal(head.Installation, 2, 2, journalEvent{
		OperationID: "op-1", Phase: journalCompleted,
	})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(intent.Ciphertext, []byte(target.ApplicationNumber)) {
		t.Fatal("target must be encrypted")
	}
	targets, err := codec.completedJournalTargets(head, []sealedJournalEvent{intent, completed})
	if err != nil || len(targets) != 1 || targets[0].journalTarget != target {
		t.Fatalf("expected one recovered target, got %v, %v", targets, err)
	}

	altered := completed
	altered.Ciphertext = bytes.Clone(altered.Ciphertext)
	altered.Ciphertext[0] ^= 1
	wrongInstallation := head
	wrongInstallation.Installation = "different"
	pendingHead := head
	pendingHead.Sequence = 1

	for _, test := range []struct {
		name    string
		head    journalHead
		records []sealedJournalEvent
	}{
		{"missing tail", head, []sealedJournalEvent{intent}},
		{"unresolved intent", pendingHead, []sealedJournalEvent{intent}},
		{"wrong installation", wrongInstallation, []sealedJournalEvent{intent, completed}},
		{"changed ciphertext", head, []sealedJournalEvent{intent, altered}},
		{"reordered records", head, []sealedJournalEvent{completed, intent}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if targets, err := codec.completedJournalTargets(test.head, test.records); err == nil || targets != nil {
				t.Fatalf("expected no recovery targets and an error, got %v, %v", targets, err)
			}
		})
	}
}
