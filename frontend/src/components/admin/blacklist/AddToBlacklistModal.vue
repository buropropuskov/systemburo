<template>
  <BaseModal
    :show="show"
    :title="title"
    width="480px"
    :z-index="zIndex"
    radius="30px"
    @close="close"
  >
    <div class="atb">
      <!-- ADD: сущность уже известна (из карточки), показываем read-only -->
      <div
        v-if="!isEdit"
        class="atb-entity"
      >
        <span class="atb-entity-label">{{ entityCaption }}</span>
        <span class="atb-entity-value">{{ entityLabel }}</span>
      </div>

      <!-- EDIT: правка идентичности записи реестра -->
      <template v-else>
        <template v-if="type === 'vehicle'">
          <!-- Поячеечный ввод номера по формату - тот же, что при создании записи ЧС (#481). -->
          <VehicleNumberFormatInput
            :key="openSeq"
            v-model="carNumber"
          />
          <FormField
            label="Марка Т/С"
            :required="true"
          >
            <BaseDropdown
              v-model="markId"
              :options="marks"
              label-key="name"
              value-key="id"
              :searchable="true"
              placeholder="Выберите марку"
            />
          </FormField>
        </template>
        <template v-else>
          <FormField
            label="Фамилия"
            :required="true"
          >
            <input
              ref="firstInput"
              v-model="lastName"
              class="lk-input"
              placeholder="Иванов"
            >
          </FormField>
          <FormField
            label="Имя"
            :required="true"
          >
            <input
              v-model="firstName"
              class="lk-input"
              placeholder="Иван"
            >
          </FormField>
          <FormField label="Отчество">
            <input
              v-model="middleName"
              class="lk-input"
              placeholder="Иванович (необязательно)"
            >
          </FormField>
        </template>
      </template>

      <FormField
        label="Причина"
        :required="true"
      >
        <textarea
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
        :disabled="!canConfirm || saving"
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
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import VehicleNumberFormatInput from './VehicleNumberFormatInput.vue';
import { listMarks } from '@/api/marks';

/**
 * Модалка добавления/редактирования записи чёрного списка.
 * - mode='add' (из карточки сущности, #443): сущность read-only, оператор вводит причину,
 *   confirm(reason: string).
 * - mode='edit' (из реестра ЧС): правка идентичности (номер+марка / ФИО) и причины,
 *   confirm(payload: object). Идентичность приходит в initialEntity.
 * Сам вызов API (create/update) - на стороне родителя; error/saving контролирует родитель.
 */
export default {
  name: 'AddToBlacklistModal',
  components: { BaseModal, FormField, BaseDropdown, VehicleNumberFormatInput },
  props: {
    show: { type: Boolean, default: false },
    type: { type: String, required: true, validator: (v) => ['vehicle', 'person'].includes(v) },
    mode: { type: String, default: 'add', validator: (v) => ['add', 'edit'].includes(v) },
    entityLabel: { type: String, default: '' },
    initialReason: { type: String, default: '' },
    // Префилл идентичности для edit: { car_number, mark_id, last_name, first_name, middle_name }.
    initialEntity: { type: Object, default: () => ({}) },
    saving: { type: Boolean, default: false },
    error: { type: String, default: '' },
    zIndex: { type: Number, default: 10500 },
  },
  emits: ['close', 'confirm'],
  data() {
    return {
      reason: '',
      carNumber: '',
      markId: null,
      lastName: '',
      firstName: '',
      middleName: '',
      marks: [],
      // Меняется при каждом открытии - форсит ремоунт VehicleNumberFormatInput, чтобы он
      // заново разложил префилл номера по ячейкам.
      openSeq: 0,
    };
  },
  computed: {
    isEdit() {
      return this.mode === 'edit';
    },
    title() {
      if (this.isEdit) return this.type === 'vehicle' ? 'Редактировать машину' : 'Редактировать человека';
      return this.type === 'vehicle' ? 'Добавить машину в чёрный список' : 'Добавить человека в чёрный список';
    },
    entityCaption() {
      return this.type === 'vehicle' ? 'Машина' : 'Человек';
    },
    confirmText() {
      if (this.isEdit) return this.saving ? 'Сохранение...' : 'Сохранить';
      return this.saving ? 'Добавление...' : 'Добавить в ЧС';
    },
    canConfirm() {
      if (!this.reason.trim() || this.saving) return false;
      if (!this.isEdit) return true;
      if (this.type === 'vehicle') return !!this.carNumber.trim() && !!this.markId;
      return !!this.lastName.trim() && !!this.firstName.trim();
    },
  },
  watch: {
    show(open) {
      if (!open) return;
      this.reason = this.initialReason || '';
      if (this.isEdit) {
        const e = this.initialEntity || {};
        this.carNumber = e.car_number || '';
        this.markId = e.mark_id ?? null;
        this.lastName = e.last_name || '';
        this.firstName = e.first_name || '';
        this.middleName = e.middle_name || '';
        this.openSeq += 1; // ремоунт ввода номера -> повторный префилл по ячейкам
        if (this.type === 'vehicle') this.loadMarks();
      }
      this.$nextTick(() => this.$refs.firstInput?.focus());
    },
  },
  methods: {
    async loadMarks() {
      try {
        const marks = await listMarks();
        const arr = Array.isArray(marks) ? marks : [];
        this.marks = arr.filter((m) => m.is_active !== false).map((m) => ({ id: m.id, name: m.name }));
      } catch {
        this.marks = [];
      }
    },
    close() {
      if (this.saving) return;
      this.$emit('close');
    },
    confirm() {
      if (!this.canConfirm) return;
      const reason = this.reason.trim();
      if (!this.isEdit) {
        this.$emit('confirm', reason);
        return;
      }
      if (this.type === 'vehicle') {
        this.$emit('confirm', { car_number: this.carNumber.trim(), mark_id: this.markId, reason });
      } else {
        this.$emit('confirm', {
          last_name: this.lastName.trim(),
          first_name: this.firstName.trim(),
          middle_name: this.middleName.trim(),
          reason,
        });
      }
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
  background: var(--color-bg, var(--accent-tint));
  border-radius: var(--radius-md, 15px);
}

.atb-entity-label {
  font-size: 12px;
  color: var(--color-text-muted, var(--text-muted));
}

.atb-entity-value {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text, var(--text));
  word-break: break-word;
}

.atb-error {
  color: var(--color-danger, var(--danger-text));
  font-size: 13px;
}
</style>
