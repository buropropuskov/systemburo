package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Строки, читаемые сырым SQL, идут мимо AfterFind, поэтому расшифровка у них
// выписана руками. Список полей при этом обязан повторять перечень шифруемых: когда
// «иное разрешение» добавили в шифрование (#2351), а в эти списки не внесли, оператор
// увидел в карточке заявки набор символов вместо документа (#2413).
//
// Проверка идёт по исходникам, потому что ловить надо именно пропуск строки, а не
// поведение: место, где расшифровку забыли, тестом на поведение не покрыто по
// определению - о нём не помнят.

// encryptedEmployeeFields - поля работника, которые шифруются и потому требуют
// ручной расшифровки везде, где строка прочитана сырым запросом.
var encryptedEmployeeFields = []string{"PassportSeriesNumber", "PatentNumber", "OtherPermission"}

func TestManualDecryptCoversAllEncryptedFields(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("не удалось перечислить исходники: %v", err)
	}

	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		text := string(body)

		counts := make(map[string]int, len(encryptedEmployeeFields))
		for _, field := range encryptedEmployeeFields {
			counts[field] = strings.Count(text, field+" = crypto.DecryptOptional")
		}
		if counts["PassportSeriesNumber"] == 0 {
			continue
		}

		want := counts["PassportSeriesNumber"]
		for _, field := range encryptedEmployeeFields {
			if counts[field] != want {
				t.Errorf("%s: расшифровок PassportSeriesNumber %d, а %s %d - список полей разъехался, "+
					"и недостающее уйдёт в интерфейс шифротекстом", name, want, field, counts[field])
			}
		}
	}
}
