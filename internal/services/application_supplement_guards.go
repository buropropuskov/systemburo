package services

// Защита состава вложения от строк дополнения, которое ещё не принято (#1685). Логику
// самого дополнения (подача, круг согласования, принятие) файл не содержит - только то,
// что должно стоять на её пути до появления: срез видимости для читателей состава и снятие
// открытого раунда с заявки, которая закрылась раньше него.

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// SupplementScope - чей взгляд на состав вложения обслуживает выборка (#1685).
//
// Дополнение приносит во вложение уже поданной заявки новых людей, машины и ТМЦ, и до
// принятия эти строки на КПП не допущены. Один и тот же состав читают две несовместимые
// аудитории: заявитель со своими согласующими и принимающим, которым непринятое видеть
// НАДО (автор только что его добавил и должен видеть, что оно на согласовании, а решать
// его - согласующему), и охрана, которой видеть его НЕЛЬЗЯ - в people-вложении едут серия
// и номер паспорта и номер патента. Одним предикатом на всех тут не обойтись.
//
// Отсюда обязательный параметр вместо второго набора методов вроде
// GetAttachmentCarsForSecurity: пара «широкий/узкий» соблазняет позвать короткое имя, и
// промах компилятор не заметит, а обязательный параметр заставляет каждый вызов назвать
// читателя явно - в security-пути в глаза бросается SupplementScopeAdmitted. Образец
// приёма - applicationArchiveChange в application_helpers.go.
//
// Нулевое значение намеренно узкое: забытый параметр и структура по умолчанию сужают
// выдачу до допущенного. Лишняя невидимая строка в карточке автора безобиднее непринятого
// человека в руках охраны.
type SupplementScope int

const (
	// SupplementScopeAdmitted - только допущенное на КПП: исходный состав подачи
	// (supplement_id IS NULL) и принятые дополнения. Этот срез уходит охране, в бланк
	// пропуска и в активацию строк.
	SupplementScopeAdmitted SupplementScope = iota
	// SupplementScopeAll - весь состав вложения, включая ещё не принятые дополнения.
	// Карточка заявки: автор, согласующие, принимающий.
	SupplementScopeAll
)

// admittedSupplementCond - SQL-условие «строка вложения допущена на КПП» для колонки
// supplement_id отношения rel (алиас запроса либо имя таблицы). Условие само по себе,
// без AND и скобок вокруг вызова - подставляется в WHERE соседним предикатом.
//
// rel приходит только литералом из кода этого пакета, снаружи в него ничего не попадает.
// Допущены строки трёх видов: исходный состав подачи (supplement_id пуст), принятый
// отдельный раунд и раунд, влитый в основной круг. Последний - не поблажка: заявка тогда
// ещё не была в работе, добавку согласовали вместе со всем составом, и своего принятия у
// неё нет и не будет. Без него такая строка не активировалась бы никогда: приём заявки в
// работу поднимает только допущенное, а перевести merged в accepted некому.
func admittedSupplementCond(rel string) string {
	return "(" + rel + ".supplement_id IS NULL OR EXISTS (" +
		"SELECT 1 FROM application_supplements sup WHERE sup.id = " + rel + ".supplement_id" +
		" AND sup.status IN ('" + models.SupplementAccepted + "', '" + models.SupplementMerged + "')))"
}

// supplementScopeWhere - кусок WHERE для запроса, читающего состав вложения: пустая строка
// для SupplementScopeAll, иначе " AND <условие допуска>". Fail-closed: любое значение,
// кроме явного SupplementScopeAll, сужает выборку.
func supplementScopeWhere(scope SupplementScope, rel string) string {
	if scope == SupplementScopeAll {
		return ""
	}
	return " AND " + admittedSupplementCond(rel)
}

// cancelOpenSupplements переводит открытые дополнения заявки в SupplementCancelled. Звать
// в той же транзакции, что и закрытие самой заявки.
//
// Круг согласования дополнения переживает смену статусов заявки намеренно: откат заявки на
// повторное согласование снял бы с КПП уже выданные пропуска. Но у закрытой заявки
// открытому раунду идти некуда - принимать его будет некому, а pending остался бы у
// согласующих вечной задачей и у автора вечным ожиданием решения.
//
// Открытым дополнение бывает максимум одно (партиальный уникальный индекс
// uidx_app_supplement_open), цикл - чтобы снятие не зависело от этого инварианта.
func (s *applicationService) cancelOpenSupplements(ctx context.Context, tx *gorm.DB, applicationID int) error {
	type openSupplement struct {
		ID     int
		Number int
		Status string
	}
	// FOR UPDATE обязателен, а не для порядка: без него параллельное решение принимающего
	// успевает закоммитить accepted (строки раунда уже активированы и стоят на КПП), после
	// чего этот UPDATE безусловно перебивал бы его на cancelled. Раунд с cancelled и
	// активными строками - худший из исходов: строки физически на посту, а все читатели
	// состава считают их непринятыми и прячут, поэтому бланк расходится с реальным допуском.
	// Под блокировкой SELECT перечитает строку после чужого коммита и такой раунд уже не
	// выберет. Порядок захвата тот же, что у всех вызывающих: сначала заявка, потом раунд.
	var open []openSupplement
	if err := tx.Raw(
		"SELECT id, number, status FROM application_supplements WHERE application_id = ? AND status IN ? FOR UPDATE",
		applicationID, models.OpenSupplementStatuses,
	).Scan(&open).Error; err != nil {
		slog.Error("Ошибка чтения открытых дополнений заявки", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load application supplements")
	}
	if len(open) == 0 {
		return nil
	}

	ids := make([]int, 0, len(open))
	for _, sup := range open {
		ids = append(ids, sup.ID)
	}
	// Условие по статусу повторяется и здесь: строки уже под блокировкой, но лишний предикат
	// стоит дёшево и не даёт снять раунд, который перестал быть открытым.
	if err := tx.Exec("UPDATE application_supplements SET status = ? WHERE id IN ? AND status IN ?",
		models.SupplementCancelled, ids, models.OpenSupplementStatuses).Error; err != nil {
		slog.Error("Ошибка снятия открытых дополнений заявки", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to cancel application supplements")
	}

	// Актор nil: раунд снял не человек, а закрытие заявки - история рисует "Система".
	for _, sup := range open {
		appID := applicationID
		oldStatus := sup.Status
		comment := fmt.Sprintf("Дополнение №%d снято: заявка закрыта", sup.Number)
		s.recorder.Log(ctx, tx, models.AuditEntityApplication, &appID,
			models.AuditActionSupplementCancelled, nil, applicationAuditDetails{
				OldValue: &oldStatus,
				NewValue: ptrString(models.SupplementCancelled),
				Comment:  &comment,
			})
	}
	return nil
}
