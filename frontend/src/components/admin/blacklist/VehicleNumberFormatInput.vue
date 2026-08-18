<template>
  <div class="vnf">
    <div class="completion__format">
      <div class="format__header">
        <label class="format__label">Формат номеров <span class="required">*</span></label>
      </div>
      <div class="format__dropdown">
        <button
          type="button"
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
  </div>
</template>

<script>
import { apiRequest } from '@/api/client';
import { validatePartValue, formatPartValue, initializeNumberParts } from '@/composables/useNumberFormat';
import AppIcon from '@/components/icons/AppIcon.vue';

/**
 * Поячеечный ввод номера Т/С по выбранному формату (формат-дропдаун + ячейки). Markup и стили
 * 1:1 повторяют ввод номера в BlacklistCreateModal, чтобы правка записи ЧС выглядела так же, как
 * создание (#481). v-model отдаёт собранную строку "часть часть". Для правки initialNumber
 * лучшим усилием раскладывается обратно по ячейкам, если число частей совпадает с форматом.
 */
export default {
  name: 'VehicleNumberFormatInput',
  components: {
    AppIcon,
  },
  props: {
    modelValue: { type: String, default: '' },
  },
  emits: ['update:modelValue'],
  data() {
    return {
      formats: [],
      selectedFormatId: null,
      isFormatDropdownOpen: false,
      numberParts: [],
    };
  },
  computed: {
    selectedFormat() {
      return this.formats.find((f) => f.format.id === this.selectedFormatId) || null;
    },
    selectedFormatText() {
      return this.selectedFormat ? this.selectedFormat.format.name : 'Выберите формат';
    },
  },
  watch: {
    numberParts: {
      deep: true,
      handler() {
        this.$emit('update:modelValue', this.numberParts.join(' ').trim());
      },
    },
  },
  mounted() {
    this.loadFormats();
    document.addEventListener('click', this.onDocumentClick);
  },
  beforeUnmount() {
    document.removeEventListener('click', this.onDocumentClick);
  },
  methods: {
    async loadFormats() {
      try {
        const res = await apiRequest('/license-plate-formats');
        const data = await res.json();
        this.formats = Array.isArray(data) ? data : [];
      } catch {
        this.formats = [];
      }
      this.applyInitial();
    },
    // Префилл правки: ищем формат, число ячеек которого совпадает с числом частей сохранённого
    // номера (приоритет дефолтному), и раскладываем; иначе берём дефолт с пустыми ячейками.
    applyInitial() {
      const raw = (this.modelValue || '').trim();
      const parts = raw ? raw.split(/\s+/).filter(Boolean) : [];
      let fmt = null;
      if (parts.length) {
        fmt = this.formats.find((f) => f.format.is_default && f.cells.length === parts.length)
          || this.formats.find((f) => f.cells.length === parts.length);
      }
      if (!fmt) fmt = this.formats.find((f) => f.format.is_default) || this.formats[0] || null;
      if (!fmt) return;
      this.selectedFormatId = fmt.format.id;
      if (parts.length && fmt.cells.length === parts.length) {
        this.numberParts = parts.slice();
      } else {
        this.numberParts = initializeNumberParts(fmt);
      }
    },
    toggleFormatDropdown() {
      this.isFormatDropdownOpen = !this.isFormatDropdownOpen;
    },
    selectFormat(format) {
      this.selectedFormatId = format.format.id;
      this.numberParts = initializeNumberParts(this.selectedFormat);
      this.isFormatDropdownOpen = false;
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
    onDocumentClick(e) {
      if (!e.target.closest('.format__dropdown')) this.isFormatDropdownOpen = false;
    },
  },
};
</script>

<!-- Стили 1:1 из BlacklistCreateModal (ввод номера), чтобы правка выглядела как создание. -->
<style scoped>
.vnf {
  display: flex;
  flex-direction: column;
}

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

.completion__number {
  flex: 1;
}

.completion__number-header {
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
