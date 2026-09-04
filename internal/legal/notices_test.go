// Package legal держит замок на перечне сторонних компонентов.
//
// Собственного кода у пакета нет намеренно: проверять нечего, кроме соответствия
// файла THIRD-PARTY-NOTICES.md реальному составу зависимостей.
package legal

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Разрешительные лицензии требуют сохранять уведомление об авторских правах при
// поставке. Условие выполняется файлом THIRD-PARTY-NOTICES.md, а он собирается
// сценарием и, значит, отстаёт ровно настолько, насколько про него забыли.
// Добавленный модуль сам о себе не напомнит: сборка от этого не ломается,
// тесты не краснеют, и расхождение обнаружилось бы у заказчика. Замок делает
// расхождение видимым в тот же день.
//
// Проверка идёт от кода к документу: за состав берётся то, что компоновщик
// реально втягивает в двоичные файлы, а не список require из go.mod. В go.mod
// лежат и модули, нужные только тестам (testify) и генератору описания
// интерфейса; в поставку они не попадают, и требовать их в перечне неверно.

var (
	goSectionRe = regexp.MustCompile(`(?m)^## \d+\. Компоненты серверной части$`)
	tableRowRe  = regexp.MustCompile("(?m)^\\| `([^`]+)` \\| ([^|]+) \\|")
	nextHeadRe  = regexp.MustCompile(`(?m)^## `)
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller не вернул путь к текущему файлу")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// listedModules вынимает из перечня строки таблицы серверных компонентов.
func listedModules(t *testing.T, notices string) map[string]string {
	t.Helper()

	start := goSectionRe.FindStringIndex(notices)
	if start == nil {
		t.Fatal("в THIRD-PARTY-NOTICES.md нет раздела о компонентах серверной части")
	}
	rest := notices[start[1]:]
	if end := nextHeadRe.FindStringIndex(rest); end != nil {
		rest = rest[:end[0]]
	}

	modules := make(map[string]string)
	for _, row := range tableRowRe.FindAllStringSubmatch(rest, -1) {
		modules[row[1]] = strings.TrimSpace(row[2])
	}
	if len(modules) == 0 {
		t.Fatal("раздел о компонентах серверной части пуст")
	}
	return modules
}

// linkedModules спрашивает у самого Go, что уходит в двоичные файлы.
func linkedModules(t *testing.T, root string) map[string]string {
	t.Helper()

	// -buildvcs=false: при docker-first прогоне /app принадлежит разработчику, а
	// контейнер работает от root, и git отвечает «dubious ownership». Go снимает
	// VCS-статус даже для list и падает; лицензии зависимостей от него не зависят.
	cmd := exec.Command("go", "list", "-deps", "-buildvcs=false",
		"-f", "{{if .Module}}{{.Module.Path}}\t{{.Module.Version}}{{end}}", "./cmd/...")
	cmd.Dir = root
	// go.mod требует более свежий тулчейн, чем бывает установлен на машине
	// разработчика: без этого go list отказывается работать вместо того, чтобы
	// его подтянуть.
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=auto")

	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		t.Fatalf("go list не отработал: %v\n%s", err, stderr)
	}

	modules := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		path, version, found := strings.Cut(strings.TrimSpace(line), "\t")
		// Строки без версии - стандартная библиотека и сам проект.
		if !found || path == "systemburo" || version == "" {
			continue
		}
		modules[path] = version
	}
	if len(modules) == 0 {
		t.Fatal("go list не вернул ни одного стороннего модуля")
	}
	return modules
}

func TestNoticesCoverLinkedModules(t *testing.T) {
	root := repoRoot(t)

	raw, err := os.ReadFile(filepath.Join(root, "THIRD-PARTY-NOTICES.md"))
	if err != nil {
		t.Fatalf("не удалось прочитать THIRD-PARTY-NOTICES.md: %v", err)
	}

	listed := listedModules(t, string(raw))
	linked := linkedModules(t, root)

	var missing, extra, mismatched []string
	for path, version := range linked {
		got, ok := listed[path]
		switch {
		case !ok:
			missing = append(missing, path+" "+version)
		case got != version:
			mismatched = append(mismatched, path+": в перечне "+got+", в сборке "+version)
		}
	}
	for path := range listed {
		if _, ok := linked[path]; !ok {
			extra = append(extra, path)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	sort.Strings(mismatched)

	const remedy = "выполните python3 scripts/gen-third-party-notices.py и закоммитьте результат"
	if len(missing) > 0 {
		t.Errorf("модули уходят в поставку, но в перечне их нет (%s):\n\t%s",
			remedy, strings.Join(missing, "\n\t"))
	}
	if len(mismatched) > 0 {
		t.Errorf("версии в перечне разошлись со сборкой (%s):\n\t%s",
			remedy, strings.Join(mismatched, "\n\t"))
	}
	if len(extra) > 0 {
		t.Errorf("перечень называет модули, которых в сборке нет (%s):\n\t%s",
			remedy, strings.Join(extra, "\n\t"))
	}
}

// Раздел о шрифтах написан руками и живёт в шаблоне сценария, а не берётся из
// зависимостей. Условие Bitstream Vera требует, чтобы лицензия сопровождала сам
// файл шрифта: перечень на это ссылается, и ссылка обязана вести к файлу.
func TestFontLicenseShippedNextToFont(t *testing.T) {
	root := repoRoot(t)

	for _, rel := range []string{
		filepath.Join("internal", "export", "fonts", "DejaVuSans.ttf"),
		filepath.Join("internal", "export", "fonts", "LICENSE"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("%s недоступен: %v", rel, err)
		}
	}

	raw, err := os.ReadFile(filepath.Join(root, "THIRD-PARTY-NOTICES.md"))
	if err != nil {
		t.Fatalf("не удалось прочитать THIRD-PARTY-NOTICES.md: %v", err)
	}
	if !strings.Contains(string(raw), "internal/export/fonts/LICENSE") {
		t.Error("перечень перестал указывать, где лежит лицензия шрифта")
	}
}
