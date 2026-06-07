<template>
  <BaseModal
    :show="show"
    :title="title"
    width="520px"
    @close="close"
  >
    <div class="bl-create">
      <template v-if="type === 'vehicle'">
        <FormField
          label="Формат номера"
          :required="true"
        >
          <BaseDropdown
            :model-value="selectedFormatId"
            :options="formatOptions"
            label-key="name"
            value-key="id"
            placeholder="Выберите формат"
            @update:model-value="onFormatChange"
          />
        </FormField>
        <FormField
          label="Номер"
          :required="true"
        >
          <div
            v-if="selectedFormat"
            class="bl-plate"
          >
            <input
              v-for="(cell, i) in selectedFormat.cells"
              :key="i"
              :value="numberParts[i]"
              class="lk-input bl-plate-cell"
              :style="{ width: cellWidth(cell) }"
              :maxlength="cell.max_length"
              :placeholder="placeholder(cell)"
              @input="onPartInput(i, $event, cell)"
              @blur="onPartBlur(i, cell)"
            >
          </div>
          <span
            v-else
            class="bl-hint"
          >Сначала выберите формат</span>
        </FormField>
        <FormField
          label="Марка"
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

      <FormField
        label="Причина"
        :required="true"
      >
        <textarea
          v-model="reason"
          class="lk-textarea"
          rows="3"
          placeholder="Опишите причину добавления в чёрный список"
        />
      </FormField>

      <div
        v-if="formError"
        class="bl-form-error"
      >
        {{ formError }}
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
        class="lk-button lk-button--primary"
        :disabled="!canSubmit || saving"
        @click="submit"
      >
        {{ saving ? 'Сохранение...' : 'Добавить' }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import FormField from '@/components/ui/FormField.vue';
import { apiRequest } from '@/api/client';
import { listMarks } from '@/api/marks';
import { validatePartValue, formatPartValue, initializeNumberParts } from '@/composables/useNumberFormat';

/**
 * Модалка добавления записи в чёрный список (#443). Тип определяет форму: 'vehicle' -
 * формат номера + поячеечный ввод (как в VehicleForm) + марка; 'person' - ФИО.
 * Создание делегируется через prop createFn (API-метод сущности); ошибка сервера
 * (409 дубль / 400) показывается инлайн.
 */
export default {
  name: 'BlacklistCreateModal',
  components: { BaseModal, BaseDropdown, FormField },
  props: {
    show: { type: Boolean, default: false },
    type: { type: String, required: true, validator: (v) => ['vehicle', 'person'].includes(v) },
    createFn: { type: Function, required: true },
  },
  emits: ['close', 'created'],
  data() {
    return {
      formats: [],
      selectedFormatId: null,
      marks: [],
      markId: null,
      numberParts: [],
      lastName: '',
      firstName: '',
      middleName: '',
      reason: '',
      saving: false,
      formError: '',
    };
  },
  computed: {
    title() {
      return this.type === 'vehicle' ? 'Добавить машину в чёрный список' : 'Добавить человека в чёрный список';
    },
    formatOptions() {
      return this.formats.map((f) => ({ id: f.format.id, name: f.format.name }));
    },
    selectedFormat() {
      return this.formats.find((f) => f.format.id === this.selectedFormatId) || null;
    },
    canSubmit() {
      if (!this.reason.trim()) return false;
      if (this.type === 'vehicle') {
        return !!this.selectedFormat && this.numberParts.length > 0 &&
          this.numberParts.every((p) => p && p.trim()) && !!this.markId;
      }
      return !!this.lastName.trim() && !!this.firstName.trim();
    },
  },
  watch: {
    show(open) {
      if (open) {
        this.resetForm();
        if (this.type === 'vehicle') this.loadVehicleData();
      }
    },
  },
  methods: {
    resetForm() {
      this.selectedFormatId = null;
      this.formats = [];
      this.markId = null;
      this.numberParts = [];
      this.lastName = '';
      this.firstName = '';
      this.middleName = '';
      this.reason = '';
      this.formError = '';
      this.saving = false;
    },
    async loadVehicleData() {
      try {
        const res = await apiRequest('/license-plate-formats');
        const data = await res.json();
        this.formats = Array.isArray(data) ? data : [];
        const def = this.formats.find((f) => f.format.is_default) || this.formats[0];
        if (def) this.onFormatChange(def.format.id);
      } catch {
        this.formError = 'Не удалось загрузить форматы номеров';
      }
      try {
        const marks = await listMarks();
        const arr = Array.isArray(marks) ? marks : [];
        this.marks = arr.filter((m) => m.is_active !== false).map((m) => ({ id: m.id, name: m.name }));
      } catch {
        this.formError = 'Не удалось загрузить марки';
      }
    },
    onFormatChange(id) {
      this.selectedFormatId = id;
      this.numberParts = initializeNumberParts(this.selectedFormat);
    },
    onPartInput(i, event, cell) {
      this.numberParts[i] = validatePartValue(event.target.value, cell);
    },
    onPartBlur(i, cell) {
      this.numberParts[i] = formatPartValue(this.numberParts[i], cell);
    },
    placeholder(cell) {
      if (cell.cell_type === 'numbers') return '0'.repeat(cell.max_length || 1);
      if (cell.cell_type === 'letters') return 'А';
      return 'А0';
    },
    cellWidth(cell) {
      return `${Math.max(2, cell.max_length || 2) * 16 + 20}px`;
    },
    close() {
      if (this.saving) return;
      this.$emit('close');
    },
    async submit() {
      if (!this.canSubmit || this.saving) return;
      this.saving = true;
      this.formError = '';
      try {
        let payload;
        let displayName;
        if (this.type === 'vehicle') {
          const carNumber = this.numberParts.join(' ').trim();
          const markName = (this.marks.find((m) => m.id === this.markId) || {}).name || '';
          payload = { car_number: carNumber, mark_id: this.markId, reason: this.reason.trim() };
          displayName = [carNumber, markName].filter(Boolean).join(' ');
        } else {
          payload = {
            last_name: this.lastName.trim(),
            first_name: this.firstName.trim(),
            middle_name: this.middleName.trim(),
            reason: this.reason.trim(),
          };
          displayName = [payload.last_name, payload.first_name, payload.middle_name].filter(Boolean).join(' ');
        }
        await this.createFn(payload);
        this.$emit('created', displayName);
      } catch (e) {
        this.formError = e?.message || 'Не удалось добавить запись';
      } finally {
        this.saving = false;
      }
    },
  },
};
</script>

<style scoped>
.bl-create {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.bl-plate {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.bl-plate-cell {
  text-align: center;
  text-transform: uppercase;
}

.bl-hint {
  color: var(--color-text-muted, #999);
  font-size: 13px;
}

.bl-form-error {
  color: var(--color-danger, #dc3545);
  font-size: 13px;
}
</style>
