package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// Этот файл - замок против класса дефекта #2027: параметр объявлен в Config
// (env-тег + envDefault), но docker-compose.base.yml передаёт бэкенду ЯВНЫЙ
// список переменных окружения, и туда его не добавили. Файл параметров можно
// заполнить как угодно - внутрь контейнера значение не попадёт, а ошибки нигде
// не будет: приложение молча стартует на дефолте. Тот же класс уже случался
// дважды (#974 - ключи доставки уведомлений, #1906/#1936 - почта), теперь #2027
// (пул БД, таймауты HTTP, конкурентность Argon2) - и при разборе обнаружился
// ещё почти весь остальной список, который scripts/init-env.sh пишет в .env,
// несмотря на то, что compose его туда даже не смотрит.
//
// Тест проверяет оба конца провода: переменная должна быть проброшена в
// docker-compose.base.yml КАК ${ИМЯ:-дефолт} (не хардкодом - иначе .env её не
// переопределит), и должна писаться генератором scripts/init-env.sh. Дефолт в
// compose обязан совпадать с envDefault в config.go - расхождение здесь хуже
// отсутствия строки, потому что выглядит рабочим.
//
// Исключения - переменные, которые сознательно устроены иначе (сетевой адрес
// контейнера, путь внутри файловой системы контейнера, конструируемая строка
// подключения и т.п.) - заносятся в envWiringExceptions с объяснением. Запись
// без объяснения не проходит: так исключение отличимо от забытого параметра.

// envWiringException документирует переменную окружения из Config, у которой
// путь до контейнера устроен не по общей схеме "${ИМЯ:-дефолт} в compose +
// строка в heredoc-е init-env.sh".
type envWiringException struct {
	// reason - для человека, который через год спросит "а почему этой
	// переменной здесь нет". Обязателен.
	reason string
	// skipCompose - переменная сознательно не идёт как ${ИМЯ:-...} в явном
	// списке docker-compose.base.yml (хардкод сетевого адреса/пути контейнера,
	// конструируемое значение, профиль-специфичный параметр из prod.yml).
	skipCompose bool
	// skipInitEnv - переменная сознательно не пишется генератором в .env.
	skipInitEnv bool
	// skipDefaultCheck - переменная присутствует и интерполируется нормально,
	// но её дефолт в compose осознанно расходится с envDefault в config.go.
	skipDefaultCheck bool
}

var envWiringExceptions = map[string]envWiringException{
	"DATABASE_URL": {
		reason: "собирается в compose из DB_USER/DB_PASSWORD/DB_NAME " +
			"(DATABASE_URL: postgres://${DB_USER}:${DB_PASSWORD}@db/${DB_NAME}), " +
			"а не пробрасывается как ${DATABASE_URL:-...}; init-env.sh пишет " +
			"готовую строку тем же способом (postgres://.../${DB_NAME})",
		skipCompose: true,
	},
	"BIND_HOST": {
		reason: "внутри контейнера всегда 0.0.0.0 - это адрес сетевого " +
			"интерфейса контейнера, а не параметр развёртывания: слушать на " +
			"loopback внутри docker-сети означало бы, что nginx до backend " +
			"вообще не достучится",
		skipCompose: true,
	},
	"BIND_PORT": {
		reason: "порт внутри контейнера фиксирован (совпадает с ожиданиями " +
			"nginx-апстрима), проброс наружу настраивается портами самого " +
			"compose, а не этой переменной",
		skipCompose: true,
	},
	"UPLOAD_PATH": {
		reason: "путь внутри контейнера, смонтированный именованным томом " +
			"uploads; каталог на хосте этим параметром не управляется вовсе",
		skipCompose: true,
	},
	"ARCHIVE_PATH": {
		reason: "путь внутри контейнера; каталог на хосте задаётся отдельной " +
			"переменной ARCHIVE_HOST_PATH через bind-mount, её init-env.sh и пишет",
		skipCompose: true,
		skipInitEnv: true,
	},
	"ENTITY_EXPORT_PATH": {
		reason: "путь внутри контейнера; каталог на хосте задаётся отдельной " +
			"переменной ENTITY_EXPORT_HOST_PATH через bind-mount, её init-env.sh и пишет",
		skipCompose: true,
		skipInitEnv: true,
	},
	"LOG_FILE_PATH": {
		reason: "путь внутри контейнера, фиксирован по профилю: задаётся " +
			"только в docker-compose.prod.yml (/app/logs/app.log + volume logs), " +
			"а не в base.yml, который проверяет этот тест. На стенде файловое " +
			"логирование сознательно выключено - пишем только в stdout",
		skipCompose: true,
		skipInitEnv: true,
	},
	"CORS_ALLOWED_ORIGINS": {
		reason: "дефолт в compose намеренно пустой, а не envDefault " +
			"http://localhost:8081 из config.go: тот дефолт рассчитан на " +
			"локальный docker-compose.yml, где фронт крутится на 8081. Для " +
			"stage/prod-профиля (base.yml) пустой CORS безопаснее случайного " +
			"дефолта на localhost, если строку в .env вдруг сотрут - init-env.sh " +
			"всё равно всегда прописывает реальный домен",
		skipDefaultCheck: true,
	},
}

// configEnvField - одно поле Config с его env-тегом.
type configEnvField struct {
	name       string
	hasDefault bool
	def        string
}

// configEnvFields читает env-теги прямо со структуры Config через reflect -
// единый источник правды, не список, который придётся держать в синхроне
// руками при каждом новом параметре.
func configEnvFields() []configEnvField {
	var out []configEnvField
	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		tag, ok := typ.Field(i).Tag.Lookup("env")
		if !ok {
			continue
		}
		name := strings.SplitN(tag, ",", 2)[0]
		def, hasDefault := typ.Field(i).Tag.Lookup("envDefault")
		out = append(out, configEnvField{name: name, hasDefault: hasDefault, def: def})
	}
	return out
}

// repoRoot вычисляется от пути самого тестового файла, а не от рабочего
// каталога - тест обязан работать независимо от того, откуда запущен go test.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller не вернул путь к текущему файлу")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func readRepoFile(t *testing.T, root, rel string) string {
	t.Helper()
	path := filepath.Join(root, rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("не удалось прочитать %s: %v", rel, err)
	}
	return string(data)
}

var (
	topServiceLineRe = regexp.MustCompile(`(?m)^  \S`)
	serviceKeyLineRe = regexp.MustCompile(`(?m)^    \S`)
	composeEnvKVRe   = regexp.MustCompile(`(?m)^ {6}([A-Z][A-Z0-9_]*):[ \t]?(.*)$`)
	composeInterpRe  = regexp.MustCompile(`^\$\{([A-Z][A-Z0-9_]*)(:-(.*))?\}$`)
	initEnvHeredocRe = regexp.MustCompile(`cat >>? "\$ENV_FILE" <<EOF\n`)
	initEnvAssignRe  = regexp.MustCompile(`(?m)^([A-Z][A-Z0-9_]*)=`)
)

// backendEnvironmentBlock вырезает из docker-compose.base.yml текст секции
// backend.environment, ориентируясь только на отступы (2/4/6 пробелов) -
// файл использует их последовательно, YAML-парсер тут был бы избыточен.
func backendEnvironmentBlock(t *testing.T, compose string) string {
	t.Helper()
	beIdx := strings.Index(compose, "\n  backend:\n")
	if beIdx == -1 {
		t.Fatal("не нашёл сервис backend в docker-compose.base.yml - структура файла изменилась сильнее, чем ожидает этот тест")
	}
	afterBackend := compose[beIdx+len("\n  backend:\n"):]
	if loc := topServiceLineRe.FindStringIndex(afterBackend); loc != nil {
		afterBackend = afterBackend[:loc[0]]
	}

	envIdx := strings.Index(afterBackend, "\n    environment:\n")
	if envIdx == -1 {
		t.Fatal("не нашёл секцию environment у сервиса backend в docker-compose.base.yml")
	}
	afterEnv := afterBackend[envIdx+len("\n    environment:\n"):]
	if loc := serviceKeyLineRe.FindStringIndex(afterEnv); loc != nil {
		afterEnv = afterEnv[:loc[0]]
	}
	return afterEnv
}

func parseComposeBackendEnv(block string) map[string]string {
	result := map[string]string{}
	for _, m := range composeEnvKVRe.FindAllStringSubmatch(block, -1) {
		result[m[1]] = strings.TrimSpace(m[2])
	}
	return result
}

// initEnvGeneratedKeys собирает имена переменных, которые scripts/init-env.sh
// реально пишет в файл параметров - только строки внутри heredoc-блоков
// "cat >> \"$ENV_FILE\"" (или "cat >"), а не временные bash-переменные
// генератора вроде CORS_ORIGINS/API_URL, которых в .env никогда не будет.
func initEnvGeneratedKeys(t *testing.T, script string) map[string]bool {
	t.Helper()
	starts := initEnvHeredocRe.FindAllStringIndex(script, -1)
	if len(starts) == 0 {
		t.Fatal("не нашёл ни одного heredoc-блока записи в $ENV_FILE в scripts/init-env.sh - структура скрипта изменилась сильнее, чем ожидает этот тест")
	}
	keys := map[string]bool{}
	for _, s := range starts {
		rest := script[s[1]:]
		end := strings.Index(rest, "\nEOF")
		if end == -1 {
			t.Fatal("не нашёл закрывающий EOF для heredoc-блока в scripts/init-env.sh")
		}
		body := rest[:end]
		for _, m := range initEnvAssignRe.FindAllStringSubmatch(body, -1) {
			keys[m[1]] = true
		}
	}
	return keys
}

// checkComposeValue проверяет, что значение в compose - это интерполяция
// ${ИМЯ...}, ссылается на само себя (не опечатка-копипаст с соседней строки),
// и что обязательность/дефолт согласованы с config.go.
func checkComposeValue(name string, hasDefault bool, def, raw string, skipDefaultCheck bool) error {
	m := composeInterpRe.FindStringSubmatch(raw)
	if m == nil {
		return fmt.Errorf("значение %q не в форме ${%s:-дефолт} - похоже на хардкод, .env его не переопределит", raw, name)
	}
	if m[1] != name {
		return fmt.Errorf("интерполяция ссылается на ${%s}, а не на ${%s} - похоже на опечатку при копировании соседней строки", m[1], name)
	}
	hasDefaultPart := m[2] != ""
	if !hasDefault {
		if hasDefaultPart {
			return fmt.Errorf("в config.go параметр обязательный (без envDefault), а в compose у него есть дефолт %q - обязательность теряется незаметно", m[3])
		}
		return nil
	}
	if !hasDefaultPart {
		return fmt.Errorf("в config.go есть envDefault %q, а в compose интерполяция без :- вовсе", def)
	}
	if skipDefaultCheck {
		return nil
	}
	if m[3] != def {
		return fmt.Errorf("дефолт в compose (%q) разошёлся с envDefault в config.go (%q)", m[3], def)
	}
	return nil
}

// TestEnvWiring_AllConfigVarsReachContainer - замок против класса #2027: параметр
// прописан в config.go, но не доходит до контейнера через docker-compose.base.yml
// и/или не пишется scripts/init-env.sh.
func TestEnvWiring_AllConfigVarsReachContainer(t *testing.T) {
	root := repoRoot(t)
	compose := readRepoFile(t, root, "docker-compose.base.yml")
	initEnv := readRepoFile(t, root, filepath.Join("scripts", "init-env.sh"))

	composeEnv := parseComposeBackendEnv(backendEnvironmentBlock(t, compose))
	initEnvKeys := initEnvGeneratedKeys(t, initEnv)

	fields := configEnvFields()

	// Исключение на переменную, которой больше нет в Config, - протухшая
	// запись: она никого больше не защищает и только засоряет карту.
	fieldNames := map[string]bool{}
	for _, f := range fields {
		fieldNames[f.name] = true
	}
	for name := range envWiringExceptions {
		if !fieldNames[name] {
			t.Errorf("envWiringExceptions[%q] ссылается на переменную, которой больше нет в Config - удали протухшую запись", name)
		}
	}
	for name, exc := range envWiringExceptions {
		if strings.TrimSpace(exc.reason) == "" {
			t.Errorf("envWiringExceptions[%q] без объяснения (reason) - так исключение не принимается", name)
		}
	}

	var missingCompose, missingInitEnv, badDefault []string
	for _, f := range fields {
		exc, hasExc := envWiringExceptions[f.name]

		if !hasExc || !exc.skipCompose {
			raw, ok := composeEnv[f.name]
			if !ok {
				missingCompose = append(missingCompose, f.name)
			} else if err := checkComposeValue(f.name, f.hasDefault, f.def, raw, hasExc && exc.skipDefaultCheck); err != nil {
				badDefault = append(badDefault, fmt.Sprintf("%s: %v", f.name, err))
			}
		}

		if !hasExc || !exc.skipInitEnv {
			if !initEnvKeys[f.name] {
				missingInitEnv = append(missingInitEnv, f.name)
			}
		}
	}

	if len(missingCompose) > 0 {
		t.Errorf("не пробрасываются в контейнер через docker-compose.base.yml (backend.environment): %s\n"+
			"добавь строку ${ИМЯ:-дефолт} с дефолтом = envDefault из internal/config/config.go, "+
			"или заведи исключение в envWiringExceptions с объяснением",
			strings.Join(missingCompose, ", "))
	}
	if len(missingInitEnv) > 0 {
		t.Errorf("не пишутся в .env генератором scripts/init-env.sh: %s\n"+
			"добавь строку ИМЯ=значение в один из heredoc-блоков записи $ENV_FILE, "+
			"или заведи исключение в envWiringExceptions с объяснением",
			strings.Join(missingInitEnv, ", "))
	}
	if len(badDefault) > 0 {
		t.Errorf("дефолт или форма значения в docker-compose.base.yml разошлись с config.go:\n%s",
			strings.Join(badDefault, "\n"))
	}
}
