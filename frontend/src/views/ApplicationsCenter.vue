<template>
    <section class="center">
        <header class="center__header">
            <div class="header-top">
                <h2 class="center__title">Центр заявок</h2>
                <div class="unread-badge" v-if="unreadCount > 0" :class="{ 'shake-animation': shouldShake }">
                    Новые: {{ unreadCount }}
                </div>
            </div>
        </header>
        
        <div class="center__filters">
            <div class="filters__main">
                <!-- Основные фильтры в строку -->
                <div class="filters-row">
                    <!-- Поиск -->
                    <div class="field search">
                        <input 
                            placeholder="Поиск заявок..." 
                            type="text" 
                            class="field__input search" 
                            v-model="searchQuery"
                            @input="applyFilters"
                        />
                        <img src="@/assets/icons/search.png" class="center__icon" />
                    </div>
                    
                    <!-- Организация -->
                    <div class="field field--select" @click="toggleDropdown('organization')">
                        <span class="select-text">{{ selectedOrganizationName || 'Организация' }}</span>
                        <img src="@/assets/icons/arrow.png" class="select-icon" :class="{ 'select-icon--rotated': showOrganizationDropdown }" />
                        <div class="custom-dropdown" v-if="showOrganizationDropdown" :class="{ 'dropdown-enter-active': showOrganizationDropdown }">
                            <div class="dropdown-search">
                                <input 
                                    type="text" 
                                    placeholder="Поиск..." 
                                    v-model="organizationSearch"
                                    @click.stop
                                    class="dropdown-search__input"
                                />
                            </div>
                            <div class="dropdown-item" @click.stop="selectOrganization(null, 'Все организации')">Все организации</div>
                            <div 
                                class="dropdown-item" 
                                v-for="org in filteredOrganizations" 
                                :key="org.id" 
                                @click.stop="selectOrganization(org.id, org.name)"
                                :class="{ 'dropdown-item--selected': org.id === selectedOrganizationId }"
                            >
                                {{ org.name }}
                            </div>
                            <div class="dropdown-no-results" v-if="filteredOrganizations.length === 0">
                                Организации не найдены
                            </div>
                        </div>
                    </div>

                    <!-- Дата -->
                    <div class="field date-field" @click="toggleDatePicker">
                        <input 
                            placeholder="Выберите дату" 
                            type="text" 
                            class="field__input" 
                            :value="dateRangeText"
                            readonly
                        />
                        <img src="@/assets/icons/calendar.png" class="center__icon" />
                        <div class="date-picker" v-if="showDatePicker" @click.stop :class="{ 'date-picker-enter-active': showDatePicker }">
                            <div class="date-picker__header">
                                <h4>Выберите период</h4>
                                <button class="date-picker__close" @click="closeDatePicker">×</button>
                            </div>
                            
                            <div class="date-picker__quick-buttons">
                                <button @click="setQuickDate('today')" class="quick-btn">Сегодня</button>
                                <button @click="setQuickDate('yesterday')" class="quick-btn">Вчера</button>
                                <button @click="setQuickDate('dayBeforeYesterday')" class="quick-btn">Позавчера</button>
                                <button @click="setQuickDate('thisWeek')" class="quick-btn">Эта неделя</button>
                                <button @click="setQuickDate('lastWeek')" class="quick-btn">Прошлая неделя</button>
                                <button @click="setQuickDate('thisMonth')" class="quick-btn">Этот месяц</button>
                                <button @click="setQuickDate('lastMonth')" class="quick-btn">Прошлый месяц</button>
                                <button @click="setQuickDate('thisYear')" class="quick-btn">Этот год</button>
                                <button @click="setQuickDate('lastYear')" class="quick-btn">Прошлый год</button>
                            </div>
                            
                            <div class="date-picker__range">
                                <div class="date-input-group">
                                    <label>С:</label>
                                    <input 
                                        type="date" 
                                        v-model="dateRangeStartInput"
                                        @change="updateDateRange"
                                        class="date-input"
                                    />
                                </div>
                                <div class="date-input-group">
                                    <label>ПО:</label>
                                    <input 
                                        type="date" 
                                        v-model="dateRangeEndInput"
                                        @change="updateDateRange"
                                        class="date-input"
                                    />
                                </div>
                            </div>
                            
                            <div class="date-picker__actions">
                                <button @click="applyDateRange" class="apply-btn">Применить</button>
                                <button @click="clearDateRange" class="clear-btn">Очистить</button>
                            </div>
                        </div>
                    </div>

                    <!-- Кнопка сброса сортировки -->
                    <button 
                        class="reset-sort-btn"
                        @click="resetSort"
                        :disabled="!sortField"
                    >
                        Сбросить сортировку
                    </button>

                    <!-- Кнопка сброса фильтров -->
                    <button 
                        class="reset-filters-btn"
                        @click="resetFilters"
                        :disabled="!hasActiveFilters"
                    >
                        Сбросить фильтры
                    </button>

                    <!-- Кнопка дополнительных фильтров -->
                    <button 
                        class="more-filters-btn"
                        @click="toggleMoreFilters"
                        :class="{ 'more-filters-btn--active': showMoreFilters }"
                    >
                        <span>Доп. фильтры</span>
                        <img src="@/assets/icons/arrow.png" class="more-filters-icon" :class="{ 'more-filters-icon--rotated': showMoreFilters }" />
                    </button>
                </div>

                <!-- Вторая строка фильтров -->
                <div class="filters-row filters-row--secondary">
                    <!-- Фильтр по подтверждению -->
                    <div class="filter-section">
                        <div class="filter-section__header">
                            <span class="filter-label">Подтверждение</span>
                        </div>
                        <div class="status-buttons">
                            <button 
                                v-for="confirmation in confirmations"
                                :key="confirmation.value"
                                class="status-btn"
                                :class="{ 'status-btn--active': selectedConfirmations.includes(confirmation.value) }"
                                @click="toggleConfirmation(confirmation.value)"
                            >
                                {{ confirmation.label }}
                            </button>
                        </div>
                    </div>

                    <!-- Фильтр по статусу заявки -->
                    <div class="filter-section">
                        <div class="filter-section__header">
                            <span class="filter-label">Статус заявки</span>
                        </div>
                        <div class="status-buttons">
                            <button 
                                v-for="status in applicationStatuses"
                                :key="status.value"
                                class="status-btn"
                                :class="{ 'status-btn--active': selectedApplicationStatuses.includes(status.value) }"
                                @click="toggleApplicationStatus(status.value)"
                            >
                                {{ status.label }}
                            </button>
                        </div>
                    </div>

                    <!-- Кнопка обновить -->
                    <div class="filter-section filter-section--refresh">
                        <RefreshButton @refresh="fetchApplications" />
                    </div>
                </div>
            </div>
        </div>

        <!-- Таблица заявок -->
        <div class="applications-table" :class="{ 'with-details': selectedApplication }">
            <div class="table-header">
                <div class="header-row">
                    <div class="header-col confirmation-col" @click="sortBy('confirmation')">
                        <p :class="{ 'active-sort': sortField === 'confirmation' }">Подтверждение</p>
                        <img 
                            src="@/assets/icons/sort.png" 
                            class="sort-icon" 
                            :class="{ 
                                'sorted': sortField === 'confirmation',
                                'desc': sortField === 'confirmation' && sortDirection === 'desc'
                            }" 
                        />
                    </div>
                    <div class="header-col number-col" @click="sortBy('number')">
                        <p :class="{ 'active-sort': sortField === 'number' }">Номер заявки</p>
                        <img 
                            src="@/assets/icons/sort.png" 
                            class="sort-icon" 
                            :class="{ 
                                'sorted': sortField === 'number',
                                'desc': sortField === 'number' && sortDirection === 'desc'
                            }" 
                        />
                    </div>
                    <div class="header-col date-col" @click="sortBy('date')">
                        <p :class="{ 'active-sort': sortField === 'date' }">Дата и время</p>
                        <img 
                            src="@/assets/icons/sort.png" 
                            class="sort-icon" 
                            :class="{ 
                                'sorted': sortField === 'date',
                                'desc': sortField === 'date' && sortDirection === 'desc'
                            }" 
                        />
                    </div>
                    <div class="header-col organization-col" @click="sortBy('organization')">
                        <p :class="{ 'active-sort': sortField === 'organization' }">Организация</p>
                        <img 
                            src="@/assets/icons/sort.png" 
                            class="sort-icon" 
                            :class="{ 
                                'sorted': sortField === 'organization',
                                'desc': sortField === 'organization' && sortDirection === 'desc'
                            }" 
                        />
                    </div>
                    <div class="header-col sender-col" @click="sortBy('sender')">
                        <p :class="{ 'active-sort': sortField === 'sender' }">Отправитель</p>
                        <img 
                            src="@/assets/icons/sort.png" 
                            class="sort-icon" 
                            :class="{ 
                                'sorted': sortField === 'sender',
                                'desc': sortField === 'sender' && sortDirection === 'desc'
                            }" 
                        />
                    </div>
                    <div class="header-col status-col" @click="sortBy('status')">
                        <p :class="{ 'active-sort': sortField === 'status' }">Статус заявки</p>
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
                        <!-- Убрано слово "Действия" -->
                    </div>
                </div>
            </div>
            
            <div class="table-body">
                <div v-if="filteredApplications.length > 0" class="applications-list">
                    <div 
                        v-for="(application, index) in sortedApplications" 
                        :key="application.id" 
                        class="application-item"
                        :class="{ 
                            'unread': application.status === 'Непрочитано',
                            'initial-load': isInitialLoad,
                            'filtered': !isInitialLoad
                        }"
                        @click="openApplication(application)"
                        :style="isInitialLoad ? { 'animation-delay': `${index * 0.05}s` } : {}"
                    >
                        <div class="application-row">
                            <div class="application-col confirmation-col">
                                <span 
                                    class="confirmation-badge"
                                    :class="getConfirmationClass(application.confirmation)"
                                >
                                    {{ application.confirmation }}
                                </span>
                            </div>
                            <div class="application-col number-col">
                                <span class="application-number">{{ application.application_number }}</span>
                            </div>
                            <div class="application-col date-col">
                                {{ formatDateTime(application.sending_datetime) }}
                            </div>
                            <div class="application-col organization-col">
                                {{ getOrganizationName(application) }}
                            </div>
                            <div class="application-col sender-col" :title="getFullNameTooltip(application.sender_name)">
                                {{ getSenderName(application.sender_name) }}
                            </div>
                            <div class="application-col status-col">
                                <span 
                                    class="status-badge"
                                    :class="getStatusClass(application.status)"
                                >
                                    {{ application.status }}
                                </span>
                            </div>
                            <div class="application-col actions-col">
                                <button 
                                    @click.stop="downloadApplication(application)" 
                                    class="download-btn"
                                    title="Скачать"
                                >
                                    <img 
                                        src="@/assets/icons/download.png" 
                                        alt="Скачать" 
                                        class="download-icon"
                                    />
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
                <p v-else class="no-data-message">
                    {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Заявок нет' }}
                </p>
            </div>
        </div>

        <!-- Детальное представление заявки -->
        <div v-if="selectedApplication" class="application-detail-overlay" @click.self="closeApplicationDetail">
            <div class="application-detail">
                <!-- Заголовок и кнопки -->
                <div class="detail-header">
                    <div class="detail-header-left">
                        <div class="detail-title-row">
                            <h3 class="detail-title">Заявка {{ selectedApplication.application_number }}</h3>
                            <div class="detail-datetime">{{ formatDateTime(selectedApplication.sending_datetime) }}</div>
                        </div>
                    </div>
                    <div class="detail-header-right">
                        <!-- Кнопки согласования для ответственных -->
                        <div v-if="isResponsibleUser && selectedApplication.confirmation === 'Согласование'" class="confirmation-buttons">
                            <button 
                                class="confirm-btn" 
                                @click="updateConfirmation('Согласовано')"
                                :disabled="updatingConfirmation"
                            >
                                Согласовать
                            </button>
                            <button 
                                class="reject-btn" 
                                @click="updateConfirmation('Не согласовано')"
                                :disabled="updatingConfirmation"
                            >
                                Отказать
                            </button>
                        </div>
                        <button class="close-detail-btn" @click="closeApplicationDetail">×</button>
                    </div>
                </div>

                <div class="detail-content">
                    <!-- Левая колонка - вложения -->
                    <div class="detail-left-column">
                        <ApplicationAttachments 
                            :application-id="selectedApplication.id"
                            :attachments="applicationAttachments"
                            @attachment-selected="selectAttachment"
                        />
                    </div>

                    <!-- Центральная колонка - детали -->
                    <div class="detail-main-column">
                        <!-- Сообщение заявки -->
                        <div class="message-section">
                            <h4>Сообщение к заявке {{ selectedApplication.application_number }}</h4>
                            <div class="message-content">
                                {{ selectedApplication.message || 'Сообщение отсутствует' }}
                            </div>
                        </div>

                        <!-- Детали выбранного вложения -->
                        <div v-if="selectedAttachment" class="attachment-details">
                            <h4>{{ selectedAttachment.attachment_display_name }}</h4>
                            
                            <!-- Даты действия -->
                            <div v-if="selectedAttachment.entry_date_from || selectedAttachment.entry_date_to" class="date-range">
                                <span class="date-label">Срок действия:</span>
                                <span class="date-value">
                                    {{ formatDate(selectedAttachment.entry_date_from) }}
                                    <span v-if="selectedAttachment.entry_date_to"> - {{ formatDate(selectedAttachment.entry_date_to) }}</span>
                                </span>
                            </div>

                            <!-- Время действия -->
                            <div v-if="selectedAttachment.entry_time_from || selectedAttachment.entry_time_to" class="time-range">
                                <span class="time-label">Время:</span>
                                <span class="time-value">
                                    {{ selectedAttachment.entry_time_from }}
                                    <span v-if="selectedAttachment.entry_time_to"> - {{ selectedAttachment.entry_time_to }}</span>
                                </span>
                            </div>

                            <!-- Данные вложения в зависимости от типа -->
                            <div class="attachment-data">
                                <!-- Автомобили -->
                                <div v-if="selectedAttachment.attachment_type === 'cars' && attachmentCars.length > 0" class="cars-section">
                                    <h5>Автомобили</h5>
                                    <div class="cars-list">
                                        <div v-for="car in attachmentCars" :key="car.id" class="car-item">
                                            <div class="car-info">
                                                <span class="car-number">{{ car.car_number }}</span>
                                                <span class="car-brand">{{ car.car_brand }}</span>
                                            </div>
                                            <!-- Места разгрузки -->
                                            <div v-if="car.unload_places && car.unload_places.length > 0" class="unload-places">
                                                <span class="places-label">Места разгрузки:</span>
                                                <span class="places-list">
                                                    {{ car.unload_places.map(p => p.name).join(', ') }}
                                                </span>
                                            </div>
                                        </div>
                                    </div>
                                </div>

                                <!-- Сотрудники -->
                                <div v-if="selectedAttachment.attachment_type === 'people' && attachmentEmployees.length > 0" class="employees-section">
                                    <h5>Сотрудники</h5>
                                    <div class="employees-list">
                                        <div v-for="employee in attachmentEmployees" :key="employee.id" class="employee-item">
                                            <div class="employee-info">
                                                <span class="employee-name">{{ employee.last_name }} {{ employee.first_name }} {{ employee.middle_name || '' }}</span>
                                                <span class="employee-position">{{ employee.position }}</span>
                                            </div>
                                            <!-- Целевые таблицы -->
                                            <div v-if="employee.target_tables && employee.target_tables.length > 0" class="target-tables">
                                                <span class="tables-label">Целевые таблицы:</span>
                                                <span class="tables-list">
                                                    {{ employee.target_tables.map(t => t.display_name).join(', ') }}
                                                </span>
                                            </div>
                                        </div>
                                    </div>
                                </div>

                                <!-- ТМЦ -->
                                <div v-if="selectedAttachment.attachment_type === 'items' && attachmentItems.length > 0" class="items-section">
                                    <h5>Товарно-материальные ценности</h5>
                                    <div class="items-list">
                                        <div v-for="item in attachmentItems" :key="item.id" class="item-item">
                                            <span class="item-name">{{ item.name }}</span>
                                            <span class="item-count">Количество: {{ item.count }}</span>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Правая колонка - информация о заявке и согласовании -->
                    <div class="detail-right-column">
                        <!-- Основная информация -->
                        <div class="basic-info-section">
                            <h4>Основная информация</h4>
                            <div class="info-grid">
                                <div class="info-row">
                                    <span class="info-label">Организация:</span>
                                    <span class="info-value">{{ selectedApplication.organization_name }}</span>
                                </div>
                                <div v-if="selectedApplication.company_name" class="info-row">
                                    <span class="info-label">Компания:</span>
                                    <span class="info-value">{{ selectedApplication.company_name }}</span>
                                </div>
                                <div class="info-row">
                                    <span class="info-label">Отправитель:</span>
                                    <span class="info-value">{{ selectedApplication.sender_full_name || selectedApplication.sender_name }}</span>
                                </div>
                            </div>
                        </div>

                        <!-- Статус согласования -->
                        <div class="confirmation-section">
                            <h4>Согласование заявки</h4>
                            <div class="confirmation-info">
                                <div class="info-row">
                                    <span class="info-label">Статус:</span>
                                    <span class="info-value" :class="getConfirmationClass(selectedApplication.confirmation)">
                                        {{ selectedApplication.confirmation }}
                                    </span>
                                </div>
                                <div v-if="selectedApplication.responsible_name" class="info-row">
                                    <span class="info-label">Ответственный:</span>
                                    <span class="info-value">{{ selectedApplication.responsible_name }}</span>
                                </div>
                                <div v-if="selectedApplication.confirmation_datetime" class="info-row">
                                    <span class="info-label">Дата согласования:</span>
                                    <span class="info-value">{{ formatDateTime(selectedApplication.confirmation_datetime) }}</span>
                                </div>
                            </div>

                            <!-- Ответственные пользователи -->
                            <div v-if="responsibleUsers.length > 0" class="responsible-users-section">
                                <h5>Ответственные за согласование</h5>
                                <div class="users-list">
                                    <div v-for="user in responsibleUsers" :key="user.id" class="user-item">
                                        <div class="user-info">
                                            <span class="user-name">{{ user.last_name }} {{ user.first_name }} {{ user.middle_name || '' }}</span>
                                            <span v-if="user.position" class="user-position">{{ user.position }}</span>
                                        </div>
                                        <span v-if="user.is_primary" class="primary-badge">Основной</span>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Комментарий ответственного -->
                        <div v-if="selectedApplication.responsible_comment" class="comment-section">
                            <h4>Комментарий ответственного</h4>
                            <div class="comment-content">
                                {{ selectedApplication.responsible_comment }}
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </section>
</template>

<script>
import RefreshButton from '../components/RefreshButton.vue'
import ApplicationAttachments from '../components/ApplicationAttachments.vue'

export default {
    components: {
        RefreshButton,
        ApplicationAttachments
    },
    data() {
        return {
            searchQuery: '',
            selectedOrganizationId: null,
            selectedOrganizationName: '',
            selectedConfirmations: [],
            selectedApplicationStatuses: [],
            showMoreFilters: false,
            organizations: [],
            showOrganizationDropdown: false,
            organizationSearch: '',
            sortField: null,
            sortDirection: 'desc',
            shouldShake: false,
            shakeInterval: null,
            isInitialLoad: true,
            
            // Дата
            showDatePicker: false,
            dateRangeStart: null,
            dateRangeEnd: null,
            dateRangeStartInput: '',
            dateRangeEndInput: '',
            
            // Конфигурации
            confirmations: [
                { value: 'Согласовано', label: 'Согласовано' },
                { value: 'Не согласовано', label: 'Не согласовано' },
                { value: 'Согласование', label: 'На согласовании' }
            ],
            applicationStatuses: [
                { value: 'Непрочитано', label: 'Непрочитано' },
                { value: 'В обработке', label: 'В обработке' },
                { value: 'В работе', label: 'В работе' },
                { value: 'Завершено', label: 'Завершено' },
                { value: 'Отказано', label: 'Отказано' }
            ],
            
            // Данные заявок
            applications: [],
            
            // Детали заявки
            selectedApplication: null,
            applicationAttachments: [],
            selectedAttachment: null,
            attachmentCars: [],
            attachmentEmployees: [],
            attachmentItems: [],
            responsibleUsers: [],
            currentUserId: null,
            currentUserName: '',
            updatingConfirmation: false
        }
    },
    computed: {
        filteredOrganizations() {
            if (!this.organizationSearch) return this.organizations;
            const searchTerm = this.organizationSearch.toLowerCase();
            return this.organizations.filter(org => 
                org.name.toLowerCase().includes(searchTerm)
            );
        },
        
        dateRangeText() {
            if (this.dateRangeStart && this.dateRangeEnd) {
                const start = this.formatDate(this.dateRangeStart);
                const end = this.formatDate(this.dateRangeEnd);
                return start === end ? start : `${start} - ${end}`;
            }
            return 'Выберите дату';
        },
        
        filteredApplications() {
            let filtered = this.applications;

            // Фильтр по поиску
            if (this.searchQuery.trim()) {
                const query = this.normalizeSearch(this.searchQuery.trim());
                filtered = filtered.filter(app => {
                    const searchFields = [
                        app.application_number,
                        this.getOrganizationName(app),
                        app.sender_name,
                        app.status,
                        app.confirmation
                    ];
                    
                    return searchFields.some(field => {
                        const normalizedField = this.normalizeSearch(field);
                        const searchWords = query.split(' ').filter(word => word.length > 0);
                        return searchWords.every(word => normalizedField.includes(word));
                    });
                });
            }

            // Фильтр по организации
            if (this.selectedOrganizationId) {
                filtered = filtered.filter(app => 
                    app.organization_id === this.selectedOrganizationId
                );
            }

            // Фильтр по подтверждению
            if (this.selectedConfirmations.length > 0) {
                filtered = filtered.filter(app => 
                    this.selectedConfirmations.includes(app.confirmation)
                );
            }

            // Фильтр по статусу заявки
            if (this.selectedApplicationStatuses.length > 0) {
                filtered = filtered.filter(app => 
                    this.selectedApplicationStatuses.includes(app.status)
                );
            }

            // Фильтр по дате
            if (this.dateRangeStart && this.dateRangeEnd) {
                filtered = filtered.filter(app => {
                    const appDate = new Date(app.sending_datetime);
                    const startOfDay = new Date(this.dateRangeStart);
                    startOfDay.setHours(0, 0, 0, 0);
                    const endOfDay = new Date(this.dateRangeEnd);
                    endOfDay.setHours(23, 59, 59, 999);
                    return appDate >= startOfDay && appDate <= endOfDay;
                });
            }

            return filtered;
        },
        
        sortedApplications() {
            const applications = [...this.filteredApplications];
            
            if (!this.sortField) {
                return applications.sort((a, b) => {
                    const dateA = new Date(a.sending_datetime);
                    const dateB = new Date(b.sending_datetime);
                    return dateB - dateA;
                });
            }
            
            return applications.sort((a, b) => {
                let valueA, valueB;
                
                switch (this.sortField) {
                    case 'confirmation':
                        valueA = a.confirmation;
                        valueB = b.confirmation;
                        break;
                    case 'number':
                        valueA = a.application_number;
                        valueB = b.application_number;
                        break;
                    case 'date':
                        valueA = new Date(a.sending_datetime);
                        valueB = new Date(b.sending_datetime);
                        break;
                    case 'organization':
                        valueA = this.getOrganizationName(a).toLowerCase();
                        valueB = this.getOrganizationName(b).toLowerCase();
                        break;
                    case 'sender':
                        valueA = a.sender_name?.toLowerCase() || '';
                        valueB = b.sender_name?.toLowerCase() || '';
                        break;
                    case 'status':
                        valueA = a.status?.toLowerCase() || '';
                        valueB = b.status?.toLowerCase() || '';
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
            return !!this.searchQuery.trim() || 
                   !!this.selectedOrganizationId || 
                   this.selectedConfirmations.length > 0 || 
                   this.selectedApplicationStatuses.length > 0 ||
                   (this.dateRangeStart && this.dateRangeEnd);
        },

        unreadCount() {
            return this.applications.filter(app => app.status === 'Непрочитано').length;
        },

        isResponsibleUser() {
            if (!this.currentUserId || !this.responsibleUsers.length) return false;
            
            return this.responsibleUsers.some(user => user.id === this.currentUserId);
        }
    },
    methods: {
        getOrganizationName(application) {
            if (application.organization_name && application.organization_name.trim()) {
                return application.organization_name;
            }
            else if (application.company_name && application.company_name.trim()) {
                return application.company_name;
            }
            return 'Не указана';
        },
        
        getSenderName(fullName) {
            if (!fullName) return '';
            return fullName;
        },
        
        getFullNameTooltip(fullName) {
            if (!fullName) return '';
            return fullName.replace(/\./g, '').trim();
        },
        
        normalizeSearch(text) {
            if (!text) return '';
            
            const translitMap = {
                'а': 'a', 'б': 'b', 'в': 'v', 'г': 'g', 'д': 'd',
                'е': 'e', 'ё': 'e', 'ж': 'zh', 'з': 'z', 'и': 'i',
                'й': 'y', 'к': 'k', 'л': 'l', 'м': 'm', 'н': 'n',
                'о': 'o', 'п': 'p', 'р': 'r', 'с': 's', 'т': 't',
                'у': 'u', 'ф': 'f', 'х': 'h', 'ц': 'ts', 'ч': 'ch',
                'ш': 'sh', 'щ': 'sch', 'ъ': '', 'ы': 'y', 'ь': '',
                'э': 'e', 'ю': 'yu', 'я': 'ya',
                'a': 'а', 'b': 'б', 'c': 'ц', 'd': 'д', 'e': 'е',
                'f': 'ф', 'g': 'г', 'h': 'х', 'i': 'и', 'j': 'й',
                'k': 'к', 'l': 'л', 'm': 'м', 'n': 'н', 'o': 'о',
                'p': 'п', 'q': 'к', 'r': 'р', 's': 'с', 't': 'т',
                'u': 'у', 'v': 'в', 'w': 'в', 'x': 'кс', 'y': 'й',
                'z': 'з'
            };
            
            let normalized = text.toString().toLowerCase();
            normalized = normalized.split('').map(char => 
                translitMap[char] || char
            ).join('');
            normalized = normalized.replace(/[^\w\sа-яё]/g, '');
            normalized = normalized.replace(/\s+/g, ' ').trim();
            
            return normalized;
        },
        
        toggleMoreFilters() {
            this.showMoreFilters = !this.showMoreFilters;
        },
        
        toggleDropdown(type) {
            if (type === 'organization') {
                this.showOrganizationDropdown = !this.showOrganizationDropdown;
                this.showDatePicker = false;
                if (this.showOrganizationDropdown) {
                    this.organizationSearch = '';
                }
            }
        },
        
        selectOrganization(id, name) {
            this.selectedOrganizationId = id;
            this.selectedOrganizationName = name;
            this.showOrganizationDropdown = false;
            this.organizationSearch = '';
            this.applyFilters();
        },
        
        toggleConfirmation(status) {
            const index = this.selectedConfirmations.indexOf(status);
            if (index > -1) {
                this.selectedConfirmations.splice(index, 1);
            } else {
                this.selectedConfirmations.push(status);
            }
            this.applyFilters();
        },
        
        toggleApplicationStatus(status) {
            const index = this.selectedApplicationStatuses.indexOf(status);
            if (index > -1) {
                this.selectedApplicationStatuses.splice(index, 1);
            } else {
                this.selectedApplicationStatuses.push(status);
            }
            this.applyFilters();
        },
        
        resetFilters() {
            this.selectedOrganizationId = null;
            this.selectedOrganizationName = '';
            this.selectedConfirmations = [];
            this.selectedApplicationStatuses = [];
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            this.dateRangeStartInput = '';
            this.dateRangeEndInput = '';
            this.searchQuery = '';
            this.isInitialLoad = false;
            this.fetchApplications();
        },
        
        toggleDatePicker() {
            this.showDatePicker = !this.showDatePicker;
            this.showOrganizationDropdown = false;
        },
        
        closeDatePicker() {
            this.showDatePicker = false;
        },
        
        formatDate(date) {
            if (!date) return '';
            if (typeof date === 'string') {
                date = new Date(date);
            }
            return date.toLocaleDateString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric'
            });
        },
        
        formatDateTime(dateTimeString) {
            if (!dateTimeString) return '';
            const date = new Date(dateTimeString);
            return date.toLocaleString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric',
                hour: '2-digit',
                minute: '2-digit'
            });
        },
        
        setQuickDate(period) {
            const today = new Date();
            let start, end;
            
            const periods = {
                today: () => [new Date(today), new Date(today)],
                yesterday: () => {
                    const date = new Date(today);
                    date.setDate(today.getDate() - 1);
                    return [date, date];
                },
                dayBeforeYesterday: () => {
                    const date = new Date(today);
                    date.setDate(today.getDate() - 2);
                    return [date, date];
                },
                thisWeek: () => {
                    const start = new Date(today);
                    start.setDate(today.getDate() - today.getDay() + (today.getDay() === 0 ? -6 : 1));
                    return [start, new Date(today)];
                },
                lastWeek: () => {
                    const start = new Date(today);
                    start.setDate(today.getDate() - today.getDay() - 6);
                    const end = new Date(start);
                    end.setDate(start.getDate() + 6);
                    return [start, end];
                },
                thisMonth: () => [
                    new Date(today.getFullYear(), today.getMonth(), 1),
                    new Date(today.getFullYear(), today.getMonth() + 1, 0)
                ],
                lastMonth: () => [
                    new Date(today.getFullYear(), today.getMonth() - 1, 1),
                    new Date(today.getFullYear(), today.getMonth(), 0)
                ],
                thisYear: () => [
                    new Date(today.getFullYear(), 0, 1),
                    new Date(today.getFullYear(), 11, 31)
                ],
                lastYear: () => [
                    new Date(today.getFullYear() - 1, 0, 1),
                    new Date(today.getFullYear() - 1, 11, 31)
                ]
            };
            
            [start, end] = periods[period]();
            start.setHours(0, 0, 0, 0);
            end.setHours(23, 59, 59, 999);
            
            this.dateRangeStart = start;
            this.dateRangeEnd = end;
            this.dateRangeStartInput = this.formatDateForInput(start);
            this.dateRangeEndInput = this.formatDateForInput(end);
            this.showDatePicker = false;
            this.applyFilters();
        },
        
        formatDateForInput(date) {
            return date ? date.toISOString().split('T')[0] : '';
        },
        
        updateDateRange() {
            if (this.dateRangeStartInput) {
                const start = new Date(this.dateRangeStartInput);
                start.setHours(0, 0, 0, 0);
                this.dateRangeStart = start;
            }
            if (this.dateRangeEndInput) {
                const end = new Date(this.dateRangeEndInput);
                end.setHours(23, 59, 59, 999);
                this.dateRangeEnd = end;
            }
        },
        
        applyDateRange() {
            this.updateDateRange();
            this.showDatePicker = false;
            this.applyFilters();
        },
        
        clearDateRange() {
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            this.dateRangeStartInput = '';
            this.dateRangeEndInput = '';
            this.showDatePicker = false;
            this.applyFilters();
        },
        
        getConfirmationClass(confirmation) {
            const classes = {
                'Согласовано': 'confirmation-approved',
                'Согласование': 'confirmation-pending',
                'Не согласовано': 'confirmation-rejected'
            };
            return classes[confirmation] || 'confirmation-default';
        },

        getStatusClass(status) {
            const statusClasses = {
                'Непрочитано': 'status-unread',
                'В обработке': 'status-processing',
                'В работе': 'status-in-progress',
                'Завершено': 'status-completed',
                'Отказано': 'status-rejected'
            };
            return statusClasses[status] || 'status-default';
        },
        
        sortBy(field) {
            if (this.sortField === field) {
                this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
            } else {
                this.sortField = field;
                this.sortDirection = 'desc';
            }
            this.isInitialLoad = false;
        },

        resetSort() {
            this.sortField = null;
            this.sortDirection = 'desc';
        },
        
        applyFilters() {
            this.isInitialLoad = false;
        },
        
        async fetchApplications() {
            try {
                const token = localStorage.getItem("token");
                if (!token) {
                    console.error("Пользователь не авторизован.");
                    return;
                }

                let url = "http://localhost:8080/applications";
                const params = new URLSearchParams();
                
                if (this.searchQuery) {
                    params.append('search_query', this.searchQuery);
                }
                if (this.selectedOrganizationId) {
                    params.append('organization_id', this.selectedOrganizationId);
                }
                if (this.selectedConfirmations.length > 0) {
                    params.append('confirmation', this.selectedConfirmations[0]);
                }
                if (this.selectedApplicationStatuses.length > 0) {
                    params.append('status', this.selectedApplicationStatuses[0]);
                }
                if (this.dateRangeStart) {
                    params.append('date_from', this.dateRangeStart.toISOString().split('T')[0]);
                }
                if (this.dateRangeEnd) {
                    params.append('date_to', this.dateRangeEnd.toISOString().split('T')[0]);
                }

                const queryString = params.toString();
                if (queryString) {
                    url += '?' + queryString;
                }

                const response = await fetch(url, {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                        "Content-Type": "application/json"
                    },
                });

                if (response.ok) {
                    const data = await response.json();
                    this.applications = data;
                } else {
                    console.error("Ошибка при загрузке заявок:", await response.text());
                }
            } catch (error) {
                console.error("Ошибка сети при загрузке заявок:", error);
            }
        },

        async fetchOrganizations() {
            try {
                const token = localStorage.getItem("token");
                const response = await fetch("http://localhost:8080/organizations", {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                    },
                });

                if (response.ok) {
                    const data = await response.json();
                    this.organizations = data;
                } else {
                    console.error("Ошибка при загрузке организаций");
                }
            } catch (error) {
                console.error("Ошибка сети при загрузке организаций:", error);
            }
        },

        downloadApplication(application) {
            console.log('Скачивание заявки:', application.application_number);
        },

        async openApplication(application) {
            console.log('Открытие заявки:', application.application_number);
            
            if (application.status === 'Непрочитано') {
                try {
                    const token = localStorage.getItem("token");
                    const response = await fetch(`http://localhost:8080/applications/${application.id}`, {
                        method: "PUT",
                        headers: {
                            "Authorization": `Bearer ${token}`,
                            "Content-Type": "application/json"
                        },
                        body: JSON.stringify({
                            status: "В обработке"
                        })
                    });

                    if (response.ok) {
                        application.status = 'В обработке';
                        this.fetchApplications();
                    } else {
                        const errorText = await response.text();
                        console.error("Ошибка при обновлении статуса заявки:", errorText);
                    }
                } catch (error) {
                    console.error("Ошибка сети при обновлении статуса заявки:", error);
                }
            }

            await this.loadApplicationDetails(application);
        },

        async loadApplicationDetails(application) {
            try {
                const token = localStorage.getItem("token");
                
                const appResponse = await fetch(`http://localhost:8080/applications/${application.id}/details`, {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                        "Content-Type": "application/json"
                    },
                });

                if (appResponse.ok) {
                    const appData = await appResponse.json();
                    this.selectedApplication = appData;
                    
                    if (appData.responsible_users) {
                        this.responsibleUsers = appData.responsible_users;
                    }
                }

                const attachmentsResponse = await fetch(`http://localhost:8080/applications/${application.id}/attachments`, {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                        "Content-Type": "application/json"
                    },
                });

                if (attachmentsResponse.ok) {
                    this.applicationAttachments = await attachmentsResponse.json();
                    if (this.applicationAttachments.length > 0) {
                        this.selectedAttachment = this.applicationAttachments[0];
                        await this.loadAttachmentDetails(this.selectedAttachment.id);
                    }
                }

            } catch (error) {
                console.error("Ошибка при загрузке деталей заявки:", error);
            }
        },

        async loadAttachmentDetails(attachmentId) {
            if (!attachmentId) return;

            try {
                const token = localStorage.getItem("token");
                
                this.attachmentCars = [];
                this.attachmentEmployees = [];
                this.attachmentItems = [];

                const attachment = this.applicationAttachments.find(a => a.id === attachmentId);
                if (!attachment) return;

                let carsResponse, employeesResponse, itemsResponse;
                
                switch (attachment.attachment_type) {
                    case 'cars':
                        carsResponse = await fetch(`http://localhost:8080/attachments/${attachmentId}/cars`, {
                            method: "GET",
                            headers: {
                                "Authorization": `Bearer ${token}`,
                            },
                        });
                        if (carsResponse.ok) {
                            this.attachmentCars = await carsResponse.json();
                        }
                        break;
                    
                    case 'people':
                        employeesResponse = await fetch(`http://localhost:8080/attachments/${attachmentId}/employees`, {
                            method: "GET",
                            headers: {
                                "Authorization": `Bearer ${token}`,
                            },
                        });
                        if (employeesResponse.ok) {
                            this.attachmentEmployees = await employeesResponse.json();
                        }
                        break;
                    
                    case 'items':
                        itemsResponse = await fetch(`http://localhost:8080/attachments/${attachmentId}/items`, {
                            method: "GET",
                            headers: {
                                "Authorization": `Bearer ${token}`,
                            },
                        });
                        if (itemsResponse.ok) {
                            this.attachmentItems = await itemsResponse.json();
                        }
                        break;
                }
            } catch (error) {
                console.error("Ошибка при загрузке деталей вложения:", error);
            }
        },

        selectAttachment(attachment) {
            this.selectedAttachment = attachment;
            this.loadAttachmentDetails(attachment.id);
        },

        closeApplicationDetail() {
            this.selectedApplication = null;
            this.selectedAttachment = null;
            this.applicationAttachments = [];
            this.attachmentCars = [];
            this.attachmentEmployees = [];
            this.attachmentItems = [];
            this.responsibleUsers = [];
        },

        async updateConfirmation(confirmation) {
            if (!this.selectedApplication || !this.isResponsibleUser) return;

            this.updatingConfirmation = true;
            try {
                const token = localStorage.getItem("token");
                const response = await fetch(`http://localhost:8080/applications/${this.selectedApplication.id}`, {
                    method: "PUT",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                        "Content-Type": "application/json"
                    },
                    body: JSON.stringify({
                        confirmation: confirmation,
                        responsible_comment: confirmation === 'Согласовано' ? 
                            `Заявка согласована пользователем ${this.currentUserName}` : 
                            `Заявка отклонена пользователем ${this.currentUserName}`,
                        status: confirmation === 'Согласовано' ? 'В работе' : 'Отказано'
                    })
                });

                if (response.ok) {
                    // const result = await response.json();
                    
                    // Мгновенное обновление в интерфейсе
                    this.selectedApplication.confirmation = confirmation;
                    this.selectedApplication.confirmation_datetime = new Date().toISOString();
                    this.selectedApplication.status = confirmation === 'Согласовано' ? 'В работе' : 'Отказано';
                    this.selectedApplication.responsible_name = this.currentUserName;
                    
                    // Обновляем информацию о текущем пользователе как ответственного
                    this.selectedApplication.responsible_comment = confirmation === 'Согласовано' ? 
                        `Заявка согласована пользователем ${this.currentUserName}` : 
                        `Заявка отклонена пользователем ${this.currentUserName}`;
                    
                    // Обновляем статус в основной таблице
                    const appIndex = this.applications.findIndex(app => app.id === this.selectedApplication.id);
                    if (appIndex !== -1) {
                        this.applications[appIndex].confirmation = confirmation;
                        this.applications[appIndex].status = confirmation === 'Согласовано' ? 'В работе' : 'Отказано';
                    }
                    
                    // Показываем уведомление
                    const message = confirmation === 'Согласовано' ? 
                        'Заявка успешно согласована!' : 
                        'Заявка отклонена!';
                    alert(message);
                } else {
                    const errorText = await response.text();
                    console.error("Ошибка при обновлении подтверждения:", errorText);
                    alert(`Ошибка: ${errorText}`);
                }
            } catch (error) {
                console.error("Ошибка сети при обновлении подтверждения:", error);
                alert("Ошибка сети при обновлении статуса");
            } finally {
                this.updatingConfirmation = false;
            }
        },

        async getCurrentUser() {
            try {
                const token = localStorage.getItem("token");
                const response = await fetch("http://localhost:8080/users/me", {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                    },
                });

                if (response.ok) {
                    const userData = await response.json();
                    this.currentUserId = userData.id;
                    this.currentUserName = `${userData.last_name} ${userData.first_name}`;
                    console.log('Текущий пользователь:', this.currentUserName, 'ID:', this.currentUserId);
                } else {
                    console.error("Ошибка при получении текущего пользователя:", await response.text());
                }
            } catch (error) {
                console.error("Ошибка сети при получении текущего пользователя:", error);
            }
        },

        startShakeAnimation() {
            this.shakeInterval = setInterval(() => {
                if (this.unreadCount > 0) {
                    this.shouldShake = true;
                    setTimeout(() => {
                        this.shouldShake = false;
                    }, 600);
                }
            }, 60000);
        }
    },
    mounted() {
        document.addEventListener('click', (e) => {
            if (!this.$el.contains(e.target)) {
                this.showOrganizationDropdown = false;
                this.showDatePicker = false;
            }
        });

        this.startShakeAnimation();
        
        this.fetchOrganizations();
        this.fetchApplications();
        this.getCurrentUser();
        
        setTimeout(() => {
            this.isInitialLoad = false;
        }, 1000);
    },
    beforeUnmount() {
        if (this.shakeInterval) {
            clearInterval(this.shakeInterval);
        }
    }
}
</script>

<style scoped>
/* Остальные стили остаются такими же, изменяем только стили для application-detail */

/* Детальное представление заявки */
.application-detail-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 1000;
}

.application-detail {
    background: white;
    border-radius: 30px;
    width: 1400px;
    max-width: 90%;
    height: 80vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
    overflow: hidden;
}

.detail-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 15px;
    border-bottom: 1px solid #e6e6e6;
    background: #fafafa;
    min-height: 80px;
}

.detail-header-left {
    display: flex;
    flex-direction: column;
    gap: 5px;
    flex: 1;
}

.detail-title-row {
    display: flex;
    align-items: center;
    gap: 20px;
    margin-bottom: 5px;
}

.detail-title {
    font-size: 20px;
    font-weight: 700;
    color: #000;
    margin: 0;
    line-height: 1.2;
}

.detail-datetime {
    font-size: 16px;
    color: #a2a2a2;
    line-height: 1.2;
    font-weight: 500;
}

.detail-header-right {
    display: flex;
    align-items: center;
    gap: 15px;
}

.confirmation-buttons {
    display: flex;
    gap: 10px;
}

.confirm-btn, .reject-btn {
    padding: 10px 24px;
    border: none;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
    min-width: 120px;
}

.confirm-btn {
    background: #57c785;
    color: white;
}

.confirm-btn:hover:not(:disabled) {
    background: #45b371;
    transform: translateY(-1px);
    box-shadow: 0 4px 8px rgba(87, 199, 133, 0.3);
}

.reject-btn {
    background: #FF6668;
    color: white;
}

.reject-btn:hover:not(:disabled) {
    background: #ff4d4f;
    transform: translateY(-1px);
    box-shadow: 0 4px 8px rgba(255, 102, 104, 0.3);
}

.confirm-btn:disabled,
.reject-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
    transform: none;
    box-shadow: none;
}

.close-detail-btn {
    background: none;
    border: none;
    font-size: 24px;
    color: #a2a2a2;
    cursor: pointer;
    width: 40px;
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    transition: all 0.2s ease;
}

.close-detail-btn:hover {
    background: #f0f0f0;
    color: #333;
}

.detail-content {
    display: flex;
    flex: 1;
    overflow: hidden;
}

.detail-left-column {
    width: 240px;
    border-right: 1px solid #e6e6e6;
    overflow-y: auto;
    background: #fafafa;
    padding: 20px;
}

.detail-main-column {
    flex: 1;
    padding: 15px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 10px;
}

.detail-right-column {
    width: 320px;
    border-left: 1px solid #e6e6e6;
    overflow-y: auto;
    padding: 25px;
    background: #fafafa;
}

.message-section {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    padding: 15px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.message-section h4 {
    font-size: 14px;
    color: #a2a2a2;
    padding-bottom: 5px;
    font-weight: 400;
}

.message-content {
    font-size: 16px;
    line-height: 1.6;
    color: #333;
    white-space: pre-wrap;
}

.attachment-details {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    padding: 15px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.attachment-details h4 {
    font-size: 18px;
    color: #4F5BDF;
    padding-bottom: 20px;
    font-weight: 700;
}

.date-range, .time-range {
    display: flex;
    flex-direction: column;
    gap: 5px;
    margin-bottom: 12px;
    font-size: 14px;
}

.date-label, .time-label {
    color: #a2a2a2;
    font-weight: 400;
    min-width: 110px;
    font-size: 14px;
}

.date-value, .time-value {
    color: #000;
    font-weight: 400;
    font-size: 16px;
}

.attachment-data {
    margin-top: 25px;
}

.attachment-data h5 {
    font-size: 16px;
    color: #333;
    margin: 0 0 15px 0;
    font-weight: 700;
    padding-bottom: 10px;
    border-bottom: 2px solid #4F5BDF;
}

.cars-list, .employees-list, .items-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
}

.car-item, .employee-item, .item-item {
    padding: 10px;
    background: #f9f9f9;
    border-radius: 15px;
    border: 1px solid #e6e6e6;
    transition: all 0.2s ease;
}

.car-item:hover, .employee-item:hover, .item-item:hover {
    border-color: #4F5BDF;
    background: #f8f9ff;
}

.car-info, .employee-info {
    display: flex;
    align-items: center;
    gap: 15px;
    padding-bottom: 6px;
    flex-wrap: wrap;
}

.car-number, .employee-name {
    font-weight: 600;
    color: #333;
    font-size: 15px;
}

.car-brand, .car-unload-place, .employee-position {
    color: #666;
    font-size: 14px;
}

.unload-places, .target-tables {
    display: flex;
    gap: 8px;
    font-size: 13px;
    align-items: flex-start;
    padding-top: 6px;
    border-top: 1px dashed #e6e6e6;
}

.places-label, .tables-label {
    color: #666;
    font-weight: 600;
    min-width: 140px;
}

.places-list, .tables-list {
    color: #333;
    flex: 1;
    line-height: 1.4;
}

.item-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.item-name {
    font-weight: 600;
    color: #333;
    font-size: 15px;
}

.item-count {
    color: #4F5BDF;
    font-size: 14px;
    font-weight: 600;
    background: rgba(79, 91, 223, 0.1);
    padding: 4px 10px;
    border-radius: 6px;
}

/* Основная информация */
.basic-info-section,
.confirmation-section,
.comment-section {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    padding: 25px;
    margin-bottom: 20px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.basic-info-section h4,
.confirmation-section h4,
.comment-section h4 {
    font-size: 18px;
    color: #4F5BDF;
    margin: 0 0 20px 0;
    font-weight: 700;
    padding-bottom: 10px;
    border-bottom: 2px solid #4F5BDF;
}

.confirmation-section h5 {
    font-size: 16px;
    color: #666;
    margin: 25px 0 15px 0;
    font-weight: 700;
    padding-bottom: 8px;
    border-bottom: 1px solid #e6e6e6;
}

.info-grid {
    display: flex;
    flex-direction: column;
    gap: 15px;
}

.info-row {
    display: flex;
    align-items: flex-start;
    gap: 15px;
    padding: 12px 0;
    border-bottom: 1px solid #f0f0f0;
}

.info-row:last-child {
    border-bottom: none;
}

.info-label {
    color: #666;
    font-size: 14px;
    font-weight: 600;
    min-width: 140px;
    text-align: left;
}

.info-value {
    color: #333;
    font-size: 14px;
    text-align: left;
    flex: 1;
    font-weight: 500;
    line-height: 1.5;
}

.users-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
}

.user-item {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: 12px;
    background: #f9f9f9;
    border-radius: 10px;
    border: 1px solid #e6e6e6;
    transition: all 0.2s ease;
}

.user-item:hover {
    border-color: #4F5BDF;
    background: #f8f9ff;
}

.user-info {
    display: flex;
    flex-direction: column;
    gap: 4px;
    flex: 1;
}

.user-name {
    font-weight: 600;
    color: #333;
    font-size: 14px;
}

.user-position {
    color: #666;
    font-size: 12px;
    font-style: italic;
}

.primary-badge {
    background: #4F5BDF;
    color: white;
    padding: 4px 10px;
    border-radius: 12px;
    font-size: 11px;
    font-weight: 600;
    margin-left: 10px;
    white-space: nowrap;
}

.comment-content {
    font-size: 14px;
    line-height: 1.6;
    color: #333;
    white-space: pre-wrap;
    padding: 15px;
    background: #f9f9f9;
    border-radius: 10px;
    border: 1px solid #e6e6e6;
    margin-top: 10px;
}

/* Стили для статусов согласования */
.confirmation-approved {
    color: #059669;
    background: #f0f9ff;
    padding: 4px 10px;
    border-radius: 6px;
    font-weight: 600;
}

.confirmation-pending {
    color: #d97706;
    background: #fffbeb;
    padding: 4px 10px;
    border-radius: 6px;
    font-weight: 600;
}

.confirmation-rejected {
    color: #dc2626;
    background: #fef2f2;
    padding: 4px 10px;
    border-radius: 6px;
    font-weight: 600;
}

/* Остальные стили остаются без изменений */
.center {
    padding: 20px;
    position: relative;
}

.center__header {
    padding-bottom: 15px;
}

.header-top {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 8px;
}

.center__title {
    font-size: 18px;
    font-weight: bold;
    color: #000;
    margin: 0;
}

.unread-badge {
    background: #4F5BDF;
    color: white;
    padding: 4px 10px;
    border-radius: 12px;
    font-size: 12px;
    font-weight: 600;
    transition: transform 0.3s ease;
}

.shake-animation {
    animation: shake 0.6s ease-in-out;
}

@keyframes shake {
    0%, 100% { transform: translateX(0); }
    10%, 30%, 50%, 70%, 90% { transform: translateX(-3px); }
    20%, 40%, 60%, 80% { transform: translateX(3px); }
}

.center__filters {
    padding-bottom: 15px;
    border-bottom: 1px solid #e6e6e6;
}

.filters__main {
    flex: 1;
}

.filters-row {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 12px;
}

.filters-row--secondary {
    align-items: flex-start;
    gap: 20px;
}

.filter-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.filter-section--refresh {
    margin-left: auto;
    margin-top: auto;
}

.filter-section__header {
    display: flex;
    align-items: center;
}

.filter-label {
    font-size: 12px;
    color: #666;
    font-weight: 500;
    white-space: nowrap;
}

.field {
    width: 200px;
    height: 35px;
    background-color: #FFF;
    border-radius: 10px;
    border: 1px solid #e6e6e6;
    padding: 0 10px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    position: relative;
}

.field--select {
    cursor: pointer;
}

.date-field {
    cursor: pointer;
}

.search {
    cursor: text;
}

.field__input {
    outline: none;
    border: none;
    background-color: transparent;
    font-size: 14px;
    width: 150px;
}

.field--select .field__input,
.date-field .field__input {
    cursor: pointer;
}

.select-text {
    font-size: 14px;
    color: #000;
    flex: 1;
}

.center__icon {
    width: 15px;
    height: 15px;
}

.select-icon {
    width: 10px;
    height: 10px;
    transition: transform 0.5s ease;
    transform: rotate(90deg);
}

.select-icon--rotated {
    transform: rotate(-90deg);
}

.status-buttons {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
}

.status-btn {
    padding: 6px 12px;
    border: 1px solid #e6e6e6;
    background: white;
    border-radius: 15px;
    cursor: pointer;
    font-size: 12px;
    transition: all 0.2s;
    height: 30px;
    color: #333;
    white-space: nowrap;
}

.status-btn:hover:not(.status-btn--active) {
    background: #f5f5f5;
}

.status-btn--active {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.status-btn--active:hover {
    background: #3a45c0;
    border-color: #3a45c0;
}

.more-filters-btn {
    padding: 6px 12px;
    border: 1px solid #e6e6e6;
    background: white;
    border-radius: 15px;
    cursor: pointer;
    font-size: 12px;
    transition: all 0.2s;
    height: 30px;
    color: #333;
    display: flex;
    align-items: center;
    gap: 6px;
}

.more-filters-btn:hover:not(.more-filters-btn--active) {
    background: #f5f5f5;
}

.more-filters-btn--active {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.more-filters-btn--active:hover {
    background: #3a45c0;
    border-color: #3a45c0;
}

.more-filters-icon {
    width: 10px;
    height: 10px;
    transition: transform 0.3s ease;
}

.more-filters-icon--rotated {
    transform: rotate(-90deg);
}

.reset-sort-btn,
.reset-filters-btn {
    padding: 6px 12px;
    border: 1px solid #e6e6e6;
    background: white;
    border-radius: 15px;
    cursor: pointer;
    font-size: 12px;
    transition: all 0.2s;
    height: 30px;
    color: #333;
    white-space: nowrap;
}

.reset-sort-btn:hover:not(:disabled),
.reset-filters-btn:hover:not(:disabled) {
    background: #f5f5f5;
}

.reset-sort-btn:disabled,
.reset-filters-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.reset-filters-btn {
    background: #fff5f5;
    border-color: #fed7d7;
    color: #c53030;
}

.reset-filters-btn:hover:not(:disabled) {
    background: #fed7d7;
}

.date-picker {
    position: absolute;
    top: calc(100% + 5px);
    left: 0;
    width: 420px;
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    padding: 12px;
    z-index: 1001;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
    transform-origin: top left;
}

.date-picker-enter-active {
    animation: scaleIn 0.2s ease-out;
}

@keyframes scaleIn {
    from {
        opacity: 0;
        transform: scale(0.95);
    }
    to {
        opacity: 1;
        transform: scale(1);
    }
}

.date-picker__header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
}

.date-picker__header h4 {
    margin: 0;
    font-size: 14px;
    color: #333;
}

.date-picker__close {
    background: none;
    border: none;
    font-size: 18px;
    cursor: pointer;
    color: #a2a2a2;
    padding: 0;
    width: 20px;
    height: 20px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.date-picker__close:hover {
    color: #333;
}

.date-picker__quick-buttons {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 6px;
    margin-bottom: 12px;
}

.quick-btn {
    padding: 4px 6px;
    border: 1px solid #e6e6e6;
    background: white;
    border-radius: 6px;
    cursor: pointer;
    font-size: 11px;
    transition: background-color 0.2s;
    color: #333;
    height: 24px;
}

.quick-btn:hover {
    background: #f5f5f5;
}

.date-picker__range {
    display: flex;
    gap: 8px;
    margin-bottom: 12px;
}

.date-input-group {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.date-input-group label {
    font-size: 11px;
    color: #a2a2a2;
}

.date-input {
    padding: 4px 6px;
    border: 1px solid #e6e6e6;
    border-radius: 6px;
    font-size: 11px;
    outline: none;
    height: 24px;
}

.date-input:focus {
    border-color: #4F5BDF;
}

.date-picker__actions {
    display: flex;
    gap: 6px;
}

.apply-btn, .clear-btn {
    flex: 1;
    padding: 6px;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    font-size: 11px;
    transition: background-color 0.2s;
    height: 26px;
}

.apply-btn {
    background: #4F5BDF;
    color: white;
}

.apply-btn:hover {
    background: #3a45c0;
}

.clear-btn {
    background: #f5f5f5;
    color: #333;
}

.clear-btn:hover {
    background: #e5e5e5;
}

.custom-dropdown {
    position: absolute;
    top: calc(100% + 5px);
    left: 0;
    width: 100%;
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    max-height: 300px;
    overflow-y: auto;
    z-index: 1001;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
    transform-origin: top left;
}

.dropdown-enter-active {
    animation: scaleIn 0.2s ease-out;
}

.dropdown-search {
    padding: 10px;
    border-bottom: 1px solid #f0f0f0;
    position: sticky;
    top: 0;
    background: white;
    z-index: 1002;
}

.dropdown-search__input {
    width: 100%;
    padding: 8px 12px;
    border: 1px solid #e6e6e6;
    border-radius: 5px;
    font-size: 14px;
    outline: none;
}

.dropdown-search__input:focus {
    border-color: #4F5BDF;
}

.dropdown-item {
    padding: 10px 15px;
    cursor: pointer;
    font-size: 14px;
    border-bottom: 1px solid #f0f0f0;
    transition: background-color 0.2s ease;
}

.dropdown-item:last-child {
    border-bottom: none;
}

.dropdown-item:hover {
    background-color: #f5f5f5;
}

.dropdown-item--selected {
    background-color: #4F5BDF;
    color: white;
}

.dropdown-item--selected:hover {
    background-color: #3a45c4;
}

.dropdown-no-results {
    padding: 15px;
    text-align: center;
    color: #999;
    font-size: 14px;
    font-style: italic;
}

.applications-table {
    background-color: #fff;
    border-radius: 30px;
    border: 1px solid #e6e6e6;
    overflow: hidden;
    margin-top: 20px;
    height: 500px;
    display: flex;
    flex-direction: column;
    transition: all 0.3s ease;
}

.applications-table.with-details {
    height: 500px;
}

.table-header {
    border-bottom: 1px solid #e6e6e6;
    padding: 0 16px;
    flex-shrink: 0;
    height: 45px;
}

.header-row {
    display: flex;
    width: 100%;
    align-items: center;
    height: 100%;
}

.header-col {
    font-weight: 500;
    color: #a2a2a2;
    text-align: left;
    font-size: 14px;
    display: flex;
    align-items: center;
    gap: 5px;
    transition: .2s;
    cursor: pointer;
    user-select: none;
    height: 100%;
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

.confirmation-col {
    width: 13%;
    min-width: 130px;
}

.number-col {
    width: 12%;
    min-width: 130px;
}

.date-col {
    width: 12%;
    min-width: 140px;
}

.organization-col {
    width: 17%;
    min-width: 130px;
}

.sender-col {
    width: 16%;
    min-width: 130px;
    position: relative;
}

.sender-col:hover::after {
    content: attr(title);
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    background: #333;
    color: white;
    padding: 5px 10px;
    border-radius: 4px;
    font-size: 12px;
    white-space: nowrap;
    z-index: 1000;
    pointer-events: none;
    opacity: 0;
    transition: opacity 0.2s;
}

.sender-col:hover::after {
    opacity: 1;
}

.status-col {
    width: 16%;
    min-width: 110px;
}

.actions-col {
    width: 4%;
    min-width: 60px;
    justify-content: center;
}

.table-body {
    flex-grow: 1;
    overflow-y: auto;
}

.applications-list {
    overflow-y: auto;
    flex-grow: 1;
}

.application-item {
    transition: background-color 0.2s ease;
    cursor: pointer;
}

.application-item.initial-load {
    animation: slideInFromTop 0.4s ease-out forwards;
    opacity: 0;
    transform: translateY(-20px);
}

.application-item.filtered {
    animation: none;
    opacity: 1;
    transform: none;
}

@keyframes slideInFromTop {
    from {
        opacity: 0;
        transform: translateY(-20px);
    }
    to {
        opacity: 1;
        transform: translateY(0);
    }
}

.application-item:hover {
    background-color: #fafafa;
}

.application-item.unread {
    background-color: #fcf7e8;
}

.application-row {
    display: flex;
    width: 100%;
    padding: 6px 16px;
    align-items: center;
    border-bottom: 1px solid #f0f0f0;
    height: 40px;
}

.application-col {
    text-align: left;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 14px;
    display: flex;
    align-items: center;
    height: 100%;
}

.confirmation-badge {
    padding: 4px 8px;
    border-radius: 12px;
    font-size: 11px;
    font-weight: 500;
    display: inline-block;
}

.confirmation-approved {
    background-color: #f0f9ff;
    color: #059669;
    border: 1px solid #a7f3d0;
}

.confirmation-pending {
    background-color: #fffbeb;
    color: #d97706;
    border: 1px solid #fcd34d;
}

.confirmation-rejected {
    background-color: #fef2f2;
    color: #dc2626;
    border: 1px solid #fecaca;
}

.confirmation-default {
    background-color: #f5f5f5;
    color: #616161;
    border: 1px solid #e0e0e0;
}

.application-number {
    color: #a2a2a2;
}

.status-badge {
    padding: 4px 8px;
    border-radius: 8px;
    font-size: 11px;
    font-weight: 500;
    display: inline-block;
    border: 1px solid;
}

.status-unread {
    background-color: #fff7ed;
    color: #ea580c;
    border-color: #fed7aa;
}

.status-processing {
    background-color: #fff3e0;
    color: #ef6c00;
    border-color: #ffe0b2;
}

.status-in-progress {
    background-color: #e3f2fd;
    color: #1565c0;
    border-color: #bbdefb;
}

.status-completed {
    background-color: #e8f5e8;
    color: #2e7d32;
    border-color: #c8e6c9;
}

.status-rejected {
    background-color: #ffebee;
    color: #c62828;
    border-color: #ffcdd2;
}

.status-default {
    background-color: #f5f5f5;
    color: #616161;
    border-color: #e0e0e0;
}

.download-btn {
    background: none;
    border: none;
    cursor: pointer;
    padding: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
    transition: background-color 0.2s ease;
}

.download-btn:hover {
    background-color: #f5f5f5;
}

.download-icon {
    width: 16px;
    height: 16px;
    opacity: 0.7;
    transition: opacity 0.2s ease;
}

.download-btn:hover .download-icon {
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

.table-body::-webkit-scrollbar {
    width: 6px;
}

.table-body::-webkit-scrollbar-track {
    background: transparent;
    margin: 2px 0;
    border-radius: 3px;
}

.table-body::-webkit-scrollbar-thumb {
    background: #D9E2FF;
    border-radius: 3px;
    border: 1px solid transparent;
    background-clip: content-box;
    transition: all 0.3s ease;
}

.table-body::-webkit-scrollbar-thumb:hover {
    background: #C5D1FF;
    border: 1px solid transparent;
    background-clip: content-box;
}

.table-body {
    scrollbar-width: thin;
    scrollbar-color: #D9E2FF transparent;
}

.applications-list::-webkit-scrollbar {
    width: 6px;
}

.applications-list::-webkit-scrollbar-track {
    background: transparent;
    margin: 2px 0;
    border-radius: 3px;
}

.applications-list::-webkit-scrollbar-thumb {
    background: #D9E2FF;
    border-radius: 3px;
    border: 1px solid transparent;
    background-clip: content-box;
}

.applications-list {
    scrollbar-width: thin;
    scrollbar-color: #D9E2FF transparent;
}
</style>