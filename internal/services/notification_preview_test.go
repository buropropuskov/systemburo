package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestFileCountLabel(t *testing.T) {
	cases := map[int]string{
		0: "", -3: "",
		1: "Вложено 1 файл", 2: "Вложено 2 файла", 5: "Вложено 5 файлов",
		11: "Вложено 11 файлов", 14: "Вложено 14 файлов",
		21: "Вложено 21 файл", 22: "Вложено 22 файла", 25: "Вложено 25 файлов",
	}
	for n, want := range cases {
		assert.Equal(t, want, fileCountLabel(n), "количество: %d", n)
	}
}

// Текст уведомления собирается строками, и пустые части выпадают целиком: у заявки может
// не быть ни организации, ни сообщения, ни файлов.
func TestPendingAcceptanceNoteMessage(t *testing.T) {
	t.Run("всё заполнено", func(t *testing.T) {
		note := pendingAcceptanceNote{
			number: "№ 20260808/003", organization: "ООО Ромашка",
			sender: "Иванов Иван Иванович", messageText: "<p>Привоз мебели</p>", fileCount: 2,
		}
		lines := strings.Split(note.message(), "\n")
		assert.Equal(t, []string{
			"Поступила новая заявка № 20260808/003",
			"«ООО Ромашка», Иванов Иван Иванович",
			"Привоз мебели",
			"Вложено 2 файла",
		}, lines)
	})

	t.Run("только номер - одна строка, без пустых ярлыков", func(t *testing.T) {
		note := pendingAcceptanceNote{number: "№ 1"}
		assert.Equal(t, "Поступила новая заявка № 1", note.message())
	})

	t.Run("без организации остаётся отправитель", func(t *testing.T) {
		note := pendingAcceptanceNote{number: "№ 2", sender: "Петров П. П."}
		assert.Contains(t, note.message(), "Петров П. П.")
		assert.NotContains(t, note.message(), "«»")
	})

	t.Run("прежней формулировки про работу больше нет", func(t *testing.T) {
		note := pendingAcceptanceNote{number: "№ 3", organization: "ООО Ромашка"}
		assert.NotContains(t, note.message(), "возьмут в работу")
	})
}
