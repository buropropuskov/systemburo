package main

import (
	"bytes"
	"flag"
	"io"
	"strings"
	"testing"
)

// Разбор аргументов сидера (#1760). Инцидент был такой: `go run ./cmd/seed --help`
// не печатал справку, а выставлял супер-администратору пароль "--help" - и на общей
// базе стенда сразу всем. Проверяем именно это: во что превращается аргумент.

// parse - разбор с ContinueOnError, чтобы тест видел ошибку вместо выхода процесса.
func parse(t *testing.T, args ...string) (string, passwordSource, error) {
	t.Helper()
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return resolvePassword(fs, args)
}

func TestResolvePassword_NoArgs_UsesDefault(t *testing.T) {
	pass, src, err := parse(t)
	if err != nil {
		t.Fatalf("без аргументов ошибки быть не должно: %v", err)
	}
	if pass != defaultPassword {
		t.Fatalf("ожидали пароль по умолчанию %q, получили %q", defaultPassword, pass)
	}
	if src != passwordDefault {
		t.Fatalf("источник пароля должен быть «по умолчанию», получили %v", src)
	}
}

func TestResolvePassword_Flag(t *testing.T) {
	pass, src, err := parse(t, "-password", "s3cret")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if pass != "s3cret" {
		t.Fatalf("ожидали s3cret, получили %q", pass)
	}
	if src != passwordFlag {
		t.Fatalf("источник должен быть «флаг», получили %v", src)
	}
}

// Позиционный аргумент оставлен рабочим намеренно: так пароль передают сборка e2e в
// CI (.github/workflows/e2e.yml) и цели Makefile. Если этот тест покраснеет после
// правки - сначала посмотри, не сломается ли CI.
func TestResolvePassword_PositionalStillWorks(t *testing.T) {
	pass, src, err := parse(t, "BuroAdmin2026!")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if pass != "BuroAdmin2026!" {
		t.Fatalf("ожидали BuroAdmin2026!, получили %q", pass)
	}
	if src != passwordPositional {
		t.Fatalf("источник должен быть «позиционный», получили %v", src)
	}
}

// Суть issue: справка не должна становиться паролем.
func TestResolvePassword_HelpIsNotAPassword(t *testing.T) {
	for _, arg := range []string{"--help", "-h", "-help"} {
		pass, _, err := parse(t, arg)
		if err == nil {
			t.Fatalf("%s должен обрабатываться разбором флагов, а не молча проходить дальше", arg)
		}
		if !strings.Contains(err.Error(), "help") {
			t.Fatalf("%s: ожидали ошибку справки, получили %v", arg, err)
		}
		if pass == arg {
			t.Fatalf("%s уехал в пароль - ровно тот дефект, из-за которого задача и появилась", arg)
		}
		if pass != "" {
			t.Fatalf("%s: пароль должен остаться пустым, получили %q", arg, pass)
		}
	}
}

// Опечатка в имени флага должна останавливать команду, а не подставляться в пароль.
func TestResolvePassword_UnknownFlagIsError(t *testing.T) {
	pass, _, err := parse(t, "-dry-run")
	if err == nil {
		t.Fatal("неизвестный флаг обязан быть ошибкой")
	}
	if pass == "-dry-run" {
		t.Fatal("неизвестный флаг уехал в пароль")
	}
}

// Явный флаг важнее позиционного: если заданы оба, берём -password.
func TestResolvePassword_FlagWinsOverPositional(t *testing.T) {
	pass, src, err := parse(t, "-password", "fromFlag", "fromPositional")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if pass != "fromFlag" {
		t.Fatalf("ожидали fromFlag, получили %q", pass)
	}
	if src != passwordFlag {
		t.Fatalf("источник должен быть «флаг», получили %v", src)
	}
}

func TestUsage_MentionsFlagAndEnv(t *testing.T) {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	_, _, _ = resolvePassword(fs, nil)

	var buf bytes.Buffer
	usage(&buf, fs)
	out := buf.String()
	for _, want := range []string{"-password", "DATABASE_URL", "SEED_DEMO", defaultPassword} {
		if !strings.Contains(out, want) {
			t.Fatalf("в справке нет %q:\n%s", want, out)
		}
	}
}
