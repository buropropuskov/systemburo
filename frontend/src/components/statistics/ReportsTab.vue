<template>
  <div class="reports">
    <!-- Загрузка каталога -->
    <div
      v-if="catalogLoading"
      class="reports__state"
    >
      <LoaderSpinner />
      <span>Загружаем конструктор отчётов…</span>
    </div>

    <div
      v-else-if="catalogError"
      class="reports__state reports__state--error"
    >
      {{ catalogError }}
    </div>

    <template v-else-if="catalog">
      <div class="reports-layout">
        <!-- Сайдбар: готовые наборы + мои шаблоны -->
        <aside class="presets-col">
          <h3 class="col-heading">
            Готовые наборы
          </h3>
          <ReportGallery
            :catalog="catalog"
            :active-id="activePresetId"
            compact
            @apply="onApplyPreset"
          />

          <h3 class="col-heading col-heading--mt">
            Мои шаблоны
          </h3>

          <!-- Сохранить текущий набор как шаблон -->
          <div
            v-if="savingMode"
            class="tpl-save-form"
          >
            <input
              v-model="newTemplateName"
              class="lk-input"
              placeholder="Название шаблона"
              maxlength="200"
              @keyup.enter="confirmSaveTemplate"
            >
            <div class="tpl-save-actions">
              <button
                type="button"
                class="lk-button lk-button--primary"
                :disabled="!newTemplateName.trim() || savingTemplate"
                @click="confirmSaveTemplate"
              >
                {{ savingTemplate ? 'Сохраняем…' : 'Сохранить' }}
              </button>
              <button
                type="button"
                class="lk-button lk-button--ghost"
                @click="savingMode = false"
              >
                Отмена
              </button>
            </div>
          </div>
          <button
            v-else
            type="button"
            class="lk-button lk-button--ghost tpl-save-btn"
            :disabled="!canSaveTemplate"
            @click="startSaveTemplate"
          >
            + Сохранить текущий
          </button>

          <div
            v-if="templatesLoading"
            class="template-placeholder"
          >
            Загрузка шаблонов…
          </div>
          <div
            v-else-if="!myTemplates.length"
            class="template-placeholder"
          >
            Сохранённых наборов пока нет. Соберите отчёт в мастере и нажмите «Сохранить текущий».
          </div>
          <ul
            v-else
            class="tpl-list"
          >
            <li
              v-for="tpl in myTemplates"
              :key="tpl.id"
              class="tpl-item"
            >
              <button
                type="button"
                class="tpl-apply"
                @click="applyTemplate(tpl)"
              >
                <span class="tpl-name">{{ tpl.name }}</span>
                <span
                  v-if="tpl.description"
                  class="tpl-desc"
                >{{ tpl.description }}</span>
              </button>
              <button
                type="button"
                class="tpl-del"
                :title="`Удалить шаблон ${tpl.name}`"
                :aria-label="`Удалить шаблон ${tpl.name}`"
                @click="removeTemplate(tpl)"
              >
                ×
              </button>
            </li>
          </ul>
        </aside>

        <!-- Мастер -->
        <section class="wizard">
          <ReportStepper :steps="steps" />
          <div class="wizard-body">
            <ReportBuilder
              :catalog="catalog"
              :period="period"
              :loading="running"
              :preset="presetPayload"
              @run="onRun"
              @change="onBuilderChange"
            />
          </div>
        </section>
      </div>

      <!-- Результат (полная ширина под мастером) -->
      <ReportResult
        :result="result"
        :loading="running"
        :error="runError"
        :meta="exportMeta"
        @export-error="onExportError"
      />
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import ReportBuilder from './ReportBuilder.vue';
import ReportResult from './ReportResult.vue';
import ReportGallery from './ReportGallery.vue';
import ReportStepper from './ReportStepper.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import {
  getReportCatalog, runReport,
  getReportTemplates, saveReportTemplate, deleteReportTemplate,
} from '@/api/statistics';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';

const props = defineProps({
  from: { type: String, default: '' },
  to: { type: String, default: '' },
});

const period = computed(() => ({ from: props.from, to: props.to }));

const catalog = ref(null);
const catalogLoading = ref(true);
const catalogError = ref('');

const result = ref(null);
const running = ref(false);
const runError = ref('');
// Подпись для выгрузки в Excel: период берём из последнего построенного запроса.
const exportMeta = ref({});

// Пресет из галереи: новый объект на каждый клик (даже по той же карточке),
// чтобы watch в ReportBuilder сработал повторно и перезаполнил конструктор.
const presetPayload = ref(null);
const activePresetId = ref('');

// Снимок состояния мастера для индикатора шагов.
const builderState = ref({ mode: 'aggregate', metric: '', dimension: '', entity: '', filterCount: 0, periodApplicable: true, periodFilled: false, config: null });

// Личные шаблоны пользователя (G2). Системные пресеты остаются в галерее «Готовые
// наборы» (reportPresets), здесь — только сохранённые наборы пользователя.
const myTemplates = ref([]);
const templatesLoading = ref(true);
const savingMode = ref(false);
const savingTemplate = ref(false);
const newTemplateName = ref('');

const canSaveTemplate = computed(() => {
  const s = builderState.value;
  return s.mode === 'list' ? Boolean(s.entity) : Boolean(s.metric);
});

async function loadTemplates() {
  templatesLoading.value = true;
  try {
    const all = await getReportTemplates();
    // Только свои личные. Системные пресеты живут в галерее; чужие расшаренные
    // (is_shared) сюда не берём — прав на их удаление нет, кнопка × дала бы 403.
    myTemplates.value = (all || []).filter((t) => !t.is_system && !t.is_shared);
  } catch (e) {
    useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'загрузить шаблоны', suffix: e?.message ? `: ${e.message}` : '', type: 'error' });
  } finally {
    templatesLoading.value = false;
  }
}

function startSaveTemplate() {
  newTemplateName.value = '';
  savingMode.value = true;
}

async function confirmSaveTemplate() {
  const name = newTemplateName.value.trim();
  if (!name || !builderState.value.config) return;
  savingTemplate.value = true;
  try {
    await saveReportTemplate({ name, config: builderState.value.config });
    savingMode.value = false;
    await loadTemplates();
    useDeletionsStore().notify({ prefix: 'Шаблон ', bold: name, suffix: ' сохранён', type: 'success' });
  } catch (e) {
    useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'сохранить шаблон', suffix: e?.message ? `: ${e.message}` : '', type: 'error' });
  } finally {
    savingTemplate.value = false;
  }
}

function applyTemplate(tpl) {
  activePresetId.value = `tpl-${tpl.id}`;
  presetPayload.value = { ...tpl.config };
}

async function removeTemplate(tpl) {
  const ok = await useUiStore().confirm({
    title: 'Удалить шаблон?',
    message: `Шаблон «${tpl.name}» будет удалён без возможности восстановить.`,
    confirmText: 'Удалить',
    cancelText: 'Отмена',
    danger: true,
  });
  if (!ok) return;
  try {
    await deleteReportTemplate(tpl.id);
    await loadTemplates();
    useDeletionsStore().notify({ prefix: 'Шаблон ', bold: tpl.name, suffix: ' удалён', type: 'success' });
  } catch (e) {
    useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'удалить шаблон', suffix: e?.message ? `: ${e.message}` : '', type: 'error' });
  }
}

const STEP_LABELS = ['1 · Что считаем', '2 · По чему разбиваем', '3 · Фильтры', '4 · Период'];

// Состояние шагов по заполненности формы: done — данные есть, current — первый
// незаполненный, upcoming — дальше. В list-режиме разрез не применим -> шаг 2
// считается выполненным по выбранной сущности. Период, неприменимый к срезу
// (машины/люди без date_range), считается пройденным — заполнять нечего.
const steps = computed(() => {
  const s = builderState.value;
  const isList = s.mode === 'list';
  const done = [
    isList ? Boolean(s.entity) : Boolean(s.metric),
    isList ? Boolean(s.entity) : Boolean(s.dimension),
    s.filterCount > 0,
    s.periodApplicable === false ? true : Boolean(s.periodFilled),
  ];
  const currentIdx = done.indexOf(false);
  return STEP_LABELS.map((label, i) => ({
    label,
    state: done[i] ? 'done' : i === currentIdx ? 'current' : 'upcoming',
  }));
});

function onBuilderChange(snapshot) {
  builderState.value = snapshot;
}

// Ошибку выгрузки показываем тостом, не подменяя :error результата (иначе таблица
// отчёта скрылась бы за текстом ошибки).
function onExportError(message) {
  useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'выгрузить отчёт в Excel', suffix: message ? `: ${message}` : '', type: 'error' });
}

function onApplyPreset(preset) {
  activePresetId.value = preset.id;
  presetPayload.value = { ...preset.form };
}

onMounted(async () => {
  loadTemplates();
  try {
    catalog.value = await getReportCatalog();
  } catch (e) {
    catalogError.value = e?.message || 'Не удалось загрузить каталог отчётов';
  } finally {
    catalogLoading.value = false;
  }
});

// Быстрое переключение пресетов запускает несколько runReport параллельно;
// токен последовательности гарантирует, что результат покажет только последний
// запрос (иначе медленный ответ предыдущего пресета затёр бы актуальный).
let runSeq = 0;
async function onRun(request) {
  const seq = ++runSeq;
  running.value = true;
  runError.value = '';
  try {
    const r = await runReport(request);
    if (seq !== runSeq) return;
    result.value = r;
    const dr = (request.filters || []).find((f) => f.key === 'date_range');
    const { from = '', to = '' } = dr || {};
    exportMeta.value = dr ? { period: { from, to } } : {};
  } catch (e) {
    if (seq !== runSeq) return;
    result.value = null;
    runError.value = e?.message || 'Не удалось построить отчёт. Проверьте параметры.';
  } finally {
    if (seq === runSeq) running.value = false;
  }
}
</script>

<style scoped>
.reports {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.reports__state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 240px;
  color: var(--color-text-muted);
}

.reports__state--error {
  color: #c0392b;
}

.reports-layout {
  display: grid;
  grid-template-columns: 280px 1fr;
  gap: 20px;
  align-items: start;
}

.presets-col {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.col-heading {
  margin: 4px 2px 8px;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-text-muted);
}

.col-heading--mt {
  margin-top: 18px;
}

.template-placeholder {
  border: 1px dashed var(--color-border);
  border-radius: var(--radius-md);
  padding: 14px;
  font-size: 12px;
  font-weight: 500;
  line-height: 1.45;
  color: var(--color-text-muted);
}

/* Шаблоны (G2) */
.tpl-save-btn {
  width: 100%;
  justify-content: center;
}

.tpl-save-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tpl-save-actions {
  display: flex;
  gap: 8px;
}

.tpl-save-actions .lk-button {
  flex: 1;
  justify-content: center;
}

.tpl-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 8px 0 0;
  padding: 0;
  list-style: none;
}

.tpl-item {
  display: flex;
  align-items: stretch;
  gap: 4px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow: hidden;
  transition: border-color 0.18s ease;
}

.tpl-item:hover {
  border-color: var(--color-primary);
}

.tpl-apply {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 9px 11px;
  border: none;
  background: transparent;
  font-family: inherit;
  text-align: left;
  cursor: pointer;
}

.tpl-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tpl-desc {
  font-size: 11px;
  color: var(--color-text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tpl-del {
  flex-shrink: 0;
  width: 32px;
  border: none;
  background: transparent;
  color: var(--color-text-muted);
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}

.tpl-del:hover {
  background: var(--color-danger);
  color: #fff;
}

.wizard {
  min-width: 0;
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.wizard-body {
  padding: 24px;
}

@media (max-width: 1180px) {
  .reports-layout {
    grid-template-columns: 240px 1fr;
  }
}

@media (max-width: 880px) {
  .reports-layout {
    grid-template-columns: 1fr;
  }
}
</style>
