package services

import (
	"fmt"
	"strings"

	"systemburo/internal/models"
)

// Коды шаблонов писем об учётной записи. Хранятся в очереди писем, по ним
// отбирают письма одного вида в отчёте и при повторной отправке.
//
// Код письма о заданном пароле берётся из кода уведомления намеренно - по тому
// же правилу, что и в прогонах по паролям: письмо и уведомление здесь два
// канала одного события, и общий код позволяет сопоставить их при разборе.
// Заведение учётной записи уведомления не порождает (получателя в системе ещё
// нет), поэтому у него свой код.
const (
	MailTemplateAccountCreated     = "account_created"
	MailTemplatePasswordSetByAdmin = NotificationTypePasswordChanged
)

// accountCreatedLetterBody собирает письмо о заведённой учётной записи. Пароль
// отдельной строкой и без знаков препинания вплотную - его переписывают руками.
func accountCreatedLetterBody(u models.User, password, baseURL string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Здравствуйте, %s.\n\n", addressee(u))
	b.WriteString("Для вас заведена учётная запись в системе бюро пропусков.\n\n")
	fmt.Fprintf(&b, "  Логин:  %s\n", u.Username)
	fmt.Fprintf(&b, "  Пароль: %s\n\n", password)
	if baseURL != "" {
		fmt.Fprintf(&b, "Адрес системы: %s\n\n", baseURL)
	}
	b.WriteString(firstLoginNotice)
	b.WriteString("\nЕсли вы не ждали этого письма, сообщите в бюро пропусков.\n")
	return b.String()
}

// passwordSetByAdminLetterBody собирает письмо о пароле, который задал
// администратор. От письма о заведении отличается поводом: учётная запись у
// человека уже есть, изменился только пароль.
func passwordSetByAdminLetterBody(u models.User, password, baseURL string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Здравствуйте, %s.\n\n", addressee(u))
	b.WriteString("Пароль вашей учётной записи в системе бюро пропусков изменён.\n\n")
	fmt.Fprintf(&b, "  Логин:  %s\n", u.Username)
	fmt.Fprintf(&b, "  Пароль: %s\n\n", password)
	if baseURL != "" {
		fmt.Fprintf(&b, "Адрес системы: %s\n\n", baseURL)
	}
	b.WriteString(firstLoginNotice)
	b.WriteString("\nЕсли пароль меняли не по вашей просьбе, сообщите в бюро пропусков.\n")
	return b.String()
}

// firstLoginNotice - общий абзац обоих писем. Признак обязательной смены
// поднимается в обоих случаях, поэтому и текст один: пароль пришёл открытым
// текстом и живёт в чужом почтовом ящике до первого входа.
const firstLoginNotice = "При первом входе система попросит задать свой пароль - придумайте его\n" +
	"сами и никому не сообщайте. Пароль из этого письма после этого\n" +
	"перестанет действовать.\n"
