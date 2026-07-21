<template>
  <div class="data__list">
    <div class="header-with-badge">
      <h4>Список транспортных средств</h4>
      <span class="vehicles-badge">{{ vehicles.length }}</span>
    </div>
    <div class="vehicles-table rt-table">
      <div class="table-header rt-head-row">
        <div
          class="header-col number-col"
          @click="$emit('sort', 'number')"
        >
          <p :class="{ 'active-sort': sortField === 'number' }">
            №
          </p>
          <img 
            src="@/assets/icons/sort.png" 
            class="sort-icon" 
            :class="{ 
              'desc': sortField === 'number' && sortDirection === 'desc'
            }" 
          >
        </div>
        <div
          class="header-col plate-col"
          @click="$emit('sort', 'plate')"
        >
          <p :class="{ 'active-sort': sortField === 'plate' }">
            Номер
          </p>
          <img 
            src="@/assets/icons/sort.png" 
            class="sort-icon" 
            :class="{ 
              'desc': sortField === 'plate' && sortDirection === 'desc'
            }" 
          >
        </div>
        <div
          class="header-col mark-col"
          @click="$emit('sort', 'mark')"
        >
          <p :class="{ 'active-sort': sortField === 'mark' }">
            Марка
          </p>
          <img 
            src="@/assets/icons/sort.png" 
            class="sort-icon" 
            :class="{ 
              'desc': sortField === 'mark' && sortDirection === 'desc'
            }" 
          >
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
        <div
          v-if="vehicles.length === 0"
          class="no-vehicles"
        >
          Нет добавленных транспортных средств
        </div>
      </div>
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
    data() {
        return {
            showDetailsModal: false,
            selectedVehicle: null
        }
    },
    methods: {
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
}

.header-with-badge {
    display: flex;
    align-items: center;
    gap: 8px;
    padding-bottom: 12px;
}

.vehicles-badge {
    background: #1976d2;
    color: white;
    padding: 2px 6px;
    border-radius: 10px;
    font-size: 11px;
    font-weight: 600;
    min-width: 18px;
    text-align: center;
    line-height: 1.2;
}

.vehicles-table {
    width: 100%;
    border: 1px solid #e0e0e0;
    border-radius:20px;
    overflow: hidden;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.table-header {
    display: flex;
    background: #f5f5f5;
    border-bottom: 1px solid #e0e0e0;
    padding: 10px 12px;
    font-weight: 500;
    color: #666;
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
    color: #333;
}

.header-col:hover .sort-icon,
.header-col.active-sort .sort-icon {
    opacity: 0.8;
}

.sort-icon {
    width: 10px;
    height: 10px;
    transition: all 0.2s ease;
    opacity: 0.4;
    transform: rotate(0deg);
}

.sort-icon.desc {
    transform: rotate(180deg);
    opacity: 0.8;
}

.table-body {
    max-height: 180px;
    overflow-y: auto;
    background: #fff;
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.table-body::-webkit-scrollbar {
    display: none;
}

.table-row {
    display: flex;
    padding: 8px 12px;
    border-bottom: 1px solid #f5f5f5;
    align-items: center;
    font-size: 13px;
    transition: background-color 0.2s ease;
}

.table-row:last-child {
    border-bottom: none;
}

.table-row:hover {
    background: #f8f9fa;
}

.table-row.has-active {
    background-color: #fff3cd;
}

.table-row.has-active:hover {
    background-color: #ffe69b;
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

.status-col {
    width: 20%;
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

.status-icon {
    width: 14px;
    height: 14px;
}

.active-badge {
    display: inline-block;
    padding: 2px 8px;
    background: #ffc107;
    color: #856404;
    border-radius: 12px;
    font-size: 10px;
    font-weight: 500;
    white-space: nowrap;
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
    background: #e3f2fd;
}

.edit-btn:hover {
    background: #e8f5e8;
}

.delete-btn:hover {
    background: #ffebee;
}

.details-icon, .edit-icon, .delete-icon {
    width: 18px;
    height: 18px;
    opacity: 0.6;
    transition: opacity 0.2s ease, color 0.2s ease;
}

.details-btn .details-icon {
    color: #4a4a4a;
}

.details-btn:hover .details-icon {
    opacity: 1;
    color: var(--color-primary, #4F5BDF);
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
    color: #666;
    font-size: 13px;
    font-style: italic;
}

h4 {
    font-size: 16px;
    color: #333;
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
        background: #fff3cd !important;
    }

    .table-col {
        width: auto !important;
        padding: 0;
    }

    .number-col {
        color: #a2a2a2;
        font-size: 12px;
    }

    .plate-col {
        font-weight: 600;
        font-size: 15px;
    }

    /* Марка уходит на вторую строку карточки. */
    .mark-col {
        flex-basis: 100%;
        color: #666;
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