<template>
    <div class="tables">
        <div class="tables__header">
            <h1 class="tables__title">Таблица <span class="table-name">КПП №4</span></h1>
            <button class="tables__instruction">
                <img src="@/assets/icons/instruction.png" class="tables__icon" />
                <p class="instruction__text">Инструкция</p>
            </button>
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
                <div class="field field--select" @click="toggleDropdown('organization')">
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
                <div class="field field--select" @click="toggleDropdown('unloading')">
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
                <!--<div class="field">
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
                </div>-->
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
            <div class="cars-fact">
                <CarsFact 
                    :search-query="searchQuery"
                    :selected-organization="selectedOrganization"
                    :selected-unloading-place="selectedUnloadingPlace"
                    :date-range-start="dateRangeStart"
                    :date-range-end="dateRangeEnd"
                    :selected-date="selectedDate"
                    @refresh-cars="refreshData"
                />
                <div class="cars-fact__info">
                    <h3>При прибытии автомобиля ПО ФАКТУ:</h3>
                    <ul class="cars-fact__steps">
                        <li class="steps__item">
                            спроси у водителя организацию
                        </li>
                        <li class="steps__item">
                            посмотри, есть ли организация в таблице слева
                        </li>
                        <li class="steps__item">
                            если организация есть - пропустить
                        </li>
                    </ul>
                    <p class="cars-fact__text">По вопросам обращаться в Бюро пропусков:</p>
                    <p class="cars-fact__number">+7 (910) 083 00-55</p>
                </div>
            </div>
            <SelectedTable 
                :search-query="searchQuery"
                :selected-organization="selectedOrganization"
                :selected-unloading-place="selectedUnloadingPlace"
                :date-range-start="dateRangeStart"
                :date-range-end="dateRangeEnd"
                :selected-date="selectedDate"
            />
            
        </div>
    </div>
</template>

<script>
import RefreshButton from './RefreshButton.vue';
// import DatePicker from './DatePicker.vue';
import CarsFact from './CarsFact.vue';
import SelectedTable from './SelectedTable.vue';

export default {
    components: {
        RefreshButton,
        // DatePicker,
        CarsFact,
        SelectedTable
    },
    data() {
        return {
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
        }
    },
    methods: {
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
                // Сбрасываем поиск при открытии
                if (this.showOrganizationDropdown) {
                    this.organizationSearch = '';
                }
            } else if (type === 'unloading') {
                this.showUnloadingDropdown = !this.showUnloadingDropdown;
                this.showOrganizationDropdown = false;
                this.showDatePicker = false;
                // Сбрасываем поиск при открытии
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
            // Эмитим событие для обновления данных в дочерних компонентах
            this.$emit('refresh-data');
        }
    },
    mounted() {
        document.addEventListener('click', (e) => {
            if (!this.$el.contains(e.target)) {
                this.showOrganizationDropdown = false;
                this.showUnloadingDropdown = false;
                this.showDatePicker = false;
            }
        });
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

.tables__instruction:hover {
    background-color: #f2f2f2;
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
}

.cars-fact {
    display: flex;
    gap: 20px;
    padding-bottom: 20px;
}

.cars-fact__info {
    width: 35%;
    height: 205px;
    background-color: #4F5BDF;
    border-radius: 30px;
    padding: 20px;
}

.cars-fact__info h3 {
    color: #FFF;
    font-size: 16px;
    font-weight: 900;
}

.cars-fact__steps {
    display: flex;
    flex-direction: column;
    gap: 5px;
    padding: 15px 15px;
}

.steps__item {
    color: #FFF;
    font-weight: 300;
}

.cars-fact__text {
    color: #FFF;
    font-weight: 400;
    font-size: 12px;
}

.cars-fact__number {
    color: #FFF;
    font-weight: semibold;
}
</style>