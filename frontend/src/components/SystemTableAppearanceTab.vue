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
      <div
        v-if="previewFields.length"
        class="appearance-tab__density-preview"
        :class="`appearance-tab__density-preview--${rowDensity}`"
        :style="{ fontSize: fontSize + 'px' }"
        aria-hidden="true"
      >
        <div
          class="appearance-tab__density-preview-row appearance-tab__density-preview-row--head"
          :style="{ gridTemplateColumns: previewGridTemplate }"
        >
          <span
            v-for="f in previewFields"
            :key="`h-${f.field_name}`"
          >{{ fieldLabel(f) }}</span>
        </div>
        <div
          v-for="(row, ri) in previewRows"
          :key="`r-${ri}`"
          class="appearance-tab__density-preview-row"
          :style="{ gridTemplateColumns: previewGridTemplate }"
        >
          <span
            v-for="f in previewFields"
            :key="`${ri}-${f.field_name}`"
          >{{ cellValue(row, f) }}</span>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import { apiRequest } from '@/api/client';
import { useDeletionsStore } from '@/stores/deletions';
import { registerDirtyTracker } from '@/utils/dirtyTracker';
import { generateSampleRows } from '@/utils/tableSamples';

const DEFAULT_FONT_SIZE = 14;
const DEFAULT_DENSITY = 'normal';

const FIELD_LABELS = {
  car_number: 'Номер Т/С',
  car_brand: 'Марка',
  organization: 'Организация',
  unload_place: 'Место разгрузки',
  valid_until: 'Действует до',
  time_range: 'Время',
  status: 'Статус',
  application_id: 'Номер заявки',
  last_name: 'Фамилия',
  first_name: 'Имя',
  middle_name: 'Отчество',
  pass_time: 'Время прохода',
  position: 'Должность',
  citizenship_name: 'Гражданство',
  company: 'Компания',
};

function formatIsoDate(iso) {
  if (typeof iso !== 'string' || iso.length !== 10) return iso || '';
  const [y, m, d] = iso.split('-');
  if (!y || !m || !d) return iso;
  return `${d}.${m}.${y}`;
}

const ROW_FIELD_MAP = {
  car_number: 'car_number',
  car_brand: 'car_brand',
  organization: 'organization_name',
  unload_place: 'unload_place',
  valid_until: row => formatIsoDate(row.entry_date_to),
  time_range: row => row.entry_time_from && row.entry_time_to
    ? `${row.entry_time_from} - ${row.entry_time_to}` : '',
  status: 'status',
  application_id: 'applicationNumber',
  last_name: 'last_name',
  first_name: 'first_name',
  middle_name: 'middle_name',
  pass_time: 'pass_time',
  position: 'position',
  citizenship_name: 'citizenshipName',
  company: 'company',
};

const PREVIEW_MAX_FIELDS = 4;

export default {
  name: 'SystemTableAppearanceTab',
  props: {
    tableId: { type: Number, required: true },
    table: { type: Object, required: true },
    // 'main' - читает/пишет font_size, row_density. 'fact' - font_size_fact,
    // row_density_fact (отдельные настройки для FactTable).
    variant: { type: String, default: 'main' },
    tableType: { type: String, default: '' },
    fields: { type: Array, default: () => [] },
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
    previewFields() {
      const list = (this.fields || [])
        .filter(f => f && f.is_visible !== false)
        .slice()
        .sort((a, b) => (a.display_order ?? 0) - (b.display_order ?? 0));
      return list.slice(0, PREVIEW_MAX_FIELDS);
    },
    previewRows() {
      if (!this.tableType || !this.previewFields.length) return [];
      return generateSampleRows(this.tableType, 2);
    },
    previewGridTemplate() {
      const n = this.previewFields.length;
      if (n <= 1) return '1fr';
      return Array(n).fill('1fr').join(' ');
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
  mounted() {
    this._stopDirtyGuard = registerDirtyTracker({
      isDirty: () => this.isDirty,
      getChanges: () => this.buildChangesList(),
      save: () => this.save(),
    });
  },
  beforeUnmount() {
    this._stopDirtyGuard?.();
  },
  methods: {
    fieldLabel(field) {
      return FIELD_LABELS[field.field_name] || field.label || field.field_name;
    },
    cellValue(row, field) {
      const mapper = ROW_FIELD_MAP[field.field_name];
      if (typeof mapper === 'function') return mapper(row) || '';
      if (typeof mapper === 'string') return row[mapper] ?? '';
      return row[field.field_name] ?? '';
    },
    buildChangesList() {
      const prefix = this.variant === 'fact' ? 'Оформление "По факту"' : 'Оформление';
      const out = [];
      if (this.fontSize !== this.originalFontSize) {
        out.push({
          label: `${prefix}: размер шрифта`,
          from: `${this.originalFontSize}px`,
          to: `${this.fontSize}px`,
        });
      }
      if (this.rowDensity !== this.originalDensity) {
        const label = v => this.densityOptions.find(o => o.value === v)?.label || v;
        out.push({
          label: `${prefix}: плотность строк`,
          from: label(this.originalDensity),
          to: label(this.rowDensity),
        });
      }
      return out;
    },
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
        useDeletionsStore().notify({ prefix: 'Оформление ', bold: 'сохранено' });
        this.$emit('update');
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Ошибка сохранения: ', bold: e.message, type: 'error' });
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
  gap: 15px;
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
  color: var(--text);
}

.appearance-tab__section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.appearance-tab__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}

.appearance-tab__hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
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
  accent-color: var(--accent-text);
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
  color: var(--accent-text);
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
  border: 1px solid var(--border);
  border-radius: 15px;
  background: var(--surface-2);
  color: var(--text);
  font-family: 'Montserrat', sans-serif;
  font-weight: 500;
  line-height: 1;
  flex-shrink: 0;
  overflow: hidden;
}

.appearance-tab__segment {
  display: inline-flex;
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
  background: var(--surface-2);
  width: fit-content;
}

.appearance-tab__segment-btn {
  padding: 8px 16px;
  background: transparent;
  border: none;
  border-right: 1px solid var(--border);
  font-size: 13px;
  color: var(--text-muted);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.appearance-tab__segment-btn:last-child {
  border-right: none;
}

.appearance-tab__segment-btn:hover:not(.appearance-tab__segment-btn--active) {
  background: var(--border);
  color: var(--text);
}

.appearance-tab__segment-btn--active {
  background: var(--accent);
  color: var(--accent-contrast);
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
  background: var(--surface);
  border-color: var(--border);
  color: var(--text);
}

.appearance-tab__btn--secondary:hover:not(:disabled) {
  background: var(--surface-2);
  border-color: var(--border);
}

.appearance-tab__btn--primary {
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

.appearance-tab__btn--primary:hover:not(:disabled) {
  background: var(--accent-hover);
  border-color: var(--accent);
}

.appearance-tab__status {
  margin: 0;
  font-size: 13px;
  color: var(--success-text);
  text-align: right;
}

.appearance-tab__status--error {
  color: var(--danger-text);
}

.appearance-tab__density-preview {
  margin-top: 14px;
  border: 1px solid var(--border);
  border-radius: 12px;
  overflow: hidden;
  background: var(--surface);
}

.appearance-tab__density-preview-row {
  display: grid;
  gap: 12px;
  align-items: center;
  padding: 6px 14px;
  color: var(--text-muted);
  border-bottom: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  transition: padding 0.25s ease, min-height 0.25s ease;
}

.appearance-tab__density-preview-row span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.appearance-tab__density-preview-row:last-child {
  border-bottom: none;
}

.appearance-tab__density-preview-row--head {
  background: var(--accent-tint);
  color: var(--text-muted);
  font-weight: 600;
}

.appearance-tab__density-preview--compact .appearance-tab__density-preview-row {
  padding-top: 3px;
  padding-bottom: 3px;
  min-height: 28px;
}

.appearance-tab__density-preview--normal .appearance-tab__density-preview-row {
  padding-top: 6px;
  padding-bottom: 6px;
  min-height: 36px;
}

.appearance-tab__density-preview--spacious .appearance-tab__density-preview-row {
  padding-top: 12px;
  padding-bottom: 12px;
  min-height: 50px;
}
</style>
