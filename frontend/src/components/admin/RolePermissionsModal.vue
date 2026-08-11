<template>
  <Teleport to="body">
    <transition name="rpm-fade">
      <div
        v-if="show"
        class="rpm-overlay"
        data-testid="role-permissions-modal"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div
          class="rpm-modal"
          @mousedown.stop
        >
          <header class="rpm-head">
            <div>
              <h3 class="rpm-title">
                Права роли{{ roleName ? ` «${roleName}»` : '' }}
              </h3>
              <p class="rpm-sub">
                Точечных: {{ ownCount }} · из групп: {{ groupCount }} · всего: {{ effectiveCount }}
              </p>
            </div>
            <button
              class="rpm-close"
              aria-label="Закрыть"
              data-testid="role-permissions-close"
              @click="close"
            >
              <svg
                width="12"
                height="12"
                viewBox="0 0 14 14"
                fill="none"
              >
                <path
                  d="M13 1L1 13M1 1L13 13"
                  stroke="#666"
                  stroke-width="2"
                  stroke-linecap="round"
                />
              </svg>
            </button>
          </header>

          <div class="rpm-groups">
            <div class="rpm-groups__label">
              Группы прав
            </div>
            <p class="rpm-groups__hint">
              Права из выбранных групп добавляются к точечным. Снятие группы вернёт собственные права роли.
            </p>
            <div class="rpm-groups__list">
              <label
                v-for="g in groups"
                :key="g.id"
                class="rpm-group"
              >
                <input
                  type="checkbox"
                  class="checkbox"
                  :checked="selectedGroupIds.has(g.id)"
                  :data-group-id="g.id"
                  data-testid="role-perms-group"
                  @change="onToggleGroup(g.id)"
                >
                <span class="rpm-group__name">{{ g.name }}</span>
                <span class="rpm-group__count">{{ (g.keys || []).length }} прав</span>
              </label>
              <p
                v-if="!groups.length"
                class="rpm-groups__empty"
              >
                Групп пока нет.
              </p>
            </div>
          </div>

          <div class="rpm-search">
            <input
              v-model="search"
              class="lk-input"
              type="text"
              placeholder="Поиск права..."
              data-testid="role-permissions-search"
            >
          </div>

          <div class="rpm-body">
            <EffectivePermissionsTree
              :catalog="filteredCatalog"
              :state-by-key="stateByKey"
              :expand-all="searchActive"
              @toggle="onToggleKey"
            />
          </div>

          <footer class="rpm-foot">
            <span class="rpm-hint">Изменения вступят в силу в течение 30 секунд</span>
            <div class="rpm-foot__actions">
              <button
                class="lk-button lk-button--ghost"
                data-testid="role-permissions-cancel"
                @click="close"
              >
                Отмена
              </button>
              <button
                class="lk-button lk-button--primary"
                :disabled="saving"
                data-testid="role-permissions-save"
                @click="emitSave"
              >
                {{ saving ? 'Сохранение...' : 'Сохранить' }}
              </button>
            </div>
          </footer>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import EffectivePermissionsTree from './EffectivePermissionsTree.vue';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { filterCatalog, flattenCatalog, TABLE_KEY_PREFIX } from '@/utils/permissionCatalog';

/**
 * Единый редактор прав роли: дефолтные группы (чекбоксы) + собственные точечные
 * права (тумблер-дерево из каталога). Ключ, который уже даёт выбранная группа,
 * показан заблокированным с бейджем "группа" -- снять его можно только сняв группу.
 * Собственные гранты роли хранятся отдельно от групп: onToggleKey меняет только их,
 * поэтому добавление/снятие группы не затирает точечные права. Эмитит
 * save({ directKeys, groupIds }) -- две независимые сущности для раздельного
 * сохранения на бэке (PUT /permissions и PUT /default-groups).
 */
export default {
  name: 'RolePermissionsModal',
  components: { EffectivePermissionsTree },
  props: {
    show: { type: Boolean, required: true },
    roleName: { type: String, default: '' },
    catalog: { type: Array, default: () => [] },
    groups: { type: Array, default: () => [] },
    initialDirectKeys: { type: Array, default: () => [] },
    initialGroupIds: { type: Array, default: () => [] },
    saving: { type: Boolean, default: false },
  },
  emits: ['close', 'save'],
  setup(props, { emit }) {
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => emit('close'));
    return { onOverlayMousedown, onOverlayMouseup };
  },
  data() {
    return {
      selectedDirect: new Set(),
      selectedGroupIds: new Set(),
      search: '',
    };
  },
  computed: {
    flatCatalog() {
      return flattenCatalog(this.catalog);
    },
    searchActive() {
      return this.search.trim().length > 0;
    },
    catalogKeySet() {
      return new Set(this.flatCatalog.map((n) => n.key));
    },
    groupKeySet() {
      const set = new Set();
      for (const g of this.groups) {
        if (!this.selectedGroupIds.has(g.id)) continue;
        for (const k of g.keys || []) set.add(k);
      }
      return set;
    },
    stateByKey() {
      const result = {};
      for (const node of this.flatCatalog) {
        const superOnly = !!node.super_only;
        const inherited = this.groupKeySet.has(node.key);
        const own = this.selectedDirect.has(node.key);
        if (superOnly) {
          result[node.key] = { on: false, source: null, locked: true };
        } else if (inherited) {
          result[node.key] = { on: true, source: 'group', locked: true };
        } else if (own) {
          result[node.key] = { on: true, source: 'role', locked: false };
        } else {
          result[node.key] = { on: false, source: null, locked: false };
        }
      }
      return result;
    },
    ownCount() {
      return this.selectedDirect.size;
    },
    groupCount() {
      return this.groupKeySet.size;
    },
    effectiveCount() {
      return new Set([...this.selectedDirect, ...this.groupKeySet]).size;
    },
    filteredCatalog() {
      return filterCatalog(this.catalog, this.search);
    },
  },
  watch: {
    show: {
      immediate: true,
      handler(val) {
        if (val) {
          this.selectedDirect = new Set(this.initialDirectKeys || []);
          this.selectedGroupIds = new Set(this.initialGroupIds || []);
          this.search = '';
        }
      },
    },
  },
  mounted() {
    document.addEventListener('keydown', this.onKeydown);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKeydown);
  },
  methods: {
    onKeydown(e) {
      if (e.key === 'Escape' && this.show) this.close();
    },
    onToggleKey(key) {
      // Locked-ключи (super_only и наследованные от группы) сюда не приходят -- дерево
      // не эмитит toggle на них, поэтому меняются только собственные гранты роли.
      const next = new Set(this.selectedDirect);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      this.selectedDirect = next;
    },
    onToggleGroup(id) {
      const next = new Set(this.selectedGroupIds);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      this.selectedGroupIds = next;
    },
    emitSave() {
      // Ключ, которого нет в каталоге, не рендерится и не управляем из UI, но
      // причины отсутствия две, и обходятся они по-разному.
      // Статический осиротевший (право выпилили из каталога) -- отбрасываем:
      // бэкенд SetPermissions отбил бы весь запрос 400 на первом неизвестном.
      // table.* -- каталог прячет его, пока таблица в архиве или удалена (#1881); бэкенд
      // такой ключ принимает (IsValidKey пускает весь префикс table.), и он
      // обязан пережить сохранение: иначе первое же открытие роли молча снимет
      // права архивных таблиц, а восстановление таблицы их не вернёт.
      this.$emit('save', {
        directKeys: [...this.selectedDirect].filter(
          (k) => this.catalogKeySet.has(k) || k.startsWith(TABLE_KEY_PREFIX),
        ),
        groupIds: [...this.selectedGroupIds],
      });
    },
    close() {
      this.$emit('close');
    },
  },
};
</script>

<style scoped>
.rpm-overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1100;
  padding: 20px;
}

.rpm-modal {
  background: var(--surface);
  border-radius: 30px;
  width: 100%;
  max-width: 640px;
  max-height: calc(var(--app-vh, 1vh) * 88);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: var(--shadow-md);
}

.rpm-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 22px 24px 14px;
}

.rpm-title {
  margin: 0;
  font-size: 17px;
  font-weight: 600;
  color: var(--color-text);
}

.rpm-sub {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--color-text-muted);
}

.rpm-close {
  border: none;
  background: var(--color-bg-secondary);
  width: 32px;
  height: 32px;
  border-radius: 50%;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  transition: background 0.15s ease;
}

.rpm-close:hover {
  background: var(--color-border);
}

.rpm-groups {
  padding: 0 24px 12px;
}

.rpm-groups__label {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.07em;
  text-transform: uppercase;
  color: var(--color-text-muted);
  margin-bottom: 4px;
}

.rpm-groups__hint {
  margin: 0 0 8px;
  font-size: 12px;
  color: var(--color-text-muted);
}

.rpm-groups__list {
  max-height: 132px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px;
  background: var(--color-bg);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
}

.rpm-group {
  display: grid;
  grid-template-columns: 18px 1fr auto;
  align-items: center;
  gap: 10px;
  padding: 7px 10px;
  border-radius: 10px;
  cursor: pointer;
  font-size: 13px;
  transition: background 0.15s ease;
}

.rpm-group:hover {
  background: var(--color-primary-tint);
}

.rpm-group__name {
  font-weight: 500;
  color: var(--color-text);
}

.rpm-group__count {
  font-size: 11px;
  color: var(--color-text-muted);
}

.rpm-groups__empty {
  margin: 0;
  padding: 8px;
  font-size: 12px;
  color: var(--color-text-muted);
  font-style: italic;
}

.rpm-search {
  padding: 0 24px 12px;
}

.rpm-search .lk-input {
  width: 100%;
  box-sizing: border-box;
}

.rpm-body {
  padding: 4px 24px;
  overflow-y: auto;
  flex: 1;
}

.rpm-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 24px 22px;
  border-top: 1px solid var(--color-border);
}

.rpm-hint {
  font-size: 12px;
  color: var(--color-text-muted);
}

.rpm-foot__actions {
  display: flex;
  gap: 10px;
}

.rpm-fade-enter-active,
.rpm-fade-leave-active {
  transition: opacity 0.2s ease;
}

.rpm-fade-enter-active .rpm-modal,
.rpm-fade-leave-active .rpm-modal {
  transition: transform 0.2s ease, opacity 0.2s ease;
}

.rpm-fade-enter-from .rpm-modal,
.rpm-fade-leave-to .rpm-modal {
  opacity: 0;
  transform: translateY(12px) scale(0.98);
}

.rpm-fade-enter-from,
.rpm-fade-leave-to {
  opacity: 0;
}
</style>
