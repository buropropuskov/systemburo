package services

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// Отпечаток должен переживать сохранение файла и читаться обратно из ОБОИХ мест
// по отдельности: у пересохранённого чужим редактором бланка выживает обычно одно.
func TestBlankFingerprintRoundTrip(t *testing.T) {
	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	// Свойства, которые админ вписал в шаблон своими руками: SetDocProps переписывает
	// их целиком по переданной структуре, поэтому штамп обязан сначала прочитать
	// текущие. Без файла с непустыми свойствами эта проверка вакуумна.
	require.NoError(t, f.SetDocProps(&excelize.DocProperties{
		Title: "Заявка на проход", Creator: "Бюро пропусков", Subject: "Бланк",
	}))
	require.NoError(t, StampBlankFingerprint(f, BlankFingerprint{
		UniqueAttachmentID: 12, TemplateID: 34, ListStartRow: 7,
	}))

	var buf bytes.Buffer
	_, err := f.WriteTo(&buf)
	require.NoError(t, err)

	saved, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	defer func() { require.NoError(t, saved.Close()) }()

	props, err := saved.GetDocProps()
	require.NoError(t, err)
	require.Equal(t, "Заявка на проход", props.Title, "штамп затёр заголовок шаблона")
	require.Equal(t, "Бюро пропусков", props.Creator, "штамп затёр автора шаблона")
	require.Equal(t, "Бланк", props.Subject, "штамп затёр тему шаблона")
	fromProps, ok := ParseBlankFingerprint(props.Category)
	require.True(t, ok, "отпечаток не читается из свойств документа: %q", props.Category)
	require.Equal(t, BlankFingerprint{UniqueAttachmentID: 12, TemplateID: 34, ListStartRow: 7}, fromProps)

	cell, err := saved.GetCellValue(blankFingerprintSheet, blankFingerprintCell)
	require.NoError(t, err)
	fromSheet, ok := ParseBlankFingerprint(cell)
	require.True(t, ok, "отпечаток не читается со скрытого листа: %q", cell)
	require.Equal(t, fromProps, fromSheet)

	visible, err := saved.GetSheetVisible(blankFingerprintSheet)
	require.NoError(t, err)
	require.False(t, visible, "служебный лист виден заполняющему бланк")

	first := saved.GetSheetName(0)
	require.NotEqual(t, blankFingerprintSheet, first, "служебный лист стал первым")
	firstVisible, err := saved.GetSheetVisible(first)
	require.NoError(t, err)
	require.True(t, firstVisible, "рабочий лист бланка оказался скрыт")
}

// Повторная простановка не плодит листов и не оставляет старое значение: бланк
// одного и того же шаблона выдаётся много раз, а файл шаблона может уже нести
// отпечаток - например, если админ загрузил как шаблон ранее скачанный бланк.
func TestBlankFingerprintRestamp(t *testing.T) {
	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	require.NoError(t, StampBlankFingerprint(f, BlankFingerprint{UniqueAttachmentID: 1, TemplateID: 2, ListStartRow: 3}))
	sheetsAfterFirst := f.GetSheetList()

	require.NoError(t, StampBlankFingerprint(f, BlankFingerprint{UniqueAttachmentID: 9, TemplateID: 8, ListStartRow: 5}))
	require.Equal(t, sheetsAfterFirst, f.GetSheetList())

	fp, ok := ReadBlankFingerprint(f)
	require.True(t, ok)
	require.Equal(t, BlankFingerprint{UniqueAttachmentID: 9, TemplateID: 8, ListStartRow: 5}, fp)
}

// Произвольный .xlsx не должен опознаваться как бланк.
func TestReadBlankFingerprintForeignFile(t *testing.T) {
	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	require.NoError(t, f.SetCellStr("Sheet1", "A1", "Иванов"))

	_, ok := ReadBlankFingerprint(f)
	require.False(t, ok, "чужой файл опознан как бланк")
}

func TestParseBlankFingerprint(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want BlankFingerprint
		ok   bool
	}{
		{"полный", "systemburo:ua=5:tpl=7:rows=12", BlankFingerprint{5, 7, 12}, true},
		{"с пробелами", "  systemburo:ua=5:tpl=7:rows=12 ", BlankFingerprint{5, 7, 12}, true},
		{"незнакомые ключи не мешают", "systemburo:ua=5:tpl=7:rows=12:ver=2", BlankFingerprint{5, 7, 12}, true},
		{"пусто", "", BlankFingerprint{}, false},
		{"чужой префикс", "othersystem:ua=5:tpl=7:rows=12", BlankFingerprint{}, false},
		{"без шаблона", "systemburo:ua=5:rows=12", BlankFingerprint{}, false},
		{"без вложения", "systemburo:tpl=7:rows=12", BlankFingerprint{}, false},
		{"нулевое вложение", "systemburo:ua=0:tpl=7:rows=12", BlankFingerprint{}, false},
		{"не число", "systemburo:ua=abc:tpl=7:rows=12", BlankFingerprint{}, false},
		{"без строки списка", "systemburo:ua=5:tpl=7", BlankFingerprint{}, false},
		{"нулевая строка списка", "systemburo:ua=5:tpl=7:rows=0", BlankFingerprint{}, false},
		{"отрицательная строка списка", "systemburo:ua=5:tpl=7:rows=-3", BlankFingerprint{}, false},
		{"мусор вместо пары", "systemburo:ua=5:tpl=7:rows", BlankFingerprint{}, false},
		{"описание пользователя", "Отчёт за июль", BlankFingerprint{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseBlankFingerprint(tc.in)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}
