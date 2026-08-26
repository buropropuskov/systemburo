<template>
  <BaseModal
    :show="show"
    :title="title"
    width="520px"
    radius="30px"
    @close="close"
  >
    <div class="bl-create">
      <template v-if="type === 'vehicle'">
        <div class="completion__format">
          <div class="format__header">
            <label class="format__label">Формат номеров <span class="required">*</span></label>
          </div>
          <div class="format__dropdown">
            <button
              class="dropdown__button"
              @click="toggleFormatDropdown"
            >
              <div class="button__content">
                <span class="button__text">{{ selectedFormatText }}</span>
                <AppIcon
                  name="arrow"
                  class="button__arrow"
                  :class="{ 'button__arrow--open': isFormatDropdownOpen }"
                />
              </div>
            </button>
            <transition name="dropdown">
              <div
                v-if="isFormatDropdownOpen"
                class="dropdown__menu"
              >
                <div
                  v-for="format in formats"
                  :key="format.format.id"
                  class="dropdown__item"
                  @click="selectFormat(format)"
                >
                  <span class="item__text">{{ format.format.name }}</span>
                </div>
                <div
                  v-if="!formats.length"
                  class="dropdown__item"
                >
                  <span class="item__text">Нет форматов</span>
                </div>
              </div>
            </transition>
          </div>
        </div>

        <div class="completion__fields">
          <div class="completion__number">
            <div class="completion__number-header">
              <label class="input__label">Номер Т/С <span class="required">*</span></label>
            </div>
            <div
              v-if="selectedFormat"
              class="number__field"
            >
              <input
                v-for="(cell, index) in selectedFormat.cells"
                :key="index"
                v-model="numberParts[index]"
                class="number__input"
                :placeholder="getPlaceholder(cell)"
                :maxlength="cell.max_length"
                :style="{ width: getInputWidth(cell) }"
                @input="validatePart(index, $event, cell)"
                @blur="formatPart(index, cell)"
              >
            </div>
            <div
              v-else
              class="no-format-message"
            >
              Выберите формат номера
            </div>
          </div>

          <div class="completion__mark">
            <div class="completion__mark-header">
              <label class="input__label">Марка Т/С <span class="required">*</span></label>
            </div>
            <div class="mark__field">
              <div class="mark__dropdown">
                <button
                  class="mark__dropdown-button"
                  @click="toggleMarkDropdown"
                >
                  <div class="mark__button-content">
                    <span class="mark__button-text">{{ selectedMark || 'Выберите марку' }}</span>
                    <AppIcon
                      name="arrow"
                      class="mark__button-arrow"
                      :class="{ 'mark__button-arrow--open': isMarkDropdownOpen }"
                    />
                  </div>
                </button>
                <transition name="dropdown">
                  <div
                    v-if="isMarkDropdownOpen"
                    class="mark__dropdown-menu"
                  >
                    <div class="mark__search">
                      <input
                        v-model="markSearch"
                        class="mark__search-input"
                        placeholder="Поиск марки..."
                      >
                    </div>
                    <div class="mark__dropdown-list">
                      <div
                        v-for="mark in filteredMarks"
                        :key="mark.id"
                        class="mark__dropdown-item"
                        @click="selectMark(mark)"
                      >
                        <span class="mark__item-text">{{ mark.name }}</span>
                      </div>
                      <div
                        v-if="!filteredMarks.length"
                        class="mark__dropdown-empty"
                      >
                        Марки не найдены
                      </div>
                    </div>
                  </div>
                </transition>
              </div>
            </div>
          </div>
        </div>
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

  <BlacklistImpactModal
    :show="showImpact"
    :subject="impactSubject"
    :impact="impact"
    :submitting="saving"
    @confirm="confirmImpact"
    @close="showImpact = false"
  />
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import FormField from '@/components/ui/FormField.vue';
import { apiRequest } from '@/api/client';
import { personBlacklistImpact, vehicleBlacklistImpact } from '@/api/blacklist';
import BlacklistImpactModal from './BlacklistImpactModal.vue';
import { listMarks } from '@/api/marks';
import { validatePartValue, formatPartValue, initializeNumberParts } from '@/composables/useNumberFormat';
import AppIcon from '@/components/icons/AppIcon.vue';

/**
 * Модалка добавления записи в чёрный список (#443). Тип определяет форму: 'vehicle' -
 * формат номера + поячеечный ввод + марка (визуально как VehicleForm); 'person' - ФИО.
 * Создание делегируется через prop createFn (API-метод сущности); ошибка сервера
 * (409 дубль / 400) показывается инлайн.
 */
export default {
  name: 'BlacklistCreateModal',
  components: { AppIcon, BaseModal, FormField, BlacklistImpactModal },
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
      isFormatDropdownOpen: false,
      marks: [],
      markId: null,
      selectedMark: '',
      isMarkDropdownOpen: false,
      markSearch: '',
      numberParts: [],
      lastName: '',
      firstName: '',
      middleName: '',
      reason: '',
      saving: false,
      formError: '',
      showImpact: false,
      impactSubject: '',
      impact: { matches: 0, tables: [], rows: [] },
    };
  },
  computed: {
    title() {
      return this.type === 'vehicle' ? 'Добавить машину в чёрный список' : 'Добавить человека в чёрный список';
    },
    selectedFormat() {
      return this.formats.find((f) => f.format.id === this.selectedFormatId) || null;
    },
    selectedFormatText() {
      return this.selectedFormat ? this.selectedFormat.format.name : 'Выберите формат';
    },
    filteredMarks() {
      const q = this.markSearch.trim().toLowerCase();
      if (!q) return this.marks;
      return this.marks.filter((m) => m.name.toLowerCase().includes(q));
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
        if (this.type === 'vehicle') {
          this.loadVehicleData();
          document.addEventListener('click', this.onDocumentClick);
        }
      } else {
        document.removeEventListener('click', this.onDocumentClick);
      }
    },
  },
  beforeUnmount() {
    document.removeEventListener('click', this.onDocumentClick);
  },
  methods: {
    resetForm() {
      this.selectedFormatId = null;
      this.formats = [];
      this.isFormatDropdownOpen = false;
      this.markId = null;
      this.selectedMark = '';
      this.isMarkDropdownOpen = false;
      this.markSearch = '';
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
        if (def) this.selectFormat(def);
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
    onDocumentClick(e) {
      if (!e.target.closest('.format__dropdown')) this.isFormatDropdownOpen = false;
      if (!e.target.closest('.mark__dropdown')) this.isMarkDropdownOpen = false;
    },
    toggleFormatDropdown() {
      this.isFormatDropdownOpen = !this.isFormatDropdownOpen;
      this.isMarkDropdownOpen = false;
    },
    selectFormat(format) {
      this.selectedFormatId = format.format.id;
      this.numberParts = initializeNumberParts(this.selectedFormat);
      this.isFormatDropdownOpen = false;
    },
    toggleMarkDropdown() {
      this.isMarkDropdownOpen = !this.isMarkDropdownOpen;
      this.isFormatDropdownOpen = false;
    },
    selectMark(mark) {
      this.markId = mark.id;
      this.selectedMark = mark.name;
      this.isMarkDropdownOpen = false;
      this.markSearch = '';
    },
    validatePart(index, event, cell) {
      const value = validatePartValue(event.target.value, cell);
      this.numberParts[index] = value;
      event.target.value = value;
    },
    formatPart(index, cell) {
      this.numberParts[index] = formatPartValue(this.numberParts[index], cell);
    },
    getPlaceholder(cell) {
      if (cell.cell_type === 'numbers') return '0'.repeat(cell.max_length);
      return 'A'.repeat(cell.max_length);
    },
    getInputWidth(cell) {
      const width = Math.max(50, (cell.max_length || 2) * 25);
      return `${width}px`;
    },
    close() {
      if (this.saving) return;
      this.$emit('close');
    },
    /** Собирает тело запроса и подпись записи - одинаково для предпросмотра и записи. */
    buildPayload() {
      if (this.type === 'vehicle') {
        const carNumber = this.numberParts.join(' ').trim();
        const markName = (this.marks.find((m) => m.id === this.markId) || {}).name || this.selectedMark || '';
        return {
          payload: { car_number: carNumber, mark_id: this.markId, reason: this.reason.trim() },
          displayName: [carNumber, markName].filter(Boolean).join(' '),
        };
      }
      const payload = {
        last_name: this.lastName.trim(),
        first_name: this.firstName.trim(),
        middle_name: this.middleName.trim(),
        reason: this.reason.trim(),
      };
      return {
        payload,
        displayName: [payload.last_name, payload.first_name, payload.middle_name].filter(Boolean).join(' '),
      };
    },

    /**
     * Перед записью показываем, где эта машина или человек сейчас фигурирует: внесение
     * деактивирует строки и уводит их с постов, и администратор должен видеть это до
     * подтверждения, а не потом в истории. Когда действующих строк нет, окно не нужно -
     * вносим сразу. Сбой предпросмотра тоже не должен мешать внесению: он вспомогательный.
     */
    async submit() {
      if (!this.canSubmit || this.saving) return;
      this.formError = '';
      const { payload, displayName } = this.buildPayload();

      this.saving = true;
      try {
        const impact = this.type === 'vehicle'
          ? await vehicleBlacklistImpact({ carNumber: payload.car_number, markId: payload.mark_id })
          : await personBlacklistImpact({
              lastName: payload.last_name,
              firstName: payload.first_name,
              middleName: payload.middle_name,
          });
        if (impact && impact.matches > 0) {
          this.impact = impact;
          this.impactSubject = displayName;
          this.showImpact = true;
          this.saving = false;
          return;
        }
      } catch (e) {
        console.warn('Предпросмотр последствий не удался, продолжаем внесение', e);
      }

      await this.persist(payload, displayName);
    },

    /** Запись в чёрный список - общий путь для случая с окном и без него. */
    async persist(payload, displayName) {
      this.saving = true;
      this.formError = '';
      try {
        await this.createFn(payload);
        this.showImpact = false;
        this.$emit('created', displayName);
      } catch (e) {
        this.showImpact = false;
        this.formError = e?.message || 'Не удалось добавить запись';
      } finally {
        this.saving = false;
      }
    },

    /** Подтверждение из окна последствий. */
    confirmImpact() {
      const { payload, displayName } = this.buildPayload();
      this.persist(payload, displayName);
    },
  },
};
</script>

<style scoped>
.bl-create {
  padding: 16px;
  display: flex;
  flex-direction: column;
}

/* Формат номера + номер + марка - визуально идентично VehicleForm */
.input__label {
  font-size: 13px;
  color: var(--text-muted);
}

.required {
  color: var(--danger-text);
}

.completion__format {
  display: flex;
  flex-direction: column;
  gap: 10px;
  position: relative;
  padding-bottom: 15px;
}

.format__header {
  display: flex;
  justify-content: space-between;
  align-items: end;
}

.format__label {
  font-size: 13px;
  color: var(--text-muted);
}

.format__dropdown {
  position: relative;
}

.dropdown__button {
  width: 100%;
  height: 40px;
  border: 1px solid var(--border);
  background-color: var(--surface);
  border-radius: 15px;
  outline: none;
  cursor: pointer;
  padding: 0 15px;
  transition: border-color 0.2s;
}

.dropdown__button:hover {
  border-color: var(--accent);
}

.button__content {
  display: flex;
  align-items: center;
  width: 100%;
  height: 100%;
  justify-content: space-between;
}

.button__text {
  font-size: 14px;
  color: var(--text);
  font-weight: 500;
  display: block;
}

.button__arrow {
  width: 10px;
  height: 10px;
  transition: transform 0.2s;
  transform: rotate(90deg);
  flex-shrink: 0;
}

.button__arrow--open {
  transform: rotate(-90deg);
}

.dropdown__menu {
  position: absolute;
  top: 100%;
  left: 0;
  width: 100%;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  margin-top: 5px;
  box-shadow: 0 3px 10px var(--shadow-drop);
  z-index: 1000;
  max-height: 300px;
  overflow-y: auto;
}

.dropdown__item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 15px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.dropdown__item:hover {
  background-color: var(--surface-2);
}

.dropdown__item:first-child {
  border-radius: 10px 10px 0 0;
}

.dropdown__item:last-child {
  border-radius: 0 0 10px 10px;
}

.item__text {
  font-size: 13px;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.completion__fields {
  display: flex;
  gap: 20px;
  align-items: flex-start;
  margin-bottom: 15px;
}

.completion__number,
.completion__mark {
  flex: 1;
}

.completion__number-header,
.completion__mark-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 5px;
}

.number__field {
  max-width: 202px;
  min-width: 202px;
  height: 40px;
  display: flex;
  border: 1px solid var(--border);
  border-radius: 15px;
  overflow: hidden;
  background: var(--surface);
}

.number__input {
  border: none;
  height: 100%;
  outline: none;
  text-align: center;
  font-size: 14px;
  background: transparent;
  flex: 1;
  min-width: 0;
  text-transform: uppercase;
}

.number__input:not(:last-child) {
  border-right: 1px solid var(--border);
}

.number__input::placeholder {
  color: var(--text-muted);
  font-size: 12px;
  text-transform: none;
}

.number__input:focus {
  background-color: var(--surface-2);
}

.no-format-message {
  font-size: 12px;
  color: var(--text-muted);
  text-align: center;
  padding: 10px;
  background: var(--surface-2);
  border-radius: 10px;
  border: 1px solid var(--border);
}

.mark__field {
  width: 100%;
  height: 40px;
  position: relative;
}

.mark__dropdown {
  width: 100%;
  height: 100%;
}

.mark__dropdown-button {
  width: 100%;
  height: 100%;
  border: 1px solid var(--border);
  background-color: var(--surface);
  border-radius: 15px;
  outline: none;
  cursor: pointer;
  padding: 0 15px;
  transition: border-color 0.2s;
}

.mark__dropdown-button:hover {
  border-color: var(--accent);
}

.mark__button-content {
  display: flex;
  align-items: center;
  width: 100%;
  height: 100%;
  justify-content: space-between;
}

.mark__button-text {
  font-size: 14px;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 150px;
  display: block;
}

.mark__button-arrow {
  width: 10px;
  height: 10px;
  transition: transform 0.2s;
  transform: rotate(90deg);
  flex-shrink: 0;
}

.mark__button-arrow--open {
  transform: rotate(-90deg);
}

.mark__dropdown-menu {
  position: absolute;
  top: 100%;
  left: 0;
  width: 100%;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  margin-top: 5px;
  box-shadow: 0 3px 10px var(--shadow-drop);
  z-index: 1000;
  max-height: 220px;
  overflow: hidden;
}

.mark__search {
  padding: 10px;
  border-bottom: 1px solid var(--border);
}

.mark__search-input {
  width: 100%;
  border: 1px solid var(--border);
  border-radius: 15px;
  padding: 5px 10px;
  outline: none;
  font-size: 14px;
}

.mark__dropdown-list {
  max-height: 144px;
  overflow-y: auto;
}

.mark__dropdown-item {
  padding: 8px 15px;
  cursor: pointer;
  transition: background-color 0.2s;
  border-bottom: 1px solid var(--surface-2);
}

.mark__dropdown-item:hover {
  background-color: var(--surface-2);
}

.mark__dropdown-item:last-child {
  border-bottom: none;
}

.mark__item-text {
  font-size: 14px;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.mark__dropdown-empty {
  padding: 10px 15px;
  font-size: 13px;
  color: var(--text-muted);
  text-align: center;
}

.bl-form-error {
  color: var(--color-danger, var(--danger-text));
  font-size: 13px;
  margin-top: 4px;
}

.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

/* Номер (202px фикс.) и марка бок о бок переполняют модалку на узких телефонах
   (320-375px) - тот же фикс, что уже стоит в VehicleForm.vue (эталон разметки,
   #481: этот блок - её 1:1 копия) для идентичной .completion__fields. */
@media (max-width: 480px) {
  .completion__fields {
    flex-direction: column;
  }
}
</style>
