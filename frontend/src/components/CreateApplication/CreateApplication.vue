<template>
    <div class="create">
        <div class="create__header">
            <div class="create__title">
                <h3>Оформление и подача заявки</h3>
                <button class="tables__instruction">
                    <img src="@/assets/icons/instruction.png" class="tables__icon" />
                    <p class="instruction__text">Инструкция</p>
                </button>
            </div>
            <h4>Автозаявка №{{ applicationNumber }}</h4>
        </div>
        <div class="create__container">
            <BlankSelector />
            <div class="create__form">
                <!-- 1 ряд: Письмо сопроводительное, Согласие, Отправка -->
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

                <!-- 2 ряд: Организация, Компания, Ответственное лицо -->
                <UserInfoRow 
                    :organization="organization"
                    :company="company"
                    :responsible-person="responsiblePerson"
                    :phone-number="phoneNumber"
                    :errors="errors"
                    @update:organization="organization = $event"
                    @update:company="company = $event"
                    @update:responsible-person="responsiblePerson = $event"
                    @update:phone-number="phoneNumber = $event"
                    @validate-field="validateField"
                    @format-phone="formatPhoneNumber"
                    @clear-phone="clearPhoneFormat"
                />

                <!-- 3 ряд: Заголовок, Дата действия, Время пребывания -->
                <div class="form__info-row">
                    <DateRangeSection
                        :is-one-day="isOneDay"
                        :start-date="startDate"
                        :end-date="endDate"
                        :single-date="singleDate"
                        :start-time="startTime"
                        :end-time="endTime"
                        :errors="errors"
                        @update:is-one-day="isOneDay = $event"
                        @update:start-date="startDate = $event"
                        @update:end-date="endDate = $event"
                        @update:single-date="singleDate = $event"
                        @update:start-time="startTime = $event"
                        @update:end-time="endTime = $event"
                        @validate-field="validateField"
                        @validate-date-range="validateDateRange"
                        @validate-time-range="validateTimeRange"
                    />
                </div>

                <!-- 4 ряд: VehicleForm и Список транспортных средств -->
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
                    <VehiclesList 
                        :vehicles="sortedVehicles"
                        :sort-field="sortField"
                        :sort-direction="sortDirection"
                        @sort="sortBy"
                        @edit-vehicle="editVehicle"
                        @delete-vehicle="deleteVehicle"
                    />
                </div>
            </div>
        </div>

        <!-- Модальное окно привязки новых машин -->
        <BindingModal
            v-if="showBindingModal"
            :new-cars-to-bind="newCarsToBind"
            :organization="organization"
            :company="company"
            :has-organization="hasOrganization"
            :has-company="hasCompany"
            @toggle-car-binding="toggleCarBinding"
            @confirm-binding="confirmBinding"
            @skip-binding="skipBinding"
            @close="closeBindingModal"
        />
    </div>
</template>

<script>
import VehicleForm from '@/components/VehicleForm.vue'
import BlankSelector from '../BlankSelector.vue';
import UserInfoRow from './UserInfoRow.vue';
import DateRangeSection from './DateRangeSection.vue';
import VehiclesList from './VehiclesList.vue';
import BindingModal from './BindingModal.vue';

export default {
    name: 'CreateApplication',
    components: {
        VehicleForm,
        BlankSelector,
        UserInfoRow,
        DateRangeSection,
        VehiclesList,
        BindingModal
    },
    data() {
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
    }
}
</script>

<style scoped>
    .create {
        padding: 20px;
    }

    .create__title {
        display: flex;
        display: flex;
        gap: 10px;
    }

    .create__header {
        display: flex;
        align-items: center;
        justify-content: space-between;
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

    .form__info-row {
        padding: 15px;
        display: flex;
        gap: 50px;
        border-bottom: 1px solid #e6e6e6;
    }

    h4 {
        font-size: 24px;
        font-weight: 900;
        text-shadow: 1px 2px rgba(0,0,0,0.2);
    }

    .form__data {
        display: flex;
    }

    .blue {
        color: #4F5BDF;
    }
</style>