/**
 * Реестр тем оформления (#1415) - единственный источник правды по списку тем.
 *
 * Палитры лежат в assets/tokens.css блоками :root[data-theme="<id>"], бэкенд
 * валидирует тот же список (internal/models/theme.go), а bootstrap-скрипт в
 * index.html дублирует id, чтобы поставить тему до первого кадра (иначе при
 * загрузке мелькнёт светлая). Дубль закреплён тестом __tests__/theme.spec.js -
 * добавляя тему, править надо все четыре места.
 */

/** Ключ localStorage: немедленный применитель, источник правды - профиль на бэке. */
export const THEME_STORAGE_KEY = 'app-theme';

/** Тема по умолчанию: текущее (светлое) оформление системы. */
export const DEFAULT_THEME = 'light';

/**
 * @typedef {{ id: string, name: string, dot: string }} ThemeOption
 * dot - цвет кружка в переключателе: акцент светлых тем, фон тёмных (по нему
 * тема и узнаётся в списке).
 */

/** @type {ThemeOption[]} */
export const THEMES = [
  { id: 'light', name: 'Светлая', dot: '#4F5BDF' },
  { id: 'dark', name: 'Тёмная', dot: '#14161c' },
  { id: 'corporate-orange', name: 'Корпоративная', dot: '#F97316' },
  { id: 'business-graphite', name: 'Деловая', dot: '#4A5568' },
  { id: 'official-blue', name: 'Официальная', dot: '#1B4F9C' },
  { id: 'dark-orange', name: 'Тёмная оранжевая', dot: '#FFA033' },
];

const THEME_IDS = THEMES.map((t) => t.id);

/**
 * @param {unknown} id
 * @returns {boolean} известна ли тема реестру
 */
export function isValidTheme(id) {
  return typeof id === 'string' && THEME_IDS.includes(id);
}

/**
 * @param {string} id
 * @returns {ThemeOption|null}
 */
export function findTheme(id) {
  return THEMES.find((t) => t.id === id) || null;
}

/**
 * Ставит data-theme на <html>. Неизвестное значение схлопывается в светлую:
 * в tokens.css её переменные лежат и на голом :root, так что интерфейс
 * остаётся рабочим при любом мусоре в хранилище.
 *
 * @param {string} id
 * @returns {string} реально применённая тема
 */
export function applyTheme(id) {
  const theme = isValidTheme(id) ? id : DEFAULT_THEME;
  document.documentElement.setAttribute('data-theme', theme);
  return theme;
}

/**
 * @returns {string|null} тема из localStorage; null если пусто, мусор или
 * хранилище недоступно (приватный режим)
 */
export function readStoredTheme() {
  try {
    const stored = localStorage.getItem(THEME_STORAGE_KEY);
    return isValidTheme(stored) ? stored : null;
  } catch {
    return null;
  }
}

/**
 * Персист выбора. Best-effort: недоступный localStorage не должен ронять
 * переключение - тема уже применена к DOM и сохранится в профиле на бэке.
 *
 * @param {string} id
 */
export function storeTheme(id) {
  try {
    localStorage.setItem(THEME_STORAGE_KEY, id);
  } catch {
    // Приватный режим/переполненное хранилище - персист не критичен.
  }
}
