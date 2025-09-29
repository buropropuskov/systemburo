<template>
    <div class="create">
        <div class="create__title">
            <h3>Оформление и подача заявки</h3>
            <button class="tables__instruction">
                <img src="@/assets/icons/instruction.png" class="tables__icon" />
                <p class="instruction__text">Инструкция</p>
            </button>
        </div>
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
                <div class="data__completion">
                    <div class="completion__header">
                        <h3>Добавление Т/С</h3>
                    </div>
                    <div class="completion__format">
                        <div class="format__header">
                            <label class="format__label">Формат номеров</label>
                            <button class="add-button" @click="addVehicle" :disabled="!canAddVehicle">
                                Добавить
                            </button>
                        </div>
                        <div class="format__dropdown">
                            <button class="dropdown__button" @click="toggleDropdown">
                                <div class="button__content">
                                    <div class="content__country">
                                        <img :src="selectedFormatFlag" class="button__flag" v-if="selectedFormat !== 'tractor'" />
                                        <span class="button__text">{{ selectedFormatText }}</span>
                                    </div>
                                    <img src="@/assets/icons/arrow.png" class="button__arrow" :class="{ 'button__arrow--open': isDropdownOpen }" />
                                </div>
                            </button>
                            <div v-if="isDropdownOpen" class="dropdown__menu">
                                <div class="dropdown__item" @click="selectFormat('russian')">
                                    <img src="@/assets/icons/flag-russian.png" class="item__flag" />
                                    <span class="item__text">Россия</span>
                                </div>
                                <div class="dropdown__item" @click="selectFormat('azerbaijan')">
                                    <img src="@/assets/icons/flag-azer.png" class="item__flag" />
                                    <span class="item__text">Азербайджан</span>
                                </div>
                                <div class="dropdown__item" @click="selectFormat('tractor')">
                                    <span class="item__text">Трактор</span>
                                </div>
                            </div>
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
                            
                            <!-- Русский формат -->
                            <div class="number__field" v-else-if="selectedFormat === 'russian'">
                                <input 
                                    class="number__input" 
                                    placeholder="А"
                                    v-model="numberParts[0]"
                                    @input="validateRussianPart(0, $event)"
                                    maxlength="1"
                                />
                                <input 
                                    class="number__input" 
                                    placeholder="777"
                                    v-model="numberParts[1]"
                                    @input="validateRussianPart(1, $event)"
                                    @blur="formatRussianDigits(1)"
                                    maxlength="3"
                                />
                                <input 
                                    class="number__input" 
                                    placeholder="АА"
                                    v-model="numberParts[2]"
                                    @input="validateRussianPart(2, $event)"
                                    maxlength="2"
                                />
                                <input 
                                    class="number__input" 
                                    placeholder="777"
                                    v-model="numberParts[3]"
                                    @input="validateRussianPart(3, $event)"
                                    @blur="formatRussianDigits(3)"
                                    maxlength="3"
                                />
                            </div>
                            
                            <!-- Азербайджанский формат -->
                            <div class="number__field" v-else-if="selectedFormat === 'azerbaijan'">
                                <input 
                                    class="number__input" 
                                    placeholder="00"
                                    v-model="numberParts[0]"
                                    @input="validateAzerbaijanPart(0, $event)"
                                    @blur="formatAzerbaijanDigits(0)"
                                    maxlength="2"
                                />
                                <input 
                                    class="number__input" 
                                    placeholder="AA"
                                    v-model="numberParts[1]"
                                    @input="validateAzerbaijanPart(1, $event)"
                                    maxlength="2"
                                />
                                <input 
                                    class="number__input" 
                                    placeholder="000"
                                    v-model="numberParts[2]"
                                    @input="validateAzerbaijanPart(2, $event)"
                                    @blur="formatAzerbaijanDigits(2)"
                                    maxlength="3"
                                />
                            </div>
                            
                            <!-- Трактор формат -->
                            <div class="number__field" v-else-if="selectedFormat === 'tractor'">
                                <input 
                                    class="number__input" 
                                    placeholder="0000"
                                    v-model="numberParts[0]"
                                    @input="validateTractorPart(0, $event)"
                                    @blur="formatTractorDigits(0)"
                                    maxlength="4"
                                />
                                <input 
                                    class="number__input" 
                                    placeholder="АА"
                                    v-model="numberParts[1]"
                                    @input="validateTractorPart(1, $event)"
                                    maxlength="2"
                                />
                                <input 
                                    class="number__input" 
                                    placeholder="00"
                                    v-model="numberParts[2]"
                                    @input="validateTractorPart(2, $event)"
                                    @blur="formatTractorDigits(2)"
                                    maxlength="2"
                                />
                            </div>
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
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Места разгрузки -->
                    <div class="completion__unloading">
                        <label class="input__label">Места разгрузки (выбор) <span class="required">*</span></label>
                        <div class="unloading__grid">
                            <div 
                                v-for="place in unloadingPlaces" 
                                :key="place.id"
                                class="unloading__item"
                                :class="{ 'unloading__item--active': selectedUnloadingPlaces.includes(place.id) }"
                                @click="toggleUnloadingPlace(place.id)"
                            >
                                {{ place.name }}
                            </div>
                        </div>
                        <div v-if="errors.unloadingPlaces" class="error-message">{{ errors.unloadingPlaces }}</div>
                    </div>
                </div>
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
</template>

<script>
export default {
    name: 'CreateApplication',
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
            
            // Number parts
            numberParts: ['', '', '', ''],
            isNumberByFact: false,
            
            // Mark data
            isMarkByFact: false,
            selectedMark: '',
            isMarkDropdownOpen: false,
            markSearch: '',
            
            // Available marks
            marks: [
                'ВАЗ', 'Мерседес', 'БМВ', 'Газель', 'ГАЗ', 'Вольво', 'Тойота', 'Митсубиси',
                'Ауди', 'Фольксваген', 'Шевроле', 'Хендай', 'Киа', 'Ниссан', 'Рено', 'Пежо',
                'Ситроен', 'Форд', 'Опель', 'Шкода', 'Лада', 'УАЗ'
            ],
            filteredMarks: [],
            
            // Unloading places
            unloadingPlaces: [
                { id: 1, name: 'Дебаркадер №1' },
                { id: 2, name: 'Дебаркадер №2' },
                { id: 3, name: 'Дебаркадер №3' },
                { id: 4, name: 'Дебаркадер №4' },
                { id: 5, name: 'Дебаркадер №5' },
                { id: 6, name: 'Территория' },
                { id: 7, name: 'Ворота Сочи' },
                { id: 8, name: 'Ворота Маугли' },
                { id: 9, name: 'Ворота Черепашки' },
                { id: 10, name: 'ПОСТ № 21' },
                { id: 11, name: 'ПОСТ № 27' },
                { id: 12, name: 'Офис' },
                { id: 13, name: 'Южный ЛП' },
                { id: 14, name: 'Северный ЛП' },
                { id: 15, name: 'Пост ЮГ' }
            ],
            selectedUnloadingPlaces: [],
            
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
            
            // Dropdown
            isDropdownOpen: false,
            selectedFormat: 'russian',
            
            allowedRussianLetters: ['А', 'В', 'Е', 'К', 'М', 'Н', 'О', 'Р', 'С', 'Т', 'У', 'Х'],
            allowedAzerbaijanLetters: ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z']
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
        selectedFormatText() {
            const formats = {
                'russian': 'Россия',
                'azerbaijan': 'Азербайджан',
                'tractor': 'Трактор'
            };
            return formats[this.selectedFormat];
        },
        selectedFormatFlag() {
            const flags = {
                'russian': require('@/assets/icons/flag-russian.png'),
                'azerbaijan': require('@/assets/icons/flag-azer.png'),
                'tractor': ''
            };
            return flags[this.selectedFormat];
        },
        canAddVehicle() {
            if (this.isNumberByFact && this.isMarkByFact && this.selectedUnloadingPlaces.length > 0) {
                return true;
            }

            if (!this.isNumberByFact) {
                switch (this.selectedFormat) {
                    case 'russian':
                        if (!this.numberParts[0] || !this.numberParts[1] || !this.numberParts[2] || !this.numberParts[3]) {
                            return false;
                        }
                        if (this.numberParts[0].length !== 1 || 
                            this.numberParts[1].length !== 3 || 
                            this.numberParts[2].length !== 2 || 
                            (this.numberParts[3].length < 2 || this.numberParts[3].length > 3)) {
                            return false;
                        }
                        break;
                    case 'azerbaijan':
                        if (!this.numberParts[0] || !this.numberParts[1] || !this.numberParts[2]) {
                            return false;
                        }
                        if (this.numberParts[0].length !== 2 || 
                            this.numberParts[1].length !== 2 || 
                            this.numberParts[2].length !== 3) {
                            return false;
                        }
                        break;
                    case 'tractor':
                        if (!this.numberParts[0] || !this.numberParts[1] || !this.numberParts[2]) {
                            return false;
                        }
                        if (this.numberParts[0].length !== 4 || 
                            this.numberParts[1].length !== 2 || 
                            this.numberParts[2].length !== 2) {
                            return false;
                        }
                        break;
                }
            }
            
            if (!this.isMarkByFact && !this.selectedMark) {
                return false;
            }
            
            if (this.selectedUnloadingPlaces.length === 0) {
                return false;
            }
            
            return true;
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
        
        handleNumberByFactChange() {
            if (this.isNumberByFact) {
                this.numberParts = ['', '', '', ''];
            }
        },
        
        handleMarkByFactChange() {
            if (this.isMarkByFact) {
                this.selectedMark = '';
            }
        },
        
        toggleUnloadingPlace(placeId) {
            const index = this.selectedUnloadingPlaces.indexOf(placeId);
            if (index > -1) {
                this.selectedUnloadingPlaces.splice(index, 1);
            } else {
                this.selectedUnloadingPlaces.push(placeId);
            }
            this.validateUnloadingPlaces();
        },
        
        validateUnloadingPlaces() {
            this.errors.unloadingPlaces = this.selectedUnloadingPlaces.length === 0 ? 'Выберите хотя бы одно место разгрузки' : '';
        },
        
        formatUnloadingPlaces() {
            if (this.selectedUnloadingPlaces.length === 0) return '';
            
            const placeNames = this.selectedUnloadingPlaces.map(placeId => {
                const place = this.unloadingPlaces.find(p => p.id === placeId);
                return place ? place.name : '';
            }).filter(name => name);
            
            return placeNames.join(', ');
        },
        
        addVehicle() {
            if (!this.canAddVehicle) {
                alert('Заполните все обязательные поля правильно');
                return;
            }
            
            let plateNumber = '';
            if (this.isNumberByFact) {
                plateNumber = 'По факту';
            } else {
                switch (this.selectedFormat) {
                    case 'russian':
                        plateNumber = `${this.numberParts[0]} ${this.numberParts[1]} ${this.numberParts[2]} ${this.numberParts[3]}`;
                        break;
                    case 'azerbaijan':
                        plateNumber = `${this.numberParts[0]} ${this.numberParts[1]} ${this.numberParts[2]}`;
                        break;
                    case 'tractor':
                        plateNumber = `${this.numberParts[0]} ${this.numberParts[1]} ${this.numberParts[2]}`;
                        break;
                }
            }
            
            const mark = this.isMarkByFact ? 'По факту' : this.selectedMark;
            const unloadingPlace = this.formatUnloadingPlaces();

            const newVehicle = {
                id: this.vehicleIdCounter++,
                plateNumber: plateNumber,
                mark: mark,
                unloadingPlace: unloadingPlace,
                unloadPlaces: [...this.selectedUnloadingPlaces] // Сохраняем ID мест разгрузки
            };
            
            this.vehicles.push(newVehicle);
            
            // Очищаем только номер и марку, места разгрузки остаются выбранными
            this.clearVehicleFormPartial();
        },
        
        deleteVehicle(vehicleId) {
            const index = this.vehicles.findIndex(vehicle => vehicle.id === vehicleId);
            if (index !== -1) {
                this.vehicles.splice(index, 1);
            }
        },
        
        clearVehicleFormPartial() {
            // Очищаем только номер и марку, места разгрузки остаются выбранными
            this.numberParts = ['', '', '', ''];
            this.selectedMark = '';
            this.isNumberByFact = false;
            this.isMarkByFact = false;
        },
        
        clearVehicleForm() {
            this.numberParts = ['', '', '', ''];
            this.selectedMark = '';
            this.selectedUnloadingPlaces = [];
            this.isNumberByFact = false;
            this.isMarkByFact = false;
            this.errors.unloadingPlaces = '';
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
                }
            }
        },

        validateTimeRange() {
            if (this.startTime && this.endTime) {
                if (this.startTime >= this.endTime) {
                    this.errors.endTime = 'Время окончания должно быть позже времени начала';
                }
            }
        },
        
        // Russian number validation
        validateRussianPart(part, event) {
            let value = event.target.value.toUpperCase();
            
            if (part === 0 || part === 2) {
                value = value.replace(/[^АВЕКМНОРСТУХ]/g, '');
            } else {
                value = value.replace(/\D/g, '');
                
                if (part === 1 && value.length > 3) {
                    value = value.slice(0, 3);
                } else if (part === 3 && value.length > 3) {
                    value = value.slice(0, 3);
                }
            }
            
            this.numberParts[part] = value;
            event.target.value = value;
        },
        
        formatRussianDigits(part) {
            if (part === 1) {
                if (this.numberParts[1].length === 1) {
                    this.numberParts[1] = '00' + this.numberParts[1];
                } else if (this.numberParts[1].length === 2) {
                    this.numberParts[1] = '0' + this.numberParts[1];
                }
            } else if (part === 3) {
                if (this.numberParts[3].length === 1) {
                    this.numberParts[3] = '0' + this.numberParts[3];
                }
            }
        },
        
        // Azerbaijan number validation
        validateAzerbaijanPart(part, event) {
            let value = event.target.value.toUpperCase();
            
            if (part === 1) {
                value = value.replace(/[^A-Z]/g, '');
            } else {
                value = value.replace(/\D/g, '');
                
                if (part === 0 && value.length > 2) {
                    value = value.slice(0, 2);
                } else if (part === 2 && value.length > 3) {
                    value = value.slice(0, 3);
                }
            }
            
            this.numberParts[part] = value;
            event.target.value = value;
        },
        
        formatAzerbaijanDigits(part) {
            if (part === 0) {
                if (this.numberParts[0].length === 1) {
                    this.numberParts[0] = '0' + this.numberParts[0];
                }
            } else if (part === 2) {
                if (this.numberParts[2].length === 1) {
                    this.numberParts[2] = '00' + this.numberParts[2];
                } else if (this.numberParts[2].length === 2) {
                    this.numberParts[2] = '0' + this.numberParts[2];
                }
            }
        },
        
        // Tractor number validation
        validateTractorPart(part, event) {
            let value = event.target.value.toUpperCase();
            
            if (part === 1) {
                value = value.replace(/[^АВЕКМНОРСТУХ]/g, '');
            } else {
                value = value.replace(/\D/g, '');
                
                if (part === 0 && value.length > 4) {
                    value = value.slice(0, 4);
                } else if (part === 2 && value.length > 2) {
                    value = value.slice(0, 2);
                }
            }
            
            this.numberParts[part] = value;
            event.target.value = value;
        },
        
        formatTractorDigits(part) {
            if (part === 0) {
                if (this.numberParts[0].length === 1) {
                    this.numberParts[0] = '000' + this.numberParts[0];
                } else if (this.numberParts[0].length === 2) {
                    this.numberParts[0] = '00' + this.numberParts[0];
                } else if (this.numberParts[0].length === 3) {
                    this.numberParts[0] = '0' + this.numberParts[0];
                }
            } else if (part === 2) {
                if (this.numberParts[2].length === 1) {
                    this.numberParts[2] = '0' + this.numberParts[2];
                }
            }
        },
        
        // Mark dropdown methods
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
        
        // Dropdown methods
        toggleDropdown() {
            this.isDropdownOpen = !this.isDropdownOpen;
        },
        
        selectFormat(format) {
            this.selectedFormat = format;
            this.numberParts = ['', '', '', ''];
            this.isDropdownOpen = false;
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
            this.selectedUnloadingPlaces = []; // Сбрасываем места разгрузки
            this.applicationNumber++;
            
            // Перезагрузка данных пользователя
            this.loadUserData();
        }
    },
    mounted() {
        this.loadUserData();
        this.filteredMarks = this.marks;

        document.addEventListener('click', (e) => {
            if (!e.target.closest('.datepicker-wrapper')) {
                this.showStartDatepicker = false;
                this.showEndDatepicker = false;
                this.showSingleDatepicker = false;
            }
            
            if (!e.target.closest('.format__dropdown')) {
                this.isDropdownOpen = false;
            }
            
            if (!e.target.closest('.mark__dropdown')) {
                this.isMarkDropdownOpen = false;
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
        width: 55%; /* Ширина 60% */
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
        display: flex; /* Flex контейнер */
        align-items: center;
        gap: 20px;
        height: 100%;
    }

    .consent-checkbox {
        display: flex;
        gap: 10px;
        max-width: 350px; /* Максимальная ширина текста */
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

    .add-button {
        background: #4F5BDF;
        color: white;
        border: none;
        border-radius: 15px;
        padding: 8px 15px;
        font-size: 12px;
        cursor: pointer;
        transition: background-color 0.2s;
        margin-top: 0; /* Опущена на уровень dropdown button */
    }

    .add-button:hover:not(:disabled) {
        background: #3a45c0;
    }

    .add-button:disabled {
        background: #a2a2a2;
        cursor: not-allowed;
        opacity: 0.6;
    }

    .format__dropdown {
        position: relative;
    }

    .dropdown__button {
        width: 200px;
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

    .completion__header {
        padding-bottom: 15px;
    }

    .content__country {
        display: flex;
        gap: 10px;
        align-items: center;
    }

    .button__flag {
        width: 16px;
        height: 12px;
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
        width: 150px;
        background: #FFF;
        border: 1px solid #e6e6e6;
        border-radius: 10px;
        margin-top: 5px;
        box-shadow: 0 3px 10px rgba(0,0,0,0.1);
        z-index: 1000;
    }

    .dropdown__item {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 8px 15px;
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

    .item__flag {
        width: 16px;
        height: 12px;
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

    .fact-text {
        font-size: 13px;
    }

    .number__field {
        width: 100%;
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
    }

    .number__input--fact {
        width: 100%;
        text-align: left;
        padding: 0 15px;
        color: #a2a2a2;
    }

    /* Стили для российского формата (4 клетки) */
    .number__field:has(.number__input:nth-child(4)) .number__input {
        width: 25%;
    }

    /* Стили для азербайджанского формата (3 клетки) */
    .number__field:has(.number__input:nth-child(3)) .number__input {
        width: 33.33%;
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
        border-radius: 10px;
        margin-top: 5px;
        box-shadow: 0 3px 10px rgba(0,0,0,0.1);
        z-index: 1000;
        max-height: 200px;
        overflow: hidden;
    }

    .mark__search {
        padding: 10px;
        border-bottom: 1px solid #e6e6e6;
    }

    .mark__search-input {
        width: 100%;
        border: 1px solid #e6e6e6;
        border-radius: 5px;
        padding: 5px 10px;
        outline: none;
        font-size: 14px;
    }

    .mark__dropdown-list {
        max-height: 150px;
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

    /* Unloading places styles */
    .completion__unloading {
        margin-top: 15px;
    }

    .unloading__grid {
        display: grid;
        grid-template-columns: repeat(3, 1fr);
        gap: 10px;
        row-gap: 5px;
        max-width: 425px;
        margin-top: 5px;
    }

    .unloading__item {
        height: 35px;
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
    }

    .unloading__item:hover:not(.unloading__item--active) {
        background: #e8e8e8;
    }

    .unloading__item--active {
        background: #4F5BDF;
        color: #fff;
        border-color: #4F5BDF;
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
        font-size: 15px; /* Увеличен шрифт до 15px */
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
    }

    .delete-btn {
        background: none;
        border: none;
        cursor: pointer;
        padding: 5px;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .delete-btn:hover {
        background: #f0f0f0;
        border-radius: 5px;
    }

    .delete-icon {
        width: 16px;
        height: 16px;
        opacity: 0.7;
    }

    .delete-btn:hover .delete-icon {
        opacity: 1;
    }

    .no-vehicles {
        text-align: center;
        padding: 20px;
        color: #a2a2a2;
        font-size: 14px;
    }
</style>