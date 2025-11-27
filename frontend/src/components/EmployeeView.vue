<template>
    <section class="employeesview">
        <header class="employeesview__header">
            <h2 class="employeesview__title">
                Список <span class="blue">сотрудников</span>
            </h2>
        </header>
        
        <div class="employeesview__filters">
            <div class="filters-container">
                <SearchComponent
                    :title="'Поиск сотрудников...'"
                    v-model="searchQuery"
                />
                <div class="filter-tabs" v-if="ownershipInfo">
                    <button 
                        v-if="ownershipInfo.has_organization"
                        class="filter-tab"
                        :class="{ 'filter-tab--active': currentFilter === 'organization' }"
                        @click="switchFilter('organization')"
                    >
                        Сотрудники организации
                    </button>
                    <button 
                        v-if="ownershipInfo.has_company"
                        class="filter-tab"
                        :class="{ 'filter-tab--active': currentFilter === 'company' }"
                        @click="switchFilter('company')"
                    >
                        Сотрудники компании
                    </button>
                    <button 
                        class="filter-tab"
                        :class="{ 'filter-tab--active': currentFilter === 'user' }"
                        @click="switchFilter('user')"
                    >
                        Мои сотрудники
                    </button>
                </div>
            </div>
        </div>

        <div class="employeesview__container">
            <!-- Таблица сотрудников -->
            <div class="employees-card">
                <div class="card-header">
                    <div class="card-header__title">
                        <h3 class="card-title">
                            <span v-if="currentFilter === 'organization'" class="highlight-text">Сотрудники <span class="blue">организации</span></span>
                            <span v-else-if="currentFilter === 'company'" class="highlight-text">Сотрудники <span class="blue">компании</span></span>
                            <span v-else class="highlight-text">Мои <span class="blue">сотрудники</span></span>
                        </h3>
                    </div>
                    <div class="card-header__settings">
                        <button class="add-button" @click="showAddEmployeeModal">
                            Добавить
                        </button>
                        <RefreshButton @refresh="fetchEmployees" />
                    </div>
                </div>
                
                <div class="card-content">
                    <!-- Заголовок таблицы всегда отображается -->
                    <div class="employees-header">
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
                            <div class="header-col name-col" @click="sortBy('last_name')">
                                <p :class="{ 'active-sort': sortField === 'last_name' }">ФИО</p>
                                <img 
                                    src="@/assets/icons/sort.png" 
                                    class="sort-icon" 
                                    :class="{ 
                                        'sorted': sortField === 'last_name',
                                        'desc': sortField === 'last_name' && sortDirection === 'desc'
                                    }" 
                                />
                            </div>
                            <div class="header-col position-col" @click="sortBy('position')">
                                <p :class="{ 'active-sort': sortField === 'position' }">Должность</p>
                                <img 
                                    src="@/assets/icons/sort.png" 
                                    class="sort-icon" 
                                    :class="{ 
                                        'sorted': sortField === 'position',
                                        'desc': sortField === 'position' && sortDirection === 'desc'
                                    }" 
                                />
                            </div>
                            <div class="header-col citizenship-col" @click="sortBy('citizenship_name')">
                                <p :class="{ 'active-sort': sortField === 'citizenship_name' }">Гражданство</p>
                                <img 
                                    src="@/assets/icons/sort.png" 
                                    class="sort-icon" 
                                    :class="{ 
                                        'sorted': sortField === 'citizenship_name',
                                        'desc': sortField === 'citizenship_name' && sortDirection === 'desc'
                                    }" 
                                />
                            </div>
                            <div class="header-col passport-col" @click="sortBy('passport_series_number')">
                                <p :class="{ 'active-sort': sortField === 'passport_series_number' }">Паспорт</p>
                                <img 
                                    src="@/assets/icons/sort.png" 
                                    class="sort-icon" 
                                    :class="{ 
                                        'sorted': sortField === 'passport_series_number',
                                        'desc': sortField === 'passport_series_number' && sortDirection === 'desc'
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
                                Действия
                            </div>
                        </div>
                    </div>
                    
                    <!-- Тело таблицы -->
                    <div class="employees-container">
                        <div v-if="filteredEmployees.length > 0" class="employees-body">
                            <div 
                                v-for="(employee) in sortedEmployees" 
                                :key="employee.id" 
                                class="employee-item"
                            >
                                <div class="employee-row">
                                    <div class="employee-col number-col">
                                        {{ employee.id }}
                                    </div>
                                    <div class="employee-col name-col">
                                        {{ formatFullName(employee) }}
                                    </div>
                                    <div class="employee-col position-col">
                                        {{ employee.position || 'Не указана' }}
                                    </div>
                                    <div class="employee-col citizenship-col">
                                        {{ employee.citizenship_name || 'Не указано' }}
                                    </div>
                                    <div class="employee-col passport-col">
                                        {{ employee.passport_series_number || 'Не указан' }}
                                    </div>
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
                                    <div class="employee-col actions-col">
                                        <button 
                                            @click="editEmployee(employee)" 
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
                                            @click="deleteEmployee(employee)" 
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
                            {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Сотрудников нет' }}
                        </p>
                    </div>
                </div>
            </div>
            
            <div class="employeesview__right-side">
                <div class="employeesview__help">
                    <p class="help__text">
                        Здесь находятся сотрудники, привязанные к вашей <strong class="blue">организации</strong>. Вы можете использовать этих сотрудников при подаче заявок на пропуск.
                    </p>
                    <p class="help__text">
                        Новые сотрудники попадают в этот список <strong class="blue">автоматически</strong>, при подаче заявки.
                    </p>
                </div>
            </div>
        </div>

        <!-- Модальное окно добавления сотрудника -->
        <div v-if="showModal" class="modal-overlay" @click="closeModal">
            <div class="modal-content" @click.stop>
                <div class="modal-header">
                    <div class="modal-header__top">
                        <h3>{{ editingEmployee ? 'Редактирование' : 'Добавление сотрудника' }}</h3>
                        <div v-if="notification.show" class="notification-badge" :class="notification.type">
                            {{ notification.message }}
                        </div>
                    </div>
                    <button class="modal-close" @click="closeModal">×</button>
                </div>
                <div class="modal-body">
                    <div class="data__completion">
                        <div class="completion__citizenship">
                            <div class="citizenship__header">
                                <label class="citizenship__label">Гражданство</label>
                                <button class="add-button" @click="saveEmployee" :disabled="!canSaveEmployee">
                                    {{ editingEmployee ? 'Сохранить' : 'Добавить' }}
                                </button>
                            </div>
                            <div class="citizenship__dropdown">
                                <button class="dropdown__button" @click="toggleCitizenshipDropdown">
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
                                        <label class="input__label">Фамилия</label>
                                    </div>
                                    <input 
                                        class="name__input" 
                                        placeholder="Введите фамилию"
                                        v-model="lastName"
                                    />
                                </div>
                                <div class="completion__first-name">
                                    <div class="completion__first-name-header">
                                        <label class="input__label">Имя</label>
                                    </div>
                                    <input 
                                        class="name__input" 
                                        placeholder="Введите имя"
                                        v-model="firstName"
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
                                    />
                                </div>
                                <div class="completion__position">
                                    <div class="completion__position-header">
                                        <label class="input__label">Должность</label>
                                    </div>
                                    <input 
                                        class="name__input" 
                                        placeholder="Введите должность"
                                        v-model="position"
                                    />
                                </div>
                            </div>
                            
                            <!-- Третья строка: Паспорт и Номер патента -->
                            <div class="completion__name-row">
                                <div class="completion__passport">
                                    <div class="completion__passport-header">
                                        <label class="input__label">Серия и номер паспорта</label>
                                    </div>
                                    <input 
                                        class="name__input" 
                                        placeholder="XXXX XXXXXX"
                                        v-model="passportSeriesNumber"
                                        @input="formatPassport"
                                    />
                                </div>
                                <div class="completion__patent">
                                    <div class="completion__patent-header">
                                        <label class="input__label">Номер патента</label>
                                    </div>
                                    <input 
                                        class="name__input" 
                                        placeholder="Введите номер патента"
                                        v-model="patentNumber"
                                        :disabled="!isPatentRequired"
                                    />
                                </div>
                            </div>
                            
                            <!-- Четвертая строка: Иное разрешение -->
                            <div class="completion__permission">
                                <div class="completion__permission-header">
                                    <label class="input__label">Иное разрешение на работы</label>
                                </div>
                                <div class="permission__dropdown">
                                    <button class="permission__dropdown-button" @click="togglePermissionDropdown" :disabled="!isPatentRequired">
                                        <div class="permission__button-content">
                                            <span class="permission__button-text">{{ selectedPermission || 'Выберите разрешение' }}</span>
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
                                        accept="image/*,.pdf,.doc,.docx"
                                        class="file-input"
                                    />
                                    <button class="upload-button" @click="triggerFileInput">
                                        Загрузить
                                    </button>
                                </div>
                                <div v-if="uploadedFiles.length > 0" class="uploaded-files">
                                    <div v-for="(file, index) in uploadedFiles" :key="index" class="uploaded-file">
                                        <span class="file-name">{{ file.name }}</span>
                                        <button @click="removeFile(index)" class="remove-file-btn">×</button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Привязка -->
                        <div class="completion__binding">
                            <label class="input__label">Привязка</label>
                            <div class="binding-info">
                                <p class="binding-note">
                                    <strong>Добавляемый сотрудник автоматически привязывается к аккаунту пользователя.</strong>
                                    Сотрудника можно привязать к организации или компании, для использования <strong>другими сотрудниками</strong>:
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
                                    <span class="user-binding-text"><strong class="red">Внимание!</strong> При привязке сотрудника к организации или компании, он будет доступен для отображения и использования для всех сотрудников, привязанных к организации/компании. </span>
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
            employeesData: [],
            searchTimeout: null,
            currentFilter: 'user',
            ownershipInfo: null,
            showModal: false,
            availableCitizenships: [],

            // Гражданство
            selectedCitizenship: null,
            isCitizenshipDropdownOpen: false,
            
            // Основные поля
            lastName: '',
            firstName: '',
            middleName: '',
            position: '',
            passportSeriesNumber: '',
            patentNumber: '',
            
            // Разрешения
            selectedPermission: '',
            isPermissionDropdownOpen: false,
            availablePermissions: [
                'Разрешение на работу временного проживания',
                'Разрешение на работу вида на жительство', 
                'Свидетельство участника Госпрограммы',
                'Разрешение на работу для высококвалифицированных специалистов',
                'Иное разрешение'
            ],
            
            // Файлы
            uploadedFiles: [],
            
            // Привязка
            bindToOrganization: false,
            bindToCompany: false,
            
            // Уведомления
            notification: {
                show: false,
                message: '',
                type: 'success'
            },
            
            // Редактирование
            editingEmployee: null,
            originalEmployeeData: null
        };
    },
    computed: {
        filteredEmployees() {
            if (!this.searchQuery.trim()) {
                return this.employeesData;
            }
            
            const query = this.searchQuery.toLowerCase().trim();
            return this.employeesData.filter(employee => {
                const fullName = this.formatFullName(employee).toLowerCase();
                return fullName.includes(query) ||
                       (employee.position && employee.position.toLowerCase().includes(query)) ||
                       (employee.citizenship_name && employee.citizenship_name.toLowerCase().includes(query)) ||
                       (employee.passport_series_number && employee.passport_series_number.toLowerCase().includes(query)) ||
                       (employee.status ? 'активен' : 'неактивен').includes(query)
            });
        },

        sortedEmployees() {
            const employees = [...this.filteredEmployees];
            
            if (!this.sortField) {
                return employees;
            }
            
            return employees.sort((a, b) => {
                let valueA, valueB;
                
                switch (this.sortField) {
                    case 'id':
                        valueA = a.id;
                        valueB = b.id;
                        break;
                        
                    case 'last_name':
                        valueA = a.last_name?.toLowerCase() || '';
                        valueB = b.last_name?.toLowerCase() || '';
                        break;
                        
                    case 'position':
                        valueA = a.position?.toLowerCase() || '';
                        valueB = b.position?.toLowerCase() || '';
                        break;
                        
                    case 'citizenship_name':
                        valueA = a.citizenship_name?.toLowerCase() || '';
                        valueB = b.citizenship_name?.toLowerCase() || '';
                        break;
                        
                    case 'passport_series_number':
                        valueA = a.passport_series_number?.toLowerCase() || '';
                        valueB = b.passport_series_number?.toLowerCase() || '';
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

        selectedCitizenshipText() {
            return this.selectedCitizenship ? this.selectedCitizenship.name : 'Выберите гражданство';
        },

        isPatentRequired() {
            return this.selectedCitizenship ? this.selectedCitizenship.patent_required : false;
        },

        canSaveEmployee() {
            // Проверяем обязательные поля
            if (!this.lastName.trim() || !this.firstName.trim()) {
                return false;
            }
            
            // Проверяем гражданство
            if (!this.selectedCitizenship) {
                return false;
            }
            
            // Проверяем паспортные данные
            if (!this.passportSeriesNumber.trim()) {
                return false;
            }
            
            // Если требуется патент, проверяем дополнительные поля
            if (this.isPatentRequired) {
                if (!this.patentNumber.trim() || !this.selectedPermission) {
                    return false;
                }
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
        },
        
        isPatentRequired(newVal) {
            if (!newVal) {
                // Если патент не требуется, очищаем связанные поля
                this.patentNumber = '';
                this.selectedPermission = '';
                this.uploadedFiles = [];
            }
        }
    },
    methods: {
        async fetchEmployees() {
            try {
                const token = localStorage.getItem("token");
                const response = await fetch(`http://localhost:8080/unique-employees?filter_type=${this.currentFilter}`, {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`
                    }
                });

                if (response.ok) {
                    this.employeesData = await response.json();
                } else {
                    console.error("Ошибка при загрузке сотрудников");
                    this.employeesData = [];
                }
            } catch (error) {
                console.error("Ошибка при загрузке сотрудников:", error);
                this.employeesData = [];
            }
        },

        async fetchOwnershipInfo() {
            try {
                const token = localStorage.getItem("token");
                const response = await fetch("http://localhost:8080/unique-employees/ownership-info", {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`
                    }
                });

                if (response.ok) {
                    this.ownershipInfo = await response.json();
                } else {
                    // Если эндпоинт не существует, используем эндпоинт для машин (они используют одну логику)
                    const carResponse = await fetch("http://localhost:8080/unique-cars/ownership-info", {
                        method: "GET",
                        headers: {
                            "Authorization": `Bearer ${token}`
                        }
                    });
                    
                    if (carResponse.ok) {
                        this.ownershipInfo = await carResponse.json();
                    }
                }
            } catch (error) {
                console.error("Ошибка при загрузке информации о владельце:", error);
            }
        },

        async fetchCitizenships() {
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
                }
            } catch (error) {
                console.error("Ошибка при загрузке гражданств:", error);
            }
        },

        async deleteEmployee(employee) {
            if (confirm(`Вы уверены, что хотите удалить сотрудника ${this.formatFullName(employee)}?`)) {
                try {
                    const token = localStorage.getItem("token");
                    const response = await fetch(`http://localhost:8080/unique-employees/${employee.id}`, {
                        method: "DELETE",
                        headers: {
                            "Authorization": `Bearer ${token}`
                        }
                    });

                    if (response.ok) {
                        await this.fetchEmployees();
                        this.showNotification('Сотрудник успешно удален!', 'success');
                    } else {
                        alert("Ошибка при удалении сотрудника");
                    }
                } catch (error) {
                    console.error("Ошибка при удалении сотрудника:", error);
                    alert("Ошибка при удалении сотрудника");
                }
            }
        },

        editEmployee(employee) {
            this.editingEmployee = employee;
            
            // Сохраняем оригинальные значения для сравнения
            this.originalEmployeeData = {
                last_name: employee.last_name,
                first_name: employee.first_name,
                middle_name: employee.middle_name,
                position: employee.position,
                citizenship_id: employee.citizenship_id,
                passport_series_number: employee.passport_series_number,
                patent_number: employee.patent_number,
                other_permission: employee.other_permission,
                organization_id: employee.organization_id,
                company_id: employee.company_id
            };
            
            // Устанавливаем текущие значения сотрудника
            this.lastName = employee.last_name || '';
            this.firstName = employee.first_name || '';
            this.middleName = employee.middle_name || '';
            this.position = employee.position || '';
            this.passportSeriesNumber = employee.passport_series_number || '';
            this.patentNumber = employee.patent_number || '';
            this.selectedPermission = employee.other_permission || '';
            
            // Находим гражданство по citizenship_id
            if (employee.citizenship_id) {
                const employeeCitizenship = this.availableCitizenships.find(c => c.id === employee.citizenship_id);
                if (employeeCitizenship) {
                    this.selectedCitizenship = employeeCitizenship;
                }
            }
            
            // Устанавливаем привязки
            this.bindToOrganization = !!employee.organization_id;
            this.bindToCompany = !!employee.company_id;
            
            this.showModal = true;
            this.hideNotification();
        },

        // Проверка наличия изменений
        hasChanges() {
            if (!this.editingEmployee) {
                return true; // Для нового сотрудника всегда отправляем запрос
            }

            // Проверяем изменения в основных полях
            if (this.lastName !== this.originalEmployeeData.last_name ||
                this.firstName !== this.originalEmployeeData.first_name ||
                this.middleName !== this.originalEmployeeData.middle_name ||
                this.position !== this.originalEmployeeData.position ||
                this.passportSeriesNumber !== this.originalEmployeeData.passport_series_number ||
                this.patentNumber !== this.originalEmployeeData.patent_number ||
                this.selectedPermission !== this.originalEmployeeData.other_permission) {
                return true;
            }

            // Проверяем изменения в гражданстве
            if (this.selectedCitizenship.id !== this.originalEmployeeData.citizenship_id) {
                return true;
            }

            // Проверяем изменения в привязке к организации
            const currentOrgId = this.bindToOrganization ? this.ownershipInfo.organization_id : null;
            if (currentOrgId !== this.originalEmployeeData.organization_id) {
                return true;
            }

            // Проверяем изменения в привязке к компании
            const currentCompanyId = this.bindToCompany ? this.ownershipInfo.company_id : null;
            if (currentCompanyId !== this.originalEmployeeData.company_id) {
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
            this.fetchEmployees();
        },

        showAddEmployeeModal() {
            this.editingEmployee = null;
            this.showModal = true;
            this.hideNotification();
            this.resetNewEmployee();
        },

        closeModal() {
            this.showModal = false;
            this.editingEmployee = null;
            this.resetNewEmployee();
            this.hideNotification();
        },

        resetNewEmployee() {
            this.selectedCitizenship = this.availableCitizenships.find(c => c.is_default) || this.availableCitizenships[0];
            this.lastName = '';
            this.firstName = '';
            this.middleName = '';
            this.position = '';
            this.passportSeriesNumber = '';
            this.patentNumber = '';
            this.selectedPermission = '';
            this.uploadedFiles = [];
            this.bindToOrganization = false;
            this.bindToCompany = false;
        },

        clearFormFields() {
            // Очищаем только основные поля, чекбоксы остаются
            this.lastName = '';
            this.firstName = '';
            this.middleName = '';
            this.position = '';
            this.passportSeriesNumber = '';
            this.patentNumber = '';
            this.selectedPermission = '';
            this.uploadedFiles = [];
            
            // Если это добавление нового сотрудника (не редактирование), сбрасываем чекбоксы
            if (!this.editingEmployee) {
                this.bindToOrganization = false;
                this.bindToCompany = false;
            }
        },

        // Уведомления
        showNotification(message, type = 'success') {
            this.notification = {
                show: true,
                message: message,
                type: type
            };
            
            setTimeout(() => {
                this.hideNotification();
            }, 3000);
        },

        hideNotification() {
            this.notification.show = false;
        },

        // Форматирование ФИО
        formatFullName(employee) {
            const parts = [];
            if (employee.last_name) parts.push(employee.last_name);
            if (employee.first_name) parts.push(employee.first_name);
            if (employee.middle_name) parts.push(employee.middle_name);
            return parts.join(' ') || 'Не указано';
        },

        // Форматирование паспорта
        formatPassport(event) {
            let value = event.target.value.replace(/\D/g, '');
            
            if (value.length > 10) {
                value = value.slice(0, 10);
            }
            
            if (value.length > 4) {
                value = value.slice(0, 4) + ' ' + value.slice(4);
            }
            
            this.passportSeriesNumber = value;
            event.target.value = value;
        },

        // Dropdown методы
        toggleCitizenshipDropdown() {
            this.isCitizenshipDropdownOpen = !this.isCitizenshipDropdownOpen;
        },

        selectCitizenship(citizenship) {
            this.selectedCitizenship = citizenship;
            this.isCitizenshipDropdownOpen = false;
        },

        togglePermissionDropdown() {
            this.isPermissionDropdownOpen = !this.isPermissionDropdownOpen;
        },

        selectPermission(permission) {
            this.selectedPermission = permission;
            this.isPermissionDropdownOpen = false;
        },

        // Загрузка файлов
        triggerFileInput() {
            this.$refs.fileInput.click();
        },

        handleFileUpload(event) {
            const files = Array.from(event.target.files);
            files.forEach(file => {
                this.uploadedFiles.push({
                    name: file.name,
                    file: file,
                    type: this.getFileType(file)
                });
            });
            
            // Очищаем input для возможности повторной загрузки тех же файлов
            event.target.value = '';
        },

        getFileType(file) {
            if (file.type.startsWith('image/')) return 'photo';
            if (file.name.toLowerCase().includes('patent')) return 'patent';
            if (file.type === 'application/pdf') return 'document';
            return 'other';
        },

        removeFile(index) {
            this.uploadedFiles.splice(index, 1);
        },

        async saveEmployee() {
            if (!this.canSaveEmployee) {
                this.showNotification('Заполните все обязательные поля правильно', 'error');
                return;
            }

            // Проверяем изменения для редактирования
            if (this.editingEmployee && !this.hasChanges()) {
                this.showNotification('Изменений не обнаружено', 'info');
                return;
            }

            try {
                const token = localStorage.getItem("token");
                
                // Формируем данные для отправки
                const employeeData = {
                    last_name: this.lastName.trim(),
                    first_name: this.firstName.trim(),
                    middle_name: this.middleName.trim(),
                    position: this.position.trim(),
                    citizenship_id: this.selectedCitizenship.id,
                    passport_series_number: this.passportSeriesNumber.trim(),
                    patent_number: this.isPatentRequired ? this.patentNumber.trim() : null,
                    other_permission: this.isPatentRequired ? this.selectedPermission : null,
                    user_id: this.ownershipInfo.user_id,
                    organization_id: this.bindToOrganization ? this.ownershipInfo.organization_id : null,
                    company_id: this.bindToCompany ? this.ownershipInfo.company_id : null
                };

                let response;
                if (this.editingEmployee) {
                    // Редактирование существующего сотрудника
                    response = await fetch(`http://localhost:8080/unique-employees/${this.editingEmployee.id}`, {
                        method: "PUT",
                        headers: {
                            "Authorization": `Bearer ${token}`,
                            "Content-Type": "application/json"
                        },
                        body: JSON.stringify(employeeData)
                    });
                } else {
                    // Создание нового сотрудника
                    response = await fetch("http://localhost:8080/unique-employees", {
                        method: "POST",
                        headers: {
                            "Authorization": `Bearer ${token}`,
                            "Content-Type": "application/json"
                        },
                        body: JSON.stringify(employeeData)
                    });
                }

                if (response.ok) {
                    const savedEmployee = await response.json();
                    const action = this.editingEmployee ? 'обновлен' : 'добавлен';
                    this.showNotification(`Сотрудник успешно ${action}!`, 'success');
                    
                    // Загружаем файлы если есть
                    if (this.uploadedFiles.length > 0 && savedEmployee.id) {
                        await this.uploadEmployeeFiles(savedEmployee.id);
                    }
                    
                    // Обновляем список сотрудников
                    this.fetchEmployees();
                    
                    // Очищаем форму только при добавлении нового сотрудника
                    if (!this.editingEmployee) {
                        this.clearFormFields();
                    } else {
                        // Обновляем оригинальные данные после успешного сохранения
                        this.originalEmployeeData = {
                            last_name: this.lastName,
                            first_name: this.firstName,
                            middle_name: this.middleName,
                            position: this.position,
                            citizenship_id: this.selectedCitizenship.id,
                            passport_series_number: this.passportSeriesNumber,
                            patent_number: employeeData.patent_number,
                            other_permission: employeeData.other_permission,
                            organization_id: employeeData.organization_id,
                            company_id: employeeData.company_id
                        };
                    }
                } else {
                    const errorData = await response.json();
                    const errorMessage = errorData.message || "Ошибка при сохранении сотрудника";
                    
                    // Специальные сообщения для дубликатов
                    if (errorMessage.includes("уже существует") || errorMessage.includes("already exists")) {
                        this.showNotification("Сотрудник уже привязан к вашему аккаунту", 'error');
                    } else {
                        this.showNotification(errorMessage, 'error');
                    }
                }
            } catch (error) {
                console.error("Ошибка при сохранении сотрудника:", error);
                this.showNotification("Ошибка при сохранении сотрудника", 'error');
            }
        },

        async uploadEmployeeFiles(employeeId) {
            try {
                const token = localStorage.getItem("token");
                const formData = new FormData();
                
                this.uploadedFiles.forEach(file => {
                    formData.append('files', file.file);
                    formData.append('file_types', file.type);
                });
                
                const response = await fetch(`http://localhost:8080/unique-employees/${employeeId}/files`, {
                    method: "POST",
                    headers: {
                        "Authorization": `Bearer ${token}`
                    },
                    body: formData
                });
                
                if (!response.ok) {
                    console.error("Ошибка при загрузке файлов");
                }
            } catch (error) {
                console.error("Ошибка при загрузке файлов:", error);
            }
        }
    },
    async mounted() {
        await Promise.all([
            this.fetchOwnershipInfo(),
            this.fetchCitizenships()
        ]);
        await this.fetchEmployees();
        
        // Закрытие dropdown при клике вне
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
.employeesview {
    padding: 20px;
}

.employeesview__container {
    display: flex;
    gap: 30px;
    margin-top: 20px;
}

.employeesview__right-side {
    width: 40%;
}

.employeesview__header {
    padding-bottom: 15px;
    display: flex;
    align-items: center;
    gap: 10px;
    height: 25px;
}

.employeesview__title {
    font-size: 18px;
}

.employeesview__filters {
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
.employees-card {
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

.employees-container {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow-y: auto;
}

/* Заголовок таблицы */
.employees-header {
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
}

.name-col {
    width: 20%;
    min-width: 150px;
}

.position-col {
    width: 20%;
    min-width: 120px;
}

.citizenship-col {
    width: 15%;
    min-width: 100px;
}

.passport-col {
    width: 15%;
    min-width: 100px;
}

.status-col {
    width: 12%;
    min-width: 80px;
}

.actions-col {
    width: 10%;
    min-width: 80px;
    justify-content: center;
}

/* Тело таблицы */
.employees-body {
    overflow-y: auto;
    flex-grow: 1;
    padding-right: 4px;
    margin-right: 4px;
    scroll-behavior: smooth;
}

.employee-item {
    transition: background-color 0.2s ease;
}

.employee-item:hover {
    background-color: #fafafa;
}

.employee-row {
    display: flex;
    width: 100%;
    padding: 10px 16px;
    align-items: center;
    border-bottom: 1px solid #f0f0f0;
}

.employee-col {
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
.number-col .employee-col,
.actions-col .employee-col {
    justify-content: center;
}

/* Стилизация скроллбара */
.employees-body::-webkit-scrollbar {
    width: 6px;
}

.employees-body::-webkit-scrollbar-track {
    background: transparent;
    margin: 2px 0;
    border-radius: 3px;
}

.employees-body::-webkit-scrollbar-thumb {
    background: #D9E2FF;
    border-radius: 3px;
    border: 1px solid transparent;
    background-clip: content-box;
    transition: all 0.3s ease;
}

.employees-body::-webkit-scrollbar-thumb:hover {
    background: #C5D1FF;
    border: 1px solid transparent;
    background-clip: content-box;
    transform: scale(1.1);
}

.employees-body {
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
    width: 600px;
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

/* Стили формы добавления сотрудника */
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

.completion__citizenship {
    display: flex;
    flex-direction: column;
    gap: 5px;
    position: relative;
    padding-bottom: 10px;
}

.citizenship__header {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.citizenship__label {
    font-size: 13px;
    color: #a2a2a2;
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
}

.permission__button-arrow {
    width: 10px;
    height: 10px;
    transition: transform 0.2s;
    transform: rotate(90deg);
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
}

.permission__dropdown-item:hover {
    background-color: #f5f5f5;
}

.permission__dropdown-item:last-child {
    border-bottom: none;
}

.permission__item-text {
    font-size: 14px;
    color: #333;
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

.upload-button:hover {
    background: #3a45c0;
}

.uploaded-files {
    margin-top: 10px;
    display: flex;
    flex-direction: column;
    gap: 5px;
}

.uploaded-file {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 5px 10px;
    background: #f8f9fa;
    border-radius: 8px;
    border: 1px solid #e6e6e6;
}

.file-name {
    font-size: 12px;
    color: #333;
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
}

.remove-file-btn:hover {
    background: #ffebee;
    border-radius: 50%;
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

@media (max-width: 768px) {
    .employees-card {
        width: 100%;
        height: auto;
    }
    
    .header-row,
    .employee-row {
        flex-wrap: wrap;
    }
    
    .header-col,
    .employee-col {
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
    
    .employeesview__container {
        flex-direction: column;
    }
    
    .employeesview__right-side {
        width: 100%;
    }
    
    .completion__name-row {
        flex-direction: column;
    }
    
    .modal-content {
        width: 95vw;
        margin: 10px;
    }
}
</style>