package models

import "testing"

func TestPagination_CheckedOffsetOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	for _, tc := range []struct {
		page, size, want int
		invalid          bool
	}{
		{1, 20, 0, false}, {3, 20, 40, false}, {maxInt, 1, maxInt - 1, false},
		{maxInt/100 + 1, 100, (maxInt / 100) * 100, false},
		{maxInt/100 + 2, 100, 0, true}, {maxInt, 100, 0, true},
		{0, 20, 0, true}, {1, 0, 0, true},
	} {
		got, err := CheckedOffset(tc.page, tc.size)
		if (err != nil) != tc.invalid || got != tc.want {
			t.Fatalf("(%d,%d): got %d, %v", tc.page, tc.size, got, err)
		}
	}
}

func TestRequestLogs_NormalizePaging(t *testing.T) {
	for _, tc := range []struct{ page, size, wantPage, wantSize int }{
		{0, 0, 1, 20}, {-2, -4, 1, 20}, {2, 200, 2, 100}, {3, 25, 3, 25},
	} {
		q := RequestLogsQuery{Page: tc.page, PerPage: tc.size}
		q.Normalize()
		if q.Page != tc.wantPage || q.PerPage != tc.wantSize {
			t.Fatalf("got %+v", q)
		}
	}
}
