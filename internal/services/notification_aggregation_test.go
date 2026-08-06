package services

import "testing"

// TestAutoNotificationGroupKey_ApplicationAnswer_UsesQuestionKey защищает правило
// схлопывания ответов по вопросу, не по заявке (#1748): у одного вопроса может быть
// несколько ответов, но application_id в payload тот же для всех вопросов заявки -
// группировка по нему схлопывала бы ответы на РАЗНЫЕ вопросы в одну запись.
func TestAutoNotificationGroupKey_ApplicationAnswer_UsesQuestionKey(t *testing.T) {
	t.Parallel()
	data := `{"application_id":10,"application_number":"№10","question_id":42}`
	key := autoNotificationGroupKey(NotificationTypeApplicationAnswer, &data)
	if key != "question:42" {
		t.Errorf("ожидался ключ question:42, получено %q", key)
	}
}

// TestAutoNotificationGroupKey_GenericAggregatable_UsesApplicationKey проверяет общее
// правило: любой другой Aggregatable-тип с application_id в data группируется по заявке.
func TestAutoNotificationGroupKey_GenericAggregatable_UsesApplicationKey(t *testing.T) {
	t.Parallel()
	// Напоминания о согласовании сюда не входят намеренно: у них свой интервал
	// повтора в днях, схлопывание им выключено в каталоге.
	cases := []string{
		NotificationTypeApplicationApprovalRequired,
		NotificationTypeApplicationForwarded,
		NotificationTypeApplicationQuestion,
		NotificationTypeDirectoryPending,
	}
	data := `{"application_id":7,"application_number":"№7"}`
	for _, notifType := range cases {
		key := autoNotificationGroupKey(notifType, &data)
		if key != "app:7" {
			t.Errorf("%s: ожидался ключ app:7, получено %q", notifType, key)
		}
	}
}

// TestAutoNotificationGroupKey_ApplicationAnswerWithoutQuestionID_NoKey - в вопросном
// payload есть и application_id, и question_id; для application_answer именно
// question_id обязателен, application_id как фолбэк использовать нельзя (иначе
// ответы на разные вопросы той же заявки схлопнулись бы в одну запись).
func TestAutoNotificationGroupKey_ApplicationAnswerWithoutQuestionID_NoKey(t *testing.T) {
	t.Parallel()
	data := `{"application_id":10,"application_number":"№10"}`
	key := autoNotificationGroupKey(NotificationTypeApplicationAnswer, &data)
	if key != "" {
		t.Errorf("ожидался пустой ключ без question_id, получено %q", key)
	}
}

// TestAutoNotificationGroupKey_NonAggregatable_Empty: тип не размечен Aggregatable в
// каталоге (application_created) - ключа нет вообще, даже если данные подходящие.
func TestAutoNotificationGroupKey_NonAggregatable_Empty(t *testing.T) {
	t.Parallel()
	data := `{"application_id":7}`
	key := autoNotificationGroupKey(NotificationTypeApplicationCreated, &data)
	if key != "" {
		t.Errorf("application_created не Aggregatable, ожидался пустой ключ, получено %q", key)
	}
}

// TestAutoNotificationGroupKey_NoMatchingField_Empty: Aggregatable-тип, но в data нет
// ни application_id, ни question_id - нормальная ветка "схлопывания нет", не ошибка.
func TestAutoNotificationGroupKey_NoMatchingField_Empty(t *testing.T) {
	t.Parallel()
	data := `{"foo":"bar"}`
	key := autoNotificationGroupKey(NotificationTypeApplicationApprovalRequired, &data)
	if key != "" {
		t.Errorf("ожидался пустой ключ без подходящего поля, получено %q", key)
	}
}

// TestAutoNotificationGroupKey_NilOrEmptyData_Empty: application_service.go создаёт
// NotificationTypeApplicationApprovalRequired с data=nil (без payload) - должно быть
// нормальной веткой без ключа, не паникой.
func TestAutoNotificationGroupKey_NilOrEmptyData_Empty(t *testing.T) {
	t.Parallel()
	if key := autoNotificationGroupKey(NotificationTypeApplicationApprovalRequired, nil); key != "" {
		t.Errorf("nil data: ожидался пустой ключ, получено %q", key)
	}
	empty := ""
	if key := autoNotificationGroupKey(NotificationTypeApplicationApprovalRequired, &empty); key != "" {
		t.Errorf("пустая строка data: ожидался пустой ключ, получено %q", key)
	}
}

// TestAutoNotificationGroupKey_InvalidJSON_Empty: битый JSON в data не должен ронять
// вызов - схлопывание просто не срабатывает.
func TestAutoNotificationGroupKey_InvalidJSON_Empty(t *testing.T) {
	t.Parallel()
	broken := `{not json`
	key := autoNotificationGroupKey(NotificationTypeApplicationApprovalRequired, &broken)
	if key != "" {
		t.Errorf("битый JSON: ожидался пустой ключ, получено %q", key)
	}
}

// TestAutoNotificationGroupKey_UnknownType_Empty: код вне каталога не должен падать -
// нормальная ветка "схлопывания нет" (сама доставка уведомления неизвестного типа
// решается отдельно в notificationAllowed).
func TestAutoNotificationGroupKey_UnknownType_Empty(t *testing.T) {
	t.Parallel()
	data := `{"application_id":7}`
	key := autoNotificationGroupKey("submit", &data)
	if key != "" {
		t.Errorf("тип вне каталога: ожидался пустой ключ, получено %q", key)
	}
}
