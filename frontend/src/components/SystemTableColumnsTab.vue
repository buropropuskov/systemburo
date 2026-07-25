<template>
  <div class="columns-tab">
    <div class="columns-tab__header">
      <div class="columns-tab__header-top">
        <h4 class="columns-tab__title">
          {{ enlargedMode ? 'Столбцы в увеличенном режиме' : 'Видимые столбцы' }}
        </h4>
        <div
          class="columns-tab__mode-toggle"
          role="radiogroup"
          aria-label="Режим настройки"
        >
          <button
            type="button"
            class="columns-tab__mode-btn"
            :class="{ 'columns-tab__mode-btn--active': !enlargedMode }"
            @click="enlargedMode = false"
          >
            Обычный
          </button>
          <button
            type="button"
            class="columns-tab__mode-btn"
            :class="{ 'columns-tab__mode-btn--active': enlargedMode }"
            @click="enlargedMode = true"
          >
            Увеличенный
          </button>
        </div>
        <div class="columns-tab__actions">
          <button
            class="columns-tab__btn columns-tab__btn--secondary"
            :disabled="!isDirty || saving"
            @click="reset"
          >
            Отменить
          </button>
          <button
            class="columns-tab__btn columns-tab__btn--primary"
            :disabled="!isDirty || saving"
            @click="save"
          >
            {{ saving ? 'Сохраняем...' : 'Сохранить' }}
          </button>
        </div>
      </div>
      <div class="columns-tab__hint-block">
        <template v-if="!enlargedMode">
          <div class="columns-tab__hint">
            <span class="columns-tab__hint-badge">Порядок</span>
            <span class="columns-tab__hint-dash">-</span>
            <span class="columns-tab__hint-text">очерёдность столбцов в таблице. Перетащите за иконку слева, чтобы переставить.</span>
          </div>
          <div class="columns-tab__hint">
            <span class="columns-tab__hint-badge">Ширина</span>
            <span class="columns-tab__hint-dash">-</span>
            <span class="columns-tab__hint-text">размер столбца в таблице. Определяет сколько пространства занимает среди остальных видимых.</span>
          </div>
          <div class="columns-tab__hint">
            <span class="columns-tab__hint-badge">Приоритет 1-5</span>
            <span class="columns-tab__hint-dash">-</span>
            <span class="columns-tab__hint-text">управляет порядком скрытия столбцов в портретном режиме. <strong>1</strong> - всегда виден, <strong>3-5</strong> - скрывается первым под кнопку "Подробнее".</span>
          </div>
          <p class="columns-tab__hint-note">
            <strong>Портретный режим</strong> - формат таблицы, когда ширины не хватает на все столбцы одновременно. Бывает на телефонах в вертикальной ориентации, в разделённых экранах и в узких окнах десктопа.
          </p>
        </template>
        <template v-else>
          <div class="columns-tab__hint">
            <span class="columns-tab__hint-badge">Применение</span>
            <span class="columns-tab__hint-dash">-</span>
            <span class="columns-tab__hint-text">настройки работают только когда пользователь сам включил Увеличенный режим в просмотре таблицы.</span>
          </div>
          <div class="columns-tab__hint">
            <span class="columns-tab__hint-badge">Ширина <strong>0</strong></span>
            <span class="columns-tab__hint-dash">-</span>
            <span class="columns-tab__hint-text">значение <strong>0</strong> означает "взять ширину из вкладки Обычный". Любое другое число задаёт собственную ширину для увеличенного режима.</span>
          </div>
          <div class="columns-tab__hint">
            <span class="columns-tab__hint-badge">Жирность <strong>0</strong></span>
            <span class="columns-tab__hint-dash">-</span>
            <span class="columns-tab__hint-text">значение <strong>0</strong> означает наследовать дефолтную жирность (<strong>500</strong>). Значения от <strong>100</strong> до <strong>900</strong> задают свою.</span>
          </div>
          <p class="columns-tab__hint-note">
            Серая строка с галочкой - столбец отключён в обычном режиме.
          </p>
        </template>
      </div>
    </div>

    <div
      v-if="!localFields.length"
      class="columns-tab__empty"
    >
      У этой таблицы пока нет настраиваемых столбцов.
    </div>

    <TransitionGroup
      v-else
      tag="ul"
      name="cols-list"
      class="columns-tab__list"
    >
      <li
        v-for="(field, index) in localFields"
        :key="field.field_name"
        class="columns-tab__item"
        :class="{
          'columns-tab__item--off': !field.is_visible,
          'columns-tab__item--dragging': draggingIndex === index,
        }"
        :data-field="field.field_name"
        :data-index="index"
      >
        <div class="columns-tab__row">
          <span
            class="columns-tab__handle"
            :title="'Перетащите для смены порядка'"
            aria-hidden="true"
            @pointerdown="onItemPointerDown(index, $event)"
          >
            <svg
              width="14"
              height="14"
              viewBox="0 0 14 14"
              fill="none"
            >
              <circle
                cx="4.5"
                cy="3"
                r="1.2"
                fill="currentColor"
              />
              <circle
                cx="9.5"
                cy="3"
                r="1.2"
                fill="currentColor"
              />
              <circle
                cx="4.5"
                cy="7"
                r="1.2"
                fill="currentColor"
              />
              <circle
                cx="9.5"
                cy="7"
                r="1.2"
                fill="currentColor"
              />
              <circle
                cx="4.5"
                cy="11"
                r="1.2"
                fill="currentColor"
              />
              <circle
                cx="9.5"
                cy="11"
                r="1.2"
                fill="currentColor"
              />
            </svg>
          </span>
          <label class="columns-tab__check-row">
            <input
              v-if="!enlargedMode"
              v-model="field.is_visible"
              type="checkbox"
              class="columns-tab__checkbox"
              :data-field="field.field_name"
            >
            <input
              v-else
              v-model="field.enlarged_is_visible"
              type="checkbox"
              class="columns-tab__checkbox"
              :data-field-enlarged="field.field_name"
            >
            <span class="columns-tab__label">{{ humanLabel(field.field_name) }}</span>
            <span class="columns-tab__field-name">{{ field.field_name }}</span>
          </label>
          <div
            class="columns-tab__width"
            :title="enlargedMode ? 'Ширина в увеличенном режиме (0 = брать обычную)' : 'Ширина столбца (flex-grow). Браузер делит ширину пропорционально весам видимых столбцов.'"
          >
            <span class="columns-tab__width-label">Ширина:</span>
            <input
              v-if="!enlargedMode"
              v-model.number="field.width"
              type="number"
              min="1"
              max="100"
              class="columns-tab__width-input"
              :data-width-field="field.field_name"
            >
            <input
              v-else
              v-model.number="field.enlarged_width"
              type="number"
              min="0"
              max="100"
              class="columns-tab__width-input"
              :data-enlarged-width-field="field.field_name"
            >
          </div>
          <div
            v-if="!enlargedMode"
            class="columns-tab__priority"
            :title="'Приоритет в портретном режиме (1-5). 1 = всегда виден, 2 = виден на узком экране, 3-5 = скрывается в портрете и доступен по кнопке Подробнее.'"
          >
            <span class="columns-tab__priority-label">Приоритет:</span>
            <select
              v-model.number="field.priority"
              class="columns-tab__priority-select"
              :data-priority-field="field.field_name"
            >
              <option
                v-for="n in 5"
                :key="n"
                :value="n"
              >
                {{ n }}
              </option>
            </select>
          </div>
          <div
            v-else
            class="columns-tab__priority columns-tab__weight"
            :title="'Жирность шрифта в увеличенном режиме. 0 = по умолчанию (500).'"
          >
            <span class="columns-tab__priority-label">Жирность:</span>
            <span
              class="columns-tab__weight-preview"
              :style="{ fontWeight: field.enlarged_font_weight || 500 }"
            >Текст</span>
            <select
              v-model.number="field.enlarged_font_weight"
              class="columns-tab__priority-select columns-tab__weight-select"
              :data-enlarged-weight-field="field.field_name"
            >
              <option :value="0">0</option>
              <option :value="400">400</option>
              <option :value="500">500</option>
              <option :value="600">600</option>
              <option :value="700">700</option>
              <option :value="800">800</option>
            </select>
          </div>
        </div>
      </li>
    </TransitionGroup>

    <div
      v-if="localFields.length && variant === 'main'"
      class="columns-tab__preview"
    >
      <h4 class="columns-tab__preview-title">
        Предпросмотр
      </h4>
      <p class="columns-tab__preview-hint">
        Так таблица будет выглядеть с текущими настройками (примерные данные).
      </p>
      <div class="columns-tab__preview-frame">
        <!--
          Преview - точная копия реальной CarsTable/PeopleTable, пропорционально уменьшенная
          через CSS transform: scale. Inner-элемент рендерится при ширине 100%/scale
          (с естественным размером шрифта и отступов), затем визуально масштабируется.
        -->
        <div class="columns-tab__preview-scale">
          <CarsTable
            v-if="tableType === 'cars'"
            :preview="true"
            :preview-fields="previewFieldsWithOrder"
            :preview-items="sampleItems"
            :table-id="tableId"
          />
          <PeopleTable
            v-else-if="tableType === 'people'"
            :preview="true"
            :preview-fields="previewFieldsWithOrder"
            :preview-items="sampleItems"
            :table-name="''"
          />
          <div
            v-else
            class="columns-tab__preview-unknown"
          >
            Превью недоступно для этого типа таблицы.
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client';
import { generateSampleRows } from '@/utils/tableSamples';
import { useDeletionsStore } from '@/stores/deletions';
import { registerDirtyTracker } from '@/utils/dirtyTracker';
import CarsTable from './CarsTable.vue';
import PeopleTable from './PeopleTable.vue';

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

export default {
  name: 'SystemTableColumnsTab',
  components: { CarsTable, PeopleTable },
  props: {
    tableId: { type: Number, required: true },
    tableType: { type: String, required: true },
    fields: { type: Array, required: true },
    // 'main' = настройки обычной CarsTable/PeopleTable (endpoint /fields).
    // 'fact' = настройки FactTable (endpoint /fact-fields).
    variant: { type: String, default: 'main' },
  },
  emits: ['update'],
  data() {
    return {
      localFields: [],
      originalVisibility: {},
      originalOrder: [],
      originalWidth: {},
      originalPriority: {},
      originalEnlargedVisibility: {},
      originalEnlargedWidth: {},
      originalEnlargedWeight: {},
      saving: false,
      draggingIndex: null,
      prevBodyCursor: '',
      enlargedMode: false,
    };
  },
  computed: {
    apiPath() {
      return this.variant === 'fact'
        ? `/system-tables/${this.tableId}/fact-fields`
        : `/system-tables/${this.tableId}/fields`;
    },
    previewFieldsWithOrder() {
      // Передаём текущий локальный порядок+ширину в превью, чтобы оно реагировало до save.
      return this.localFields.map((f, i) => ({
        field_name: f.field_name,
        is_visible: f.is_visible,
        display_order: i,
        width: f.width,
        priority: f.priority,
      }));
    },
    sampleItems() {
      return generateSampleRows(this.tableType, 5);
    },
    isDirty() {
      const visibilityChanged = this.localFields.some(
        f => this.originalVisibility[f.field_name] !== f.is_visible,
      );
      const orderChanged = this.localFields.some(
        (f, i) => this.originalOrder[i] !== f.field_name,
      );
      const widthChanged = this.localFields.some(
        f => this.originalWidth[f.field_name] !== f.width,
      );
      const priorityChanged = this.localFields.some(
        f => this.originalPriority[f.field_name] !== f.priority,
      );
      const enlargedVisChanged = this.localFields.some(
        f => this.originalEnlargedVisibility[f.field_name] !== f.enlarged_is_visible,
      );
      const enlargedWChanged = this.localFields.some(
        f => this.originalEnlargedWidth[f.field_name] !== f.enlarged_width,
      );
      const enlargedWeightChanged = this.localFields.some(
        f => this.originalEnlargedWeight[f.field_name] !== f.enlarged_font_weight,
      );
      return visibilityChanged || orderChanged || widthChanged || priorityChanged
        || enlargedVisChanged || enlargedWChanged || enlargedWeightChanged;
    },
  },
  watch: {
    fields: {
      handler() {
        this.reset();
      },
      immediate: true,
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
    // На случай если пользователь ушёл с вкладки во время drag.
    this.cleanupPointerListeners();
    this._stopDirtyGuard?.();
  },
  methods: {
    humanLabel(name) {
      return FIELD_LABELS[name] || name;
    },
    buildChangesList() {
      const prefix = this.variant === 'fact' ? 'Колонки "По факту": ' : 'Колонки: ';
      const out = [];
      const visDiff = this.localFields.filter(
        f => this.originalVisibility[f.field_name] !== f.is_visible,
      );
      if (visDiff.length) {
        const names = visDiff.slice(0, 3).map(f => `"${this.humanLabel(f.field_name)}"`).join(', ');
        const more = visDiff.length > 3 ? ` и ещё ${visDiff.length - 3}` : '';
        out.push(`${prefix}изменена видимость ${names}${more}`);
      }
      const orderChanged = this.localFields.some(
        (f, i) => this.originalOrder[i] !== f.field_name,
      );
      if (orderChanged) out.push(`${prefix}изменён порядок столбцов`);
      const widthDiff = this.localFields.filter(
        f => this.originalWidth[f.field_name] !== f.width,
      );
      if (widthDiff.length) out.push(`${prefix}изменена ширина (${widthDiff.length})`);
      const priorityDiff = this.localFields.filter(
        f => this.originalPriority[f.field_name] !== f.priority,
      );
      if (priorityDiff.length) out.push(`${prefix}изменён приоритет (${priorityDiff.length})`);
      const enlPrefix = this.variant === 'fact'
        ? 'Увеличенный (По факту): '
        : 'Увеличенный режим: ';
      const enlVisDiff = this.localFields.filter(
        f => this.originalEnlargedVisibility[f.field_name] !== f.enlarged_is_visible,
      );
      if (enlVisDiff.length) {
        const names = enlVisDiff.slice(0, 3).map(f => `"${this.humanLabel(f.field_name)}"`).join(', ');
        const more = enlVisDiff.length > 3 ? ` и ещё ${enlVisDiff.length - 3}` : '';
        out.push(`${enlPrefix}изменена видимость ${names}${more}`);
      }
      const enlWDiff = this.localFields.filter(
        f => this.originalEnlargedWidth[f.field_name] !== f.enlarged_width,
      );
      if (enlWDiff.length) out.push(`${enlPrefix}изменена ширина (${enlWDiff.length})`);
      const enlWeightDiff = this.localFields.filter(
        f => this.originalEnlargedWeight[f.field_name] !== f.enlarged_font_weight,
      );
      if (enlWeightDiff.length) out.push(`${enlPrefix}изменена жирность (${enlWeightDiff.length})`);
      return out;
    },
    reset() {
      const sorted = [...(this.fields || [])].sort((a, b) => {
        const ao = a.display_order ?? 0;
        const bo = b.display_order ?? 0;
        return ao - bo;
      });
      this.localFields = sorted.map(f => ({
        field_name: f.field_name,
        is_visible: f.is_visible !== false,
        width: typeof f.width === 'number' && f.width > 0 ? f.width : 10,
        priority: typeof f.priority === 'number' && f.priority > 0 ? f.priority : 3,
        enlarged_is_visible: f.enlarged_is_visible !== false,
        enlarged_width: typeof f.enlarged_width === 'number' ? f.enlarged_width : 0,
        enlarged_font_weight: typeof f.enlarged_font_weight === 'number' ? f.enlarged_font_weight : 0,
      }));
      this.originalVisibility = Object.fromEntries(
        this.localFields.map(f => [f.field_name, f.is_visible]),
      );
      this.originalOrder = this.localFields.map(f => f.field_name);
      this.originalWidth = Object.fromEntries(
        this.localFields.map(f => [f.field_name, f.width]),
      );
      this.originalPriority = Object.fromEntries(
        this.localFields.map(f => [f.field_name, f.priority]),
      );
      this.originalEnlargedVisibility = Object.fromEntries(
        this.localFields.map(f => [f.field_name, f.enlarged_is_visible]),
      );
      this.originalEnlargedWidth = Object.fromEntries(
        this.localFields.map(f => [f.field_name, f.enlarged_width]),
      );
      this.originalEnlargedWeight = Object.fromEntries(
        this.localFields.map(f => [f.field_name, f.enlarged_font_weight]),
      );
    },
    /**
     * Pointer-events DnD (вместо HTML5 native, который не даёт контроля над курсором).
     * - pointerdown на handle: запускаем drag, ставим body cursor=grabbing.
     * - pointermove document: определяем элемент под курсором, midpoint-swap.
     * - pointerup document: останавливаем drag, восстанавливаем курсор.
     */
    onItemPointerDown(index, event) {
      event.preventDefault();
      this.draggingIndex = index;
      // Глобальный курсор - захватываем body, чтобы grabbing был везде куда мышь идёт.
      this.prevBodyCursor = document.body.style.cursor;
      document.body.style.cursor = 'grabbing';
      document.body.style.userSelect = 'none';
      document.addEventListener('pointermove', this.onPointerMove);
      document.addEventListener('pointerup', this.onPointerUp);
      document.addEventListener('pointercancel', this.onPointerUp);
    },
    onPointerMove(event) {
      if (this.draggingIndex === null) return;
      // Находим <li> под курсором.
      const target = document.elementFromPoint(event.clientX, event.clientY);
      if (!target) return;
      const li = target.closest('.columns-tab__item');
      if (!li) return;
      const targetIndex = Number(li.dataset.index);
      if (Number.isNaN(targetIndex) || targetIndex === this.draggingIndex) return;
      // Midpoint-алгоритм: swap только при пересечении середины цели в нужном направлении.
      const rect = li.getBoundingClientRect();
      const midpoint = rect.top + rect.height / 2;
      const movingDown = this.draggingIndex < targetIndex;
      if (movingDown && event.clientY < midpoint) return;
      if (!movingDown && event.clientY > midpoint) return;
      const arr = this.localFields.slice();
      const [moved] = arr.splice(this.draggingIndex, 1);
      arr.splice(targetIndex, 0, moved);
      this.localFields = arr;
      this.draggingIndex = targetIndex;
    },
    onPointerUp() {
      this.draggingIndex = null;
      this.cleanupPointerListeners();
    },
    cleanupPointerListeners() {
      document.removeEventListener('pointermove', this.onPointerMove);
      document.removeEventListener('pointerup', this.onPointerUp);
      document.removeEventListener('pointercancel', this.onPointerUp);
      document.body.style.cursor = this.prevBodyCursor || '';
      document.body.style.userSelect = '';
    },
    async save() {
      if (!this.isDirty || this.saving) return;
      this.saving = true;
      try {
        const response = await apiRequest(this.apiPath, {
          method: 'PUT',
          body: JSON.stringify({
            fields: this.localFields.map((f, i) => ({
              field_name: f.field_name,
              is_visible: f.is_visible,
              display_order: i,
              width: Math.max(1, Math.min(100, Number(f.width) || 10)),
              priority: Math.max(1, Math.min(5, Number(f.priority) || 3)),
              enlarged_is_visible: f.enlarged_is_visible !== false,
              enlarged_width: Math.max(0, Math.min(100, Number(f.enlarged_width) || 0)),
              enlarged_font_weight: Number(f.enlarged_font_weight) || 0,
            })),
          }),
        });
        if (!response.ok) {
          const err = await response.json().catch(() => ({}));
          throw new Error(err.message || `HTTP ${response.status}`);
        }
        useDeletionsStore().notify({ prefix: 'Настройки столбцов ', bold: 'сохранены' });
        this.originalVisibility = Object.fromEntries(
          this.localFields.map(f => [f.field_name, f.is_visible]),
        );
        this.originalOrder = this.localFields.map(f => f.field_name);
        this.originalWidth = Object.fromEntries(
          this.localFields.map(f => [f.field_name, f.width]),
        );
        this.originalPriority = Object.fromEntries(
          this.localFields.map(f => [f.field_name, f.priority]),
        );
        this.originalEnlargedVisibility = Object.fromEntries(
          this.localFields.map(f => [f.field_name, f.enlarged_is_visible]),
        );
        this.originalEnlargedWidth = Object.fromEntries(
          this.localFields.map(f => [f.field_name, f.enlarged_width]),
        );
        this.originalEnlargedWeight = Object.fromEntries(
          this.localFields.map(f => [f.field_name, f.enlarged_font_weight]),
        );
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
.columns-tab {
  display: flex;
  flex-direction: column;
  gap: 15px;
  line-height: 1.5;
}

.columns-tab__header {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

.columns-tab__header-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

/* Защита от прыжков: тайтл фиксируем по более длинному варианту, чтобы
   toggle "Обычный | Увеличенный" не сдвигался при переключении режима. */
.columns-tab__title {
  min-width: 290px;
}

/* Блок подсказок фиксированной высоты - предотвращает скачки высоты вкладки
   при смене режима, где у подсказок разная длина текста. */
.columns-tab__hint-block {
  min-height: 100px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

/* Toggle "Обычный | Увеличенный" - сегмент-контрол как в Оформление. */
.columns-tab__mode-toggle {
  display: inline-flex;
  border: 1px solid var(--border);
  border-radius: 50px;
  overflow: hidden;
  background: var(--surface-2);
}

.columns-tab__mode-btn {
  padding: 6px 14px;
  background: transparent;
  border: none;
  font-size: 12px;
  color: var(--text-muted);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.columns-tab__mode-btn:hover:not(.columns-tab__mode-btn--active) {
  background: var(--border);
  color: var(--text);
}

.columns-tab__mode-btn--active {
  background: var(--accent);
  color: var(--accent-contrast);
  font-weight: 500;
}

.columns-tab__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}

.columns-tab__hint {
  display: flex;
  align-items: baseline;
  gap: 10px;
  font-size: 13px;
  color: var(--text-muted);
  line-height: 1.45;
}

.columns-tab__hint-badge {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  background: var(--accent);
  color: var(--accent-contrast);
  font-size: 11px;
  font-weight: 600;
  padding: 2px 9px;
  border-radius: 999px;
  letter-spacing: 0.02em;
}

.columns-tab__hint-badge strong {
  font-weight: 700;
  margin: 0 2px;
}

.columns-tab__hint-dash {
  flex-shrink: 0;
  color: var(--text-muted);
}

.columns-tab__hint-text {
  min-width: 0;
  flex: 1;
}

.columns-tab__hint-text strong {
  color: var(--text);
  font-weight: 600;
}

.columns-tab__hint-note {
  margin: 4px 0 0;
  padding-top: 6px;
  border-top: 1px dashed var(--border);
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.4;
}

.columns-tab__hint-note strong {
  color: var(--accent-text);
  font-weight: 600;
}

.columns-tab__empty {
  padding: 24px;
  text-align: center;
  color: var(--text-muted);
  background: var(--accent-tint);
  border-radius: 12px;
}

.columns-tab__list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.columns-tab__item {
  border: 1px solid var(--border);
  border-radius: 10px;
  transition: opacity 0.15s ease, background-color 0.2s ease;
  background: var(--surface);
}

.columns-tab__item:hover {
  background: var(--accent-tint);
}

.columns-tab__item--off {
  background: var(--surface-2);
  border-color: var(--border);
}

.columns-tab__item--dragging {
  /* Полупрозрачный placeholder в "родной" позиции, чтобы было видно откуда
     тянем. transition: none убирает анимацию перетаскиваемого узла -
     дрожания между DOM и ghost-картинкой курсора нет. */
  opacity: 0.4;
  background: var(--accent-tint);
  border-color: var(--accent);
  border-style: dashed;
  transition: none !important;
}

.columns-tab__row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  user-select: none;
}

.columns-tab__handle {
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  cursor: grab;
  flex-shrink: 0;
  transition: color 0.15s ease;
}

.columns-tab__handle:hover {
  color: var(--accent-text);
}

.columns-tab__item--dragging .columns-tab__handle,
.columns-tab__handle:active {
  cursor: grabbing;
}

.columns-tab__check-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  cursor: pointer;
  padding: 4px;
}

.columns-tab__checkbox {
  width: 12px;
  height: 12px;
  accent-color: var(--accent-text);
  cursor: pointer;
  flex-shrink: 0;
}

.columns-tab__label {
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
  flex: 1;
}

.columns-tab__item--off .columns-tab__label {
  color: var(--text-muted);
}

.columns-tab__field-name {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 11px;
  color: var(--text-muted);
}

.columns-tab__width {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
  padding-right: 6px;
}

.columns-tab__width-label {
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
}

.columns-tab__width-input {
  width: 32px;
  padding: 2px 4px;
  border: 1px solid var(--border);
  border-radius: 6px;
  font-size: 11px;
  text-align: right;
  color: var(--text);
  background: var(--surface);
  -moz-appearance: textfield;
}

.columns-tab__width-input::-webkit-outer-spin-button,
.columns-tab__width-input::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.columns-tab__width-input:focus {
  outline: none;
  border-color: var(--accent);
}

.columns-tab__priority {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  padding-right: 4px;
}

.columns-tab__priority-label {
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
}

.columns-tab__priority-select {
  width: 38px;
  padding: 2px 4px;
  border: 1px solid var(--border);
  border-radius: 6px;
  font-size: 11px;
  color: var(--text);
  background: var(--surface);
  cursor: pointer;
}

.columns-tab__priority-select:focus {
  outline: none;
  border-color: var(--accent);
}

/* Селект "Жирность" - шире обычного priority-select, чтобы трёхзначные значения
   (700, 800) не наезжали на стрелочку. */
.columns-tab__weight-select {
  width: 56px;
  padding-right: 6px;
}

/* Бейдж предпросмотра жирности рядом с селектом.
   Высота не больше select - paddingи и font-size совпадают с .columns-tab__priority-select. */
.columns-tab__weight-preview {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 42px;
  padding: 2px 6px;
  border: 1px solid var(--border);
  border-radius: 6px;
  font-size: 11px;
  color: var(--text);
  background: var(--surface);
  white-space: nowrap;
  line-height: 1;
}

.columns-tab__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.columns-tab__btn {
  padding: 8px 18px;
  border-radius: 14px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid transparent;
}

.columns-tab__btn:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.columns-tab__btn--secondary {
  background: var(--surface);
  border-color: var(--border);
  color: var(--text);
}

.columns-tab__btn--secondary:hover:not(:disabled) {
  background: var(--surface-2);
  border-color: var(--border);
}

.columns-tab__btn--primary {
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

.columns-tab__btn--primary:hover:not(:disabled) {
  background: var(--accent-hover);
  border-color: var(--accent);
}

.columns-tab__status {
  margin: 0;
  font-size: 13px;
  color: var(--success-text);
  text-align: right;
}

.columns-tab__status--error {
  color: var(--danger-text);
}

.columns-tab__preview {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.columns-tab__preview-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
}

.columns-tab__preview-hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}

/* Scale-обёртка: внутренний элемент рендерится при ширине 100%/SCALE и затем
   уменьшается transform: scale(SCALE) - все размеры (шрифт, padding, gap)
   сжимаются пропорционально, layout 1:1 с реальной таблицей. */
.columns-tab__preview-frame {
  width: 100%;
  overflow: hidden;
  /* Высота под 5 строк-примеров: ~50px header + 5×40px row = 250px,
     × scale 0.6 = 150px + ~20px запас на скругление/границу = 175px. */
  height: 175px;
  border-radius: 18px;
  border: 1px solid var(--border);
  background: var(--surface);
}

.columns-tab__preview-scale {
  /* SCALE=0.6 -> 1/0.6 = 166.67% */
  width: 166.6667%;
  transform: scale(0.6);
  transform-origin: top left;
}

.columns-tab__preview-unknown {
  padding: 16px;
  text-align: center;
  color: var(--text-muted);
  background: var(--accent-tint);
  border: 1px solid var(--border);
  border-radius: 12px;
}

/* FLIP-анимация перестановки столбцов в админ-списке. Vue TransitionGroup
   автоматически вычисляет старое и новое положение каждого элемента
   и применяет transform с заданным transition. */
.cols-list-move {
  transition: transform 0.3s ease;
}
</style>
