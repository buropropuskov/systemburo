<template>
  <section
    ref="root"
    class="carsview"
    data-testid="cars-page"
  >
    <header class="carsview__header">
      <h2 class="carsview__title">
        Список <span class="blue">автомобилей</span>
      </h2>
      <p class="carsview__subtitle">
        Вкладка для просмотра автомобилей, которые вы или ваша организация/компания когда-либо привязывали к заявкам.
      </p>
    </header>

    <!-- Десктоп: поиск и табы области над карточкой. На мобилке этот блок физически
         переезжает под шапку списка (.carsview__toolbar) - через v-if, а не скрытой
         копией: иначе data-testid и якорь тура задвоились бы в DOM. -->
    <div
      v-if="!isNarrow"
      class="carsview__filters"
    >
      <div
        class="filters-container"
        data-testid="ob-cars-filters"
      >
        <SearchComponent
          v-model="searchQuery"
          :title="'Поиск машин...'"
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
            title="Автомобили, которых привязывали пользователи вашей организации"
            @click="switchFilter('organization')"
          >
            Машины организации
          </button>
          <button
            v-if="ownershipInfo.has_company && canSeeCompany"
            class="filter-tab"
            data-testid="filter-tab-company"
            :class="{ 'filter-tab--active': currentFilter === 'company' }"
            title="Автомобили, которых привязывали пользователи вашей компании"
            @click="switchFilter('company')"
          >
            Машины компании
          </button>
          <button
            class="filter-tab"
            data-testid="filter-tab-user"
            :class="{ 'filter-tab--active': currentFilter === 'user' }"
            title="Только те автомобили, которых привязывали лично вы"
            @click="switchFilter('user')"
          >
            Мои машины
          </button>
          <button
            v-if="canSeeAllSystem"
            class="filter-tab"
            data-testid="filter-tab-all-system"
            :class="{ 'filter-tab--active': currentFilter === 'all_system' }"
            title="Все автомобили, когда-либо зарегистрированные в системе"
            @click="switchFilter('all_system')"
          >
            Все машины системы
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
            data-testid="cars-scope-organization"
            :class="{ 'filter-tab--active': currentFilter === 'organization' }"
            @click="switchScopeFromSheet('organization')"
          >
            Машины организации
          </button>
          <button
            v-if="ownershipInfo.has_company && canSeeCompany"
            class="filter-tab"
            data-testid="cars-scope-company"
            :class="{ 'filter-tab--active': currentFilter === 'company' }"
            @click="switchScopeFromSheet('company')"
          >
            Машины компании
          </button>
          <button
            class="filter-tab"
            data-testid="cars-scope-user"
            :class="{ 'filter-tab--active': currentFilter === 'user' }"
            @click="switchScopeFromSheet('user')"
          >
            Мои машины
          </button>
          <button
            v-if="canSeeAllSystem"
            class="filter-tab"
            data-testid="cars-scope-all-system"
            :class="{ 'filter-tab--active': currentFilter === 'all_system' }"
            @click="switchScopeFromSheet('all_system')"
          >
            Все машины системы
          </button>
        </div>
      </div>
    </FilterSheet>

    <div class="carsview__container">
      <!-- Таблица автомобилей -->
      <div
        class="cars-card"
        data-testid="ob-cars-table"
      >
        <div
          class="card-header"
          data-testid="ob-cars-table-head"
        >
          <div class="card-header__title">
            <h3 class="card-title">
              <span
                v-if="currentFilter === 'organization'"
                class="highlight-text"
              >Машины <span class="blue">организации</span></span>
              <span
                v-else-if="currentFilter === 'company'"
                class="highlight-text"
              >Машины <span class="blue">компании</span></span>
              <span
                v-else-if="currentFilter === 'all_system'"
                class="highlight-text"
              >Все <span class="blue">машины системы</span></span>
              <span
                v-else
                class="highlight-text"
              >Мои <span class="blue">автомобили</span></span>
            </h3>
            <!-- Счётчик записей рядом с заголовком - только на мобилке: на десктопе
                 то же число уже стоит в футере таблицы («Показано X из Y»). -->
            <span
              v-if="isNarrow"
              class="card-header__count"
              data-testid="cars-count-badge"
            >{{ carsTotal }}</span>
          </div>
          <div class="card-header__settings">
            <!-- На мобилке «Добавить» переезжает в панель у нижнего края экрана
                 (.carsview__action-bar), в шапке остаётся одно действие. -->
            <button
              v-if="!isNarrow && currentFilter !== 'all_system' && canWriteCars"
              class="add-button"
              data-testid="cars-view-add-button"
              @click="showAddCarModal"
            >
              Добавить
            </button>
            <!-- Журнал реестра открыт администратору, см. EmployeeView. -->
            <button
              v-if="canManageAllEntities"
              class="log-button"
              data-testid="cars-registry-log"
              @click="showRegistryLog = true"
            >
              Журнал
            </button>
            <RefreshButton
              :loading="loading"
              @refresh="fetchCars"
            />
          </div>
        </div>

        <!-- Мобилка: поиск и «Фильтр» отдельной полосой 36px под шапкой экрана. -->
        <div
          v-if="isNarrow"
          class="carsview__toolbar"
          data-testid="ob-cars-filters"
        >
          <SearchComponent
            v-model="searchQuery"
            :title="'Поиск машин...'"
          />
          <FilterButton
            v-if="ownershipInfo"
            :active="scopeFilterActive"
            data-testid="cars-filter-btn"
            @click="showScopeSheet = true"
          />
        </div>

        <div class="card-content rt-table">
          <!-- Заголовок таблицы всегда отображается (на мобилке скрыт rt-head-row, строки -> карточки) -->
          <div class="cars-header rt-head-row">
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
                class="header-col car-number-col"
                @click="sortBy('number')"
              >
                <p :class="{ 'active-sort': sortField === 'number' }">
                  Номер
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'number',
                    'desc': sortField === 'number' && sortDirection === 'desc'
                  }"
                />
              </div>
              <div
                class="header-col brand-col"
                @click="sortBy('mark')"
              >
                <p :class="{ 'active-sort': sortField === 'mark' }">
                  Марка
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'mark',
                    'desc': sortField === 'mark' && sortDirection === 'desc'
                  }"
                />
              </div>
              <div
                class="header-col format-col"
                @click="sortBy('format_name')"
              >
                <p :class="{ 'active-sort': sortField === 'format_name' }">
                  Формат номера
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'format_name',
                    'desc': sortField === 'format_name' && sortDirection === 'desc'
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
          <div class="cars-container">
            <div class="cars-table-area">
              <div
                v-if="loading"
                class="loading-message"
              >
                <LoaderSpinner label="Загрузка машин…" />
              </div>
              <div
                v-else-if="sortedCars.length > 0"
                ref="carsBody"
                class="cars-body"
              >
                <div
                  v-for="(car) in sortedCars"
                  :key="car.id"
                  class="car-item"
                >
                  <div
                    class="car-row rt-row"
                    title="Открыть детали машины"
                    @click="openCarDetails(car)"
                  >
                    <div
                      class="car-col number-col"
                      data-label="№"
                    >
                      {{ car.id }}
                    </div>
                    <div
                      class="car-col car-number-col"
                      data-label="Номер"
                    >
                      {{ car.number }}
                    </div>
                    <div
                      class="car-col brand-col"
                      data-label="Марка"
                    >
                      {{ car.mark }}
                    </div>
                    <div
                      class="car-col format-col"
                      data-label="Формат номера"
                    >
                      {{ car.format_name || 'Не указан' }}
                    </div>
                    <div
                      class="car-col status-col"
                      data-label="Статус"
                    >
                      <StatusBadge
                        v-if="isCarBlacklisted(car)"
                        status="Чёрный список"
                      />
                      <StatusBadge
                        v-else
                        :status="car.status ? 'Активна' : 'Неактивна'"
                      />
                    </div>
                    <div
                      v-if="currentFilter === 'organization' || currentFilter === 'all_system'"
                      class="car-col org-col"
                      data-label="Организация"
                      :title="car.organization_name || ''"
                    >
                      {{ car.organization_name || '—' }}
                    </div>
                    <div
                      v-if="currentFilter === 'company' || currentFilter === 'all_system'"
                      class="car-col company-col"
                      data-label="Компания"
                      :title="car.company_name || ''"
                    >
                      {{ car.company_name || '—' }}
                    </div>
                    <div class="car-col actions-col">
                      <button
                        v-if="showEditCar(car)"
                        class="edit-btn"
                        title="Изменить"
                        @click.stop="editCar(car)"
                      >
                        <AppIcon
                          name="edit"
                          class="edit-icon"
                        />
                        <span class="action-btn__label">Изменить</span>
                      </button>
                      <button
                        v-if="showDeleteCar(car)"
                        class="delete-btn"
                        title="Удалить"
                        @click.stop="openDeleteCarConfirmation(car)"
                      >
                        <AppIcon
                          name="trashcan"
                          class="delete-icon"
                        />
                        <span class="action-btn__label">Удалить</span>
                      </button>
                      <span
                        v-if="!showEditCar(car) && !showDeleteCar(car)"
                        class="read-only-text"
                        :title="canEditTooltip(car)"
                      >
                        Только просмотр
                      </span>
                    </div>
                  </div>
                </div>

                <!-- Бесшовная подгрузка (#1158): sentinel внизу СКРОЛЛИРУЕМОГО cars-body -
                     IntersectionObserver триггерит loadMore без кнопки "Показать ещё". -->
                <div
                  v-if="hasMoreCars"
                  :ref="setCarsSentinelRef"
                  class="scroll-sentinel"
                  data-testid="cars-scroll-sentinel"
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
                    data-testid="cars-scroll-sentinel-error"
                  >
                    <span>Не удалось загрузить ещё</span>
                    <button
                      type="button"
                      class="lk-button lk-button--secondary lk-button--sm"
                      :disabled="listLoading"
                      @click="retryCars"
                    >
                      {{ listLoading ? 'Повтор…' : 'Повторить' }}
                    </button>
                  </div>
                </div>
              </div>
              <!-- In-flight retry при пустом списке (#1173): пока listLoading -
                   спиннер, не проваливаемся в error/"Автомобилей нет". listLoading
                   выставляет composable из retry() (this.loading он не трогает). -->
              <div
                v-else-if="listLoading"
                class="loading-message"
                data-testid="cars-list-loading"
              >
                <LoaderSpinner label="Загрузка…" />
              </div>
              <!-- Первичная загрузка упала (#1173): список пуст из-за ошибки бэка, а
                   не потому что машин реально нет. -->
              <div
                v-else-if="listError"
                class="list-error-state"
                data-testid="cars-list-error"
              >
                <p>Не удалось загрузить автомобили. Проверьте соединение.</p>
                <button
                  type="button"
                  class="lk-button lk-button--secondary"
                  :disabled="listLoading"
                  @click="retryCars"
                >
                  {{ listLoading ? 'Повтор…' : 'Повторить' }}
                </button>
              </div>
              <p
                v-else
                class="no-data-message"
              >
                {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Автомобилей нет' }}
              </p>
            </div>
            <div
              v-if="!loading && sortedCars.length"
              class="table-footer"
              data-testid="cars-table-footer"
            >
              {{ footerText }}
            </div>
          </div>
        </div>
      </div>
            
      <div class="carsview__right-side">
        <div class="carsview__help">
          <template v-if="currentFilter === 'organization'">
            <p class="help__text">
              Здесь находятся автомобили, привязанные к вашей <strong class="blue">организации</strong>. Вы можете использовать эти автомобили при подаче автозаявок.
            </p>
            <p class="help__text">
              Новые номера машин попадают в этот список <strong class="blue">автоматически</strong>, при подаче заявки.
            </p>
          </template>
          <template v-else-if="currentFilter === 'company'">
            <p class="help__text">
              Здесь находятся автомобили, привязанные к вашей <strong class="blue">компании</strong>. Вы можете использовать эти автомобили при подаче автозаявок.
            </p>
            <p class="help__text">
              Новые номера машин попадают в этот список <strong class="blue">автоматически</strong>, при подаче заявки.
            </p>
          </template>
          <template v-else-if="currentFilter === 'user'">
            <p class="help__text">
              Здесь находятся <strong class="blue">ваши автомобили</strong>, добавленные лично. Вы можете использовать их при подаче автозаявок.
            </p>
            <p class="help__text">
              Новые номера машин попадают в этот список <strong class="blue">автоматически</strong>, при подаче заявки.
            </p>
          </template>
          <template v-else-if="currentFilter === 'all_system'">
            <p
              v-if="canManageAllEntities"
              class="help__text"
            >
              Здесь отображаются <strong class="blue">все автомобили</strong>, которые есть в системе. Как администратор вы можете изменить или удалить любую машину, к какой бы организации она ни была привязана. Добавлять машины нужно на вкладках выше - там видно, за кем закрепится запись.
            </p>
            <p
              v-else
              class="help__text"
            >
              Здесь отображаются <strong class="blue">все автомобили</strong>, которые есть в системе. В этой вкладке доступен только просмотр, добавление, редактирование и удаление машин недоступно.
            </p>
          </template>
        </div>
      </div>
    </div>

    <!-- Мобилка: главное действие экрана - широкая кнопка в панели, прижатой к низу
         экрана, у пальца. data-bottom-action-bar - контракт для ScrollTopButton:
         кнопка «наверх» поднимается над панелью, чтобы не лечь на «Добавить». -->
    <div
      v-if="isNarrow && currentFilter !== 'all_system' && canWriteCars"
      class="carsview__action-bar"
      data-bottom-action-bar
    >
      <button
        class="add-button add-button--wide"
        data-testid="cars-view-add-button"
        @click="showAddCarModal"
      >
        Добавить машину
      </button>
    </div>

    <!-- Модальное окно добавления машины - контракт окна из BaseModal (крестик,
         overlay, Escape, свайп-вниз/bottom-sheet на мобилке), а не своя разметка. -->
    <BaseModal
      :show="showModal && (currentFilter !== 'all_system' || !!editingCar)"
      :title="editingCar ? 'Редактирование' : 'Добавление Т/С'"
      width="500px"
      radius="30px"
      content-testid="cars-view-modal"
      @close="closeModal"
    >
      <div class="data__completion">
        <div class="completion__format">
          <div class="format__header">
            <label class="format__label">Формат номеров</label>
            <button
              class="add-button"
              :disabled="!canSaveCar"
              @click="saveCar"
            >
              {{ editingCar ? 'Сохранить' : 'Добавить' }}
            </button>
          </div>
          <div class="format__dropdown">
            <button
              class="dropdown__button"
              data-testid="cars-view-format-dropdown"
              @click="toggleFormatDropdown"
            >
              <div class="button__content">
                <span class="button__text">{{ selectedFormatText }}</span>
                <svg
                  class="button__arrow"
                  :class="{ 'button__arrow--open': isFormatDropdownOpen }"
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
              </div>
            </button>
            <transition name="dropdown">
              <div
                v-if="isFormatDropdownOpen"
                class="dropdown__menu"
              >
                <div 
                  v-for="format in availableFormats" 
                  :key="format.format.id"
                  class="dropdown__item" 
                  @click="selectFormat(format)"
                >
                  <span class="item__text">{{ format.format.name }}</span>
                </div>
              </div>
            </transition>
          </div>
        </div>
                        
        <div class="completion__fields">
          <div class="completion__number">
            <div class="completion__number-header">
              <label class="input__label">Номер Т/C <span class="required">*</span></label>
            </div>
                                
            <!-- Динамический формат из базы данных -->
            <div
              v-if="selectedFormat"
              class="number__field"
            >
              <input 
                v-for="(cell, index) in selectedFormat.cells" 
                :key="index"
                v-model="numberParts[index]" 
                class="number__input"
                :placeholder="getPlaceholder(cell)"
                :maxlength="cell.max_length"
                :style="{ width: getInputWidth(cell) }"
                @input="validatePart(index, $event, cell)"
                @blur="formatPart(index, cell)"
              >
            </div>
            <div
              v-else
              class="no-format-message"
            >
              Выберите формат номера
            </div>
          </div>
                            
          <div class="completion__mark">
            <div class="completion__mark-header">
              <label class="input__label">Марка Т/С <span class="required">*</span></label>
            </div>
            <div class="mark__field">
              <div class="mark__dropdown">
                <button
                  class="mark__dropdown-button"
                  @click="toggleMarkDropdown"
                >
                  <div class="mark__button-content">
                    <span class="mark__button-text">{{ selectedMark || 'Выберите марку' }}</span>
                    <svg
                      class="mark__button-arrow"
                      :class="{ 'mark__button-arrow--open': isMarkDropdownOpen }"
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
                  </div>
                </button>
                <transition name="dropdown">
                  <div
                    v-if="isMarkDropdownOpen"
                    class="mark__dropdown-menu"
                  >
                    <div class="mark__search">
                      <input 
                        v-model="markSearch" 
                        class="mark__search-input"
                        placeholder="Поиск марки..."
                        @input="filterMarks"
                      >
                    </div>
                    <div class="mark__dropdown-list">
                      <div
                        v-for="mark in filteredMarks"
                        :key="mark.id"
                        class="mark__dropdown-item"
                        @click="selectMark(mark)"
                      >
                        <span class="mark__item-text">{{ mark.name }}</span>
                      </div>
                      <div
                        v-if="!filteredMarks.length"
                        class="mark__dropdown-empty"
                      >
                        Марки не найдены
                      </div>
                    </div>
                  </div>
                </transition>
              </div>
            </div>
          </div>
        </div>

        <!-- Привязка чужой записи: администратор её не переносит на себя, поэтому
             вместо переключателей «привязать к моей организации» показываем, за кем
             запись закреплена. -->
        <div
          v-if="editingForeignRecord"
          class="completion__binding"
        >
          <label class="input__label">Привязка</label>
          <div class="binding-info">
            <p
              class="binding-note"
              data-testid="cars-foreign-binding-note"
            >
              Запись закреплена за
              <strong v-if="editingCar.user_name">пользователем «{{ editingCar.user_name }}»</strong>
              <strong v-else>другим пользователем</strong>
              <template v-if="editingCar.organization_name">
                , организация «{{ editingCar.organization_name }}»
              </template>
              <template v-if="editingCar.company_name">
                , компания «{{ editingCar.company_name }}»
              </template>.
              Правка данных привязку не меняет.
            </p>
          </div>
        </div>

        <!-- Привязка -->
        <div
          v-if="currentFilter !== 'all_system' && !editingForeignRecord"
          class="completion__binding"
        >
          <label class="input__label">Привязка</label>
          <div class="binding-info">
            <p class="binding-note">
              <strong>Добавляемый автомобиль автоматически привязывается к аккаунту пользователя.</strong>
              Автомобиль можно привязать к организации или компании, для использования <strong>другими сотрудниками</strong>:
            </p>
          </div>
          <div class="binding-options">
            <label
              v-if="ownershipInfo && ownershipInfo.has_organization"
              class="binding-option"
            >
              <input
                v-model="bindToOrganization"
                type="checkbox"
              >
              <span>Привязать к организации<template v-if="ownershipInfo.organization_name"> «{{ ownershipInfo.organization_name }}»</template></span>
            </label>
            <label
              v-if="ownershipInfo && ownershipInfo.has_company"
              class="binding-option"
            >
              <input
                v-model="bindToCompany"
                type="checkbox"
              >
              <span>Привязать к компании<template v-if="ownershipInfo.company_name"> «{{ ownershipInfo.company_name }}»</template></span>
            </label>
            <div class="user-binding">
              <span class="user-binding-text"><strong class="red">Внимание!</strong> При привязке автомобиля к организации или компании, он будет доступен для отображения и использования для всех сотрудников, привязанных к организации/компании. </span>
            </div>
          </div>
        </div>
      </div>
    </BaseModal>
    <RegistryLogModal
      :show="showRegistryLog"
      entity="cars"
      @close="showRegistryLog = false"
    />

    <ConfirmationModal
      :show="showDeleteCarModal"
      title="Подтверждение удаления"
      :message="`Вы уверены, что хотите удалить автомобиль ${carToDelete?.number || ''}?`"
      confirm-text="Удалить"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#ff4444', borderColor: '#ff4444' }"
      @confirm="confirmDeleteCar"
      @cancel="cancelDeleteCar"
    />

    <VehicleDetailsModal
      :show="showDetailsViewModal"
      :vehicle="detailsCar"
      :all-unloading-places="allUnloadingPlaces"
      :license-plate-formats="[]"
      :current-user-id="ownershipInfo?.user_id || null"
      :current-user-name="''"
      :show-car-features="false"
      source="carsview"
      @close="closeCarDetails"
      @open-application="handleOpenApplication"
    />

    <ApplicationDetail
      v-if="showApplicationDetail"
      :application="selectedApplication"
      :current-user-id="ownershipInfo?.user_id || null"
      :current-user-name="''"
      :mode="'center'"
      @close="closeApplicationDetail"
      @application-changed="fetchCars"
    />
  </section>
</template>

<script>
import { readSearchFromRoute, writeSearchToRoute } from '@/utils/searchQueryParam';
import { apiRequest } from '@/api/client'
import { getViewportZoom } from '@/utils/viewportScale'
import { getUniqueCarsPaginated } from '@/api/cars'
import { useInfiniteList } from '@/composables/useInfiniteList'
import { useApplicationDetailLink } from '@/composables/useApplicationDetailLink'
import { openItemFromRoute } from '@/utils/openQueryParam'
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';
import SearchComponent from '@/components/SearchComponent.vue';
import FilterButton from '@/components/ui/FilterButton.vue';
import FilterSheet from '@/components/ui/FilterSheet.vue';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import RefreshButton from '@/components/RefreshButton.vue';
import RegistryLogModal from '@/components/RegistryLogModal.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import StatusBadge from '@/components/ui/StatusBadge.vue';
import ConfirmationModal from '@/components/ConfirmationModal.vue';
import VehicleDetailsModal from '@/components/CreateApplication/VehicleDetailsModal.vue';
import ApplicationDetail from '@/components/ApplicationDetail/ApplicationDetail.vue';
import BaseModal from '@/components/ui/BaseModal.vue';
import AppIcon from '@/components/icons/AppIcon.vue';

// Размер порции бесшовной подгрузки реестра машин (#1158, срез 2) - аналог
// APPLICATIONS_PER_PAGE в ApplicationsCenter.
const CARS_PER_PAGE = 30;

export default {
    components: {
        SearchComponent,
        FilterButton,
        FilterSheet,
        RefreshButton,
        RegistryLogModal,
        LoaderSpinner,
        StatusBadge,
        ConfirmationModal,
        VehicleDetailsModal,
        ApplicationDetail,
        BaseModal,
        AppIcon,
    },
    setup() {
        // Бесшовная подгрузка реестра машин порциями (#1158, срез 2): composable
        // инкапсулирует page/per_page/аккумуляцию/hasMore/seq-guard, тот же паттерн,
        // что useInfiniteList в ApplicationsCenter. carsData - алиас infiniteList.items:
        // pre-existing спека (CarsViewPermissionGating) пишет wrapper.vm.carsData
        // напрямую, переименование сломало бы её без пользы.
        const infiniteList = useInfiniteList({ perPage: CARS_PER_PAGE });
        // Мобилка (<=768): табы области сворачиваются в кнопку «Фильтр» + FilterSheet
        // (эпик mobile-filter-collapse, S3); десктоп-табы остаются инлайн.
        const { isNarrow } = useNarrowScreen();
        return {
            isNarrow,
            ...useApplicationDetailLink(),
            carsData: infiniteList.items,
            carsTotal: infiniteList.total,
            carsPage: infiniteList.page,
            hasMoreCars: infiniteList.hasMore,
            // canLoadMoreCars/listError/retryCarsList (#1173) - устойчивость бесшовной
            // подгрузки к ошибкам бэка (5xx/сеть): canLoadMore гейтит АВТОдогрузку
            // (observer + loadAllRemaining), hasMoreCars по-прежнему гейтит видимость
            // sentinel-контейнера (внутри него рисуется error+retry).
            canLoadMoreCars: infiniteList.canLoadMore,
            listLoading: infiniteList.loading,
            listError: infiniteList.error,
            loadCarsList: infiniteList.load,
            loadMoreCarsList: infiniteList.loadMore,
            retryCarsList: infiniteList.retry,
            observeCarsSentinel: infiniteList.observeSentinel,
            disconnectCarsSentinel: infiniteList.disconnectObserver,
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
            // carsData/carsTotal/hasMoreCars/listLoading выставлены из useInfiniteList
            // в setup() (#1158, срез 2).
            searchTimeout: null,
            // seq-guard (#632/#1158): смена фильтра/поиска до резолва предыдущего
            // fetchCars не должна запускать/продолжать устаревший loadAllRemainingCars.
            fetchSeq: 0,
            currentFilter: 'user',
            // Мобилка: bottom-sheet с табами области (S3 эпика mobile-filter-collapse).
            showScopeSheet: false,
            ownershipInfo: null,
            showModal: false,
            showRegistryLog: false,
            showDeleteCarModal: false,
            carToDelete: null,
            availableFormats: [],
            showDetailsViewModal: false,
            detailsCar: null,
            // Места разгрузки: список для имён + карта active_car_id -> [place ids]
            allUnloadingPlaces: [],
            carUnloadPlacesMap: {},

            
            // Формат номера
            selectedFormat: null,
            isFormatDropdownOpen: false,
            numberParts: [],
            
            // Марка
            selectedMark: '',
            selectedMarkId: null,
            isMarkDropdownOpen: false,
            markSearch: '',
            marks: [],
            filteredMarks: [],
            
            // Привязка
            bindToOrganization: false,
            bindToCompany: false,
            
            // Редактирование
            editingCar: null,
            originalCarData: null
        };
    },
    computed: {
        // Вкладка «Автомобили организации» (раздел реестра по организации).
        canSeeOrganization() {
            return usePermissionsStore().hasPermission('section.registry.organization');
        },
        // Вкладка «Автомобили компании» (раздел реестра по компании).
        canSeeCompany() {
            return usePermissionsStore().hasPermission('section.registry.company');
        },
        // Вкладка «Все машины системы» (all_system) - по разделу каталога;
        // супер/админ проходят, обычный юзер без гранта не видит. Бэк дополнительно
        // отдаёт 403 на all_system без прав.
        canSeeAllSystem() {
            return usePermissionsStore().hasPermission('section.registry.all_system');
        },
        // Право изменять реестр авто (кнопки «Добавить»/«Редактировать»). Базовая
        // роль выдаёт его по умолчанию; админ может отозвать ролью.
        canWriteCars() {
            return usePermissionsStore().hasPermission('entity.cars.write');
        },
        // Право удалять из реестра авто (кнопка «Удалить»). Базовая роль выдаёт
        // по умолчанию; админ может отозвать ролью, не затрагивая изменение.
        canDeleteCars() {
            return usePermissionsStore().hasPermission('entity.cars.delete');
        },
        // Администратор системы: правит и удаляет запись независимо от привязки.
        // Признак приходит из ownership-info - того же ответа, которым решает бэкенд,
        // иначе кнопка появилась бы там, где сервер отвечает 403.
        canManageAllEntities() {
            return this.ownershipInfo?.can_manage_all === true;
        },
        // Правим запись, которая не относится ни к нам, ни к нашей организации или
        // компании: так бывает только у администратора. Форма в этом режиме не
        // предлагает переключатели привязки - они говорят про МОЮ организацию.
        editingForeignRecord() {
            return !!this.editingCar && !this.carBelongsToUser(this.editingCar);
        },
        // Поиск по тексту выполняется на бэке через search_query (#1158, срез 2) -
        // здесь не дублируем, carsData уже отфильтрован сервером.
        sortedCars() {
            const cars = [...this.carsData];
            
            if (!this.sortField) {
                return cars;
            }
            
            return cars.sort((a, b) => {
                let valueA, valueB;
                
                switch (this.sortField) {
                    case 'id':
                        valueA = a.id;
                        valueB = b.id;
                        break;
                        
                    case 'number':
                        valueA = a.number?.toLowerCase() || '';
                        valueB = b.number?.toLowerCase() || '';
                        break;
                        
                    case 'mark':
                        valueA = a.mark?.toLowerCase() || '';
                        valueB = b.mark?.toLowerCase() || '';
                        break;
                        
                    case 'format_name':
                        valueA = a.format_name?.toLowerCase() || '';
                        valueB = b.format_name?.toLowerCase() || '';
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

        // Область отличается от дефолтной («Мои машины») - точка-индикатор на кнопке
        // «Фильтр» и доступность «Сбросить» в мобильном bottom-sheet (S3). Поиск сюда
        // не входит: он остаётся снаружи sheet (в шапке), как в S1/S2.
        scopeFilterActive() {
            return this.currentFilter !== 'user';
        },

        // Сортировка по колонкам - клиентская и должна идти по ВСЕМУ набору (как на
        // dev до пагинации), а не по одной загруженной порции: при активной сортировке
        // догружаем остаток (см. loadAllRemainingCars, #1158). Других клиентских
        // фильтров не осталось (поиск и filter_type - серверные), поэтому unlike
        // ApplicationsCenter здесь isFullLoad зависит только от sortField.
        isFullLoad() {
            return !!this.sortField;
        },

        // Футер "Показано X из Y": клиентских фильтров, урезающих carsData, не
        // осталось (сортировка не убирает строки), поэтому shown всегда равен total
        // загруженных, а "из carsTotal" - серверному счётчику всех совпадений.
        footerText() {
            return `Показано ${this.sortedCars.length} из ${this.carsTotal}`;
        },

        selectedFormatText() {
            return this.selectedFormat ? this.selectedFormat.format.name : 'Выберите формат';
        },

        canSaveCar() {
            // Проверяем формат номера
            if (!this.selectedFormat || !this.numberParts.length) {
                return false;
            }

            // Проверяем каждую клетку формата
            for (let i = 0; i < this.selectedFormat.cells.length; i++) {
                const cell = this.selectedFormat.cells[i];
                const part = this.numberParts[i];
                
                if (!part || part.length < cell.min_length || part.length > cell.max_length) {
                    return false;
                }
            }
            
            // Проверяем марку
            if (!this.selectedMark) {
                return false;
            }
            
            return true;
        }
    },
    watch: {
        // Пользователь уже на странице: mounted не перевызовется, а адрес сменился.
        '$route.query.open'(val) { if (val) this.openFromSearchLink(); },
        // Поиск - на сервере (#1158, срез 2): дебаунс 300мс перед fetchCars (reset на
        // стр.1 + очистка аккумулятора уже даёт loadCarsList({reset:true})). withPlaces:false
        // - места разгрузки от search_query не зависят, тянуть их на каждый ввод не нужно.
        searchQuery(val) {
            writeSearchToRoute(this.$router, this.$route, val);
            clearTimeout(this.searchTimeout);
            this.searchTimeout = setTimeout(() => {
                this.fetchCars({ withPlaces: false });
            }, 300);
        }
    },
    async mounted() {
        await Promise.all([
            this.fetchOwnershipInfo(),
            this.fetchFormats(),
            this.loadMarks()
        ]);
        // fetchCars сам подтягивает места разгрузки (allUnloadingPlaces + карта по машинам).
        await this.fetchCars();
        this.openFromSearchLink();
        
        // Закрытие dropdown при клике вне
        document.addEventListener('click', (e) => {
            if (!e.target.closest('.format__dropdown')) {
                this.isFormatDropdownOpen = false;
            }

            if (!e.target.closest('.mark__dropdown')) {
                this.isMarkDropdownOpen = false;
            }
        });

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
        this.disconnectCarsSentinel();
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
            openItemFromRoute({ router: this.$router, route: this.$route, items: this.carsData, open: this.openCarDetails });
        },

        /**
         * Тянет страницу на доступную высоту вьюпорта (под шапкой), чтобы таблица
         * занимала весь экран без скролла страницы. На мобильном (<=768px) сбрасываем.
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
        openCarDetails(car) {
            this.detailsCar = {
                id: car.id,
                plateNumber: car.number,
                mark: car.mark,
                formatId: car.format_id || null,
                organization: car.active_app_org_name || car.organization_name || null,
                organizationId: car.organization_id || null,
                company: car.active_app_company_name || car.company_name || null,
                companyId: car.company_id || null,
                isExisting: true,
                // active_car_id - id заявочной строки активной заявки; по нему тянем
                // места разгрузки и статус территории (в реестре их нет).
                activeCarId: car.active_car_id || null,
                // id самой заявки - для кнопки "Открыть заявку" (open-application).
                applicationId: car.active_application_id || null,
                unloadPlaces: this.carUnloadPlacesMap[car.active_car_id] || [],
                entry_date_to: car.active_entry_date_to,
                entry_time_from: car.active_entry_time_from,
                entry_time_to: car.active_entry_time_to,
                isActive: car.status,
                // Логин владельца сервер отдаёт только администратору, поэтому карточка
                // рисует строку по факту наличия значения, а не по своей проверке роли.
                user_name: car.user_name || null,
            };
            this.showDetailsViewModal = true;
        },
        closeCarDetails() {
            this.showDetailsViewModal = false;
            this.detailsCar = null;
        },

        /**
         * Можно ли текущему пользователю редактировать/удалять машину.
         * Логика совпадает с backend canEditCar (unique_car_service.go):
         * администратор системы правит любую запись, остальные - свою, своей
         * организации или своей компании. До этого вкладка «Все в системе» была
         * read-only для всех (PR #198); бюро обязано чинить записи контрагентов.
         */
        canEditCar(car) {
            if (this.canManageAllEntities) return true;
            if (this.currentFilter === 'all_system') return false;
            return this.carBelongsToUser(car);
        },
        /**
         * Машина «своя»: автор записи, её организация или компания совпадает с
         * текущим пользователем. Вынесено из canEditCar, потому что администратору
         * право даёт не принадлежность, а роль - а форме правки нужно знать именно
         * принадлежность, чтобы не переписать чужую привязку своей.
         */
        carBelongsToUser(car) {
            if (!this.ownershipInfo) return false;
            if (car.user_id != null && car.user_id === this.ownershipInfo.user_id) return true;
            if (car.organization_id != null && this.ownershipInfo.organization_id != null
                && car.organization_id === this.ownershipInfo.organization_id) return true;
            if (car.company_id != null && this.ownershipInfo.company_id != null
                && car.company_id === this.ownershipInfo.company_id) return true;
            return false;
        },
        showEditCar(car) {
            return this.canEditCar(car) && this.canWriteCars;
        },
        showDeleteCar(car) {
            return this.canEditCar(car) && this.canDeleteCars;
        },
        canEditTooltip(car) {
            if (this.currentFilter === 'all_system' && !this.canManageAllEntities) return 'В режиме «Все в системе» редактирование доступно только администратору';
            if (!this.canEditCar(car)) return 'Машина не привязана к вашей организации/компании - редактирование запрещено';
            return 'Недостаточно прав для изменения или удаления';
        },

        isCarBlacklisted(car) {
            return car.is_blacklisted === true;
        },

        /**
         * @param {{withPlaces?: boolean}} [opts] withPlaces=false пропускает две
         *   тяжёлые полные выборки мест разгрузки (/unload-places + /cars/unload-places),
         *   которые не зависят от search_query - при поиске (дебаунс на каждый ввод) их
         *   дёргать не нужно. Дефолт true (mount/смена filter_type/refresh/удаление/
         *   application-changed) - там active_car_id и набор мест могли измениться.
         *   Событийные вызовы из шаблона (@refresh/@application-changed) передают event
         *   первым аргументом, а не { withPlaces: false } -> `!== false` даёт true, места
         *   грузятся. seq-токен защищает от продолжения устаревшего прохода (#632).
         */
        async fetchCars(opts = {}) {
            const withPlaces = opts.withPlaces !== false;
            const seq = ++this.fetchSeq;
            this.loading = true;
            try {
                await this.loadCarsList(this.buildCarsPage, { reset: true });
                if (seq !== this.fetchSeq) return; // устарел - актуальный запрос уже идёт

                // Клиентская сортировка требует ВЕСЬ набор (как на dev до пагинации):
                // догружаем оставшиеся порции, чтобы сортировка шла по полному списку (#1158).
                if (this.isFullLoad) {
                    await this.loadAllRemainingCars(seq);
                    if (seq !== this.fetchSeq) return;
                }

                // Места разгрузки: при смене активной заявки у машины меняется
                // active_car_id и набор мест, иначе карта устаревает после рефреша.
                // При поиске (withPlaces=false) не трогаем - от search_query не зависят.
                if (withPlaces) {
                    await Promise.all([this.fetchUnloadingPlaces(), this.fetchCarUnloadPlaces()]);
                }
            } catch (error) {
                console.error("Ошибка при загрузке машин:", error);
            } finally {
                if (seq === this.fetchSeq) this.loading = false;
            }
        },

        // Догрузка всех оставшихся порций (full-load режим: активная клиентская
        // сортировка, #1158). seq-guard прерывает устаревший проход, если пользователь
        // сменил фильтр/поиск и стартовал новый fetchCars; guard - от бесконечного
        // цикла, если total/hasMore разъедутся.
        async loadAllRemainingCars(seq) {
            let guard = 0;
            // canLoadMoreCars (не hasMoreCars, #1173): при ошибке бэка на промежуточной
            // странице circuit-breaker останавливает цикл сразу, не дожидаясь guard>200.
            while (this.canLoadMoreCars && seq === this.fetchSeq) {
                await this.loadMoreCarsList(this.buildCarsPage);
                if (++guard > 200) break;
            }
        },

        /**
         * fetchPage для useInfiniteList (#1158): строит параметры текущего
         * фильтра/поиска плюс page/per_page - бэк переключается на GetAllPaginated,
         * как только видит per_page (internal/handlers/unique_cars.go).
         */
        async buildCarsPage(page, perPage) {
            const params = { filter_type: this.currentFilter, page, per_page: perPage };
            if (this.searchQuery.trim()) {
                params.search_query = this.searchQuery.trim();
            }
            const { items, meta } = await getUniqueCarsPaginated(params);
            return { items, total: (meta && meta.total) || 0 };
        },

        // Автодогрузка следующей порции по пересечению sentinel с cars-body (#1158).
        // root - сам .cars-body: у него свой overflow-y:auto, не документ, дефолтный
        // root (viewport) пересечение бы не заметил. el=null (v-if="hasMoreCars"===false)
        // просто отключает observer.
        setCarsSentinelRef(el) {
            this.observeCarsSentinel(el, this.buildCarsPage, { root: this.$refs.carsBody || null });
        },

        // Ручной повтор упавшей страницы (первичной или догрузки, #1173) - composable
        // сам помнит, какой fetchPage/режим (reset/append) последним завершился ошибкой.
        async retryCars() {
            try {
                await this.retryCarsList();
                // full-load (клиентская сортировка): retry вернул только упавшую
                // страницу, но сортировка идёт по ВСЕМУ набору - дозагружаем остаток,
                // иначе результат по НЕПОЛНОМУ списку до ручного доскролла (#1173).
                if (this.isFullLoad) {
                    await this.loadAllRemainingCars(this.fetchSeq);
                }
            } catch (error) {
                console.error("Ошибка сети при повторной попытке загрузки машин:", error);
            }
        },

        async fetchOwnershipInfo() {
            try {
                const response = await apiRequest("/unique-cars/ownership-info", {
                    method: "GET"});

                if (response.ok) {
                    this.ownershipInfo = await response.json();
                }
            } catch (error) {
                console.error("Ошибка при загрузке информации о владельце:", error);
            }
        },

        async fetchFormats() {
            try {
                const response = await apiRequest("/license-plate-formats", {
                    method: "GET"});

                if (response.ok) {
                    this.availableFormats = await response.json();
                    // Выбираем формат по умолчанию или первый формат
                    const defaultFormat = this.availableFormats.find(f => f.format.is_default);
                    this.selectedFormat = defaultFormat || this.availableFormats[0];
                    this.initializeNumberParts();
                }
            } catch (error) {
                console.error("Ошибка при загрузке форматов:", error);
            }
        },

        async fetchUnloadingPlaces() {
            try {
                const response = await apiRequest('/unload-places', { method: 'GET' });
                if (response.ok) this.allUnloadingPlaces = await response.json();
            } catch (error) {
                console.error('Ошибка при загрузке мест разгрузки:', error);
            }
        },

        // Карта active_car_id -> [unload_place_id]. Реестр /unique-cars не несёт места
        // разгрузки (они в заявочной cars), поэтому мапим по id активной заявочной строки.
        async fetchCarUnloadPlaces() {
            try {
                const response = await apiRequest('/cars/unload-places', { method: 'GET' });
                if (!response.ok) return;
                const rows = await response.json();
                const map = {};
                (rows || []).forEach(cup => {
                    if (!map[cup.car_id]) map[cup.car_id] = [];
                    if (!map[cup.car_id].includes(cup.unload_place_id)) {
                        map[cup.car_id].push(cup.unload_place_id);
                    }
                });
                this.carUnloadPlacesMap = map;
            } catch (error) {
                console.error('Ошибка при загрузке мест разгрузки машин:', error);
            }
        },

        openDeleteCarConfirmation(car) {
            this.carToDelete = car;
            this.showDeleteCarModal = true;
        },

        cancelDeleteCar() {
            this.showDeleteCarModal = false;
            this.carToDelete = null;
        },

        async confirmDeleteCar() {
            if (!this.carToDelete) return;
            const car = this.carToDelete;
            this.showDeleteCarModal = false;
            this.carToDelete = null;

            try {
                const response = await apiRequest(`/unique-cars/${car.id}`, {
                    method: "DELETE"
                });

                if (response.ok) {
                    await this.fetchCars();
                    const num = car.number || '';
                    useDeletionsStore().notify({
                        prefix: 'Автомобиль ',
                        bold: num,
                        suffix: ' удалён',
                    });
                } else {
                    useDeletionsStore().notify({ prefix: 'Ошибка при удалении автомобиля', type: 'error' });
                }
            } catch (error) {
                console.error("Ошибка при удалении автомобиля:", error);
                useDeletionsStore().notify({ prefix: 'Ошибка при удалении автомобиля', type: 'error' });
            }
        },

        editCar(car) {
            if (this.currentFilter === 'all_system') {
                return; // Запрещаем редактирование для вкладки "Все машины системы"
            }
            
            this.editingCar = car;
            
            // Сохраняем оригинальные значения для сравнения
            this.originalCarData = {
                mark: car.mark,
                format_id: car.format_id,
                number: car.number,
                organization_id: car.organization_id,
                company_id: car.company_id
            };
            
            // Устанавливаем текущие значения машины
            this.selectedMark = car.mark;
            
            // Находим формат по format_id
            if (car.format_id) {
                const carFormat = this.availableFormats.find(f => f.format.id === car.format_id);
                if (carFormat) {
                    this.selectedFormat = carFormat;
                    // Разбиваем номер на части согласно формату
                    this.numberParts = car.number.split(' ');
                } else {
                    // Если формат не найден, используем формат по умолчанию
                    const defaultFormat = this.availableFormats.find(f => f.format.is_default) || this.availableFormats[0];
                    this.selectedFormat = defaultFormat;
                    this.initializeNumberParts();
                }
            }
            
            // Устанавливаем привязки
            this.bindToOrganization = !!car.organization_id;
            this.bindToCompany = !!car.company_id;
            
            this.showModal = true;
        },

        // Проверка наличия изменений
        hasChanges() {
            if (!this.editingCar) {
                return true; // Для новой машины всегда отправляем запрос
            }

            // Проверяем изменения в марке
            if (this.selectedMark !== this.originalCarData.mark) {
                return true;
            }

            // Проверяем изменения в формате
            if (this.selectedFormat.format.id !== this.originalCarData.format_id) {
                return true;
            }

            // Проверяем изменения в номере
            const currentNumber = this.numberParts.join(' ');
            if (currentNumber !== this.originalCarData.number) {
                return true;
            }

            // У чужой записи привязку форма не показывает и не отправляет, поэтому и
            // сравнивать нечего: иначе организация администратора не совпала бы с
            // организацией записи, и «изменения» находились бы всегда.
            if (this.editingForeignRecord) {
                return false;
            }

            // Проверяем изменения в привязке к организации
            const currentOrgId = this.bindToOrganization ? this.ownershipInfo.organization_id : null;
            if (currentOrgId !== this.originalCarData.organization_id) {
                return true;
            }

            // Проверяем изменения в привязке к компании
            const currentCompanyId = this.bindToCompany ? this.ownershipInfo.company_id : null;
            if (currentCompanyId !== this.originalCarData.company_id) {
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
            // Сортировка клиентская - должна идти по всему набору. Если ещё не всё
            // загружено, догружаем остаток (тот же паттерн, что в Центре заявок, #1158).
            if (this.isFullLoad && this.hasMoreCars) {
                this.fetchCars();
            }
        },

        switchFilter(filterType) {
            this.currentFilter = filterType;
            this.fetchCars();
        },

        // Мобильный bottom-sheet: выбор области применяется и закрывает лист (одиночный
        // выбор, как пикер), поиск остаётся снаружи (S3 эпика mobile-filter-collapse).
        switchScopeFromSheet(filterType) {
            this.switchFilter(filterType);
            this.showScopeSheet = false;
        },

        // «Сбросить фильтры» в sheet - вернуть дефолтную область «Мои машины» и закрыть
        // лист. Кнопка активна только при scopeFilterActive, лишнего fetch на дефолте нет.
        resetScopeFilter() {
            this.switchFilter('user');
            this.showScopeSheet = false;
        },

        showAddCarModal() {
            if (this.currentFilter === 'all_system') {
                return; // Запрещаем добавление для вкладки "Все машины системы"
            }
            
            this.editingCar = null;
            this.showModal = true;
            this.filteredMarks = this.marks;
            this.resetNewCar();
        },

        closeModal() {
            this.showModal = false;
            this.editingCar = null;
            this.resetNewCar();
        },

        resetNewCar() {
            this.selectedFormat = this.availableFormats.find(f => f.format.is_default) || this.availableFormats[0];
            this.initializeNumberParts();
            this.selectedMark = '';
            this.markSearch = '';
            this.filteredMarks = this.marks;
            this.bindToOrganization = false;
            this.bindToCompany = false;
        },

        clearFormFields() {
            // Очищаем только номер и марку, чекбоксы остаются
            this.initializeNumberParts();
            this.selectedMark = '';
            this.markSearch = '';
            this.filteredMarks = this.marks;
            
            // Если это добавление новой машины (не редактирование), сбрасываем чекбоксы
            if (!this.editingCar) {
                this.bindToOrganization = false;
                this.bindToCompany = false;
            }
            // При редактировании поля НЕ очищаются - остаются текущие значения машины
        },

        // Формат номера методы
        initializeNumberParts() {
            if (this.selectedFormat) {
                this.numberParts = new Array(this.selectedFormat.cells.length).fill('');
            } else {
                this.numberParts = [];
            }
        },

        getPlaceholder(cell) {
            if (cell.cell_type === 'numbers') {
                return '0'.repeat(cell.max_length);
            } else {
                return 'A'.repeat(cell.max_length);
            }
        },

        getInputWidth(cell) {
            const baseWidth = 25;
            const minWidth = 50;
            const width = Math.max(minWidth, cell.max_length * baseWidth);
            return `${width}px`;
        },

        validatePart(index, event, cell) {
            let value = event.target.value.toUpperCase();
            
            if (cell.cell_type === 'numbers') {
                value = value.replace(/\D/g, '');
            } else if (cell.cell_type === 'letters') {
                if (cell.alphabet_type === 'cyrillic') {
                    value = this.filterCyrillicLetters(value, cell.allowed_letters);
                } else if (cell.alphabet_type === 'latin') {
                    value = this.filterLatinLetters(value, cell.allowed_letters);
                } else if (cell.alphabet_type === 'both') {
                    value = this.filterBothLetters(value, cell.allowed_letters);
                }
            } else if (cell.cell_type === 'mixed') {
                if (cell.alphabet_type === 'cyrillic') {
                    value = this.filterMixedCyrillic(value, cell.allowed_letters);
                } else if (cell.alphabet_type === 'latin') {
                    value = this.filterMixedLatin(value, cell.allowed_letters);
                } else if (cell.alphabet_type === 'both') {
                    value = this.filterMixedBoth(value, cell.allowed_letters);
                }
            }
            
            if (value.length > cell.max_length) {
                value = value.slice(0, cell.max_length);
            }
            
            this.numberParts[index] = value;
            event.target.value = value;
        },

        formatPart(index, cell) {
            if (cell.cell_type === 'numbers' && cell.padding_side && this.numberParts[index]) {
                let value = this.numberParts[index];
                const targetLength = cell.max_length;
                
                if (value.length < targetLength) {
                    const paddingChar = cell.padding_char || '0';
                    if (cell.padding_side === 'left') {
                        value = value.padStart(targetLength, paddingChar);
                    } else {
                        value = value.padEnd(targetLength, paddingChar);
                    }
                    this.numberParts[index] = value;
                }
            }
        },

        filterCyrillicLetters(value, allowedLetters) {
            if (allowedLetters) {
                const allowedChars = allowedLetters.split('');
                return value.split('').filter(char => allowedChars.includes(char)).join('');
            } else {
                return value.replace(/[^АВЕКМНОРСТУХ]/g, '');
            }
        },

        filterLatinLetters(value, allowedLetters) {
            if (allowedLetters) {
                const allowedChars = allowedLetters.split('');
                return value.split('').filter(char => allowedChars.includes(char)).join('');
            } else {
                return value.replace(/[^A-Z]/g, '');
            }
        },

        filterBothLetters(value, allowedLetters) {
            if (allowedLetters) {
                const allowedChars = allowedLetters.split('');
                return value.split('').filter(char => allowedChars.includes(char)).join('');
            } else {
                return value.replace(/[^A-ZА-Я]/g, '');
            }
        },

        filterMixedCyrillic(value, allowedLetters) {
            const numericPart = value.replace(/\D/g, '');
            const letterPart = this.filterCyrillicLetters(value.replace(/[0-9]/g, ''), allowedLetters);
            return numericPart + letterPart;
        },

        filterMixedLatin(value, allowedLetters) {
            const numericPart = value.replace(/\D/g, '');
            const letterPart = this.filterLatinLetters(value.replace(/[0-9]/g, ''), allowedLetters);
            return numericPart + letterPart;
        },

        filterMixedBoth(value, allowedLetters) {
            const numericPart = value.replace(/\D/g, '');
            const letterPart = this.filterBothLetters(value.replace(/[0-9]/g, ''), allowedLetters);
            return numericPart + letterPart;
        },

        // Dropdown методы
        toggleFormatDropdown() {
            this.isFormatDropdownOpen = !this.isFormatDropdownOpen;
        },

        selectFormat(format) {
            this.selectedFormat = format;
            this.initializeNumberParts();
            this.isFormatDropdownOpen = false;
        },

        toggleMarkDropdown() {
            this.isMarkDropdownOpen = !this.isMarkDropdownOpen;
            if (this.isMarkDropdownOpen) {
                this.filterMarks();
            }
        },

        async loadMarks() {
            try {
                const { listMarks } = await import('@/api/marks');
                const res = await listMarks();
                const arr = Array.isArray(res) ? res : (res?.marks || []);
                this.marks = arr
                    .filter(m => m.is_active !== false)
                    .map(m => ({ id: m.id, name: m.name }));
                this.filteredMarks = this.marks;
            } catch (err) {
                console.error('Не удалось загрузить справочник марок', err);
                this.marks = [];
                this.filteredMarks = [];
            }
        },

        filterMarks() {
            if (!this.markSearch) {
                this.filteredMarks = this.marks;
            } else {
                const searchTerm = this.markSearch.toLowerCase();
                this.filteredMarks = this.marks.filter(mark =>
                    mark.name.toLowerCase().includes(searchTerm)
                );
            }
        },

        selectMark(mark) {
            this.selectedMark = mark.name;
            this.selectedMarkId = mark.id;
            this.isMarkDropdownOpen = false;
            this.markSearch = '';
        },

        async saveCar() {
            if (!this.canSaveCar) {
                useDeletionsStore().notify({ bold: 'Заполните обязательные поля', type: 'error' });
                return;
            }

            // Проверяем изменения для редактирования
            if (this.editingCar && !this.hasChanges()) {
                useDeletionsStore().notify({ bold: 'Изменений не обнаружено', type: 'info' });
                return;
            }

            try {
                // Формируем номер из частей
                const number = this.numberParts.join(' ');
                
                // Формируем данные для отправки
                const carData = {
                    number: number,
                    mark: this.selectedMark,
                    format_id: this.selectedFormat.format.id
                };
                if (this.editingForeignRecord) {
                    // Администратор правит машину чужой организации: привязку переносим
                    // как есть. Прежние поля брались из ownership-info правящего, то есть
                    // машина контрагента переехала бы к бюро вместе с исправлением марки.
                    // user_id не отправляем вовсе - сервер сохранит прежнего владельца.
                    carData.organization_id = this.editingCar.organization_id ?? null;
                    carData.company_id = this.editingCar.company_id ?? null;
                } else {
                    carData.user_id = this.ownershipInfo.user_id;
                    carData.organization_id = this.bindToOrganization ? this.ownershipInfo.organization_id : null;
                    carData.company_id = this.bindToCompany ? this.ownershipInfo.company_id : null;
                }

                let response;
                if (this.editingCar) {
                    // Редактирование существующей машины
                    response = await apiRequest(`/unique-cars/${this.editingCar.id}`, {
                        method: "PUT",
                        body: JSON.stringify(carData)
                    });
                } else {
                    // Создание новой машины
                    response = await apiRequest("/unique-cars", {
                        method: "POST",
                        body: JSON.stringify(carData)
                    });
                }

                if (response.ok) {
                    const action = this.editingCar ? 'обновлён' : 'добавлен';
                    useDeletionsStore().notify({ prefix: 'Автомобиль ', bold: action, type: 'success' });
                    
                    // Обновляем список машин
                    this.fetchCars();
                    
                    // Очищаем форму только при добавлении новой машины
                    if (!this.editingCar) {
                        this.clearFormFields();
                    } else {
                        // Обновляем оригинальные данные после успешного сохранения
                        this.originalCarData = {
                            mark: this.selectedMark,
                            format_id: this.selectedFormat.format.id,
                            number: number,
                            organization_id: carData.organization_id,
                            company_id: carData.company_id
                        };
                    }
                } else {
                    const errorData = await response.json();
                    // Текст сервера показываем как есть - см. EmployeeEditModal (#2021):
                    // он различает, где именно нашлась машина, а подмена отправляла
                    // человека искать её в свой список, где её нет.
                    const errorMessage = errorData.message || "Ошибка при сохранении автомобиля";
                    useDeletionsStore().notify({ bold: errorMessage, type: 'error' });
                }
            } catch (error) {
                console.error("Ошибка при сохранении автомобиля:", error);
                useDeletionsStore().notify({ prefix: 'Не удалось сохранить ', bold: 'автомобиль', type: 'error' });
            }
        }
    }
}
</script>

<style scoped>
.carsview {
    padding: 20px;
    display: flex;
    flex-direction: column;
}

.carsview__container {
    display: flex;
    gap: 30px;
    margin-top: 20px;
    flex: 1;
    min-height: 0;
}

.carsview__right-side {
    width: 25%;
}

.carsview__help {
    /* Карточка-подсказка лежит на фоне страницы и несёт его же цвет без этой строки. */
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 15px;
    padding: 16px 20px;
}

.carsview__header {
    padding-bottom: 15px;
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.carsview__title {
    font-size: 18px;
    display: inline-flex;
    align-items: center;
    gap: 6px;
}

.carsview__subtitle {
    font-size: 13px;
    color: var(--color-text-muted, var(--text-muted));
    margin: 0;
}

.carsview__filters {
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
.cars-card {
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

.cars-container {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    overflow-y: auto;
}

/* cars-header повторяет геометрию cars-body (padding-right + margin-right 4px),
   чтобы доступная ширина колонок совпала и заголовки выровнялись с данными. */
.cars-header {
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
    padding-right: 4px;
    margin-right: 4px;
}

/* header-row повторяет геометрию car-row: padding 12/16 + flex. */
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

.car-number-col {
    width: 20%;
    min-width: 120px;
}

.brand-col {
    width: 20%;
    min-width: 120px;
}

.format-col {
    width: 25%;
    min-width: 120px;
}

.status-col {
    width: 21%;
    min-width: 135px;
}

.actions-col {
    width: 10%;
    min-width: 80px;
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
.cars-body {
    overflow-y: auto;
    flex-grow: 1;
    padding-right: 4px;
    margin-right: 4px;
    scroll-behavior: smooth;
}

.car-item {
    transition: background-color 0.2s ease;
}

.car-item:hover {
    background-color: var(--surface-2);
}

.car-row {
    display: flex;
    width: 100%;
    padding: 10px 16px;
    align-items: center;
    border-bottom: 1px solid var(--border);
    cursor: pointer;
}

.car-col {
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
.number-col .car-col,
.actions-col .car-col {
    justify-content: center;
}

/* Стилизация скроллбара */
.cars-body::-webkit-scrollbar {
    width: 6px;
}

.cars-body::-webkit-scrollbar-track {
    background: transparent;
    margin: 2px 0;
    border-radius: 3px;
}

.cars-body::-webkit-scrollbar-thumb {
    background: color-mix(in srgb, var(--accent) 22%, var(--surface));
    border-radius: 3px;
    border: 1px solid transparent;
    background-clip: content-box;
    transition: all 0.3s ease;
}

.cars-body::-webkit-scrollbar-thumb:hover {
    background: color-mix(in srgb, var(--accent) 22%, var(--surface));
    border: 1px solid transparent;
    background-clip: content-box;
    transform: scale(1.1);
}

.cars-body {
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
   .cars-body растягивался и скроллился, а кольцо/пусто центрировались по высоте. */
.cars-table-area {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
}

/* Бесшовная подгрузка (#1158): sentinel внизу .cars-body, футер под таблицей. */
.scroll-sentinel {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 24px;
    padding: 10px 0;
}

/* Устойчивость к ошибкам бэка (#1173): первичная загрузка упала - список пуст,
   вместо "Автомобилей нет" показываем причину + retry. */
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

/* Модальное окно добавления машины теперь на BaseModal (шапка/крестик/overlay/
   Escape/bottom-sheet - его контракт). base-modal__body у BaseModal идёт БЕЗ padding
   (отступы несёт содержимое) - без них поля упирались в края окна и на телефоне
   читались еле-еле ("отступов нет по бокам"). Значение - как у соседних окон на
   BaseModal (ChangePasswordModal/AttachmentMappingCopyModal): 20px по бокам вровень
   с заголовком шапки. */
.data__completion {
    padding: 14px 20px 18px;
}

.input__label {
    font-size: 13px;
    color: var(--text-muted);
}

.required {
    color: var(--danger-text);
}

.completion__format {
    display: flex;
    flex-direction: column;
    gap: 5px;
    position: relative;
    padding-bottom: 10px;
}

.format__header {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.format__label {
    font-size: 13px;
    color: var(--text-muted);
}

.format__dropdown {
    position: relative;
}

.dropdown__button {
    width: 100%;
    height: 30px;
    border: 1px solid var(--border);
    background-color: var(--surface);
    border-radius: 50px;
    outline: none;
    cursor: pointer;
    padding: 0 15px;
    transition: border-color 0.2s;
}

.dropdown__button:hover {
    border-color: var(--accent);
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
    color: var(--text);
    font-weight: 500;
}

.button__arrow {
    width: 10px;
    height: 6px;
    flex-shrink: 0;
    color: var(--text-muted);
    transition: transform 0.2s;
}

.button__arrow--open {
    transform: rotate(180deg);
}

.dropdown__menu {
    position: absolute;
    top: 100%;
    left: 0;
    width: 100%;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    margin-top: 5px;
    box-shadow: 0 3px 10px var(--shadow-drop);
    z-index: 1000;
    max-height: 300px;
    overflow-y: auto;
}

.dropdown__item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 15px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.dropdown__item:hover {
    background-color: var(--surface-2);
}

.dropdown__item:first-child {
    border-radius: 10px 10px 0 0;
}

.dropdown__item:last-child {
    border-radius: 0 0 10px 10px;
}

.item__text {
    font-size: 13px;
    color: var(--text);
}

.completion__fields {
    display: flex;
    gap: 20px;
    align-items: flex-start;
    margin-bottom: 15px;
}

.completion__number,
.completion__mark {
    flex: 1;
}

.completion__number-header,
.completion__mark-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 5px;
}

.number__field {
    max-width: 100%;
    min-width: 100%;
    height: 40px;
    display: flex;
    border: 1px solid var(--border);
    border-radius: 15px;
    overflow: hidden;
    background: var(--surface);
}

.no-format-message {
    font-size: 12px;
    color: var(--text-muted);
    text-align: center;
    padding: 10px;
    background: var(--surface-2);
    border-radius: 10px;
}

.number__input {
    border: none;
    height: 100%;
    outline: none;
    text-align: center;
    font-size: 14px;
    background: transparent;
    flex: 1;
    min-width: 0;
}

.number__input:not(:last-child) {
    border-right: 1px solid var(--border);
}

.number__input:first-child {
    border-radius: 15px 0 0 15px;
}

.number__input:last-child {
    border-radius: 0 15px 15px 0;
}

.number__input::placeholder {
    color: var(--text-muted);
    font-size: 12px;
}

.number__input:focus {
    background-color: var(--surface-2);
}

/* Mark dropdown styles */
.mark__field {
    width: 100%;
    height: 40px;
    position: relative;
}

.mark__dropdown {
    width: 100%;
    height: 100%;
}

.mark__dropdown-button {
    width: 100%;
    height: 100%;
    border: 1px solid var(--border);
    background-color: var(--surface);
    border-radius: 15px;
    outline: none;
    cursor: pointer;
    padding: 0 15px;
    transition: border-color 0.2s;
}

.mark__dropdown-button:hover {
    border-color: var(--accent);
}

.mark__button-content {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    height: 100%;
    justify-content: space-between;
}

.mark__button-text {
    font-size: 14px;
    color: var(--text);
    /* Без нулевого минимума флекс-элемент не сжимается ниже своего текста,
       и длинная марка лезет под стрелку вместо многоточия. */
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.mark__button-arrow {
    width: 10px;
    height: 6px;
    flex-shrink: 0;
    color: var(--text-muted);
    transition: transform 0.2s;
}

.mark__button-arrow--open {
    transform: rotate(180deg);
}

.mark__dropdown-menu {
    position: absolute;
    top: 100%;
    left: 0;
    width: 100%;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    margin-top: 5px;
    box-shadow: 0 3px 10px var(--shadow-drop);
    z-index: 1000;
    max-height: 220px;
    overflow: hidden;
}

.mark__search {
    padding: 10px;
    border-bottom: 1px solid var(--border);
}

.mark__search-input {
    width: 100%;
    border: 1px solid var(--border);
    border-radius: 15px;
    padding: 5px 10px;
    outline: none;
    font-size: 14px;
}

.mark__dropdown-list {
    max-height: 144px;
    overflow-y: auto;
}

.mark__dropdown-item {
    padding: 8px 15px;
    cursor: pointer;
    transition: background-color 0.2s;
    border-bottom: 1px solid var(--surface-2);
}

.mark__dropdown-item:hover {
    background-color: var(--surface-2);
}

.mark__dropdown-item:last-child {
    border-bottom: none;
}

.mark__item-text {
    font-size: 14px;
    color: var(--text);
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

/* Привязка */
.completion__binding {
    margin-top: 15px;
    padding-top: 15px;
    border-top: 1px solid var(--border);
}

.binding-info {
    margin-top: 10px;
    margin-bottom: 10px;
}

.binding-note {
    font-size: 12px;
    color: var(--text-muted);
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
    color: var(--text);
    font-weight: 400;
}

.red {
    color: var(--danger-text);
}

@media (max-width: 768px) {
    .cars-card {
        width: 100%;
        height: auto;
    }

    /* Таблица машин -> карточки через rt-* (responsive-tables.css, брейкпоинт 767.98).
       Прежний горизонтальный скролл 800px убран: строки собираются в карточки,
       заголовок таблицы скрыт (rt-head-row). Зазор между карточками - ниже, в
       блоке 767.98 (сиблинг .rt-row+.rt-row не сработает: rt-row на .car-row,
       вложенном в .car-item - v-for-обёртку). */

    /* Заголовок страницы с подзаголовком на мобилке не показываем: то же название
       («Мои автомобили») несёт шапка списка, а поясняющий абзац дублирует блок
       подсказки под таблицей. Освобождённые ~60px уходят под сами карточки. */
    .carsview__header {
        display: none;
    }

    /* Поиск и «Фильтр» - отдельная полоса 36px под шапкой списка. Разметка гейтится
       isNarrow (768), поэтому и правила стоят здесь, а не в блоке 767.98: иначе ровно
       на 768.0 полоса отрендерилась бы без своих стилей.

       Боковой отступ - 8px, столько же, сколько у .cars-body: рамка поиска встаёт ровно
       над рамкой первой карточки. Прежние 12px давали третий уступ между шапкой и
       списком. */
    .carsview__toolbar {
        display: flex;
        flex-shrink: 0;
        align-items: center;
        gap: 8px;
        padding: 8px;
        border-bottom: 1px solid var(--border);
    }

    .carsview__toolbar :deep(.search) {
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
    .carsview__action-bar {
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

    /* Шапка списка и модалка машины делят класс .add-button, поэтому широкий вариант -
       отдельный модификатор, а не правило по контексту: в модалке кнопка сохранения
       остаётся своей ширины. */
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
       телепортнутого контента sheet: каждый таб на всю ширину строкой - единый ровный
       вид на любой ширине телефона (тексты табов разной длины). */
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
    .carsview {
        padding: var(--gutter);
        padding-bottom: calc(64px + var(--gutter) + env(safe-area-inset-bottom, 0px));
    }

    .cars-card {
        width: 100%;
        flex: none;
    }

    .carsview__container {
        flex-direction: column;
    }

    .carsview__right-side {
        width: 100%;
    }
    
    .completion__fields {
        flex-direction: column;
    }

    .format-actions {
        flex-direction: column;
        width: 100%;
    }
    
    /* Кнопка сохранения в модалке машины - по содержимому, а не на всю ширину
       (третий круг замечаний владельца, #1097 w9): прежний width:100% растягивал
       «Добавить»/«Сохранить» в «сосиску» на всю строку шапки, из-за чего подпись
       «Формат номеров» слева не помещалась и переносилась на вторую строку.
       .format__header остаётся flex + justify-content:space-between - кнопка
       по содержимому справа, подпись слева на одной строке. */

    /* Подписи чекбоксов привязки («Привязать к организации/компании») на телефоне
       было еле видно - +2px к шрифту и увеличенный чекбокс (12px -> 18px, тот же приём,
       что у выбора машин/сотрудников в ExistingCarsModal/ExistingEmployeesModal: видимый
       квадрат остаётся некрупным, а тач-таргет строки дотягивает до нормы проекта 36px
       через min-height, а не раздутый чекбокс). */
    .binding-option {
        min-height: 36px;
        font-size: 14px;
    }

    .binding-option input[type="checkbox"] {
        width: 18px;
        height: 18px;
        flex-shrink: 0;
    }

    .user-binding-text {
        font-size: 12px;
    }
}

/* Шапка блока и карточки машин на мобилке. Порог 767.98 - тот же, на котором таблица
   превращается в карточки (responsive-tables.css) и на котором RefreshButton сворачивается
   в иконку: шапка и тело переключаются вместе, гибрида на 768.0 нет. Зазор между карточками
   отдельным правилом (а не через .rt-row+.rt-row): rt-row навешен на .car-row, вложенный в
   .car-item (v-for-обёртку), поэтому соседние .rt-row не являются прямыми сиблингами. */
@media (max-width: 767.98px) {
    /* Прокручивается страница, и только она (#1097 волна 6). Список лежал в трёх
       вложенных областях прокрутки подряд (.card-content -> .cars-container ->
       .cars-body), палец попадал в них, а не в документ, и экран стоял на месте.
       Проверено настоящим тач-драгом; window.scrollTo этот дефект не ловит. */
    .card-content,
    .cars-container,
    .cars-table-area,
    .cars-body {
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
       заголовок («Все машины системы») выдавил бы кнопки за край экрана вместо того,
       чтобы обрезаться многоточием. */
    .card-header__title {
        flex: 1 1 auto;
        min-width: 0;
    }

    /* 18px - кегль имени экрана в проекте (.carsview__title, .employeesview__title,
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

    /* Одинаковый зазор вокруг карточек: у .cars-body асимметрия от десктопного
       скроллбара (padding-right + margin-right по 4px), из-за неё слева карточки
       упирались в рамку блока, а справа висел отступ 8px. */
    .cars-body {
        padding: 8px;
        margin-right: 0;
    }

    .cars-body .car-item + .car-item {
        margin-top: 8px;
    }

    /* Карточка по образцу среза 2 (ApplicationAttachmentDetail.vue): подписи полей убраны,
       значения выровнены влево, порядковый номер на мобилке не показываем вовсе.
       !important обязателен: правило-источник в responsive-tables.css стоит на той же
       специфичности (0,3,0), а оба view - lazy route-чанки, их scoped-CSS грузится позже
       глобального, и при равной специфичности исход решал бы порядок загрузки. */
    .car-row.rt-row > .number-col {
        display: none !important;
    }

    .car-row.rt-row > [data-label]::before {
        display: none !important;
    }

    /* Исключение из «убрать все подписи»: «Формат номера» без подписи нечитаем (значение
       вида «Российский» само по себе ни о чём не говорит), а организация и компания идут
       двумя соседними строками с однотипными названиями - без подписи не различить, где
       какая. */
    .car-row.rt-row > .format-col::before,
    .car-row.rt-row > .org-col::before,
    .car-row.rt-row > .company-col::before {
        display: block !important;
    }

    /* Поле карточки - строка не ниже 30px с содержимым по центру по вертикали: без
       общего минимума строки разной высоты (бейдж статуса против одной строки текста)
       давали рваную сетку, и список читался кашей.

       Разделитель полей рисуем СВЕРХУ у ячеек 2..N, а не снизу: последней в строке идёт
       колонка действий без data-label, глобальное `[data-label]:last-child` до неё не
       достаёт и пунктир висел бы оторванной чертой над нижним краем карточки.
       white-space/overflow-wrap - потому что .car-col держит nowrap ради десктопной
       таблицы, и длинное название организации уезжало бы вправо за край.
       break-word, а не anywhere: anywhere разрешает браузеру считать разрыв ПОСЕРЕДИНЕ
       слова годной точкой при расчёте min-content для флекс-элемента, из-за чего значение
       рвалось прямо внутри слова («Российска|я Федераци|я», формат номера) даже когда
       слово почти помещалось целиком - break-word ломает слово только как последний
       выход, когда для него в принципе нет места на строке. */
    .car-row.rt-row > .car-col {
        align-items: center !important;
        justify-content: flex-start !important;
        min-height: 30px;
        height: auto;
        border-bottom: none !important;
        text-align: left !important;
        white-space: normal;
        overflow-wrap: break-word;
    }

    /* Формат номера и статус делят одну строку. В колоночном стеке бейдж занимал собственную
       полосу, в которой кроме него ничего не было - пустая строка в каждой карточке, и
       читалось это как случайно уехавший элемент. Пара выбрана по соседству в разметке
       (формат идёт прямо перед статусом и рендерится всегда, без v-if), приём взят у карточек
       проходной (PeopleTable/CarsTable): карточка переводится из колонки в строку с переносом,
       каждой ячейке задаётся базис 100% (во флекс-строке перенос держит БАЗИС, а не width), а
       паре-исключению - ширина по содержимому.

       Специфичность выше правил-источников и !important обязательны: responsive-tables.css
       объявляет и flex-direction, и width со своим !important, коротким селектором их не
       перебить. */
    .rt-table .car-row.rt-row {
        flex-direction: row !important;
        flex-wrap: wrap !important;
        column-gap: 8px;
        row-gap: 0;
    }

    .rt-table .car-row.rt-row > * {
        flex: 0 0 100% !important;
        width: 100% !important;
        min-width: 0 !important;
    }

    /* Номер и марка - одна строка карточки, приёмом талона проходной (см.
       responsive-tables.css rt-pass__plate/rt-pass__mark): владелец сверяет номер с
       маркой одним взглядом, а совмещённая строка освобождает высоту сверху карточки -
       раньше её съедала пара «формат номера + бейдж статуса», сам бейдж теперь стоит в
       подвале карточки (см. .status-col ниже), поэтому format-col занимает свою строку
       целиком и не нуждается в колоночной раскладке. border-top: none у brand-col - она
       делит строку с car-number-col, а не начинает свою, пунктир-разделитель ей не нужен. */
    .rt-table .car-row.rt-row > .car-number-col {
        flex: 0 1 auto !important;
        width: auto !important;
        min-width: 0;
        font-weight: 700;
    }

    .rt-table .car-row.rt-row > .brand-col {
        flex: 1 1 auto !important;
        width: auto !important;
        min-width: 0;
        color: var(--text-muted);
        border-top: none !important;
    }

    .rt-table .car-row.rt-row > .format-col::before {
        text-align: left;
    }

    /* Бейдж статуса - в подвал карточки, одной строкой с кнопками «Изменить»/«Удалить»
       (разбор второго круга замечаний владельца, #1097 w8). Раньше бейдж делил строку с
       format-col: тот нёс пунктирную границу сверху, а бейдж - нет, но оба выравнивались
       по одной Y-координате через align-self: flex-start, поэтому бейдж вставал прямо на
       чужую границу («стоит поперёк»).

       align-self: flex-start, а не center (третий круг замечаний, #1097 w9): бейдж и
       кнопки - соседние ячейки одной обёрнутой flex-строки, их высоты по контенту не
       совпадают ровно (badge ~27px против пилюль-кнопок 28px). center центрирует каждую
       ячейку НЕЗАВИСИМО в высоте строки - верхние края (а с ними border-top) расходятся
       на разницу высот, и сплошная линия подвала «переламывается» ровно после бейджа.
       flex-start прижимает обе ячейки к верхнему краю строки - border-top гарантированно
       на одной Y без зависимости от разницы высот контента. */
    .rt-table .car-row.rt-row > .status-col {
        order: 10;
        flex: 0 0 auto !important;
        width: auto !important;
        align-self: flex-start;
        margin-top: 2px;
        padding-top: 8px;
        border-top: 1px solid color-mix(in srgb, var(--border) 45%, var(--surface)) !important;
    }

    .car-row.rt-row > .car-col ~ .car-col {
        border-top: 1px dashed color-mix(in srgb, var(--border) 60%, var(--surface));
    }

    /* Номер строки скрыт, но в DOM он первый - без сброса верхний пунктир достался бы
       гос. номеру и висел бы отдельной чертой у верхнего края карточки. */
    .car-row.rt-row > .number-col + .car-col {
        border-top: none;
    }

    /* Колонка действий не несёт data-label, поэтому держала бы desktop-ширину 80px и
       обрезала бы кнопки / «Только просмотр». Подвал карточки отделён сплошной линией:
       пунктир остаётся разделителем полей, сплошная - границей данных и действий.
       !important нужен, чтобы перебить пунктир из сиблинг-правила выше: оно специфичнее
       (0,4,0 против 0,3,0) и иначе выигрывает. order/flex ставят колонку действий ПОСЛЕ
       бейджа статуса в той же строке подвала (order: 11 > 10 у .status-col), а
       justify-content прижимает кнопки к правому краю, оставляя бейдж слева. */
    .car-row.rt-row > .actions-col {
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
           родителя (.car-row.rt-row) - это пустое место без бордюра, в котором линия
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
    .car-row.rt-row .edit-btn,
    .car-row.rt-row .delete-btn {
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

    .car-row.rt-row .edit-btn::before,
    .car-row.rt-row .delete-btn::before {
        content: "";
        position: absolute;
        inset: -8px -2px;
    }

    .car-row.rt-row .edit-btn {
        color: var(--accent-text);
        border-color: var(--accent);
    }

    .car-row.rt-row .delete-btn {
        color: var(--danger-text);
        border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
    }

    .car-row.rt-row .edit-icon,
    .car-row.rt-row .delete-icon {
        display: none;
    }

    .car-row.rt-row .action-btn__label {
        position: static;
        width: auto;
        height: auto;
        margin: 0;
        overflow: visible;
        clip: auto;
        white-space: nowrap;
    }

    .car-row.rt-row > .actions-col .read-only-text {
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

    .carsview__toolbar {
        gap: 6px;
    }
}
</style>