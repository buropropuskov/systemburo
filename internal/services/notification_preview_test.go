package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Сообщение заявки хранится размеченным текстом из редактора, и в уведомление его
// нельзя класть как есть - человек увидел бы теги (#974).
func TestPlainTextFromRichText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"теги снимаются", "<p>Привоз мебели</p>", "Привоз мебели"},
		{"абзацы не слипаются", "<p>Первый</p><p>Второй</p>", "Первый\nВторой"},
		{"перенос строки", "Первая<br>Вторая", "Первая\nВторая"},
		{"сущности разворачиваются", "&laquo;Ромашка&raquo; &amp; Ко", "«Ромашка» & Ко"},
		{"неразрывный пробел становится обычным", "Груз&nbsp;для&nbsp;склада", "Груз для склада"},
		{"вложенная разметка", "<div><b>Важно</b>: <i>сегодня</i></div>", "Важно: сегодня"},
		{"пусто остаётся пустым", "", ""},
		{"только разметка даёт пустоту", "<p></p><br>", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, plainTextFromRichText(c.in))
		})
	}
}

func TestPreviewText(t *testing.T) {
	t.Run("короткий текст не трогается", func(t *testing.T) {
		assert.Equal(t, "Привоз мебели", previewText("Привоз мебели", 50))
	})

	t.Run("длинный обрезается по границе слова", func(t *testing.T) {
		in := "Привоз офисной мебели и оргтехники на склад номер три к девяти утра"
		got := previewText(in, 30)
		assert.True(t, strings.HasSuffix(got, "..."), "ожидалось многоточие, получено %q", got)
		assert.NotContains(t, got, "оргтехник", "слово должно было отрезаться целиком")
		assert.LessOrEqual(t, len([]rune(got)), 33)
	})

	t.Run("кириллица не рубится посреди буквы", func(t *testing.T) {
		got := previewText(strings.Repeat("я", 300), 20)
		assert.True(t, len(got) > 0)
		// Битая руна декодируется как U+FFFD - её появление и означало бы обрезку по байтам.
		assert.NotContains(t, got, "�")
	})
}

// «Вложено 2 файла» не говорит ничего, а по названию человек понимает, накладная это
// или паспорт машины (#974).
func TestFilesLabel(t *testing.T) {
	t.Run("один файл", func(t *testing.T) {
		assert.Equal(t, "Файл: nakladnaya.pdf", filesLabel([]string{"nakladnaya.pdf"}))
	})

	t.Run("несколько - через запятую", func(t *testing.T) {
		assert.Equal(t, "Файлы: akt.pdf, pasport.pdf", filesLabel([]string{"akt.pdf", "pasport.pdf"}))
	})

	t.Run("длинный список обрезается, а не растягивает уведомление", func(t *testing.T) {
		got := filesLabel([]string{
			"Накладная на привоз мебели от 08.08.2026.pdf",
			"Паспорт транспортного средства.pdf",
			"Доверенность на водителя.pdf",
		})
		assert.True(t, strings.HasPrefix(got, "Файлы: "))
		assert.True(t, strings.HasSuffix(got, "..."), "ожидалось многоточие, получено %q", got)
		assert.LessOrEqual(t, len([]rune(got)), len("Файлы: ")+notificationFilesLimit+3)
	})

	t.Run("без вложений строки нет", func(t *testing.T) {
		assert.Equal(t, "", filesLabel(nil))
		assert.Equal(t, "", filesLabel([]string{"", "   "}))
	})
}

// Текст уведомления собирается строками, и пустые части выпадают целиком: у заявки может
// не быть ни организации, ни сообщения, ни файлов.
func TestPendingAcceptanceNoteMessage(t *testing.T) {
	t.Run("номер и организация в одной строке - её видно в свёрнутом уведомлении", func(t *testing.T) {
		note := pendingAcceptanceNote{
			number: "№ 20260808/003", organization: "ООО Ромашка",
			sender: "Иванов Иван Иванович", messageText: "<p>Привоз мебели</p>",
			fileNames: []string{"akt.pdf", "pasport.pdf"},
		}
		assert.Equal(t, "№ 20260808/003 · ООО Ромашка\n\nИванов Иван Иванович\n\nПривоз мебели\n\nФайлы: akt.pdf, pasport.pdf",
			note.message())
	})

	t.Run("вложения отдельным блоком, а не хвостом превью", func(t *testing.T) {
		note := pendingAcceptanceNote{number: "№ 9", messageText: "Груз", fileNames: []string{"akt.pdf"}}
		assert.Equal(t, "№ 9\n\nГруз\n\nФайл: akt.pdf", note.message())
	})

	t.Run("длинное сообщение обрезается до намёка", func(t *testing.T) {
		note := pendingAcceptanceNote{
			number:      "№ 10",
			messageText: "Привоз офисной мебели на склад номер три, просьба открыть ворота к девяти утра",
		}
		lines := strings.Split(note.message(), "\n\n")
		require.Len(t, lines, 2)
		assert.True(t, strings.HasSuffix(lines[1], "..."), "ожидалось многоточие, получено %q", lines[1])
		assert.LessOrEqual(t, len([]rune(lines[1])), notificationPreviewLimit+3)
	})

	t.Run("только номер - одна строка, без висящих отступов", func(t *testing.T) {
		note := pendingAcceptanceNote{number: "№ 1"}
		assert.Equal(t, "№ 1", note.message())
	})

	t.Run("без организации первая строка - только номер", func(t *testing.T) {
		note := pendingAcceptanceNote{number: "№ 2", sender: "Петров П. П."}
		assert.Equal(t, "№ 2\n\nПетров П. П.", note.message())
	})

	t.Run("без сообщения и файлов остаётся одна строка", func(t *testing.T) {
		note := pendingAcceptanceNote{number: "№ 4", organization: "ООО Ромашка"}
		assert.Equal(t, "№ 4 · ООО Ромашка", note.message())
	})

	t.Run("заголовок не дублируется в тексте", func(t *testing.T) {
		note := pendingAcceptanceNote{number: "№ 3", organization: "ООО Ромашка"}
		assert.NotContains(t, note.message(), "Поступила новая заявка")
		assert.NotContains(t, note.message(), "возьмут в работу")
	})
}
