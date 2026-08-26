package services

import (
	"strings"
	"testing"
)

// Сдвиг ссылок в формулах правил условного форматирования: строки от границы вставки
// уезжают на тот же шаг, что и диапазоны правил, остальное остаётся как есть.
func TestShiftFormulaRows(t *testing.T) {
	cases := []struct {
		name    string
		expr    string
		fromRow int
		offset  int
		want    string
	}{
		{"абсолютная ссылка ниже границы", `$A$38=""`, 34, 2, `$A$40=""`},
		{"ссылка выше границы не двигается", `$B$15=""`, 34, 2, `$B$15=""`},
		{"относительная ссылка", `A38>0`, 34, 2, `A40>0`},
		{"диапазон", `SUM($A$38:$D$40)>0`, 34, 2, `SUM($A$40:$D$42)>0`},
		{"строка ровно на границе", `$A$34=""`, 34, 2, `$A$36=""`},
		{"строка перед границей", `$A$33=""`, 34, 2, `$A$33=""`},
		{"адрес внутри текста не адрес", `$A$38="A38"`, 34, 2, `$A$40="A38"`},
		{"функция с цифрой в имени", `LOG10($A$38)>1`, 34, 2, `LOG10($A$40)>1`},
		{"смешанные ссылки", `AND($A$38<>"",B40=1)`, 34, 2, `AND($A$40<>"",B42=1)`},
		{"без ссылок", `1=1`, 34, 2, `1=1`},
		{"ссылка с листом", `'Лист 2'!$A$38=""`, 34, 2, `'Лист 2'!$A$40=""`},
		{"трёхбуквенная колонка", `$AAB$38=""`, 34, 2, `$AAB$40=""`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shiftFormulaRows(c.expr, c.fromRow, c.offset); got != c.want {
				t.Fatalf("shiftFormulaRows(%q) = %q, ожидалось %q", c.expr, got, c.want)
			}
		})
	}
}

// В листе трогаем только формулы внутри правил: SQRef уже сдвинул excelize, а формулы
// обычных ячеек он правит сам.
func TestShiftSheetConditionalFormulas(t *testing.T) {
	sheet := `<worksheet>` +
		`<sheetData><row r="40"><c r="B40"><f>SUM($A$38:$A$39)</f></c></row></sheetData>` +
		`<conditionalFormatting sqref="A40:D40"><cfRule type="expression" dxfId="1" priority="1">` +
		`<formula>$A$38=""</formula></cfRule></conditionalFormatting>` +
		`<conditionalFormatting sqref="B15"><cfRule type="expression" dxfId="2" priority="2">` +
		`<formula>$B$15=""</formula></cfRule></conditionalFormatting>` +
		`</worksheet>`

	got := shiftSheetConditionalFormulas(sheet, 34, 2)

	if want := `<formula>$A$40=""</formula>`; !strings.Contains(got, want) {
		t.Fatalf("формула правила не сдвинулась: %s", got)
	}
	if want := `<formula>$B$15=""</formula>`; !strings.Contains(got, want) {
		t.Fatalf("правило выше вставки тронуто: %s", got)
	}
	if want := `<f>SUM($A$38:$A$39)</f>`; !strings.Contains(got, want) {
		t.Fatalf("формула ячейки не должна меняться: %s", got)
	}
	if want := `sqref="A40:D40"`; !strings.Contains(got, want) {
		t.Fatalf("диапазон правила тронут: %s", got)
	}
}

func TestShiftConditionalFormatFormulas_НетВставки(t *testing.T) {
	data := []byte("не архив")
	got, err := shiftConditionalFormatFormulas(data, 34, 0)
	if err != nil {
		t.Fatalf("без вставки ошибок быть не должно: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("без вставки файл должен остаться прежним")
	}
}
