<template>
    <div class="data__list">
        <div class="header-with-badge">
            <h4>Список сотрудников</h4>
            <span class="employees-badge">{{ employees.length }}</span>
        </div>
        <div class="employees-table">
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
                <div class="header-col name-col" @click="$emit('sort', 'name')">
                    <p :class="{ 'active-sort': sortField === 'name' }">ФИО</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'name' && sortDirection === 'desc'
                        }" 
                    />
                </div>
                <div class="header-col position-col" @click="$emit('sort', 'position')">
                    <p :class="{ 'active-sort': sortField === 'position' }">Должность</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'position' && sortDirection === 'desc'
                        }" 
                    />
                </div>
                <div class="header-col tables-col" @click="$emit('sort', 'tables')">
                    <p :class="{ 'active-sort': sortField === 'tables' }">Места прохода</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'tables' && sortDirection === 'desc'
                        }" 
                    />
                </div>
                <div class="header-col actions-col">
                    Действия
                </div>
            </div>
            <div class="table-body">
                <div 
                    v-for="(employee, index) in employees" 
                    :key="employee.id"
                    class="table-row"
                >
                    <div class="table-col number-col">{{ index + 1 }}</div>
                    <div class="table-col name-col">
                        {{ formatFullName(employee) }}
                        <span v-if="employee.patentNumber" class="patent-indicator" title="Есть патент">📄</span>
                    </div>
                    <div class="table-col position-col">{{ employee.position }}</div>
                    <div class="table-col tables-col">{{ employee.passageTables }}</div>
                    <div class="table-col actions-col">
                        <button 
                            class="details-btn"
                            @click="showEmployeeDetails(employee)"
                            title="Детали"
                        >
                            <img 
                                src="@/assets/icons/info.png" 
                                alt="Детали" 
                                class="details-icon"
                            />
                        </button>
                        <button 
                            class="edit-btn"
                            @click="$emit('edit-employee', employee)"
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
                            @click="$emit('delete-employee', employee.id)"
                        >
                            <img 
                                src="@/assets/icons/trashcan.png" 
                                alt="Удалить" 
                                class="delete-icon"
                            />
                        </button>
                    </div>
                </div>
                <div v-if="employees.length === 0" class="no-employees">
                    Нет добавленных сотрудников
                </div>
            </div>
        </div>

        <!-- Модальное окно деталей сотрудника -->
        <div v-if="showDetailsModal" class="modal-overlay" @click="closeDetailsModal">
            <div class="modal-content" @click.stop>
                <div class="modal-header">
                    <div class="modal-header__top">
                        <h3>Детальная информация о сотруднике</h3>
                    </div>
                    <button class="modal-close" @click="closeDetailsModal">×</button>
                </div>
                <div class="modal-body">
                    <div class="employee-details" v-if="selectedEmployee">
                        <div class="details-section">
                            <h4>Основная информация</h4>
                            <div class="details-grid">
                                <div class="detail-item">
                                    <label class="detail-label">Фамилия:</label>
                                    <span class="detail-value">{{ selectedEmployee.lastName || 'Не указано' }}</span>
                                </div>
                                <div class="detail-item">
                                    <label class="detail-label">Имя:</label>
                                    <span class="detail-value">{{ selectedEmployee.firstName || 'Не указано' }}</span>
                                </div>
                                <div class="detail-item">
                                    <label class="detail-label">Отчество:</label>
                                    <span class="detail-value">{{ selectedEmployee.middleName || 'Не указано' }}</span>
                                </div>
                                <div class="detail-item">
                                    <label class="detail-label">Должность:</label>
                                    <span class="detail-value">{{ selectedEmployee.position || 'Не указана' }}</span>
                                </div>
                                <div class="detail-item">
                                    <label class="detail-label">Гражданство:</label>
                                    <span class="detail-value">{{ selectedEmployee.citizenshipName || 'Не указано' }}</span>
                                </div>
                            </div>
                        </div>

                        <div class="details-section">
                            <h4>Документы</h4>
                            <div class="details-grid">
                                <div class="detail-item">
                                    <label class="detail-label">Серия и номер паспорта:</label>
                                    <span class="detail-value">{{ selectedEmployee.passportSeriesNumber || 'Не указан' }}</span>
                                </div>
                                <div v-if="selectedEmployee.patentNumber" class="detail-item">
                                    <label class="detail-label">Номер патента:</label>
                                    <span class="detail-value">{{ selectedEmployee.patentNumber }}</span>
                                </div>
                                <div v-if="selectedEmployee.otherPermission" class="detail-item">
                                    <label class="detail-label">Иное разрешение:</label>
                                    <span class="detail-value">{{ selectedEmployee.otherPermission }}</span>
                                </div>
                            </div>
                        </div>

                        <div class="details-section">
                            <h4>Места прохода</h4>
                            <div class="tables-list">
                                <span class="tables-value">{{ selectedEmployee.passageTables || 'Не указаны' }}</span>
                            </div>
                        </div>

                        <div v-if="selectedEmployee.isExisting" class="details-section">
                            <div class="existing-badge">
                                <span class="badge-text">Существующий сотрудник</span>
                            </div>
                        </div>
                    </div>
                </div>
                <div class="modal-actions">
                    <button class="close-btn" @click="closeDetailsModal">Закрыть</button>
                </div>
            </div>
        </div>
    </div>
</template>

<script>
export default {
    name: 'EmployeesList',
    props: {
        employees: Array,
        sortField: String,
        sortDirection: String
    },
    emits: ['sort', 'edit-employee', 'delete-employee'],
    data() {
        return {
            showDetailsModal: false,
            selectedEmployee: null
        }
    },
    methods: {
        formatFullName(employee) {
            const parts = [];
            if (employee.lastName) parts.push(employee.lastName);
            if (employee.firstName) parts.push(employee.firstName);
            if (employee.middleName) parts.push(employee.middleName);
            return parts.join(' ') || 'Не указано';
        },

        showEmployeeDetails(employee) {
            this.selectedEmployee = employee;
            this.showDetailsModal = true;
        },

        closeDetailsModal() {
            this.showDetailsModal = false;
            this.selectedEmployee = null;
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

.employees-badge {
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

/* Employees table styles */
.employees-table {
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
    width: 8%;
    text-align: center;
}

.name-col {
    width: 25%;
    display: flex;
    align-items: center;
    gap: 5px;
}

.position-col {
    width: 22%;
}

.tables-col {
    width: 30%;
}

.actions-col {
    width: 15%;
    text-align: center;
    display: flex;
    justify-content: center;
    gap: 4px;
}

.patent-indicator {
    font-size: 12px;
    opacity: 0.7;
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
    width: 14px;
    height: 14px;
    opacity: 0.6;
    transition: opacity 0.2s ease;
}

.details-btn:hover .details-icon {
    opacity: 0.9;
}

.edit-btn:hover .edit-icon {
    opacity: 0.9;
}

.delete-btn:hover .delete-icon {
    opacity: 0.9;
}

.no-employees {
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

/* Модальное окно деталей */
.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
}

.modal-content {
    background: white;
    border-radius: 20px;
    padding: 0;
    width: 500px;
    max-width: 90vw;
    max-height: 80vh;
    overflow: hidden;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px;
    border-bottom: 1px solid #e6e6e6;
}

.modal-header h3 {
    margin: 0;
    color: #333;
    font-size: 18px;
}

.modal-close {
    background: none;
    border: none;
    font-size: 24px;
    cursor: pointer;
    color: #a2a2a2;
    padding: 0;
    width: 30px;
    height: 30px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.modal-close:hover {
    color: #333;
}

.modal-body {
    padding: 20px;
    max-height: 60vh;
    overflow-y: auto;
}

.employee-details {
    display: flex;
    flex-direction: column;
    gap: 20px;
}

.details-section {
    border: 1px solid #e6e6e6;
    border-radius: 12px;
    padding: 15px;
}

.details-section h4 {
    margin: 0 0 15px 0;
    font-size: 16px;
    color: #333;
    border-bottom: 1px solid #e6e6e6;
    padding-bottom: 8px;
}

.details-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
}

.detail-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.detail-label {
    font-size: 12px;
    color: #666;
    font-weight: 500;
}

.detail-value {
    font-size: 14px;
    color: #333;
    font-weight: 500;
}

.tables-list {
    padding: 8px 0;
}

.tables-value {
    font-size: 14px;
    color: #333;
}

.existing-badge {
    display: flex;
    justify-content: center;
}

.badge-text {
    background: #e3f2fd;
    color: #1976d2;
    padding: 6px 12px;
    border-radius: 20px;
    font-size: 12px;
    font-weight: 500;
}

.modal-actions {
    display: flex;
    justify-content: flex-end;
    padding: 15px 20px;
    border-top: 1px solid #e6e6e6;
}

.close-btn {
    background: #4F5BDF;
    color: white;
    border: none;
    border-radius: 12px;
    padding: 10px 20px;
    font-size: 14px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.close-btn:hover {
    background: #3a45c0;
}

@media (max-width: 768px) {
    .details-grid {
        grid-template-columns: 1fr;
    }
    
    .table-row {
        flex-wrap: wrap;
    }
    
    .table-col {
        width: 50% !important;
        margin-bottom: 4px;
    }
    
    .actions-col {
        width: 100%;
        justify-content: flex-end;
    }
}
</style>