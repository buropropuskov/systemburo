<template>
    <div class="data__completion">
        <div class="completion__header">
            <h3>Добавление Т/С</h3>
            <button class="completion__button" @click="openExistingCarsModal">
                Добавить существующую(-ие)
            </button>
        </div>

        <!-- Отображение количества выбранных существующих машин -->
        <div v-if="selectedExistingCars.length > 0" class="existing-cars-info">
            <div class="existing-cars-header">
                <span class="existing-cars-count">Машин добавлено: {{ selectedExistingCars.length }}</span>
                <div class="existing-cars-actions">
                    <button class="view-cars-btn" @click="openExistingCarsModal">Просмотреть</button>
                    <button class="add-existing-btn" @click="addExistingCars" :disabled="!canAddExistingCars">
                        Добавить
                    </button>
                </div>
            </div>
        </div>

        <!-- Форма для добавления новой машины -->
        <div v-else>
            <div class="completion__format">
                <div class="format__header">
                    <label class="format__label">Формат номеров</label>
                    <div class="format-actions">
                        <button class="cancel-edit-btn" @click="cancelEdit" v-if="editingVehicle">
                            Отменить
                        </button>
                        <button 
                            class="add-button" 
                            @click="addVehicle"
                            :disabled="!canAddVehicle"
                            @mouseenter="showTooltip = true"
                            @mouseleave="showTooltip = false"
                        >
                            {{ editingVehicle ? 'Применить' : 'Добавить' }}
                        </button>
                        <!-- Подсказка для кнопки -->
                        <div v-if="showTooltip && !canAddVehicle" class="tooltip">
                            <div class="tooltip-content">
                                {{ getTooltipMessage }}
                            </div>
                        </div>
                    </div>
                </div>
                <div class="format__dropdown">
                    <button 
                        class="dropdown__button" 
                        @click="toggleFormatDropdown"
                        :disabled="editingVehicle && editingVehicle.isExisting"
                    >
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
                        <div class="number-fact">
                            <input 
                                class="fact-checkbox" 
                                type="checkbox" 
                                v-model="isNumberByFact"
                                @change="handleNumberByFactChange"
                                :disabled="editingVehicle && editingVehicle.isExisting"
                            />
                            <p class="fact-text">по факту</p>
                        </div>
                    </div>
                    <!-- Поле "по факту" -->
                    <div class="number__field number__field--fact" v-if="isNumberByFact">
                        <input 
                            class="number__input number__input--fact" 
                            value="По факту"
                            readonly
                        />
                    </div>
                    
                    <!-- Динамический формат из базы данных -->
                    <div class="number__field" v-else-if="selectedFormat">
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
                            :disabled="editingVehicle && editingVehicle.isExisting"
                        />
                    </div>
                    <div v-else class="no-format-message">
                        Выберите формат номера
                    </div>

                    <!-- Блок предупреждения об активной заявке для номера -->
                    
                </div>
                
                <div class="completion__mark">
                    <div class="completion__mark-header">
                        <label class="input__label">Марка Т/С <span class="required">*</span></label>
                        <div class="mark-fact">
                            <input 
                                class="fact-checkbox" 
                                type="checkbox" 
                                v-model="isMarkByFact"
                                @change="handleMarkByFactChange"
                            />
                            <p class="fact-text">по факту</p>
                        </div>
                    </div>
                    <div class="mark__field mark__field--fact" v-if="isMarkByFact">
                        <input 
                            class="mark__input mark__input--fact" 
                            value="По факту"
                            readonly
                        />
                    </div>
                    <div class="mark__field" v-else>
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
            <div v-if="activeCarInfo && !isNumberByFact" class="active-warning">
                     
                        <div class="warning-text">
                            <p class="warning-title">На это авто уже есть активная заявка!</p>
                            <p class="warning-details">
                                Действует до: {{ formatDate(activeCarInfo.entry_date_to) }} {{ formatTime(activeCarInfo.entry_time_to) }}<br>
                                Заявка {{ activeCarInfo.application_number }}<br>
                                Организация: {{ activeCarInfo.organization_name || 'Не указана' }}<br>
                                Компания: {{ activeCarInfo.company_name || 'Не указана' }}
                            </p>
                        </div>
                    </div>
        </div>

        <!-- Места разгрузки -->
        <div class="completion__unloading">
            <label class="input__label">Места разгрузки (выбор) <span class="required">*</span></label>
            <div class="unloading__grid" v-if="!loadingUnloadingPlaces && allUnloadingPlaces.length > 0">
                <div 
                    v-for="place in allUnloadingPlaces" 
                    :key="place.id"
                    class="unloading__item"
                    :class="{ 
                        'unloading__item--active': selectedUnloadingPlaces.includes(place.id) && place.status === 'active',
                        'unloading__item--attached': attachedPlacesIds.includes(place.id),
                        'unloading__item--inactive': place.status !== 'active'
                    }"
                    @click="toggleUnloadingPlace(place)"
                    @mouseenter="showInactiveTooltip(place, $event)"
                    @mouseleave="hideInactiveTooltip"
                >
                    {{ place.name }}
                </div>
            </div>
            <div v-else-if="loadingUnloadingPlaces" class="loading-message">
                Загрузка мест разгрузки...
            </div>
            <div v-else class="no-places-message">
                Нет доступных мест разгрузки
            </div>
            <div v-if="errors.unloadingPlaces" class="error-message">{{ errors.unloadingPlaces }}</div>
        </div>

        <!-- Tooltip для неактивных мест -->
        <div v-if="inactiveTooltip.visible" 
             class="inactive-tooltip"
             :style="{ top: inactiveTooltip.y + 'px', left: inactiveTooltip.x + 'px' }"
        >
            <div class="inactive-tooltip-content">
                {{ inactiveTooltip.text }}
            </div>
        </div>

        <!-- Модальное окно выбора существующих машин -->
        <ExistingCarsModal
            :visible="showExistingCarsModal"
            :already-added-vehicles="existingVehicles"
            :user-organization-id="userOrganizationId"
            :user-company-id="userCompanyId"
            :initial-selected-cars="selectedExistingCars"
            @cars-selected="onExistingCarsSelected"
            @close="closeExistingCarsModal"
        />
    </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { useFormValidation } from '@/composables/useFormValidation'
import { validatePartValue, formatPartValue, initializeNumberParts } from '@/composables/useNumberFormat'
import { getCurrentInstance } from 'vue'
import ExistingCarsModal from '@/components/CreateApplication/ExistingCarsModal.vue'

export default {
    name: 'VehicleForm',
    components: {
        ExistingCarsModal
    },
    props: {
        userOrganization: {
            type: String,
            default: ''
        },
        userOrganizationId: {
            type: Number,
            default: null
        },
        userCompany: {
            type: String,
            default: ''
        },
        userCompanyId: {
            type: Number,
            default: null
        },
        existingVehicles: {
            type: Array,
            default: () => []
        }
    },
    setup() {
        const instance = getCurrentInstance()

        const { isValid, tooltipMessage, showTooltip } = useFormValidation(() => {
            const vm = instance.proxy
            const hasInactiveSelected = vm.selectedUnloadingPlaces.some(placeId => {
                const place = vm.allUnloadingPlaces.find(p => p.id === placeId)
                return place && place.status !== 'active'
            })

            if (vm.selectedExistingCars.length > 0) {
                return [
                    { check: vm.selectedUnloadingPlaces.length > 0, message: 'хотя бы одно место разгрузки' }
                ]
            }

            return [
                { check: !vm.activeCarInfo || vm.isNumberByFact, message: 'На этот автомобиль уже есть активная заявка' },
                { check: (vm.isNumberByFact && vm.isMarkByFact) || !hasInactiveSelected, message: 'Невозможно выбрать неактивные места разгрузки' },
                { check: vm.isNumberByFact || !!vm.selectedFormat, message: 'формат номера' },
                {
                    check: vm.isNumberByFact || (
                        !!vm.selectedFormat &&
                        vm.numberParts.length > 0 &&
                        vm.numberParts.every((part, i) => {
                            const cell = vm.selectedFormat.cells[i]
                            return part && part.length >= cell.min_length && part.length <= cell.max_length
                        })
                    ),
                    message: 'номер Т/С'
                },
                { check: vm.isMarkByFact || !!vm.selectedMark, message: 'марка Т/С' },
                { check: vm.selectedUnloadingPlaces.length > 0, message: 'хотя бы одно место разгрузки' }
            ]
        })

        return { canAddVehicle: isValid, getTooltipMessage: tooltipMessage, showTooltip }
    },
    data() {
        return {
            numberParts: [],
            isNumberByFact: false,
            availableFormats: [],
            selectedFormat: null,
            isFormatDropdownOpen: false,
            isMarkByFact: false,
            selectedMark: '',
            isMarkDropdownOpen: false,
            markSearch: '',
            marks: [
                'ВАЗ', 'Мерседес', 'БМВ', 'Газель', 'ГАЗ', 'Вольво', 'Тойота', 'Митсубиси',
                'Ауди', 'Фольксваген', 'Шевроле', 'Хендай', 'Киа', 'Ниссан', 'Рено', 'Пежо',
                'Ситроен', 'Форд', 'Опель', 'Шкода', 'Лада', 'УАЗ'
            ],
            filteredMarks: [],
            allUnloadingPlaces: [],
            attachedUnloadingPlaces: [],
            selectedUnloadingPlaces: [],
            loadingUnloadingPlaces: false,
            errors: { unloadingPlaces: '' },
            allowedCyrillicLetters: ['А', 'В', 'Е', 'К', 'М', 'Н', 'О', 'Р', 'С', 'Т', 'У', 'Х'],
            allowedLatinLetters: ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z'],
            showExistingCarsModal: false,
            selectedExistingCars: [],
            editingVehicle: null,
            inactiveTooltip: {
                visible: false,
                text: '',
                x: 0,
                y: 0
            },
            // Новые поля для проверки активных заявок
            activeCarInfo: null,
            checkingTimeout: null
        }
    },
    computed: {
        selectedFormatText() {
            return this.selectedFormat ? this.selectedFormat.format.name : 'Выберите формат';
        },
        attachedPlacesIds() {
            return this.attachedUnloadingPlaces.map(place => place.id);
        },
        canAddExistingCars() {
            // Проверяем, что среди выбранных мест нет неактивных
            const hasInactiveSelected = this.selectedUnloadingPlaces.some(placeId => {
                const place = this.allUnloadingPlaces.find(p => p.id === placeId);
                return place && place.status !== 'active';
            });
            
            return this.selectedExistingCars.length > 0 && this.selectedUnloadingPlaces.length > 0 && !hasInactiveSelected;
        },
    },
    methods: {
        // Новый метод для проверки активной заявки
        async checkVehicleActive() {
            // Отменяем предыдущий таймаут
            if (this.checkingTimeout) {
                clearTimeout(this.checkingTimeout);
            }

            // Если не заполнены обязательные поля или выбрано "по факту", не проверяем
            if (this.isNumberByFact || !this.selectedFormat || !this.numberParts.every(part => part)) {
                this.activeCarInfo = null;
                return;
            }

            // Собираем номер из частей
            const plateNumber = this.numberParts.join(' ');

            // Ждем небольшую паузу, чтобы не дёргать сервер на каждый символ
            this.checkingTimeout = setTimeout(async () => {
                try {
                    const params = new URLSearchParams();
                    params.append('car_number', plateNumber);
                    params.append('car_brand', this.selectedMark || '');
                    if (this.userOrganizationId) params.append('organization_id', this.userOrganizationId);
                    if (this.userCompanyId) params.append('company_id', this.userCompanyId);

                    const response = await apiRequest(`/cars/check-active?${params.toString()}`, {});

                    if (response.ok) {
                        const data = await response.json();
                        if (data.active) {
                            this.activeCarInfo = data;
                        } else {
                            this.activeCarInfo = null;
                        }
                    }
                } catch (error) {
                    console.error('Ошибка при проверке активности авто:', error);
                    this.activeCarInfo = null;
                }
            }, 500);
        },

        async loadLicensePlateFormats() {
            try {
                const response = await apiRequest("/license-plate-formats", {
                    method: "GET"});

                if (response.ok) {
                    this.availableFormats = await response.json();
                    const defaultFormat = this.availableFormats.find(f => f.format.is_default);
                    this.selectedFormat = defaultFormat || this.availableFormats[0];
                    this.initializeNumberParts();
                } else {
                    console.error("Ошибка при загрузке форматов номеров");
                }
            } catch (error) {
                console.error("Ошибка при загрузке форматов номеров:", error);
            }
        },

        async loadUnloadingPlaces() {
            this.loadingUnloadingPlaces = true;
            this.allUnloadingPlaces = [];
            this.attachedUnloadingPlaces = [];
            this.selectedUnloadingPlaces = [];
            
            try {
                const authStore = useAuthStore();
                if (!authStore.token) {
                    console.error("Токен не найден");
                    return;
                }

                const allPlacesResponse = await apiRequest("/unload-places", {
                    method: "GET"});

                if (allPlacesResponse.ok) {
                    this.allUnloadingPlaces = await allPlacesResponse.json();
                }

                if (this.userOrganizationId) {
                    const orgPlacesResponse = await apiRequest(`/organizations/${this.userOrganizationId}/unload-places`, {
                        method: "GET"});

                    if (orgPlacesResponse.ok) {
                        this.attachedUnloadingPlaces = await orgPlacesResponse.json();
                        // Автоматически выбираем только активные привязанные места
                        const activeAttachedPlaces = this.attachedUnloadingPlaces.filter(place => place.status === 'active');
                        this.selectedUnloadingPlaces = activeAttachedPlaces.map(place => place.id);
                    }
                }

                if (this.attachedUnloadingPlaces.length === 0 && this.userCompanyId) {
                    const companyPlacesResponse = await apiRequest(`/companies/${this.userCompanyId}/unload-places`, {
                        method: "GET"});

                    if (companyPlacesResponse.ok) {
                        this.attachedUnloadingPlaces = await companyPlacesResponse.json();
                        // Автоматически выбираем только активные привязанные места
                        const activeAttachedPlaces = this.attachedUnloadingPlaces.filter(place => place.status === 'active');
                        this.selectedUnloadingPlaces = activeAttachedPlaces.map(place => place.id);
                    }
                }

                this.validateUnloadingPlaces();

            } catch (error) {
                console.error("Ошибка при загрузке мест разгрузки:", error);
                this.allUnloadingPlaces = [];
                this.attachedUnloadingPlaces = [];
            } finally {
                this.loadingUnloadingPlaces = false;
            }
        },

        getPlaceTooltip(place) {
            if (place.status !== 'active') {
                if (place.status_comment) {
                    return `Недоступно: ${place.status_comment}`;
                }
                return 'Недоступно';
            }
            return '';
        },

        showInactiveTooltip(place, event) {
            if (place.status !== 'active') {
                const tooltipText = place.status_comment 
                    ? `Недоступно: ${place.status_comment}`
                    : 'Недоступно';
                
                this.inactiveTooltip.text = tooltipText;
                this.inactiveTooltip.visible = true;
                
                // Позиционируем тултип
                this.$nextTick(() => {
                    const rect = event.target.getBoundingClientRect();
                    this.inactiveTooltip.x = rect.left + rect.width / 2;
                    this.inactiveTooltip.y = rect.top - 10;
                });
            }
        },

        hideInactiveTooltip() {
            this.inactiveTooltip.visible = false;
        },

        initializeNumberParts() {
            this.numberParts = initializeNumberParts(this.selectedFormat);
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
            const value = validatePartValue(event.target.value, cell);
            this.numberParts[index] = value;
            event.target.value = value;

            // Проверяем активность после изменения номера
            this.checkVehicleActive();
        },

        formatPart(index, cell) {
            if (this.numberParts[index]) {
                const formatted = formatPartValue(this.numberParts[index], cell);
                if (formatted !== this.numberParts[index]) {
                    this.numberParts[index] = formatted;
                }
            }
        },

        handleNumberByFactChange() {
            if (this.isNumberByFact) {
                this.numberParts = [];
                this.activeCarInfo = null; // Сбрасываем информацию об активной заявке
            } else {
                this.initializeNumberParts();
            }
        },
        
        handleMarkByFactChange() {
            if (this.isMarkByFact) {
                this.selectedMark = '';
            }
        },
        
        toggleUnloadingPlace(place) {
            // Не даем выбрать неактивное место
            if (place.status !== 'active') {
                return;
            }
            
            const index = this.selectedUnloadingPlaces.indexOf(place.id);
            if (index > -1) {
                this.selectedUnloadingPlaces.splice(index, 1);
            } else {
                this.selectedUnloadingPlaces.push(place.id);
            }
        },
        
        validateUnloadingPlaces() {
            this.errors.unloadingPlaces = this.selectedUnloadingPlaces.length === 0 ? '' : '';
        },
        
        formatUnloadingPlaces() {
            if (this.selectedUnloadingPlaces.length === 0) return '';
            
            const placeNames = this.selectedUnloadingPlaces.map(placeId => {
                const place = this.allUnloadingPlaces.find(p => p.id === placeId);
                return place ? place.name : '';
            }).filter(name => name);
            
            if (placeNames.length > 1) {
                return placeNames[0] + ' и др.';
            }
            
            return placeNames[0] || '';
        },

        formatDate(dateString) {
            if (!dateString) return '';
            const [year, month, day] = dateString.split('-');
            return `${day}.${month}.${year}`;
        },

        formatTime(timeString) {
            if (!timeString) return '';
            return timeString.substring(0, 5);
        },
        
        addVehicle() {
            if (!this.canAddVehicle) {
                return;
            }

            // Проверка активной заявки
            if (this.activeCarInfo && !this.isNumberByFact) {
                alert('Невозможно добавить автомобиль, на который уже есть активная заявка');
                return;
            }
            
            if (this.selectedExistingCars.length > 0) {
                this.addExistingCars();
                return;
            }
            
            let plateNumber = '';
            if (this.isNumberByFact) {
                plateNumber = 'По факту';
            } else {
                plateNumber = this.numberParts.join(' ');
            }
            
            const mark = this.isMarkByFact ? 'По факту' : this.selectedMark;

            const newVehicle = {
                plateNumber: plateNumber,
                mark: mark,
                unloadingPlace: this.formatUnloadingPlaces(),
                unloadPlaces: [...this.selectedUnloadingPlaces],
                formatId: this.selectedFormat ? this.selectedFormat.format.id : null,
                isExisting: false
            };
            
            if (this.editingVehicle) {
                newVehicle.id = this.editingVehicle.id;
                this.$emit('vehicle-updated', newVehicle);
                this.cancelEdit();
            } else {
                this.$emit('vehicle-added', newVehicle);
                this.clearVehicleFormPartial();
            }
        },
        
        clearVehicleFormPartial() {
            this.initializeNumberParts();
            this.selectedMark = '';
            this.isNumberByFact = false;
            this.isMarkByFact = false;
            this.activeCarInfo = null; // Сбрасываем информацию об активной заявке
        },
        
        clearVehicleForm() {
            this.initializeNumberParts();
            this.selectedMark = '';
            this.selectedUnloadingPlaces = [];
            this.isNumberByFact = false;
            this.isMarkByFact = false;
            this.errors.unloadingPlaces = '';
            this.selectedExistingCars = [];
            this.editingVehicle = null;
            this.activeCarInfo = null; // Сбрасываем информацию об активной заявке
        },

        openExistingCarsModal() {
            this.showExistingCarsModal = true;
        },

        closeExistingCarsModal() {
            this.showExistingCarsModal = false;
        },

        onExistingCarsSelected(cars) {
            this.selectedExistingCars = cars;
            this.showExistingCarsModal = false;
            this.clearVehicleFormPartial();
        },

        addExistingCars() {
            if (this.selectedExistingCars.length === 0) {
                alert('Выберите машины для добавления');
                return;
            }

            if (this.selectedUnloadingPlaces.length === 0) {
                alert('Выберите места разгрузки');
                return;
            }

            const vehicles = this.selectedExistingCars.map(car => ({
                plateNumber: car.number,
                mark: car.mark,
                unloadingPlace: this.formatUnloadingPlaces(),
                unloadPlaces: [...this.selectedUnloadingPlaces],
                formatId: car.format_id,
                isExisting: true,
                existingCarId: car.id
            }));
            
            this.$emit('vehicles-added', vehicles);
            this.clearExistingCarsSelection();
        },

        clearExistingCarsSelection() {
            this.selectedExistingCars = [];
        },

        editVehicle(vehicle) {
            this.editingVehicle = vehicle;
            this.selectedExistingCars = [];
            this.activeCarInfo = null; // Сбрасываем информацию об активной заявке
            
            if (vehicle.isExisting) {
                this.selectedMark = vehicle.mark;
                this.isMarkByFact = vehicle.mark === 'По факту';
                this.isNumberByFact = vehicle.plateNumber === 'По факту';
                this.selectedUnloadingPlaces = vehicle.unloadPlaces || [];
                
                if (vehicle.formatId) {
                    const format = this.availableFormats.find(f => f.format.id === vehicle.formatId);
                    if (format) {
                        this.selectedFormat = format;
                    }
                }
            } else {
                if (vehicle.plateNumber === 'По факту') {
                    this.isNumberByFact = true;
                } else {
                    this.isNumberByFact = false;
                    const format = this.availableFormats.find(f => f.format.id === vehicle.formatId);
                    if (format) {
                        this.selectedFormat = format;
                        this.numberParts = vehicle.plateNumber.split(' ');
                    }
                }
                
                if (vehicle.mark === 'По факту') {
                    this.isMarkByFact = true;
                } else {
                    this.isMarkByFact = false;
                    this.selectedMark = vehicle.mark;
                }
                
                this.selectedUnloadingPlaces = vehicle.unloadPlaces || [];
            }
        },

        cancelEdit() {
            this.$emit('edit-cancelled');
            this.editingVehicle = null;
            this.clearVehicleForm();
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

            // Проверяем активность после выбора марки
            this.checkVehicleActive();
        },
        
        toggleFormatDropdown() {
            this.isFormatDropdownOpen = !this.isFormatDropdownOpen;
        },
        
        selectFormat(format) {
            this.selectedFormat = format;
            this.initializeNumberParts();
            this.isFormatDropdownOpen = false;

            // Проверяем активность после смены формата
            this.checkVehicleActive();
        }
    },
    watch: {
        // Следим за изменениями частей номера для проверки активности
        numberParts: {
            deep: true,
            handler() {
                this.checkVehicleActive();
            }
        }
    },
    async mounted() {
        await Promise.all([
            this.loadLicensePlateFormats(),
            this.loadUnloadingPlaces()
        ]);
        
        this.filteredMarks = this.marks;

        document.addEventListener('click', (e) => {
            if (!e.target.closest('.format__dropdown')) {
                this.isFormatDropdownOpen = false;
            }
            
            if (!e.target.closest('.mark__dropdown')) {
                this.isMarkDropdownOpen = false;
            }
        });
    },
    beforeUnmount() {
        if (this.checkingTimeout) {
            clearTimeout(this.checkingTimeout);
        }
    }
}
</script>

<style scoped>
.input__label {
    font-size: 13px;
    color: #a2a2a2;
}

.required {
    color: #ff4444;
}

.data__completion {
    padding: 15px;
    width: 450px;
    border-right: 1px solid #e6e6e6;
}

.completion__format {
    display: flex;
    flex-direction: column;
    gap: 10px;
    position: relative;
    padding-bottom: 15px;
}

.format__header {
    display: flex;
    justify-content: space-between;
    align-items: end;
}

.format__label {
    font-size: 13px;
    color: #a2a2a2;
}

.format-actions {
    display: flex;
    gap: 10px;
    align-items: center;
    position: relative;
}

.cancel-edit-btn {
    background: #f8f8f8;
    color: #333;
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    padding: 8px 15px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.cancel-edit-btn:hover {
    background: #e8e8e8;
}

.add-button {
    background: #4F5BDF;
    color: white;
    border: none;
    border-radius: 15px;
    padding: 8px 15px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
    margin-top: 0;
    position: relative;
}

.add-button:hover:not(:disabled) {
    background: #3a45c0;
}

.add-button:disabled {
    background: #a2a2a2;
    cursor: not-allowed;
    opacity: 0.6;
}

.tooltip {
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: 5px;
    z-index: 1000;
}

.tooltip-content {
    background: #333;
    color: white;
    padding: 10px 12px;
    border-radius: 8px;
    font-size: 12px;
    max-width: 500px;
    white-space: nowrap;
    box-shadow: 0 2px 8px rgba(0,0,0,0.2);
}

.tooltip-content::before {
    content: '';
    position: absolute;
    bottom: 100%;
    right: 40px;
    border: 5px solid transparent;
    border-bottom-color: #333;
}

.inactive-tooltip {
    position: fixed;
    transform: translateX(-50%) translateY(-100%);
    z-index: 10000;
    pointer-events: none;
}

.inactive-tooltip-content {
    background: #333;
    color: white;
    padding: 8px 12px;
    border-radius: 8px;
    font-size: 12px;
    max-width: 300px;
    box-shadow: 0 2px 8px rgba(0,0,0,0.2);
}

.inactive-tooltip-content::before {
    content: '';
    position: absolute;
    top: 100%;
    left: 50%;
    transform: translateX(-50%);
    border: 5px solid transparent;
    border-top-color: #333;
}

.active-warning {
    width: 100%;
    margin-top: 10px;
    padding: 12px;
    background: #fff3cd;
    border: 1px solid #ffeeba;
    border-radius: 10px;
    display: flex;
    gap: 12px;
    align-items: flex-start;
}

.warning-icon {
    font-size: 20px;
    flex-shrink: 0;
}

.warning-text {
    flex: 1;
}

.warning-title {
    font-weight: 600;
    color: #856404;
    margin: 0 0 5px 0;
    font-size: 14px;
}

.warning-details {
    color: #856404;
    margin: 0;
    font-size: 12px;
    line-height: 1.5;
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

.dropdown__button:hover:not(:disabled) {
    border-color: #4F5BDF;
}

.dropdown__button:disabled {
    background-color: #f5f5f5;
    cursor: not-allowed;
    opacity: 0.6;
}

.button__content {
    display: flex;
    align-items: center;
    width: 100%;
    height: 100%;
    justify-content: space-between;
}

.completion__header {
    padding-bottom: 15px;
    display: flex;
    justify-content: space-between;
}

.button__text {
    font-size: 14px;
    color: #000;
    font-weight: 500;
    display: block;
}

.button__arrow {
    width: 10px;
    height: 10px;
    transition: transform 0.2s;
    transform: rotate(90deg);
    flex-shrink: 0;
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
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
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

.number-fact,
.mark-fact {
    display: flex;
    align-items: center;
    gap: 5px;
}

.fact-checkbox {
    width: 12px;
    height: 12px;
    cursor: pointer;
}

.fact-checkbox:disabled {
    cursor: not-allowed;
    opacity: 0.6;
}

.fact-text {
    font-size: 13px;
}

.number__field {
    max-width: 202px;
    min-width: 202px;
    height: 40px;
    display: flex;
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    overflow: hidden;
    background: #FFF;
}

.number__field--fact {
    display: block;
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

.number__input:disabled {
    background-color: #f5f5f5;
    cursor: not-allowed;
}

.number__input--fact {
    width: 100%;
    text-align: left;
    padding: 0 15px;
    color: #a2a2a2;
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

.no-format-message {
    font-size: 12px;
    color: #a2a2a2;
    text-align: center;
    padding: 10px;
    background: #f8f8f8;
    border-radius: 10px;
    border: 1px solid #e6e6e6;
}

.mark__field {
    width: 100%;
    height: 40px;
    position: relative;
}

.mark__field--fact {
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    overflow: hidden;
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
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 150px;
    display: block;
}

.mark__button-arrow {
    width: 10px;
    height: 10px;
    transition: transform 0.2s;
    transform: rotate(90deg);
    flex-shrink: 0;
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
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.mark__input {
    width: 100%;
    height: 100%;
    border: none;
    outline: none;
    background: transparent;
    padding: 0 15px;
    font-size: 14px;
    color: #a2a2a2;
}

.mark__input--fact::placeholder {
    color: #a2a2a2;
    font-size: 12px;
}

.completion__unloading {
    margin-top: 15px;
}

.unloading-info {
    margin-bottom: 10px;
}

.unloading-source {
    font-size: 12px;
    color: #4F5BDF;
    font-weight: 500;
    background: #f0f2ff;
    padding: 5px 10px;
    border-radius: 8px;
    display: inline-block;
}

.attached-count {
    font-size: 11px;
    color: #666;
    font-weight: normal;
}

.attached-badge {
    position: absolute;
    top: 2px;
    right: 2px;
    background: #4F5BDF;
    color: white;
    border-radius: 50%;
    width: 16px;
    height: 16px;
    font-size: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.unloading__grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
    row-gap: 5px;
    max-width: 425px;
    margin-top: 5px;
    position: relative;
}

.unloading__item {
    height: 30px;
    background: #F2F2F2;
    color: #a2a2a2;
    border-radius: 50px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s;
    padding: 0 10px;
    text-align: center;
    border: 1px solid transparent;
    position: relative;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.unloading__item:hover:not(.unloading__item--active):not(.unloading__item--inactive) {
    background: #e8e8e8;
}

.unloading__item--active {
    background: #4F5BDF;
    color: #fff;
    border-color: #4F5BDF;
}

.unloading__item--inactive {
    background: #ffe6e6;
    color: #ff6b6b;
    border-color: #ffcccc;
    cursor: not-allowed;
    opacity: 0.7;
}

.unloading__item--attached {
    border-left: 3px solid #4F5BDF;
}

.error-message {
    font-size: 11px;
    color: #ff4444;
    margin-top: 5px;
}

.loading-message {
    font-size: 12px;
    color: #a2a2a2;
    text-align: center;
    padding: 20px;
}

.no-places-message {
    font-size: 12px;
    color: #ff6b6b;
    text-align: center;
    padding: 20px;
    background: #fff5f5;
    border-radius: 8px;
    margin-top: 10px;
}

.completion__button {
    width: fit-content;
    height: 25px;
    padding: 0 15px;
    border-radius: 50px;
    background: #FFF;
    border: 1px solid #e6e6e6;
    outline: none;
    font-size: 11px;
    color: #4F5BDF;
    font-weight: 600;
    cursor: pointer;
}

.completion__button:hover {
    background-color: #f5f5f5;
}

.existing-cars-info {
    margin-bottom: 15px;
    padding: 10px;
    background: #f8f9fa;
    border-radius: 10px;
    border: 1px solid #e6e6e6;
}

.existing-cars-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.existing-cars-count {
    font-size: 14px;
    font-weight: 500;
    color: #333;
}

.existing-cars-actions {
    display: flex;
    gap: 10px;
}

.view-cars-btn {
    background: white;
    color: #4F5BDF;
    border: 1px solid #4F5BDF;
    border-radius: 15px;
    padding: 5px 10px;
    font-size: 11px;
    cursor: pointer;
}

.view-cars-btn:hover {
    background: #f0f2ff;
}

.add-existing-btn {
    background: #4F5BDF;
    color: white;
    border: none;
    border-radius: 15px;
    padding: 5px 10px;
    font-size: 11px;
    cursor: pointer;
}

.add-existing-btn:hover:not(:disabled) {
    background: #3a45c0;
}

.add-existing-btn:disabled {
    background: #a2a2a2;
    cursor: not-allowed;
    opacity: 0.6;
}

.dropdown-enter-active,
.dropdown-leave-active {
    transition: all 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
    opacity: 0;
    transform: translateY(-10px);
}
</style>