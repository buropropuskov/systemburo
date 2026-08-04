package fakedata

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"systemburo/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// InstanceKindKey -- ключ в system_settings, которым экземпляр объявлен проверочным
// стендом. Признака окружения в параметрах нет и вычислить его неоткуда: имя базы
// произвольное, а на стенд едет тот же образ, что на рабочий сервер. Поэтому отметка
// ставится явно и руками, один раз на экземпляр.
const InstanceKindKey = "instance.kind"

// InstanceKindStand -- значение отметки, при котором наливка разрешена.
const InstanceKindStand = "staging"

// ErrNotMarked -- экземпляр не отмечен как стенд.
var ErrNotMarked = errors.New("экземпляр не отмечен как проверочный стенд")

// GuardOptions -- как команда обходит отсутствующую отметку.
type GuardOptions struct {
	// ForceUnmarked разрешает наливку на неотмеченном экземпляре, но только вместе с
	// ConfirmDB. Одного флага мало: его легко дописать по привычке, а имя базы
	// приходится посмотреть и ввести, и это последняя остановка перед рабочим сервером.
	ForceUnmarked bool
	ConfirmDB     string
}

// InstanceKind читает отметку экземпляра. Пустая строка -- отметки нет.
func InstanceKind(ctx context.Context, db *gorm.DB) (string, error) {
	var setting models.SystemSetting
	err := db.WithContext(ctx).Where("key = ?", InstanceKindKey).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать отметку экземпляра: %w", err)
	}
	return strings.TrimSpace(setting.Value), nil
}

// MarkStand отмечает экземпляр как проверочный стенд.
func MarkStand(ctx context.Context, db *gorm.DB) error {
	setting := models.SystemSetting{
		Key:   InstanceKindKey,
		Value: InstanceKindStand,
		Type:  "string",
	}
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "type"}),
	}).Create(&setting).Error
	if err != nil {
		return fmt.Errorf("не удалось поставить отметку стенда: %w", err)
	}
	return nil
}

// EnsureStand пускает наливку только на отмеченном стенде либо на явно подтверждённом
// экземпляре. Возвращает ошибку с готовой инструкцией: её читает сотрудник
// подразделения ИТ, а не разработчик.
func EnsureStand(ctx context.Context, db *gorm.DB, dsn string, opts GuardOptions) error {
	kind, err := InstanceKind(ctx, db)
	if err != nil {
		return err
	}
	// Регистр не важен: значение правят и руками через панель управления базой, а отказ
	// из-за «Staging» с большой буквы толкает к -force-unmarked там, где обход не нужен.
	if strings.EqualFold(kind, InstanceKindStand) {
		return nil
	}

	dbName := DatabaseName(dsn)
	if !opts.ForceUnmarked {
		return fmt.Errorf("%w (база %q).\n\n"+
			"Если это действительно стенд, отметьте его один раз:\n"+
			"  server fake -mark-stand\n\n"+
			"Если это рабочий сервер, наливать вымышленные данные нельзя.",
			ErrNotMarked, dbName)
	}

	confirm := strings.TrimSpace(opts.ConfirmDB)
	if confirm == "" {
		return fmt.Errorf("обход отметки требует подтверждения имени базы: -confirm-db=%s", dbName)
	}
	if confirm != dbName {
		return fmt.Errorf("имя базы не совпадает: указано %q, подключение к %q.\n"+
			"Повторите с -confirm-db=%s, если наливать надо именно сюда.", confirm, dbName, dbName)
	}
	return nil
}

// DatabaseName достаёт имя базы из строки подключения. Разбирается только формат URL:
// config.Validate не пускает систему стартовать с DATABASE_URL другого вида, поэтому
// набор ключей со значениями сюда не доходит.
//
// Пустая строка -- разобрать не удалось. Вызывающий печатает её в подсказке, а сравнение
// с подтверждением пустое имя не пропускает, поэтому падать здесь не на чем.
func DatabaseName(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
		return ""
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Path, "/")
}
