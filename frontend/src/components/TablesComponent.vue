<template>
    <div class="tables">
        <div class="tables__header">
            <h1 class="tables__title">Таблица <span class="table-name">{{ tableDisplayName }}</span></h1>
            <button class="tables__instruction" @click="showInstruction = true">
                <img src="@/assets/icons/instruction.png" class="tables__icon" />
                <p class="instruction__text">Инструкция</p>
            </button>
        </div>
        
        <!-- Модальное окно с инструкцией -->
        <div v-if="showInstruction" class="modal-overlay" @click.self="showInstruction = false">
            <div class="instruction-modal-large">
                <div class="modal-header">
                    <h3>Инструкция по использованию таблицы <span class="blue">{{ tableDisplayName }}</span></h3>
                    <button @click="showInstruction = false" class="modal-close">×</button>
                </div>
                <div class="instruction-content">
                    <div v-if="tableInstruction" class="text-constructor-content" v-html="sanitizedInstruction"></div>
                    <div v-else class="no-instruction">
                        <div class="no-instruction-icon">📝</div>
                        <h4>Инструкция не добавлена</h4>
                        <p>Для этой таблицы пока не создана и не написана инструкция.</p>
                        <p>Обратитесь к Бюро пропусков для добавления инструкции.</p>
                    </div>
                </div>
            </div>
        </div>

        <div class="tables__filters">
            <div class="filters__fields">
                <!-- Поиск -->
                <div class="field search">
                    <input 
                        placeholder="Поиск.." 
                        type="text" 
                        class="field__input search" 
                        v-model="searchQuery"
                        @input="applyFilters"
                    />
                    <img src="@/assets/icons/search.png" class="tables__icon" />
                </div>
                
                <!-- Организация через компонент -->
                <OrganizationFilter
                    v-if="showOrganizationFilter"
                    ref="organizationFilter"
                    v-model="selectedOrganizationId"
                    :organizations="organizations"
                    @change="handleOrganizationChange"
                />

                <!-- Место разгрузки через компонент -->
                <UnloadingPlaceFilter
                    v-if="showUnloadingFilter"
                    ref="unloadingPlaceFilter"
                    v-model="selectedUnloadingPlaceId"
                    @change="handleUnloadingPlaceChange"
                />

                <!-- Новый DateFilter -->
                <DateFilter
                    ref="dateFilter"
                    :mode="'range'"
                    :selected-date="selectedDate"
                    :date-range-start="dateRangeStart"
                    :date-range-end="dateRangeEnd"
                    @update:selectedDate="updateSelectedDate"
                    @update:dateRangeStart="updateDateRangeStart"
                    @update:dateRangeEnd="updateDateRangeEnd"
                    @apply="applyDateFilters"
                    @clear="clearDate"
                />
            </div>
            <div class="filters__options">
                <img src="@/assets/icons/trashcan.png" class="options__icon" @click="clearFilters" />
                <img src="@/assets/icons/recent-changes.png" class="options__icon" />
                <button class="options__export">
                    <img src="@/assets/icons/export.png" class="tables__icon" />
                    <p class="options__text">Экспорт</p>
                </button>
                <RefreshButton @refresh="refreshData" />
            </div>
        </div>

        <div class="tables__content">
            <!-- Таблица по факту с подсказкой -->
            <div v-if="showFactTable" class="fact-section">
                <FactTable 
                    v-if="currentUserId" 
                    :table-type="tableType"
                    :search-query="searchQuery"
                    :selected-organization="selectedOrganizationName"
                    :selected-unloading-place="selectedUnloadingPlaceName"
                    :date-range-start="dateRangeStart"
                    :date-range-end="dateRangeEnd"
                    :selected-date="selectedDate"
                    :current-user-id="currentUserId"
                    :current-user-name="currentUserName"
                    @refresh-data="refreshData"
                />
                <!-- Подсказка на синем фоне -->
                <div class="fact-hint-card" v-if="tableFactHint">
                    <div class="text-constructor-content hint-content" v-html="sanitizedHint"></div>
                </div>
            </div>
            
            <!-- Основная таблица - разные компоненты для разных типов -->
            <CarsTable
                v-if="tableType === 'cars' && currentUserId"
                :table-name="tableSystemName"
                :table-id="tableData?.table?.id"
                :search-query="searchQuery"
                :selected-organization-id="selectedOrganizationId"
                :selected-unloading-place-id="selectedUnloadingPlaceId"
                :date-range-start="dateRangeStart"
                :date-range-end="dateRangeEnd"
                :selected-date="selectedDate"
                :current-user-id="currentUserId"
                :current-user-name="currentUserName"
                @refresh-data="refreshData"
                @open-application="handleOpenApplication"
            />
            
            <PeopleTable
                v-if="tableType === 'people'"
                :table-name="tableSystemName"
                :search-query="searchQuery"
                :selected-organization-id="selectedOrganizationId"
                :selected-unloading-place-id="selectedUnloadingPlaceId"
                :date-range-start="dateRangeStart"
                :date-range-end="dateRangeEnd"
                :selected-date="selectedDate"
                :current-user-id="currentUserId"
                :current-user-name="currentUserName"
                @refresh-data="refreshData"
                @open-application="handleOpenApplication"
            />
        </div>

        <ApplicationDetail
            v-if="showApplicationDetail"
            :application="selectedApplication"
            :current-user-id="currentUserId"
            :current-user-name="currentUserName"
            :mode="'center'"
            @close="closeApplicationDetail"
            @application-changed="handleApplicationChanged"
        />
    </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { sanitizeHtml } from '@/utils/sanitize';
import OrganizationFilter from '@/components/OrganizationFilter.vue';
import UnloadingPlaceFilter from '@/components/UnloadingPlaceFilter.vue';
import RefreshButton from './RefreshButton.vue';
import DateFilter from './DateFilter.vue';
import FactTable from './FactTable.vue';
import CarsTable from './CarsTable.vue';
import PeopleTable from './PeopleTable.vue';
import ApplicationDetail from './ApplicationDetail/ApplicationDetail.vue';

export default {
    name: 'TablesComponent',
    components: {
        OrganizationFilter,
        UnloadingPlaceFilter,
        RefreshButton,
        DateFilter,
        FactTable,
        CarsTable,
        PeopleTable,
        ApplicationDetail
    },
    data() {
        return {
            tableData: null,
            searchQuery: '',
            selectedOrganizationId: null,
            selectedOrganizationName: '',
            selectedUnloadingPlaceId: null,
            selectedUnloadingPlaceName: '',
            
            organizations: [],
            
            showInstruction: false,
            selectedDate: null,
            dateRangeStart: null,
            dateRangeEnd: null,
            
            currentUserId: null,
            currentUserName: '',

            showApplicationDetail: false,
            selectedApplication: null
        };
    },
    computed: {
        tableSystemName() {
            return this.tableData?.table?.name || '';
        },
        
        tableDisplayName() {
            return this.tableData?.table?.display_name || 'Загрузка...';
        },
        
        tableType() {
            return this.tableData?.table?.table_type || '';
        },
        
        showFactTable() {
            return this.tableData?.table?.show_fact_table || false;
        },
        
        tableFactHint() {
            return this.tableData?.table?.fact_table_hint || '';
        },
        
        tableInstruction() {
            return this.tableData?.table?.instruction || '';
        },
        
        showOrganizationFilter() {
            return this.tableType === 'cars' || this.tableType === 'people';
        },
        
        showUnloadingFilter() {
            return this.tableType === 'cars';
        },
        
        sanitizedHint() {
            return this.sanitizeHtml(this.tableFactHint);
        },
        
        sanitizedInstruction() {
            return this.sanitizeHtml(this.tableInstruction);
        },
        
        hasActiveFilters() {
            return !!this.searchQuery.trim() || 
                   !!this.selectedOrganizationId || 
                   !!this.selectedUnloadingPlaceId ||
                   !!this.selectedDate ||
                   (this.dateRangeStart && this.dateRangeEnd);
        }
    },
    methods: {
        handleApplicationUpdate(updatedApp) {
            console.log('Application updated:', updatedApp);
            this.refreshData();
        },

        async fetchCurrentUser() {
            try {
                const response = await apiRequest("/users/me", {
                });

                if (response.ok) {
                    const userData = await response.json();
                    this.currentUserId = userData.id;

                    const nameParts = [userData.last_name, userData.first_name, userData.middle_name].filter(Boolean);
                    this.currentUserName = nameParts.join(' ') || userData.username;
                }
            } catch (error) {
                console.error("Ошибка при загрузке данных пользователя:", error);
            }
        },

        refreshData() {
            this.fetchTableData();
            this.$emit('refresh-data');
        },
        
        sanitizeHtml(content) {
            return sanitizeHtml(content);
        },

        async fetchTableData() {
            const tableName = this.$route.params.tableName;
            if (!tableName) return;
            
            try {
                const response = await apiRequest(`/system-tables/name/${tableName}`, {
                });
                
                if (response.ok) {
                    const data = await response.json();
                    console.log('Table data received:', data);
                    this.tableData = data;
                    
                    await this.fetchOrganizationsForTable();
                } else {
                    console.error('Table not found');
                    this.$router.push('/404');
                }
            } catch (error) {
                console.error("Error fetching table data:", error);
            }
        },

        async fetchOrganizationsForTable() {
            try {
                const response = await apiRequest("/organizations", {
                    method: "GET",
                });

                if (response.ok) {
                    const data = await response.json();
                    console.log('Organizations loaded:', data);
                    this.organizations = data;
                } else {
                    console.error("Ошибка при загрузке организаций");
                }
            } catch (error) {
                console.error("Ошибка сети при загрузке организаций:", error);
            }
        },

        handleOrganizationChange({ id, name }) {
            this.selectedOrganizationId = id;
            this.selectedOrganizationName = name;
            this.applyFilters();
        },
        
        handleUnloadingPlaceChange({ id, name }) {
            this.selectedUnloadingPlaceId = id;
            this.selectedUnloadingPlaceName = name;
            this.applyFilters();
        },
        
        updateSelectedDate(date) {
            this.selectedDate = date;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
        },
        
        updateDateRangeStart(date) {
            this.dateRangeStart = date;
            this.selectedDate = null;
        },
        
        updateDateRangeEnd(date) {
            this.dateRangeEnd = date;
            this.selectedDate = null;
        },
        
        applyDateFilters() {
            this.applyFilters();
        },
        
        clearDate() {
            this.selectedDate = null;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            this.applyFilters();
        },
        
        applyFilters() {
            // Фильтры применяются автоматически через props в дочерних компонентах
        },
        
        clearFilters() {
            this.searchQuery = '';
            
            this.selectedDate = null;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            
            if (this.$refs.organizationFilter && this.$refs.organizationFilter.reset) {
                this.$refs.organizationFilter.reset();
            }
            
            if (this.$refs.unloadingPlaceFilter && this.$refs.unloadingPlaceFilter.reset) {
                this.$refs.unloadingPlaceFilter.reset();
            }
            
            if (this.$refs.dateFilter && this.$refs.dateFilter.clearSelection) {
                this.$refs.dateFilter.clearSelection();
            }
        },

        async handleOpenApplication(applicationId) {
            try {
                const response = await apiRequest(`/applications/${applicationId}/details`, {});
                if (response.ok) {
                    const appData = await response.json();
                    this.selectedApplication = appData;
                    this.showApplicationDetail = true;
                } else {
                    console.error('Не удалось загрузить заявку');
                }
            } catch (error) {
                console.error('Ошибка загрузки заявки:', error);
            }
        },

        closeApplicationDetail() {
            this.showApplicationDetail = false;
            this.selectedApplication = null;
        },

        handleApplicationChanged() {
            this.refreshData();
        }
    },
    async mounted() {
        await this.fetchCurrentUser();  // Ждём загрузки пользователя
        this.fetchTableData();          // потом таблицы
    },
    watch: {
        '$route.params.tableName': {
            handler() {
                this.fetchTableData();
                this.clearFilters();
            },
            immediate: true
        }
    }
}
</script>

<style scoped>
/* Все стили остаются без изменений */
.tables {
    padding: 20px;
    position: relative;
}

.tables__title {
    font-size: 18px;
    font-weight: bold;
    color: #000;
}

.table-name {
    color: #4F5BDF;
}

.tables__header {
    display: flex;
    gap: 10px;
    padding-bottom: 15px;
}

.tables__instruction {
    width: fit-content;
    font-size: 14px;
    font-weight: 500;
    color: #4F5BDF;
    padding: 0 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 5px;
    border-radius: 50px;
    background: #FFF;
    border: 1px solid #e6e6e6;
    outline: none;
    cursor: pointer;
    height: 25px;
    transition: all 0.2s ease;
}

.tables__instruction:hover {
    background-color: #f2f2f2;
    border-color: #4F5BDF;
}

.tables__icon {
    width: 15px;
    height: 15px;
}

.tables__filters {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 15px;
    border-bottom: 1px solid #e6e6e6;
}

.filters__fields {
    display: flex;
    align-items: center;
    gap: 10px;
    position: relative;
}

.field {
    width: 200px;
    height: 35px;
    background-color: #FFF;
    border-radius: 10px;
    border: 1px solid #e6e6e6;
    padding: 0 10px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    position: relative;
    cursor: pointer;
}

.field--select {
    cursor: pointer;
}

.field__input {
    outline: none;
    border: none;
    background-color: transparent;
    font-size: 14px;
    width: 150px;
    cursor: pointer;
}

.select-text {
    font-size: 14px;
    color: #000;
    flex: 1;
}

.search {
    cursor: text;
}

.filters__options {
    display: flex;
    align-items: center;
    gap: 15px;
}

.options__icon {
    width: 20px;
    height: 20px;
    cursor: pointer;
}

.options__export {
    width: 100px;
    height: 25px;
    background: #FFF;
    border: 1px solid #e6e6e6;
    outline: none;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 10px;
    gap: 5px;
}

.options__export:hover {
    background: #f2f2f2;
}

.options__text {
    font-weight: 500;
}

.tables__content {
    margin-top: 15px;
    display: flex;
    flex-direction: column;
    gap: 20px;
}

/* Fact Section with Hint */
.fact-section {
    display: flex;
    gap: 20px;
}

.fact-hint-card {
    flex: 0 0 35%;
    background-color: #4F5BDF;
    border-radius: 30px;
    padding: 20px;
    display: flex;
    gap: 15px;
    align-items: flex-start;
    min-height: 205px;
    box-shadow: 0 3px 10px rgba(79, 91, 223, 0.2);
}

.hint-content {
    flex: 1;
    color: #FFF;
}

.reset-filters-btn {
    padding: 6px 12px;
    border: 1px solid #e6e6e6;
    background: #fff5f5;
    border-radius: 15px;
    cursor: pointer;
    font-size: 12px;
    transition: all 0.2s;
    height: 35px;
    color: #c53030;
    white-space: nowrap;
    display: flex;
    align-items: center;
    gap: 5px;
    border-color: #fed7d7;
}

.reset-filters-btn:hover:not(:disabled) {
    background: #fed7d7;
}

.reset-filters-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
    background: #f5f5f5;
    color: #999;
    border-color: #e6e6e6;
}

.reset-filters-btn:disabled .options__icon {
    filter: grayscale(100%);
}

/* Text Constructor Content Styles */
.text-constructor-content :deep(*) {
    line-height: 150%;
}

.text-constructor-content :deep(strong) {
    font-weight: 600 !important;
}

.text-constructor-content :deep(em) {
    font-style: italic !important;
}

.text-constructor-content :deep(u) {
    text-decoration: underline !important;
}

.text-constructor-content :deep(ul),
.text-constructor-content :deep(ol) {
    padding-left: 20px !important;
}

.text-constructor-content :deep(li) {
    line-height: 150% !important;
}

.text-constructor-content :deep(h1) {
    font-size: 20px !important;
    font-weight: 700 !important;
    margin: 0 0 12px 0 !important;
    line-height: 1.2 !important;
}

.text-constructor-content :deep(h2) {
    font-size: 18px !important;
    font-weight: 600 !important;
    margin: 0 0 10px 0 !important;
    line-height: 1.3 !important;
}

.text-constructor-content :deep(h3) {
    font-size: 16px !important;
    font-weight: 500 !important;
    margin: 0 0 8px 0 !important;
    line-height: 1.4 !important;
}

.text-constructor-content :deep(.black-text) { color: #000 !important; }
.text-constructor-content :deep(.red-text) { color: #FF0000 !important; }
.text-constructor-content :deep(.green-text) { color: #079D1D !important; }
.text-constructor-content :deep(.blue-text) { color: #4F5BDF !important; }

.text-constructor-content :deep(.font-size-10) { font-size: 10px !important; }
.text-constructor-content :deep(.font-size-12) { font-size: 12px !important; }
.text-constructor-content :deep(.font-size-14) { font-size: 14px !important; }
.text-constructor-content :deep(.font-size-16) { font-size: 16px !important; }
.text-constructor-content :deep(.font-size-18) { font-size: 18px !important; }
.text-constructor-content :deep(.font-size-20) { font-size: 20px !important; }

.text-constructor-content :deep(.font-weight-300) { font-weight: 300 !important; }
.text-constructor-content :deep(.font-weight-400) { font-weight: 400 !important; }
.text-constructor-content :deep(.font-weight-500) { font-weight: 500 !important; }
.text-constructor-content :deep(.font-weight-600) { font-weight: 600 !important; }
.text-constructor-content :deep(.font-weight-900) { font-weight: 900 !important; }

.text-constructor-content :deep(.heading-h1) { 
    font-size: 24px !important; 
    font-weight: 700 !important;
    margin: 0 0 8px 0 !important;
    line-height: 1.2 !important;
}

.text-constructor-content :deep(.heading-h2) { 
    font-size: 20px !important; 
    font-weight: 600 !important;
    margin: 8px 0 6px 0 !important;
    line-height: 1.3 !important;
}

/* Hint specific styles */
.hint-content :deep(*) {
    color: #FFF !important;
}

.hint-content :deep(.black-text) { color: #FFF !important; }
.hint-content :deep(.red-text) { color: #FFB3B3 !important; }
.hint-content :deep(.green-text) { color: #B3FFC6 !important; }
.hint-content :deep(.blue-text) { color: #B3C6FF !important; }

/* Instruction Modal */
.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.05);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 20px;
}

.instruction-modal-large {
    background: #fff;
    border-radius: 16px;
    padding: 0;
    max-width: 700px;
    width: 100%;
    max-height: 80vh;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
    display: flex;
    flex-direction: column;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px;
    border-bottom: 1px solid #e6e6e6;
}

.modal-header h3 {
    margin: 0;
    font-size: 1.2em;
    font-weight: 600;
    color: #000;
}

.modal-close {
    background: none;
    border: none;
    font-size: 20px;
    cursor: pointer;
    color: #999;
    padding: 0;
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: color 0.2s ease;
}

.modal-close:hover {
    color: #000;
}

.instruction-content {
    padding: 20px;
    overflow-y: auto;
    flex: 1;
}

.instruction-content .text-constructor-content :deep(h1) {
    color: #000 !important;
}

.instruction-content .text-constructor-content :deep(h2) {
    color: #000 !important;
}

.instruction-content .text-constructor-content :deep(h3) {
    color: #000 !important;
}

.instruction-content .text-constructor-content :deep(p) {
    color: #000 !important;
}

.instruction-content .text-constructor-content :deep(li) {
    color: #000 !important;
}

.no-instruction {
    text-align: center;
    padding: 40px 20px;
    color: #666;
}

.no-instruction-icon {
    font-size: 48px;
    margin-bottom: 16px;
    opacity: 0.5;
}

.no-instruction h4 {
    margin: 0 0 12px 0;
    color: #000;
    font-size: 1.2em;
}

.no-instruction p {
    margin: 0 0 8px 0;
    line-height: 1.5;
}

.blue {
    color: #4F5BDF;
}

@media (max-width: 768px) {
    .fact-section {
        flex-direction: column;
    }
    
    .fact-hint-card {
        flex: none;
        width: 100%;
    }
    
    .tables__header {
        flex-direction: column;
        align-items: flex-start;
        gap: 10px;
    }
    
    .tables__filters {
        flex-direction: column;
        gap: 15px;
        align-items: flex-start;
    }
    
    .filters__fields {
        flex-wrap: wrap;
    }
    
    .field {
        width: 100%;
    }
    
    .instruction-modal-large {
        margin: 10px;
        max-height: 90vh;
    }
}
</style>