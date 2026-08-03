package main

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Разбор размеров: настройка через консоль должна принимать человеческую запись,
// иначе оператор будет считать байты в уме и ошибётся на порядок.
func TestParseSizeArg(t *testing.T) {
	cases := []struct {
		raw  string
		want int64
	}{
		{"0", 0},
		{"1048576", 1048576},
		{"512K", 512 << 10},
		{"2M", 2 << 20},
		{"3G", 3 << 30},
		{"1T", 1 << 40},
		{" 4g ", 4 << 30},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := parseSizeArg(tc.raw)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseSizeArg_Rejects(t *testing.T) {
	for _, raw := range []string{"", "гигабайт", "2ГБ", "-1", "-5G", "1.5G"} {
		t.Run(raw, func(t *testing.T) {
			_, err := parseSizeArg(raw)
			assert.Error(t, err, "мусор не должен молча превращаться в размер")
		})
	}
}

// Журнал получает только реально изменившиеся значения: заданное флагом значение
// может совпасть с текущим, и запись «изменено» о нём была бы враньём.
func TestArchiveConsoleDiff(t *testing.T) {
	before := &models.ArchiveSettings{
		Enabled: false, DirTemplate: "{год}", FileTemplate: "{тип}",
		QuotaBytes: 0, MinFreeBytes: 2 << 30, WarnPercent: 80,
		RecheckDays: 30, FreezeAfterDays: 30, ZipMaxBytes: 2 << 30,
	}
	after := *before
	after.Enabled = true
	after.FreezeAfterDays = 60

	details := archiveConsoleDiff(before, &after)

	require.Len(t, details, 2, "меняли два значения - в журнал идут два: %v", details)
	assert.Equal(t, map[string]any{"old": false, "new": true}, details["enabled"])
	assert.Equal(t, map[string]any{"old": 30, "new": 60}, details["freeze_after_days"])
	assert.NotContains(t, details, "dir_template", "нетронутая настройка в журнал не попадает")
}

func TestArchiveConsoleDiff_NoChanges(t *testing.T) {
	s := &models.ArchiveSettings{DirTemplate: "{год}", FreezeAfterDays: 30}
	same := *s
	assert.Empty(t, archiveConsoleDiff(s, &same), "без изменений записывать нечего")
}

// Ошибка проверки настроек приходит из общего сервиса ответом echo. В консоли нужен
// сам текст, и берётся он по типу ошибки: разбор строки чужой библиотеки сломался бы
// на первом же изменении её формата.
func TestArchiveErrorText(t *testing.T) {
	assert.Equal(t, "Шаблон папок: неизвестный плейсхолдер",
		archiveErrorText(echo.NewHTTPError(http.StatusBadRequest, "Шаблон папок: неизвестный плейсхолдер")))

	// Обычная ошибка отдаётся как есть.
	assert.Equal(t, "нет соединения с базой", archiveErrorText(errors.New("нет соединения с базой")))

	// Обёрнутая ошибка echo тоже разворачивается.
	wrapped := fmt.Errorf("сохранение: %w", echo.NewHTTPError(http.StatusBadRequest, "Квота не может быть отрицательной"))
	assert.Equal(t, "Квота не может быть отрицательной", archiveErrorText(wrapped))
}
