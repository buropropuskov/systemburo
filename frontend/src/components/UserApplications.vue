<template>
  <div class="applications-card">
    <div class="card-header">
      <div class="card-header__title">
        <h3 class="card-title">
          Список заявок
        </h3>
        
        <!-- Фильтр Мои/Организации - один выпадающий список (было 2 таба - каша). -->
        <BaseDropdown
          class="cabinet__filter-dropdown"
          :model-value="currentFilter"
          :options="filterOptions"
          value-key="key"
          label-key="label"
          @update:model-value="setFilter"
        />

        <!-- Чип "Обновления" (#1349): серверный фильтр status_updated=true (только
             заявки с обновлённым статусом) + счётчик из отдельного эндпоинта ЛК.
             Переиспользуем пилюлю .filter-tab (была под 2 таба Мои/Организации,
             заменённые дропдауном - стиль остался). -->
        <button
          type="button"
          class="filter-tab"
          :class="{ 'filter-tab--active': statusUpdatedOnly }"
          data-testid="lk-button-updates"
          @click="toggleStatusUpdated"
        >
          Обновления<template v-if="statusUpdateCount > 0">: {{ statusUpdateCount }}</template>
        </button>
      </div>
      
      <!-- Якорь тура на всю строку настроек, а не на сам поиск: на десктопе поиск -
           инпут SearchComponent, на <768 он схлопывается в иконку-тоггл, и шаг про
           поиск, привязанный к иконке, на десктопе просто не показался бы. -->
      <div
        class="card-header__settings"
        data-testid="ob-cabinet-search"
      >
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
          @clear="clearDateRange"
        />
        <!-- Десктоп: всегда-видимый инпут поиска. На мобилке он сплющивается -
             там его заменяет иконка-тоггл ниже (зеркало Центра, срез 3). -->
        <SearchComponent
          v-if="!isMobileHeader"
          v-model="searchQuery"
          :title="'Поиск заявок..'"
        />
        <RefreshButton @refresh="refreshApplications" />

        <!-- Мобилка: иконка-тоггл раскрывает поле поиска оверлеем влево (не всегда-видимый инпут). -->
        <button
          v-if="isMobileHeader"
          type="button"
          class="search-icon-btn"
          :class="{ 'search-icon-btn--active': showMobileSearch || !!searchQuery.trim() }"
          aria-label="Поиск заявок"
          data-testid="cabinet-search-icon"
          @click="toggleMobileSearch"
        >
          <AppIcon
            name="search"
            class="search-icon-btn__img"
          />
        </button>

        <!-- Мобилка: поле поиска раскрывается ВЛЕВО оверлеем поверх ряда настроек
             (дата/обновить), не отдельным рядом. Иконка справа - тоггл, крестик
             внутри - очистить и закрыть. -->
        <Transition name="cabinet-search">
          <div
            v-if="isMobileHeader && showMobileSearch"
            class="cabinet__search-overlay"
          >
            <div class="field search">
              <input
                ref="mobileSearchInput"
                v-model="searchQuery"
                placeholder="Поиск заявок.."
                type="text"
                class="cabinet__search-input"
                data-testid="cabinet-input-search"
              >
              <button
                v-if="searchQuery.trim()"
                type="button"
                class="cabinet__search-clear"
                aria-label="Очистить поиск"
                @click="clearMobileSearch"
              >
                &times;
              </button>
            </div>
          </div>
        </Transition>
      </div>
    </div>
    
    <div class="card-content">
      <div class="applications-container">
        <!-- Левая часть - таблица заявок -->
        <div class="applications-list rt-table">
          <!-- Заголовок таблицы -->
          <div
            class="applications-header"
            data-testid="ob-applications-head"
          >
            <div class="header-row rt-head-row">
              <div
                class="header-col id-col"
                @click="sortBy('application_number')"
              >
                <p :class="{ 'active-sort': sortField === 'application_number' }">
                  Номер заявки
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'application_number',
                    'desc': sortField === 'application_number' && sortDirection === 'desc'
                  }"
                />
              </div>
              <div
                class="header-col date-col"
                @click="sortBy('sending_datetime')"
              >
                <p :class="{ 'active-sort': sortField === 'sending_datetime' }">
                  Дата и время
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'sending_datetime',
                    'desc': sortField === 'sending_datetime' && sortDirection === 'desc'
                  }"
                />
              </div>
              <div
                class="header-col sender-col"
                @click="sortBy('sender_name')"
              >
                <p :class="{ 'active-sort': sortField === 'sender_name' }">
                  Отправитель
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'sender_name',
                    'desc': sortField === 'sender_name' && sortDirection === 'desc'
                  }"
                />
              </div>
              <div
                class="header-col confirmation-col"
                @click="sortBy('confirmation')"
              >
                <p :class="{ 'active-sort': sortField === 'confirmation' }">
                  Подтверждение
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'confirmation',
                    'desc': sortField === 'confirmation' && sortDirection === 'desc'
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
              <div class="header-col tags-col">
                <p>Теги</p>
              </div>
              <div class="header-col actions-col" />
            </div>
          </div>
          
          <div
            ref="applicationsBody"
            class="applications-body"
          >
            <!-- Спиннер ТОЛЬКО когда показывать нечего (первая загрузка/пустой список).
                 На обновлении список остаётся на месте: подмена его спиннером схлопывала
                 высоту документа, и страница прыгала в начало (обратная связь об
                 обновлении - точки перезарядки на самой кнопке). -->
            <div
              v-if="isLoading && !filteredApplications.length"
              class="loading-message"
            >
              <LoaderSpinner label="Загрузка заявок…" />
            </div>

            <template v-else>
              <div
                v-if="filteredApplications.length > 0"
                class="applications-list-content"
              >
                <transition-group
                  name="fade-list"
                  tag="div"
                  class="applications-transition-group"
                  @before-leave="pinLeavingElement"
                >
                  <template
                    v-for="group in applicationGroups"
                    :key="group.key"
                  >
                    <!-- Разделитель периода (серая линия + подпись), как в Центре. -->
                    <div
                      v-if="group.label"
                      :key="`${group.key}-sep`"
                      class="applications-day-separator"
                    >
                      <span class="applications-day-label">{{ group.label }}</span>
                    </div>
                    <div
                      v-for="application in group.apps"
                      :key="application.id"
                      class="application-item"
                      :class="{ 'status-updated': application.has_status_update }"
                      data-testid="ob-application-row"
                      @click="openApplication(application)"
                    >
                    <div class="application-row rt-row">
                      <div
                        class="application-col id-col"
                        data-label="Номер заявки"
                      >
                        <span
                          class="application-id application-number--copyable"
                          tabindex="0"
                          data-tooltip="Копировать"
                          @click.stop="copyApplicationNumber(application.application_number)"
                          @keydown.enter.prevent="copyApplicationNumber(application.application_number)"
                        >{{ application.application_number }}</span>
                      </div>
                      <div
                        class="application-col date-col"
                        data-label="Дата и время"
                      >
                        {{ formatDateTime(application.sending_datetime) }}
                      </div>
                      <div
                        class="application-col sender-col"
                        data-label="Отправитель"
                      >
                        <span class="ellip">{{ application.sender_name || application.sender_full_name || '—' }}</span>
                      </div>
                      <!-- Организация и сообщение - только в компактной карточке на мобилке
                           (W3.11); в десктоп-таблице отдельных колонок нет (base display:none). -->
                      <div
                        v-if="application.organization_name"
                        class="application-col organization-col"
                      >
                        <span class="ellip">{{ application.organization_name }}</span>
                      </div>
                      <div
                        v-if="application.message"
                        class="application-col message-col"
                      >
                        {{ messagePreview(application.message) }}
                      </div>
                      <div
                        class="application-col confirmation-col"
                        data-label="Подтверждение"
                      >
                        <span
                          class="confirmation-badge"
                          :class="getConfirmationClass(application.confirmation)"
                          :title="application.confirmation"
                        >
                          {{ application.confirmation }}
                        </span>
                      </div>
                      <div
                        class="application-col status-col"
                        data-label="Статус"
                      >
                        <span class="status-badge-wrap">
                          <span
                            class="status-badge"
                            :class="getStatusClass(application.status)"
                            :title="application.status"
                          >
                            {{ application.status }}
                          </span>
                          <!-- Пульс-точка "статус обновился" (#1349). В ЛК нет гейта
                               прочтения (у отправителя нет строк application_reads) -
                               показываем по одному флагу has_status_update. -->
                          <span
                            v-if="application.has_status_update"
                            class="status-update-dot"
                            :data-testid="`lk-status-dot-${application.id}`"
                            aria-hidden="true"
                          />
                        </span>
                      </div>
                      <div
                        class="application-col tags-col"
                        data-label="Теги"
                      >
                        <div
                          v-if="blacklistFlagCount(application) > 0 || application.has_roof_access || application.has_free_parking || application.has_unseen_questions || pendingApprovalDays(application) !== null"
                          class="application-tags"
                          :class="{
                            'application-tags--both': application.has_roof_access && application.has_free_parking,
                            'application-tags--chs': blacklistFlagCount(application) > 0
                          }"
                        >
                          <Badge
                            v-if="pendingApprovalDays(application) !== null"
                            variant="warning"
                            size="sm"
                            class="rt-tag rt-tag--awaiting tag-hint"
                            :data-hint="pendingApprovalLabel(pendingApprovalDays(application))"
                          >
                            <svg
                              class="rt-tag__icon rt-tag__icon--fixed"
                              width="13"
                              height="13"
                              viewBox="0 0 24 24"
                              fill="none"
                              stroke="currentColor"
                              stroke-width="2"
                              stroke-linecap="round"
                              stroke-linejoin="round"
                            ><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></svg>
                            <span class="rt-tag__text">{{ pendingApprovalShort(pendingApprovalDays(application)) }}</span>
                          </Badge>
                          <Badge
                            v-if="blacklistFlagCount(application) > 0"
                            variant="danger"
                            size="sm"
                            dot
                            class="rt-tag rt-tag--chs blacklist-flag-badge tag-hint"
                            :data-hint="blacklistFlagTitle()"
                          >
                            <span class="rt-tag__text">{{ blacklistFlagLabel(application) }}</span>
                          </Badge>
                          <Badge
                            v-if="application.has_roof_access"
                            variant="primary"
                            size="sm"
                            class="rt-tag rt-tag--roof tag-hint"
                            data-hint="Доступ на крышу"
                          >
                            <svg
                              class="rt-tag__icon"
                              width="13"
                              height="13"
                              viewBox="0 0 24 24"
                              fill="none"
                              stroke="currentColor"
                              stroke-width="2"
                              stroke-linecap="round"
                              stroke-linejoin="round"
                            ><path d="M3 11l9-7 9 7" /><path d="M5 10v9h14v-9" /></svg>
                            <span class="rt-tag__text">Крыша</span>
                          </Badge>
                          <Badge
                            v-if="application.has_free_parking"
                            variant="warning"
                            size="sm"
                            class="rt-tag rt-tag--parking tag-hint"
                            data-hint="Бесплатная парковка"
                          >
                            <svg
                              class="rt-tag__icon"
                              width="13"
                              height="13"
                              viewBox="0 0 24 24"
                              fill="none"
                              stroke="currentColor"
                              stroke-width="2.2"
                              stroke-linecap="round"
                              stroke-linejoin="round"
                            ><path d="M8 4h8a4 4 0 0 1 4 4v8a4 4 0 0 1-4 4H8a4 4 0 0 1-4-4V8a4 4 0 0 1 4-4z" /><path d="M9 16V8h3.2a2.4 2.4 0 0 1 0 4.8H9" /></svg>
                            <span class="rt-tag__text">Парковка</span>
                          </Badge>
                          <Badge
                            v-if="application.has_unseen_questions"
                            variant="primary"
                            size="sm"
                            class="rt-tag rt-tag--questions tag-hint"
                            data-hint="Есть новые сообщения в обсуждении"
                            :data-testid="`user-questions-badge-${application.id}`"
                          >
                            <svg
                              class="rt-tag__q-svg"
                              width="13"
                              height="13"
                              viewBox="0 0 24 24"
                              fill="none"
                              stroke="currentColor"
                              stroke-width="2"
                              stroke-linecap="round"
                              stroke-linejoin="round"
                            ><path d="M4 5.5h16a1 1 0 0 1 1 1v8.5a1 1 0 0 1-1 1H9.5L5.5 20v-3.5H4a1 1 0 0 1-1-1V6.5a1 1 0 0 1 1-1z" /></svg>
                            <span class="rt-tag__text">Обсуждение</span>
                            <span
                              class="rt-tag__q-dot"
                              aria-hidden="true"
                            />
                          </Badge>
                        </div>
                      </div>
                      <div class="application-col actions-col">
                        <button
                          v-if="application.has_blank_template"
                          class="download-btn"
                          data-testid="ob-application-download"
                          title="Скачать"
                          @click.stop="downloadApplication(application)"
                        >
                          Скачать
                        </button>
                      </div>
                      </div>
                    </div>
                  </template>
                </transition-group>
              </div>
              
              <!-- In-flight retry при пустом списке (#1173): пока listLoading -
                   спиннер, не проваливаемся в error/"Заявок нет". listLoading выставляет
                   composable из retry() (this.isLoading он не трогает). -->
              <div
                v-else-if="listLoading"
                class="loading-message"
                data-testid="user-applications-list-loading"
              >
                <LoaderSpinner label="Загрузка…" />
              </div>
              <!-- Первичная загрузка упала (#1173): список пуст из-за ошибки бэка, а
                   не потому что заявок реально нет. -->
              <div
                v-else-if="listError"
                class="list-error-state"
                data-testid="user-applications-list-error"
              >
                <p>Не удалось загрузить заявки. Проверьте соединение.</p>
                <button
                  type="button"
                  class="lk-button lk-button--secondary"
                  :disabled="listLoading"
                  @click="retryApplications"
                >
                  {{ listLoading ? 'Повтор…' : 'Повторить' }}
                </button>
              </div>
              <div
                v-else
                class="no-data-message"
              >
                <p>{{ hasActiveFilters ? 'Нет заявок по выбранным фильтрам' : 'Заявок нет' }}</p>
                <p
                  v-if="!hasActiveFilters"
                  class="hint"
                >
                  {{ getNoDataHint() }}
                </p>
              </div>
            </template>

            <!-- Бесшовная подгрузка (#1158 срез 4): sentinel внизу скроллируемого
                 applications-body - IntersectionObserver триггерит loadMore без кнопки.
                 root - сам applications-body (свой overflow-y:auto, не документ). -->
            <div
              v-if="hasMoreApplications"
              :ref="setSentinelRef"
              class="user-applications-sentinel"
              data-testid="user-applications-sentinel"
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
                data-testid="user-applications-sentinel-error"
              >
                <span>Не удалось загрузить ещё</span>
                <button
                  type="button"
                  class="lk-button lk-button--secondary lk-button--sm"
                  :disabled="listLoading"
                  @click="retryApplications"
                >
                  {{ listLoading ? 'Повтор…' : 'Повторить' }}
                </button>
              </div>
            </div>
          </div>

          <div
            v-if="!isLoading && sortedApplications.length"
            class="user-applications-footer"
            data-testid="user-applications-footer"
          >
            {{ footerText }}
          </div>
        </div>
      </div>
    </div>

    <teleport to="body">
      <ApplicationDetail
        v-if="showDetailModal"
        :application="selectedApplication"
        :current-user-id="currentUserId"
        :current-user-name="currentUserName"
        :mode="'user'"
        @close="closeApplicationDetail"
        @duplicate="handleDuplicate"
        @withdraw="handleWithdraw"
        @questions-read="onQuestionsRead"
        @download="downloadApplication"
      />
    </teleport>
    <DownloadBlanksModal
      :show="!!(showDownloadModal && downloadAppId)"
      :application-id="downloadAppId || 0"
      :application-info="downloadAppInfo"
      @close="showDownloadModal = false"
    />
  </div>
</template>

<script>
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { pinLeavingElement } from '@/utils/listTransition';
import { apiRequest } from '@/api/client'
import { getUserApplicationsPaginated, getApplicationById, getUserStatusUpdatesCount } from '@/api/applications'
import { useAuthStore } from '@/stores/auth'
import { useDeletionsStore } from '@/stores/deletions'
import { copyText } from '@/utils/clipboard'
import { useInfiniteList } from '@/composables/useInfiniteList'
import { useRevealFirstApplication } from '@/composables/useRevealFirstApplication'
import RefreshButton from './RefreshButton.vue';
import SearchComponent from './SearchComponent.vue';
import DateFilter from './DateFilter.vue';
import ApplicationDetail from './ApplicationDetail/ApplicationDetail.vue';
import DownloadBlanksModal from './applications/DownloadBlanksModal.vue';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import Badge from './ui/Badge.vue';
import BaseDropdown from './ui/BaseDropdown.vue';
import { blacklistFlagCount, blacklistFlagLabel, BLACKLIST_FLAG_TITLE } from '@/utils/blacklistBadge';
import { pendingApprovalDays, pendingApprovalLabel, pendingApprovalShort } from '@/utils/pendingApproval';
import { stripHtml } from '@/utils/sanitize';
import { groupApplicationsByPeriod } from '@/utils/applicationPeriod';
import { sortApplications } from '@/utils/applicationSort';
import AppIcon from '@/components/icons/AppIcon.vue';

// Размер порции бесшовной подгрузки ЛК (#1158 срез 4) - как в Центре заявок.
const USER_APPLICATIONS_PER_PAGE = 30;

export default {
  components: {
    RefreshButton,
    SearchComponent,
    DateFilter,
    ApplicationDetail,
    DownloadBlanksModal,
    LoaderSpinner,
    Badge,
    BaseDropdown,
    AppIcon,
  },
  props: {
    userOrganizationId: {
      type: Number,
      default: null
    },
    userId: {
      type: Number,
      default: null
    },
    userOrganization: {
      type: String,
      default: ""
    }
  },
  setup() {
    // Бесшовная подгрузка ЛК порциями (#1158 срез 4): composable инкапсулирует
    // page/per_page/аккумуляцию/hasMore/seq-guard. fetchPage строится в methods
    // (нужен доступ к this для фильтров) и передаётся при каждом вызове - setup()
    // не имеет доступа к this. applications - алиас infiniteList.items: существующий
    // deep-link спек читает/пишет wrapper.vm.applications напрямую (зеркало Центра #1163).
    const infiniteList = useInfiniteList({ perPage: USER_APPLICATIONS_PER_PAGE });
    return {
      applications: infiniteList.items,
      total: infiniteList.total,
      applicationsPage: infiniteList.page,
      hasMoreApplications: infiniteList.hasMore,
      // canLoadMoreApplications/listError/retryApplicationsList (#1173) - устойчивость
      // бесшовной подгрузки к ошибкам бэка (5xx/сеть): canLoadMore гейтит АВТОдогрузку
      // (observer + loadAllRemaining), hasMoreApplications по-прежнему гейтит видимость
      // sentinel-контейнера (внутри него рисуется error+retry).
      canLoadMoreApplications: infiniteList.canLoadMore,
      listLoading: infiniteList.loading,
      listError: infiniteList.error,
      loadApplicationsList: infiniteList.load,
      loadMoreApplicationsList: infiniteList.loadMore,
      retryApplicationsList: infiniteList.retry,
      observeApplicationsSentinel: infiniteList.observeSentinel,
      disconnectApplicationsSentinel: infiniteList.disconnectObserver,
    };
  },
  data() {
    return {
      selectedApplication: null,
      showDetailModal: false,
      responsibleUsers: [],
      searchQuery: '',
      searchDebounceTimer: null,
      sortField: null,
      sortDirection: 'desc',
      isLoading: false,
      currentFilter: 'my',
      currentUserId: null,
      currentUserName: '',
      showDownloadModal: false,
      downloadAppId: null,
      downloadAppInfo: null,
      selectedDate: null,
      dateRangeStart: null,
      dateRangeEnd: null,
      // Мобильная шапка (зеркало Центра, срез 3): на <=768 поиск раскрывается по иконке
      // оверлеем, а не всегда-видимым инпутом (он там сплющивается).
      isMobileHeader: false,
      showMobileSearch: false,
      // Чип "Обновления" (#1349): серверный фильтр только заявок с обновлённым статусом
      // + счётчик из GET /applications/user/status-updates-count (scope-wide, не из
      // загруженной порции - у ЛК свой эндпоинт без гейта прочтения).
      statusUpdatedOnly: false,
      statusUpdateCount: 0,
      // seq-токен (#632/#840): fetchUserApplications дёргается фильтрами/поиском/сменой
      // вкладки/сортировкой - управляет isLoading, отдельно от собственного seq-guard
      // items/total внутри useInfiniteList.
      fetchSeq: 0
    };
  },
  computed: {
    /**
     * Владелец списка ЛК. Пропс userId приходит из /users/me и на первых кадрах пуст,
     * а маркер доступа несёт тот же идентификатор (claim user_id) синхронно - поэтому
     * первый же запрос уходит со scope владельца. Без этого запрос уходил без
     * sender_user_id, бэк отдавал весь скоуп ЛК (свои + заявки организации), и чужие
     * строки успевали отрисоваться до перезапроса (#2218).
     *
     * Маркер важнее пропса: режим "войти как пользователь" подменяет маркер сразу, а
     * пропс до перечитывания /users/me держит прежнего человека - запрос всё равно
     * исполняется от личности маркера.
     */
    ownerUserId() {
      return useAuthStore().userId || this.userId || null;
    },

    /**
     * Известен ли scope выдачи для текущей вкладки. Пока неизвестен, запрос не уходит:
     * без sender_user_id/organization_id бэк отдаёт весь скоуп ЛК целиком (#2218).
     */
    hasApplicationsScope() {
      return this.currentFilter === 'organization'
        ? !!this.userOrganizationId
        : !!this.ownerUserId;
    },

    // Опции фильтра Мои/Организации для BaseDropdown (заменил 2 таба одним списком).
    filterOptions() {
      const opts = [{ key: 'my', label: 'Мои заявки' }];
      if (this.userOrganizationId) {
        opts.push({ key: 'organization', label: 'Заявки организации (отдела)' });
      }
      return opts;
    },
    // Поиск теперь СЕРВЕРНЫЙ (search_query уходит в buildUserApplicationsPage) - здесь
    // его больше не дублируем (#1158 срез 4): клиентский matchesSearch резал бы уже
    // подгруженную порцию по неточному совпадению с серверным fuzzy-поиском (та же
    // проблема, что чинили в Центре до #1163). Дата остаётся как редундантный клиентский
    // мирроринг серверного date_from/date_to (безопасен - точное совпадение, не сужает
    // относительно уже отфильтрованного сервером набора).
    filteredApplications() {
      let filtered = this.applications;

      if (this.selectedDate) {
        filtered = filtered.filter(app => {
          const appDate = new Date(app.sending_datetime);
          const filterDate = new Date(this.selectedDate);
          appDate.setHours(0, 0, 0, 0);
          filterDate.setHours(0, 0, 0, 0);
          return appDate.getTime() === filterDate.getTime();
        });
      } else if (this.dateRangeStart && this.dateRangeEnd) {
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

    hasActiveFilters() {
      return !!this.searchQuery ||
             !!this.selectedDate ||
             (this.dateRangeStart && this.dateRangeEnd);
    },

    // Сортировка по колонке - клиентская, поэтому должна идти по ВСЕМУ набору, не по
    // одной подгруженной порции (зеркало Центра #1158): при активном sortField
    // fetchUserApplications догружает остаток через loadAllRemaining.
    isFullLoad() {
      return !!this.sortField;
    },

    // Футер "Показано X из Y": X - реально видимое число строк (после клиентского
    // date-мирроринга), total - серверный счётчик по текущим фильтрам.
    showTotalInFooter() {
      return this.sortedApplications.length === this.applications.length;
    },

    footerText() {
      const shown = this.sortedApplications.length;
      return this.showTotalInFooter ? `Показано ${shown} из ${this.total}` : `Показано ${shown}`;
    },

    // Группировка по периодам для разделителей списка - как в Центре заявок
    // (общий utils/applicationPeriod). При сортировке НЕ по дате подачи порядок не
    // хронологический, поэтому разделители не рисуются.
    applicationGroups() {
      const sortedByDate = !this.sortField || this.sortField === 'sending_datetime';
      return groupApplicationsByPeriod(this.sortedApplications, sortedByDate);
    },
    sortedApplications() {
      return sortApplications(this.filteredApplications, this.sortField, this.sortDirection);
    }
  },
  watch: {
    searchQuery() {
      // Дебаунс (#1158 срез 4, зеркало Центра): поиск теперь бьёт по бэку на каждое
      // изменение - без дебаунса быстрый ввод плодит запрос на каждую букву.
      clearTimeout(this.searchDebounceTimer);
      this.searchDebounceTimer = setTimeout(() => {
        this.fetchUserApplications();
      }, 300);
    },
    ownerUserId() {
      // Владелец разрешился (или сменился - режим "войти как пользователь" подменяет
      // маркер): список перезагружается, тогда же пробуем открыть заявку из deep-link
      // (на холодной навигации mounted-попытка была с пустым списком). Обычно маркер и
      // /users/me дают один и тот же идентификатор - значение не меняется, лишнего
      // запроса нет.
      this.fetchUserApplications().then(() => this.openFromDeepLink());
    },
    // Переход из уведомления в кабинет: /personal-cabinet?open=<id> (#973).
    '$route.query.open'(val) {
      if (val) this.openFromDeepLink();
    },
  },
  // Онбординг просит показать карточку заявки (reveal.open) - деталь это модалка
  // внутри кабинета, а не роут, сам тур её открыть не может. Контракт общий с
  // Центром заявок, живёт в композабле.
  created() {
    this._tourReveal = useRevealFirstApplication({
      first: () => this.sortedApplications[0],
      isOpen: () => this.showDetailModal,
      open: (application) => this.openApplication(application),
      close: () => this.closeApplicationDetail(),
    });
  },
  mounted() {
    this.fetchUserApplications().then(() => this.openFromDeepLink());
    this.fetchStatusUpdateCount();
    this.getCurrentUser();
    this.initMobileWatcher();
  },
  beforeUnmount() {
    this.disconnectApplicationsSentinel();
    this._tourReveal?.stop();
    clearTimeout(this.searchDebounceTimer);
    if (this._mobileMql) {
      if (this._mobileMql.removeEventListener) {
        this._mobileMql.removeEventListener('change', this._onMobileChange);
      } else if (this._mobileMql.removeListener) {
        this._mobileMql.removeListener(this._onMobileChange);
      }
    }
    releaseBodyScrollLock(this);
  },
  methods: {
    pinLeavingElement,
    /**
     * Мобильный брейкпоинт: тот же 767.98, что у card-правил responsive-tables.css.
     * Порог держим равным CSS @media, иначе на ровно 768px (iPad-портрет) иконка
     * появилась бы без своих стилей оверлея (урок S8 про рассинхрон 768/767.98).
     */
    initMobileWatcher() {
      if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
      this._mobileMql = window.matchMedia('(max-width: 767.98px)');
      this.isMobileHeader = this._mobileMql.matches;
      this._onMobileChange = (e) => {
        this.isMobileHeader = e.matches;
        // Возврат на десктоп - гасим мобильное раскрытие поиска.
        if (!e.matches) this.showMobileSearch = false;
      };
      if (this._mobileMql.addEventListener) {
        this._mobileMql.addEventListener('change', this._onMobileChange);
      } else if (this._mobileMql.addListener) {
        this._mobileMql.addListener(this._onMobileChange);
      }
    },
    toggleMobileSearch() {
      this.showMobileSearch = !this.showMobileSearch;
      if (this.showMobileSearch) {
        this.$nextTick(() => {
          if (this.$refs.mobileSearchInput) this.$refs.mobileSearchInput.focus();
        });
      }
    },
    clearMobileSearch() {
      this.searchQuery = '';
      this.showMobileSearch = false;
    },
    /**
     * fetchPage для useInfiniteList (#1158 срез 4): строит query-параметры фильтра
     * (поиск, вкладка "Мои"/"Организация", дата) плюс page/per_page - бэк переключается
     * на GetUserApplicationsPaginated, как только видит per_page (см.
     * internal/handlers/applications.go GetUserApplications). currentFilter теперь
     * СЕРВЕРНЫЙ (sender_user_id/organization_id), а не клиентский .filter() по всему
     * массиву - раньше GetUserApplications вообще не ограничивал доступ по пользователю,
     * клиент лишь ОТОБРАЖАЛ подмножество (см. фикс applyUserApplicationsAccessFilter).
     */
    async buildUserApplicationsPage(page, perPage) {
      const params = {};

      if (this.searchQuery) {
        params.search_query = this.searchQuery;
      }

      if (this.currentFilter === 'my' && this.ownerUserId) {
        params.sender_user_id = this.ownerUserId;
      } else if (this.currentFilter === 'organization' && this.userOrganizationId) {
        params.organization_id = this.userOrganizationId;
      }

      // Чип "Обновления" (#1349): бэк фильтрует по hasStatusUpdatePredicate БЕЗ гейта
      // прочтения (applyStatusUpdatedFilter requireRead=false для ЛК).
      if (this.statusUpdatedOnly) {
        params.status_updated = 'true';
      }

      // Дата - ЛОКАЛЬНЫМИ частями (не toISOString: UTC-сдвиг увёл бы выбранный день
      // назад у пользователей восточнее UTC, #1076/#1158).
      if (this.selectedDate) {
        const day = this.toLocalYMD(this.selectedDate);
        params.date_from = day;
        params.date_to = day;
      } else if (this.dateRangeStart && this.dateRangeEnd) {
        params.date_from = this.toLocalYMD(this.dateRangeStart);
        params.date_to = this.toLocalYMD(this.dateRangeEnd);
      }

      params.page = page;
      params.per_page = perPage;

      const { items, meta } = await getUserApplicationsPaginated(params);
      return { items, total: (meta && meta.total) || 0 };
    },

    // Date -> YYYY-MM-DD по локальным частям (см. комментарий в buildUserApplicationsPage).
    toLocalYMD(date) {
      const d = date instanceof Date ? date : new Date(date);
      const pad = (n) => String(n).padStart(2, '0');
      return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
    },

    async fetchUserApplications() {
      const authStore = useAuthStore();
      if (!authStore.token) {
        console.error("Пользователь не авторизован.");
        return;
      }

      // Без scope запрос ушёл бы голым, а бэк на такой запрос отдаёт весь скоуп ЛК
      // (свои ИЛИ заявки организации, applyUserApplicationsAccessFilter). Раньше эта
      // выдача отрисовывалась и уезжала, когда резолвился /users/me (#2218): ждём scope
      // под спиннером, перезапрос сделает watcher ownerUserId.
      if (!this.hasApplicationsScope) {
        this.isLoading = true;
        return;
      }

      // seq-токен управляет isLoading, отдельно от собственного seq-guard items/total
      // внутри useInfiniteList (#632/#840): пачка вызовов (поиск/фильтр/сортировка)
      // не должна оставить isLoading залипшим на устаревшем запросе.
      const seq = ++this.fetchSeq;
      this.isLoading = true;
      try {
        await this.loadApplicationsList(this.buildUserApplicationsPage, { reset: true });
        if (seq !== this.fetchSeq) return; // устарел - актуальный запрос уже идёт

        // Сортировка по колонке - клиентская, должна идти по всему набору: догружаем
        // оставшиеся порции (зеркало Центра #1158).
        if (this.isFullLoad) {
          await this.loadAllRemaining(seq);
          if (seq !== this.fetchSeq) return;
        }
      } catch (error) {
        console.error("Ошибка сети при загрузке заявок:", error);
      } finally {
        if (seq === this.fetchSeq) this.isLoading = false;
      }
    },

    // Обновление по кнопке: перечитываем и список, и счётчик чипа "Обновления" (#1349).
    // Счётчик scope-wide (свой эндпоинт), от фильтров/поиска не зависит - поэтому не
    // дёргаем его на каждый search/date/sort, только на явном refresh, mount и withdraw.
    refreshApplications() {
      this.fetchUserApplications();
      this.fetchStatusUpdateCount();
    },

    // Счётчик чипа "Обновления" (#1349): число заявок ЛК с обновлённым статусом
    // (scope автора/организации, без гейта прочтения). При сбое сохраняем последнее
    // известное значение (не обнуляем - восстановление не должно выглядеть как
    // "обновления пропали").
    async fetchStatusUpdateCount() {
      try {
        const { status_updates } = await getUserStatusUpdatesCount();
        this.statusUpdateCount = status_updates || 0;
      } catch (error) {
        console.error('Не удалось загрузить счётчик обновлений статуса:', error);
      }
    },

    toggleStatusUpdated() {
      this.statusUpdatedOnly = !this.statusUpdatedOnly;
      this.fetchUserApplications();
    },

    // Догрузка всех оставшихся порций (full-load режим: сортировка по всему набору).
    // seq-guard - устаревший проход прекращается при старте нового fetchUserApplications.
    async loadAllRemaining(seq) {
      let guard = 0;
      // canLoadMoreApplications (не hasMoreApplications, #1173): при ошибке бэка на
      // промежуточной странице circuit-breaker останавливает цикл сразу, не дожидаясь
      // guard>200.
      while (this.canLoadMoreApplications && seq === this.fetchSeq) {
        await this.loadMoreApplicationsList(this.buildUserApplicationsPage);
        if (++guard > 200) break;
      }
    },

    // Автодогрузка следующей порции по пересечению sentinel с applications-body (#1158).
    // el=null при v-if="hasMoreApplications"===false просто отключает observer.
    setSentinelRef(el) {
      // root: на ДЕСКТОПЕ скроллпорт - .applications-body. На МОБИЛКЕ @media снимает
      // внутренний скролл (overflow-y:visible) и список скроллит документ: тогда
      // .applications-body НЕ скроллпорт, его прямоугольник равен всей высоте списка,
      // sentinel пересечён ВСЕГДА -> loadMore зацикливается (тот же баг чинили в
      // Центре, #1273). Поэтому на мобилке root=null - скроллер документ.
      const root = this.isMobileHeader ? null : (this.$refs.applicationsBody || null);
      this.observeApplicationsSentinel(el, this.buildUserApplicationsPage, { root });
    },

    // Ручной повтор упавшей страницы (первичной или догрузки, #1173) - composable
    // сам помнит, какой fetchPage/режим (reset/append) последним завершился ошибкой.
    async retryApplications() {
      try {
        await this.retryApplicationsList();
        // full-load (клиентская сортировка): retry вернул только упавшую страницу,
        // но сортировка идёт по ВСЕМУ набору - дозагружаем остаток, иначе результат
        // по НЕПОЛНОМУ списку до ручного доскролла (#1173).
        if (this.isFullLoad) {
          await this.loadAllRemaining(this.fetchSeq);
        }
      } catch (error) {
        console.error("Ошибка сети при повторной попытке загрузки заявок:", error);
      }
    },

    async fetchResponsibleUsers(applicationId) {
      try {
        const authStore = useAuthStore();
        if (!authStore.token) return;

        const response = await apiRequest(`/applications/${applicationId}/responsible-users`, {
          method: "GET",
          headers: {
            "Accept": "application/json"
          },
        });

        if (response.ok) {
          this.responsibleUsers = await response.json();
        } else {
          console.error("Failed to fetch responsible users:", response.status);
          this.responsibleUsers = [];
        }
      } catch (error) {
        console.error("Error fetching responsible users:", error);
        this.responsibleUsers = [];
      }
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

    // Сообщение заявки - rich-HTML из TextConstructor. В компактной карточке (мобилка)
    // показываем плоский текст одной строкой с обрезкой (без тегов).
    messagePreview(html) {
      return stripHtml(html);
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
        'Отказано': 'status-rejected',
        'Отозвана': 'status-rejected'
      };
      return statusClasses[status] || 'status-default';
    },

    blacklistFlagCount,
    blacklistFlagLabel,
    blacklistFlagTitle() {
      return BLACKLIST_FLAG_TITLE;
    },

    pendingApprovalDays,
    pendingApprovalLabel,
    pendingApprovalShort,

    getNoDataHint() {
      switch (this.currentFilter) {
        case 'my':
          return 'У вас ещё нет отправленных заявок';
        case 'organization':
          return 'Ваша организация ещё не отправляла заявки';
        default:
          return 'Заявок нет';
      }
    },

    async openApplication(application) {
      // Оптимистичное гашение флага "статус обновился" (#1349): открытие детали дёргает
      // GET /:id/details -> MarkStatusSeen (seen_at=now) гасит флаг на сервере. Точку и
      // счётчик чипа гасим сразу, не дожидаясь рефреша списка.
      if (application.has_status_update) {
        application.has_status_update = false;
        this.statusUpdateCount = Math.max(0, this.statusUpdateCount - 1);
      }

      // Маркер обсуждений гасит ПРОЧТЕНИЕ тем в детали (клик), не факт открытия (#973):
      // иконка обновится при следующей загрузке списка. Оптимистично не снимаем.
      this.selectedApplication = application;
      await this.fetchResponsibleUsers(application.id);
      this.showDetailModal = true;

      // Блокируем скролл body при открытии модального окна
      setBodyScrollLock(this, true);
    },

    // Всё обсуждение заявки прочитано в детали -> гасим маркер в списке оптимистично (#973).
    onQuestionsRead(applicationId) {
      const app = this.applications.find(a => a.id === applicationId);
      if (app) {
        app.has_unseen_questions = false;
      }
    },

    // Переход из уведомления: /personal-cabinet?open=<id> открывает заявку и чистит
    // query, чтобы обновление страницы её повторно не открывало (#973). Query чистим
    // ТОЛЬКО когда заявка реально найдена и открыта: на холодной навигации список
    // грузится после разрешения userId (AccountComponent), поэтому первый вызов может
    // прийти с пустым списком - тогда откроем после fetch по вотчеру userId.
    // Заявка может быть вне загруженных порций (страница 2+, другая вкладка/дата) -
    // при пагинации (#1158 срез 4) полагаться на присутствие в списке нельзя: если её
    // нет в накопленном - точечно догружаем по id (зеркало Центра #1163).
    async openFromDeepLink() {
      const openId = Number(this.$route.query.open);
      if (!openId) return;
      let app = this.applications.find(a => a.id === openId);
      if (!app) {
        try {
          const fetched = await getApplicationById(openId);
          // apiRequest на !success отдаёт {message}, без id - значит нет доступа/не
          // найдена: оставляем ?open, откроется при следующей попытке.
          if (fetched && fetched.id) app = fetched;
        } catch (e) {
          console.error('Не удалось загрузить заявку из deep-link:', e);
        }
      }
      if (!app) return;
      this.openApplication(app);
      const query = { ...this.$route.query };
      delete query.open;
      this.$router.replace({ query }).catch(() => {});
    },

    closeApplicationDetail() {
      this.showDetailModal = false;
      this.selectedApplication = null;
      this.responsibleUsers = [];
      // Деталь закрыли (крестик, Esc, дубликат) - тур больше не «владелец».
      this._tourReveal?.release();

      // Разблокируем скролл body при закрытии модального окна
      releaseBodyScrollLock(this);
    },

    handleDuplicate() {
      this.closeApplicationDetail();
      this.$router.push('/new-application');
    },

    // Заявка отозвана (#951) - деталь сама закрылась, обновляем список,
    // чтобы заявка переехала в завершённые с актуальным статусом "Отозвана".
    // Счётчик чипа тоже перечитываем - смена статуса могла изменить набор обновлений.
    handleWithdraw() {
      this.fetchUserApplications();
      this.fetchStatusUpdateCount();
    },

    downloadApplication(application) {
      this.downloadAppId = application.id;
      this.downloadAppInfo = application;
      this.showDownloadModal = true;
    },

    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'desc';
      }
      // Сортировка клиентская - должна идти по всему набору (#1158). При входе в
      // full-load, если ещё не всё загружено, догружаем остаток.
      if (this.isFullLoad && this.hasMoreApplications) {
        this.fetchUserApplications();
      }
    },

    setFilter(filter) {
      this.currentFilter = filter;
      this.selectedApplication = null;
      this.responsibleUsers = [];
      this.fetchUserApplications();
    },

    updateSelectedDate(date) {
      this.selectedDate = date;
      if (date) {
        this.dateRangeStart = null;
        this.dateRangeEnd = null;
      }
    },

    updateDateRangeStart(date) {
      this.dateRangeStart = date;
      if (date) {
        this.selectedDate = null;
      }
    },

    updateDateRangeEnd(date) {
      this.dateRangeEnd = date;
      if (date) {
        this.selectedDate = null;
      }
    },

    applyDateFilters() {
      this.fetchUserApplications();
    },

    clearDateRange() {
      this.selectedDate = null;
      this.dateRangeStart = null;
      this.dateRangeEnd = null;
      this.fetchUserApplications();
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

    async copyApplicationNumber(number) {
      if (!number) return;
      const copied = await copyText(number);
      useDeletionsStore().notify(copied
        ? { prefix: 'Скопирован номер ', bold: String(number), type: 'success' }
        : { prefix: 'Не удалось ', bold: 'скопировать номер', type: 'error' });
    }
  }
};
</script>

<style scoped>
.applications-card {
  background-color: var(--surface);
  border-radius: 30px;
  border: 1px solid var(--border);
  overflow: hidden;
  width: 100%;
  /* Тянемся на всю высоту flex-обёртки кабинета (desktop). На <=1200px высоту
     задаёт адаптивная фикс-высота ниже - там родитель без жёсткой высоты, и
     flex-basis отдаёт управление height. */
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  position: relative;
}

.card-header {
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0px 20px;
  height: auto;
  min-height: 50px;
  flex-shrink: 0;
}

.card-header__title {
  display: flex;
  gap: 15px;
  align-items: center;
  flex: 1;
}

.card-header__settings {
  display: flex;
  gap: 8px;
  align-items: center;
}

.card-title {
  margin: 0;
  color: var(--text);
  font-weight: 600;
  font-size: 1.1em;
}

/* Стили для кнопок фильтров в шапке */
.filter-tabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.filter-tab {
  padding: 4px 12px;
  border: 1px solid var(--border);
  background: var(--surface);
  border-radius: 16px;
  cursor: pointer;
  font-size: 13px;
  transition: all 0.2s;
  color: var(--text);
  white-space: nowrap;
  flex-shrink: 0;
}

.filter-tab:hover:not(.filter-tab--active) {
  background: var(--surface-2);
  border-color: var(--border);
}

.filter-tab--active {
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

.filter-tab--active:hover {
  background: var(--accent-hover);
  border-color: var(--accent-hover);
}

.card-content {
  padding: 0;
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.applications-container {
  display: flex;
  flex: 1;
  width: 100%;
  overflow: hidden;
}

/* Разделитель периода (зеркало Центра): серая полоса с подписью периода.
   Первый разделитель без верхней линии - граница шапки уже есть. */
.applications-day-separator {
  padding: 8px 16px;
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
  background: var(--surface-2);
  display: flex;
  align-items: center;
}

.applications-transition-group > .applications-day-separator:first-child {
  border-top: none;
}

.applications-day-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

/* Левая часть - таблица заявок */
.applications-list {
  width: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  height: 100%;
}

/* Заголовок таблицы */
.applications-header {
  border-bottom: 1px solid var(--border);
  padding: 12px 0;
  flex-shrink: 0;
  height: 44px;
  box-sizing: border-box;
}

.header-row {
  display: flex;
  width: 100%;
}

.header-col {
  font-weight: 500;
  color: var(--text-muted);
  text-align: left;
  font-size: 14px;
  display: flex;
  align-items: center;
  gap: 5px;
  transition: .2s;
  cursor: pointer;
  user-select: none;
  padding: 0 16px;
  overflow: hidden;
  white-space: nowrap;
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
  flex-shrink: 0;
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

/* Колонки с пропорциональной шириной для 5 столбцов */
.id-col {
  flex: 1.2; /* Немного шире для номера заявки */
  min-width: 160px;
}

/* column-stack (номер + бейдж ЧС) только в строках данных. У заголовка остаётся row из
   .header-col, иначе иконка сортировки уезжает под текст "Номер заявки" и шапка кривится.
   Двойной класс заодно перебивает overflow: hidden из .application-col, иначе бейдж под
   номером обрезается. */
.application-col.id-col {
  overflow: visible;
  flex-direction: column;
  justify-content: center;
  align-items: flex-start;
  gap: 4px;
}

.blacklist-flag-badge {
  max-width: 100%;
}

/* у .application-col нет overflow:hidden после оверрайда - даём бейджу перенестись в
   пределах фикс-ширины колонки, а не вылезать (специфичность бьёт nowrap из Badge). */
.id-col .blacklist-flag-badge {
  white-space: normal;
}

.date-col {
  flex: 1.3; /* Шире для даты и времени */
  min-width: 150px;
}

.sender-col {
  flex: 1.4; /* Отправитель */
  min-width: 160px;
}

.confirmation-col {
  flex: 1; /* Подтверждение */
  min-width: 140px;
}

.status-col {
  flex: 1.1;
  min-width: 150px;
}

/* Пульс-точку "статус обновился" (#1349) режет overflow:hidden базового
   .application-col - у колонки статуса ellipsis не нужен (бейдж фиксированный),
   поэтому разрешаем выход точки за границы (спецификой 0,2,0 бьём .application-col). */
.application-col.status-col {
  overflow: visible;
}

.tags-col {
  flex: 1.5;
  min-width: 150px;
  container-type: inline-size;
}

.application-col.tags-col {
  overflow: visible;
  white-space: normal;
}

.header-col.tags-col {
  cursor: default;
}

.header-col.tags-col:hover {
  color: var(--text-muted);
}

.tags-col .application-tags {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  align-items: center;
}

/* текст с многоточием в flex-ячейке */
.ellip {
  display: block;
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* hover-подсказка #333 под тегом */
.tag-hint {
  position: relative;
}

.tag-hint::after {
  content: attr(data-hint);
  position: absolute;
  top: calc(100% + 6px);
  left: 50%;
  transform: translateX(-50%);
  width: max-content;
  max-width: 200px;
  background: var(--hint-bg);
  color: var(--hint-text);
  padding: 5px 9px;
  border-radius: 6px;
  font-size: 11px;
  line-height: 1.3;
  text-align: center;
  white-space: normal;
  z-index: 1000;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.15s;
  box-shadow: 0 2px 8px var(--shadow-drop);
}

.tag-hint::before {
  content: '';
  position: absolute;
  top: 100%;
  left: 50%;
  transform: translateX(-50%);
  border: 5px solid transparent;
  border-bottom-color: var(--hint-bg);
  z-index: 1001;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.15s;
}

.tag-hint:hover::after,
.tag-hint:hover::before {
  opacity: 1;
}

/* Рамка тега цвета текста - ТОЛЬКО в тёмной теме: там приглушённая color-mix-рамка
   Badge сливалась с подложкой. В светлой она остаётся прежней, как была до правки
   (#1415). */
[data-theme="dark"] .rt-tag {
  border-color: currentColor;
}

.tags-col .rt-tag__icon {
  display: none;
}

/* Иконка часов у бейджа "ждёт согласования" видима (прочие теги в кабинете - текстом). */
.tags-col .rt-tag__icon--fixed {
  display: inline-block;
  width: 13px;
  height: 13px;
  opacity: 1;
  margin-right: 3px;
  flex-shrink: 0;
}

/* Маркер обсуждения (#973): чат-иконка (всегда видна) + красная точка-индикатор. */
.rt-tag__q-svg {
  flex-shrink: 0;
}

.rt-tag--questions {
  position: relative;
}

.rt-tag__q-dot {
  position: absolute;
  top: -3px;
  right: -3px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-danger);
  border: 1.5px solid var(--surface);
  pointer-events: none;
}

/* ЧС не сворачивается. Крыша/Парковка -> иконки только когда ОБА (--both) и тесно.
   Условия --both/--chs считаются во Vue (надёжнее :has() в scoped-стилях). */
@container (max-width: 232px) {
  .tags-col .application-tags--both .rt-tag--roof .rt-tag__text,
  .tags-col .application-tags--both .rt-tag--parking .rt-tag__text {
    display: none;
  }

  .tags-col .application-tags--both .rt-tag--roof .rt-tag__icon,
  .tags-col .application-tags--both .rt-tag--parking .rt-tag__icon {
    display: block;
  }

  .tags-col .application-tags--both .rt-tag--roof.badge--sm,
  .tags-col .application-tags--both .rt-tag--parking.badge--sm {
    padding: 4px;
  }
}

/* без ЧС крыша+парковка держим текстом, пока колонка не станет узкой */
@container (min-width: 125px) {
  .tags-col .application-tags--both:not(.application-tags--chs) .rt-tag--roof .rt-tag__text,
  .tags-col .application-tags--both:not(.application-tags--chs) .rt-tag--parking .rt-tag__text {
    display: inline;
  }

  .tags-col .application-tags--both:not(.application-tags--chs) .rt-tag--roof .rt-tag__icon,
  .tags-col .application-tags--both:not(.application-tags--chs) .rt-tag--parking .rt-tag__icon {
    display: none;
  }

  .tags-col .application-tags--both:not(.application-tags--chs) .rt-tag--roof.badge--sm,
  .tags-col .application-tags--both:not(.application-tags--chs) .rt-tag--parking.badge--sm {
    padding: 3px 8px;
  }
}

.actions-col {
  flex: 0 0 100px;
  min-width: 100px;
  max-width: 100px;
  justify-content: flex-end;
  cursor: default;
}

.header-col.actions-col:hover {
  color: var(--text-muted);
}

.download-btn {
  height: 25px;
  background-color: var(--surface);
  color: var(--text);
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
  background-color: var(--surface-2);
  border-color: var(--border);
}

/* Тело таблицы */
.applications-body {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  position: relative;
}

.applications-list-content {
  flex: 1;
  /* Скролл только на .applications-body - не создаём вложенный скролл здесь */
  position: relative;
}

/* Sentinel бесшовной подгрузки (#1158 срез 4) - невидимая полоса внизу списка,
   пересечение которой в applications-body триггерит loadMore. min-height даёт
   IntersectionObserver что засечь даже без спиннера (listLoading=false). */
.user-applications-sentinel {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 24px;
  padding: 10px 0;
  flex-shrink: 0;
}

/* Устойчивость к ошибкам бэка (#1173): первичная загрузка упала - список пуст,
   вместо "Заявок нет" показываем причину + retry. */
.list-error-state {
  text-align: center;
  color: var(--danger-text);
  padding: 40px 20px;
  margin: 0;
  font-size: 14px;
  width: 100%;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
}

.list-error-state p {
  margin: 0;
}

/* Ошибка догрузки следующей порции (#1173) - компактный вариант рядом с sentinel. */
.sentinel-error {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--danger-text);
  font-size: 13px;
}

.user-applications-footer {
  flex-shrink: 0;
  padding: 10px 20px;
  border-top: 1px solid var(--border);
  font-size: 13px;
  color: var(--text-muted);
}

.applications-transition-group {
  position: relative;
  width: 100%;
}

.application-item {
  border-bottom: 1px solid var(--border);
  cursor: pointer;
  transition: background-color 0.2s ease;
  flex-shrink: 0;
  position: relative;
}

.application-item:hover {
  background-color: var(--surface-2);
}

/* Заявка с обновлённым статусом (#1349): мягкий фиолетовый фон + пульс-точка на бейдже.
   В ЛК гейта прочтения нет (у отправителя нет строк application_reads) - фон по одному
   флагу has_status_update. Ставим после hover, чтобы подсветка держалась при наведении. */
.application-item.status-updated {
  background-color: var(--accent-tint);
  /* Левая полоса-акцент (inset - без reflow): заметный сигнал "обновление" даже там,
     где мягкого фона мало (мобильная карточка, где точка статуса скрыта). */
  box-shadow: inset 3px 0 0 0 var(--accent-text);
}

.status-badge-wrap {
  position: relative;
  display: inline-block;
}

.status-update-dot {
  position: absolute;
  top: -3px;
  right: -3px;
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background-color: var(--accent);
  box-shadow: 0 0 0 2px #fff;
}

.status-update-dot::after {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: 50%;
  background-color: var(--accent);
  animation: statusUpdatePulse 1.6s ease-out infinite;
}

@keyframes statusUpdatePulse {
  0% {
    transform: scale(1);
    opacity: 0.55;
  }
  100% {
    transform: scale(2.6);
    opacity: 0;
  }
}

.application-row {
  display: flex;
  width: 100%;
  padding: 6px 0;
  align-items: center;
  min-height: 40px;
}

/* flex НЕ задаём здесь: ширину колонок задают пер-колоночные правила (.id-col и т.д.),
   общие для .header-col и .application-col - иначе данные и заголовки разъезжаются. */
.application-col {
  padding: 0 16px;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  align-items: center;
  display: flex;
  height: 100%;
}

/* Организация и сообщение показываем только в компактной карточке на мобилке
   (W3.11); в десктоп-таблице отдельных колонок под них нет. */
.organization-col,
.message-col {
  display: none;
}

.application-id {
  color: var(--accent-text);
  font-weight: 600;
}

.application-number--copyable {
  position: relative;
  cursor: pointer;
  transition: color 0.15s;
  user-select: none;
  border-radius: 4px;
  outline: none;
}

.application-number--copyable:hover,
.application-number--copyable:focus-visible {
  color: var(--accent-hover);
}

.application-number--copyable:focus-visible {
  box-shadow: 0 0 0 2px rgba(79, 91, 223, 0.3);
}

.application-number--copyable::after {
  content: attr(data-tooltip);
  position: absolute;
  top: calc(100% + 6px);
  left: 50%;
  transform: translateX(-50%);
  background: var(--hint-bg);
  color: var(--hint-text);
  padding: 4px 8px;
  border-radius: 6px;
  font-size: 11px;
  white-space: nowrap;
  z-index: 1000;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.15s;
  box-shadow: 0 2px 8px var(--shadow-drop);
}

.application-number--copyable:hover::after {
  opacity: 1;
}

/* Бейджи подтверждения */
.confirmation-badge {
  display: inline-block;
  min-width: 115px;
  box-sizing: border-box;
  padding: 4px 10px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 500;
  text-align: center;
  white-space: nowrap;
}

.confirmation-approved {
  background-color: var(--success-bg);
  color: var(--success-text);
  border: 1px solid color-mix(in srgb, var(--success) 30%, var(--surface));
}

.confirmation-pending {
  background-color: var(--warning-bg);
  color: var(--warning-text);
  border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
}

.confirmation-rejected {
  background-color: var(--danger-bg);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.confirmation-default {
  background-color: var(--surface-2);
  color: var(--text-muted);
  border: 1px solid var(--border);
}

/* Бейджи статуса */
.status-badge {
  display: inline-block;
  min-width: 120px;
  box-sizing: border-box;
  padding: 4px 10px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
  text-align: center;
  white-space: nowrap;
}

/* Статусы */
.status-unread {
  background-color: var(--warning-bg);
  color: var(--warning-text);
  border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
}

.status-processing {
  background-color: var(--warning-bg);
  color: var(--warning-text);
  border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
}

.status-in-progress {
  background-color: var(--info-bg);
  color: var(--info-text);
  border: 1px solid color-mix(in srgb, var(--info) 30%, var(--surface));
}

.status-completed {
  background-color: var(--success-bg);
  color: var(--success-text);
  border: 1px solid color-mix(in srgb, var(--success) 30%, var(--surface));
}

.status-rejected {
  background-color: var(--danger-bg);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.status-default {
  background-color: var(--surface-2);
  color: var(--text-muted);
  border: 1px solid var(--border);
}

/* Сообщения о загрузке и отсутствии данных */
.no-data-message {
  text-align: center;
  color: var(--text-muted);
  padding: 40px 20px;
  margin: 0;
  font-size: 14px;
  width: 100%;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 8px;
}

.loading-message {
  text-align: center;
  color: var(--accent-text);
  padding: 40px 20px;
  margin: 0;
  font-size: 14px;
  width: 100%;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
}

.loader {
  width: 30px;
  height: 30px;
  border: 3px solid var(--surface-2);
  border-top: 3px solid var(--accent);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

/* Анимация для списка заявок - исправленная */
.fade-list-enter-active {
  transition: all 0.3s ease;
  position: relative;
}

.fade-list-leave-active {
  transition: all 0.3s ease;
  position: absolute !important;
  width: 100%;
  left: 0;
}

.fade-list-enter-from {
  opacity: 0;
  transform: translateY(10px);
}

.fade-list-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

.fade-list-move {
  transition: transform 0.3s ease;
}

/* Стили для прокрутки */
.applications-body::-webkit-scrollbar {
  width: 6px;
}

.applications-body::-webkit-scrollbar-track {
  background: transparent;
  margin: 2px 0;
  border-radius: 3px;
}

.applications-body::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, var(--accent) 22%, var(--surface));
  border-radius: 3px;
  border: 1px solid transparent;
  background-clip: content-box;
  transition: all 0.3s ease;
}

.applications-body::-webkit-scrollbar-thumb:hover {
  background: color-mix(in srgb, var(--accent) 22%, var(--surface));
  border: 1px solid transparent;
  background-clip: content-box;
}

.applications-body {
  scrollbar-width: thin;
  scrollbar-color: color-mix(in srgb, var(--accent) 22%, var(--surface)) transparent;
}

@media (max-width: 1200px) {
  .applications-card {
    height: 450px;
  }
  
  .header-col,
  .application-col {
    padding: 0 12px;
  }
  
  .header-col:first-child,
  .application-col:first-child {
    padding-left: 12px;
  }
}

@media (max-width: 992px) {
  .applications-card {
    height: 500px;
  }
  
  .card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
    height: auto;
    padding: 16px;
  }
  
  .card-header__title {
    width: 100%;
    /* Заголовок + выпадающий фильтр Мои/Организации в один ряд (#1097 p2 r2:
       раньше 2 таба, теперь один список - без каши). Чип "Обновления" (#1349)
       переносится на вторую строку (flex-wrap) - на узком экране в один ряд с
       заголовком и дропдауном не влезает. */
    flex-direction: row;
    flex-wrap: wrap;
    align-items: center;
    gap: 10px;
  }

  /* Выпадающий фильтр забирает остаток ширины, длинный лейбл обрезается. */
  .cabinet__filter-dropdown {
    flex: 1;
    min-width: 0;
  }

  .card-header__settings {
    width: 100%;
    /* Следующая строка: дата (меньше) + Обновить-иконка + поиск. */
    justify-content: flex-start;
    gap: 8px;
  }

  /* Обновить - только иконка (кружок) на мобилке, текст скрыт (как в Обзоре). */
  .card-header__settings :deep(.refresh-btn) {
    width: 40px;
    height: 40px;
    padding: 0;
    justify-content: center;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .card-header__settings :deep(.refresh-btn__text) {
    display: none;
  }

  /* Дата (DateFilter) компактнее на мобилке - меньше шрифт и высота. */
  .card-header__settings :deep(.date-field) {
    font-size: 13px;
    min-height: 40px;
  }
  .card-header__settings :deep(.date-field .field-input) {
    font-size: 13px;
  }
  
  .applications-container {
    flex-direction: column;
    height: 100%;
  }
  
  .applications-list {
    width: 100% !important;
    height: 100% !important;
  }
  
  .header-row,
  .application-row {
    flex-wrap: wrap;
  }
  
  .header-col,
  .application-col {
    width: 33.33% !important;
    margin-bottom: 4px;
    min-width: 100px !important;
    max-width: none !important;
    flex: none !important;
    padding: 0 8px;
  }
  
  .header-col:first-child,
  .application-col:first-child {
    padding-left: 8px;
  }
  
  .date-col {
    min-width: 140px !important;
  }
  
  .confirmation-badge,
  .status-badge {
    min-width: 60px;
    font-size: 10px;
  }
}

@media (max-width: 767.98px) {
  /* Панель заявок edge-to-edge: без боковой рамки и скругления, чтобы список писем
     шёл от края до края экрана (боковой padding дашборда гасит AccountComponent).
     overflow:visible - чтобы sticky-шапка ниже прилипала к вьюпорту, а не клипалась
     внутри карточки (базовый overflow:hidden клипал бы sticky). */
  .applications-card {
    height: auto;
    max-height: none;
    border-left: none;
    border-right: none;
    border-radius: 0;
    overflow: visible;
  }

  /* Шапка кабинета закреплена под app bar (= --mobile-header-height, sticky top:0),
     список заявок скроллит страница под ней (зеркало Центра, срез 3). Нижняя граница
     (уже есть из базового .card-header) - разделитель между шапкой и списком. */
  .card-header {
    position: sticky;
    top: var(--mobile-header-height);
    z-index: 20;
    /* Компактнее и фон #FAFAFA - как шапка Центра (зеркалим правки Центра). Гасит
       щедрый padding:16px/gap:12px из блока 992px -> шапка ниже и опрятнее. */
    background: var(--surface-2);
    padding: 8px 12px 12px;
    gap: 8px;
  }

  /* Оверлей поиска (мобилка): поверх ряда настроек, растёт справа налево (clip-path).
     right:48px оставляет справа иконку-тоггл (40px) открытой. */
  .card-header__settings {
    position: relative;
  }

  .cabinet__search-overlay {
    position: absolute;
    top: 0;
    bottom: 0;
    left: 0;
    right: 48px;
    z-index: 1;
    display: flex;
    align-items: center;
    background: var(--surface);
    /* Скруглить фон под скруглённое поле (15px) - иначе белые квадратные углы
       оверлея торчат за pill-полем рядом с круглой иконкой поиска (#1097 R3-3). */
    border-radius: var(--radius-md);
  }

  .cabinet__search-overlay .field.search {
    display: flex;
    align-items: center;
    width: 100%;
    height: 40px;
    border: 1px solid var(--border);
    border-radius: 15px;
    padding: 0 12px;
    box-sizing: border-box;
  }

  .cabinet__search-input {
    flex: 1;
    min-width: 0;
    border: none;
    outline: none;
    background: transparent;
    font-size: 14px;
    color: var(--text);
  }

  /* Крестик очистки внутри поля (появляется при вводе): сбрасывает и закрывает поиск. */
  .cabinet__search-clear {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    padding: 0;
    border: none;
    background: transparent;
    color: var(--text-muted);
    font-size: 20px;
    line-height: 1;
    cursor: pointer;
    flex-shrink: 0;
  }

  .cabinet__search-clear:hover {
    color: var(--accent-text);
  }

  /* Раскрытие влево - clip-path (композитится, не двигает ряд). */
  .cabinet-search-enter-active,
  .cabinet-search-leave-active {
    transition: clip-path 0.25s ease;
  }

  .cabinet-search-enter-from,
  .cabinet-search-leave-to {
    clip-path: inset(0 0 0 100%);
  }

  .cabinet-search-enter-to,
  .cabinet-search-leave-from {
    clip-path: inset(0 0 0 0);
  }

  /* Иконка-кнопка поиска (мобилка): тоггл оверлея поиска (раскрывается влево поверх ряда). */
  .search-icon-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 40px;
    height: 40px;
    border: 1px solid var(--border);
    border-radius: 50%;
    background: var(--surface);
    cursor: pointer;
    flex-shrink: 0;
    transition: background 0.15s ease, border-color 0.15s ease;
  }

  .search-icon-btn:hover,
  .search-icon-btn--active {
    background: var(--surface-2);
    border-color: var(--accent);
  }

  .search-icon-btn__img {
    width: 16px;
    height: 16px;
    color: var(--text);
    stroke-width: 2.1;
  }

  /* Пустая полоса шапки колонок: .rt-head-row скрыт (responsive-tables.css), но
     обёртка .applications-header со своим border/padding/height:44px остаётся
     видимой полосой перед первой карточкой - схлопываем её целиком (урок S9a). */
  .applications-header {
    display: none;
  }

  .applications-list {
    overflow: visible !important;
  }

  .applications-body {
    overflow-y: visible !important;
    height: auto !important;
    max-height: none !important;
  }

  /* На мобилке список - карточки на белом, серая полоса разделителя лишняя
     (зеркало Центра). */
  .applications-day-separator {
    background: transparent;
    border-top: none;
    border-bottom: none;
    padding: 15px 16px 6px;
  }

  /* Карточки вплотную - разделены нижней границей (см. ниже), без зазора-«плитки». */
  .application-item + .application-item {
    margin-top: 0;
  }
  .application-item {
    border-bottom: none;
  }

  /* Компактная карточка-письмо БЕЗ подписей (W3.11, зеркало Центра):
     согласование / номер (мелко) / организация (жирным) / отправитель / сообщение;
     статус скрыт, дата в углу, всё влево, боковой padding у полей убран. */
  .application-row.rt-row {
    position: relative;
    gap: 3px;
  }

  /* Edge-to-edge список: гасим боковые/верхнюю границу и скругление (иначе скруглённый
     угол торчит у края), карточки разделяем только нижней границей как строки. Полный
     префикс + !important перебивают `.rt-table .rt-row{border;border-radius}!important`. */
  .applications-list .application-row.rt-row {
    border-top: none !important;
    border-left: none !important;
    border-right: none !important;
    border-radius: 0 !important;
  }

  /* Прячем подписи полей (data-label ::before из responsive-tables.css). */
  .applications-list .application-row.rt-row > .application-col::before {
    display: none !important;
  }

  /* Ячейки в столбик, без бордюров/бокового padding, авто-высота, влево.
     width:100% !important - иначе легаси-правило @media(max-width:992px)
     .application-col{width:33.33% !important} держит колонки БЕЗ data-label
     (organization/message) на 1/3 ширины (у data-label-колонок width:100% приходит
     из responsive-tables). !important нужен, чтобы перебить чужой !important. */
  .applications-list .application-row.rt-row > .application-col {
    flex: none;
    display: block;
    width: 100% !important;
    max-width: 100% !important;
    padding: 0;
    border: none;
    height: auto;
    overflow: visible;
    white-space: normal;
    text-align: left;
  }

  /* Длинные значения (организация/отправитель/сообщение) - одна строка с обрезкой "..".
     Специфичность (0,5,0) выше общего блочного правила выше (0,4,0), иначе его
     white-space:normal/overflow:visible победил бы и сообщение переносилось бы. */
  .applications-list .application-row.rt-row > .application-col.organization-col,
  .applications-list .application-row.rt-row > .application-col.sender-col,
  .applications-list .application-row.rt-row > .application-col.message-col {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  /* Статус заявки скрыт в компактной карточке (W3.11). */
  .applications-list .application-row.rt-row > .application-col.status-col {
    display: none;
  }

  /* Дата+время прихода - в правом верхнем углу карточки (W3.11, зеркало Центра).
     Полный префикс (0,5,0) перебивает `...> .application-col{display:block; width:100%}`. */
  .applications-list .application-row.rt-row > .application-col.date-col {
    position: absolute;
    top: 10px;
    right: 14px;
    width: auto !important;
    max-width: 55% !important;
    padding: 0;
    font-size: 14px;
    color: var(--text-muted);
    white-space: nowrap;
    text-align: right;
  }

  /* Резерв справа у бейджа согласования, чтобы дата в углу не наезжала. Полный
     префикс (0,5,0) - иначе общий `...> .application-col{padding:0}` перебивает. */
  .applications-list .application-row.rt-row > .application-col.confirmation-col {
    padding-right: 140px;
  }

  /* Пустой блок тегов - скрыть строку (у sender всегда есть фолбэк "—", :empty там мёртв). */
  .applications-list .application-row.rt-row > .tags-col:empty {
    display: none;
  }

  /* Порядок: согласование / номер / организация / отправитель / сообщение / теги / скачать. */
  .application-col.confirmation-col { order: 1; margin-bottom: 4px; }
  .application-col.id-col { order: 2; }
  .application-col.organization-col { order: 3; }
  .application-col.sender-col { order: 4; }
  .application-col.message-col { order: 5; }
  .application-col.tags-col { order: 6; margin-top: 6px; }
  /* Скачивание на мобилке перенесено в открытую заявку (W3.8) - в строке прячем.
     Полный префикс - иначе общее `...> .application-col{display:block}` перебивает по специфичности. */
  .applications-list .application-row.rt-row > .application-col.actions-col { display: none; }

  /* Типографика строк карточки. */
  .application-col.id-col .application-id {
    font-size: 12px;
    color: var(--text-muted);
    font-weight: 500;
  }
  .application-col.organization-col {
    font-size: 15px;
    font-weight: 700;
    color: var(--text);
  }
  .application-col.sender-col {
    font-size: 13px;
    color: var(--text);
  }
  .application-col.message-col {
    font-size: 13px;
    color: var(--text-muted);
  }


  .download-btn {
    height: 44px;
  }

  /* Номер заявки компактный (как в Центре): min-height:44px делал строку номера
     44px и текст "№..." висел по центру с огромными вертикальными отступами -
     карточка раздувалась до ~150px. Копирование по тапу остаётся (клик на текст). */
  .application-number--copyable {
    display: inline-flex;
    align-items: center;
    min-height: 0;
  }
}

@media (max-width: 576px) {
  .rt-table .rt-row > [data-label] {
    font-size: 13px;
  }
}
</style>