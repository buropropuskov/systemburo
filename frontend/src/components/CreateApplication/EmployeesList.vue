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
                <div class="header-col lastName-col" @click="$emit('sort', 'lastName')">
                    <p :class="{ 'active-sort': sortField === 'lastName' }">Фамилия</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'lastName' && sortDirection === 'desc'
                        }" 
                    />
                </div>
                <div class="header-col firstName-col" @click="$emit('sort', 'firstName')">
                    <p :class="{ 'active-sort': sortField === 'firstName' }">Имя</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'firstName' && sortDirection === 'desc'
                        }" 
                    />
                </div>
                <div class="header-col middleName-col" @click="$emit('sort', 'middleName')">
                    <p :class="{ 'active-sort': sortField === 'middleName' }">Отчество</p>
                    <img 
                        src="@/assets/icons/sort.png" 
                        class="sort-icon" 
                        :class="{ 
                            'desc': sortField === 'middleName' && sortDirection === 'desc'
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
                    <div class="table-col lastName-col">
                        {{ employee.lastName || 'Не указано' }}
                    </div>
                    <div class="table-col firstName-col">
                        {{ employee.firstName || 'Не указано' }}
                    </div>
                    <div class="table-col middleName-col">
                        {{ employee.middleName || 'Не указано' }}
                    </div>
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
        <transition name="modal-fade">
            <div v-if="showDetailsModal" class="modal-overlay" @click.self="closeDetailsModal">
                <div class="modal-content compact-modal">
                    <div class="modal-header">
                        <h3 class="modal-title">Детальная информация о сотруднике</h3>
                        <button @click="closeDetailsModal" class="modal-close">
                            <svg width="10" height="10" viewBox="0 0 14 14" fill="none">
                                <path d="M13 1L1 13M1 1L13 13" stroke="#666" stroke-width="2" stroke-linecap="round"/>
                            </svg>
                        </button>
                    </div>
                    
                    <div class="modal-body">
                        <div class="employee-details" v-if="selectedEmployee">
                            <div class="details-section">
                                <h4 class="section-title">Основная информация</h4>
                                <div class="details-grid two-columns">
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
                                <h4 class="section-title">Документы</h4>
                                <div class="details-grid two-columns">
                                    <div class="detail-item">
                                        <label class="detail-label">Серия и номер паспорта:</label>
                                        <div class="sensitive-data">
                                            <span 
                                                class="data-text"
                                                :class="{ 'hidden-data': !showFullPassport }"
                                            >
                                                {{ selectedEmployee.passportSeriesNumber || 'Не указан' }}
                                            </span>
                                            <button 
                                                v-if="selectedEmployee.passportSeriesNumber"
                                                @click="togglePassportVisibility"
                                                class="show-more-btn"
                                                :class="{ 'active': showFullPassport }"
                                            >
                                                {{ showFullPassport ? 'Скрыть' : 'Показать' }}
                                            </button>
                                        </div>
                                    </div>
                                    <div v-if="selectedEmployee.patentNumber" class="detail-item">
                                        <label class="detail-label">Номер патента:</label>
                                        <div class="sensitive-data">
                                            <span 
                                                class="data-text"
                                                :class="{ 'hidden-data': !showFullPatent }"
                                            >
                                                {{ selectedEmployee.patentNumber }}
                                            </span>
                                            <button 
                                                @click="togglePatentVisibility"
                                                class="show-more-btn"
                                                :class="{ 'active': showFullPatent }"
                                            >
                                                {{ showFullPatent ? 'Скрыть' : 'Показать' }}
                                            </button>
                                        </div>
                                    </div>
                                    <div v-if="selectedEmployee.otherPermission" class="detail-item full-width">
                                        <label class="detail-label">Иное разрешение:</label>
                                        <span class="detail-value">{{ selectedEmployee.otherPermission }}</span>
                                    </div>
                                </div>
                            </div>

                            <div class="details-section">
                                <h4 class="section-title">Места прохода</h4>
                                <div class="passage-tables-list">
                                    <div 
                                        v-for="table in getAllPassageTables(selectedEmployee)" 
                                        :key="table"
                                        class="passage-table-item"
                                    >
                                        {{ table }}
                                    </div>
                                    <div v-if="getAllPassageTables(selectedEmployee).length === 0" class="no-tables">
                                        Не указаны
                                    </div>
                                </div>
                            </div>

                            <div v-if="selectedEmployee.isExisting" class="existing-badge">
                                <span class="badge-text">Существующий сотрудник</span>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </transition>
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
            selectedEmployee: null,
            showFullPassport: false,
            showFullPatent: false
        }
    },
    methods: {
        togglePassportVisibility() {
            this.showFullPassport = !this.showFullPassport;
        },

        togglePatentVisibility() {
            this.showFullPatent = !this.showFullPatent;
        },

        showEmployeeDetails(employee) {
            this.selectedEmployee = employee;
            this.showFullPassport = false;
            this.showFullPatent = false;
            this.showDetailsModal = true;
        },

        closeDetailsModal() {
            this.showDetailsModal = false;
            this.selectedEmployee = null;
            this.showFullPassport = false;
            this.showFullPatent = false;
        },

        getAllPassageTables(employee) {
            if (!employee || !employee.passageTables) {
                return [];
            }
            
            // Если passageTables - это строка, разбиваем ее по запятым
            if (typeof employee.passageTables === 'string') {
                return employee.passageTables.split(',').map(table => table.trim()).filter(table => table);
            }
            
            // Если passageTables - это массив, возвращаем его
            if (Array.isArray(employee.passageTables)) {
                return employee.passageTables;
            }
            
            return [];
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

.lastName-col {
    width: 22%;
}

.firstName-col {
    width: 22%;
}

.middleName-col {
    width: 22%;
}

.actions-col {
    width: 26%;
    text-align: center;
    display: flex;
    justify-content: center;
    gap: 4px;
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

/* Стили для улучшенного модального окна */
.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.3);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    backdrop-filter: blur(1px);
    animation: overlayAppear 0.3s ease-out;
}

@keyframes overlayAppear {
    from {
        background: rgba(0, 0, 0, 0);
        backdrop-filter: blur(0px);
    }
    to {
        background: rgba(0, 0, 0, 0.3);
        backdrop-filter: blur(1px);
    }
}

.modal-content {
    background: #fff;
    border-radius: 50px;
    padding: 0;
    padding-bottom: 15px;
    width: 500px;
    max-width: 90vw;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
    animation: modalAppear 0.3s ease-out;
}

.modal-content.compact-modal {
    width: 520px;
    max-height: 80vh;
}

@keyframes modalAppear {
    from {
        opacity: 0;
        transform: scale(0.8) translateY(-20px);
    }
    to {
        opacity: 1;
        transform: scale(1) translateY(0);
    }
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px 30px 16px;
    border-bottom: 1px solid #f0f0f0;
}

.modal-title {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: #1a1a1a;
}

.modal-close {
    background: none;
    border: none;
    cursor: pointer;
    padding: 6px;
    border-radius: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s ease;
}

.modal-close:hover {
    background-color: #f5f5f5;
    transform: rotate(90deg);
}

.modal-body {   
    padding: 20px 30px;
    max-height: 65vh;
    overflow-y: auto;
}

.modal-body.no-scroll {
    max-height: none;
    overflow-y: visible;
}

/* Стили для деталей сотрудника */
.employee-details {
    display: flex;
    flex-direction: column;
    gap: 16px;
}

.details-section {
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    padding: 20px;
    background: #fafafa;
}

.section-title {
    margin: 0 0 12px 0;
    font-size: 14px;
    font-weight: 600;
    color: #333;
    padding-bottom: 8px;
    border-bottom: 1px solid #e6e6e6;
}

.details-grid.two-columns {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
}

.detail-item {
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.detail-item.full-width {
    grid-column: 1 / -1;
}

.detail-label {
    font-size: 12px;
    color: #666;
    font-weight: 500;
}

.detail-value {
    font-size: 13px;
    color: #333;
    font-weight: 500;
}

/* Стили для чувствительных данных */
.sensitive-data {
    display: flex;
    align-items: center;
    gap: 15px;
}

.data-text {
    font-size: 13px;
    color: #333;
    font-weight: 500;
    letter-spacing: 0.5px;
    transition: all 0.3s ease;
    word-break: break-all;
}

.data-text.hidden-data {
    filter: blur(4px);
    user-select: none;
}

.show-more-btn {
    background: #f8f9fa;
    border: 1px solid #e0e0e0;
    color: #4F5BDF;
    font-size: 11px;
    cursor: pointer;
    padding: 4px 8px;
    border-radius: 15px;
    transition: all 0.2s;
    font-weight: 500;
    white-space: nowrap;
    min-width: 75px;
    text-align: center;
}

.show-more-btn:hover {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.show-more-btn.active {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
    min-width: 75px;
}

/* Стили для списка мест прохода */
.passage-tables-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: 120px;
    overflow-y: auto;
    padding-right: 4px;
}

.passage-table-item {
    padding: 8px 12px;
    background: white;
    border: 1px solid #e0e0e0;
    border-radius: 8px;
    font-size: 13px;
    color: #333;
    transition: all 0.2s ease;
}

.passage-table-item:hover {
    background: #f8f9fa;
    border-color: #4F5BDF;
}

.no-tables {
    text-align: center;
    padding: 12px;
    color: #666;
    font-size: 13px;
    font-style: italic;
}

.existing-badge {
    display: flex;
    justify-content: center;
    margin-top: 8px;
}

.badge-text {
    background: #e3f2fd;
    color: #1976d2;
    padding: 6px 12px;
    border-radius: 16px;
    font-size: 12px;
    font-weight: 500;
}

/* Анимации для модального окна */
.modal-fade-enter-active,
.modal-fade-leave-active {
    transition: all 0.3s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
    opacity: 0;
}

.modal-fade-enter-active .modal-overlay,
.modal-fade-leave-active .modal-overlay {
    transition: all 0.3s ease;
}

.modal-fade-enter-active .modal-content,
.modal-fade-leave-active .modal-content {
    transition: all 0.3s ease;
}

.modal-fade-enter-from .modal-overlay,
.modal-fade-leave-to .modal-overlay {
    background: rgba(0, 0, 0, 0);
    backdrop-filter: blur(0px);
}

.modal-fade-enter-from .modal-content,
.modal-fade-leave-to .modal-content {
    opacity: 0;
    transform: scale(0.8) translateY(-20px);
}

/* Стили для скроллбара в списке мест прохода */
.passage-tables-list::-webkit-scrollbar {
    width: 4px;
}

.passage-tables-list::-webkit-scrollbar-track {
    background: #f1f1f1;
    border-radius: 2px;
}

.passage-tables-list::-webkit-scrollbar-thumb {
    background: #c1c1c1;
    border-radius: 2px;
}

.passage-tables-list::-webkit-scrollbar-thumb:hover {
    background: #a8a8a8;
}

@media (max-width: 768px) {
    .details-grid.two-columns {
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
    
    .lastName-col,
    .firstName-col,
    .middleName-col {
        width: 30% !important;
    }
    
    .modal-content.compact-modal {
        width: 95%;
        margin: 20px;
        border-radius: 30px;
    }
    
    .modal-body {
        padding: 16px 20px;
    }
    
    .modal-header {
        padding: 16px 20px;
    }
    
    .details-section {
        padding: 16px;
        border-radius: 16px;
    }
    
    .sensitive-data {
        flex-direction: column;
        align-items: flex-start;
        gap: 6px;
    }
    
    .show-more-btn {
        align-self: flex-start;
    }
}

@media (max-width: 480px) {
    .modal-header {
        padding: 12px 16px;
    }
    
    .modal-title {
        font-size: 14px;
    }
    
    .section-title {
        font-size: 13px;
    }
    
    .detail-label {
        font-size: 11px;
    }
    
    .detail-value {
        font-size: 12px;
    }
    
    .modal-content.compact-modal {
        border-radius: 20px;
    }
    
    .details-section {
        border-radius: 12px;
        padding: 12px;
    }
}
</style>