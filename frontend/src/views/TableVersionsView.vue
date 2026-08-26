<template>
  <section class="versions-view">
    <header class="versions-header">
      <div class="versions-titlebar">
        <h2 class="versions-title">
          <RouterLink
            :to="backTo"
            class="versions-title__link"
          >
            <span class="versions-title__prefix">Таблица</span>
            <span class="versions-title__name">{{ displayName }}</span>
          </RouterLink>
          <span class="versions-title__sep">/ Версии</span>
        </h2>
        <RouterLink
          :to="backTo"
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
              stroke="currentColor"
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
      <!-- Строка действий: заголовок раздела слева, операции над версиями справа. -->
      <div class="versions-toolbar">
        <h3 class="versions-toolbar__title">
          Версии таблицы
        </h3>
        <div class="versions-toolbar__actions">
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
          <button
            v-if="canCleanup"
            type="button"
            class="lk-button lk-button--danger versions-action"
            :disabled="!tableID || !!error || cleanupRunning"
            data-testid="tv-cleanup"
            @click="openCleanup"
          >
            {{ cleanupRunning ? 'Очистка...' : 'Очистить старые версии' }}
          </button>
          <RefreshButton
            :loading="listLoading"
            @refresh="refresh"
          />
        </div>
      </div>

      <!-- Строка поиска ВЕРСИИ: выбор снимка из списка + сужение списка по дате.
           Отдельно от поиска по строкам таблицы (тот - в тулбаре самой таблицы). -->
      <div class="versions-filter">
        <div class="versions-filter__group">
          <span class="versions-filter__label">Версия</span>
          <BaseDropdown
            v-if="items.length"
            :model-value="selectedId"
            :options="versionOptions"
            label-key="label"
            value-key="id"
            placeholder="Выберите версию"
            searchable
            class="versions-filter__dropdown"
            data-testid="tv-version-select"
            @update:model-value="selectSnapshot"
          />
          <span
            v-else
            class="versions-filter__none"
          >
            {{ listLoading ? 'Загрузка...' : (dateFrom ? 'нет версий за период' : 'нет версий') }}
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
        </div>

        <div class="versions-filter__group">
          <span class="versions-filter__label">Найти по дате</span>
          <!-- Календарь (стандартный DateFilter проекта) - основной способ найти
               версию: список снимков растёт ~1/день. Диапазонный режим покрывает и
               один день (клик по дню = день-день), и период вроде "Прошлый месяц";
               границы берём из выбора и шлём в fetchList как from/to. -->
          <DateFilter
            :mode="'range'"
            :date-range-start="rangeStart"
            :date-range-end="rangeEnd"
            class="versions-filter__date"
            data-testid="tv-date-filter"
            @update:date-range-start="onRangeStart"
            @update:date-range-end="onRangeEnd"
            @apply="applyVersionDate"
            @clear="clearDate"
          />
        </div>
      </div>

      <div
        v-if="error"
        class="versions-state versions-state--error"
        data-testid="tv-error"
      >
        {{ error }}
      </div>

      <div
        v-else-if="!items.length && !listLoading"
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

      <!-- Метаданные выбранной версии: дата, тип, автор, счётчики. Берём из
           элемента списка (детальный ответ снимка автора/counts-в-удобной-форме
           не содержит), поэтому мета видна сразу, пока грузится payload. -->
      <div
        v-else-if="selectedItem"
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

      <div
        class="versions-footer"
        data-testid="tv-footer"
      >
        Всего версий: {{ total }}
      </div>
    </article>

    <!-- Сама таблица снимка - отдельной карточкой ПОД карточкой версий: свой
         тулбар (заголовок + поиск по строкам, поиск ВНУТРИ таблицы как на основной
         странице) и preview-режим реальных CarsTable/PeopleTable с колонками и
         строками на момент снимка. Показываем, когда есть версии и нет ошибки. -->
    <article
      v-if="!error && (items.length || listLoading) && !listError"
      class="versions-table-card"
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
        class="versions-table"
        data-testid="tv-preview"
      >
        <!-- Поиск по строкам - в РОДНОЙ шапке CarsTable/PeopleTable
             (слот header-actions), отдельного заголовка над таблицей нет. -->
        <div class="versions-preview">
          <CarsTable
            v-if="detailType === 'cars'"
            :preview="true"
            :preview-fields="previewFields"
            :preview-items="previewItems"
            :search-query="searchQuery"
            :table-id="tableID"
          >
            <template #header-actions>
              <SearchComponent
                v-model="searchQuery"
                title="Поиск по строкам"
              />
            </template>
          </CarsTable>
          <PeopleTable
            v-else-if="detailType === 'people'"
            :preview="true"
            :preview-fields="previewFields"
            :preview-items="previewItems"
            :search-query="searchQuery"
            :table-name="''"
          >
            <template #header-actions>
              <SearchComponent
                v-model="searchQuery"
                title="Поиск по строкам"
              />
            </template>
          </PeopleTable>
        </div>
      </div>
    </article>

    <BaseModal
      :show="cleanupOpen"
      title="Очистка старых версий"
      width="440px"
      :close-on-overlay="!cleanupRunning"
      :closable="!cleanupRunning"
      data-testid="tv-cleanup-modal"
      @close="closeCleanup"
    >
      <div class="cleanup-dialog">
        <span class="cleanup-dialog__label">Удалить версии старше:</span>
        <FilterTabs
          v-model="cleanupPeriod"
          :tabs="CLEANUP_TABS"
          data-testid="tv-cleanup-period"
        />
        <p
          class="cleanup-dialog__hint"
          data-testid="tv-cleanup-hint"
        >
          {{ cleanupHint }}
        </p>
      </div>
      <template #actions>
        <button
          type="button"
          class="lk-button lk-button--secondary"
          :disabled="cleanupRunning"
          @click="closeCleanup"
        >
          Отмена
        </button>
        <button
          type="button"
          class="lk-button lk-button--danger"
          :disabled="cleanupRunning"
          data-testid="tv-cleanup-confirm"
          @click="confirmCleanup"
        >
          {{ cleanupRunning ? 'Удаление...' : 'Удалить' }}
        </button>
      </template>
    </BaseModal>
  </section>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import RefreshButton from '@/components/RefreshButton.vue';
import Badge from '@/components/ui/Badge.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import BaseModal from '@/components/ui/BaseModal.vue';
import FilterTabs from '@/components/ui/FilterTabs.vue';
import SearchComponent from '@/components/SearchComponent.vue';
import DateFilter from '@/components/DateFilter.vue';
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

// Максимум бэка (per_page 1-100). Грузим крупным блоком, чтобы поиск в дропдауне
// версий покрывал сразу много снимков; дальше "Ещё"/календарь по дате.
const PER_PAGE = 100;

// Периоды чистки как сегмент-контрол (FilterTabs требует строковые key).
// Дефолт хранения версий - 24 месяца = "2 лет" (context.md).
const CLEANUP_TABS = [
  { key: '12', label: '1 года' },
  { key: '24', label: '2 лет' },
];

const route = useRoute();
const deletions = useDeletionsStore();
const { can } = usePermission();

const tableName = computed(() => route.params.tableName);

// Куда ведёт "Назад": из админ-конструктора (from=admin, архивная таблица) - обратно
// в конструктор, с open=<name> чтобы он открыл архив и выбрал ту же таблицу (юзер
// вернётся ровно туда, где был); с публичной страницы таблицы - на неё же.
const backTo = computed(() =>
  route.query?.from === 'admin'
    ? { path: '/table-constructor', query: { open: tableName.value } }
    : `/table/${tableName.value}`,
);

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

// Поиск по строкам показанной версии (фильтрует внутри CarsTable/PeopleTable).
const searchQuery = ref('');
// Применённые границы периода (ISO, уходят в BE как from/to); пустые - фильтра нет.
const dateFrom = ref('');
const dateTo = ref('');
// Выбор в календаре до нажатия "Применить".
const rangeStart = ref(null);
const rangeEnd = ref(null);

// Действия: ручной снимок, экспорт выбранной версии, чистка старых.
const snapshotSaving = ref(false);
const exporting = ref(''); // '' | 'xlsx' | 'pdf' - какой формат сейчас выгружается
// Выбранный порог чистки как строка (key сегмента), в месяцы конвертим при вызове API.
const cleanupPeriod = ref('24');
const cleanupOpen = ref(false);
const cleanupRunning = ref(false);

// Кнопку чистки показываем только тем, кого пустит BE-гейт requireAdmin
// (page.admin) - иначе "вижу кнопку, но 403" (#976). super/admin проходят.
const canCleanup = computed(() => can('page.admin'));

// Пояснение под сегментом: что именно удалится. Грамматика согласована с выбором.
const cleanupHint = computed(() => {
  const phrase = cleanupPeriod.value === '12' ? 'одного года' : 'двух лет';
  return `Будут безвозвратно удалены все версии этой таблицы старше ${phrase}. Более свежие версии сохранятся.`;
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

// Границы выбранного периода в ISO (RFC3339) по ЛОКАЛЬНОЙ зоне браузера. BE парсит
// голую YYYY-MM-DD в UTC, а дропдаун версий рендерит taken_at через formatDateTime
// в локальной зоне - для снимка, чьё UTC-время попадает в другой календарный день,
// фильтр по видимой дате разъехался бы на сутки. Локальные границы + офсет в ISO
// (parseSnapshotBound понимает RFC3339) держат фильтр в тех же сутках, что и лейбл.
function dayStartISO(date) {
  const d = new Date(date);
  d.setHours(0, 0, 0, 0);
  return d.toISOString();
}

function dayEndISO(date) {
  const d = new Date(date);
  d.setHours(23, 59, 59, 999);
  return d.toISOString();
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
    // allow_archived: версии открываются из архива, где таблица уже is_active=false;
    // без флага резолвер по имени её не найдёт и вернёт "Таблица не найдена".
    const res = await apiRequest(`/system-tables/name/${tableName.value}?allow_archived=1`);
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
      from: dateFrom.value,
      to: dateTo.value,
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
    dateFrom.value = '';
    dateTo.value = '';
    rangeStart.value = null;
    rangeEnd.value = null;
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
  if (cleanupRunning.value || !tableID.value) return;
  // Каждый раз открываем с дефолтным порогом - предсказуемо, без залипшего выбора.
  cleanupPeriod.value = '24';
  cleanupOpen.value = true;
}

function closeCleanup() {
  if (cleanupRunning.value) return;
  cleanupOpen.value = false;
}

async function confirmCleanup() {
  if (cleanupRunning.value || !tableID.value) return;
  cleanupRunning.value = true;
  try {
    const { deleted } = await cleanupTableSnapshots(tableID.value, Number(cleanupPeriod.value));
    cleanupOpen.value = false;
    if (deleted > 0) {
      deletions.notify({ prefix: 'Удалено старых версий:', bold: String(deleted), type: 'success' });
      refresh();
    } else {
      deletions.notify({ prefix: 'Старых версий для удаления не нашлось', type: 'success' });
    }
  } catch {
    // Модалку не закрываем - даём повторить или отменить.
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

// Календарь эмитит границы выбора перед apply - запоминаем, применяем по кнопке.
function onRangeStart(date) {
  rangeStart.value = date;
}

function onRangeEnd(date) {
  rangeEnd.value = date;
}

// Применение выбора в календаре - сузить список версий к периоду и автовыбрать первую.
function applyVersionDate() {
  const start = rangeStart.value || rangeEnd.value;
  const end = rangeEnd.value || rangeStart.value;
  dateFrom.value = start ? dayStartISO(start) : '';
  dateTo.value = end ? dayEndISO(end) : '';
  refresh();
}

function clearDate() {
  rangeStart.value = null;
  rangeEnd.value = null;
  if (!dateFrom.value && !dateTo.value) return;
  dateFrom.value = '';
  dateTo.value = '';
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
  color: var(--text-muted);
  transition: color 0.2s ease;
}

.versions-title__prefix {
  margin-right: 0.35em;
}

.versions-title__sep {
  margin-left: 0.35em;
  color: var(--text);
}

.versions-title__link:hover .versions-title__prefix {
  color: var(--text);
}

.versions-title__link:hover .versions-title__name {
  color: var(--accent-text);
}

.versions-back-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 25px;
  padding: 0 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 50px;
  font-weight: 500;
  font-size: 14px;
  color: var(--accent-text);
  text-decoration: none;
  white-space: nowrap;
  transition: all 0.2s ease;
}

.versions-back-btn:hover {
  background: var(--surface-2);
  border-color: var(--accent);
}

.versions-back-btn__icon {
  color: var(--accent-text);
  width: 14px;
  height: 14px;
}

.versions-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 30px;
  /* overflow: visible - иначе выпадающее меню версии (BaseDropdown, position:
     absolute, z-index 1000, без Teleport) обрезается краем низкой карточки.
     Дети прозрачные, скругление 30px держится на самой карточке, углы целы.
     Попап DateFilter через Teleport/fixed - от overflow не зависит. */
  overflow: visible;
  display: flex;
  flex-direction: column;
}

/* Сама таблица снимка - отдельной карточкой под карточкой версий. */
.versions-table-card {
  margin-top: 15px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 30px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.versions-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px 16px;
  padding: 14px 20px;
  border-bottom: 1px solid var(--border);
  flex-wrap: wrap;
}

.versions-toolbar__title {
  margin: 0;
  font-weight: 700;
  font-size: 16px;
  color: var(--text);
}

.versions-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.versions-filter {
  display: flex;
  align-items: center;
  gap: 12px 28px;
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
  flex-wrap: wrap;
}

.versions-filter__group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.versions-filter__label {
  font-weight: 600;
  font-size: 14px;
  color: var(--text);
  white-space: nowrap;
}

/* Фиксированная ширина контролов фильтра - не скачут при смене выбранной версии/даты. */
.versions-filter__dropdown {
  width: 280px;
}

.versions-filter__date {
  flex-shrink: 0;
}

.versions-filter__none {
  font-size: 14px;
  color: var(--text-muted);
}

.versions-action {
  height: 34px;
  padding: 0 16px;
  font-size: 13px;
}

.cleanup-dialog {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 20px;
}

.cleanup-dialog__label {
  font-weight: 600;
  font-size: 15px;
  color: var(--text);
}

.cleanup-dialog__hint {
  margin: 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--text-muted);
}

.versions-load-more {
  height: 30px;
  padding: 0 14px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 50px;
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  font-weight: 500;
  color: var(--accent-text);
  cursor: pointer;
  transition: background 0.2s ease;
}

.versions-load-more:hover:not(:disabled) {
  background: var(--surface-2);
}

.versions-load-more:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.versions-table {
  display: flex;
  flex-direction: column;
}

.versions-meta {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 14px 20px;
  border-bottom: 1px solid var(--border);
}

.versions-meta__head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.versions-meta__date {
  font-weight: 600;
  font-size: 15px;
  color: var(--text);
}

.versions-meta__actor {
  font-size: 13px;
  color: var(--text-muted);
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
  color: var(--success-text);
}

.versions-count--exit {
  color: var(--text-muted);
}

.versions-count--not {
  color: var(--warning-text);
}

.versions-count--total {
  color: var(--text);
}

/* Preview-таблица занимает всю ширину как основная страница; её собственные
   отступы/скролл управляются самим компонентом CarsTable/PeopleTable. */
.versions-preview {
  padding: 4px 6px 10px;
}

/* Preview рендерит реальный CarsTable/PeopleTable, чей корень .selected-table-card
   имеет свою рамку+скругление 30px - внутри нашей карточки таблицы это лишняя
   "карточка в карточке". Снимаем рамку и радиус у вложенной таблицы. */
.versions-preview :deep(.selected-table-card) {
  border: none;
  border-radius: 0;
}

.versions-footer {
  flex-shrink: 0;
  padding: 12px 20px;
  border-top: 1px solid var(--border);
  font-size: 14px;
  color: var(--text-muted);
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
  color: var(--text-muted);
  font-size: 14px;
}

.versions-state--error {
  color: var(--danger-text);
}

.versions-empty p {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}

.versions-empty__hint {
  font-size: 13px;
  color: var(--text-muted);
  max-width: 360px;
}

.versions-spinner {
  width: 28px;
  height: 28px;
  border: 3px solid var(--border);
  border-top-color: var(--accent-text);
  border-radius: 50%;
  animation: versions-spin 0.8s linear infinite;
}

@keyframes versions-spin {
  to { transform: rotate(360deg); }
}

/* --- Мобилка. Брейкпоинт 768 намеренно совпадает с DateFilter
   (.date-filter -> width:100% на <=768): фильтр-стек и растяжение календаря
   должны включаться в один момент. Иначе на строчной раскладке календарь
   шириной 100% распирает строку фильтра и вылезает за экран на ~100px, из-за
   чего мобильный браузер ужимает страницу (shrink-to-fit) и layout viewport
   растёт до ~502 вместо 390. Сама preview-таблица (CarsTable/PeopleTable)
   превращается в карточки своей инфрой responsive-tables.css (767.98) - тут её
   не трогаем. */
@media (max-width: 768px) {
  .versions-view {
    padding: 12px;
  }

  /* Заголовок слева, «Назад» справа; длинное имя таблицы переносится строкой,
     а не толкает кнопку за край экрана. */
  .versions-titlebar {
    justify-content: space-between;
    align-items: flex-start;
  }

  .versions-title {
    min-width: 0;
    font-size: 17px;
    line-height: 21px;
  }

  .versions-back-btn {
    flex-shrink: 0;
  }

  .versions-toolbar {
    padding: 12px 14px;
  }

  .versions-meta,
  .versions-footer {
    padding-left: 14px;
    padding-right: 14px;
  }

  /* Фильтр версий в столбик: селектор версии и календарь - каждый на своей строке
     во всю ширину группы. Подпись поля идёт над контролом. */
  .versions-filter {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
    padding: 12px 14px;
  }

  .versions-filter__group {
    flex-direction: column;
    align-items: stretch;
    gap: 6px;
  }

  /* Фиксированная десктопная ширина 280px уступает место полной ширине группы. */
  .versions-filter__dropdown,
  .versions-filter__date {
    width: 100%;
  }

  /* «Ещё» (появляется при >100 версий) - интринзик-пилюля: при stretch-группе
     иначе растянулась бы во всю ширину. */
  .versions-load-more {
    align-self: flex-start;
  }
}

@media (max-width: 480px) {
  .versions-view {
    padding: 10px;
  }
}
</style>
