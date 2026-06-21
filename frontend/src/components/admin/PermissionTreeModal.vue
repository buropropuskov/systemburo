<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="modal-overlay"
        @click.self="$emit('close')"
      >
        <div
          class="modal-content"
          data-testid="permission-tree-modal"
        >
          <header class="modal-content__header">
            <h3>{{ title }}</h3>
            <button
              class="close-btn"
              data-testid="permission-tree-close"
              @click="$emit('close')"
            >
              ×
            </button>
          </header>

          <div class="modal-content__body">
            <input
              v-model="search"
              type="text"
              placeholder="Поиск по ключу или описанию..."
              class="lk-input search-input"
              data-testid="permission-tree-search"
            >

            <div class="tree">
              <div
                v-for="group in filteredGroups"
                :key="group.prefix"
                class="tree-group"
                :data-testid="`permission-tree-group-${group.prefix.replace('.', '')}`"
              >
                <button
                  class="tree-group__header"
                  :class="{ 'tree-group__header--collapsed': collapsed[group.prefix] }"
                  :data-testid="`permission-tree-group-toggle-${group.prefix.replace('.', '')}`"
                  @click="toggleGroup(group.prefix)"
                >
                  <span class="tree-group__chevron">▾</span>
                  <span class="tree-group__title">{{ group.title }}</span>
                  <span class="tree-group__count">
                    {{ groupSelectedCount(group) }}/{{ group.keys.length }}
                  </span>
                  <span
                    class="tree-group__toggle-all"
                    @click.stop="toggleAllInGroup(group)"
                  >
                    {{ allInGroupSelected(group) ? 'Снять все' : 'Выбрать все' }}
                  </span>
                </button>
                <div
                  v-show="!collapsed[group.prefix]"
                  class="tree-group__items"
                >
                  <label
                    v-for="key in group.keys"
                    :key="key.value"
                    class="tree-item"
                    :class="{
                      'tree-item--changed': hasChanged(key.value)
                    }"
                    :data-testid="`permission-tree-key-${key.value}`"
                  >
                    <input
                      type="checkbox"
                      :checked="isSelected(key.value)"
                      @change="toggleKey(key.value)"
                    >
                    <span class="tree-item__key">{{ key.value }}</span>
                    <span
                      v-if="key.description"
                      class="tree-item__desc"
                    >
                      {{ key.description }}
                    </span>
                  </label>
                </div>
              </div>
            </div>
          </div>

          <footer class="modal-content__footer">
            <span
              v-if="changedCount > 0"
              class="changed-hint"
            >
              Несохранённых изменений: {{ changedCount }}
            </span>
            <button
              class="lk-button lk-button--ghost"
              data-testid="permission-tree-cancel"
              @click="$emit('close')"
            >
              Отмена
            </button>
            <button
              class="lk-button lk-button--primary"
              :disabled="changedCount === 0 || saving"
              data-testid="permission-tree-save"
              @click="save"
            >
              {{ saving ? 'Сохранение...' : 'Сохранить' }}
            </button>
          </footer>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
/**
 * PermissionTreeModal -- редактор набора прав.
 * @prop {boolean} show -- видимость модалки.
 * @prop {string} title -- заголовок (например, "Права группы XYZ").
 * @prop {string[]} initialKeys -- текущие выбранные ключи.
 * @prop {{value: string, description?: string}[]} availableKeys -- все возможные ключи.
 * @emits close -- закрытие без сохранения.
 * @emits save (selectedKeys: string[]) -- сохранение нового набора.
 */
const PREFIX_TITLES = {
  'page.': 'Страницы',
  'tab.': 'Вкладки',
  'component.': 'Компоненты',
  'action.': 'Действия',
  'entity.': 'Сущности (CRUD)',
  'table.': 'Таблицы (динамические)',
  'permission.': 'Управление правами и аудит',
};

export default {
  name: 'PermissionTreeModal',
  props: {
    show: { type: Boolean, default: false },
    title: { type: String, required: true },
    initialKeys: { type: Array, default: () => [] },
    availableKeys: { type: Array, default: () => [] },
  },
  emits: ['close', 'save'],
  data() {
    return {
      selected: new Set(),
      original: new Set(),
      search: '',
      collapsed: {},
      saving: false,
    };
  },
  computed: {
    groups() {
      const map = new Map();
      for (const k of this.availableKeys) {
        const value = typeof k === 'string' ? k : k.value;
        const desc = typeof k === 'string' ? null : k.description;
        const prefix = this.detectPrefix(value);
        if (!map.has(prefix)) {
          map.set(prefix, { prefix, title: PREFIX_TITLES[prefix] || prefix, keys: [] });
        }
        map.get(prefix).keys.push({ value, description: desc });
      }
      return Array.from(map.values()).sort((a, b) => a.title.localeCompare(b.title, 'ru'));
    },
    filteredGroups() {
      if (!this.search.trim()) return this.groups;
      const q = this.search.toLowerCase();
      return this.groups
        .map(g => ({
          ...g,
          keys: g.keys.filter(k => k.value.toLowerCase().includes(q) || (k.description || '').toLowerCase().includes(q)),
        }))
        .filter(g => g.keys.length > 0);
    },
    changedCount() {
      let count = 0;
      for (const k of this.selected) if (!this.original.has(k)) count++;
      for (const k of this.original) if (!this.selected.has(k)) count++;
      return count;
    },
  },
  watch: {
    show(v) {
      if (v) this.reset();
    },
    initialKeys() {
      this.reset();
    },
  },
  mounted() {
    this.reset();
  },
  methods: {
    reset() {
      this.selected = new Set(this.initialKeys);
      this.original = new Set(this.initialKeys);
      this.search = '';
      this.saving = false;
    },
    detectPrefix(key) {
      for (const p of Object.keys(PREFIX_TITLES)) {
        if (key.startsWith(p)) return p;
      }
      return 'other.';
    },
    isSelected(key) {
      return this.selected.has(key);
    },
    hasChanged(key) {
      return this.selected.has(key) !== this.original.has(key);
    },
    toggleKey(key) {
      if (this.selected.has(key)) this.selected.delete(key);
      else this.selected.add(key);
      this.selected = new Set(this.selected);
    },
    groupSelectedCount(group) {
      return group.keys.filter(k => this.selected.has(k.value)).length;
    },
    allInGroupSelected(group) {
      return group.keys.length > 0 && group.keys.every(k => this.selected.has(k.value));
    },
    toggleAllInGroup(group) {
      const all = this.allInGroupSelected(group);
      const next = new Set(this.selected);
      for (const k of group.keys) {
        if (all) next.delete(k.value);
        else next.add(k.value);
      }
      this.selected = next;
    },
    toggleGroup(prefix) {
      this.collapsed = { ...this.collapsed, [prefix]: !this.collapsed[prefix] };
    },
    async save() {
      this.saving = true;
      try {
        this.$emit('save', Array.from(this.selected));
      } finally {
        this.saving = false;
      }
    },
  },
};
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}

.modal-content {
  background: #fff;
  border-radius: var(--radius-lg);
  width: 100%;
  max-width: 720px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.2);
}

.modal-content__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--color-border);
}

.modal-content__header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  cursor: pointer;
  color: var(--color-text-muted);
  padding: 0 4px;
  line-height: 1;
}

.modal-content__body {
  padding: 16px 20px;
  overflow-y: auto;
  flex: 1;
}

.search-input {
  margin-bottom: 16px;
}

.tree {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tree-group {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.tree-group__header {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 10px 14px;
  background: var(--color-bg);
  border: none;
  cursor: pointer;
  font-family: inherit;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  text-align: left;
  transition: background 0.15s ease;
}

.tree-group__header:hover {
  background: #eef0ff;
}

.tree-group__chevron {
  transition: transform 0.15s ease;
  font-size: 10px;
  color: var(--color-text-muted);
}

.tree-group__header--collapsed .tree-group__chevron {
  transform: rotate(-90deg);
}

.tree-group__title {
  flex: 1;
}

.tree-group__count {
  font-weight: 400;
  color: var(--color-text-muted);
  font-size: 12px;
}

.tree-group__toggle-all {
  font-size: 11px;
  color: var(--color-primary);
  font-weight: 500;
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  cursor: pointer;
}

.tree-group__toggle-all:hover {
  background: #fff;
}

.tree-group__items {
  padding: 8px 14px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.tree-item {
  display: grid;
  grid-template-columns: 18px 1fr 2fr;
  gap: 10px;
  align-items: center;
  padding: 6px 6px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: 12px;
  transition: background 0.15s ease;
}

.tree-item:hover {
  background: var(--color-bg);
}

.tree-item--changed {
  background: #fef3c7;
}

.tree-item--changed:hover {
  background: #fde68a;
}

.tree-item__key {
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  color: var(--color-text);
}

.tree-item__desc {
  color: var(--color-text-muted);
  font-size: 11px;
}

.modal-content__footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding: 12px 20px;
  border-top: 1px solid var(--color-border);
}

.changed-hint {
  font-size: 12px;
  color: #92400e;
  margin-right: auto;
}

.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.25s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}
</style>
