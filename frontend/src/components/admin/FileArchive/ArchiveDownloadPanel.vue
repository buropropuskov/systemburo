<template>
  <section class="adp">
    <h4 class="adp__title">
      Скачать сохранённые бланки за период
    </h4>
    <div class="adp__controls">
      <DateFilter
        mode="range"
        :date-range-start="rangeStart"
        :date-range-end="rangeEnd"
        @update:date-range-start="onRangeStart"
        @update:date-range-end="onRangeEnd"
        @apply="onApply"
        @clear="onClear"
      />
      <button
        type="button"
        class="lk-button lk-button--primary"
        data-testid="adp-download"
        :disabled="!canDownload"
        @click="download"
      >
        {{ issuing ? 'Готовим билет...' : 'Скачать ZIP' }}
      </button>
    </div>

    <p
      v-if="loadingEstimate"
      class="adp__hint"
    >
      Считаем объём выгрузки...
    </p>
    <p
      v-else-if="estimateError"
      class="form-error"
      data-testid="adp-estimate-error"
    >
      {{ estimateError }}
    </p>
    <template v-else-if="estimate">
      <p
        class="adp__estimate"
        data-testid="adp-estimate"
      >
        Файлов: {{ estimate.file_count }}, объём: {{ formatBytes(estimate.bytes) }}
      </p>
      <p
        v-if="estimate.exceeds_limit"
        class="form-error"
        data-testid="adp-exceeds"
      >
        Выгрузка превышает допустимый предел одного ZIP-архива - сузьте период.
      </p>
      <p
        v-else-if="isEmpty"
        class="adp__hint"
        data-testid="adp-empty"
      >
        За выбранный период в архиве нет сохранённых файлов.
      </p>
      <p
        v-else-if="isLarge"
        class="adp__warning"
        data-testid="adp-large-warning"
      >
        Большой архив - скачивание может занять продолжительное время.
      </p>
    </template>

    <p
      v-if="downloadError"
      class="form-error"
      data-testid="adp-download-error"
    >
      {{ downloadError }}
    </p>
  </section>
</template>

<script setup>
/**
 * Скачивание сохранённых бланков файлового архива за период (#1615, срез C4):
 * период через DateFilter -> оценка объёма (POST /estimate, seq-guard от гонки
 * ответов при быстрой смене периода) -> билет -> навигация через
 * startTicketDownload (utils/download.js, скрытый <a download>, без fetch+blob -
 * ZIP может весить гигабайты).
 */
import { ref, computed } from 'vue';
import DateFilter from '@/components/DateFilter.vue';
import { estimateArchiveDownload, issueArchiveDownloadTicket } from '@/api/fileArchive';
import { formatBytes, startTicketDownload } from '@/utils/download';
import { useDeletionsStore } from '@/stores/deletions';

// Порог предупреждения "большой архив" - независим от archive.zip_max_bytes
// (тот считает хард-лимит, exceeds_limit): 1 ГБ уже ощутимо долго качать даже
// при разрешённом объёме.
const LARGE_WARNING_BYTES = 1024 * 1024 * 1024;

const rangeStart = ref(null);
const rangeEnd = ref(null);
const estimate = ref(null);
const loadingEstimate = ref(false);
const estimateError = ref('');
const downloadError = ref('');
const issuing = ref(false);

let estimateSeq = 0;

function toDateParam(d) {
  if (!d) return '';
  const dt = new Date(d);
  const p = (n) => String(n).padStart(2, '0');
  return `${dt.getFullYear()}-${p(dt.getMonth() + 1)}-${p(dt.getDate())}`;
}

const period = computed(() => ({ dateFrom: toDateParam(rangeStart.value), dateTo: toDateParam(rangeEnd.value) }));
const hasPeriod = computed(() => !!period.value.dateFrom && !!period.value.dateTo);
const isEmpty = computed(() => !!estimate.value && estimate.value.file_count === 0);
const isLarge = computed(() => !!estimate.value && estimate.value.bytes > LARGE_WARNING_BYTES);
const canDownload = computed(() => hasPeriod.value
  && !!estimate.value && !estimate.value.exceeds_limit && !isEmpty.value && !issuing.value);

async function runEstimate() {
  if (!hasPeriod.value) {
    estimate.value = null;
    return;
  }
  const mySeq = (estimateSeq += 1);
  loadingEstimate.value = true;
  estimateError.value = '';
  try {
    const data = await estimateArchiveDownload(period.value);
    if (mySeq !== estimateSeq) return; // ответ устарел - период уже сменили
    estimate.value = data;
  } catch (e) {
    if (mySeq !== estimateSeq) return;
    estimate.value = null;
    estimateError.value = e?.message || 'Не удалось оценить объём выгрузки';
  } finally {
    if (mySeq === estimateSeq) loadingEstimate.value = false;
  }
}

function onRangeStart(v) { rangeStart.value = v; }
function onRangeEnd(v) { rangeEnd.value = v; }
function onApply() { runEstimate(); }
function onClear() {
  rangeStart.value = null;
  rangeEnd.value = null;
  estimate.value = null;
  estimateError.value = '';
  downloadError.value = '';
}

async function download() {
  if (!canDownload.value) return;
  issuing.value = true;
  downloadError.value = '';
  try {
    const { ticket } = await issueArchiveDownloadTicket(period.value);
    startTicketDownload('/file-archive/download', ticket);
  } catch (e) {
    downloadError.value = e?.message || 'Не удалось получить билет на скачивание';
    useDeletionsStore().notify({ prefix: 'Не удалось начать ', bold: 'скачивание архива', type: 'error' });
  } finally {
    issuing.value = false;
  }
}
</script>

<style scoped>
.adp {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.adp__title {
  margin: 0;
  font-size: 0.95em;
  font-weight: 600;
  color: var(--text);
}

.adp__controls {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.adp__hint,
.adp__estimate {
  margin: 0;
  font-size: 0.85em;
  color: var(--text-muted);
}

.adp__warning {
  margin: 0;
  font-size: 0.85em;
  color: var(--warning-text);
}

.form-error {
  color: var(--danger-text);
  font-size: 0.85em;
  margin: 0;
}

/* Тач-таргет поля периода привязан к брейкпоинту самого DateFilter (768): на
   iPad-портрете его календарь уже открывается листом, значит и по полю попадают
   пальцем. Высота поля зашита в 35px, min-height её перебивает. */
@media (max-width: 768px) {
  .adp__controls :deep(.date-field) {
    min-height: 44px;
  }
}

@media (max-width: 767.98px) {
  .adp__controls {
    flex-direction: column;
    align-items: stretch;
  }

  /* Ширина у DateFilter зашита в 215px - в колоночной раскладке тянем поле на
     всю строку. */
  .adp__controls :deep(.date-filter),
  .adp__controls :deep(.date-field) {
    width: 100%;
  }

  .adp__controls .lk-button {
    min-height: 44px;
  }
}
</style>
