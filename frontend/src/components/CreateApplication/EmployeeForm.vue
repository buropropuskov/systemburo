<template>
    <div class="data__completion">
        <div class="completion__header">
            <h3>Новый сотрудник</h3>
            <button class="completion__button" @click="openExistingEmployeesModal">
                Добавить существующего(-их)
            </button>
        </div>

        <!-- Отображение количества выбранных существующих сотрудников -->
        <div v-if="selectedExistingEmployees.length > 0" class="existing-employees-info">
            <div class="existing-employees-header">
                <span class="existing-employees-count">Сотрудников добавлено: {{ selectedExistingEmployees.length }}</span>
                <div class="existing-employees-actions">
                    <button class="view-employees-btn" @click="openExistingEmployeesModal">Просмотреть</button>
                    <button class="add-existing-btn" @click="addExistingEmployees" :disabled="!canAddExistingEmployees">
                        Добавить
                    </button>
                </div>
            </div>
        </div>

        <!-- Форма для добавления нового сотрудника -->
        <div v-else>
            <div class="completion__citizenship">
                <div class="citizenship__header">
                    <label class="citizenship__label">Гражданство <span class="required">*</span></label>
                    <div class="citizenship-actions">
                        <button class="cancel-edit-btn" @click="cancelEdit" v-if="editingEmployee">
                            Отменить
                        </button>
                        <button 
                            class="add-button" 
                            @click="addEmployee"
                            :disabled="!canAddEmployee"
                            @mouseenter="showTooltip = true"
                            @mouseleave="showTooltip = false"
                        >
                            {{ editingEmployee ? 'Применить' : 'Добавить' }}
                        </button>
                        <!-- Подсказка для кнопки -->
                        <div v-if="showTooltip && !canAddEmployee" class="tooltip">
                            <div class="tooltip-content">
                                {{ getTooltipMessage }}
                            </div>
                        </div>
                    </div>
                </div>
                <div class="citizenship__dropdown">
                    <button 
                        class="dropdown__button" 
                        @click="toggleCitizenshipDropdown"
                        :disabled="editingEmployee && editingEmployee.isExisting"
                    >
                        <div class="button__content">
                            <span class="button__text">{{ selectedCitizenshipText }}</span>
                            <img src="@/assets/icons/arrow.png" class="button__arrow" :class="{ 'button__arrow--open': isCitizenshipDropdownOpen }" />
                        </div>
                    </button>
                    <transition name="dropdown">
                        <div v-if="isCitizenshipDropdownOpen" class="dropdown__menu">
                            <div 
                                v-for="citizenship in availableCitizenships" 
                                :key="citizenship.id"
                                class="dropdown__item" 
                                @click="selectCitizenship(citizenship)"
                            >
                                <span class="item__text">{{ citizenship.name }}</span>
                                <span v-if="citizenship.patent_required" class="patent-required-badge">Требуется патент</span>
                            </div>
                        </div>
                    </transition>
                </div>
            </div>
            
            <div class="completion__fields">
                <!-- Первая строка: Фамилия и Имя -->
                <div class="completion__name-row">
                    <div class="completion__last-name">
                        <div class="completion__last-name-header">
                            <label class="input__label">Фамилия <span class="required">*</span></label>
                        </div>
                        <input 
                            class="name__input" 
                            placeholder="Введите фамилию"
                            v-model="lastName"
                            @blur="formatNameField('lastName')"
                            :disabled="editingEmployee && editingEmployee.isExisting"
                        />
                    </div>
                    <div class="completion__first-name">
                        <div class="completion__first-name-header">
                            <label class="input__label">Имя <span class="required">*</span></label>
                        </div>
                        <input 
                            class="name__input" 
                            placeholder="Введите имя"
                            v-model="firstName"
                            @blur="formatNameField('firstName')"
                            :disabled="editingEmployee && editingEmployee.isExisting"
                        />
                    </div>
                </div>
                
                <!-- Вторая строка: Отчество и Должность -->
                <div class="completion__name-row">
                    <div class="completion__middle-name">
                        <div class="completion__middle-name-header">
                            <label class="input__label">Отчество</label>
                        </div>
                        <input 
                            class="name__input" 
                            placeholder="Введите отчество"
                            v-model="middleName"
                            @blur="formatNameField('middleName')"
                            :disabled="editingEmployee && editingEmployee.isExisting"
                        />
                    </div>
                    <div class="completion__position">
                        <div class="completion__position-header">
                            <label class="input__label">Должность <span class="required">*</span></label>
                        </div>
                        <input 
                            class="name__input" 
                            placeholder="Введите должность"
                            v-model="position"
                            @blur="formatNameField('position')"
                            :disabled="editingEmployee && editingEmployee.isExisting"
                        />
                    </div>
                </div>
                
                <!-- Третья строка: Паспорт и Номер патента -->
                <div class="completion__name-row">
                    <div class="completion__passport">
                        <div class="completion__passport-header">
                            <label class="input__label">Паспортные данные <span class="required">*</span></label>
                        </div>
                        <input 
                            class="name__input" 
                            placeholder="Введите паспортные данные"
                            v-model="passportSeriesNumber"
                            :disabled="editingEmployee && editingEmployee.isExisting"
                        />
                    </div>
                    <div class="completion__patent" :class="{ 'disabled-field': !isPatentRequired }">
                        <div class="completion__patent-header">
                            <label class="input__label">Номер патента</label>
                        </div>
                        <input 
                            class="name__input" 
                            :placeholder="isPatentRequired ? 'Номер патента' : 'Не требуется'"
                            v-model="patentNumber"
                            :disabled="!isPatentRequired || patentFieldDisabled || (editingEmployee && editingEmployee.isExisting)"
                            @input="handlePatentInput"
                        />
                    </div>
                </div>
                
                <!-- Четвертая строка: Иное разрешение -->
                <div class="completion__permission" :class="{ 'disabled-field': !isPatentRequired }">
                    <div class="completion__permission-header">
                        <label class="input__label">Иное разрешение на работы</label>
                    </div>
                    <div class="permission__dropdown">
                        <button class="permission__dropdown-button" 
                                @click="togglePermissionDropdown" 
                                :disabled="!isPatentRequired || permissionFieldDisabled || (editingEmployee && editingEmployee.isExisting)">
                            <div class="permission__button-content">
                                <span class="permission__button-text">{{ selectedPermission || (isPatentRequired ? 'Выберите разрешение' : 'Не требуется') }}</span>
                                <img src="@/assets/icons/arrow.png" class="permission__button-arrow" :class="{ 'permission__button-arrow--open': isPermissionDropdownOpen }" />
                            </div>
                        </button>
                        <transition name="dropdown">
                            <div v-if="isPermissionDropdownOpen" class="permission__dropdown-menu">
                                <div 
                                    v-for="permission in availablePermissions" 
                                    :key="permission"
                                    class="permission__dropdown-item"
                                    @click="selectPermission(permission)"
                                >
                                    <span class="permission__item-text">{{ permission }}</span>
                                </div>
                            </div>
                        </transition>
                    </div>
                </div>
                
                <!-- Загрузка файлов -->
                <div class="completion__files" v-if="isPatentRequired">
                    <div class="completion__files-header">
                        <label class="input__label">Фото, скан документа(-ов), подтверждающее иное разрешение на работы</label>
                    </div>
                    <div class="files__upload">
                        <input 
                            type="file" 
                            ref="fileInput"
                            @change="handleFileUpload"
                            multiple
                            accept="image/*,.pdf,.doc,.docx,.xlsx,.xls"
                            class="file-input"
                            :disabled="editingEmployee && editingEmployee.isExisting"
                        />
                        <button class="upload-button" @click="triggerFileInput" :disabled="editingEmployee && editingEmployee.isExisting">
                            Загрузить
                        </button>
                    </div>
                    <div v-if="uploadedFiles.length > 0" class="uploaded-files">
                        <div v-for="(file, index) in uploadedFiles" :key="index" class="uploaded-file">
                            <div class="file-preview">
                                <img v-if="file.type === 'image'" :src="file.preview" class="file-preview-image" />
                                <img v-else :src="getFileIcon(file.extension)" class="file-icon" />
                            </div>
                            <span class="file-name">{{ file.name }}</span>
                            <button @click="removeFile(index)" class="remove-file-btn" :disabled="editingEmployee && editingEmployee.isExisting">×</button>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Места прохода -->
        <div class="completion__passage">
            <label class="input__label">Места прохода (целевые таблицы) <span class="required">*</span></label>
            <div class="passage__grid" v-if="!loadingPassageTables && allPassageTables.length > 0">
                <div 
                    v-for="table in allPassageTables" 
                    :key="table.id"
                    class="passage__item"
                    :class="{ 
                        'passage__item--active': selectedPassageTables.includes(table.id),
                        'passage__item--attached': attachedTablesIds.includes(table.id)
                    }"
                    @click="togglePassageTable(table.id)"
                >
                    {{ table.display_name }}
                </div>
            </div>
            <div v-else-if="loadingPassageTables" class="loading-message">
                Загрузка мест прохода...
            </div>
            <div v-else class="no-tables-message">
                Нет доступных мест прохода
            </div>
            <div v-if="errors.passageTables" class="error-message">{{ errors.passageTables }}</div>
        </div>

        <!-- Модальное окно выбора существующих сотрудников -->
        <div v-if="showExistingEmployeesModal" class="modal-overlay" @click="closeExistingEmployeesModal">
            <div class="modal-content" @click.stop>
                <div class="modal-header">
                    <div class="modal-header__top">
                        <h3>Выбор существующих сотрудников</h3>
                        <div class="selected-count-badge" v-if="tempSelectedEmployees.length > 0">
                            Выбрано: {{ tempSelectedEmployees.length }}
                        </div>
                    </div>
                    <button class="modal-close" @click="closeExistingEmployeesModal">×</button>
                </div>
                <div class="modal-body">
                    <!-- Вкладки фильтров -->
                    <div class="filter-tabs">
                        <button 
                            v-for="tab in filterTabs" 
                            :key="tab.value"
                            class="filter-tab"
                            :class="{ 'filter-tab--active': currentFilter === tab.value }"
                            @click="switchFilter(tab.value)"
                        >
                            {{ tab.label }}
                        </button>
                    </div>

                    <!-- Список сотрудников -->
                    <div class="employees-list">
                        <div class="employees-header">
                            <div class="header-row">
                                <div class="header-col select-col">
                                    <!-- Убрана кнопка "Выбрать всё" -->
                                </div>
                                <div class="header-col number-col">№</div>
                                <div class="header-col name-col">ФИО</div>
                                <div class="header-col position-col">Должность</div>
                                <div class="header-col citizenship-col">Гражданство</div>
                                <div class="header-col passport-col">Паспорт</div>
                                <div class="header-col status-col">Статус</div>
                            </div>
                        </div>
                        <div class="employees-body">
                            <div 
                                v-for="employee in filteredEmployees" 
                                :key="employee.id"
                                class="employee-item"
                                :class="{ 
                                    'employee-item--disabled': isEmployeeDisabled(employee),
                                    'employee-item--selected': isEmployeeSelected(employee)
                                }"
                                @click="toggleEmployeeSelection(employee)"
                            >
                                <div class="employee-row">
                                    <div class="employee-col select-col">
                                        <input 
                                            type="checkbox" 
                                            :checked="isEmployeeSelected(employee)"
                                            @change="toggleEmployeeSelection(employee)"
                                            :disabled="isEmployeeDisabled(employee)"
                                            @click.stop
                                        />
                                    </div>
                                    <div class="employee-col number-col">{{ employee.id }}</div>
                                    <div class="employee-col name-col">{{ formatFullName(employee) }}</div>
                                    <div class="employee-col position-col">{{ employee.position || 'Не указана' }}</div>
                                    <div class="employee-col citizenship-col">{{ employee.citizenship_name || 'Не указано' }}</div>
                                    <div class="employee-col passport-col">{{ employee.passport_series_number || 'Не указан' }}</div>
                                    <div class="employee-col status-col">
                                        <span 
                                            class="status-badge"
                                            :class="{
                                                'status-active': employee.status,
                                                'status-inactive': !employee.status
                                            }"
                                        >
                                            {{ employee.status ? 'Активен' : 'Неактивен' }}
                                        </span>
                                    </div>
                                </div>
                            </div>
                            <div v-if="filteredEmployees.length === 0" class="no-employees-message">
                                Нет доступных сотрудников
                            </div>
                        </div>
                    </div>

                    <div class="modal-actions">
                        <button class="cancel-btn" @click="closeExistingEmployeesModal">Отмена</button>
                        <button class="select-btn" @click="confirmExistingEmployeesSelection">
                            {{ tempSelectedEmployees.length > 0 ? `Выбрать (${tempSelectedEmployees.length})` : 'Выбрать' }}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script>
export default {
    name: 'EmployeeForm',
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
        existingEmployees: {
            type: Array,
            default: () => []
        }
    },
    data() {
        return {
            // Основные поля
            lastName: '',
            firstName: '',
            middleName: '',
            position: '',
            passportSeriesNumber: '',
            patentNumber: '',
            
            // Гражданство
            availableCitizenships: [],
            selectedCitizenship: null,
            isCitizenshipDropdownOpen: false,
            
            // Разрешения
            selectedPermission: '',
            isPermissionDropdownOpen: false,
            availablePermissions: [
                'Иностранцы с видом на жительство (ВНЖ) или разрешением на временное проживание (РВП)',
                'Беженцы или получившие временное убежище в России',
                'Участники Госпрограммы переселения соотечественников в РФ и члены их семей',
                'Люди с временным удостоверением личности лица без гражданства, выданным в России',
                'Студенты, которые работают в образовательных организациях или хозяйственных обществах и партнёрствах, созданных этими организациями',
                'Студенты, обучающиеся очно в образовательных организациях',
                'Работники посольств и консульств',
                'Аккредитованные журналисты',
                'Специалисты аккредитованных ИТ‑компаний',
                'Специалисты иностранных компаний, которых пригласили для монтажных работ или гарантийно‑сервисного обслуживания оборудования',
                'Сотрудники представительств иностранных организаций',
                'Медики, педагоги, учёные, которые работают на территории международного медицинского кластера',
                'Педагоги и учёные, которых пригласили на работу в образовательные или научные организации',
                'Педагоги и учёные, прибывшие с деловой или гуманитарной целью в образовательные или научные организации, кроме духовных',
                'Творческие работники, учёные и педагоги, прибывшие с гостевым или деловым визитом — до 30 календарных дней',
                'Творческие работники, учёные и педагоги, прибывшие по приглашению госучреждений культуры и искусства для участия в мероприятиях — до 30 календарных дней'
            ],
            
            // Файлы
            uploadedFiles: [],
            
            // Места прохода
            allPassageTables: [],
            attachedPassageTables: [],
            selectedPassageTables: [],
            loadingPassageTables: false,
            
            // Валидация
            errors: {
                passageTables: ''
            },
            
            // Существующие сотрудники
            showExistingEmployeesModal: false,
            allEmployees: [],
            tempSelectedEmployees: [],
            selectedExistingEmployees: [],
            currentFilter: 'all',
            filterTabs: [
                { label: 'Все сотрудники', value: 'all' },
                { label: 'Сотрудники организации', value: 'organization' },
                { label: 'Сотрудники компании', value: 'company' },
                { label: 'Мои сотрудники', value: 'user' }
            ],

            // Редактирование
            editingEmployee: null,

            // Tooltip
            showTooltip: false
        }
    },
    computed: {
        selectedCitizenshipText() {
            return this.selectedCitizenship ? this.selectedCitizenship.name : 'Выберите гражданство';
        },
        isPatentRequired() {
            return this.selectedCitizenship ? this.selectedCitizenship.patent_required : false;
        },
        patentFieldDisabled() {
            return this.selectedPermission !== '';
        },
        permissionFieldDisabled() {
            return this.patentNumber.trim() !== '';
        },
        canAddEmployee() {
            // Если выбраны существующие сотрудники
            if (this.selectedExistingEmployees.length > 0) {
                return this.selectedPassageTables.length > 0;
            }

            // Если добавляется новый сотрудник
            if (!this.lastName.trim() || !this.firstName.trim()) {
                return false;
            }
            
            if (!this.selectedCitizenship) {
                return false;
            }
            
            if (!this.passportSeriesNumber.trim()) {
                return false;
            }
            
            if (!this.position.trim()) {
                return false;
            }
            
            // Если требуется патент, проверяем дополнительные поля
            if (this.isPatentRequired) {
                if (!this.patentNumber.trim() && !this.selectedPermission) {
                    return false;
                }
            }
            
            if (this.selectedPassageTables.length === 0) {
                return false;
            }
            
            return true;
        },
        attachedTablesIds() {
            return this.attachedPassageTables.map(table => table.id);
        },
        filteredEmployees() {
            if (this.currentFilter === 'all') {
                return this.allEmployees;
            } else if (this.currentFilter === 'organization') {
                return this.allEmployees.filter(employee => employee.organization_id !== null);
            } else if (this.currentFilter === 'company') {
                return this.allEmployees.filter(employee => employee.company_id !== null);
            } else if (this.currentFilter === 'user') {
                return this.allEmployees.filter(employee => employee.user_id !== null);
            }
            return this.allEmployees;
        },
        canAddExistingEmployees() {
            return this.selectedExistingEmployees.length > 0 && this.selectedPassageTables.length > 0;
        },
        getTooltipMessage() {
            const missingFields = [];
            
            if (this.selectedExistingEmployees.length === 0) {
                if (!this.lastName.trim()) missingFields.push('фамилию');
                if (!this.firstName.trim()) missingFields.push('имя');
                if (!this.position.trim()) missingFields.push('должность');
                if (!this.selectedCitizenship) missingFields.push('гражданство');
                if (!this.passportSeriesNumber.trim()) missingFields.push('паспортные данные');
                
                if (this.isPatentRequired) {
                    if (!this.patentNumber.trim() && !this.selectedPermission) missingFields.push('номер патента или иное разрешение');
                }
            }
            
            if (this.selectedPassageTables.length === 0) {
                missingFields.push('выберите хотя бы одно место прохода');
            }
            
            if (missingFields.length === 0) {
                return '';
            }
            
            return `Заполните: ${missingFields.join(', ')}`;
        }
    },
    methods: {
        async loadCitizenships() {
            try {
                const token = localStorage.getItem("token");
                const response = await fetch("http://localhost:8080/citizenships", {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`
                    }
                });

                if (response.ok) {
                    this.availableCitizenships = await response.json();
                    // Выбираем гражданство по умолчанию или первое гражданство
                    const defaultCitizenship = this.availableCitizenships.find(c => c.is_default);
                    this.selectedCitizenship = defaultCitizenship || this.availableCitizenships[0];
                } else {
                    console.error("Ошибка при загрузке гражданств");
                }
            } catch (error) {
                console.error("Ошибка при загрузке гражданств:", error);
            }
        },

        async loadPassageTables() {
            this.loadingPassageTables = true;
            this.allPassageTables = [];
            this.attachedPassageTables = [];
            this.selectedPassageTables = [];
            
            try {
                const token = localStorage.getItem("token");
                if (!token) {
                    console.error("Токен не найден");
                    return;
                }

                // Загружаем все доступные системные таблицы
                const allTablesResponse = await fetch("http://localhost:8080/system-tables", {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`
                    }
                });

                if (allTablesResponse.ok) {
                    this.allPassageTables = await allTablesResponse.json();
                }

                // Загружаем привязанные таблицы организации
                if (this.userOrganizationId) {
                    const orgTablesResponse = await fetch(`http://localhost:8080/organizations/${this.userOrganizationId}/tables`, {
                        method: "GET",
                        headers: {
                            "Authorization": `Bearer ${token}`
                        }
                    });

                    if (orgTablesResponse.ok) {
                        this.attachedPassageTables = await orgTablesResponse.json();
                        // Автоматически выбираем привязанные таблицы
                        this.selectedPassageTables = this.attachedPassageTables.map(table => table.id);
                    }
                }

                // Если нет привязанных таблиц организации, пробуем компанию
                if (this.attachedPassageTables.length === 0 && this.userCompanyId) {
                    const companyTablesResponse = await fetch(`http://localhost:8080/companies/${this.userCompanyId}/tables`, {
                        method: "GET",
                        headers: {
                            "Authorization": `Bearer ${token}`
                        }
                    });

                    if (companyTablesResponse.ok) {
                        this.attachedPassageTables = await companyTablesResponse.json();
                        // Автоматически выбираем привязанные таблицы
                        this.selectedPassageTables = this.attachedPassageTables.map(table => table.id);
                    }
                }

                this.validatePassageTables();

            } catch (error) {
                console.error("Ошибка при загрузке мест прохода:", error);
                this.allPassageTables = [];
                this.attachedPassageTables = [];
            } finally {
                this.loadingPassageTables = false;
            }
        },

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
                    this.allEmployees = await response.json();
                } else {
                    console.error("Ошибка при загрузке существующих сотрудников");
                }
            } catch (error) {
                console.error("Ошибка при загрузке существующих сотрудников:", error);
            }
        },

        formatNameField(fieldName) {
            if (this[fieldName]) {
                this[fieldName] = this.formatName(this[fieldName]);
            }
        },

        formatName(name) {
            if (!name) return '';
            return name.toLowerCase()
                .split(' ')
                .map(word => word.charAt(0).toUpperCase() + word.slice(1))
                .join(' ');
        },

        formatFullName(employee) {
            const parts = [];
            if (employee.last_name) parts.push(employee.last_name);
            if (employee.first_name) parts.push(employee.first_name);
            if (employee.middle_name) parts.push(employee.middle_name);
            return parts.join(' ') || 'Не указано';
        },
        
        togglePassageTable(tableId) {
            const index = this.selectedPassageTables.indexOf(tableId);
            if (index > -1) {
                this.selectedPassageTables.splice(index, 1);
            } else {
                this.selectedPassageTables.push(tableId);
            }
        },
        
        validatePassageTables() {
            this.errors.passageTables = this.selectedPassageTables.length === 0 ? '' : '';
        },
        
        formatPassageTables() {
            if (this.selectedPassageTables.length === 0) return '';
            
            const tableNames = this.selectedPassageTables.map(tableId => {
                const table = this.allPassageTables.find(t => t.id === tableId);
                return table ? table.display_name : '';
            }).filter(name => name);
            
            if (tableNames.length > 1) {
                return tableNames[0] + ' и др.';
            }
            
            return tableNames[0] || '';
        },
        
        addEmployee() {
            if (!this.canAddEmployee) {
                return;
            }
            
            // Если выбраны существующие сотрудники
            if (this.selectedExistingEmployees.length > 0) {
                this.addExistingEmployees();
                return;
            }
            
            // Если добавляется новый сотрудник
            const newEmployee = {
                lastName: this.lastName.trim(),
                firstName: this.firstName.trim(),
                middleName: this.middleName.trim(),
                position: this.position.trim(),
                citizenshipId: this.selectedCitizenship.id,
                citizenshipName: this.selectedCitizenship.name,
                passportSeriesNumber: this.passportSeriesNumber.trim(),
                patentNumber: this.isPatentRequired ? this.patentNumber.trim() : null,
                otherPermission: this.isPatentRequired ? this.selectedPermission : null,
                passageTables: this.formatPassageTables(),
                targetTables: [...this.selectedPassageTables],
                isExisting: false
            };
            
            if (this.editingEmployee) {
                newEmployee.id = this.editingEmployee.id;
                this.$emit('employee-updated', newEmployee);
                this.cancelEdit();
            } else {
                this.$emit('employee-added', newEmployee);
                this.clearEmployeeFormPartial();
            }
        },
        
        clearEmployeeFormPartial() {
            // Очищаем только основные поля, места прохода остаются выбранными
            this.lastName = '';
            this.firstName = '';
            this.middleName = '';
            this.position = '';
            this.passportSeriesNumber = '';
            this.patentNumber = '';
            this.selectedPermission = '';
            this.uploadedFiles = [];
        },
        
        clearEmployeeForm() {
            this.lastName = '';
            this.firstName = '';
            this.middleName = '';
            this.position = '';
            this.passportSeriesNumber = '';
            this.patentNumber = '';
            this.selectedPermission = '';
            this.selectedPassageTables = [];
            this.uploadedFiles = [];
            this.errors.passageTables = '';
            this.selectedExistingEmployees = [];
            this.editingEmployee = null;
        },

        // Методы для существующих сотрудников
        openExistingEmployeesModal() {
            this.showExistingEmployeesModal = true;
            this.tempSelectedEmployees = [...this.selectedExistingEmployees];
            this.loadExistingEmployees();
        },

        closeExistingEmployeesModal() {
            this.showExistingEmployeesModal = false;
            this.tempSelectedEmployees = [];
        },

        switchFilter(filter) {
            this.currentFilter = filter;
        },

        isEmployeeSelected(employee) {
            return this.tempSelectedEmployees.some(selectedEmployee => selectedEmployee.id === employee.id);
        },

        isEmployeeDisabled(employee) {
            // Проверяем, не добавлен ли уже сотрудник в список
            return this.existingEmployees.some(emp => 
                (emp.isExisting && emp.existingEmployeeId === employee.id) ||
                (!emp.isExisting && emp.passportSeriesNumber === employee.passport_series_number)
            );
        },

        toggleEmployeeSelection(employee) {
            if (this.isEmployeeDisabled(employee)) return;

            const index = this.tempSelectedEmployees.findIndex(selectedEmployee => selectedEmployee.id === employee.id);
            if (index > -1) {
                this.tempSelectedEmployees.splice(index, 1);
            } else {
                this.tempSelectedEmployees.push(employee);
            }
        },

        confirmExistingEmployeesSelection() {
            this.selectedExistingEmployees = [...this.tempSelectedEmployees];
            this.closeExistingEmployeesModal();
            // Очищаем форму нового сотрудника
            this.clearEmployeeFormPartial();
        },

        // Метод для добавления выбранных существующих сотрудников
        addExistingEmployees() {
            if (this.selectedExistingEmployees.length === 0) {
                alert('Выберите сотрудников для добавления');
                return;
            }

            if (this.selectedPassageTables.length === 0) {
                alert('Выберите места прохода');
                return;
            }

            const employees = this.selectedExistingEmployees.map(employee => ({
                lastName: employee.last_name,
                firstName: employee.first_name,
                middleName: employee.middle_name,
                position: employee.position,
                citizenshipId: employee.citizenship_id,
                citizenshipName: employee.citizenship_name,
                passportSeriesNumber: employee.passport_series_number,
                patentNumber: employee.patent_number,
                otherPermission: employee.other_permission,
                passageTables: this.formatPassageTables(),
                targetTables: [...this.selectedPassageTables],
                isExisting: true,
                existingEmployeeId: employee.id
            }));
            
            this.$emit('employees-added', employees);
            this.clearExistingEmployeesSelection();
        },

        clearExistingEmployeesSelection() {
            this.selectedExistingEmployees = [];
        },

        // Методы редактирования
        editEmployee(employee) {
            this.editingEmployee = employee;
            this.selectedExistingEmployees = [];
            
            if (employee.isExisting) {
                // Загружаем данные существующего сотрудника
                this.lastName = employee.lastName;
                this.firstName = employee.firstName;
                this.middleName = employee.middleName;
                this.position = employee.position;
                this.passportSeriesNumber = employee.passportSeriesNumber;
                this.patentNumber = employee.patentNumber;
                this.selectedPermission = employee.otherPermission;
                this.selectedPassageTables = employee.targetTables || [];
                
                // Для существующих сотрудников находим гражданство
                if (employee.citizenshipId) {
                    const citizenship = this.availableCitizenships.find(c => c.id === employee.citizenshipId);
                    if (citizenship) {
                        this.selectedCitizenship = citizenship;
                    }
                }
            } else {
                // Загружаем данные нового сотрудника
                this.lastName = employee.lastName;
                this.firstName = employee.firstName;
                this.middleName = employee.middleName;
                this.position = employee.position;
                this.passportSeriesNumber = employee.passportSeriesNumber;
                this.patentNumber = employee.patentNumber;
                this.selectedPermission = employee.otherPermission;
                this.selectedPassageTables = employee.targetTables || [];
                
                // Находим гражданство
                if (employee.citizenshipId) {
                    const citizenship = this.availableCitizenships.find(c => c.id === employee.citizenshipId);
                    if (citizenship) {
                        this.selectedCitizenship = citizenship;
                    }
                }
            }
        },

        cancelEdit() {
            this.$emit('edit-cancelled');
            this.editingEmployee = null;
            this.clearEmployeeForm();
        },
        
        // Dropdown методы
        toggleCitizenshipDropdown() {
            this.isCitizenshipDropdownOpen = !this.isCitizenshipDropdownOpen;
        },
        
        selectCitizenship(citizenship) {
            this.selectedCitizenship = citizenship;
            this.isCitizenshipDropdownOpen = false;
            // Сбрасываем поля патента и разрешения при смене гражданства
            if (!citizenship.patent_required) {
                this.patentNumber = '';
                this.selectedPermission = '';
            }
        },
        
        togglePermissionDropdown() {
            this.isPermissionDropdownOpen = !this.isPermissionDropdownOpen;
        },
        
        selectPermission(permission) {
            this.selectedPermission = permission;
            this.isPermissionDropdownOpen = false;
            // Очищаем поле патента при выборе разрешения
            this.patentNumber = '';
        },

        handlePatentInput() {
            // Очищаем выбранное разрешение при вводе патента
            if (this.patentNumber.trim() !== '') {
                this.selectedPermission = '';
            }
        },

        // Загрузка файлов
        triggerFileInput() {
            this.$refs.fileInput.click();
        },

        handleFileUpload(event) {
            const files = Array.from(event.target.files);
            files.forEach(file => {
                const fileExtension = file.name.split('.').pop().toLowerCase();
                const fileType = file.type.startsWith('image/') ? 'image' : 'document';
                const fileData = {
                    name: file.name,
                    file: file,
                    type: fileType,
                    extension: fileExtension
                };

                if (fileType === 'image') {
                    const reader = new FileReader();
                    reader.onload = (e) => {
                        fileData.preview = e.target.result;
                        this.uploadedFiles.push(fileData);
                    };
                    reader.readAsDataURL(file);
                } else {
                    this.uploadedFiles.push(fileData);
                }
            });
            
            event.target.value = '';
        },

        getFileIcon(extension) {
            const iconMap = {
                'pdf': require('@/assets/icons/pdf.png'),
                'doc': require('@/assets/icons/doc.png'),
                'docx': require('@/assets/icons/doc.png'),
                'xlsx': require('@/assets/icons/xlsx.png'),
                'xls': require('@/assets/icons/xlsx.png')
            };
            return iconMap[extension] || require('@/assets/icons/document.png');
        },

        removeFile(index) {
            this.uploadedFiles.splice(index, 1);
        }
    },
    async mounted() {
        // Загружаем гражданства и места прохода
        await Promise.all([
            this.loadCitizenships(),
            this.loadPassageTables()
        ]);

        document.addEventListener('click', (e) => {
            if (!e.target.closest('.citizenship__dropdown')) {
                this.isCitizenshipDropdownOpen = false;
            }
            
            if (!e.target.closest('.permission__dropdown')) {
                this.isPermissionDropdownOpen = false;
            }
        });
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

.completion__citizenship {
    display: flex;
    flex-direction: column;
    gap: 10px;
    position: relative;
    padding-bottom: 15px;
}

.citizenship__header {
    display: flex;
    justify-content: space-between;
    align-items: end;
}

.citizenship__label {
    font-size: 13px;
    color: #a2a2a2;
}

.citizenship-actions {
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

/* Tooltip styles */
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
    max-width: 420px;
    min-width: 420px;
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

.citizenship__dropdown {
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
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 380px;
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
    justify-content: space-between;
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

.patent-required-badge {
    background: #ffebee;
    color: #c62828;
    padding: 2px 6px;
    border-radius: 8px;
    font-size: 10px;
    font-weight: 500;
}

.completion__fields {
    display: flex;
    flex-direction: column;
    gap: 15px;
    margin-bottom: 15px;
}

.completion__name-row {
    display: flex;
    gap: 20px;
}

.completion__last-name,
.completion__first-name,
.completion__middle-name,
.completion__position,
.completion__passport,
.completion__patent {
    flex: 1;
}

.completion__last-name-header,
.completion__first-name-header,
.completion__middle-name-header,
.completion__position-header,
.completion__passport-header,
.completion__patent-header,
.completion__permission-header,
.completion__files-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 5px;
}

.name__input {
    width: 100%;
    height: 40px;
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    padding: 0 15px;
    outline: none;
    font-size: 14px;
    background: #FFF;
}

.name__input:focus {
    border-color: #4F5BDF;
}

.name__input:disabled {
    background-color: #f5f5f5;
    cursor: not-allowed;
}

/* Disabled field styles */
.disabled-field {
    opacity: 0.5;
}

.disabled-field .name__input,
.disabled-field .permission__dropdown-button {
    background-color: #f5f5f5;
    cursor: not-allowed;
}

/* Permission dropdown styles */
.completion__permission {
    width: 100%;
}

.permission__dropdown {
    width: 100%;
    height: 40px;
    position: relative;
}

.permission__dropdown-button {
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

.permission__dropdown-button:hover:not(:disabled) {
    border-color: #4F5BDF;
}

.permission__dropdown-button:disabled {
    background-color: #f5f5f5;
    cursor: not-allowed;
    opacity: 0.6;
}

.permission__button-content {
    display: flex;
    align-items: center;
    width: 100%;
    height: 100%;
    justify-content: space-between;
}

.permission__button-text {
    font-size: 14px;
    color: #000;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 150px;
    display: block;
}

.permission__button-arrow {
    width: 10px;
    height: 10px;
    transition: transform 0.2s;
    transform: rotate(90deg);
    flex-shrink: 0;
}

.permission__button-arrow--open {
    transform: rotate(-90deg);
}

.permission__dropdown-menu {
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

.permission__dropdown-item {
    padding: 8px 15px;
    cursor: pointer;
    transition: background-color 0.2s;
    border-bottom: 1px solid #f5f5f5;
    font-size: 12px;
}

.permission__dropdown-item:hover {
    background-color: #f5f5f5;
}

.permission__dropdown-item:last-child {
    border-bottom: none;
}

.permission__item-text {
    font-size: 12px;
    color: #333;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

/* File upload styles */
.completion__files {
    margin-top: 10px;
}

.files__upload {
    display: flex;
    gap: 10px;
    align-items: center;
}

.file-input {
    display: none;
}

.upload-button {
    background: #4F5BDF;
    color: white;
    border: none;
    border-radius: 15px;
    padding: 8px 15px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.upload-button:hover:not(:disabled) {
    background: #3a45c0;
}

.upload-button:disabled {
    background: #a2a2a2;
    cursor: not-allowed;
    opacity: 0.6;
}

.uploaded-files {
    margin-top: 10px;
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.uploaded-file {
    display: flex;
    align-items: center;
    padding: 8px 10px;
    background: #f8f9fa;
    border-radius: 8px;
    border: 1px solid #e6e6e6;
    gap: 10px;
}

.file-preview {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
}

.file-preview-image {
    width: 100%;
    height: 100%;
    object-fit: cover;
    border-radius: 4px;
}

.file-icon {
    width: 20px;
    height: 20px;
}

.file-name {
    font-size: 12px;
    color: #333;
    flex: 1;
}

.remove-file-btn {
    background: none;
    border: none;
    color: #ff4444;
    cursor: pointer;
    font-size: 16px;
    padding: 0;
    width: 20px;
    height: 20px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
}

.remove-file-btn:hover:not(:disabled) {
    background: #ffebee;
    border-radius: 50%;
}

.remove-file-btn:disabled {
    cursor: not-allowed;
    opacity: 0.6;
}

/* Passage tables styles */
.completion__passage {
    margin-top: 15px;
}

.passage__grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
    row-gap: 5px;
    max-width: 425px;
    margin-top: 5px;
}

.passage__item {
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

.passage__item:hover:not(.passage__item--active) {
    background: #e8e8e8;
}

.passage__item--active {
    background: #4F5BDF;
    color: #fff;
    border-color: #4F5BDF;
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

.no-tables-message {
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

/* Стили для существующих сотрудников */
.existing-employees-info {
    margin-bottom: 15px;
    padding: 10px;
    background: #f8f9fa;
    border-radius: 10px;
    border: 1px solid #e6e6e6;
}

.existing-employees-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.existing-employees-count {
    font-size: 14px;
    font-weight: 500;
    color: #333;
}

.existing-employees-actions {
    display: flex;
    gap: 10px;
}

.view-employees-btn {
    background: white;
    color: #4F5BDF;
    border: 1px solid #4F5BDF;
    border-radius: 15px;
    padding: 5px 10px;
    font-size: 11px;
    cursor: pointer;
}

.view-employees-btn:hover {
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
    width: 900px;
    max-width: 90vw;
    max-height: 80vh;
    overflow: hidden;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px;
    border-bottom: 1px solid #e6e6e6;
}

.modal-header__top {
    display: flex;
    align-items: center;
    gap: 15px;
}

.modal-header h3 {
    margin: 0;
    color: #333;
    font-size: 18px;
}

.selected-count-badge {
    background: #4F5BDF;
    color: white;
    border-radius: 50px;
    padding: 5px 12px;
    font-size: 12px;
    font-weight: 500;
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
}

.modal-close:hover {
    color: #333;
}

.modal-body {
    padding: 20px;
    max-height: 60vh;
    overflow-y: auto;
}

/* Вкладки фильтров */
.filter-tabs {
    display: flex;
    gap: 10px;
    margin-bottom: 20px;
    border-bottom: 1px solid #e6e6e6;
    padding-bottom: 10px;
}

.filter-tab {
    padding: 8px 16px;
    border: 1px solid #e6e6e6;
    background: white;
    border-radius: 50px;
    cursor: pointer;
    font-size: 12px;
    transition: all 0.2s;
    white-space: nowrap;
}

.filter-tab:hover {
    border-color: #4F5BDF;
}

.filter-tab--active {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

/* Список сотрудников */
.employees-list {
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    overflow: hidden;
    margin-bottom: 20px;
}

.employees-header {
    background: #f8f8f8;
    border-bottom: 1px solid #e6e6e6;
    padding: 12px 15px;
}

.header-row {
    display: flex;
    width: 100%;
    align-items: center;
    font-weight: 500;
    color: #a2a2a2;
    font-size: 14px;
}

.header-col {
    padding: 0 5px;
}

.select-col {
    width: 5%;
    text-align: center;
}

.number-col {
    width: 8%;
}

.name-col {
    width: 25%;
}

.position-col {
    width: 20%;
}

.citizenship-col {
    width: 15%;
}

.passport-col {
    width: 15%;
}

.status-col {
    width: 12%;
}

.employees-body {
    max-height: 300px;
    overflow-y: auto;
}

.employee-item {
    border-bottom: 1px solid #f0f0f0;
    transition: background-color 0.2s;
    cursor: pointer;
}

.employee-item:hover {
    background-color: #fafafa;
}

.employee-item--disabled {
    background-color: #f5f5f5;
    opacity: 0.6;
    cursor: not-allowed;
}

.employee-item--disabled:hover {
    background-color: #f5f5f5;
}

.employee-item--selected {
    background-color: #f0f9ff;
}

.employee-item--selected:hover {
    background-color: #e0f2fe;
}

.employee-row {
    display: flex;
    padding: 10px 15px;
    align-items: center;
}

.employee-col {
    padding: 0 5px;
}

.status-badge {
    padding: 4px 8px;
    border-radius: 12px;
    font-size: 11px;
    font-weight: 500;
    display: inline-block;
}

.status-active {
    background-color: #f0f9ff;
    color: #0369a1;
    border: 1px solid #bae6fd;
}

.status-inactive {
    background-color: #fef2f2;
    color: #991b1b;
    border: 1px solid #fecaca;
}

.no-employees-message {
    text-align: center;
    padding: 40px 20px;
    color: #a2a2a2;
    font-size: 14px;
}

.modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
}

.cancel-btn {
    background: white;
    color: #333;
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    padding: 8px 16px;
    font-size: 12px;
    cursor: pointer;
}

.cancel-btn:hover {
    background: #f5f5f5;
}

.select-btn {
    background: #4F5BDF;
    color: white;
    border: none;
    border-radius: 15px;
    padding: 8px 16px;
    font-size: 12px;
    cursor: pointer;
}

.select-btn:hover {
    background: #3a45c0;
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
</style>