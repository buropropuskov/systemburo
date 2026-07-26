<template>
  <div class="tables">
    <div class="tables__header">
      <h1 class="tables__title">
        Таблица <span class="table-name">{{ tableDisplayName }}</span>
      </h1>
      <button
        class="tables__instruction"
        @click="openInstruction"
      >
        <img
          src="@/assets/icons/instruction.png"
          class="tables__icon"
        >
        <p class="instruction__text">
          Инструкция
        </p>
      </button>
    </div>
        
    <!-- Модальное окно с инструкцией -->
    <Teleport to="body">
      <transition name="instruction-modal">
        <div
          v-if="showInstruction"
          class="modal-overlay"
          @mousedown="onOverlayMousedown"
          @mouseup="onOverlayMouseup"
        >
          <div
            class="instruction-modal-large"
            @mousedown.stop
          >
            <div class="modal-header">
              <h3>Инструкция по использованию таблицы <span class="blue">{{ tableDisplayName }}</span></h3>
              <button
                class="modal-close"
                @click="closeInstruction"
              >
                ×
              </button>
            </div>
            <div class="instruction-content">
              <div
                v-if="tableInstruction"
                class="text-constructor-content"
                v-html="sanitizedInstruction"
              />
              <div
                v-else
                class="no-instruction"
              >
                <div class="no-instruction-icon">
                  📝
                </div>
                <h4>Инструкция не добавлена</h4>
                <p>Для этой таблицы пока не создана и не написана инструкция.</p>
                <p>Обратитесь к Бюро пропусков для добавления инструкции.</p>
              </div>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <div class="tables__filters">
      <div class="filters__fields">
        <!-- Поиск -->
        <div class="field search">
          <input 
            v-model="searchQuery" 
            placeholder="Поиск.." 
            type="text" 
            class="field__input search"
            @input="applyFilters"
          >
          <img
            src="@/assets/icons/search.png"
            class="tables__icon"
          >
        </div>
                
        <!-- Организация через компонент -->
        <OrganizationFilter
          v-if="showOrganizationFilter"
          ref="organizationFilter"
          v-model="selectedOrganizationId"
          :organizations="organizations"
          @change="handleOrganizationChange"
        />

        <!-- Компания через тот же компонент (reuse OrganizationFilter) -->
        <OrganizationFilter
          v-if="showOrganizationFilter"
          ref="companyFilter"
          v-model="selectedCompanyId"
          :organizations="companies"
          all-label="Все компании"
          placeholder-text="Компания"
          @change="handleCompanyChange"
        />

        <!-- Место разгрузки через компонент -->
        <UnloadingPlaceFilter
          v-if="showUnloadingFilter"
          ref="unloadingPlaceFilter"
          v-model="selectedUnloadingPlaceId"
          @change="handleUnloadingPlaceChange"
        />

        <!-- Новый DateFilter -->
        <DateFilter
          ref="dateFilter"
          :mode="'range'"
          :selected-date="selectedDate"
          :date-range-start="dateRangeStart"
          :date-range-end="dateRangeEnd"
          @update:selected-date="updateSelectedDate"
          @update:date-range-start="updateDateRangeStart"
          @update:date-range-end="updateDateRangeEnd"
          @apply="applyDateFilters"
          @clear="clearDate"
        />
      </div>
      <div class="filters__options">
        <button
          v-if="canManualAdd"
          class="options__manual-add"
          data-testid="manual-add-button"
          @click="showManualAdd = true"
        >
          <span class="options__manual-add-icon">+</span>
          <span class="options__text">Добавить вручную</span>
        </button>
        <RouterLink
          v-if="can(`table.${$route.params.tableName}.versions`)"
          :to="`/table/${$route.params.tableName}/versions`"
          class="options__versions-link"
          title="Версии состояния таблицы"
          data-testid="table-versions-link"
          aria-label="Версии состояния таблицы"
        >
          <img
            src="@/assets/icons/recent-changes.png"
            class="options__icon"
            alt=""
          >
        </RouterLink>
        <RouterLink
          v-if="can(`table.${$route.params.tableName}.trash`)"
          :to="`/table/${$route.params.tableName}/trash`"
          class="options__trash-link"
          title="Корзина таблицы"
          data-testid="table-trash-link"
          aria-label="Корзина таблицы"
        >
          <img
            src="@/assets/icons/trashcan.png"
            class="options__icon"
            alt=""
          >
        </RouterLink>
        <button
          v-if="can(`table.${$route.params.tableName}.export`)"
          class="options__export"
          @click="handleExport"
        >
          <img
            src="@/assets/icons/export.png"
            class="tables__icon"
          >
          <p class="options__text">
            Экспорт
          </p>
        </button>
        <button
          v-if="can(`table.${$route.params.tableName}.report`)"
          class="options__export"
          data-testid="pass-report-button"
          @click="showPassReport = true"
        >
          <img
            src="@/assets/icons/stats.png"
            class="tables__icon"
          >
          <p class="options__text">
            Отчёт
          </p>
        </button>
      </div>
    </div>

    <div class="tables__content">
      <!-- Таблица по факту с подсказкой -->
      <div
        v-if="showFactTable"
        class="fact-section"
      >
        <FactTable
          v-if="currentUserId"
          ref="factTable"
          :table-type="tableType"
          :table-id="tableData?.table?.id"
          :table-data="tableData"
          :search-query="searchQuery"
          :selected-organization-id="selectedOrganizationId"
          :selected-company-id="selectedCompanyId"
          :selected-unloading-place="selectedUnloadingPlaceName"
          :date-range-start="dateRangeStart"
          :date-range-end="dateRangeEnd"
          :selected-date="selectedDate"
          :current-user-id="currentUserId"
          :current-user-name="currentUserName"
          :loading="isRefreshing"
          :grid="gridMode"
          @refresh-data="refreshData"
          @open-application="handleOpenApplication"
        />
        <!-- Подсказка на синем фоне -->
        <div
          v-if="tableFactHint"
          class="fact-hint-card"
        >
          <div
            class="text-constructor-content hint-content"
            v-html="sanitizedHint"
          />
        </div>
      </div>
            
      <!-- Основная таблица - разные компоненты для разных типов -->
      <CarsTable
        v-if="tableType === 'cars' && currentUserId"
        ref="carsTable"
        v-model:grid="gridMode"
        :table-name="tableSystemName"
        :table-title="tableDisplayName"
        :table-id="tableData?.table?.id"
        :search-query="searchQuery"
        :selected-organization-id="selectedOrganizationId"
        :selected-company-id="selectedCompanyId"
        :selected-unloading-place-id="selectedUnloadingPlaceId"
        :date-range-start="dateRangeStart"
        :date-range-end="dateRangeEnd"
        :selected-date="selectedDate"
        :current-user-id="currentUserId"
        :current-user-name="currentUserName"
        @refresh-data="refreshData"
        @open-application="handleOpenApplication"
      />

      <PeopleTable
        v-if="tableType === 'people'"
        ref="peopleTable"
        v-model:grid="gridMode"
        :table-name="tableSystemName"
        :search-query="searchQuery"
        :selected-organization-id="selectedOrganizationId"
        :selected-company-id="selectedCompanyId"
        :selected-unloading-place-id="selectedUnloadingPlaceId"
        :date-range-start="dateRangeStart"
        :date-range-end="dateRangeEnd"
        :selected-date="selectedDate"
        :current-user-id="currentUserId"
        :current-user-name="currentUserName"
        @refresh-data="refreshData"
        @open-application="handleOpenApplication"
      />
    </div>

    <ApplicationDetail
      v-if="showApplicationDetail"
      :application="selectedApplication"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      :mode="'center'"
      @close="closeApplicationDetail"
      @application-changed="handleApplicationChanged"
    />

    <TableExportModal
      :show="showExportModal"
      @close="showExportModal = false"
      @export="handleExportChoice"
    />

    <ManualAddModal
      :show="showManualAdd"
      :mode="tableType || 'cars'"
      :table-id="tableData?.table?.id"
      :table-name="tableDisplayName"
      @close="showManualAdd = false"
      @added="onManualAdded"
    />

    <PassReportModal
      :show="showPassReport"
      :table-id="tableData?.table?.id"
      :table-type="tableType"
      :table-display-name="tableDisplayName"
      :current-user-name="currentUserName"
      @close="showPassReport = false"
    />
  </div>
</template>

<script>
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { ref, watch, onMounted, onBeforeUnmount } from 'vue';
import { apiRequest } from '@/api/client'
import { sanitizeHtml } from '@/utils/sanitize';
import { useOverlayClose } from '@/composables/useOverlayClose';
import OrganizationFilter from '@/components/OrganizationFilter.vue';
import UnloadingPlaceFilter from '@/components/UnloadingPlaceFilter.vue';
import DateFilter from './DateFilter.vue';
import FactTable from './FactTable.vue';
import CarsTable from './CarsTable.vue';
import PeopleTable from './PeopleTable.vue';
import ApplicationDetail from './ApplicationDetail/ApplicationDetail.vue';
import TableExportModal from './TableExportModal.vue';
import ManualAddModal from './ManualAddModal.vue';
import PassReportModal from './PassReportModal.vue';
import { usePermissionsStore } from '@/stores/permissions';

export default {
    name: 'TablesComponent',
    components: {
        OrganizationFilter,
        UnloadingPlaceFilter,
        DateFilter,
        FactTable,
        CarsTable,
        PeopleTable,
        ApplicationDetail,
        TableExportModal,
        ManualAddModal,
        PassReportModal,
    },
    emits: ['refresh-data'],
    setup() {
        const showInstruction = ref(false);
        const openInstruction = () => { showInstruction.value = true; };
        const closeInstruction = () => { showInstruction.value = false; };
        const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(closeInstruction);

        const onKeydown = (e) => {
            if (e.key === 'Escape' && showInstruction.value) closeInstruction();
        };

        watch(showInstruction, (open) => {
            setBodyScrollLock(this, open);
        });

        onMounted(() => document.addEventListener('keydown', onKeydown));
        onBeforeUnmount(() => {
            document.removeEventListener('keydown', onKeydown);
            releaseBodyScrollLock(this);
        });

        const permissionsStore = usePermissionsStore();

        return { showInstruction, openInstruction, closeInstruction, onOverlayMousedown, onOverlayMouseup, permissionsStore };
    },
    data() {
        return {
            tableData: null,
            isRefreshing: false,
            searchQuery: '',
            selectedOrganizationId: null,
            selectedOrganizationName: '',
            selectedCompanyId: null,
            selectedCompanyName: '',
            selectedUnloadingPlaceId: null,
            selectedUnloadingPlaceName: '',

            organizations: [],
            companies: [],

            selectedDate: null,
            dateRangeStart: null,
            dateRangeEnd: null,
            
            currentUserId: null,
            currentUserName: '',

            showApplicationDetail: false,
            selectedApplication: null,
            showExportModal: false,
            showManualAdd: false,
            showPassReport: false,

            // Режим "Сетка" (#1289): один тумблер страницы на обе таблицы
            // (по факту + основная). Состояние своё у каждой таблицы.
            gridMode: false,
        };
    },
    computed: {
        tableSystemName() {
            return this.tableData?.table?.name || '';
        },
        
        tableDisplayName() {
            return this.tableData?.table?.display_name || 'Загрузка...';
        },
        
        tableType() {
            return this.tableData?.table?.table_type || '';
        },
        
        showFactTable() {
            return this.tableData?.table?.show_fact_table || false;
        },
        
        tableFactHint() {
            return this.tableData?.table?.fact_table_hint || '';
        },
        
        tableInstruction() {
            return this.tableData?.table?.instruction || '';
        },
        
        showOrganizationFilter() {
            return this.tableType === 'cars' || this.tableType === 'people';
        },
        
        showUnloadingFilter() {
            return this.tableType === 'cars';
        },

        // Кнопка "Добавить вручную" (#1049): для cars - право entity.cars.manual_add,
        // для people - entity.employees.manual_add (super/admin проходят авто). Ключ
        // зеркалит BE-гейт роутов /cars/manual и /employees/manual (RequirePermissionV2).
        canManualAdd() {
            if (this.tableType === 'cars') return this.can('entity.cars.manual_add');
            if (this.tableType === 'people') return this.can('entity.employees.manual_add');
            return false;
        },
        
        sanitizedHint() {
            return this.sanitizeHtml(this.tableFactHint);
        },
        
        sanitizedInstruction() {
            return this.sanitizeHtml(this.tableInstruction);
        },
        
        hasActiveFilters() {
            return !!this.searchQuery.trim() ||
                   !!this.selectedOrganizationId ||
                   !!this.selectedCompanyId ||
                   !!this.selectedUnloadingPlaceId ||
                   !!this.selectedDate ||
                   (this.dateRangeStart && this.dateRangeEnd);
        }
    },
    watch: {
        '$route.params.tableName': {
            handler() {
                this.fetchTableData();
                this.clearFilters();
                this.loadGridMode();
            },
            immediate: true
        },
        gridMode(value) {
            this.saveGridMode(value);
        }
    },
    async mounted() {
        await this.fetchCurrentUser();  // Ждём загрузки пользователя
        this.fetchTableData();          // потом таблицы
    },
    methods: {
        // Гейтинг по правам (#187 Фаза 2). super -> всегда true, admin -> всё кроме
        // denied, обычный -> по эффективному гранту. Реактивно: читает стор прав.
        can(key) {
            return this.permissionsStore.hasPermission(key);
        },

        // Ключ от имени таблицы в роуте, а не от tableData.table.id: id приезжает
        // асинхронно, тумблер успел бы моргнуть до загрузки настроек таблицы.
        gridStorageKey() {
            return `grid-mode:${this.$route.params.tableName || 'default'}`;
        },

        loadGridMode() {
            try {
                this.gridMode = localStorage.getItem(this.gridStorageKey()) === '1';
            } catch {
                this.gridMode = false;
            }
        },

        saveGridMode(value) {
            try {
                localStorage.setItem(this.gridStorageKey(), value ? '1' : '0');
            } catch {
                // localStorage недоступен (приватный режим) - режим просто не запомнится.
            }
        },
        handleApplicationUpdate(updatedApp) {
            console.log('Application updated:', updatedApp);
            this.refreshData();
        },

        async fetchCurrentUser() {
            try {
                const response = await apiRequest("/users/me", {
                });

                if (response.ok) {
                    const userData = await response.json();
                    this.currentUserId = userData.id;

                    const nameParts = [userData.last_name, userData.first_name, userData.middle_name].filter(Boolean);
                    this.currentUserName = nameParts.join(' ') || userData.username;
                }
            } catch (error) {
                console.error("Ошибка при загрузке данных пользователя:", error);
            }
        },

        async refreshData() {
            this.isRefreshing = true;
            try {
                await this.fetchTableData();
                this.$emit('refresh-data');
            } finally {
                this.isRefreshing = false;
            }
        },

        // Ручные записи появились в текущей и fact-таблице (#1049) - перегружаем строки
        // напрямую (real-time #840 продублирует по target-таблице, здесь для мгновенного отклика).
        onManualAdded() {
            this.$refs.carsTable?.loadData?.();
            this.$refs.peopleTable?.loadData?.();
            this.$refs.factTable?.loadData?.();
        },
        
        sanitizeHtml(content) {
            return sanitizeHtml(content);
        },

        async fetchTableData() {
            const tableName = this.$route.params.tableName;
            if (!tableName) return;
            
            try {
                const response = await apiRequest(`/system-tables/name/${tableName}`, {
                });
                
                if (response.ok) {
                    const data = await response.json();
                    console.log('Table data received:', data);
                    this.tableData = data;

                    await this.fetchOrganizationsForTable();
                    await this.fetchCompaniesForTable();
                } else {
                    console.error('Table not found');
                    this.$router.push('/404');
                }
            } catch (error) {
                console.error("Error fetching table data:", error);
            }
        },

        async fetchOrganizationsForTable() {
            try {
                const response = await apiRequest("/organizations", {
                    method: "GET",
                });

                if (response.ok) {
                    const data = await response.json();
                    console.log('Organizations loaded:', data);
                    this.organizations = data;
                } else {
                    console.error("Ошибка при загрузке организаций");
                }
            } catch (error) {
                console.error("Ошибка сети при загрузке организаций:", error);
            }
        },

        async fetchCompaniesForTable() {
            try {
                const response = await apiRequest("/companies", {
                    method: "GET",
                });

                if (response.ok) {
                    this.companies = await response.json();
                } else {
                    console.error("Ошибка при загрузке компаний");
                }
            } catch (error) {
                console.error("Ошибка сети при загрузке компаний:", error);
            }
        },

        handleOrganizationChange({ id, name }) {
            this.selectedOrganizationId = id;
            this.selectedOrganizationName = name;
            this.applyFilters();
        },

        handleCompanyChange({ id, name }) {
            this.selectedCompanyId = id;
            this.selectedCompanyName = name;
            this.applyFilters();
        },

        handleUnloadingPlaceChange({ id, name }) {
            this.selectedUnloadingPlaceId = id;
            this.selectedUnloadingPlaceName = name;
            this.applyFilters();
        },
        
        updateSelectedDate(date) {
            this.selectedDate = date;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
        },
        
        updateDateRangeStart(date) {
            this.dateRangeStart = date;
            this.selectedDate = null;
        },
        
        updateDateRangeEnd(date) {
            this.dateRangeEnd = date;
            this.selectedDate = null;
        },
        
        applyDateFilters() {
            this.applyFilters();
        },
        
        clearDate() {
            this.selectedDate = null;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            this.applyFilters();
        },
        
        applyFilters() {
            // Фильтры применяются автоматически через props в дочерних компонентах
        },
        
        clearFilters() {
            this.searchQuery = '';
            
            this.selectedDate = null;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            
            if (this.$refs.organizationFilter && this.$refs.organizationFilter.reset) {
                this.$refs.organizationFilter.reset();
            }

            if (this.$refs.companyFilter && this.$refs.companyFilter.reset) {
                this.$refs.companyFilter.reset();
            }

            if (this.$refs.unloadingPlaceFilter && this.$refs.unloadingPlaceFilter.reset) {
                this.$refs.unloadingPlaceFilter.reset();
            }
            
            if (this.$refs.dateFilter && this.$refs.dateFilter.clearSelection) {
                this.$refs.dateFilter.clearSelection();
            }
        },

        async handleOpenApplication(applicationId) {
            try {
                const response = await apiRequest(`/applications/${applicationId}/details`, {});
                if (response.ok) {
                    const appData = await response.json();
                    this.selectedApplication = appData;
                    this.showApplicationDetail = true;
                } else {
                    console.error('Не удалось загрузить заявку');
                }
            } catch (error) {
                console.error('Ошибка загрузки заявки:', error);
            }
        },

        closeApplicationDetail() {
            this.showApplicationDetail = false;
            this.selectedApplication = null;
        },

        handleApplicationChanged() {
            this.refreshData();
        },

        handleExport() {
            if (this.showFactTable) {
                // Показываем модалку выбора только когда есть факт-таблица
                this.showExportModal = true;
            } else {
                // Только основная таблица — экспортируем сразу без диалога
                this.exportMainTable();
            }
        },

        async handleExportChoice(choice) {
            if (choice === 'both' || choice === 'fact') {
                const factRef = this.$refs.factTable;
                if (factRef && typeof factRef.exportToExcel === 'function') {
                    await factRef.exportToExcel();
                }
            }
            if (choice === 'both' || choice === 'main') {
                await this.exportMainTable();
            }
        },

        async exportMainTable() {
            const mainRef = this.$refs.carsTable || this.$refs.peopleTable;
            if (mainRef && typeof mainRef.exportToExcel === 'function') {
                await mainRef.exportToExcel();
            }
        },
    }
}
</script>

<style scoped>
/* Все стили остаются без изменений */
.tables {
    padding: 20px;
    position: relative;
}

.tables__title {
    font-size: 18px;
    font-weight: bold;
    color: var(--text);
}

.table-name {
    color: var(--accent-text);
}

.tables__header {
    display: flex;
    gap: 10px;
    padding-bottom: 15px;
    align-items: center;
    flex-wrap: wrap;
}

.tables__header-actions {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-left: auto;
    flex-wrap: wrap;
    justify-content: flex-end;
}

.tables__instruction {
    width: fit-content;
    font-size: 14px;
    font-weight: 500;
    color: var(--accent-text);
    padding: 0 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 5px;
    border-radius: 50px;
    background: var(--surface);
    border: 1px solid var(--border);
    outline: none;
    cursor: pointer;
    height: 25px;
    transition: all 0.2s ease;
}

.tables__instruction:hover {
    background-color: var(--surface-2);
    border-color: var(--accent);
}

.tables__icon {
    width: 15px;
    height: 15px;
}

.tables__filters {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 15px;
    border-bottom: 1px solid var(--border);
    gap: clamp(6px, 0.8vw, 10px);
    min-width: 0;
    flex-wrap: nowrap;
}

/* clamp на gap/padding/height/font-size - адаптивное масштабирование без
   media-queries. Фильтры всегда в одну строку: на узком экране становятся
   мельче, но не переносятся и не уезжают. */
.filters__fields {
    display: flex;
    align-items: center;
    gap: clamp(6px, 0.8vw, 10px);
    position: relative;
    min-width: 0;
    flex-wrap: nowrap;
    flex-shrink: 1;
}


.field {
    width: clamp(120px, 14vw, 200px);
    height: clamp(28px, 3vw, 35px);
    background-color: var(--surface);
    border-radius: 15px;
    border: 1px solid var(--border);
    padding: 0 clamp(6px, 0.8vw, 10px);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: clamp(4px, 0.6vw, 10px);
    position: relative;
    cursor: pointer;
    flex-shrink: 1;
    min-width: 0;
}

.field--select {
    cursor: pointer;
}

.field__input {
    outline: none;
    border: none;
    background-color: transparent;
    font-size: clamp(11px, 1.1vw, 14px);
    width: 100%;
    min-width: 0;
    cursor: pointer;
}

.select-text {
    font-size: clamp(11px, 1.1vw, 14px);
    color: var(--text);
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.search {
    cursor: text;
}

.filters__options {
    display: flex;
    align-items: center;
    gap: clamp(6px, 1vw, 15px);
    justify-content: flex-end;
    flex-wrap: nowrap;
    flex-shrink: 0;
}

.options__icon {
    width: clamp(14px, 1.6vw, 20px);
    height: clamp(14px, 1.6vw, 20px);
    cursor: pointer;
    flex-shrink: 0;
}

.options__export {
    width: clamp(70px, 8vw, 100px);
    height: clamp(22px, 2.4vw, 25px);
    background: var(--surface);
    border: 1px solid var(--border);
    outline: none;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 10px;
    gap: 5px;
    font-size: clamp(10px, 1vw, 13px);
}

.options__export:hover {
    background: var(--surface-2);
}

.options__manual-add {
    height: clamp(22px, 2.4vw, 25px);
    padding: 0 clamp(8px, 1vw, 14px);
    background: var(--surface);
    color: var(--text);
    border: 1px solid var(--border);
    outline: none;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 10px;
    gap: 5px;
    font-size: clamp(10px, 1vw, 13px);
    flex-shrink: 0;
    white-space: nowrap;
}

.options__manual-add:hover {
    background: var(--surface-2);
}

.options__manual-add-icon {
    font-size: clamp(13px, 1.3vw, 16px);
    line-height: 1;
    font-weight: 600;
}

.options__text {
    font-weight: 500;
}

.tables__content {
    margin-top: 15px;
    display: flex;
    flex-direction: column;
    gap: 20px;
}

/* Fact Section with Hint */
.fact-section {
    display: flex;
    gap: 20px;
}

.fact-hint-card {
    flex: 0 0 35%;
    /* Карточка не задаёт высоту секции: её текст лежит в absolute-слое, поэтому в
       поток идёт только рамка, а высоту строки диктует таблица «по факту». Длинная
       подсказка прокручивается внутри, а не вытягивает блок выше таблицы. */
    position: relative;
    overflow: hidden;
    /* Через токены: в светлых темах акцентная плашка, в тёмных - тёмная карточка
       в тон темы (сплошной синий блок на тёмном фоне читался плохо). Тени нет:
       крупные карточки страниц отделяет рамка, тень оставлена окнам и всплывающему. */
    background-color: var(--hint-card-bg);
    border: 1px solid var(--hint-card-border);
    border-radius: 30px;
    padding: 20px;
    display: flex;
    gap: 15px;
    align-items: flex-start;
}

.hint-content {
    position: absolute;
    inset: 20px;
    overflow-y: auto;
    scrollbar-width: thin;
    color: var(--hint-card-text);
}

.reset-filters-btn {
    padding: 6px 12px;
    border: 1px solid var(--border);
    background: var(--danger-bg);
    border-radius: 15px;
    cursor: pointer;
    font-size: 12px;
    transition: all 0.2s;
    height: 35px;
    color: var(--danger-text);
    white-space: nowrap;
    display: flex;
    align-items: center;
    gap: 5px;
    border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.reset-filters-btn:hover:not(:disabled) {
    background: color-mix(in srgb, var(--danger) 22%, var(--surface));
}

.reset-filters-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
    background: var(--surface-2);
    color: var(--text-muted);
    border-color: var(--border);
}

.reset-filters-btn:disabled .options__icon {
    filter: grayscale(100%);
}

/* Text Constructor Content Styles */
.text-constructor-content :deep(*) {
    line-height: 150%;
    overflow-wrap: break-word;
}

.text-constructor-content :deep(img) {
    max-width: 100%;
    border-radius: 8px;
}

.text-constructor-content :deep(img:not([height])) {
    height: auto;
}

.text-constructor-content :deep(.constructor-image.img-align-left) { float: left; margin: 0 14px 10px 0; }
.text-constructor-content :deep(.constructor-image.img-align-right) { float: right; margin: 0 0 10px 14px; }
.text-constructor-content :deep(.constructor-image.img-align-center) { display: block; margin: 10px auto; float: none; }
.text-constructor-content::after { content: ''; display: block; clear: both; }

.text-constructor-content :deep(strong) {
    font-weight: 600 !important;
}

.text-constructor-content :deep(em) {
    font-style: italic !important;
}

.text-constructor-content :deep(u) {
    text-decoration: underline !important;
}

.text-constructor-content :deep(ul),
.text-constructor-content :deep(ol) {
    padding-left: 20px !important;
}

.text-constructor-content :deep(li) {
    line-height: 150% !important;
}

.text-constructor-content :deep(h1) {
    font-size: 20px !important;
    font-weight: 700 !important;
    margin: 0 0 12px 0 !important;
    line-height: 1.2 !important;
}

.text-constructor-content :deep(h2) {
    font-size: 18px !important;
    font-weight: 600 !important;
    margin: 0 0 10px 0 !important;
    line-height: 1.3 !important;
}

.text-constructor-content :deep(h3) {
    font-size: 16px !important;
    font-weight: 500 !important;
    margin: 0 0 8px 0 !important;
    line-height: 1.4 !important;
}

.text-constructor-content :deep(.black-text) { color: #000 !important; }
.text-constructor-content :deep(.red-text) { color: #FF0000 !important; }
.text-constructor-content :deep(.green-text) { color: #079D1D !important; }
.text-constructor-content :deep(.blue-text) { color: var(--accent-text) !important; }

.text-constructor-content :deep(.font-size-10) { font-size: 10px !important; }
.text-constructor-content :deep(.font-size-12) { font-size: 12px !important; }
.text-constructor-content :deep(.font-size-14) { font-size: 14px !important; }
.text-constructor-content :deep(.font-size-16) { font-size: 16px !important; }
.text-constructor-content :deep(.font-size-18) { font-size: 18px !important; }
.text-constructor-content :deep(.font-size-20) { font-size: 20px !important; }

.text-constructor-content :deep(.font-weight-300) { font-weight: 300 !important; }
.text-constructor-content :deep(.font-weight-400) { font-weight: 400 !important; }
.text-constructor-content :deep(.font-weight-500) { font-weight: 500 !important; }
.text-constructor-content :deep(.font-weight-600) { font-weight: 600 !important; }
.text-constructor-content :deep(.font-weight-900) { font-weight: 900 !important; }

.text-constructor-content :deep(.heading-h1) { 
    font-size: 24px !important;
    font-weight: 700 !important;
    margin: 0 0 8px 0 !important;
    line-height: 1.2 !important;
}

.text-constructor-content :deep(.heading-h2) {
    font-size: 20px !important;
    font-weight: 600 !important;
    margin: 8px 0 6px 0 !important;
    line-height: 1.3 !important;
}

.text-constructor-content :deep(.text-align-left) { text-align: left !important; }
.text-constructor-content :deep(.text-align-center) { text-align: center !important; }
.text-constructor-content :deep(.text-align-right) { text-align: right !important; }

/* Hint specific styles */
.hint-content :deep(*) {
    color: var(--hint-card-text) !important;
}

.hint-content :deep(.black-text) { color: var(--hint-card-text) !important; }
.hint-content :deep(.red-text) { color: #FFB3B3 !important; }
.hint-content :deep(.green-text) { color: #B3FFC6 !important; }
.hint-content :deep(.blue-text) { color: #B3C6FF !important; }

.hint-content :deep(.text-align-left) { text-align: left !important; }
.hint-content :deep(.text-align-center) { text-align: center !important; }
.hint-content :deep(.text-align-right) { text-align: right !important; }

/* Instruction Modal */
.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: var(--overlay);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 20px;
    backdrop-filter: blur(0.1px);
    -webkit-backdrop-filter: blur(0.1px);
}

.instruction-modal-large {
    background: var(--surface);
    border-radius: 30px;
    padding: 0;
    max-width: 700px;
    width: 100%;
    max-height: calc(var(--app-vh, 1vh) * 80);
    box-shadow: 0 10px 30px var(--shadow-drop);
    display: flex;
    flex-direction: column;
}

.instruction-modal-enter-active,
.instruction-modal-leave-active {
    transition: opacity 0.2s ease;
}

.instruction-modal-enter-active .instruction-modal-large,
.instruction-modal-leave-active .instruction-modal-large {
    transition: transform 0.2s ease-out, opacity 0.2s ease-out;
}

.instruction-modal-enter-from,
.instruction-modal-leave-to {
    opacity: 0;
}

.instruction-modal-enter-from .instruction-modal-large,
.instruction-modal-leave-to .instruction-modal-large {
    transform: translateY(20px);
    opacity: 0;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border);
}

.modal-header h3 {
    margin: 0;
    font-size: 1.2em;
    font-weight: 600;
    color: var(--text);
}

.modal-close {
    background: none;
    border: none;
    font-size: 20px;
    cursor: pointer;
    color: var(--text-muted);
    padding: 0;
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: color 0.2s ease;
}

.modal-close:hover {
    color: var(--text);
}

.instruction-content {
    padding: 20px;
    overflow-y: auto;
    flex: 1;
}

.instruction-content .text-constructor-content :deep(h1) {
    color: var(--text) !important;
}

.instruction-content .text-constructor-content :deep(h2) {
    color: var(--text) !important;
}

.instruction-content .text-constructor-content :deep(h3) {
    color: var(--text) !important;
}

.instruction-content .text-constructor-content :deep(p) {
    color: var(--text) !important;
}

.instruction-content .text-constructor-content :deep(li) {
    color: var(--text) !important;
}

.no-instruction {
    text-align: center;
    padding: 40px 20px;
    color: var(--text-muted);
}

.no-instruction-icon {
    font-size: 48px;
    margin-bottom: 16px;
    opacity: 0.5;
}

.no-instruction h4 {
    margin: 0 0 12px 0;
    color: var(--text);
    font-size: 1.2em;
}

.no-instruction p {
    margin: 0 0 8px 0;
    line-height: 1.5;
}

.blue {
    color: var(--accent-text);
}

@media (max-width: 768px) {
    .fact-section {
        flex-direction: column;
    }
    
    .fact-hint-card {
        flex: none;
        width: 100%;
        overflow: visible;
    }

    .hint-content {
        position: static;
        inset: auto;
        overflow: visible;
    }
    
    .tables__header {
        flex-direction: column;
        align-items: flex-start;
        gap: 10px;
    }
    
    .tables__filters {
        flex-direction: column;
        gap: 15px;
        align-items: flex-start;
    }
    
    .filters__fields {
        flex-wrap: wrap;
    }
    
    .field {
        width: 100%;
    }
    
    .instruction-modal-large {
        margin: 10px;
        max-height: 90vh;
    }
}
</style>