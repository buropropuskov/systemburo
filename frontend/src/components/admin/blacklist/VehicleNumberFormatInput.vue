<template>
  <div class="vnf">
    <div class="vnf__format">
      <div class="vnf__header">
        <label class="vnf__label">Формат номеров <span class="vnf__required">*</span></label>
      </div>
      <div class="vnf__dropdown">
        <button
          type="button"
          class="vnf__dropdown-button"
          @click="toggleFormatDropdown"
        >
          <div class="vnf__button-content">
            <span class="vnf__button-text">{{ selectedFormatText }}</span>
            <img
              src="@/assets/icons/arrow.png"
              class="vnf__button-arrow"
              :class="{ 'vnf__button-arrow--open': isFormatDropdownOpen }"
            >
          </div>
        </button>
        <transition name="vnf-dropdown">
          <div
            v-if="isFormatDropdownOpen"
            class="vnf__dropdown-menu"
          >
            <div
              v-for="format in formats"
              :key="format.format.id"
              class="vnf__dropdown-item"
              @click="selectFormat(format)"
            >
              <span class="vnf__item-text">{{ format.format.name }}</span>
            </div>
            <div
              v-if="!formats.length"
              class="vnf__dropdown-item"
            >
              <span class="vnf__item-text">Нет форматов</span>
            </div>
          </div>
        </transition>
      </div>
    </div>

    <div class="vnf__number">
      <div class="vnf__header">
        <label class="vnf__label">Номер Т/С <span class="vnf__required">*</span></label>
      </div>
      <div
        v-if="selectedFormat"
        class="vnf__number-field"
      >
        <input
          v-for="(cell, index) in selectedFormat.cells"
          :key="index"
          v-model="numberParts[index]"
          class="vnf__number-input"
          :placeholder="getPlaceholder(cell)"
          :maxlength="cell.max_length"
          :style="{ width: getInputWidth(cell) }"
          @input="validatePart(index, $event, cell)"
          @blur="formatPart(index, cell)"
        >
      </div>
      <div
        v-else
        class="vnf__no-format"
      >
        Выберите формат номера
      </div>
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client';
import { validatePartValue, formatPartValue, initializeNumberParts } from '@/composables/useNumberFormat';

/**
 * Поячеечный ввод номера Т/С по выбранному формату (формат-дропдаун + ячейки). Единый
 * источник UX номера для создания (BlacklistCreateModal) и правки (AddToBlacklistModal)
 * записи ЧС - раньше правка использовала простой text-input и расходилась с созданием (#481).
 * v-model отдаёт собранную строку номера ("часть часть"). Для правки initialNumber лучшим
 * усилием раскладывается обратно по ячейкам, если число частей совпадает с форматом.
 */
export default {
  name: 'VehicleNumberFormatInput',
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
      if (!e.target.closest('.vnf__dropdown')) this.isFormatDropdownOpen = false;
    },
  },
};
</script>

<style scoped>
.vnf {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  flex-wrap: wrap;
}

.vnf__format {
  display: flex;
  flex-direction: column;
  gap: 6px;
  position: relative;
  flex: 1;
  min-width: 180px;
}

.vnf__number {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.vnf__header {
  display: flex;
  align-items: end;
}

.vnf__label {
  font-size: 13px;
  color: #a2a2a2;
}

.vnf__required {
  color: #ff4444;
}

.vnf__dropdown {
  position: relative;
}

.vnf__dropdown-button {
  width: 100%;
  height: 40px;
  border: 1px solid #e6e6e6;
  background-color: #fff;
  border-radius: 15px;
  outline: none;
  cursor: pointer;
  padding: 0 15px;
  transition: border-color 0.2s;
}

.vnf__dropdown-button:hover {
  border-color: #4f5bdf;
}

.vnf__button-content {
  display: flex;
  align-items: center;
  width: 100%;
  height: 100%;
  justify-content: space-between;
}

.vnf__button-text {
  font-size: 14px;
  color: #000;
  font-weight: 500;
  display: block;
}

.vnf__button-arrow {
  width: 10px;
  height: 10px;
  transition: transform 0.2s;
  transform: rotate(90deg);
  flex-shrink: 0;
}

.vnf__button-arrow--open {
  transform: rotate(-90deg);
}

.vnf__dropdown-menu {
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
  max-height: 300px;
  overflow-y: auto;
}

.vnf__dropdown-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 15px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.vnf__dropdown-item:hover {
  background-color: #f5f5f5;
}

.vnf__item-text {
  font-size: 13px;
  color: #333;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.vnf__number-field {
  height: 40px;
  display: flex;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  overflow: hidden;
  background: #fff;
}

.vnf__number-input {
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

.vnf__number-input:not(:last-child) {
  border-right: 1px solid #e6e6e6;
}

.vnf__number-input::placeholder {
  color: #a2a2a2;
  font-size: 12px;
  text-transform: none;
}

.vnf__number-input:focus {
  background-color: #f8f8f8;
}

.vnf__no-format {
  font-size: 12px;
  color: #a2a2a2;
  text-align: center;
  padding: 10px;
  background: #f8f8f8;
  border-radius: 10px;
  border: 1px solid #e6e6e6;
}

.vnf-dropdown-enter-active,
.vnf-dropdown-leave-active {
  transition: all 0.2s ease;
}

.vnf-dropdown-enter-from,
.vnf-dropdown-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}
</style>
