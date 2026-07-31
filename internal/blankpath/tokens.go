// Package blankpath строит пути файлового архива бланков: раскладывает шаблон с
// плейсхолдерами в уровни каталогов и имя файла и приводит их к виду, который
// откроется по сети с Windows.
//
// Пакет чистый: ни базы, ни HTTP, ни моделей. Вызывающий собирает Values из заявки
// и вложения, пакет отвечает за подстановку, схлопывание пустых плейсхолдеров и
// безопасность имён.
package blankpath

import (
	"strconv"
	"strings"
	"time"
)

// Scope ограничивает, где плейсхолдер имеет смысл. Тип вложения нельзя подставить
// в имя папки: папка принадлежит заявке, а вложений разных типов у неё несколько,
// и одного "типа" у папки просто не существует.
type Scope uint8

const (
	// ScopeDir - плейсхолдер допустим в шаблоне каталогов.
	ScopeDir Scope = 1 << iota
	// ScopeFile - плейсхолдер допустим в шаблоне имени файла.
	ScopeFile
	// ScopeBoth - допустим везде.
	ScopeBoth = ScopeDir | ScopeFile
)

// Группы плейсхолдеров для палитры конструктора в интерфейсе.
const (
	GroupDate         = "Дата"
	GroupApplication  = "Заявка"
	GroupOrganization = "Организация"
	GroupAttachment   = "Вложение"
)

// Token - один плейсхолдер шаблона. Key указывается без фигурных скобок.
type Token struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Group   string `json:"group"`
	Example string `json:"example"`
	Scope   Scope  `json:"-"`
	// DirAllowed и FileAllowed дублируют Scope для интерфейса: фронт фильтрует
	// палитру по тому, какое из трёх полей настроек сейчас редактируется.
	DirAllowed  bool `json:"dir_allowed"`
	FileAllowed bool `json:"file_allowed"`
}

var tokens = []Token{
	{Key: "год", Label: "Год", Group: GroupDate, Example: "2026", Scope: ScopeBoth},
	{Key: "месяц_число", Label: "Месяц числом", Group: GroupDate, Example: "7", Scope: ScopeBoth},
	{Key: "месяц_2", Label: "Месяц двумя цифрами", Group: GroupDate, Example: "07", Scope: ScopeBoth},
	{Key: "МЕСЯЦ", Label: "Месяц прописными", Group: GroupDate, Example: "ИЮЛЬ", Scope: ScopeBoth},
	{Key: "месяц", Label: "Месяц строчными", Group: GroupDate, Example: "июль", Scope: ScopeBoth},
	{Key: "день", Label: "День", Group: GroupDate, Example: "31", Scope: ScopeBoth},
	{Key: "день_2", Label: "День двумя цифрами", Group: GroupDate, Example: "31", Scope: ScopeBoth},
	{Key: "дата", Label: "Дата целиком", Group: GroupDate, Example: "31.07.2026", Scope: ScopeBoth},

	{Key: "номер", Label: "Номер заявки", Group: GroupApplication, Example: "20260731-001", Scope: ScopeBoth},
	{Key: "id", Label: "Идентификатор заявки", Group: GroupApplication, Example: "4821", Scope: ScopeBoth},
	{Key: "заявитель", Label: "Заявитель (Фамилия И.О.)", Group: GroupApplication, Example: "Иванов И.И.", Scope: ScopeBoth},
	{Key: "инициатор", Label: "Инициатор заявки", Group: GroupApplication, Example: "Петров П.П.", Scope: ScopeBoth},
	{Key: "статус", Label: "Статус", Group: GroupApplication, Example: "Завершено", Scope: ScopeBoth},
	{Key: "согласование", Label: "Состояние согласования", Group: GroupApplication, Example: "Согласовано", Scope: ScopeBoth},

	{Key: "организация", Label: "Организация", Group: GroupOrganization, Example: "Мегобари", Scope: ScopeBoth},
	{Key: "компания", Label: "Компания", Group: GroupOrganization, Example: "ООО СтройГрупп", Scope: ScopeBoth},

	{Key: "тип", Label: "Тип вложения", Group: GroupAttachment, Example: "Заявка на работы", Scope: ScopeFile},
	{Key: "период", Label: "Период действия", Group: GroupAttachment, Example: "01.08.2026 - 05.08.2026", Scope: ScopeFile},
	{Key: "вложение_id", Label: "Идентификатор вложения", Group: GroupAttachment, Example: "9134", Scope: ScopeFile},
}

// Tokens возвращает копию реестра плейсхолдеров для палитры конструктора.
func Tokens() []Token {
	out := make([]Token, len(tokens))
	for i, t := range tokens {
		t.DirAllowed = t.Scope&ScopeDir != 0
		t.FileAllowed = t.Scope&ScopeFile != 0
		out[i] = t
	}
	return out
}

// TokenByKey ищет плейсхолдер по ключу без скобок.
func TokenByKey(key string) (Token, bool) {
	for _, t := range tokens {
		if t.Key == key {
			return t, true
		}
	}
	return Token{}, false
}

// Values - исходные данные для подстановки. Собирается вызывающим из заявки и
// вложения; пакет не знает ни про gorm, ни про модели.
type Values struct {
	// Date определяет и уровни год/месяц/день, и токен {дата}. Вызывающий обязан
	// привести её к рабочей таймзоне заранее: sending_datetime хранится в UTC, а
	// "31.07.2026" оператора - московская дата, и заявка после 21:00 МСК иначе
	// уедет в каталог следующего дня.
	Date time.Time

	// Number - номер заявки. Можно передавать как есть из базы ("№ 20260731/001"):
	// ведущий знак номера снимается здесь, чтобы шаблон "№{номер}" не давал "№№ ...".
	Number        string
	ApplicationID int
	Sender        string
	Initiator     string
	Status        string
	Confirmation  string

	Organization string
	Company      string

	// Поля вложения. Имеют смысл только в шаблоне имени файла.
	AttachmentType string
	Period         string
	AttachmentID   int
}

// lookup возвращает значение плейсхолдера и признак того, что ключ известен.
// Пустое значение - это не ошибка: пустые плейсхолдеры схлопываются при рендере.
func (v Values) lookup(key string) (string, bool) {
	switch key {
	case "год":
		return strconv.Itoa(v.Date.Year()), true
	case "месяц_число":
		return strconv.Itoa(int(v.Date.Month())), true
	case "месяц_2":
		return v.Date.Format("01"), true
	case "МЕСЯЦ":
		return MonthNameUpper(int(v.Date.Month())), true
	case "месяц":
		return MonthName(int(v.Date.Month())), true
	case "день":
		return strconv.Itoa(v.Date.Day()), true
	case "день_2":
		return v.Date.Format("02"), true
	case "дата":
		return v.Date.Format("02.01.2006"), true

	case "номер":
		return normalizeNumber(v.Number), true
	case "id":
		return positiveID(v.ApplicationID), true
	case "заявитель":
		return v.Sender, true
	case "инициатор":
		return v.Initiator, true
	case "статус":
		return v.Status, true
	case "согласование":
		return v.Confirmation, true

	case "организация":
		return v.Organization, true
	case "компания":
		return v.Company, true

	case "тип":
		return v.AttachmentType, true
	case "период":
		return v.Period, true
	case "вложение_id":
		return positiveID(v.AttachmentID), true
	}
	return "", false
}

// normalizeNumber снимает ведущий знак номера, чтобы шаблон "№{номер}" не удваивал его.
func normalizeNumber(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "№")
	s = strings.TrimPrefix(s, "#")
	return strings.TrimSpace(s)
}

// positiveID отдаёт идентификатор строкой; ноль и отрицательные считаются отсутствием
// значения, чтобы плейсхолдер схлопнулся вместо подстановки "0".
func positiveID(id int) string {
	if id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}
