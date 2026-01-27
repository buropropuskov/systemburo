<template>
    <section class="carsview">
        <header class="carsview__header">
            <h2 class="carsview__title">
                Список <span class="blue">автомобилей</span>
            </h2>
        </header>
        
        <div class="carsview__filters">
            <div class="filters-container">
                <SearchComponent
                    :title="'Поиск машин...'"
                    v-model="searchQuery"
                />
                <div class="filter-tabs" v-if="ownershipInfo">
                    <button 
                        v-if="ownershipInfo.has_organization"
                        class="filter-tab"
                        :class="{ 'filter-tab--active': currentFilter === 'organization' }"
                        @click="switchFilter('organization')"
                    >
                        Машины организации
                    </button>
                    <button 
                        v-if="ownershipInfo.has_company"
                        class="filter-tab"
                        :class="{ 'filter-tab--active': currentFilter === 'company' }"
                        @click="switchFilter('company')"
                    >
                        Машины компании
                    </button>
                    <button 
                        class="filter-tab"
                        :class="{ 'filter-tab--active': currentFilter === 'user' }"
                        @click="switchFilter('user')"
                    >
                        Мои машины
                    </button>
                    <button 
                        class="filter-tab"
                        :class="{ 'filter-tab--active': currentFilter === 'system' }"
                        @click="switchFilter('system')"
                    >
                        Все машины системы
                    </button>
                </div>
            </div>
        </div>

        <div class="carsview__container">
            <!-- Таблица автомобилей -->
            <div class="cars-card">
                <div class="card-header">
                    <div class="card-header__title">
                        <h3 class="card-title">
                            <span v-if="currentFilter === 'organization'" class="highlight-text">Машины <span class="blue">организации</span></span>
                            <span v-else-if="currentFilter === 'company'" class="highlight-text">Машины <span class="blue">компании</span></span>
                            <span v-else-if="currentFilter === 'system'" class="highlight-text">Все машины <span class="blue">системы</span></span>
                            <span v-else class="highlight-text">Мои <span class="blue">автомобили</span></span>
                        </h3>
                    </div>
                    <div class="card-header__settings">
                        <button 
                            class="add-button" 
                            @click="showAddCarModal"
                            v-if="currentFilter !== 'system'"
                        >
                            Добавить
                        </button>
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
                            <div class="header-col car-number-col" @click="sortBy('number')">
                                <p :class="{ 'active-sort': sortField === 'number' }">Номер</p>
                                <img 
                                    src="@/assets/icons/sort.png" 
                                    class="sort-icon" 
                                    :class="{ 
                                        'sorted': sortField === 'number',
                                        'desc': sortField === 'number' && sortDirection === 'desc'
                                    }" 
                                />
                            </div>
                            <div class="header-col brand-col" @click="sortBy('mark')">
                                <p :class="{ 'active-sort': sortField === 'mark' }">Марка</p>
                                <img 
                                    src="@/assets/icons/sort.png" 
                                    class="sort-icon" 
                                    :class="{ 
                                        'sorted': sortField === 'mark',
                                        'desc': sortField === 'mark' && sortDirection === 'desc'
                                    }" 
                                />
                            </div>
                            <div class="header-col format-col" @click="sortBy('format_name')">
                                <p :class="{ 'active-sort': sortField === 'format_name' }">Формат номера</p>
                                <img 
                                    src="@/assets/icons/sort.png" 
                                    class="sort-icon" 
                                    :class="{ 
                                        'sorted': sortField === 'format_name',
                                        'desc': sortField === 'format_name' && sortDirection === 'desc'
                                    }" 
                                />
                            </div>
                            <div class="header-col source-col" v-if="currentFilter === 'system'" @click="sortBy('source')">
                                <p :class="{ 'active-sort': sortField === 'source' }">Источник</p>
                                <img 
                                    src="@/assets/icons/sort.png" 
                                    class="sort-icon" 
                                    :class="{ 
                                        'sorted': sortField === 'source',
                                        'desc': sortField === 'source' && sortDirection === 'desc'
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
                            <div class="header-col actions-col" v-if="currentFilter !== 'system'">
                                Действия
                            </div>
                        </div>
                    </div>
                    
                    <!-- Тело таблицы -->
                    <div class="cars-container">
                        <div v-if="filteredCars.length > 0" class="cars-body">
                            <div 
                                v-for="(car) in sortedCars" 
                                :key="`${car.source || 'unique'}-${car.id}`" 
                                class="car-item"
                            >
                                <div class="car-row">
                                    <div class="car-col number-col">
                                        {{ car.id }}
                                    </div>
                                    <div class="car-col car-number-col">
                                        {{ car.number }}
                                    </div>
                                    <div class="car-col brand-col">
                                        {{ car.mark }}
                                    </div>
                                    <div class="car-col format-col">
                                        {{ car.format_name || 'Не указан' }}
                                    </div>
                                    <div class="car-col source-col" v-if="currentFilter === 'system'">
                                        <span class="source-badge" :class="`source-${car.source}`">
                                            {{ car.source === 'unique' ? 'База машин' : 'Заявки' }}
                                        </span>
                                    </div>
                                    <div class="car-col status-col">
                                        <span 
                                            class="status-badge"
                                            :class="{
                                                'status-active': car.status,
                                                'status-inactive': !car.status
                                            }"
                                        >
                                            {{ car.status ? 'Активна' : 'Неактивна' }}
                                        </span>
                                    </div>
                                    <div class="car-col actions-col" v-if="currentFilter !== 'system'">
                                        <button 
                                            @click="editCar(car)" 
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
                                            @click="deleteCar(car)" 
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
                            {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Автомобилей нет' }}
                        </p>
                    </div>
                </div>
            </div>
            
            <div class="carsview__right-side">
                <div class="carsview__help">
                    <p class="help__text">
                        <span v-if="currentFilter === 'organization'">
                            Здесь находятся автомобили, привязанные к вашей <strong class="blue">организации</strong>. Вы можете использовать эти автомобили при подаче автозаявок.
                        </span>
                        <span v-else-if="currentFilter === 'company'">
                            Здесь находятся автомобили, привязанные к вашей <strong class="blue">компании</strong>. Вы можете использовать эти автомобили при подаче автозаявок.
                        </span>
                        <span v-else-if="currentFilter === 'system'">
                            Здесь находятся <strong class="blue">все автомобили системы</strong> (из базы машин и заявок). Только для просмотра.
                        </span>
                        <span v-else>
                            Здесь находятся автомобили, привязанные к вашему <strong class="blue">аккаунту</strong>. Вы можете использовать эти автомобили при подаче автозаявок.
                        </span>
                    </p>
                    <p class="help__text" v-if="currentFilter !== 'system'">
                        Новые номера машин попадают в этот список <strong class="blue">автоматически</strong>, при подаче заявки.
                    </p>
                </div>
            </div>
        </div>

        <!-- Модальное окно добавления машины -->
        <div v-if="showModal" class="modal-overlay" @click="closeModal">
            <div class="modal-content" @click.stop>
                <div class="modal-header">
                    <div class="modal-header__top">
                        <h3>{{ editingCar ? 'Редактирование' : 'Добавление Т/С' }}</h3>
                        <div v-if="notification.show" class="notification-badge" :class="notification.type">
                            {{ notification.message }}
                        </div>
                    </div>
                    <button class="modal-close" @click="closeModal">×</button>
                </div>
                <div class="modal-body">
                    <div class="data__completion">
                        <div class="completion__format">
                            <div class="format__header">
                                <label class="format__label">Формат номеров</label>
                                <button class="add-button" @click="saveCar" :disabled="!canSaveCar">
                                    {{ editingCar ? 'Сохранить' : 'Добавить' }}
                                </button>
                            </div>
                            <div class="format__dropdown">
                                <button class="dropdown__button" @click="toggleFormatDropdown">
                                    <div class="button__content">
                                        <span class="button__text">{{ selectedFormatText }}</span>
                                        <img src="@/assets/icons/arrow.png" class="button__arrow" :class="{ 'button__arrow--open': isFormatDropdownOpen }" />
                                    </div>
                                </button>
                                <transition name="dropdown">
                                    <div v-if="isFormatDropdownOpen" class="dropdown__menu">
                                        <div 
                                            v-for="format in availableFormats" 
                                            :key="format.format.id"
                                            class="dropdown__item" 
                                            @click="selectFormat(format)"
                                        >
                                            <span class="item__text">{{ format.format.name }}</span>
                                        </div>
                                    </div>
                                </transition>
                            </div>
                        </div>
                        
                        <div class="completion__fields">
                            <div class="completion__number">
                                <div class="completion__number-header">
                                    <label class="input__label">Номер Т/C <span class="required">*</span></label>
                                </div>
                                
                                <!-- Динамический формат из базы данных -->
                                <div class="number__field" v-if="selectedFormat">
                                    <input 
                                        v-for="(cell, index) in selectedFormat.cells" 
                                        :key="index"
                                        class="number__input" 
                                        :placeholder="getPlaceholder(cell)"
                                        v-model="numberParts[index]"
                                        @input="validatePart(index, $event, cell)"
                                        @blur="formatPart(index, cell)"
                                        :maxlength="cell.max_length"
                                        :style="{ width: getInputWidth(cell) }"
                                    />
                                </div>
                                <div v-else class="no-format-message">
                                    Выберите формат номера
                                </div>
                            </div>
                            
                            <div class="completion__mark">
                                <div class="completion__mark-header">
                                    <label class="input__label">Марка Т/С <span class="required">*</span></label>
                                </div>
                                <div class="mark__field">
                                    <div class="mark__dropdown">
                                        <button class="mark__dropdown-button" @click="toggleMarkDropdown">
                                            <div class="mark__button-content">
                                                <span class="mark__button-text">{{ selectedMark || 'Выберите марку' }}</span>
                                                <img src="@/assets/icons/arrow.png" class="mark__button-arrow" :class="{ 'mark__button-arrow--open': isMarkDropdownOpen }" />
                                            </div>
                                        </button>
                                        <transition name="dropdown">
                                            <div v-if="isMarkDropdownOpen" class="mark__dropdown-menu">
                                                <div class="mark__search">
                                                    <input 
                                                        class="mark__search-input" 
                                                        placeholder="Поиск марки..."
                                                        v-model="markSearch"
                                                        @input="filterMarks"
                                                    />
                                                </div>
                                                <div class="mark__dropdown-list">
                                                    <div 
                                                        v-for="mark in filteredMarks" 
                                                        :key="mark"
                                                        class="mark__dropdown-item"
                                                        @click="selectMark(mark)"
                                                    >
                                                        <span class="mark__item-text">{{ mark }}</span>
                                                    </div>
                                                </div>
                                            </div>
                                        </transition>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Привязка -->
                        <div class="completion__binding">
                            <label class="input__label">Привязка</label>
                            <div class="binding-info">
                                <p class="binding-note">
                                    <strong>Добавляемый автомобиль автоматически привязывается к аккаунту пользователя.</strong>
                                    Автомобиль можно привязать к организации или компании, для использования <strong>другими сотрудниками</strong>:
                                </p>
                            </div>
                            <div class="binding-options">
                                <label class="binding-option" v-if="ownershipInfo && ownershipInfo.has_organization">
                                    <input 
                                        type="checkbox" 
                                        v-model="bindToOrganization"
                                        :disabled="bindToCompany"
                                    />
                                    <span>Привязать к организации</span>
                                </label>
                                <label class="binding-option" v-if="ownershipInfo && ownershipInfo.has_company">
                                    <input 
                                        type="checkbox" 
                                        v-model="bindToCompany"
                                        :disabled="bindToOrganization"
                                    />
                                    <span>Привязать к компании</span>
                                </label>
                                <div class="user-binding">
                                    <span class="user-binding-text"><strong class="red">Внимание!</strong> При привязке автомобиля к организации или компании, он будет доступен для отображения и использования для всех сотрудников, привязанных к организации/компании. </span>
                                </div>
                            </div>
                        </div>
                    </div>
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
            searchTimeout: null,
            currentFilter: 'user',
            ownershipInfo: null,
            showModal: false,
            availableFormats: [],

            // Формат номера
            selectedFormat: null,
            isFormatDropdownOpen: false,
            numberParts: [],
            
            // Марка
            selectedMark: '',
            isMarkDropdownOpen: false,
            markSearch: '',
            marks: [
                'ВАЗ', 'Мерседес', 'БМВ', 'Газель', 'ГАЗ', 'Вольво', 'Тойота', 'Митсубиси',
                'Ауди', 'Фольксваген', 'Шевроле', 'Хендай', 'Киа', 'Ниссан', 'Рено', 'Пежо',
                'Ситроен', 'Форд', 'Опель', 'Шкода', 'Лада', 'УАЗ'
            ],
            filteredMarks: [],
            
            // Привязка
            bindToOrganization: false,
            bindToCompany: false,
            
            // Уведомления
            notification: {
                show: false,
                message: '',
                type: 'success' // 'success' или 'error'
            },
            
            // Редактирование
            editingCar: null,
            originalCarData: null
        };
    },
    computed: {
        filteredCars() {
            if (!this.searchQuery.trim()) {
                return this.carsData;
            }
            
            const query = this.searchQuery.toLowerCase().trim();
            return this.carsData.filter(car => 
                car.number.toLowerCase().includes(query) ||
                car.mark.toLowerCase().includes(query) ||
                (car.format_name && car.format_name.toLowerCase().includes(query)) ||
                (car.source && car.source.toLowerCase().includes(query)) ||
                (car.status ? 'активна' : 'неактивна').includes(query)
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
                        
                    case 'number':
                        valueA = a.number?.toLowerCase() || '';
                        valueB = b.number?.toLowerCase() || '';
                        break;
                        
                    case 'mark':
                        valueA = a.mark?.toLowerCase() || '';
                        valueB = b.mark?.toLowerCase() || '';
                        break;
                        
                    case 'format_name':
                        valueA = a.format_name?.toLowerCase() || '';
                        valueB = b.format_name?.toLowerCase() || '';
                        break;
                        
                    case 'source':
                        valueA = a.source?.toLowerCase() || '';
                        valueB = b.source?.toLowerCase() || '';
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
        },

        selectedFormatText() {
            return this.selectedFormat ? this.selectedFormat.format.name : 'Выберите формат';
        },

        canSaveCar() {
            // Проверяем формат номера
            if (!this.selectedFormat || !this.numberParts.length) {
                return false;
            }

            // Проверяем каждую клетку формата
            for (let i = 0; i < this.selectedFormat.cells.length; i++) {
                const cell = this.selectedFormat.cells[i];
                const part = this.numberParts[i];
                
                if (!part || part.length < cell.min_length || part.length > cell.max_length) {
                    return false;
                }
            }
            
            // Проверяем марку
            if (!this.selectedMark) {
                return false;
            }
            
            return true;
        }
    },
    watch: {
        searchQuery() {
            clearTimeout(this.searchTimeout);
            this.searchTimeout = setTimeout(() => {
                this.$forceUpdate();
            }, 50);
        },
        
        bindToOrganization(newVal) {
            if (newVal) {
                this.bindToCompany = false;
            }
        },
        
        bindToCompany(newVal) {
            if (newVal) {
                this.bindToOrganization = false;
            }
        }
    },
    methods: {
        async fetchCars() {
            try {
                const token = localStorage.getItem("token");
                
                if (this.currentFilter === 'system') {
                    // Загружаем все машины системы (unique_cars + cars)
                    const [uniqueCarsResponse, systemCarsResponse] = await Promise.all([
                        fetch(`http://localhost:8080/unique-cars?filter_type=all`, {
                            method: "GET",
                            headers: {
                                "Authorization": `Bearer ${token}`
                            }
                        }),
                        fetch(`http://localhost:8080/cars/system-all`, {
                            method: "GET",
                            headers: {
                                "Authorization": `Bearer ${token}`
                            }
                        })
                    ]);

                    if (uniqueCarsResponse.ok && systemCarsResponse.ok) {
                        const uniqueCars = await uniqueCarsResponse.json();
                        const systemCars = await systemCarsResponse.json();
                        
                        // Добавляем source для идентификации
                        const uniqueWithSource = uniqueCars.map(car => ({
                            ...car,
                            source: 'unique'
                        }));
                        
                        // Объединяем и убираем дубликаты (по номеру и марке)
                        const allCarsMap = new Map();
                        
                        // Сначала добавляем уникальные машины
                        uniqueWithSource.forEach(car => {
                            const key = `${car.number.toLowerCase()}_${car.mark.toLowerCase()}`;
                            allCarsMap.set(key, car);
                        });
                        
                        // Затем добавляем машины из заявок (перезаписываем если нужно)
                        systemCars.forEach(car => {
                            const key = `${car.number.toLowerCase()}_${car.mark.toLowerCase()}`;
                            if (!allCarsMap.has(key)) {
                                allCarsMap.set(key, {
                                    ...car,
                                    source: 'cars',
                                    format_name: car.format_name || 'Не указан',
                                    status: car.status !== undefined ? car.status : true
                                });
                            }
                        });
                        
                        this.carsData = Array.from(allCarsMap.values());
                    } else {
                        console.error("Ошибка при загрузке всех машин системы");
                        this.carsData = [];
                    }
                } else {
                    // Загружаем машины по фильтру (user, organization, company)
                    const response = await fetch(`http://localhost:8080/unique-cars?filter_type=${this.currentFilter}`, {
                        method: "GET",
                        headers: {
                            "Authorization": `Bearer ${token}`
                        }
                    });

                    if (response.ok) {
                        this.carsData = await response.json();
                    } else {
                        console.error("Ошибка при загрузке машин");
                        this.carsData = [];
                    }
                }
            } catch (error) {
                console.error("Ошибка при загрузке машин:", error);
                this.carsData = [];
            }
        },

        async fetchOwnershipInfo() {
            try {
                const token = localStorage.getItem("token");
                const response = await fetch("http://localhost:8080/unique-cars/ownership-info", {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`
                    }
                });

                if (response.ok) {
                    this.ownershipInfo = await response.json();
                }
            } catch (error) {
                console.error("Ошибка при загрузке информации о владельце:", error);
            }
        },

        async fetchFormats() {
            try {
                const token = localStorage.getItem("token");
                const response = await fetch("http://localhost:8080/license-plate-formats", {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`
                    }
                });

                if (response.ok) {
                    this.availableFormats = await response.json();
                    // Выбираем формат по умолчанию или первый формат
                    const defaultFormat = this.availableFormats.find(f => f.format.is_default);
                    this.selectedFormat = defaultFormat || this.availableFormats[0];
                    this.initializeNumberParts();
                }
            } catch (error) {
                console.error("Ошибка при загрузке форматов:", error);
            }
        },

        async deleteCar(car) {
            if (confirm(`Вы уверены, что хотите удалить автомобиль ${car.number}?`)) {
                try {
                    const token = localStorage.getItem("token");
                    const response = await fetch(`http://localhost:8080/unique-cars/${car.id}`, {
                        method: "DELETE",
                        headers: {
                            "Authorization": `Bearer ${token}`
                        }
                    });

                    if (response.ok) {
                        await this.fetchCars();
                        this.showNotification('Автомобиль успешно удален!', 'success');
                    } else {
                        alert("Ошибка при удалении автомобиля");
                    }
                } catch (error) {
                    console.error("Ошибка при удалении автомобиля:", error);
                    alert("Ошибка при удалении автомобиля");
                }
            }
        },

        editCar(car) {
            // Не разрешаем редактировать машины из системы
            if (this.currentFilter === 'system' || car.source === 'cars') {
                this.showNotification('Редактирование недоступно для машин из заявок', 'error');
                return;
            }
            
            this.editingCar = car;
            
            // Сохраняем оригинальные значения для сравнения
            this.originalCarData = {
                mark: car.mark,
                format_id: car.format_id,
                number: car.number,
                organization_id: car.organization_id,
                company_id: car.company_id
            };
            
            // Устанавливаем текущие значения машины
            this.selectedMark = car.mark;
            
            // Находим формат по format_id
            if (car.format_id) {
                const carFormat = this.availableFormats.find(f => f.format.id === car.format_id);
                if (carFormat) {
                    this.selectedFormat = carFormat;
                    // Разбиваем номер на части согласно формату
                    this.numberParts = car.number.split(' ');
                } else {
                    // Если формат не найден, используем формат по умолчанию
                    const defaultFormat = this.availableFormats.find(f => f.format.is_default) || this.availableFormats[0];
                    this.selectedFormat = defaultFormat;
                    this.initializeNumberParts();
                }
            }
            
            // Устанавливаем привязки
            this.bindToOrganization = !!car.organization_id;
            this.bindToCompany = !!car.company_id;
            
            this.showModal = true;
            this.hideNotification();
        },

        // Проверка наличия изменений
        hasChanges() {
            if (!this.editingCar) {
                return true; // Для новой машины всегда отправляем запрос
            }

            // Проверяем изменения в марке
            if (this.selectedMark !== this.originalCarData.mark) {
                return true;
            }

            // Проверяем изменения в формате
            if (this.selectedFormat.format.id !== this.originalCarData.format_id) {
                return true;
            }

            // Проверяем изменения в номере
            const currentNumber = this.numberParts.join(' ');
            if (currentNumber !== this.originalCarData.number) {
                return true;
            }

            // Проверяем изменения в привязке к организации
            const currentOrgId = this.bindToOrganization ? this.ownershipInfo.organization_id : null;
            if (currentOrgId !== this.originalCarData.organization_id) {
                return true;
            }

            // Проверяем изменения в привязке к компании
            const currentCompanyId = this.bindToCompany ? this.ownershipInfo.company_id : null;
            if (currentCompanyId !== this.originalCarData.company_id) {
                return true;
            }

            return false;
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
            this.fetchCars();
        },

        showAddCarModal() {
            // Не разрешаем добавлять в системную вкладку
            if (this.currentFilter === 'system') {
                this.showNotification('Добавление недоступно во вкладке "Все машины системы"', 'error');
                return;
            }
            
            this.editingCar = null;
            this.showModal = true;
            this.filteredMarks = this.marks;
            this.hideNotification();
            this.resetNewCar();
        },

        closeModal() {
            this.showModal = false;
            this.editingCar = null;
            this.resetNewCar();
            this.hideNotification();
        },

        resetNewCar() {
            this.selectedFormat = this.availableFormats.find(f => f.format.is_default) || this.availableFormats[0];
            this.initializeNumberParts();
            this.selectedMark = '';
            this.markSearch = '';
            this.filteredMarks = this.marks;
            this.bindToOrganization = false;
            this.bindToCompany = false;
        },

        clearFormFields() {
            // Очищаем только номер и марку, чекбоксы остаются
            this.initializeNumberParts();
            this.selectedMark = '';
            this.markSearch = '';
            this.filteredMarks = this.marks;
            
            // Если это добавление новой машины (не редактирование), сбрасываем чекбоксы
            if (!this.editingCar) {
                this.bindToOrganization = false;
                this.bindToCompany = false;
            }
            // При редактировании поля НЕ очищаются - остаются текущие значения машины
        },

        // Уведомления
        showNotification(message, type = 'success') {
            this.notification = {
                show: true,
                message: message,
                type: type
            };
            
            // Автоматически скрываем уведомление через 3 секунды
            setTimeout(() => {
                this.hideNotification();
            }, 3000);
        },

        hideNotification() {
            this.notification.show = false;
        },

        // Формат номера методы
        initializeNumberParts() {
            if (this.selectedFormat) {
                this.numberParts = new Array(this.selectedFormat.cells.length).fill('');
            } else {
                this.numberParts = [];
            }
        },

        getPlaceholder(cell) {
            if (cell.cell_type === 'numbers') {
                return '0'.repeat(cell.max_length);
            } else {
                return 'A'.repeat(cell.max_length);
            }
        },

        getInputWidth(cell) {
            const baseWidth = 25;
            const minWidth = 50;
            const width = Math.max(minWidth, cell.max_length * baseWidth);
            return `${width}px`;
        },

        validatePart(index, event, cell) {
            let value = event.target.value.toUpperCase();
            
            if (cell.cell_type === 'numbers') {
                value = value.replace(/\D/g, '');
            } else if (cell.cell_type === 'letters') {
                if (cell.alphabet_type === 'cyrillic') {
                    value = this.filterCyrillicLetters(value, cell.allowed_letters);
                } else if (cell.alphabet_type === 'latin') {
                    value = this.filterLatinLetters(value, cell.allowed_letters);
                } else if (cell.alphabet_type === 'both') {
                    value = this.filterBothLetters(value, cell.allowed_letters);
                }
            } else if (cell.cell_type === 'mixed') {
                if (cell.alphabet_type === 'cyrillic') {
                    value = this.filterMixedCyrillic(value, cell.allowed_letters);
                } else if (cell.alphabet_type === 'latin') {
                    value = this.filterMixedLatin(value, cell.allowed_letters);
                } else if (cell.alphabet_type === 'both') {
                    value = this.filterMixedBoth(value, cell.allowed_letters);
                }
            }
            
            if (value.length > cell.max_length) {
                value = value.slice(0, cell.max_length);
            }
            
            this.numberParts[index] = value;
            event.target.value = value;
        },

        formatPart(index, cell) {
            if (cell.cell_type === 'numbers' && cell.padding_side && this.numberParts[index]) {
                let value = this.numberParts[index];
                const targetLength = cell.max_length;
                
                if (value.length < targetLength) {
                    const paddingChar = cell.padding_char || '0';
                    if (cell.padding_side === 'left') {
                        value = value.padStart(targetLength, paddingChar);
                    } else {
                        value = value.padEnd(targetLength, paddingChar);
                    }
                    this.numberParts[index] = value;
                }
            }
        },

        filterCyrillicLetters(value, allowedLetters) {
            if (allowedLetters) {
                const allowedChars = allowedLetters.split('');
                return value.split('').filter(char => allowedChars.includes(char)).join('');
            } else {
                return value.replace(/[^АВЕКМНОРСТУХ]/g, '');
            }
        },

        filterLatinLetters(value, allowedLetters) {
            if (allowedLetters) {
                const allowedChars = allowedLetters.split('');
                return value.split('').filter(char => allowedChars.includes(char)).join('');
            } else {
                return value.replace(/[^A-Z]/g, '');
            }
        },

        filterBothLetters(value, allowedLetters) {
            if (allowedLetters) {
                const allowedChars = allowedLetters.split('');
                return value.split('').filter(char => allowedChars.includes(char)).join('');
            } else {
                return value.replace(/[^A-ZА-Я]/g, '');
            }
        },

        filterMixedCyrillic(value, allowedLetters) {
            const numericPart = value.replace(/\D/g, '');
            const letterPart = this.filterCyrillicLetters(value.replace(/[0-9]/g, ''), allowedLetters);
            return numericPart + letterPart;
        },

        filterMixedLatin(value, allowedLetters) {
            const numericPart = value.replace(/\D/g, '');
            const letterPart = this.filterLatinLetters(value.replace(/[0-9]/g, ''), allowedLetters);
            return numericPart + letterPart;
        },

        filterMixedBoth(value, allowedLetters) {
            const numericPart = value.replace(/\D/g, '');
            const letterPart = this.filterBothLetters(value.replace(/[0-9]/g, ''), allowedLetters);
            return numericPart + letterPart;
        },

        // Dropdown методы
        toggleFormatDropdown() {
            this.isFormatDropdownOpen = !this.isFormatDropdownOpen;
        },

        selectFormat(format) {
            this.selectedFormat = format;
            this.initializeNumberParts();
            this.isFormatDropdownOpen = false;
        },

        toggleMarkDropdown() {
            this.isMarkDropdownOpen = !this.isMarkDropdownOpen;
            if (this.isMarkDropdownOpen) {
                this.filterMarks();
            }
        },

        filterMarks() {
            if (!this.markSearch) {
                this.filteredMarks = this.marks;
            } else {
                const searchTerm = this.markSearch.toLowerCase();
                this.filteredMarks = this.marks.filter(mark => 
                    mark.toLowerCase().includes(searchTerm)
                );
            }
        },

        selectMark(mark) {
            this.selectedMark = mark;
            this.isMarkDropdownOpen = false;
            this.markSearch = '';
        },

        async saveCar() {
            if (!this.canSaveCar) {
                this.showNotification('Заполните все обязательные поля правильно', 'error');
                return;
            }

            // Проверяем изменения для редактирования
            if (this.editingCar && !this.hasChanges()) {
                this.showNotification('Изменений не обнаружено', 'info');
                return;
            }

            try {
                const token = localStorage.getItem("token");
                
                // Формируем номер из частей
                const number = this.numberParts.join(' ');
                
                // Формируем данные для отправки
                const carData = {
                    number: number,
                    mark: this.selectedMark,
                    format_id: this.selectedFormat.format.id,
                    user_id: this.ownershipInfo.user_id,
                    organization_id: this.bindToOrganization ? this.ownershipInfo.organization_id : null,
                    company_id: this.bindToCompany ? this.ownershipInfo.company_id : null
                };

                let response;
                if (this.editingCar) {
                    // Редактирование существующей машины
                    response = await fetch(`http://localhost:8080/unique-cars/${this.editingCar.id}`, {
                        method: "PUT",
                        headers: {
                            "Authorization": `Bearer ${token}`,
                            "Content-Type": "application/json"
                        },
                        body: JSON.stringify(carData)
                    });
                } else {
                    // Создание новой машины
                    response = await fetch("http://localhost:8080/unique-cars", {
                        method: "POST",
                        headers: {
                            "Authorization": `Bearer ${token}`,
                            "Content-Type": "application/json"
                        },
                        body: JSON.stringify(carData)
                    });
                }

                if (response.ok) {
                    const action = this.editingCar ? 'обновлен' : 'добавлен';
                    this.showNotification(`Автомобиль успешно ${action}!`, 'success');
                    
                    // Обновляем список машин
                    this.fetchCars();
                    
                    // Очищаем форму только при добавлении новой машины
                    if (!this.editingCar) {
                        this.clearFormFields();
                    } else {
                        // Обновляем оригинальные данные после успешного сохранения
                        this.originalCarData = {
                            mark: this.selectedMark,
                            format_id: this.selectedFormat.format.id,
                            number: number,
                            organization_id: carData.organization_id,
                            company_id: carData.company_id
                        };
                    }
                } else {
                    const errorData = await response.json();
                    const errorMessage = errorData.message || "Ошибка при сохранении автомобиля";
                    
                    // Специальные сообщения для дубликатов
                    if (errorMessage.includes("уже существует") || errorMessage.includes("already exists")) {
                        this.showNotification("Автомобиль уже привязан к вашему аккаунту", 'error');
                    } else {
                        this.showNotification(errorMessage, 'error');
                    }
                }
            } catch (error) {
                console.error("Ошибка при сохранении автомобиля:", error);
                this.showNotification("Ошибка при сохранении автомобиля", 'error');
            }
        }
    },
    async mounted() {
        await Promise.all([
            this.fetchOwnershipInfo(),
            this.fetchFormats()
        ]);
        await this.fetchCars();
        
        // Закрытие dropdown при клике вне
        document.addEventListener('click', (e) => {
            if (!e.target.closest('.format__dropdown')) {
                this.isFormatDropdownOpen = false;
            }
            
            if (!e.target.closest('.mark__dropdown')) {
                this.isMarkDropdownOpen = false;
            }
        });
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
.cars-card {
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
    width: 6%;
    min-width: 40px;
}

.car-number-col {
    width: 18%;
    min-width: 120px;
}

.brand-col {
    width: 18%;
    min-width: 120px;
}

.format-col {
    width: 20%;
    min-width: 120px;
}

.source-col {
    width: 14%;
    min-width: 90px;
}

.status-col {
    width: 14%;
    min-width: 90px;
}

.actions-col {
    width: 10%;
    min-width: 80px;
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

/* Бейджи для источника */
.source-badge {
    padding: 4px 10px;
    border-radius: 12px;
    font-size: 12px;
    font-weight: 500;
    display: inline-block;
}

.source-unique {
    background-color: #e6f4ea;
    color: #0d652d;
    border: 1px solid #a7d7b8;
}

.source-cars {
    background-color: #e6f2ff;
    color: #0066cc;
    border: 1px solid #99ccff;
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

/* Модальное окно */
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
    max-height: 90vh;
    overflow: hidden;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: 20px;
    border-bottom: 1px solid #e6e6e6;
}

.modal-header__top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex: 1;
    height: 25px;
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
    margin-left: 10px;
}

.modal-close:hover {
    color: #333;
}

.modal-body {
    padding: 20px;
    max-height: 70vh;
    overflow-y: auto;
}

/* Бейдж уведомления */
.notification-badge {
    padding: 6px 12px;
    border-radius: 12px;
    font-size: 11px;
    font-weight: 500;
    display: inline-block;
    max-width: fit-content;
    animation: slideIn 0.3s ease-out;
}

.notification-badge.success {
    background-color: #f0f9ff;
    color: #0369a1;
    border: 1px solid #bae6fd;
}

.notification-badge.info {
    background-color: #fffbeb;
    color: #b45309;
    border: 1px solid #fcd34d;
}

.notification-badge.error {
    background-color: #fef2f2;
    color: #991b1b;
    border: 1px solid #fecaca;
}

@keyframes slideIn {
    from {
        opacity: 0;
        transform: translateY(-10px);
    }
    to {
        opacity: 1;
        transform: translateY(0);
    }
}

/* Стили формы добавления машины */
.data__completion {
    padding: 0;
}

.input__label {
    font-size: 13px;
    color: #a2a2a2;
}

.required {
    color: #ff4444;
}

.completion__format {
    display: flex;
    flex-direction: column;
    gap: 5px;
    position: relative;
    padding-bottom: 10px;
}

.format__header {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.format__label {
    font-size: 13px;
    color: #a2a2a2;
}

.format__dropdown {
    position: relative;
}

.dropdown__button {
    width: 100%;
    height: 30px;
    border: 1px solid #e6e6e6;
    background-color: #FFF;
    border-radius: 50px;
    outline: none;
    cursor: pointer;
    padding: 0 15px;
    transition: border-color 0.2s;
}

.dropdown__button:hover {
    border-color: #4F5BDF;
}

.button__content {
    display: flex;
    align-items: center;
    width: 100%;
    height: 100%;
    justify-content: space-between;
}

.button__text {
    font-size: 14px;
    color: #000;
    font-weight: 500;
}

.button__arrow {
    width: 10px;
    height: 10px;
    transition: transform 0.2s;
    transform: rotate(90deg);
}

.button__arrow--open {
    transform: rotate(-90deg);
}

.dropdown__menu {
    position: absolute;
    top: 100%;
    left: 0;
    width: 100%;
    background: #FFF;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    margin-top: 5px;
    box-shadow: 0 3px 10px rgba(0,0,0,0.1);
    z-index: 1000;
    max-height: 300px;
    overflow-y: auto;
}

.dropdown__item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 15px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.dropdown__item:hover {
    background-color: #f5f5f5;
}

.dropdown__item:first-child {
    border-radius: 10px 10px 0 0;
}

.dropdown__item:last-child {
    border-radius: 0 0 10px 10px;
}

.item__text {
    font-size: 13px;
    color: #333;
}

.completion__fields {
    display: flex;
    gap: 20px;
    align-items: flex-start;
    margin-bottom: 15px;
}

.completion__number,
.completion__mark {
    flex: 1;
}

.completion__number-header,
.completion__mark-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 5px;
}

.number__field {
    max-width: 100%;
    min-width: 100%;
    height: 40px;
    display: flex;
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    overflow: hidden;
    background: #FFF;
}

.no-format-message {
    font-size: 12px;
    color: #a2a2a2;
    text-align: center;
    padding: 10px;
    background: #f8f8f8;
    border-radius: 10px;
}

.number__input {
    border: none;
    height: 100%;
    outline: none;
    text-align: center;
    font-size: 14px;
    background: transparent;
    flex: 1;
    min-width: 0;
}

.number__input:not(:last-child) {
    border-right: 1px solid #e6e6e6;
}

.number__input:first-child {
    border-radius: 15px 0 0 15px;
}

.number__input:last-child {
    border-radius: 0 15px 15px 0;
}

.number__input::placeholder {
    color: #a2a2a2;
    font-size: 12px;
}

.number__input:focus {
    background-color: #f8f8f8;
}

/* Mark dropdown styles */
.mark__field {
    width: 100%;
    height: 40px;
    position: relative;
}

.mark__dropdown {
    width: 100%;
    height: 100%;
}

.mark__dropdown-button {
    width: 100%;
    height: 100%;
    border: 1px solid #e6e6e6;
    background-color: #FFF;
    border-radius: 15px;
    outline: none;
    cursor: pointer;
    padding: 0 15px;
    transition: border-color 0.2s;
}

.mark__dropdown-button:hover {
    border-color: #4F5BDF;
}

.mark__button-content {
    display: flex;
    align-items: center;
    width: 100%;
    height: 100%;
    justify-content: space-between;
}

.mark__button-text {
    font-size: 14px;
    color: #000;
}

.mark__button-arrow {
    width: 10px;
    height: 10px;
    transition: transform 0.2s;
    transform: rotate(90deg);
}

.mark__button-arrow--open {
    transform: rotate(-90deg);
}

.mark__dropdown-menu {
    position: absolute;
    top: 100%;
    left: 0;
    width: 100%;
    background: #FFF;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    margin-top: 5px;
    box-shadow: 0 3px 10px rgba(0,0,0,0.1);
    z-index: 1000;
    max-height: 220px;
    overflow: hidden;
}

.mark__search {
    padding: 10px;
    border-bottom: 1px solid #e6e6e6;
}

.mark__search-input {
    width: 100%;
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    padding: 5px 10px;
    outline: none;
    font-size: 14px;
}

.mark__dropdown-list {
    max-height: 144px;
    overflow-y: auto;
}

.mark__dropdown-item {
    padding: 8px 15px;
    cursor: pointer;
    transition: background-color 0.2s;
    border-bottom: 1px solid #f5f5f5;
}

.mark__dropdown-item:hover {
    background-color: #f5f5f5;
}

.mark__dropdown-item:last-child {
    border-bottom: none;
}

.mark__item-text {
    font-size: 14px;
    color: #333;
}

/* Анимации для dropdown */
.dropdown-enter-active,
.dropdown-leave-active {
    transition: all 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
    opacity: 0;
    transform: translateY(-10px);
}

/* Привязка */
.completion__binding {
    margin-top: 15px;
    padding-top: 15px;
    border-top: 1px solid #e6e6e6;
}

.binding-info {
    margin-top: 10px;
    margin-bottom: 10px;
}

.binding-note {
    font-size: 12px;
    color: #666;
    line-height: 1.4;
    margin: 0 0 10px 0;
}

.binding-options {
    display: flex;
    flex-direction: column;
    gap: 10px;
}

.binding-option {
    display: flex;
    align-items: center;
    gap: 10px;
    cursor: pointer;
    font-size: 12px;
}

.binding-option input[type="checkbox"] {
    width: 12px;
    height: 12px;
    cursor: pointer;
}

.user-binding {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 5px 0;
}

.user-binding-text {
    font-size: 10px;
    color: #000;
    font-weight: 400;
}

.red {
    color: #ff4444;
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
    
    .carsview__container {
        flex-direction: column;
    }
    
    .carsview__right-side {
        width: 100%;
    }
    
    .completion__fields {
        flex-direction: column;
    }
    
    .modal-content {
        width: 95vw;
        margin: 10px;
    }
    
    .format-actions {
        flex-direction: column;
        width: 100%;
    }
    
    .add-button,
    .add-button-secondary {
        width: 100%;
    }
}
</style>