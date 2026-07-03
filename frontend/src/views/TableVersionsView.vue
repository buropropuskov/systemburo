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
        <h3 class="versions-card__title">
          Сохранённые версии
        </h3>
        <div class="versions-card__spacer" />
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

      <div
        v-else
        class="versions-content"
      >
        <!-- Список снимков (master) -->
        <div
          class="versions-list-section"
          :class="{ 'with-details': selectedId !== null }"
        >
          <div
            v-if="listLoading && !items.length"
            class="versions-state"
            data-testid="tv-loading"
          >
            <span class="versions-spinner" />
          </div>

          <div
            v-else-if="items.length"
            class="versions-cards"
            data-testid="tv-list"
          >
            <button
              v-for="s in items"
              :key="s.id"
              type="button"
              class="versions-card-item"
              :class="{ 'versions-card-item--active': s.id === selectedId }"
              data-testid="tv-card"
              @click="selectSnapshot(s.id)"
            >
              <div class="versions-card-item__head">
                <Badge
                  :variant="reasonVariant(s.reason)"
                  size="sm"
                >
                  {{ reasonLabel(s.reason) }}
                </Badge>
                <span class="versions-card-item__date">{{ formatDateTime(s.taken_at) }}</span>
              </div>
              <div class="versions-card-item__counts">
                <span class="versions-count versions-count--on">На территории: {{ s.counts.on_territory }}</span>
                <span class="versions-count versions-count--exit">Выехал: {{ s.counts.exited }}</span>
                <span class="versions-count versions-count--total">Всего: {{ s.counts.total }}</span>
              </div>
              <div
                v-if="s.reason === 'manual' && s.actor_name"
                class="versions-card-item__actor"
              >
                {{ s.actor_name }}
              </div>
            </button>

            <button
              v-if="items.length < total"
              type="button"
              class="versions-load-more"
              :disabled="listLoading"
              data-testid="tv-load-more"
              @click="loadMore"
            >
              Показать ещё
            </button>
          </div>

          <div
            v-else
            class="versions-state versions-empty"
            data-testid="tv-empty"
          >
            <p>Версий пока нет</p>
            <span class="versions-empty__hint">
              Снимки состояния создаются автоматически ночью перед сбросом статусов
              (в 06:00) и вручную.
            </span>
          </div>

          <div
            class="versions-footer"
            data-testid="tv-footer"
          >
            Всего: {{ total }}
          </div>
        </div>

        <!-- Состав снимка (detail) -->
        <div
          v-if="selectedId !== null"
          class="versions-detail-section"
          data-testid="tv-detail"
        >
          <template v-if="detail">
            <div class="versions-detail__head">
              <h4 class="versions-detail__title">
                Состав на {{ formatDateTime(detail.taken_at) }}
              </h4>
              <div class="versions-detail__counts">
                <span class="versions-count versions-count--on">На территории: {{ detailCounts.on_territory }}</span>
                <span class="versions-count versions-count--exit">Выехал: {{ detailCounts.exited }}</span>
                <span class="versions-count versions-count--not">Не въезжал: {{ detailCounts.not_entered }}</span>
                <span class="versions-count versions-count--total">Всего: {{ detailCounts.total }}</span>
              </div>
            </div>

            <div
              v-if="detailRows.length"
              class="versions-table-wrap"
            >
              <table
                class="versions-table"
                data-testid="tv-composition"
              >
                <thead>
                  <tr>
                    <th
                      v-for="col in detailColumns"
                      :key="col.key"
                      class="versions-table__th"
                    >
                      {{ col.label }}
                    </th>
                    <th class="versions-table__th">
                      Статус
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="(row, idx) in detailRows"
                    :key="idx"
                    class="versions-table__row"
                    data-testid="tv-row"
                  >
                    <td
                      v-for="col in detailColumns"
                      :key="col.key"
                      class="versions-table__td"
                    >
                      {{ cellValue(row, col) }}
                    </td>
                    <td class="versions-table__td">
                      <span
                        class="versions-status"
                        :class="statusClass(row.territory_status)"
                      >
                        {{ statusLabel(row.territory_status) }}
                      </span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div
              v-else
              class="versions-state"
              data-testid="tv-detail-empty"
            >
              На момент снимка таблица была пуста
            </div>
          </template>

          <div
            v-else
            class="versions-state"
            data-testid="tv-detail-loading"
          >
            <span class="versions-spinner" />
          </div>
        </div>
      </div>
    </article>
  </section>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import RefreshButton from '@/components/RefreshButton.vue';
import Badge from '@/components/ui/Badge.vue';
import { apiRequest } from '@/api/client';
import { listTableSnapshots, getTableSnapshot } from '@/api/system-tables';
import { useDeletionsStore } from '@/stores/deletions';
import { formatDateTime, formatDateRu } from '@/utils/datetime';

const PER_PAGE = 20;

const route = useRoute();
const deletions = useDeletionsStore();

const tableName = computed(() => route.params.tableName);

const tableID = ref(0);
const tableType = ref('');
const displayName = ref('');
const error = ref('');

const items = ref([]);
const total = ref(0);
const page = ref(1);
const listLoading = ref(false);

const selectedId = ref(null);
const detail = ref(null);

// Токены последовательности от гонки устаревшего ответа (#632): быстрое
// переключение снимков/повторный refresh пускают параллельные запросы в общий
// ref, применяем только ответ последнего.
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

// territory_status: 1=на территории, 2=выехал, 0/null=не въезжал (модель SnapshotCounts).
function statusLabel(status) {
  if (status === 1) return 'На территории';
  if (status === 2) return 'Выехал';
  return 'Не въезжал';
}

function statusClass(status) {
  if (status === 1) return 'versions-status--on';
  if (status === 2) return 'versions-status--exit';
  return 'versions-status--not';
}

const CARS_COLUMNS = [
  { key: 'car_number', label: 'Номер Т/С' },
  { key: 'car_brand', label: 'Марка' },
  { key: 'organization', label: 'Организация' },
  { key: 'entry_date_to', label: 'Действует до', format: 'date' },
];
const PEOPLE_COLUMNS = [
  { key: 'last_name', label: 'Фамилия' },
  { key: 'first_name', label: 'Имя' },
  { key: 'middle_name', label: 'Отчество' },
  { key: 'organization', label: 'Организация' },
  { key: 'position', label: 'Должность' },
];

// Тип берём из payload снимка (что было на момент), а не из текущей таблицы -
// снимок самодостаточен.
const detailType = computed(() => detail.value?.payload?.table_type || tableType.value);
const detailColumns = computed(() => (detailType.value === 'people' ? PEOPLE_COLUMNS : CARS_COLUMNS));
const detailRows = computed(() => {
  const rows = detail.value?.payload?.rows;
  return Array.isArray(rows) ? rows : [];
});
const detailCounts = computed(() => detail.value?.counts || { on_territory: 0, exited: 0, not_entered: 0, total: 0 });

function cellValue(row, col) {
  const raw = row[col.key];
  if (raw === null || raw === undefined || raw === '') return '—';
  if (col.format === 'date') return formatDateRu(raw) || '—';
  return raw;
}

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
  const seq = ++listSeq;
  try {
    const { items: data, total: t } = await listTableSnapshots(tableID.value, {
      page: page.value,
      perPage: PER_PAGE,
    });
    if (seq !== listSeq) return;
    items.value = reset ? data : [...items.value, ...data];
    total.value = t;
    // Автовыбор первого снимка на свежем списке - сразу показываем состав, как
    // master-detail эталон (не пустая правая панель).
    if (reset && items.value.length && selectedId.value === null) {
      selectSnapshot(items.value[0].id);
    }
  } catch {
    if (seq !== listSeq) return;
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
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.05);
  border-radius: 30px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  max-height: 640px;
}

.versions-card__header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 20px;
  height: 50px;
  border-bottom: 1px solid #e6e6e6;
  flex-shrink: 0;
}

.versions-card__title {
  margin: 0;
  font-weight: 600;
  font-size: 16px;
  color: #000;
}

.versions-card__spacer {
  flex: 1;
}

.versions-content {
  display: flex;
  min-height: 0;
  flex: 1;
  overflow: hidden;
}

.versions-list-section {
  width: 100%;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.versions-list-section.with-details {
  width: 42%;
  border-right: 1px solid #e6e6e6;
}

.versions-cards {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 15px;
}

.versions-card-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: 100%;
  text-align: left;
  padding: 12px 14px;
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  cursor: pointer;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background 0.2s ease;
}

.versions-card-item:hover {
  border-color: #4f5bdf;
  box-shadow: 0 3px 10px rgba(79, 91, 223, 0.15);
}

.versions-card-item--active {
  border-color: #4f5bdf;
  background: #f5f6ff;
  box-shadow: 0 3px 10px rgba(79, 91, 223, 0.18);
}

.versions-card-item__head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.versions-card-item__date {
  font-weight: 600;
  font-size: 14px;
  color: #1a1a1a;
}

.versions-card-item__counts {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 12px;
}

.versions-card-item__actor {
  font-size: 12.5px;
  color: #8a8a8a;
}

.versions-count {
  font-size: 12.5px;
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

.versions-load-more {
  align-self: center;
  margin-top: 4px;
  height: 30px;
  padding: 0 18px;
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

.versions-footer {
  flex-shrink: 0;
  padding: 12px 20px;
  border-top: 1px solid #e6e6e6;
  font-size: 14px;
  color: #666;
}

.versions-detail-section {
  width: 58%;
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow-y: auto;
  padding: 15px;
}

.versions-detail__head {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-bottom: 12px;
  border-bottom: 1px solid #eee;
}

.versions-detail__title {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  color: #4f5bdf;
}

.versions-detail__counts {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 14px;
}

.versions-table-wrap {
  overflow-x: auto;
}

.versions-table {
  width: 100%;
  border-collapse: collapse;
}

.versions-table__th {
  padding: 10px 12px;
  text-align: left;
  font-weight: 500;
  font-size: 13px;
  color: #a2a2a2;
  white-space: nowrap;
  border-bottom: 1px solid #f0f0f0;
}

.versions-table__row:hover {
  background: #f8f9ff;
}

.versions-table__td {
  padding: 10px 12px;
  font-size: 13.5px;
  color: #000;
  border-top: 1px solid #f0f0f0;
  white-space: nowrap;
}

.versions-status {
  display: inline-flex;
  align-items: center;
  padding: 3px 10px;
  border-radius: 50px;
  font-size: 11px;
  font-weight: 500;
  white-space: nowrap;
  border: 1px solid;
}

.versions-status--on {
  background: #e6f6ec;
  color: #1f9d55;
  border-color: #bfe6cd;
}

.versions-status--exit {
  background: #f1f3f5;
  color: #475569;
  border-color: #e2e8f0;
}

.versions-status--not {
  background: #fff;
  color: #a2a2a2;
  border-color: #e6e6e6;
}

.versions-state {
  flex: 1;
  min-height: 160px;
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
  max-width: 340px;
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

@media (max-width: 900px) {
  .versions-content {
    flex-direction: column;
  }

  .versions-list-section,
  .versions-list-section.with-details,
  .versions-detail-section {
    width: 100%;
  }

  .versions-list-section.with-details {
    border-right: none;
    border-bottom: 1px solid #e6e6e6;
  }
}
</style>
