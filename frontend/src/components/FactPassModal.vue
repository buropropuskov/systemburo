<template>
  <BaseModal
    :show="show"
    title="Пропуск по факту"
    width="440px"
    radius="30px"
    @close="$emit('close')"
  >
    <div class="fact-pass">
      <p class="fact-pass__subtitle">
        Введите данные машины, которую пропускаете. Исходная строка «по факту» в
        таблице не меняется, данные сохранятся в истории и карточке.
      </p>

      <div class="fact-pass__field">
        <label class="fact-pass__label">
          Формат номера <span class="fact-pass__req">*</span>
        </label>
        <BaseDropdown
          :model-value="selectedFormatId"
          :options="formatOptions"
          label-key="name"
          value-key="id"
          placeholder="Выберите формат"
          teleport
          @update:model-value="onFormatChange"
        />
      </div>

      <div class="fact-pass__field">
        <label class="fact-pass__label">
          Номер Т/С <span class="fact-pass__req">*</span>
        </label>
        <div
          v-if="selectedFormat"
          class="fpm-number"
        >
          <input
            v-for="(cell, index) in selectedFormat.cells"
            :key="index"
            v-model="numberParts[index]"
            class="fpm-cell"
            :placeholder="getPlaceholder(cell)"
            :maxlength="cell.max_length"
            data-testid="fact-pass-number-cell"
            @input="validatePart(index, $event, cell)"
            @blur="formatPart(index, cell)"
          >
        </div>
        <p
          v-else
          class="fact-pass__hint"
        >
          Сначала выберите формат номера
        </p>
      </div>

      <div class="fact-pass__field">
        <label class="fact-pass__label">Марка Т/С</label>
        <BaseDropdown
          :model-value="selectedMarkId"
          :options="marks"
          label-key="name"
          value-key="id"
          placeholder="По факту"
          searchable
          teleport
          @update:model-value="selectedMarkId = $event"
        />
      </div>

      <p
        v-if="error"
        class="fact-pass__error"
      >
        {{ error }}
      </p>

      <div class="fact-pass__actions">
        <button
          class="lk-button lk-button--ghost"
          type="button"
          @click="$emit('close')"
        >
          Отмена
        </button>
        <button
          class="lk-button lk-button--primary"
          type="button"
          :disabled="!canConfirm"
          data-testid="fact-pass-confirm"
          @click="confirm"
        >
          Пропустить
        </button>
      </div>
    </div>
  </BaseModal>
</template>

<script setup>
import { ref, computed, watch } from 'vue';
import BaseModal from '@/components/ui/BaseModal.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import { validatePartValue, formatPartValue, initializeNumberParts } from '@/composables/useNumberFormat';
import { listMarks } from '@/api/marks';

const props = defineProps({
  show: { type: Boolean, required: true },
  // Форматы номеров (GET /license-plate-formats), загруженные родителем.
  // Форма элемента: { format: { id, name, is_default }, cells: [{ cell_type, min_length, max_length, ... }] }
  formats: { type: Array, default: () => [] },
  // Флаг сохранения (родитель дёргает territory-status и управляет им).
  loading: { type: Boolean, default: false },
  // Текст ошибки сохранения (родитель управляет).
  error: { type: String, default: '' },
});

const emit = defineEmits(['close', 'confirm']);

const selectedFormatId = ref(null);
const numberParts = ref([]);
const marks = ref([]);
const marksLoaded = ref(false);
const selectedMarkId = ref(null);

const formatOptions = computed(() => props.formats.map((f) => ({ id: f.format.id, name: f.format.name })));
const selectedFormat = computed(() => props.formats.find((f) => f.format.id === selectedFormatId.value) || null);
const selectedMark = computed(() => marks.value.find((m) => m.id === selectedMarkId.value) || null);

// Номер заполнен, когда выбран формат и каждая ячейка в пределах min/max длины
// (та же проверка, что в VehicleForm.canAddVehicle).
const numberComplete = computed(() => {
  const fmt = selectedFormat.value;
  if (!fmt || numberParts.value.length === 0) return false;
  return numberParts.value.every((part, i) => {
    const cell = fmt.cells[i];
    return part && part.length >= cell.min_length && part.length <= cell.max_length;
  });
});

const canConfirm = computed(() => !!selectedFormat.value && numberComplete.value && !props.loading);

function pickDefaultFormat() {
  const def = props.formats.find((f) => f.format.is_default) || props.formats[0] || null;
  selectedFormatId.value = def ? def.format.id : null;
  numberParts.value = initializeNumberParts(def);
}

function onFormatChange(id) {
  selectedFormatId.value = id;
  numberParts.value = initializeNumberParts(selectedFormat.value);
}

function validatePart(index, event, cell) {
  const value = validatePartValue(event.target.value, cell);
  numberParts.value[index] = value;
  event.target.value = value;
}

function formatPart(index, cell) {
  if (numberParts.value[index]) {
    const formatted = formatPartValue(numberParts.value[index], cell);
    if (formatted !== numberParts.value[index]) numberParts.value[index] = formatted;
  }
}

function getPlaceholder(cell) {
  return (cell.cell_type === 'numbers' ? '0' : 'A').repeat(cell.max_length);
}

async function loadMarks() {
  if (marksLoaded.value) return;
  try {
    const res = await listMarks();
    const arr = Array.isArray(res) ? res : (res?.marks || []);
    marks.value = arr.filter((m) => m.is_active !== false).map((m) => ({ id: m.id, name: m.name }));
  } catch (err) {
    console.error('Не удалось загрузить справочник марок', err);
    marks.value = [];
  } finally {
    marksLoaded.value = true;
  }
}

function reset() {
  pickDefaultFormat();
  selectedMarkId.value = null;
}

function confirm() {
  if (!canConfirm.value) return;
  const fmt = selectedFormat.value;
  emit('confirm', {
    number: numberParts.value.join(' '),
    format_id: fmt.format.id,
    format_name: fmt.format.name,
    mark_id: selectedMarkId.value,
    mark_name: selectedMark.value ? selectedMark.value.name : null,
  });
}

watch(
  () => props.show,
  (open) => {
    if (open) {
      reset();
      loadMarks();
    }
  },
  { immediate: true },
);

// Форматы могут догрузиться уже после открытия модалки - подставим дефолт, когда приедут.
watch(
  () => props.formats,
  () => {
    if (props.show && !selectedFormat.value) pickDefaultFormat();
  },
);
</script>

<style scoped>
.fact-pass {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 20px;
}

.fact-pass__subtitle {
  margin: 0;
  font-size: 13px;
  line-height: 1.4;
  color: var(--color-text-secondary, var(--text-muted));
}

.fact-pass__field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.fact-pass__label {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary, var(--text));
}

.fact-pass__req {
  color: var(--color-danger, var(--danger-text));
}

/* Ввод номера 1:1 как в VehicleForm: один bordered-контейнер (radius 15) с
   ячейками-инпутами внутри, разделёнными border-right (не отдельные боксы). */
.fpm-number {
  max-width: 202px;
  min-width: 202px;
  height: 40px;
  display: flex;
  border: 1px solid var(--border);
  border-radius: 15px;
  overflow: hidden;
  background: var(--surface);
}

.fpm-cell {
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

.fpm-cell:not(:last-child) {
  border-right: 1px solid var(--border);
}

.fpm-cell::placeholder {
  color: var(--text-muted);
  font-size: 12px;
}

.fpm-cell:focus {
  background-color: var(--surface-2);
}

.fact-pass__hint {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-secondary, var(--text-muted));
}

.fact-pass__error {
  margin: 0;
  font-size: 13px;
  color: var(--color-danger, var(--danger-text));
}

.fact-pass__actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 4px;
}
</style>
