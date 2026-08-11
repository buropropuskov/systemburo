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
            placeholder="Поиск"
            data-testid="aa-search"
            @input="onSearchInput"
          >
        </div>
        <!-- Десктоп: вторичные фильтры инлайн в строке (как было). -->
        <template v-if="!isNarrow">
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
            class="lk-button filters__toggle"
            :class="nightFilter ? 'lk-button--primary' : 'lk-button--ghost'"
            :aria-pressed="nightFilter"
            title="Окно въезда пересекает ночь (22:00-06:00)"
            data-testid="aa-filter-night"
            @click="toggleNight"
          >
            Ночь
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
        </template>

        <!-- Мобилка: вторичные фильтры свёрнуты в кнопку «Фильтр» (поиск - снаружи). -->
        <FilterButton
          v-else
          :active="secondaryFiltersActive"
          data-testid="aa-filter-btn"
          @click="showFilterSheet = true"
        />
      </div>

      <!-- Мобилка: вторичные фильтры в bottom-sheet. -->
      <FilterSheet
        v-if="isNarrow"
        :show="showFilterSheet"
        :has-active-filters="hasActiveFilters"
        @close="showFilterSheet = false"
        @reset="resetFilters"
      >
        <div class="filter-section">
          <span class="filter-label">Тип</span>
          <BaseDropdown
            :model-value="typeFilter"
            :options="typeOptions"
            teleport
            data-testid="aa-sheet-type"
            @update:model-value="setType"
          />
        </div>
        <div class="filter-section">
          <span class="filter-label">Организация</span>
          <BaseDropdown
            :model-value="orgFilter"
            :options="orgOptions"
            searchable
            teleport
            data-testid="aa-sheet-org"
            @update:model-value="setOrg"
          />
        </div>
        <div class="filter-section">
          <span class="filter-label">Компания</span>
          <BaseDropdown
            :model-value="companyFilter"
            :options="companyOptions"
            searchable
            teleport
            data-testid="aa-sheet-company"
            @update:model-value="setCompany"
          />
        </div>
        <div class="filter-section">
          <span class="filter-label">Допуск</span>
          <div class="filter-sheet-toggles">
            <button
              type="button"
              class="lk-button filters__toggle"
              :class="completedFilter ? 'lk-button--primary' : 'lk-button--ghost'"
              :aria-pressed="completedFilter"
              data-testid="aa-sheet-completed"
              @click="toggleCompleted"
            >
              Завершённые
            </button>
            <button
              type="button"
              class="lk-button filters__toggle"
              :class="nightFilter ? 'lk-button--primary' : 'lk-button--ghost'"
              :aria-pressed="nightFilter"
              data-testid="aa-sheet-night"
              @click="toggleNight"
            >
              Ночь
            </button>
          </div>
        </div>
      </FilterSheet>

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
              <div class="attachment-card__head">
                <Badge
                  :variant="typeVariant(a.attachment_type)"
                  size="sm"
                >
                  {{ typeLabel(a.attachment_type) }}
                </Badge>
                <span class="attachment-card__org">{{ orgLine(a) }}</span>
                <span
                  v-if="dateRange(a)"
                  class="attachment-card__date"
                  data-testid="aa-card-date"
                >
                  {{ dateRange(a) }}
                </span>
                <StatusBadge
                  v-if="statusText(a)"
                  class="attachment-card__status"
                  :status="statusText(a)"
                />
              </div>
              <div class="attachment-card__name">
                {{ displayName(a) }}
              </div>
              <div
                v-if="metaLine(a)"
                class="attachment-card__meta"
              >
                {{ metaLine(a) }}
              </div>
              <div
                v-if="a.places"
                class="attachment-card__places"
              >
                <span class="attachment-card__places-label">Места:</span> {{ a.places }}
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
          ref="detailSection"
          class="detail-section"
          data-testid="aa-detail"
        >
          <!-- Мобилка: список и деталь не помещаются рядом, поэтому при выборе список
               скрывается и остаётся одна деталь - назад к списку по этой кнопке. -->
          <button
            type="button"
            class="lk-button lk-button--ghost detail-back"
            data-testid="aa-detail-back"
            @click="clearSelection"
          >
            <span class="detail-back__arrow" aria-hidden="true">←</span> К списку
          </button>

          <template v-if="detail">
            <div class="application-block">
              <h4 class="application-block__title">
                <span>Заявка<template v-if="detail.attachment.application_number"> № {{ detail.attachment.application_number }}</template></span>
                <StatusBadge
                  v-if="statusText(detail.attachment)"
                  :status="statusText(detail.attachment)"
                />
              </h4>
              <div class="application-block__grid">
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
                  data-testid="ob-aa-sender"
                >
                  <span class="application-block__label">Отправитель</span>
                  <span class="application-block__value">{{ senderName(detail.attachment) }}</span>
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

            <div
              v-if="detail.attachment.has_blank"
              class="detail-actions"
            >
              <button
                type="button"
                class="lk-button lk-button--primary"
                :disabled="previewLoading"
                data-testid="aa-preview-blank"
                @click="openPreview"
              >
                {{ previewLoading ? 'Загрузка...' : 'Посмотреть файл' }}
              </button>
            </div>

            <!-- AvailableAttachment не несёт roof_access/free_parking/custom_values -
                 эти опц. блоки детали просто не отрисуются (v-if по undefined). -->
            <!-- Карточку машины/сотрудника здесь не открываем: обработчиков нет,
                 поэтому строка не должна выглядеть кликабельной (#1392). -->
            <ApplicationAttachmentDetail
              :attachment="detail.attachment"
              :cars="detail.cars || []"
              :employees="detail.employees || []"
              :items="detail.items || []"
              :interactive="false"
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

    <BaseModal
      :show="previewOpen"
      title="Предпросмотр бланка"
      width="900px"
      content-class="blank-preview-modal"
      content-testid="ob-blank-preview"
      @close="closePreview"
    >
      <div class="blank-preview">
        <div
          v-if="previewLoading"
          class="blank-preview__state"
          data-testid="aa-preview-loading"
        >
          Загрузка превью...
        </div>
        <div
          v-else-if="previewError"
          class="blank-preview__state blank-preview__state--error"
          data-testid="aa-preview-error"
        >
          {{ previewError }}
        </div>
        <XlsxViewer
          v-else
          :file-buffer="previewBuffer"
          data-testid="aa-preview-viewer"
        />
      </div>
    </BaseModal>
  </AdminPageShell>
</template>

<script setup>
import { ref, computed, nextTick, watch, onMounted, onBeforeUnmount } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useOnboardingStore } from '@/stores/onboarding';
import AdminPageShell from '@/views/admin/AdminPageShell.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import Badge from '@/components/ui/Badge.vue';
import StatusBadge from '@/components/ui/StatusBadge.vue';
import SkeletonCard from '@/components/ui/SkeletonCard.vue';
import ApplicationAttachmentDetail from '@/components/ApplicationDetail/ApplicationAttachmentDetail.vue';
import BaseModal from '@/components/ui/BaseModal.vue';
import FilterSheet from '@/components/ui/FilterSheet.vue';
import FilterButton from '@/components/ui/FilterButton.vue';
import XlsxViewer from '@/components/admin/XlsxViewer.vue';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import { getAccessibleAttachments, getAccessibleAttachmentDetail } from '@/api/applications';
import { previewBlank } from '@/api/attachment-templates';
import { getOrganizations, getCompanies } from '@/api/organizations';
import { useDeletionsStore } from '@/stores/deletions';
import { formatDateRu, formatDateTime } from '@/utils/datetime';
import eventStream from '@/services/eventStream';

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
// "Ночь" - окно въезда вложения пересекает [22:00, 06:00). Считается на бэке из
// entry_time_from/to; при активном поиске бэк игнорирует флаг (как и completed).
const nightFilter = ref(route.query.night === '1');

const typeOptions = [
  { id: '', name: 'Все типы' },
  { id: 'cars', name: 'Автомобили' },
  { id: 'people', name: 'Сотрудники' },
  { id: 'items', name: 'ТМЦ' },
];
const orgOptions = ref([{ id: 0, name: 'Все организации' }]);
const companyOptions = ref([{ id: 0, name: 'Все компании' }]);

// Вторичные фильтры (без поиска) - один источник для точки-индикатора кнопки
// «Фильтр» и sheet; hasActiveFilters добавляет к ним поиск. Новый фильтр правится
// в ОДНОМ месте (плюс buildParams/resetFilters), а не в двух списках.
const secondaryFiltersActive = computed(
  () => !!typeFilter.value
    || orgFilter.value !== 0
    || companyFilter.value !== 0
    || completedFilter.value
    || nightFilter.value,
);
const hasActiveFilters = computed(() => !!search.value.trim() || secondaryFiltersActive.value);

// Мобилка: вторичные фильтры сворачиваются в кнопку «Фильтр» + FilterSheet; поиск
// остаётся снаружи. Точка-индикатор на кнопке - по secondaryFiltersActive, БЕЗ
// поиска (он виден отдельно), как hasModalFilters в Центре.
const { isNarrow } = useNarrowScreen();
const showFilterSheet = ref(false);

const items = ref([]);
const total = ref(0);
const page = ref(1);
const listLoading = ref(false);

const selectedId = ref(null);
const detail = ref(null);
const detailSection = ref(null);

// Предпросмотр заполненного бланка (#706 S4): модалка с XlsxViewer по тому же
// эндпоинту, что и скачивание, но буфер парсится во вьювере, а не сохраняется файлом.
const previewOpen = ref(false);
const previewBuffer = ref(null);
const previewLoading = ref(false);
const previewError = ref('');
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

// Шапка карточки: организация - главный ориентир охранника. organization_name уже
// COALESCE(орг, компания) от бэка; компанию добавляем второй, только если отличается,
// иначе вышло бы "Ромашка / Ромашка" для заявок без отдельной организации.
function orgLine(a) {
  const org = a.organization_name || '';
  const company = a.company_name || '';
  if (org && company && org !== company) return `${org} / ${company}`;
  return org || company || 'Без организации';
}

// Компактная строка меты под названием: номер, отправитель, даты через разделитель.
// Даты сюда не кладём: срок действия вынесен в отдельный контрастный бейдж шапки
// (рядом со статусом), чтобы читался чётко, а не терялся в серой мете.
function metaLine(a) {
  const parts = [];
  if (a.application_number) parts.push(`№ ${a.application_number}`);
  const sender = senderName(a);
  if (sender) parts.push(sender);
  return parts.join(' · ');
}

function buildParams() {
  const params = { page: page.value, per_page: PER_PAGE };
  const s = search.value.trim();
  if (s) params.search = s;
  if (typeFilter.value) params.attachment_type = typeFilter.value;
  if (orgFilter.value) params.organization_id = orgFilter.value;
  if (companyFilter.value) params.company_id = companyFilter.value;
  if (completedFilter.value) params.completed = true;
  if (nightFilter.value) params.night = true;
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
  if (nightFilter.value) query.night = '1';
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

function toggleNight() {
  nightFilter.value = !nightFilter.value;
  applyFilters();
}

function resetFilters() {
  clearTimeout(searchTimer);
  search.value = '';
  typeFilter.value = '';
  orgFilter.value = 0;
  companyFilter.value = 0;
  completedFilter.value = false;
  nightFilter.value = false;
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
  // Мобилка (<=900): деталь занимает место списка, а не встаёт под ним - иначе она
  // рендерится ниже длинного списка, за кромкой экрана, и «открыть вложение» на
  // телефоне выглядело как «ничего не произошло». Скроллим деталь в поле зрения.
  // matchMedia/scrollIntoView может не быть (jsdom) - тогда просто пропускаем.
  if (window.matchMedia?.('(max-width: 900px)')?.matches) {
    nextTick(() => detailSection.value?.scrollIntoView?.({ block: 'start' }));
  }
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

// Мобилка: вернуться от детали к списку (кнопка «К списку»). detailSeq++ гасит
// возможный ответ в пути, чтобы он не «переоткрыл» деталь после возврата.
function clearSelection() {
  detailSeq++;
  selectedId.value = null;
  detail.value = null;
}

// Открывается по явному клику на текущей детали; модалка-оверлей блокирует список,
// так что гонки с переключением вложения нет. Двойной клик гасит :disabled по previewLoading.
async function openPreview() {
  const att = detail.value?.attachment;
  if (!att) return;
  previewOpen.value = true;
  previewError.value = '';
  previewBuffer.value = null;
  previewLoading.value = true;
  try {
    previewBuffer.value = await previewBlank(att.application_id, att.attachment_id);
  } catch {
    previewError.value = 'Не удалось загрузить бланк для предпросмотра';
  } finally {
    previewLoading.value = false;
  }
}

function closePreview() {
  previewOpen.value = false;
  previewBuffer.value = null;
  previewError.value = '';
}

// Real-time (#840 V3): при согласовании заявки её вложения появляются в списке -
// сервер шлёт available.new, обновляем список без F5. Раньше вкладка была F5-only,
// поллинг не добавляем (fetchList сбрасывает пагинацию - периодический сброс мешал
// бы прокрутке; approve - редкое событие, SSE даёт живость без дизрапта).
let unsubAvailable = null;
/*
 * Раскрытие карточки вложения для онбординга: без него шаги про детали, скрытое
 * ФИО и просмотр бланка выпадали - их целей на экране нет, пока карточка не
 * выбрана, и тур молча перескакивал через весь разбор.
 *
 * Открываем первую карточку по сигналу и закрываем, когда сигнал гаснет, - но
 * только если открыли её сами (человек мог выбрать свою до начала шага).
 */
let attachmentOpenedByTour = false;
watch(
  () => [useOnboardingStore().revealOpen, items.value.length],
  ([target]) => {
    if (target === 'first-attachment') {
      const first = items.value[0];
      if (!first || selectedId.value !== null) return;
      attachmentOpenedByTour = true;
      selectAttachment(first.attachment_id);
      return;
    }
    // Просмотр бланка живёт ВНУТРИ открытой карточки: на этом сигнале карточку
    // держим, иначе шаг про бланк сам же и закрывает то, из чего бланк
    // открывается.
    if (target === 'attachment-blank') return;
    if (!attachmentOpenedByTour) return;
    attachmentOpenedByTour = false;
    selectedId.value = null;
    detail.value = null;
  },
);

/*
 * Тур просит показать сам бланк: рассказывать про кнопку «Посмотреть файл» и не
 * открывать файл - половина объяснения. Открываем предпросмотр по сигналу и
 * закрываем, когда сигнал гаснет; чужое окно не трогаем.
 */
let previewOpenedByTour = false;
watch(
  () => [useOnboardingStore().revealOpen, detail.value?.attachment?.has_blank],
  ([target, hasBlank]) => {
    if (target === 'attachment-blank') {
      if (!hasBlank || previewOpen.value) return;
      previewOpenedByTour = true;
      openPreview();
      return;
    }
    if (!previewOpenedByTour) return;
    previewOpenedByTour = false;
    closePreview();
  },
);

onMounted(() => {
  loadFilterOptions();
  fetchList({ reset: true });
  eventStream.connect();
  unsubAvailable = eventStream.subscribe('available', () => fetchList({ reset: true }));
});

onBeforeUnmount(() => {
  clearTimeout(searchTimer);
  if (unsubAvailable) {
    unsubAvailable();
    unsubAvailable = null;
  }
  eventStream.disconnect();
});
</script>

<style scoped>
.accessible-attachments {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  overflow: hidden;
}

.management-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 15px 20px;
  border-bottom: 1px solid var(--border);
}

.management-title {
  margin: 0;
  font-size: 1.2em;
  font-weight: 600;
  color: var(--text);
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
  border-bottom: 1px solid var(--border);
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

/* Кнопка «Фильтр» на мобилке - общий FilterButton (стиль из него). Здесь только
   раскладка тумблеров внутри sheet. */
.filter-sheet-toggles {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

/* Десктоп >640: дропдауны в ряд с переносом (на мобилке они не рендерятся). */
@media (max-width: 640px) {
  .filters__dropdown {
    flex: 1 1 100%;
  }
}

/* Мобилка (<=768, инлайн-фильтры скрыты): поиск в один ряд с кнопкой «Фильтр».
   flex-basis:0 (не auto) - иначе basis по длинному placeholder распирает поиск и
   flex-wrap переносит кнопку на строку ниже; с basis 0 поиск растёт заполняя
   остаток после кнопки, и обе держатся в одной строке. */
@media (max-width: 768px) {
  .filters__search {
    flex: 1 1 0;
    min-width: 0;
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
  background: var(--surface);
}

.list-section.with-details {
  width: 40%;
  border-right: 1px solid var(--border);
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
  gap: 5px;
  width: 100%;
  text-align: left;
  padding: 12px 14px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 15px;
  box-shadow: var(--shadow-sm, 0 1px 2px rgba(16, 24, 40, 0.06));
  cursor: pointer;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
  animation: card-in 0.3s ease-out forwards;
  opacity: 0;
  transform: translateY(10px);
}

@keyframes card-in {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

.attachment-card:hover {
  border-color: var(--accent);
  box-shadow: 0 3px 10px rgba(79, 91, 223, 0.15);
}

.attachment-card--active {
  border-color: var(--accent);
  background: var(--accent-tint);
  box-shadow: 0 3px 10px rgba(79, 91, 223, 0.18);
}

.attachment-card__head {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

/* min-width держит организацию читаемой: на узкой карточке (открыта деталь, 40%)
   дата и статус переносятся на вторую строку, а не сжимают организацию в ничто. */
.attachment-card__org {
  flex: 1;
  min-width: 90px;
  font-weight: 700;
  font-size: 14.5px;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Срок действия: контрастный бейдж (тёмный текст на белом + обводка), чтобы дата
   читалась чётко рядом со статусом, а не терялась в серой мете. */
.attachment-card__date {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  padding: 3px 9px;
  border-radius: 999px;
  background: var(--surface);
  border: 1px solid var(--border);
  color: var(--text);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.attachment-card__status {
  flex-shrink: 0;
  min-width: auto;
}

.attachment-card__name {
  font-weight: 600;
  font-size: 14px;
  color: var(--accent-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.attachment-card__meta {
  font-size: 12.5px;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.attachment-card__places {
  font-size: 12.5px;
  color: var(--text);
  line-height: 1.35;
}

.attachment-card__places-label {
  color: var(--text-muted);
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
  color: var(--text);
}

.empty-state__hint {
  font-size: 13px;
  color: var(--text-muted);
  max-width: 320px;
}

.list-footer {
  flex-shrink: 0;
  padding: 12px 20px;
  border-top: 1px solid var(--border);
  font-size: 14px;
  color: var(--text-muted);
}

.detail-section {
  width: 60%;
  display: flex;
  flex-direction: column;
  gap: 15px;
  background: var(--surface);
  overflow-y: auto;
  padding: 15px;
}

.application-block {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 16px 18px;
  box-shadow: var(--shadow-sm, 0 1px 2px rgba(16, 24, 40, 0.06));
}

.application-block__title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin: 0 0 14px 0;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--border);
  font-size: 15px;
  font-weight: 700;
  color: var(--accent-text);
}

.application-block__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 14px 20px;
}

.application-block__row {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
  font-size: 14px;
}

.application-block__label {
  color: var(--text-muted);
  font-weight: 400;
  font-size: 12.5px;
}

.application-block__value {
  color: var(--text);
  font-size: 14.5px;
  font-weight: 500;
  word-break: break-word;
}

.detail-loading {
  padding: 10px;
}

/* Кнопка возврата к списку - только мобилка (<=900), где список скрыт под деталью.
   На десктопе список и деталь стоят рядом, возвращаться некуда. */
.detail-back {
  display: none;
  align-self: flex-start;
}

.detail-back__arrow {
  margin-right: 2px;
  font-size: 16px;
  line-height: 1;
}

.detail-actions {
  display: flex;
  justify-content: flex-start;
}

/*
 * «Посмотреть файл» на время загрузки подписывается «Загрузка...», и кнопка
 * прыгала в ширине. Держим её размер по длинной подписи.
 */
.detail-actions .lk-button {
  min-width: 172px;
  justify-content: center;
}

.blank-preview {
  padding: 16px 20px 20px;
}

.blank-preview__state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
  color: var(--text-muted);
  font-size: 14px;
}

.blank-preview__state--error {
  color: var(--danger-text);
}

@media (max-width: 900px) {
  .content-container {
    flex-direction: column;
    height: auto;
  }

  .list-section,
  .detail-section {
    width: 100%;
  }

  /* Выбрано вложение - список уступает место детали целиком (не встаёт под ним за
     кромкой экрана). Возврат к списку - кнопкой «К списку» в детали. */
  .list-section.with-details {
    display: none;
  }

  .detail-back {
    display: inline-flex;
  }

  /* Скролл списка: на десктопе content-container - фикс-высота 540px с ВНУТРЕННИМ
     скроллом cards-list (master-detail). На мобилке height:auto убирает эту опору,
     и внутренний overflow-y:auto у cards-list перестаёт работать (нет ограниченной
     высоты для flex:1), а overflow:hidden выше по цепочке (.accessible-attachments,
     .content-container) КЛАМПИТ и обрезает список - скролл мёртв. Даём контенту течь
     естественно - список скроллится вместе со страницей (внешним скролл-контейнером
     роута), а не запертым внутренним overflow. */
  .accessible-attachments {
    overflow: visible;
  }

  .content-container {
    overflow: visible;
  }

  .cards-list,
  .detail-section {
    overflow-y: visible;
  }
}
</style>

<style>
.blank-preview-modal.base-modal {
  border-radius: 35px;
}

@media (max-width: 768px) {
  .blank-preview-modal.base-modal {
    border-radius: 16px 16px 0 0;
  }
}
</style>
