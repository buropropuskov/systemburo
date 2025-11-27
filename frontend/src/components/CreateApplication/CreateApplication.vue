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
            <h4>Заявка на проведение работ №{{ applicationNumber }}</h4>
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

                <!-- 4 ряд: EmployeeForm и Список сотрудников -->
                <div class="form__data">
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
                        @edit-cancelled="handleEditCancelled"
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
                </div>
            </div>
        </div>

        <!-- Модальное окно привязки новых сотрудников -->
        <EmployeeBindingModal
            v-if="showBindingModal"
            :new-employees-to-bind="newEmployeesToBind"
            :organization="organization"
            :company="company"
            :has-organization="hasOrganization"
            :has-company="hasCompany"
            @toggle-employee-binding="toggleEmployeeBinding"
            @confirm-binding="confirmBinding"
            @skip-binding="skipBinding"
            @close="closeBindingModal"
        />
    </div>
</template>

<script>
// import VehicleForm from '@/components/VehicleForm.vue'
// import VehiclesList from './VehiclesList.vue';
import BlankSelector from '../BlankSelector.vue';
import UserInfoRow from './UserInfoRow.vue';
import DateRangeSection from './DateRangeSection.vue';
import EmployeeForm from './EmployeeForm.vue';
import EmployeesList from './EmployeesList.vue';
import EmployeeBindingModal from './EmployeeBindingModal.vue';

export default {
    name: 'CreateApplication',
    components: {
        // VehicleForm,
        // VehiclesList,
        BlankSelector,
        UserInfoRow,
        DateRangeSection,
        EmployeeForm,
        EmployeesList,
        EmployeeBindingModal
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
            
            // Employees list
            employees: [],
            employeeIdCounter: 1,
            
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
                passageTables: ''
            },
            
            // Key for forcing EmployeeForm re-render
            employeeFormKey: 0,

            // Привязка новых сотрудников
            showBindingModal: false,
            newEmployeesToBind: [],
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
                this.employees.length > 0 &&
                this.consentGiven;

            // Проверка дат в зависимости от типа заявки
            const hasValidDates = this.isOneDay 
                ? this.singleDate && this.startTime && this.endTime
                : this.startDate && this.endDate && this.startTime && this.endTime;

            // Проверка времени
            const hasValidTime = this.startTime && this.endTime && this.startTime < this.endTime;

            return hasRequiredFields && hasValidDates && hasValidTime;
        },
        sortedEmployees() {
            if (!this.sortField) {
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
                    
                    // Принудительно обновляем EmployeeForm после загрузки данных пользователя
                    this.employeeFormKey += 1;
                    
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
        
        deleteEmployee(employeeId) {
            const index = this.employees.findIndex(employee => employee.id === employeeId);
            if (index !== -1) {
                this.employees.splice(index, 1);
            }
        },

        editEmployee(employee) {
            this.$refs.employeeForm.editEmployee(employee);
        },

        handleEditCancelled() {
            // Обработка отмены редактирования
            this.employeeFormKey += 1;
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

        handleEmployeeAdded(newEmployee) {
            const employeeWithId = {
                ...newEmployee,
                id: this.employeeIdCounter++
            };
            this.employees.push(employeeWithId);
        },

        handleEmployeesAdded(employees) {
            employees.forEach(employee => {
                const employeeWithId = {
                    ...employee,
                    id: this.employeeIdCounter++
                };
                this.employees.push(employeeWithId);
            });
        },

        handleEmployeeUpdated(updatedEmployee) {
            const index = this.employees.findIndex(e => e.id === updatedEmployee.id);
            if (index !== -1) {
                this.employees.splice(index, 1, updatedEmployee);
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

        formatFullName(employee) {
            const parts = [];
            if (employee.lastName) parts.push(employee.lastName);
            if (employee.firstName) parts.push(employee.firstName);
            if (employee.middleName) parts.push(employee.middleName);
            return parts.join(' ') || 'Не указано';
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
                alert('Заполните все обязательные поля и добавьте хотя бы одного сотрудника');
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

            // Определяем новых сотрудников для добавления в unique_employees
            const newEmployees = this.employees.filter(employee => !employee.isExisting);
            
            // Сначала создаем уникальных сотрудников если есть новые
            if (newEmployees.length > 0) {
                await this.createUniqueEmployees(newEmployees);
            } else {
                // Если нет новых сотрудников для привязки, сразу отправляем заявку
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

        async createUniqueEmployees(newEmployees) {
            try {
                const token = localStorage.getItem("token");
                
                // Создаем массив промисов для создания сотрудников
                const createPromises = newEmployees.map(employee => {
                    const employeeData = {
                        last_name: employee.lastName,
                        first_name: employee.firstName,
                        middle_name: employee.middleName,
                        position: employee.position,
                        citizenship_id: employee.citizenshipId,
                        passport_series_number: employee.passportSeriesNumber,
                        patent_number: employee.patentNumber,
                        other_permission: employee.otherPermission,
                        user_id: null, // Будет установлен на сервере автоматически
                        organization_id: null,
                        company_id: null
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

                // Ждем завершения всех запросов
                const responses = await Promise.all(createPromises);
                
                // Проверяем результаты
                const results = await Promise.all(responses.map(async (response, index) => {
                    if (response.ok) {
                        const createdEmployee = await response.json();
                        return { 
                            success: true, 
                            employee: newEmployees[index], 
                            createdEmployee,
                            employeeId: createdEmployee.id // Сохраняем ID
                        };
                    } else {
                        const error = await response.json();
                        return { success: false, employee: newEmployees[index], error };
                    }
                }));

                // Разделяем успешные и неуспешные создания
                const successfulCreations = results.filter(result => result.success);
                const failedCreations = results.filter(result => !result.success);

                // Показываем ошибки если есть неуспешные создания
                if (failedCreations.length > 0) {
                    console.error('Ошибки при создании сотрудников:', failedCreations);
                    // Продолжаем с успешными созданиями
                }

                // Сохраняем успешно созданных сотрудников с их ID и флагом привязки
                this.newEmployeesToBind = successfulCreations.map(result => ({
                    lastName: result.employee.lastName,
                    firstName: result.employee.firstName,
                    middleName: result.employee.middleName,
                    position: result.employee.position,
                    citizenshipId: result.employee.citizenshipId,
                    passportSeriesNumber: result.employee.passportSeriesNumber,
                    patentNumber: result.employee.patentNumber,
                    otherPermission: result.employee.otherPermission,
                    employeeId: result.employeeId, // Сохраняем ID для обновления
                    bindToEntity: true // По умолчанию все сотрудники будут привязаны
                }));

                // Показываем модальное окно привязки если есть успешно созданные сотрудники
                if (this.newEmployeesToBind.length > 0) {
                    this.bindingInProgress = true;
                    this.showBindingModal = true;
                } else {
                    // Если нет сотрудников для привязки, отправляем заявку
                    await this.sendApplication();
                }

            } catch (error) {
                console.error('Ошибка при создании уникальных сотрудников:', error);
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
                employees: this.employees.map(employee => ({
                    employee: {
                        last_name: employee.lastName,
                        first_name: employee.firstName,
                        middle_name: employee.middleName,
                        position: employee.position,
                        citizenship_id: employee.citizenshipId,
                        passport_series_number: employee.passportSeriesNumber,
                        patent_number: employee.patentNumber,
                        other_permission: employee.otherPermission
                    },
                    target_tables: employee.targetTables ? employee.targetTables.map((tableId, index) => ({
                        table_id: tableId,
                        order_index: index + 1
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

                const response = await fetch("http://localhost:8080/submit-employee-application", {
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

        // Переключение привязки для конкретного сотрудника
        toggleEmployeeBinding(employee) {
            employee.bindToEntity = !employee.bindToEntity;
        },

        // Метод для привязки сотрудников
        async confirmBinding() {
            try {
                const token = localStorage.getItem("token");
                
                // Обновляем привязку для каждого сотрудника по ID
                const updatePromises = this.newEmployeesToBind.map(employee => {
                    const updateData = {
                        last_name: employee.lastName,
                        first_name: employee.firstName,
                        middle_name: employee.middleName,
                        position: employee.position,
                        citizenship_id: employee.citizenshipId,
                        passport_series_number: employee.passportSeriesNumber,
                        patent_number: employee.patentNumber,
                        other_permission: employee.otherPermission,
                        organization_id: employee.bindToEntity && this.bindToOrganization ? this.organizationId : null,
                        company_id: employee.bindToEntity && this.bindToCompany ? this.companyId : null,
                        user_id: null // Привязываем к пользователю (будет установлен на сервере автоматически)
                    };

                    // Используем обычный эндпоинт обновления по ID
                    return fetch(`http://localhost:8080/unique-employees/${employee.employeeId}`, {
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
                    console.error('Ошибки при обновлении сотрудников:', failedUpdates);
                }

                console.log(`Успешно обновлено ${successfulUpdates.length} из ${results.length} сотрудников`);

                this.closeBindingModal();
                await this.sendApplication();

            } catch (error) {
                console.error('Ошибка при привязке сотрудников:', error);
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
            this.newEmployeesToBind = [];
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
            this.employees = [];
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
                passageTables: ''
            };
            
            // Увеличиваем ключ для принудительного пересоздания EmployeeForm
            this.employeeFormKey += 1;
            
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