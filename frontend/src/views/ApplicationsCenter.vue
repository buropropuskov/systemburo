<template>
  <section ref="root" class="center">
    <header class="center__header">
      <div class="header-top">
        <h2 class="center__title">
          Центр заявок
        </h2>

        <!-- Десктоп: переключатель Активные/Архив в шапке (как до волны 3).
             На мобилке переключатель - дропдаун во втором ряду. -->
        <div
          v-if="canViewArchive && !isMobileHeader"
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

        <div class="header-top__actions">
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

          <!-- Мобилка: иконка-тоггл раскрывает поле поиска оверлеем влево (не всегда-видимый инпут) -->
          <button
            v-if="isMobileHeader"
            type="button"
            class="search-icon-btn"
            :class="{ 'search-icon-btn--active': showMobileSearch || !!searchQuery.trim() }"
            aria-label="Поиск заявок"
            @click="toggleMobileSearch"
          >
            <img
              src="@/assets/icons/search.png"
              class="search-icon-btn__img"
              alt=""
            >
          </button>
        </div>

        <!-- Мобилка: поле поиска раскрывается ВЛЕВО оверлеем поверх первого ряда
             (заголовок/бейдж/звук), не отдельным рядом ниже. Иконка справа - тоггл,
             крестик внутри - очистить и закрыть. -->
        <Transition name="center-search">
          <div
            v-if="isMobileHeader && showMobileSearch"
            class="center__search-overlay"
          >
            <div class="field search">
              <input
                ref="mobileSearchInput"
                v-model="searchQuery"
                placeholder="Поиск заявок..."
                type="text"
                class="field__input search"
                data-testid="center-input-search"
                @input="onSearchInput"
              >
              <button
                v-if="searchQuery.trim()"
                type="button"
                class="center__search-clear"
                aria-label="Очистить поиск"
                @click="clearMobileSearch"
              >
                &times;
              </button>
            </div>
          </div>
        </Transition>
      </div>

      <!-- Десктоп: инлайн-фильтры Центра (как до волны 3). На мобилке - в модалке. -->
      <div
        v-if="!isMobileHeader"
        class="center__filters"
      >
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
                alt=""
              >
            </div>

            <OrganizationFilter
              :value="selectedOrganizationId"
              :organizations="organizations"
              @change="handleOrganizationChange"
            />

            <OrganizationFilter
              :value="selectedCompanyId"
              :organizations="companies"
              all-label="Все компании"
              placeholder-text="Компания"
              @change="handleCompanyChange"
            />

            <DateFilter
              ref="dateFilter"
              mode="range"
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
                <button
                  class="status-btn status-btn--updates"
                  :class="{ 'status-btn--active': statusUpdatedOnly }"
                  data-testid="center-button-updates"
                  @click="toggleStatusUpdated"
                >
                  Обновления<template v-if="statusUpdateCount > 0">: {{ statusUpdateCount }}</template>
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
                    viewBox="0 0 24 24"
                    fill="none"
                    xmlns="http://www.w3.org/2000/svg"
                  >
                    <path
                      d="M6 9L12 15L18 9"
                      stroke="currentColor"
                      stroke-width="2.5"
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

      <!-- Мобилка: второй ряд - переключатель Активные/Архив (дропдаун) + кнопка Фильтр -->
      <div
        v-if="isMobileHeader"
        class="header-row2"
      >
        <div
          v-if="canViewArchive"
          class="center__tabs center__tabs--mobile"
        >
          <BaseDropdown
            :model-value="archiveMode"
            :options="archiveTabs"
            value-key="key"
            label-key="label"
            @update:model-value="archiveMode = $event"
          />
        </div>

        <!-- Обновить: на десктопе живёт в шапке ТАБЛИЦЫ (как исторически), здесь -
             только мобильный вариант, иконкой без текста (шапка таблицы на мобилке
             скрыта, иначе обновить нечем). Стоит перед «Фильтром» и одной с ним высоты. -->
        <RefreshButton
          :loading="refreshing"
          @refresh="fetchApplications"
        />

        <button
          type="button"
          class="filter-btn"
          :class="{ 'filter-btn--active': hasModalFilters }"
          data-testid="center-button-filter"
          @click="showFilterModal = true"
        >
          <svg
            class="filter-btn__icon"
            width="15"
            height="15"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <path d="M22 3H2l8 9.46V19l4 2v-8.54L22 3z" />
          </svg>
          Фильтр
          <span
            v-if="hasModalFilters"
            class="filter-btn__dot"
            aria-hidden="true"
          />
        </button>
      </div>
    </header>

    <!-- Модалка вторичных фильтров - только на мобилке (десктоп фильтрует инлайн).
         Организация тоже внутри модалки на мобилке (в шапке её нет). -->
    <ApplicationsFilterModal
      v-if="isMobileHeader"
      ref="filterModal"
      :show="showFilterModal"
      :organizations="organizations"
      :selected-organization-id="selectedOrganizationId"
      :companies="companies"
      :selected-company-id="selectedCompanyId"
      :selected-date="selectedDate"
      :date-range-start="dateRangeStart"
      :date-range-end="dateRangeEnd"
      :active-today="activeToday"
      :status-updated-only="statusUpdatedOnly"
      :status-update-count="statusUpdateCount"
      :confirmations="confirmations"
      :selected-confirmations="selectedConfirmations"
      :application-statuses="applicationStatuses"
      :selected-application-statuses="selectedApplicationStatuses"
      :tags="tags"
      :selected-tags="selectedTags"
      :sort-field="sortField"
      :sort-direction="sortDirection"
      :has-active-filters="hasActiveFilters"
      @close="showFilterModal = false"
      @organization-change="handleOrganizationChange"
      @company-change="handleCompanyChange"
      @update:selected-date="updateSelectedDate"
      @update:date-range-start="updateDateRangeStart"
      @update:date-range-end="updateDateRangeEnd"
      @apply-date="applyDateFilters"
      @clear-date="clearDateRange"
      @toggle-today="toggleActiveToday"
      @toggle-status-updated="toggleStatusUpdated"
      @toggle-confirmation="toggleConfirmation"
      @toggle-status="toggleApplicationStatus"
      @toggle-tag="toggleTag"
      @sort-by="sortBy"
      @reset-sort="resetSort"
      @reset-filters="resetFilters"
    />

    <div class="applications-table rt-table">
      <div class="table-header">
        <div class="header-row rt-head-row">
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
        ref="tableBody"
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
          <TransitionGroup
            v-if="filteredApplications.length > 0"
            tag="div"
            name="app-row"
            class="applications-list"
          >
            <template
              v-for="group in applicationGroups"
              :key="group.key"
            >
              <!-- Разделитель периода (серая линия + подпись), без дат (#1097 r2). -->
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
                :class="{
                  'unread': !application.is_read,
                  'status-updated': application.has_status_update && application.is_read,
                  'initial-load': isInitialLoad,
                  'filtered': !isInitialLoad
                }"
                :data-testid="`center-row-${application.id}`"
                @click="openApplication(application)"
              >
              <div class="application-row rt-row">
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
                  class="application-col number-col"
                  data-label="Номер заявки"
                >
                  <span
                    class="application-number application-number--copyable"
                    data-tooltip="Копировать"
                    role="button"
                    tabindex="0"
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
                  class="application-col organization-col"
                  data-label="Организация"
                >
                  <span class="ellip">{{ getOrganizationName(application) }}</span>
                </div>
                <div
                  class="application-col sender-col"
                  data-label="Отправитель"
                >
                  <span
                    v-if="application.sender_name"
                    class="sender-tooltip-anchor"
                    :data-tooltip="application.sender_full_name || application.sender_name"
                  ><span class="ellip">{{ application.sender_name }}</span></span>
                </div>
                <!-- Сопроводительное сообщение - только в компактной карточке на мобилке
                     (W3.7). На десктопе колонка display:none, отдельной колонки таблицы нет. -->
                <div
                  v-if="application.message"
                  class="application-col message-col"
                >
                  {{ messagePreview(application.message) }}
                </div>
                <div
                  class="application-col status-col"
                  data-label="Статус заявки"
                >
                  <span class="status-badge-wrap">
                    <span
                      class="status-badge"
                      :class="getStatusClass(application.status)"
                      :title="application.status"
                    >
                      {{ application.status }}
                    </span>
                    <!-- Пульс-точка "статус обновился" (#1349): только у прочитанных заявок -
                         непрочитанные и так подсвечены жёлтым, флаг обновления для них шум. -->
                    <span
                      v-if="application.has_status_update && application.is_read"
                      class="status-update-dot"
                      :data-testid="`center-status-dot-${application.id}`"
                      aria-hidden="true"
                    />
                  </span>
                </div>
                <div
                  class="application-col tags-col"
                  data-label="Теги"
                >
                  <div
                    v-if="blacklistFlagCount(application) > 0 || application.has_roof_access || application.has_free_parking || application.sender_is_important || application.has_unseen_questions || pendingApprovalDays(application) !== null"
                    class="application-tags"
                    :class="{ 'application-tags--compact': tagsAreCompact(application) }"
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
            </template>
          </TransitionGroup>
          <!-- In-flight retry / первичная догрузка при пустом списке (#1173): пока
               listLoading, показываем спиннер, а не проваливаемся в error/"Заявок нет".
               listLoading выставляет composable из retry() (верхнеуровневый loading он
               не трогает), поэтому без этой ветки клик "Повторить" на долю секунды
               рисует "Заявок нет". -->
          <div
            v-else-if="listLoading"
            class="list-loading-state"
            data-testid="center-list-loading"
          >
            <LoaderSpinner label="Загрузка…" />
          </div>
          <!-- Первичная загрузка упала (#1173): список пуст из-за ошибки бэка, а не
               потому что заявок реально нет - показываем error+retry вместо "Заявок нет". -->
          <div
            v-else-if="listError"
            class="list-error-state"
            data-testid="center-list-error"
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
          <p
            v-else
            class="no-data-message"
          >
            {{ searchQuery.trim() ? 'Ничего не найдено' : hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Заявок нет' }}
          </p>
        </SkeletonTransition>

        <!-- Бесшовная подгрузка (#1158): sentinel внизу СКРОЛЛИРУЕМОГО table-body -
             IntersectionObserver триггерит loadMore без кнопки "Показать ещё".
             root - сам table-body: у него свой overflow-y:scroll, не документ,
             дефолтный root (viewport) пересечение бы не заметил. -->
        <div
          v-if="hasMoreApplications"
          :ref="setSentinelRef"
          class="scroll-sentinel"
          data-testid="center-scroll-sentinel"
        >
          <LoaderSpinner
            v-if="listLoading && !refreshing"
            label="Загрузка…"
          />
          <!-- Ошибка догрузки следующей порции (#1173): список уже частично загружен,
               автодогрузка остановлена circuit-breaker'ом - компактный retry рядом с sentinel
               вместо бесконечного зависшего спиннера. -->
          <div
            v-else-if="listError"
            class="sentinel-error"
            data-testid="center-scroll-sentinel-error"
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
        v-if="!loading && sortedApplications.length"
        class="table-footer"
        data-testid="center-table-footer"
      >
        {{ footerText }}
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
      @download="downloadApplication"
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
import { getApplicationsPaginated, getApplicationById } from '@/api/applications'
import eventStream from '@/services/eventStream'
import { useAuthStore } from '@/stores/auth'
import { useSoundStore } from '@/stores/sound'
import { usePermissionsStore } from '@/stores/permissions'
import { useInfiniteList } from '@/composables/useInfiniteList'
import { getViewportZoom } from '@/utils/viewportScale'
import { playPreset, SOUND_PRESETS } from '@/utils/notificationSound'
import { groupApplicationsByPeriod } from '@/utils/applicationPeriod'
import OrganizationFilter from '@/components/OrganizationFilter.vue';
import DateFilter from '@/components/DateFilter.vue';
import RefreshButton from '../components/RefreshButton.vue';
import ApplicationDetail from '../components/ApplicationDetail/ApplicationDetail.vue';
import ApplicationsFilterModal from '@/components/ApplicationsFilterModal.vue';
import FilterTabs from '@/components/ui/FilterTabs.vue';
import SkeletonTransition from '@/components/ui/SkeletonTransition.vue';
import SkeletonTable from '@/components/ui/SkeletonTable.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import DownloadBlanksModal from '@/components/applications/DownloadBlanksModal.vue';
import Badge from '@/components/ui/Badge.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import { blacklistFlagCount, blacklistFlagLabel, BLACKLIST_FLAG_TITLE } from '@/utils/blacklistBadge';
import { pendingApprovalDays, pendingApprovalLabel, pendingApprovalShort } from '@/utils/pendingApproval';
import { stripHtml } from '@/utils/sanitize';
import { useDeletionsStore } from '@/stores/deletions';

// Размер порции бесшовной подгрузки Центра (#1158, срез 1) - аналог PER_PAGE
// в AccessibleAttachmentsView/TableVersionsView.
const APPLICATIONS_PER_PAGE = 30;

export default {
    name: 'ApplicationsCenter',
    components: {
        OrganizationFilter,
        DateFilter,
        RefreshButton,
        ApplicationDetail,
        ApplicationsFilterModal,
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
        // Бесшовная подгрузка Центра порциями (#1158, срез 1): composable инкапсулирует
        // page/per_page/аккумуляцию/hasMore/seq-guard. fetchPage строится в methods
        // (нужен доступ к this для фильтров) и передаётся при каждом вызове -
        // setup() не имеет доступа к this, поэтому composable не хранит fetchPage сам.
        // applications - алиас infiniteList.items: pre-existing спеки читают/пишут
        // wrapper.vm.applications напрямую, переименование сломало бы их без пользы.
        const infiniteList = useInfiniteList({ perPage: APPLICATIONS_PER_PAGE })
        return {
            soundStore,
            permissionsStore,
            applications: infiniteList.items,
            total: infiniteList.total,
            applicationsPage: infiniteList.page,
            hasMoreApplications: infiniteList.hasMore,
            // canLoadMoreApplications/listError/retryApplicationsList (#1173) - устойчивость
            // бесшовной подгрузки к ошибкам бэка (5xx/сеть). canLoadMore гейтит АВТОдогрузку
            // (observer + loadAllRemaining) - в отличие от hasMoreApplications, которым
            // по-прежнему гейтится видимость sentinel-контейнера (внутри него рисуется
            // error+retry, поэтому он должен остаться видим и при ошибке).
            canLoadMoreApplications: infiniteList.canLoadMore,
            listLoading: infiniteList.loading,
            listError: infiniteList.error,
            loadApplicationsList: infiniteList.load,
            loadMoreApplicationsList: infiniteList.loadMore,
            retryApplicationsList: infiniteList.retry,
            observeApplicationsSentinel: infiniteList.observeSentinel,
            disconnectApplicationsSentinel: infiniteList.disconnectObserver,
        }
    },
    data() {
        return {
            showDownloadModal: false,
            showFilterModal: false,
            showSoundPopover: false,
            // Мобильная шапка (<=768): двухрядная раскладка, фильтры в модалке,
            // поиск раскрывается по иконке. Десктоп - инлайн-фильтры (как до волны 3).
            isMobileHeader: false,
            showMobileSearch: false,
            tagsDropdownOpen: false,
            soundPresets: SOUND_PRESETS,
            downloadAppId: 0,
            downloadAppInfo: null,
            searchQuery: '',
            searchDebounceTimer: null,
            selectedOrganizationId: null,
            selectedOrganizationName: '',
            selectedCompanyId: null,
            selectedCompanyName: '',
            selectedConfirmations: [],
            selectedApplicationStatuses: [],
            selectedTags: [],
            organizations: [],
            companies: [],
            sortField: null,
            sortDirection: 'desc',
            shouldShake: false,
            shakeInterval: null,
            applicationsPollInterval: null,
            sseConnected: false,
            eventStreamOff: null,
            eventStreamStatusOff: null,
            fetchSeq: 0,
            pendingRefreshCount: 0,
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
            // Чип "Обновления" (#1349): серверный фильтр status_updated=true - только
            // заявки, статус/подтверждение которых менялись после последнего просмотра.
            statusUpdatedOnly: false,

            loading: true,
            refreshing: false,

            // Данные заявок: applications/total/hasMoreApplications/listLoading
            // выставлены из useInfiniteList в setup() (#1158).

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

            // Фильтр по компании
            if (this.selectedCompanyId) {
                filtered = filtered.filter(app =>
                    app.company_id === this.selectedCompanyId
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
        
        // Группировка по периодам для визуальных разделителей (#1097 r2). Возвращает
        // массив групп {label,key,apps[]}; при сортировке НЕ по дате - одна группа без
        // подписи (порядок не по времени, разделители не рисуем). Разделитель ставится,
        // когда бакет периода меняется в уже отсортированном списке -> учитывает сортировку.
        applicationGroups() {
            const sortedByDate = !this.sortField || this.sortField === 'date';
            return groupApplicationsByPeriod(this.sortedApplications, sortedByDate);
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
                   !!this.selectedCompanyId ||
                   this.hasModalFilters;
        },

        // Только фильтры ВНУТРИ модалки «Фильтр» - для точки-индикатора на кнопке
        // (кнопка мобильная, поэтому набор = что в модалке на мобилке). Организация тоже
        // в модалке на мобилке -> входит в индикатор. Поиск НЕ входит: он в шапке отдельно
        // (иконка -> раскрывающееся поле), иначе точка горела бы при пустой модалке.
        hasModalFilters() {
            return this.selectedConfirmations.length > 0 ||
                   this.selectedApplicationStatuses.length > 0 ||
                   this.selectedTags.length > 0 ||
                   !!this.selectedDate ||
                   !!(this.dateRangeStart && this.dateRangeEnd) ||
                   this.activeToday ||
                   this.statusUpdatedOnly ||
                   !!this.selectedOrganizationId ||
                   !!this.selectedCompanyId;
        },

        // Известное ограничение (#1158 срез 1): applications - только загруженные
        // порции, не весь набор по текущим фильтрам, значит счётчик может занижать
        // реальное число непрочитанных, если их больше, чем в загруженных порциях.
        // На практике непрочитанные - обычно самые свежие (sending_datetime DESC),
        // поэтому почти всегда попадают в первую порцию; точный счётчик независимо
        // от пагинации - отдельный срез (напр. через getUnreadCount с текущими фильтрами).
        unreadCount() {
            return this.applications.filter(app => !app.is_read).length;
        },

        // Число заявок с обновлённым статусом среди загруженных (#1349). То же
        // ограничение пагинации, что и unreadCount (счётчик по загруженным порциям):
        // при активном чипе фильтр возвращает ровно flagged-заявки и счётчик точен.
        // Гейт is_read зеркалит визуальную точку (Центр показывает флаг лишь у
        // прочитанных - у непрочитанных своя жёлтая подсветка).
        statusUpdateCount() {
            return this.applications.filter(app => app.has_status_update && app.is_read).length;
        },

        // Теги фильтруются клиентски (бэк их не знает), сортировка по колонкам - тоже
        // клиентски. Чтобы они работали по ВСЕМУ набору (как на dev до пагинации), а не
        // по одной загруженной порции, при их активности отключаем инкрементальную
        // пагинацию и догружаем весь набор (см. loadAllRemaining). Долг: перенос
        // тегов/сортировки на бэк - отдельный будущий срез (#1158).
        isFullLoad() {
            return this.selectedTags.length > 0 || !!this.sortField;
        },

        // Футер "Показано X из Y": показываем реально ВИДИМОЕ число строк
        // (sortedApplications), а не applications - иначе при клиентском фильтре по тегам/
        // мультивыборе он завышал бы. "из {total}" дописываем только когда клиентские
        // фильтры набор не урезали (sorted === applications): total бэковый, тегов/
        // мультивыбора не знает и с ними врал бы (#1158).
        showTotalInFooter() {
            return this.sortedApplications.length === this.applications.length;
        },

        footerText() {
            const shown = this.sortedApplications.length;
            return this.showTotalInFooter ? `Показано ${shown} из ${this.total}` : `Показано ${shown}`;
        }
    },
    watch: {
        archiveMode() {
            // Смена «вселенной» данных: Активные и Архив - непересекающиеся пространства
            // id. Снимок известных id (_pollKnownIds) от прошлого набора здесь стал бы
            // стейл - инкрементальный опрос принял бы ВСЕ недогруженные страницы нового
            // набора за «новые» (bulk-prepend + рассинхрон total + ложный звук, класс
            // #632/#840). Инвалидируем снимок и снимаем prime, чтобы первый снимок нового
            // набора не сыграл звук, ДО refetch (#1158).
            this._pollKnownIds = null;
            this.pollPrimed = false;
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
        this.fetchCompanies();
        this.fetchApplications().then(() => this.openFromDeepLink());
        this.getCurrentUser();

        // Polling 30s (#1158): фоновый рефреш НЕ должен схлопывать накопленный скролл.
        // Если подгружено >1 порции - только инкрементальный синк (prepend новых +
        // обновление полей существующих), без reset. Иначе (первая порция, терять
        // нечего) - раз в 5 минут полный reload статусов, между ними инкрементальный
        // опрос без real-time (при активном SSE новые прилетают сигналом).
        let _fullReloadCounter = 0;
        this.applicationsPollInterval = setInterval(() => {
            if (this.isInitialLoad) return;
            if (this.applicationsPage > 1) {
                // Инкрементальный синк нужен только без real-time: при активном SSE
                // новые/изменения прилетают сигналом (refreshFromRealtime). Гейт
                // симметричен ветке page===1 ниже (#1158 yellow).
                if (!this.sseConnected) this._pollApplicationsIncremental();
                return;
            }
            _fullReloadCounter++;
            if (_fullReloadCounter >= 10) {
                _fullReloadCounter = 0;
                this.fetchApplications();
            } else if (!this.sseConnected) {
                this._pollApplicationsIncremental();
            }
        }, 30000);

        // Real-time обновление Центра (#840): по сигналу сервера тихо рефрешим (#1158).
        eventStream.connect();
        this.eventStreamOff = eventStream.subscribe('applications-center', async () => {
            if (this.isInitialLoad) return; // как поллинг: не трогаем в первую секунду после mount
            await this.refreshFromRealtime();
        });
        this.eventStreamStatusOff = eventStream.onStatus((status) => {
            this.sseConnected = status === 'connected';
        });

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

        this.initMobileWatcher();

        // Десктоп: тянем .center на оставшуюся высоту вьюпорта, чтобы таблица
        // скроллилась внутри (.table-body), а документ НЕ рос и не мигал скроллбаром
        // (иначе появление/исчезновение скроллбара двигало обе шапки по горизонтали, а
        // resize пересчитывал --app-vh -> осцилляция). Эталон CarsView/EmployeeView;
        // на мобилке (<=768) высота сбрасывается - там список скроллит страница
        // (sticky-шапка, @media 767).
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
        this.disconnectApplicationsSentinel();
        window.removeEventListener('resize', this._applyHeight);
        if (this._headerObs) {
            this._headerObs.disconnect();
            this._headerObs = null;
        }
        if (this.shakeInterval) {
            clearInterval(this.shakeInterval);
        }
        if (this.applicationsPollInterval) {
            clearInterval(this.applicationsPollInterval);
        }
        if (this.eventStreamOff) this.eventStreamOff();
        if (this.eventStreamStatusOff) this.eventStreamStatusOff();
        eventStream.disconnect();
        if (this.searchDebounceTimer) {
            clearTimeout(this.searchDebounceTimer);
        }
        document.removeEventListener('keydown', this._soundEscapeHandler);
        document.removeEventListener('mousedown', this._soundClickOutsideHandler);
        if (this._mobileMql && this._onMobileChange) {
            if (this._mobileMql.removeEventListener) {
                this._mobileMql.removeEventListener('change', this._onMobileChange);
            } else if (this._mobileMql.removeListener) {
                this._mobileMql.removeListener(this._onMobileChange);
            }
        }
    },
    methods: {
        can(key) {
            return this.permissionsStore.hasPermission(key);
        },
        /**
         * Десктоп: фиксирует высоту .center = оставшаяся высота вьюпорта под шапкой,
         * чтобы таблица заполняла экран и скроллилась внутри, а документ не рос (нет
         * мигания скроллбара -> нет прыжка шапок). На мобилке (<=768) сбрасываем -
         * список скроллит страница (sticky-шапка). rect.top под корневым zoom -
         * device-px, innerHeight - НЕзумленный: делим на zoom (эталон
         * AdminPageShell/EmployeeView), иначе на мониторах >1440 .center уходит ниже экрана.
         */
        _applyHeight() {
            const el = this.$refs.root;
            if (!el) return;
            if (window.innerWidth <= 768) {
                el.style.height = '';
                this._lastHeight = -1;
                return;
            }
            const top = el.getBoundingClientRect().top;
            const height = Math.max(0, Math.round((window.innerHeight - top) / getViewportZoom()));
            if (height === this._lastHeight) return;
            this._lastHeight = height;
            el.style.height = `${height}px`;
        },
        /**
         * Реактивно отслеживает мобильный брейкпоинт (совпадает с CSS @media 768,
         * тот же порог, что в TheHeader): на нём шапка Центра двухрядная, фильтры в
         * модалке, поиск раскрывается по иконке. На десктопе - инлайн-фильтры.
         */
        initMobileWatcher() {
            if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
            this._mobileMql = window.matchMedia('(max-width: 768px)');
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
            this.onSearchInput();
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
                useDeletionsStore().notify({ prefix: 'Номер ', bold: String(number), suffix: ' скопирован' });
            } catch {
                useDeletionsStore().notify({ prefix: 'Не удалось скопировать номер', type: 'error' });
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

        // Компания фильтруется зеркально организации: server-параметр в buildApplicationsPage
        // + клиентский предикат в filteredApplications, applyFilters перезапрашивает список.
        handleCompanyChange({ id, name }) {
            this.selectedCompanyId = id;
            this.selectedCompanyName = name;
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

        toggleStatusUpdated() {
            this.statusUpdatedOnly = !this.statusUpdatedOnly;
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
            this.selectedCompanyId = null;
            this.selectedCompanyName = '';
            this.selectedConfirmations = [];
            this.selectedApplicationStatuses = [];
            this.selectedTags = [];
            this.selectedDate = null;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            this.activeToday = false;
            this.statusUpdatedOnly = false;

            this.resetSort();
            this.tagsDropdownOpen = false;

            // Организация: OrganizationFilter привязан через :value к selectedOrganizationId
            // (и в шапке десктопа, и в модалке), поэтому обнуление выше уже гасит её отображение
            // через immediate-watcher компонента - ref.reset() не нужен.

            // DateFilter: сброс date-пропсов в null гасит отображение через его watcher'ы,
            // а clearDateFilter/clearSelection добивают activeQuickDate-подсветку. На десктопе
            // DateFilter инлайн (ref dateFilter), на мобилке - внутри модалки (ref filterModal).
            if (this.$refs.dateFilter && this.$refs.dateFilter.clearSelection) {
                this.$refs.dateFilter.clearSelection();
            }
            if (this.$refs.filterModal && this.$refs.filterModal.clearDateFilter) {
                this.$refs.filterModal.clearDateFilter();
            }

            this.isInitialLoad = false;
            // fetchApplications() вместо applyFilters() — часть фильтров (organization_id,
            // date, archive) применяется на бэке через URL params. Без fetch applications
            // остаётся подмножеством, и после сброса таблица продолжает показывать только его.
            this.fetchApplications();
        },
        
        // Организация/подтверждение/статус/дата - все читаются бэком в buildApplicationsPage
        // (#1158): без refetch applications остался бы первой уже ЗАГРУЖЕННОЙ порцией,
        // а не полным набором по новому фильтру (до пагинации applications держал весь
        // список, и клиентский фильтр этого не требовал). Теги (ЧС/крыша/парковка/важный)
        // бэком не поддерживаются - для них refetch не меняет ответ, но всё равно
        // возвращает на страницу 1, не запирая пользователя в маленьком уже загруженном
        // подмножестве без шанса догрузить больше подходящих через скролл.
        applyFilters() {
            this.isInitialLoad = false;
            this.fetchApplications();
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
            // Сортировка по колонке клиентская - должна идти по всему набору (как на dev).
            // При входе в full-load, если ещё не всё загружено, догружаем остаток (#1158).
            if (this.isFullLoad && this.hasMoreApplications) {
                this.fetchApplications();
            }
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

        // Сообщение заявки - rich-HTML из TextConstructor. В компактной карточке
        // показываем плоский текст одной строкой с обрезкой (без тегов).
        messagePreview(html) {
            return stripHtml(html);
        },

        // Date -> YYYY-MM-DD по ЛОКАЛЬНЫМ частям (не toISOString: UTC-сдвиг увёл бы
        // выбранный день назад у пользователей восточнее UTC, #1158/#1076).
        toLocalYMD(date) {
            const d = date instanceof Date ? date : new Date(date);
            const pad = (n) => String(n).padStart(2, '0');
            return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
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

        pendingApprovalDays,
        pendingApprovalLabel,
        pendingApprovalShort,

        // Колонка тегов фиксированная (120/90px) - один текстовый тег влезает, два и больше нет.
        // При 2+ тегах сворачиваем крыша/парковка/важный в иконки (ЧС держим текстом). Решение
        // по данным, а не замером DOM - реактивно, без дёрганья на ре-рендерах.
        tagsAreCompact(application) {
            const count =
                (this.blacklistFlagCount(application) > 0 ? 1 : 0) +
                (this.pendingApprovalDays(application) !== null ? 1 : 0) +
                (application.has_roof_access ? 1 : 0) +
                (application.has_free_parking ? 1 : 0) +
                (application.sender_is_important ? 1 : 0) +
                (application.has_unseen_questions ? 1 : 0);
            return count >= 2;
        },

        // API методы

        /**
         * Тихий real-time рефреш Центра (#1158). НЕ пересобираем накопленный скролл:
         * при подгруженных >1 порции обновляем инкрементально (prepend новых сверху +
         * синк полей существующих), не затирая страницы 2+. Первая порция - терять
         * нечего, тихо перезапрашиваем page 1 (фильтр-aware) со звуком на новую заявку.
         */
        async refreshFromRealtime() {
            if (this.applicationsPage > 1) {
                await this._pollApplicationsIncremental();
                return;
            }
            const beforeIds = new Set(this.applications.map((a) => a.id));
            await this.fetchApplications(true); // тихо: без оверлея, TransitionGroup анимирует дельту
            // Звук на реально новую заявку. route === '/center' симметрично гейту NavMenu
            // (там route !== '/center') - если юзер ушёл со страницы, пока летел запрос,
            // звук сыграет только NavMenu, без двойного бипа на стыке навигации.
            const hasNew = this.applications.some((a) => !beforeIds.has(a.id));
            if (hasNew && this.pollPrimed && this.soundStore.enabled && this.$route?.path === '/center') {
                playPreset(this.soundStore.selectedPreset, this.soundStore.volume);
            }
        },

        /**
         * Инкрементальный синк без reset (#1158): фоново подтягивает полный серверный
         * список (только архив) и (1) обновляет изменяемые поля уже загруженных строк -
         * это безопасно всегда, набор не двигается и скролл не прыгает; (2) БЕЗ активных
         * фильтров prepend'ит реально новые заявки сверху со звуком.
         *
         * Новизну определяем membership'ом против снимка ВСЕГО серверного списка с
         * прошлого опроса (this._pollKnownIds), а НЕ по id-порогу: id-порог не ловил
         * появление в архиве, где id не монотонен дате подачи (#1158 yellow). Первый
         * опрос лишь инициализирует снимок (prevIds пуст -> ничего не prepend'им).
         */
        async _pollApplicationsIncremental() {
            try {
                const authStore = useAuthStore();
                if (!authStore.token) return;

                const params = new URLSearchParams();
                params.append('archive', this.archiveMode === 'archive' ? 'true' : 'false');
                const response = await apiRequest(`/applications?${params}`, { method: 'GET' });
                if (!response.ok) return;

                const fresh = await response.json();
                if (!Array.isArray(fresh)) return;

                // (1) Синк изменяемых серверных полей у уже загруженных строк - безопасно
                // всегда (не двигает набор, не сбрасывает скролл), работает и под фильтрами.
                const freshById = Object.fromEntries(fresh.map(a => [a.id, a]));
                this.applications = this.applications.map(a => {
                    const updated = freshById[a.id];
                    if (!updated) return a;
                    if (
                        updated.status !== a.status ||
                        updated.confirmation !== a.confirmation ||
                        updated.is_read !== a.is_read ||
                        updated.has_unseen_questions !== a.has_unseen_questions ||
                        updated.has_status_update !== a.has_status_update
                    ) {
                        return { ...a, ...updated };
                    }
                    return a;
                });

                // (2) Prepend новых - только без активных фильтров: под фильтром новая
                // заявка может не подходить под условие, prepend был бы некорректен (её
                // судьбу решит явное действие/следующий reset).
                const hasFilters = !!(
                    this.searchQuery ||
                    this.selectedOrganizationId ||
                    this.selectedCompanyId ||
                    this.selectedConfirmations.length ||
                    this.selectedApplicationStatuses.length ||
                    this.selectedTags.length ||
                    this.selectedDate ||
                    (this.dateRangeStart && this.dateRangeEnd) ||
                    this.activeToday ||
                    this.statusUpdatedOnly
                );
                if (!hasFilters) {
                    const prevIds = this._pollKnownIds;
                    const loadedIds = new Set(this.applications.map(a => a.id));
                    const newlyAppeared = prevIds
                        ? fresh.filter(a => !prevIds.has(a.id) && !loadedIds.has(a.id))
                        : [];
                    if (newlyAppeared.length > 0) {
                        // Порядок в сыром массиве косметичен: sortedApplications всё равно
                        // сортирует по дате. Синкаем total композабла, иначе hasMore/футер
                        // "Показано X из Y" разъедутся (#1158 yellow).
                        this.applications = [...newlyAppeared, ...this.applications];
                        this.total += newlyAppeared.length;
                        if (this.pollPrimed && this.soundStore.enabled && this.$route?.path === '/center') {
                            playPreset(this.soundStore.selectedPreset, this.soundStore.volume);
                        }
                    }
                }

                // Снимок всех серверных id для сравнения на следующем опросе.
                this._pollKnownIds = new Set(fresh.map(a => a.id));
            } catch {
                // сетевой сбой — не критично, следующий poll сам восстановится.
                // Базу/снимок НЕ трогаем (иначе восстановление = ложный "рост с нуля", #840).
            }
        },

        /**
         * fetchPage для useInfiniteList (#1158): строит те же query-параметры фильтра,
         * что и раньше, плюс page/per_page - бэк переключается на GetApplicationsPaginated,
         * как только видит per_page (см. internal/handlers/applications.go). Идёт через
         * getApplicationsPaginated (api/applications.js), которая читает envelope.meta
         * через apiRequestRaw - apiRequest снимает его вместе с data (см. getAccessibleAttachments).
         */
        async buildApplicationsPage(page, perPage) {
            const params = {};

            if (this.searchQuery) {
                params.search_query = this.searchQuery;
            }
            if (this.selectedOrganizationId) {
                params.organization_id = this.selectedOrganizationId;
            }
            if (this.selectedCompanyId) {
                params.company_id = this.selectedCompanyId;
            }
            // Мультивыбор чипов = OR: шлём ВСЕ выбранные (бэк матчит IN), а не только
            // первый - иначе при выборе нескольких статусов/подтверждений сервер отдавал
            // подмножество по [0] и с пагинацией остальные не подгружались.
            if (this.selectedConfirmations.length > 0) {
                params.confirmation = this.selectedConfirmations.join(',');
            }
            if (this.selectedApplicationStatuses.length > 0) {
                // "Непрочитано" - псевдо-статус (нет записи в application_reads для юзера),
                // а НЕ значение колонки a.status (мигрирован в "В обработке"). Шлём его
                // отдельным флагом unread, иначе бэк искал бы a.status='Непрочитано' и
                // возвращал пусто ("нет заявок"). Остальные статусы - как есть.
                const statuses = this.selectedApplicationStatuses.filter(s => s !== 'Непрочитано');
                if (statuses.length > 0) {
                    params.status = statuses.join(',');
                }
                if (this.selectedApplicationStatuses.includes('Непрочитано')) {
                    params.unread = 'true';
                }
            }

            // Добавляем параметр архива
            params.archive = this.archiveMode === 'archive' ? 'true' : 'false';

            if (this.activeToday) {
                params.active_today = 'true';
            }

            // Чип "Обновления" (#1349): бэк фильтрует по hasStatusUpdatePredicate с гейтом
            // прочтения (requireRead=true для Центра, applyStatusUpdatedFilter).
            if (this.statusUpdatedOnly) {
                params.status_updated = 'true';
            }

            // Дата в query - ЛОКАЛЬНЫМИ частями (не toISOString: UTC увозит день назад
            // у пользователей восточнее UTC). Одиночная дата: бэк не знает поля `date`
            // (только date_from/date_to в ApplicationFilter), поэтому шлём day как
            // date_from=date_to=day (#1158).
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

            const { items, meta } = await getApplicationsPaginated(params);
            return { items, total: (meta && meta.total) || 0 };
        },

        async fetchApplications(silent = false) {
            // seq-токен: fetchApplications дёргается фильтрами, поллингом, ручным refresh
            // и SSE-сигналом (#840) - при пачке вызовов пишем только ответ последнего (#632).
            // Собственный seq-guard записи items/total уже даёт useInfiniteList - этот
            // токен управляет loading/refreshing/pendingRefreshCount (другой side-effect).
            // silent (real-time push): без оверлея refreshing - список обновляется тихо,
            // а TransitionGroup анимирует только дельту (новые заявки въезжают, соседи едут).
            const seq = ++this.fetchSeq;
            if (!silent) {
                this.pendingRefreshCount += 1;
                this.refreshing = true;
            }
            try {
                const authStore = useAuthStore();
                if (!authStore.token) {
                    console.error("Пользователь не авторизован.");
                    return;
                }

                // reset: смена фильтра/поиска/архива/сортировки-по-серверу должна начинать
                // с первой страницы и затирать накопленное - fetchApplications уже вызывается
                // из applyFilters/resetFilters/тоглов и т.п., поэтому reset здесь всегда true;
                // догрузка следующих порций идёт через loadMoreApplications (сентинел).
                await this.loadApplicationsList(this.buildApplicationsPage, { reset: true });
                if (seq !== this.fetchSeq) return; // устарел - актуальный запрос уже идёт
                // Клиентские теги/сортировка требуют ВЕСЬ набор (как на dev): догружаем
                // оставшиеся порции, чтобы фильтр/сортировка шли по полному списку (#1158).
                if (this.isFullLoad) {
                    await this.loadAllRemaining(seq);
                    if (seq !== this.fetchSeq) return;
                }
                // После первого полного fetch включаем инкрементальный polling со звуком.
                this.pollPrimed = true;
            } catch (error) {
                console.error("Ошибка сети при загрузке заявок:", error);
            } finally {
                if (seq === this.fetchSeq) {
                    this.loading = false;
                }
                // refreshing развязан от seq-токена (по ревью): считаем активные non-silent
                // запросы, оверлей гаснет с завершением последнего. Иначе гонка silent(SSE)
                // и non-silent(фильтр) могла оставить оверлей «Обновление…» залипшим.
                if (!silent) {
                    this.pendingRefreshCount -= 1;
                    if (this.pendingRefreshCount <= 0) {
                        this.pendingRefreshCount = 0;
                        this.refreshing = false;
                    }
                }
            }
        },

        // Догрузка всех оставшихся порций (full-load режим: теги/сортировка по всему
        // набору, #1158). seq-guard: если пользователь сменил фильтр/сортировку и стартовал
        // новый fetchApplications - прекращаем устаревший проход. Предохранитель guard от
        // бесконечного цикла на случай, если total/hasMore разъедутся.
        async loadAllRemaining(seq) {
            let guard = 0;
            // canLoadMoreApplications (не hasMoreApplications, #1173): если бэк упал на
            // какой-то из промежуточных страниц, circuit-breaker останавливает цикл сразу,
            // а не только на guard>200 - иначе tight-loop долбит упавший бэк 200 раз подряд.
            while (this.canLoadMoreApplications && seq === this.fetchSeq) {
                await this.loadMoreApplicationsList(this.buildApplicationsPage);
                if (++guard > 200) break;
            }
        },

        // Автодогрузка следующей порции по пересечению sentinel (#1158).
        // el=null при v-if="hasMoreApplications"===false просто отключает observer.
        // root: на ДЕСКТОПЕ скроллпорт - .table-body (overflow-y:scroll). На МОБИЛКЕ
        // @media снимает внутренний скролл (overflow-y:visible) и список скроллит
        // документ: .table-body там НЕ скроллпорт, его root-прямоугольник равен полной
        // высоте списка -> sentinel пересечён ВСЕГДА -> loadMore зацикливается и
        // непрерывно наращивает DOM во время прокрутки. Поэтому на мобилке root=null
        // (скроллер - документ).
        setSentinelRef(el) {
            const root = this.isMobileHeader ? null : (this.$refs.tableBody || null);
            this.observeApplicationsSentinel(el, this.buildApplicationsPage, { root });
        },

        // Ручной повтор упавшей страницы (первичной или догрузки, #1173) - composable
        // сам помнит, какой fetchPage/режим (reset/append) последним завершился ошибкой.
        async retryApplications() {
            try {
                await this.retryApplicationsList();
                // full-load (клиентские теги/сортировка): retry вернул только упавшую
                // страницу, но сортировка/фильтр идут по ВСЕМУ набору - дозагружаем
                // остаток, иначе результат по НЕПОЛНОМУ списку до ручного доскролла (#1173).
                if (this.isFullLoad) {
                    await this.loadAllRemaining(this.fetchSeq);
                }
            } catch (error) {
                console.error("Ошибка сети при повторной попытке загрузки заявок:", error);
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

        async fetchCompanies() {
            try {
                const response = await apiRequest("/companies", {
                    method: "GET",
                });

                if (response.ok) {
                    const data = await response.json();
                    this.companies = Array.isArray(data) ? data : [];
                } else {
                    console.error("Ошибка при загрузке компаний");
                }
            } catch (error) {
                console.error("Ошибка сети при загрузке компаний:", error);
            }
        },

        downloadApplication(application) {
            this.downloadAppId = application.id;
            this.downloadAppInfo = application;
            this.showDownloadModal = true;
        },

        async openApplication(application) {
            // Копится причина пересчитать бейдж NavMenu (прочтение и/или гашение флага
            // статуса). Эмитим ОДИН раз в конце: заявка, что и непрочитана, и с обновлением
            // статуса, иначе дала бы два запроса /unread-count подряд в одном тике.
            let badgeChanged = false;

            if (!application.is_read) {
                try {
                    const response = await apiRequest(`/applications/${application.id}/read`, {
                        method: "POST"
                    });
                    if (response.ok) {
                        application.is_read = true;
                        badgeChanged = true;
                    }
                } catch (error) {
                    console.error("Ошибка при отметке прочтения:", error);
                }
            }

            // Оптимистичное гашение флага "статус обновился" (#1349): открытие детали
            // дёргает GET /:id/details -> MarkStatusSeen (seen_at=now) гасит флаг на сервере.
            // Гасим точку в списке сразу, не дожидаясь следующего опроса.
            if (application.has_status_update) {
                application.has_status_update = false;
                badgeChanged = true;
            }

            // NavMenu слушает 'application-read' и перезапрашивает {count, status_updates}
            // разом, не дожидаясь 30с-опроса.
            if (badgeChanged) {
                this.$bus.emit('application-read', application.id);
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
        async openFromDeepLink() {
            const openId = Number(this.$route.query.open);
            if (!openId) return;
            // Заявка может быть вне загруженных порций (страница 2+, старая дата) - при
            // пагинации (#1158) полагаться на присутствие в списке нельзя. Если её нет в
            // накопленном - точечно догружаем по id (открытие детали и так работает по id).
            let app = this.applications.find(a => a.id === openId);
            if (!app) {
                try {
                    const fetched = await getApplicationById(openId);
                    // apiRequest на !success отдаёт {message}, без id - значит нет доступа/
                    // не найдена: оставляем ?open, откроется при следующей попытке.
                    if (fetched && fetched.id) app = fetched;
                } catch (e) {
                    console.error('Не удалось загрузить заявку из deep-link:', e);
                }
            }
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
    /* Высота проставляется JS (_applyHeight) = оставшийся вьюпорт; flex-column, чтобы
       таблица (flex:1) заполнила его и скроллилась внутри, а документ не рос. */
    display: flex;
    flex-direction: column;
}

.center__header {
    padding-bottom: 15px;
    display: flex;
    flex-direction: column;
    gap: 12px;
}

/* Первый ряд шапки: заголовок + переключатель/бейдж слева, действия (звук,
   на мобилке - иконка поиска) справа. Действия прижаты вправо margin-left:auto. */
.header-top {
    position: relative;
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
}

.header-top__actions {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
}

/* Мобильный второй ряд: переключатель Активные/Архив слева, Фильтр справа. */
.header-row2 {
    display: flex;
    align-items: center;
    gap: 10px;
}

/* Группа [Обновить][Фильтр] прижата вправо; отступ от дропдаума даёт auto на
   первом элементе группы, между ними остаётся gap ряда. */
.header-row2 :deep(.refresh-btn) {
    margin-left: auto;
}

/* Переключатель Активные/Архив: ширина фиксирована, чтобы пилюля не прыгала при
   смене значения (подписи разной длины). 132px - чуть больше самой длинной подписи
   («Активные» со стрелкой занимает 127px). Контейнер - флекс (.center__tabs),
   поэтому самому дропдауну нужен flex:1, иначе он ужимается по тексту. */
.center__tabs--mobile {
    min-width: 132px;
    max-width: 132px;
}

.center__tabs--mobile :deep(.base-dropdown) {
    flex: 1;
}

/* Мобилка: оверлей поиска - поверх первого ряда, растёт справа налево (clip-path в
   Transition, без reflow). right:44px оставляет справа иконку-тоггл открытой. */
.center__search-overlay {
    position: absolute;
    top: 0;
    bottom: 0;
    left: 0;
    right: 44px;
    z-index: 1;
    display: flex;
    align-items: center;
    background: #fff;
    /* Скруглить фон под скруглённое поле (15px) - иначе белые квадратные углы
       оверлея торчат за pill-полем рядом с круглой иконкой поиска (#1097 R3-3). */
    border-radius: var(--radius-md);
}

.center__search-overlay .field.search {
    width: 100%;
    margin: 0;
}

/* Крестик очистки внутри поля (появляется при вводе): сбрасывает и закрывает поиск. */
.center__search-clear {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    padding: 0;
    border: none;
    background: transparent;
    color: #888;
    font-size: 20px;
    line-height: 1;
    cursor: pointer;
    flex-shrink: 0;
}

.center__search-clear:hover {
    color: var(--color-primary);
}

/* Раскрытие влево - clip-path (композитится, не двигает ряд). */
.center-search-enter-active,
.center-search-leave-active {
    transition: clip-path 0.25s ease;
}

.center-search-enter-from,
.center-search-leave-to {
    clip-path: inset(0 0 0 100%);
}

.center-search-enter-to,
.center-search-leave-from {
    clip-path: inset(0 0 0 0);
}

/* Иконка-кнопка поиска (мобилка): тоггл оверлея поиска (раскрывается влево поверх ряда). */
.search-icon-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 40px;
    height: 40px;
    border: 1px solid var(--color-border);
    border-radius: 50%;
    background: #fff;
    cursor: pointer;
    flex-shrink: 0;
    transition: background 0.15s ease, border-color 0.15s ease;
}

.search-icon-btn:hover,
.search-icon-btn--active {
    background: var(--color-bg);
    border-color: var(--color-primary);
}

.search-icon-btn__img {
    width: 16px;
    height: 16px;
}

/* Мобилка: «Обновить» во втором ряду шапки Центра - только иконка, без подписи
   (эталон «Обзор и новости»), высотой с кнопку «Фильтр» рядом (34px).
   Ширина больше высоты и форма-пилюля (как у «Фильтра»): во время перезарядки
   кнопка показывает три точки шириной 27px, в кружке 34px они упирались в рамку.
   На десктопе кнопка живёт в шапке таблицы. */
@media (max-width: 767.98px) {
    .header-row2 :deep(.refresh-btn) {
        /* Ширина 45px - как у кнопки-иконки «Обновить» на «Обзор и новости»
           (NewsAndReview): фиксирована под самое широкое состояние - три точки
           перезарядки, иначе кнопка дёргается при переходе иконка<->точки. */
        width: 45px;
        height: 34px;
        padding: 0;
        justify-content: center;
        border-radius: var(--radius-pill);
        box-sizing: border-box;
    }

    .header-row2 :deep(.refresh-btn__text) {
        display: none;
    }
}

/* ── Десктоп: инлайн-фильтры Центра (как до волны 3) ── */
.center__filters {
    padding: 14px 16px;
    background: #fff;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
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
    flex-wrap: wrap;
}

.filter-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
    justify-content: flex-end;
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

.status-buttons {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
}

.status-btn,
.reset-sort-btn,
.reset-filters-btn {
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
.reset-sort-btn:hover:not(:disabled) {
    background: var(--color-bg);
    border-color: var(--color-primary);
    color: var(--color-primary);
}

.status-btn--active {
    background: var(--color-primary);
    color: white;
    border-color: var(--color-primary);
}

.status-btn--active:hover {
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

/* Дропдаун тегов (десктоп) */
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
    width: 9px;
    height: 9px;
    flex-shrink: 0;
    color: #555;
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

/* Кнопка «Фильтр» (мобилка) открывает модалку вторичных фильтров. Точка-индикатор -
   когда есть активные фильтры. Pill-стиль под высоту переключателя. */
.filter-btn {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: 7px;
    height: 36px;
    padding: 0 16px;
    border: 1px solid var(--color-border);
    background: #fff;
    border-radius: var(--radius-pill);
    cursor: pointer;
    font-size: 13px;
    font-weight: 500;
    color: var(--color-text);
    white-space: nowrap;
    transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.filter-btn:hover {
    background: var(--color-bg);
    border-color: var(--color-primary);
    color: var(--color-primary);
}

.filter-btn--active {
    border-color: var(--color-primary);
    color: var(--color-primary);
}

.filter-btn__icon {
    flex-shrink: 0;
}

.filter-btn__dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--color-primary);
    flex-shrink: 0;
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

.center__tabs {
    display: flex;
}

.table-toolbar {
    display: flex;
    justify-content: flex-end;
    padding: 8px 0;
    border-bottom: 1px solid var(--color-border);
    background: #fafafa;
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

.applications-table {
    min-width: 300px;
    background-color: #fff;
    border-radius: 30px;
    border: 1px solid var(--color-border);
    overflow: hidden;
    container-type: inline-size;
    margin-top: 20px;
    /* Таблица заполняет оставшуюся высоту .center (высота задана JS в _applyHeight)
       и скроллит внутри .table-body. Раньше высота считалась от var(--app-vh) с
       магической поправкой -340px: несовпадение с реальным чромом то роняло, то
       переполняло документ -> мигал скроллбар и прыгали шапки, а resize пересчитывал
       --app-vh -> осцилляция. flex:1 + min-height:0 убирают эту зависимость. На
       мобилке (@media 767) переопределяется на height:auto (скролл страницы). */
    flex: 1;
    min-height: 0;

    display: flex;
    flex-direction: column;
    transition: all 0.3s ease;
    position: relative;
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

/* Иконка бейджа "ждёт согласования" видима всегда: бейдж компактный (иконка + "N дн."),
   в отличие от roof/parking он не прячет текст и не участвует в --compact-свёртке. */
.rt-tag__icon--fixed {
    width: 13px;
    opacity: 1;
    margin-right: 3px;
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

/* Пульс-точку "статус обновился" (#1349) режет overflow:hidden базового
   .application-col - у колонки статуса ellipsis не нужен (бейдж фиксированный),
   поэтому разрешаем выход точки за границы (спецификой 0,2,0 бьём .application-col). */
.application-col.status-col {
    overflow: visible;
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

/* Sentinel бесшовной подгрузки (#1158) - невидимая полоса внизу списка,
   пересечение которой в table-body триггерит loadMore. min-height даёт
   IntersectionObserver что засечь даже без спиннера (loading=false). */
.scroll-sentinel {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 24px;
    padding: 10px 0;
}

/* In-flight состояние подгрузки при пустом списке (#1173): спиннер по центру,
   чтобы клик retry не мигал "Заявок нет" на время round-trip. */
.list-loading-state {
    flex-grow: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 40px 20px;
}

/* Устойчивость к ошибкам бэка (#1173): первичная загрузка упала - список пуст,
   вместо "Заявок нет" показываем причину + retry. */
.list-error-state {
    text-align: center;
    color: var(--color-danger);
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

/* Ошибка догрузки следующей порции (#1173) - компактный вариант рядом с sentinel,
   список уже частично загружен и остаётся на экране. */
.sentinel-error {
    display: flex;
    align-items: center;
    gap: 10px;
    color: var(--color-danger);
    font-size: 13px;
}

.table-footer {
    flex-shrink: 0;
    padding: 10px 20px;
    border-top: 1px solid var(--color-border);
    font-size: 13px;
    color: #8a8a8a;
}

.applications-list {
    flex-grow: 1;
    /* relative - чтобы leave-строка (position:absolute) держалась в пределах списка,
       пока соседи плавно смыкаются (move). */
    position: relative;
}

/* Скрытый hover-тултип отправителя последней строки (::after, position:absolute,
   bottom:-33px) торчал ниже контента и раздувал scrollHeight table-body на ~22px -
   между последней заявкой и футером висела пустая полоса. Клипаем список по его же
   границе (совпадает с окном скролла table-body, тултипы видимых строк не страдают).
   Одностраничный режим исключаем - там table-body overflow:visible показывает
   тултип единственной строки, которую скрывать нечем. */
.table-body:not(.table-body--single-row) .applications-list {
    overflow: hidden;
}

/* Разделитель периода (#1097 r2): серая линия сверху, под ней подпись периода (без дат).
   Десктоп и мобилка. Первый разделитель без верхней линии (граница шапки уже есть). */
.applications-day-separator {
    padding: 8px 16px;
    border-top: 1px solid #e2e2e6;
    border-bottom: 1px solid #e2e2e6;
    /* Серая полоса-разделитель периодов на десктопе (полоса на всю ширину .applications-list).
       Лейбл - отдельный span, флекс выравнивает его по вертикали (align-items); по
       горизонтали не центрируем - подпись идёт слева, как в списке.
       На мобилке (карточки) возвращаем transparent - см. @media ниже. */
    background: #FAFAFA;
    display: flex;
    align-items: center;
}
.applications-list .applications-day-separator:first-child {
    border-top: none;
}
.applications-day-label {
    font-size: 12px;
    font-weight: 600;
    color: #9a9aae;
}
/* На мобилке список - карточки на белом, серая полоса разделителя лишняя;
   фон/бордеры - десктопные, тут сбрасываем. */
@media (max-width: 767.98px) {
    .applications-day-separator {
        background: transparent;
        border-top: none;
        border-bottom: none;
        padding: 15px 16px 6px;
    }
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
    /* Отключаем каскад первой загрузки для последующих обновлений. opacity/transform
       НЕ фиксируем - ими управляет TransitionGroup (app-row) при вставке новых строк. */
    animation: none;
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

/* Точечная анимация списка Центра при real-time добавлении (#840): новая заявка
   плавно проявляется и въезжает на свою позицию (с учётом сортировки), соседи
   едут на новое место (FLIP move). Только transform/opacity. Пачка новых за раз
   отрабатывается штатно - каждая enter + все сдвиги move одновременно. */
.app-row-enter-active {
    transition: opacity 0.3s ease-out, transform 0.3s ease-out;
}
.app-row-enter-from {
    opacity: 0;
    transform: translateY(-20px);
}
.app-row-move {
    transition: transform 0.3s ease;
}
.app-row-leave-active {
    transition: opacity 0.25s ease, transform 0.25s ease;
    /* Выводим из потока, чтобы соседи плавно сомкнулись при выпадении заявки. */
    position: absolute;
    width: 100%;
}
.app-row-leave-to {
    opacity: 0;
    transform: translateY(-10px);
}

.application-item:hover:not(.download-btn:hover) {
    background-color: #f0f0f0;
}

.application-item.unread {
    background-color: #fff5e0;
}

/* Заявка с обновлённым статусом (#1349): мягкий фиолетовый фон + пульс-точка на бейдже.
   Взаимоисключимо с .unread (флаг показываем только у прочитанных), поэтому фоны не
   конфликтуют. Ставим после hover-правил, чтобы подсветка держалась и при наведении. */
.application-item.status-updated {
    background-color: #ede9fe;
    /* Левая полоса-акцент (inset - без reflow): заметный сигнал "обновление" даже там,
       где мягкого фона мало (мобильная карточка, где точка статуса скрыта). */
    box-shadow: inset 3px 0 0 0 #7c3aed;
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
    background-color: #7c3aed;
    box-shadow: 0 0 0 2px #fff;
}

.status-update-dot::after {
    content: "";
    position: absolute;
    inset: 0;
    border-radius: 50%;
    background-color: #7c3aed;
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
    padding: 6px 16px;
    align-items: center;
    gap: 14px;
    border-bottom: 1px solid #f5f5f5;
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

/* Сообщение показываем ТОЛЬКО в компактной карточке на мобилке (W3.7); в десктоп-
   таблице отдельной колонки нет - иначе сломался бы ряд из 8 колонок. */
.message-col {
    display: none;
}

@media (max-width: 767.98px) {
    .center {
        /* Боковой отступ страницы вынесен в переменную: full-bleed блоки (шапка,
           список) гасят его отрицательным margin через ту же переменную. Иначе
           значения разъезжаются на узких брейкпоинтах и блок вылезает за вьюпорт. */
        --center-pad: 12px;
        /* Верхний отступ убран - шапка Центра примыкает к app bar без «странного»
           просвета сверху (#1097 p2). Боковой/нижний padding сохранён. */
        padding: 0 var(--center-pad) var(--center-pad);
        /* На мобилке - обычный поток (высота сброшена в _applyHeight), список
           скроллит страница под sticky-шапкой; десктопный flex-fill не нужен. */
        display: block;
        height: auto;
    }

    /* Шапка Центра закреплена под app bar (TheHeader = --mobile-header-height, sticky top:0),
       список заявок скроллит страницу под ней. Full-bleed белый фон (отрицательный margin
       гасит боковой padding .center) - edge-to-edge карточки уходят под шапку без просвета.
       Нижняя граница - разделитель между шапкой и списком (гасит «белую пустоту» перед
       первой карточкой), держится закреплённой при скролле. */
    .center__header {
        position: sticky;
        /* Липнет ровно под fixed-шапкой (= --mobile-header-height). sticky отслеживает
           скролл документа нативно на композиторе - без JS-переменной и reflow-дёрганья. */
        top: var(--mobile-header-height);
        z-index: 20;
        gap: 8px;
        background: #FAFAFA;
        margin: 0 calc(-1 * var(--center-pad));
        padding: 8px var(--center-pad) 12px;
        border-bottom: 1px solid var(--color-border);
    }

    /* Дропдаун Активные/Архив и кнопка Фильтр - одной высоты (34px). */
    .header-row2 .filter-btn {
        height: 34px;
    }

    .center__tabs--mobile :deep(.base-dropdown__button) {
        min-height: 34px;
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

    /*
     * Таблица -> карточки-письма (rt-table/rt-head-row/rt-row из responsive-tables.css,
     * #1097 S5). rt-head-row прячет .header-row целиком - вместе с ней схлопываем и
     * пустую .table-header (иначе остаётся полоса 45px с бордером без содержимого);
     * оверлей обновления подстраивает top под освободившееся место.
     */
    .table-header {
        display: none;
    }

    .refresh-overlay {
        top: 0;
    }

    /* Список не заперт в невысокий скролл-бокс - карточки скроллит страница целиком,
     * как раньше был устроен горизонтальный скролл (без вложенного overflow). */
    .applications-table {
        max-height: none;
        height: auto;
        overflow: visible;
        /* Карточки на всю ширину экрана: гасим боковой padding .center
           отрицательным margin, убираем рамку/скругление таблицы - список идёт от
           края до края (боковой отступ экрана у заявок = 0). Верхний отступ убран -
           список примыкает к разделителю шапки, между ними только padding table-body. */
        margin: 0 calc(-1 * var(--center-pad));
        border: none;
        border-radius: 0;
    }

    .table-body {
        overflow-y: visible;
        flex-grow: unset;
        /* Только вертикальный отступ - по бокам карточки прилегают к краю экрана. */
        padding: 8px 0;
    }

    .applications-list {
        overflow: visible;
    }

    /* Карточки вплотную - разделены нижней границей (см. ниже), без зазора-«плитки». */
    .application-item + .application-item {
        margin-top: 0;
    }

    /* Непрочитанность (правка среза 5) - дата синим bold + красная точка слева от неё,
       без жёлтой заливки/скруглённых углов и БЕЗ подсветки номера: номер остаётся
       обычным серым как у прочитанных. Специфичность 0,4,0 бьёт базовый цвет даты
       (0,2,0 ниже), не полагаясь на порядок источника. */
    .application-item.unread .application-col.date-col {
        color: var(--color-primary);
        font-weight: 600;
        font-size: 13px;
    }
    /* Красная точка перед датой в углу карточки - маркер непрочитанной заявки. Инлайн
       ::before в right-aligned nowrap date-col садится слева от текста даты. Полный
       префикс .applications-table + display:!important перебивают скрывающее подписи
       `.rt-row > .application-col::before{display:none!important}` (равная специфичность
       0,4,1, но то правило ниже по источнику - без префикса выиграло бы тай-брейк). */
    .applications-table .application-item.unread .application-col.date-col::before {
        content: '';
        display: inline-block !important;
        width: 5px;
        height: 5px;
        margin-right: 5px;
        border-radius: 50%;
        background: var(--color-danger);
        vertical-align: middle;
    }

    /* Непрочитанная заявка на мобилке - бледно-жёлтый фон карточки как на десктопе
       (доп. просьба). Полный префикс + !important бьют `.rt-table .rt-row{background:
       #fff !important}` (responsive-tables.css). Синяя дата + красная точка остаются. */
    .applications-table .application-item.unread .application-row.rt-row {
        background-color: #fff5e0 !important;
    }

    /* Компактная карточка-письмо БЕЗ подписей (W3.7): бейдж согласования + дата в углу
       сверху, затем номер (мелко) / организация (жирным) / отправитель / сообщение, теги
       внизу. Статус скрыт, боковой padding у полей убран - падинг у карточки. */
    .application-row.rt-row {
        /* position:relative - якорь для даты в правом верхнем углу.
           padding карточки задаёт глобальный responsive-tables.css (10px 14px !important). */
        position: relative;
        gap: 3px;
    }

    /* Edge-to-edge список: гасим боковые/верхнюю границу и скругление (иначе скруглённый
       угол карточки торчит у самого края full-bleed экрана - те самые «уголки»), карточки
       разделяем только нижней границей как строки. Полный префикс + !important перебивают
       глобальное `.rt-table .rt-row{border;border-radius}!important` (responsive-tables.css). */
    .applications-table .application-row.rt-row {
        border-top: none !important;
        border-left: none !important;
        border-right: none !important;
        border-radius: 0 !important;
        /* Разделитель МЕЖДУ заявками бледнее (#f0f0f0), чтобы отличался от более
           тёмной линии разделителя периода (#e2e2e6) (#1097 r2). */
        border-bottom-color: #f0f0f0 !important;
    }

    /* Прячем подписи полей (data-label ::before из responsive-tables.css). */
    .applications-table .application-row.rt-row > .application-col::before {
        display: none !important;
    }

    /* Ячейки в столбик, без бордюр-разделителей и бокового padding, авто-высота.
       text-align:left перебивает text-align:right из responsive-tables.css (там
       значение выравнивалось вправо от подписи - без подписей нам нужно влево). */
    .applications-table .application-row.rt-row > .application-col {
        flex: none;
        display: block;
        width: 100%;
        max-width: 100%;
        padding: 0;
        border: none;
        overflow: visible;
        white-space: normal;
        text-align: left;
    }

    /* Длинные значения (организация/отправитель/сообщение) - одна строка с обрезкой "..".
       Полный префикс (0,5,0) выше общего блочного `...> .application-col{white-space:normal}`
       (0,4,0) - иначе normal побеждает и сообщение переносится на несколько строк. */
    .applications-table .application-row.rt-row > .application-col.organization-col,
    .applications-table .application-row.rt-row > .application-col.sender-col,
    .applications-table .application-row.rt-row > .application-col.message-col {
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
    }

    /* number-col: обычный ряд (сброс десктопного flex-column номер-над-ЧС). */
    .application-col.number-col {
        flex-direction: row;
    }

    /* Статус заявки скрыт в компактной карточке (W3.11). Полный префикс - иначе общее
       `...> .application-col{display:block}` перебивает по специфичности. */
    .applications-table .application-row.rt-row > .application-col.status-col {
        display: none;
    }

    /* Дата+время прихода - в правом верхнем углу карточки (W3.11). Полный префикс (0,5,0)
       перебивает `...> .application-col{display:block; width:100%}`. Цвет НЕ задаём здесь -
       иначе он побьёт unread-цвет (базовый серый и синий unread идут ниже). */
    .applications-table .application-row.rt-row > .application-col.date-col {
        position: absolute;
        top: 10px;
        right: 14px;
        width: auto;
        max-width: 55%;
        /* height:auto сбрасывает базовое .application-col{height:100%} - иначе absolute-блок
           даты растягивается на всю высоту карточки невидимым оверлеем и перехватывает hover
           тегов/организации под ним (расширенный max-width усилил бы это). */
        height: auto;
        padding: 0;
        font-size: 14px;
        white-space: nowrap;
        text-align: right;
    }

    /* Базовый (прочитанный) цвет даты - приглушённый серый; unread перекрывает синим выше. */
    .application-col.date-col {
        color: #9a9aae;
    }

    /* Резерв справа у бейджа согласования, чтобы дата в углу не наезжала на него.
       Полный префикс (0,5,0) - иначе общий `...> .application-col{padding:0}` перебивает. */
    .applications-table .application-row.rt-row > .application-col.confirmation-col {
        padding-right: 118px;
    }

    /* Пустой отправитель/теги - скрыть строку. */
    .applications-table .application-row.rt-row > .tags-col:empty,
    .applications-table .application-row.rt-row > .sender-col:empty {
        display: none;
    }

    /* Порядок карточки: согласование / номер / организация / отправитель / сообщение / теги. */
    .application-col.confirmation-col { order: 1; margin-bottom: 4px; }
    .application-col.number-col { order: 2; }
    .application-col.organization-col { order: 3; }
    .application-col.sender-col { order: 4; }
    .application-col.message-col { order: 5; display: block; }
    .application-col.tags-col { order: 6; margin-top: 6px; }
    /* Скачивание на мобилке перенесено в открытую заявку (W3.8) - в строке прячем.
       Полный префикс - иначе общее `...> .application-col{display:block}` перебивает по специфичности. */
    .applications-table .application-row.rt-row > .application-col.actions-col { display: none; }

    /* Типографика строк карточки. */
    .application-col.number-col .application-number {
        font-size: 12px;
        color: #9a9aae;
        font-weight: 500;
    }
    .application-col.organization-col {
        font-size: 15px;
        font-weight: 700;
        color: #1a1a2e;
    }
    .application-col.sender-col {
        font-size: 13px;
        color: #555;
    }
    .application-col.message-col {
        font-size: 13px;
        color: #7a7a8c;
    }

    /* Теги в компактной карточке НЕ сворачиваем в иконки (W3.11) - показываем полным
       текстом с переносом на новую строку. Нейтрализуем свёртку tagsAreCompact: те же
       селекторы (0,3,0) идут ниже в источнике -> перебивают базовую свёртку. */
    .application-tags {
        flex-wrap: wrap;
    }
    .application-tags--compact .rt-tag--roof .rt-tag__text,
    .application-tags--compact .rt-tag--parking .rt-tag__text,
    .application-tags--compact .rt-tag--important .rt-tag__text,
    .application-tags--compact .rt-tag--questions .rt-tag__text {
        max-width: 150px;
        opacity: 1;
    }
    .application-tags--compact .rt-tag--roof .rt-tag__icon,
    .application-tags--compact .rt-tag--parking .rt-tag__icon,
    .application-tags--compact .rt-tag--important .rt-tag__icon,
    .application-tags--compact .rt-tag--questions .rt-tag__icon {
        width: 0;
        opacity: 0;
    }
    .application-tags--compact .rt-tag--roof.badge--sm,
    .application-tags--compact .rt-tag--parking.badge--sm,
    .application-tags--compact .rt-tag--important.badge--sm,
    .application-tags--compact .rt-tag--questions.badge--sm {
        padding: 3px 8px;
    }

    /* Тач-таргеты >= 44px. Кнопка "Скачать" в карточке идёт собственной строкой (без
     * data-label) - можно смело увеличить саму кнопку. Копирование номера остаётся
     * компактной надписью - расширяем зону клика невидимым псевдоэлементом, не раздувая
     * визуально строку "Номер заявки". */
    .download-btn {
        min-height: 44px;
        height: auto;
        padding: 10px 16px;
    }

    .application-number--copyable::before {
        content: '';
        position: absolute;
        inset: -12px -8px;
    }
}

@media (max-width: 480px) {
    .center {
        /* Верхний отступ убран (#1097 p2), боковой/нижний 10px сохранён. Full-bleed
           шапка и список гасят его тем же --center-pad, поэтому переопределяем только
           переменную: разъезд «margin -12 против padding 10» (overshoot 2px ->
           горизонтальный скролл документа) структурно невозможен. */
        --center-pad: 10px;
        padding: 0 var(--center-pad) var(--center-pad);
    }

    .center__tabs {
        gap: 6px;
    }
}

/* Кнопка звука в шапке. Выравнивание вправо задаёт контейнер .header-top__actions
   (margin-left:auto), поэтому здесь margin-left не нужен - иначе он разрывал бы
   группу [звук][поиск] внутри actions. */
.sound-btn-wrap {
    position: relative;
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