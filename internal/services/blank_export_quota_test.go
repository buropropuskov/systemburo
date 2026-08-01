package services

import (
	"testing"

	"systemburo/internal/diskspace"
	"systemburo/internal/models"
)

func TestSoftThresholdTripped(t *testing.T) {
	cases := []struct {
		name    string
		usage   diskspace.Usage
		percent int
		want    bool
	}{
		{"ниже порога", diskspace.Usage{TotalBytes: 100, FreeBytes: 30}, 80, false},
		{"ровно порог", diskspace.Usage{TotalBytes: 100, FreeBytes: 20}, 80, true},
		{"выше порога", diskspace.Usage{TotalBytes: 100, FreeBytes: 5}, 80, true},
		{"порог не задан", diskspace.Usage{TotalBytes: 100, FreeBytes: 0}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := &models.ArchiveSettings{WarnPercent: tc.percent}
			if got := softThresholdTripped(tc.usage, settings); got != tc.want {
				t.Errorf("softThresholdTripped() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHardThresholdTripped(t *testing.T) {
	cases := []struct {
		name         string
		usage        diskspace.Usage
		archiveBytes int64
		settings     models.ArchiveSettings
		wantTripped  bool
		wantReason   string
	}{
		{
			name:        "достаточно места, квота не задана",
			usage:       diskspace.Usage{FreeBytes: 10 << 30},
			settings:    models.ArchiveSettings{MinFreeBytes: 2 << 30},
			wantTripped: false,
		},
		{
			name:        "свободного места меньше минимума",
			usage:       diskspace.Usage{FreeBytes: 1 << 30},
			settings:    models.ArchiveSettings{MinFreeBytes: 2 << 30},
			wantTripped: true,
			wantReason:  "insufficient_free_space",
		},
		{
			name:         "архив достиг квоты",
			usage:        diskspace.Usage{FreeBytes: 100 << 30},
			archiveBytes: 5 << 30,
			settings:     models.ArchiveSettings{QuotaBytes: 5 << 30},
			wantTripped:  true,
			wantReason:   "quota_exceeded",
		},
		{
			name:         "архив ниже квоты",
			usage:        diskspace.Usage{FreeBytes: 100 << 30},
			archiveBytes: 4 << 30,
			settings:     models.ArchiveSettings{QuotaBytes: 5 << 30},
			wantTripped:  false,
		},
		{
			name:        "оба порога выключены (0)",
			usage:       diskspace.Usage{FreeBytes: 0},
			settings:    models.ArchiveSettings{MinFreeBytes: 0, QuotaBytes: 0},
			wantTripped: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotTripped, gotReason := hardThresholdTripped(tc.usage, tc.archiveBytes, &tc.settings)
			if gotTripped != tc.wantTripped {
				t.Errorf("hardThresholdTripped() tripped = %v, want %v", gotTripped, tc.wantTripped)
			}
			if gotReason != tc.wantReason {
				t.Errorf("hardThresholdTripped() reason = %q, want %q", gotReason, tc.wantReason)
			}
		})
	}
}

func TestFormatBytesRu(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{500, "500 Б"},
		{2048, "2 КБ"},
		{5 * 1024 * 1024, "5.0 МБ"},
		{3 * 1024 * 1024 * 1024, "3.0 ГБ"},
	}
	for _, tc := range cases {
		if got := formatBytesRu(tc.n); got != tc.want {
			t.Errorf("formatBytesRu(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}
