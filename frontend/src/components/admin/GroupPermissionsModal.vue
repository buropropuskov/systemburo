<template>
  <Teleport to="body">
    <transition name="gpm-fade">
      <div
        v-if="show"
        class="gpm-overlay"
        @click.self="close"
      >
        <div
          class="gpm-modal"
          data-testid="group-permissions-modal"
        >
          <header class="gpm-head">
            <div>
              <h3 class="gpm-title">
                {{ title }}
              </h3>
              <p class="gpm-sub">
                Выбрано прав: {{ selectedCount }}
              </p>
            </div>
            <button
              class="gpm-close"
              aria-label="Закрыть"
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

          <div class="gpm-search">
            <input
              v-model="search"
              class="lk-input"
              type="text"
              placeholder="Поиск права..."
              data-testid="group-permissions-search"
            >
          </div>

          <div class="gpm-body">
            <EffectivePermissionsTree
              :catalog="filteredCatalog"
              :state-by-key="stateByKey"
              :expand-all="searchActive"
              @toggle="onToggle"
            />
          </div>

          <footer class="gpm-foot">
            <span class="gpm-hint">Изменения вступят в силу в течение 30 секунд</span>
            <div class="gpm-foot__actions">
              <button
                class="lk-button lk-button--ghost"
                data-testid="group-permissions-cancel"
                @click="close"
              >
                Отмена
              </button>
              <button
                class="lk-button lk-button--primary"
                :disabled="saving"
                data-testid="group-permissions-save"
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
import { filterCatalog, flattenCatalog } from '@/utils/permissionCatalog';

/**
 * Редактор набора точечных прав группы на тумблер-дереве из каталога
 * (тот же стиль, что и карточка прав пользователя). super_only-права показываются
 * заблокированными -- их нельзя выдать через группу. Эмитит save(keys[]).
 */
export default {
  name: 'GroupPermissionsModal',
  components: { EffectivePermissionsTree },
  props: {
    show: { type: Boolean, required: true },
    title: { type: String, default: 'Права группы' },
    catalog: { type: Array, default: () => [] },
    initialKeys: { type: Array, default: () => [] },
    saving: { type: Boolean, default: false },
  },
  emits: ['close', 'save'],
  data() {
    return {
      selected: new Set(),
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
    stateByKey() {
      const result = {};
      for (const node of this.flatCatalog) {
        result[node.key] = {
          on: this.selected.has(node.key),
          source: null,
          locked: !!node.super_only,
        };
      }
      return result;
    },
    selectedCount() {
      return this.selected.size;
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
          this.selected = new Set(this.initialKeys || []);
          this.search = '';
        }
      },
    },
  },
  methods: {
    onToggle(key) {
      const next = new Set(this.selected);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      this.selected = next;
    },
    emitSave() {
      this.$emit('save', [...this.selected]);
    },
    close() {
      this.$emit('close');
    },
  },
};
</script>

<style scoped>
.gpm-overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1100;
  padding: 20px;
}

.gpm-modal {
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

.gpm-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 22px 24px 14px;
}

.gpm-title {
  margin: 0;
  font-size: 17px;
  font-weight: 600;
  color: var(--color-text);
}

.gpm-sub {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--color-text-muted);
}

.gpm-close {
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

.gpm-close:hover {
  background: var(--color-border);
}

.gpm-search {
  padding: 0 24px 12px;
}

.gpm-search .lk-input {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 10px 14px;
  font-size: 14px;
  outline: none;
  transition: box-shadow 0.15s ease, border-color 0.15s ease;
}

.gpm-search .lk-input:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.18);
}

.gpm-body {
  padding: 4px 24px;
  overflow-y: auto;
  flex: 1;
}

.gpm-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 24px 22px;
  border-top: 1px solid var(--color-border);
}

.gpm-hint {
  font-size: 12px;
  color: var(--color-text-muted);
}

.gpm-foot__actions {
  display: flex;
  gap: 10px;
}

.lk-button {
  border: none;
  border-radius: var(--radius-pill);
  padding: 10px 20px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s ease, opacity 0.15s ease;
}

.lk-button--primary {
  background: var(--color-primary);
  color: var(--accent-contrast);
}

.lk-button--primary:hover {
  background: var(--color-primary-hover);
}

.lk-button--primary:disabled {
  opacity: 0.6;
  cursor: default;
}

.lk-button--ghost {
  background: transparent;
  color: var(--color-text-muted);
}

.lk-button--ghost:hover {
  background: var(--color-bg-secondary);
}

.gpm-fade-enter-active,
.gpm-fade-leave-active {
  transition: opacity 0.2s ease;
}

.gpm-fade-enter-from,
.gpm-fade-leave-to {
  opacity: 0;
}
</style>
