<template>
  <div class="columns-tab">
    <div class="columns-tab__header">
      <h4 class="columns-tab__title">
        Видимые столбцы
      </h4>
      <p class="columns-tab__hint">
        Отключите столбцы, которые не нужны в этой таблице. Скрытые столбцы не отображаются и не учитываются в фильтрах.
      </p>
    </div>

    <div
      v-if="!visibleFields.length"
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
        :class="{ 'columns-tab__item--off': !field.is_visible }"
      >
        <div class="columns-tab__row">
          <div class="columns-tab__order">
            <button
              type="button"
              class="columns-tab__order-btn"
              :disabled="index === 0"
              :data-action="`move-up-${field.field_name}`"
              :title="'Переместить вверх'"
              @click="moveUp(index)"
            >
              &uarr;
            </button>
            <button
              type="button"
              class="columns-tab__order-btn"
              :disabled="index === localFields.length - 1"
              :data-action="`move-down-${field.field_name}`"
              :title="'Переместить вниз'"
              @click="moveDown(index)"
            >
              &darr;
            </button>
          </div>
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
  </div>
</template>

<script>
import { apiRequest } from '@/api/client';

/**
 * Маппинг внутреннего имени поля -> человеческое название столбца.
 * Используется и в админ-вкладке "Колонки", и в реальных таблицах для совпадения подписей.
 */
const FIELD_LABELS = {
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
    };
  },
  computed: {
    visibleFields() {
      return this.localFields;
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
    moveUp(index) {
      if (index <= 0) return;
      const arr = this.localFields.slice();
      [arr[index - 1], arr[index]] = [arr[index], arr[index - 1]];
      this.localFields = arr;
    },
    moveDown(index) {
      if (index >= this.localFields.length - 1) return;
      const arr = this.localFields.slice();
      [arr[index], arr[index + 1]] = [arr[index + 1], arr[index]];
      this.localFields = arr;
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
  gap: 6px;
}

.columns-tab__item {
  border: 1px solid #e6e6e6;
  border-radius: 12px;
  transition: background-color 0.2s ease;
}

.columns-tab__item:hover {
  background: #f9fafb;
}

.columns-tab__item--off {
  background: #fafafa;
  border-color: #ececec;
}

.columns-tab__row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  user-select: none;
}

.columns-tab__order {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex-shrink: 0;
}

.columns-tab__order-btn {
  width: 24px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 6px;
  font-size: 11px;
  line-height: 1;
  cursor: pointer;
  color: #4F5BDF;
  transition: all 0.15s ease;
}

.columns-tab__order-btn:hover:not(:disabled) {
  background: #f0f4ff;
  border-color: #4F5BDF;
}

.columns-tab__order-btn:disabled {
  cursor: not-allowed;
  opacity: 0.3;
  color: #a2a2a2;
}

.columns-tab__check-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
  cursor: pointer;
  padding: 4px 4px 4px 8px;
}

.columns-tab__checkbox {
  width: 18px;
  height: 18px;
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
  font-size: 12px;
  color: #a2a2a2;
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
</style>
