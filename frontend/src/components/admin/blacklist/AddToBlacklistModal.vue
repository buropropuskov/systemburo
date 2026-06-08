<template>
  <BaseModal
    :show="show"
    :title="title"
    width="480px"
    :z-index="zIndex"
    @close="close"
  >
    <div class="atb">
      <div class="atb-entity">
        <span class="atb-entity-label">{{ entityCaption }}</span>
        <span class="atb-entity-value">{{ entityLabel }}</span>
      </div>

      <FormField
        label="Причина"
        :required="true"
      >
        <textarea
          ref="reasonInput"
          v-model="reason"
          class="lk-textarea"
          rows="3"
          placeholder="Опишите причину попадания в чёрный список"
        />
      </FormField>

      <div
        v-if="error"
        class="atb-error"
      >
        {{ error }}
      </div>
    </div>

    <template #actions>
      <button
        class="lk-button lk-button--ghost"
        :disabled="saving"
        @click="close"
      >
        Отмена
      </button>
      <button
        class="lk-button"
        :class="isEdit ? 'lk-button--primary' : 'lk-button--danger'"
        :disabled="!reason.trim() || saving"
        @click="confirm"
      >
        {{ confirmText }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import FormField from '@/components/ui/FormField.vue';

/**
 * Модалка-подтверждение добавления в чёрный список из карточки сущности (#443).
 * Сущность уже известна (показывается read-only), оператору остаётся ввести причину.
 * Само создание (резолв mark_id для машин, вызов API) - на стороне родителя: модалка
 * только собирает причину и эмитит confirm(reason). error/saving контролирует родитель.
 */
export default {
  name: 'AddToBlacklistModal',
  components: { BaseModal, FormField },
  props: {
    show: { type: Boolean, default: false },
    type: { type: String, required: true, validator: (v) => ['vehicle', 'person'].includes(v) },
    // 'add' - добавление из карточки; 'edit' - редактирование причины из страницы ЧС.
    mode: { type: String, default: 'add', validator: (v) => ['add', 'edit'].includes(v) },
    entityLabel: { type: String, default: '' },
    initialReason: { type: String, default: '' },
    saving: { type: Boolean, default: false },
    error: { type: String, default: '' },
    // Открывается из карточки Т/С/сотрудника (z-index 10001) - перекрываем её. Ниже
    // DirtyConfirmModal (11000).
    zIndex: { type: Number, default: 10500 },
  },
  emits: ['close', 'confirm'],
  data() {
    return { reason: '' };
  },
  computed: {
    isEdit() {
      return this.mode === 'edit';
    },
    title() {
      if (this.isEdit) return 'Редактировать причину';
      return this.type === 'vehicle' ? 'Добавить машину в чёрный список' : 'Добавить человека в чёрный список';
    },
    entityCaption() {
      return this.type === 'vehicle' ? 'Машина' : 'Человек';
    },
    confirmText() {
      if (this.isEdit) return this.saving ? 'Сохранение...' : 'Сохранить';
      return this.saving ? 'Добавление...' : 'Добавить в ЧС';
    },
  },
  watch: {
    show(open) {
      if (open) {
        this.reason = this.initialReason || '';
        this.$nextTick(() => this.$refs.reasonInput?.focus());
      }
    },
  },
  methods: {
    close() {
      if (this.saving) return;
      this.$emit('close');
    },
    confirm() {
      const reason = this.reason.trim();
      if (!reason || this.saving) return;
      this.$emit('confirm', reason);
    },
  },
};
</script>

<style scoped>
.atb {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.atb-entity {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 10px 14px;
  background: var(--color-bg, #f8f9ff);
  border-radius: var(--radius-md, 15px);
}

.atb-entity-label {
  font-size: 12px;
  color: var(--color-text-muted, #999);
}

.atb-entity-value {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text, #333);
  word-break: break-word;
}

.atb-error {
  color: var(--color-danger, #dc3545);
  font-size: 13px;
}
</style>
