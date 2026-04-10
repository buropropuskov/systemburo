<template>
    <section class="employeesview">
        <header class="employeesview__header">
            <h2 class="employeesview__title">
                Список <span class="blue">сотрудников</span>
            </h2>
        </header>
        
        <div class="employeesview__filters">
            <div class="filters-container">
                <SearchComponent
                    :title="'Поиск сотрудников...'"
                    v-model="searchQuery"
                />
                <div class="filter-tabs" v-if="ownershipInfo">
                    <button 
                        v-if="ownershipInfo.has_organization"
                        class="filter-tab"
                        :class="{ 'filter-tab--active': currentFilter === 'organization' }"
                        @click="switchFilter('organization')"
                    >
                        Сотрудники организации
                    </button>
                    <button 
                        v-if="ownershipInfo.has_company"
                        class="filter-tab"
                        :class="{ 'filter-tab--active': currentFilter === 'company' }"
                        @click="switchFilter('company')"
                    >
                        Сотрудники компании
                    </button>
                    <button 
                        class="filter-tab"
                        :class="{ 'filter-tab--active': currentFilter === 'user' }"
                        @click="switchFilter('user')"
                    >
                        Мои сотрудники
                    </button>
                </div>
            </div>
        </div>

        <div class="employeesview__container">
            <!-- Таблица сотрудников -->
            <div class="employees-card">
                <div class="card-header">
                    <div class="card-header__title">
                        <h3 class="card-title">
                            <span v-if="currentFilter === 'organization'" class="highlight-text">Сотрудники <span class="blue">организации</span></span>
                            <span v-else-if="currentFilter === 'company'" class="highlight-text">Сотрудники <span class="blue">компании</span></span>
                            <span v-else class="highlight-text">Мои <span class="blue">сотрудники</span></span>
                        </h3>
                    </div>
                    <div class="card-header__settings">
                        <button class="add-button" @click="showAddEmployeeModal">
                            Добавить
                        </button>
                        <RefreshButton @refresh="fetchEmployees" />
                    </div>
                </div>
                
                <div class="card-content">
                    <!-- Заголовок таблицы всегда отображается -->
                    <div class="employees-header">
                        <div class="header-row">
                            <div class="header-col number-col" @click="sortBy('id')">
                                <p :class="{ 'active-sort': sortField === 'id' }">№</p>
                                <img 
                                    src="@/assets/icons/sort.png" 
                                    class="sort-icon" 
                                    :class="{ 
                                        'sorted': sortField === 'id',
                                        'desc': sortField === 'id' && sortDirection === 'desc'
                                    }" 
                                />
                            </div>
                            <div class="header-col name-col" @click="sortBy('last_name')">
                                <p :class="{ 'active-sort': sortField === 'last_name' }">ФИО</p>
                                <img 
                                    src="@/assets/icons/sort.png" 
                                    class="sort-icon" 
                                    :class="{ 
                                        'sorted': sortField === 'last_name',
                                        'desc': sortField === 'last_name' && sortDirection === 'desc'
                                    }" 
                                />
                            </div>
                            <div class="header-col position-col" @click="sortBy('position')">
                                <p :class="{ 'active-sort': sortField === 'position' }">Должность</p>
                                <img 
                                    src="@/assets/icons/sort.png" 
                                    class="sort-icon" 
                                    :class="{ 
                                        'sorted': sortField === 'position',
                                        'desc': sortField === 'position' && sortDirection === 'desc'
                                    }" 
                                />
                            </div>
                            <div class="header-col status-col" @click="sortBy('status')">
                                <p :class="{ 'active-sort': sortField === 'status' }">Статус</p>
                                <img 
                                    src="@/assets/icons/sort.png" 
                                    class="sort-icon" 
                                    :class="{ 
                                        'sorted': sortField === 'status',
                                        'desc': sortField === 'status' && sortDirection === 'desc'
                                    }" 
                                />
                            </div>
                            <div class="header-col actions-col">
                                Действия
                            </div>
                        </div>
                    </div>
                    
                    <!-- Тело таблицы -->
                    <div class="employees-container">
                        <SkeletonTransition :loading="loading">
                            <template #skeleton>
                                <SkeletonTable :rows="6" :columns="5" />
                            </template>
                        <div v-if="filteredEmployees.length > 0" class="employees-body">
                            <div 
                                v-for="(employee) in sortedEmployees" 
                                :key="employee.id" 
                                class="employee-item"
                            >
                                <div class="employee-row">
                                    <div class="employee-col number-col">
                                        {{ employee.id }}
                                    </div>
                                    <div class="employee-col name-col" :title="formatFullName(employee)">
                                        {{ truncateText(formatFullName(employee), 20) }}
                                    </div>
                                    <div class="employee-col position-col" :title="employee.position || 'Не указана'">
                                        {{ truncateText(employee.position || 'Не указана', 20) }}
                                    </div>
                                    <div class="employee-col status-col">
                                        <span 
                                            class="status-badge"
                                            :class="{
                                                'status-active': employee.status,
                                                'status-inactive': !employee.status
                                            }"
                                        >
                                            {{ employee.status ? 'Активен' : 'Неактивен' }}
                                        </span>
                                    </div>
                                    <div class="employee-col actions-col">
                                        <button 
                                            @click="editEmployee(employee)" 
                                            class="edit-btn"
                                            title="Редактировать"
                                        >
                                            <img 
                                                src="@/assets/icons/edit.png" 
                                                alt="Редактировать" 
                                                class="edit-icon"
                                            />
                                        </button>
                                        <button 
                                            @click="deleteEmployee(employee)" 
                                            class="delete-btn"
                                            title="Удалить"
                                        >
                                            <img 
                                                src="@/assets/icons/trashcan.png" 
                                                alt="Удалить" 
                                                class="delete-icon"
                                            />
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>
                        <p v-else class="no-data-message">
                            {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Сотрудников нет' }}
                        </p>
                        </SkeletonTransition>
                    </div>
                </div>
            </div>
            
            <div class="employeesview__right-side">
                <div class="employeesview__help">
                    <p class="help__text">
                        Здесь находятся сотрудники, привязанные к вашей <strong class="blue">организации</strong>. Вы можете использовать этих сотрудников при подаче заявок на пропуск.
                    </p>
                    <p class="help__text">
                        Новые сотрудники попадают в этот список <strong class="blue">автоматически</strong>, при подаче заявки.
                    </p>
                </div>
            </div>
        </div>

        <EmployeeEditModal
            :visible="showModal"
            :editingEmployee="editingEmployee"
            :citizenships="availableCitizenships"
            :ownershipInfo="ownershipInfo"
            @saved="onEmployeeSaved"
            @close="closeModal"
        />
    </section>
</template>

<script>
import { apiRequest } from '@/api/client'
import SearchComponent from '@/components/SearchComponent.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import SkeletonTransition from '@/components/ui/SkeletonTransition.vue';
import SkeletonTable from '@/components/ui/SkeletonTable.vue';
import EmployeeEditModal from '@/components/EmployeeEditModal.vue';

export default {
    components: {
        SearchComponent,
        RefreshButton,
        SkeletonTransition,
        SkeletonTable,
        EmployeeEditModal
    },
    data() {
        return {
            loading: true,
            searchQuery: '',
            sortField: null,
            sortDirection: 'desc',
            employeesData: [],
            searchTimeout: null,
            currentFilter: 'user',
            ownershipInfo: null,
            showModal: false,
            availableCitizenships: [],
            editingEmployee: null
        };
    },
    computed: {
        filteredEmployees() {
            if (!this.searchQuery.trim()) {
                return this.employeesData;
            }
            
            const query = this.searchQuery.toLowerCase().trim();
            return this.employeesData.filter(employee => {
                const fullName = this.formatFullName(employee).toLowerCase();
                return fullName.includes(query) ||
                       (employee.position && employee.position.toLowerCase().includes(query)) ||
                       (employee.status ? 'активен' : 'неактивен').includes(query)
            });
        },

        sortedEmployees() {
            const employees = [...this.filteredEmployees];
            
            if (!this.sortField) {
                return employees;
            }
            
            return employees.sort((a, b) => {
                let valueA, valueB;
                
                switch (this.sortField) {
                    case 'id':
                        valueA = a.id;
                        valueB = b.id;
                        break;
                        
                    case 'last_name':
                        valueA = a.last_name?.toLowerCase() || '';
                        valueB = b.last_name?.toLowerCase() || '';
                        break;
                        
                    case 'position':
                        valueA = a.position?.toLowerCase() || '';
                        valueB = b.position?.toLowerCase() || '';
                        break;
                        
                    case 'status':
                        valueA = a.status;
                        valueB = b.status;
                        break;
                        
                    default:
                        return 0;
                }
                
                if (valueA < valueB) {
                    return this.sortDirection === 'asc' ? -1 : 1;
                }
                if (valueA > valueB) {
                    return this.sortDirection === 'asc' ? 1 : -1;
                }
                return 0;
            });
        },

        hasActiveFilters() {
            return !!this.searchQuery.trim();
        }
    },
    watch: {
        searchQuery() {
            clearTimeout(this.searchTimeout);
            this.searchTimeout = setTimeout(() => {
                this.$forceUpdate();
            }, 50);
        }
    },
    methods: {
        async fetchEmployees() {
            try {
                const response = await apiRequest(`/unique-employees?filter_type=${this.currentFilter}`, {
                    method: "GET"});

                if (response.ok) {
                    this.employeesData = await response.json();
                } else {
                    console.error("Ошибка при загрузке сотрудников");
                    this.employeesData = [];
                }
            } catch (error) {
                console.error("Ошибка при загрузке сотрудников:", error);
                this.employeesData = [];
            } finally {
                this.loading = false;
            }
        },

        async fetchOwnershipInfo() {
            try {
                const response = await apiRequest("/unique-employees/ownership-info", {
                    method: "GET"});

                if (response.ok) {
                    this.ownershipInfo = await response.json();
                } else {
                    // Если эндпоинт не существует, используем эндпоинт для машин (они используют одну логику)
                    const carResponse = await apiRequest("/unique-cars/ownership-info", {
                        method: "GET"});
                    
                    if (carResponse.ok) {
                        this.ownershipInfo = await carResponse.json();
                    }
                }
            } catch (error) {
                console.error("Ошибка при загрузке информации о владельце:", error);
            }
        },

        async fetchCitizenships() {
            try {
                const response = await apiRequest("/citizenships", {
                    method: "GET"});

                if (response.ok) {
                    this.availableCitizenships = await response.json();
                }
            } catch (error) {
                console.error("Ошибка при загрузке гражданств:", error);
            }
        },

        async deleteEmployee(employee) {
            if (confirm(`Вы уверены, что хотите удалить сотрудника ${this.formatFullName(employee)}?`)) {
                try {
                    const response = await apiRequest(`/unique-employees/${employee.id}`, {
                        method: "DELETE"});

                    if (response.ok) {
                        await this.fetchEmployees();
                    } else {
                        alert("Ошибка при удалении сотрудника");
                    }
                } catch (error) {
                    console.error("Ошибка при удалении сотрудника:", error);
                    alert("Ошибка при удалении сотрудника");
                }
            }
        },

        editEmployee(employee) {
            this.editingEmployee = employee;
            this.showModal = true;
        },

        sortBy(field) {
            if (this.sortField === field) {
                this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
            } else {
                this.sortField = field;
                this.sortDirection = 'desc';
            }
        },

        switchFilter(filterType) {
            this.currentFilter = filterType;
            this.fetchEmployees();
        },

        showAddEmployeeModal() {
            this.editingEmployee = null;
            this.showModal = true;
        },

        closeModal() {
            this.showModal = false;
            this.editingEmployee = null;
        },

        onEmployeeSaved() {
            this.fetchEmployees();
        },

        // Форматирование ФИО
        formatFullName(employee) {
            const parts = [];
            if (employee.last_name) parts.push(employee.last_name);
            if (employee.first_name) parts.push(employee.first_name);
            if (employee.middle_name) parts.push(employee.middle_name);
            return parts.join(' ') || 'Не указано';
        },

        // Обрезка текста с добавлением точек
        truncateText(text, maxLength) {
            if (!text) return '';
            if (text.length <= maxLength) return text;
            return text.substring(0, maxLength) + '...';
        },

    },
    async mounted() {
        await Promise.all([
            this.fetchOwnershipInfo(),
            this.fetchCitizenships()
        ]);
        await this.fetchEmployees();
    }
}
</script>

<style scoped>
.employeesview {
    padding: 20px;
}

.employeesview__container {
    display: flex;
    gap: 30px;
    margin-top: 20px;
}

.employeesview__right-side {
    width: 40%;
}

.employeesview__header {
    padding-bottom: 15px;
    display: flex;
    align-items: center;
    gap: 10px;
    height: 25px;
}

.employeesview__title {
    font-size: 18px;
}

.employeesview__filters {
    padding-bottom: 15px;
    width: 100%;
    border-bottom: 1px solid #e6e6e6;
}

.filters-container {
    display: flex;
    gap: 15px;
    align-items: center;
}

.filter-tabs {
    display: flex;
    gap: 10px;
}

.filter-tab {
    padding: 0px 16px;
    border: 1px solid #e6e6e6;
    background: white;
    border-radius: 50px;
    cursor: pointer;
    font-size: 14px;
    transition: all 0.2s;
    height: 30px;
}

.filter-tab:hover {
    border-color: #4F5BDF;
}

.filter-tab--active {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.blue {
    color: #4F5BDF;
}

/* Стили для таблицы */
.employees-card {
    background-color: #fff;
    border-radius: 30px;
    border: 1px solid #e6e6e6;
    overflow: hidden;
    width: 60%;
    height: 450px;
    box-shadow: 0 3px 10px rgba(0,0,0,0.05);
}

.card-header {
    border-bottom: 1px solid #e6e6e6;
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0px 20px;
    height: 40px;
}

.card-header__title {
    display: flex;
    gap: 8px;
    align-items: center;
}

.card-header__settings {
    display: flex;
    gap: 8px;
    align-items: center;
}

.add-button {
    background: #4F5BDF;
    color: white;
    border: none;
    border-radius: 15px;
    padding: 6px 12px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.add-button:hover:not(:disabled) {
    background: #3a45c0;
}

.add-button:disabled {
    background: #a2a2a2;
    cursor: not-allowed;
    opacity: 0.6;
}

.card-title {
    margin: 0;
    color: #000;
    font-weight: 600;
    font-size: 1.0em;
}

.highlight-text {
    color: #000;
}

.card-content {
    padding: 0;
    height: calc(100% - 40px);
    display: flex;
    flex-direction: column;
}

.employees-container {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow-y: auto;
}

/* Заголовок таблицы */
.employees-header {
    border-bottom: 1px solid #e6e6e6;
    padding: 12px 16px;
    flex-shrink: 0;
}

.header-row {
    display: flex;
    width: 100%;
    align-items: center;
}

.header-col {
    font-weight: 500;
    color: #a2a2a2;
    text-align: left;
    padding: 0 0px;
    font-size: 14px;
    display: flex;
    align-items: center;
    gap: 5px;
    transition: .2s;
    cursor: pointer;
    user-select: none;
}

.header-col:hover {
    color: #333;
}

.header-col:hover .sort-icon {
    filter: brightness(0);
}

.sort-icon {
    width: 12px;
    height: 12px;
    transition: .2s;
}

.sort-icon.sorted {
    filter: brightness(0);
}

.sort-icon.desc {
    transform: rotate(180deg);
}

.active-sort {
    color: #333 !important;
    font-weight: 500 !important;
}

/* Колонки с фиксированной шириной */
.number-col {
    width: 8%;
    min-width: 40px;
}

.name-col {
    width: 35%;
    min-width: 200px;
}

.position-col {
    width: 25%;
    min-width: 150px;
}

.status-col {
    width: 17%;
    min-width: 100px;
}

.actions-col {
    width: 15%;
    min-width: 100px;
    justify-content: center;
}

/* Тело таблицы */
.employees-body {
    overflow-y: auto;
    flex-grow: 1;
    padding-right: 4px;
    margin-right: 4px;
    scroll-behavior: smooth;
}

.employee-item {
    transition: background-color 0.2s ease;
}

.employee-item:hover {
    background-color: #fafafa;
}

.employee-row {
    display: flex;
    width: 100%;
    padding: 10px 16px;
    align-items: center;
    border-bottom: 1px solid #f0f0f0;
}

.employee-col {
    padding: 0 8px;
    text-align: left;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 14px;
    display: flex;
    align-items: center;
    height: 100%;
}

/* Выравнивание содержимого колонок */
.number-col .employee-col,
.actions-col .employee-col {
    justify-content: center;
}

/* Стилизация скроллбара */
.employees-body::-webkit-scrollbar {
    width: 6px;
}

.employees-body::-webkit-scrollbar-track {
    background: transparent;
    margin: 2px 0;
    border-radius: 3px;
}

.employees-body::-webkit-scrollbar-thumb {
    background: #D9E2FF;
    border-radius: 3px;
    border: 1px solid transparent;
    background-clip: content-box;
    transition: all 0.3s ease;
}

.employees-body::-webkit-scrollbar-thumb:hover {
    background: #C5D1FF;
    border: 1px solid transparent;
    background-clip: content-box;
    transform: scale(1.1);
}

.employees-body {
    scrollbar-width: thin;
    scrollbar-color: #D9E2FF transparent;
    scroll-behavior: smooth;
    overscroll-behavior: contain;
}

/* Стили для бейджей статуса */
.status-badge {
    padding: 4px 12px;
    border-radius: 12px;
    font-size: 12px;
    font-weight: 500;
    display: inline-block;
}

.status-active {
    background-color: #f0f9ff;
    color: #0369a1;
    border: 1px solid #bae6fd
}

.status-inactive {
    background-color: #fef2f2;
    color: #991b1b;
    border: 1px solid #fecaca;
}

/* Кнопки действий */
.edit-btn, .delete-btn {
    background: none;
    border: none;
    cursor: pointer;
    padding: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
    transition: background-color 0.2s ease;
    margin: 0 2px;
}

.edit-btn:hover {
    background-color: #f5f5f5;
}

.delete-btn:hover {
    background-color: #f5f5f5;
}

.edit-icon, .delete-icon {
    width: 16px;
    height: 16px;
    opacity: 0.7;
    transition: opacity 0.2s ease;
}

.edit-btn:hover .edit-icon,
.delete-btn:hover .delete-icon {
    opacity: 1;
}

.no-data-message {
    text-align: center;
    color: #a2a2a2;
    padding: 40px 20px;
    margin: 0;
    font-size: 14px;
    flex-grow: 1;
    display: flex;
    align-items: center;
    justify-content: center;
}

.help__text {
    line-height: 150%; font-size: 14px;
}

@media (max-width: 768px) {
    .employees-card {
        width: 100%;
        height: auto;
    }

    .header-row,
    .employee-row {
        flex-wrap: wrap;
    }

    .header-col,
    .employee-col {
        width: 50% !important;
        margin-bottom: 4px;
    }

    .card-header {
        flex-direction: column;
        align-items: flex-start;
        gap: 12px;
        height: auto;
        padding: 16px;
    }

    .card-header__settings {
        width: 100%;
        justify-content: flex-end;
    }

    .employeesview__container {
        flex-direction: column;
    }

    .employeesview__right-side {
        width: 100%;
    }
}
</style>