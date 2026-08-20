<template>
  <div>
    <div class="chart-section">
      <div class="chart-header">
        <h4 class="chart-title">
          Запросы {{ selectedPeriod.title }}
        </h4>
        <div class="chart-header-actions">
          <span class="chart-interval">интервал: {{ selectedPeriod.intervalHuman }}</span>
          <BaseDropdown
            :model-value="chartPeriod"
            class="chart-period-dd"
            :options="chartPeriodOptions"
            value-key="value"
            label-key="label"
            @update:model-value="onChartPeriodChange"
          />
        </div>
      </div>
      <RealTimeChart
        :data="timelineData"
        :height="180"
        :interval-label="selectedPeriod.xAxisLabel"
      />
    </div>

    <div class="filters-bar">
      <SearchComponent
        v-model="state.search"
        :title="'Поиск по логам...'"
        @keyup.enter="refreshLogs"
      />
      <div class="filter-controls">
        <BaseDropdown
          v-for="dd in filterDropdowns"
          :key="dd.key"
          :model-value="dd.value"
          :class="['filter-dd', { 'filter-dd--wide': dd.wide }]"
          :options="dd.options"
          value-key="value"
          label-key="label"
          :placeholder="dd.placeholder"
          :searchable="dd.searchable"
          @update:model-value="value => applyFilter(dd.key, value)"
        />
        <DateFilter
          mode="range"
          :date-range-start="journalRange.start"
          :date-range-end="journalRange.end"
          @update:date-range-start="value => (state.from = dateToYmd(value))"
          @update:date-range-end="value => (state.to = dateToYmd(value))"
          @apply="onDateFilterChange"
          @clear="onDateFilterChange"
        />
      </div>
      <div class="filter-presets">
        <button
          v-for="preset in journalPresets"
          :key="preset.key"
          class="lk-button lk-button--sm"
          :class="presetOn(preset.key) ? 'lk-button--secondary' : 'lk-button--ghost'"
          :title="preset.title"
          @click="togglePreset(preset.key)"
        >
          {{ preset.label }}
        </button>
      </div>
      <ToggleSwitch
        v-model="autoRefresh"
        :title="refreshBlock || 'Список обновляется сам каждые 10 секунд'"
      >
        Лента{{ autoRefresh && refreshBlock ? ` (${refreshBlock})` : '' }}
      </ToggleSwitch>
      <button
        class="lk-button lk-button--secondary"
        data-testid="journal-clear"
        @click="clearFilters"
      >
        Сбросить
      </button>
      <button
        class="lk-button lk-button--primary"
        data-testid="journal-export"
        :disabled="isExporting"
        @click="exportLogs"
      >
        {{ isExporting ? 'Экспорт...' : 'Экспорт' }}
      </button>
    </div>

    <div class="content-container">
      <div class="table-container rt-table">
        <div class="table-header rt-head-row">
          <div
            v-for="col in sortableColumns"
            :key="col.field"
            class="header-col"
            :class="col.cls"
            @click="sortBy(col.field)"
          >
            <p :class="{ 'active-sort': state.sort === col.field }">
              {{ col.label }}
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{
                'sorted': state.sort === col.field,
                'desc': state.sort === col.field && state.order === 'desc'
              }"
            />
          </div>
        </div>

        <!-- Порядок строк на узком экране: шапка колонок там скрыта
             карточным режимом, и кликать по заголовкам негде. Меню в body -
             контейнер таблицы держит overflow: hidden и обрезал бы список. -->
        <div class="sort-bar">
          <BaseDropdown
            :model-value="state.sort"
            class="sort-dd"
            :options="sortOptions"
            value-key="value"
            label-key="label"
            teleport
            @update:model-value="setSort"
          />
          <button
            class="lk-button lk-button--secondary sort-order-btn"
            :title="orderTitle"
            :aria-label="orderTitle"
            data-testid="journal-sort-order"
            @click="toggleOrder"
          >
            <AppIcon
              name="sort"
              class="sort-icon sorted"
              :class="{ 'desc': state.order === 'desc' }"
            />
          </button>
        </div>

        <div class="table-body">
          <LoaderSpinner
            v-if="isFirstLoad"
            class="table-loading"
            size="large"
            label="Загрузка журнала…"
          />
          <div
            v-for="log in logs"
            v-else
            :key="log.id"
            class="table-row rt-row"
            :class="{
              'selected': selectedLog && selectedLog.id === log.id,
              'error-row': log.response_status && log.response_status >= 400
            }"
            @click="selectLog(log)"
          >
            <div
              class="table-col time-col"
              data-label="Время"
            >
              <span
                class="cell-content"
                :title="formatFullDate(log.created_at)"
              >
                {{ formatTime(log.created_at) }}
              </span>
            </div>
            <div
              class="table-col method-col"
              data-label="Метод"
            >
              <RequestLogBadge
                kind="method"
                :value="log.method"
              />
            </div>
            <div
              class="table-col path-col"
              data-label="URL"
            >
              <span
                class="truncate-text"
                :title="log.url"
              >
                {{ truncatePath(log.url) }}
              </span>
            </div>
            <div
              class="table-col status-col"
              data-label="Статус"
            >
              <RequestLogBadge
                kind="status"
                :value="log.response_status"
              />
            </div>
            <div
              class="table-col user-col"
              data-label="Пользователь"
            >
              <span class="cell-content">
                {{ log.username || 'Аноним' }}
                <span
                  v-if="log.user_id"
                  class="user-id"
                >(ID: {{ log.user_id }})</span>
              </span>
            </div>
            <div
              class="table-col duration-col"
              data-label="Отклик"
            >
              <span class="cell-content">
                {{ formatDuration(log) }}
              </span>
            </div>
          </div>
          <p
            v-if="!isFirstLoad && !logs.length"
            class="empty-hint"
          >
            {{ journalError || 'Записей по такому отбору нет' }}
          </p>
        </div>

        <div class="table-footer">
          <div class="pagination-controls">
            <button
              :disabled="pagination.page <= 1"
              class="pagination-btn"
              @click="goToPage(-1)"
            >
              &larr;
            </button>
            <span class="page-info">
              Страница {{ pagination.page }} из {{ totalPages }}
            </span>
            <button
              :disabled="pagination.page >= totalPages"
              class="pagination-btn"
              @click="goToPage(1)"
            >
              &rarr;
            </button>
            <!-- Меню в body: подвал таблицы лежит в контейнере с
                 overflow: hidden, и список размеров обрезался по нижней
                 кромке - последний пункт выбрать было нельзя. -->
            <BaseDropdown
              :model-value="perPage"
              class="page-size-dd"
              :options="pageSizeOptions"
              value-key="value"
              label-key="label"
              teleport
              @update:model-value="changePageSize"
            />
          </div>
          <span class="items-count">
            Показано {{ logs.length }} из {{ pagination.total || 0 }} записей
          </span>
        </div>

        <!-- Обновление накрывает плёнкой только таблицу: раздел с показателями,
             графиком и отбором при этом остаётся живым и читаемым. -->
        <RefreshOverlay v-if="isRefreshing" />
      </div>

      <LogDetails
        :log="selectedLog"
        @close="selectedLog = null"
      />
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import SearchComponent from '@/components/SearchComponent.vue';
import RealTimeChart from '@/components/RealTimeChart.vue';
import AppIcon from '@/components/icons/AppIcon.vue';
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import RefreshOverlay from '@/components/ui/RefreshOverlay.vue';
import DateFilter from '@/components/DateFilter.vue';
import LogDetails from './LogDetails.vue';
import RequestLogBadge from './RequestLogBadge.vue';
import { apiRequest, apiRequestRaw } from '@/api/client';
import { downloadRequestLogs } from '@/api/requestLogs';
import { useDeletionsStore } from '@/stores/deletions';
import { formatLogin } from '@/utils/formatName';
import {
  SORTABLE_COLUMNS, JOURNAL_PRESETS, PAGE_SIZE_OPTIONS,
  journalStateFromQuery, mergeJournalQuery, filterParamsFromState,
  isJournalPresetOn, toggleJournalPreset, dateToYmd, ymdToDate, journalFilterDropdowns
} from '@/utils/requestLogsQuery';
import { CHART_PERIODS, DEFAULT_CHART_PERIOD, JOURNAL_REFRESH_MS, journalRefreshBlock } from '@/utils/requestLogsLive';
import {
  describeLoadError, exportNotice, formatDuration, formatFullDate, formatTime, truncatePath
} from '@/utils/requestLogsFormat';

/**
 * Вкладка «Журнал»: живая лента обращений с отбором, сортировкой и выгрузкой.
 * Отбор целиком лежит в адресной строке, поэтому присланная ссылка открывает
 * тот же экран, а обновление страницы его не теряет.
 */
const props = defineProps({
  // Вкладка показана. В фоне лента стоит: обновлять невидимый список незачем.
  active: { type: Boolean, default: true },
  // Вкладка браузера ушла в фон - опросы не идут вовсе.
  hidden: { type: Boolean, default: false },
});

const emit = defineEmits(['update:loading', 'refresh-stats']);

const route = useRoute();
const router = useRouter();
const deletions = useDeletionsStore();

const sortableColumns = SORTABLE_COLUMNS;
const journalPresets = JOURNAL_PRESETS;
const pageSizeOptions = PAGE_SIZE_OPTIONS;
const chartPeriods = CHART_PERIODS;
const chartPeriodOptions = CHART_PERIODS.map(p => ({ value: p.key, label: p.label }));
const sortOptions = SORTABLE_COLUMNS.map(c => ({ value: c.field, label: c.label }));

const state = reactive(journalStateFromQuery(route?.query || {}));
const logs = ref([]);
const users = ref([]);
const selectedLog = ref(null);
const perPage = ref(String(state.perPage));
const pagination = reactive({ page: state.page, per_page: state.perPage, total: 0 });
const isLoading = ref(false);
// Список уже показывали хотя бы раз: до этого места плёнка обновления не нужна -
// накрывать под ней нечего, там лоадер вместо строк.
const hasLoaded = ref(false);
// Список перечитывает сам себя по таймеру ленты. Такой заход не показывается
// вовсе: плёнка раз в десять секунд - мельтешение, а поднятый признак загрузки
// глотал бы клик по «Обновить» (кнопка игнорирует его, пока грузится).
const isSilentRefresh = ref(false);
const isExporting = ref(false);
const journalError = ref('');
const autoRefresh = ref(true);
const timelineData = ref([]);
const chartPeriod = ref(DEFAULT_CHART_PERIOD);
// Причины отказов разделов, о которых уже сообщили: опрос идёт по таймеру, и
// тост на каждый отказ выстраивал бы очередь одинаковых сообщений.
const sectionErrors = new Set();
// Номер последнего запроса списка. Список дёргают фильтр, сортировка, страница
// и обновление подряд, а отвечают они не в том порядке, в каком ушли: без
// номера медленный ответ прошлого фильтра затирает свежий.
let logsSeq = 0;
let logsTimer = null;
let timelineTimer = null;

const filterDropdowns = computed(() => journalFilterDropdowns(journalState(), users.value, formatLogin));
// Календарь работает с датами, а отбор хранит дни строками - как в адресе.
const journalRange = computed(() => ({ start: ymdToDate(state.from), end: ymdToDate(state.to) }));
const totalPages = computed(() => Math.max(1, Math.ceil((pagination.total || 0) / (pagination.per_page || 20))));
const selectedPeriod = computed(() => chartPeriods.find(p => p.key === chartPeriod.value) || chartPeriods[4]);
const orderTitle = computed(() => (state.order === 'desc' ? 'По убыванию' : 'По возрастанию'));
const isFirstLoad = computed(() => isLoading.value && !hasLoaded.value);
const isRefreshing = computed(() => isLoading.value && hasLoaded.value && !isSilentRefresh.value);
const isVisibleLoading = computed(() => isLoading.value && !isSilentRefresh.value);

/** Причина, по которой живая лента сейчас стоит. Пустая - лента обновляется. */
const refreshBlock = computed(() => journalRefreshBlock({
  tab: props.active ? 'journal' : 'analytics',
  hidden: props.hidden,
  hasSelection: Boolean(selectedLog.value),
  page: pagination.page,
}));

watch(isVisibleLoading, (value) => emit('update:loading', value));

/** Текущий отбор в том виде, в каком его читают утилиты адреса. */
function journalState() {
  return { ...state, page: pagination.page, perPage: pagination.per_page };
}

/**
 * Раскладывает отбор по полям формы. Страницу и размер выставляет вызывающий:
 * они меняются не всегда вместе с фильтрами.
 */
function applyJournalState(next) {
  Object.assign(state, {
    search: next.search, method: next.method, status: next.status, user: next.user,
    from: next.from, to: next.to, since: next.since, minDuration: next.minDuration,
    sort: next.sort, order: next.order,
  });
}

function syncQueryFromState() {
  if (!router) return;
  const next = mergeJournalQuery(route?.query || {}, journalState());
  if (next) router.replace({ query: next }).catch(() => {});
}

/**
 * Читает страницу журнала. `silent` ставит только самообновление ленты: строки
 * подменяются молча, без плёнки поверх таблицы.
 * @param {{ silent?: boolean }} [options]
 */
async function fetchLogs({ silent = false } = {}) {
  const seq = ++logsSeq;
  syncQueryFromState();
  isSilentRefresh.value = silent;
  isLoading.value = true;
  try {
    const params = new URLSearchParams({
      page: pagination.page,
      per_page: pagination.per_page,
      sort: state.sort,
      order: state.order,
      ...filterParamsFromState(journalState()),
    });

    const response = await apiRequestRaw(`/request-logs?${params}`);
    if (seq !== logsSeq) return;

    if (!response.ok) {
      journalError.value = describeLoadError(response, 'загрузить журнал');
      return;
    }
    const body = await response.json();
    if (seq !== logsSeq) return;
    if (body && body.success) {
      journalError.value = '';
      logs.value = body.data || [];
      if (body.meta) {
        pagination.total = body.meta.total || 0;
        pagination.page = body.meta.page || 1;
        pagination.per_page = body.meta.per_page || 20;
      }
    }
  } catch (error) {
    if (seq !== logsSeq) return;
    journalError.value = describeLoadError(error, 'загрузить журнал');
    deletions.notify({ prefix: 'Не удалось загрузить ', bold: 'логи', type: 'error' });
  } finally {
    if (seq === logsSeq) {
      isLoading.value = false;
      isSilentRefresh.value = false;
      hasLoaded.value = true;
    }
  }
}

/**
 * Читает раздел и отдаёт ответ обработчику. Сбой одного раздела не гасит
 * остальные: список, график и показатели живут независимо.
 */
async function loadSection(path, apply, label) {
  try {
    const response = await apiRequest(path);
    if (!response.ok) return reportSectionError(response, label);
    const data = await response.json();
    if (data) apply(data);
  } catch (error) {
    reportSectionError(error, label);
  }
}

function reportSectionError(source, label) {
  const key = `${label}:${(source && source.status) || 'net'}`;
  if (sectionErrors.has(key)) return;
  sectionErrors.add(key);
  deletions.notify({ bold: describeLoadError(source, `загрузить ${label}`), type: 'error' });
}

function fetchTimeline() {
  const p = selectedPeriod.value;
  const params = new URLSearchParams({ interval: String(p.interval), limit: String(p.limit) });
  return loadSection(`/request-logs/timeline?${params}`, data => {
    if (Array.isArray(data)) timelineData.value = data;
  }, 'график');
}

function fetchUsers() {
  return loadSection('/request-logs/users', data => {
    if (Array.isArray(data)) users.value = data;
  }, 'список пользователей');
}

function onChartPeriodChange(period) {
  chartPeriod.value = period;
  fetchTimeline();
}

/** Смена значения в списке отбора: поле состояния приходит ключом. */
function applyFilter(key, value) {
  state[key] = value;
  refreshLogs();
}

function refreshLogs() {
  reload();
  emit('refresh-stats');
}

/**
 * Перечитать список с первой страницы. Показатели шапки отсюда не дёргаются:
 * когда обновление пришло от кнопки в шапке, она читает их сама, и запрос
 * ушёл бы дважды.
 */
function reload() {
  pagination.page = 1;
  return fetchLogs();
}

defineExpose({ refresh: reload });

/**
 * Порядок строк задаёт сервер: в списке одна страница, и перестановка её на
 * месте показывала бы «самые медленные» только среди двадцати видимых.
 */
function sortBy(field) {
  if (state.sort === field) {
    state.order = state.order === 'asc' ? 'desc' : 'asc';
  } else {
    state.sort = field;
    state.order = 'desc';
  }
  pagination.page = 1;
  fetchLogs();
}

/** Выбор поля в списке на мобилке. Повтор того же поля порядок не трогает. */
function setSort(field) {
  if (state.sort === field) return;
  sortBy(field);
}

/** Направление отдельной кнопкой: sortBy на том же поле его и переворачивает. */
function toggleOrder() {
  sortBy(state.sort);
}

function selectLog(log) {
  selectedLog.value = log;
}

/**
 * Перелистывание. Шаг за границы списка игнорируется: кнопки блокируются
 * разметкой, но клавиатура и повторный клик до ответа сервера мимо неё.
 */
function goToPage(step) {
  const page = pagination.page + step;
  if (page < 1 || page > totalPages.value) return;
  pagination.page = page;
  fetchLogs();
}

function changePageSize(size) {
  perPage.value = size;
  pagination.per_page = parseInt(size, 10);
  pagination.page = 1;
  fetchLogs();
}

// Порядок строк остаётся: кнопка стоит среди фильтров, а сортировку задаёт
// заголовок таблицы - сбрасывать её заодно человек не просил.
function clearFilters() {
  const { sort, order } = journalState();
  applyJournalState({ ...journalStateFromQuery({}), sort, order });
  refreshLogs();
}

/** Быстрый отбор: включает или снимает набор обычных фильтров. */
function togglePreset(key) {
  applyJournalState(toggleJournalPreset(journalState(), key));
  refreshLogs();
}

function presetOn(key) {
  return isJournalPresetOn(journalState(), key);
}

/**
 * Ввод даты руками снимает отбор «последний час»: иначе поле показывает день,
 * а список отобран по моменту, и человек видит не то, что выбрал.
 */
function onDateFilterChange() {
  state.since = '';
  refreshLogs();
}

async function exportLogs() {
  isExporting.value = true;
  try {
    // Порядок тот же, что на экране: выгрузка «самых медленных» должна
    // начинаться с самых медленных, а не с последних по времени.
    const res = await downloadRequestLogs({
      sort: state.sort,
      order: state.order,
      ...filterParamsFromState(journalState()),
    });
    deletions.notify(exportNotice(res));
  } catch (error) {
    journalError.value = describeLoadError(error, 'выгрузить журнал');
    deletions.notify({ prefix: 'Не удалось выполнить ', bold: 'экспорт', type: 'error' });
  } finally {
    isExporting.value = false;
  }
}

/**
 * Очередное самообновление ленты. Причины остановки собраны в refreshBlock,
 * а незавершённый запрос второй раз не дёргается.
 */
function tickLogs() {
  if (!autoRefresh.value || refreshBlock.value || isLoading.value) return;
  fetchLogs({ silent: true });
}

onMounted(async () => {
  await Promise.all([fetchLogs(), fetchTimeline(), fetchUsers()]);
  // Опросы в фоновой вкладке не идут вовсе: раздел сам же и вычищали от шума
  // самозапросов, а показатели за время отсутствия догоняются при возврате.
  // Показатели шапки опрашивает оболочка своим таймером - здесь только график,
  // иначе один и тот же адрес читался бы дважды за окно.
  timelineTimer = setInterval(() => {
    if (props.hidden) return;
    fetchTimeline();
  }, 30000);
  logsTimer = setInterval(tickLogs, JOURNAL_REFRESH_MS);
});

onBeforeUnmount(() => {
  [logsTimer, timelineTimer].forEach(clearInterval);
  logsTimer = null;
  timelineTimer = null;
});

// Вернувшись из фона, лента догоняет пропущенное сразу, а не через очередной
// интервал: иначе первое, что видит человек, - устаревший список.
watch(() => props.hidden, (hidden) => {
  if (!hidden) tickLogs();
});
</script>

<style scoped>
.chart-section {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
}

.chart-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.chart-title {
  margin: 0;
  font-size: 0.95em;
  font-weight: 600;
  color: var(--text);
}

.chart-interval {
  font-size: 0.8em;
  color: var(--text-muted);
}

.chart-header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.chart-period-dd {
  width: 150px;
}

.filters-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
  flex-wrap: wrap;
}

.filter-controls {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.filter-dd {
  width: 150px;
}

/* Список пользователей шире прочих: «Все пользователи» в 150px не помещается
   и обрывается многоточием прямо в состоянии по умолчанию. */
.filter-dd--wide {
  width: 185px;
}

/* Высота списков подтягивается к соседям по ряду - поиск и календарь ростом
   35px, штатные 30px у выпадающего списка сбивали бы линию. */
.filter-dd :deep(.base-dropdown__button),
.sort-dd :deep(.base-dropdown__button),
.chart-period-dd :deep(.base-dropdown__button),
.page-size-dd :deep(.base-dropdown__button) {
  min-height: 35px;
}

/* Быстрый отбор держится одной группой: при переносе строки чипы уезжали
   поодиночке и «Только ошибки» оставалась в ряду со списками. */
.filter-presets {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

/* Деталь запроса открывается окном, поэтому таблица держит всю ширину раздела,
   а не 65% под соседнюю колонку. Высоту на узких экранах снимает не этот блок, а
   AdminPageShell: у него :deep(.content-container) с height:auto. */
.content-container {
  display: flex;
  height: 500px;
  width: 100%;
}

.table-container {
  background: var(--surface);
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-width: 0;
  height: 100%;
}

.table-header {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  height: 43px;
  align-items: center;
}

.header-col {
  padding: 0 8px;
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 600;
  text-align: left;
  display: flex;
  align-items: center;
  gap: 5px;
  transition: .2s;
  cursor: pointer;
  user-select: none;
}

.header-col:hover {
  color: var(--text);
}

.header-col:hover .sort-icon {
  color: var(--text);
}

.sort-icon {
  color: var(--text-muted);
  width: 12px;
  height: 12px;
  transition: .2s;
}

.sort-icon.sorted {
  color: var(--text);
}

.sort-icon.desc {
  transform: rotate(180deg);
}

.active-sort {
  color: var(--text) !important;
  font-weight: 600 !important;
}

.time-col { width: 12%; min-width: 80px; }
.method-col { width: 8%; min-width: 70px; }
.path-col { width: 30%; min-width: 150px; }
.status-col { width: 8%; min-width: 70px; }
.user-col { width: 20%; min-width: 120px; }
.duration-col { width: 10%; min-width: 80px; }

/* Порядок строк на мобилке. На десктопе блока нет: там сортируют кликом по
   заголовку колонки, а два способа задать одно и то же расходятся на вид. */
.sort-bar {
  display: none;
  align-items: center;
  gap: 8px;
  padding: 10px 16px 0;
}

.sort-dd {
  flex: 1;
  min-width: 0;
}

.sort-order-btn {
  position: relative;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 35px;
  height: 35px;
  padding: 0;
}

/* Зона нажатия 44px без раздувания самой кнопки. */
.sort-order-btn::before {
  content: '';
  position: absolute;
  inset: -5px;
}

.table-body {
  flex: 1;
  overflow-y: auto;
  max-height: 500px;
}

.table-loading {
  padding: 32px 16px;
}

.table-row {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  align-items: center;
  transition: background-color 0.2s ease;
  cursor: pointer;
  height: 40px;
  font-size: 12px;
}

.table-row:hover {
  background-color: var(--surface-2);
}

.table-row.selected {
  background-color: var(--accent-tint);
}

/* Ошибка помечается полосой у левого края, а не заливкой строки: успешных
   записей в журнале больше девяноста процентов, и сплошная зелёная подложка
   под ними не выделяла ничего - выделять надо редкое. Полоса переживает и
   карточный режим, где фон строки перебит правилом responsive-tables. */
.table-row.error-row {
  box-shadow: inset 3px 0 0 var(--danger);
}

.table-col {
  padding: 0 8px;
}

.cell-content {
  display: block;
  padding: 4px 0;
}

.user-id {
  font-size: 10px;
  color: var(--text-muted);
  margin-left: 4px;
}

.truncate-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
  display: block;
}

.table-footer {
  padding: 12px 20px;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: var(--accent-tint);
}

.pagination-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.pagination-btn {
  padding: 4px 8px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: 14px;
}

.pagination-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.page-info {
  font-size: 12px;
  color: var(--text-muted);
}

.page-size-dd {
  width: 175px;
}

.items-count {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}

.empty-hint {
  color: var(--text-muted);
  font-size: 13px;
  text-align: center;
  padding: 16px;
}

@media (max-width: 768px) {
  .filters-bar {
    flex-direction: column;
    align-items: stretch;
    padding: 12px 16px;
  }

  .filter-controls {
    flex-wrap: wrap;
  }

  .filter-dd {
    flex: 1;
    width: auto;
    min-width: 130px;
  }

  /* Календарь на узком экране тянется по ряду, а не держит свои 215px. */
  .filters-bar :deep(.date-filter),
  .filters-bar :deep(.date-field) {
    width: 100%;
  }

  .table-header,
  .table-row {
    padding: 0 16px;
  }

  .time-col { width: 15%; }
  .method-col { width: 10%; }
  .path-col { width: 35%; }
  .status-col { width: 10%; }
  .user-col { width: 20%; }
  .duration-col { width: 10%; }

  .table-footer {
    flex-direction: column;
    gap: 12px;
    align-items: stretch;
  }

  .pagination-controls {
    justify-content: center;
  }

  .chart-section {
    padding: 12px 16px;
  }
}

/* Карточки (responsive-tables.css). Порог тот же 767.98, что у самой
   инфраструктуры: на ровно 768 правила выше уже применились бы, а карточки
   ещё нет - строка осталась бы таблицей без шапки. */
@media (max-width: 767.98px) {
  .sort-bar {
    display: flex;
  }

  /* Своих полей у тела таблицы нет - на десктопе их держала строка. Без них
     карточки прилипают к кромкам раздела. */
  .table-body {
    padding: 12px 16px;
  }

  /* В карточке под адрес есть вся ширина строки, и обрезать его многоточием
     незачем: от длинного пути осталось бы начало без хвоста запроса. */
  .path-col .truncate-text {
    white-space: normal;
    overflow: visible;
    overflow-wrap: anywhere;
  }
}
</style>
