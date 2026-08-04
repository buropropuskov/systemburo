package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Кворум круга согласования (#1685, срез S3). Функция общая для основного круга заявки и
// круга раунда дополнения, поэтому её расклады зафиксированы отдельно от обоих: разъехаться
// им нельзя, и первым это заметит именно этот тест.
func TestTallyApprovals(t *testing.T) {
	t.Parallel()

	required := func(status string) approvalVote { return approvalVote{Required: true, Status: &status} }
	optional := func(status string) approvalVote { return approvalVote{Required: false, Status: &status} }

	cases := []struct {
		name  string
		votes []approvalVote
		want  string
	}{
		{
			name:  "круг без голосующих вердикта не даёт",
			votes: nil,
			want:  voteStatusPending,
		},
		{
			name:  "единственный обязательный согласовал",
			votes: []approvalVote{required(voteStatusApproved)},
			want:  voteStatusApproved,
		},
		{
			name:  "единственный обязательный отказал",
			votes: []approvalVote{required(voteStatusRejected)},
			want:  voteStatusRejected,
		},
		{
			name:  "обязательные согласовали не все - круг идёт",
			votes: []approvalVote{required(voteStatusApproved), required(voteStatusPending)},
			want:  voteStatusPending,
		},
		{
			name:  "согласовали все обязательные",
			votes: []approvalVote{required(voteStatusApproved), required(voteStatusApproved)},
			want:  voteStatusApproved,
		},
		{
			name:  "один обязательный отказ хоронит круг, даже если остальные за",
			votes: []approvalVote{required(voteStatusApproved), required(voteStatusRejected)},
			want:  voteStatusRejected,
		},
		{
			name:  "обязательный отказ весомее незакрытых голосов",
			votes: []approvalVote{required(voteStatusRejected), required(voteStatusPending)},
			want:  voteStatusRejected,
		},
		{
			name:  "необязательные при живых обязательных на итог не влияют",
			votes: []approvalVote{required(voteStatusPending), optional(voteStatusApproved), optional(voteStatusRejected)},
			want:  voteStatusPending,
		},
		{
			name:  "отказ необязательного не мешает согласию всех обязательных",
			votes: []approvalVote{required(voteStatusApproved), optional(voteStatusRejected)},
			want:  voteStatusApproved,
		},
		{
			name:  "обязательных нет: хватает одного согласия необязательного",
			votes: []approvalVote{optional(voteStatusApproved), optional(voteStatusPending)},
			want:  voteStatusApproved,
		},
		{
			name:  "обязательных нет: отказ необязательного перевешивает согласие соседа",
			votes: []approvalVote{optional(voteStatusApproved), optional(voteStatusRejected)},
			want:  voteStatusRejected,
		},
		{
			name:  "обязательных нет и никто не голосовал",
			votes: []approvalVote{optional(voteStatusPending), optional(voteStatusPending)},
			want:  voteStatusPending,
		},
		{
			name:  "NULL в статусе равнозначен pending",
			votes: []approvalVote{{Required: true, Status: nil}},
			want:  voteStatusPending,
		},
		{
			name:  "NULL у необязательного не считается согласием",
			votes: []approvalVote{{Required: false, Status: nil}},
			want:  voteStatusPending,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tallyApprovals(tc.votes))
		})
	}
}
