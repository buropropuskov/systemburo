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
            <FilterTabs
                :tabs="archiveTabs"
                v-model="archiveMode"
            />
            <div class="filters__main">
                <div class="filters-row">
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
                    
                    <OrganizationFilter
                        ref="organizationFilter"
                        v-model="selectedOrganizationId"
                        :organizations="organizations"
                        @change="handleOrganizationChange"
                    />

                    <!-- Новый DateFilter -->
                    <DateFilter
                        ref="dateFilter"
                        :mode="'range'"
                        :selected-date="selectedDate"
                        :date-range-start="dateRangeStart"
                        :date-range-end="dateRangeEnd"
                        @update:selectedDate="updateSelectedDate"
                        @update:dateRangeStart="updateDateRangeStart"
                        @update:dateRangeEnd="updateDateRangeEnd"
                        @apply="applyDateFilters"
                        @clear="clearDateRange"
                    />

                    <button 
                        class="reset-sort-btn"
                        @click="resetSort"
                        :disabled="!sortField"
                    >
                        Сбросить сортировку
                    </button>

                    <button 
                        class="reset-filters-btn"
                        @click="resetFilters"
                        :disabled="!hasActiveFilters"
                    >
                        Сбросить фильтры
                    </button>
                </div>

                <div class="filters-row filters-row--secondary">
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
                    
                    <div class="filter-section filter-section--refresh">
                        <RefreshButton @refresh="fetchApplications" />
                    </div>
                </div>
            </div>
        </div>

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
                    <div class="header-col actions-col"></div>
                </div>
            </div>
            
            <div class="table-body">
                <SkeletonTransition :loading="loading">
                    <template #skeleton>
                        <SkeletonTable :rows="10" :columns="6" />
                    </template>
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
                                    Скачать
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
                <p v-else class="no-data-message">
                    {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Заявок нет' }}
                </p>
                </SkeletonTransition>
            </div>
        </div>

        <!-- Исправлено: используем selectedApplication вместо showDetail -->
        <ApplicationDetail
            v-if="selectedApplication"
            :application="selectedApplication"
            :current-user-id="currentUserId"
            :current-user-name="currentUserName"
            :mode="'center'"
            @close="closeDetail"
            @confirmation-updated="handleConfirmationUpdate"
            @application-updated="handleApplicationUpdate"
            @duplicate="handleDuplicate"
            @application-changed="handleApplicationChanged"
        />
    </section>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import OrganizationFilter from '@/components/OrganizationFilter.vue';
import RefreshButton from '../components/RefreshButton.vue';
import ApplicationDetail from '../components/ApplicationDetail/ApplicationDetail.vue';
import DateFilter from '../components/DateFilter.vue';
import FilterTabs from '@/components/ui/FilterTabs.vue';
import SkeletonTransition from '@/components/ui/SkeletonTransition.vue';
import SkeletonTable from '@/components/ui/SkeletonTable.vue';

export default {
    name: 'ApplicationsCenter',
    components: {
        OrganizationFilter,
        RefreshButton,
        ApplicationDetail,
        DateFilter,
        FilterTabs,
        SkeletonTransition,
        SkeletonTable
    },
    data() {
        return {
            searchQuery: '',
            selectedOrganizationId: null,
            selectedOrganizationName: '',
            selectedConfirmations: [],
            selectedApplicationStatuses: [],
            organizations: [],
            sortField: null,
            sortDirection: 'desc',
            shouldShake: false,
            shakeInterval: null,
            isInitialLoad: true,
            
            // Дата - теперь поддерживаем и одиночную дату, и диапазон
            selectedDate: null,
            dateRangeStart: null,
            dateRangeEnd: null,
            
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
            
            archiveMode: 'active',
            archiveTabs: [
                { key: 'active', label: 'Активные' },
                { key: 'archive', label: 'Архив' },
            ],

            loading: true,

            // Данные заявок
            applications: [],
            
            // Детали заявки
            selectedApplication: null,
            currentUserId: null,
            currentUserName: ''
        };
    },
    computed: {
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

            // Фильтр по дате - поддерживаем и одиночную дату, и диапазон
            if (this.selectedDate) {
                // Фильтр по одной дате
                filtered = filtered.filter(app => {
                    const appDate = new Date(app.sending_datetime);
                    const filterDate = new Date(this.selectedDate);
                    
                    // Сравниваем даты без времени
                    appDate.setHours(0, 0, 0, 0);
                    filterDate.setHours(0, 0, 0, 0);
                    
                    return appDate.getTime() === filterDate.getTime();
                });
            } else if (this.dateRangeStart && this.dateRangeEnd) {
                // Фильтр по диапазону дат
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
                   !!this.selectedDate ||
                   (this.dateRangeStart && this.dateRangeEnd);
        },

        unreadCount() {
            return this.applications.filter(app => app.status === 'Непрочитано').length;
        }
    },
    methods: {
        // Организация
        getOrganizationName(application) {
            if (application.organization_name && application.organization_name.trim()) {
                return application.organization_name;
            }
            else if (application.company_name && application.company_name.trim()) {
                return application.company_name;
            }
            return 'Не указана';
        },
        
        handleOrganizationChange({ id, name }) {
            this.selectedOrganizationId = id;
            this.selectedOrganizationName = name;
            this.applyFilters();
        },
        
        // Отправитель
        getSenderName(fullName) {
            if (!fullName) return '';
            return fullName;
        },
        
        getFullNameTooltip(fullName) {
            if (!fullName) return '';
            return fullName.replace(/\./g, '').trim();
        },
        
        // Поиск
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
        
        // Фильтры
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
            // Сбрасываем все фильтры
            this.searchQuery = '';
            this.selectedOrganizationId = null;
            this.selectedOrganizationName = '';
            this.selectedConfirmations = [];
            this.selectedApplicationStatuses = [];
            this.selectedDate = null;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            
            // Сбрасываем сортировку
            this.resetSort();
            
            // Сбрасываем фильтр организации через метод reset
            if (this.$refs.organizationFilter && this.$refs.organizationFilter.reset) {
                this.$refs.organizationFilter.reset();
            }
            
            // Сбрасываем фильтр даты
            if (this.$refs.dateFilter && this.$refs.dateFilter.clearSelection) {
                this.$refs.dateFilter.clearSelection();
            }
            
            // Сбрасываем анимацию загрузки и обновляем данные
            this.isInitialLoad = false;
            this.applyFilters();
        },
        
        applyFilters() {
            this.isInitialLoad = false;
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

        resetSort() {
            this.sortField = null;
            this.sortDirection = 'desc';
        },
        
        // Дата
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
        
        updateSelectedDate(date) {
            this.selectedDate = date;
            // При выборе одиночной даты сбрасываем диапазон
            if (date) {
                this.dateRangeStart = null;
                this.dateRangeEnd = null;
            }
        },
        
        updateDateRangeStart(date) {
            this.dateRangeStart = date;
            // При выборе диапазона сбрасываем одиночную дату
            if (date) {
                this.selectedDate = null;
            }
        },
        
        updateDateRangeEnd(date) {
            this.dateRangeEnd = date;
            // При выборе диапазона сбрасываем одиночную дату
            if (date) {
                this.selectedDate = null;
            }
        },
        
        applyDateFilters() {
            this.applyFilters();
        },
        
        clearDateRange() {
            this.selectedDate = null;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            this.applyFilters();
        },
        
        // Стилизация
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
        
        // API методы
        async fetchApplications() {
            try {
                const authStore = useAuthStore();
                if (!authStore.token) {
                    console.error("Пользователь не авторизован.");
                    return;
                }

                let url = "/applications";
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
                
                // Добавляем параметр архива
                params.append('archive', this.archiveMode === 'archive' ? 'true' : 'false');

                // Добавляем параметры даты в запрос к API
                if (this.selectedDate) {
                    const dateStr = this.selectedDate.toISOString().split('T')[0];
                    params.append('date', dateStr);
                } else if (this.dateRangeStart && this.dateRangeEnd) {
                    params.append('date_from', this.dateRangeStart.toISOString().split('T')[0]);
                    params.append('date_to', this.dateRangeEnd.toISOString().split('T')[0]);
                }

                const queryString = params.toString();
                if (queryString) {
                    url += '?' + queryString;
                }

                const response = await apiRequest(url, {
                    method: "GET",
                });

                if (response.ok) {
                    const data = await response.json();
                    this.applications = data;
                } else {
                    console.error("Ошибка при загрузке заявок:", await response.text());
                }
            } catch (error) {
                console.error("Ошибка сети при загрузке заявок:", error);
            } finally {
                this.loading = false;
            }
        },

        async fetchOrganizations() {
            try {
                const response = await apiRequest("/organizations", {
                    method: "GET",
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
                    const response = await apiRequest(`/applications/${application.id}`, {
                        method: "PUT",
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

            this.selectedApplication = application;
        },

        closeDetail() {
            this.selectedApplication = null;
        },

        handleConfirmationUpdate(updatedData) {
            if (this.selectedApplication) {
                Object.assign(this.selectedApplication, updatedData);
                
                const appIndex = this.applications.findIndex(app => app.id === this.selectedApplication.id);
                if (appIndex !== -1) {
                    this.applications[appIndex] = { ...this.applications[appIndex], ...updatedData };
                }
            }
            
            this.$emit('refresh-data');
        },

        handleApplicationUpdate(updatedApp) {
            console.log('Application updated in center:', updatedApp);
            this.fetchApplications(); // Обновляем список заявок
        },

        handleApplicationChanged(updatedApp) {
            console.log('Application changed in center (via application-changed):', updatedApp);
            
            // Обновляем данные в списке
            const appIndex = this.applications.findIndex(app => app.id === updatedApp.id);
            if (appIndex !== -1) {
                // Обновляем существующую заявку
                this.applications[appIndex] = {
                    ...this.applications[appIndex],
                    ...updatedApp
                };
                
                // Если это открытая заявка, обновляем и её
                if (this.selectedApplication && this.selectedApplication.id === updatedApp.id) {
                    this.selectedApplication = {
                        ...this.selectedApplication,
                        ...updatedApp
                    };
                }
                
                // Принудительно обновляем список для пересчета computed свойств
                this.applications = [...this.applications];
            } else {
                // Если заявка не найдена в списке (например, из-за фильтров), просто перезагружаем весь список
                this.fetchApplications();
            }
        },

        handleDuplicate(application) {
            console.log('Дублирование заявки из ApplicationsCenter:', application?.application_number);
        },

        async getCurrentUser() {
            try {
                const response = await apiRequest("/users/me", {
                    method: "GET",
                });

                if (response.ok) {
                    const userData = await response.json();
                    this.currentUserId = userData.id;
                    this.currentUserName = `${userData.last_name} ${userData.first_name}`;
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
    watch: {
        archiveMode() {
            this.fetchApplications();
        },
        '$route.query.archive'(val) {
            this.archiveMode = val === 'true' ? 'archive' : 'active';
        },
    },
    mounted() {
        this.startShakeAnimation();

        if (this.$route.query.archive === 'true') {
            this.archiveMode = 'archive';
        }

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
    background: var(--color-primary);
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
    border-bottom: 1px solid var(--color-border);
}

.filters-row {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 12px;
}

.filters-row--secondary {
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
    border: 1px solid var(--color-border);
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
    border: 1px solid var(--color-border);
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
    background: var(--color-primary);
    color: white;
    border-color: var(--color-primary);
}

.status-btn--active:hover {
    background: #3a45c0;
    border-color: #3a45c0;
}

.reset-sort-btn,
.reset-filters-btn {
    padding: 6px 12px;
    border: 1px solid var(--color-border);
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

.applications-table {
    min-width: 300px;
    background-color: #fff;
    border-radius: 30px;
    border: 1px solid var(--color-border);
    overflow: hidden;
    margin-top: 20px;
    height: fit-content;
    max-height: 500px;
   
    display: flex;
    flex-direction: column;
    transition: all 0.3s ease;
}

.applications-table.with-details {
    height: 500px;
}

.table-header {
    border-bottom: 1px solid var(--color-border);
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

/* Перераспределение размеров колонок */
.confirmation-col {
    width: 15%;
}

.number-col {
    width: 15%;
}

.date-col {
    width: 15%;
}

.organization-col {
    width: 20%;
}

.sender-col {
    width: 15%;
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
    width: 15%;
}

.actions-col {
    width: 7%;
    justify-content: flex-end;
    cursor: default;
}

.header-col.actions-col:hover {
    color: #a2a2a2;
}

.table-body {
    flex-grow: 1;
    overflow-y: scroll;
}

.applications-list {
    flex-grow: 1;
}

.application-item {
    transition: background-color 0.2s ease;
    cursor: pointer;
}

.application-item:hover {
    background-color: #a2a2a2;
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

.application-item:hover:not(.download-btn:hover) {
    background-color: #f0f0f0;
}

.application-item.unread {
    background-color: #fffeda;
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
    font-size: 14px;
    display: flex;
    align-items: center;
    height: 100%;
}

/* Стили для кнопки "Скачать" */
.download-btn {
    height: 25px;
    background-color: #fff;
    color: #000;
    border-radius: 50px;
    border: 1px solid var(--color-border);
    font-size: 12px;
    font-weight: 500;
    padding: 0 12px;
    cursor: pointer;
    transition: all 0.2s ease;
    display: flex;
    align-items: center;
    justify-content: center;
    white-space: nowrap;
    min-width: 80px;
}

.download-btn:hover {
    background-color: #f5f5f5;
    border-color: #d0d0d0;
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