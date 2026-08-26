package services

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Уведомления называют факт, а не должность того, кто нажал кнопку: «Ваша учётная запись
// заблокирована», а не «Администратор заблокировал вашу учётную запись». Причина не в
// вежливости - сменить пароль, снять роль или заблокировать может не только
// администратор, так что прежние формулировки ещё и вводили в заблуждение. Требование
// владельца по итогам работы на стенде (#974).
//
// Замок смотрит ровно на тексты уведомлений: строковые аргументы CreateForUser в
// исходниках пакета и подписи каталога. Ошибки доступа («Доступ только для
// супер-администратора»), подсказки («Обратитесь к администратору») и записи журнала
// аудита («сброшено администратором») под правило не попадают - там роль названа по делу.
func TestNotificationWording_NamesTheFactNotTheActor(t *testing.T) {
	for _, text := range notificationTextsFromSources(t) {
		assertNamesFact(t, text)
	}
	for code, meta := range notificationCatalog {
		assertNamesFact(t, code+": "+meta.Label)
		assertNamesFact(t, code+": "+meta.Description)
	}
}

func assertNamesFact(t *testing.T, text string) {
	t.Helper()
	lower := strings.ToLower(text)
	// «супер-администратор» - это название роли в отказе доступа, а не исполнитель.
	cleaned := strings.ReplaceAll(lower, "супер-администратор", "")
	if strings.Contains(cleaned, "администратор") {
		t.Errorf("текст уведомления называет должность вместо факта: %q", text)
	}
}

// notificationTextsFromSources собирает строковые литералы, уходящие в CreateForUser -
// это заголовок и сообщение уведомления. Рукописный перечень текстов пришлось бы
// пополнять при каждом новом уведомлении, и он молча пропустил бы забытое.
func notificationTextsFromSources(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("не удалось прочитать пакет: %v", err)
	}

	var texts []string
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Clean(name), nil, 0)
		if err != nil {
			t.Fatalf("не удалось разобрать %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !strings.HasPrefix(sel.Sel.Name, "CreateForUser") {
				return true
			}
			for _, arg := range call.Args {
				lit, ok := arg.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				if s, err := strconv.Unquote(lit.Value); err == nil {
					texts = append(texts, s)
				}
			}
			return true
		})
	}
	if len(texts) == 0 {
		t.Fatal("в пакете не найдено ни одного текста уведомления - замок смотрит не туда")
	}
	return texts
}
