<template>
  <AdminPageShell>
    <div class="table-constructor-container dashboard-card">
      <div class="management-header rt-header-inline">
        <h3 class="management-title">
          Таблицы системы
        </h3>
        <div class="header-controls">
          <BaseDropdown
            class="archive-dropdown"
            :model-value="showArchive ? 'archive' : 'active'"
            :options="archiveOptions"
            label-key="label"
            value-key="value"
            @update:model-value="onArchiveModeChange"
          />
          <SearchComponent
            v-model="searchQuery"
            :title="'Поиск таблиц...'"
          />
          <button
            class="add-header-button rt-btn-compact"
            aria-label="Создать таблицу"
            @click="showAddModal = true"
          >
            <span
              class="rt-btn-icon"
              aria-hidden="true"
            >+</span>
            <span class="rt-btn-label">Создать таблицу</span>
          </button>
          <RefreshButton
            :loading="refreshing"
            @refresh="refreshData"
          />
        </div>
      </div>

      <div
        v-if="selectedIds.length"
        class="bulk-bar"
        data-testid="systemtables-bulk-bar"
      >
        <span class="bulk-count">Выбрано: {{ selectedIds.length }}</span>
        <div class="bulk-actions">
          <button
            v-if="!showArchive"
            class="pill pill-danger"
            data-testid="systemtables-bulk-archive"
            @click="startBulkOperation('archive')"
          >
            В архив
          </button>
          <button
            v-else
            class="pill pill-restore"
            data-testid="systemtables-bulk-restore"
            @click="startBulkOperation('restore')"
          >
            Восстановить
          </button>
          <button
            class="pill pill-ghost bulk-clear"
            data-testid="systemtables-bulk-clear"
            @click="clearSelection"
          >
            Снять выбор
          </button>
        </div>
      </div>

      <div class="content-container">
        <!-- Левая часть - список таблиц -->
        <div
          class="table-section"
          :class="{'with-details': selectedTable}"
        >
          <div class="table-container rt-table">
            <div class="table-header rt-head-row">
              <div
                class="header-col check-col"
                @click.stop
              >
                <input
                  type="checkbox"
                  class="bulk-check"
                  :checked="allSelected"
                  :indeterminate.prop="someSelected"
                  aria-label="Выбрать все"
                  data-testid="systemtables-select-all"
                  @change="toggleSelectAll"
                >
              </div>
              <div
                class="header-col id-col"
                @click="sortBy('id')"
              >
                <p :class="{ 'active-sort': sortField === 'id' }">
                  ID
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
                @click="sortBy('name')"
              >
                <p :class="{ 'active-sort': sortField === 'name' }">
                  Наименование
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'name',
                    'desc': sortField === 'name' && sortDirection === 'desc'
                  }"
                />
              </div>
              <div
                class="header-col type-col"
                @click="sortBy('type')"
              >
                <p :class="{ 'active-sort': sortField === 'type' }">
                  Тип
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'type',
                    'desc': sortField === 'type' && sortDirection === 'desc'
                  }"
                />
              </div>
              <div class="header-col status-col">
                <p>Статус</p>
              </div>
            </div>

            <div class="table-body">
              <div
                v-for="(table, index) in sortedTables"
                :key="table.table.id"
                class="table-row rt-row"
                :class="{
                  'selected': selectedTable && selectedTable.table.id === table.table.id,
                  'inactive': !table.table.is_active,
                }"
                @click="selectTable(table)"
              >
                <div
                  class="table-col check-col"
                  @click.stop
                >
                  <input
                    type="checkbox"
                    class="bulk-check"
                    :checked="isSelected(table.table.id)"
                    :aria-label="`Выбрать ${table.table.display_name}`"
                    data-testid="systemtables-row-check"
                    @click="onRowCheck(table.table, index, $event)"
                  >
                </div>
                <div
                  class="table-col id-col"
                  data-label="ID"
                >
                  <span class="cell-content id-value">{{ table.table.id }}</span>
                </div>
                <div
                  class="table-col name-col"
                  data-label="Наименование"
                >
                  <span
                    class="truncate-text"
                    :title="table.table.display_name"
                  >
                    {{ table.table.display_name }}
                    <span
                      v-if="!table.table.is_active"
                      class="inactive-badge"
                    >(архив)</span>
                  </span>
                </div>
                <div
                  class="table-col type-col"
                  data-label="Тип"
                >
                  <span
                    class="type-badge"
                    :class="table.table.table_type"
                  >
                    {{ getTableTypeLabel(table.table.table_type) }}
                  </span>
                </div>
                <div
                  class="table-col status-col"
                  data-label="Статус"
                >
                  <span
                    class="status-badge"
                    :class="getTableStatusClass(table)"
                  >
                    {{ getTableStatusText(table) }}
                  </span>
                </div>
              </div>
              <div
                v-if="!sortedTables.length"
                class="no-results"
              >
                {{ emptyText }}
              </div>
            </div>

            <div class="table-footer">
              <span class="items-count">
                {{ showArchive ? 'В архиве' : 'Всего таблиц' }}: {{ filteredTables.length }}
              </span>
            </div>
          </div>
        </div>

        <!-- Правая часть - детали таблицы -->
        <div
          v-if="selectedTable"
          class="details-section"
        >
          <div
            v-if="selectedTable.table.is_active"
            class="details-tabs"
          >
            <div class="details-tabs__row">
              <button
                class="tab-btn"
                :class="{ 'active': activeTab === 'main' }"
                @click="switchTab('main')"
              >
                Основное
              </button>
              <button
                class="tab-btn"
                :class="{ 'active': activeTab === 'schedule' }"
                @click="switchTab('schedule')"
              >
                Расписание
              </button>
              <button
                class="tab-btn"
                :class="{ 'active': activeTab === 'warnings' }"
                @click="switchTab('warnings')"
              >
                Предупреждения
              </button>
              <button
                class="tab-btn"
                :class="{ 'active': activeTab === 'location' }"
                @click="switchTab('location')"
              >
                Местоположение
              </button>
              <button
                class="tab-btn"
                :class="{ 'active': activeTab === 'columns' }"
                @click="switchTab('columns')"
              >
                Колонки
              </button>
              <button
                class="tab-btn"
                :class="{ 'active': activeTab === 'appearance' }"
                @click="switchTab('appearance')"
              >
                Оформление
              </button>
            </div>
            <div
              v-if="selectedTable.table.show_fact_table"
              class="details-tabs__row details-tabs__row--fact"
            >
              <span class="details-tabs__group-label">По факту:</span>
              <button
                class="tab-btn"
                :class="{ 'active': activeTab === 'fact-columns' }"
                @click="switchTab('fact-columns')"
              >
                Колонки
              </button>
              <button
                class="tab-btn"
                :class="{ 'active': activeTab === 'fact-appearance' }"
                @click="switchTab('fact-appearance')"
              >
                Оформление
              </button>
            </div>
          </div>

          <!-- Вкладка Основное -->
          <div
            v-if="activeTab === 'main'"
            class="tab-content"
          >
            <div class="details-header">
              <div class="details-title-wrapper">
                <div class="table-info-title">
                  <h3 class="details-title">
                    {{ selectedTable.table.display_name }}
                  </h3>
                  <span
                    class="table-type-badge"
                    :class="selectedTable.table.table_type"
                  >
                    {{ getTableTypeLabel(selectedTable.table.table_type) }}
                  </span>
                </div>
                <div class="table-info-row">
                  <span
                    class="current-status-badge"
                    :class="getTableCurrentStatusClass(selectedTable)"
                  >
                    {{ getTableCurrentStatusText(selectedTable) }}
                  </span>
                  <span class="system-name">{{ selectedTable.table.name }}</span>
                </div>
              </div>
              <div class="details-header-actions">
                <span
                  v-if="!selectedTable.table.is_active"
                  class="archive-badge"
                >В архиве</span>
                <button
                  class="action-btn history-btn"
                  @click="historyTable = selectedTable.table"
                >
                  История
                </button>
                <button
                  v-if="!selectedTable.table.is_active && can(`table.${selectedTable.table.name}.versions`)"
                  class="action-btn versions-btn"
                  data-testid="table-versions-btn"
                  @click="openVersions"
                >
                  Версии
                </button>
                <button
                  v-if="selectedTable.table.is_active"
                  class="action-btn view-btn"
                  @click="openTable"
                >
                  Открыть
                </button>
                <button
                  v-if="!selectedTable.table.is_active"
                  class="action-btn restore-btn"
                  @click="restoreTable(selectedTable)"
                >
                  Восстановить
                </button>
                <button
                  v-if="selectedTable.table.is_active"
                  class="delete-icon-btn"
                  title="Удалить таблицу"
                  @click="confirmDeleteTable(selectedTable)"
                >
                  <AppIcon
                    name="delete"
                    class="delete-icon"
                  />
                </button>
              </div>
            </div>
          
            <div class="details-body">
              <div class="compact-form">
                <div class="form-row">
                  <div class="form-group compact">
                    <label class="detail-label">Наименование таблицы:</label>
                    <input 
                      v-model="selectedTable.table.display_name" 
                      class="lk-input"
                      placeholder="Название таблицы"
                      autocomplete="off"
                      @change="updateTableField('display_name')"
                    >
                  </div>
                  <div class="form-group compact">
                    <label class="detail-label">Тип таблицы:</label>
                    <div class="custom-select">
                      <div
                        class="select-header"
                        @click="toggleTableTypeDropdown"
                      >
                        <span class="select-value">{{ getTableTypeLabel(selectedTable.table.table_type) }}</span>
                        <AppIcon
                          name="arrow"
                          class="select-arrow"
                          :class="{ rotated: tableTypeDropdownOpen }"
                        />
                      </div>
                      <transition name="dropdown-fade">
                        <div
                          v-if="tableTypeDropdownOpen"
                          class="select-dropdown"
                        >
                          <div 
                            class="select-option"
                            :class="{ active: selectedTable.table.table_type === 'cars' }"
                            @click="selectTableType('cars')"
                          >
                            Машины
                          </div>
                          <div 
                            class="select-option"
                            :class="{ active: selectedTable.table.table_type === 'people' }"
                            @click="selectTableType('people')"
                          >
                            Люди
                          </div>
                        </div>
                      </transition>
                    </div>
                  </div>
                </div>

                <!-- Статус в виде кнопок -->
                <div class="form-group">
                  <label class="detail-label">Статус:</label>
                  <p class="field-hint">
                    "Не активно" / "На обслуживании" - таблицу нельзя выбрать в
                    новой заявке. Пользователь видит причину.
                  </p>
                  <div class="status-toggle">
                    <button 
                      class="status-btn" 
                      :class="{ 'active': selectedTable.table.status === 'active' }"
                      @click="setTableStatus('active')"
                    >
                      Активно
                    </button>
                    <button 
                      class="status-btn" 
                      :class="{ 'active': selectedTable.table.status === 'inactive' }"
                      @click="setTableStatus('inactive')"
                    >
                      Не активно
                    </button>
                    <button 
                      class="status-btn" 
                      :class="{ 'active': selectedTable.table.status === 'maintenance' }"
                      @click="setTableStatus('maintenance')"
                    >
                      На обслуживании
                    </button>
                  </div>
                </div>

                <!-- Комментарий к статусу (только для неактивных) -->
                <div
                  v-if="selectedTable.table.status !== 'active'"
                  class="form-group"
                >
                  <label class="detail-label">Причина:</label>
                  <textarea 
                    v-model="selectedTable.table.status_comment" 
                    class="form-textarea"
                    placeholder="Укажите причину"
                    rows="2"
                    @change="updateTableField('status_comment')"
                  />
                </div>

                <div class="settings-section">
                  <label class="section-label">Настройки отображения:</label>

                  <div class="checkbox-group">
                    <label class="checkbox-label">
                      <input
                        v-model="selectedTable.table.show_fact_table"
                        type="checkbox"
                        class="checkbox-input"
                        @change="updateTableField('show_fact_table')"
                      >
                      <span class="checkbox-text">Отображать таблицу "по факту"</span>
                    </label>
                    <p class="field-hint">
                      На странице с основной таблицей отображается таблица
                      "по факту". В ней отображаются люди/машины, данные которых
                      заранее не известны.
                    </p>
                  </div>

                  <div
                    v-if="selectedTable.table.show_fact_table"
                    class="hint-section"
                  >
                    <div class="section-header-with-actions">
                      <label class="detail-label">Подсказка для таблицы "по факту":</label>
                      <div
                        v-if="hintHasChanges"
                        class="editor-actions"
                      >
                        <button
                          class="compact-btn cancel-btn"
                          @click="cancelHintEdit"
                        >
                          Отмена
                        </button>
                        <button
                          class="compact-btn save-btn"
                          @click="saveHint"
                        >
                          Сохранить
                        </button>
                      </div>
                    </div>
                    <TextConstructor
                      ref="hintConstructor"
                      v-model="selectedTable.table.fact_table_hint"
                      :placeholder="getDefaultHint(selectedTable.table.table_type)"
                      rows="4"
                    />
                  </div>
                </div>

                <div class="instruction-section">
                  <p class="field-hint">
                    Видна в шапке таблицы по клику на "Инструкция". Здесь пишут
                    правила прохода/проезда для охранника на этой точке.
                  </p>
                  <div class="section-header-with-actions">
                    <label class="detail-label">Инструкция к таблице:</label>
                    <div
                      v-if="instructionHasChanges"
                      class="editor-actions"
                    >
                      <button
                        class="compact-btn cancel-btn"
                        @click="cancelInstructionEdit"
                      >
                        Отмена
                      </button>
                      <button
                        class="compact-btn save-btn"
                        @click="saveInstruction"
                      >
                        Сохранить
                      </button>
                    </div>
                  </div>
                  <TextConstructor
                    ref="instructionConstructor"
                    v-model="selectedTable.table.instruction"
                    placeholder="Введите инструкцию для таблицы..."
                    rows="4"
                  />
                </div>

                <!-- Привязки к организациям/компаниям + «Отвязать всё» (#1379) -->
                <div class="usage-section usage-section--inline">
                  <div class="usage-header">
                    <div class="usage-header__text">
                      <h4 class="section-title">
                        Привязано к организациям и компаниям
                      </h4>
                      <p class="field-hint">
                        Пока таблица привязана хотя бы к одной организации или компании,
                        её нельзя удалить. Отвяжите все, чтобы освободить таблицу.
                      </p>
                    </div>
                    <button
                      v-if="canDetachTable && !usageLoading && !usageError && usageHasBindings"
                      class="action-btn detach-all-btn"
                      :disabled="detaching || detachingOne"
                      @click="confirmDetachAll"
                    >
                      {{ detaching ? 'Отвязываем...' : 'Отвязать всё' }}
                    </button>
                  </div>

                  <div
                    v-if="usageLoading"
                    class="usage-state"
                  >
                    Загрузка привязок...
                  </div>
                  <div
                    v-else-if="usageError"
                    class="usage-state usage-state--error"
                  >
                    {{ usageError }}
                  </div>
                  <template v-else>
                    <div class="usage-group">
                      <div class="usage-group__title">
                        Организации: {{ usage.organizations.length }}
                      </div>
                      <ul
                        v-if="usage.organizations.length"
                        class="usage-list"
                      >
                        <li
                          v-for="org in usage.organizations"
                          :key="'org-' + org.id"
                          class="usage-item"
                        >
                          <span class="usage-item__name">{{ org.name }}</span>
                          <span
                            v-if="!org.is_active"
                            class="usage-item__archived"
                          >(архив)</span>
                          <button
                            v-if="canDetachTable"
                            class="usage-item__detach"
                            data-hint="Отвязать"
                            :disabled="detaching || detachingOne"
                            @click="confirmDetachOne('organization', org)"
                          >
                            &times;
                          </button>
                        </li>
                      </ul>
                      <p
                        v-else
                        class="usage-empty"
                      >
                        Нет привязанных организаций
                      </p>
                    </div>

                    <div class="usage-group">
                      <div class="usage-group__title">
                        Компании: {{ usage.companies.length }}
                      </div>
                      <ul
                        v-if="usage.companies.length"
                        class="usage-list"
                      >
                        <li
                          v-for="comp in usage.companies"
                          :key="'comp-' + comp.id"
                          class="usage-item"
                        >
                          <span class="usage-item__name">{{ comp.name }}</span>
                          <span
                            v-if="!comp.is_active"
                            class="usage-item__archived"
                          >(архив)</span>
                          <button
                            v-if="canDetachTable"
                            class="usage-item__detach"
                            data-hint="Отвязать"
                            :disabled="detaching || detachingOne"
                            @click="confirmDetachOne('company', comp)"
                          >
                            &times;
                          </button>
                        </li>
                      </ul>
                      <p
                        v-else
                        class="usage-empty"
                      >
                        Нет привязанных компаний
                      </p>
                    </div>
                  </template>
                </div>
              </div>
            </div>
          </div>

          <!-- Вкладка Расписание -->
          <div
            v-if="activeTab === 'schedule'"
            class="tab-content"
          >
            <WorkScheduleTab
              :resource-url="'/system-tables/' + selectedTable.table.id"
              :time-slots="selectedTable.time_slots"
              @update="refreshSelectedTable"
            />
          </div>

          <!-- Вкладка Предупреждения -->
          <div
            v-if="activeTab === 'warnings'"
            class="tab-content"
          >
            <div class="warnings-section">
              <h4 class="section-title">
                Свободное предупреждение
              </h4>
              <p class="field-hint">
                Показывается заявителю всегда при добавлении машины/человека
                в эту таблицу проходной.
              </p>
              <textarea
                v-model="selectedTable.table.warning"
                class="form-textarea"
                placeholder="Например: проход только по предварительной записи"
                rows="2"
                @change="updateTableField('warning')"
              />
            </div>

            <div class="warnings-section">
              <WarningWindowsEditor
                :resource-url="'/system-tables/' + selectedTable.table.id"
                :windows="selectedTable.warning_windows || []"
                @update="refreshSelectedTable"
              />
            </div>
          </div>

          <!-- Вкладка Местоположение -->
          <div
            v-if="activeTab === 'location'"
            class="tab-content"
          >
            <div class="location-section">
              <h4 class="section-title">
                Описание местоположения
              </h4>
              <textarea 
                v-model="selectedTable.table.location_description" 
                class="form-textarea"
                placeholder="Введите описание местоположения..."
                rows="3"
                @change="updateTableField('location_description')"
              />
            </div>

            <div class="location-section">
              <h4 class="section-title">
                Ссылка на карту
              </h4>
              <div class="map-link-group">
                <input 
                  v-model="selectedTable.table.map_link" 
                  class="form-input"
                  placeholder="https://maps.google.com/..."
                  autocomplete="off"
                  @change="updateTableField('map_link')"
                >
                <a 
                  v-if="selectedTable.table.map_link" 
                  :href="selectedTable.table.map_link" 
                  target="_blank" 
                  class="map-link-btn"
                >
                  Открыть карту
                </a>
              </div>
            </div>

            <TableConstructorPhotoSection
              :table-id="selectedTable.table.id"
              :photos="selectedTable.photos || []"
              @photos-changed="refreshSelectedTable"
            />
          </div>

          <!-- Вкладка Колонки (#345) -->
          <div
            v-if="activeTab === 'columns'"
            class="tab-content"
          >
            <SystemTableColumnsTab
              :table-id="selectedTable.table.id"
              :table-type="selectedTable.table.table_type"
              :fields="selectedTable.fields || []"
              @update="refreshSelectedTable"
            />
          </div>

          <!-- Вкладка Оформление (#345 фазы 1D+1E) -->
          <div
            v-if="activeTab === 'appearance'"
            class="tab-content"
          >
            <SystemTableAppearanceTab
              :table-id="selectedTable.table.id"
              :table="selectedTable.table"
              :table-type="selectedTable.table.table_type"
              :fields="selectedTable.fields || []"
              @update="refreshSelectedTable"
            />
          </div>

          <!-- Колонки FactTable (#345) -->
          <div
            v-if="activeTab === 'fact-columns' && selectedTable.table.show_fact_table"
            class="tab-content"
          >
            <SystemTableColumnsTab
              :table-id="selectedTable.table.id"
              :table-type="selectedTable.table.table_type"
              :fields="selectedTable.fact_fields || []"
              variant="fact"
              @update="refreshSelectedTable"
            />
          </div>

          <!-- Оформление FactTable (#345) -->
          <div
            v-if="activeTab === 'fact-appearance' && selectedTable.table.show_fact_table"
            class="tab-content"
          >
            <SystemTableAppearanceTab
              :table-id="selectedTable.table.id"
              :table="selectedTable.table"
              :table-type="selectedTable.table.table_type"
              :fields="selectedTable.fact_fields || []"
              variant="fact"
              @update="refreshSelectedTable"
            />
          </div>
        </div>

        <div
          v-else
          class="no-selection-message"
        >
          <p>Выберите таблицу для просмотра и редактирования</p>
        </div>
      </div>


      <!-- Модальное окно создания таблицы -->
      <TableConstructorCreateModal
        :show="showAddModal"
        @created="onTableCreated"
        @close="showAddModal = false"
      />

      <!-- Модалка истории таблицы -->
      <SystemTableHistoryModal
        v-if="historyTable"
        :table="historyTable"
        @close="historyTable = null"
      />

      <!-- Подтверждение архивации таблицы -->
      <ConfirmationModal
        :show="!!deleteConfirmTable"
        title="Архивация таблицы"
        :message="deleteConfirmTable ? `Архивировать таблицу «${deleteConfirmTable.table.display_name}»? Её можно будет восстановить из архива.` : ''"
        confirm-text="В архив"
        cancel-text="Отмена"
        :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
        @confirm="performDeleteTable"
        @cancel="deleteConfirmTable = null"
      />

      <!-- Подтверждение групповой архивации/восстановления -->
      <ConfirmationModal
        :show="bulkConfirmVisible"
        :title="bulkConfirmTitle"
        :message="bulkConfirmMessage"
        :confirm-text="bulkConfirmText"
        cancel-text="Отмена"
        :confirm-button-style="bulkConfirmButtonStyle"
        @confirm="applyBulkArchiveRestore"
        @cancel="cancelBulkConfirm"
      />

      <!-- Подтверждение отвязки от всех организаций и компаний -->
      <ConfirmationModal
        :show="detachConfirmVisible"
        title="Отвязать все организации и компании"
        :message="detachConfirmMessage"
        confirm-text="Отвязать всё"
        cancel-text="Отмена"
        :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
        @confirm="performDetachAll"
        @cancel="detachConfirmVisible = false"
      />

      <!-- Подтверждение отвязки одной организации/компании -->
      <ConfirmationModal
        :show="!!detachOneTarget"
        title="Отвязать привязку"
        :message="detachOneConfirmMessage"
        confirm-text="Отвязать"
        cancel-text="Отмена"
        :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
        @confirm="performDetachOne"
        @cancel="detachOneTarget = null"
      />
    </div>
  </AdminPageShell>
</template>

<script>
import { apiRequest } from '@/api/client'
import { bulkArchiveSystemTables, bulkRestoreSystemTables, getSystemTableUsage, detachAllSystemTable, detachOrganizationFromSystemTable, detachCompanyFromSystemTable } from '@/api/system-tables'
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants'
import { confirmIfAnyDirty } from '@/utils/dirtyTracker';
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';
import RefreshButton from './RefreshButton.vue';
import SearchComponent from './SearchComponent.vue';
import TextConstructor from './TextConstructor.vue';
import WorkScheduleTab from './WorkScheduleTab.vue';
import WarningWindowsEditor from './WarningWindowsEditor.vue';
import SystemTableColumnsTab from './SystemTableColumnsTab.vue';
import SystemTableAppearanceTab from './SystemTableAppearanceTab.vue';
import TableConstructorCreateModal from './TableConstructorCreateModal.vue';
import TableConstructorPhotoSection from './TableConstructorPhotoSection.vue';
import SystemTableHistoryModal from './SystemTableHistoryModal.vue';
import ConfirmationModal from './ConfirmationModal.vue';
import BaseDropdown from './ui/BaseDropdown.vue';
import AdminPageShell from '@/views/admin/AdminPageShell.vue';
import AppIcon from '@/components/icons/AppIcon.vue';
import { openFromSearchLink } from '@/mixins/openFromSearchLink'

export default {
  name: 'TableConstructor',
  mixins: [openFromSearchLink((vm) => vm.tables, 'selectTable', (row) => row?.table?.id, 'searchQuery')],
  components: {
    SearchComponent,
    RefreshButton,
    TextConstructor,
    WorkScheduleTab,
    WarningWindowsEditor,
    SystemTableColumnsTab,
    SystemTableAppearanceTab,
    TableConstructorCreateModal,
    TableConstructorPhotoSection,
    SystemTableHistoryModal,
    ConfirmationModal,
    BaseDropdown,
    AdminPageShell,
    AppIcon,
  },
  setup() {
    const permissionsStore = usePermissionsStore();
    return { permissionsStore };
  },
  data() {
    return {
      refreshing: false,
      tables: [],
      showAddModal: false,
      selectedTable: null,
      sortField: null,
      sortDirection: 'asc',
      activeTab: 'main',
      originalHint: '',
      originalInstruction: '',
      tableTypeDropdownOpen: false,
      showArchive: false,
      historyTable: null,
      deleteConfirmTable: null,
      archiveOptions: [
        { label: 'Активные', value: 'active' },
        { label: 'Архив', value: 'archive' },
      ],
      // Групповой выбор (по id). lastSelectedId - якорь shift-диапазона.
      selectedIds: [],
      lastSelectedId: null,
      pendingBulkOp: null,
      bulkConfirmVisible: false,
      bulkSubmitting: false,
      // Привязки (блок на вкладке «Основное»): организации/компании, держащие таблицу.
      usage: { organizations: [], companies: [] },
      usageLoading: false,
      usageError: '',
      usageSeq: 0,
      detaching: false,
      detachConfirmVisible: false,
      // Точечная отвязка: { kind: 'organization'|'company', id, name } | null.
      detachOneTarget: null,
      detachingOne: false,
    };
  },
  computed: {
    emptyText() {
      if (this.searchQuery.trim()) return 'Ничего не найдено по запросу';
      return this.showArchive ? 'В архиве пусто' : 'Таблиц пока нет';
    },
    filteredTables() {
      const variants = buildSearchVariants(this.searchQuery);
      if (!variants.length) return this.tables;
      return this.tables.filter(table => matchesSearch(
        `${table.table.display_name} ${table.table.name} ${table.table.id}`,
        variants,
      ));
    },
    sortedTables() {
      const tables = [...this.filteredTables];
      
      if (!this.sortField) {
        return tables.sort((a, b) => a.table.display_name.localeCompare(b.table.display_name));
      }
      
      return tables.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'id':
            valueA = a.table.id;
            valueB = b.table.id;
            break;
          case 'name':
            valueA = a.table.display_name;
            valueB = b.table.display_name;
            break;
          case 'type':
            valueA = a.table.table_type;
            valueB = b.table.table_type;
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
    hintHasChanges() {
      return this.selectedTable && this.selectedTable.table.fact_table_hint !== this.originalHint;
    },
    instructionHasChanges() {
      return this.selectedTable && this.selectedTable.table.instruction !== this.originalInstruction;
    },
    allSelected() {
      return this.sortedTables.length > 0 && this.selectedIds.length === this.sortedTables.length;
    },
    someSelected() {
      return this.selectedIds.length > 0 && !this.allSelected;
    },
    bulkConfirmTitle() {
      return this.pendingBulkOp === 'restore' ? 'Восстановление таблиц' : 'Архивация таблиц';
    },
    bulkConfirmMessage() {
      const n = this.selectedIds.length;
      return this.pendingBulkOp === 'restore'
        ? `Восстановить выбранные таблицы (${n})?`
        : `Архивировать выбранные таблицы (${n})? Их можно будет восстановить из архива.`;
    },
    bulkConfirmText() {
      return this.pendingBulkOp === 'restore' ? 'Восстановить' : 'В архив';
    },
    bulkConfirmButtonStyle() {
      return this.pendingBulkOp === 'restore'
        ? { background: '#10b981', borderColor: '#10b981' }
        : { background: '#c62828', borderColor: '#c62828' };
    },
    usageHasBindings() {
      return this.usage.organizations.length > 0 || this.usage.companies.length > 0;
    },
    // Зеркалит BE-гейт detach-all (requireAdmin = RequirePermissionV2 page.admin):
    // права page.admin.tables_constructor (открывающего экран) недостаточно.
    canDetachTable() {
      return this.can('page.admin');
    },
    detachConfirmMessage() {
      if (!this.selectedTable) return '';
      const o = this.usage.organizations.length;
      const c = this.usage.companies.length;
      return `Отвязать таблицу «${this.selectedTable.table.display_name}» от всех организаций (${o}) и компаний (${c})? Это освободит таблицу, чтобы её можно было удалить.`;
    },
    detachOneConfirmMessage() {
      if (!this.detachOneTarget || !this.selectedTable) return '';
      const kind = this.detachOneTarget.kind === 'organization' ? 'организацию' : 'компанию';
      return `Отвязать ${kind} «${this.detachOneTarget.name}» от таблицы «${this.selectedTable.table.display_name}»?`;
    },
  },
  watch: {
    // Если активна вкладка фактовой таблицы, а пользователь снял галочку
    // show_fact_table - возвращаем на главную, чтобы не висел пустой контент.
    'selectedTable.table.show_fact_table'(val) {
      if (!val && (this.activeTab === 'fact-columns' || this.activeTab === 'fact-appearance')) {
        this.activeTab = 'main';
      }
    },
    selectedTable(newVal) {
      // При переключении таблицы сбрасываем тикеры fact-вкладок, если на новой
      // таблице фактовая часть выключена.
      if (newVal && !newVal.table?.show_fact_table) {
        if (this.activeTab === 'fact-columns' || this.activeTab === 'fact-appearance') {
          this.activeTab = 'main';
        }
      }
    },
    // Смена фильтра/поиска/режима меняет видимый список - убираем из выбора
    // строки, которых больше не видно (реактивно, не только после refresh).
    sortedTables() {
      this.pruneSelection();
    },
    // Привязки показываются на вкладке «Основное» - грузим при смене таблицы
    // (id меняется), а не по правке полей той же таблицы (id тот же).
    'selectedTable.table.id'(id) {
      if (id) this.loadUsage();
    },
  },
  async mounted() {
    // Один `open` обслуживает два перехода: ИМЯ архивной таблицы со страницы версий
    // (тогда открываем архив) и числовой id из поиска - тот разбирает примесь.
    const openName = Number.isNaN(Number(this.$route?.query?.open)) ? this.$route?.query?.open : null;
    if (openName) this.showArchive = true;
    await this.refreshData();
    if (openName) {
      const target = this.tables.find((t) => t.table.name === openName);
      if (target) this.selectTable(target);
      this.$router.replace({ query: {} });
    }

    // Закрываем dropdown при клике вне них
    document.addEventListener('click', (e) => {
      if (!this.$el.contains(e.target)) {
        this.tableTypeDropdownOpen = false;
      }
    });
  },
  methods: {

    /**
     * Переключение вкладки с защитой: если на текущей вкладке есть pending
     * правки - сначала спросить подтверждение. confirmIfAnyDirty опрашивает
     * всех зарегистрированных через registerDirtyTracker.
     */
    async switchTab(tab) {
      if (this.activeTab === tab) return;
      if (!(await confirmIfAnyDirty())) return;
      this.activeTab = tab;
    },

    async refreshData() {
      this.refreshing = true;
      try {
        await this.fetchTables();
      } finally {
        this.refreshing = false;
      }
    },
    
    async fetchTables() {
      try {
        const url = this.showArchive
          ? '/system-tables?include_archived=true'
          : '/system-tables';
        const response = await apiRequest(url, {
        });
        if (response.ok) {
          const data = await response.json();
          this.tables = data;
          this.openFromSearchLink();
        }
      } catch (error) {
        console.error("Error fetching system tables:", error);
        useDeletionsStore().notify({ prefix: 'Ошибка при ', bold: 'загрузке таблиц', type: 'error' });
      }
    },

    async onArchiveModeChange(value) {
      const wantArchive = value === 'archive';
      if (wantArchive === this.showArchive) return;
      if (!(await confirmIfAnyDirty())) return;
      this.showArchive = wantArchive;
      this.selectedTable = null;
      this.activeTab = 'main';
      this.clearSelection();
      await this.refreshData();
    },

    /**
     * Достаёт человекочитаемое сообщение об ошибке из ответа apiRequest.
     * wrapJsonUnwrap на !success кладёт текст ошибки бэка в поле message (в самом
     * envelope ключ - error), поэтому читаем response.json().message, а не сырое
     * тело: иначе в уведомление попадает JSON целиком (скобки, имена полей).
     * @param {Response} response
     * @returns {Promise<string>}
     */
    async requestErrorMessage(response) {
      try {
        const body = await response.json();
        return (body && body.message) || 'неизвестная ошибка';
      } catch {
        return 'неизвестная ошибка';
      }
    },

    async restoreTable(tableObj) {
      try {
        const response = await apiRequest(`/system-tables/${tableObj.table.id}/restore`, {
          method: 'POST',
        });
        if (!response.ok) {
          const message = await this.requestErrorMessage(response);
          useDeletionsStore().notify({ prefix: 'Ошибка восстановления: ', bold: message, type: 'error' });
          return;
        }
        const restoredName = tableObj.table.display_name;
        // После restore таблица уходит из архивного списка - убираем её локально.
        this.tables = this.tables.filter(t => t.table.id !== tableObj.table.id);
        this.selectedTable = null;
        useDeletionsStore().notify({ prefix: 'Таблица ', bold: restoredName, suffix: ' восстановлена' });
      } catch (error) {
        console.error('Error restoring table:', error);
        useDeletionsStore().notify({ prefix: 'Ошибка ', bold: 'сети', type: 'error' });
      }
    },
    
    async refreshSelectedTable() {
      if (!this.selectedTable) return;

      try {
        const response = await apiRequest(`/system-tables/${this.selectedTable.table.id}`, {
        });
        if (response.ok) {
          const data = await response.json();
          
          // Исправляем URL фотографий
          if (data.photos) {
            data.photos = data.photos.map(photo => ({
              ...photo,
              photo_url: photo.photo_url
            }));
          }
          
          this.selectedTable = data;
          this.originalHint = data.table.fact_table_hint || '';
          this.originalInstruction = data.table.instruction || '';
          
          // Обновляем в общем списке
          const index = this.tables.findIndex(t => t.table.id === data.table.id);
          if (index !== -1) {
            this.tables[index] = data;
          }
        }
      } catch (error) {
        console.error("Error refreshing table:", error);
      }
    },
    
    async onTableCreated(result) {
      this.showAddModal = false;
      // Новая таблица всегда активна - если юзер был в архиве, переключаем на Активные,
      // чтобы он увидел созданную (иначе она "пропала бы" из списка).
      if (this.showArchive) {
        this.showArchive = false;
      }
      await this.refreshData();
      const newTable = this.tables.find(t => t.table.id === result.id);
      if (newTable) {
        this.selectTable(newTable);
      }
      useDeletionsStore().notify({
        bold: result.display_name || 'Таблица',
        suffix: result.display_name ? ' создана' : '',
      });
    },

    async updateTable(field) {
      if (!this.selectedTable) return;
      
      const updateData = {};
      
      switch (field) {
        case 'display_name':
          updateData.display_name = this.selectedTable.table.display_name;
          break;
        case 'table_type':
          updateData.table_type = this.selectedTable.table.table_type;
          break;
        case 'show_fact_table':
          updateData.show_fact_table = this.selectedTable.table.show_fact_table;
          break;
        case 'fact_table_hint':
          updateData.fact_table_hint = this.selectedTable.table.fact_table_hint;
          break;
        case 'instruction':
          updateData.instruction = this.selectedTable.table.instruction;
          break;
        case 'map_link':
          updateData.map_link = this.selectedTable.table.map_link;
          break;
        case 'status':
          updateData.status = this.selectedTable.table.status;
          break;
        case 'status_comment':
          updateData.status_comment = this.selectedTable.table.status_comment;
          break;
        case 'location_description':
          updateData.location_description = this.selectedTable.table.location_description;
          break;
        case 'warning':
          updateData.warning = this.selectedTable.table.warning;
          break;
      }
      
      try {
        const response = await apiRequest(`/system-tables/${this.selectedTable.table.id}`, {
          method: "PUT",
          body: JSON.stringify(updateData),
        });
        
        if (response.ok) {
          if (field === 'fact_table_hint') {
            this.originalHint = this.selectedTable.table.fact_table_hint || '';
          } else if (field === 'instruction') {
            this.originalInstruction = this.selectedTable.table.instruction || '';
          }
          const fieldPhrases = {
            display_name: { bold: 'Наименование таблицы', suffix: ' изменено' },
            table_type: { bold: 'Тип таблицы', suffix: ' изменён' },
            show_fact_table: { bold: 'Отображение таблицы по факту', suffix: ' изменено' },
            fact_table_hint: { bold: 'Подсказка', suffix: ' изменена' },
            instruction: { bold: 'Инструкция', suffix: ' изменена' },
            map_link: { bold: 'Ссылка на карту', suffix: ' изменена' },
            status: { bold: 'Статус', suffix: ' изменён' },
            status_comment: { bold: 'Комментарий статуса', suffix: ' изменён' },
            location_description: { bold: 'Описание местоположения', suffix: ' изменено' },
            warning: { bold: 'Предупреждение', suffix: ' изменено' },
          };
          const phrase = fieldPhrases[field] || { bold: 'Изменения', suffix: ' сохранены' };
          useDeletionsStore().notify(phrase);
          await this.refreshSelectedTable();
        } else {
          const message = await this.requestErrorMessage(response);
          useDeletionsStore().notify({ prefix: 'Не удалось обновить: ', bold: message, type: 'error' });
        }
      } catch (error) {
        console.error("Error updating table:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось подключиться: ', bold: 'нет связи с сервером', type: 'error' });
      }
    },
    
    updateTableField(field) {
      this.updateTable(field);
    },
    
    setTableStatus(status) {
      if (!this.selectedTable) return;
      this.selectedTable.table.status = status;
      if (status === 'active') {
        this.selectedTable.table.status_comment = null;
      }
      this.updateTable('status');
    },
    
    confirmDeleteTable(table) {
      // Открываем ConfirmationModal вместо window.confirm. Реальный delete делает performDeleteTable.
      this.deleteConfirmTable = table;
    },

    async performDeleteTable() {
      const table = this.deleteConfirmTable;
      this.deleteConfirmTable = null;
      if (!table) return;
      try {
        const response = await apiRequest(`/system-tables/${table.table.id}`, {
          method: "DELETE",
        });
        if (response.ok) {
          const archivedName = table.table.display_name;
          this.selectedTable = null;
          this.activeTab = 'main';
          await this.refreshData();
          useDeletionsStore().notify({ prefix: 'Таблица ', bold: archivedName, suffix: ' архивирована' });
        } else {
          const message = await this.requestErrorMessage(response);
          useDeletionsStore().notify({ prefix: 'Не удалось архивировать: ', bold: message, type: 'error' });
        }
      } catch (error) {
        console.error('Error deleting system table:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось подключиться: ', bold: 'нет связи с сервером', type: 'error' });
      }
    },
    
    async selectTable(table) {
      // Защита от потери: если текущая вкладка settings dirty - спросить.
      if (this.selectedTable && this.selectedTable.table.id !== table.table.id) {
        if (!(await confirmIfAnyDirty())) return;
      }
      this.selectedTable = JSON.parse(JSON.stringify(table));
      this.originalHint = table.table.fact_table_hint || '';
      this.originalInstruction = table.table.instruction || '';
      this.activeTab = 'main';
    },
    
    openTable() {
      if (this.selectedTable) {
        this.$router.push(`/table/${this.selectedTable.table.name}`);
      }
    },

    // Гейтинг кнопки «Версии» тем же ключом, что роут /table/:name/versions и
    // кнопка входа на странице таблицы (#980) - иначе кнопка видна, а роут отбивает.
    can(key) {
      return this.permissionsStore.hasPermission(key);
    },

    // seq-guard: быстрое переключение таблиц не даст устаревшему ответу затереть
    // актуальные привязки (last-resolve-wins иначе показал бы чужую таблицу).
    async loadUsage() {
      if (!this.selectedTable) return;
      const seq = ++this.usageSeq;
      this.usageLoading = true;
      this.usageError = '';
      // Гасим привязки предыдущей таблицы сразу: пока грузятся новые, кнопка
      // «Отвязать всё» и текст подтверждения не должны показывать чужие цифры.
      this.usage = { organizations: [], companies: [] };
      try {
        const data = await getSystemTableUsage(this.selectedTable.table.id);
        if (seq !== this.usageSeq) return;
        this.usage = {
          organizations: data?.organizations || [],
          companies: data?.companies || [],
        };
      } catch (err) {
        if (seq !== this.usageSeq) return;
        this.usage = { organizations: [], companies: [] };
        this.usageError = err instanceof TypeError
          ? 'Не удалось загрузить привязки (ошибка сети)'
          : (err.message || 'Не удалось загрузить привязки');
      } finally {
        if (seq === this.usageSeq) this.usageLoading = false;
      }
    },

    confirmDetachAll() {
      this.detachConfirmVisible = true;
    },

    async performDetachAll() {
      this.detachConfirmVisible = false;
      const table = this.selectedTable;
      if (!table) return;
      this.detaching = true;
      try {
        const res = await detachAllSystemTable(table.table.id);
        const orgN = res?.organizations_detached || 0;
        const compN = res?.companies_detached || 0;
        // Перезагружаем привязки только если пользователь не ушёл на другую таблицу,
        // пока летел запрос (иначе затрём usage чужой таблицы).
        if (this.selectedTable && this.selectedTable.table.id === table.table.id) {
          await this.loadUsage();
        }
        useDeletionsStore().notify({
          prefix: 'Таблица ',
          bold: table.table.display_name,
          suffix: ` отвязана от организаций (${orgN}) и компаний (${compN})`,
        });
      } catch (err) {
        const msg = err instanceof TypeError ? 'ошибка сети' : (err.message || 'ошибка');
        useDeletionsStore().notify({ prefix: 'Не удалось отвязать: ', bold: msg, type: 'error' });
      } finally {
        this.detaching = false;
      }
    },

    confirmDetachOne(kind, item) {
      this.detachOneTarget = { kind, id: item.id, name: item.name };
    },

    async performDetachOne() {
      const target = this.detachOneTarget;
      const table = this.selectedTable;
      this.detachOneTarget = null;
      if (!target || !table) return;
      this.detachingOne = true;
      try {
        if (target.kind === 'organization') {
          await detachOrganizationFromSystemTable(table.table.id, target.id);
        } else {
          await detachCompanyFromSystemTable(table.table.id, target.id);
        }
        // Перезагружаем привязки, только если не ушли на другую таблицу.
        if (this.selectedTable && this.selectedTable.table.id === table.table.id) {
          await this.loadUsage();
        }
        useDeletionsStore().notify({
          prefix: target.kind === 'organization' ? 'Организация ' : 'Компания ',
          bold: target.name,
          suffix: ' отвязана от таблицы',
        });
      } catch (err) {
        const msg = err instanceof TypeError ? 'ошибка сети' : (err.message || 'ошибка');
        useDeletionsStore().notify({ prefix: 'Не удалось отвязать: ', bold: msg, type: 'error' });
      } finally {
        this.detachingOne = false;
      }
    },

    openVersions() {
      if (this.selectedTable) {
        // from=admin: страница версий откроется для архивной таблицы, и "Назад"
        // должно вернуть в конструктор (сюда), а не на публичную /table/:name,
        // которой для архивной таблицы нет.
        this.$router.push(`/table/${this.selectedTable.table.name}/versions?from=admin`);
      }
    },

    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'asc';
      }
    },
    
    getTableTypeLabel(type) {
      return type === 'cars' ? 'Машины' : 'Люди';
    },
    
    getTableStatusClass(table) {
      if (table.table.status !== 'active') {
        return 'status-inactive';
      }
      return table.current_status === 'open' ? 'status-open' : 'status-closed';
    },
    
    getTableStatusText(table) {
      if (table.table.status !== 'active') {
        return 'Неактивно';
      }
      return table.current_status === 'open' ? 'Открыто' : 'Закрыто';
    },
    
    getTableCurrentStatusClass(table) {
      if (table.table.status !== 'active') {
        return 'status-inactive-badge';
      }
      return table.current_status === 'open' ? 'status-open-badge' : 'status-closed-badge';
    },
    
    getTableCurrentStatusText(table) {
      if (table.table.status !== 'active') {
        if (table.table.status === 'maintenance') return 'На обслуживании';
        return 'Неактивно';
      }
      return table.current_status === 'open' ? 'Открыто сейчас' : 'Закрыто сейчас';
    },
    
    
    getDefaultHint(tableType) {
      if (tableType === 'cars') {
        return 'При прибытии автомобиля ПО ФАКТУ: спроси у водителя организацию, посмотри, есть ли организация в таблице слева, если организация есть - пропустить';
      } else {
        return 'При проходе человека ПО ФАКТУ: проверьте документы, сверьте с данными в системе';
      }
    },
    
    saveHint() {
      this.updateTable('fact_table_hint');
    },
    
    cancelHintEdit() {
      if (this.selectedTable) {
        this.selectedTable.table.fact_table_hint = this.originalHint;
      }
    },
    
    saveInstruction() {
      this.updateTable('instruction');
    },
    
    cancelInstructionEdit() {
      if (this.selectedTable) {
        this.selectedTable.table.instruction = this.originalInstruction;
      }
    },
    
    toggleTableTypeDropdown() {
      this.tableTypeDropdownOpen = !this.tableTypeDropdownOpen;
    },
    
    selectTableType(type) {
      if (this.selectedTable) {
        this.selectedTable.table.table_type = type;
        this.tableTypeDropdownOpen = false;
        this.updateTable('table_type');
      }
    },

    // --- Групповой выбор ---
    isSelected(id) {
      return this.selectedIds.includes(id);
    },
    toggleSelect(id) {
      const i = this.selectedIds.indexOf(id);
      if (i === -1) this.selectedIds.push(id);
      else this.selectedIds.splice(i, 1);
    },
    // onRowCheck: обычный клик - toggle; shift-клик - диапазон от якоря до текущей.
    onRowCheck(table, index, event) {
      if (event.shiftKey && window.getSelection) window.getSelection().removeAllRanges();
      if (event.shiftKey && this.lastSelectedId != null && this.lastSelectedId !== table.id) {
        const list = this.sortedTables.map(item => item.table);
        const anchor = list.findIndex(t => t.id === this.lastSelectedId);
        if (anchor !== -1) {
          const [from, to] = anchor < index ? [anchor, index] : [index, anchor];
          const target = !this.isSelected(table.id);
          for (let i = from; i <= to; i++) {
            const id = list[i].id;
            const sel = this.isSelected(id);
            if (target && !sel) this.selectedIds.push(id);
            else if (!target && sel) this.selectedIds.splice(this.selectedIds.indexOf(id), 1);
          }
          this.lastSelectedId = table.id;
          return;
        }
      }
      this.toggleSelect(table.id);
      this.lastSelectedId = table.id;
    },
    toggleSelectAll() {
      this.selectedIds = this.allSelected ? [] : this.sortedTables.map(item => item.table.id);
      this.lastSelectedId = null;
    },
    clearSelection() {
      this.selectedIds = [];
      this.lastSelectedId = null;
      this.pendingBulkOp = null;
    },
    pruneSelection() {
      if (!this.selectedIds.length) return;
      const visible = new Set(this.sortedTables.map(item => item.table.id));
      const pruned = this.selectedIds.filter(id => visible.has(id));
      if (pruned.length !== this.selectedIds.length) this.selectedIds = pruned;
    },
    startBulkOperation(operation) {
      this.pendingBulkOp = operation;
      this.bulkConfirmVisible = true;
    },
    cancelBulkConfirm() {
      if (this.bulkSubmitting) return;
      this.bulkConfirmVisible = false;
      this.pendingBulkOp = null;
    },
    async applyBulkArchiveRestore() {
      const ids = [...this.selectedIds];
      const op = this.pendingBulkOp;
      if (this.bulkSubmitting) return;
      if (!ids.length || (op !== 'archive' && op !== 'restore')) {
        this.bulkConfirmVisible = false;
        this.pendingBulkOp = null;
        return;
      }
      this.bulkSubmitting = true;
      let result;
      try {
        result = op === 'archive' ? await bulkArchiveSystemTables(ids) : await bulkRestoreSystemTables(ids);
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось выполнить групповую операцию', type: 'error' });
        this.bulkSubmitting = false;
        return;
      }
      this.bulkSubmitting = false;
      if (this.handleBulkResult(op, result, ids)) {
        this.bulkConfirmVisible = false;
        this.pendingBulkOp = null;
      }
    },
    // Разбор BulkOpResult: полный успех -> notify, частичный -> ui.warning с
    // перечнем непрошедших. false при ошибке-envelope (держим модалку для повтора).
    handleBulkResult(op, result, ids) {
      if (!result || typeof result.success_count !== 'number') {
        useDeletionsStore().notify({ prefix: result?.message || 'Не удалось выполнить групповую операцию', type: 'error' });
        return false;
      }
      const label = op === 'restore' ? 'Восстановлено' : 'Архивировано';
      if (result.error_count > 0) {
        const failed = (result.errors || []).map(e => e.name || `#${e.id}`).join(', ');
        useDeletionsStore().notify({ prefix: 'Выполнено ', bold: `${result.success_count} из ${ids.length}`, suffix: `. Не удалось: ${failed}`, type: 'warning' });
      } else {
        useDeletionsStore().notify({ prefix: `${label}: `, bold: String(result.success_count) });
      }
      // Успешно обработанная строка уходит из текущего вида (архив <-> активные) -
      // как и в одиночном performDeleteTable/restoreTable, сбрасываем открытую
      // деталь, если она была среди реально обработанных (не среди failed).
      if (this.selectedTable) {
        const failedIds = new Set((result.errors || []).map(e => e.id));
        if (ids.includes(this.selectedTable.table.id) && !failedIds.has(this.selectedTable.table.id)) {
          this.selectedTable = null;
          this.activeTab = 'main';
        }
      }
      this.clearSelection();
      this.refreshData();
      return true;
    },
  },
}
</script>

<style scoped>
.table-constructor-container {
  background: var(--surface);
  border-radius: 16px;
  border: 1px solid var(--border);
  overflow: hidden;
  width: 100%;
  height: 550px;
  position: relative;
}

.management-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  height: 50px;
}

.management-title {
  font-size: 1.2em;
  margin: 0;
  font-weight: 600;
  color: var(--text);
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.add-header-button {
  padding: 8px 16px;
  background: var(--accent);
  color: var(--accent-contrast);
  border: none;
  border-radius: 50px;
  cursor: pointer;
  font-size: 0.9em;
  transition: background-color 0.2s ease;
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.add-header-button:hover {
  background: var(--accent-hover);
}

.archive-dropdown {
  min-width: 130px;
}

/* Панель групповых операций - оверлей поверх .management-header (не reflow,
   список не прыгает при выборе - урок #510). Высота = высоте шапки (50px).
   .table-constructor-container - контекст позиционирования (position:relative). */
.bulk-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 6;
  display: flex;
  align-items: center;
  gap: 14px;
  height: 50px;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  background: var(--accent-tint-solid);
  overflow-x: auto;
  overflow-y: hidden;
}

.bulk-count {
  font-size: 14px;
  font-weight: 600;
  color: var(--accent-text);
  white-space: nowrap;
}

.bulk-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: nowrap;
  margin-left: auto;
}

.bulk-actions .pill {
  flex: 0 0 auto;
  white-space: nowrap;
}

.pill {
  display: inline-flex;
  align-items: center;
  height: 30px;
  padding: 0 14px;
  border-radius: 50px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  border: none;
  font-family: inherit;
  white-space: nowrap;
  transition: background 0.2s, border-color 0.2s;
}

.pill-ghost {
  background: var(--surface);
  color: var(--accent-text);
  border: 1px solid var(--accent);
}

.pill-ghost:hover {
  background: var(--accent-tint);
}

.bulk-clear {
  color: var(--text-muted);
  border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
}

.bulk-clear:hover {
  background: var(--surface-2);
}

.pill-danger {
  background: var(--surface);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.pill-danger:hover {
  background: var(--danger-bg);
  border-color: var(--danger);
}

.pill-restore {
  background: var(--success);
  color: var(--fill-text);
}

.pill-restore:hover {
  background: color-mix(in srgb, var(--success) 85%, var(--text));
}

.table-row.inactive {
  background: var(--surface-2);
  color: var(--text-muted);
}

.inactive-badge {
  margin-left: 6px;
  font-size: 0.75em;
  color: var(--text-muted);
  font-style: italic;
}

.archive-badge {
  background: var(--text-muted);
  color: var(--surface);
  padding: 4px 10px;
  border-radius: 50px;
  font-size: 0.75em;
  font-weight: 500;
  white-space: nowrap;
}

.restore-btn {
  padding: 8px 16px;
  background: var(--success);
  color: var(--fill-text);
  border: none;
  border-radius: 10px;
  font-size: 0.85em;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s ease;
}

.restore-btn:hover {
  background: color-mix(in srgb, var(--success) 85%, var(--text));
}

.history-btn,
.versions-btn {
  padding: 8px 16px;
  background: var(--surface);
  color: var(--accent-text);
  border: 1px solid var(--accent);
  border-radius: 10px;
  font-size: 0.85em;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s ease;
}

.history-btn:hover,
.versions-btn:hover {
  background: var(--accent-tint);
}

.content-container {
  display: flex;
  height: 500px;
  width: 100%;
}

.table-section {
  width: 40%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--border);
  background: var(--surface);
}

.table-section.with-details {
  width: 40%;
}

.table-container {
  background: var(--surface);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.table-header {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  height: 43px;
  align-items: center;
}

.header-col {
  padding: 0 8px;
  font-size: 14px;
  color: var(--text-muted);
  font-weight: 600;
  text-align: left;
  display: flex;
  align-items: center;
  gap: 5px;
  transition: .2s;
  cursor: pointer;
  user-select: none;
  /* В узкой секции метка обрезается, а не наезжает на соседнюю колонку. */
  overflow: hidden;
}

.header-col p {
  margin: 0;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
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
  font-weight: 600 !important;
}

/* Групповой выбор (#345-bulk): чекбокс-колонка отъедает 8% у остальных
   (пропорционально урезаны ниже), их сумма с check-col снова даёт 100%. */
.check-col {
  width: 8%;
  min-width: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 8px;
  cursor: default;
}

.bulk-check {
  width: 15px;
  height: 15px;
  cursor: pointer;
  accent-color: var(--accent-text);
  margin: 0;
}

.id-col {
  width: 11%;
  min-width: 50px;
}

/* Наименование сжимается первым (текст обрезается через .truncate-text),
   отдавая место колонкам Тип/Статус с бейджами фиксированной ширины. */
.name-col {
  width: 29%;
  min-width: 84px;
}

.type-col {
  width: 24%;
  min-width: 76px;
}

/* Под самый широкий бейдж статуса ("На обслуживании", nowrap ~108px), иначе
   он вылезает за колонку и прижимается к правому краю секции. */
.status-col {
  width: 28%;
  min-width: 108px;
}

.table-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.table-row {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  align-items: center;
  transition: background-color 0.2s ease;
  cursor: pointer;
  height: 42px;
  font-size: 14px;
}

.table-row:hover {
  background-color: var(--surface-2);
}

.table-row.selected {
  background-color: var(--accent-tint);
}

.table-row:last-child {
  border-bottom: none;
}

.table-col {
  padding: 0 8px;
}

.cell-content {
  display: block;
  padding: 4px 0;
}

.id-value {
  font-weight: 600;
  color: var(--text);
}

.truncate-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
  display: block;
}

.type-badge {
  display: inline-block;
  padding: 4px 10px;
  border-radius: 20px;
  font-size: 11px;
  font-weight: 600;
  min-width: 70px;
  text-align: center;
}

.type-badge.cars {
  background-color: var(--success-bg);
  color: var(--success-text);
  border: 1px solid var(--success);
}

.type-badge.people {
  background-color: var(--info-bg);
  color: var(--accent-text);
  border: 1px solid color-mix(in srgb, var(--info) 30%, var(--surface));
}

.status-badge {
  display: inline-block;
  padding: 4px 10px;
  border-radius: 20px;
  font-size: 11px;
  font-weight: 500;
  min-width: 70px;
  text-align: center;
}

.status-open {
  background-color: var(--success-bg);
  color: var(--success-text);
  border: 1px solid var(--success);
}

.status-closed {
  background-color: var(--warning-bg);
  color: var(--warning-text);
  border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
}

.status-inactive {
  background-color: var(--danger-bg);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.table-footer {
  padding: 6px 20px;
  border-top: 1px solid var(--border);
  text-align: right;
  background: var(--accent-tint);
}

.items-count {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}

.details-section {
  width: 60%;
  display: flex;
  flex-direction: column;
  background: var(--surface);
  overflow: hidden;
}

.details-tabs {
  display: flex;
  flex-direction: column;
  gap: 6px;
  border-bottom: 1px solid var(--border);
  background: var(--surface-2);
  padding: 10px 16px;
}

/* Когда показывается ряд "По факту" - снизу есть свой padding-bottom через
   --fact, верхний padding общего блока этого хватает. */
.details-tabs:has(.details-tabs__row--fact) {
  padding-bottom: 0;
}

.details-tabs__row {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
}

.details-tabs__row--fact {
  padding: 6px 0 10px;
  border-top: 1px dashed var(--border);
  margin-top: 2px;
}

.details-tabs__group-label {
  font-size: 11px;
  color: var(--text-muted);
  font-weight: 500;
  letter-spacing: 0.3px;
  padding-right: 4px;
}

.tab-btn {
  padding: 8px 18px;
  background: var(--surface);
  border: 1px solid transparent;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-muted);
  transition: color 0.2s ease, background 0.2s ease, border-color 0.2s ease;
  border-radius: 50px;
  white-space: nowrap;
  flex-shrink: 0;
}

.tab-btn:hover {
  color: var(--accent-text);
  background: var(--accent-tint);
}

.tab-btn.active {
  color: var(--accent-text);
  border-color: var(--accent);
  background: var(--surface);
}

@media (max-width: 1100px) {
  .tab-btn {
    padding: 8px 12px;
    font-size: 12px;
  }
}

.tab-content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background: var(--surface);
  line-height: 1.5;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

/* table-info-title + table-info-row в одну строку:
   слева - название + тип-бейдж, справа - system-name + status-бейдж. */
.details-title-wrapper {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.table-info-title {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.details-title {
  margin: 0;
  color: var(--text);
  font-size: 1.2em;
  font-weight: 600;
}

.table-type-badge {
  display: inline-block;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
}

.table-type-badge.cars {
  background-color: var(--success-bg);
  color: var(--success-text);
  border: 1px solid var(--success);
}

.table-type-badge.people {
  background-color: var(--info-bg);
  color: var(--accent-text);
  border: 1px solid color-mix(in srgb, var(--info) 30%, var(--surface));
}

.table-info-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.system-name {
  font-size: 12px;
  color: var(--text-muted);
  background: transparent;
  padding: 4px 12px;
  border: 1px solid var(--border);
  border-radius: 50px;
}

.current-status-badge {
  display: inline-block;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
}

.status-open-badge {
  background-color: var(--success-bg);
  color: var(--success-text);
  border: 1px solid var(--success);
}

.status-closed-badge {
  background-color: var(--warning-bg);
  color: var(--warning-text);
  border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
}

.status-inactive-badge {
  background-color: var(--danger-bg);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.details-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.action-btn {
  padding: 8px 16px;
  border: none;
  border-radius: 30px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  transition: background 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.view-btn {
  background: var(--accent);
  color: var(--accent-contrast);
}

.view-btn:hover {
  background: var(--accent-hover);
}

.delete-icon-btn {
  outline: none;
  border: none;
  width: 30px;
  height: 30px;
  padding: 5px;
  border-radius: 10px;
  display: flex;
  align-items:center;
  justify-content: center;
  transition: .2s;
}

.delete-icon {
  color: var(--danger);
  width: 20px;
  height: 20px;
}

.delete-icon-btn:hover {
  background-color: var(--border);
  cursor:pointer;
}

.details-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.compact-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-row {
  display: flex;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group.compact {
  flex: 1;
}

.detail-label {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}

/* Подсказка под полем настройки - объясняет где видно/зачем нужно. */
.field-hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
}

.form-textarea {
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 13px;
  width: 100%;
  transition: border-color 0.2s;
  background: var(--surface);
  resize: vertical;
  font-family: inherit;
}

.form-textarea:focus {
  border-color: var(--accent);
  outline: none;
}

.custom-select {
  position: relative;
  width: 100%;
}

.select-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 15px;
  background: var(--surface);
  cursor: pointer;
  font-size: 13px;
  height: 35px;
  transition: all 0.2s ease;
}

.select-header:hover {
  border-color: var(--border);
  background: var(--surface-2);
}

.select-value {
  color: var(--text);
}

.select-arrow {
  width: 10px;
  height: 10px;
  transition: transform 0.2s ease;
  transform: rotate(90deg);
}

.select-arrow.rotated {
  transform: rotate(-90deg);
}

.select-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 15px;
  box-shadow: 0 4px 12px var(--shadow-drop);
  z-index: 10;
  margin-top: 4px;
  overflow: hidden;
}

.select-option {
  padding: 8px 12px;
  font-size: 13px;
  cursor: pointer;
  transition: background 0.2s;
  border-bottom: 1px solid var(--border);
}

.select-option:last-child {
  border-bottom: none;
}

.select-option:hover {
  background: var(--accent-tint);
  color: var(--accent-text);
}

.select-option.active {
  background: var(--accent-tint);
  color: var(--accent-text);
  font-weight: 500;
}

.dropdown-fade-enter-active,
.dropdown-fade-leave-active {
  transition: all 0.2s ease;
}

.dropdown-fade-enter-from,
.dropdown-fade-leave-to {
  opacity: 0;
  transform: translateY(-5px);
}

.status-toggle {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.status-btn {
  padding: 6px 16px;
  border: 1px solid var(--border);
  background: var(--surface);
  border-radius: 30px;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
  color: var(--text-muted);
}

.status-btn:hover {
  border-color: var(--accent);
  color: var(--accent-text);
}

.status-btn.active {
  background: var(--accent);
  border-color: var(--accent);
  color: var(--accent-contrast);
}

.settings-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-label {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  margin: 0;
}

.checkbox-group {
  padding: 12px;
  background: var(--accent-tint);
  border-radius: 20px;
  border: 1px solid var(--border);
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.checkbox-input {
  width: 16px;
  height: 16px;
  cursor: pointer;
  accent-color: var(--accent-text);
}

.checkbox-text {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
}

.hint-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 10px;
}

.instruction-section {
  background: var(--accent-tint);
  border-radius: 10px;
  padding: 12px;
  border: 1px solid var(--border);
  margin-bottom: 10px;
}

.section-header-with-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.editor-actions {
  display: flex;
  gap: 5px;
}

.compact-btn {
  padding: 4px 10px;
  border: none;
  border-radius: 15px;
  cursor: pointer;
  font-size: 11px;
  font-weight: 500;
  transition: all 0.2s;
}

.save-btn {
  background: var(--accent);
  color: var(--accent-contrast);
}

.save-btn:hover {
  background: var(--accent-hover);
}

.cancel-btn {
  background: var(--text-muted);
  color: var(--surface);
}

.cancel-btn:hover {
  background: color-mix(in srgb, var(--text-muted) 78%, var(--text));
}

.location-section {
  margin-bottom: 24px;
}

.warnings-section {
  margin-bottom: 24px;
}

.section-title {
  margin: 0 0 12px 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}

.map-link-group {
  display: flex;
  gap: 12px;
  align-items: stretch;
}

/* Input и кнопка одинаковой высоты через height + box-sizing - чтобы кнопка
   не вылезала выше инпута из-за разного line-height у <a> и <input>. */
.map-link-group .form-input,
.map-link-group .map-link-btn {
  height: 40px;
  box-sizing: border-box;
  display: inline-flex;
  align-items: center;
}

.map-link-group .form-input {
  flex: 1;
  padding: 0 14px;
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 13px;
  background: var(--surface);
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
  outline: none;
}

.map-link-group .form-input:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.12);
}

.map-link-group .form-input::placeholder {
  color: var(--text-muted);
}

.map-link-btn {
  padding: 0 18px;
  background: var(--accent-tint);
  color: var(--accent-text);
  text-decoration: none;
  border-radius: 30px;
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  transition: background-color 0.2s ease;
  border: 1px solid var(--accent);
}

.map-link-btn:hover {
  background: var(--accent);
  color: var(--accent-contrast);
}

.no-selection-message {
  width: 60%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-weight: 400;
  font-size: 14px;
}

.no-results {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-muted);
  width: 100%;
}

/* Модальное окно */
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
  z-index: 10000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
  animation: overlayAppear 0.3s ease-out;
}

@keyframes overlayAppear {
  from {
    background: var(--overlay);
    backdrop-filter: blur(0px);
  }
  to {
    background: var(--overlay);
    backdrop-filter: blur(0.1px);
  }
}

.modal-content {
  background: var(--surface);
  border-radius: 12px;
  padding: 0;
  width: 420px;
  max-width: 90vw;
  box-shadow: 0 20px 60px var(--shadow-drop);
  animation: modalAppear 0.3s ease-out;
}

@keyframes modalAppear {
  from {
    opacity: 0;
    transform: scale(0.8) translateY(-20px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
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
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}

.modal-close {
  background: none;
  border: none;
  cursor: pointer;
  padding: 6px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s ease;
}

.modal-close:hover {
  background-color: var(--surface-2);
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 20px;
  border-top: 1px solid var(--border);
}

.modal-btn {
  padding: 8px 20px;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  transition: background-color 0.2s ease;
  min-width: 90px;
}

.modal-btn--cancel {
  background: var(--surface-2);
  color: var(--text-muted);
  border: 1px solid var(--border);
}

.modal-btn--cancel:hover {
  background: var(--accent-tint);
}

.modal-btn--confirm {
  background: var(--accent);
  color: var(--accent-contrast);
}

.modal-btn--confirm:hover:not(:disabled) {
  background: var(--accent-hover);
}

.modal-btn--disabled {
  background: var(--border);
  cursor: not-allowed;
}

/* Анимации */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: all 0.3s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .modal-overlay,
.modal-fade-leave-active .modal-overlay {
  transition: all 0.3s ease;
}

.modal-fade-enter-active .modal-content,
.modal-fade-leave-active .modal-content {
  transition: all 0.3s ease;
}

.modal-fade-enter-from .modal-overlay,
.modal-fade-leave-to .modal-overlay {
  background: transparent;
  backdrop-filter: blur(0px);
}

.modal-fade-enter-from .modal-content,
.modal-fade-leave-to .modal-content {
  opacity: 0;
  transform: scale(0.8) translateY(-20px);
}

/* Скроллбары */
.table-body::-webkit-scrollbar {
  width: 6px;
}

.table-body::-webkit-scrollbar-track {
  background: var(--surface-2);
  border-radius: 3px;
}

.table-body::-webkit-scrollbar-thumb {
  background: var(--border);
  border-radius: 3px;
}

.table-body::-webkit-scrollbar-thumb:hover {
  background: var(--text-muted);
}

@media (max-width: 767.98px) {
  .form-row {
    flex-direction: column;
  }

  .map-link-group {
    flex-direction: column;
  }

  .map-link-btn {
    width: 100%;
    text-align: center;
  }

  .notification {
    left: 20px;
    right: 20px;
    transform: translateY(-100%);
    min-width: auto;
  }

  /* Референс инлайн-кнопок шапки (responsive-tables.css .rt-header-inline)
     сжимает Обновить/Создать до иконок, но дропдаун архива и поиск остаются
     полноразмерными - на узких экранах сужаем и их, иначе строка контролов
     не помещается рядом с заголовком и переносится некрасиво. */
  .management-header {
    padding: 10px var(--gutter, 16px);
  }

  .archive-dropdown {
    min-width: 92px;
  }

  :deep(.search) {
    width: 120px;
  }

  /* .rt-header-inline форсирует height:auto!important и перенос controls на
     мобилке (responsive-tables.css) - фиксированная высота шапки (50px), под
     которую подогнан .bulk-bar{position:absolute}, здесь уже не гарантирована.
     Возвращаем панель в поток (урок NumberFormat/#345-bulk), иначе оверлей
     перекрывает перенесённые controls или обрезается. */
  .bulk-bar {
    position: static;
    height: auto;
    padding: 12px var(--gutter, 16px);
    overflow-x: visible;
  }

  .bulk-actions {
    flex-wrap: wrap;
  }

  .check-col {
    min-height: 44px;
  }

  /* Master-detail стек на мобилке (эталон CitizenshipManagement/#1097 S9):
     список и панель деталей ужимались бок о бок (40%/60%) в 390px - заголовок
     ID/Наименование обрезался, поля деталей нечитаемы. Складываем в колонку:
     список карточками сверху (rt-* конверсия из responsive-tables.css скрывает
     desktop-шапку и рендерит строки карточками), панель деталей полной ширины
     снизу при выборе. */
  .content-container {
    flex-direction: column;
    height: auto;
  }

  .table-section,
  .table-section.with-details,
  .details-section,
  .no-selection-message {
    width: 100%;
  }

  .table-section {
    border-right: none;
    border-bottom: 1px solid var(--border);
  }

  /* Список не занимает весь экран - панель деталей достижима скроллом ниже. */
  .table-body {
    max-height: 300px;
  }

  @keyframes slideDown {
    from {
      transform: translateY(-100%);
    }
    to {
      transform: translateY(0);
    }
  }
}

/* Блок привязок на вкладке «Основное». */
.usage-section {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* Отделяем блок привязок от секции инструкции сверху. */
.usage-section--inline {
  margin-top: 24px;
  padding-top: 24px;
  border-top: 1px solid var(--border);
}

.usage-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}

.usage-header__text {
  flex: 1;
  min-width: 0;
}

.usage-header .section-title {
  margin: 0 0 4px 0;
}

.usage-header .field-hint {
  margin: 0;
}

.detach-all-btn {
  background: var(--surface);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
  white-space: nowrap;
}

.detach-all-btn:hover:not(:disabled) {
  background: var(--danger-bg);
  border-color: var(--danger);
}

.detach-all-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.usage-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.usage-group__title {
  font-size: 0.9em;
  font-weight: 600;
  color: var(--text);
}

.usage-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.usage-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: var(--accent-tint);
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 14px;
}

.usage-item__name {
  color: var(--text);
}

.usage-item__archived {
  color: var(--text-muted);
  font-size: 0.8em;
  font-weight: 500;
}

/* Крестик «Отвязать» на строке привязки (виден админу). */
.usage-item__detach {
  margin-left: auto;
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: transparent;
  color: var(--text-muted);
  font-size: 20px;
  line-height: 1;
  border-radius: 8px;
  cursor: pointer;
  transition: color 0.15s, background 0.15s;
  position: relative;
}

.usage-item__detach:hover:not(:disabled) {
  color: var(--danger-text);
  background: var(--danger-bg);
}

.usage-item__detach:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Всплывающая подсказка #333 как у прочих hint проекта (не native title). */
.usage-item__detach::after {
  content: attr(data-hint);
  position: absolute;
  bottom: calc(100% + 6px);
  right: 0;
  background: var(--hint-bg);
  color: var(--hint-text);
  font-size: 12px;
  white-space: nowrap;
  padding: 4px 8px;
  border-radius: 6px;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.15s;
  z-index: 1;
}

.usage-item__detach:hover:not(:disabled)::after {
  opacity: 1;
}

.usage-empty {
  margin: 0;
  font-size: 13px;
  color: var(--text-muted);
}

.usage-state {
  font-size: 14px;
  color: var(--text-muted);
}

.usage-state--error {
  color: var(--danger-text);
}
</style>