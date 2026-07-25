<template>
  <BaseModal
    :show="show"
    :title="modalTitle"
    width="560px"
    content-class="user-bulk-op-modal"
    @close="$emit('close')"
  >
    <div
      class="bulk-op"
      data-testid="user-bulk-op-modal"
    >
      <p class="bulk-op__summary">
        Операция применится к <b>{{ selectedCount }}</b> {{ userWord }}.
      </p>

      <div class="bulk-op__section">
        <label class="bulk-op__label">{{ fieldLabel }}</label>
        <BaseDropdown
          data-testid="user-bulk-op-value"
          teleport
          searchable
          :model-value="value"
          :options="options"
          label-key="name"
          value-key="id"
          :placeholder="placeholder"
          @update:model-value="value = $event"
        />
      </div>
    </div>

    <template #actions>
      <button
        class="bulk-op__cancel"
        data-testid="user-bulk-op-cancel"
        @click="$emit('close')"
      >
        Отмена
      </button>
      <button
        class="bulk-op__apply"
        data-testid="user-bulk-op-apply"
        :disabled="!canApply"
        @click="onApply"
      >
        {{ submitting ? 'Применение...' : `Применить (${selectedCount})` }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';

/**
 * Модалка групповой операции над пользователями: назначить организацию/компанию
 * или сменить тип. Переиспользует BaseDropdown (те же дропдауны, что в карточке
 * пользователя) - тип из userTypes, организации/компании из stores родителя.
 * Собирает выбранный id и эмитит `apply(id)`; батч-API, разбор BulkOpResult и
 * refresh делает родитель (UserControl). Архив/восстановление здесь нет - они
 * идут через ConfirmationModal в родителе.
 */
export default {
  name: 'UserBulkOperationsModal',
  components: { BaseModal, BaseDropdown },
  props: {
    show: { type: Boolean, default: false },
    // 'type' | 'organization' | 'company'
    operation: { type: String, default: '' },
    selectedCount: { type: Number, default: 0 },
    userTypes: { type: Array, default: () => [] },
    organizations: { type: Array, default: () => [] },
    companies: { type: Array, default: () => [] },
    submitting: { type: Boolean, default: false },
  },
  emits: ['close', 'apply'],
  data() {
    return {
      value: null,
    };
  },
  computed: {
    modalTitle() {
      return {
        type: 'Сменить тип',
        organization: 'Назначить организацию',
        company: 'Назначить компанию',
      }[this.operation] || 'Групповая операция';
    },
    fieldLabel() {
      return {
        type: 'Новый тип',
        organization: 'Организация',
        company: 'Компания',
      }[this.operation] || '';
    },
    placeholder() {
      return {
        type: 'Выберите тип',
        organization: 'Выберите организацию',
        company: 'Выберите компанию',
      }[this.operation] || 'Выберите значение';
    },
    options() {
      switch (this.operation) {
        case 'type':
          return this.userTypes;
        case 'organization':
          return this.organizations;
        case 'company':
          return this.companies;
        default:
          return [];
      }
    },
    // Слово в дательном падеже мн.ч. после числа: «к N пользователям».
    userWord() {
      return 'пользователям';
    },
    canApply() {
      return !this.submitting && this.value !== null && this.value !== undefined;
    },
    // Единый ключ открытия: reset один раз по финальным значениям show+operation
    // (родитель ставит их синхронно в одном тике - урок #1090 о двойном watcher'е).
    openKey() {
      return this.show ? this.operation : null;
    },
  },
  watch: {
    openKey(val, old) {
      if (val && val !== old) this.reset();
    },
  },
  methods: {
    reset() {
      this.value = null;
    },
    onApply() {
      if (!this.canApply) return;
      this.$emit('apply', this.value);
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

/* кнопки действий (эталон BulkOperationsModal actions) */
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
