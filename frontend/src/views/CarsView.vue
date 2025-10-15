<template>
    <section class="carsview">
        <header class="carsview__header">
            <h2 class="carsview__title">
                Список <span class="blue">автомобилей</span>
            </h2>
        </header>
        
        <div class="carsview__filters">
            <SearchComponent
                :title="'Поиск машин...'"
                v-model="searchQuery"
            />
        </div>

        <div class="carsview__container">
        <!-- Таблица "Мои автомобили" -->
        <div class="cars-card">
            <div class="card-header">
                <div class="card-header__title">
                    <h3 class="card-title">Мои <span class="highlight-text">автомобили</span></h3>
                </div>
                <div class="card-header__settings">
                    <RefreshButton @refresh="fetchCars" />
                </div>
            </div>
            
            <div class="card-content">
                <!-- Заголовок таблицы всегда отображается -->
                <div class="cars-header">
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
                        <div class="header-col car-number-col" @click="sortBy('carNumber')">
                            <p :class="{ 'active-sort': sortField === 'carNumber' }">Номер</p>
                            <img 
                                src="@/assets/icons/sort.png" 
                                class="sort-icon" 
                                :class="{ 
                                    'sorted': sortField === 'carNumber',
                                    'desc': sortField === 'carNumber' && sortDirection === 'desc'
                                }" 
                            />
                        </div>
                        <div class="header-col brand-col" @click="sortBy('brand')">
                            <p :class="{ 'active-sort': sortField === 'brand' }">Марка</p>
                            <img 
                                src="@/assets/icons/sort.png" 
                                class="sort-icon" 
                                :class="{ 
                                    'sorted': sortField === 'brand',
                                    'desc': sortField === 'brand' && sortDirection === 'desc'
                                }" 
                            />
                        </div>
                        <div class="header-col format-col" @click="sortBy('numberFormat')">
                            <p :class="{ 'active-sort': sortField === 'numberFormat' }">Формат номера</p>
                            <img 
                                src="@/assets/icons/sort.png" 
                                class="sort-icon" 
                                :class="{ 
                                    'sorted': sortField === 'numberFormat',
                                    'desc': sortField === 'numberFormat' && sortDirection === 'desc'
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
                            <!-- Пустой заголовок для действий -->
                        </div>
                    </div>
                </div>
                
                <!-- Тело таблицы -->
                <div class="cars-container">
                    <div v-if="filteredCars.length > 0" class="cars-body">
                        <div 
                            v-for="(car) in sortedCars" 
                            :key="car.id" 
                            class="car-item"
                        >
                            <div class="car-row">
                                <div class="car-col number-col">
                                    {{ car.id }}
                                </div>
                                <div class="car-col car-number-col">
                                    {{ car.carNumber }}
                                </div>
                                <div class="car-col brand-col">
                                    {{ car.brand }}
                                </div>
                                <div class="car-col format-col">
                                    {{ car.numberFormat }}
                                </div>
                                <div class="car-col status-col">
                                    <span 
                                        class="status-badge"
                                        :class="{
                                            'status-active': car.status === 'Активна',
                                            'status-inactive': car.status === 'Неактивна'
                                        }"
                                    >
                                        {{ car.status }}
                                    </span>
                                </div>
                                <div class="car-col actions-col">
                                    <button 
                                        @click="deleteCar(car)" 
                                        class="delete-btn"
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
                        {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Автомобилей нет' }}
                    </p>
                </div>
            </div>
        </div>
            <div class="carsview__right-side">
                <div class="carsview__help">
                <p class="help__text">
                    Здесь находятся автомобили, привязанные к вашей <strong class="blue">организации</strong>. Вы можете использовать эти автомобили при подаче автозаявок, нажав 
                </p>
                <p class="help__text">
                    Новые номера машин попадают в этот список <strong class="blue">автоматически</strong>, при подаче заявки.
                </p>
            </div>
        </div>
        </div>
        
    </section>
</template>

<script>
import SearchComponent from '@/components/SearchComponent.vue';
import RefreshButton from '@/components/RefreshButton.vue';

export default {
    components: {
        SearchComponent,
        RefreshButton
    },
    data() {
        return {
            searchQuery: '',
            sortField: null,
            sortDirection: 'desc',
            carsData: [],
            searchTimeout: null
        };
    },
    computed: {
        filteredCars() {
            if (!this.searchQuery.trim()) {
                return this.carsData;
            }
            
            const query = this.searchQuery.toLowerCase().trim();
            return this.carsData.filter(car => 
                car.carNumber.toLowerCase().includes(query) ||
                car.brand.toLowerCase().includes(query) ||
                car.numberFormat.toLowerCase().includes(query) ||
                car.status.toLowerCase().includes(query)
            );
        },

        sortedCars() {
            const cars = [...this.filteredCars];
            
            if (!this.sortField) {
                return cars;
            }
            
            return cars.sort((a, b) => {
                let valueA, valueB;
                
                switch (this.sortField) {
                    case 'id':
                        valueA = a.id;
                        valueB = b.id;
                        break;
                        
                    case 'carNumber':
                        // Сортировка по центральным цифрам как числам
                        valueA = parseInt(this.extractCentralDigits(a.carNumber)) || 0;
                        valueB = parseInt(this.extractCentralDigits(b.carNumber)) || 0;
                        break;
                        
                    case 'brand':
                    case 'numberFormat':
                    case 'status':
                        valueA = a[this.sortField]?.toLowerCase() || '';
                        valueB = b[this.sortField]?.toLowerCase() || '';
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
            // Дебаунс для быстрого поиска без конфликтов с анимациями
            clearTimeout(this.searchTimeout);
            this.searchTimeout = setTimeout(() => {
                // Принудительное обновление без анимаций
                this.$forceUpdate();
            }, 50);
        }
    },
    methods: {
        fetchCars() {
            // Заглушка с 13 шаблонными машинами
            this.carsData = [
                { id: 1, carNumber: 'А 100 ВС 77', brand: 'Тойота', numberFormat: 'Россия', status: 'Активна' },
                { id: 2, carNumber: 'А 121 ВС 77', brand: 'Хендай', numberFormat: 'Россия', status: 'Активна' },
                { id: 3, carNumber: 'А 189 ВС 77', brand: 'Киа', numberFormat: 'Россия', status: 'Неактивна' },
                { id: 4, carNumber: 'А 222 ВС 77', brand: 'Лада', numberFormat: 'Россия', status: 'Активна' },
                { id: 5, carNumber: 'А 300 ВС 77', brand: 'Фольксваген', numberFormat: 'Россия', status: 'Активна' },
                { id: 6, carNumber: 'В 456 ЕК 77', brand: 'Шкода', numberFormat: 'Россия', status: 'Неактивна' },
                { id: 7, carNumber: 'В 500 ЕК 77', brand: 'Рено', numberFormat: 'Россия', status: 'Активна' },
                { id: 8, carNumber: 'В 600 ЕК 77', brand: 'Шевроле', numberFormat: 'Россия', status: 'Активна' },
                { id: 9, carNumber: 'С 246 ВВ 77', brand: 'Ниссан', numberFormat: 'Россия', status: 'Неактивна' },
                { id: 10, carNumber: 'С 350 ВВ 77', brand: 'БМВ', numberFormat: 'Россия', status: 'Активна' },
                { id: 11, carNumber: 'С 450 ВВ 77', brand: 'Мерседес', numberFormat: 'Россия', status: 'Активна' },
                { id: 12, carNumber: 'Е 192 РР 77', brand: 'Ауди', numberFormat: 'Россия', status: 'Неактивна' },
                { id: 13, carNumber: 'Е 638 ОО 77', brand: 'Лексус', numberFormat: 'Россия', status: 'Активна' }
            ];
        },
        
        deleteCar(car) {
            if (confirm(`Вы уверены, что хотите удалить автомобиль ${car.carNumber}?`)) {
                this.carsData = this.carsData.filter(c => c.id !== car.id);
            }
        },
        
        sortBy(field) {
            if (this.sortField === field) {
                this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
            } else {
                this.sortField = field;
                this.sortDirection = 'desc';
            }
        },

        // Извлечение центральных цифр из номера
        extractCentralDigits(carNumber) {
            // Убираем пробелы и извлекаем цифры
            const digits = carNumber.replace(/\s/g, '').match(/\d+/g);
            if (digits && digits.length > 0) {
                // Берем первую группу цифр (центральные 3 цифры)
                return digits[0];
            }
            return '';
        }
    },
    mounted() {
        this.fetchCars();
    }
}
</script>

<style scoped>
.carsview {
    padding: 20px;
}

.carsview__container {
    display: flex;
    gap: 30px;
    margin-top: 20px;
}

.carsview__right-side {
    width: 40%;
}

.carsview__header {
    padding-bottom: 15px;
    display: flex;
    align-items: center;
    gap: 10px;
    height: 25px;
}

.carsview__title {
    font-size: 18px;
}

.carsview__filters {
    padding-bottom: 15px;
    width: 100%;
    border-bottom: 1px solid #e6e6e6;
}

.blue {
    color: #4F5BDF;
}

/* Стили для таблицы */
.cars-card {
    background-color: #fff;
    border-radius: 30px;
    border: 1px solid #e6e6e6;
    overflow: hidden;
    width: 60%;
    height:360px;
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

.card-title {
    margin: 0;
    color: #000;
    font-weight: 600;
    font-size: 1.0em;
}

.highlight-text {
    color: #4F5BDF;
}

.card-content {
    padding: 0;
    height: calc(100% - 40px);
    display: flex;
    flex-direction: column;
}

.cars-container {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow-y: auto;
}

/* Заголовок таблицы */
.cars-header {
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
    justify-content: center;
}

.car-number-col {
    width: 20%;
    min-width: 120px;
}

.brand-col {
    width: 20%;
    min-width: 120px;
}

.format-col {
    width: 25%;
    min-width: 120px;
}

.status-col {
    width: 18%;
    min-width: 100px;
}

.actions-col {
    width: 10%;
    min-width: 40px;
    justify-content: center;
}

/* Тело таблицы */
.cars-body {
    overflow-y: auto;
    flex-grow: 1;
    padding-right: 4px;
    margin-right: 4px;
    scroll-behavior: smooth;
}

.car-item {
    transition: background-color 0.2s ease;
}

.car-item:hover {
    background-color: #fafafa;
}

.car-row {
    display: flex;
    width: 100%;
    padding: 10px 16px;
    align-items: center;
    border-bottom: 1px solid #f0f0f0;
}

.car-col {
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
.number-col .car-col,
.actions-col .car-col {
    justify-content: center;
}

/* Стилизация скроллбара */
.cars-body::-webkit-scrollbar {
    width: 6px;
}

.cars-body::-webkit-scrollbar-track {
    background: transparent;
    margin: 2px 0;
    border-radius: 3px;
}

.cars-body::-webkit-scrollbar-thumb {
    background: #D9E2FF;
    border-radius: 3px;
    border: 1px solid transparent;
    background-clip: content-box;
    transition: all 0.3s ease;
}

.cars-body::-webkit-scrollbar-thumb:hover {
    background: #C5D1FF;
    border: 1px solid transparent;
    background-clip: content-box;
    transform: scale(1.1);
}

.cars-body {
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

.delete-btn {
    background: none;
    border: none;
    cursor: pointer;
    padding: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
    transition: background-color 0.2s ease;
}

.delete-btn:hover {
    background-color: #f5f5f5;
}

.delete-icon {
    width: 16px;
    height: 16px;
    opacity: 0.7;
    transition: opacity 0.2s ease;
}

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
    line-height: 150%;
}

@media (max-width: 768px) {
    .cars-card {
        width: 100%;
        height: auto;
    }
    
    .header-row,
    .car-row {
        flex-wrap: wrap;
    }
    
    .header-col,
    .car-col {
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
}
</style>