<template>
    <div class="tables">
        <div class="tables__header">
            <h1 class="tables__title">Таблица <span class="table-name">{{ currentTable?.display_name }}</span></h1>
            <button class="tables__instruction" @click="showInstruction = true">
                <img src="@/assets/icons/instruction.png" class="tables__icon" />
                <p class="instruction__text">Инструкция</p>
            </button>
        </div>
        
        <!-- Модальное окно с инструкцией -->
        <div v-if="showInstruction" class="modal-overlay" @click.self="showInstruction = false">
            <div class="instruction-modal-large">
                <div class="modal-header">
                    <h3>Инструкция по использованию таблицы <span class="blue">{{ currentTable?.display_name }}</span></h3>
                    <button @click="showInstruction = false" class="modal-close">×</button>
                </div>
                <div class="instruction-content">
                    <div v-if="currentTable?.instruction" class="text-constructor-content" v-html="sanitizedInstruction"></div>
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
                
                <!-- Организация -->
                <div class="field field--select" @click="toggleDropdown('organization')" v-if="showOrganizationFilter">
                    <select class="field__input field__select" v-model="selectedOrganization" style="display: none;">
                        <option value="">Организация</option>
                        <option v-for="org in organizations" :key="org" :value="org">{{ org }}</option>
                    </select>
                    <span class="select-text">{{ selectedOrganization || 'Организация' }}</span>
                    <img src="@/assets/icons/arrow.png" class="select-icon" :class="{ 'select-icon--rotated': showOrganizationDropdown }" />
                    <div class="custom-dropdown" v-if="showOrganizationDropdown">
                        <div class="dropdown-search">
                            <input 
                                type="text" 
                                placeholder="Поиск..." 
                                v-model="organizationSearch"
                                @click.stop
                                class="dropdown-search__input"
                            />
                        </div>
                        <div class="dropdown-item" @click.stop="selectOrganization('')">Все организации</div>
                        <div 
                            class="dropdown-item" 
                            v-for="org in filteredOrganizations" 
                            :key="org" 
                            @click.stop="selectOrganization(org)"
                            :class="{ 'dropdown-item--selected': org === selectedOrganization }"
                        >
                            {{ org }}
                        </div>
                        <div class="dropdown-no-results" v-if="filteredOrganizations.length === 0">
                            Организации не найдены
                        </div>
                    </div>
                </div>

                <!-- Место разгрузки -->
                <div class="field field--select" @click="toggleDropdown('unloading')" v-if="showUnloadingFilter">
                    <select class="field__input field__select" v-model="selectedUnloadingPlace" style="display: none;">
                        <option value="">Место разгрузки</option>
                        <option v-for="place in unloadingPlaces" :key="place" :value="place">{{ place }}</option>
                    </select>
                    <span class="select-text">{{ selectedUnloadingPlace || 'Место разгрузки' }}</span>
                    <img src="@/assets/icons/arrow.png" class="select-icon" :class="{ 'select-icon--rotated': showUnloadingDropdown }" />
                    <div class="custom-dropdown" v-if="showUnloadingDropdown">
                        <div class="dropdown-search">
                            <input 
                                type="text" 
                                placeholder="Поиск..." 
                                v-model="unloadingSearch"
                                @click.stop
                                class="dropdown-search__input"
                            />
                        </div>
                        <div class="dropdown-item" @click.stop="selectUnloadingPlace('')">Все места</div>
                        <div 
                            class="dropdown-item" 
                            v-for="place in filteredUnloadingPlaces" 
                            :key="place" 
                            @click.stop="selectUnloadingPlace(place)"
                            :class="{ 'dropdown-item--selected': place === selectedUnloadingPlace }"
                        >
                            {{ place }}
                        </div>
                        <div class="dropdown-no-results" v-if="filteredUnloadingPlaces.length === 0">
                            Места разгрузки не найдены
                        </div>
                    </div>
                </div>

                <!-- Дата -->
                <div class="field">
                    <input 
                        placeholder="Выберите дату" 
                        type="text" 
                        class="field__input" 
                        @click="toggleDatePicker"
                        :value="dateRangeText"
                        readonly
                    />
                    <img src="@/assets/icons/calendar.png" class="tables__icon" />
                    <DatePicker 
                        v-if="showDatePicker"
                        :selectedDate="selectedDate"
                        :dateRangeStart="dateRangeStart"
                        :dateRangeEnd="dateRangeEnd"
                        @update:selectedDate="updateSelectedDate"
                        @update:dateRange="updateDateRange"
                        @apply="applyDateRange"
                        @clear="clearDate"
                    />
                </div>
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
            <div v-if="currentTable?.show_fact_table" class="fact-section">
                <FactTable 
                    :table-type="currentTable?.table_type"
                    :search-query="searchQuery"
                    :selected-organization="selectedOrganization"
                    :selected-unloading-place="selectedUnloadingPlace"
                    :date-range-start="dateRangeStart"
                    :date-range-end="dateRangeEnd"
                    :selected-date="selectedDate"
                    @refresh-data="refreshData"
                />
                <!-- Подсказка на синем фоне -->
                <div class="fact-hint-card" v-if="currentTable?.fact_table_hint">
                    <div class="text-constructor-content hint-content" v-html="sanitizedHint"></div>
                </div>
            </div>
            
            <!-- Основная таблица -->
            <SelectedTable 
    :table-type="currentTable?.table_type"  
    :table-name="currentTable?.name"     
    :search-query="searchQuery"
    :selected-organization="selectedOrganization"
    :selected-unloading-place="selectedUnloadingPlace"
    :date-range-start="dateRangeStart"
    :date-range-end="dateRangeEnd"
    :selected-date="selectedDate"
    @refresh-data="refreshData"
/>
        </div>
    </div>
</template>

<script>
import RefreshButton from './RefreshButton.vue';
import DatePicker from './DatePicker.vue';
import FactTable from './FactTable.vue';
import SelectedTable from './SelectedTable.vue';

export default {
    components: {
        RefreshButton,
        DatePicker,
        FactTable,
        SelectedTable
    },
    data() {
        return {
            currentTable: null,
            searchQuery: '',
            selectedOrganization: '',
            selectedUnloadingPlace: '',
            organizations: [
                'ООО "Ромашка"',
                'ИП Иванов',
                'ЗАО "Весна"',
                'ОАО "Технопром"',
                'ТОО "Стройсервис"',
                'ООО "Нефтегаз"',
                'ИП Петров',
                'ЗАО "Металлург"',
                'ОАО "Строймаш"',
                'ТОО "Транспорт"'
            ],
            unloadingPlaces: [
                'Дебаркадер №1',
                'Дебаркадер №2',
                'Дебаркадер №3',
                'Дебаркадер №4',
                'Дебаркадер №5',
                'Пост №21',
                'Пост №27',
                'Пост Север',
                'Ворота Сочи',
                'Терминал А',
                'Терминал Б',
                'Склад №1',
                'Склад №2'
            ],
            showOrganizationDropdown: false,
            showUnloadingDropdown: false,
            showDatePicker: false,
            showInstruction: false,
            selectedDate: null,
            dateRangeStart: null,
            dateRangeEnd: null,
            organizationSearch: '',
            unloadingSearch: ''
        }
    },
    computed: {
        dateRangeText() {
            if (this.dateRangeStart && this.dateRangeEnd) {
                return `${this.formatDate(this.dateRangeStart)} - ${this.formatDate(this.dateRangeEnd)}`;
            } else if (this.selectedDate) {
                return this.formatDate(this.selectedDate);
            }
            return 'Выберите дату';
        },
        filteredOrganizations() {
            if (!this.organizationSearch) {
                return this.organizations;
            }
            const searchTerm = this.organizationSearch.toLowerCase();
            return this.organizations.filter(org => 
                org.toLowerCase().includes(searchTerm)
            );
        },
        filteredUnloadingPlaces() {
            if (!this.unloadingSearch) {
                return this.unloadingPlaces;
            }
            const searchTerm = this.unloadingSearch.toLowerCase();
            return this.unloadingPlaces.filter(place => 
                place.toLowerCase().includes(searchTerm)
            );
        },
        showOrganizationFilter() {
            return this.currentTable?.table_type === 'cars' || this.currentTable?.table_type === 'people';
        },
        showUnloadingFilter() {
            return this.currentTable?.table_type === 'cars';
        },
        sanitizedHint() {
            return this.sanitizeHtml(this.currentTable?.fact_table_hint || '');
        },
        sanitizedInstruction() {
            return this.sanitizeHtml(this.currentTable?.instruction || '');
        }
    },
    methods: {
        sanitizeHtml(content) {
            if (!content) return '';
            
            // Запрещенные теги для безопасности
            const forbiddenTags = [
                'script', 'style', 'link', 'meta', 'iframe', 'frame', 'frameset', 
                'object', 'embed', 'applet', 'form', 'input', 'button', 'select',
                'textarea', 'label', 'fieldset', 'legend', 'marquee', 'blink'
            ];
            
            let sanitizedContent = content;
            
            // Удаляем запрещенные теги
            forbiddenTags.forEach(tag => {
                const regex = new RegExp(`<${tag}[^>]*>.*?</${tag}>`, 'gis');
                sanitizedContent = sanitizedContent.replace(regex, '');
            });
            
            // Удаляем опасные атрибуты
            sanitizedContent = sanitizedContent.replace(/ on\w+="[^"]*"/gi, '');
            sanitizedContent = sanitizedContent.replace(/ javascript:/gi, '');
            sanitizedContent = sanitizedContent.replace(/ expression\(/gi, '');
            
            return sanitizedContent;
        },

        async fetchTableData() {
            const tableName = this.$route.params.tableName;
            if (!tableName) return;
            
            try {
                const token = localStorage.getItem("token");
                const response = await fetch(`http://localhost:8080/system-tables/name/${tableName}`, {
                    headers: {
                        "Authorization": `Bearer ${token}`,
                    },
                });
                if (response.ok) {
                    this.currentTable = await response.json();
                } else {
                    console.error('Table not found');
                    this.$router.push('/404');
                }
            } catch (error) {
                console.error("Error fetching table data:", error);
            }
        },
        
        formatDate(date) {
            if (!date) return '';
            return date.toLocaleDateString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric'
            });
        },
        
        // Dropdown methods
        toggleDropdown(type) {
            if (type === 'organization') {
                this.showOrganizationDropdown = !this.showOrganizationDropdown;
                this.showUnloadingDropdown = false;
                this.showDatePicker = false;
                if (this.showOrganizationDropdown) {
                    this.organizationSearch = '';
                }
            } else if (type === 'unloading') {
                this.showUnloadingDropdown = !this.showUnloadingDropdown;
                this.showOrganizationDropdown = false;
                this.showDatePicker = false;
                if (this.showUnloadingDropdown) {
                    this.unloadingSearch = '';
                }
            }
        },
        
        selectOrganization(org) {
            this.selectedOrganization = org;
            this.showOrganizationDropdown = false;
            this.organizationSearch = '';
            this.applyFilters();
        },
        
        selectUnloadingPlace(place) {
            this.selectedUnloadingPlace = place;
            this.showUnloadingDropdown = false;
            this.unloadingSearch = '';
            this.applyFilters();
        },
        
        // Date picker methods
        toggleDatePicker() {
            this.showDatePicker = !this.showDatePicker;
            this.showOrganizationDropdown = false;
            this.showUnloadingDropdown = false;
        },
        
        updateSelectedDate(date) {
            this.selectedDate = date;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            this.applyFilters();
        },
        
        updateDateRange(range) {
            this.dateRangeStart = range.start;
            this.dateRangeEnd = range.end;
            this.selectedDate = null;
            this.applyFilters();
        },
        
        clearDate() {
            this.selectedDate = null;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            this.applyFilters();
        },
        
        applyDateRange() {
            this.showDatePicker = false;
            this.applyFilters();
        },
        
        applyFilters() {
            // Фильтры применяются автоматически через props в дочерних компонентах
        },
        
        clearFilters() {
            this.searchQuery = '';
            this.selectedOrganization = '';
            this.selectedUnloadingPlace = '';
            this.selectedDate = null;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            this.organizationSearch = '';
            this.unloadingSearch = '';
        },
        
        refreshData() {
            this.fetchTableData();
            this.$emit('refresh-data');
        }
    },
    mounted() {
        this.fetchTableData();
        
        document.addEventListener('click', (e) => {
            if (!this.$el.contains(e.target)) {
                this.showOrganizationDropdown = false;
                this.showUnloadingDropdown = false;
                this.showDatePicker = false;
            }
        });
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

.select-icon {
    width: 10px;
    height: 10px;
    transition: transform 0.5s ease;
    transform: rotate(90deg);
}

.select-icon--rotated {
    transform: rotate(-90deg);
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

.field__select {
    cursor: pointer;
    appearance: none;
    width: 100%;
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
    width:100px;
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

.search {
    cursor: text;
}

/* Custom Dropdown */
.custom-dropdown {
    position: absolute;
    top: calc(100% + 5px);
    left: 0;
    width: 100%;
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    max-height: 300px;
    overflow-y: auto;
    z-index: 1001;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.dropdown-search {
    padding: 10px;
    border-bottom: 1px solid #f0f0f0;
    position: sticky;
    top: 0;
    background: white;
    z-index: 1002;
}

.dropdown-search__input {
    width: 100%;
    padding: 8px 12px;
    border: 1px solid #e6e6e6;
    border-radius: 5px;
    font-size: 14px;
    outline: none;
}

.dropdown-search__input:focus {
    border-color: #4F5BDF;
}

.dropdown-item {
    padding: 10px 15px;
    cursor: pointer;
    font-size: 14px;
    border-bottom: 1px solid #f0f0f0;
    transition: background-color 0.2s ease;
}

.dropdown-item:last-child {
    border-bottom: none;
}

.dropdown-item:hover {
    background-color: #f5f5f5;
}

.dropdown-item--selected {
    background-color: #4F5BDF;
    color: white;
}

.dropdown-item--selected:hover {
    background-color: #3a45c4;
}

.dropdown-no-results {
    padding: 15px;
    text-align: center;
    color: #999;
    font-size: 14px;
    font-style: italic;
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

.hint-icon {
    font-size: 24px;
    flex-shrink: 0;
    margin-top: 4px;
}

.hint-content {
    flex: 1;
    color: #FFF;
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