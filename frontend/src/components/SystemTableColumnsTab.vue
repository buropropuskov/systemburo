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

    <ul
      v-else
      class="columns-tab__list"
    >
      <li
        v-for="(field, index) in localFields"
        :key="field.field_name"
        class="columns-tab__item"
        :class="{
          'columns-tab__item--off': !field.is_visible,
          'columns-tab__item--dragging': draggingIndex === index,
          'columns-tab__item--drop-target': dragOverIndex === index && draggingIndex !== index,
        }"
        :draggable="true"
        :data-field="field.field_name"
        @dragstart="onDragStart(index, $event)"
        @dragover.prevent="onDragOver(index, $event)"
        @dragleave="onDragLeave(index)"
        @drop.prevent="onDrop(index)"
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
    </ul>

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
      v-if="visibleFieldsInOrder.length"
      class="columns-tab__preview"
    >
      <h4 class="columns-tab__preview-title">
        Предпросмотр
      </h4>
      <p class="columns-tab__preview-hint">
        Так таблица будет выглядеть с текущими настройками (примерные данные).
      </p>
      <div class="preview-card">
        <div class="preview-row preview-row--header">
          <div
            v-for="f in visibleFieldsInOrder"
            :key="f.field_name"
            class="preview-cell"
          >
            {{ humanLabel(f.field_name) }}
          </div>
        </div>
        <div
          v-for="(row, rowIdx) in sampleRows"
          :key="rowIdx"
          class="preview-row preview-row--data"
        >
          <div
            v-for="f in visibleFieldsInOrder"
            :key="f.field_name"
            class="preview-cell"
          >
            {{ row[f.field_name] || '-' }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client';

/**
 * Маппинг внутреннего имени поля -> человеческое название столбца.
 * Используется и в админ-вкладке "Колонки", и в реальных таблицах для совпадения подписей.
 */
const FIELD_LABELS = {
  // Базовые поля cars
  car_number: 'Номер Т/С',
  car_brand: 'Марка',
  organization: 'Организация',
  unload_place: 'Место разгрузки',
  valid_until: 'Действует до',
  time_range: 'Время',
  status: 'Статус',
  // Расширенные поля cars (по умолчанию скрыты)
  application_id: 'Номер заявки',
  // Базовые поля people
  last_name: 'Фамилия',
  first_name: 'Имя',
  middle_name: 'Отчество',
  pass_time: 'Время прохода',
  // Расширенные поля people (по умолчанию скрыты)
  position: 'Должность',
  citizenship_name: 'Гражданство',
  // Общие
  company: 'Компания',
};

/**
 * Примеры значений для каждого поля. Из них генерируются 10 строк-примеров в предпросмотре.
 * Длина массивов должна быть >= 10.
 */
const SAMPLE_VALUES = {
  car_number: ['А 123 БВ 77', 'М 456 ГД 99', 'О 789 ЕЖ 50', 'Х 012 ЗИ 77', 'Т 345 КЛ 99',
    'Е 678 МН 50', 'У 901 ОП 77', 'К 234 РС 99', 'В 567 ТУ 50', 'С 890 ФХ 77'],
  car_brand: ['Тойота', 'Лада', 'Газель', 'Камаз', 'Вольво', 'МАН', 'Мерседес', 'Рено', 'Скания', 'ДАФ'],
  organization: ['ООО Альфа', 'ООО Бета', 'ЗАО Гамма', 'ИП Дельта', 'ООО Эпсилон',
    'АО Дзета', 'ООО Эта', 'ИП Тета', 'ООО Йота', 'ЗАО Каппа'],
  company: ['Альфа-Сервис', 'Бета-Логистик', 'Гамма-Транс', 'Дельта-Карго', 'Эпсилон-Экспресс',
    'Дзета-Авто', 'Эта-Логист', 'Тета-Линия', 'Йота-Доставка', 'Каппа-Транс'],
  application_id: ['20260530/00148', '20260530/00149', '20260530/00150', '20260530/00151',
    '20260530/00152', '20260531/00001', '20260531/00002', '20260531/00003', '20260531/00004', '20260531/00005'],
  unload_place: ['Дебаркадер №1', 'Дебаркадер №2', 'Склад А', 'Склад Б', 'Площадка №3',
    'Зона разгрузки', 'Пандус', 'Склад В', 'Площадка №7', 'Дебаркадер №5'],
  valid_until: ['31.05.2026', '01.06.2026', '15.06.2026', '30.06.2026', '07.06.2026',
    '14.06.2026', '21.06.2026', '28.06.2026', '05.07.2026', '12.07.2026'],
  time_range: ['08:00 - 23:59', '09:00 - 18:00', '06:00 - 22:00', '10:00 - 16:00', '08:00 - 20:00',
    '07:00 - 19:00', '00:00 - 23:59', '08:00 - 17:00', '09:00 - 21:00', '06:00 - 14:00'],
  status: ['В работе', 'В работе', 'В работе', 'В работе', 'В работе',
    'В работе', 'В работе', 'В работе', 'В работе', 'В работе'],
  last_name: ['Иванов', 'Петров', 'Сидоров', 'Кузнецов', 'Смирнов',
    'Попов', 'Лебедев', 'Соколов', 'Морозов', 'Волков'],
  first_name: ['Иван', 'Пётр', 'Александр', 'Сергей', 'Михаил',
    'Андрей', 'Дмитрий', 'Николай', 'Алексей', 'Владимир'],
  middle_name: ['Иванович', 'Петрович', 'Сергеевич', 'Александрович', 'Михайлович',
    'Андреевич', 'Дмитриевич', 'Николаевич', 'Алексеевич', 'Владимирович'],
  position: ['Грузчик', 'Водитель', 'Экспедитор', 'Кладовщик', 'Менеджер',
    'Оператор', 'Инженер', 'Логист', 'Контролёр', 'Бригадир'],
  citizenship_name: ['Россия', 'Россия', 'Беларусь', 'Казахстан', 'Россия',
    'Узбекистан', 'Россия', 'Армения', 'Россия', 'Таджикистан'],
  pass_time: ['10:00 - 15:00', '09:00 - 18:00', '08:00 - 17:00', '07:00 - 14:00', '11:00 - 19:00',
    '06:00 - 12:00', '13:00 - 21:00', '08:00 - 16:00', '10:00 - 18:00', '14:00 - 22:00'],
};

export default {
  name: 'SystemTableColumnsTab',
  props: {
    tableId: { type: Number, required: true },
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
      dragOverIndex: null,
    };
  },
  computed: {
    visibleFieldsInOrder() {
      return this.localFields.filter(f => f.is_visible);
    },
    sampleRows() {
      return Array.from({ length: 10 }, (_, i) => {
        const row = {};
        for (const field of this.localFields) {
          const values = SAMPLE_VALUES[field.field_name];
          row[field.field_name] = values ? values[i % values.length] : '—';
        }
        return row;
      });
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
      // Сортируем по display_order, чтобы порядок в админке отражал реальный.
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
      event.dataTransfer.setData('text/plain', String(index));
    },
    onDragOver(index, event) {
      event.dataTransfer.dropEffect = 'move';
      this.dragOverIndex = index;
    },
    onDragLeave(index) {
      if (this.dragOverIndex === index) {
        this.dragOverIndex = null;
      }
    },
    onDrop(targetIndex) {
      const from = this.draggingIndex;
      if (from === null || from === targetIndex) {
        this.draggingIndex = null;
        this.dragOverIndex = null;
        return;
      }
      const arr = this.localFields.slice();
      const [moved] = arr.splice(from, 1);
      arr.splice(targetIndex, 0, moved);
      this.localFields = arr;
      this.draggingIndex = null;
      this.dragOverIndex = null;
    },
    onDragEnd() {
      this.draggingIndex = null;
      this.dragOverIndex = null;
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
  transition: background-color 0.2s ease, transform 0.15s ease, box-shadow 0.15s ease, opacity 0.15s ease;
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
  opacity: 0.4;
  border-style: dashed;
}

.columns-tab__item--drop-target {
  border-color: #4F5BDF;
  box-shadow: 0 0 0 2px rgba(79, 91, 223, 0.18);
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

.columns-tab__handle:active {
  cursor: grabbing;
}

.columns-tab__check-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  cursor: pointer;
  padding: 4px 4px 4px 4px;
}

.columns-tab__checkbox {
  width: 14px;
  height: 14px;
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

/* Preview pane: миниатюрная таблица, отражает видимость и порядок столбцов. */
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

.preview-card {
  border: 1px solid #e6e6e6;
  border-radius: 12px;
  overflow: hidden;
  background: #fff;
  font-size: 9px;
  line-height: 1.2;
}

.preview-row {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 2px;
  padding: 3px 6px;
}

.preview-row--header {
  background: #f9fafb;
  border-bottom: 1px solid #e6e6e6;
  color: #6b7280;
  font-weight: 500;
  font-size: 8px;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.preview-row--data + .preview-row--data {
  border-top: 1px solid #f3f4f6;
}

.preview-cell {
  flex: 1 0 0;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #1f2937;
}

.preview-row--header .preview-cell {
  color: #6b7280;
}
</style>
