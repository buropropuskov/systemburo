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
                        <span class="select-text">{{ selectedOrganization || 'Организация' }}</span>
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
                            <div class="dropdown-item" @click.stop="selectOrganization('')">Все организации</div>
                            <div 
                                class="dropdown-item" 
                                v-for="org in filteredOrganizations" 
                                :key="org" 
                                @click.stop="selectOrganization(org)"
                                :class="{ 'dropdown-item--selected': org === selectedOrganization }"
                            >
                                {{ org }}
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
                        <RefreshButton @refresh="refreshData" />
                    </div>
                </div>

                <!-- Дополнительные фильтры -->
                <transition name="more-filters">
                    <div class="more-filters" v-if="showMoreFilters">
                        <div class="filter-section">
                            <div class="filter-section__header">
                                <span class="filter-label">Теги</span>
                            </div>
                            <div class="tags-buttons">
                                <button 
                                    v-for="tag in availableTags" 
                                    :key="tag"
                                    class="tag-btn"
                                    :class="{ 'tag-btn--active': selectedTags.includes(tag) }"
                                    @click="toggleTag(tag)"
                                >
                                    {{ tag }}
                                </button>
                            </div>
                        </div>
                    </div>
                </transition>
            </div>
        </div>

        <!-- Таблица заявок -->
        <div class="applications-table">
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
                    <div class="header-col tags-col">
                        <p>Теги</p>
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
                            'unread': application.unread,
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
                                    {{ getConfirmationText(application.confirmation) }}
                                </span>
                            </div>
                            <div class="application-col number-col">
                                <span class="application-number">{{ application.number }}</span>
                            </div>
                            <div class="application-col date-col">
                                {{ application.date }}
                            </div>
                            <div class="application-col organization-col">
                                {{ application.organization }}
                            </div>
                            <div class="application-col sender-col">
                                {{ application.sender }}
                            </div>
                            <div class="application-col tags-col">
                                <div class="tags-container">
                                    <span 
                                        v-for="tag in application.tags" 
                                        :key="tag" 
                                        class="tag-badge"
                                        :class="getTagClass(tag)"
                                    >
                                        {{ tag }}
                                    </span>
                                </div>
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
    </section>
</template>

<script>
import RefreshButton from '../components/RefreshButton.vue'

export default {
    components: {
        RefreshButton
    },
    data() {
        return {
            searchQuery: '',
            selectedOrganization: '',
            selectedConfirmations: [],
            selectedApplicationStatuses: [],
            selectedTags: [],
            showMoreFilters: false,
            organizations: [
                'р-н Мегобари',
                'ООО "Ромашка"',
                'ИП Иванов',
                'ЗАО "Весна"',
                'ОАО "Технопром"',
                'ТОО "Стройсервис"',
                'ООО "Нефтегаз"',
                'ИП Петров',
                'ЗАО "Металлург"',
                'ОАО "Строймаш"'
            ],
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
                { value: 'approved', label: 'Согласовано' },
                { value: 'rejected', label: 'Не согласовано' },
                { value: 'pending', label: 'На согласовании' }
            ],
            applicationStatuses: [
                { value: 'Непрочитано', label: 'Непрочитано' },
                { value: 'В обработке', label: 'В обработке' },
                { value: 'В работе', label: 'В работе' },
                { value: 'Завершено', label: 'Завершено' },
                { value: 'Отказано', label: 'Отказано' }
            ],
            
            // Данные заявок
            applications: [
    {
        id: 1,
        confirmation: 'approved',
        number: '№ 20250609/001',
        date: '09.06.2025 21:07',
        organization: 'р-н Мегобари',
        sender: 'Мякотных С.М.',
        tags: ['Крыша'],
        status: 'В работе',
        unread: false
    },
    {
        id: 2,
        confirmation: 'pending',
        number: '№ 20250609/002',
        date: '09.06.2025 18:30',
        organization: 'р-н Мегобари',
        sender: 'Мякотных С.М.',
        tags: [],
        status: 'В обработке',
        unread: false
    },
    {
        id: 3,
        confirmation: 'rejected',
        number: '№ 20250609/003',
        date: '09.06.2025 15:45',
        organization: 'р-н Мегобари',
        sender: 'Мякотных С.М.',
        tags: [],
        status: 'Отказано',
        unread: false
    },
    {
        id: 4,
        confirmation: 'approved',
        number: '№ 20250610/001',
        date: '10.06.2025 14:20',
        organization: 'ООО "Ромашка"',
        sender: 'Иванов А.П.',
        tags: ['Срочно', 'VIP'],
        status: 'Завершено',
        unread: false
    },
    {
        id: 5,
        confirmation: 'pending',
        number: '№ 20250610/002',
        date: '10.06.2025 12:15',
        organization: 'ИП Иванов',
        sender: 'Петров В.С.',
        tags: ['Ночная'],
        status: 'В обработке',
        unread: false
    },
    {
        id: 6,
        confirmation: 'approved',
        number: '№ 20250611/001',
        date: '11.06.2025 09:30',
        organization: 'ЗАО "Весна"',
        sender: 'Сидорова М.К.',
        tags: ['Склад'],
        status: 'В работе',
        unread: false
    },
    {
        id: 7,
        confirmation: 'rejected',
        number: '№ 20250611/002',
        date: '11.06.2025 08:00',
        organization: 'ОАО "Технопром"',
        sender: 'Козлов Д.В.',
        tags: [],
        status: 'Отказано',
        unread: false
    },
    {
        id: 8,
        confirmation: 'pending',
        number: '№ 20250612/001',
        date: '12.06.2025 16:45',
        organization: 'ТОО "Стройсервис"',
        sender: 'Николаев П.С.',
        tags: ['Важно'],
        status: 'Непрочитано',
        unread: true
    },
    {
        id: 9,
        confirmation: 'approved',
        number: '№ 20250612/002',
        date: '12.06.2025 11:20',
        organization: 'ООО "Нефтегаз"',
        sender: 'Волков А.А.',
        tags: ['Терминал'],
        status: 'Завершено',
        unread: false
    },
    {
        id: 10,
        confirmation: 'pending',
        number: '№ 20250613/001',
        date: '13.06.2025 22:10',
        organization: 'ИП Петров',
        sender: 'Семенов К.Д.',
        tags: ['Выезд'],
        status: 'Непрочитано',
        unread: true
    }
            ]
        }
    },
    computed: {
        filteredOrganizations() {
            if (!this.organizationSearch) return this.organizations;
            const searchTerm = this.organizationSearch.toLowerCase();
            return this.organizations.filter(org => 
                org.toLowerCase().includes(searchTerm)
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
        
        availableTags() {
            const allTags = this.applications.flatMap(app => app.tags);
            return [...new Set(allTags)].sort();
        },
        
        filteredApplications() {
            let filtered = this.applications;

            // Фильтр по поиску
            if (this.searchQuery.trim()) {
                const query = this.normalizeSearch(this.searchQuery.trim());
                filtered = filtered.filter(app => {
                    const searchFields = [
                        app.number,
                        app.organization,
                        app.sender,
                        app.status,
                        ...app.tags
                    ];
                    
                    return searchFields.some(field => {
                        const normalizedField = this.normalizeSearch(field);
                        const searchWords = query.split(' ').filter(word => word.length > 0);
                        return searchWords.every(word => normalizedField.includes(word));
                    });
                });
            }

            // Фильтр по организации
            if (this.selectedOrganization) {
                filtered = filtered.filter(app => 
                    app.organization === this.selectedOrganization
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

            // Фильтр по тегам
            if (this.selectedTags.length > 0) {
                filtered = filtered.filter(app => 
                    this.selectedTags.some(tag => app.tags.includes(tag))
                );
            }

            // Фильтр по дате
            if (this.dateRangeStart && this.dateRangeEnd) {
                filtered = filtered.filter(app => {
                    const appDate = this.parseDateTime(app.date);
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
                    const dateA = this.parseDateTime(a.date);
                    const dateB = this.parseDateTime(b.date);
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
                        valueA = a.number;
                        valueB = b.number;
                        break;
                    case 'date':
                        valueA = this.parseDateTime(a.date);
                        valueB = this.parseDateTime(b.date);
                        break;
                    case 'organization':
                        valueA = a.organization?.toLowerCase() || '';
                        valueB = b.organization?.toLowerCase() || '';
                        break;
                    case 'sender':
                        valueA = a.sender?.toLowerCase() || '';
                        valueB = b.sender?.toLowerCase() || '';
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
                   !!this.selectedOrganization || 
                   this.selectedConfirmations.length > 0 || 
                   this.selectedApplicationStatuses.length > 0 ||
                   this.selectedTags.length > 0 ||
                   (this.dateRangeStart && this.dateRangeEnd);
        },

        unreadCount() {
            return this.applications.filter(app => app.unread).length;
        }
    },
    methods: {
        // Нормализация поискового запроса
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
        
        // Переключение дополнительных фильтров
        toggleMoreFilters() {
            this.showMoreFilters = !this.showMoreFilters;
        },
        
        // Dropdown методы
        toggleDropdown(type) {
            if (type === 'organization') {
                this.showOrganizationDropdown = !this.showOrganizationDropdown;
                this.showDatePicker = false;
                if (this.showOrganizationDropdown) {
                    this.organizationSearch = '';
                }
            }
        },
        
        selectOrganization(org) {
            this.selectedOrganization = org;
            this.showOrganizationDropdown = false;
            this.organizationSearch = '';
            this.applyFilters();
        },
        
        // Фильтры подтверждения
        toggleConfirmation(status) {
            const index = this.selectedConfirmations.indexOf(status);
            if (index > -1) {
                this.selectedConfirmations.splice(index, 1);
            } else {
                this.selectedConfirmations.push(status);
            }
            this.applyFilters();
        },
        
        // Фильтры статуса заявки
        toggleApplicationStatus(status) {
            const index = this.selectedApplicationStatuses.indexOf(status);
            if (index > -1) {
                this.selectedApplicationStatuses.splice(index, 1);
            } else {
                this.selectedApplicationStatuses.push(status);
            }
            this.applyFilters();
        },
        
        // Фильтры тегов
        toggleTag(tag) {
            const index = this.selectedTags.indexOf(tag);
            if (index > -1) {
                this.selectedTags.splice(index, 1);
            } else {
                this.selectedTags.push(tag);
            }
            this.applyFilters();
        },
        
        // Сброс фильтров
        resetFilters() {
            this.selectedOrganization = '';
            this.selectedConfirmations = [];
            this.selectedApplicationStatuses = [];
            this.selectedTags = [];
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            this.dateRangeStartInput = '';
            this.dateRangeEndInput = '';
            this.isInitialLoad = false;
        },
        
        // Дата методы
        toggleDatePicker() {
            this.showDatePicker = !this.showDatePicker;
            this.showOrganizationDropdown = false;
        },
        
        closeDatePicker() {
            this.showDatePicker = false;
        },
        
        formatDate(date) {
            if (!date) return '';
            return date.toLocaleDateString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric'
            });
        },
        
        parseDateTime(dateTimeString) {
            const [datePart, timePart] = dateTimeString.split(' ');
            const [day, month, year] = datePart.split('.');
            const [hours, minutes] = timePart.split(':');
            return new Date(year, month - 1, day, hours, minutes);
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
        
        getConfirmationText(confirmation) {
            const texts = {
                'approved': 'Согласовано',
                'pending': 'Согласование',
                'rejected': 'Не согласовано'
            };
            return texts[confirmation] || '';
        },

        getConfirmationClass(confirmation) {
            return `confirmation-${confirmation}`;
        },

        getTagClass(tag) {
            const tagColors = {
                'Крыша': 'tag-roof',
                'Срочно': 'tag-urgent',
                'VIP': 'tag-vip',
                'Ночная': 'tag-night',
                'Склад': 'tag-warehouse',
                'Важно': 'tag-important',
                'Терминал': 'tag-terminal',
                'Выезд': 'tag-departure'
            };
            return tagColors[tag] || 'tag-default';
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
        
        // Сортировка
        sortBy(field) {
            if (this.sortField === field) {
                this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
            } else {
                this.sortField = field;
                this.sortDirection = 'desc';
            }
            this.isInitialLoad = false;
        },

        // Сброс сортировки
        resetSort() {
            this.sortField = null;
            this.sortDirection = 'desc';
        },
        
        // Фильтры
        applyFilters() {
            this.isInitialLoad = false;
        },
        
        // Обновление данных
        refreshData() {
            console.log('Обновление данных заявок...');
            this.isInitialLoad = false;
        },

        // Скачивание
        downloadApplication(application) {
            console.log('Скачивание заявки:', application.number);
        },

        // Открытие заявки
        openApplication(application) {
            console.log('Открытие заявки:', application.number);
            if (application.status === 'Непрочитано') {
                application.status = 'В обработке';
                application.unread = false;
            }
        },

        // Анимация тряски для бейджа
        startShakeAnimation() {
            this.shakeInterval = setInterval(() => {
                if (this.unreadCount > 0) {
                    this.shouldShake = true;
                    setTimeout(() => {
                        this.shouldShake = false;
                    }, 600);
                }
            }, 60000); // 60 секунд
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
        
        // Отключаем анимацию начальной загрузки через 1 секунду после монтирования
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
.center {
    padding: 20px;
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

/* Status Buttons */
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

/* More Filters Button */
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

/* More Filters Panel */
.more-filters-enter-active,
.more-filters-leave-active {
    transition: all 0.3s ease;
    overflow: hidden;
}

.more-filters-enter-from,
.more-filters-leave-to {
    opacity: 0;
    max-height: 0;
    transform: translateY(-10px);
}

.more-filters-enter-to,
.more-filters-leave-from {
    opacity: 1;
    max-height: 200px;
    transform: translateY(0);
}

.more-filters {
    padding: 10px 0;
}

.tags-buttons {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
}

.tag-btn {
    padding: 4px 8px;
    border: 1px solid #e6e6e6;
    background: white;
    border-radius: 12px;
    cursor: pointer;
    font-size: 11px;
    transition: all 0.2s;
    color: #333;
}

.tag-btn:hover:not(.tag-btn--active) {
    background: #f5f5f5;
}

.tag-btn--active {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.tag-btn--active:hover {
    background: #3a45c0;
    border-color: #3a45c0;
}

/* Reset Buttons */
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

/* Date Picker */
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

/* Custom Dropdown */
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

/* Таблица */
.applications-table {
    background-color: #fff;
    border-radius: 30px;
    border: 1px solid #e6e6e6;
    overflow: hidden;
    margin-top: 20px;
    height: 500px;
    display: flex;
    flex-direction: column;
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

/* Ширины колонок */
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
    width: 15%;
    min-width: 130px;
}

.sender-col {
    width: 14%;
    min-width: 130px;
}

.tags-col {
    width: 14%;
    min-width: 110px;
}

.status-col {
    width: 14%;
    min-width: 110px;
}

.actions-col {
    width: 4%;
    min-width: 60px;
    justify-content: center;
}

/* Тело таблицы */
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

/* Бейджи подтверждения */
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

/* Номер заявки */
.application-number {
    color: #a2a2a2;
}

/* Теги */
.tags-container {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
}

.tag-badge {
    padding: 2px 6px;
    border-radius: 8px;
    font-size: 10px;
    font-weight: 500;
    border: 1px solid;
}

.tag-roof {
    background-color: #e0f2fe;
    color: #0369a1;
    border-color: #bae6fd;
}

.tag-urgent {
    background-color: #fef2f2;
    color: #dc2626;
    border-color: #fecaca;
}

.tag-vip {
    background-color: #fef7cd;
    color: #854d0e;
    border-color: #fde68a;
}

.tag-night {
    background-color: #f3e8ff;
    color: #7c3aed;
    border-color: #ddd6fe;
}

.tag-warehouse {
    background-color: #dcfce7;
    color: #166534;
    border-color: #bbf7d0;
}

.tag-important {
    background-color: #ffedd5;
    color: #9a3412;
    border-color: #fed7aa;
}

.tag-terminal {
    background-color: #e0e7ff;
    color: #3730a3;
    border-color: #c7d2fe;
}

.tag-departure {
    background-color: #fce7f3;
    color: #be185d;
    border-color: #fbcfe8;
}

.tag-default {
    background-color: #f3f4f6;
    color: #374151;
    border-color: #e5e7eb;
}

/* Бейджи статуса заявки */
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

/* Кнопка скачивания */
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

/* Стилизация скроллбара */
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