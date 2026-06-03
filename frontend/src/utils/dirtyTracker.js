/**
 * Глобальный реестр форм с несохранёнными изменениями.
 *
 * Компонент с формой регистрирует геттер isDirty -> при попытке покинуть
 * страницу (роутер, перезагрузка, переключение вкладок внутри view) показывается
 * confirm. Если ни одной dirty-формы нет - переходы свободные.
 *
 * Использование в Options API:
 *   import { registerDirtyTracker } from '@/utils/dirtyTracker';
 *   mounted() { this._stopGuard = registerDirtyTracker(() => this.isDirty); },
 *   beforeUnmount() { this._stopGuard?.(); }
 */

const trackers = new Map();
let nextId = 1;

/**
 * Регистрирует геттер isDirty. Возвращает функцию-отписку.
 */
export function registerDirtyTracker(getterFn) {
  const id = nextId++;
  trackers.set(id, getterFn);
  return () => trackers.delete(id);
}

/**
 * Есть ли хотя бы одна форма с несохранёнными изменениями?
 */
export function hasAnyDirty() {
  for (const get of trackers.values()) {
    try {
      if (get()) return true;
    } catch {
      /* геттер мог обратиться к удалённому компоненту - игнорируем */
    }
  }
  return false;
}

/**
 * Если есть dirty-формы - спросить подтверждение. Возвращает true если можно
 * продолжать (нет dirty либо пользователь подтвердил).
 */
export function confirmIfAnyDirty(
  message = 'У вас есть несохранённые изменения. Покинуть страницу без сохранения?',
) {
  if (!hasAnyDirty()) return true;
  return window.confirm(message);
}

let installed = false;
function beforeUnloadHandler(event) {
  if (!hasAnyDirty()) return;
  // Браузеры показывают свой стандартный диалог при returnValue/preventDefault.
  // Текст игнорируется (защита от спама), но dialog появится.
  event.preventDefault();
  event.returnValue = '';
}

/**
 * Один раз на всё приложение подписывает window.beforeunload на проверку
 * dirty-форм. Безопасно вызывать многократно - повторные вызовы no-op.
 */
export function installBeforeUnloadGuard() {
  if (installed) return;
  installed = true;
  window.addEventListener('beforeunload', beforeUnloadHandler);
}
