<template>
  <BaseModal
    :show="show"
    :title="title"
    width="520px"
    @close="close"
  >
    <div class="bl-create">
      <template v-if="type === 'vehicle'">
        <!-- Поячеечный ввод номера по формату вынесен в общий компонент - он же используется
             при правке записи ЧС, чтобы создание и редактирование не расходились (#481). -->
        <VehicleNumberFormatInput
          v-model="carNumber"
          class="bl-create__number"
        />

        <div class="completion__fields">
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
                    <img
                      src="@/assets/icons/arrow.png"
                      class="mark__button-arrow"
                      :class="{ 'mark__button-arrow--open': isMarkDropdownOpen }"
                    >
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
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import FormField from '@/components/ui/FormField.vue';
import VehicleNumberFormatInput from './VehicleNumberFormatInput.vue';
import { listMarks } from '@/api/marks';

/**
 * Модалка добавления записи в чёрный список (#443). Тип определяет форму: 'vehicle' -
 * формат номера + поячеечный ввод + марка (визуально как VehicleForm); 'person' - ФИО.
 * Создание делегируется через prop createFn (API-метод сущности); ошибка сервера
 * (409 дубль / 400) показывается инлайн.
 */
export default {
  name: 'BlacklistCreateModal',
  components: { BaseModal, FormField, VehicleNumberFormatInput },
  props: {
    show: { type: Boolean, default: false },
    type: { type: String, required: true, validator: (v) => ['vehicle', 'person'].includes(v) },
    createFn: { type: Function, required: true },
  },
  emits: ['close', 'created'],
  data() {
    return {
      carNumber: '',
      marks: [],
      markId: null,
      selectedMark: '',
      isMarkDropdownOpen: false,
      markSearch: '',
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
    filteredMarks() {
      const q = this.markSearch.trim().toLowerCase();
      if (!q) return this.marks;
      return this.marks.filter((m) => m.name.toLowerCase().includes(q));
    },
    canSubmit() {
      if (!this.reason.trim()) return false;
      if (this.type === 'vehicle') {
        return !!this.carNumber.trim() && !!this.markId;
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
      this.carNumber = '';
      this.markId = null;
      this.selectedMark = '';
      this.isMarkDropdownOpen = false;
      this.markSearch = '';
      this.lastName = '';
      this.firstName = '';
      this.middleName = '';
      this.reason = '';
      this.formError = '';
      this.saving = false;
    },
    async loadVehicleData() {
      try {
        const marks = await listMarks();
        const arr = Array.isArray(marks) ? marks : [];
        this.marks = arr.filter((m) => m.is_active !== false).map((m) => ({ id: m.id, name: m.name }));
      } catch {
        this.formError = 'Не удалось загрузить марки';
      }
    },
    onDocumentClick(e) {
      if (!e.target.closest('.mark__dropdown')) this.isMarkDropdownOpen = false;
    },
    toggleMarkDropdown() {
      this.isMarkDropdownOpen = !this.isMarkDropdownOpen;
    },
    selectMark(mark) {
      this.markId = mark.id;
      this.selectedMark = mark.name;
      this.isMarkDropdownOpen = false;
      this.markSearch = '';
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
          const carNumber = this.carNumber.trim();
          const markName = (this.marks.find((m) => m.id === this.markId) || {}).name || this.selectedMark || '';
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
  padding: 16px;
  display: flex;
  flex-direction: column;
}

/* Формат номера + номер + марка - визуально идентично VehicleForm */
.input__label {
  font-size: 13px;
  color: #a2a2a2;
}

.required {
  color: #ff4444;
}

.completion__fields {
  display: flex;
  gap: 20px;
  align-items: flex-start;
  margin-bottom: 15px;
}

.completion__mark {
  flex: 1;
}

.completion__mark-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 5px;
}

.bl-create__number {
  margin-bottom: 15px;
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
  border: 1px solid #e6e6e6;
  background-color: #fff;
  border-radius: 15px;
  outline: none;
  cursor: pointer;
  padding: 0 15px;
  transition: border-color 0.2s;
}

.mark__dropdown-button:hover {
  border-color: #4f5bdf;
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
  color: #000;
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
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 20px;
  margin-top: 5px;
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.1);
  z-index: 1000;
  max-height: 220px;
  overflow: hidden;
}

.mark__search {
  padding: 10px;
  border-bottom: 1px solid #e6e6e6;
}

.mark__search-input {
  width: 100%;
  border: 1px solid #e6e6e6;
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
  border-bottom: 1px solid #f5f5f5;
}

.mark__dropdown-item:hover {
  background-color: #f5f5f5;
}

.mark__dropdown-item:last-child {
  border-bottom: none;
}

.mark__item-text {
  font-size: 14px;
  color: #333;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.mark__dropdown-empty {
  padding: 10px 15px;
  font-size: 13px;
  color: #a2a2a2;
  text-align: center;
}

.bl-form-error {
  color: var(--color-danger, #dc3545);
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
</style>
