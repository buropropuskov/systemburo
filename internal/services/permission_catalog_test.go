package services

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestCatalogNoDuplicateKeys(t *testing.T) {
	t.Parallel()
	seen := make(map[string]struct{})
	for _, n := range Catalog() {
		if _, dup := seen[n.Key]; dup {
			t.Errorf("дубликат ключа в каталоге: %s", n.Key)
		}
		seen[n.Key] = struct{}{}
		if n.Key == "" || n.DisplayName == "" || n.Category == "" {
			t.Errorf("узел каталога с пустым полем: %+v", n)
		}
	}
	if len(seen) == 0 {
		t.Fatal("каталог пуст")
	}
}

func TestIsSuperOnly(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		KeyPageSystemControl: true,
		KeyActionGrantAdmin:  true,
		KeyPageCenter:        false,
		KeyPageAdminUsers:    false,
		"table.kpp4.view":    false,
	}
	for key, want := range cases {
		if got := IsSuperOnly(key); got != want {
			t.Errorf("IsSuperOnly(%q) = %v, ожидалось %v", key, got, want)
		}
	}
}

func TestSuperOnlyKeysPresentInCatalog(t *testing.T) {
	t.Parallel()
	for k := range superOnlyKeys {
		if !IsCatalogKey(k) {
			t.Errorf("super-only ключ %q отсутствует в каталоге", k)
		}
	}
}

// SuperOnlyKeys нужен ответу /permissions/my (#1997), чтобы super-only ключи
// попадали в denied для обычного admin.
func TestSuperOnlyKeys(t *testing.T) {
	t.Parallel()
	got := SuperOnlyKeys()
	if len(got) != len(superOnlyKeys) {
		t.Fatalf("SuperOnlyKeys() вернул %d ключей, ожидалось %d", len(got), len(superOnlyKeys))
	}
	for _, k := range got {
		if !IsSuperOnly(k) {
			t.Errorf("SuperOnlyKeys() вернул %q, который IsSuperOnly отрицает", k)
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Errorf("SuperOnlyKeys() не отсортирован: %v", got)
		}
	}
}

func TestIsCatalogKey(t *testing.T) {
	t.Parallel()
	if !IsCatalogKey(KeyPageCenter) {
		t.Error("KeyPageCenter должен быть в каталоге")
	}
	if !IsCatalogKey(KeyDetailFullHistory) {
		t.Error("KeyDetailFullHistory должен быть в каталоге")
	}
	if IsCatalogKey("detail.blacklist") {
		t.Error("detail.blacklist убран из каталога (ЧС гейтится page.admin.blacklist)")
	}
	if IsCatalogKey("nonexistent.key") {
		t.Error("несуществующий ключ не должен быть в каталоге")
	}
	if IsCatalogKey("table.kpp4.view") {
		t.Error("динамический table.* не входит в статический каталог")
	}
}

func TestIsValidKey(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		KeyPageCenter:        true,
		KeyDetailDocuments:   true,
		"table.kpp4.view":    true, // динамический префикс
		"table.post72.entry": true,
		"garbage":            false,
		"page.unknown":       false,
	}
	for key, want := range cases {
		if got := IsValidKey(key); got != want {
			t.Errorf("IsValidKey(%q) = %v, ожидалось %v", key, got, want)
		}
	}
}

// Гигиена каталога (#permission-gating срез 1): «Аналитика» гейтится реально
// enforced ключом page.statistics, а не orphan page.analytics; управление новостями
// живёт в Администрировании (/admin/news), отдельного тумблера news.manage на
// «Обзор и новости» нет.
func TestCatalogHygiene(t *testing.T) {
	t.Parallel()
	if !IsCatalogKey(KeyPageStatistics) {
		t.Error("Аналитика должна гейтиться page.statistics (ключ обязан быть в каталоге)")
	}
	if IsCatalogKey("page.analytics") {
		t.Error("orphan page.analytics не должен остаться в каталоге")
	}
	if IsCatalogKey("news.manage") {
		t.Error("news.manage убран с «Обзор и новости» (управление новостями -- в Администрировании)")
	}
}

// Гейтинг руководства (#permission-gating срез 7): все три раздела
// (user/guard/admin) гейтятся по guide.<role>, поэтому каждый ключ обязан быть
// тумблером каталога -- иначе раздел enforced на бэке, но неуправляем (guide.guard
// был именно таким до этого среза).
func TestGuideKeysInCatalog(t *testing.T) {
	t.Parallel()
	for _, role := range GuideRoles {
		key := GuideKeyForRole(role)
		if !IsCatalogKey(key) {
			t.Errorf("ключ руководства %q должен быть в каталоге (раздел %q)", key, role)
		}
	}
}

// TestCatalogMeta фиксирует единый SoT каталога (#887): метаданные каталожного
// ключа берутся из Go-каталога; динамические table.* и мусор метаданных не имеют.
func TestCatalogMeta(t *testing.T) {
	t.Parallel()
	meta, ok := CatalogMeta(KeyPageCenter)
	if !ok {
		t.Fatal("page.center должен быть в каталоге")
	}
	if meta.DisplayName != "Центр заявок" || meta.Category != CatNavigation {
		t.Errorf("неожиданные метаданные page.center: %+v", meta)
	}
	if _, ok := CatalogMeta("table.kpp4.view"); ok {
		t.Error("динамический table.* не должен иметь метаданных в статическом каталоге")
	}
	if _, ok := CatalogMeta("garbage"); ok {
		t.Error("мусорный ключ не должен иметь метаданных")
	}
}

func TestAllCatalogKeysMatchesSet(t *testing.T) {
	t.Parallel()
	keys := AllCatalogKeys()
	if len(keys) == 0 {
		t.Fatal("AllCatalogKeys пуст")
	}
	for _, k := range keys {
		if !IsCatalogKey(k) {
			t.Errorf("AllCatalogKeys вернул ключ %q вне каталога", k)
		}
	}
}

// TestManualAddKeysInCatalog фиксирует ключи ручного добавления в таблицы (#1049):
// право должно быть валидным для назначения роли (IsValidKey), присутствовать в
// каталоге с метаданными категории «Сотрудники и автомобили» и не быть super-only
// -- иначе кнопку «Добавить вручную» нельзя выдать роли/группе.
func TestManualAddKeysInCatalog(t *testing.T) {
	t.Parallel()
	for _, key := range []string{KeyEntityCarsManualAdd, KeyEntityEmployeesManualAdd} {
		if !IsValidKey(key) {
			t.Errorf("ключ %q должен быть валиден для назначения роли", key)
		}
		meta, ok := CatalogMeta(key)
		if !ok {
			t.Errorf("ключ %q должен быть в каталоге с метаданными", key)
			continue
		}
		if meta.Category != CatRegistry {
			t.Errorf("ключ %q: категория %q, ожидалась %q", key, meta.Category, CatRegistry)
		}
		if meta.DisplayName == "" {
			t.Errorf("ключ %q: пустой DisplayName", key)
		}
		if IsSuperOnly(key) {
			t.Errorf("ключ %q не должен быть super-only (super/admin проходят через allowAll)", key)
		}
	}
}

// TestPageAdminNotUmbrellaName фиксирует #1998: после переезда справочников
// (#1982) page.admin больше не открывает раздел администрирования целиком --
// за ним остались два пункта меню («Руководство», «Обработка данных») и
// россыпь действий по системе. Название "Администрирование" (как у самой
// категории CatAdmin) вводило раздающего права в заблуждение: по нему решишь,
// что выдаёшь весь раздел, а получишь два пункта меню. Замок стережёт, что
// название переименовано и объяснено, а не просто выдаёт пустое описание.
func TestPageAdminNotUmbrellaName(t *testing.T) {
	t.Parallel()
	meta, ok := CatalogMeta(KeyPageAdmin)
	if !ok {
		t.Fatal("page.admin должен быть в каталоге")
	}
	if meta.DisplayName == "Администрирование" || meta.DisplayName == CatAdmin {
		t.Errorf("page.admin: название %q совпадает со старым зонтичным именем раздела администрирования", meta.DisplayName)
	}
	if meta.Description == "" {
		t.Error("page.admin: нужно описание -- название короткое и само по себе не объясняет узкий состав права")
	}
	if !strings.Contains(meta.Description, "Руководство") || !strings.Contains(meta.Description, "Обработка данных") {
		t.Errorf("page.admin: описание должно называть оба пункта меню, которые оно открывает: %q", meta.Description)
	}
	if !strings.Contains(meta.Description, KeyPageAdminDirectories) {
		t.Errorf("page.admin: описание должно явно отделять его от справочников (%s), иначе путаница вернётся", KeyPageAdminDirectories)
	}
	// Ключ и его категория (навигация) не меняются -- дробление отклонено
	// владельцем, чтобы сохранить обратную совместимость с уже выданными правами.
	if meta.Category != CatNavigation {
		t.Errorf("page.admin: категория изменилась на %q, ожидалась %q (правка #1998 - только текст)", meta.Category, CatNavigation)
	}
}

// tourCoverageEntry -- решение по одному элементу системы: либо ссылка на шаг
// онбординг-тура, либо мотивированный отказ.
type tourCoverageEntry struct {
	Tour string `json:"tour"`
	Step string `json:"step"`
	Skip string `json:"skip"`
}

type tourCoverageFile struct {
	Permissions map[string]tourCoverageEntry `json:"permissions"`
}

// tourCoveragePath -- реестр покрытия туров живёт на фронтенде: его основной
// потребитель -- vitest-замок по роутам и навигации, и держать две копии значило
// бы получить расхождение. Кросс-язычная сверка в проекте уже применяется
// (doc_facts.py читает Go ради сверки документации), так что направление обратное,
// но приём тот же.
const tourCoveragePath = "../../frontend/src/components/onboarding/tourCoverage.json"

func loadTourCoverage(t *testing.T) tourCoverageFile {
	t.Helper()
	raw, err := os.ReadFile(tourCoveragePath)
	if err != nil {
		// Пропавший реестр -- не повод молча позеленеть: замок держит правило
		// «завёл право -- реши про обучение», и без файла правила нет.
		t.Fatalf("не прочитан реестр покрытия туров %s: %v", tourCoveragePath, err)
	}
	var file tourCoverageFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("реестр покрытия туров невалиден: %v", err)
	}
	if len(file.Permissions) == 0 {
		t.Fatal("в реестре покрытия туров нет раздела permissions")
	}
	return file
}

// TestCatalogKeysCoveredByTourRegistry -- замок «завёл право, реши про обучение».
// Каждый ключ статического каталога обязан иметь запись в реестре покрытия туров:
// либо шаг, который его показывает, либо причину, по которой в тур он не идёт.
// Отклонить -- нормальный исход, промолчать -- нет: именно молчание за год
// оставило тур без сквозного поиска, вопросов к заявке и дополнения (эпик #1736).
//
// Существование самого шага проверяет vitest-замок (tourCoverage.spec.js): шаги
// описаны в JS, и резолвить их отсюда пришлось бы вторым парсером.
func TestCatalogKeysCoveredByTourRegistry(t *testing.T) {
	t.Parallel()
	coverage := loadTourCoverage(t)

	for _, key := range AllCatalogKeys() {
		entry, ok := coverage.Permissions[key]
		if !ok {
			t.Errorf("ключ каталога %q без записи в %s: добавьте {\"tour\":..., \"step\":...} либо {\"skip\":\"причина\"}",
				key, tourCoveragePath)
			continue
		}
		switch {
		case entry.Tour != "" && entry.Step == "":
			t.Errorf("ключ %q: указан тур %q без шага", key, entry.Tour)
		case entry.Tour == "" && entry.Skip == "":
			t.Errorf("ключ %q: пустая запись, нужен либо шаг тура, либо skip с причиной", key)
		case entry.Tour != "" && entry.Skip != "":
			t.Errorf("ключ %q: одновременно шаг тура и skip", key)
		}
	}
}

// TestTourRegistryHasNoStalePermissionKeys ловит обратную рассинхронизацию:
// право переименовали или убрали, а запись о нём осталась и делает вид, что
// элемент обучением покрыт.
func TestTourRegistryHasNoStalePermissionKeys(t *testing.T) {
	t.Parallel()
	for key := range loadTourCoverage(t).Permissions {
		if !IsCatalogKey(key) {
			t.Errorf("запись %q в %s не соответствует ни одному ключу каталога", key, tourCoveragePath)
		}
	}
}
