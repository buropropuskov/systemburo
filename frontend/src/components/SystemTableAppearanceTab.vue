<template>
  <div class="appearance-tab">
    <div class="appearance-tab__header-top">
      <h4 class="appearance-tab__header-title">
        Оформление таблицы
      </h4>
      <div class="appearance-tab__actions">
        <button
          class="appearance-tab__btn appearance-tab__btn--secondary"
          :disabled="!isDirty || saving"
          @click="reset"
        >
          Отменить
        </button>
        <button
          class="appearance-tab__btn appearance-tab__btn--primary"
          :disabled="!isDirty || saving"
          data-testid="appearance-save"
          @click="save"
        >
          {{ saving ? 'Сохраняем...' : 'Сохранить' }}
        </button>
      </div>
    </div>
    <div class="appearance-tab__section">
      <h4 class="appearance-tab__title">
        Размер шрифта строк
      </h4>
      <p class="appearance-tab__hint">
        Действует на содержимое строк CarsTable и PeopleTable. Шрифт фиксирован - Montserrat.
      </p>
      <div class="appearance-tab__row">
        <input
          v-model.number="fontSize"
          type="range"
          min="10"
          max="24"
          step="1"
          class="appearance-tab__slider"
          data-testid="appearance-fontsize-slider"
        >
        <span
          class="appearance-tab__value"
          data-testid="appearance-fontsize-value"
        >{{ fontSize }}px</span>
        <span
          class="appearance-tab__preview-badge"
          :style="{ fontSize: fontSize + 'px' }"
          data-testid="appearance-fontsize-preview"
        >Текст</span>
      </div>
    </div>

    <div class="appearance-tab__section">
      <h4 class="appearance-tab__title">
        Плотность строк
      </h4>
      <p class="appearance-tab__hint">
        Управляет вертикальными отступами в ячейках.
      </p>
      <div
        class="appearance-tab__segment"
        role="radiogroup"
        aria-label="Плотность строк"
      >
        <button
          v-for="opt in densityOptions"
          :key="opt.value"
          type="button"
          class="appearance-tab__segment-btn"
          :class="{ 'appearance-tab__segment-btn--active': rowDensity === opt.value }"
          :data-density="opt.value"
          :aria-pressed="rowDensity === opt.value"
          @click="rowDensity = opt.value"
        >
          {{ opt.label }}
        </button>
      </div>
    </div>

  </div>
</template>

<script>
import { apiRequest } from '@/api/client';
import { useToast } from '@/composables/useToast';

const DEFAULT_FONT_SIZE = 14;
const DEFAULT_DENSITY = 'normal';

export default {
  name: 'SystemTableAppearanceTab',
  setup() {
    return { toast: useToast() };
  },
  props: {
    tableId: { type: Number, required: true },
    table: { type: Object, required: true },
    // 'main' - читает/пишет font_size, row_density. 'fact' - font_size_fact,
    // row_density_fact (отдельные настройки для FactTable).
    variant: { type: String, default: 'main' },
  },
  emits: ['update'],
  data() {
    return {
      fontSize: DEFAULT_FONT_SIZE,
      rowDensity: DEFAULT_DENSITY,
      originalFontSize: DEFAULT_FONT_SIZE,
      originalDensity: DEFAULT_DENSITY,
      saving: false,
      densityOptions: [
        { value: 'compact', label: 'Компактно' },
        { value: 'normal', label: 'Обычно' },
        { value: 'spacious', label: 'Просторно' },
      ],
    };
  },
  computed: {
    fontSizeKey() {
      return this.variant === 'fact' ? 'font_size_fact' : 'font_size';
    },
    densityKey() {
      return this.variant === 'fact' ? 'row_density_fact' : 'row_density';
    },
    isDirty() {
      return this.fontSize !== this.originalFontSize
        || this.rowDensity !== this.originalDensity;
    },
  },
  watch: {
    table: {
      handler() {
        this.reset();
      },
      immediate: true,
      deep: true,
    },
  },
  methods: {
    reset() {
      const fs = Number(this.table?.[this.fontSizeKey]);
      this.fontSize = fs >= 10 && fs <= 24 ? fs : DEFAULT_FONT_SIZE;
      const dens = this.table?.[this.densityKey];
      this.rowDensity = ['compact', 'normal', 'spacious'].includes(dens) ? dens : DEFAULT_DENSITY;
      this.originalFontSize = this.fontSize;
      this.originalDensity = this.rowDensity;
    },
    async save() {
      if (!this.isDirty || this.saving) return;
      this.saving = true;
      try {
        const fs = Math.max(10, Math.min(24, Number(this.fontSize) || DEFAULT_FONT_SIZE));
        const response = await apiRequest(`/system-tables/${this.tableId}`, {
          method: 'PUT',
          body: JSON.stringify({
            [this.fontSizeKey]: fs,
            [this.densityKey]: this.rowDensity,
          }),
        });
        if (!response.ok) {
          const err = await response.json().catch(() => ({}));
          throw new Error(err.message || `HTTP ${response.status}`);
        }
        this.originalFontSize = fs;
        this.originalDensity = this.rowDensity;
        this.toast.success('Оформление сохранено');
        this.$emit('update');
      } catch (e) {
        this.toast.error(`Ошибка сохранения: ${e.message}`);
      } finally {
        this.saving = false;
      }
    },
  },
};
</script>

<style scoped>
.appearance-tab {
  display: flex;
  flex-direction: column;
  gap: 20px;
  line-height: 1.5;
}

.appearance-tab__header-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.appearance-tab__header-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #000;
}

.appearance-tab__section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.appearance-tab__title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: #000;
}

.appearance-tab__hint {
  margin: 0;
  font-size: 12px;
  color: #6b7280;
  line-height: 1.5;
}

.appearance-tab__row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 0;
}

.appearance-tab__slider {
  flex: 1;
  accent-color: #4F5BDF;
  cursor: grab;
}

.appearance-tab__slider:active {
  cursor: grabbing;
}

/* WebKit/Blink (Chrome, Edge, Safari) - кастомизация thumb cursor. */
.appearance-tab__slider::-webkit-slider-thumb {
  cursor: grab;
}

.appearance-tab__slider:active::-webkit-slider-thumb {
  cursor: grabbing;
}

/* Firefox */
.appearance-tab__slider::-moz-range-thumb {
  cursor: grab;
}

.appearance-tab__slider:active::-moz-range-thumb {
  cursor: grabbing;
}

.appearance-tab__value {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
  font-weight: 500;
  color: #4F5BDF;
  min-width: 48px;
  text-align: right;
}

/* Live-preview шрифта: бейдж 'Текст' с size = fontSize. Фиксированная
   ширина 80px рассчитана под max шрифт 24px - гарантирует что бейдж не
   ломает layout и сам не растягивается. */
.appearance-tab__preview-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 80px;
  height: 36px;
  padding: 0 8px;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  background: #fafafa;
  color: #333;
  font-family: 'Montserrat', sans-serif;
  font-weight: 500;
  line-height: 1;
  flex-shrink: 0;
  overflow: hidden;
}

.appearance-tab__segment {
  display: inline-flex;
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  overflow: hidden;
  background: #fafafa;
  width: fit-content;
}

.appearance-tab__segment-btn {
  padding: 8px 16px;
  background: transparent;
  border: none;
  border-right: 1px solid #e6e6e6;
  font-size: 13px;
  color: #6b7280;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.appearance-tab__segment-btn:last-child {
  border-right: none;
}

.appearance-tab__segment-btn:hover:not(.appearance-tab__segment-btn--active) {
  background: #f0f0f0;
  color: #333;
}

.appearance-tab__segment-btn--active {
  background: #4F5BDF;
  color: #fff;
  font-weight: 500;
}

.appearance-tab__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.appearance-tab__btn {
  padding: 8px 18px;
  border-radius: 14px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid transparent;
}

.appearance-tab__btn:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.appearance-tab__btn--secondary {
  background: #fff;
  border-color: #e6e6e6;
  color: #333;
}

.appearance-tab__btn--secondary:hover:not(:disabled) {
  background: #f5f5f5;
  border-color: #c0c0c0;
}

.appearance-tab__btn--primary {
  background: #4F5BDF;
  color: #fff;
  border-color: #4F5BDF;
}

.appearance-tab__btn--primary:hover:not(:disabled) {
  background: #3b48c4;
  border-color: #3b48c4;
}

.appearance-tab__status {
  margin: 0;
  font-size: 13px;
  color: #079D1D;
  text-align: right;
}

.appearance-tab__status--error {
  color: #c62828;
}
</style>
