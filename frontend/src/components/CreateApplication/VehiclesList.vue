<template>
  <div class="data__list">
    <div class="header-with-badge">
      <h4>Список транспортных средств</h4>
      <span class="vehicles-badge">{{ vehicles.length }}</span>
      <div
        v-if="canImport || vehicles.length"
        class="header-actions"
      >
        <div
          v-if="canImport"
          class="import-entry"
        >
          <span
            class="hint-anchor"
            data-hint="Массовый ввод из бланка в опытной эксплуатации: проверяйте, что попало в список"
          >
            <Badge
              variant="warning"
              size="sm"
              label="Experimental"
            />
          </span>
          <button
            type="button"
            class="lk-button lk-button--secondary lk-button--sm import-entry__btn"
            data-testid="vehicles-import-btn"
            :aria-pressed="importActive ? 'true' : 'false'"
            @click="$emit('toggle-import')"
          >
            {{ importActive ? 'Закрыть импорт' : 'Импорт' }}
          </button>
        </div>
        <button
          v-if="vehicles.length"
          type="button"
          class="lk-button lk-button--danger lk-button--sm header-actions__btn"
          data-testid="vehicles-clear-btn"
          @click="showClearConfirm = true"
        >
          Очистить
        </button>
      </div>
    </div>

    <!-- Импорт бланком (blank-import) может завести до 2000 машин - показываем поиск
         и постраничную навигацию только когда список реально большой, чтобы обычная
         ручная подача из нескольких машин выглядела как раньше. -->
    <div
      v-if="showToolbar"
      class="list-toolbar"
    >
      <input
        v-model="searchQuery"
        type="text"
        class="lk-input list-search"
        placeholder="Поиск по номеру или марке"
        data-testid="vehicles-search"
      >
      <Pager
        v-if="totalPages > 1"
        class="list-pager"
        :page="currentPage"
        :total-pages="totalPages"
        :total="filteredVehicles.length"
        page-prefix="Стр. "
        @update:page="goToPage"
      />
    </div>

    <!-- Десктоп/планшет: колоночная раскладка. В DOM ячейки идут по столбцам, поэтому
         выделение мышью сверху-вниз захватывает один столбец и копируется как список
         значений (Ctrl+C). Мобилка (карточки) - строковый блок ниже. -->
    <div
      v-if="!isNarrow"
      class="vehicles-cols"
      @mouseleave="hoveredIndex = null"
    >
      <div class="vcol vcol--index">
        <div
          class="vcol__head"
          @click="$emit('sort', 'number')"
        >
          <p :class="{ 'active-sort': sortField === 'number' }">
            №
          </p>
          <img
            src="@/assets/icons/sort.png"
            class="sort-icon"
            :class="{ 'desc': sortField === 'number' && sortDirection === 'desc' }"
          >
        </div>
        <div
          v-for="(row, index) in pagedVehicles"
          :key="row.item.id"
          class="vcol__cell vcol__cell--muted"
          data-testid="vehicles-row"
          :class="rowState(row.item, index)"
          @mouseenter="hoveredIndex = index"
        >
          {{ row.number }}
        </div>
      </div>

      <div class="vcol vcol--plate">
        <div
          class="vcol__head"
          @click="$emit('sort', 'plate')"
        >
          <p :class="{ 'active-sort': sortField === 'plate' }">
            Номер
          </p>
          <img
            src="@/assets/icons/sort.png"
            class="sort-icon"
            :class="{ 'desc': sortField === 'plate' && sortDirection === 'desc' }"
          >
        </div>
        <div
          v-for="(row, index) in pagedVehicles"
          :key="row.item.id"
          class="vcol__cell vcol__cell--text"
          :class="rowState(row.item, index)"
          @mouseenter="hoveredIndex = index"
        >
          {{ row.item.plateNumber || 'Не указано' }}
        </div>
      </div>

      <div class="vcol vcol--mark">
        <div
          class="vcol__head"
          @click="$emit('sort', 'mark')"
        >
          <p :class="{ 'active-sort': sortField === 'mark' }">
            Марка
          </p>
          <img
            src="@/assets/icons/sort.png"
            class="sort-icon"
            :class="{ 'desc': sortField === 'mark' && sortDirection === 'desc' }"
          >
        </div>
        <div
          v-for="(row, index) in pagedVehicles"
          :key="row.item.id"
          class="vcol__cell vcol__cell--text"
          :class="rowState(row.item, index)"
          @mouseenter="hoveredIndex = index"
        >
          {{ row.item.mark || 'Не указано' }}
        </div>
      </div>

      <div class="vcol vcol--actions">
        <div class="vcol__head vcol__head--actions">
          Действия
        </div>
        <div
          v-for="(row, index) in pagedVehicles"
          :key="row.item.id"
          class="vcol__cell vcol__cell--actions"
          :class="rowState(row.item, index)"
          @mouseenter="hoveredIndex = index"
        >
          <button
            class="details-btn"
            title="Детали"
            @click="showVehicleDetails(row.item)"
          >
            <DetailsIcon class="details-icon" />
          </button>
          <button
            class="edit-btn"
            title="Редактировать"
            @click="$emit('edit-vehicle', row.item)"
          >
            <img
              src="@/assets/icons/edit.png"
              alt="Редактировать"
              class="edit-icon"
            >
          </button>
          <button
            class="delete-btn"
            title="Удалить"
            @click="$emit('delete-vehicle', row.item.id)"
          >
            <img
              src="@/assets/icons/trashcan.png"
              alt="Удалить"
              class="delete-icon"
            >
          </button>
        </div>
      </div>
    </div>

    <!-- Мобилка: строки становятся карточками (rt-* из responsive-tables.css). -->
    <div
      v-else
      class="vehicles-table rt-table"
    >
      <div class="table-header rt-head-row">
        <div
          class="header-col number-col"
          @click="$emit('sort', 'number')"
        >
          <p :class="{ 'active-sort': sortField === 'number' }">
            №
          </p>
        </div>
        <div
          class="header-col plate-col"
          @click="$emit('sort', 'plate')"
        >
          <p :class="{ 'active-sort': sortField === 'plate' }">
            Номер
          </p>
        </div>
        <div
          class="header-col mark-col"
          @click="$emit('sort', 'mark')"
        >
          <p :class="{ 'active-sort': sortField === 'mark' }">
            Марка
          </p>
        </div>
        <div class="header-col actions-col">
          Действия
        </div>
      </div>
      <div class="table-body">
        <div
          v-for="row in pagedVehicles"
          :key="row.item.id"
          class="table-row rt-row"
          data-testid="vehicles-row"
          :class="{ 'has-active': row.item.activeInfo, 'is-pending': row.item.isPending }"
        >
          <div class="table-col number-col">
            {{ row.number }}
          </div>
          <div class="table-col plate-col">
            <div class="cell-with-icon">
              {{ row.item.plateNumber || 'Не указано' }}
            </div>
          </div>
          <div class="table-col mark-col">
            <div class="cell-with-icon">
              {{ row.item.mark || 'Не указано' }}
            </div>
          </div>
          <div class="table-col actions-col">
            <button
              class="details-btn"
              title="Детали"
              @click="showVehicleDetails(row.item)"
            >
              <DetailsIcon class="details-icon" />
            </button>
            <button
              class="edit-btn"
              title="Редактировать"
              @click="$emit('edit-vehicle', row.item)"
            >
              <img
                src="@/assets/icons/edit.png"
                alt="Редактировать"
                class="edit-icon"
              >
            </button>
            <button
              class="delete-btn"
              title="Удалить"
              @click="$emit('delete-vehicle', row.item.id)"
            >
              <img
                src="@/assets/icons/trashcan.png"
                alt="Удалить"
                class="delete-icon"
              >
            </button>
          </div>
        </div>
      </div>
    </div>

    <div
      v-if="vehicles.length === 0"
      class="no-vehicles"
    >
      Нет добавленных транспортных средств
    </div>
    <div
      v-else-if="filteredVehicles.length === 0"
      class="no-vehicles"
    >
      Ничего не найдено по запросу «{{ searchQuery }}»
    </div>

    <!-- Очистка списка необратима (отмены на странице нет), поэтому идёт только через
         подтверждение; само удаление делает родитель по clear-list. -->
    <ConfirmationModal
      :show="showClearConfirm"
      title="Очистить список"
      :message="clearConfirmMessage"
      confirm-text="Очистить"
      cancel-text="Отмена"
      :confirm-button-style="{ background: 'var(--danger)', borderColor: 'var(--danger)', color: 'var(--fill-text)' }"
      @confirm="confirmClear"
      @cancel="showClearConfirm = false"
    />

    <!-- Модальное окно деталей транспортного средства -->
    <VehicleDetailsModal
      :show="showDetailsModal"
      :vehicle="selectedVehicle"
      :all-unloading-places="allUnloadingPlaces"
      :license-plate-formats="licensePlateFormats"
      :show-car-features="false"
      :readonly="true"
      :active-info="selectedVehicle?.activeInfo"
      @close="closeDetailsModal"
    />
  </div>
</template>

<script>
import VehicleDetailsModal from './VehicleDetailsModal.vue';
import ConfirmationModal from '@/components/ConfirmationModal.vue';
import Badge from '@/components/ui/Badge.vue';
import DetailsIcon from '@/components/ui/DetailsIcon.vue';
import Pager from '@/components/ui/Pager.vue';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import { useListSearchPagination } from '@/composables/useListSearchPagination';

export default {
    name: 'VehiclesList',
    components: {
        VehicleDetailsModal,
        ConfirmationModal,
        Badge,
        DetailsIcon,
        Pager
    },
    props: {
        vehicles: {
            type: Array,
            required: true
        },
        sortField: { type: String, default: null },
        sortDirection: { type: String, default: null },
        allUnloadingPlaces: {
            type: Array,
            default: () => []
        },
        licensePlateFormats: {
            type: Array,
            default: () => []
        },
        showStatus: {
            type: Boolean,
            default: true
        },
        // Дата+время текущего вложения - на сущности машины их нет, подмешиваем
        // при открытии карточки просмотра (срок действия и время пребывания).
        detailInfo: {
            type: Object,
            default: () => ({})
        },
        // Вход в массовый ввод из бланка (blank-import-ux, U4): гейт права
        // action.import.list считает родитель, здесь только показ кнопки.
        canImport: {
            type: Boolean,
            default: false
        },
        importActive: {
            type: Boolean,
            default: false
        }
    },
    emits: ['sort', 'edit-vehicle', 'delete-vehicle', 'toggle-import', 'clear-list'],
    // 767.98 - тот же порог, что у карточного @media: ниже него рендерим карточки,
    // выше - колоночную раскладку с выделением по столбцам.
    setup(props) {
        const { isNarrow } = useNarrowScreen(767.98);

        // Поиск+постраничный показ - см. useListSearchPagination (blank-import E1: до
        // 2000 машин, рендерить всё v-for'ом - да ещё продублированным на 4 колонки в
        // десктопной раскладке - не годится).
        const {
            searchQuery,
            currentPage,
            showToolbar,
            filteredItems: filteredVehicles,
            totalPages,
            pagedItems: pagedVehicles,
            goToPage
        } = useListSearchPagination(
            () => props.vehicles,
            (vehicle, q) => `${vehicle.plateNumber || ''} ${vehicle.mark || ''}`.toLowerCase().includes(q)
        );

        return {
            isNarrow,
            searchQuery,
            currentPage,
            showToolbar,
            filteredVehicles,
            totalPages,
            pagedVehicles,
            goToPage
        };
    },
    data() {
        return {
            showDetailsModal: false,
            selectedVehicle: null,
            showClearConfirm: false,
            // Индекс строки под курсором: подсвечиваем ячейки того же индекса во всех
            // столбцах (в колоночном DOM «строки» как элемента нет - синхроним по index).
            hoveredIndex: null
        }
    },
    computed: {
        // Считаем по ВСЕМУ списку, а не по видимой странице: поиск и пейджер режут показ,
        // а чистится вложение целиком - иначе число в вопросе обещало бы меньше, чем уйдёт.
        clearConfirmMessage() {
            const pending = this.vehicles.filter(vehicle => vehicle.isPending).length;
            const fromBlank = pending > 0 ? `, из них предварительных из бланка: ${pending}` : '';
            return `Будет убрано строк: ${this.vehicles.length}${fromBlank}. Отменить это действие нельзя.`;
        }
    },
    watch: {
        currentPage() {
            // Клик по пейджеру не даёт mouseleave - без сброса подсветка осталась бы
            // на строке с тем же ЛОКАЛЬНЫМ индексом, то есть на другой машине.
            this.hoveredIndex = null;
        }
    },
    methods: {
        // Классы состояния ячейки: жёлтая подсветка уже активной машины и hover-строка
        // по общему индексу (колонки - отдельные элементы, CSS :hover строку не даёт).
        rowState(vehicle, index) {
            return {
                'vcol__cell--active': !!vehicle.activeInfo,
                'vcol__cell--hover': this.hoveredIndex === index,
                // Строка из бланка, ещё не добавленная в заявку (blank-import-ux, U5).
                'vcol__cell--pending': !!vehicle.isPending
            };
        },

        confirmClear() {
            this.showClearConfirm = false;
            this.$emit('clear-list');
        },

        showVehicleDetails(vehicle) {
            const info = this.detailInfo || {};
            this.selectedVehicle = {
                ...vehicle,
                organization: vehicle.organization || info.organization,
                company: vehicle.company || info.company,
                entry_date_to: vehicle.entry_date_to || info.entryDateTo,
                entry_time_from: vehicle.entry_time_from || info.timeFrom,
                entry_time_to: vehicle.entry_time_to || info.timeTo
            };
            this.showDetailsModal = true;
        },

        closeDetailsModal() {
            this.showDetailsModal = false;
            this.selectedVehicle = null;
        }
    }
}
</script>

<style scoped>
.data__list {
    padding: 12px;
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
}

.header-with-badge {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    padding-bottom: 12px;
}

/* Действия списка прижаты к правому краю шапки: вход в импорт, за ним очистка.
   Перенос обязателен: у .lk-button white-space: nowrap, и на узком экране пара
   «Закрыть импорт» + «Очистить» иначе выехала бы за край шапки. */
.header-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    flex-wrap: wrap;
    gap: 8px;
    margin-left: auto;
}

.import-entry {
    display: flex;
    align-items: center;
    gap: 8px;
}

@media (max-width: 768px) {
    .import-entry__btn,
    .header-actions__btn {
        min-height: 44px;
        padding: 4px 14px;
    }
}

.vehicles-badge {
    background: var(--accent);
    color: var(--accent-contrast);
    padding: 2px 6px;
    border-radius: 10px;
    font-size: 11px;
    font-weight: 600;
    min-width: 18px;
    text-align: center;
    line-height: 1.2;
}

.list-toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 8px;
    padding-bottom: 10px;
}

/* Границы/фон/фокус/тёмная тема - на .lk-input, здесь только раскладка в тулбаре. */
.list-search {
    flex: 1;
    min-width: 160px;
    height: 32px;
    padding: 6px 12px;
    font-size: 13px;
}

.list-pager {
    flex-shrink: 0;
    color: var(--text-muted);
}

/* Колоночная раскладка (десктоп/планшет): каждый столбец - отдельный flex-column,
   ячейки одного индекса выровнены по фиксированной высоте. Растягиваем на высоту
   соседней формы (data__list stretch по form__data) - список показывает максимум
   строк, а не фиксированные ~180px; переполнение уходит во внутренний скролл. */
.vehicles-cols {
    display: flex;
    flex: 1;
    min-height: 180px;
    overflow-y: auto;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    box-shadow: 0 1px 3px var(--shadow-drop);
    scrollbar-width: thin;
}

.vcol {
    display: flex;
    flex-direction: column;
    min-width: 0;
}

.vcol--index {
    width: 10%;
    flex-shrink: 0;
    user-select: none;
    text-align: center;
}

.vcol--plate {
    width: 32%;
    /* Столбцы с данными выделяемы для копирования; служебные (№, действия) - нет. */
    user-select: text;
}

.vcol--mark {
    width: 30%;
    user-select: text;
}

.vcol--actions {
    width: 28%;
    flex-shrink: 0;
    user-select: none;
}

.vcol__head {
    position: sticky;
    top: 0;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: 4px;
    height: 40px;
    padding: 0 12px;
    box-sizing: border-box;
    background: var(--surface-2);
    border-bottom: 1px solid var(--border);
    font-weight: 500;
    color: var(--text-muted);
    font-size: 13px;
    cursor: pointer;
    user-select: none;
    transition: color 0.2s ease;
}

.vcol--index .vcol__head {
    justify-content: center;
}

.vcol__head--actions {
    cursor: default;
    justify-content: center;
}

.vcol__head p {
    margin: 0;
}

.vcol__head:hover:not(.vcol__head--actions) p,
.vcol__head p.active-sort {
    color: var(--text);
}

.sort-icon {
    width: 10px;
    height: 10px;
    transition: all 0.2s ease;
    opacity: 0.4;
    transform: rotate(0deg);
}

.vcol__head:hover .sort-icon,
.vcol__head .active-sort ~ .sort-icon {
    opacity: 0.8;
}

.sort-icon.desc {
    transform: rotate(180deg);
    opacity: 0.8;
}

.vcol__cell {
    height: 38px;
    padding: 0 12px;
    box-sizing: border-box;
    line-height: 38px;
    font-size: 13px;
    border-bottom: 1px solid var(--surface-2);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    transition: background-color 0.2s ease;
}

.vcol__cell:last-child {
    border-bottom: none;
}

.vcol__cell--muted {
    color: var(--text-muted);
    text-align: center;
}

.vcol__cell--actions {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    line-height: normal;
    overflow: visible;
}

.vcol__cell--hover {
    background: var(--surface-2);
}

.vcol__cell--active {
    background: var(--warning-bg);
    border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
}

.vcol__cell--active.vcol__cell--hover {
    background: var(--warning);
}

/* Предварительная строка (blank-import-ux, U5): разобрана из бланка, но в заявку ещё не
   добавлена - текст приглушён, а действия (детали, правка, удаление) работают как у
   обычной. Метка слева - inset-тень, а не border: он сдвинул бы текст ячейки. */
.vcol__cell--pending {
    color: var(--text-muted);
}

.vcol--index .vcol__cell--pending {
    box-shadow: inset 2px 0 0 var(--text-muted);
}

/* --- Строковая раскладка (мобильные карточки) --- */
.vehicles-table {
    width: 100%;
    border: 1px solid var(--border);
    border-radius:20px;
    overflow: hidden;
    box-shadow: 0 1px 3px var(--shadow-drop);
}

.table-header {
    display: flex;
    background: var(--surface-2);
    border-bottom: 1px solid var(--border);
    padding: 10px 12px;
    font-weight: 500;
    color: var(--text-muted);
    font-size: 13px;
}

.header-col {
    display: flex;
    align-items: center;
    gap: 4px;
    cursor: pointer;
    transition: all 0.2s ease;
    user-select: none;
}

.header-col:hover,
.header-col.active-sort {
    color: var(--text);
}

.table-body {
    max-height: 180px;
    overflow-y: auto;
    background: var(--surface);
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.table-body::-webkit-scrollbar {
    display: none;
}

.table-row {
    display: flex;
    padding: 8px 12px;
    border-bottom: 1px solid var(--surface-2);
    align-items: center;
    font-size: 13px;
    transition: background-color 0.2s ease;
}

.table-row:last-child {
    border-bottom: none;
}

.table-row:hover {
    background: var(--surface-2);
}

/* Карточная раскладка: та же приглушённость, что и в колонках выше. */
.table-row.is-pending {
    color: var(--text-muted);
    box-shadow: inset 3px 0 0 var(--text-muted);
}

.table-row.has-active {
    background-color: var(--warning-bg);
    border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
}

.table-row.has-active:hover {
    background-color: var(--warning);
}

.header-col, .table-col {
    padding: 0 4px;
}

.number-col {
    width: 10%;
    text-align: center;
}

.plate-col {
    width: 25%;
}

.mark-col {
    width: 25%;
}

.actions-col {
    width: 20%;
    text-align: center;
    display: flex;
    justify-content: center;
    gap: 4px;
}

.cell-with-icon {
    display: flex;
    align-items: center;
    gap: 6px;
}

.details-btn, .edit-btn, .delete-btn {
    background: none;
    border: none;
    cursor: pointer;
    padding: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
    transition: all 0.2s ease;
}

.details-btn:hover {
    background: var(--info-bg);
}

.edit-btn:hover {
    background: var(--success-bg);
}

.delete-btn:hover {
    background: var(--danger-bg);
}

.details-icon, .edit-icon, .delete-icon {
    width: 18px;
    height: 18px;
    opacity: 0.6;
    transition: opacity 0.2s ease, color 0.2s ease;
}

.details-btn .details-icon {
    color: var(--text);
}

.details-btn:hover .details-icon {
    opacity: 1;
    color: var(--color-primary, var(--accent-text));
}

.edit-btn:hover .edit-icon {
    opacity: 0.9;
}

.delete-btn:hover .delete-icon {
    opacity: 0.9;
}

.no-vehicles {
    text-align: center;
    padding: 16px;
    color: var(--text-muted);
    font-size: 13px;
    font-style: italic;
}

h4 {
    font-size: 16px;
    color: var(--text);
    font-weight: 600;
    margin: 0;
}

/* Мобилка: строки таблицы становятся карточками (rt-* из responsive-tables.css).
   Подписи полей не выводим - решение по эпику: карточки без лейблов, как в Центре;
   номер и марка читаются сами по себе. Брейкпоинт 767.98 - как у инфраструктуры. */
@media (max-width: 767.98px) {
    .vehicles-table {
        border: none;
        border-radius: 0;
        box-shadow: none;
        /* Только по Y: по X инфраструктура держит свой overflow-x: hidden. */
        overflow-y: visible;
    }

    /* Тач-таргет 44px (WCAG 2.5.5, эталон адаптивности #1097). */
    .list-search {
        height: 44px;
        font-size: 16px;
    }

    .list-pager :deep(.lk-button) {
        min-height: 44px;
    }

    /* Список больше не скроллится внутри 180px - страница скроллит сама. */
    .table-body {
        max-height: none;
        overflow-y: visible;
        background: transparent;
    }

    .table-row.rt-row {
        position: relative;
        flex-direction: row !important;
        flex-wrap: wrap;
        align-items: center;
        gap: 2px 8px;
        min-height: 56px;
        /* Резерв под три кнопки действий, приколотые справа. */
        padding: 10px 136px 10px 12px !important;
        font-size: 14px;
    }

    /* Подсветку уже заведённой машины возвращаем: карточный фон приходит
       из инфраструктуры с !important и иначе её съедает. */
    .table-row.rt-row.has-active {
        background: var(--warning-bg) !important;
        border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface)) !important;
    }

    .table-col {
        width: auto !important;
        padding: 0;
    }

    .number-col {
        color: var(--text-muted);
        font-size: 12px;
    }

    .plate-col {
        font-weight: 600;
        font-size: 15px;
    }

    /* Марка уходит на вторую строку карточки. */
    .mark-col {
        flex-basis: 100%;
        color: var(--text-muted);
        font-size: 13px;
    }

    .actions-col {
        position: absolute;
        top: 50%;
        right: 8px;
        transform: translateY(-50%);
        width: auto !important;
        gap: 2px;
    }

    .details-btn,
    .edit-btn,
    .delete-btn {
        width: 40px;
        height: 40px;
    }

    .details-icon,
    .edit-icon,
    .delete-icon {
        width: 20px;
        height: 20px;
        opacity: 0.75;
    }
}
</style>
