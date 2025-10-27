<template>
    <div class="create">
        <div class="create__title">
            <h3>Оформление и подача заявки</h3>
            <button class="tables__instruction">
                <img src="@/assets/icons/instruction.png" class="tables__icon" />
                <p class="instruction__text">Инструкция</p>
            </button>
        </div>
        <div class="create__container">
            <BlankSelector />
            <div class="create__form">
                <div class="form__header">
                    <div class="header__content">
                        <textarea 
                            placeholder="Введите сопроводительное письмо / сообщение" 
                            class="form__textarea"
                            v-model="message"
                        ></textarea>
                        <div class="header__right">
                            <div class="consent-section">
                                <div class="consent-checkbox">
                                    <input 
                                        type="checkbox" 
                                        id="consent"
                                        v-model="consentGiven"
                                        required
                                    />
                                    <label for="consent">
                                        Даю <span class="blue">согласие</span> на обработку, хранение, передачу
                                        персональных данных, изложенных в заявке
                                    </label>
                                </div>
                                <button class="send-all-btn" @click="submitApplication" :disabled="!canSubmit">
                                    Отправить заявку
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
                <div class="form__info">
                    <h4>Автозаявка №{{ applicationNumber }}</h4>
                    <div class="info__user">
                        <div class="user__input">
                            <label class="input__label">Организация / Отдел <span class="required">*</span></label>
                            <input 
                                class="input" 
                                placeholder="Введите организацию" 
                                v-model="organization"
                                @blur="validateField('organization')"
                                :class="{ 'input--error': errors.organization }"
                            />
                            <div v-if="errors.organization" class="error-message">{{ errors.organization }}</div>
                        </div>
                        <div class="user__input">
                            <label class="input__label">Компания <span class="required">*</span></label>
                            <input 
                                class="input" 
                                placeholder="Введите компанию" 
                                v-model="company"
                                @blur="validateField('company')"
                                :class="{ 'input--error': errors.company }"
                            />
                            <div v-if="errors.company" class="error-message">{{ errors.company }}</div>
                        </div>
                        <div class="user__input responsible">
                            <label class="input__label responsible">Ответственное лицо <span class="required">*</span></label>
                            <div class="input contacts" :class="{ 'input--error': errors.responsiblePerson || errors.phone }">
                                <input 
                                    class="contact-input" 
                                    placeholder="Введите ФИО" 
                                    v-model="responsiblePerson"
                                    @blur="validateField('responsiblePerson')"
                                />
                                <input 
                                    class="contact-input phone" 
                                    placeholder="Номер телефона"
                                    v-model="phoneNumber"
                                    @blur="formatPhoneNumber"
                                    @focus="clearPhoneFormat"
                                    @input="validateField('phone')"
                                />
                            </div>
                            <div v-if="errors.responsiblePerson" class="error-message">{{ errors.responsiblePerson }}</div>
                            <div v-if="errors.phone" class="error-message">{{ errors.phone }}</div>
                        </div>
                    </div>
                    <div class="form__date">
                        <div class="date__input">
                            <label class="input__label">Дата действия <span class="required">*</span></label>
                            <div class="date-container">
                                <div class="date" v-if="!isOneDay">
                                    <p class="date__text">с</p>
                                    <div class="datepicker-wrapper">
                                        <input 
                                            class="input__date" 
                                            placeholder="дд.мм.гг" 
                                            v-model="startDate"
                                            @focus="openDatepicker('start')"
                                            @blur="validateDateRange"
                                            :class="{ 'input--error': errors.startDate }"
                                            readonly
                                        />
                                        <div v-if="showStartDatepicker" class="datepicker">
                                            <div class="datepicker__header">
                                                <button @click="prevMonth" class="datepicker__nav">
                                                    <img src="@/assets/icons/arrow.png" class="datepicker__arrow datepicker__arrow--left" />
                                                </button>
                                                <span class="datepicker__month">{{ currentMonth }} {{ currentYear }}</span>
                                                <button @click="nextMonth" class="datepicker__nav">
                                                    <img src="@/assets/icons/arrow.png" class="datepicker__arrow" />
                                                </button>
                                            </div>
                                            <div class="datepicker__weekdays">
                                                <div v-for="day in weekdays" :key="day" class="datepicker__weekday">{{ day }}</div>
                                            </div>
                                            <div class="datepicker__days">
                                                <div 
                                                    v-for="day in calendarDays" 
                                                    :key="day.date"
                                                    class="datepicker__day"
                                                    :class="{
                                                        'datepicker__day--selected': isSelectedDate(day.date),
                                                        'datepicker__day--other-month': !day.isCurrentMonth,
                                                        'datepicker__day--today': isToday(day.date)
                                                    }"
                                                    @click="selectStartDate(day.date)"
                                                >
                                                    {{ day.day }}
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                    <p class="date__text">по</p>
                                    <div class="datepicker-wrapper">
                                        <input 
                                            class="input__date" 
                                            placeholder="дд.мм.гг" 
                                            v-model="endDate"
                                            @focus="openDatepicker('end')"
                                            @blur="validateDateRange"
                                            :class="{ 'input--error': errors.endDate }"
                                            readonly
                                        />
                                        <div v-if="showEndDatepicker" class="datepicker">
                                            <div class="datepicker__header">
                                                <button @click="prevMonth" class="datepicker__nav">
                                                    <img src="@/assets/icons/arrow.png" class="datepicker__arrow datepicker__arrow--left" />
                                                </button>
                                                <span class="datepicker__month">{{ currentMonth }} {{ currentYear }}</span>
                                                <button @click="nextMonth" class="datepicker__nav">
                                                    <img src="@/assets/icons/arrow.png" class="datepicker__arrow" />
                                                </button>
                                            </div>
                                            <div class="datepicker__weekdays">
                                                <div v-for="day in weekdays" :key="day" class="datepicker__weekday">{{ day }}</div>
                                            </div>
                                            <div class="datepicker__days">
                                                <div 
                                                    v-for="day in calendarDays" 
                                                    :key="day.date"
                                                    class="datepicker__day"
                                                    :class="{
                                                        'datepicker__day--selected': isSelectedDate(day.date),
                                                        'datepicker__day--other-month': !day.isCurrentMonth,
                                                        'datepicker__day--today': isToday(day.date)
                                                    }"
                                                    @click="selectEndDate(day.date)"
                                                >
                                                    {{ day.day }}
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                                <div v-else class="single-date">
                                    <div class="datepicker-wrapper">
                                        <input 
                                            class="input__date" 
                                            placeholder="дд.мм.гг" 
                                            v-model="singleDate"
                                            @focus="openDatepicker('single')"
                                            @blur="validateField('singleDate')"
                                            :class="{ 'input--error': errors.singleDate }"
                                            readonly
                                        />
                                        <div v-if="showSingleDatepicker" class="datepicker">
                                            <div class="datepicker__header">
                                                <button @click="prevMonth" class="datepicker__nav">
                                                    <img src="@/assets/icons/arrow.png" class="datepicker__arrow datepicker__arrow--left" />
                                                </button>
                                                <span class="datepicker__month">{{ currentMonth }} {{ currentYear }}</span>
                                                <button @click="nextMonth" class="datepicker__nav">
                                                    <img src="@/assets/icons/arrow.png" class="datepicker__arrow" />
                                                </button>
                                            </div>
                                            <div class="datepicker__weekdays">
                                                <div v-for="day in weekdays" :key="day" class="datepicker__weekday">{{ day }}</div>
                                            </div>
                                            <div class="datepicker__days">
                                                <div 
                                                    v-for="day in calendarDays" 
                                                    :key="day.date"
                                                    class="datepicker__day"
                                                    :class="{
                                                        'datepicker__day--selected': isSelectedDate(day.date),
                                                        'datepicker__day--other-month': !day.isCurrentMonth,
                                                        'datepicker__day--today': isToday(day.date)
                                                    }"
                                                    @click="selectSingleDate(day.date)"
                                                >
                                                    {{ day.day }}
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                                <div v-if="errors.startDate || errors.endDate || errors.singleDate" class="error-message">
                                    {{ errors.startDate || errors.endDate || errors.singleDate }}
                                </div>
                            </div>
                            <div class="one-day">
                                <input 
                                    type="checkbox" 
                                    class="one-day__checkbox" 
                                    v-model="isOneDay"
                                    @change="handleOneDayChange"
                                />
                                <p>однодневная заявка</p>
                            </div>
                        </div>
                        <div class="date__input">
                            <label class="input__label">Время пребывания (проезда) <span class="required">*</span></label>
                            <div class="date">
                                <p class="date__text">с</p>
                                <input 
                                    class="input__date" 
                                    placeholder="00:00" 
                                    v-model="startTime"
                                    @blur="validateTimeRange"
                                    :class="{ 'input--error': errors.startTime }"
                                    type="time"
                                />
                                <p class="date__text">по</p>
                                <input 
                                    class="input__date" 
                                    placeholder="00:00" 
                                    v-model="endTime"
                                    @blur="validateTimeRange"
                                    :class="{ 'input--error': errors.endTime }"
                                    type="time"
                                />
                            </div>
                            <p class="time-message">Укажите время, в которое должна будет действовать заявка и будет разрешен въезд</p>
                            <div v-if="errors.startTime || errors.endTime" class="error-message">
                                {{ errors.startTime || errors.endTime }}
                            </div>
                        </div>
                    </div>
                </div>
                <div class="form__data">
                    <VehicleForm 
                        :user-organization="organization"
                        :user-organization-id="organizationId"
                        :user-company="company"
                        :user-company-id="companyId"
                        :existing-vehicles="vehicles"
                        :key="vehicleFormKey"
                        @vehicle-added="handleVehicleAdded"
                        @vehicles-added="handleVehiclesAdded"
                        @vehicle-updated="handleVehicleUpdated"
                        @edit-cancelled="handleEditCancelled"
                        ref="vehicleForm"
                    />
                    <div class="data__list">
                        <h4>Список транспортных средств ({{ vehicles.length }})</h4>
                        <div class="vehicles-table">
                            <div class="table-header">
                                <div class="header-col number-col" @click="sortBy('number')">
                                    <p :class="{ 'active-sort': sortField === 'number' }">№</p>
                                    <img 
                                        src="@/assets/icons/sort.png" 
                                        class="sort-icon" 
                                        :class="{ 
                                            'sorted': sortField === 'number',
                                            'desc': sortField === 'number' && sortDirection === 'desc'
                                        }" 
                                    />
                                </div>
                                <div class="header-col plate-col" @click="sortBy('plate')">
                                    <p :class="{ 'active-sort': sortField === 'plate' }">Номер</p>
                                    <img 
                                        src="@/assets/icons/sort.png" 
                                        class="sort-icon" 
                                        :class="{ 
                                            'sorted': sortField === 'plate',
                                            'desc': sortField === 'plate' && sortDirection === 'desc'
                                        }" 
                                    />
                                </div>
                                <div class="header-col mark-col" @click="sortBy('mark')">
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
                                <div class="header-col place-col" @click="sortBy('place')">
                                    <p :class="{ 'active-sort': sortField === 'place' }">Место разгрузки</p>
                                    <img 
                                        src="@/assets/icons/sort.png" 
                                        class="sort-icon" 
                                        :class="{ 
                                            'sorted': sortField === 'place',
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
                                    v-for="(vehicle, index) in sortedVehicles" 
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
                                            @click="editVehicle(vehicle)"
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
                                            @click="deleteVehicle(vehicle.id)"
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
                </div>
            </div>
        </div>

        <!-- Модальное окно привязки новых машин -->
        <div v-if="showBindingModal" class="modal-overlay" @click="closeBindingModal">
            <div class="modal-content" @click.stop>
                <div class="modal-header">
                    <div class="modal-header__top">
                        <h3>Привязка новых автомобилей</h3>
                    </div>
                    <button class="modal-close" @click="closeBindingModal">×</button>
                </div>
                <div class="modal-body">
                    <div class="binding-info">
                        <p class="binding-description">
                            Все добавленные автомобили ниже <strong>автоматически привязываются</strong> к вашему аккаунту.
                            Вы можете выбрать и привязать автомобили к организации и/или компании для использования <strong>другими сотрудниками</strong>:
                        </p>
                        
                        <div class="cars-list-section">
                            <p class="section-title">Список новых автомобилей:</p>
                            <div class="cars-list">
                                <div 
                                    v-for="car in newCarsToBind" 
                                    :key="car.plateNumber"
                                    class="car-item"
                                    :class="{ 'car-item--shared': car.bindToEntity }"
                                    @click="toggleCarBinding(car)"
                                >
                                    <div class="car-selector">
                                        <div class="selector-checkbox">
                                            <div class="checkbox" :class="{ 'checkbox--checked': car.bindToEntity }"></div>
                                        </div>
                                        <div class="car-info">
                                            <span class="car-number">{{ car.plateNumber }}</span>
                                            <span class="car-mark">{{ car.mark }}</span>
                                        </div>
                                    </div>
                                    <div class="car-binding-status">
                                        <span v-if="car.bindToEntity" class="status-shared">
                                            Будет доступна
                                        </span>
                                        <span v-else class="status-private">
                                            Привязка только к вам
                                        </span>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div class="binding-options-section">
                            <p class="section-title">Привязать выбранные автомобили к:</p>
                            <div class="binding-options">
                                <label class="binding-option" v-if="hasOrganization">
                                    <input 
                                        type="checkbox" 
                                        v-model="bindToOrganization"
                                        :disabled="bindToCompany"
                                    />
                                    <span class="option-text">К организации "{{ organization }}"</span>
                                </label>
                                <label class="binding-option" v-if="hasCompany">
                                    <input 
                                        type="checkbox" 
                                        v-model="bindToCompany"
                                        :disabled="bindToOrganization"
                                    />
                                    <span class="option-text">К компании "{{ company }}"</span>
                                </label>
                            </div>
                        </div>

                        <div class="warning-section">
                            <p class="warning-text">
                                <strong class="red">Внимание!</strong> При привязке автомобиля к организации или компании, он будет доступен для отображения и использования для всех сотрудников, привязанных к организации/компании.
                            </p>
                        </div>
                    </div>
                    
                    <div class="modal-actions">
                        <button class="cancel-btn" @click="skipBinding">Пропустить</button>
                        <button class="confirm-btn" @click="confirmBinding">Привязать и отправить</button>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script>
import VehicleForm from '@/components/VehicleForm.vue'
import BlankSelector from './BlankSelector.vue';

export default {
    name: 'CreateApplication',
    components: {
        VehicleForm,
        BlankSelector
    },
    data() {
        const today = new Date();
        return {
            // Form data
            message: '',
            organization: '',
            company: '',
            responsiblePerson: '',
            phoneNumber: '',
            rawPhoneNumber: '',
            isOneDay: false,
            startDate: '',
            endDate: '',
            singleDate: '',
            startTime: '',
            endTime: '',
            consentGiven: false,
            applicationNumber: 1,

            // IDs
            organizationId: null,
            companyId: null,
            
            // Vehicles list
            vehicles: [],
            vehicleIdCounter: 1,
            
            // Sorting
            sortField: null,
            sortDirection: 'asc',
            
            // Validation
            errors: {
                organization: '',
                company: '',
                responsiblePerson: '',
                phone: '',
                startDate: '',
                endDate: '',
                singleDate: '',
                startTime: '',
                endTime: '',
                unloadingPlaces: ''
            },
            
            // Datepicker
            showStartDatepicker: false,
            showEndDatepicker: false,
            showSingleDatepicker: false,
            currentDate: today,
            weekdays: ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'],
            
            // Key for forcing VehicleForm re-render
            vehicleFormKey: 0,

            // Привязка новых машин
            showBindingModal: false,
            newCarsToBind: [],
            bindToOrganization: false,
            bindToCompany: false,
            hasOrganization: false,
            hasCompany: false,
            
            // Флаг для отслеживания процесса привязки
            bindingInProgress: false
        }
    },
    computed: {
        currentYear() {
            return this.currentDate.getFullYear();
        },
        currentMonth() {
            const months = [
                'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
                'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь'
            ];
            return months[this.currentDate.getMonth()];
        },
        calendarDays() {
            const year = this.currentDate.getFullYear();
            const month = this.currentDate.getMonth();
            
            const firstDay = new Date(year, month, 1);
            const lastDay = new Date(year, month + 1, 0);
            
            const days = [];
            
            const prevMonthLastDay = new Date(year, month, 0).getDate();
            const firstDayOfWeek = firstDay.getDay() === 0 ? 6 : firstDay.getDay() - 1;
            
            for (let i = firstDayOfWeek - 1; i >= 0; i--) {
                const date = new Date(year, month - 1, prevMonthLastDay - i);
                days.push({
                    day: date.getDate(),
                    date: this.formatDate(date),
                    isCurrentMonth: false
                });
            }
            
            for (let i = 1; i <= lastDay.getDate(); i++) {
                const date = new Date(year, month, i);
                days.push({
                    day: i,
                    date: this.formatDate(date),
                    isCurrentMonth: true
                });
            }
            
            const totalCells = 42;
            const nextMonthDays = totalCells - days.length;
            for (let i = 1; i <= nextMonthDays; i++) {
                const date = new Date(year, month + 1, i);
                days.push({
                    day: i,
                    date: this.formatDate(date),
                    isCurrentMonth: false
                });
            }
            
            return days;
        },
        canSubmit() {
            // Проверка обязательных полей
            const hasRequiredFields = 
                this.organization && 
                this.company && 
                this.responsiblePerson && 
                this.phoneNumber &&
                this.vehicles.length > 0 &&
                this.consentGiven;

            // Проверка дат в зависимости от типа заявки
            const hasValidDates = this.isOneDay 
                ? this.singleDate && this.startTime && this.endTime
                : this.startDate && this.endDate && this.startTime && this.endTime;

            // Проверка времени
            const hasValidTime = this.startTime && this.endTime && this.startTime < this.endTime;

            return hasRequiredFields && hasValidDates && hasValidTime;
        },
        sortedVehicles() {
            if (!this.sortField) {
                return this.vehicles;
            }
            
            return [...this.vehicles].sort((a, b) => {
                let valueA, valueB;
                
                switch (this.sortField) {
                    case 'number':
                        return this.sortDirection === 'asc' ? a.id - b.id : b.id - a.id;
                    case 'plate':
                        valueA = a.plateNumber.toLowerCase();
                        valueB = b.plateNumber.toLowerCase();
                        break;
                    case 'mark':
                        valueA = a.mark.toLowerCase();
                        valueB = b.mark.toLowerCase();
                        break;
                    case 'place':
                        valueA = a.unloadingPlace.toLowerCase();
                        valueB = b.unloadingPlace.toLowerCase();
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
        }
    },
    watch: {
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
        async loadUserData() {
            const token = localStorage.getItem("token");
            if (!token) {
                console.error("Токен не найден");
                return;
            }

            try {
                const response = await fetch("http://localhost:8080/user-data", {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`
                    }
                });

                if (response.ok) {
                    const userData = await response.json();
                    // Автозаполнение данных пользователя
                    this.organization = userData.organization || '';
                    this.company = userData.company || '';
                    
                    // Сохраняем ID организации и компании если они есть в ответе
                    this.organizationId = userData.organization_id || null;
                    this.companyId = userData.company_id || null;
                    
                    // Проверяем наличие организации и компании
                    this.hasOrganization = !!this.organizationId;
                    this.hasCompany = !!this.companyId;
                    
                    // Формирование ФИО
                    const lastName = userData.last_name || '';
                    const firstName = userData.first_name || '';
                    const middleName = userData.middle_name || '';
                    this.responsiblePerson = `${lastName} ${firstName} ${middleName}`.trim();
                    
                    // Форматирование телефона сразу при загрузке
                    this.phoneNumber = userData.phone || '';
                    if (this.phoneNumber) {
                        this.formatPhoneNumberImmediately(this.phoneNumber);
                    }
                    
                    // Принудительно обновляем VehicleForm после загрузки данных пользователя
                    this.vehicleFormKey += 1;
                    
                } else {
                    console.error("Ошибка загрузки данных пользователя");
                }
            } catch (error) {
                console.error("Ошибка:", error);
            }
        },

        formatPhoneNumberImmediately(phone) {
            if (!phone) return;
            
            this.rawPhoneNumber = phone.replace(/\D/g, '');
            
            let formattedNumber = this.rawPhoneNumber;
            
            if (formattedNumber.length === 11 && formattedNumber.startsWith('8')) {
                formattedNumber = '7' + formattedNumber.substring(1);
            }
            
            if (formattedNumber.length === 10) {
                formattedNumber = '7' + formattedNumber;
            }
            
            if (formattedNumber.length === 11 && formattedNumber.startsWith('7')) {
                formattedNumber = formattedNumber.replace(
                    /(\d{1})(\d{3})(\d{3})(\d{2})(\d{2})/,
                    '+$1 ($2) $3 $4-$5'
                );
            }
            
            this.phoneNumber = formattedNumber;
        },

        formatPhoneNumber() {
            if (!this.phoneNumber) return;
            
            this.rawPhoneNumber = this.phoneNumber.replace(/\D/g, '');
            
            let formattedNumber = this.rawPhoneNumber;
            
            if (formattedNumber.length === 11 && formattedNumber.startsWith('8')) {
                formattedNumber = '7' + formattedNumber.substring(1);
            }
            
            if (formattedNumber.length === 10) {
                formattedNumber = '7' + formattedNumber;
            }
            
            if (formattedNumber.length === 11 && formattedNumber.startsWith('7')) {
                formattedNumber = formattedNumber.replace(
                    /(\d{1})(\d{3})(\d{3})(\d{2})(\d{2})/,
                    '+$1 ($2) $3 $4-$5'
                );
            }
            
            this.phoneNumber = formattedNumber;
            this.validateField('phone');
        },
        
        clearPhoneFormat() {
            if (this.rawPhoneNumber) {
                this.phoneNumber = this.rawPhoneNumber;
            }
        },
        
        handleOneDayChange() {
            if (this.isOneDay) {
                this.startDate = '';
                this.endDate = '';
            } else {
                this.singleDate = '';
            }
        },
        
        deleteVehicle(vehicleId) {
            const index = this.vehicles.findIndex(vehicle => vehicle.id === vehicleId);
            if (index !== -1) {
                this.vehicles.splice(index, 1);
            }
        },

        editVehicle(vehicle) {
            this.$refs.vehicleForm.editVehicle(vehicle);
        },

        handleEditCancelled() {
            // Обработка отмены редактирования
            this.vehicleFormKey += 1;
        },
        
        validateField(field) {
            let phoneRegex;
            let timeRegex;

            switch (field) {
                case 'organization':
                    this.errors.organization = this.organization ? '' : 'Обязательное поле';
                    break;
                case 'company':
                    this.errors.company = this.company ? '' : 'Обязательное поле';
                    break;
                case 'responsiblePerson':
                    this.errors.responsiblePerson = this.responsiblePerson ? '' : 'Обязательное поле';
                    break;
                case 'phone':
                    phoneRegex = /^(\+7|8)?[\s-]?\(?[489][0-9]{2}\)?[\s-]?[0-9]{3}[\s-]?[0-9]{2}[\s-]?[0-9]{2}$/;
                    this.errors.phone = this.phoneNumber ? (phoneRegex.test(this.rawPhoneNumber) ? '' : 'Введите корректный номер') : 'Обязательное поле';
                    break;
                case 'startDate':
                    this.errors.startDate = this.isOneDay ? '' : (this.startDate ? '' : 'Укажите дату начала');
                    break;
                case 'endDate':
                    this.errors.endDate = this.isOneDay ? '' : (this.endDate ? '' : 'Укажите дату окончания');
                    break;
                case 'singleDate':
                    this.errors.singleDate = !this.isOneDay ? '' : (this.singleDate ? '' : 'Укажите дату');
                    break;
                case 'startTime':
                    timeRegex = /^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$/;
                    this.errors.startTime = this.startTime && timeRegex.test(this.startTime) ? '' : 'Введите время в формате ЧЧ:ММ';
                    break;
                case 'endTime':
                    timeRegex = /^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$/;
                    this.errors.endTime = this.endTime && timeRegex.test(this.endTime) ? '' : 'Введите время в формате ЧЧ:ММ';
                    break;
            }
        },

        validateDateRange() {
            if (!this.isOneDay && this.startDate && this.endDate) {
                const start = new Date(this.startDate.split('.').reverse().join('-'));
                const end = new Date(this.endDate.split('.').reverse().join('-'));
                if (start > end) {
                    this.errors.endDate = 'Дата окончания не может быть раньше даты начала';
                } else {
                    this.errors.endDate = '';
                }
            }
        },

        validateTimeRange() {
            if (this.startTime && this.endTime) {
                if (this.startTime >= this.endTime) {
                    this.errors.endTime = 'Время окончания должно быть позже времени начала';
                } else {
                    this.errors.endTime = '';
                }
            }
        },
        
        // Datepicker methods
        formatDate(date) {
            const day = date.getDate().toString().padStart(2, '0');
            const month = (date.getMonth() + 1).toString().padStart(2, '0');
            const year = date.getFullYear();
            return `${day}.${month}.${year}`;
        },
        
        parseDate(dateStr) {
            const [day, month, year] = dateStr.split('.');
            return new Date(`${year}-${month}-${day}`);
        },
        
        isSelectedDate(date) {
            if (this.isOneDay) {
                return date === this.singleDate;
            } else {
                return date === this.startDate || date === this.endDate;
            }
        },
        
        isToday(date) {
            const today = this.formatDate(new Date());
            return date === today;
        },
        
        openDatepicker(type) {
            this.showStartDatepicker = false;
            this.showEndDatepicker = false;
            this.showSingleDatepicker = false;
            
            if (type === 'start') {
                this.showStartDatepicker = true;
            } else if (type === 'end') {
                this.showEndDatepicker = true;
            } else if (type === 'single') {
                this.showSingleDatepicker = true;
            }
        },
        
        selectStartDate(date) {
            this.startDate = date;
            this.showStartDatepicker = false;
            this.validateDateRange();
        },
        
        selectEndDate(date) {
            this.endDate = date;
            this.showEndDatepicker = false;
            this.validateDateRange();
        },
        
        selectSingleDate(date) {
            this.singleDate = date;
            this.showSingleDatepicker = false;
            this.validateField('singleDate');
        },
        
        prevMonth() {
            this.currentDate = new Date(this.currentDate.getFullYear(), this.currentDate.getMonth() - 1, 1);
        },
        
        nextMonth() {
            this.currentDate = new Date(this.currentDate.getFullYear(), this.currentDate.getMonth() + 1, 1);
        },

        handleVehicleAdded(newVehicle) {
            const vehicleWithId = {
                ...newVehicle,
                id: this.vehicleIdCounter++
            };
            this.vehicles.push(vehicleWithId);
        },

        handleVehiclesAdded(vehicles) {
            vehicles.forEach(vehicle => {
                const vehicleWithId = {
                    ...vehicle,
                    id: this.vehicleIdCounter++
                };
                this.vehicles.push(vehicleWithId);
            });
        },

        handleVehicleUpdated(updatedVehicle) {
            const index = this.vehicles.findIndex(v => v.id === updatedVehicle.id);
            if (index !== -1) {
                this.vehicles.splice(index, 1, updatedVehicle);
            }
        },
        
        // Sorting methods
        sortBy(field) {
            if (this.sortField === field) {
                this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
            } else {
                this.sortField = field;
                this.sortDirection = 'asc';
            }
        },

        // Submit application
        async submitApplication() {
            // Проверяем, не идет ли уже процесс привязки
            if (this.bindingInProgress) {
                return;
            }

            // Валидируем все поля перед отправкой
            this.validateAllFields();
            
            if (!this.canSubmit) {
                alert('Заполните все обязательные поля и добавьте хотя бы одно транспортное средство');
                return;
            }

            // Валидация дат
            if (!this.isOneDay) {
                const start = new Date(this.startDate.split('.').reverse().join('-'));
                const end = new Date(this.endDate.split('.').reverse().join('-'));
                if (start > end) {
                    alert('Дата окончания не может быть раньше даты начала');
                    return;
                }
            }

            // Валидация времени
            if (this.startTime >= this.endTime) {
                alert('Время окончания должно быть позже времени начала');
                return;
            }

            // Определяем новые машины для добавления в unique_cars
            // Исключаем машины "по факту" и существующие машины
            const newCars = this.vehicles.filter(vehicle => 
                !vehicle.isExisting && 
                vehicle.plateNumber !== 'По факту' && 
                vehicle.mark !== 'По факту'
            );
            
            // Сначала создаем уникальные машины если есть новые
            if (newCars.length > 0) {
                await this.createUniqueCars(newCars);
            } else {
                // Если нет новых машин для привязки, сразу отправляем заявку
                await this.sendApplication();
            }
        },

        validateAllFields() {
            this.validateField('organization');
            this.validateField('company');
            this.validateField('responsiblePerson');
            this.validateField('phone');
            this.validateField('startDate');
            this.validateField('endDate');
            this.validateField('singleDate');
            this.validateField('startTime');
            this.validateField('endTime');
            this.validateDateRange();
            this.validateTimeRange();
        },

        async createUniqueCars(newCars) {
            try {
                const token = localStorage.getItem("token");
                
                // Создаем массив промисов для создания машин
                const createPromises = newCars.map(car => {
                    const carData = {
                        number: car.plateNumber,
                        mark: car.mark,
                        format_id: car.formatId,
                        user_id: null, // Будет установлен на сервере автоматически
                        organization_id: null,
                        company_id: null
                    };

                    return fetch("http://localhost:8080/unique-cars", {
                        method: "POST",
                        headers: {
                            "Authorization": `Bearer ${token}`,
                            "Content-Type": "application/json"
                        },
                        body: JSON.stringify(carData)
                    });
                });

                // Ждем завершения всех запросов
                const responses = await Promise.all(createPromises);
                
                // Проверяем результаты
                const results = await Promise.all(responses.map(async (response, index) => {
                    if (response.ok) {
                        const createdCar = await response.json();
                        return { 
                            success: true, 
                            car: newCars[index], 
                            createdCar,
                            carId: createdCar.id // Сохраняем ID
                        };
                    } else {
                        const error = await response.json();
                        return { success: false, car: newCars[index], error };
                    }
                }));

                // Разделяем успешные и неуспешные создания
                const successfulCreations = results.filter(result => result.success);
                const failedCreations = results.filter(result => !result.success);

                // Показываем ошибки если есть неуспешные создания
                if (failedCreations.length > 0) {
                    console.error('Ошибки при создании машин:', failedCreations);
                    // Продолжаем с успешными созданиями
                }

                // Сохраняем успешно созданные машины с их ID и флагом привязки
                this.newCarsToBind = successfulCreations.map(result => ({
                    plateNumber: result.car.plateNumber,
                    mark: result.car.mark,
                    formatId: result.car.formatId,
                    carId: result.carId, // Сохраняем ID для обновления
                    bindToEntity: true // По умолчанию все машины будут привязаны
                }));

                // Показываем модальное окно привязки если есть успешно созданные машины
                if (this.newCarsToBind.length > 0) {
                    this.bindingInProgress = true;
                    this.showBindingModal = true;
                } else {
                    // Если нет машин для привязки, отправляем заявку
                    await this.sendApplication();
                }

            } catch (error) {
                console.error('Ошибка при создании уникальных машин:', error);
                // Продолжаем отправку заявки даже при ошибке
                await this.sendApplication();
            }
        },

        async sendApplication() {
            // Подготовка данных для отправки
            const applicationData = {
                message: this.message || null,
                application: {
                    organization: this.organization,
                    responsible_person: this.responsiblePerson,
                    contact_phone: this.phoneNumber.replace(/\D/g, ''), // Оставляем только цифры
                    entry_date_from: this.formatDateForAPI(this.isOneDay ? this.singleDate : this.startDate),
                    entry_date_to: this.formatDateForAPI(this.isOneDay ? this.singleDate : this.endDate),
                    entry_time_from: this.startTime + ":00", // Добавляем секунды
                    entry_time_to: this.endTime + ":00"      // Добавляем секунды
                },
                cars: this.vehicles.map(vehicle => ({
                    car: {
                        car_number: vehicle.plateNumber,
                        car_brand: vehicle.mark
                    },
                    unload_places: vehicle.unloadPlaces ? vehicle.unloadPlaces.map((placeId, index) => ({
                        unload_place_id: placeId,
                        order_index: index + 1,
                        planned_time: null,
                        notes: null
                    })) : []
                }))
            };

            console.log('Отправляемые данные:', JSON.stringify(applicationData, null, 2));

            try {
                const token = localStorage.getItem("token");
                if (!token) {
                    alert('Токен не найден. Пожалуйста, войдите заново.');
                    return;
                }

                const response = await fetch("http://localhost:8080/submit-v2", {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        "Authorization": `Bearer ${token}`
                    },
                    body: JSON.stringify(applicationData)
                });

                // Проверяем Content-Type перед парсингом
                const contentType = response.headers.get('content-type');
                
                if (!contentType || !contentType.includes('application/json')) {
                    const text = await response.text();
                    console.error('Сервер вернул не JSON:', text);
                    throw new Error(`Сервер вернул ошибку: ${response.status} ${response.statusText}`);
                }

                const result = await response.json();

                if (response.ok) {
                    alert('Заявка успешно отправлена!');
                    this.resetForm();
                } else {
                    alert(`Ошибка: ${result.message || 'Неизвестная ошибка'}`);
                }
            } catch (error) {
                console.error('Ошибка отправки заявки:', error);
                alert(`Произошла ошибка при отправке заявки: ${error.message}`);
            }
        },

        // Переключение привязки для конкретной машины
        toggleCarBinding(car) {
            car.bindToEntity = !car.bindToEntity;
        },

        // Метод для привязки машин
        async confirmBinding() {
            try {
                const token = localStorage.getItem("token");
                
                // Обновляем привязку для каждой машины по ID
                const updatePromises = this.newCarsToBind.map(car => {
                    const updateData = {
                        number: car.plateNumber,
                        mark: car.mark,
                        format_id: car.formatId,
                        organization_id: car.bindToEntity && this.bindToOrganization ? this.organizationId : null,
                        company_id: car.bindToEntity && this.bindToCompany ? this.companyId : null,
                        user_id: null // Привязываем к пользователю (будет установлен на сервере автоматически)
                    };

                    // Используем обычный эндпоинт обновления по ID
                    return fetch(`http://localhost:8080/unique-cars/${car.carId}`, {
                        method: "PUT",
                        headers: {
                            "Authorization": `Bearer ${token}`,
                            "Content-Type": "application/json"
                        },
                        body: JSON.stringify(updateData)
                    });
                });

                const results = await Promise.allSettled(updatePromises);
                
                // Проверяем результаты
                const successfulUpdates = results.filter(result => result.status === 'fulfilled');
                const failedUpdates = results.filter(result => result.status === 'rejected');
                
                if (failedUpdates.length > 0) {
                    console.error('Ошибки при обновлении машин:', failedUpdates);
                }

                console.log(`Успешно обновлено ${successfulUpdates.length} из ${results.length} машин`);

                this.closeBindingModal();
                await this.sendApplication();

            } catch (error) {
                console.error('Ошибка при привязке машин:', error);
                this.closeBindingModal();
                await this.sendApplication();
            }
        },

        skipBinding() {
            this.closeBindingModal();
            this.sendApplication();
        },

        closeBindingModal() {
            this.showBindingModal = false;
            this.newCarsToBind = [];
            this.bindToOrganization = false;
            this.bindToCompany = false;
            this.bindingInProgress = false;
        },

        // Добавьте этот вспомогательный метод для форматирования даты
        formatDateForAPI(dateStr) {
            if (!dateStr) return null;
            const [day, month, year] = dateStr.split('.');
            return `${year}-${month}-${day}`; // Формат YYYY-MM-DD
        },

        resetForm() {
            // Сброс формы после успешной отправки
            this.message = '';
            this.organization = '';
            this.company = '';
            this.responsiblePerson = '';
            this.phoneNumber = '';
            this.isOneDay = false;
            this.startDate = '';
            this.endDate = '';
            this.singleDate = '';
            this.startTime = '';
            this.endTime = '';
            this.consentGiven = false;
            this.vehicles = [];
            this.applicationNumber++;

            // Сбрасываем ID
            this.organizationId = null;
            this.companyId = null;
            
            // Сбрасываем ошибки
            this.errors = {
                organization: '',
                company: '',
                responsiblePerson: '',
                phone: '',
                startDate: '',
                endDate: '',
                singleDate: '',
                startTime: '',
                endTime: '',
                unloadingPlaces: ''
            };
            
            // Увеличиваем ключ для принудительного пересоздания VehicleForm
            this.vehicleFormKey += 1;
            
            // Перезагрузка данных пользователя
            this.loadUserData();
        }
    },
    mounted() {
        this.loadUserData();

        document.addEventListener('click', (e) => {
            if (!e.target.closest('.datepicker-wrapper')) {
                this.showStartDatepicker = false;
                this.showEndDatepicker = false;
                this.showSingleDatepicker = false;
            }
        });
    }
}
</script>

<style scoped>
    .create {
        padding: 20px;
    }

    .create__title {
        display: flex;
        gap: 10px;
        padding-bottom: 15px;
    }

    .create__container {
        display: flex;
        gap: 15px;
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

    .tables__instruction:hover {
        background-color: #f2f2f2;
    }

    .create__form {
        width: 100%;
        height: fit-content;
        background-color: #FFF;
        border: 1px solid #e6e6e6;
        border-radius: 30px;
        box-shadow: 0 3px 10px rgba(0,0,0,0.05);
    }

    .form__header {
        width: 100%;
        height: 80px;
        border-bottom: 1px solid #e6e6e6;
        padding: 15px;
    }

    .header__content {
        display: flex;
        gap: 20px;
        height: 100%;
    }

    .form__textarea {
        width: 55%;
        border: 1px solid #e6e6e6;
        outline: none;
        border-radius: 15px;
        height: 50px;
        padding: 10px;
        resize: none;
    }

    .header__right {
        display: flex;
        flex-direction: column;
        gap: 10px;
        flex: 1;
    }

    .consent-section {
        display: flex;
        align-items: center;
        gap: 20px;
        height: 100%;
    }

    .consent-checkbox {
        display: flex;
        gap: 10px;
        max-width: 350px;
    }

    .consent-checkbox input[type="checkbox"] {
        width: 14px;
        height: 14px;
        cursor: pointer;
        flex-shrink: 0;
    }

    .consent-checkbox label {
        font-size: 12px;
        color: #333;
        cursor: pointer;
        line-height: 1.2;
    }

    .send-all-btn {
        background: #4F5BDF;
        color: white;
        border: none;
        border-radius: 15px;
        padding: 8px 15px;
        font-size: 12px;
        cursor: pointer;
        transition: background-color 0.2s;
        width: fit-content;
        flex-shrink: 0;
        height: fit-content;
    }

    .send-all-btn:hover:not(:disabled) {
        background: #3a45c0;
    }

    .send-all-btn:disabled {
        background: #a2a2a2;
        cursor: not-allowed;
        opacity: 0.6;
    }

    h4 {
        font-size: 18px;
        padding-bottom: 15px;
    }

    .user__input {
        width: 200px;
        display: flex;
        flex-direction: column;
        gap: 5px;
        position: relative;
    }

    .contacts {
        display: flex;
        justify-content: space-between;
    }

    .contact-input {
        border: none;
        outline: none;
        background: transparent;
        width: 60%;
    }

    .phone {
        width: 35%;
        text-align: end;
    }

    .input__label {
        font-size: 13px;
        color: #a2a2a2;
    }

    .required {
        color: #ff4444;
    }

    .input {
        width: 100%;
        height: 40px;
        border: 1px solid #e6e6e6;
        outline: none;
        background: #FFF;
        border-radius: 10px;
        padding: 5px 10px;
    }

    .input--error {
        border-color: #ff4444;
    }

    .error-message {
        font-size: 11px;
        color: #ff4444;
        position: absolute;
        bottom: -15px;
        left: 0;
    }

    .form__info {
        padding: 15px;
        height: 210px;
        display: flex;
        flex-direction: column;
        flex-wrap: wrap;
        border-bottom: 1px solid #e6e6e6;
    }

    .info__user {
        display: flex;
        flex-wrap: wrap;
        gap: 30px;
        row-gap: 15px;
        width: 430px;
    }

    .responsible {
        width: 430px !important;
    }

    .date-container {
        min-height: 40px;
        display: flex;
        flex-direction: column;
        gap: 5px;
        width: 250px;
    }

    .date {
        display: flex;
        align-items: center;
        gap: 5px;
        width: 100%;
    }

    .input__date {
        width: 105px;
        height: 40px;
        border: 1px solid #e6e6e6;
        outline: none;
        background: #FFF;
        border-radius: 10px;
        padding: 5px 10px;
    }

    .date__text {
        color: #4F5BDF;
        font-weight: 600;
        white-space: nowrap;
    }

    .date__input {
        display: flex;
        flex-direction: column;
        gap: 5px;
        position: relative;
    }

    .form__date {
        display: flex;
        gap: 30px;
    }

    .one-day {
        display: flex;
        gap: 5px;
        align-items: center;
        margin-top: 5px;
    }

    .one-day p {
        font-size: 12px;
    }

    .one-day__checkbox {
        width: 13px;
        height: 13px;
        cursor: pointer;
    }

    /* Datepicker styles */
    .datepicker-wrapper {
        position: relative;
        display: inline-block;
    }

    .datepicker {
        position: absolute;
        top: calc(100% + 10px);
        left: 0;
        background: white;
        border: 1px solid #e6e6e6;
        border-radius: 10px;
        padding: 10px;
        z-index: 1000;
        box-shadow: 0 3px 10px rgba(0,0,0,0.1);
        min-width: 250px;
    }

    .datepicker__header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 10px;
    }

    .datepicker__nav {
        background: none;
        border: none;
        cursor: pointer;
        padding: 5px;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .datepicker__arrow {
        width: 10px;
        height: 10px;
    }

    .datepicker__arrow--left {
        transform: rotate(180deg);
    }

    .datepicker__month {
        font-weight: 600;
        font-size: 14px;
    }

    .datepicker__weekdays {
        display: grid;
        grid-template-columns: repeat(7, 1fr);
        gap: 5px;
        margin-bottom: 5px;
    }

    .datepicker__weekday {
        text-align: center;
        font-size: 12px;
        color: #a2a2a2;
        font-weight: 500;
    }

    .datepicker__days {
        display: grid;
        grid-template-columns: repeat(7, 1fr);
        gap: 5px;
    }

    .datepicker__day {
        text-align: center;
        padding: 5px;
        font-size: 12px;
        cursor: pointer;
        border-radius: 5px;
        transition: background-color 0.2s;
    }

    .datepicker__day:hover {
        background-color: #f0f0f0;
    }

    .datepicker__day--selected {
        background-color: #4F5BDF;
        color: white;
    }

    .datepicker__day--other-month {
        color: #ccc;
    }

    .datepicker__day--today {
        font-weight: bold;
        border: 1px solid #4F5BDF;
    }

    .single-date {
        display: flex;
        align-items: center;
        width: 100%;
    }

    .time-message {
        font-size: 10px;
        width: 250px;
    }

    .form__data {
        display: flex;
    }

    .data__list {
        padding: 15px;
        flex: 1;
    }

    .data__completion {
        padding: 15px;
        width: 450px;
        border-right: 1px solid #e6e6e6;
    }

    /* Vehicles table styles */
    .vehicles-table {
        width: 100%;
        border: 1px solid #e6e6e6;
        border-radius: 15px;
        overflow: hidden;
    }

    .table-header {
        display: flex;
        background: #f8f8f8;
        border-bottom: 1px solid #e6e6e6;
        padding: 12px 15px;
        font-weight: 500;
        color: #a2a2a2;
        font-size: 14px;
    }

    .header-col {
        display: flex;
        align-items: center;
        gap: 5px;
        cursor: pointer;
        transition: color 0.2s;
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
        transition: all 0.2s;
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

    .table-body {
        max-height: 200px;
        overflow-y: auto;
    }

    .table-row {
        display: flex;
        padding: 10px 15px;
        border-bottom: 1px solid #f0f0f0;
        align-items: center;
        font-size: 15px;
    }

    .table-row:last-child {
        border-bottom: none;
    }

    .table-row:hover {
        background: #fafafa;
    }

    .header-col, .table-col {
        padding: 0 5px;
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
        gap: 5px;
    }

    .edit-btn, .delete-btn {
        background: none;
        border: none;
        cursor: pointer;
        padding: 5px;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .edit-btn:hover, .delete-btn:hover {
        background: #f0f0f0;
        border-radius: 5px;
    }

    .edit-icon, .delete-icon {
        width: 16px;
        height: 16px;
        opacity: 0.7;
    }

    .edit-btn:hover .edit-icon,
    .delete-btn:hover .delete-icon {
        opacity: 1;
    }

    .no-vehicles {
        text-align: center;
        padding: 20px;
        color: #a2a2a2;
        font-size: 14px;
    }

       /* Модальное окно привязки - супер минималистичный дизайн */
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
        align-items: flex-start;
        padding: 15px;
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
        max-height: 60vh;
        overflow-y: auto;
    }

    .binding-info {
        margin-bottom: 20px;
    }

    .binding-description {
        font-size: 14px;
        line-height: 1.5;
        color: #666;
        margin-bottom: 20px;
        text-align: left;
    }

    .section-title {
        font-size: 14px;
        font-weight: 600;
        color: #333;
        margin-bottom: 10px;
    }

    .cars-list-section {
        margin-bottom: 25px;
    }

    .cars-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .car-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 12px 15px;
        border: 1px solid #e6e6e6;
        border-radius: 10px;
        transition: all 0.2s;
        cursor: pointer;
    }

    .car-item:hover {
        border-color: #4F5BDF;
    }

    .car-item--shared {
        background: #f8f9ff;
        border-color: #4F5BDF;
    }

    .car-selector {
        display: flex;
        align-items: center;
        gap: 12px;
    }

    .selector-checkbox {
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .checkbox {
        width: 18px;
        height: 18px;
        border: 2px solid #e6e6e6;
        border-radius: 4px;
        transition: all 0.2s;
        position: relative;
    }

    .checkbox--checked {
        background: #4F5BDF;
        border-color: #4F5BDF;
    }

    .checkbox--checked::after {
        content: "✓";
        position: absolute;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        color: white;
        font-size: 12px;
        font-weight: bold;
    }

    .car-info {
        display: flex;
        align-items: center;
        gap: 15px;
    }

    .car-number {
        font-weight: 600;
        color: #333;
        font-size: 14px;
    }

    .car-mark {
        color: #666;
        font-size: 13px;
    }

    .car-binding-status {
        font-size: 12px;
        font-weight: 500;
    }

    .status-shared {
        color: #4F5BDF;
        display: flex;
        align-items: center;
        gap: 5px;
    }

    .status-private {
        color: #666;
        display: flex;
        align-items: center;
        gap: 5px;
    }

    .status-icon {
        font-size: 14px;
    }

    .binding-options-section {
        margin-bottom: 20px;
        padding-top: 20px;
        border-top: 1px solid #e6e6e6;
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
        font-size: 14px;
        padding: 5px 0;
    }

    .binding-option input[type="checkbox"] {
        width: 14px;
        height: 14px;
        cursor: pointer;
    }

    .option-text {
        color: #333;
    }

    .warning-section {
     
    }

    .warning-text {
        font-size: 11px;
        line-height: 1.5;
        color: #666;
        margin: 0;
        text-align: left;
    }

    .modal-actions {
        display: flex;
        justify-content: flex-end;
        gap: 12px;
        padding-top: 20px;
        border-top: 1px solid #e6e6e6;
    }

    .cancel-btn {
        background: white;
        color: #666;
        border: 1px solid #e6e6e6;
        border-radius: 12px;
        padding: 10px 20px;
        font-size: 14px;
        cursor: pointer;
        transition: all 0.2s;
    }

    .cancel-btn:hover {
        background: #f5f5f5;
        border-color: #ccc;
    }

    .confirm-btn {
        background: #4F5BDF;
        color: white;
        border: none;
        border-radius: 12px;
        padding: 10px 20px;
        font-size: 14px;
        cursor: pointer;
        transition: background-color 0.2s;
    }

    .confirm-btn:hover {
        background: #3a45c0;
    }

    .blue {
        color: #4F5BDF;
    }

    

    @media (max-width: 768px) {
        .modal-content {
            width: 95vw;
            margin: 10px;
        }
        
        .modal-actions {
            flex-direction: column;
        }
        
        .cancel-btn,
        .confirm-btn {
            width: 100%;
        }
        
        .car-item {
            flex-direction: column;
            align-items: flex-start;
            gap: 8px;
        }
        
        .car-selector {
            width: 100%;
            justify-content: space-between;
        }
    }
</style>