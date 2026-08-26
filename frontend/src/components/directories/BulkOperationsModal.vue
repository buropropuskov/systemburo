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
          teleport
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

        <SelectUnloadPlaces
          v-if="operation === 'unload-places'"
          v-model="placeIds"
          selection-mode
          :entity-type="entityType"
        />
        <SelectTables
          v-else
          v-model="tableIds"
          selection-mode
          :entity-type="entityType"
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
          {{ modeHint }} Обязательное согласование задаётся на каждого отдельно. Главного ответственного групповая операция не назначает.
        </p>

        <ResponsibleUsersSection
          v-model="responsibleUsers"
          selection-mode
          :entity-type="entityType"
        />
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
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import SelectUnloadPlaces from '@/components/SelectUnloadPlaces.vue';
import SelectTables from '@/components/SelectTables.vue';
import ResponsibleUsersSection from '@/components/ResponsibleUsersSection.vue';
import { ORG_TYPE_CREATE_OPTIONS } from '@/constants/orgTypes';

// Сентинел «снять тип» в дропдауне: BaseDropdown с modelValue=null показывает
// плейсхолдер, поэтому «не указан» несём отдельным значением и разворачиваем в
// null на выходе (BE BulkTypeRequest.Type=nil снимает тип).
const NONE_TYPE = '__none__';

/**
 * Модалка групповой операции над справочником (организации/компании).
 * Переиспользует эталонные секции выбора сайта (SelectUnloadPlaces/SelectTables/
 * ResponsibleUsersSection) в режиме selectionMode - тот же вид, что в детали.
 * Собирает параметры и эмитит `apply` с payload; батч-API, разбор BulkOpResult и
 * refresh делает родитель (*Management). Архив/восстановление - ConfirmationModal
 * в родителе, здесь их нет.
 */
export default {
  name: 'BulkOperationsModal',
  components: { BaseModal, BaseDropdown, SelectUnloadPlaces, SelectTables, ResponsibleUsersSection },
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
      placeIds: [],
      tableIds: [],
      responsibleUsers: [], // [{username, required_approval}] - согласование индивидуально
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
    canApply() {
      if (this.submitting) return false;
      switch (this.operation) {
        case 'type':
          return this.typeValue !== null;
        case 'unload-places':
          return this.placeIds.length > 0;
        case 'tables':
          return this.tableIds.length > 0;
        case 'users':
          return this.responsibleUsers.length > 0;
        default:
          return false;
      }
    },
  },
  watch: {
    show(val) {
      if (val) this.reset();
    },
    operation() {
      if (this.show) this.reset();
    },
  },
  methods: {
    reset() {
      this.typeValue = null;
      this.mode = 'replace';
      this.placeIds = [];
      this.tableIds = [];
      this.responsibleUsers = [];
    },

    onApply() {
      if (!this.canApply) return;
      let payload;
      switch (this.operation) {
        case 'type':
          payload = { type: this.typeValue === NONE_TYPE ? null : this.typeValue };
          break;
        case 'unload-places':
          payload = { unloadPlaceIds: [...this.placeIds], mode: this.mode };
          break;
        case 'tables':
          payload = { tableIds: [...this.tableIds], mode: this.mode };
          break;
        case 'users':
          payload = {
            users: this.responsibleUsers.map(u => ({ username: u.username, required_approval: !!u.required_approval })),
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
  color: var(--text-muted);
}

.bulk-op__summary b {
  color: var(--accent-text);
}

.bulk-op__section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.bulk-op__label {
  font-size: 0.78em;
  color: var(--text-muted);
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
  color: var(--text-muted);
  font-weight: 600;
}

/* сегментированный переключатель Заменить/Добавить */
.seg {
  display: inline-flex;
  border: 1px solid var(--border);
  border-radius: 50px;
  padding: 3px;
  background: var(--accent-tint);
}

.seg__btn {
  border: none;
  background: transparent;
  border-radius: 50px;
  padding: 5px 16px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-muted);
  cursor: pointer;
  font-family: inherit;
  transition: background 0.2s, color 0.2s;
}

.seg__btn--active {
  background: var(--accent);
  color: var(--accent-contrast);
}

.bulk-op__hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}

/* кнопки действий (эталон SelectionModal actions) */
.bulk-op__cancel {
  padding: 10px 20px;
  border: 1px solid var(--border);
  background: var(--surface);
  border-radius: 8px;
  cursor: pointer;
  font-size: 14px;
  transition: background 0.2s;
}

.bulk-op__cancel:hover {
  background: var(--surface-2);
}

.bulk-op__apply {
  padding: 10px 20px;
  background: var(--accent);
  color: var(--accent-contrast);
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
