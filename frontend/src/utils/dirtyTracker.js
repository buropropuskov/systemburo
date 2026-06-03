/**
 * Глобальный реестр форм с несохранёнными изменениями + красивый confirm-модал.
 *
 * Компонент с формой регистрирует геттер isDirty -> при попытке покинуть
 * страницу (роутер, перезагрузка, переключение вкладок внутри view) показывается
 * красивый Vue-модал (DirtyConfirmModal в App.vue). Для window.beforeunload
 * браузер показывает свой нативный диалог - кастомный показать нельзя.
 *
 * Использование в Options API:
 *   import { registerDirtyTracker } from '@/utils/dirtyTracker';
 *   mounted() { this._stopGuard = registerDirtyTracker(() => this.isDirty); },
 *   beforeUnmount() { this._stopGuard?.(); }
 *
 * Перед навигацией:
 *   if (!(await confirmIfAnyDirty())) return;
 */

import { reactive } from 'vue';

const trackers = new Map();
let nextId = 1;

const DEFAULT_MESSAGE = 'У вас есть несохранённые изменения. Покинуть страницу без сохранения?';

/**
 * Реактивный singleton-стейт для модалки подтверждения. DirtyConfirmModal
 * монтируется один раз в App.vue, подписан на этот стейт.
 */
export const confirmState = reactive({
  show: false,
  message: DEFAULT_MESSAGE,
  resolve: null,
});

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
 * Async. Если есть dirty-формы - показывает модал подтверждения, возвращает
 * Promise<boolean>. true = нет dirty либо пользователь подтвердил выход;
 * false = пользователь отменил.
 *
 * Если есть открытый ранее модал - он перебивается новым (предыдущий
 * Promise разрешается false, чтобы старый код не висел).
 */
export function confirmIfAnyDirty(message) {
  if (!hasAnyDirty()) return Promise.resolve(true);
  if (confirmState.resolve) {
    confirmState.resolve(false);
    confirmState.resolve = null;
  }
  return new Promise((resolve) => {
    confirmState.message = message || DEFAULT_MESSAGE;
    confirmState.resolve = resolve;
    confirmState.show = true;
  });
}

/**
 * Вызывается из DirtyConfirmModal кнопками "Подтвердить" / "Отмена".
 */
export function resolveDirtyConfirm(value) {
  if (confirmState.resolve) {
    confirmState.resolve(value);
    confirmState.resolve = null;
  }
  confirmState.show = false;
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
