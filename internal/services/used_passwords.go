package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// usedPasswordDepth - сколько последних паролей учётной записи проверяются на
// повтор и сколько записей за ней сохраняется.
//
// Десять, а не «все», потому что сравнение с перечнем - это не сравнение строк:
// у каждой записи своя соль, и проверить пароль против записи можно только
// вычислив Argon2id заново (m=19 МиБ, t=2 - около 100 мс на запись). Десять
// записей дают до секунды на самом плохом случае, когда совпадение находится
// последним; на нескольких десятках форма смены пароля начала бы думать
// секундами, а работник - жать кнопку второй раз.
const usedPasswordDepth = 10

// passwordGenerateAttempts - сколько раз плановая смена перепридумывает пароль,
// наткнувшись на совпадение с прежними. Совпадение случайного пароля с одним из
// десяти прошлых практически невероятно, но проверка уже сделана, и переген
// стоит дешевле разбора «почему человеку выслали пароль, который система же и
// запрещает».
const passwordGenerateAttempts = 5

// passwordReusedMessage - что видит человек, попытавшийся вернуть старый пароль.
// Текст не называет, каким по счёту был этот пароль: подсказка о содержимом
// перечня посторонним ни к чему.
const passwordReusedMessage = "Этот пароль уже использовался. Придумайте пароль, которым вы раньше не пользовались."

// errPasswordReused собирается заново на каждый отказ: echo.HTTPError -
// изменяемая структура, и общий экземпляр на весь процесс один вызывающий мог
// бы дополнить внутренней ошибкой в ущерб остальным.
func errPasswordReused() error {
	return echo.NewHTTPError(http.StatusBadRequest, passwordReusedMessage)
}

// passwordAlreadyUsed сообщает, встречался ли пароль среди последних
// usedPasswordDepth записей. db может быть как обычным дескриптором, так и
// транзакцией - проверка идёт только на чтение.
func passwordAlreadyUsed(ctx context.Context, db *gorm.DB, userID int, password string) (bool, error) {
	var hashes []string
	// Порядок с добавкой по id: у записей, созданных в одной транзакции, время
	// совпадает до микросекунд, и без второго ключа выборка «последних десяти»
	// зависела бы от того, как база вернёт строки.
	err := db.WithContext(ctx).Model(&models.UsedPassword{}).
		Where("user_id = ?", userID).
		Order("created_at DESC, id DESC").
		Limit(usedPasswordDepth).
		Pluck("password_hash", &hashes).Error
	if err != nil {
		return false, fmt.Errorf("чтение прежних паролей: %w", err)
	}
	for _, h := range hashes {
		matched, err := verifyPassword(h, password)
		if err != nil {
			// Одна из последних usedPasswordDepth записей повреждена - дефект
			// данных этой конкретной учётки, а не текущей попытки. Отказывать
			// в смене пароля из-за него - тот же грех, что и запирать вход по
			// нему (#2017), поэтому запись просто пропускаем как несовпавшую,
			// но громко логируем, чтобы дефект не потерялся молча.
			slog.Warn("не удалось разобрать сохранённый прежний пароль", "user_id", userID, "error", err)
			continue
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

// ensurePasswordNotReused - та же проверка, сразу в виде отказа для ручки.
func ensurePasswordNotReused(ctx context.Context, db *gorm.DB, userID int, password string) error {
	used, err := passwordAlreadyUsed(ctx, db, userID, password)
	if err != nil {
		return err
	}
	if used {
		return errPasswordReused()
	}
	return nil
}

// recordUsedPassword запоминает хеш нового пароля и обрезает хвост перечня.
// Вызывать ТОЛЬКО внутри той же транзакции, что меняет users.password: запись,
// сделанная отдельно, разъедется с действительностью на первом же сбое между
// двумя запросами - и человек либо потеряет запрет на пароль, который поставил,
// либо получит запрет на пароль, который у него не сохранился.
//
// Хвост сверх глубины удаляется здесь же, а не отдельной уборкой по расписанию:
// записи глубже десятой не читает никто, но это остаётся набор хешей
// действовавших когда-то паролей, и хранить их без единого потребителя незачем.
// Обрезка на записи держит таблицу в размере «десять строк на учётную запись»
// сама, без периодической задачи, за которой нужно следить.
func recordUsedPassword(ctx context.Context, tx *gorm.DB, userID int, passwordHash string) error {
	entry := models.UsedPassword{UserID: userID, PasswordHash: passwordHash}
	if err := tx.WithContext(ctx).Create(&entry).Error; err != nil {
		return fmt.Errorf("запись прежнего пароля: %w", err)
	}

	err := tx.WithContext(ctx).Exec(`
		DELETE FROM used_passwords
		WHERE user_id = ? AND id NOT IN (
			SELECT id FROM used_passwords
			WHERE user_id = ?
			ORDER BY created_at DESC, id DESC
			LIMIT ?
		)`, userID, userID, usedPasswordDepth).Error
	if err != nil {
		return fmt.Errorf("обрезка перечня прежних паролей: %w", err)
	}
	return nil
}
