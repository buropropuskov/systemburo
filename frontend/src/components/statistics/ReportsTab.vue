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
          <div class="template-placeholder">
            Сохранённые наборы появятся здесь. Соберите отчёт в мастере и сохраните его как шаблон.
          </div>
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
import { getReportCatalog, runReport } from '@/api/statistics';
import { useDeletionsStore } from '@/stores/deletions';

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
const builderState = ref({ mode: 'aggregate', metric: '', dimension: '', entity: '', filterCount: 0, periodApplicable: true, periodFilled: false });

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
