<template>
  <section class="versions-view">
    <header class="versions-header">
      <div class="versions-titlebar">
        <h2 class="versions-title">
          <RouterLink
            :to="`/table/${tableName}`"
            class="versions-title__link"
          >
            <span class="versions-title__prefix">Таблица</span>
            <span class="versions-title__name">{{ displayName }}</span>
          </RouterLink>
          <span class="versions-title__sep">/ Версии</span>
        </h2>
        <RouterLink
          :to="`/table/${tableName}`"
          class="versions-back-btn"
          data-testid="tv-back"
        >
          <svg
            class="versions-back-btn__icon"
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
          >
            <path
              d="M15 18L9 12L15 6"
              stroke="#4F5BDF"
              stroke-width="2.5"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
          Назад
        </RouterLink>
      </div>
    </header>

    <article class="versions-card">
      <div class="versions-card__header">
        <div class="versions-picker">
          <span class="versions-picker__label">Версия</span>
          <BaseDropdown
            v-if="items.length"
            :model-value="selectedId"
            :options="versionOptions"
            label-key="label"
            value-key="id"
            placeholder="Выберите версию"
            class="versions-picker__dropdown"
            data-testid="tv-version-select"
            @update:model-value="selectSnapshot"
          />
          <span
            v-else
            class="versions-picker__none"
          >
            {{ listLoading ? 'Загрузка...' : (dateFilter ? 'нет версий за дату' : 'нет версий') }}
          </span>
          <button
            v-if="items.length < total"
            type="button"
            class="versions-load-more"
            :disabled="listLoading"
            data-testid="tv-load-more"
            @click="loadMore"
          >
            Ещё
          </button>
          <label class="versions-datefilter">
            <span class="versions-datefilter__label">Дата</span>
            <input
              type="date"
              class="versions-datefilter__input"
              :value="dateFilter"
              data-testid="tv-date-filter"
              @change="onDateChange"
            >
            <button
              v-if="dateFilter"
              type="button"
              class="versions-datefilter__clear"
              aria-label="Сбросить дату"
              data-testid="tv-date-clear"
              @click="clearDate"
            >
              &times;
            </button>
          </label>
        </div>

        <div class="versions-card__spacer" />

        <button
          type="button"
          class="lk-button lk-button--primary versions-action"
          :disabled="!tableID || !!error || snapshotSaving"
          data-testid="tv-snapshot-now"
          @click="saveSnapshotNow"
        >
          {{ snapshotSaving ? 'Сохранение...' : 'Сохранить сейчас' }}
        </button>
        <button
          type="button"
          class="lk-button lk-button--secondary versions-action"
          :disabled="selectedId === null || exporting !== ''"
          data-testid="tv-export-xlsx"
          @click="exportSnapshot('xlsx')"
        >
          {{ exporting === 'xlsx' ? 'Выгрузка...' : 'Excel' }}
        </button>
        <button
          type="button"
          class="lk-button lk-button--ghost versions-action"
          :disabled="selectedId === null || exporting !== ''"
          data-testid="tv-export-pdf"
          @click="exportSnapshot('pdf')"
        >
          {{ exporting === 'pdf' ? 'Выгрузка...' : 'PDF' }}
        </button>
        <div
          v-if="canCleanup"
          class="versions-cleanup"
        >
          <select
            v-model.number="cleanupMonths"
            class="lk-select versions-cleanup__select"
            :disabled="cleanupRunning"
            data-testid="tv-cleanup-period"
          >
            <option
              v-for="opt in CLEANUP_OPTIONS"
              :key="opt.months"
              :value="opt.months"
            >
              {{ opt.label }}
            </option>
          </select>
          <button
            type="button"
            class="lk-button lk-button--danger versions-action"
            :disabled="!tableID || !!error || cleanupRunning"
            data-testid="tv-cleanup"
            @click="openCleanup"
          >
            Очистить старые
          </button>
        </div>
        <RefreshButton
          :loading="listLoading"
          @refresh="refresh"
        />
      </div>

      <div
        v-if="error"
        class="versions-state versions-state--error"
        data-testid="tv-error"
      >
        {{ error }}
      </div>

      <template v-else>
        <div
          v-if="!items.length && !listLoading"
          class="versions-state versions-empty"
          data-testid="tv-empty"
        >
          <p>Версий пока нет</p>
          <span class="versions-empty__hint">
            Снимки состояния создаются автоматически ночью перед сбросом статусов
            (в 06:00) и вручную кнопкой «Сохранить сейчас».
          </span>
        </div>

        <div
          v-else-if="listError"
          class="versions-state versions-state--error"
          data-testid="tv-list-error"
        >
          Не удалось загрузить версии
        </div>

        <template v-else>
          <!-- Метаданные выбранной версии: дата, тип, автор, счётчики. Берём из
               элемента списка (детальный ответ снимка автора/counts-в-удобной-форме
               не содержит), поэтому мета видна сразу, пока грузится payload. -->
          <div
            v-if="selectedItem"
            class="versions-meta"
            data-testid="tv-meta"
          >
            <div class="versions-meta__head">
              <Badge
                :variant="reasonVariant(selectedItem.reason)"
                size="sm"
              >
                {{ reasonLabel(selectedItem.reason) }}
              </Badge>
              <span class="versions-meta__date">{{ formatDateTime(selectedItem.taken_at) }}</span>
              <span
                v-if="selectedItem.reason === 'manual' && selectedItem.actor_name"
                class="versions-meta__actor"
              >
                {{ selectedItem.actor_name }}
              </span>
            </div>
            <div class="versions-meta__counts">
              <span class="versions-count versions-count--on">На территории: {{ detailCounts.on_territory }}</span>
              <span class="versions-count versions-count--exit">Выехал: {{ detailCounts.exited }}</span>
              <span class="versions-count versions-count--not">Не въезжал: {{ detailCounts.not_entered }}</span>
              <span class="versions-count versions-count--total">Всего: {{ detailCounts.total }}</span>
            </div>
          </div>

          <!-- Поиск по строкам снимка. SearchComponent как на основной странице;
               фильтрация идёт внутри CarsTable/PeopleTable по :search-query тем же
               buildSearchVariants/matchesSearch, счётчики версии не трогает. -->
          <div
            v-if="detail && previewItems.length"
            class="versions-subbar"
            data-testid="tv-subbar"
          >
            <SearchComponent
              v-model="searchQuery"
              title="Поиск по таблице"
              class="versions-search"
            />
          </div>

          <!-- Таблица снимка на всю ширину: preview-режим реальных CarsTable/PeopleTable
               с колонками (previewFields) и строками (previewItems) на момент снимка. -->
          <div
            class="versions-body"
            data-testid="tv-body"
          >
            <div
              v-if="!detail"
              class="versions-state"
              data-testid="tv-detail-loading"
            >
              <span class="versions-spinner" />
            </div>
            <div
              v-else-if="!previewItems.length"
              class="versions-state"
              data-testid="tv-detail-empty"
            >
              На момент снимка таблица была пуста
            </div>
            <div
              v-else
              class="versions-preview"
              data-testid="tv-preview"
            >
              <CarsTable
                v-if="detailType === 'cars'"
                :preview="true"
                :preview-fields="previewFields"
                :preview-items="previewItems"
                :search-query="searchQuery"
                :table-id="tableID"
              />
              <PeopleTable
                v-else-if="detailType === 'people'"
                :preview="true"
                :preview-fields="previewFields"
                :preview-items="previewItems"
                :search-query="searchQuery"
                :table-name="''"
              />
            </div>
          </div>
        </template>
      </template>

      <div
        class="versions-footer"
        data-testid="tv-footer"
      >
        Всего версий: {{ total }}
      </div>
    </article>

    <ConfirmationModal
      :show="cleanupOpen"
      title="Очистка старых версий"
      :message="cleanupMessage"
      confirm-text="Удалить"
      cancel-text="Отмена"
      @confirm="confirmCleanup"
      @cancel="cleanupOpen = false"
    />
  </section>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import RefreshButton from '@/components/RefreshButton.vue';
import Badge from '@/components/ui/Badge.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import ConfirmationModal from '@/components/ConfirmationModal.vue';
import SearchComponent from '@/components/SearchComponent.vue';
import CarsTable from '@/components/CarsTable.vue';
import PeopleTable from '@/components/PeopleTable.vue';
import { apiRequest } from '@/api/client';
import {
  listTableSnapshots,
  getTableSnapshot,
  createTableSnapshot,
  exportTableSnapshot,
  cleanupTableSnapshots,
} from '@/api/system-tables';
import { saveBlobAs } from '@/api/attachment-templates';
import { usePermission } from '@/composables/usePermission';
import { useDeletionsStore } from '@/stores/deletions';
import { formatDateTime } from '@/utils/datetime';
import { normalizeSnapshotRows } from '@/utils/snapshotRows';

const PER_PAGE = 20;

// Периоды чистки: дефолт хранения версий - 24 месяца (context.md).
const CLEANUP_OPTIONS = [
  { months: 12, label: 'Старше 1 года' },
  { months: 24, label: 'Старше 2 лет' },
];

const route = useRoute();
const deletions = useDeletionsStore();
const { can } = usePermission();

const tableName = computed(() => route.params.tableName);

const tableID = ref(0);
const tableType = ref('');
const displayName = ref('');
const error = ref('');
// Текущие поля таблицы - фолбэк колонок для старых снимков без payload.fields
// (снимки до r1). Приходят тем же ответом, что и таблица (data.fields).
const fallbackFields = ref([]);

const items = ref([]);
const total = ref(0);
const page = ref(1);
const listLoading = ref(false);
// Ошибка загрузки списка отдельно от "версий пока нет": пустой items при провале
// иначе рисует тот же блок "версий нет" и вводит в заблуждение (тост исчезает).
const listError = ref(false);

const selectedId = ref(null);
const detail = ref(null);

// Поиск по строкам показанной версии (фильтрует внутри CarsTable/PeopleTable) и
// фильтр списка версий по дате (YYYY-MM-DD, уходит как from=to в BE-эндпоинт).
const searchQuery = ref('');
const dateFilter = ref('');

// Действия: ручной снимок, экспорт выбранной версии, чистка старых.
const snapshotSaving = ref(false);
const exporting = ref(''); // '' | 'xlsx' | 'pdf' - какой формат сейчас выгружается
const cleanupMonths = ref(24);
const cleanupOpen = ref(false);
const cleanupRunning = ref(false);

// Кнопку чистки показываем только тем, кого пустит BE-гейт requireAdmin
// (page.admin) - иначе "вижу кнопку, но 403" (#976). super/admin проходят.
const canCleanup = computed(() => can('page.admin'));

const cleanupMessage = computed(() => {
  const opt = CLEANUP_OPTIONS.find((o) => o.months === cleanupMonths.value);
  const label = opt ? opt.label.toLowerCase() : `старше ${cleanupMonths.value} мес.`;
  return `Удалить все сохранённые версии таблицы ${label}? Действие необратимо.`;
});

// Токены последовательности от гонки устаревшего ответа (#632): быстрое
// переключение версий/повторный refresh пускают параллельные запросы в общий ref,
// применяем только ответ последнего.
let listSeq = 0;
let detailSeq = 0;

const REASON_LABELS = { scheduled: 'Плановый', manual: 'Ручной' };
const REASON_VARIANTS = { scheduled: 'neutral', manual: 'primary' };

function reasonLabel(reason) {
  return REASON_LABELS[reason] || reason || 'Снимок';
}

function reasonVariant(reason) {
  return REASON_VARIANTS[reason] || 'neutral';
}

// Опции дропдауна версий: дата + тип. Счётчики показываем под дропдауном для
// выбранной версии, чтобы метка оставалась короткой.
const versionOptions = computed(() =>
  items.value.map((s) => ({
    id: s.id,
    label: `${formatDateTime(s.taken_at)} · ${reasonLabel(s.reason)}`,
  })),
);

// Тип и колонки берём из payload снимка (что было на момент), а не из текущей
// таблицы - снимок самодостаточен. Для старых снимков без fields - фолбэк на
// текущие поля таблицы.
const detailType = computed(() => detail.value?.payload?.table_type || tableType.value);
const previewFields = computed(() => {
  const snapFields = detail.value?.payload?.fields;
  if (Array.isArray(snapFields) && snapFields.length) return snapFields;
  return fallbackFields.value;
});
const previewItems = computed(() =>
  normalizeSnapshotRows(detail.value?.payload?.rows, detailType.value),
);
const selectedItem = computed(
  () => items.value.find((s) => s.id === selectedId.value) || null,
);
const detailCounts = computed(
  () => selectedItem.value?.counts || { on_territory: 0, exited: 0, not_entered: 0, total: 0 },
);

async function fetchTable() {
  error.value = '';
  try {
    const res = await apiRequest(`/system-tables/name/${tableName.value}`);
    const data = await res.json();
    const tbl = (data && data.table) || data;
    if (!tbl || !tbl.id) {
      error.value = 'Таблица не найдена';
      return false;
    }
    tableID.value = tbl.id;
    tableType.value = tbl.table_type;
    displayName.value = tbl.display_name || tbl.name || tableName.value;
    fallbackFields.value = Array.isArray(data?.fields) ? data.fields : [];
    if (tableType.value !== 'cars' && tableType.value !== 'people') {
      error.value = 'Этот тип таблицы не поддерживает версии';
      return false;
    }
    return true;
  } catch {
    error.value = 'Ошибка загрузки таблицы';
    return false;
  }
}

async function fetchList({ reset = true } = {}) {
  if (!tableID.value) return;
  if (reset) page.value = 1;
  listLoading.value = true;
  listError.value = false;
  const seq = ++listSeq;
  try {
    const { items: data, total: t } = await listTableSnapshots(tableID.value, {
      page: page.value,
      perPage: PER_PAGE,
      // Один день: from=to=дата, BE трактует to как конец суток включительно.
      from: dateFilter.value,
      to: dateFilter.value,
    });
    if (seq !== listSeq) return;
    items.value = reset ? data : [...items.value, ...data];
    total.value = t;
    // Автовыбор первого снимка на свежем списке - сразу показываем состав.
    if (reset && items.value.length && selectedId.value === null) {
      selectSnapshot(items.value[0].id);
    }
  } catch {
    if (seq !== listSeq) return;
    if (reset) listError.value = true;
    deletions.notify({ prefix: 'Не удалось загрузить версии', type: 'error' });
  } finally {
    if (seq === listSeq) listLoading.value = false;
  }
}

async function selectSnapshot(id) {
  selectedId.value = id;
  detail.value = null;
  const seq = ++detailSeq;
  try {
    const data = await getTableSnapshot(tableID.value, id);
    if (seq !== detailSeq) return;
    detail.value = data;
  } catch {
    if (seq !== detailSeq) return;
    selectedId.value = null;
    deletions.notify({ prefix: 'Не удалось открыть версию', type: 'error' });
  }
}

async function saveSnapshotNow() {
  if (snapshotSaving.value || !tableID.value) return;
  snapshotSaving.value = true;
  try {
    await createTableSnapshot(tableID.value);
    deletions.notify({ prefix: 'Сохранена версия таблицы', bold: displayName.value, type: 'success' });
    // Сбрасываем фильтр даты - свежий снимок сегодняшний, под старым фильтром не виден.
    dateFilter.value = '';
    refresh();
  } catch {
    deletions.notify({ prefix: 'Не удалось сохранить версию', type: 'error' });
  } finally {
    snapshotSaving.value = false;
  }
}

async function exportSnapshot(format) {
  if (exporting.value || selectedId.value === null) return;
  exporting.value = format;
  try {
    const { blob, filename } = await exportTableSnapshot(tableID.value, selectedId.value, format);
    saveBlobAs(blob, filename);
  } catch {
    deletions.notify({ prefix: 'Не удалось выгрузить файл', type: 'error' });
  } finally {
    exporting.value = '';
  }
}

function openCleanup() {
  if (cleanupRunning.value) return;
  cleanupOpen.value = true;
}

async function confirmCleanup() {
  cleanupOpen.value = false;
  if (cleanupRunning.value || !tableID.value) return;
  cleanupRunning.value = true;
  try {
    const { deleted } = await cleanupTableSnapshots(tableID.value, cleanupMonths.value);
    if (deleted > 0) {
      deletions.notify({ prefix: 'Удалено старых версий:', bold: String(deleted), type: 'success' });
      refresh();
    } else {
      deletions.notify({ prefix: 'Старых версий для удаления не нашлось', type: 'success' });
    }
  } catch {
    deletions.notify({ prefix: 'Не удалось очистить старые версии', type: 'error' });
  } finally {
    cleanupRunning.value = false;
  }
}

function loadMore() {
  if (listLoading.value) return;
  page.value += 1;
  fetchList({ reset: false });
}

function refresh() {
  selectedId.value = null;
  detail.value = null;
  fetchList({ reset: true });
}

// Смена даты - сузить список версий к выбранному дню и автовыбрать первую.
function onDateChange(e) {
  dateFilter.value = e.target.value || '';
  refresh();
}

function clearDate() {
  if (!dateFilter.value) return;
  dateFilter.value = '';
  refresh();
}

onMounted(async () => {
  const ok = await fetchTable();
  if (ok) await fetchList({ reset: true });
});
</script>

<style scoped>
.versions-view {
  padding: 20px;
  font-family: 'Montserrat', sans-serif;
  position: relative;
}

.versions-header {
  display: flex;
  flex-direction: column;
  gap: 15px;
  margin-bottom: 15px;
}

.versions-titlebar {
  display: flex;
  align-items: center;
  gap: 10px;
}

.versions-title {
  margin: 0;
  font-weight: 700;
  font-size: 18px;
  line-height: 22px;
}

.versions-title__link {
  text-decoration: none;
}

.versions-title__prefix,
.versions-title__name {
  color: #a2a2a2;
  transition: color 0.2s ease;
}

.versions-title__prefix {
  margin-right: 0.35em;
}

.versions-title__sep {
  margin-left: 0.35em;
  color: #000;
}

.versions-title__link:hover .versions-title__prefix {
  color: #000;
}

.versions-title__link:hover .versions-title__name {
  color: #4f5bdf;
}

.versions-back-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 25px;
  padding: 0 12px;
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 50px;
  font-weight: 500;
  font-size: 14px;
  color: #4f5bdf;
  text-decoration: none;
  white-space: nowrap;
  transition: all 0.2s ease;
}

.versions-back-btn:hover {
  background: #f2f2f2;
  border-color: #4f5bdf;
}

.versions-back-btn__icon {
  width: 14px;
  height: 14px;
}

.versions-card {
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 30px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.versions-card__header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 20px;
  min-height: 50px;
  border-bottom: 1px solid #e6e6e6;
  flex-shrink: 0;
  flex-wrap: wrap;
}

.versions-picker {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
}

.versions-picker__label {
  font-weight: 600;
  font-size: 15px;
  color: #000;
}

.versions-picker__dropdown {
  min-width: 260px;
}

.versions-picker__none {
  font-size: 14px;
  color: #a2a2a2;
}

.versions-card__spacer {
  flex: 1;
}

.versions-action {
  height: 34px;
  padding: 0 16px;
  font-size: 13px;
}

.versions-cleanup {
  display: flex;
  align-items: center;
  gap: 8px;
}

.versions-cleanup__select {
  height: 34px;
  padding: 0 12px;
  font-size: 13px;
  width: auto;
}

.versions-load-more {
  height: 30px;
  padding: 0 14px;
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 50px;
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  font-weight: 500;
  color: #4f5bdf;
  cursor: pointer;
  transition: background 0.2s ease;
}

.versions-load-more:hover:not(:disabled) {
  background: #f2f2f2;
}

.versions-load-more:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.versions-datefilter {
  display: flex;
  align-items: center;
  gap: 8px;
}

.versions-datefilter__label {
  font-weight: 600;
  font-size: 15px;
  color: #000;
}

.versions-datefilter__input {
  height: 34px;
  padding: 0 10px;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  color: #1a1a1a;
}

.versions-datefilter__clear {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  background: #f2f2f2;
  border: none;
  border-radius: 50%;
  font-size: 16px;
  line-height: 1;
  color: #666;
  cursor: pointer;
  transition: background 0.2s ease;
}

.versions-datefilter__clear:hover {
  background: #e6e6e6;
  color: #000;
}

.versions-subbar {
  display: flex;
  align-items: center;
  padding: 12px 20px 0;
}

.versions-meta {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 14px 20px;
  border-bottom: 1px solid #f0f0f0;
}

.versions-meta__head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.versions-meta__date {
  font-weight: 600;
  font-size: 15px;
  color: #1a1a1a;
}

.versions-meta__actor {
  font-size: 13px;
  color: #8a8a8a;
}

.versions-meta__counts {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 16px;
}

.versions-count {
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
}

.versions-count--on {
  color: #1f9d55;
}

.versions-count--exit {
  color: #8a8a8a;
}

.versions-count--not {
  color: #a2792b;
}

.versions-count--total {
  color: #555;
}

.versions-body {
  min-height: 200px;
}

/* Preview-таблица занимает всю ширину как основная страница; её собственные
   отступы/скролл управляются самим компонентом CarsTable/PeopleTable. */
.versions-preview {
  padding: 4px 6px 10px;
}

.versions-footer {
  flex-shrink: 0;
  padding: 12px 20px;
  border-top: 1px solid #e6e6e6;
  font-size: 14px;
  color: #666;
}

.versions-state {
  min-height: 200px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 40px 24px;
  text-align: center;
  color: #a2a2a2;
  font-size: 14px;
}

.versions-state--error {
  color: #ff6668;
}

.versions-empty p {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #333;
}

.versions-empty__hint {
  font-size: 13px;
  color: #a2a2a2;
  max-width: 360px;
}

.versions-spinner {
  width: 28px;
  height: 28px;
  border: 3px solid #e6e6e6;
  border-top-color: #4f5bdf;
  border-radius: 50%;
  animation: versions-spin 0.8s linear infinite;
}

@keyframes versions-spin {
  to { transform: rotate(360deg); }
}
</style>
