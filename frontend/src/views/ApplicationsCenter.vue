<template>
  <section class="center">
    <header class="center__header">
      <div class="header-top">
        <h2 class="center__title">
          Центр заявок
        </h2>
        <div
          v-if="canViewArchive"
          class="center__tabs"
        >
          <FilterTabs
            v-model="archiveMode"
            :tabs="archiveTabs"
          />
        </div>
        <div
          v-if="unreadCount > 0"
          class="unread-badge"
          data-testid="center-badge-unread"
          :class="{ 'shake-animation': shouldShake }"
        >
          Новые: {{ unreadCount }}
        </div>

        <!-- Настройки звука -->
        <div
          ref="soundPopoverRef"
          class="sound-btn-wrap"
        >
          <button
            type="button"
            class="sound-icon-btn"
            :class="{ 'sound-icon-btn--active': soundStore.enabled }"
            :aria-label="soundStore.enabled ? 'Настройки звука (включён)' : 'Настройки звука (выключен)'"
            :title="soundStore.enabled ? 'Настройки звука (включён)' : 'Настройки звука (выключен)'"
            @click="toggleSoundPopover"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M11 5 6 9H2v6h4l5 4V5z" />
              <template v-if="soundStore.enabled">
                <path d="M15.54 8.46a5 5 0 0 1 0 7.07" />
                <path d="M19.07 4.93a10 10 0 0 1 0 14.14" />
              </template>
              <template v-else>
                <line
                  x1="23"
                  y1="9"
                  x2="17"
                  y2="15"
                />
                <line
                  x1="17"
                  y1="9"
                  x2="23"
                  y2="15"
                />
              </template>
            </svg>
          </button>

          <Transition name="sound-popover">
            <div
              v-if="showSoundPopover"
              class="sound-popover"
              role="dialog"
              aria-label="Настройки звука"
            >
              <div class="sound-popover__header">
                <span class="sound-popover__title">Звук уведомлений</span>
                <label
                  class="sound-toggle"
                  :aria-label="soundStore.enabled ? 'Выключить звук' : 'Включить звук'"
                >
                  <input
                    type="checkbox"
                    class="sound-toggle__input"
                    :checked="soundStore.enabled"
                    @change="soundStore.setEnabled($event.target.checked)"
                  >
                  <span class="sound-toggle__track" />
                </label>
              </div>

              <template v-if="soundStore.enabled">
                <div class="sound-popover__field">
                  <label class="sound-popover__label">Пресет</label>
                  <BaseDropdown
                    :model-value="soundStore.selectedPreset"
                    :options="soundPresets"
                    value-key="value"
                    label-key="label"
                    @update:model-value="soundStore.setPreset($event)"
                  />
                </div>

                <div class="sound-popover__field">
                  <label class="sound-popover__label">Громкость {{ Math.round(soundStore.volume * 100) }}%</label>
                  <input
                    type="range"
                    class="sound-volume"
                    min="0"
                    max="1"
                    step="0.01"
                    :value="soundStore.volume"
                    @input="soundStore.setVolume($event.target.value)"
                  >
                </div>

                <button
                  type="button"
                  class="lk-button lk-button--ghost sound-popover__preview"
                  @click="previewSound"
                >
                  Прослушать
                </button>
              </template>
            </div>
          </Transition>
        </div>
      </div>
    </header>

    <div class="center__filters">
      <div class="filters__main">
        <div class="filters-row">
          <div class="field search">
            <input
              v-model="searchQuery"
              placeholder="Поиск заявок..."
              type="text"
              class="field__input search"
              data-testid="center-input-search"
              @input="onSearchInput"
            >
            <img
              src="@/assets/icons/search.png"
              class="center__icon"
            >
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
            @update:selected-date="updateSelectedDate"
            @update:date-range-start="updateDateRangeStart"
            @update:date-range-end="updateDateRangeEnd"
            @apply="applyDateFilters"
            @clear="clearDateRange"
          />

          <button 
            class="reset-sort-btn"
            :disabled="!sortField"
            @click="resetSort"
          >
            Сбросить сортировку
          </button>

          <button
            class="reset-filters-btn"
            data-testid="center-button-reset-filters"
            :disabled="!hasActiveFilters"
            @click="resetFilters"
          >
            Сбросить фильтры
          </button>
        </div>

        <div class="filters-row filters-row--secondary">
          <div class="filter-section">
            <div class="filter-section__header">
              <span class="filter-label">Заявки</span>
            </div>
            <div class="status-buttons">
              <button
                class="status-btn"
                :class="{ 'status-btn--active': activeToday }"
                data-testid="center-button-today"
                @click="toggleActiveToday"
              >
                Заявки на сегодня
              </button>
            </div>
          </div>
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
                :data-testid="`center-button-confirmation-${confirmation.value}`"
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
                :data-testid="`center-button-status-${status.value}`"
                @click="toggleApplicationStatus(status.value)"
              >
                {{ status.label }}
              </button>
            </div>
          </div>

          <div class="filter-section">
            <div class="filter-section__header">
              <span class="filter-label">Теги</span>
            </div>
            <div class="tags-dropdown">
              <button
                class="tags-dropdown__btn"
                :class="{ 'tags-dropdown__btn--active': selectedTags.length > 0 }"
                @click="tagsDropdownOpen = !tagsDropdownOpen"
              >
                {{ selectedTags.length ? `Выбрано: ${selectedTags.length}` : 'Все теги' }}
                <svg
                  class="tags-dropdown__arrow"
                  :class="{ 'tags-dropdown__arrow--open': tagsDropdownOpen }"
                  viewBox="0 0 10 6"
                  fill="none"
                  xmlns="http://www.w3.org/2000/svg"
                >
                  <path
                    d="M1 1L5 5L9 1"
                    stroke="currentColor"
                    stroke-width="1.5"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  />
                </svg>
              </button>
              <div
                v-if="tagsDropdownOpen"
                class="tags-dropdown__backdrop"
                @click="tagsDropdownOpen = false"
              />
              <transition name="tags-dd">
                <div
                  v-if="tagsDropdownOpen"
                  class="tags-dropdown__panel"
                >
                  <button
                    v-for="tag in tags"
                    :key="tag.value"
                    class="status-btn tags-dropdown__item"
                    :class="{ 'status-btn--active': selectedTags.includes(tag.value) }"
                    @click="toggleTag(tag.value)"
                  >
                    {{ tag.label }}
                  </button>
                </div>
              </transition>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div
      class="applications-table"
      :class="{ 'with-details': selectedApplication }"
    >
      <div class="table-header">
        <div class="header-row">
          <div
            class="header-col confirmation-col"
            @click="sortBy('confirmation')"
          >
            <p :class="{ 'active-sort': sortField === 'confirmation' }">
              Подтверждение
            </p>
            <img 
              src="@/assets/icons/sort.png" 
              class="sort-icon" 
              :class="{ 
                'sorted': sortField === 'confirmation',
                'desc': sortField === 'confirmation' && sortDirection === 'desc'
              }" 
            >
          </div>
          <div
            class="header-col number-col"
            @click="sortBy('number')"
          >
            <p :class="{ 'active-sort': sortField === 'number' }">
              Номер заявки
            </p>
            <img 
              src="@/assets/icons/sort.png" 
              class="sort-icon" 
              :class="{ 
                'sorted': sortField === 'number',
                'desc': sortField === 'number' && sortDirection === 'desc'
              }" 
            >
          </div>
          <div
            class="header-col date-col"
            @click="sortBy('date')"
          >
            <p :class="{ 'active-sort': sortField === 'date' }">
              Дата и время
            </p>
            <img 
              src="@/assets/icons/sort.png" 
              class="sort-icon" 
              :class="{ 
                'sorted': sortField === 'date',
                'desc': sortField === 'date' && sortDirection === 'desc'
              }" 
            >
          </div>
          <div
            class="header-col organization-col"
            @click="sortBy('organization')"
          >
            <p :class="{ 'active-sort': sortField === 'organization' }">
              Организация
            </p>
            <img 
              src="@/assets/icons/sort.png" 
              class="sort-icon" 
              :class="{ 
                'sorted': sortField === 'organization',
                'desc': sortField === 'organization' && sortDirection === 'desc'
              }" 
            >
          </div>
          <div
            class="header-col sender-col"
            @click="sortBy('sender')"
          >
            <p :class="{ 'active-sort': sortField === 'sender' }">
              Отправитель
            </p>
            <img 
              src="@/assets/icons/sort.png" 
              class="sort-icon" 
              :class="{ 
                'sorted': sortField === 'sender',
                'desc': sortField === 'sender' && sortDirection === 'desc'
              }" 
            >
          </div>
          <div
            class="header-col status-col"
            @click="sortBy('status')"
          >
            <p :class="{ 'active-sort': sortField === 'status' }">
              Статус заявки
            </p>
            <img 
              src="@/assets/icons/sort.png" 
              class="sort-icon" 
              :class="{ 
                'sorted': sortField === 'status',
                'desc': sortField === 'status' && sortDirection === 'desc'
              }" 
            >
          </div>
          <div class="header-col tags-col">
            <p>Теги</p>
          </div>
          <div class="header-col actions-col">
            <RefreshButton
              :loading="refreshing"
              @refresh="fetchApplications"
            />
          </div>
        </div>
      </div>
            
      <div
        class="table-body"
        :class="{ 'table-body--single-row': filteredApplications.length === 1 }"
      >
        <SkeletonTransition :loading="loading">
          <template #skeleton>
            <SkeletonTable
              :rows="10"
              :columns="6"
            />
          </template>
          <div
            v-if="filteredApplications.length > 0"
            class="applications-list"
          >
            <div
              v-for="(application, index) in sortedApplications"
              :key="application.id"
              class="application-item"
              :class="{
                'unread': !application.is_read,
                'initial-load': isInitialLoad,
                'filtered': !isInitialLoad
              }"
              :data-testid="`center-row-${application.id}`"
              :style="isInitialLoad ? { 'animation-delay': `${index * 0.05}s` } : {}"
              @click="openApplication(application)"
            >
              <div class="application-row">
                <div class="application-col confirmation-col">
                  <span
                    class="confirmation-badge"
                    :class="getConfirmationClass(application.confirmation)"
                    :title="application.confirmation"
                  >
                    {{ application.confirmation }}
                  </span>
                </div>
                <div class="application-col number-col">
                  <span
                    class="application-number application-number--copyable"
                    data-tooltip="Копировать"
                    role="button"
                    tabindex="0"
                    @click.stop="copyApplicationNumber(application.application_number)"
                    @keydown.enter.prevent="copyApplicationNumber(application.application_number)"
                  >{{ application.application_number }}</span>
                </div>
                <div class="application-col date-col">
                  {{ formatDateTime(application.sending_datetime) }}
                </div>
                <div class="application-col organization-col">
                  <span class="ellip">{{ getOrganizationName(application) }}</span>
                </div>
                <div
                  class="application-col sender-col"
                >
                  <span
                    v-if="application.sender_name"
                    class="sender-tooltip-anchor"
                    :data-tooltip="application.sender_full_name || application.sender_name"
                  ><span class="ellip">{{ application.sender_name }}</span></span>
                </div>
                <div class="application-col status-col">
                  <span
                    class="status-badge"
                    :class="getStatusClass(application.status)"
                    :title="application.status"
                  >
                    {{ application.status }}
                  </span>
                </div>
                <div class="application-col tags-col">
                  <div
                    v-if="blacklistFlagCount(application) > 0 || application.has_roof_access || application.has_free_parking || application.sender_is_important || application.has_unseen_questions"
                    class="application-tags"
                    :class="{ 'application-tags--compact': tagsAreCompact(application) }"
                  >
                    <Badge
                      v-if="blacklistFlagCount(application) > 0"
                      variant="danger"
                      size="sm"
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
                      v-if="application.sender_is_important"
                      variant="info"
                      size="sm"
                      class="rt-tag rt-tag--important tag-hint"
                      data-hint="Важный пользователь"
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
                      ><polygon points="12 2 15 8.6 22 9.3 16.8 14 18.3 21 12 17.3 5.7 21 7.2 14 2 9.3 9 8.6" /></svg>
                      <span class="rt-tag__text">Важный</span>
                    </Badge>
                    <Badge
                      v-if="application.has_unseen_questions"
                      variant="primary"
                      size="sm"
                      class="rt-tag rt-tag--questions tag-hint"
                      data-hint="Есть новые вопросы или ответы"
                      :data-testid="`center-questions-badge-${application.id}`"
                    >
                      <svg
                        class="rt-tag__icon"
                        width="14"
                        height="14"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      ><path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" /></svg>
                      <span class="rt-tag__text">Вопросы</span>
                      <span
                        class="rt-tag__q-dot"
                        aria-hidden="true"
                      />
                    </Badge>
                  </div>
                </div>
                <div class="application-col actions-col">
                  <button
                    v-if="application.has_blank_template && can('action.export.applications')"
                    class="download-btn"
                    title="Скачать"
                    @click.stop="downloadApplication(application)"
                  >
                    Скачать
                  </button>
                </div>
              </div>
            </div>
          </div>
          <p
            v-else
            class="no-data-message"
          >
            {{ searchQuery.trim() ? 'Ничего не найдено' : hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Заявок нет' }}
          </p>
        </SkeletonTransition>
      </div>

      <!-- При первой загрузке работает скелетон (loading), поэтому оверлей только на refreshing. -->
      <div
        v-if="refreshing && !loading"
        class="refresh-overlay"
      >
        <LoaderSpinner label="Обновление…" />
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
      @questions-read="onQuestionsRead"
    />
    <DownloadBlanksModal
      :show="!!(showDownloadModal && downloadAppId)"
      :application-id="downloadAppId || 0"
      :application-info="downloadAppInfo"
      @close="showDownloadModal = false"
    />
  </section>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { useSoundStore } from '@/stores/sound'
import { usePermissionsStore } from '@/stores/permissions'
import { playPreset, SOUND_PRESETS } from '@/utils/notificationSound'
import OrganizationFilter from '@/components/OrganizationFilter.vue';
import RefreshButton from '../components/RefreshButton.vue';
import ApplicationDetail from '../components/ApplicationDetail/ApplicationDetail.vue';
import DateFilter from '../components/DateFilter.vue';
import FilterTabs from '@/components/ui/FilterTabs.vue';
import SkeletonTransition from '@/components/ui/SkeletonTransition.vue';
import SkeletonTable from '@/components/ui/SkeletonTable.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import DownloadBlanksModal from '@/components/applications/DownloadBlanksModal.vue';
import Badge from '@/components/ui/Badge.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import { blacklistFlagCount, blacklistFlagLabel, BLACKLIST_FLAG_TITLE } from '@/utils/blacklistBadge';
import { useToast } from '@/composables/useToast';

export default {
    name: 'ApplicationsCenter',
    components: {
        OrganizationFilter,
        RefreshButton,
        ApplicationDetail,
        DateFilter,
        FilterTabs,
        SkeletonTransition,
        SkeletonTable,
        LoaderSpinner,
        DownloadBlanksModal,
        Badge,
        BaseDropdown,
    },
    emits: ['refresh-data'],
    setup() {
        const soundStore = useSoundStore()
        const permissionsStore = usePermissionsStore()
        return { soundStore, permissionsStore }
    },
    data() {
        return {
            showDownloadModal: false,
            showSoundPopover: false,
            soundPresets: SOUND_PRESETS,
            downloadAppId: 0,
            downloadAppInfo: null,
            searchQuery: '',
            searchDebounceTimer: null,
            selectedOrganizationId: null,
            selectedOrganizationName: '',
            selectedConfirmations: [],
            selectedApplicationStatuses: [],
            selectedTags: [],
            tagsDropdownOpen: false,
            organizations: [],
            sortField: null,
            sortDirection: 'desc',
            shouldShake: false,
            shakeInterval: null,
            applicationsPollInterval: null,
            isInitialLoad: true,
            // Инкрементальный polling: после первого полного fetch прибавляем только новые
            // заявки в начало списка без перерисовки всего. pollPrimed=false пока не
            // завершился первый fetchApplications — чтобы не играть звук при открытии страницы.
            pollPrimed: false,
            
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
            
            tags: [
                { value: 'chs', label: 'ЧС' },
                { value: 'roof', label: 'Крыша' },
                { value: 'parking', label: 'Парковка' },
                { value: 'important', label: 'Важный' },
            ],

            archiveMode: 'active',
            archiveTabs: [
                { key: 'active', label: 'Активные' },
                { key: 'archive', label: 'Архив' },
            ],

            activeToday: false,

            loading: true,
            refreshing: false,

            // Данные заявок
            applications: [],
            
            // Детали заявки
            selectedApplication: null,
            currentUserId: null,
            currentUserName: ''
        };
    },
    computed: {
        canViewArchive() {
            return this.permissionsStore.hasPermission('center.archive');
        },
        filteredApplications() {
            let filtered = this.applications;

            // Поиск по тексту выполняется на бэке через search_query — здесь не дублируем.

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

            // Фильтр по статусу заявки. "Непрочитано" срабатывает и на статусе заявки
            // "Непрочитано", и на заявках, не прочитанных пользователем (!is_read).
            if (this.selectedApplicationStatuses.length > 0) {
                const includeUnread = this.selectedApplicationStatuses.includes('Непрочитано');
                filtered = filtered.filter(app =>
                    this.selectedApplicationStatuses.includes(app.status) ||
                    (includeUnread && !app.is_read)
                );
            }

            // Фильтр по тегам (OR: хотя бы один из выбранных тегов присутствует)
            if (this.selectedTags.length > 0) {
                filtered = filtered.filter(app =>
                    this.selectedTags.some(tag => {
                        switch (tag) {
                            case 'chs': return this.blacklistFlagCount(app) > 0;
                            case 'roof': return app.has_roof_access;
                            case 'parking': return app.has_free_parking;
                            case 'important': return app.sender_is_important;
                            default: return false;
                        }
                    })
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
                   this.selectedTags.length > 0 ||
                   !!this.selectedDate ||
                   (this.dateRangeStart && this.dateRangeEnd) ||
                   this.activeToday;
        },

        unreadCount() {
            return this.applications.filter(app => !app.is_read).length;
        }
    },
    watch: {
        archiveMode() {
            this.fetchApplications();
        },
        '$route.query.archive'(val) {
            this.archiveMode = val === 'true' && this.canViewArchive ? 'archive' : 'active';
        },
        // Переход из уведомления, когда пользователь уже на /center: mounted не
        // перевызывается, поэтому открываем заявку по смене query.open (#973).
        '$route.query.open'(val) {
            if (val) this.openFromDeepLink();
        },
    },
    mounted() {
        this.startShakeAnimation();

        if (this.$route.query.archive === 'true' && this.canViewArchive) {
            this.archiveMode = 'archive';
        }

        this.fetchOrganizations();
        this.fetchApplications().then(() => this.openFromDeepLink());
        this.getCurrentUser();

        // Polling 30s: инкрементально добавляет новые заявки в начало и
        // играет звук при появлении; для обновления статусов существующих
        // раз в 5 минут делается полный reload.
        let _fullReloadCounter = 0;
        this.applicationsPollInterval = setInterval(() => {
            if (!this.isInitialLoad) {
                _fullReloadCounter++;
                if (_fullReloadCounter >= 10) {
                    _fullReloadCounter = 0;
                    this.fetchApplications();
                } else {
                    this._pollApplicationsIncremental();
                }
            }
        }, 30000);

        setTimeout(() => {
            this.isInitialLoad = false;
        }, 1000);

        this._soundEscapeHandler = (e) => {
            if (e.key === 'Escape' && this.showSoundPopover) {
                this.showSoundPopover = false;
            }
        };
        this._soundClickOutsideHandler = (e) => {
            if (!this.showSoundPopover) return;
            const wrap = this.$refs.soundPopoverRef;
            if (wrap && !wrap.contains(e.target)) {
                this.showSoundPopover = false;
            }
        };
        document.addEventListener('keydown', this._soundEscapeHandler);
        document.addEventListener('mousedown', this._soundClickOutsideHandler);
    },
    beforeUnmount() {
        if (this.shakeInterval) {
            clearInterval(this.shakeInterval);
        }
        if (this.applicationsPollInterval) {
            clearInterval(this.applicationsPollInterval);
        }
        if (this.searchDebounceTimer) {
            clearTimeout(this.searchDebounceTimer);
        }
        document.removeEventListener('keydown', this._soundEscapeHandler);
        document.removeEventListener('mousedown', this._soundClickOutsideHandler);
    },
    methods: {
        can(key) {
            return this.permissionsStore.hasPermission(key);
        },
        toggleSoundPopover() {
            this.showSoundPopover = !this.showSoundPopover;
        },
        previewSound() {
            playPreset(this.soundStore.selectedPreset, this.soundStore.volume);
        },
        async copyApplicationNumber(number) {
            if (!number) return;
            try {
                if (navigator.clipboard?.writeText) {
                    await navigator.clipboard.writeText(String(number));
                } else {
                    const textarea = document.createElement('textarea');
                    textarea.value = String(number);
                    textarea.setAttribute('readonly', '');
                    textarea.style.position = 'absolute';
                    textarea.style.left = '-9999px';
                    document.body.appendChild(textarea);
                    textarea.select();
                    document.execCommand('copy');
                    document.body.removeChild(textarea);
                }
                useToast().success(`Номер ${number} скопирован`);
            } catch {
                useToast().error('Не удалось скопировать номер');
            }
        },

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
        
        onSearchInput() {
            this.isInitialLoad = false;
            clearTimeout(this.searchDebounceTimer);
            this.searchDebounceTimer = setTimeout(() => {
                this.fetchApplications();
            }, 300);
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
        
        toggleActiveToday() {
            this.activeToday = !this.activeToday;
            this.isInitialLoad = false;
            this.fetchApplications();
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

        toggleTag(tag) {
            const index = this.selectedTags.indexOf(tag);
            if (index > -1) {
                this.selectedTags.splice(index, 1);
            } else {
                this.selectedTags.push(tag);
            }
            this.applyFilters();
        },
        
        resetFilters() {
            this.searchQuery = '';
            clearTimeout(this.searchDebounceTimer);
            this.selectedOrganizationId = null;
            this.selectedOrganizationName = '';
            this.selectedConfirmations = [];
            this.selectedApplicationStatuses = [];
            this.selectedTags = [];
            this.selectedDate = null;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            this.activeToday = false;

            this.resetSort();

            if (this.$refs.organizationFilter && this.$refs.organizationFilter.reset) {
                this.$refs.organizationFilter.reset();
            }

            if (this.$refs.dateFilter && this.$refs.dateFilter.clearSelection) {
                this.$refs.dateFilter.clearSelection();
            }

            this.isInitialLoad = false;
            // fetchApplications() вместо applyFilters() — часть фильтров (organization_id,
            // date, archive) применяется на бэке через URL params. Без fetch applications
            // остаётся подмножеством, и после сброса таблица продолжает показывать только его.
            this.fetchApplications();
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

        blacklistFlagCount,
        blacklistFlagLabel,
        blacklistFlagTitle() {
            return BLACKLIST_FLAG_TITLE;
        },

        // Колонка тегов фиксированная (120/90px) - один текстовый тег влезает, два и больше нет.
        // При 2+ тегах сворачиваем крыша/парковка/важный в иконки (ЧС держим текстом). Решение
        // по данным, а не замером DOM - реактивно, без дёрганья на ре-рендерах.
        tagsAreCompact(application) {
            const count =
                (this.blacklistFlagCount(application) > 0 ? 1 : 0) +
                (application.has_roof_access ? 1 : 0) +
                (application.has_free_parking ? 1 : 0) +
                (application.sender_is_important ? 1 : 0) +
                (application.has_unseen_questions ? 1 : 0);
            return count >= 2;
        },

        // API методы

        /**
         * Инкрементальный polling: запрашивает список без активных фильтров (только архив),
         * находит заявки, чей id отсутствует в текущем массиве, и prepend'ит их.
         * Звук играет в момент добавления, а не при росте unread-счётчика NavMenu —
         * так пользователь слышит звук именно когда новая строка появляется в таблице.
         * Не запускается если активны фильтры: новая заявка может просто не попасть
         * под фильтр, и prepend без полного reload будет некорректен.
         */
        async _pollApplicationsIncremental() {
            // При активных фильтрах инкрементальный prepend некорректен — делаем полный reload.
            const hasFilters = !!(
                this.searchQuery ||
                this.selectedOrganizationId ||
                this.selectedConfirmations.length ||
                this.selectedApplicationStatuses.length ||
                this.selectedTags.length ||
                this.selectedDate ||
                (this.dateRangeStart && this.dateRangeEnd) ||
                this.activeToday
            );
            if (hasFilters) {
                await this.fetchApplications();
                return;
            }

            try {
                const authStore = useAuthStore();
                if (!authStore.token) return;

                const params = new URLSearchParams();
                params.append('archive', this.archiveMode === 'archive' ? 'true' : 'false');
                const response = await apiRequest(`/applications?${params}`, { method: 'GET' });
                if (!response.ok) return;

                const fresh = await response.json();
                if (!Array.isArray(fresh)) return;

                const knownIds = new Set(this.applications.map(a => a.id));
                const newApps = fresh.filter(a => !knownIds.has(a.id));

                if (newApps.length > 0) {
                    // Prepend: новые заявки в начало (у них id больше — они свежее)
                    this.applications = [...newApps, ...this.applications];

                    if (this.pollPrimed && this.soundStore.enabled) {
                        playPreset(this.soundStore.selectedPreset, this.soundStore.volume);
                    }
                }

                // Синхронизируем обновлённые поля существующих (статус/confirmation/is_read),
                // чтобы не ждать полного reload раз в 5 минут.
                const freshById = Object.fromEntries(fresh.map(a => [a.id, a]));
                this.applications = this.applications.map(a => {
                    const updated = freshById[a.id];
                    if (!updated) return a;
                    // Только изменяемые серверные поля, не трогаем локальные (selected и т.п.)
                    if (
                        updated.status !== a.status ||
                        updated.confirmation !== a.confirmation ||
                        updated.is_read !== a.is_read ||
                        updated.has_unseen_questions !== a.has_unseen_questions
                    ) {
                        return { ...a, ...updated };
                    }
                    return a;
                });
            } catch {
                // сетевой сбой — не критично, следующий poll сам восстановится
            }
        },

        async fetchApplications() {
            this.refreshing = true;
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

                if (this.activeToday) {
                    params.append('active_today', 'true');
                }

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
                    // После первого полного fetch включаем инкрементальный polling со звуком.
                    this.pollPrimed = true;
                } else {
                    console.error("Ошибка при загрузке заявок:", await response.text());
                }
            } catch (error) {
                console.error("Ошибка сети при загрузке заявок:", error);
            } finally {
                this.loading = false;
                this.refreshing = false;
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
            this.downloadAppId = application.id;
            this.downloadAppInfo = application;
            this.showDownloadModal = true;
        },

        async openApplication(application) {
            if (!application.is_read) {
                try {
                    const response = await apiRequest(`/applications/${application.id}/read`, {
                        method: "POST"
                    });
                    if (response.ok) {
                        application.is_read = true;
                    }
                } catch (error) {
                    console.error("Ошибка при отметке прочтения:", error);
                }
            }

            // Маркер вопросов гасит ПРОЧТЕНИЕ топиков в детали (клик), а не факт открытия
            // заявки (#973). Когда все вопросы прочитаны, деталь эмитит questions-read ->
            // onQuestionsRead снимает иконку в списке сразу (см. ниже).
            this.selectedApplication = application;
        },

        // Все вопросы заявки прочитаны в детали -> гасим маркер в списке оптимистично (#973),
        // не дожидаясь перезагрузки списка.
        onQuestionsRead(applicationId) {
            const app = this.applications.find(a => a.id === applicationId);
            if (app) {
                app.has_unseen_questions = false;
            }
        },

        // Переход из уведомления: /center?open=<id> открывает заявку и чистит query,
        // чтобы обновление страницы её повторно не открывало (#973).
        openFromDeepLink() {
            const openId = Number(this.$route.query.open);
            if (!openId) return;
            const app = this.applications.find(a => a.id === openId);
            // Query чистим только когда заявка найдена и открыта: если список ещё не
            // подъехал (или заявка под фильтром/архивом), оставляем ?open до след. попытки.
            if (!app) return;
            this.openApplication(app);
            const query = { ...this.$route.query };
            delete query.open;
            this.$router.replace({ query }).catch(() => {});
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
            
            // П.40: подтягиваем актуальное состояние списка из Центра сразу после изменения
            this.fetchApplications();
            this.$emit('refresh-data');
        },

        handleApplicationUpdate() {
            this.fetchApplications();
        },

        handleApplicationChanged(updatedApp) {
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
                // П.40: дотягиваем актуальное состояние с сервера (локальный мердж может быть неполным)
                this.fetchApplications();
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
            }, 10000);
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
    padding: 14px 16px;
    background: #fff;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
    margin-bottom: 16px;
}

.center__tabs {
    display: flex;
}

.filters-row {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 12px;
    flex-wrap: wrap;
}

.filters-row--secondary {
    gap: 20px;
    margin-bottom: 0;
    padding-top: 12px;
    border-top: 1px dashed var(--color-border);
    align-items: flex-end;
}

.filter-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
    justify-content: flex-end;
}

.table-toolbar {
    display: flex;
    justify-content: flex-end;
    padding: 8px 0;
    border-bottom: 1px solid var(--color-border);
    background: #fafafa;
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
    height: 36px;
    background-color: #FFF;
    border-radius: var(--radius-md);
    border: 1px solid var(--color-border);
    padding: 0 12px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    position: relative;
    transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.field:focus-within {
    border-color: var(--color-primary);
    box-shadow: var(--shadow-focus);
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

.status-btn,
.reset-sort-btn,
.reset-filters-btn,
.today-filter-btn {
    padding: 7px 14px;
    border: 1px solid var(--color-border);
    background: white;
    border-radius: var(--radius-pill);
    cursor: pointer;
    font-size: 12px;
    font-weight: 500;
    transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease, box-shadow 0.15s ease;
    height: 32px;
    color: var(--color-text);
    white-space: nowrap;
    display: inline-flex;
    align-items: center;
}

.status-btn:hover:not(.status-btn--active),
.reset-sort-btn:hover:not(:disabled),
.today-filter-btn:hover:not(.today-filter-btn--active) {
    background: var(--color-bg);
    border-color: var(--color-primary);
    color: var(--color-primary);
}

.status-btn--active,
.today-filter-btn--active {
    background: var(--color-primary);
    color: white;
    border-color: var(--color-primary);
}

.status-btn--active:hover,
.today-filter-btn--active:hover {
    background: var(--color-primary-hover);
    border-color: var(--color-primary-hover);
}

.reset-sort-btn:disabled,
.reset-filters-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.reset-filters-btn {
    background: #fff;
    border-color: #fecaca;
    color: var(--color-danger);
}

.reset-filters-btn:hover:not(:disabled) {
    background: var(--color-danger);
    border-color: var(--color-danger);
    color: #fff;
}

.applications-table {
    min-width: 300px;
    background-color: #fff;
    border-radius: 30px;
    border: 1px solid var(--color-border);
    overflow: hidden;
    container-type: inline-size;
    margin-top: 20px;
    height: fit-content;
    max-height: 500px;
   
    display: flex;
    flex-direction: column;
    transition: all 0.3s ease;
    position: relative;
}

.applications-table.with-details {
    height: 500px;
}

/* Overlay-лоадер при refresh - накрывает только область данных (ниже шапки
   45px), вне скролл-контейнера .table-body, поэтому держится на месте при
   прокрутке. Высота таблицы не схлопывается. */
.refresh-overlay {
    position: absolute;
    top: 45px;
    left: 0;
    right: 0;
    bottom: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(255, 255, 255, 0.75);
    backdrop-filter: blur(1px);
    z-index: 2;
    pointer-events: none;
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
    gap: 14px;
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
    white-space: nowrap;
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

/* Перераспределение размеров колонок (пропорциональный рост, см. .organization-col) */
.confirmation-col {
    flex: 150 1 0;
    min-width: 150px;
}

.number-col {
    flex: 140 1 0;
    min-width: 132px;
}

/* column-stack (номер + бейдж) только в строках данных. У заголовка остаётся row из
   .header-col, иначе иконка сортировки уезжает под текст. */
.application-col.number-col {
    flex-direction: column;
    justify-content: center;
    align-items: flex-start;
    gap: 4px;
}

.blacklist-flag-badge {
    max-width: 100%;
}

/* у .application-col нет overflow:hidden - на узкой раскладке даём бейджу перенестись,
   а не вылезать в соседнюю колонку (специфичность бьёт white-space:nowrap из Badge). */
.number-col .blacklist-flag-badge {
    white-space: normal;
}

/* теги вложения (ЧС/крыша/парковка) в отдельной колонке (#529). Всё в ОДНУ строку (nowrap).
   ЧС не сворачивается. Крыша/парковка -> иконки когда нав-меню закреплено (тесно) И в строке
   есть ЧС или оба тега; одиночные крыша/парковка - текст. ЧС+оба (3 тега) - всегда иконки. */
.application-tags {
    display: flex;
    gap: 4px;
    flex-wrap: nowrap;
    align-items: center;
    min-width: 0;
}

/* Анимация сворачивания крыша/парковка: текст схлопывается по ширине, иконка раскрывается
   (display не анимируется, поэтому через ширину + прозрачность). */
.rt-tag {
    gap: 0;
    transition: padding 0.28s ease;
}

.rt-tag__text {
    display: inline-block;
    max-width: 150px;
    opacity: 1;
    overflow: hidden;
    white-space: nowrap;
    transition: max-width 0.28s ease, opacity 0.2s ease;
}

.rt-tag__icon {
    display: inline-block;
    width: 0;
    height: 13px;
    opacity: 0;
    overflow: hidden;
    flex-shrink: 0;
    transition: width 0.28s ease, opacity 0.2s ease;
}

/* текст с многоточием в flex-ячейке (на самой ячейке text-overflow:ellipsis не работает) */
.ellip {
    display: block;
    min-width: 0;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

/* hover-подсказка #333 под тегом (как у Отправителя) */
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
    background: #333;
    color: #fff;
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
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.tag-hint::before {
    content: '';
    position: absolute;
    top: 100%;
    left: 50%;
    transform: translateX(-50%);
    border: 5px solid transparent;
    border-bottom-color: #333;
    z-index: 1001;
    pointer-events: none;
    opacity: 0;
    transition: opacity 0.15s;
}

.tag-hint:hover::after,
.tag-hint:hover::before {
    opacity: 1;
}

/* Свёртка тегов: при 2+ тегах в фикс-колонке (класс --compact вешается по данным в
   tagsAreCompact) крыша/парковка/важный схлопывают текст в иконку, без переноса на новую
   строку. ЧС держим полным текстом - критичный флаг, его не прячем. */
.application-tags--compact .rt-tag--roof .rt-tag__text,
.application-tags--compact .rt-tag--parking .rt-tag__text,
.application-tags--compact .rt-tag--important .rt-tag__text,
.application-tags--compact .rt-tag--questions .rt-tag__text {
    max-width: 0;
    opacity: 0;
}

.application-tags--compact .rt-tag--roof .rt-tag__icon,
.application-tags--compact .rt-tag--parking .rt-tag__icon,
.application-tags--compact .rt-tag--important .rt-tag__icon,
.application-tags--compact .rt-tag--questions .rt-tag__icon {
    width: 13px;
    opacity: 1;
}

.application-tags--compact .rt-tag--roof.badge--sm,
.application-tags--compact .rt-tag--parking.badge--sm,
.application-tags--compact .rt-tag--important.badge--sm,
.application-tags--compact .rt-tag--questions.badge--sm {
    padding: 4px;
}

/* Маркер вопросов: красная точка-индикатор поверх бейджа (видна всегда, #973). */
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
    border: 1.5px solid #fff;
    pointer-events: none;
}

/* Колонка тегов фиксированная: 120px когда таблица просторная, 90px когда тесно (нав-меню
   закреплено). Базовое правило ДО @container, чтобы контейнерное переопределение (90px)
   победило по порядку источника при равной специфичности. */
.tags-col {
    flex: 0 0 120px;
    transition: flex-basis 0.3s ease;
}

.application-col.tags-col {
    overflow: visible;
}

/* тесно (нав-меню закреплено, ширина таблицы < 1300): колонка тегов -> 90px. Свёртку текста
   в иконки делает tagsAreCompact по числу тегов (см. выше). */
@container (max-width: 1300px) {
    .tags-col {
        flex: 0 0 90px;
    }
}

.date-col {
    flex: 135 1 0;
    min-width: 126px;
}

/* Все текстовые колонки растут ПРОПОРЦИОНАЛЬНО (flex-basis: 0 + вес во flex-grow), поэтому
   на широком экране ширина распределяется между ними, а не достаётся одной Организации.
   min-width у каждой - под ширину её заголовка (иначе текст заголовка обрезается при тесной
   раскладке). Организация с меньшим весом - не доминирует над остальными. */
.organization-col {
    flex: 175 1 0;
    min-width: 130px;
}

.sender-col {
    flex: 160 1 0;
    min-width: 124px;
}

.application-col.sender-col {
    overflow: visible;
}

.sender-tooltip-anchor {
    position: relative;
    cursor: default;
    display: block;
    min-width: 0;
    max-width: 100%;
}

.sender-tooltip-anchor::after {
    content: attr(data-tooltip);
    position: absolute;
    top: calc(100% + 6px);
    left: 50%;
    transform: translateX(-50%);
    background: #333;
    color: #fff;
    padding: 6px 10px;
    border-radius: 6px;
    font-size: 12px;
    white-space: nowrap;
    z-index: 1000;
    pointer-events: none;
    opacity: 0;
    transition: opacity 0.15s;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.sender-tooltip-anchor::before {
    content: '';
    position: absolute;
    top: 100%;
    left: 50%;
    transform: translateX(-50%);
    border: 5px solid transparent;
    border-bottom-color: #333;
    z-index: 1001;
    pointer-events: none;
    opacity: 0;
    transition: opacity 0.15s;
}

.sender-tooltip-anchor:hover::after,
.sender-tooltip-anchor:hover::before {
    opacity: 1;
}

.status-col {
    flex: 140 1 0;
    min-width: 130px;
}

.actions-col {
    flex: 0 0 96px;
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
    background-color: #fff5e0;
}

.application-row {
    display: flex;
    width: 100%;
    padding: 6px 16px;
    align-items: center;
    gap: 14px;
    border-bottom: 1px solid #f0f0f0;
    min-height: 40px;
}

.application-col {
    text-align: left;
    font-size: 14px;
    display: flex;
    align-items: center;
    height: 100%;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
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
    color: #333;
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
    background: #333;
    color: #fff;
    padding: 4px 8px;
    border-radius: 6px;
    font-size: 11px;
    white-space: nowrap;
    z-index: 1000;
    pointer-events: none;
    opacity: 0;
    transition: opacity 0.15s;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.application-number--copyable:hover::after {
    opacity: 1;
}

.status-badge {
    display: inline-block;
    min-width: 120px;
    box-sizing: border-box;
    padding: 4px 10px;
    border-radius: 8px;
    font-size: 11px;
    font-weight: 500;
    text-align: center;
    white-space: nowrap;
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

/* При одной строке таблица низкая, а overflow:hidden + scroll обрезали подсказки
   (раскрываются вниз, top:100%). Скролл при одной строке не нужен - включаем
   overflow:visible, и подсказка показывается целиком вниз от элемента. */
.applications-table:has(.table-body--single-row) {
    overflow: visible;
}
.table-body--single-row {
    overflow: visible;
}

/* Фильтр по тегам - компактный дропдаун, чтобы фильтры держались в строке. */
.tags-dropdown {
    position: relative;
}
.tags-dropdown__btn {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 12px;
    border: 1px solid #e0e0e0;
    border-radius: 12px;
    background: #fff;
    cursor: pointer;
    font-size: 13px;
    white-space: nowrap;
}
.tags-dropdown__btn--active {
    border-color: var(--color-primary, #4F5BDF);
    color: var(--color-primary, #4F5BDF);
}
.tags-dropdown__arrow {
    width: 10px;
    height: 10px;
    flex-shrink: 0;
    color: #666;
    transition: transform 0.2s;
}
.tags-dropdown__arrow--open {
    transform: rotate(180deg);
}
.tags-dropdown__backdrop {
    position: fixed;
    inset: 0;
    z-index: 40;
}
/* Плавное появление панели (как BaseDropdown). */
.tags-dd-enter-active {
    transition: opacity 0.2s ease, transform 0.2s ease;
}
.tags-dd-leave-active {
    transition: opacity 0.15s ease, transform 0.15s ease;
}
.tags-dd-enter-from,
.tags-dd-leave-to {
    opacity: 0;
    transform: translateY(-4px);
}
.tags-dropdown__panel {
    transform-origin: top left;
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    z-index: 50;
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    min-width: 170px;
    padding: 10px;
    background: #fff;
    border: 1px solid #e0e0e0;
    border-radius: 12px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
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

@media (max-width: 768px) {
    .center {
        padding: 12px;
    }

    .filters-row {
        flex-wrap: wrap;
        gap: 8px;
    }

    .filters-row--secondary {
        flex-wrap: wrap;
        gap: 12px;
    }

    .table-toolbar {
        padding: 6px 8px;
    }

    .field {
        width: 100%;
    }

    .field__input {
        width: 100%;
    }

    .status-buttons {
        width: 100%;
    }

    /*
     * Таблица с фиксированными % колонками не помещается на мобильный экран.
     * Native scrollbar скрываем - вместо него рендерим собственный
     * .table-scroll-indicator под таблицей (всегда visible, не пропадает после
     * touch-скролла на iOS/Android).
     *
     * display: block вместо flex-column - базовый flex-container не даёт
     * children реально overflow'ить parent по X, scrollLeft кеппировался
     * на ~34px вместо 388. Плюс убираем max-height - vertical scroll делается
     * страницей body, inner overflow-y: scroll в .table-body отключён.
     */
    .applications-table {
        display: block;
        overflow-x: scroll;
        /* overflow-y: visible недопустим с overflow-x: scroll (CSS spec: computes to auto),
         * поэтому hidden. Вертикальный scroll страницы остаётся. */
        overflow-y: hidden;
        max-height: none;
        height: auto;
        /* transition: all 0.3s и глобальный scroll-behavior: smooth анимировали
         * scrollLeft - scroll останавливался посередине. Отключаем оба. */
        transition: none;
        scroll-behavior: auto;
        /* Native scrollbar виден в начальном состоянии (Firefox thin + Chrome 10px),
         * на touch-устройствах он скроется после инерции - это native-поведение. */
        scrollbar-width: auto;
        scrollbar-color: #4F5BDF #ededf5;
    }

    .applications-table::-webkit-scrollbar {
        height: 10px;
        -webkit-appearance: none;
    }

    .applications-table::-webkit-scrollbar-track {
        background: #ededf5;
        border-radius: 5px;
    }

    .applications-table::-webkit-scrollbar-thumb {
        background: #4F5BDF;
        border-radius: 5px;
    }

    .table-header,
    .table-body,
    .header-row,
    .application-item {
        /* actions-col скрыт на мобильном, суммарная ширина: 110+100+110+120+110+110 = 660 */
        min-width: 660px;
    }

    /* Padding 0 16px делал scrollWidth больше реальной ширины content - scroll
     * упирался в середину. Убираем padding, переносим в первый/последний col. */
    .table-header,
    .application-row {
        padding: 0;
    }

    .header-col:first-child,
    .application-col:first-child {
        padding-left: 16px;
    }

    /* actions-col скрыта, последний visible - status-col */
    .header-col.status-col,
    .application-col.status-col {
        padding-right: 16px;
    }

    .table-body {
        overflow-y: visible;
        overflow-x: visible;
        flex-grow: unset;
    }

    .applications-list {
        overflow: visible;
        -webkit-overflow-scrolling: touch;
    }

    .confirmation-col { min-width: 110px; }
    .number-col { min-width: 100px; }
    .date-col { min-width: 110px; }
    .organization-col { min-width: 120px; }
    .sender-col { min-width: 110px; }
    .status-col { min-width: 110px; }

    /* На мобильном actions-col (download button в каждой строке) скрываем -
     * download доступен через детали заявки при клике на строку. */
    .header-col.actions-col,
    .application-col.actions-col {
        display: none;
    }

    /* Заголовки header-col в одну строку - без wrap, визуально выровнены */
    .header-col p {
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .header-col {
        font-size: 13px;
    }

    /* На мобильном индикатор всегда видим */
    .table-scroll-indicator {
        display: block;
    }
}

@media (max-width: 480px) {
    .center {
        padding: 10px;
    }

    .center__tabs {
        gap: 6px;
    }
}

/* Кнопка звука в шапке */
.sound-btn-wrap {
    position: relative;
    margin-left: auto;
    flex-shrink: 0;
}

.sound-icon-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 34px;
    height: 34px;
    border: none;
    border-radius: 50%;
    background: transparent;
    color: var(--color-text-muted, #888);
    cursor: pointer;
    transition: background 0.15s ease, color 0.15s ease;
}

.sound-icon-btn:hover {
    background: var(--color-border, #e6e6e6);
    color: #444;
}

.sound-icon-btn--active {
    color: var(--color-primary);
}

.sound-icon-btn--active:hover {
    background: color-mix(in srgb, var(--color-primary) 12%, transparent);
}

/* Поповер настроек звука */
.sound-popover {
    position: absolute;
    top: calc(100% + 8px);
    right: 0;
    z-index: 500;
    width: 230px;
    background: #fff;
    border: 1px solid var(--color-border, #e6e6e6);
    border-radius: 20px;
    padding: 14px 16px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
    display: flex;
    flex-direction: column;
    gap: 12px;
}

.sound-popover__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
}

.sound-popover__title {
    font-size: 13px;
    font-weight: 600;
    color: #1a1a1a;
}

.sound-popover__field {
    display: flex;
    flex-direction: column;
    gap: 5px;
}

.sound-popover__label {
    font-size: 11px;
    color: var(--color-text-muted, #888);
    font-weight: 500;
}

.sound-popover__preview {
    align-self: flex-start;
    font-size: 12px;
    padding: 5px 14px;
}

/* Переиспользуем из AccountComponent */
.sound-toggle {
    position: relative;
    display: inline-flex;
    align-items: center;
    cursor: pointer;
    flex-shrink: 0;
}

.sound-toggle__input {
    position: absolute;
    opacity: 0;
    width: 0;
    height: 0;
}

.sound-toggle__track {
    display: inline-block;
    width: 36px;
    height: 20px;
    background: var(--color-border, #e6e6e6);
    border-radius: 999px;
    transition: background 0.2s ease;
    position: relative;
}

.sound-toggle__track::after {
    content: '';
    position: absolute;
    top: 3px;
    left: 3px;
    width: 14px;
    height: 14px;
    background: #fff;
    border-radius: 50%;
    transition: transform 0.2s ease;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.15);
}

.sound-toggle__input:checked + .sound-toggle__track {
    background: var(--color-primary);
}

.sound-toggle__input:checked + .sound-toggle__track::after {
    transform: translateX(16px);
}

.sound-volume {
    width: 100%;
    accent-color: var(--color-primary);
    cursor: pointer;
    height: 4px;
    border-radius: 4px;
}

/* Анимация появления поповера */
.sound-popover-enter-active,
.sound-popover-leave-active {
    transition: opacity 0.15s ease, transform 0.15s ease;
}

.sound-popover-enter-from,
.sound-popover-leave-to {
    opacity: 0;
    transform: translateY(-6px);
}
</style>