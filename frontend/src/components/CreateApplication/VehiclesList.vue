<template>
  <div class="data__list">
    <div class="header-with-badge">
      <h4>Список транспортных средств</h4>
      <span class="vehicles-badge">{{ vehicles.length }}</span>
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
          v-for="(vehicle, index) in vehicles"
          :key="vehicle.id"
          class="vcol__cell vcol__cell--muted"
          :class="rowState(vehicle, index)"
          @mouseenter="hoveredIndex = index"
        >
          {{ index + 1 }}
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
          v-for="(vehicle, index) in vehicles"
          :key="vehicle.id"
          class="vcol__cell vcol__cell--text"
          :class="rowState(vehicle, index)"
          @mouseenter="hoveredIndex = index"
        >
          {{ vehicle.plateNumber || 'Не указано' }}
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
          v-for="(vehicle, index) in vehicles"
          :key="vehicle.id"
          class="vcol__cell vcol__cell--text"
          :class="rowState(vehicle, index)"
          @mouseenter="hoveredIndex = index"
        >
          {{ vehicle.mark || 'Не указано' }}
        </div>
      </div>

      <div class="vcol vcol--actions">
        <div class="vcol__head vcol__head--actions">
          Действия
        </div>
        <div
          v-for="(vehicle, index) in vehicles"
          :key="vehicle.id"
          class="vcol__cell vcol__cell--actions"
          :class="rowState(vehicle, index)"
          @mouseenter="hoveredIndex = index"
        >
          <button
            class="details-btn"
            title="Детали"
            @click="showVehicleDetails(vehicle)"
          >
            <DetailsIcon class="details-icon" />
          </button>
          <button
            class="edit-btn"
            title="Редактировать"
            @click="$emit('edit-vehicle', vehicle)"
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
            @click="$emit('delete-vehicle', vehicle.id)"
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
          v-for="(vehicle, index) in vehicles"
          :key="vehicle.id"
          class="table-row rt-row"
          :class="{ 'has-active': vehicle.activeInfo }"
        >
          <div class="table-col number-col">
            {{ index + 1 }}
          </div>
          <div class="table-col plate-col">
            <div class="cell-with-icon">
              {{ vehicle.plateNumber || 'Не указано' }}
            </div>
          </div>
          <div class="table-col mark-col">
            <div class="cell-with-icon">
              {{ vehicle.mark || 'Не указано' }}
            </div>
          </div>
          <div class="table-col actions-col">
            <button
              class="details-btn"
              title="Детали"
              @click="showVehicleDetails(vehicle)"
            >
              <DetailsIcon class="details-icon" />
            </button>
            <button
              class="edit-btn"
              title="Редактировать"
              @click="$emit('edit-vehicle', vehicle)"
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
              @click="$emit('delete-vehicle', vehicle.id)"
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
import DetailsIcon from '@/components/ui/DetailsIcon.vue';
import { useNarrowScreen } from '@/composables/useNarrowScreen';

export default {
    name: 'VehiclesList',
    components: {
        VehicleDetailsModal,
        DetailsIcon
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
        }
    },
    emits: ['sort', 'edit-vehicle', 'delete-vehicle'],
    // 767.98 - тот же порог, что у карточного @media: ниже него рендерим карточки,
    // выше - колоночную раскладку с выделением по столбцам.
    setup() {
        const { isNarrow } = useNarrowScreen(767.98);
        return { isNarrow };
    },
    data() {
        return {
            showDetailsModal: false,
            selectedVehicle: null,
            // Индекс строки под курсором: подсвечиваем ячейки того же индекса во всех
            // столбцах (в колоночном DOM «строки» как элемента нет - синхроним по index).
            hoveredIndex: null
        }
    },
    methods: {
        // Классы состояния ячейки: жёлтая подсветка уже активной машины и hover-строка
        // по общему индексу (колонки - отдельные элементы, CSS :hover строку не даёт).
        rowState(vehicle, index) {
            return {
                'vcol__cell--active': !!vehicle.activeInfo,
                'vcol__cell--hover': this.hoveredIndex === index
            };
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
    gap: 8px;
    padding-bottom: 12px;
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
