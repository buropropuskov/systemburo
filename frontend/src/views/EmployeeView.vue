<template>
  <section
    ref="root"
    class="employeesview"
    data-testid="employees-page"
  >
    <header class="employeesview__header">
      <h2 class="employeesview__title">
        Список <span class="blue">сотрудников</span>
      </h2>
      <p class="employeesview__subtitle">
        Вкладка для просмотра сотрудников, которых вы или ваша организация/компания когда-либо привязывали к заявкам.
      </p>
    </header>

    <!-- Десктоп: поиск и табы области над карточкой. На мобилке этот блок физически
         переезжает под шапку списка (.employeesview__toolbar) - через v-if, а не
         скрытой копией: иначе data-testid и якорь тура задвоились бы в DOM. -->
    <div
      v-if="!isNarrow"
      class="employeesview__filters"
    >
      <div
        class="filters-container"
        data-testid="ob-employees-filters"
      >
        <SearchComponent
          v-model="searchQuery"
          :title="'Поиск сотрудников...'"
        />
        <div
          v-if="ownershipInfo"
          class="filter-tabs"
        >
          <button
            v-if="ownershipInfo.has_organization && canSeeOrganization"
            class="filter-tab"
            data-testid="filter-tab-organization"
            :class="{ 'filter-tab--active': currentFilter === 'organization' }"
            title="Сотрудники, которых привязывали пользователи вашей организации"
            @click="switchFilter('organization')"
          >
            Сотрудники организации
          </button>
          <button
            v-if="ownershipInfo.has_company && canSeeCompany"
            class="filter-tab"
            data-testid="filter-tab-company"
            :class="{ 'filter-tab--active': currentFilter === 'company' }"
            title="Сотрудники, которых привязывали пользователи вашей компании"
            @click="switchFilter('company')"
          >
            Сотрудники компании
          </button>
          <button
            class="filter-tab"
            data-testid="filter-tab-user"
            :class="{ 'filter-tab--active': currentFilter === 'user' }"
            title="Только те сотрудники, которых привязывали лично вы"
            @click="switchFilter('user')"
          >
            Мои сотрудники
          </button>
          <button
            v-if="canSeeAllSystem"
            class="filter-tab"
            data-testid="filter-tab-all-system"
            :class="{ 'filter-tab--active': currentFilter === 'all_system' }"
            title="Все сотрудники, когда-либо зарегистрированные в системе"
            @click="switchFilter('all_system')"
          >
            Все сотрудники системы
          </button>
        </div>
      </div>
    </div>

    <!-- Мобилка: табы области в bottom-sheet. Место в разметке роли не играет -
         FilterSheet рендерится через BaseModal, а тот телепортирует себя в body. -->
    <FilterSheet
      v-if="isNarrow"
      :show="showScopeSheet"
      :has-active-filters="scopeFilterActive"
      @close="showScopeSheet = false"
      @reset="resetScopeFilter"
    >
      <div
        v-if="ownershipInfo"
        class="filter-section"
      >
        <span class="filter-label">Область</span>
        <div class="filter-tabs">
          <button
            v-if="ownershipInfo.has_organization && canSeeOrganization"
            class="filter-tab"
            data-testid="employees-scope-organization"
            :class="{ 'filter-tab--active': currentFilter === 'organization' }"
            @click="switchScopeFromSheet('organization')"
          >
            Сотрудники организации
          </button>
          <button
            v-if="ownershipInfo.has_company && canSeeCompany"
            class="filter-tab"
            data-testid="employees-scope-company"
            :class="{ 'filter-tab--active': currentFilter === 'company' }"
            @click="switchScopeFromSheet('company')"
          >
            Сотрудники компании
          </button>
          <button
            class="filter-tab"
            data-testid="employees-scope-user"
            :class="{ 'filter-tab--active': currentFilter === 'user' }"
            @click="switchScopeFromSheet('user')"
          >
            Мои сотрудники
          </button>
          <button
            v-if="canSeeAllSystem"
            class="filter-tab"
            data-testid="employees-scope-all-system"
            :class="{ 'filter-tab--active': currentFilter === 'all_system' }"
            @click="switchScopeFromSheet('all_system')"
          >
            Все сотрудники системы
          </button>
        </div>
      </div>
    </FilterSheet>

    <div class="employeesview__container">
      <!-- Таблица сотрудников -->
      <div
        class="employees-card"
        data-testid="ob-employees-table"
      >
        <div
          class="card-header"
          data-testid="ob-employees-table-head"
        >
          <div class="card-header__title">
            <h3 class="card-title">
              <span
                v-if="currentFilter === 'organization'"
                class="highlight-text"
              >Сотрудники <span class="blue">организации</span></span>
              <span
                v-else-if="currentFilter === 'company'"
                class="highlight-text"
              >Сотрудники <span class="blue">компании</span></span>
              <span
                v-else-if="currentFilter === 'all_system'"
                class="highlight-text"
              >Все <span class="blue">сотрудники системы</span></span>
              <span
                v-else
                class="highlight-text"
              >Мои <span class="blue">сотрудники</span></span>
            </h3>
            <!-- Счётчик записей рядом с заголовком - только на мобилке: на десктопе
                 то же число уже стоит в футере таблицы («Показано X из Y»). -->
            <span
              v-if="isNarrow"
              class="card-header__count"
              data-testid="employees-count-badge"
            >{{ employeesTotal }}</span>
          </div>
          <div class="card-header__settings">
            <!-- На мобилке «Добавить» переезжает в панель у нижнего края экрана
                 (.employeesview__action-bar), в шапке остаётся одно действие. -->
            <button
              v-if="!isNarrow && currentFilter !== 'all_system' && canWriteEmployees"
              class="add-button"
              data-testid="ob-employees-add-button"
              @click="showAddEmployeeModal"
            >
              Добавить
            </button>
            <!-- Журнал реестра открыт администратору: только там видно, кем и когда
                 удалена запись - самой строки в реестре уже нет. -->
            <button
              v-if="canManageAllEntities"
              class="log-button"
              data-testid="employees-registry-log"
              @click="showRegistryLog = true"
            >
              Журнал
            </button>
            <RefreshButton
              :loading="loading"
              @refresh="fetchEmployees"
            />
          </div>
        </div>

        <!-- Мобилка: поиск и «Фильтр» отдельной полосой 36px под шапкой экрана. -->
        <div
          v-if="isNarrow"
          class="employeesview__toolbar"
          data-testid="ob-employees-filters"
        >
          <SearchComponent
            v-model="searchQuery"
            :title="'Поиск сотрудников...'"
          />
          <FilterButton
            v-if="ownershipInfo"
            :active="scopeFilterActive"
            data-testid="employees-filter-btn"
            @click="showScopeSheet = true"
          />
        </div>

        <div class="card-content rt-table">
          <!-- Заголовок таблицы всегда отображается (на мобилке скрыт rt-head-row, строки -> карточки) -->
          <div class="employees-header rt-head-row">
            <div class="header-row">
              <div
                class="header-col number-col"
                @click="sortBy('id')"
              >
                <p :class="{ 'active-sort': sortField === 'id' }">
                  №
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'id',
                    'desc': sortField === 'id' && sortDirection === 'desc'
                  }"
                />
              </div>
              <div
                class="header-col name-col"
                @click="sortBy('last_name')"
              >
                <p :class="{ 'active-sort': sortField === 'last_name' }">
                  ФИО
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'last_name',
                    'desc': sortField === 'last_name' && sortDirection === 'desc'
                  }"
                />
              </div>
              <div
                class="header-col position-col"
                @click="sortBy('position')"
              >
                <p :class="{ 'active-sort': sortField === 'position' }">
                  Должность
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'position',
                    'desc': sortField === 'position' && sortDirection === 'desc'
                  }"
                />
              </div>
              <div
                class="header-col status-col"
                @click="sortBy('status')"
              >
                <p :class="{ 'active-sort': sortField === 'status' }">
                  Статус
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'status',
                    'desc': sortField === 'status' && sortDirection === 'desc'
                  }"
                />
              </div>
              <div
                v-if="currentFilter === 'organization' || currentFilter === 'all_system'"
                class="header-col org-col"
                @click="sortBy('organization_name')"
              >
                <p :class="{ 'active-sort': sortField === 'organization_name' }">
                  Организация
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'organization_name',
                    'desc': sortField === 'organization_name' && sortDirection === 'desc'
                  }"
                />
              </div>
              <div
                v-if="currentFilter === 'company' || currentFilter === 'all_system'"
                class="header-col company-col"
                @click="sortBy('company_name')"
              >
                <p :class="{ 'active-sort': sortField === 'company_name' }">
                  Компания
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'company_name',
                    'desc': sortField === 'company_name' && sortDirection === 'desc'
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
            <div class="employees-table-area">
              <div
                v-if="loading"
                class="loading-message"
              >
                <LoaderSpinner label="Загрузка сотрудников…" />
              </div>
              <div
                v-else-if="sortedEmployees.length > 0"
                ref="employeesBody"
                class="employees-body"
              >
                <div
                  v-for="(employee) in sortedEmployees"
                  :key="employee.id"
                  class="employee-item"
                >
                  <div
                    class="employee-row rt-row"
                    title="Открыть детали сотрудника"
                    @click="openEmployeeDetails(employee)"
                  >
                    <div
                      class="employee-col number-col"
                      data-label="№"
                    >
                      {{ employee.id }}
                    </div>
                    <div
                      class="employee-col name-col"
                      data-label="ФИО"
                      :title="formatFullName(employee)"
                    >
                      <span class="cell-text">{{ formatFullName(employee) }}</span>
                    </div>
                    <div
                      class="employee-col position-col"
                      data-label="Должность"
                      :title="employee.position || 'Не указана'"
                    >
                      <span class="cell-text">{{ employee.position || 'Не указана' }}</span>
                    </div>
                    <div
                      class="employee-col status-col"
                      data-label="Статус"
                    >
                      <StatusBadge
                        v-if="isEmployeeBlacklisted(employee)"
                        status="Чёрный список"
                      />
                      <StatusBadge
                        v-else
                        :status="employee.status ? 'Активен' : 'Неактивен'"
                      />
                    </div>
                    <div
                      v-if="currentFilter === 'organization' || currentFilter === 'all_system'"
                      class="employee-col org-col"
                      data-label="Организация"
                      :title="employee.organization_name || ''"
                    >
                      {{ employee.organization_name || '—' }}
                    </div>
                    <div
                      v-if="currentFilter === 'company' || currentFilter === 'all_system'"
                      class="employee-col company-col"
                      data-label="Компания"
                      :title="employee.company_name || ''"
                    >
                      {{ employee.company_name || '—' }}
                    </div>
                    <div class="employee-col actions-col">
                      <button
                        v-if="showEditEmployee(employee)"
                        class="edit-btn"
                        title="Изменить"
                        @click.stop="editEmployee(employee)"
                      >
                        <AppIcon
                          name="edit"
                          class="edit-icon"
                        />
                        <span class="action-btn__label">Изменить</span>
                      </button>
                      <button
                        v-if="showDeleteEmployee(employee)"
                        class="delete-btn"
                        title="Удалить"
                        @click.stop="deleteEmployee(employee)"
                      >
                        <AppIcon
                          name="trashcan"
                          class="delete-icon"
                        />
                        <span class="action-btn__label">Удалить</span>
                      </button>
                      <span
                        v-if="!showEditEmployee(employee) && !showDeleteEmployee(employee)"
                        class="read-only-text"
                        :title="canEditTooltip(employee)"
                      >
                        Только просмотр
                      </span>
                    </div>
                  </div>
                </div>

                <!-- Бесшовная подгрузка (#1158): sentinel внизу СКРОЛЛИРУЕМОГО
                     employees-body - IntersectionObserver триггерит loadMore без
                     кнопки "Показать ещё". -->
                <div
                  v-if="hasMoreEmployees"
                  :ref="setEmployeesSentinelRef"
                  class="scroll-sentinel"
                  data-testid="employees-scroll-sentinel"
                >
                  <LoaderSpinner
                    v-if="listLoading"
                    label="Загрузка…"
                  />
                  <!-- Ошибка догрузки следующей порции (#1173): список уже частично
                       загружен, автодогрузка остановлена circuit-breaker'ом. -->
                  <div
                    v-else-if="listError"
                    class="sentinel-error"
                    data-testid="employees-scroll-sentinel-error"
                  >
                    <span>Не удалось загрузить ещё</span>
                    <button
                      type="button"
                      class="lk-button lk-button--secondary lk-button--sm"
                      :disabled="listLoading"
                      @click="retryEmployees"
                    >
                      {{ listLoading ? 'Повтор…' : 'Повторить' }}
                    </button>
                  </div>
                </div>
              </div>
              <!-- In-flight retry при пустом списке (#1173): пока listLoading -
                   спиннер, не проваливаемся в error/"Сотрудников нет". listLoading
                   выставляет composable из retry() (this.loading он не трогает). -->
              <div
                v-else-if="listLoading"
                class="loading-message"
                data-testid="employees-list-loading"
              >
                <LoaderSpinner label="Загрузка…" />
              </div>
              <!-- Первичная загрузка упала (#1173): список пуст из-за ошибки бэка, а
                   не потому что сотрудников реально нет. -->
              <div
                v-else-if="listError"
                class="list-error-state"
                data-testid="employees-list-error"
              >
                <p>Не удалось загрузить сотрудников. Проверьте соединение.</p>
                <button
                  type="button"
                  class="lk-button lk-button--secondary"
                  :disabled="listLoading"
                  @click="retryEmployees"
                >
                  {{ listLoading ? 'Повтор…' : 'Повторить' }}
                </button>
              </div>
              <p
                v-else
                class="no-data-message"
              >
                {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Сотрудников нет' }}
              </p>
            </div>
            <div
              v-if="!loading && sortedEmployees.length"
              class="table-footer"
              data-testid="employees-table-footer"
            >
              {{ footerText }}
            </div>
          </div>
        </div>
      </div>
            
      <div class="employeesview__right-side">
        <div class="employeesview__help">
          <template v-if="currentFilter === 'organization'">
            <p class="help__text">
              Здесь находятся сотрудники, привязанные к вашей <strong class="blue">организации</strong>. Вы можете использовать этих сотрудников при подаче заявок на пропуск.
            </p>
            <p class="help__text">
              Новые сотрудники попадают в этот список <strong class="blue">автоматически</strong>, при подаче заявки.
            </p>
          </template>
          <template v-else-if="currentFilter === 'company'">
            <p class="help__text">
              Здесь находятся сотрудники, привязанные к вашей <strong class="blue">компании</strong>. Вы можете использовать этих сотрудников при подаче заявок на пропуск.
            </p>
            <p class="help__text">
              Новые сотрудники попадают в этот список <strong class="blue">автоматически</strong>, при подаче заявки.
            </p>
          </template>
          <template v-else-if="currentFilter === 'user'">
            <p class="help__text">
              Здесь находятся <strong class="blue">ваши сотрудники</strong>, добавленные лично. Вы можете использовать их при подаче заявок на пропуск.
            </p>
            <p class="help__text">
              Новые сотрудники попадают в этот список <strong class="blue">автоматически</strong>, при подаче заявки.
            </p>
          </template>
          <template v-else-if="currentFilter === 'all_system'">
            <p
              v-if="canManageAllEntities"
              class="help__text"
            >
              Здесь отображаются <strong class="blue">все сотрудники</strong>, которые есть в системе. Как администратор вы можете изменить или удалить любую запись, к какой бы организации она ни была привязана. Добавлять сотрудников нужно на вкладках выше - там видно, за кем закрепится запись.
            </p>
            <p
              v-else
              class="help__text"
            >
              Здесь отображаются <strong class="blue">все сотрудники</strong>, которые есть в системе. В этой вкладке доступен только просмотр, добавление, редактирование и удаление сотрудников недоступно.
            </p>
          </template>
        </div>
      </div>
    </div>

    <!-- Мобилка: главное действие экрана - широкая кнопка в панели, прижатой к низу
         экрана, у пальца. data-bottom-action-bar - контракт для ScrollTopButton:
         кнопка «наверх» поднимается над панелью, чтобы не лечь на «Добавить». -->
    <div
      v-if="isNarrow && currentFilter !== 'all_system' && canWriteEmployees"
      class="employeesview__action-bar"
      data-bottom-action-bar
    >
      <button
        class="add-button add-button--wide"
        data-testid="ob-employees-add-button"
        @click="showAddEmployeeModal"
      >
        Добавить сотрудника
      </button>
    </div>

    <RegistryLogModal
      :show="showRegistryLog"
      entity="employees"
      @close="showRegistryLog = false"
    />

    <EmployeeEditModal
      :visible="showModal"
      :editing-employee="editingEmployee"
      :citizenships="availableCitizenships"
      :ownership-info="ownershipInfo"
      :foreign-record="!!editingEmployee && !employeeBelongsToUser(editingEmployee)"
      @saved="onEmployeeSaved"
      @close="closeModal"
    />

    <EmployeeDetailsModal
      :show="showDetailsModal"
      :employee="detailsEmployee"
      :all-tables="[]"
      :current-user-id="ownershipInfo?.user_id || null"
      :current-user-name="''"
      source="employeesview"
      @close="closeDetailsModal"
      @open-application="handleOpenApplication"
    />

    <ApplicationDetail
      v-if="showApplicationDetail"
      :application="selectedApplication"
      :current-user-id="ownershipInfo?.user_id || null"
      :current-user-name="''"
      :mode="'center'"
      @close="closeApplicationDetail"
      @application-changed="fetchEmployees"
    />
  </section>
</template>

<script>
import { readSearchFromRoute, writeSearchToRoute } from '@/utils/searchQueryParam';
import { apiRequest } from '@/api/client'
import { getViewportZoom } from '@/utils/viewportScale'
import { getUniqueEmployeesPaginated } from '@/api/employees'
import { useInfiniteList } from '@/composables/useInfiniteList'
import { useApplicationDetailLink } from '@/composables/useApplicationDetailLink'
import { openItemFromRoute } from '@/utils/openQueryParam'
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';
import { usePermissionsStore } from '@/stores/permissions';
import SearchComponent from '@/components/SearchComponent.vue';
import FilterButton from '@/components/ui/FilterButton.vue';
import FilterSheet from '@/components/ui/FilterSheet.vue';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import RefreshButton from '@/components/RefreshButton.vue';
import RegistryLogModal from '@/components/RegistryLogModal.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import StatusBadge from '@/components/ui/StatusBadge.vue';
import EmployeeEditModal from '@/components/EmployeeEditModal.vue';
import EmployeeDetailsModal from '@/components/CreateApplication/EmployeeDetailsModal.vue';
import ApplicationDetail from '@/components/ApplicationDetail/ApplicationDetail.vue';
import AppIcon from '@/components/icons/AppIcon.vue';

// Размер порции бесшовной подгрузки реестра сотрудников (#1158, срез 3) - аналог
// CARS_PER_PAGE в CarsView.
const EMPLOYEES_PER_PAGE = 30;

export default {
    components: {
        SearchComponent,
        FilterButton,
        FilterSheet,
        RefreshButton,
        RegistryLogModal,
        LoaderSpinner,
        StatusBadge,
        EmployeeEditModal,
        EmployeeDetailsModal,
        ApplicationDetail,
        AppIcon,
    },
    setup() {
        // Бесшовная подгрузка реестра сотрудников порциями (#1158, срез 3): composable
        // инкапсулирует page/per_page/аккумуляцию/hasMore/seq-guard, тот же паттерн,
        // что useInfiniteList в CarsView. employeesData - алиас infiniteList.items:
        // pre-existing спека (EmployeeViewPermissionGating) пишет wrapper.vm.employeesData
        // напрямую, переименование сломало бы её без пользы.
        const infiniteList = useInfiniteList({ perPage: EMPLOYEES_PER_PAGE });
        // Мобилка (<=768): табы области сворачиваются в кнопку «Фильтр» + FilterSheet
        // (эпик mobile-filter-collapse, S3); десктоп-табы остаются инлайн.
        const { isNarrow } = useNarrowScreen();
        return {
            isNarrow,
            ...useApplicationDetailLink(),
            employeesData: infiniteList.items,
            employeesTotal: infiniteList.total,
            employeesPage: infiniteList.page,
            hasMoreEmployees: infiniteList.hasMore,
            // canLoadMoreEmployees/listError/retryEmployeesList (#1173) - устойчивость
            // бесшовной подгрузки к ошибкам бэка (5xx/сеть): canLoadMore гейтит АВТОдогрузку
            // (observer + loadAllRemaining), hasMoreEmployees по-прежнему гейтит видимость
            // sentinel-контейнера (внутри него рисуется error+retry).
            canLoadMoreEmployees: infiniteList.canLoadMore,
            listLoading: infiniteList.loading,
            listError: infiniteList.error,
            loadEmployeesList: infiniteList.load,
            loadMoreEmployeesList: infiniteList.loadMore,
            retryEmployeesList: infiniteList.retry,
            observeEmployeesSentinel: infiniteList.observeSentinel,
            disconnectEmployeesSentinel: infiniteList.disconnectObserver,
        };
    },
    data() {
        return {
            loading: true,
            // Из адреса: переход из сквозного поиска приносит запрос с собой,
            // и список должен уйти на сервер сразу с ним.
            searchQuery: readSearchFromRoute(this.$route),
            sortField: null,
            sortDirection: 'desc',
            // employeesData/employeesTotal/hasMoreEmployees/listLoading выставлены из
            // useInfiniteList в setup() (#1158, срез 3).
            searchTimeout: null,
            // seq-guard (#632/#1158): смена фильтра/поиска до резолва предыдущего
            // fetchEmployees не должна запускать/продолжать устаревший
            // loadAllRemainingEmployees.
            fetchSeq: 0,
            currentFilter: 'user',
            // Мобилка: bottom-sheet с табами области (S3 эпика mobile-filter-collapse).
            showScopeSheet: false,
            ownershipInfo: null,
            showModal: false,
            showRegistryLog: false,
            availableCitizenships: [],
            editingEmployee: null,
            showDetailsModal: false,
            detailsEmployee: null,
        };
    },
    computed: {
        // Вкладка «Сотрудники организации» (раздел реестра по организации).
        canSeeOrganization() {
            return usePermissionsStore().hasPermission('section.registry.organization');
        },
        // Вкладка «Сотрудники компании» (раздел реестра по компании).
        canSeeCompany() {
            return usePermissionsStore().hasPermission('section.registry.company');
        },
        // Вкладка «Все сотрудники системы» (all_system) - по разделу каталога;
        // супер/админ проходят, обычный юзер без гранта не видит. Бэк дополнительно
        // отдаёт 403 на all_system без прав.
        canSeeAllSystem() {
            return usePermissionsStore().hasPermission('section.registry.all_system');
        },
        // Право изменять реестр сотрудников (кнопки «Добавить»/«Редактировать»).
        // Базовая роль выдаёт его по умолчанию; админ может отозвать ролью.
        canWriteEmployees() {
            return usePermissionsStore().hasPermission('entity.employees.write');
        },
        // Право удалять из реестра сотрудников (кнопка «Удалить»). Базовая роль
        // выдаёт по умолчанию; админ может отозвать ролью, не затрагивая изменение.
        // Администратор системы: правит и удаляет запись независимо от привязки.
        // Признак приходит из ownership-info - того же ответа, по которому решает
        // бэкенд, иначе кнопка появилась бы там, где сервер отвечает 403.
        canManageAllEntities() {
            return this.ownershipInfo?.can_manage_all === true;
        },
        canDeleteEmployees() {
            return usePermissionsStore().hasPermission('entity.employees.delete');
        },
        // Поиск по тексту выполняется на бэке через search_query (#1158, срез 3) -
        // здесь не дублируем, employeesData уже отфильтрован сервером.
        sortedEmployees() {
            const employees = [...this.employeesData];

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
                        
                    case 'status':
                        valueA = a.status;
                        valueB = b.status;
                        break;

                    case 'organization_name':
                        valueA = (a.organization_name || '').toLowerCase();
                        valueB = (b.organization_name || '').toLowerCase();
                        break;

                    case 'company_name':
                        valueA = (a.company_name || '').toLowerCase();
                        valueB = (b.company_name || '').toLowerCase();
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

        // Область отличается от дефолтной («Мои сотрудники») - точка-индикатор на кнопке
        // «Фильтр» и доступность «Сбросить» в мобильном bottom-sheet (S3). Поиск сюда
        // не входит: он остаётся снаружи sheet (в шапке), как в S1/S2.
        scopeFilterActive() {
            return this.currentFilter !== 'user';
        },

        // Сортировка по колонкам - клиентская и должна идти по ВСЕМУ набору (как на
        // dev до пагинации), а не по одной загруженной порции: при активной сортировке
        // догружаем остаток (см. loadAllRemainingEmployees, #1158). Других клиентских
        // фильтров не осталось (поиск и filter_type - серверные), поэтому isFullLoad
        // зависит только от sortField.
        isFullLoad() {
            return !!this.sortField;
        },

        // Футер "Показано X из Y": клиентских фильтров, урезающих employeesData, не
        // осталось (сортировка не убирает строки), поэтому shown всегда равен total
        // загруженных, а "из employeesTotal" - серверному счётчику всех совпадений.
        footerText() {
            return `Показано ${this.sortedEmployees.length} из ${this.employeesTotal}`;
        }
    },
    watch: {
        // Пользователь уже на странице: mounted не перевызовется, а адрес сменился.
        '$route.query.open'(val) { if (val) this.openFromSearchLink(); },
        // Поиск - на сервере (#1158, срез 3): дебаунс 300мс перед fetchEmployees
        // (reset на стр.1 + очистка аккумулятора уже даёт loadEmployeesList({reset:true})).
        searchQuery(val) {
            writeSearchToRoute(this.$router, this.$route, val);
            clearTimeout(this.searchTimeout);
            this.searchTimeout = setTimeout(() => {
                this.fetchEmployees();
            }, 300);
        }
    },
    async mounted() {
        await Promise.all([
            this.fetchOwnershipInfo(),
            this.fetchCitizenships()
        ]);
        await this.fetchEmployees();
        this.openFromSearchLink();
        this._lastHeight = -1;
        this.$nextTick(this._applyHeight);
        window.addEventListener('resize', this._applyHeight);
        const header = document.querySelector('.theheader');
        if (header && typeof ResizeObserver !== 'undefined') {
            this._headerObs = new ResizeObserver(this._applyHeight);
            this._headerObs.observe(header);
        }
    },
    beforeUnmount() {
        this.disconnectEmployeesSentinel();
        if (this.searchTimeout) {
            clearTimeout(this.searchTimeout);
        }
        window.removeEventListener('resize', this._applyHeight);
        if (this._headerObs) {
            this._headerObs.disconnect();
            this._headerObs = null;
        }
    },
    methods: {
        /** Переход из сквозного поиска: `?q` сузил список, `?open` раскрывает карточку. */
        openFromSearchLink() {
            openItemFromRoute({ router: this.$router, route: this.$route, items: this.employeesData, open: this.openEmployeeDetails });
        },

        /**
         * Тянет страницу на доступную высоту вьюпорта (под шапкой), чтобы таблица
         * занимала весь экран без скролла страницы. На мобильном (<=768px) сбрасываем:
         * там естественный поток и горизонтальный скролл таблицы.
         */
        _applyHeight() {
            const el = this.$refs.root;
            if (!el) return;
            if (window.innerWidth <= 768) {
                el.style.height = '';
                this._lastHeight = -1;
                return;
            }
            // rect.top под корневым zoom - device-px, innerHeight - НЕзумленный;
            // делим на zoom, чтобы высота была в layout-px (иначе на мониторах >1440
            // контейнер выходит в zoom раз ниже экрана). См. AdminPageShell/AccountComponent.
            const top = el.getBoundingClientRect().top;
            const height = Math.max(0, Math.round((window.innerHeight - top) / getViewportZoom()));
            if (height === this._lastHeight) return;
            this._lastHeight = height;
            el.style.height = `${height}px`;
        },
        /**
         * Можно ли редактировать/удалять сотрудника. Совпадает с backend
         * canEditEmployee (unique_employee_service.go): администратор системы правит
         * любую запись, остальные - свою, своей организации или своей компании.
         */
        canEditEmployee(emp) {
            if (this.canManageAllEntities) return true;
            if (this.currentFilter === 'all_system') return false;
            return this.employeeBelongsToUser(emp);
        },
        /**
         * Сотрудник «свой»: запись автора, его организации или компании. Отдельно от
         * canEditEmployee, потому что администратору право даёт роль, а карточке правки
         * нужна именно принадлежность - чтобы не переписать чужую привязку своей.
         */
        employeeBelongsToUser(emp) {
            if (!this.ownershipInfo) return false;
            if (emp.user_id != null && emp.user_id === this.ownershipInfo.user_id) return true;
            if (emp.organization_id != null && this.ownershipInfo.organization_id != null
                && emp.organization_id === this.ownershipInfo.organization_id) return true;
            if (emp.company_id != null && this.ownershipInfo.company_id != null
                && emp.company_id === this.ownershipInfo.company_id) return true;
            return false;
        },
        showEditEmployee(emp) {
            return this.canEditEmployee(emp) && this.canWriteEmployees;
        },
        showDeleteEmployee(emp) {
            return this.canEditEmployee(emp) && this.canDeleteEmployees;
        },
        canEditTooltip(emp) {
            if (this.currentFilter === 'all_system' && !this.canManageAllEntities) return 'В режиме «Все в системе» редактирование доступно только администратору';
            if (!this.canEditEmployee(emp)) return 'Сотрудник не привязан к вашей организации/компании - редактирование запрещено';
            return 'Недостаточно прав для изменения или удаления';
        },
        isEmployeeBlacklisted(employee) {
            return employee.is_blacklisted === true;
        },

        async fetchEmployees() {
            const seq = ++this.fetchSeq;
            this.loading = true;
            try {
                await this.loadEmployeesList(this.buildEmployeesPage, { reset: true });
                if (seq !== this.fetchSeq) return; // устарел - актуальный запрос уже идёт

                // Клиентская сортировка требует ВЕСЬ набор (как на dev до пагинации):
                // догружаем оставшиеся порции, чтобы сортировка шла по полному списку (#1158).
                if (this.isFullLoad) {
                    await this.loadAllRemainingEmployees(seq);
                }
            } catch (error) {
                console.error("Ошибка при загрузке сотрудников:", error);
            } finally {
                if (seq === this.fetchSeq) this.loading = false;
            }
        },

        // Догрузка всех оставшихся порций (full-load режим: активная клиентская
        // сортировка, #1158). seq-guard прерывает устаревший проход, если пользователь
        // сменил фильтр/поиск и стартовал новый fetchEmployees; guard - от бесконечного
        // цикла, если total/hasMore разъедутся.
        async loadAllRemainingEmployees(seq) {
            let guard = 0;
            // canLoadMoreEmployees (не hasMoreEmployees, #1173): при ошибке бэка на
            // промежуточной странице circuit-breaker останавливает цикл сразу, не дожидаясь
            // guard>200.
            while (this.canLoadMoreEmployees && seq === this.fetchSeq) {
                await this.loadMoreEmployeesList(this.buildEmployeesPage);
                if (++guard > 200) break;
            }
        },

        /**
         * fetchPage для useInfiniteList (#1158): строит параметры текущего
         * фильтра/поиска плюс page/per_page - бэк переключается на GetAllPaginated,
         * как только видит per_page (internal/handlers/unique_employees.go).
         */
        async buildEmployeesPage(page, perPage) {
            const params = { filter_type: this.currentFilter, page, per_page: perPage };
            if (this.searchQuery.trim()) {
                params.search_query = this.searchQuery.trim();
            }
            const { items, meta } = await getUniqueEmployeesPaginated(params);
            return { items, total: (meta && meta.total) || 0 };
        },

        // Автодогрузка следующей порции по пересечению sentinel с employees-body (#1158).
        // root - сам .employees-body: у него свой overflow-y:auto, не документ,
        // дефолтный root (viewport) пересечение бы не заметил. el=null (v-if=
        // "hasMoreEmployees"===false) просто отключает observer.
        setEmployeesSentinelRef(el) {
            this.observeEmployeesSentinel(el, this.buildEmployeesPage, { root: this.$refs.employeesBody || null });
        },

        // Ручной повтор упавшей страницы (первичной или догрузки, #1173) - composable
        // сам помнит, какой fetchPage/режим (reset/append) последним завершился ошибкой.
        async retryEmployees() {
            try {
                await this.retryEmployeesList();
                // full-load (клиентская сортировка): retry вернул только упавшую
                // страницу, но сортировка идёт по ВСЕМУ набору - дозагружаем остаток,
                // иначе результат по НЕПОЛНОМУ списку до ручного доскролла (#1173).
                if (this.isFullLoad) {
                    await this.loadAllRemainingEmployees(this.fetchSeq);
                }
            } catch (error) {
                console.error("Ошибка сети при повторной попытке загрузки сотрудников:", error);
            }
        },

        async fetchOwnershipInfo() {
            try {
                const response = await apiRequest("/unique-employees/ownership-info", {
                    method: "GET"});

                if (response.ok) {
                    this.ownershipInfo = await response.json();
                } else {
                    // Если эндпоинт не существует, используем эндпоинт для машин (они используют одну логику)
                    const carResponse = await apiRequest("/unique-cars/ownership-info", {
                        method: "GET"});
                    
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
                const response = await apiRequest("/citizenships", {
                    method: "GET"});

                if (response.ok) {
                    this.availableCitizenships = await response.json();
                }
            } catch (error) {
                console.error("Ошибка при загрузке гражданств:", error);
            }
        },

        async deleteEmployee(employee) {
            const fullName = this.formatFullName(employee);
            const ok = await useUiStore().confirm({
                title: 'Удаление сотрудника',
                message: `Вы уверены, что хотите удалить сотрудника ${fullName}?`,
                confirmText: 'Удалить',
                danger: true,
            });
            if (!ok) return;
            try {
                const response = await apiRequest(`/unique-employees/${employee.id}`, {
                    method: "DELETE"});

                if (response.ok) {
                    await this.fetchEmployees();
                    useDeletionsStore().notify({
                        prefix: 'Сотрудник ',
                        bold: fullName,
                        suffix: ' удалён',
                    });
                } else {
                    useDeletionsStore().notify({ prefix: 'Ошибка при удалении сотрудника', type: 'error' });
                }
            } catch (error) {
                console.error("Ошибка при удалении сотрудника:", error);
                useDeletionsStore().notify({ prefix: 'Ошибка при удалении сотрудника', type: 'error' });
            }
        },

        editEmployee(employee) {
            this.editingEmployee = employee;
            this.showModal = true;
        },

        sortBy(field) {
            if (this.sortField === field) {
                this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
            } else {
                this.sortField = field;
                this.sortDirection = 'desc';
            }
            // Сортировка клиентская - должна идти по всему набору. Если ещё не всё
            // загружено, догружаем остаток (тот же паттерн, что в CarsView, #1158).
            if (this.isFullLoad && this.hasMoreEmployees) {
                this.fetchEmployees();
            }
        },

        switchFilter(filterType) {
            this.currentFilter = filterType;
            this.fetchEmployees();
        },

        // Мобильный bottom-sheet: выбор области применяется и закрывает лист (одиночный
        // выбор, как пикер), поиск остаётся снаружи (S3 эпика mobile-filter-collapse).
        switchScopeFromSheet(filterType) {
            this.switchFilter(filterType);
            this.showScopeSheet = false;
        },

        // «Сбросить фильтры» в sheet - вернуть дефолтную область «Мои сотрудники» и закрыть
        // лист. Кнопка активна только при scopeFilterActive, лишнего fetch на дефолте нет.
        resetScopeFilter() {
            this.switchFilter('user');
            this.showScopeSheet = false;
        },

        openEmployeeDetails(employee) {
            // EmployeeDetailsModal читает snake_case (last_name, position, ...)
            // и поддерживает source=employeesview - заголовок \"Информация о сотруднике\"
            this.detailsEmployee = {
                id: employee.id,
                // id заявочной строки активной заявки; по нему карточка тянет статус
                // территории (current-status ключуется по employees.id, не по реестру).
                activeEmployeeId: employee.active_employee_id || null,
                // id самой заявки - для кнопки "Открыть заявку" (open-application).
                applicationId: employee.active_application_id || null,
                last_name: employee.last_name,
                first_name: employee.first_name,
                middle_name: employee.middle_name,
                position: employee.position,
                citizenshipName: employee.citizenship_name,
                passport_series_number: employee.passport_series_number,
                patent_number: employee.patent_number,
                other_permission: employee.other_permission,
                organization: employee.active_app_org_name || employee.organization_name,
                company: employee.active_app_company_name || employee.company_name,
                entry_date_to: employee.active_entry_date_to,
                pass_time: employee.active_pass_time,
                isActive: employee.status,
                // Логин владельца сервер отдаёт только администратору, поэтому карточка
                // рисует строку по факту наличия значения, а не по своей проверке роли.
                user_name: employee.user_name || null,
                pd_consent_at: employee.pd_consent_at || null,
                target_tables: []
            };
            this.showDetailsModal = true;
        },
        closeDetailsModal() {
            this.showDetailsModal = false;
            this.detailsEmployee = null;
        },

        showAddEmployeeModal() {
            this.editingEmployee = null;
            this.showModal = true;
        },

        closeModal() {
            this.showModal = false;
            this.editingEmployee = null;
        },

        onEmployeeSaved() {
            this.fetchEmployees();
        },

        // Форматирование ФИО
        formatFullName(employee) {
            const parts = [];
            if (employee.last_name) parts.push(employee.last_name);
            if (employee.first_name) parts.push(employee.first_name);
            if (employee.middle_name) parts.push(employee.middle_name);
            return parts.join(' ') || 'Не указано';
        },

    }
}
</script>

<style scoped>
.employeesview {
    padding: 20px;
    display: flex;
    flex-direction: column;
}

.employeesview__container {
    display: flex;
    gap: 30px;
    margin-top: 20px;
    flex: 1;
    min-height: 0;
}

.employeesview__right-side {
    width: 25%;
}

.employeesview__help {
    /* Карточка-подсказка лежит на фоне страницы и несёт его же цвет без этой строки. */
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 15px;
    padding: 16px 20px;
}

.employeesview__header {
    padding-bottom: 15px;
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.employeesview__title {
    font-size: 18px;
    display: inline-flex;
    align-items: center;
    gap: 6px;
}

.employeesview__subtitle {
    font-size: 13px;
    color: var(--color-text-muted, var(--text-muted));
    margin: 0;
}

.employeesview__filters {
    padding-bottom: 15px;
    width: 100%;
    border-bottom: 1px solid var(--border);
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
    border: 1px solid var(--border);
    background: var(--surface);
    border-radius: 50px;
    cursor: pointer;
    font-size: 14px;
    transition: all 0.2s;
    height: 30px;
}

.filter-tab:hover {
    border-color: var(--accent);
}

.filter-tab--active {
    background: var(--accent);
    color: var(--accent-contrast);
    border-color: var(--accent);
}

.blue {
    color: var(--accent-text);
}

/* Стили для таблицы */
.employees-card {
    background-color: var(--surface);
    border-radius: 30px;
    border: 1px solid var(--border);
    overflow: hidden;
    width: 75%;
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
}

.card-header {
    border-bottom: 1px solid var(--border);
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0px 20px;
    height: 40px;
    flex-shrink: 0;
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

/* Кнопка журнала стоит в шапке между «Добавить» и «Обновить», поэтому повторяет их
   мерки: высота 25px, радиус 50px, текст 12px. Общий .lk-button здесь выбивался из
   ряда - он крупнее и с другим радиусом. */
.log-button {
    height: 25px;
    padding: 0 12px;
    border-radius: 50px;
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--text);
    font-size: 12px;
    line-height: 1;
    cursor: pointer;
    transition: background-color 0.2s;
}

.log-button:hover {
    background: var(--surface-2);
}

.add-button {
    background: var(--accent);
    color: var(--accent-contrast);
    border: none;
    border-radius: 15px;
    padding: 6px 12px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.add-button:hover:not(:disabled) {
    background: var(--accent-hover);
}

.add-button:disabled {
    background: var(--text-muted);
    cursor: not-allowed;
    opacity: 0.6;
}

.card-title {
    margin: 0;
    color: var(--text);
    font-weight: 600;
    font-size: 1.0em;
}

.highlight-text {
    color: var(--text);
}

.card-content {
    padding: 0;
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
}

.employees-container {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    overflow-y: auto;
}

/* employees-header повторяет геометрию employees-body (padding-right + margin-right 4px),
   чтобы доступная ширина колонок совпала и заголовки выровнялись с данными. */
.employees-header {
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
    padding-right: 4px;
    margin-right: 4px;
}

/* header-row повторяет геометрию employee-row: padding 12/16 + flex. */
.header-row {
    padding: 12px 16px;
    display: flex;
    width: 100%;
    align-items: center;
}

.header-col {
    font-weight: 500;
    color: var(--text-muted);
    text-align: left;
    padding: 0 8px;
    font-size: 14px;
    display: flex;
    align-items: center;
    gap: 5px;
    transition: .2s;
    cursor: pointer;
    user-select: none;
}

.header-col:hover {
    color: var(--text);
}

.header-col:hover .sort-icon {
    color: var(--text);
}

.sort-icon {
    color: var(--text-muted);
    width: 12px;
    height: 12px;
    transition: .2s;
}

.sort-icon.sorted {
    color: var(--text);
}

.sort-icon.desc {
    transform: rotate(180deg);
}

.active-sort {
    color: var(--text) !important;
    font-weight: 500 !important;
}

/* Колонки с фиксированной шириной */
.number-col {
    width: 8%;
    min-width: 40px;
}

.name-col {
    width: 35%;
    min-width: 200px;
}

.position-col {
    width: 25%;
    min-width: 150px;
}

.status-col {
    width: 20%;
    min-width: 135px;
}

.actions-col {
    width: 15%;
    min-width: 100px;
    justify-content: center;
}

.org-col {
    width: 18%;
    min-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.company-col {
    width: 18%;
    min-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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
    background-color: var(--surface-2);
}

.employee-row {
    display: flex;
    width: 100%;
    padding: 10px 16px;
    align-items: center;
    border-bottom: 1px solid var(--border);
    cursor: pointer;
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
    min-width: 0;
}

/* Текст ячейки усекается ellipsis только при реальной нехватке места
   (на display:flex-контейнере ellipsis работает только через вложенный блок). */
.employee-col .cell-text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
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
    background: color-mix(in srgb, var(--accent) 22%, var(--surface));
    border-radius: 3px;
    border: 1px solid transparent;
    background-clip: content-box;
    transition: all 0.3s ease;
}

.employees-body::-webkit-scrollbar-thumb:hover {
    background: color-mix(in srgb, var(--accent) 22%, var(--surface));
    border: 1px solid transparent;
    background-clip: content-box;
    transform: scale(1.1);
}

.employees-body {
    scrollbar-width: thin;
    scrollbar-color: color-mix(in srgb, var(--accent) 22%, var(--surface)) transparent;
    scroll-behavior: smooth;
    overscroll-behavior: contain;
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
    background-color: var(--surface-2);
}

.delete-btn:hover {
    background-color: var(--surface-2);
}

.edit-icon, .delete-icon {
    color: var(--text);
    width: 16px;
    height: 16px;
    opacity: 0.7;
    transition: opacity 0.2s ease;
}

.edit-btn:hover .edit-icon,
.delete-btn:hover .delete-icon {
    opacity: 1;
}

/* Подпись кнопки действия. На десктопе кнопка остаётся иконкой, поэтому подпись прячем
   clip-приёмом, а не display: none - она служит доступным именем кнопки (у иконки alt
   пустой, она декоративная). На мобилке подпись показывается вместо иконки. */
.action-btn__label {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
}

.read-only-text {
    font-size: 12px;
    color: var(--text-muted);
    font-style: italic;
}

.no-data-message {
    text-align: center;
    color: var(--text-muted);
    padding: 40px 20px;
    margin: 0;
    font-size: 14px;
    flex-grow: 1;
    display: flex;
    align-items: center;
    justify-content: center;
}

/* Обёртка тела таблицы (заменила SkeletonTransition): держит flex-контекст, чтобы
   .employees-body растягивался и скроллился, а кольцо/пусто центрировались по высоте. */
.employees-table-area {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
}

/* Бесшовная подгрузка (#1158): sentinel внизу .employees-body, футер под таблицей. */
.scroll-sentinel {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 24px;
    padding: 10px 0;
}

/* Устойчивость к ошибкам бэка (#1173): первичная загрузка упала - список пуст,
   вместо "Сотрудников нет" показываем причину + retry. */
.list-error-state {
    text-align: center;
    color: var(--danger-text);
    padding: 40px 20px;
    margin: 0;
    font-size: 14px;
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
}

.list-error-state p {
    margin: 0;
}

/* Кнопка повтора - единственное действие на пустом экране, поэтому добираем ей норму
   тач-таргета здесь, а не общим правилом пилюли (то раздувает кнопки всей системы). */
.list-error-state .lk-button {
    min-height: 36px;
}

/* Ошибка догрузки следующей порции (#1173) - компактный вариант рядом с sentinel. */
.sentinel-error {
    display: flex;
    align-items: center;
    gap: 10px;
    color: var(--danger-text);
    font-size: 13px;
}

.table-footer {
    flex-shrink: 0;
    padding: 10px 20px;
    border-top: 1px solid var(--border);
    font-size: 13px;
    color: var(--text-muted);
}

.loading-message {
    text-align: center;
    color: var(--text-muted);
    padding: 40px 20px;
    font-size: 14px;
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
}

.help__text {
    line-height: 150%; font-size: 14px;
}

@media (max-width: 768px) {
    .employees-card {
        width: 100%;
        flex: none;
    }

    /* Таблица сотрудников -> карточки через rt-* (responsive-tables.css, брейкпоинт 767.98).
       Прежний горизонтальный скролл 700px убран: строки собираются в карточки, заголовок
       скрыт (rt-head-row). Зазор между карточками - ниже, в блоке 767.98 (сиблинг
       .rt-row+.rt-row не сработает: rt-row на .employee-row, вложенном в .employee-item). */

    /* Заголовок страницы с подзаголовком на мобилке не показываем: то же название
       («Мои сотрудники») несёт шапка списка, а поясняющий абзац дублирует блок
       подсказки под таблицей. Освобождённые ~60px уходят под сами карточки. */
    .employeesview__header {
        display: none;
    }

    /* Поиск и «Фильтр» - отдельная полоса 36px под шапкой списка. Разметка гейтится
       isNarrow (768), поэтому и правила стоят здесь, а не в блоке 767.98: иначе ровно
       на 768.0 полоса отрендерилась бы без своих стилей.

       Боковой отступ - 8px, столько же, сколько у .employees-body: рамка поиска встаёт
       ровно над рамкой первой карточки. Прежние 12px давали третий уступ между шапкой
       и списком. */
    .employeesview__toolbar {
        display: flex;
        flex-shrink: 0;
        align-items: center;
        gap: 8px;
        padding: 8px;
        border-bottom: 1px solid var(--border);
    }

    .employeesview__toolbar :deep(.search) {
        flex: 1 1 auto;
        width: auto;
        min-width: 0;
        height: 36px;
    }

    /* Счётчик записей рядом с заголовком экрана. */
    .card-header__count {
        display: inline-flex;
        flex-shrink: 0;
        align-items: center;
        justify-content: center;
        min-width: 22px;
        height: 22px;
        padding: 0 7px;
        border-radius: var(--radius-pill);
        background: var(--accent-tint);
        color: var(--accent-text);
        font-size: 12px;
        font-weight: 700;
    }

    /* Панель главного действия у нижнего края. fixed, а не sticky: панель обязана
       держаться у нижней кромки видимой области независимо от прокрутки, поэтому
       нижний отступ страницы резервирует её высоту (8 + 44 + 12 = 64px), иначе
       последняя карточка уезжает под панель. */
    .employeesview__action-bar {
        position: fixed;
        right: 0;
        bottom: 0;
        left: 0;
        z-index: 90;
        display: flex;
        gap: 8px;
        padding: 8px var(--gutter) calc(12px + env(safe-area-inset-bottom, 0px));
        background: var(--surface);
        border-top: 1px solid var(--border);
    }

    .add-button--wide {
        flex: 1;
        height: 44px;
        padding: 0 16px;
        border-radius: var(--radius-pill);
        font-size: 15px;
        font-weight: 700;
    }

    /* .filter-tabs/.filter-tab на мобилке рендерятся ТОЛЬКО внутри FilterSheet
       (десктоп-табы скрыты v-if="!isNarrow"). Правила через data-v достают до
       телепортнутого контента sheet: каждый таб на всю ширину строкой. */
    .filter-tabs {
        flex-wrap: wrap;
        gap: 10px;
    }

    .filter-tab {
        flex: 1 1 100%;
        white-space: nowrap;
        text-align: center;
    }

    /* Отступы страницы по токену --gutter (12px на <=768, 10px на <=480). Хардкод
       20px съедал 40px ширины: на 320px шапка карточки не помещалась в строку в
       принципе, из-за чего кнопки и уезжали во вторую. Сама шапка - ниже, в блоке
       767.98 (переключается вместе с телом таблицы). */
    .employeesview {
        padding: var(--gutter);
        padding-bottom: calc(64px + var(--gutter) + env(safe-area-inset-bottom, 0px));
    }

    .employeesview__container {
        flex-direction: column;
    }

    .employeesview__right-side {
        width: 100%;
    }
}

/* Шапка блока и карточки сотрудников на мобилке. Порог 767.98 - тот же, на котором
   таблица превращается в карточки (responsive-tables.css) и на котором RefreshButton
   сворачивается в иконку: шапка и тело переключаются вместе, гибрида на 768.0 нет.
   Зазор между карточками отдельным правилом (не через .rt-row+.rt-row): rt-row висит на
   .employee-row, вложенном в .employee-item (v-for-обёртку). */
@media (max-width: 767.98px) {
    /* Прокручивается страница, и только она (#1097 волна 6).
       Список лежал в трёх вложенных областях прокрутки подряд: .card-content ->
       .employees-container -> .employees-body. Палец попадал в них, а не в документ, и
       экран стоял на месте - пользователь описал это как «скроллю вниз, он пытается
       проскроллить список и стоит». Проверено настоящим тач-драгом (CDP
       Input.dispatchTouchEvent): до правки страница не двигалась вовсе при документе
       1312px в окне 844. Замер window.scrollTo этого НЕ ловит - он двигает документ
       мимо жеста и показывает ложное «работает». */
    .card-content,
    .employees-container,
    .employees-table-area,
    .employees-body {
        height: auto !important;
        max-height: none !important;
        overflow: visible !important;
    }

    /* Шапка экрана одной строкой 48px: заголовок, счётчик записей, «Обновить».
       «Добавить» отсюда ушло вниз, в панель у пальца - в шапке рядом с заголовком
       место есть ровно на одно действие.

       Боковой отступ собран из слагаемых, а не записан числом: заголовок обязан
       стоять на той же вертикали, что текст карточек списка, а тот отбит от рамки
       панели отступом тела (8) + рамкой карточки (1) + её внутренним отступом
       (14 - глобальный responsive-tables.css). Прежние 12px ставили заголовок на
       11px левее текста карточек - шапка читалась прижатой к краю. */
    .card-header {
        flex-wrap: nowrap;
        gap: 8px;
        height: 48px;
        padding: 0 calc(8px + 1px + 14px);
    }

    /* min-width: 0 обязателен: flex-элемент не сжимается ниже min-content, и длинный
       заголовок («Все сотрудники системы») выдавил бы кнопки за край экрана вместо
       того, чтобы обрезаться многоточием. */
    .card-header__title {
        flex: 1 1 auto;
        min-width: 0;
    }

    /* 18px - кегль имени экрана в проекте (.employeesview__title, .carsview__title,
       .center__title). На мобилке заголовок страницы скрыт, и имя экрана несёт именно
       эта строка, поэтому она берёт его размер, а не уменьшенный табличный. */
    .card-title {
        min-width: 0;
        overflow: hidden;
        font-size: 18px;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .card-header__settings {
        flex-shrink: 0;
        gap: 6px;
    }

    /* Своих правил для «Обновить» шапка больше не держит: RefreshButton на этом же пороге
       сам сворачивается в круг 36px с иконкой. Прежние оверрайды возвращали текстовую
       пилюлю 88px, а на <=480 гасили саму стрелку - её пропажу и забраковали. */

    /* Одинаковый зазор вокруг карточек: у .employees-body асимметрия от десктопного
       скроллбара (padding-right + margin-right по 4px), из-за неё слева карточки
       упирались в рамку блока, а справа висел отступ 8px. */
    .employees-body {
        padding: 8px;
        margin-right: 0;
    }

    .employees-body .employee-item + .employee-item {
        margin-top: 8px;
    }

    /* Карточка по образцу среза 2 (ApplicationAttachmentDetail.vue): подписи полей убраны,
       значения выровнены влево, порядковый номер на мобилке не показываем вовсе.
       !important обязателен: правило-источник в responsive-tables.css стоит на той же
       специфичности (0,3,0), а оба view - lazy route-чанки, их scoped-CSS грузится позже
       глобального, и при равной специфичности исход решал бы порядок загрузки. */
    .employee-row.rt-row > .number-col {
        display: none !important;
    }

    .employee-row.rt-row > [data-label]::before {
        display: none !important;
    }

    /* Исключение из «убрать все подписи»: организация и компания идут двумя соседними
       строками с однотипными названиями (фильтр «Все сотрудники системы»), без подписи
       не различить, где какая. */
    .employee-row.rt-row > .org-col::before,
    .employee-row.rt-row > .company-col::before {
        display: block !important;
    }

    /* Поле карточки - строка не ниже 30px с содержимым по центру по вертикали: без
       общего минимума строки разной высоты (бейдж статуса против одной строки текста)
       давали рваную сетку, и список читался кашей.

       Разделитель полей рисуем СВЕРХУ у ячеек 2..N, а не снизу: последней в строке идёт
       колонка действий без data-label, глобальное `[data-label]:last-child` до неё не
       достаёт и пунктир висел бы оторванной чертой над нижним краем карточки.
       white-space/overflow-wrap - потому что .employee-col держит nowrap ради десктопной
       таблицы, и длинное ФИО или организация уезжали бы вправо за край.
       break-word, а не anywhere: anywhere разрешает браузеру считать разрыв ПОСЕРЕДИНЕ
       слова годной точкой при расчёте min-content для флекс-элемента, из-за чего значение
       рвалось прямо внутри слова на увеличенном системном шрифте (тот же класс дефекта,
       что «Российска|я Федераци|я» у карточки машины) - break-word ломает слово только
       как последний выход, когда для него в принципе нет места на строке. */
    .employee-row.rt-row > .employee-col {
        align-items: center !important;
        justify-content: flex-start !important;
        min-height: 30px;
        height: auto;
        border-bottom: none !important;
        text-align: left !important;
        white-space: normal;
        overflow-wrap: break-word;
    }

    /* Должность и статус делят одну строку. В колоночном стеке бейдж занимал собственную
       полосу, в которой кроме него ничего не было - пустая строка в каждой карточке, и
       читалось это как случайно уехавший элемент. Приём взят у карточек проходной
       (PeopleTable/CarsTable): карточка переводится из колонки в строку с переносом, каждой
       ячейке задаётся базис 100% (во флекс-строке перенос держит БАЗИС, а не width), а
       паре-исключению - ширина по содержимому.

       Специфичность выше правил-источников и !important обязательны: responsive-tables.css
       объявляет и flex-direction, и width со своим !important, коротким селектором их не
       перебить. */
    .rt-table .employee-row.rt-row {
        flex-direction: row !important;
        flex-wrap: wrap !important;
        column-gap: 8px;
        row-gap: 0;
    }

    .rt-table .employee-row.rt-row > * {
        flex: 0 0 100% !important;
        width: 100% !important;
        min-width: 0 !important;
    }

    /* Должность больше не делит строку с бейджем (тот переехал в подвал, см. .status-col
       ниже) - занимает свою строку целиком, как ФИО/организация/компания, отдельного
       правила ей не нужно.

       Бейдж статуса - в подвал карточки, одной строкой с кнопками «Изменить»/«Удалить»
       (разбор второго круга замечаний владельца, #1097 w8). Раньше бейдж делил строку с
       должностью: та несла пунктирную границу сверху, а бейдж - нет, но оба выравнивались
       по одной Y-координате через align-self: flex-start, поэтому бейдж вставал прямо на
       чужую границу («стоит поперёк»).

       align-self: flex-start, а не center (третий круг замечаний, #1097 w9): бейдж и
       кнопки - соседние ячейки одной обёрнутой flex-строки, их высоты по контенту не
       совпадают ровно (badge ~27px против пилюль-кнопок 28px). center центрирует каждую
       ячейку НЕЗАВИСИМО в высоте строки - верхние края (а с ними border-top) расходятся
       на разницу высот, и сплошная линия подвала «переламывается» ровно после бейджа.
       flex-start прижимает обе ячейки к верхнему краю строки - border-top гарантированно
       на одной Y без зависимости от разницы высот контента. */
    .rt-table .employee-row.rt-row > .status-col {
        order: 10;
        flex: 0 0 auto !important;
        width: auto !important;
        align-self: flex-start;
        margin-top: 2px;
        padding-top: 8px;
        border-top: 1px solid color-mix(in srgb, var(--border) 45%, var(--surface)) !important;
    }

    .employee-row.rt-row > .employee-col ~ .employee-col {
        border-top: 1px dashed color-mix(in srgb, var(--border) 60%, var(--surface));
    }

    /* Номер скрыт, но в DOM он первый - без сброса верхний пунктир достался бы ФИО и
       висел бы отдельной чертой у верхнего края карточки. */
    .employee-row.rt-row > .number-col + .employee-col {
        border-top: none;
    }

    .employee-row.rt-row > .employee-col .cell-text {
        overflow: visible;
        white-space: normal;
        text-overflow: clip;
    }

    /* Колонка действий не несёт data-label, поэтому держала бы desktop-ширину и обрезала
       бы кнопки / «Только просмотр». Подвал карточки отделён сплошной линией: пунктир
       остаётся разделителем полей, сплошная - границей данных и действий. !important
       нужен, чтобы перебить пунктир из сиблинг-правила выше: оно специфичнее (0,4,0
       против 0,3,0) и иначе выигрывает. order/flex ставят колонку действий ПОСЛЕ бейджа
       статуса в той же строке подвала (order: 11 > 10 у .status-col), а justify-content
       прижимает кнопки к правому краю, оставляя бейдж слева. */
    .employee-row.rt-row > .actions-col {
        order: 11;
        flex: 1 1 auto !important;
        width: auto !important;
        min-width: 0 !important;
        align-self: flex-start;
        /* Базовое правило поля карточки несёт flex-start с !important - без такой же
           пометки кнопки липнут к бейджу вместо правого края. */
        justify-content: flex-end !important;
        gap: 6px;
        margin-top: 2px;
        padding-top: 8px;
        border-top: 1px solid color-mix(in srgb, var(--border) 45%, var(--surface)) !important;
        /* Настоящая причина разрыва линии подвала: .status-col и .actions-col - два
           РАЗНЫХ flex-элемента, у каждого свой border-top, а между ними column-gap: 8px
           родителя (.employee-row.rt-row) - это пустое место без бордюра, в котором линия
           физически прерывается, хотя цвет/толщина границ совпадают. margin-left тянет
           бокс .actions-col (а с ним и border-top) вплотную к .status-col, закрывая зазор;
           кнопки внутри не сдвигаются - их прижимает вправо justify-content: flex-end. */
        margin-left: -8px;
    }

    /* Действия - компактные бейджи 28px в подвале карточки. Прежние пилюли 44px
       («жирные огромные») забирали половину карточки; зона нажатия остаётся 44px за счёт
       невидимого ::before (28 + 8 сверху + 8 снизу), то есть глаз видит бейдж, а палец
       по-прежнему не промахивается (эталон §8, тач-таргет >=44px). Классы .lk-button на
       разметку не вешаем: на десктопе это остаётся безрамочная иконка, поэтому
       pill-геометрию повторяем здесь, в мобильном блоке. */
    .employee-row.rt-row .edit-btn,
    .employee-row.rt-row .delete-btn {
        position: relative;
        height: 28px;
        margin: 0;
        padding: 0 10px;
        border: 1px solid var(--border);
        border-radius: var(--radius-pill);
        font-size: 12.5px;
        font-weight: 600;
        line-height: 1.2;
        white-space: nowrap;
    }

    .employee-row.rt-row .edit-btn::before,
    .employee-row.rt-row .delete-btn::before {
        content: "";
        position: absolute;
        inset: -8px -2px;
    }

    .employee-row.rt-row .edit-btn {
        color: var(--accent-text);
        border-color: var(--accent);
    }

    .employee-row.rt-row .delete-btn {
        color: var(--danger-text);
        border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
    }

    .employee-row.rt-row .edit-icon,
    .employee-row.rt-row .delete-icon {
        display: none;
    }

    .employee-row.rt-row .action-btn__label {
        position: static;
        width: auto;
        height: auto;
        margin: 0;
        overflow: visible;
        clip: auto;
        white-space: nowrap;
    }

    .employee-row.rt-row > .actions-col .read-only-text {
        white-space: normal;
    }
}

/* Очень узкие телефоны (--bp-mobile-sm = 480px): в шапке остались заголовок, счётчик и
   круг «Обновить» 36px, ужимать нечего - боковой отступ страницы уже меньше (--gutter
   10px), а сам ряд держит те же вертикали, что список. Кегль заголовка и отступы шапки
   здесь НЕ уменьшаем: разный размер имени экрана между 481 и 480 - тот самый разнобой,
   на который жаловался владелец («микроскопический шрифт»). На 320px заголовку остаётся
   320 - 2x(10+1+23) - 36 - 6 - счётчик - 6 = ~176px, длинное имя обрезается многоточием. */
@media (max-width: 480px) {
    .card-header {
        gap: 6px;
    }

    .card-header__settings {
        gap: 4px;
    }

    .employeesview__toolbar {
        gap: 6px;
    }
}
</style>