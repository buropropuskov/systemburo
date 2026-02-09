<template>
    <div class="data__list">
        <div class="header-with-badge">
            <h4>Список транспортных средств</h4>
            <span class="vehicles-badge">{{ vehicles.length }}</span>
        </div>
        <div class="vehicles-table">
            <div class="table-header">
                <div class="header-col number-col" @click="$emit('sort', 'number')">
                    <p :class="{ 'active-sort': sortField === 'number' }">№</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'number' && sortDirection === 'desc'
                        }" 
                    />
                </div>
                <div class="header-col plate-col" @click="$emit('sort', 'plate')">
                    <p :class="{ 'active-sort': sortField === 'plate' }">Номер</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'plate' && sortDirection === 'desc'
                        }" 
                    />
                </div>
                <div class="header-col mark-col" @click="$emit('sort', 'mark')">
                    <p :class="{ 'active-sort': sortField === 'mark' }">Марка</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'mark' && sortDirection === 'desc'
                        }" 
                    />
                </div>
                <div class="header-col place-col" @click="$emit('sort', 'place')">
                    <p :class="{ 'active-sort': sortField === 'place' }">Место разгрузки</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'place' && sortDirection === 'desc'
                        }" 
                    />
                </div>
                <div class="header-col actions-col">
                    <!-- Убрано слово "Действие" -->
                </div>
            </div>
            <div class="table-body">
                <div 
                    v-for="(vehicle, index) in vehicles" 
                    :key="vehicle.id"
                    class="table-row"
                >
                    <div class="table-col number-col">{{ index + 1 }}</div>
                    <div class="table-col plate-col">{{ vehicle.plateNumber }}</div>
                    <div class="table-col mark-col">{{ vehicle.mark }}</div>
                    <div class="table-col place-col">{{ vehicle.unloadingPlace }}</div>
                    <div class="table-col actions-col">
                        <button 
                            class="edit-btn"
                            @click="$emit('edit-vehicle', vehicle)"
                            title="Редактировать"
                        >
                            <img 
                                src="@/assets/icons/edit.png" 
                                alt="Редактировать" 
                                class="edit-icon"
                            />
                        </button>
                        <button 
                            class="delete-btn"
                            @click="$emit('delete-vehicle', vehicle.id)"
                        >
                            <img 
                                src="@/assets/icons/trashcan.png" 
                                alt="Удалить" 
                                class="delete-icon"
                            />
                        </button>
                    </div>
                </div>
                <div v-if="vehicles.length === 0" class="no-vehicles">
                    Нет добавленных транспортных средств
                </div>
            </div>
        </div>
    </div>
</template>

<script>
export default {
    name: 'VehiclesList',
    props: {
        vehicles: Array,
        sortField: String,
        sortDirection: String
    },
    emits: ['sort', 'edit-vehicle', 'delete-vehicle']
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

/* Vehicles table styles */
.vehicles-table {
    width: 100%;
    border: 1px solid #e0e0e0;
    border-radius: 12px;
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
    transform: rotate(0deg); /* По умолчанию направлена вверх */
}

.sort-icon.desc {
    transform: rotate(180deg); /* При desc направлена вниз */
    opacity: 0.8;
}

.table-body {
    max-height: 180px;
    overflow-y: auto;
    background: #fff;
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

.place-col {
    width: 30%;
}

.actions-col {
    width: 10%;
    text-align: center;
    display: flex;
    justify-content: center;
    gap: 4px;
}

.edit-btn, .delete-btn {
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

.edit-btn:hover {
    background: #e3f2fd;
}

.delete-btn:hover {
    background: #ffebee;
}

.edit-icon, .delete-icon {
    width: 14px;
    height: 14px;
    opacity: 0.6;
    transition: opacity 0.2s ease;
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
    color: #000;
    font-weight: 600;
    margin: 0;
}

/* Scrollbar styling */
.table-body::-webkit-scrollbar {
    width: 4px;
}

.table-body::-webkit-scrollbar-track {
    background: #f1f1f1;
}

.table-body::-webkit-scrollbar-thumb {
    background: #c1c1c1;
    border-radius: 2px;
}

.table-body::-webkit-scrollbar-thumb:hover {
    background: #a8a8a8;
}
</style>