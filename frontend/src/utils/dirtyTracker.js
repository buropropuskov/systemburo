/**
 * Глобальный реестр форм с несохранёнными изменениями + красивый confirm-модал.
 *
 * Компонент с формой регистрирует геттер isDirty (и опционально getChanges) -
 * при попытке покинуть страницу (роутер, перезагрузка, переключение вкладок
 * внутри view) показывается красивый Vue-модал (DirtyConfirmModal в App.vue)
 * со списком изменений. Для window.beforeunload браузер показывает свой
 * нативный диалог - кастомный показать нельзя.
 *
 * Использование в Options API:
 *   import { registerDirtyTracker } from '@/utils/dirtyTracker';
 *   // короткая форма (как раньше):
 *   mounted() {
 *     this._stopGuard = registerDirtyTracker(() => this.isDirty);
 *   }
 *   // расширенная с описанием изменений:
 *   mounted() {
 *     this._stopGuard = registerDirtyTracker({
 *       isDirty: () => this.isDirty,
 *       getChanges: () => ['Изменена ширина "Марка"', 'Скрыта "Организация"'],
 *     });
 *   }
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
  changes: [],
  resolve: null,
});

/**
 * Регистрирует трекер. Аргумент:
 *  - функция: () => boolean (backward-compat, без списка изменений)
 *  - объект:  { isDirty: () => boolean, getChanges?: () => string[] }
 * Возвращает функцию-отписку.
 */
export function registerDirtyTracker(arg) {
  const entry = typeof arg === 'function'
    ? { isDirty: arg, getChanges: null }
    : { isDirty: arg?.isDirty, getChanges: arg?.getChanges ?? null };

  if (typeof entry.isDirty !== 'function') {
    throw new Error('registerDirtyTracker: isDirty must be a function');
  }

  const id = nextId++;
  trackers.set(id, entry);
  return () => trackers.delete(id);
}

/**
 * Есть ли хотя бы одна форма с несохранёнными изменениями?
 */
export function hasAnyDirty() {
  for (const entry of trackers.values()) {
    try {
      if (entry.isDirty()) return true;
    } catch {
      /* геттер мог обратиться к удалённому компоненту - игнорируем */
    }
  }
  return false;
}

/**
 * Собирает список изменений со всех dirty-трекеров. Элемент может быть строкой
 * (старый формат) или объектом { label, from, to } (новый формат - render со
 * svg-стрелкой в DirtyConfirmModal).
 */
function collectAllChanges() {
  const all = [];
  for (const entry of trackers.values()) {
    try {
      if (!entry.isDirty()) continue;
      if (!entry.getChanges) continue;
      const items = entry.getChanges();
      if (!Array.isArray(items)) continue;
      for (const it of items) {
        if (typeof it === 'string' && it.trim()) {
          all.push(it.trim());
        } else if (it && typeof it === 'object' && typeof it.label === 'string' && it.label.trim()) {
          all.push({
            label: it.label.trim(),
            from: it.from != null ? String(it.from) : '',
            to: it.to != null ? String(it.to) : '',
          });
        }
      }
    } catch {
      /* пропускаем сломанные трекеры */
    }
  }
  return all;
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
    confirmState.changes = collectAllChanges();
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
