<template>
  <section class="abp">
    <h4 class="abp__title">
      Пересобрать бланки за период
    </h4>
    <p class="abp__hint">
      Ставит в очередь все заявки периода - действие переписывает файлы уже выгруженных заявок,
      неизменившиеся файлы диск не трогает (сверка по содержимому).
    </p>

    <div class="abp__controls">
      <DateFilter
        mode="range"
        :date-range-start="rangeStart"
        :date-range-end="rangeEnd"
        @update:date-range-start="onRangeStart"
        @update:date-range-end="onRangeEnd"
        @clear="onClear"
      />
    </div>

    <ToggleSwitch
      v-model="narrowByType"
      data-testid="abp-narrow-toggle"
    >
      Ограничить одним типом вложения
    </ToggleSwitch>
    <BaseDropdown
      v-if="narrowByType"
      v-model="selectedAttachmentId"
      class="abp__type-select"
      :options="attachmentOptions"
      label-key="display_name"
      value-key="id"
      placeholder="Выберите тип вложения"
      data-testid="abp-type-select"
    />

    <div class="abp__actions">
      <button
        type="button"
        class="lk-button lk-button--primary"
        data-testid="abp-submit"
        :disabled="!canSubmit"
        @click="requestBackfill"
      >
        {{ preparing ? 'Считаем...' : 'Поставить в очередь' }}
      </button>
    </div>

    <p
      v-if="submitError"
      class="form-error"
      data-testid="abp-submit-error"
    >
      {{ submitError }}
    </p>

    <p
      v-if="queued !== null"
      class="abp__result"
      data-testid="abp-queued"
    >
      Поставлено в очередь: {{ queued }} заявок.
    </p>

    <div
      v-if="polling"
      class="abp__progress"
      data-testid="abp-progress"
    >
      <p class="abp__hint">
        Осталось необработанных строк в очереди (весь архив, не только эта заявка):
        {{ pendingTotal === null ? '...' : pendingTotal }}
      </p>
      <button
        type="button"
        class="lk-button lk-button--ghost"
        data-testid="abp-stop-progress"
        @click="stopPolling"
      >
        Остановить отслеживание
      </button>
    </div>

    <ConfirmationModal
      :show="confirmVisible"
      title="Пересобрать бланки за период"
      :message="confirmMessage"
      confirm-text="Поставить в очередь"
      cancel-text="Отмена"
      @confirm="confirmBackfill"
      @cancel="confirmVisible = false"
    />
  </section>
</template>

<script setup>
/**
 * Ручной бэкфилл файлового архива за период (#1615, срез C4): та же ручка
 * обслуживает и полный период, и «пересоздать бланки этого типа» после правки
 * шаблона (тумблер сужения типом вложения добавляет unique_attachment_id).
 * ConfirmationModal показывает оценку - сколько файлов уже лежит в архиве за
 * период (POST /estimate, единственная доступная оценка объёма; точное число
 * заявок, которые встанут в очередь, узнаётся только из ответа POST /backfill).
 *
 * Прогресс - опрос общего числа строк реестра со статусом pending (GET /items):
 * бэк не различает "эта партия" от остальной активности воркера, поэтому
 * счётчик показывает всю очередь целиком, а не только то, что поставил именно
 * этот бэкфилл. Опрос best-effort и не критичен - неудачный тик молча
 * пропускается, а не рвёт индикатор тостом на каждые 4 секунды.
 */
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import DateFilter from '@/components/DateFilter.vue';
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import ConfirmationModal from '@/components/ConfirmationModal.vue';
import {
  estimateArchiveDownload, runArchiveBackfill, listArchiveItems,
} from '@/api/fileArchive';
import { listAttachments } from '@/api/attachments';
import { formatBytes } from '@/utils/download';
import { useDeletionsStore } from '@/stores/deletions';

const POLL_INTERVAL_MS = 4000;

const deletions = useDeletionsStore();

const rangeStart = ref(null);
const rangeEnd = ref(null);
const narrowByType = ref(false);
const selectedAttachmentId = ref(null);
const attachments = ref([]);

const preparing = ref(false);
const submitError = ref('');
const queued = ref(null);

const confirmVisible = ref(false);
const backfillEstimate = ref(null);

const polling = ref(false);
const pendingTotal = ref(null);
let pollTimer = null;

function toDateParam(d) {
  if (!d) return '';
  const dt = new Date(d);
  const p = (n) => String(n).padStart(2, '0');
  return `${dt.getFullYear()}-${p(dt.getMonth() + 1)}-${p(dt.getDate())}`;
}

const period = computed(() => ({ dateFrom: toDateParam(rangeStart.value), dateTo: toDateParam(rangeEnd.value) }));
const hasPeriod = computed(() => !!period.value.dateFrom && !!period.value.dateTo);
const canSubmit = computed(() => hasPeriod.value
  && (!narrowByType.value || !!selectedAttachmentId.value)
  && !preparing.value);

const attachmentOptions = computed(() => attachments.value.map((a) => ({
  id: a.id,
  display_name: a.display_name || a.name || `#${a.id}`,
})));

const selectedAttachmentLabel = computed(() => {
  const found = attachmentOptions.value.find((o) => o.id === selectedAttachmentId.value);
  return found ? found.display_name : '';
});

const confirmMessage = computed(() => {
  const scope = narrowByType.value && selectedAttachmentLabel.value
    ? `, только тип вложения «${selectedAttachmentLabel.value}»`
    : '';
  const known = backfillEstimate.value
    ? ` За этот период в архиве уже сохранено файлов: ${backfillEstimate.value.file_count} (${formatBytes(backfillEstimate.value.bytes)}).`
    : '';
  return `Поставить в очередь пересборку бланков за период ${period.value.dateFrom} - ${period.value.dateTo}${scope}?`
    + `${known} Уже выгруженные файлы будут переписаны при расхождении с текущими данными.`;
});

function onRangeStart(v) { rangeStart.value = v; }
function onRangeEnd(v) { rangeEnd.value = v; }
function onClear() {
  rangeStart.value = null;
  rangeEnd.value = null;
  queued.value = null;
  submitError.value = '';
}

async function loadAttachments() {
  try {
    attachments.value = await listAttachments();
  } catch {
    // Список типов - вспомогательный (сужение необязательно): без него тумблер
    // просто останется пустым выбором, бэкфилл за весь период по-прежнему работает.
    attachments.value = [];
  }
}

async function requestBackfill() {
  if (!canSubmit.value) return;
  preparing.value = true;
  submitError.value = '';
  try {
    backfillEstimate.value = await estimateArchiveDownload(period.value);
  } catch {
    backfillEstimate.value = null; // оценка необязательна для подтверждения, продолжаем без неё
  } finally {
    preparing.value = false;
  }
  confirmVisible.value = true;
}

async function confirmBackfill() {
  confirmVisible.value = false;
  submitError.value = '';
  try {
    const result = await runArchiveBackfill({
      ...period.value,
      uniqueAttachmentId: narrowByType.value ? selectedAttachmentId.value : null,
    });
    queued.value = result.queued;
    deletions.notify({ prefix: 'Поставлено в очередь ', bold: `${result.queued} заявок` });
    startPolling();
  } catch (e) {
    submitError.value = e?.message || 'Не удалось поставить бэкфилл в очередь';
    deletions.notify({ prefix: 'Не удалось поставить бэкфилл: ', bold: submitError.value, type: 'error' });
  }
}

async function pollPending() {
  try {
    const { meta } = await listArchiveItems({ status: 'pending', page: 1, perPage: 1 });
    pendingTotal.value = meta.total;
    if (meta.total === 0) stopPolling();
  } catch {
    // Best-effort опрос прогресса - неудачный тик не критичен, следующий тик повторит попытку.
  }
}

function startPolling() {
  stopPolling();
  polling.value = true;
  pendingTotal.value = null;
  pollPending();
  pollTimer = setInterval(pollPending, POLL_INTERVAL_MS);
}

function stopPolling() {
  polling.value = false;
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

onMounted(loadAttachments);
onBeforeUnmount(stopPolling);
</script>

<style scoped>
.abp {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.abp__title {
  margin: 0;
  font-size: 0.95em;
  font-weight: 600;
  color: var(--text);
}

.abp__hint {
  margin: 0;
  font-size: 0.85em;
  color: var(--text-muted);
  line-height: 1.4;
}

.abp__controls {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.abp__type-select {
  max-width: 320px;
}

.abp__actions {
  display: flex;
}

.abp__result {
  margin: 0;
  font-size: 0.85em;
  color: var(--text);
}

.abp__progress {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.form-error {
  color: var(--danger-text);
  font-size: 0.85em;
  margin: 0;
}

/* Тач-таргет поля периода - по брейкпоинту DateFilter (768), как в панели
   скачивания: на 768 его календарь уже лист, а поле осталось бы 35px. */
@media (max-width: 768px) {
  .abp__controls :deep(.date-field) {
    min-height: 44px;
  }
}

@media (max-width: 767.98px) {
  .abp__controls,
  .abp__progress {
    flex-direction: column;
    align-items: stretch;
  }

  /* Ширина поля периода зашита в 215px - в колонке тянем на всю строку. */
  .abp__controls :deep(.date-filter),
  .abp__controls :deep(.date-field) {
    width: 100%;
  }

  .abp__type-select {
    max-width: 100%;
  }

  .abp__type-select :deep(.base-dropdown__button) {
    min-height: 44px;
  }

  /* Тумблер кликается всей строкой с подписью - тач-таргет по её высоте. */
  .abp :deep(.toggle-switch) {
    min-height: 44px;
  }

  .abp__actions .lk-button,
  .abp__progress .lk-button {
    width: 100%;
    min-height: 44px;
  }
}
</style>
