<template>
  <AdminPageShell>
    <div class="accessible-attachments dashboard-card">
      <div class="management-header">
        <h3 class="management-title">
          Доступные мне
        </h3>
        <div class="header-controls">
          <RefreshButton
            :loading="listLoading"
            @refresh="refresh"
          />
        </div>
      </div>

      <div
        class="filters"
        data-testid="aa-filters"
      >
        <div class="filters__search">
          <img
            class="filters__search-icon"
            src="@/assets/icons/search.png"
            alt=""
          >
          <input
            v-model="search"
            type="text"
            class="lk-input filters__search-input"
            placeholder="Поиск: заявка, машина, ФИО, место разгрузки, согласующий..."
            data-testid="aa-search"
            @input="onSearchInput"
          >
        </div>
        <BaseDropdown
          :model-value="typeFilter"
          :options="typeOptions"
          class="filters__dropdown"
          data-testid="aa-filter-type"
          @update:model-value="setType"
        />
        <BaseDropdown
          :model-value="orgFilter"
          :options="orgOptions"
          searchable
          class="filters__dropdown"
          data-testid="aa-filter-org"
          @update:model-value="setOrg"
        />
        <BaseDropdown
          :model-value="companyFilter"
          :options="companyOptions"
          searchable
          class="filters__dropdown"
          data-testid="aa-filter-company"
          @update:model-value="setCompany"
        />
        <button
          type="button"
          class="lk-button filters__toggle"
          :class="completedFilter ? 'lk-button--primary' : 'lk-button--ghost'"
          :aria-pressed="completedFilter"
          data-testid="aa-filter-completed"
          @click="toggleCompleted"
        >
          Завершённые
        </button>
        <button
          type="button"
          class="lk-button lk-button--danger filters__reset"
          :disabled="!hasActiveFilters"
          data-testid="aa-filter-reset"
          @click="resetFilters"
        >
          Сбросить
        </button>
      </div>

      <div class="content-container">
        <!-- Список вложений -->
        <div
          class="list-section"
          :class="{ 'with-details': selectedId !== null }"
        >
          <div
            v-if="listLoading && !items.length"
            class="cards-list"
            data-testid="aa-skeleton"
          >
            <SkeletonCard
              v-for="i in 5"
              :key="i"
              :lines="3"
            />
          </div>

          <div
            v-else-if="items.length"
            class="cards-list"
            data-testid="aa-list"
          >
            <button
              v-for="a in items"
              :key="a.attachment_id"
              type="button"
              class="attachment-card"
              :class="{ 'attachment-card--active': a.attachment_id === selectedId }"
              data-testid="aa-card"
              @click="selectAttachment(a.attachment_id)"
            >
              <div class="attachment-card__top">
                <Badge
                  :variant="typeVariant(a.attachment_type)"
                  size="sm"
                >
                  {{ typeLabel(a.attachment_type) }}
                </Badge>
                <span class="attachment-card__name">{{ displayName(a) }}</span>
                <StatusBadge
                  v-if="statusText(a)"
                  class="attachment-card__status"
                  :status="statusText(a)"
                />
              </div>
              <div class="attachment-card__meta">
                <span
                  v-if="a.application_number"
                  class="attachment-card__field"
                >
                  <span class="attachment-card__label">Заявка №</span>{{ a.application_number }}
                </span>
                <span
                  v-if="a.organization_name"
                  class="attachment-card__field"
                >
                  <span class="attachment-card__label">Организация:</span>{{ a.organization_name }}
                </span>
                <span
                  v-if="senderName(a)"
                  class="attachment-card__field"
                >
                  <span class="attachment-card__label">Отправитель:</span>{{ senderName(a) }}
                </span>
                <span
                  v-if="dateRange(a)"
                  class="attachment-card__field"
                >
                  <span class="attachment-card__label">Даты:</span>{{ dateRange(a) }}
                </span>
                <span
                  v-if="a.places"
                  class="attachment-card__field attachment-card__field--places"
                >
                  <span class="attachment-card__label">Места:</span>{{ a.places }}
                </span>
              </div>
            </button>

            <button
              v-if="items.length < total"
              type="button"
              class="lk-button lk-button--secondary load-more"
              :disabled="listLoading"
              data-testid="aa-load-more"
              @click="loadMore"
            >
              Показать ещё
            </button>
          </div>

          <div
            v-else
            class="empty-state"
            data-testid="aa-empty"
          >
            <p>{{ hasActiveFilters ? 'Ничего не найдено' : 'Доступных вложений нет' }}</p>
            <span class="empty-state__hint">
              {{ hasActiveFilters
                ? 'Измените условия поиска или сбросьте фильтры.'
                : 'Здесь появятся вложения подтверждённых заявок по вашим местам доступа.' }}
            </span>
            <button
              v-if="hasActiveFilters"
              type="button"
              class="lk-button lk-button--secondary"
              data-testid="aa-empty-reset"
              @click="resetFilters"
            >
              Сбросить фильтры
            </button>
          </div>

          <div class="list-footer">
            Всего: {{ total }}
          </div>
        </div>

        <!-- Деталь вложения -->
        <div
          v-if="selectedId !== null"
          class="detail-section"
          data-testid="aa-detail"
        >
          <template v-if="detail">
            <div class="application-block">
              <h4 class="application-block__title">
                Заявка
              </h4>
              <div class="application-block__grid">
                <div
                  v-if="detail.attachment.application_number"
                  class="application-block__row"
                >
                  <span class="application-block__label">Номер</span>
                  <span class="application-block__value">{{ detail.attachment.application_number }}</span>
                </div>
                <div
                  v-if="detail.attachment.organization_name"
                  class="application-block__row"
                >
                  <span class="application-block__label">Организация</span>
                  <span class="application-block__value">{{ detail.attachment.organization_name }}</span>
                </div>
                <div
                  v-if="detail.attachment.company_name"
                  class="application-block__row"
                >
                  <span class="application-block__label">Компания</span>
                  <span class="application-block__value">{{ detail.attachment.company_name }}</span>
                </div>
                <div
                  v-if="senderName(detail.attachment)"
                  class="application-block__row"
                >
                  <span class="application-block__label">Отправитель</span>
                  <span class="application-block__value">{{ senderName(detail.attachment) }}</span>
                </div>
                <div
                  v-if="statusText(detail.attachment)"
                  class="application-block__row"
                >
                  <span class="application-block__label">Статус</span>
                  <StatusBadge :status="statusText(detail.attachment)" />
                </div>
                <div
                  v-if="detail.attachment.sending_datetime"
                  class="application-block__row"
                >
                  <span class="application-block__label">Отправлена</span>
                  <span class="application-block__value">{{ formatDateTime(detail.attachment.sending_datetime) }}</span>
                </div>
              </div>
            </div>

            <!-- AvailableAttachment не несёт roof_access/free_parking/custom_values -
                 эти опц. блоки детали просто не отрисуются (v-if по undefined). -->
            <ApplicationAttachmentDetail
              :attachment="detail.attachment"
              :cars="detail.cars || []"
              :employees="detail.employees || []"
              :items="detail.items || []"
            />
          </template>

          <div
            v-else
            class="detail-loading"
            data-testid="aa-detail-loading"
          >
            <SkeletonCard :lines="6" />
          </div>
        </div>
      </div>
    </div>
  </AdminPageShell>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import AdminPageShell from '@/views/admin/AdminPageShell.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import Badge from '@/components/ui/Badge.vue';
import StatusBadge from '@/components/ui/StatusBadge.vue';
import SkeletonCard from '@/components/ui/SkeletonCard.vue';
import ApplicationAttachmentDetail from '@/components/ApplicationDetail/ApplicationAttachmentDetail.vue';
import { getAccessibleAttachments, getAccessibleAttachmentDetail } from '@/api/applications';
import { getOrganizations, getCompanies } from '@/api/organizations';
import { useDeletionsStore } from '@/stores/deletions';
import { formatDateRu, formatDateTime } from '@/utils/datetime';

const PER_PAGE = 30;
const SEARCH_DEBOUNCE_MS = 300;
const VALID_TYPES = ['cars', 'people', 'items'];

const deletions = useDeletionsStore();
const route = useRoute();
const router = useRouter();

// Фильтры инициализируем из query синхронно (до onMounted), чтобы перезагрузка и
// диплинк восстановили состояние одним стартовым запросом, без лишнего рефетча.
// Сентинелы "все": '' для типа, 0 для организации/компании - у BaseDropdown нет
// очистки (null всегда показывает placeholder), поэтому "все" - явный пункт.
const search = ref(typeof route.query.search === 'string' ? route.query.search : '');
const typeFilter = ref(VALID_TYPES.includes(route.query.type) ? route.query.type : '');
const orgFilter = ref(Number(route.query.organization) || 0);
const companyFilter = ref(Number(route.query.company) || 0);
// Завершённые заявки по умолчанию скрыты; тоггл показывает только их. При активном
// поиске бэк отдаёт и завершённые, и нет независимо от флага (см. сервис фильтра).
const completedFilter = ref(route.query.completed === '1');

const typeOptions = [
  { id: '', name: 'Все типы' },
  { id: 'cars', name: 'Автомобили' },
  { id: 'people', name: 'Сотрудники' },
  { id: 'items', name: 'ТМЦ' },
];
const orgOptions = ref([{ id: 0, name: 'Все организации' }]);
const companyOptions = ref([{ id: 0, name: 'Все компании' }]);

const hasActiveFilters = computed(
  () => !!search.value.trim()
    || !!typeFilter.value
    || orgFilter.value !== 0
    || companyFilter.value !== 0
    || completedFilter.value,
);

const items = ref([]);
const total = ref(0);
const page = ref(1);
const listLoading = ref(false);

const selectedId = ref(null);
const detail = ref(null);
// Токен последовательности: быстрые клики по карточкам пускают параллельные
// запросы детали в общий ref, и медленный ответ предыдущего вложения мог бы
// затереть актуальный (#632). Применяем только ответ последнего запроса.
let detailSeq = 0;
// Тот же приём для списка: смена фильтра/поиска быстро пускает несколько запросов
// в общий items/total, медленный предыдущий мог бы затереть актуальный (#632).
let listSeq = 0;

const TYPE_LABELS = { cars: 'Автомобили', people: 'Сотрудники', items: 'ТМЦ' };
const TYPE_VARIANTS = { cars: 'primary', people: 'info', items: 'warning' };

function typeLabel(type) {
  return TYPE_LABELS[type] || type || 'Вложение';
}

function typeVariant(type) {
  return TYPE_VARIANTS[type] || 'neutral';
}

function displayName(a) {
  return a.attachment_display_name || a.attachment_name || 'Без названия';
}

function senderName(a) {
  return a.sender_full_name || a.sender_name || '';
}

function statusText(a) {
  return a.status || a.confirmation || '';
}

function dateRange(a) {
  const from = formatDateRu(a.entry_date_from);
  const to = formatDateRu(a.entry_date_to);
  if (from && to) return from === to ? from : `${from} - ${to}`;
  if (from) return `с ${from}`;
  if (to) return `по ${to}`;
  return '';
}

function buildParams() {
  const params = { page: page.value, per_page: PER_PAGE };
  const s = search.value.trim();
  if (s) params.search = s;
  if (typeFilter.value) params.attachment_type = typeFilter.value;
  if (orgFilter.value) params.organization_id = orgFilter.value;
  if (companyFilter.value) params.company_id = companyFilter.value;
  if (completedFilter.value) params.completed = true;
  return params;
}

function syncUrl() {
  const query = {};
  const s = search.value.trim();
  if (s) query.search = s;
  if (typeFilter.value) query.type = typeFilter.value;
  if (orgFilter.value) query.organization = String(orgFilter.value);
  if (companyFilter.value) query.company = String(companyFilter.value);
  if (completedFilter.value) query.completed = '1';
  // catch гасит navigation-cancel при быстрой смене фильтров: vue-router отклоняет
  // отменённую replace - это не ошибка приложения, а нормальная гонка ввода.
  router.replace({ query }).catch(() => {});
}

async function fetchList({ reset = true } = {}) {
  if (reset) page.value = 1;
  listLoading.value = true;
  const seq = ++listSeq;
  try {
    const { items: data, meta } = await getAccessibleAttachments(buildParams());
    if (seq !== listSeq) return;
    items.value = reset ? data : [...items.value, ...data];
    total.value = meta.total || 0;
  } catch {
    if (seq !== listSeq) return;
    deletions.notify({
      type: 'error',
      bold: 'Не удалось загрузить',
      suffix: 'список доступных вложений',
    });
  } finally {
    if (seq === listSeq) listLoading.value = false;
  }
}

// Аккумулирующую страницу load-more в URL не кладём: с "Показать ещё" её число
// не диплинкается осмысленно. Фильтры и поиск - кладём (syncUrl).
function applyFilters() {
  selectedId.value = null;
  detail.value = null;
  syncUrl();
  fetchList({ reset: true });
}

let searchTimer = null;
function onSearchInput() {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(applyFilters, SEARCH_DEBOUNCE_MS);
}

function setType(value) {
  typeFilter.value = value;
  applyFilters();
}

function setOrg(value) {
  orgFilter.value = value;
  applyFilters();
}

function setCompany(value) {
  companyFilter.value = value;
  applyFilters();
}

function toggleCompleted() {
  completedFilter.value = !completedFilter.value;
  applyFilters();
}

function resetFilters() {
  clearTimeout(searchTimer);
  search.value = '';
  typeFilter.value = '';
  orgFilter.value = 0;
  companyFilter.value = 0;
  completedFilter.value = false;
  applyFilters();
}

function refresh() {
  selectedId.value = null;
  detail.value = null;
  fetchList({ reset: true });
}

function loadMore() {
  if (listLoading.value) return;
  page.value += 1;
  fetchList({ reset: false });
}

// Пикеры орг/компаний - вспомогательные: без них поиск и фильтр по типу всё равно
// работают, дропдауны просто остаются с одним пунктом "Все". Список не блокируем.
async function loadFilterOptions() {
  try {
    const [orgs, companies] = await Promise.all([getOrganizations(), getCompanies()]);
    if (Array.isArray(orgs)) {
      orgOptions.value = [{ id: 0, name: 'Все организации' }, ...orgs];
    }
    if (Array.isArray(companies)) {
      companyOptions.value = [{ id: 0, name: 'Все компании' }, ...companies];
    }
  } catch {
    deletions.notify({
      type: 'error',
      bold: 'Не удалось загрузить',
      suffix: 'списки для фильтров',
    });
  }
}

async function selectAttachment(id) {
  selectedId.value = id;
  detail.value = null;
  const seq = ++detailSeq;
  try {
    const data = await getAccessibleAttachmentDetail(id);
    if (seq !== detailSeq) return;
    detail.value = data;
  } catch {
    if (seq !== detailSeq) return;
    selectedId.value = null;
    deletions.notify({
      type: 'error',
      bold: 'Не удалось открыть',
      suffix: 'вложение',
    });
  }
}

onMounted(() => {
  loadFilterOptions();
  fetchList({ reset: true });
});

onBeforeUnmount(() => clearTimeout(searchTimer));
</script>

<style scoped>
.accessible-attachments {
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 16px;
  overflow: hidden;
}

.management-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 15px 20px;
  border-bottom: 1px solid #e6e6e6;
}

.management-title {
  margin: 0;
  font-size: 1.2em;
  font-weight: 600;
  color: #000;
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.filters {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  padding: 12px 20px;
  border-bottom: 1px solid #e6e6e6;
}

/* Раскладка как в Центре заявок: поиск фикс. ширины + дропдауны фикс. ширины в
   один ряд с переносом. Поиск не растягиваем на всю строку, дропдауны не тянем -
   иначе ширины скачут, а кнопка сброса (всегда видима, :disabled) не двигает ряд. */
.filters__search {
  position: relative;
  flex: 0 1 280px;
  min-width: 220px;
}

.filters__search-icon {
  position: absolute;
  left: 14px;
  top: 50%;
  transform: translateY(-50%);
  width: 16px;
  height: 16px;
  opacity: 0.5;
  pointer-events: none;
}

.filters__search-input {
  padding-left: 38px;
}

.filters__dropdown {
  flex: 0 0 180px;
}

.filters__toggle,
.filters__reset {
  flex-shrink: 0;
}

@media (max-width: 640px) {
  .filters__search,
  .filters__dropdown {
    flex: 1 1 100%;
  }
}

.content-container {
  display: flex;
  height: 540px;
  width: 100%;
  overflow: hidden;
}

.list-section {
  width: 100%;
  display: flex;
  flex-direction: column;
  background: #fff;
}

.list-section.with-details {
  width: 40%;
  border-right: 1px solid #e6e6e6;
}

.cards-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 15px;
}

.attachment-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  text-align: left;
  padding: 12px 14px;
  background: #f9f9f9;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  cursor: pointer;
  transition: border-color 0.2s ease, background-color 0.2s ease, transform 0.2s ease;
  animation: card-in 0.3s ease-out forwards;
  opacity: 0;
  transform: translateY(10px);
}

@keyframes card-in {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

.attachment-card:hover {
  border-color: #4F5BDF;
  background: #f8f9ff;
}

.attachment-card--active {
  border-color: #4F5BDF;
  background: #f0f2ff;
}

.attachment-card__top {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.attachment-card__name {
  font-weight: 600;
  color: #333;
  font-size: 15px;
  flex: 1;
  min-width: 0;
}

.attachment-card__status {
  flex-shrink: 0;
  min-width: auto;
}

.attachment-card__meta {
  display: flex;
  flex-direction: column;
  gap: 3px;
  font-size: 13px;
  color: #333;
}

.attachment-card__field {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.attachment-card__field--places {
  white-space: normal;
}

.attachment-card__label {
  color: #a2a2a2;
  margin-right: 5px;
}

.load-more {
  align-self: center;
  margin-top: 4px;
}

.empty-state {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px 20px;
  text-align: center;
}

.empty-state p {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #333;
}

.empty-state__hint {
  font-size: 13px;
  color: #a2a2a2;
  max-width: 320px;
}

.list-footer {
  flex-shrink: 0;
  padding: 12px 20px;
  border-top: 1px solid #e6e6e6;
  font-size: 14px;
  color: #666;
}

.detail-section {
  width: 60%;
  display: flex;
  flex-direction: column;
  gap: 15px;
  background: #fff;
  overflow-y: auto;
  padding: 15px;
}

.application-block {
  background: #f9f9f9;
  border: 1px solid #e6e6e6;
  border-radius: 20px;
  padding: 15px;
}

.application-block__title {
  margin: 0 0 12px 0;
  font-size: 16px;
  font-weight: 700;
  color: #4F5BDF;
}

.application-block__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 12px 24px;
}

.application-block__row {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 14px;
}

.application-block__label {
  color: #a2a2a2;
  font-weight: 400;
}

.application-block__value {
  color: #000;
  font-size: 15px;
}

.detail-loading {
  padding: 10px;
}

@media (max-width: 900px) {
  .content-container {
    flex-direction: column;
    height: auto;
  }

  .list-section,
  .list-section.with-details,
  .detail-section {
    width: 100%;
  }

  .list-section.with-details {
    border-right: none;
    border-bottom: 1px solid #e6e6e6;
  }
}
</style>
