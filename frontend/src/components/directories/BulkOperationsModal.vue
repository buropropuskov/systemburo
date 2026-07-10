<template>
  <BaseModal
    :show="show"
    :title="modalTitle"
    width="620px"
    content-class="bulk-op-modal"
    @close="$emit('close')"
  >
    <div
      class="bulk-op"
      data-testid="bulk-op-modal"
    >
      <p class="bulk-op__summary">
        Операция применится к <b>{{ selectedIds.length }}</b> {{ entityWord }}.
      </p>

      <!-- Тип -->
      <div
        v-if="operation === 'type'"
        class="bulk-op__section"
      >
        <label class="bulk-op__label">Новый тип</label>
        <BaseDropdown
          data-testid="bulk-op-type"
          :model-value="typeValue"
          :options="typeOptions"
          label-key="label"
          value-key="value"
          placeholder="Выберите тип"
          @update:model-value="typeValue = $event"
        />
      </div>

      <!-- Места разгрузки / целевые таблицы -->
      <div
        v-else-if="isGridOp"
        class="bulk-op__section"
      >
        <div class="bulk-op__mode">
          <span class="bulk-op__mode-label">Режим:</span>
          <div class="seg">
            <button
              type="button"
              class="seg__btn"
              :class="{ 'seg__btn--active': mode === 'replace' }"
              data-testid="bulk-op-mode-replace"
              @click="mode = 'replace'"
            >
              Заменить
            </button>
            <button
              type="button"
              class="seg__btn"
              :class="{ 'seg__btn--active': mode === 'add' }"
              data-testid="bulk-op-mode-add"
              @click="mode = 'add'"
            >
              Добавить
            </button>
          </div>
        </div>
        <p class="bulk-op__hint">
          {{ modeHint }}
        </p>
        <GridSelector
          :items="gridItems"
          :model-value="gridSelectedIds"
          item-key="id"
          item-label="name"
          :loading="loading"
          :error-message="loadError"
          @update:model-value="gridSelectedIds = $event"
        />
      </div>

      <!-- Ответственные -->
      <div
        v-else-if="operation === 'users'"
        class="bulk-op__section"
      >
        <div class="bulk-op__mode">
          <span class="bulk-op__mode-label">Режим:</span>
          <div class="seg">
            <button
              type="button"
              class="seg__btn"
              :class="{ 'seg__btn--active': mode === 'replace' }"
              data-testid="bulk-op-mode-replace"
              @click="mode = 'replace'"
            >
              Заменить
            </button>
            <button
              type="button"
              class="seg__btn"
              :class="{ 'seg__btn--active': mode === 'add' }"
              data-testid="bulk-op-mode-add"
              @click="mode = 'add'"
            >
              Добавить
            </button>
          </div>
        </div>
        <p class="bulk-op__hint">
          {{ modeHint }} Главного ответственного групповая операция не назначает.
        </p>

        <button
          type="button"
          class="bulk-op__pick"
          data-testid="bulk-op-pick-users"
          @click="userPickerOpen = true"
        >
          ＋ Выбрать пользователей
        </button>

        <div
          v-if="selectedUsers.length"
          class="bulk-op__chips"
        >
          <span
            v-for="u in selectedUsers"
            :key="u.username"
            class="bulk-op__chip"
          >
            {{ userLabel(u) }}
            <button
              type="button"
              class="bulk-op__chip-x"
              aria-label="Убрать"
              @click="removeUser(u)"
            >
              ×
            </button>
          </span>
        </div>
        <p
          v-else
          class="bulk-op__empty"
        >
          Никто не выбран
        </p>

        <label class="bulk-op__toggle">
          <ToggleSwitch
            v-model="requiredApproval"
            data-testid="bulk-op-required-approval"
          >
            Обязательное согласование для назначенных
          </ToggleSwitch>
        </label>
      </div>
    </div>

    <template #actions>
      <button
        class="bulk-op__cancel"
        data-testid="bulk-op-cancel"
        @click="$emit('close')"
      >
        Отмена
      </button>
      <button
        class="bulk-op__apply"
        data-testid="bulk-op-apply"
        :disabled="!canApply"
        @click="onApply"
      >
        {{ submitting ? 'Применение...' : `Применить (${selectedIds.length})` }}
      </button>
    </template>
  </BaseModal>

  <SelectionModal
    v-if="operation === 'users'"
    :show="userPickerOpen"
    title="Выбор ответственных"
    search-placeholder="Поиск пользователя..."
    :items="userPickerItems"
    :loading="loading"
    :disabled-ids="selectedUserIds"
    confirm-label="Выбрать"
    empty-text="Пользователи не найдены"
    @close="userPickerOpen = false"
    @confirm="onUsersConfirmed"
    @search="onUserSearch"
  >
    <template #columns>
      <div class="bulk-op__pick-col bulk-op__pick-col--name">
        Пользователь
      </div>
      <div class="bulk-op__pick-col">
        Должность
      </div>
    </template>
    <template #row="{ item }">
      <div class="bulk-op__pick-col bulk-op__pick-col--name">
        {{ userLabel(item) }}
        <span class="bulk-op__pick-username">@{{ item.username }}</span>
      </div>
      <div class="bulk-op__pick-col">
        {{ item.position || '—' }}
      </div>
    </template>
  </SelectionModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import GridSelector from '@/components/ui/GridSelector.vue';
import SelectionModal from '@/components/ui/SelectionModal.vue';
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue';
import { apiRequest } from '@/api/client';
import { ORG_TYPE_CREATE_OPTIONS } from '@/constants/orgTypes';
import { matchesSearch } from '@/utils/searchVariants';

// Сентинел «снять тип» в дропдауне: BaseDropdown с modelValue=null показывает
// плейсхолдер, поэтому «не указан» несём отдельным значением и разворачиваем в
// null на выходе (BE BulkTypeRequest.Type=nil снимает тип).
const NONE_TYPE = '__none__';

/**
 * Модалка групповой операции над справочником (организации/компании).
 * Собирает параметры операции и эмитит `apply` с payload; сам батч-API,
 * разбор BulkOpResult и refresh выполняет родитель (*Management). Архив и
 * восстановление идут через ConfirmationModal в родителе, здесь их нет.
 */
export default {
  name: 'BulkOperationsModal',
  components: { BaseModal, BaseDropdown, GridSelector, SelectionModal, ToggleSwitch },
  props: {
    show: { type: Boolean, default: false },
    entityType: {
      type: String,
      required: true,
      validator: (v) => ['organization', 'company'].includes(v),
    },
    selectedIds: { type: Array, required: true },
    // 'type' | 'unload-places' | 'tables' | 'users'
    operation: { type: String, default: '' },
    submitting: { type: Boolean, default: false },
  },
  emits: ['close', 'apply'],
  data() {
    return {
      typeValue: null,
      mode: 'replace',
      gridSelectedIds: [],
      selectedUsers: [],
      requiredApproval: false,
      userPickerOpen: false,
      userSearchVariants: [],
      allUnloadPlaces: [],
      allTables: [],
      allUsers: [],
      loading: false,
      loadError: '',
      typeOptions: [
        ...ORG_TYPE_CREATE_OPTIONS,
        { label: 'Снять тип (не указан)', value: NONE_TYPE },
      ],
    };
  },
  computed: {
    isGridOp() {
      return this.operation === 'unload-places' || this.operation === 'tables';
    },
    entityWord() {
      // родительный падеж мн.ч. после числа: «к N организациям/компаниям»
      return this.entityType === 'company' ? 'компаниям' : 'организациям';
    },
    modalTitle() {
      return {
        type: 'Сменить тип',
        'unload-places': 'Назначить места разгрузки',
        tables: 'Назначить целевые таблицы',
        users: 'Назначить ответственных',
      }[this.operation] || 'Групповая операция';
    },
    modeHint() {
      return this.mode === 'add'
        ? 'Добавить к текущим привязкам, не снимая существующие.'
        : 'Заменить текущие привязки выбранным набором.';
    },
    gridItems() {
      // GridSelector гейтит клик по status==='active'; места несут реальный
      // status, но таблицы (SystemTableWithDetails) — нет, поэтому нормализуем
      // всё к плоскому активному элементу (как одиночные пикеры, где кликается
      // любой элемент).
      if (this.operation === 'unload-places') {
        return this.allUnloadPlaces.map((p) => ({ id: p.id, name: p.name, status: 'active' }));
      }
      if (this.operation === 'tables') {
        return this.allTables
          .map((t) => (t.table ? t.table : t))
          .filter((t) => t.table_type !== 'cars')
          .map((t) => ({ id: t.id, name: t.display_name || t.name || 'Без названия', status: 'active' }));
      }
      return [];
    },
    selectedUserIds() {
      return this.selectedUsers.map((u) => u.id);
    },
    userPickerItems() {
      if (!this.userSearchVariants.length) return this.allUsers;
      return this.allUsers.filter((u) =>
        matchesSearch(`${this.userLabel(u)} ${u.username} ${u.position || ''}`, this.userSearchVariants)
      );
    },
    canApply() {
      if (this.submitting) return false;
      switch (this.operation) {
        case 'type':
          return this.typeValue !== null;
        case 'unload-places':
        case 'tables':
          return this.gridSelectedIds.length > 0;
        case 'users':
          return this.selectedUsers.length > 0;
        default:
          return false;
      }
    },
  },
  watch: {
    show(val) {
      if (val) this.resetAndLoad();
    },
    operation() {
      if (this.show) this.resetAndLoad();
    },
  },
  methods: {
    resetAndLoad() {
      this.typeValue = null;
      this.mode = 'replace';
      this.gridSelectedIds = [];
      this.selectedUsers = [];
      this.requiredApproval = false;
      this.userPickerOpen = false;
      this.userSearchVariants = [];
      this.loadError = '';
      if (this.operation === 'unload-places') this.fetchUnloadPlaces();
      else if (this.operation === 'tables') this.fetchTables();
      else if (this.operation === 'users') this.fetchUsers();
    },

    async fetchUnloadPlaces() {
      this.loading = true;
      this.loadError = '';
      try {
        const res = await apiRequest('/unload-places');
        if (!res.ok) throw new Error('bad status');
        const data = await res.json();
        this.allUnloadPlaces = Array.isArray(data) ? data : [];
      } catch {
        this.loadError = 'Не удалось загрузить места разгрузки';
        this.allUnloadPlaces = [];
      } finally {
        this.loading = false;
      }
    },

    async fetchTables() {
      this.loading = true;
      this.loadError = '';
      try {
        const res = await apiRequest('/system-tables');
        if (!res.ok) throw new Error('bad status');
        const data = await res.json();
        this.allTables = Array.isArray(data) ? data : [];
      } catch {
        this.loadError = 'Не удалось загрузить таблицы';
        this.allTables = [];
      } finally {
        this.loading = false;
      }
    },

    async fetchUsers() {
      this.loading = true;
      this.loadError = '';
      try {
        const res = await apiRequest('/users/all');
        if (!res.ok) throw new Error('bad status');
        const data = await res.json();
        this.allUsers = Array.isArray(data) ? data : [];
      } catch {
        this.loadError = 'Не удалось загрузить пользователей';
        this.allUsers = [];
      } finally {
        this.loading = false;
      }
    },

    userLabel(u) {
      const parts = [u.last_name, u.first_name, u.middle_name].filter(Boolean);
      return parts.join(' ') || u.username;
    },

    onUserSearch(variants) {
      this.userSearchVariants = variants || [];
    },

    onUsersConfirmed(users) {
      const existing = new Set(this.selectedUsers.map((u) => u.username));
      users.forEach((u) => {
        if (!existing.has(u.username)) this.selectedUsers.push(u);
      });
    },

    removeUser(u) {
      this.selectedUsers = this.selectedUsers.filter((x) => x.username !== u.username);
    },

    onApply() {
      if (!this.canApply) return;
      let payload;
      switch (this.operation) {
        case 'type':
          payload = { type: this.typeValue === NONE_TYPE ? null : this.typeValue };
          break;
        case 'unload-places':
          payload = { unloadPlaceIds: [...this.gridSelectedIds], mode: this.mode };
          break;
        case 'tables':
          payload = { tableIds: [...this.gridSelectedIds], mode: this.mode };
          break;
        case 'users':
          payload = {
            usernames: this.selectedUsers.map((u) => u.username),
            requiredApproval: this.requiredApproval,
            mode: this.mode,
          };
          break;
        default:
          return;
      }
      this.$emit('apply', payload);
    },
  },
};
</script>

<style scoped>
.bulk-op {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.bulk-op__summary {
  margin: 0;
  font-size: 14px;
  color: #5a6472;
}

.bulk-op__summary b {
  color: #4F5BDF;
}

.bulk-op__section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.bulk-op__label {
  font-size: 0.78em;
  color: #5a6472;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.bulk-op__mode {
  display: flex;
  align-items: center;
  gap: 12px;
}

.bulk-op__mode-label {
  font-size: 13px;
  color: #5a6472;
  font-weight: 600;
}

/* сегментированный переключатель Заменить/Добавить */
.seg {
  display: inline-flex;
  border: 1px solid #e6e6e6;
  border-radius: 50px;
  padding: 3px;
  background: #f5f6fa;
}

.seg__btn {
  border: none;
  background: transparent;
  border-radius: 50px;
  padding: 5px 16px;
  font-size: 13px;
  font-weight: 600;
  color: #5a6472;
  cursor: pointer;
  font-family: inherit;
  transition: background 0.2s, color 0.2s;
}

.seg__btn--active {
  background: #4F5BDF;
  color: #fff;
}

.bulk-op__hint {
  margin: 0;
  font-size: 12px;
  color: #a2a2a2;
}

.bulk-op__pick {
  align-self: flex-start;
  height: 36px;
  border: 1px dashed #d5d9e0;
  border-radius: 12px;
  padding: 0 16px;
  font-size: 13px;
  color: #4F5BDF;
  background: #fff;
  cursor: pointer;
  font-family: inherit;
  transition: border-color 0.2s;
}

.bulk-op__pick:hover {
  border-style: solid;
  border-color: #4F5BDF;
}

.bulk-op__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.bulk-op__chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 6px 0 12px;
  border-radius: 50px;
  background: #eef0ff;
  color: #4F5BDF;
  font-size: 12px;
  font-weight: 600;
}

.bulk-op__chip-x {
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: transparent;
  color: #4F5BDF;
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  border-radius: 50%;
}

.bulk-op__chip-x:hover {
  background: rgba(79, 91, 223, 0.15);
}

.bulk-op__empty {
  margin: 0;
  font-size: 13px;
  color: #a2a2a2;
  font-style: italic;
}

.bulk-op__toggle {
  display: flex;
  align-items: center;
  cursor: pointer;
}

.bulk-op__pick-col {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 6px;
}

.bulk-op__pick-col--name {
  flex: 2;
}

.bulk-op__pick-username {
  font-size: 11px;
  color: #a2a2a2;
}

/* кнопки действий (эталон SelectionModal actions) */
.bulk-op__cancel {
  padding: 10px 20px;
  border: 1px solid #e6e6e6;
  background: #fff;
  border-radius: 8px;
  cursor: pointer;
  font-size: 14px;
  transition: background 0.2s;
}

.bulk-op__cancel:hover {
  background: #f5f5f5;
}

.bulk-op__apply {
  padding: 10px 20px;
  background: #4F5BDF;
  color: #fff;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 14px;
  transition: opacity 0.2s;
}

.bulk-op__apply:hover:not(:disabled) {
  opacity: 0.9;
}

.bulk-op__apply:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
