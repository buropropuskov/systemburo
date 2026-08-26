package models

// Темы оформления (#1415). Список зеркалит реестр фронта
// (frontend/src/utils/theme.js) и блоки :root[data-theme] в assets/tokens.css:
// добавляя тему, править надо оба места плюс bootstrap-скрипт index.html.
const (
	ThemeLight = "light"
	ThemeDark  = "dark"
)

// DefaultTheme - оформление для тех, кто тему не выбирал (текущий светлый вид).
const DefaultTheme = ThemeLight

// ThemeIDs - допустимые значения User.Theme.
var ThemeIDs = []string{
	ThemeLight,
	ThemeDark,
}

// IsValidTheme сообщает, знает ли система такую тему. Неизвестное значение
// отклоняем на записи: в БД должны лежать только id, для которых есть палитра.
// У тех, кто успел выбрать снятую тему, в профиле остаётся старое значение -
// фронт схлопывает неизвестную тему в светлую при чтении.
func IsValidTheme(theme string) bool {
	for _, id := range ThemeIDs {
		if id == theme {
			return true
		}
	}
	return false
}
