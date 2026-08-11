package services

import (
	"fmt"
	"strings"

	"systemburo/internal/models"
)

// Коды шаблонов писем о паролях, которые отправляет сервис пользователей.
// Отдельно от кодов уведомлений: уведомления внутри системы об этих событиях не
// создаются - работник о них узнаёт как раз из письма.
const (
	MailTemplateAccountCreated     = "account_created"
	MailTemplatePasswordSetByAdmin = "password_set_by_admin"
)

// letterRule - строка разделителя. Письма читают в почтовых клиентах без разметки,
// поэтому структуру задают отступы и линейки, а не жирный шрифт.
const letterRule = "--------------------------------------------------"

// accountCreatedLetterBody - письмо о заведённой учётной записи.
//
// Логин и пароль вынесены в отдельный блок между линейками: их переписывают руками
// или копируют, и в сплошном тексте они теряются. Пароль стоит последним в блоке -
// глазу проще найти его в конце строки, чем посреди абзаца.
func accountCreatedLetterBody(u *models.User, password, baseURL string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Здравствуйте, %s.\n\n", addressee(*u))
	b.WriteString("Для вас заведена учётная запись в системе бюро пропусков.\n")
	b.WriteString("Через неё оформляют заявки на проход и проезд на территорию.\n\n")

	b.WriteString(letterRule + "\n")
	fmt.Fprintf(&b, "  Логин:   %s\n", u.Username)
	fmt.Fprintf(&b, "  Пароль:  %s\n", password)
	if baseURL != "" {
		fmt.Fprintf(&b, "  Адрес:   %s\n", baseURL)
	}
	b.WriteString(letterRule + "\n\n")

	b.WriteString("При первом входе система попросит задать свой пароль.\n")
	b.WriteString("Придумайте его сами и никому не сообщайте - пароль из этого письма\n")
	b.WriteString("после этого перестанет действовать.\n\n")

	b.WriteString(letterSignature())
	return b.String()
}

// passwordSetLetterBody - письмо о пароле, который задал администратор.
//
// Здесь важен не факт выдачи доступа, а то, что прежний пароль перестал работать:
// человек мог сидеть в системе и не понять, почему его выкинуло.
func passwordSetLetterBody(u *models.User, password, baseURL string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Здравствуйте, %s.\n\n", addressee(*u))
	b.WriteString("Бюро пропусков задало новый пароль для вашей учётной записи.\n")
	b.WriteString("Прежний пароль больше не действует, а начатые сеансы работы завершены.\n\n")

	b.WriteString(letterRule + "\n")
	fmt.Fprintf(&b, "  Логин:   %s\n", u.Username)
	fmt.Fprintf(&b, "  Пароль:  %s\n", password)
	if baseURL != "" {
		fmt.Fprintf(&b, "  Адрес:   %s\n", baseURL)
	}
	b.WriteString(letterRule + "\n\n")

	b.WriteString("При входе система попросит задать свой пароль. Придумайте его сами\n")
	b.WriteString("и никому не сообщайте - пароль из этого письма после этого\n")
	b.WriteString("перестанет действовать.\n\n")

	b.WriteString("Если вы не просили менять пароль, сообщите в бюро пропусков:\n")
	b.WriteString("возможно, доступ к вашей учётной записи получил кто-то ещё.\n\n")

	b.WriteString(letterSignature())
	return b.String()
}

// letterSignature - общий хвост писем о паролях.
func letterSignature() string {
	return "Отвечать на это письмо не нужно, оно отправлено автоматически.\n" +
		"По вопросам доступа обращайтесь в бюро пропусков.\n"
}
