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
                            :all-unloading-places="allUnloadingPlaces"
                            :license-plate-formats="licensePlateFormats"
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
                            :all-tables="allPassageTables"
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
import { apiRequest } from '@/api/client'
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
            message: '',
            organization: '',
            company: '',
            responsiblePerson: '',
            phoneNumber: '',
            rawPhoneNumber: '',
            consentGiven: false,
            applicationNumber: 1,

            allUnloadingPlaces: [],

            organizationId: null,
            companyId: null,
            
            selectedAttachment: null,
            attachments: [],

            licensePlateFormats: [],

            vehiclesByAttachment: {},
            employeesByAttachment: {},
            itemsByAttachment: {},
            
            attachmentDatesByAttachment: {},
            
            vehicleIdCounter: 1,
            employeeIdCounter: 1,
            itemIdCounter: 1,
            
            sortField: null,
            sortDirection: 'asc',
            
            errors: {
                organization: '',
                company: '',
                responsiblePerson: '',
                phone: ''
            },
            
            vehicleFormKey: 0,
            employeeFormKey: 0,
            itemsFormKey: 0,

            showBindingModal: false,
            newVehiclesToBind: [],
            newEmployeesToBind: [],
            hasOrganization: false,
            hasCompany: false,
            
            currentApplicationData: {},
            
            allPassageTables: []
        }
    },
    computed: {
        currentFormTitle() {
            if (this.selectedAttachment) {
                return this.selectedAttachment.display_name;
            }
            return 'Добавьте новое вложение';
        },
        
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
        
        currentAttachmentData() {
            if (!this.selectedAttachment) {
                return this.getDefaultDateData();
            }
            const data = this.attachmentDatesByAttachment[this.selectedAttachment.id];
            return data || this.getDefaultDateData();
        },
        
        currentAttachmentErrors() {
            if (!this.selectedAttachment) return {};
            const data = this.attachmentDatesByAttachment[this.selectedAttachment.id];
            return data?.errors || {};
        },
        
        canSubmit() {
            if (this.attachments.length === 0) {
                return false;
            }

            const hasRequiredFields = 
                this.organization && 
                this.company && 
                this.responsiblePerson && 
                this.phoneNumber &&
                this.consentGiven;

            if (!hasRequiredFields) {
                return false;
            }

            let allAttachmentsValid = true;
            
            this.attachments.forEach(attachment => {
                let hasAttachmentData = false;
                let hasValidDates = false;
                let hasValidTime = false;

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
        
        sortedVehicles() {
            if (!this.sortField || !this.vehicles.length) {
                return this.vehicles;
            }
            
            return [...this.vehicles].sort((a, b) => {
                let valueA, valueB;
                
                switch (this.sortField) {
                    case 'number':
                        return this.sortDirection === 'asc' ? a.id - b.id : b.id - a.id;
                    case 'plate':
                        valueA = a.plateNumber ? a.plateNumber.toLowerCase() : '';
                        valueB = b.plateNumber ? b.plateNumber.toLowerCase() : '';
                        break;
                    case 'mark':
                        valueA = a.mark ? a.mark.toLowerCase() : '';
                        valueB = b.mark ? b.mark.toLowerCase() : '';
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
                        valueA = a.position ? a.position.toLowerCase() : '';
                        valueB = b.position ? b.position.toLowerCase() : '';
                        break;
                    case 'tables':
                        valueA = a.passageTables ? a.passageTables.toLowerCase() : '';
                        valueB = b.passageTables ? b.passageTables.toLowerCase() : '';
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
                        valueA = a.itemName ? a.itemName.toLowerCase() : '';
                        valueB = b.itemName ? b.itemName.toLowerCase() : '';
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
        
        async loadPassageTables() {
            try {
                const token = localStorage.getItem("token");
                const response = await apiRequest("/system-tables", {
                    method: "GET"});

                if (response.ok) {
                    const tables = await response.json();
                    console.log('Загруженные таблицы в CreateApplication:', tables);
                    this.allPassageTables = tables;
                } else {
                    console.error("Ошибка при загрузке системных таблиц");
                }
            } catch (error) {
                console.error("Ошибка при загрузке таблиц:", error);
            }
        },
        async checkVehiclesBeforeSubmit() {
  const activeVehicles = [];
  const token = localStorage.getItem("token");
  
  for (const attachment of this.attachments) {
    if (attachment.attachment_type !== 'cars') continue;
    
    const vehicles = this.vehiclesByAttachment[attachment.id] || [];
    
    for (const vehicle of vehicles) {
      try {
        const url = new URL('/cars/check-active', window.location.origin);
        url.searchParams.append('car_number', vehicle.plateNumber);
        url.searchParams.append('car_brand', vehicle.mark);
        
        if (this.organizationId) {
          url.searchParams.append('organization_id', this.organizationId);
        }
        
        if (this.companyId) {
          url.searchParams.append('company_id', this.companyId);
        }

        const response = await apiRequest(url, {});

        if (response.ok) {
          const data = await response.json();
          if (data.active) {
            activeVehicles.push({
              ...vehicle,
              activeInfo: data
            });
          }
        }
      } catch (error) {
        console.error('Ошибка при проверке авто:', error);
      }
    }
  }
  
  return activeVehicles;
},
        async loadLicensePlateFormats() {
            try {
                const token = localStorage.getItem("token");
                const response = await apiRequest("/license-plate-formats", {
                    method: "GET"});

                if (response.ok) {
                    this.licensePlateFormats = await response.json();
                } else {
                    console.error("Ошибка при загрузке форматов номеров");
                }
            } catch (error) {
                console.error("Ошибка при загрузке форматов номеров:", error);
            }
        },

        async loadAllUnloadingPlaces() {
            try {
                const token = localStorage.getItem("token");
                const response = await apiRequest("/unload-places", {
                    method: "GET"});

                if (response.ok) {
                    this.allUnloadingPlaces = await response.json();
                }
            } catch (error) {
                console.error("Ошибка при загрузке мест разгрузки:", error);
            }
        },
        
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
        
        updateAttachmentData(field, value) {
            if (!this.selectedAttachment) return;
            
            const attachmentId = this.selectedAttachment.id;
            
            if (!this.attachmentDatesByAttachment[attachmentId]) {
                this.attachmentDatesByAttachment[attachmentId] = this.getDefaultDateData();
            }
            
            this.attachmentDatesByAttachment[attachmentId][field] = value;
            
            this.saveToLocalStorage();
        },
        
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
        
        clearFormData() {
            if (this.attachments.length === 0 && this.selectedAttachment === null) {
                this.message = '';
                this.consentGiven = false;
                
                this.errors = {
                    organization: '',
                    company: '',
                    responsiblePerson: '',
                    phone: ''
                };
            }
        },
        
        async loadUserData() {
            const token = localStorage.getItem("token");
            if (!token) {
                console.error("Токен не найден");
                return;
            }

            try {
                const response = await apiRequest("/user-data", {
                    method: "GET"});

                if (response.ok) {
                    const userData = await response.json();
                    this.organization = userData.organization || '';
                    this.company = userData.company || '';
                    
                    this.organizationId = userData.organization_id || null;
                    this.companyId = userData.company_id || null;
                    
                    this.hasOrganization = !!this.organizationId;
                    this.hasCompany = !!this.companyId;
                    
                    const lastName = userData.last_name || '';
                    const firstName = userData.first_name || '';
                    const middleName = userData.middle_name || '';
                    this.responsiblePerson = `${lastName} ${firstName} ${middleName}`.trim();
                    
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
                this.clearFormData();
                return;
            }
            
            this.selectedAttachment = attachment;
            this.restoreAttachmentData(attachment);
        },

        handleAttachmentAdded(attachment) {
            this.attachments.push(attachment);
            
            if (attachment.attachment_type === 'cars') {
                this.vehiclesByAttachment[attachment.id] = [];
            } else if (attachment.attachment_type === 'people') {
                this.employeesByAttachment[attachment.id] = [];
            } else if (attachment.attachment_type === 'items') {
                this.itemsByAttachment[attachment.id] = [];
            }
            
            this.attachmentDatesByAttachment[attachment.id] = this.getDefaultDateData();
            
            this.selectAttachment(attachment);
            
            this.saveToLocalStorage();
        },

        selectAttachment(attachment) {
            this.selectedAttachment = attachment;
        },

        handleAttachmentRemoved(attachment) {
            this.attachments = this.attachments.filter(a => a.id !== attachment.id);
            
            if (attachment.attachment_type === 'cars') {
                delete this.vehiclesByAttachment[attachment.id];
            } else if (attachment.attachment_type === 'people') {
                delete this.employeesByAttachment[attachment.id];
            } else if (attachment.attachment_type === 'items') {
                delete this.itemsByAttachment[attachment.id];
            }
            
            delete this.attachmentDatesByAttachment[attachment.id];
            
            if (this.selectedAttachment && this.selectedAttachment.id === attachment.id) {
                this.selectedAttachment = null;
            }
            
            this.clearFormData();
            
            this.saveToLocalStorage();
        },

        restoreAttachmentData(attachment) {
            if (this.selectedAttachment) {
                this.saveCurrentAttachmentData();
            }
            
            this.loadAttachmentData(attachment);
        },

        saveCurrentAttachmentData() {
            if (!this.selectedAttachment) return;
            
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
            
            this.saveToLocalStorage();
        },

        loadAttachmentData(attachment) {
            if (!attachment) return;
            
            switch (attachment.attachment_type) {
                case 'cars':
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
            
            if (!this.attachmentDatesByAttachment[attachment.id]) {
                this.attachmentDatesByAttachment[attachment.id] = this.getDefaultDateData();
            }
        },

        handleVehicleAdded(newVehicle) {
            const vehicleWithId = {
                ...newVehicle,
                id: this.vehicleIdCounter++,
                isExisting: false,
                organization: this.organization,
                organizationId: this.organizationId,
                company: this.company,
                companyId: this.companyId
            };
            
            if (this.selectedAttachment) {
                if (!this.vehiclesByAttachment[this.selectedAttachment.id]) {
                    this.vehiclesByAttachment[this.selectedAttachment.id] = [];
                }
                this.vehiclesByAttachment[this.selectedAttachment.id].push(vehicleWithId);
                
                this.saveToLocalStorage();
            }
        },

        handleVehiclesAdded(vehicles) {
            vehicles.forEach(vehicle => {
                const vehicleWithId = {
                    ...vehicle,
                    id: this.vehicleIdCounter++,
                    isExisting: false,
                    organization: this.organization,
                    organizationId: this.organizationId,
                    company: this.company,
                    companyId: this.companyId
                };
                
                if (this.selectedAttachment) {
                    if (!this.vehiclesByAttachment[this.selectedAttachment.id]) {
                        this.vehiclesByAttachment[this.selectedAttachment.id] = [];
                    }
                    this.vehiclesByAttachment[this.selectedAttachment.id].push(vehicleWithId);
                }
            });
            
            this.saveToLocalStorage();
        },

        handleVehicleUpdated(updatedVehicle) {
            if (!this.selectedAttachment) return;
            
            const vehicles = this.vehiclesByAttachment[this.selectedAttachment.id];
            if (!vehicles) return;
            
            const index = vehicles.findIndex(v => v.id === updatedVehicle.id);
            if (index !== -1) {
                vehicles.splice(index, 1, updatedVehicle);
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
                this.saveToLocalStorage();
            }
        },

        editVehicle(vehicle) {
            if (this.$refs.vehicleForm) {
                this.$refs.vehicleForm.editVehicle(vehicle);
            }
        },

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
            
            this.saveToLocalStorage();
        },

        handleEmployeeUpdated(updatedEmployee) {
            if (!this.selectedAttachment) return;
            
            const employees = this.employeesByAttachment[this.selectedAttachment.id];
            if (!employees) return;
            
            const index = employees.findIndex(e => e.id === updatedEmployee.id);
            if (index !== -1) {
                employees.splice(index, 1, updatedEmployee);
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
            
            this.saveToLocalStorage();
        },

        handleItemUpdated(updatedItem) {
            if (!this.selectedAttachment) return;
            
            const items = this.itemsByAttachment[this.selectedAttachment.id];
            if (!items) return;
            
            const index = items.findIndex(e => e.id === updatedItem.id);
            if (index !== -1) {
                items.splice(index, 1, updatedItem);
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
                this.saveToLocalStorage();
            }
        },

        editItem(item) {
            if (this.$refs.itemsForm) {
                this.$refs.itemsForm.editItem(item);
            }
        },

        sortBy(field) {
            if (this.sortField === field) {
                this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
            } else {
                this.sortField = field;
                this.sortDirection = 'asc';
            }
        },

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

        async submitApplication() {
            this.validateAllFields();
            
            if (!this.canSubmit) {
                alert('Заполните все обязательные поля во всех вложениях');
                return;
            }

            // Проверяем активные машины
  const activeVehicles = await this.checkVehiclesBeforeSubmit();
  
  if (activeVehicles.length > 0) {
    const vehicleList = activeVehicles.map(v => `${v.plateNumber} ${v.mark}`).join('\n');
    alert(`Невозможно отправить заявку. Следующие автомобили уже имеют активные заявки:\n${vehicleList}`);
    return;
  }

            let hasDateErrors = false;
            let errorMessage = '';
            
            this.attachments.forEach(attachment => {
                const dateData = this.attachmentDatesByAttachment[attachment.id];
                if (dateData) {
                    if (!dateData.isOneDay && dateData.startDate && dateData.endDate) {
                        const start = new Date(dateData.startDate.split('.').reverse().join('-'));
                        const end = new Date(dateData.endDate.split('.').reverse().join('-'));
                        if (start > end) {
                            hasDateErrors = true;
                            errorMessage = `В вложении "${attachment.display_name}" дата окончания не может быть раньше даты начала`;
                        }
                    }

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

            await this.collectNewDataForBinding();

            const vehiclesForBinding = this.newVehiclesToBind.filter(vehicle => 
                vehicle.plateNumber !== 'По факту' && vehicle.mark !== 'По факту'
            );
            
            const employeesForBinding = this.newEmployeesToBind.filter(employee => 
                employee.passportSeriesNumber !== 'По факту' && employee.position !== 'По факту'
            );

            if (vehiclesForBinding.length > 0 || employeesForBinding.length > 0) {
                this.showBindingModal = true;
            } else {
                await this.sendCompleteApplication();
            }
        },

        async collectNewDataForBinding() {
            this.newVehiclesToBind = [];
            this.newEmployeesToBind = [];
            
            const existingVehicles = await this.loadExistingVehicles();
            const existingEmployees = await this.loadExistingEmployees();
            
            Object.keys(this.vehiclesByAttachment).forEach(attachmentId => {
                const vehicles = this.vehiclesByAttachment[attachmentId] || [];
                vehicles.forEach(vehicle => {
                    if (!vehicle.isExisting) {
                        const isByFact = vehicle.plateNumber === 'По факту' || vehicle.mark === 'По факту';
                        
                        if (!isByFact) {
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
            
            Object.keys(this.employeesByAttachment).forEach(attachmentId => {
                const employees = this.employeesByAttachment[attachmentId] || [];
                employees.forEach(employee => {
                    if (!employee.isExisting) {
                        const isByFact = employee.passportSeriesNumber === 'По факту' || 
                                       employee.position === 'По факту';
                        
                        if (!isByFact) {
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

        async loadExistingVehicles() {
            try {
                const token = localStorage.getItem("token");
                const response = await apiRequest("/unique-cars?filter_type=all", {
                    method: "GET"});

                if (response.ok) {
                    return await response.json();
                }
                return [];
            } catch (error) {
                console.error("Ошибка при загрузке существующих автомобилей:", error);
                return [];
            }
        },

        async loadExistingEmployees() {
            try {
                const token = localStorage.getItem("token");
                const response = await apiRequest("/unique-employees?filter_type=all", {
                    method: "GET"});

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
                
                if (this.newVehiclesToBind.length > 0 && bindingData.vehicles.hasVehiclesForBinding) {
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

                            return apiRequest("/unique-cars", {
                                method: "POST",
                                body: JSON.stringify(vehicleData)
                            });
                        });

                        await Promise.all(vehiclePromises);
                    }
                }

                if (this.newEmployeesToBind.length > 0 && bindingData.employees.hasEmployeesForBinding) {
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

                            return apiRequest("/unique-employees", {
                                method: "POST",
                                body: JSON.stringify(employeeData)
                            });
                        });

                        await Promise.all(employeePromises);
                    }
                }

                this.closeBindingModal();
                await this.sendCompleteApplication();

            } catch (error) {
                console.error('Ошибка при привязке:', error);
                this.closeBindingModal();
            }
        },

        skipBinding() {
            this.closeBindingModal();
            this.sendCompleteApplication();
        },

        closeBindingModal() {
            this.showBindingModal = false;
            this.newVehiclesToBind = [];
            this.newEmployeesToBind = [];
        },

        clearAllAttachments() {
            this.vehiclesByAttachment = {};
            this.employeesByAttachment = {};
            this.itemsByAttachment = {};
            this.attachmentDatesByAttachment = {};
            
            this.vehicleIdCounter = 1;
            this.employeeIdCounter = 1;
            this.itemIdCounter = 1;
            
            this.vehicleFormKey += 1;
            this.employeeFormKey += 1;
            this.itemsFormKey += 1;
            
            this.message = '';
            this.consentGiven = false;
            
            this.errors = {
                organization: '',
                company: '',
                responsiblePerson: '',
                phone: ''
            };
            
            if (this.$refs.blankSelector) {
                this.$refs.blankSelector.clearAttachments();
            }
            this.selectedAttachment = null;
            this.attachments = [];
            
            this.clearLocalStorageAfterSubmit();
            
            alert('Заявка успешно отправлена! Данные очищены.');
        },

        async sendCompleteApplication() {
            if (this.attachments.length === 0) {
                alert('Добавьте вложения для отправки');
                return;
            }

            const applicationData = {
                message: this.message || null,
                organization: this.organization,
                company: this.company || null,
                responsible_person: this.responsiblePerson,
                contact_phone: this.phoneNumber.replace(/\D/g, ''),
                data_approval: this.consentGiven,
                attachments: []
            };

            for (const attachment of this.attachments) {
                const dateData = this.attachmentDatesByAttachment[attachment.id] || this.getDefaultDateData();
                
                const attachmentData = {
                    attachment_type: attachment.attachment_type,
                    attachment_name: attachment.name,
                    attachment_display_name: attachment.display_name,
                    unique_attachment_id: attachment.id,
                    entry_date_from: this.formatDateForAPI(dateData.isOneDay ? dateData.singleDate : dateData.startDate),
                    entry_date_to: this.formatDateForAPI(dateData.isOneDay ? dateData.singleDate : dateData.endDate),
                    entry_time_from: dateData.startTime + ":00",
                    entry_time_to: dateData.endTime + ":00",
                    data: {}
                };

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

            try {
                const token = localStorage.getItem("token");
                if (!token) {
                    alert('Токен не найден');
                    return;
                }

                const requiredUsers = [];
                
                if (this.organizationId) {
                    try {
                        const orgResponse = await apiRequest(`/organizations/${this.organizationId}/users`, {});
                        
                        if (orgResponse.ok) {
                            const orgUsers = await orgResponse.json();
                            orgUsers.forEach(user => {
                                if (user.required_approval) {
                                    requiredUsers.push({
                                        user_id: user.id,
                                        required_approval: true
                                    });
                                }
                            });
                        }
                    } catch (error) {
                        console.error("Ошибка при загрузке ответственных из организации:", error);
                    }
                }
                
                if (this.companyId) {
                    try {
                        const companyResponse = await apiRequest(`/companies/${this.companyId}/users`, {});
                        
                        if (companyResponse.ok) {
                            const companyUsers = await companyResponse.json();
                            companyUsers.forEach(user => {
                                const alreadyAdded = requiredUsers.some(u => u.user_id === user.id);
                                if (!alreadyAdded && user.required_approval) {
                                    requiredUsers.push({
                                        user_id: user.id,
                                        required_approval: true
                                    });
                                }
                            });
                        }
                    } catch (error) {
                        console.error("Ошибка при загрузке ответственных из компании:", error);
                    }
                }

                const finalRequestData = {
                    ...applicationData,
                    required_users: requiredUsers
                };

                const response = await apiRequest("/applications/submit-complete-application", {
                    method: "POST",
                    body: JSON.stringify(finalRequestData)
                });

                if (response.ok) {
                    const result = await response.json();
                    alert(`Заявка успешно отправлена! Номер заявки: ${result.application_number}`);
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
        
        async loadRequiredResponsibles() {
            const requiredUsers = [];
            const token = localStorage.getItem("token");
            
            if (this.organizationId) {
                try {
                    const orgResponse = await apiRequest(`/organizations/${this.organizationId}/users`, {});
                    
                    if (orgResponse.ok) {
                        const orgUsers = await orgResponse.json();
                        orgUsers.forEach(user => {
                            if (user.required_approval) {
                                requiredUsers.push({
                                    user_id: user.id,
                                    required_approval: true
                                });
                            }
                        });
                    }
                } catch (error) {
                    console.error("Ошибка при загрузке ответственных из организации:", error);
                }
            }
            
            if (this.companyId) {
                try {
                    const companyResponse = await apiRequest(`/companies/${this.companyId}/users`, {});
                    
                    if (companyResponse.ok) {
                        const companyUsers = await companyResponse.json();
                        companyUsers.forEach(user => {
                            const alreadyAdded = requiredUsers.some(u => u.user_id === user.id);
                            if (!alreadyAdded && user.required_approval) {
                                requiredUsers.push({
                                    user_id: user.id,
                                    required_approval: true
                                });
                            }
                        });
                    }
                } catch (error) {
                    console.error("Ошибка при загрузке ответственных из компании:", error);
                }
            }
            
            return requiredUsers;
        },

        clearLocalStorageAfterSubmit() {
            localStorage.removeItem('draftApplicationState');
            localStorage.removeItem('draftApplication');
            
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
        },
        
        formatDateForAPI(dateStr) {
            if (!dateStr) return null;
            const [day, month, year] = dateStr.split('.');
            return `${year}-${month}-${day}`;
        },

        resetForm() {
            this.clearFormData();
            
            this.applicationNumber++;

            this.vehiclesByAttachment = {};
            this.employeesByAttachment = {};
            this.itemsByAttachment = {};
            this.attachmentDatesByAttachment = {};
            
            this.vehicleIdCounter = 1;
            this.employeeIdCounter = 1;
            this.itemIdCounter = 1;
            
            this.vehicleFormKey += 1;
            this.employeeFormKey += 1;
            this.itemsFormKey += 1;

            this.organizationId = null;
            this.companyId = null;
            
            this.loadUserData();
            
            if (this.$refs.blankSelector) {
                this.$refs.blankSelector.clearAttachments();
            }
            this.selectedAttachment = null;
            
            this.clearLocalStorageAfterSubmit();
        },

        saveToLocalStorage() {
            try {
                const hasAttachments = Object.keys(this.vehiclesByAttachment).length > 0 || 
                                      Object.keys(this.employeesByAttachment).length > 0 || 
                                      Object.keys(this.itemsByAttachment).length > 0;
                
                const savedData = {
                    message: this.message,
                    organization: this.organization,
                    company: this.company,
                    responsiblePerson: this.responsiblePerson,
                    phoneNumber: this.phoneNumber,
                    rawPhoneNumber: this.rawPhoneNumber,
                    consentGiven: this.consentGiven,
                    
                    vehiclesByAttachment: hasAttachments ? this.vehiclesByAttachment : {},
                    employeesByAttachment: hasAttachments ? this.employeesByAttachment : {},
                    itemsByAttachment: hasAttachments ? this.itemsByAttachment : {},
                    
                    attachmentDatesByAttachment: hasAttachments ? this.attachmentDatesByAttachment : {},
                    
                    vehicleIdCounter: this.vehicleIdCounter,
                    employeeIdCounter: this.employeeIdCounter,
                    itemIdCounter: this.itemIdCounter,
                    
                    savedAt: new Date().toISOString()
                };
                
                localStorage.setItem('draftApplicationState', JSON.stringify(savedData));
            } catch (error) {
                console.error('Ошибка сохранения состояния в localStorage:', error);
            }
        },

        restoreFromLocalStorage() {
            try {
                const savedData = localStorage.getItem('draftApplicationState');
                if (savedData) {
                    const parsedData = JSON.parse(savedData);
                    
                    this.message = parsedData.message || '';
                    this.organization = parsedData.organization || '';
                    this.company = parsedData.company || '';
                    this.responsiblePerson = parsedData.responsiblePerson || '';
                    this.phoneNumber = parsedData.phoneNumber || '';
                    this.rawPhoneNumber = parsedData.rawPhoneNumber || '';
                    this.consentGiven = parsedData.consentGiven || false;
                    
                    this.vehiclesByAttachment = parsedData.vehiclesByAttachment || {};
                    this.employeesByAttachment = parsedData.employeesByAttachment || {};
                    this.itemsByAttachment = parsedData.itemsByAttachment || {};
                    
                    this.attachmentDatesByAttachment = parsedData.attachmentDatesByAttachment || {};
                    
                    this.vehicleIdCounter = parsedData.vehicleIdCounter || 1;
                    this.employeeIdCounter = parsedData.employeeIdCounter || 1;
                    this.itemIdCounter = parsedData.itemIdCounter || 1;
                    
                    const hasAttachments = Object.keys(this.vehiclesByAttachment).length > 0 || 
                                          Object.keys(this.employeesByAttachment).length > 0 || 
                                          Object.keys(this.itemsByAttachment).length > 0;
                    
                    if (!hasAttachments) {
                        this.message = '';
                        this.consentGiven = false;
                    }
                }
            } catch (error) {
                console.error('Ошибка восстановления состояния из localStorage:', error);
            }
        },

        async loadApplication(applicationId) {
            try {
                const token = localStorage.getItem("token");
                const response = await apiRequest(`/application/${applicationId}`, {});

                if (response.ok) {
                    const data = await response.json();
                    this.currentApplicationData = data;
                    
                    this.message = data.message || '';
                    this.organization = data.organization || '';
                    this.company = data.company || '';
                    this.responsiblePerson = data.responsible_person || '';
                    this.phoneNumber = data.contact_phone || '';
                    
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
        this.restoreFromLocalStorage();
        this.loadUserData();
        this.loadAllUnloadingPlaces();
        this.loadLicensePlateFormats();
        this.loadPassageTables();
        
        window.addEventListener('beforeunload', () => {
            this.saveCurrentAttachmentData();
            this.saveToLocalStorage();
        });
    },
    beforeUnmount() {
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