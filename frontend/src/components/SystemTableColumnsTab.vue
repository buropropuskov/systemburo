<template>
  <div class="columns-tab">
    <div class="columns-tab__header">
      <div class="columns-tab__header-top">
        <h4 class="columns-tab__title">
          Видимые столбцы
        </h4>
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
      <p class="columns-tab__hint">
        Переставляйте столбцы перетаскиванием за иконку слева. Скрытые столбцы не отображаются в таблице.
      </p>
      <p class="columns-tab__hint">
        <b>Ширина</b> - относительный вес столбца. Браузер делит доступную ширину
        видимых столбцов пропорционально этим весам. <b>Приоритет</b> (1-5) -
        видимость на вертикальном (книжном) экране охранника: 1 - всегда виден,
        2 - виден на узком экране, 3-5 - скрывается и доступен по кнопке "Подробнее"
        под строкой.
      </p>
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
              v-model="field.is_visible"
              type="checkbox"
              class="columns-tab__checkbox"
              :data-field="field.field_name"
            >
            <span class="columns-tab__label">{{ humanLabel(field.field_name) }}</span>
            <span class="columns-tab__field-name">{{ field.field_name }}</span>
          </label>
          <div
            class="columns-tab__width"
            :title="'Ширина столбца (вес flex-grow). Браузер делит доступную ширину пропорционально весам видимых столбцов.'"
          >
            <span class="columns-tab__width-label">Ширина:</span>
            <input
              v-model.number="field.width"
              type="number"
              min="1"
              max="100"
              class="columns-tab__width-input"
              :data-width-field="field.field_name"
            >
          </div>
          <div
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
import { useToast } from '@/composables/useToast';
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
  setup() {
    return { toast: useToast() };
  },
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
      saving: false,
      draggingIndex: null,
      prevBodyCursor: '',
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
      return visibilityChanged || orderChanged || widthChanged || priorityChanged;
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
  beforeUnmount() {
    // На случай если пользователь ушёл с вкладки во время drag.
    this.cleanupPointerListeners();
  },
  methods: {
    humanLabel(name) {
      return FIELD_LABELS[name] || name;
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
            })),
          }),
        });
        if (!response.ok) {
          const err = await response.json().catch(() => ({}));
          throw new Error(err.message || `HTTP ${response.status}`);
        }
        this.toast.success('Настройки столбцов сохранены');
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

.columns-tab__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #000;
}

.columns-tab__hint {
  margin: 0;
  font-size: 13px;
  color: #6b7280;
  line-height: 1.5;
}

.columns-tab__empty {
  padding: 24px;
  text-align: center;
  color: #a2a2a2;
  background: #f9fafb;
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
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  transition: opacity 0.15s ease, background-color 0.2s ease;
  background: #fff;
}

.columns-tab__item:hover {
  background: #f9fafb;
}

.columns-tab__item--off {
  background: #fafafa;
  border-color: #ececec;
}

.columns-tab__item--dragging {
  /* Полупрозрачный placeholder в "родной" позиции, чтобы было видно откуда
     тянем. transition: none убирает анимацию перетаскиваемого узла -
     дрожания между DOM и ghost-картинкой курсора нет. */
  opacity: 0.4;
  background: #f0f4ff;
  border-color: #4F5BDF;
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
  color: #a2a2a2;
  cursor: grab;
  flex-shrink: 0;
  transition: color 0.15s ease;
}

.columns-tab__handle:hover {
  color: #4F5BDF;
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
  accent-color: #4F5BDF;
  cursor: pointer;
  flex-shrink: 0;
}

.columns-tab__label {
  font-size: 14px;
  font-weight: 500;
  color: #000;
  flex: 1;
}

.columns-tab__item--off .columns-tab__label {
  color: #a2a2a2;
}

.columns-tab__field-name {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 11px;
  color: #c0c0c0;
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
  color: #6b7280;
  white-space: nowrap;
}

.columns-tab__width-input {
  width: 32px;
  padding: 2px 4px;
  border: 1px solid #e6e6e6;
  border-radius: 6px;
  font-size: 11px;
  text-align: right;
  color: #333;
  background: #fff;
  -moz-appearance: textfield;
}

.columns-tab__width-input::-webkit-outer-spin-button,
.columns-tab__width-input::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.columns-tab__width-input:focus {
  outline: none;
  border-color: #4F5BDF;
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
  color: #6b7280;
  white-space: nowrap;
}

.columns-tab__priority-select {
  width: 38px;
  padding: 2px 4px;
  border: 1px solid #e6e6e6;
  border-radius: 6px;
  font-size: 11px;
  color: #333;
  background: #fff;
  cursor: pointer;
}

.columns-tab__priority-select:focus {
  outline: none;
  border-color: #4F5BDF;
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
  background: #fff;
  border-color: #e6e6e6;
  color: #333;
}

.columns-tab__btn--secondary:hover:not(:disabled) {
  background: #f5f5f5;
  border-color: #c0c0c0;
}

.columns-tab__btn--primary {
  background: #4F5BDF;
  color: #fff;
  border-color: #4F5BDF;
}

.columns-tab__btn--primary:hover:not(:disabled) {
  background: #3b48c4;
  border-color: #3b48c4;
}

.columns-tab__status {
  margin: 0;
  font-size: 13px;
  color: #079D1D;
  text-align: right;
}

.columns-tab__status--error {
  color: #c62828;
}

.columns-tab__preview {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 8px;
}

.columns-tab__preview-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: #000;
}

.columns-tab__preview-hint {
  margin: 0;
  font-size: 12px;
  color: #6b7280;
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
  border: 1px solid #e6e6e6;
  background: #fff;
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
  color: #a2a2a2;
  background: #f9fafb;
  border: 1px solid #e6e6e6;
  border-radius: 12px;
}

/* FLIP-анимация перестановки столбцов в админ-списке. Vue TransitionGroup
   автоматически вычисляет старое и новое положение каждого элемента
   и применяет transform с заданным transition. */
.cols-list-move {
  transition: transform 0.3s ease;
}
</style>
