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
            <h4>{{ currentFormTitle }}</h4>
        </div>
        <div class="create__container">
            <BlankSelector 
                ref="blankSelector"
                :current-application-data="currentApplicationData"
                @attachment-selected="handleAttachmentSelected"
                @attachment-added="handleAttachmentAdded"
                @attachment-removed="handleAttachmentRemoved"
            />
            
            <div v-if="selectedAttachment" class="create__form">
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

                <!-- 3 ряд: Заголовок, Дата действия, Время пребывания (теперь индивидуально для вложения) -->
                <div class="form__info-row">
                    <DateRangeSection
                        :is-one-day="currentAttachmentData.isOneDay"
                        :start-date="currentAttachmentData.startDate"
                        :end-date="currentAttachmentData.endDate"
                        :single-date="currentAttachmentData.singleDate"
                        :start-time="currentAttachmentData.startTime"
                        :end-time="currentAttachmentData.endTime"
                        :roof-access="currentAttachmentData.roofAccess"
                        :free-parking="currentAttachmentData.freeParking"
                        :notify-situation-center="currentAttachmentData.notifySituationCenter"
                        :errors="currentAttachmentErrors"
                        @update:is-one-day="updateAttachmentData('isOneDay', $event)"
                        @update:start-date="updateAttachmentData('startDate', $event)"
                        @update:end-date="updateAttachmentData('endDate', $event)"
                        @update:single-date="updateAttachmentData('singleDate', $event)"
                        @update:start-time="updateAttachmentData('startTime', $event)"
                        @update:end-time="updateAttachmentData('endTime', $event)"
                        @update:roof-access="updateAttachmentData('roofAccess', $event)"
                        @update:free-parking="updateAttachmentData('freeParking', $event)"
                        @update:notify-situation-center="updateAttachmentData('notifySituationCenter', $event)"
                        @validate-field="validateAttachmentField"
                        @validate-date-range="validateAttachmentDateRange"
                        @validate-time-range="validateAttachmentTimeRange"
                    />
                </div>

                <!-- 4 ряд: Динамические формы в зависимости от типа вложения -->
                <div class="form__data">
                    <!-- Для автомобилей -->
                    <template v-if="selectedAttachment && selectedAttachment.attachment_type === 'cars'">
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
                            @edit-cancelled="handleVehicleEditCancelled"
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
                    </template>

                    <!-- Для людей/сотрудников -->
                    <template v-else-if="selectedAttachment && selectedAttachment.attachment_type === 'people'">
                        <EmployeeForm 
                            :user-organization="organization"
                            :user-organization-id="organizationId"
                            :user-company="company"
                            :user-company-id="companyId"
                            :existing-employees="employees"
                            :key="employeeFormKey"
                            @employee-added="handleEmployeeAdded"
                            @employees-added="handleEmployeesAdded"
                            @employee-updated="handleEmployeeUpdated"
                            @edit-cancelled="handleEmployeeEditCancelled"
                            ref="employeeForm"
                        />
                        <EmployeesList 
                            :employees="sortedEmployees"
                            :sort-field="sortField"
                            :sort-direction="sortDirection"
                            @sort="sortBy"
                            @edit-employee="editEmployee"
                            @delete-employee="deleteEmployee"
                        />
                    </template>

                    <!-- Для ТМЦ -->
                    <template v-else-if="selectedAttachment && selectedAttachment.attachment_type === 'items'">
                        <ItemsForm 
                            :existing-items="items"
                            :key="itemsFormKey"
                            @item-added="handleItemAdded"
                            @items-added="handleItemsAdded"
                            @item-updated="handleItemUpdated"
                            @edit-cancelled="handleItemEditCancelled"
                            ref="itemsForm"
                        />
                        <ItemsList 
                            :items="sortedItems"
                            :sort-field="sortField"
                            :sort-direction="sortDirection"
                            @sort="sortBy"
                            @edit-item="editItem"
                            @delete-item="deleteItem"
                        />
                    </template>
                </div>
            </div>
            
            <!-- Заглушка для формы, когда вложение не выбрано -->
            <div v-else class="form-placeholder">
                <div class="placeholder-content">
                    <p>Добавьте или выберите вложение для начала работы</p>
                </div>
            </div>
        </div>

        <!-- Универсальное модальное окно привязки -->
        <UniversalBindingModal
            v-if="showBindingModal"
            :new-vehicles-to-bind="newVehiclesToBind"
            :new-employees-to-bind="newEmployeesToBind"
            :organization="organization"
            :company="company"
            :has-organization="hasOrganization"
            :has-company="hasCompany"
            @confirm-binding="confirmBinding"
            @skip-binding="skipBinding"
            @close="closeBindingModal"
        />
    </div>
</template>

<script>
import BlankSelector from '../BlankSelector.vue';
import UserInfoRow from './UserInfoRow.vue';
import DateRangeSection from './DateRangeSection.vue';
import VehicleForm from './VehicleForm.vue';
import VehiclesList from './VehiclesList.vue';
import EmployeeForm from './EmployeeForm.vue';
import EmployeesList from './EmployeesList.vue';
import ItemsForm from './ItemsForm.vue';
import ItemsList from './ItemsList.vue';
import UniversalBindingModal from './UniversalBindingModal.vue';

export default {
    name: 'CreateApplication',
    components: {
        BlankSelector,
        UserInfoRow,
        DateRangeSection,
        VehicleForm,
        VehiclesList,
        EmployeeForm,
        EmployeesList,
        ItemsForm,
        ItemsList,
        UniversalBindingModal
    },
    data() {
        return {
            // Общие данные формы (общие для всех вложений)
            message: '',
            organization: '',
            company: '',
            responsiblePerson: '',
            phoneNumber: '',
            rawPhoneNumber: '',
            consentGiven: false,
            applicationNumber: 1,

            // IDs
            organizationId: null,
            companyId: null,
            
            // Данные вложений
            selectedAttachment: null,
            attachments: [],

            // Данные для разных типов вложений (храним по attachment_id)
            vehiclesByAttachment: {},
            employeesByAttachment: {},
            itemsByAttachment: {},
            
            // Данные дат и времени для каждого вложения (храним по attachment_id)
            attachmentDatesByAttachment: {},
            
            // Счетчики ID (храним глобально)
            vehicleIdCounter: 1,
            employeeIdCounter: 1,
            itemIdCounter: 1,
            
            // Сортировка
            sortField: null,
            sortDirection: 'asc',
            
            // Валидация
            errors: {
                organization: '',
                company: '',
                responsiblePerson: '',
                phone: ''
            },
            
            // Ключи для принудительного перерисовывания форм
            vehicleFormKey: 0,
            employeeFormKey: 0,
            itemsFormKey: 0,

            // Модальное окно привязки
            showBindingModal: false,
            newVehiclesToBind: [],
            newEmployeesToBind: [],
            hasOrganization: false,
            hasCompany: false,
            
            // Данные текущей заявки (для редактирования)
            currentApplicationData: {}
        }
    },
    computed: {
        currentFormTitle() {
            if (this.selectedAttachment) {
                return this.selectedAttachment.display_name;
            }
            return 'Добавьте новое вложение';
        },
        
        // Получаем данные для текущего выбранного вложения
        vehicles() {
            if (!this.selectedAttachment) return [];
            return this.vehiclesByAttachment[this.selectedAttachment.id] || [];
        },
        
        employees() {
            if (!this.selectedAttachment) return [];
            return this.employeesByAttachment[this.selectedAttachment.id] || [];
        },
        
        items() {
            if (!this.selectedAttachment) return [];
            return this.itemsByAttachment[this.selectedAttachment.id] || [];
        },
        
        // Данные дат и времени для текущего вложения
        currentAttachmentData() {
            if (!this.selectedAttachment) {
                return this.getDefaultDateData();
            }
            const data = this.attachmentDatesByAttachment[this.selectedAttachment.id];
            return data || this.getDefaultDateData();
        },
        
        // Ошибки для текущего вложения
        currentAttachmentErrors() {
            if (!this.selectedAttachment) return {};
            const data = this.attachmentDatesByAttachment[this.selectedAttachment.id];
            return data?.errors || {};
        },
        
        canSubmit() {
            // Если вложений нет, нельзя отправлять
            if (this.attachments.length === 0) {
                return false;
            }

            // Проверка общих обязательных полей
            const hasRequiredFields = 
                this.organization && 
                this.company && 
                this.responsiblePerson && 
                this.phoneNumber &&
                this.consentGiven;

            if (!hasRequiredFields) {
                return false;
            }

            // Проверка наличия данных во всех вложениях
            let allAttachmentsValid = true;
            
            // Проверяем каждое вложение
            this.attachments.forEach(attachment => {
                let hasAttachmentData = false;
                let hasValidDates = false;
                let hasValidTime = false;

                // Проверка наличия данных
                switch (attachment.attachment_type) {
                    case 'cars':
                        hasAttachmentData = (this.vehiclesByAttachment[attachment.id] || []).length > 0;
                        break;
                    case 'people':
                        hasAttachmentData = (this.employeesByAttachment[attachment.id] || []).length > 0;
                        break;
                    case 'items':
                        hasAttachmentData = (this.itemsByAttachment[attachment.id] || []).length > 0;
                        break;
                }

                // Проверка дат и времени
                const dateData = this.attachmentDatesByAttachment[attachment.id];
                if (dateData) {
                    hasValidDates = dateData.isOneDay 
                        ? !!(dateData.singleDate && dateData.startTime && dateData.endTime)
                        : !!(dateData.startDate && dateData.endDate && dateData.startTime && dateData.endTime);

                    hasValidTime = !!(dateData.startTime && dateData.endTime && dateData.startTime < dateData.endTime);
                }

                if (!hasAttachmentData || !hasValidDates || !hasValidTime) {
                    allAttachmentsValid = false;
                }
            });

            return allAttachmentsValid;
        },
        
        // Сортированные списки
        sortedVehicles() {
            if (!this.sortField || !this.vehicles.length) {
                return this.vehicles;
            }
            
            return [...this.vehicles].sort((a, b) => {
                let valueA, valueB;
                
                switch (this.sortField) {
                    case 'number':
                        return this.sortDirection === 'asc' ? a.id - b.id : b.id - a.id;
                    case 'brand':
                        valueA = a.brand.toLowerCase();
                        valueB = b.brand.toLowerCase();
                        break;
                    case 'model':
                        valueA = a.model.toLowerCase();
                        valueB = b.model.toLowerCase();
                        break;
                    case 'plate':
                        valueA = a.licensePlate.toLowerCase();
                        valueB = b.licensePlate.toLowerCase();
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
        
        sortedEmployees() {
            if (!this.sortField || !this.employees.length) {
                return this.employees;
            }
            
            return [...this.employees].sort((a, b) => {
                let valueA, valueB;
                
                switch (this.sortField) {
                    case 'number':
                        return this.sortDirection === 'asc' ? a.id - b.id : b.id - a.id;
                    case 'name':
                        valueA = this.formatFullName(a).toLowerCase();
                        valueB = this.formatFullName(b).toLowerCase();
                        break;
                    case 'position':
                        valueA = a.position.toLowerCase();
                        valueB = b.position.toLowerCase();
                        break;
                    case 'tables':
                        valueA = a.passageTables.toLowerCase();
                        valueB = b.passageTables.toLowerCase();
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
        
        sortedItems() {
            if (!this.sortField || !this.items.length) {
                return this.items;
            }
            
            return [...this.items].sort((a, b) => {
                let valueA, valueB;
                
                switch (this.sortField) {
                    case 'number':
                        return this.sortDirection === 'asc' ? a.id - b.id : b.id - a.id;
                    case 'name':
                        valueA = a.itemName.toLowerCase();
                        valueB = b.itemName.toLowerCase();
                        break;
                    case 'quantity':
                        return this.sortDirection === 'asc' ? a.quantity - b.quantity : b.quantity - a.quantity;
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
        // Получение дефолтных данных дат
        getDefaultDateData() {
            return {
                isOneDay: false,
                startDate: '',
                endDate: '',
                singleDate: '',
                startTime: '',
                endTime: '',
                roofAccess: false,
                freeParking: false,
                notifySituationCenter: false,
                errors: {}
            };
        },
        
        // Обновление данных для текущего вложения
        updateAttachmentData(field, value) {
            if (!this.selectedAttachment) return;
            
            const attachmentId = this.selectedAttachment.id;
            
            // Инициализируем данные, если их нет
            if (!this.attachmentDatesByAttachment[attachmentId]) {
                this.attachmentDatesByAttachment[attachmentId] = this.getDefaultDateData();
            }
            
            // Обновляем поле
            this.attachmentDatesByAttachment[attachmentId][field] = value;
            
            // Сохраняем в localStorage
            this.saveToLocalStorage();
        },
        
        // Валидация полей для текущего вложения
        validateAttachmentField(field) {
            if (!this.selectedAttachment) return;
            
            const attachmentId = this.selectedAttachment.id;
            const dateData = this.attachmentDatesByAttachment[attachmentId];
            if (!dateData) return;
            
            let timeRegex;
            
            if (!dateData.errors) {
                dateData.errors = {};
            }
            
            switch (field) {
                case 'startDate':
                    dateData.errors.startDate = dateData.isOneDay ? '' : (dateData.startDate ? '' : 'Укажите дату начала');
                    break;
                case 'endDate':
                    dateData.errors.endDate = dateData.isOneDay ? '' : (dateData.endDate ? '' : 'Укажите дату окончания');
                    break;
                case 'singleDate':
                    dateData.errors.singleDate = !dateData.isOneDay ? '' : (dateData.singleDate ? '' : 'Укажите дату');
                    break;
                case 'startTime':
                    timeRegex = /^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$/;
                    dateData.errors.startTime = dateData.startTime && timeRegex.test(dateData.startTime) ? '' : 'Введите время в формате ЧЧ:ММ';
                    break;
                case 'endTime':
                    timeRegex = /^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$/;
                    dateData.errors.endTime = dateData.endTime && timeRegex.test(dateData.endTime) ? '' : 'Введите время в формате ЧЧ:ММ';
                    break;
            }
            
            this.saveToLocalStorage();
        },
        
        validateAttachmentDateRange() {
            if (!this.selectedAttachment) return;
            
            const attachmentId = this.selectedAttachment.id;
            const dateData = this.attachmentDatesByAttachment[attachmentId];
            if (!dateData || !dateData.errors) return;
            
            if (!dateData.isOneDay && dateData.startDate && dateData.endDate) {
                const start = new Date(dateData.startDate.split('.').reverse().join('-'));
                const end = new Date(dateData.endDate.split('.').reverse().join('-'));
                if (start > end) {
                    dateData.errors.endDate = 'Дата окончания не может быть раньше даты начала';
                } else {
                    dateData.errors.endDate = '';
                }
            }
            
            this.saveToLocalStorage();
        },
        
        validateAttachmentTimeRange() {
            if (!this.selectedAttachment) return;
            
            const attachmentId = this.selectedAttachment.id;
            const dateData = this.attachmentDatesByAttachment[attachmentId];
            if (!dateData || !dateData.errors) return;
            
            if (dateData.startTime && dateData.endTime) {
                if (dateData.startTime >= dateData.endTime) {
                    dateData.errors.endTime = 'Время окончания должно быть позже времени начала';
                } else {
                    dateData.errors.endTime = '';
                }
            }
            
            this.saveToLocalStorage();
        },
        
        // Новый метод для очистки данных формы
        clearFormData() {
            // Очищаем данные формы ТОЛЬКО если нет вложений
            if (this.attachments.length === 0 && this.selectedAttachment === null) {
                this.message = '';
                this.consentGiven = false;
                
                // Сбрасываем ошибки
                this.errors = {
                    organization: '',
                    company: '',
                    responsiblePerson: '',
                    phone: ''
                };
            }
        },
        
        // Методы для работы с пользователем
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
                    
                    // Форматирование телефона
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

        handleAttachmentSelected(attachment) {
            if (!attachment) {
                this.selectedAttachment = null;
                // Если нет выбранных вложений, проверяем нужно ли очищать данные
                this.clearFormData();
                return;
            }
            
            this.selectedAttachment = attachment;
            // Восстанавливаем данные для этого вложения
            this.restoreAttachmentData(attachment);
        },

        handleAttachmentAdded(attachment) {
            this.attachments.push(attachment);
            // Создаем пустой массив данных для нового вложения
            if (attachment.attachment_type === 'cars') {
                this.vehiclesByAttachment[attachment.id] = [];
            } else if (attachment.attachment_type === 'people') {
                this.employeesByAttachment[attachment.id] = [];
            } else if (attachment.attachment_type === 'items') {
                this.itemsByAttachment[attachment.id] = [];
            }
            
            // Создаем дефолтные данные дат для нового вложения
            this.attachmentDatesByAttachment[attachment.id] = this.getDefaultDateData();
            
            // Выбираем это вложение
            this.selectAttachment(attachment);
            
            // Сохраняем состояние в localStorage
            this.saveToLocalStorage();
        },

        selectAttachment(attachment) {
            this.selectedAttachment = attachment;
        },

        handleAttachmentRemoved(attachment) {
            this.attachments = this.attachments.filter(a => a.id !== attachment.id);
            
            // Удаляем данные вложения
            if (attachment.attachment_type === 'cars') {
                delete this.vehiclesByAttachment[attachment.id];
            } else if (attachment.attachment_type === 'people') {
                delete this.employeesByAttachment[attachment.id];
            } else if (attachment.attachment_type === 'items') {
                delete this.itemsByAttachment[attachment.id];
            }
            
            // Удаляем данные дат вложения
            delete this.attachmentDatesByAttachment[attachment.id];
            
            if (this.selectedAttachment && this.selectedAttachment.id === attachment.id) {
                this.selectedAttachment = null;
            }
            
            // Проверяем нужно ли очищать данные формы (после удаления)
            this.clearFormData();
            
            // Сохраняем состояние в localStorage
            this.saveToLocalStorage();
        },

        restoreAttachmentData(attachment) {
            // При переключении вложения сохраняем текущие данные
            if (this.selectedAttachment) {
                this.saveCurrentAttachmentData();
            }
            
            // Загружаем данные для нового вложения
            this.loadAttachmentData(attachment);
        },

        saveCurrentAttachmentData() {
            if (!this.selectedAttachment) return;
            
            // Сохраняем текущие данные в соответствующее хранилище
            switch (this.selectedAttachment.attachment_type) {
                case 'cars':
                    this.vehiclesByAttachment[this.selectedAttachment.id] = this.vehicles;
                    break;
                case 'people':
                    this.employeesByAttachment[this.selectedAttachment.id] = this.employees;
                    break;
                case 'items':
                    this.itemsByAttachment[this.selectedAttachment.id] = this.items;
                    break;
            }
            
            // Сохраняем в localStorage
            this.saveToLocalStorage();
        },

        loadAttachmentData(attachment) {
            if (!attachment) return;
            
            // Загружаем данные из соответствующего хранилища
            switch (attachment.attachment_type) {
                case 'cars':
                    // Если данных нет, создаем пустой массив
                    if (!this.vehiclesByAttachment[attachment.id]) {
                        this.vehiclesByAttachment[attachment.id] = [];
                    }
                    break;
                case 'people':
                    if (!this.employeesByAttachment[attachment.id]) {
                        this.employeesByAttachment[attachment.id] = [];
                    }
                    break;
                case 'items':
                    if (!this.itemsByAttachment[attachment.id]) {
                        this.itemsByAttachment[attachment.id] = [];
                    }
                    break;
            }
            
            // Загружаем данные дат
            if (!this.attachmentDatesByAttachment[attachment.id]) {
                this.attachmentDatesByAttachment[attachment.id] = this.getDefaultDateData();
            }
        },

        // Методы для автомобилей
        handleVehicleAdded(newVehicle) {
            const vehicleWithId = {
                ...newVehicle,
                id: this.vehicleIdCounter++,
                isExisting: false
            };
            
            if (this.selectedAttachment) {
                if (!this.vehiclesByAttachment[this.selectedAttachment.id]) {
                    this.vehiclesByAttachment[this.selectedAttachment.id] = [];
                }
                this.vehiclesByAttachment[this.selectedAttachment.id].push(vehicleWithId);
                
                // Сохраняем в localStorage
                this.saveToLocalStorage();
            }
        },

        handleVehiclesAdded(vehicles) {
            vehicles.forEach(vehicle => {
                const vehicleWithId = {
                    ...vehicle,
                    id: this.vehicleIdCounter++,
                    isExisting: false
                };
                
                if (this.selectedAttachment) {
                    if (!this.vehiclesByAttachment[this.selectedAttachment.id]) {
                        this.vehiclesByAttachment[this.selectedAttachment.id] = [];
                    }
                    this.vehiclesByAttachment[this.selectedAttachment.id].push(vehicleWithId);
                }
            });
            
            // Сохраняем в localStorage
            this.saveToLocalStorage();
        },

        handleVehicleUpdated(updatedVehicle) {
            if (!this.selectedAttachment) return;
            
            const vehicles = this.vehiclesByAttachment[this.selectedAttachment.id];
            if (!vehicles) return;
            
            const index = vehicles.findIndex(v => v.id === updatedVehicle.id);
            if (index !== -1) {
                vehicles.splice(index, 1, updatedVehicle);
                // Сохраняем в localStorage
                this.saveToLocalStorage();
            }
        },

        handleVehicleEditCancelled() {
            this.vehicleFormKey += 1;
        },

        deleteVehicle(vehicleId) {
            if (!this.selectedAttachment) return;
            
            const vehicles = this.vehiclesByAttachment[this.selectedAttachment.id];
            if (!vehicles) return;
            
            const index = vehicles.findIndex(vehicle => vehicle.id === vehicleId);
            if (index !== -1) {
                vehicles.splice(index, 1);
                // Сохраняем в localStorage
                this.saveToLocalStorage();
            }
        },

        editVehicle(vehicle) {
            if (this.$refs.vehicleForm) {
                this.$refs.vehicleForm.editVehicle(vehicle);
            }
        },

        // Методы для сотрудников
        handleEmployeeAdded(newEmployee) {
            const employeeWithId = {
                ...newEmployee,
                id: this.employeeIdCounter++,
                isExisting: false
            };
            
            if (this.selectedAttachment) {
                if (!this.employeesByAttachment[this.selectedAttachment.id]) {
                    this.employeesByAttachment[this.selectedAttachment.id] = [];
                }
                this.employeesByAttachment[this.selectedAttachment.id].push(employeeWithId);
                
                // Сохраняем в localStorage
                this.saveToLocalStorage();
            }
        },

        handleEmployeesAdded(employees) {
            employees.forEach(employee => {
                const employeeWithId = {
                    ...employee,
                    id: this.employeeIdCounter++,
                    isExisting: false
                };
                
                if (this.selectedAttachment) {
                    if (!this.employeesByAttachment[this.selectedAttachment.id]) {
                        this.employeesByAttachment[this.selectedAttachment.id] = [];
                    }
                    this.employeesByAttachment[this.selectedAttachment.id].push(employeeWithId);
                }
            });
            
            // Сохраняем в localStorage
            this.saveToLocalStorage();
        },

        handleEmployeeUpdated(updatedEmployee) {
            if (!this.selectedAttachment) return;
            
            const employees = this.employeesByAttachment[this.selectedAttachment.id];
            if (!employees) return;
            
            const index = employees.findIndex(e => e.id === updatedEmployee.id);
            if (index !== -1) {
                employees.splice(index, 1, updatedEmployee);
                // Сохраняем в localStorage
                this.saveToLocalStorage();
            }
        },

        handleEmployeeEditCancelled() {
            this.employeeFormKey += 1;
        },

        deleteEmployee(employeeId) {
            if (!this.selectedAttachment) return;
            
            const employees = this.employeesByAttachment[this.selectedAttachment.id];
            if (!employees) return;
            
            const index = employees.findIndex(employee => employee.id === employeeId);
            if (index !== -1) {
                employees.splice(index, 1);
                // Сохраняем в localStorage
                this.saveToLocalStorage();
            }
        },

        editEmployee(employee) {
            if (this.$refs.employeeForm) {
                this.$refs.employeeForm.editEmployee(employee);
            }
        },

        formatFullName(employee) {
            const parts = [];
            if (employee.lastName) parts.push(employee.lastName);
            if (employee.firstName) parts.push(employee.firstName);
            if (employee.middleName) parts.push(employee.middleName);
            return parts.join(' ') || 'Не указано';
        },

        // Методы для ТМЦ
        handleItemAdded(newItem) {
            const itemWithId = {
                ...newItem,
                id: this.itemIdCounter++
            };
            
            if (this.selectedAttachment) {
                if (!this.itemsByAttachment[this.selectedAttachment.id]) {
                    this.itemsByAttachment[this.selectedAttachment.id] = [];
                }
                this.itemsByAttachment[this.selectedAttachment.id].push(itemWithId);
                
                // Сохраняем в localStorage
                this.saveToLocalStorage();
            }
        },

        handleItemsAdded(items) {
            items.forEach(item => {
                const itemWithId = {
                    ...item,
                    id: this.itemIdCounter++
                };
                
                if (this.selectedAttachment) {
                    if (!this.itemsByAttachment[this.selectedAttachment.id]) {
                        this.itemsByAttachment[this.selectedAttachment.id] = [];
                    }
                    this.itemsByAttachment[this.selectedAttachment.id].push(itemWithId);
                }
            });
            
            // Сохраняем в localStorage
            this.saveToLocalStorage();
        },

        handleItemUpdated(updatedItem) {
            if (!this.selectedAttachment) return;
            
            const items = this.itemsByAttachment[this.selectedAttachment.id];
            if (!items) return;
            
            const index = items.findIndex(e => e.id === updatedItem.id);
            if (index !== -1) {
                items.splice(index, 1, updatedItem);
                // Сохраняем в localStorage
                this.saveToLocalStorage();
            }
        },

        handleItemEditCancelled() {
            this.itemsFormKey += 1;
        },

        deleteItem(itemId) {
            if (!this.selectedAttachment) return;
            
            const items = this.itemsByAttachment[this.selectedAttachment.id];
            if (!items) return;
            
            const index = items.findIndex(item => item.id === itemId);
            if (index !== -1) {
                items.splice(index, 1);
                // Сохраняем в localStorage
                this.saveToLocalStorage();
            }
        },

        editItem(item) {
            if (this.$refs.itemsForm) {
                this.$refs.itemsForm.editItem(item);
            }
        },

        // Сортировка
        sortBy(field) {
            if (this.sortField === field) {
                this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
            } else {
                this.sortField = field;
                this.sortDirection = 'asc';
            }
        },

        // Валидация общих полей
        validateField(field) {
            let phoneRegex;

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
            }
        },

        validateAllFields() {
            this.validateField('organization');
            this.validateField('company');
            this.validateField('responsiblePerson');
            this.validateField('phone');
            
            // Валидируем даты для всех вложений
            Object.keys(this.attachmentDatesByAttachment).forEach(attachmentId => {
                const dateData = this.attachmentDatesByAttachment[attachmentId];
                if (dateData) {
                    this.validateAttachmentDateRangeForAttachment(attachmentId);
                    this.validateAttachmentTimeRangeForAttachment(attachmentId);
                }
            });
        },
        
        validateAttachmentDateRangeForAttachment(attachmentId) {
            const dateData = this.attachmentDatesByAttachment[attachmentId];
            if (!dateData || !dateData.errors) return;
            
            if (!dateData.isOneDay && dateData.startDate && dateData.endDate) {
                const start = new Date(dateData.startDate.split('.').reverse().join('-'));
                const end = new Date(dateData.endDate.split('.').reverse().join('-'));
                if (start > end) {
                    dateData.errors.endDate = 'Дата окончания не может быть раньше даты начала';
                } else {
                    dateData.errors.endDate = '';
                }
            }
        },
        
        validateAttachmentTimeRangeForAttachment(attachmentId) {
            const dateData = this.attachmentDatesByAttachment[attachmentId];
            if (!dateData || !dateData.errors) return;
            
            if (dateData.startTime && dateData.endTime) {
                if (dateData.startTime >= dateData.endTime) {
                    dateData.errors.endTime = 'Время окончания должно быть позже времени начала';
                } else {
                    dateData.errors.endTime = '';
                }
            }
        },

        // Отправка заявки
        async submitApplication() {
            this.validateAllFields();
            
            if (!this.canSubmit) {
                alert('Заполните все обязательные поля во всех вложениях');
                return;
            }

            // Валидация дат для всех вложений
            let hasDateErrors = false;
            let errorMessage = '';
            
            this.attachments.forEach(attachment => {
                const dateData = this.attachmentDatesByAttachment[attachment.id];
                if (dateData) {
                    // Валидация дат
                    if (!dateData.isOneDay && dateData.startDate && dateData.endDate) {
                        const start = new Date(dateData.startDate.split('.').reverse().join('-'));
                        const end = new Date(dateData.endDate.split('.').reverse().join('-'));
                        if (start > end) {
                            hasDateErrors = true;
                            errorMessage = `В вложении "${attachment.display_name}" дата окончания не может быть раньше даты начала`;
                        }
                    }

                    // Валидация времени
                    if (dateData.startTime && dateData.endTime && dateData.startTime >= dateData.endTime) {
                        hasDateErrors = true;
                        errorMessage = `В вложении "${attachment.display_name}" время окончания должно быть позже времени начала`;
                    }
                }
            });

            if (hasDateErrors) {
                alert(errorMessage);
                return;
            }

            // Собираем все новые автомобили и сотрудники из ВСЕХ вложений
            await this.collectNewDataForBinding();

            // Фильтруем автомобили и сотрудники "По факту"
            const vehiclesForBinding = this.newVehiclesToBind.filter(vehicle => 
                vehicle.plateNumber !== 'По факту' && vehicle.mark !== 'По факту'
            );
            
            const employeesForBinding = this.newEmployeesToBind.filter(employee => 
                employee.passportSeriesNumber !== 'По факту' && employee.position !== 'По факту'
            );

            // Показываем модальное окно привязки, если есть новые автомобили или сотрудники для привязки
            if (vehiclesForBinding.length > 0 || employeesForBinding.length > 0) {
                this.showBindingModal = true;
            } else {
                // Если нет новых автомобилей/сотрудников для привязки, сразу отправляем заявку
                await this.sendCompleteApplication();
            }
        },

        // Сбор всех новых данных для привязки из всех вложений
        async collectNewDataForBinding() {
            this.newVehiclesToBind = [];
            this.newEmployeesToBind = [];
            
            // Загружаем существующие автомобили и сотрудники из БД
            const existingVehicles = await this.loadExistingVehicles();
            const existingEmployees = await this.loadExistingEmployees();
            
            // Собираем все автомобили из всех вложений типа 'cars'
            Object.keys(this.vehiclesByAttachment).forEach(attachmentId => {
                const vehicles = this.vehiclesByAttachment[attachmentId] || [];
                vehicles.forEach(vehicle => {
                    if (!vehicle.isExisting) {
                        // Проверяем, не "По факту" ли это автомобиль
                        const isByFact = vehicle.plateNumber === 'По факту' || vehicle.mark === 'По факту';
                        
                        if (!isByFact) {
                            // Проверяем, существует ли уже такая машина в БД
                            const alreadyExists = existingVehicles.some(existingVehicle => 
                                existingVehicle.number === vehicle.plateNumber && 
                                existingVehicle.mark === vehicle.mark
                            );
                            
                            if (!alreadyExists) {
                                this.newVehiclesToBind.push({
                                    ...vehicle,
                                    attachmentId: parseInt(attachmentId)
                                });
                            }
                        }
                    }
                });
            });
            
            // Собираем всех сотрудников из всех вложений типа 'people'
            Object.keys(this.employeesByAttachment).forEach(attachmentId => {
                const employees = this.employeesByAttachment[attachmentId] || [];
                employees.forEach(employee => {
                    if (!employee.isExisting) {
                        // Проверяем, не "По факту" ли это сотрудник
                        const isByFact = employee.passportSeriesNumber === 'По факту' || 
                                       employee.position === 'По факту';
                        
                        if (!isByFact) {
                            // Проверяем, существует ли уже такой сотрудник в БД
                            const alreadyExists = existingEmployees.some(existingEmployee => 
                                existingEmployee.passport_series_number === employee.passportSeriesNumber
                            );
                            
                            if (!alreadyExists) {
                                this.newEmployeesToBind.push({
                                    ...employee,
                                    attachmentId: parseInt(attachmentId)
                                });
                            }
                        }
                    }
                });
            });
        },

        // Загрузка существующих автомобилей из БД
        async loadExistingVehicles() {
            try {
                const token = localStorage.getItem("token");
                const response = await fetch("http://localhost:8080/unique-cars?filter_type=all", {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`
                    }
                });

                if (response.ok) {
                    return await response.json();
                }
                return [];
            } catch (error) {
                console.error("Ошибка при загрузке существующих автомобилей:", error);
                return [];
            }
        },

        // Загрузка существующих сотрудников из БД
        async loadExistingEmployees() {
            try {
                const token = localStorage.getItem("token");
                const response = await fetch("http://localhost:8080/unique-employees?filter_type=all", {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`
                    }
                });

                if (response.ok) {
                    return await response.json();
                }
                return [];
            } catch (error) {
                console.error("Ошибка при загрузке существующих сотрудников:", error);
                return [];
            }
        },
        
        async confirmBinding(bindingData) {
            try {
                const token = localStorage.getItem("token");
                
                // Привязка автомобилей (только если есть автомобили для привязки)
                if (this.newVehiclesToBind.length > 0 && bindingData.vehicles.hasVehiclesForBinding) {
                    // Фильтруем автомобили "По факту"
                    const vehiclesToBind = this.newVehiclesToBind.filter(vehicle => 
                        vehicle.plateNumber !== 'По факту' && vehicle.mark !== 'По факту'
                    );
                    
                    if (vehiclesToBind.length > 0) {
                        const vehiclePromises = vehiclesToBind.map(vehicle => {
                            const vehicleData = {
                                number: vehicle.plateNumber,
                                mark: vehicle.mark,
                                user_id: null,
                                organization_id: bindingData.vehicles.bindToOrganization ? this.organizationId : null,
                                company_id: bindingData.vehicles.bindToCompany ? this.companyId : null,
                                format_id: vehicle.formatId
                            };

                            return fetch("http://localhost:8080/unique-cars", {
                                method: "POST",
                                headers: {
                                    "Authorization": `Bearer ${token}`,
                                    "Content-Type": "application/json"
                                },
                                body: JSON.stringify(vehicleData)
                            });
                        });

                        await Promise.all(vehiclePromises);
                    }
                }

                // Привязка сотрудников (только если есть сотрудники для привязки)
                if (this.newEmployeesToBind.length > 0 && bindingData.employees.hasEmployeesForBinding) {
                    // Фильтруем сотрудников "По факту"
                    const employeesToBind = this.newEmployeesToBind.filter(employee => 
                        employee.passportSeriesNumber !== 'По факту' && employee.position !== 'По факту'
                    );
                    
                    if (employeesToBind.length > 0) {
                        const employeePromises = employeesToBind.map(employee => {
                            const employeeData = {
                                last_name: employee.lastName,
                                first_name: employee.firstName,
                                middle_name: employee.middleName,
                                position: employee.position,
                                citizenship_id: employee.citizenshipId,
                                passport_series_number: employee.passportSeriesNumber,
                                patent_number: employee.patentNumber,
                                other_permission: employee.otherPermission,
                                user_id: null,
                                organization_id: bindingData.employees.bindToOrganization ? this.organizationId : null,
                                company_id: bindingData.employees.bindToCompany ? this.companyId : null
                            };

                            return fetch("http://localhost:8080/unique-employees", {
                                method: "POST",
                                headers: {
                                    "Authorization": `Bearer ${token}`,
                                    "Content-Type": "application/json"
                                },
                                body: JSON.stringify(employeeData)
                            });
                        });

                        await Promise.all(employeePromises);
                    }
                }

                this.closeBindingModal();
                // После привязки отправляем заявку
                await this.sendCompleteApplication();

            } catch (error) {
                console.error('Ошибка при привязке:', error);
                this.closeBindingModal();
            }
        },

        skipBinding() {
            this.closeBindingModal();
            // Пропускаем привязку и отправляем заявку
            this.sendCompleteApplication();
        },

        closeBindingModal() {
            this.showBindingModal = false;
            this.newVehiclesToBind = [];
            this.newEmployeesToBind = [];
        },

        // Метод для очистки всех вложений
        clearAllAttachments() {
            // Сбрасываем данные всех вложений
            this.vehiclesByAttachment = {};
            this.employeesByAttachment = {};
            this.itemsByAttachment = {};
            this.attachmentDatesByAttachment = {};
            
            // Сбрасываем ID счетчиков
            this.vehicleIdCounter = 1;
            this.employeeIdCounter = 1;
            this.itemIdCounter = 1;
            
            // Сбрасываем ключи форм
            this.vehicleFormKey += 1;
            this.employeeFormKey += 1;
            this.itemsFormKey += 1;
            
            // Сбрасываем общие данные формы
            this.message = '';
            this.consentGiven = false;
            
            // Сбрасываем ошибки
            this.errors = {
                organization: '',
                company: '',
                responsiblePerson: '',
                phone: ''
            };
            
            // Очистка вложений в BlankSelector
            if (this.$refs.blankSelector) {
                this.$refs.blankSelector.clearAttachments();
            }
            this.selectedAttachment = null;
            this.attachments = [];
            
            // Очищаем ВСЕ данные из localStorage
            this.clearLocalStorageAfterSubmit();
            
            alert('Заявка успешно отправлена! Данные очищены.');
        },

        // Новый метод для отправки полной заявки с вложениями
        async sendCompleteApplication() {
            if (this.attachments.length === 0) {
                alert('Добавьте вложения для отправки');
                return;
            }

            // Подготавливаем данные для отправки
            const applicationData = {
                message: this.message || null,
                organization: this.organization,
                company: this.company || null,
                responsible_person: this.responsiblePerson,
                contact_phone: this.phoneNumber.replace(/\D/g, ''),
                data_approval: this.consentGiven,
                attachments: []
            };

            // Для каждого вложения добавляем данные
            for (const attachment of this.attachments) {
                const dateData = this.attachmentDatesByAttachment[attachment.id] || this.getDefaultDateData();
                
                const attachmentData = {
                    attachment_type: attachment.attachment_type,
                    attachment_name: attachment.name,
                    attachment_display_name: attachment.display_name,
                    entry_date_from: this.formatDateForAPI(dateData.isOneDay ? dateData.singleDate : dateData.startDate),
                    entry_date_to: this.formatDateForAPI(dateData.isOneDay ? dateData.singleDate : dateData.endDate),
                    entry_time_from: dateData.startTime + ":00",
                    entry_time_to: dateData.endTime + ":00",
                    data: {}
                };

                // Добавляем данные в зависимости от типа вложения
                switch (attachment.attachment_type) {
                    case 'cars': {
                        const vehicles = this.vehiclesByAttachment[attachment.id] || [];
                        attachmentData.data.vehicles = vehicles.map(vehicle => ({
                            car_number: vehicle.plateNumber,
                            car_brand: vehicle.mark,
                            unload_place: vehicle.unloadingPlace,
                            unload_places: vehicle.unloadPlaces || []
                        }));
                        break;
                    }
                    case 'people': {
                        const employees = this.employeesByAttachment[attachment.id] || [];
                        attachmentData.data.employees = employees.map(employee => ({
                            last_name: employee.lastName,
                            first_name: employee.firstName,
                            middle_name: employee.middleName,
                            citizenship_id: employee.citizenshipId,
                            position: employee.position,
                            passport_series_number: employee.passportSeriesNumber,
                            patent_number: employee.patentNumber,
                            other_permission: employee.otherPermission,
                            target_tables: employee.targetTables || []
                        }));
                        break;
                    }
                    case 'items': {
                        const items = this.itemsByAttachment[attachment.id] || [];
                        attachmentData.data.items = items.map((item, index) => ({
                            name: item.itemName,
                            count: item.quantity,
                            order_index: index + 1
                        }));
                        break;
                    }
                }

                applicationData.attachments.push(attachmentData);
            }

            // Отправляем заявку
            try {
                const token = localStorage.getItem("token");
                if (!token) {
                    alert('Токен не найден');
                    return;
                }

                const response = await fetch("http://localhost:8080/applications/submit-complete-application", {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        "Authorization": `Bearer ${token}`
                    },
                    body: JSON.stringify(applicationData)
                });

                if (response.ok) {
                    const result = await response.json();
                    alert(`Заявка успешно отправлена! Номер заявки: ${result.application_number}`);
                    // Очищаем форму после успешной отправки
                    this.clearAllAttachments();
                } else {
                    const errorText = await response.text();
                    console.error('Ошибка отправки заявки:', errorText);
                    alert('Ошибка отправки заявки: ' + errorText);
                }
            } catch (error) {
                console.error('Ошибка отправки заявки:', error);
                alert(`Произошла ошибка при отправке заявки: ${error.message}`);
            }
        },

        // Новый метод для очистки localStorage после отправки
        clearLocalStorageAfterSubmit() {
            // Очищаем ВСЕ данные из localStorage
            localStorage.removeItem('draftApplicationState');
            localStorage.removeItem('draftApplication');
            
            // Также очищаем любые другие данные, связанные с заявкой
            const keysToRemove = [];
            for (let i = 0; i < localStorage.length; i++) {
                const key = localStorage.key(i);
                if (key.includes('attachment') || key.includes('draft') || key.includes('application')) {
                    keysToRemove.push(key);
                }
            }
            
            keysToRemove.forEach(key => {
                localStorage.removeItem(key);
            });
            
            console.log('Все данные заявки очищены из localStorage');
        },
        
        formatDateForAPI(dateStr) {
            if (!dateStr) return null;
            const [day, month, year] = dateStr.split('.');
            return `${year}-${month}-${day}`;
        },

        resetForm() {
            // Используем метод очистки данных формы
            this.clearFormData();
            
            // Данные организации/компании/ФИО/телефона НЕ очищаем - они загружаются из данных пользователя
            
            this.applicationNumber++;

            // Сброс данных всех вложений
            this.vehiclesByAttachment = {};
            this.employeesByAttachment = {};
            this.itemsByAttachment = {};
            this.attachmentDatesByAttachment = {};
            
            // Сброс ID счетчиков
            this.vehicleIdCounter = 1;
            this.employeeIdCounter = 1;
            this.itemIdCounter = 1;
            
            // Сброс ключей форм
            this.vehicleFormKey += 1;
            this.employeeFormKey += 1;
            this.itemsFormKey += 1;

            // Сброс ID
            this.organizationId = null;
            this.companyId = null;
            
            // Загрузка данных пользователя заново (восстанавливает организацию, компанию, ФИО, телефон)
            this.loadUserData();
            
            // Очистка вложений и localStorage
            if (this.$refs.blankSelector) {
                this.$refs.blankSelector.clearAttachments();
            }
            this.selectedAttachment = null;
            
            // Очищаем localStorage
            this.clearLocalStorageAfterSubmit();
        },

        // Сохранение состояния в localStorage
        saveToLocalStorage() {
            try {
                // Проверяем, есть ли вложения
                const hasAttachments = Object.keys(this.vehiclesByAttachment).length > 0 || 
                                      Object.keys(this.employeesByAttachment).length > 0 || 
                                      Object.keys(this.itemsByAttachment).length > 0;
                
                const savedData = {
                    // Общие данные формы - сохраняем ВСЕГДА
                    message: this.message,
                    organization: this.organization,
                    company: this.company,
                    responsiblePerson: this.responsiblePerson,
                    phoneNumber: this.phoneNumber,
                    rawPhoneNumber: this.rawPhoneNumber,
                    consentGiven: this.consentGiven,
                    
                    // Данные вложений
                    vehiclesByAttachment: hasAttachments ? this.vehiclesByAttachment : {},
                    employeesByAttachment: hasAttachments ? this.employeesByAttachment : {},
                    itemsByAttachment: hasAttachments ? this.itemsByAttachment : {},
                    
                    // Данные дат для каждого вложения
                    attachmentDatesByAttachment: hasAttachments ? this.attachmentDatesByAttachment : {},
                    
                    // Счетчики ID
                    vehicleIdCounter: this.vehicleIdCounter,
                    employeeIdCounter: this.employeeIdCounter,
                    itemIdCounter: this.itemIdCounter,
                    
                    // Время сохранения
                    savedAt: new Date().toISOString()
                };
                
                localStorage.setItem('draftApplicationState', JSON.stringify(savedData));
            } catch (error) {
                console.error('Ошибка сохранения состояния в localStorage:', error);
            }
        },

        // Восстановление состояния из localStorage
        restoreFromLocalStorage() {
            try {
                const savedData = localStorage.getItem('draftApplicationState');
                if (savedData) {
                    const parsedData = JSON.parse(savedData);
                    
                    // Восстанавливаем ВСЕ данные формы
                    this.message = parsedData.message || '';
                    this.organization = parsedData.organization || '';
                    this.company = parsedData.company || '';
                    this.responsiblePerson = parsedData.responsiblePerson || '';
                    this.phoneNumber = parsedData.phoneNumber || '';
                    this.rawPhoneNumber = parsedData.rawPhoneNumber || '';
                    this.consentGiven = parsedData.consentGiven || false;
                    
                    // Восстанавливаем данные вложений
                    this.vehiclesByAttachment = parsedData.vehiclesByAttachment || {};
                    this.employeesByAttachment = parsedData.employeesByAttachment || {};
                    this.itemsByAttachment = parsedData.itemsByAttachment || {};
                    
                    // Восстанавливаем данные дат для каждого вложения
                    this.attachmentDatesByAttachment = parsedData.attachmentDatesByAttachment || {};
                    
                    // Восстанавливаем счетчики ID
                    this.vehicleIdCounter = parsedData.vehicleIdCounter || 1;
                    this.employeeIdCounter = parsedData.employeeIdCounter || 1;
                    this.itemIdCounter = parsedData.itemIdCounter || 1;
                    
                    console.log('Данные восстановлены из localStorage');
                    
                    // Проверяем, есть ли вложения
                    const hasAttachments = Object.keys(this.vehiclesByAttachment).length > 0 || 
                                          Object.keys(this.employeesByAttachment).length > 0 || 
                                          Object.keys(this.itemsByAttachment).length > 0;
                    
                    if (!hasAttachments) {
                        // Если нет вложений, очищаем данные формы
                        this.message = '';
                        this.consentGiven = false;
                    }
                }
            } catch (error) {
                console.error('Ошибка восстановления состояния из localStorage:', error);
            }
        },

        // Загрузка существующей заявки для редактирования
        async loadApplication(applicationId) {
            try {
                const token = localStorage.getItem("token");
                const response = await fetch(`http://localhost:8080/application/${applicationId}`, {
                    headers: {
                        "Authorization": `Bearer ${token}`
                    }
                });

                if (response.ok) {
                    const data = await response.json();
                    this.currentApplicationData = data;
                    
                    // Заполняем данные формы
                    this.message = data.message || '';
                    this.organization = data.organization || '';
                    this.company = data.company || '';
                    this.responsiblePerson = data.responsible_person || '';
                    this.phoneNumber = data.contact_phone || '';
                    
                    // Для каждого вложения загружаем свои даты
                    if (data.attachments && data.attachments.length > 0) {
                        this.attachments = data.attachments;
                        data.attachments.forEach(attachment => {
                            this.attachmentDatesByAttachment[attachment.id] = {
                                isOneDay: data.entry_date_from === data.entry_date_to,
                                startDate: data.entry_date_from ? this.formatDateFromAPI(data.entry_date_from) : '',
                                endDate: data.entry_date_to ? this.formatDateFromAPI(data.entry_date_to) : '',
                                singleDate: data.entry_date_from === data.entry_date_to ? 
                                    (data.entry_date_from ? this.formatDateFromAPI(data.entry_date_from) : '') : '',
                                startTime: data.entry_time_from ? data.entry_time_from.substring(0, 5) : '',
                                endTime: data.entry_time_to ? data.entry_time_to.substring(0, 5) : '',
                                roofAccess: false,
                                freeParking: false,
                                notifySituationCenter: false,
                                errors: {}
                            };
                        });
                    }
                    
                    // Загружаем данные в зависимости от типа вложения
                    if (data.attachment_type === 'cars' && data.vehicles) {
                        this.vehicles = data.vehicles.map((vehicle, index) => ({
                            ...vehicle,
                            id: index + 1,
                            isExisting: true
                        }));
                    } else if (data.attachment_type === 'people' && data.employees) {
                        this.employees = data.employees.map((employee, index) => ({
                            ...employee,
                            id: index + 1,
                            isExisting: true
                        }));
                    } else if (data.attachment_type === 'items' && data.items) {
                        this.items = data.items.map((item, index) => ({
                            ...item,
                            id: index + 1
                        }));
                    }
                }
            } catch (error) {
                console.error('Ошибка загрузки заявки:', error);
            }
        },

        formatDateFromAPI(dateStr) {
            if (!dateStr) return '';
            const [year, month, day] = dateStr.split('-');
            return `${day}.${month}.${year}`;
        }
    },
    mounted() {
        // Восстанавливаем состояние из localStorage
        this.restoreFromLocalStorage();
        
        // Загружаем данные пользователя
        this.loadUserData();
        
        // Сохраняем состояние при закрытии/обновлении страницы
        window.addEventListener('beforeunload', () => {
            this.saveCurrentAttachmentData();
            this.saveToLocalStorage();
        });
    },
    beforeUnmount() {
        // Удаляем обработчик при уничтожении компонента
        window.removeEventListener('beforeunload', this.saveToLocalStorage);
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

    .form-placeholder {
        width: 100%;
        height: fit-content;
        min-height: 490px;
        background-color: #FFF;
        border: 1px solid #e6e6e6;
        border-radius: 30px;
        box-shadow: 0 3px 10px rgba(0,0,0,0.05);
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .placeholder-content {
        text-align: center;
        color: #a2a2a2;
    }

    .placeholder-content p {
        font-size: 16px;
        margin: 0;
        padding: 20px;
    }
</style>