import { usePermissionsStore } from '@/stores/permissions';

/**
 * v-permission-scope -- декларативная защита элемента/компонента (#187e).
 *
 * Использование:
 *   <button v-permission-scope="'action.delete.employee'">Удалить</button>
 *   <button v-permission-scope:disable="'action.delete.employee'">Удалить</button>
 *   <button v-permission-scope="{ key: 'action.delete.employee', mode: 'disable' }">Удалить</button>
 *
 * Modifiers:
 *   `:hide` (по умолчанию) -- элемент скрывается через display: none.
 *   `:disable` -- элемент остаётся видимым с pointer-events: none и opacity 0.4.
 *
 * Vite-plugin (build/vite-plugin-permissions.js) сканирует все *.vue
 * на v-permission-scope и собирает список используемых ключей в
 * frontend/src/generated/permission-keys.json.
 */
function resolve(binding) {
  let key;
  let mode = 'hide';
  if (typeof binding.value === 'string') {
    key = binding.value;
  } else if (binding.value && typeof binding.value === 'object') {
    key = binding.value.key;
    if (binding.value.mode) mode = binding.value.mode;
  }
  if (binding.arg === 'disable') mode = 'disable';
  if (binding.arg === 'hide') mode = 'hide';
  return { key, mode };
}

function apply(el, key, mode) {
  const store = usePermissionsStore();
  const allowed = !key || store.hasPermission(key);
  if (allowed) {
    el.style.display = el.dataset._psPrevDisplay || '';
    el.style.pointerEvents = '';
    el.style.opacity = '';
    el.removeAttribute('aria-disabled');
  } else if (mode === 'disable') {
    el.style.pointerEvents = 'none';
    el.style.opacity = '0.4';
    el.setAttribute('aria-disabled', 'true');
  } else {
    if (el.dataset._psPrevDisplay === undefined) {
      el.dataset._psPrevDisplay = el.style.display || '';
    }
    el.style.display = 'none';
  }
}

export const vPermissionScope = {
  mounted(el, binding) {
    const { key, mode } = resolve(binding);
    apply(el, key, mode);
  },
  updated(el, binding) {
    const { key, mode } = resolve(binding);
    apply(el, key, mode);
  },
};
