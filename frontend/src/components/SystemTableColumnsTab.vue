<template>
  <div class="columns-tab">
    <div class="columns-tab__header">
      <h4 class="columns-tab__title">
        Видимые столбцы
      </h4>
      <p class="columns-tab__hint">
        Переставляйте столбцы перетаскиванием за иконку слева. Скрытые столбцы не отображаются в таблице.
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
        :draggable="true"
        :data-field="field.field_name"
        @dragstart="onDragStart(index, $event)"
        @dragenter.prevent="onDragEnter(index)"
        @dragover.prevent="onDragOver($event)"
        @drop.prevent="onDrop"
        @dragend="onDragEnd"
      >
        <div class="columns-tab__row">
          <span
            class="columns-tab__handle"
            :title="'Перетащите для смены порядка'"
            aria-hidden="true"
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
        </div>
      </li>
    </TransitionGroup>

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

    <p
      v-if="statusMessage"
      class="columns-tab__status"
      :class="{ 'columns-tab__status--error': statusError }"
    >
      {{ statusMessage }}
    </p>

    <div
      v-if="localFields.length"
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
  },
  emits: ['update'],
  data() {
    return {
      localFields: [],
      originalVisibility: {},
      originalOrder: [],
      saving: false,
      statusMessage: '',
      statusError: false,
      draggingIndex: null,
    };
  },
  computed: {
    previewFieldsWithOrder() {
      // Передаём текущий локальный порядок в превью, чтобы оно реагировало до save.
      return this.localFields.map((f, i) => ({
        field_name: f.field_name,
        is_visible: f.is_visible,
        display_order: i,
      }));
    },
    sampleItems() {
      return generateSampleRows(this.tableType, 10);
    },
    isDirty() {
      const visibilityChanged = this.localFields.some(
        f => this.originalVisibility[f.field_name] !== f.is_visible,
      );
      const orderChanged = this.localFields.some(
        (f, i) => this.originalOrder[i] !== f.field_name,
      );
      return visibilityChanged || orderChanged;
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
      }));
      this.originalVisibility = Object.fromEntries(
        this.localFields.map(f => [f.field_name, f.is_visible]),
      );
      this.originalOrder = this.localFields.map(f => f.field_name);
      this.statusMessage = '';
      this.statusError = false;
    },
    onDragStart(index, event) {
      this.draggingIndex = index;
      event.dataTransfer.effectAllowed = 'move';
      // Минимальный data payload для Chromium.
      event.dataTransfer.setData('text/plain', String(index));
    },
    onDragOver(event) {
      // Всегда move - никаких "копировать"/"запрещено" курсоров.
      event.dataTransfer.dropEffect = 'move';
    },
    onDragEnter(targetIndex) {
      // Live-перестановка: как только курсор над другим элементом - меняем порядок сразу.
      const from = this.draggingIndex;
      if (from === null || from === targetIndex) return;
      const arr = this.localFields.slice();
      const [moved] = arr.splice(from, 1);
      arr.splice(targetIndex, 0, moved);
      this.localFields = arr;
      this.draggingIndex = targetIndex;
    },
    onDrop() {
      this.draggingIndex = null;
    },
    onDragEnd() {
      this.draggingIndex = null;
    },
    async save() {
      if (!this.isDirty || this.saving) return;
      this.saving = true;
      this.statusMessage = '';
      this.statusError = false;

      try {
        const response = await apiRequest(`/system-tables/${this.tableId}/fields`, {
          method: 'PUT',
          body: JSON.stringify({
            fields: this.localFields.map((f, i) => ({
              field_name: f.field_name,
              is_visible: f.is_visible,
              display_order: i,
            })),
          }),
        });
        if (!response.ok) {
          const err = await response.json().catch(() => ({}));
          throw new Error(err.message || `HTTP ${response.status}`);
        }
        this.statusMessage = 'Настройки столбцов сохранены';
        this.originalVisibility = Object.fromEntries(
          this.localFields.map(f => [f.field_name, f.is_visible]),
        );
        this.originalOrder = this.localFields.map(f => f.field_name);
        this.$emit('update');
      } catch (e) {
        this.statusError = true;
        this.statusMessage = `Ошибка сохранения: ${e.message}`;
      } finally {
        this.saving = false;
      }
    },
  },
};
</script>

<style scoped>
.columns-tab {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.columns-tab__header {
  display: flex;
  flex-direction: column;
  gap: 4px;
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
  line-height: 1.4;
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
  opacity: 0.35;
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

.columns-tab__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  border-top: 1px solid #f0f0f0;
  padding-top: 12px;
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
  /* Высота - clamp на типовую высоту: реальная max-height 575 * scale + запас. */
  height: 360px;
  border-radius: 30px;
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
