package services

import (
	"slices"
	"testing"
)

func TestMergeUniqueIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		sender int
		groups [][]int
		want   []int // отсортированный ожидаемый набор
	}{
		{"только sender", 1, nil, []int{1}},
		{"sender + ответственные", 1, [][]int{{2, 3}}, []int{1, 2, 3}},
		{"sender дублируется в группе", 1, [][]int{{1, 2}}, []int{1, 2}},
		{"пересечение групп схлопывается", 1, [][]int{{2, 3}, {3, 4}}, []int{1, 2, 3, 4}},
		{"пустые группы дают только sender", 5, [][]int{{}, nil}, []int{5}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mergeUniqueIDs(tt.sender, tt.groups...)
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("mergeUniqueIDs = %v, ожидали %v", got, tt.want)
			}
		})
	}
}
