<template>
  <div
    class="selected-table-card"
    :class="[
      { 'enlarged': enlarged, 'grid-mode': grid, 'is-portrait': isCompact, 'config-not-ready': !configReady },
      `density-${rowDensity}`,
    ]"
    :style="{ '--table-font-size': tableFontSize + 'px' }"
    data-testid="cars-table"
  >
    <div class="card-header">
      <div class="card-header__title">
        <!-- Хвост «по заявке» на телефоне скрыт: имя экрана кеглем 18 и так не
             помещается рядом со счётчиком и «Обновить», а различает таблицы первая
             половина - соседний блок называется «Автомобили по факту». -->
        <h3 class="card-title">
          <span class="blue">Номера автомобилей</span><span class="card-title__tail"> по заявке</span>
        </h3>
      </div>
      <div
        v-if="$slots['header-actions']"
        class="card-header__actions"
      >
        <slot name="header-actions" />
      </div>
      <div
        v-if="!preview"
        class="card-header__settings"
      >
        <span class="items-count">
          <!-- Подпись счётчика на телефоне короче: отдельным классом, а не обрезкой,
               иначе она съедает место у имени экрана (эталон §2.2). -->
          <span
            class="items-count__text"
            data-testid="ob-on-territory"
          ><span class="items-count__label">Машин на территории:</span><span
            class="items-count__label-short"
          >На территории:</span> <AnimatedCounter :value="carsOnTerritory" /></span>
          <button
            v-if="can(`table.${tableName}.history`)"
            class="history-btn"
            @click="openCarsTableHistory"
          >
            История
          </button>
        </span>
        <SwitchToggle
          v-model="enlarged"
          class="enlarged-toggle"
          data-testid="enlarged-toggle"
        />
        <SwitchToggle
          class="grid-toggle"
          :model-value="grid"
          label="Сетка"
          title="Показать границы ячеек таблицы"
          data-testid="grid-toggle"
          @update:model-value="$emit('update:grid', $event)"
        />
      </div>
      <!-- «Обновить» - прямой ребёнок шапки, а не часть .card-header__settings:
           на мобилке (#1097 S6) заголовок и «Обновить» обязаны остаться в одной
           строке, а счётчик с тумблерами уезжают ниже. Десктоп не меняется -
           .card-header__settings прижат вправо margin-left: auto, кнопка идёт
           сразу за ним с тем же зазором. -->
      <RefreshButton
        v-if="!preview"
        class="card-header__refresh"
        :loading="refreshing"
        @refresh="loadData"
      />
    </div>

    <!-- Панель групповых операций (#1194) - оверлей поверх .card-header (не
         reflow, урок #510), появляется при ctrl/shift-выделении строк. -->
    <div
      v-if="!preview && selectedCount > 0"
      class="bulk-bar"
      data-testid="cars-bulk-bar"
    >
      <span class="bulk-count">Выбрано: {{ selectedCount }}</span>
      <div class="bulk-actions">
        <!-- Экспорт выбранных (#1194 S6) - клиентский ExcelJS по уже загруженным
             строкам, read-only. Гейт - то же право, что у полного экспорта
             (table.<name>.export), НЕ page.admin: доступно любому, кто видит таблицу. -->
        <button
          v-if="can(`table.${tableName}.export`)"
          class="lk-button lk-button--secondary lk-button--sm"
          data-testid="cars-bulk-export"
          @click="exportSelectedToExcel"
        >
          Экспорт выбранных
        </button>
        <!-- Перенос/добавление - BE гейтит requireAdmin (page.admin), FE-кнопки
             тем же правом (см. api/system-tables.js cleanupTableSnapshots), иначе
             "видно, но 403" при клике не-админом. -->
        <template v-if="can('page.admin')">
          <button
            class="lk-button lk-button--secondary lk-button--sm"
            data-testid="cars-bulk-move"
            @click="openBulkModal('move')"
          >
            Перенести
          </button>
          <button
            class="lk-button lk-button--secondary lk-button--sm"
            data-testid="cars-bulk-add"
            @click="openBulkModal('add')"
          >
            Добавить в таблицу
          </button>
          <button
            class="lk-button lk-button--danger lk-button--sm"
            data-testid="cars-bulk-remove"
            @click="openBulkRemoveConfirm"
          >
            Убрать
          </button>
        </template>
        <button
          class="lk-button lk-button--ghost lk-button--sm bulk-clear"
          data-testid="cars-bulk-clear"
          @click="clearSelection"
        >
          Снять выбор
        </button>
      </div>
    </div>

    <div class="card-content rt-table">
      <div class="items-header">
        <div class="header-row rt-head-row">
          <!-- Въезд - отдельная колонка -->
          <div
            class="col entry-col"
            style="order: 0;"
          >
            Въезд
          </div>
          <!-- Выезд - отдельная колонка -->
          <div
            class="col exit-col"
            style="order: 1;"
          >
            Выезд
          </div>
          <div
            v-if="isFieldInDom('car_number')"
            class="col number-col"
            :class="fieldColClass('car_number')"
            :style="getColStyle('car_number', true)"
            @click="sortBy('car_number')"
          >
            <p :class="{ 'active-sort': sortField === 'car_number' }">
              Номер Т/С
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'car_number', 'desc': sortField === 'car_number' && sortDirection === 'desc' }"
            />
          </div>
          <div
            v-if="isFieldInDom('car_brand')"
            class="col brand-col"
            :class="fieldColClass('car_brand')"
            :style="getColStyle('car_brand', true)"
            @click="sortBy('car_brand')"
          >
            <p :class="{ 'active-sort': sortField === 'car_brand' }">
              Марка
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'car_brand', 'desc': sortField === 'car_brand' && sortDirection === 'desc' }"
            />
          </div>
          <div
            v-if="isFieldInDom('organization')"
            class="col organization-col"
            :class="fieldColClass('organization')"
            :style="getColStyle('organization', true)"
            @click="sortBy('organization')"
          >
            <p :class="{ 'active-sort': sortField === 'organization' }">
              Организация
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'organization', 'desc': sortField === 'organization' && sortDirection === 'desc' }"
            />
          </div>
          <div
            v-if="isFieldInDom('company')"
            class="col company-col"
            :class="fieldColClass('company')"
            :style="getColStyle('company', true)"
            @click="sortBy('company')"
          >
            <p :class="{ 'active-sort': sortField === 'company' }">
              Компания
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'company', 'desc': sortField === 'company' && sortDirection === 'desc' }"
            />
          </div>
          <div
            v-if="isFieldInDom('application_id')"
            class="col application-col"
            :class="fieldColClass('application_id')"
            :style="getColStyle('application_id', true)"
            @click="sortBy('application_id')"
          >
            <p :class="{ 'active-sort': sortField === 'application_id' }">
              Номер заявки
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'application_id', 'desc': sortField === 'application_id' && sortDirection === 'desc' }"
            />
          </div>
          <div
            v-if="isFieldInDom('unload_place')"
            class="col place-col"
            :class="fieldColClass('unload_place')"
            :style="getColStyle('unload_place', true)"
            @click="sortBy('unload_place')"
          >
            <p :class="{ 'active-sort': sortField === 'unload_place' }">
              Место разгрузки
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'unload_place', 'desc': sortField === 'unload_place' && sortDirection === 'desc' }"
            />
          </div>
          <div
            v-if="isFieldInDom('valid_until')"
            class="col date-col"
            :class="fieldColClass('valid_until')"
            :style="getColStyle('valid_until', true)"
            @click="sortBy('entry_date_to')"
          >
            <p :class="{ 'active-sort': sortField === 'entry_date_to' }">
              Действует до
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'entry_date_to', 'desc': sortField === 'entry_date_to' && sortDirection === 'desc' }"
            />
          </div>
          <div
            v-if="isFieldInDom('time_range')"
            class="col time-col"
            :class="fieldColClass('time_range')"
            :style="getColStyle('time_range', true)"
            @click="sortBy('entry_time')"
          >
            <p :class="{ 'active-sort': sortField === 'entry_time' }">
              Время
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'entry_time', 'desc': sortField === 'entry_time' && sortDirection === 'desc' }"
            />
          </div>
          <div
            v-if="isFieldInDom('status')"
            class="col status-col"
            :class="fieldColClass('status')"
            :style="getColStyle('status', true)"
            @click="sortBy('status')"
          >
            <p :class="{ 'active-sort': sortField === 'status' }">
              Статус
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'status', 'desc': sortField === 'status' && sortDirection === 'desc' }"
            />
          </div>
          <!-- Пустой spacer-заголовок над chevron-кнопкой "Подробнее" в строке -
               без него шапка короче строки на 1 колонку и заголовки уезжают. -->
          <div
            v-if="hiddenInPortraitFields().length"
            class="col expand-col"
            style="order: 9998;"
          />
          <div
            class="col actions-col"
            style="order: 9999;"
          />
        </div>
      </div>
      
      <div class="items-container">
        <!-- Полноэкранный лоадер - только при первой загрузке (isLoading),
             когда данных ещё нет вообще. -->
        <div
          v-if="isLoading"
          class="loading-message"
        >
          <LoaderSpinner label="Загрузка машин…" />
        </div>

        <!-- Оверлей-лоадер при refresh: накрывает таблицу полупрозрачной
             плёнкой, не схлопывает строки - высота остаётся стабильной. -->
        <div
          v-if="refreshing && !isLoading"
          class="refresh-overlay"
        >
          <LoaderSpinner label="Обновление…" />
        </div>

        <div
          v-if="!isLoading && displayItems.length > 0"
          class="items-body"
          :class="{ 'is-drag-selecting': isDragging }"
        >
          <transition-group
            name="fade-list"
            tag="div"
          >
            <div
              v-for="(item, index) in displayItems"
              :key="item.id"
              class="item-row"
              :class="{ 'item-row--expanded': expandedRows[item.id], 'item-row--selected': isSelected(item.id) }"
              :data-testid="index === 0 ? 'ob-pass-row' : null"
              :style="{ animationDelay: `${index * 0.05}s` }"
              @click="preview ? null : onRowClick($event, item)"
              @mousedown="preview ? null : onRowMouseDown($event, item)"
              @mouseenter="preview ? null : dragOver(item.id)"
            >
              <!-- rt-pass: строка собирается талоном на мобилке (responsive-tables.css,
                   часть 3). Общая инфраструктура с таблицей «по факту» - обе стоят на
                   одном экране одна под другой, и разнобой между ними виден целиком. -->
              <div class="item-data rt-row rt-pass">
                <!-- Въезд - кнопка -->
                <div
                  class="col entry-col"
                  style="order: 0;"
                  data-label="Въезд"
                  @click.stop
                >
                  <button
                    class="action-btn entry-btn"
                    :class="{ 'active': item.entry_checked }"
                    :disabled="preview || item.entry_checked"
                    data-testid="ob-pass-entry"
                    @click="preview ? null : handleEntryExit(item, 'entry')"
                  >
                    Въезд
                  </button>
                </div>
                <!-- Выезд - кнопка -->
                <div
                  class="col exit-col"
                  style="order: 1;"
                  data-label="Выезд"
                  @click.stop
                >
                  <button
                    class="action-btn exit-btn"
                    :class="{ 'active': item.exit_checked }"
                    :disabled="preview || !item.entry_checked || item.exit_checked"
                    data-testid="ob-pass-exit"
                    @click="preview ? null : handleEntryExit(item, 'exit')"
                  >
                    Выезд
                  </button>
                </div>
                <div
                  v-if="isFieldInDom('car_number')"
                  class="col number-col rt-pass__plate"
                  :class="fieldColClass('car_number')"
                  :style="getColStyle('car_number')"
                  data-label="Номер Т/С"
                >
                  {{ item.car_number }}
                </div>
                <div
                  v-if="isFieldInDom('car_brand')"
                  class="col brand-col rt-pass__mark"
                  :class="fieldColClass('car_brand')"
                  :style="getColStyle('car_brand')"
                  data-label="Марка"
                >
                  {{ item.car_brand }}
                </div>
                <div
                  v-if="isFieldInDom('organization')"
                  class="col organization-col"
                  :class="fieldColClass('organization')"
                  :style="getColStyle('organization')"
                  data-label="Организация"
                >
                  {{ item.organization_name }}
                </div>
                <div
                  v-if="isFieldInDom('company')"
                  class="col company-col"
                  :class="fieldColClass('company')"
                  :style="getColStyle('company')"
                  data-label="Компания"
                >
                  {{ item.company || '-' }}
                </div>
                <div
                  v-if="isFieldInDom('application_id')"
                  class="col application-col"
                  :class="fieldColClass('application_id')"
                  :style="getColStyle('application_id')"
                  data-label="Номер заявки"
                >
                  <span
                    v-if="isManualItem(item)"
                    class="manual-badge"
                  >Добавлено вручную</span>
                  <template v-else>
                    {{ item.applicationNumber || '-' }}
                  </template>
                </div>
                <div
                  v-if="isFieldInDom('unload_place')"
                  class="col place-col"
                  :class="fieldColClass('unload_place')"
                  :style="getColStyle('unload_place')"
                  data-label="Место разгрузки"
                >
                  {{ formatUnloadPlaces(item) }}
                </div>
                <div
                  v-if="isFieldInDom('valid_until')"
                  class="col date-col"
                  :class="fieldColClass('valid_until')"
                  :style="getColStyle('valid_until')"
                  data-label="Действует до"
                >
                  {{ formatDate(item.entry_date_to) }}
                </div>
                <div
                  v-if="isFieldInDom('time_range')"
                  class="col time-col"
                  :class="fieldColClass('time_range')"
                  :style="getColStyle('time_range')"
                  data-label="Время"
                >
                  {{ formatTimeRange(item.entry_time_from, item.entry_time_to) }}
                </div>
                <div
                  v-if="isFieldInDom('status')"
                  class="col status-col"
                  :class="fieldColClass('status')"
                  :style="getColStyle('status')"
                  data-label="Статус"
                >
                  <StatusBadge :status="item.status" />
                </div>
                <div
                  v-if="hiddenInPortraitFields().length"
                  class="col expand-col"
                  style="order: 9998;"
                  @click.stop
                >
                  <button
                    type="button"
                    class="expand-btn rt-pass__act"
                    :class="{ 'expand-btn--open': expandedRows[item.id] }"
                    :aria-expanded="!!expandedRows[item.id]"
                    :aria-label="expandedRows[item.id] ? 'Скрыть' : 'Подробнее'"
                    @click="toggleRowExpand(item.id)"
                  >
                    <svg
                      width="14"
                      height="14"
                      viewBox="0 0 14 14"
                      fill="none"
                    >
                      <path
                        d="M3.5 5L7 8.5L10.5 5"
                        stroke="currentColor"
                        stroke-width="1.5"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                    </svg>
                    <!-- Подпись видна только в талоне: там шеврон стоит в подвале
                         рядом с «Удалить» и один значок не объясняет, что за ним. -->
                    <span class="rt-pass__act-label">{{ expandedRows[item.id] ? 'Скрыть' : 'Подробнее' }}</span>
                  </button>
                </div>
                <div
                  v-if="can('entity.cars.delete')"
                  class="col actions-col"
                  style="order: 9999;"
                  @click.stop
                >
                  <!-- Машина привязана к нескольким таблицам (#1194 S5) - выбор
                       между "убрать только отсюда" и глобальной деактивацией.
                       Единственная привязка ИЛИ не-админ - корзина работает как
                       раньше (unbind-table гейтится page.admin на бэке, иначе
                       "видно, но 403"). -->
                  <TableRowRemoveMenu
                    v-if="(item.target_tables_count || 0) > 1 && can('page.admin')"
                    :disabled="preview || isLoading"
                    @remove-current="removeFromCurrentTableWithNotification(item)"
                    @remove-all="removeItemWithNotification(item)"
                  />
                  <button
                    v-else
                    class="delete-btn rt-pass__act rt-pass__act--danger"
                    title="Удалить"
                    :disabled="preview || isLoading"
                    @click="preview ? null : removeItemWithNotification(item)"
                  >
                    <AppIcon
                      name="trashcan"
                      class="delete-icon rt-pass__act-icon"
                    />
                    <span class="rt-pass__act-label">Удалить</span>
                  </button>
                </div>
              </div>
              <div
                v-if="expandedRows[item.id] && hiddenInPortraitFields().length"
                class="item-row__details"
                @click.stop
              >
                <div
                  v-for="name in hiddenInPortraitFields()"
                  :key="name"
                  class="detail-item"
                >
                  <span class="detail-item__label">{{ portraitFieldLabel(name) }}</span>
                  <span class="detail-item__value">
                    <StatusBadge
                      v-if="name === 'status'"
                      :status="item.status"
                    />
                    <template v-else>{{ portraitFieldValue(item, name) }}</template>
                  </span>
                </div>
              </div>
            </div>
          </transition-group>
        </div>
        
        <div
          v-else-if="!isLoading"
          class="no-data-message"
        >
          {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Нет активных автомобилей' }}
        </div>
      </div>
    </div>

    <!-- Модальное окно с деталями автомобиля -->
    <VehicleDetailsModal
      v-if="!preview"
      :show="showVehicleDetails"
      :vehicle="selectedVehicle"
      :all-unloading-places="allUnloadingPlaces"
      :license-plate-formats="licensePlateFormats"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      :show-car-features="true"
      :source="'carstable'"
      @close="closeVehicleDetails"
      @open-application="$emit('open-application', $event)"
    />

    <!-- Модальное окно истории всех машин -->
    <CarsTableHistoryModal
      v-if="!preview && showCarsTableHistory"
      :cars="itemsData"
      :table-id="tableId"
      :table-title="tableTitle"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      @close="showCarsTableHistory = false"
    />

    <!-- Групповые операции "Перенести"/"Добавить в таблицу" (#1194) -->
    <TableBulkTargetModal
      v-if="!preview"
      :show="bulkModalVisible"
      :mode="bulkModalMode"
      entity-type="cars"
      :exclude-table-id="tableId"
      :selected-count="selectedCount"
      :submitting="bulkSubmitting"
      @close="closeBulkModal"
      @apply="applyBulkTableOp"
    />

    <!-- Групповое "Убрать" (#1194 S5): снимает привязку к текущей таблице у
         выделенных строк; последняя привязка -> BE деактивирует машину сам. -->
    <ConfirmationModal
      v-if="!preview"
      :show="bulkRemoveConfirmVisible"
      title="Убрать из таблицы"
      :message="`Убрать выбранные машины (${selectedCount}) из этой таблицы? Если это последняя таблица машины, она будет деактивирована.`"
      confirm-text="Убрать"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
      @confirm="confirmBulkRemove"
      @cancel="cancelBulkRemove"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants'
import { idFilterSet } from '@/utils/idFilter';
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';
import eventStream from '@/services/eventStream';
import { useOrientation } from '@/composables/useOrientation';
import { useRowSelection } from '@/composables/useRowSelection';
import RefreshButton from './RefreshButton.vue';
import VehicleDetailsModal from './CreateApplication/VehicleDetailsModal.vue';
import CarsTableHistoryModal from './CarsTableHistoryModal.vue';
import TableBulkTargetModal from './TableBulkTargetModal.vue';
import TableRowRemoveMenu from './TableRowRemoveMenu.vue';
import ConfirmationModal from './ConfirmationModal.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import StatusBadge from '@/components/ui/StatusBadge.vue';
import SwitchToggle from '@/components/ui/SwitchToggle.vue';
import AnimatedCounter from '@/components/ui/AnimatedCounter.vue';
import ExcelJS from 'exceljs';
import { bulkMoveCarsTable, bulkAddCarsTable, bulkUnbindCarsTable } from '@/api/cars';
import { pickOverflowFields, columnMinWidth, measureRowAvailableWidth, SERVICE_COLUMNS_WIDTH } from '@/utils/tableColumnFit';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import AppIcon from '@/components/icons/AppIcon.vue';

const ENLARGED_KEY_PREFIX = 'enlarged-mode:cars:';

/**
 * Состав карточки на телефоне: что видно сразу, остальное - в «Подробнее».
 *
 * Подбор столбцов по ширине (#1307) в карточке не работает и работать не может:
 * поля стоят своими строками и за ширину не конкурируют, а мерить он пытается
 * скрытую строку заголовков. Скатывался он всегда в одно и то же - оставить
 * `keepAtLeast` столбца из девяти, поэтому в талоне были только номер и марка, а
 * организация, срок и место уезжали под шеврон вместе с бейджем статуса.
 *
 * Набор - шапка талона из мокапа (docs/mockups/mobile-ux.html, экран «Проходная»):
 * номер, марка, срок и место. Организация к ним добавлена по подсказке самого
 * экрана - «спроси у водителя организацию и найди её в списке», - а статус нужен
 * подвалу. Компания и номер заявки остаются в «Подробнее»: на посту по ним не
 * сверяют, а карточку они удлиняют на две строки.
 */
const MOBILE_CARD_FIELDS = [
  'car_number',
  'car_brand',
  'organization',
  'unload_place',
  'valid_until',
  'time_range',
  'status',
];

export default {
  name: 'CarsTable',
  components: {
    RefreshButton,
    VehicleDetailsModal,
    CarsTableHistoryModal,
    TableBulkTargetModal,
    TableRowRemoveMenu,
    ConfirmationModal,
    LoaderSpinner,
    StatusBadge,
    SwitchToggle,
    AnimatedCounter,
    AppIcon,
  },
  setup() {
    const { isPortrait, isCompact } = useOrientation();
    // Порог тот же, что у card-правил responsive-tables.css: брейкпоинт компонента
    // обязан совпадать с брейкпоинтом инфраструктуры, которой он пользуется.
    const { isNarrow } = useNarrowScreen(767.98);
    const permissionsStore = usePermissionsStore();
    const rowSelection = useRowSelection();
    return { isPortrait, isCompact, isNarrow, permissionsStore, ...rowSelection };
  },
  props: {
    tableName: { type: String, default: '' },
    // Отображаемое имя таблицы - показывается в заголовке истории.
    tableTitle: { type: String, default: '' },
    tableId: { type: Number, default: null },
    searchQuery: { type: String, default: '' },
    // Мультивыбор (#1398): пустой массив - фильтр выключен. Дефолт обязателен -
    // preview-монтирования (вкладка «Колонки», версии таблицы) пропсы не передают.
    selectedOrganizationIds: { type: Array, default: () => [] },
    selectedCompanyIds: { type: Array, default: () => [] },
    selectedUnloadingPlaceIds: { type: Array, default: () => [] },
    dateRangeStart: { type: Date, default: null },
    dateRangeEnd: { type: Date, default: null },
    selectedDate: { type: Date, default: null },
    currentUserId: { type: Number, default: null },
    currentUserName: { type: String, default: '' },
    // Preview-режим для админ-вкладки "Колонки" (#345): seed-данные, без API и без кнопок.
    preview: { type: Boolean, default: false },
    previewFields: { type: Array, default: null },
    previewItems: { type: Array, default: null },
    // Режим "Сетка" (#1289): границы ячеек. Управляется одним тумблером страницы.
    grid: { type: Boolean, default: false }
  },
  emits: ['open-application', 'update:grid'],
  data() {
    return {
      sortField: null,
      sortDirection: 'desc',
      itemsData: [],
      pendingDeleteIds: [],
      isLoading: false,
      refreshing: false,
      organizationsMap: {},
      carUnloadPlacesMap: {},
      allUnloadingPlaces: [],
      licensePlateFormats: [],
      showVehicleDetails: false,
      selectedVehicle: null,
      // Столбцы, не поместившиеся по ширине (#1307): скрываются от наименее
      // важных, при равном приоритете - правые. Значения остаются в «Подробнее».
      overflowFields: [],
      showCarsTableHistory: false,
      pollingInterval: null,
      // Real-time (#840): подписка на tables:<tableId>, статус SSE-соединения и seq-токен
      // против гонки конкурентных silentRefresh (таймер + SSE-сигнал, урок #632/#840).
      eventStreamOff: null,
      eventStreamStatusOff: null,
      sseConnected: false,
      refreshSeq: 0,
      enlarged: false,
      fieldsVisibility: {},
      fieldOrders: {},
      fieldWidths: {},
      fieldPriorities: {},
      fieldsEnlargedVisibility: {},
      fieldsEnlargedWidth: {},
      fieldsEnlargedWeight: {},
      tableFontSize: 14,
      rowDensity: 'normal',
      expandedRows: {},
      compactPriorityThreshold: 2,
      // false до тех пор пока не подгрузим конфиг таблицы. Корневой class
      // config-not-ready подавляет transitions, чтобы шапка/столбцы не
      // "ездили" между дефолтом и сохранёнными значениями при первом рендере.
      configReady: false,
      // Групповые операции "Перенести"/"Добавить в таблицу" (#1194 S4).
      bulkModalVisible: false,
      bulkModalMode: 'move',
      bulkSubmitting: false,
      // Групповое "Убрать" (#1194 S5).
      bulkRemoveConfirmVisible: false,
      bulkRemoveSubmitting: false,
    };
  },
  computed: {
    displayItems() {
      let filtered = this.itemsData.filter(i => !this.pendingDeleteIds.includes(i.id));
      if (this.searchQuery) {
        const variants = buildSearchVariants(this.searchQuery);
        filtered = filtered.filter(item => {
          const searchFields = [
            item.car_number,
            item.car_brand,
            item.organization_name,
            item.company,
            this.formatUnloadPlaces(item),
            this.formatDate(item.entry_date_to),
            item.status
          ];
          return matchesSearch(searchFields.filter(Boolean).join(' '), variants);
        });
      }
      const organizations = idFilterSet(this.selectedOrganizationIds);
      if (organizations) {
        filtered = filtered.filter(item => organizations.has(String(item.organization_id)));
      }
      const companies = idFilterSet(this.selectedCompanyIds);
      if (companies) {
        filtered = filtered.filter(item => companies.has(String(item.company_id)));
      }
      const unloadPlaceIds = idFilterSet(this.selectedUnloadingPlaceIds);
      if (unloadPlaceIds) {
        filtered = filtered.filter(item => {
          const unloadPlaces = this.carUnloadPlacesMap[item.id] || [];
          return unloadPlaces.some(place => unloadPlaceIds.has(String(place.id)));
        });
      }
      if (this.selectedDate) {
        const selectedDateStr = this.selectedDate.toISOString().split('T')[0];
        filtered = filtered.filter(item => item.entry_date_to === selectedDateStr);
      } else if (this.dateRangeStart && this.dateRangeEnd) {
        filtered = filtered.filter(item => {
          const itemDate = new Date(item.entry_date_to);
          return itemDate >= this.dateRangeStart && itemDate <= this.dateRangeEnd;
        });
      }
      if (this.sortField) {
        filtered.sort((a, b) => {
          let valueA, valueB;
          switch (this.sortField) {
            case 'car_number':
            case 'car_brand':
            case 'organization':
            case 'company':
            case 'status':
              valueA = (a[this.sortField] || '').toString().toLowerCase();
              valueB = (b[this.sortField] || '').toString().toLowerCase();
              break;
            case 'unload_place':
              valueA = this.formatUnloadPlaces(a).toLowerCase();
              valueB = this.formatUnloadPlaces(b).toLowerCase();
              break;
            case 'entry_date_to':
              valueA = a.entry_date_to ? new Date(a.entry_date_to) : new Date(0);
              valueB = b.entry_date_to ? new Date(b.entry_date_to) : new Date(0);
              break;
            case 'entry_time':
              valueA = this.extractStartTime(a.entry_time_from);
              valueB = this.extractStartTime(b.entry_time_from);
              break;
            default:
              return 0;
          }
          if (valueA < valueB) return this.sortDirection === 'asc' ? -1 : 1;
          if (valueA > valueB) return this.sortDirection === 'asc' ? 1 : -1;
          return 0;
        });
      }
      return filtered;
    },
    carsOnTerritory() {
      return this.itemsData.filter(item => item.entry_checked && !item.exit_checked).length;
    },
    hasActiveFilters() {
      return !!(
        this.searchQuery ||
        idFilterSet(this.selectedOrganizationIds) ||
        idFilterSet(this.selectedCompanyIds) ||
        idFilterSet(this.selectedUnloadingPlaceIds) ||
        this.selectedDate ||
        (this.dateRangeStart && this.dateRangeEnd)
      );
    }
  },
  watch: {
    tableName: {
      immediate: true,
      handler(newVal) {
        if (this.preview) return;
        if (newVal) {
          this.stopPolling();
          this.startPolling();
        }
      }
    },
    tableId: {
      immediate: true,
      handler(newVal) {
        if (this.preview) return;
        this.loadEnlargedFromStorage();
        if (newVal) {
          this.fetchFieldsVisibility();
        }
        this.subscribeTableScope(newVal);
      }
    },
    enlarged(value) {
      if (this.preview) return;
      this.saveEnlargedToStorage(value);
      // У увеличенного режима свой набор видимых столбцов - пересобираем подгонку.
      this.$nextTick(() => this.recalcOverflowFields());
    },
    // Поворот телефона и переход через брейкпоинт меняют правила подбора столбцов:
    // ResizeObserver сюда не доедет - ширина карточки при этом может не измениться.
    isNarrow() {
      this.recalcOverflowFields();
    },
    // Строки, ушедшие из видимого списка (фильтр/поиск/удаление/поллинг),
    // убираем из выделения - счётчик "Выбрано: N" не должен врать (#1194).
    displayItems(items) {
      this.pruneSelection(items.map(i => i.id));
    },
    previewItems: {
      immediate: true,
      handler(newVal) {
        if (this.preview && Array.isArray(newVal)) {
          this.itemsData = newVal;
        }
      }
    },
    previewFields: {
      immediate: true,
      handler(newVal) {
        if (!this.preview || !Array.isArray(newVal)) return;
        const nextVis = {};
        const nextOrd = {};
        const nextW = {};
        const nextP = {};
        newVal.forEach((f, i) => {
          nextVis[f.field_name] = f.is_visible !== false;
          nextOrd[f.field_name] = typeof f.display_order === 'number' ? f.display_order : i;
          if (typeof f.width === 'number' && f.width > 0) nextW[f.field_name] = f.width;
          if (typeof f.priority === 'number' && f.priority > 0) nextP[f.field_name] = f.priority;
        });
        this.fieldsVisibility = nextVis;
        this.fieldOrders = nextOrd;
        this.fieldWidths = nextW;
        this.fieldPriorities = nextP;
        this.$nextTick(() => this.recalcOverflowFields());
        this.markConfigReady();
      }
    }
  },
  mounted() {
    if (this.preview) return;
    this.startPolling();
    this.loadEnlargedFromStorage();
    // Real-time (#840): по сигналу продюсера tables.refresh тихо перезагружаем строки
    // вместо ожидания поллинга. Сама подписка на scope - в watch tableId (уже immediate).
    eventStream.connect();
    this.eventStreamStatusOff = eventStream.onStatus((status) => {
      this.sseConnected = status === 'connected';
    });

    // Drag-select (#1227 P4): mouseup может произойти вне строки (курсор ушёл
    // за пределы таблицы) - слушаем на window, иначе drag "залипнет".
    this.onGlobalMouseUp = () => this.endDrag();
    window.addEventListener('mouseup', this.onGlobalMouseUp);
    this.$nextTick(() => this.bindWidthWatcher());
  },
  beforeUnmount() {
    this.unbindWidthWatcher();
    this.stopPolling();
    if (this.onGlobalMouseUp) {
      window.removeEventListener('mouseup', this.onGlobalMouseUp);
      this.onGlobalMouseUp = null;
    }
    if (this.preview) return; // preview никогда не подключался - нечего отключать
    if (this.eventStreamOff) {
      this.eventStreamOff();
      this.eventStreamOff = null;
    }
    if (this.eventStreamStatusOff) {
      this.eventStreamStatusOff();
      this.eventStreamStatusOff = null;
    }
    eventStream.disconnect();
  },
  methods: {
    // Гейтинг по правам (#187 Фаза 2). super -> всегда true, admin -> всё кроме
    // denied, обычный -> по эффективному гранту. Реактивно: читает стор прав.
    can(key) {
      return this.permissionsStore.hasPermission(key);
    },
    // Машина добавлена вручную без заявки (#1049): application_id === null (BE отдаёт
    // NULL для вложения-сироты). Строгий null - у обычных строк applicationId - число.
    isManualItem(item) {
      return item.applicationId === null;
    },
    // Real-time (#840): подписка на scope конкретной таблицы, пересобирается при смене tableId.
    subscribeTableScope(tableId) {
      if (this.eventStreamOff) {
        this.eventStreamOff();
        this.eventStreamOff = null;
      }
      if (!tableId) return;
      this.eventStreamOff = eventStream.subscribe(`tables:${tableId}`, () => {
        this.silentRefresh();
      });
    },
    // Основной метод загрузки данных с флагом silent (без показа лоадера).
    // seq-токен против гонки конкурентных вызовов (поллинг + SSE-сигнал, #632/#840):
    // устаревший (медленно резолвнутый) ответ не должен затирать более свежие данные.
    async _loadData(silent = false) {
      const seq = ++this.refreshSeq;
      if (!silent && this.isLoading) return;
      if (!silent) this.isLoading = true;
      try {
        await this.fetchUnloadingPlaces();
        await this.fetchLicensePlateFormats();
        await this.fetchCarsData(seq, silent);
        await this.fetchCarUnloadPlaces(seq);
        await this.fetchCarHistoryStatus(seq);
        return true;
      } catch (error) {
        console.error('Ошибка при загрузке машин:', error);
        return false;
      } finally {
        if (!silent) this.isLoading = false;
      }
    },

    // Для внешнего вызова (кнопка Refresh) - тихо, без скачка высоты таблицы.
    // Заодно перечитываем настройки таблицы (видимость/порядок/ширина/шрифт/
    // плотность), чтобы изменения админа применились без перезагрузки страницы.
    async loadData() {
      this.refreshing = true;
      try {
        const [ok] = await Promise.all([
          this._loadData(true),
          this.fetchFieldsVisibility(),
        ]);
        // Тихий сбой оставляет прежние строки (не чистим таблицу), но на ЯВНОЕ
        // обновление пользователь должен получить сигнал, что данные не свежие.
        if (!ok) {
          useDeletionsStore().notify({ prefix: 'Не удалось обновить таблицу: ', bold: 'показаны последние данные', type: 'error' });
        }
      } finally {
        this.refreshing = false;
      }
    },

    // Для тихого обновления по таймеру
    async silentRefresh() {
      await this._loadData(true);
    },

    async fetchUnloadingPlaces() {
      try {
        const response = await apiRequest("/unload-places", {});
        if (response.ok) this.allUnloadingPlaces = await response.json();
      } catch (error) {
        console.error("Ошибка при загрузке мест разгрузки:", error);
      }
    },

    async fetchLicensePlateFormats() {
      try {
        const response = await apiRequest("/license-plate-formats", {});
        if (response.ok) this.licensePlateFormats = await response.json();
      } catch (error) {
        console.error("Ошибка при загрузке форматов номеров:", error);
      }
    },

    async fetchCarsData(seq, silent = false) {
      if (!this.tableId) return;
      try {
        const response = await apiRequest(`/cars/active-for-table/${this.tableId}`, {});
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
        const cars = await response.json();
        await this.fetchOrganizations();
        const nameToIdMap = {};
        Object.keys(this.organizationsMap).forEach(id => {
          nameToIdMap[this.organizationsMap[id]] = id;
        });
        // Территориальное состояние уже отрисованных строк - страховка на случай,
        // если строка пришла без territory_status: без неё каждая перезагрузка
        // (real-time сигнал, поллинг) обнуляла бы отметки въезда и счётчик на
        // территории проваливался бы в 0 до ответа /cars/history/current-status.
        const prevTerritory = new Map(
          this.itemsData.map(item => [item.id, {
            entry_checked: item.entry_checked,
            exit_checked: item.exit_checked,
            entry_time: item.entry_time,
            exit_time: item.exit_time,
            territory_status: item.territory_status,
          }])
        );
        const regularCars = cars.filter(car => {
          if (car.status !== 1) return false;
          const carNumber = car.car_number?.toLowerCase().trim();
          return carNumber !== 'по факту';
        });
        // Преобразуем в нужный формат
        const newItems = regularCars.map(car => {
          const orgName = car.organization || '';
          const orgId = nameToIdMap[orgName] || car.organization_id;
          // Статус берём из самой строки (тот же источник, что у current-status -
          // колонка cars.territory_status), а при его отсутствии - из предыдущего
          // состояния строки. Так отметки въезда/выезда и счётчик не мигают.
          const prev = prevTerritory.get(car.id);
          const entryChecked = car.territory_status != null
            ? car.territory_status === 1
            : (prev?.entry_checked ?? false);
          const exitChecked = car.territory_status != null
            ? car.territory_status === 2
            : (prev?.exit_checked ?? false);
          return {
            id: car.id,
            car_number: car.car_number || '',
            car_brand: car.car_brand || '',
            organization_id: orgId,
            organization_name: orgName || 'Не указана',
            company: car.company || null,
            company_id: car.company_id,
            unload_place: car.unload_place || '-',
            unload_place_ids: car.unload_place_ids || [],
            entry_date_to: car.entry_date_to || '',
            entry_time_from: car.entry_time_from || '',
            entry_time_to: car.entry_time_to || '',
            status: 'В работе',
            checked: false,
            entry_checked: entryChecked,
            exit_checked: exitChecked,
            entry_time: car.territory_entry_time ?? prev?.entry_time ?? null,
            exit_time: prev?.exit_time ?? null,
            applicationId: car.application_id,
            applicationNumber: car.application_number,
            territory_status: car.territory_status ?? prev?.territory_status ?? 0,
            plateNumber: car.car_number,
            mark: car.car_brand,
            formatId: null,
            unloadPlaces: car.unload_place_ids || [],
            // Число таблиц «Проезд», к которым привязана машина (#1194 S5) -
            // >1 включает per-row подменю «Убрать из этой/из всех».
            target_tables_count: car.target_tables_count || 0,
            // Список привязок {id,name,source} (#1227 P1) - карточка машины показывает
            // бейдж источника («из заявки»/«добавлено») в секции «Проезд».
            target_tables: car.target_tables || []
          };
        });
        if (seq !== undefined && seq !== this.refreshSeq) return; // устарел - новее уже в работе/загружен
        // Заменяем массив целиком – Vue отреагирует оптимально
        this.itemsData = newItems;
      } catch (error) {
        console.error("Ошибка при загрузке данных машин:", error);
        // Тихое обновление (real-time сигнал, поллинг) при сбое сети оставляет
        // последние известные строки: очистка стирала бы таблицу и счётчик под
        // пользователем на ровном месте (тот же класс, что обнуление счётчика #1021).
        if (!silent && (seq === undefined || seq === this.refreshSeq)) this.itemsData = [];
        throw error;
      }
    },

    async fetchCarHistoryStatus(seq) {
      try {
        const response = await apiRequest("/cars/history/current-status", {});
        if (response.ok) {
          const statuses = await response.json();
          if (seq !== undefined && seq !== this.refreshSeq) return; // устарел
          const statusMap = {};
          statuses.forEach(status => { statusMap[status.car_id] = status; });
          this.itemsData.forEach(item => {
            const status = statusMap[item.id];
            if (status) {
              item.entry_checked = status.territory_status === 1;
              item.exit_checked = status.territory_status === 2;
              item.entry_time = status.entry_time;
              item.exit_time = status.last_exit_time;
            }
          });
        }
      } catch (error) {
        console.error("Ошибка при загрузке статусов въезда/выезда:", error);
      }
    },

    async fetchCarUnloadPlaces(seq) {
      try {
        const response = await apiRequest("/cars/unload-places", {});
        if (response.ok) {
          const carUnloadPlaces = await response.json();
          // seq-guard (#632/#840): устаревший ответ не должен перезаписывать карту
          // мест разгрузки и мутировать unload_place_ids уже отрисованных строк.
          if (seq !== undefined && seq !== this.refreshSeq) return;
          this.carUnloadPlacesMap = {};
          carUnloadPlaces.forEach(cup => {
            if (!this.carUnloadPlacesMap[cup.car_id]) this.carUnloadPlacesMap[cup.car_id] = [];
            this.carUnloadPlacesMap[cup.car_id].push({
              id: cup.unload_place_id,
              name: cup.unload_place_name || `Место #${cup.unload_place_id}`
            });
            const car = this.itemsData.find(c => c.id === cup.car_id);
            if (car) {
              if (!car.unload_place_ids) car.unload_place_ids = [];
              if (!car.unload_place_ids.includes(cup.unload_place_id)) {
                car.unload_place_ids.push(cup.unload_place_id);
                car.unloadPlaces = car.unload_place_ids;
              }
            }
          });
        }
      } catch (error) {
        console.error("Ошибка при загрузке связей машин с местами разгрузки:", error);
        if (seq === undefined || seq === this.refreshSeq) this.carUnloadPlacesMap = {};
      }
    },

    async fetchOrganizations() {
      try {
        const response = await apiRequest("/organizations", {});
        if (response.ok) {
          const data = await response.json();
          this.organizationsMap = {};
          data.forEach(org => { this.organizationsMap[org.id] = org.name; });
        }
      } catch (error) {
        console.error("Ошибка при загрузке организаций:", error);
      }
    },

    formatUnloadPlaces(item) {
      if (item.unload_place_ids && item.unload_place_ids.length > 0) {
        const placeNames = item.unload_place_ids
          .map(id => {
            const place = this.allUnloadingPlaces.find(p => p.id === id);
            return place ? place.name : null;
          })
          .filter(name => name);
        if (placeNames.length === 0) return '-';
        if (placeNames.length === 1) return placeNames[0];
        return `${placeNames[0]} и др.`;
      }
      return item.unload_place || '-';
    },

    formatDate(dateString) {
      if (!dateString) return '';
      try {
        const [year, month, day] = dateString.split('-');
        const date = new Date(year, month - 1, day);
        return date.toLocaleDateString('ru-RU');
      } catch {
        return '';
      }
    },

    formatTimeRange(timeFrom, timeTo) {
      if (!timeFrom && !timeTo) return '-';
      const formatTime = (timeStr) => {
        if (!timeStr) return '';
        const parts = timeStr.split(':');
        if (parts.length >= 2) return `${parts[0]}:${parts[1]}`;
        return timeStr;
      };
      const formattedTimeFrom = formatTime(timeFrom);
      const formattedTimeTo = formatTime(timeTo);
      if (!formattedTimeTo) return formattedTimeFrom;
      if (!formattedTimeFrom) return formattedTimeTo;
      return `${formattedTimeFrom} - ${formattedTimeTo}`;
    },

    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'desc';
      }
    },

    extractStartTime(timeString) {
      if (!timeString || timeString === '-') return 0;
      const parts = timeString.split(':');
      if (parts.length >= 2) {
        const hours = parseInt(parts[0]) || 0;
        const minutes = parseInt(parts[1]) || 0;
        return hours * 60 + minutes;
      }
      return 0;
    },

    async handleEntryExit(item, type) {
      if (!this.currentUserId) return;
      try {
        let territory_status = type === 'entry' ? 1 : 2;
        const response = await apiRequest(`/cars/${item.id}/territory-status`, {
          method: "PUT",
          body: JSON.stringify({ territory_status, user_id: this.currentUserId, table_id: this.tableId })
        });
        if (response.ok) {
          const index = this.itemsData.findIndex(i => i.id === item.id);
          if (index !== -1) {
            const updatedItem = { ...this.itemsData[index] };
            if (type === 'entry') {
              updatedItem.entry_checked = true;
              updatedItem.exit_checked = false;
            } else {
              updatedItem.entry_checked = false;
              updatedItem.exit_checked = true;
            }
            this.itemsData.splice(index, 1, updatedItem);
          }
          useDeletionsStore().notify({ prefix: 'Машина ', bold: item.car_number, suffix: type === 'entry' ? ' отмечена о прибытии' : ' уехала', type: 'success' });
        } else {
          const err = await response.json();
          console.error('Ошибка при обновлении статуса:', err);
          useDeletionsStore().notify({ prefix: 'Не удалось отметить машину: ', bold: err.message || 'ошибка сервера', type: 'error' });
        }
      } catch (error) {
        console.error('Ошибка сети:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось отметить машину: ', bold: 'ошибка сети', type: 'error' });
      }
    },

    removeItemWithNotification(item) {
      if (this.isLoading) return;
      if (this.pendingDeleteIds.includes(item.id)) return;
      const carId = item.id;
      const tableId = this.tableId;
      const userId = this.currentUserId;
      // Прячем строку через displayItems-фильтр (устойчиво к polling), пока идёт окно отмены.
      this.pendingDeleteIds.push(carId);
      useDeletionsStore().enqueue({
        prefix: 'Машина ',
        bold: item.car_number,
        suffix: ' удалена',
        onConfirm: () => this.commitDelete(carId, tableId, userId),
        onUndo: () => this.unhidePending(carId),
      });
    },

    unhidePending(carId) {
      this.pendingDeleteIds = this.pendingDeleteIds.filter(id => id !== carId);
    },

    async commitDelete(carId, tableId, userId) {
      try {
        const response = await apiRequest(`/cars/${carId}/deactivate`, {
          method: "PUT",
          body: JSON.stringify({ status: 0, user_id: userId, table_id: tableId })
        });
        if (!response.ok) {
          console.error("Ошибка при удалении");
          this.unhidePending(carId);
          return;
        }
        await this._loadData(true);
        this.unhidePending(carId);
      } catch (error) {
        console.error("Ошибка сети при удалении:", error);
        this.unhidePending(carId);
      }
    },

    // Убрать ТОЛЬКО из текущей таблицы (#1194 S5) - альтернатива глобальной
    // деактивации, доступная per-row через TableRowRemoveMenu, когда машина
    // привязана к нескольким таблицам. Тот же enqueue/undo UX, что и обычное
    // удаление, коммит идёт через bulkUnbindCarsTable ([id], tableId).
    removeFromCurrentTableWithNotification(item) {
      if (this.isLoading) return;
      if (this.pendingDeleteIds.includes(item.id)) return;
      const carId = item.id;
      const tableId = this.tableId;
      this.pendingDeleteIds.push(carId);
      useDeletionsStore().enqueue({
        prefix: 'Машина ',
        bold: item.car_number,
        suffix: ' убрана из таблицы',
        onConfirm: () => this.commitUnbindFromCurrentTable(carId, tableId),
        onUndo: () => this.unhidePending(carId),
      });
    },

    async commitUnbindFromCurrentTable(carId, tableId) {
      try {
        const result = await bulkUnbindCarsTable([carId], tableId);
        if (!result || typeof result.success_count !== 'number' || result.error_count > 0) {
          const message = result?.errors?.[0]?.error || result?.message || 'ошибка сервера';
          useDeletionsStore().notify({ prefix: 'Не удалось убрать машину: ', bold: message, type: 'error' });
        }
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось убрать машину: ', bold: 'ошибка сети', type: 'error' });
      } finally {
        await this._loadData(true);
        this.unhidePending(carId);
      }
    },

    // Ctrl/Shift-клик по строке (#1194) - групповое выделение вместо открытия
    // детали; обычный клик поведение не меняет (handleRowClick вернёт false).
    onRowClick(event, item) {
      const orderedIds = this.displayItems.map(i => i.id);
      if (this.handleRowClick(event, item.id, orderedIds)) return;
      this.openVehicleDetails(item);
    },

    // Ctrl(Cmd)+зажать курсор и вести (#1227 P4) - строки под курсором ДОБАВЛЯЮТСЯ
    // к выделению (dragOver на @mouseenter соседних строк, endDrag по глобальному
    // mouseup в mounted). preventDefault только когда drag реально стартовал -
    // иначе обычный клик/выделение текста браузером не ломаются.
    onRowMouseDown(event, item) {
      if (this.startDrag(item.id, event)) event.preventDefault();
    },

    // Групповые операции над выделенными строками (#1194 S4): 'move' - перенести
    // (снять с текущей таблицы, привязать к выбранным), 'add' - добавить в выбранные,
    // не снимая с текущей.
    openBulkModal(mode) {
      this.bulkModalMode = mode;
      this.bulkModalVisible = true;
    },
    closeBulkModal() {
      if (this.bulkSubmitting) return;
      this.bulkModalVisible = false;
    },
    async applyBulkTableOp(toTableIds) {
      const ids = [...this.selectedIds];
      const mode = this.bulkModalMode;
      if (!ids.length || !toTableIds.length) return;
      this.bulkSubmitting = true;
      let result;
      try {
        result = mode === 'move'
          ? await bulkMoveCarsTable(ids, this.tableId, toTableIds)
          : await bulkAddCarsTable(ids, toTableIds);
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось выполнить групповую операцию', type: 'error' });
        this.bulkSubmitting = false;
        return;
      }
      this.bulkSubmitting = false;
      if (this.handleBulkTableOpResult(mode, result, ids.length)) {
        this.bulkModalVisible = false;
      }
    },
    // Разбор BulkOpResult: полный успех -> notify success, частичный -> notify
    // warning с перечнем непрошедших имён (образец MarksManagement.handleBulkResult).
    // false при структурной ошибке-envelope (модалка остаётся открытой для повтора).
    handleBulkTableOpResult(mode, result, total) {
      if (!result || typeof result.success_count !== 'number') {
        useDeletionsStore().notify({ prefix: result?.message || 'Не удалось выполнить групповую операцию', type: 'error' });
        return false;
      }
      const label = mode === 'move' ? 'Перенесено машин: ' : 'Добавлено машин: ';
      if (result.error_count > 0) {
        const failed = (result.errors || []).map(e => e.name || `#${e.id}`).join(', ');
        useDeletionsStore().notify({ prefix: 'Выполнено ', bold: `${result.success_count} из ${total}`, suffix: `. Не удалось: ${failed}`, type: 'warning' });
      } else {
        useDeletionsStore().notify({ prefix: label, bold: String(result.success_count) });
      }
      this.clearSelection();
      this._loadData(true);
      return true;
    },

    // Групповое "Убрать" (#1194 S5): снимает привязку выделенных машин к ТЕКУЩЕЙ
    // таблице (bulkUnbindCarsTable). Последняя привязка -> BE сам деактивирует
    // машину (status=0) - фронту достаточно показать результат, без отдельного
    // deactivate-вызова.
    openBulkRemoveConfirm() {
      this.bulkRemoveConfirmVisible = true;
    },
    cancelBulkRemove() {
      if (this.bulkRemoveSubmitting) return;
      this.bulkRemoveConfirmVisible = false;
    },
    async confirmBulkRemove() {
      const ids = [...this.selectedIds];
      if (!ids.length || this.bulkRemoveSubmitting) return;
      this.bulkRemoveSubmitting = true;
      let result;
      try {
        result = await bulkUnbindCarsTable(ids, this.tableId);
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось выполнить групповую операцию', type: 'error' });
        this.bulkRemoveSubmitting = false;
        return;
      }
      this.bulkRemoveSubmitting = false;
      if (this.handleBulkRemoveResult(result, ids.length)) {
        this.bulkRemoveConfirmVisible = false;
      }
    },
    // Разбор BulkOpResult для "Убрать" - тот же формат, что move/add (см.
    // handleBulkTableOpResult), отдельный метод из-за другой метки успеха.
    handleBulkRemoveResult(result, total) {
      if (!result || typeof result.success_count !== 'number') {
        useDeletionsStore().notify({ prefix: result?.message || 'Не удалось выполнить групповую операцию', type: 'error' });
        return false;
      }
      if (result.error_count > 0) {
        const failed = (result.errors || []).map(e => e.name || `#${e.id}`).join(', ');
        useDeletionsStore().notify({ prefix: 'Убрано ', bold: `${result.success_count} из ${total}`, suffix: `. Не удалось: ${failed}`, type: 'warning' });
      } else {
        useDeletionsStore().notify({ prefix: 'Убрано машин: ', bold: String(result.success_count) });
      }
      this.clearSelection();
      this._loadData(true);
      return true;
    },

    openVehicleDetails(item) {
      this.selectedVehicle = {
        ...item,
        plateNumber: item.car_number,
        mark: item.car_brand,
        formatId: null,
        organization: item.organization_name,
        organizationId: item.organization_id,
        company: item.company,
        companyId: item.company_id,
        isExisting: true,
        unloadPlaces: item.unload_place_ids || [],
        entry_date_to: item.entry_date_to,
        entry_time_from: item.entry_time_from,
        entry_time_to: item.entry_time_to,
        applicationId: item.applicationId,
        entry_checked: item.entry_checked,
        exit_checked: item.exit_checked
      };
      this.showVehicleDetails = true;
    },

    closeVehicleDetails() {
      this.showVehicleDetails = false;
      this.selectedVehicle = null;
    },

    openCarsTableHistory() {
      this.showCarsTableHistory = true;
    },

    startPolling() {
      if (this.pollingInterval) return;
      this.silentRefresh(); // сразу загружаем без лоадера
      this.pollingInterval = setInterval(() => {
        // На живом SSE поллинг молчит (обновление уже пришло сигналом tables.refresh) -
        // таймер остаётся подстраховкой на 60с и мгновенно подхватывает при разрыве (#840).
        if (this.sseConnected) return;
        this.silentRefresh();
      }, 60000);
    },

    stopPolling() {
      if (this.pollingInterval) {
        clearInterval(this.pollingInterval);
        this.pollingInterval = null;
      }
    },

    enlargedStorageKey() {
      return `${ENLARGED_KEY_PREFIX}${this.tableId ?? 'default'}`;
    },

    loadEnlargedFromStorage() {
      try {
        this.enlarged = localStorage.getItem(this.enlargedStorageKey()) === '1';
      } catch {
        this.enlarged = false;
      }
    },

    saveEnlargedToStorage(value) {
      try {
        localStorage.setItem(this.enlargedStorageKey(), value ? '1' : '0');
      } catch {
        /* localStorage недоступен - игнорируем */
      }
    },

    isFieldVisible(fieldName) {
      // Видимость определяется ОТДЕЛЬНО для каждого режима, иначе enlarged
      // молча "подтягивал" бы обычные настройки is_visible.
      if (this.enlarged) {
        const ev = this.fieldsEnlargedVisibility[fieldName];
        if (ev === false) return false;
      } else {
        const v = this.fieldsVisibility[fieldName];
        const visible = v === undefined ? true : v;
        if (!visible) return false;
      }
      // На телефоне состав карточки задан списком, а не подбором по ширине.
      if (this.isNarrow) return MOBILE_CARD_FIELDS.includes(fieldName);
      // В портретном компактном режиме показываем только столбцы с priority<=threshold.
      if (this.isCompact) {
        const p = this.fieldPriorities[fieldName];
        if (typeof p === 'number' && p > this.compactPriorityThreshold) return false;
      }
      // Не поместившиеся по ширине (#1307).
      if (this.overflowFields.includes(fieldName)) return false;
      return true;
    },

    /**
     * Столбец ВСЕГДА в DOM - скрытие через класс col--collapsed с transition.
     * Альтернатива (v-if) удаляла бы из DOM мгновенно, transition не успевал.
     */
    isFieldInDom() {
      return true;
    },

    /**
     * col--collapsed - класс, который схлопывает столбец до нуля ширины с
     * плавной анимацией. Применяется когда поле скрыто в текущем режиме
     * (обычный/enlarged) либо когда priority выше порога в компактном.
     */
    /**
     * Поля, видимые по настройкам таблицы, в порядке отображения слева направо.
     */
    configuredFields() {
      const source = this.enlarged ? this.fieldsEnlargedVisibility : this.fieldsVisibility;
      return Object.keys(source)
        .filter(name => source[name] !== false)
        .sort((a, b) => (this.fieldOrders[a] ?? 0) - (this.fieldOrders[b] ?? 0));
    },

    /**
     * Пересчитывает, какие столбцы не помещаются в текущую ширину таблицы.
     */
    recalcOverflowFields() {
      // В карточке столбцы за ширину не конкурируют - состав задан MOBILE_CARD_FIELDS.
      if (this.isNarrow) {
        this.overflowFields = [];
        return;
      }
      const host = this.$el && this.$el.querySelector('.card-content');
      if (!host) return;
      const reserved = SERVICE_COLUMNS_WIDTH.passage
        + (this.can('entity.cars.delete') ? SERVICE_COLUMNS_WIDTH.actions : 0)
        + SERVICE_COLUMNS_WIDTH.expand;
      // Мерим строку заголовков, а не всю область: её ширина уже без отступов и
      // зазоров между ячейками (#1097 S8 волна 4).
      const measured = measureRowAvailableWidth(host.querySelector('.header-row'));
      this.overflowFields = pickOverflowFields({
        fields: this.configuredFields(),
        available: measured || host.clientWidth,
        priorities: this.fieldPriorities,
        orders: this.fieldOrders,
        reserved,
      });
    },

    bindWidthWatcher() {
      const host = this.$el && this.$el.querySelector('.card-content');
      if (!host || typeof ResizeObserver !== 'function') return;
      this.widthObserver = new ResizeObserver(() => this.recalcOverflowFields());
      this.widthObserver.observe(host);
      this.recalcOverflowFields();
    },

    unbindWidthWatcher() {
      if (this.widthObserver) {
        this.widthObserver.disconnect();
        this.widthObserver = null;
      }
    },

    fieldColClass(fieldName) {
      if (this.enlarged) {
        const ev = this.fieldsEnlargedVisibility[fieldName];
        if (ev === false) return 'col--collapsed';
      } else {
        const v = this.fieldsVisibility[fieldName];
        if (v === false) return 'col--collapsed';
      }
      if (this.isNarrow) {
        return MOBILE_CARD_FIELDS.includes(fieldName) ? '' : 'col--collapsed';
      }
      if (this.isCompact) {
        const p = this.fieldPriorities[fieldName];
        if (typeof p === 'number' && p > this.compactPriorityThreshold) {
          return 'col--collapsed';
        }
      }
      if (this.overflowFields.includes(fieldName)) return 'col--collapsed';
      return '';
    },

    /**
     * Возвращает список field_name, которые видимы по is_visible, но скрыты
     * в текущем портретном режиме из-за priority. Используется для блока
     * "Подробнее" под строкой - показать пользователю недостающие поля.
     */
    hiddenInPortraitFields() {
      // В карточке «Подробнее» показывает всё, что не вошло в её состав.
      if (this.isNarrow) {
        return this.configuredFields().filter(name => !MOBILE_CARD_FIELDS.includes(name));
      }
      const portrait = this.isCompact
        ? Object.keys(this.fieldsVisibility)
          .filter(name => this.fieldsVisibility[name] !== false)
          .filter(name => {
            const p = this.fieldPriorities[name];
            return typeof p === 'number' && p > this.compactPriorityThreshold;
          })
        : [];
      // Не поместившиеся столбцы прячутся так же, как портретные, - значения
      // остаются доступны в панели «Подробнее» (#1307).
      const merged = [...portrait, ...this.overflowFields];
      return [...new Set(merged)]
        .sort((a, b) => (this.fieldOrders[a] ?? 0) - (this.fieldOrders[b] ?? 0));
    },

    toggleRowExpand(rowId) {
      const next = { ...this.expandedRows };
      next[rowId] = !next[rowId];
      this.expandedRows = next;
    },

    portraitFieldLabel(name) {
      const LABELS = {
        car_number: 'Номер Т/С',
        car_brand: 'Марка',
        organization: 'Организация',
        company: 'Компания',
        application_id: 'Номер заявки',
        unload_place: 'Место разгрузки',
        valid_until: 'Действует до',
        time_range: 'Время',
        status: 'Статус',
      };
      return LABELS[name] || name;
    },

    portraitFieldValue(item, name) {
      switch (name) {
        case 'car_number': return item.car_number || '-';
        case 'car_brand': return item.car_brand || '-';
        case 'organization': return item.organization_name || '-';
        case 'company': return item.company || '-';
        case 'application_id': return this.isManualItem(item) ? 'Добавлено вручную' : (item.applicationNumber || '-');
        case 'unload_place': return this.formatUnloadPlaces(item);
        case 'valid_until': return this.formatDate(item.entry_date_to);
        case 'time_range': return this.formatTimeRange(item.entry_time_from, item.entry_time_to);
        default: return '-';
      }
    },

    /**
     * Стиль конфигурируемой ячейки:
     * - order: 10 + display_order (между фиксированными entry/exit/actions).
     * - flex-grow: width из конфига (если задан) - переопределяет дефолт из CSS.
     *
     * В enlarged-режиме НЕ задаём inline flex-grow ни для одного столбца -
     * пусть CSS управляет пропорциями. Это даёт честное распределение
     * освободившегося status-пространства по ВСЕМ оставшимся столбцам
     * пропорционально их базовым flex-grow весам, а не "слепляет" всё
     * в organization.
     */
    getColStyle(fieldName, isHeader = false) {
      const order = this.fieldOrders[fieldName];
      const width = this.fieldWidths[fieldName];
      const style = {};
      if (order !== undefined) style.order = 10 + order;
      if (this.enlarged) {
        // В enlarged: используем enlarged_width если >0, иначе CSS-дефолт (без inline grow).
        const ew = this.fieldsEnlargedWidth[fieldName];
        if (typeof ew === 'number' && ew > 0) style.flexGrow = ew;
        // enlarged_font_weight - применяется ТОЛЬКО к данным, не к заголовку столбца.
        if (!isHeader) {
          const eweight = this.fieldsEnlargedWeight[fieldName];
          if (typeof eweight === 'number' && eweight > 0) style.fontWeight = eweight;
        }
      } else if (width !== undefined && width > 0) {
        style.flexGrow = width;
      }
      // Ниже этого порога столбец не сжимается - значения переставали читаться
      // (#1307). Не поместившиеся столбцы к этому моменту уже скрыты, поэтому
      // минимум не переполняет строку. В портретном режиме своя раскладка.
      if (!this.isCompact) style.minWidth = columnMinWidth(fieldName) + 'px';
      return Object.keys(style).length ? style : null;
    },

    async fetchFieldsVisibility() {
      if (!this.tableId) return;
      try {
        const response = await apiRequest(`/system-tables/${this.tableId}`, {});
        if (!response.ok) return;
        const data = await response.json();
        const nextVis = {};
        const nextOrd = {};
        const nextW = {};
        const nextP = {};
        const nextEnlVis = {};
        const nextEnlW = {};
        const nextEnlWeight = {};
        (data.fields || []).forEach(f => {
          nextVis[f.field_name] = f.is_visible !== false;
          if (typeof f.display_order === 'number') {
            nextOrd[f.field_name] = f.display_order;
          }
          if (typeof f.width === 'number' && f.width > 0) {
            nextW[f.field_name] = f.width;
          }
          if (typeof f.priority === 'number' && f.priority > 0) {
            nextP[f.field_name] = f.priority;
          }
          nextEnlVis[f.field_name] = f.enlarged_is_visible !== false;
          if (typeof f.enlarged_width === 'number' && f.enlarged_width > 0) {
            nextEnlW[f.field_name] = f.enlarged_width;
          }
          if (typeof f.enlarged_font_weight === 'number' && f.enlarged_font_weight > 0) {
            nextEnlWeight[f.field_name] = f.enlarged_font_weight;
          }
        });
        this.fieldsVisibility = nextVis;
        this.fieldOrders = nextOrd;
        this.fieldWidths = nextW;
        this.fieldPriorities = nextP;
        this.fieldsEnlargedVisibility = nextEnlVis;
        this.fieldsEnlargedWidth = nextEnlW;
        this.fieldsEnlargedWeight = nextEnlWeight;
        this.$nextTick(() => this.recalcOverflowFields());
        // Применяем стиль уровня таблицы (#345 фазы 1D+1E).
        const tbl = data.table || {};
        const fs = Number(tbl.font_size);
        if (fs >= 10 && fs <= 24) this.tableFontSize = fs;
        const dens = tbl.row_density;
        if (['compact', 'normal', 'spacious'].includes(dens)) this.rowDensity = dens;
      } catch (error) {
        console.error('Ошибка загрузки настроек столбцов:', error);
      } finally {
        // Снимаем флаг config-not-ready после первой загрузки.
        // 2 rAF + 100ms - гарантия что layout с итоговыми ширинами применился
        // и transition не сработает на стартовом расхождении.
        this.markConfigReady();
      }
    },
    markConfigReady() {
      if (typeof requestAnimationFrame !== 'function') {
        this.configReady = true;
        return;
      }
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          setTimeout(() => { this.configReady = true; }, 100);
        });
      });
    },

    async exportToExcel() {
      await this.buildCarsExcel(this.displayItems, 'Avtomobili');
    },

    // Экспорт только выделенных строк (#1194 S6) - reuse форматирования полного
    // экспорта (buildCarsExcel), фильтр по selectedIds (useRowSelection).
    async exportSelectedToExcel() {
      const rows = this.displayItems.filter(item => this.selectedIds.includes(item.id));
      if (!rows.length) return;
      await this.buildCarsExcel(rows, 'Avtomobili_vybrannye');
      useDeletionsStore().notify({ prefix: 'Выгружено строк: ', bold: String(rows.length) });
    },

    // Общий билдер книги cars-таблицы: набор строк и префикс имени файла -
    // единственное, что различается между полным экспортом и экспортом выбранных.
    async buildCarsExcel(rows, filenamePrefix) {
      if (!rows.length) return;

      const workbook = new ExcelJS.Workbook();
      const worksheet = workbook.addWorksheet('Avtomobili');

      const headers = [
        'Въезд', 'Выезд', 'Номер Т/С', 'Марка', 'Организация',
        'Компания', 'Место разгрузки', 'Дата до', 'Время', '№ заявки', 'Статус',
      ];

      const headerRow = worksheet.addRow(headers);
      headerRow.height = 25;
      headerRow.eachCell((cell) => {
        cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF4F5BDF' } };
        cell.font = { name: 'Verdana', size: 11, bold: true, color: { argb: 'FFFFFFFF' } };
        cell.alignment = { vertical: 'middle', horizontal: 'center' };
        cell.border = {
          top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
          bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
          left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
          right: { style: 'thin', color: { argb: 'FFE6E6E6' } },
        };
      });

      rows.forEach((item, index) => {
        const row = worksheet.addRow([
          item.entry_checked ? 'Да' : 'Нет',
          item.exit_checked ? 'Да' : 'Нет',
          item.car_number || '-',
          item.car_brand || '-',
          item.organization_name || '-',
          item.company || '-',
          this.formatUnloadPlaces(item),
          this.formatDate(item.entry_date_to),
          this.formatTimeRange(item.entry_time_from, item.entry_time_to),
          item.applicationNumber || '-',
          item.status || '-',
        ]);
        row.height = 20;
        const fillColor = index % 2 === 0 ? 'FFF0F5FF' : 'FFE0E9FF';
        row.eachCell((cell) => {
          cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: fillColor } };
          cell.font = { name: 'Verdana', size: 9, color: { argb: 'FF333333' } };
          cell.alignment = { vertical: 'middle' };
          cell.border = {
            top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            right: { style: 'thin', color: { argb: 'FFE6E6E6' } },
          };
        });
      });

      const colCount = headers.length;
      const lastDataRow = rows.length;
      for (let r = 1; r <= lastDataRow + 1; r++) {
        const rc = worksheet.getCell(r, colCount);
        rc.border = { ...rc.border, right: { style: 'medium', color: { argb: 'FF000000' } } };
        const lc = worksheet.getCell(r, 1);
        lc.border = { ...lc.border, left: { style: 'medium', color: { argb: 'FF000000' } } };
      }
      for (let c = 1; c <= colCount; c++) {
        const tc = worksheet.getCell(1, c);
        tc.border = { ...tc.border, top: { style: 'medium', color: { argb: 'FF000000' } } };
        const bc = worksheet.getCell(lastDataRow + 1, c);
        bc.border = { ...bc.border, bottom: { style: 'medium', color: { argb: 'FF000000' } } };
      }

      worksheet.addRow([]);
      const now = new Date();
      const dateStr = now.toLocaleString('ru-RU', {
        day: '2-digit', month: '2-digit', year: 'numeric',
        hour: '2-digit', minute: '2-digit', second: '2-digit',
      }).replace(',', '');
      const userDisplay = (this.currentUserName || '').trim() || 'Пользователь';
      [
        worksheet.addRow(['Отчёт сформировал:', userDisplay]),
        worksheet.addRow(['Дата формирования:', dateStr]),
      ].forEach(row => {
        row.eachCell((cell) => {
          cell.font = { name: 'Verdana', size: 10, color: { argb: 'FF333333' } };
          cell.alignment = { vertical: 'middle' };
          cell.border = {
            top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            right: { style: 'thin', color: { argb: 'FFE6E6E6' } },
          };
        });
      });

      worksheet.columns = [
        { width: 10 }, { width: 10 }, { width: 18 }, { width: 22 }, { width: 35 },
        { width: 25 }, { width: 30 }, { width: 14 }, { width: 18 }, { width: 16 }, { width: 20 },
      ];

      const buffer = await workbook.xlsx.writeBuffer();
      const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.download = `${filenamePrefix}_${dateStr.replace(/[.:,\s]/g, '-')}.xlsx`;
      a.href = url;
      a.click();
      window.URL.revokeObjectURL(url);
    },
  }
};
</script>

<style scoped>
/* Стили остаются без изменений (как в оригинале) */
.selected-table-card {
  position: relative; /* контекст для оверлей-панели .bulk-bar поверх .card-header (#1194) */
  background-color: var(--surface);
  border-radius: 30px;
  border: 1px solid var(--border);
  overflow: hidden;
  width: 100%;
  max-height: 575px;
  display: flex;
  flex-direction: column;
}

.card-header {
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 8px 20px;
  min-height: 50px;
  flex-shrink: 0;
  flex-wrap: wrap;
}

/* Панель групповых операций (#1194) - оверлей поверх .card-header (не
   reflow, урок #510). Высота = высоте шапки (50px). */
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
  margin-left: auto;
}

.card-header__title {
  display: flex;
  gap: 12px;
  align-items: center;
  min-width: 0;
  flex-shrink: 1;
}

/* Слот для действий в шапке (напр. поиск в preview версий) - прижат вправо. */
.card-header__actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 12px;
}

/* Прижат вправо явным auto-margin, а не justify-content: space-between шапки:
   после выноса «Обновить» отдельным элементом (#1097 S6) между ними распределялось
   бы свободное место и настройки уехали бы в середину. Слот .card-header__actions
   со своим margin-left: auto здесь не конкурирует - он живёт только в preview,
   где .card-header__settings не рендерится вовсе. */
.card-header__settings {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
  margin-left: auto;
}

.card-title {
  margin: 0;
  color: var(--text);
  font-weight: 600;
  font-size: 1.1em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.blue {
  color: var(--accent-text);
}

.items-count {
  color: var(--accent-text);
  font-weight: 500;
  font-size: 0.9em;
  display: flex;
  align-items: center;
  gap: 10px;
  white-space: nowrap;
}

.items-count__text {
  /* Подпись и число - один flex-элемент: иначе gap контейнера вставил бы
     лишний зазор между «...территории:» и цифрой. */
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
}

/* Короткая подпись счётчика включается только в мобильной шапке. */
.items-count__label-short {
  display: none;
}

@media (max-width: 1100px) {
  .card-header {
    gap: 8px;
    padding: 6px 12px;
  }
  .card-title {
    font-size: 0.95em;
  }
  .items-count {
    font-size: 0.8em;
    gap: 6px;
  }
  .history-btn {
    padding: 2px 8px;
    font-size: 11px;
  }
}

.history-btn {
  padding: 4px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 12px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
}

.history-btn:hover {
  background: var(--surface-2);
  border-color: var(--accent);
}

.card-content {
  padding: 0;
  flex-grow: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* items-header повторяет геометрию items-body (padding-right + margin-right 4px),
   чтобы доступная ширина для колонок совпала и заголовки выровнялись с данными. */
.items-header {
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
  padding-right: 4px;
  margin-right: 4px;
}

/* header-row повторяет геометрию item-data: padding 10/16 + flex + gap 4. */
.header-row {
  padding: 10px 10px;
  display: flex;
  width: 100%;
  align-items: center;
  gap: 4px;
}

.col {
  flex-shrink: 0;
  box-sizing: border-box;
  text-align: left;
  /* Данные не липнут к границам столбца (заметно в режиме "Сетка", #1289).
     Боковой отступ строки уменьшен на столько же, поэтому крайние столбцы
     остались на прежнем расстоянии от края карточки. */
  padding-left: 6px;
  padding-right: 6px;

  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Веса колонок (flex-grow). Браузер делит доступную ширину пропорционально весам
   видимых колонок. При скрытии любых через v-if остальные автоматически расширяются,
   занимая освободившееся место. */
.entry-col { flex: 6.5 0 0; }
.exit-col { flex: 8 0 0; }
.number-col { flex: 10 0 0; }
.brand-col { flex: 8.5 0 0; }
.organization-col { flex: 18 0 0; }
.company-col { flex: 10 0 0; }
.application-col { flex: 8 0 0; }

.manual-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 11px;
  font-weight: 500;
  line-height: 1.4;
  color: var(--accent-text);
  background: color-mix(in srgb, var(--accent) 10%, var(--surface));
  white-space: nowrap;
}
.place-col { flex: 15 0 0; }
.date-col { flex: 11.5 0 0; }
.time-col { flex: 10 0 0; }
.status-col { flex: 7 0 0; }
.actions-col { flex: 2 0 0; }

.header-row .col {
  font-weight: 500;
  color: var(--text-muted);
  cursor: pointer;
  user-select: none;
  display: flex;
  align-items: center;
  gap: 5px;
}

/* Подпись столбца сжимается с многоточием, а иконка сортировки - нет: раньше
   длинный заголовок выталкивал её за пределы ячейки, и она либо срезалась,
   либо наезжала на соседний столбец в режиме «Сетка» (#1307). */
.header-row .col > p {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin: 0;
}

.header-row .col .sort-icon {
  flex-shrink: 0;
}

.header-row .col:hover {
  color: var(--text);
}

.header-row .col:hover .sort-icon {
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

.items-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  position: relative;
}

/* Overlay-лоадер при refresh - сохраняет высоту таблицы (не схлопывает
   строки). Не используется при первой загрузке (там полноэкранный лоадер). */
.refresh-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in srgb, var(--surface) 75%, transparent);
  backdrop-filter: blur(1px);
  z-index: 2;
  pointer-events: none;
}

.items-body {
  overflow-y: auto;
  flex-grow: 1;
  padding-right: 4px;
  margin-right: 4px;
  min-height: 80px;
}

/* Drag-select (#1227 P4): пока ведём курсор с зажатым Ctrl - не даём браузеру
   выделить текст строк под курсором. */
.items-body.is-drag-selecting,
.items-body.is-drag-selecting * {
  user-select: none;
}

.item-row {
  transition: background-color 0.2s ease;
  opacity: 0;
  transform: translateY(10px);
  animation: fadeInUp 0.3s ease forwards;
  cursor: pointer;
}

/* Тач-экран hover не отдаёт, но :hover после тапа залипает до следующего касания -
   подсветка висела на карточке, по которой уже отработали (эталон §1.5). Гейтим
   ровно то, до чего на телефоне можно дотронуться: строку и кнопки карточки. */
@media (hover: hover) {
  .item-row:hover {
    background-color: var(--surface-2);
  }
}

/* Подсветка ctrl/shift-выделенной строки (#1194) - тот же тон, что фон
   .bulk-bar, чтобы связь выбора с панелью читалась визуально. */
.item-row--selected {
  background-color: var(--accent-tint);
}

@media (hover: hover) {
  .item-row--selected:hover {
    background: color-mix(in srgb, var(--accent) 18%, var(--surface));
  }
}

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.item-data {
  display: flex;
  width: 100%;
  padding: 10px 10px;
  align-items: center;
  border-bottom: 1px solid var(--border);
  gap: 4px;
}

.entry-col, .exit-col {
  display: flex;
}

.action-btn {
  width: 70px;
  height: 30px;
  border-radius: 50px;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}

@media (hover: hover) {
  .action-btn:hover:not(:disabled) {
    background: var(--surface-2);
    border-color: var(--text-muted);
  }
}

.action-btn:disabled {
  cursor: not-allowed;
  opacity: 0.7;
}

.action-btn.entry-btn.active {
  background: var(--success-bg);
  color: var(--success-text);
  border-color: var(--success);
  font-weight: 600;
}

.action-btn.exit-btn.active {
  background: var(--danger-bg);
  color: var(--danger-text);
  border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
  font-weight: 600;
}

.status-text {
  color: var(--success-text);
  font-weight: 500;
}

.delete-btn {
  background: none;
  border: none;
  cursor: pointer;
  padding: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
}

@media (hover: hover) {
  .delete-btn:hover:not(:disabled) {
    background-color: transparent;
  }
}

.delete-btn:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.delete-icon {
  color: var(--text);
  width: 16px;
  height: 16px;
  opacity: 0.7;
  transition: opacity 0.2s ease;
}

@media (hover: hover) {
  .delete-btn:hover:not(:disabled) .delete-icon {
    opacity: 1;
  }
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

.loading-message {
  text-align: center;
  color: var(--text-muted);
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

.fade-list-enter-active,
.fade-list-leave-active {
  transition: all 0.3s ease;
}

.fade-list-enter-from,
.fade-list-leave-to {
  opacity: 0;
  transform: translateY(10px);
}

.fade-list-move {
  transition: transform 0.3s ease;
}

/* "Увеличенный режим": плавный переход в обе стороны.
   Transition объединён на .col (font-size + width + opacity), чтобы избежать
   конфликта специфичности (раньше .items-body .col перекрывал отдельные
   правила .status-col / .organization-col и данные не анимировались). */
.selected-table-card .col {
  transition:
    font-size 0.4s ease-in-out,
    flex-grow 0.4s ease-in-out,
    flex-basis 0.4s ease-in-out,
    max-width 0.4s ease-in-out,
    min-width 0.4s ease-in-out,
    padding 0.4s ease-in-out,
    opacity 0.3s ease-in-out;
}

/* Скрытый в "Увеличенный режим" столбец - не удаляем из DOM, а схлопываем.
   Транзишен на .col даёт плавную анимацию ширины/прозрачности. */
.selected-table-card .col.col--collapsed {
  flex-grow: 0 !important;
  flex-basis: 0 !important;
  min-width: 0 !important;
  max-width: 0 !important;
  padding-left: 0 !important;
  padding-right: 0 !important;
  opacity: 0;
  pointer-events: none;
  overflow: hidden;
  white-space: nowrap;
}

/* Пока конфиг не загружен - запрещаем transitions на всех потомках, чтобы
   шапка/столбцы не "ездили" между дефолтными и сохранёнными значениями
   при первом рендере. */
.selected-table-card.config-not-ready,
.selected-table-card.config-not-ready * {
  transition: none !important;
}

.selected-table-card .item-data {
  transition: min-height 0.4s ease-in-out;
}

.selected-table-card .status-col {
  overflow: hidden;
}

.selected-table-card.enlarged .items-body .col {
  font-size: 18px;
}

/* Номер Т/С и Марка - ключевые столбцы, не должны схлопываться по ellipsis
   при увеличенном шрифте. Даём фикс. min-width в enlarged-режиме. */
.selected-table-card.enlarged .number-col {
  min-width: 130px;
}

.selected-table-card.enlarged .brand-col {
  min-width: 110px;
}

.selected-table-card.enlarged .item-data {
  min-height: 36px;
}

@media (max-width: 767.98px) {
  /* Убирать рамку панели целиком владелец не просил - без неё "куда пропала
     таблица Машин? почему нету границ таблицы?" (талон читается разрозненными
     строками, а не таблицей). Радиус тот же, что на десктопе (30px) и что у
     таблицы «по факту» - владелец забраковал 15px волны 7 отдельно ("таблицам
     больше скругление нужно дать, как и было"). Против «квадрата в квадрате»
     отвечают уже сами строки ниже: скругление получают только верхний край
     первой и нижний последней, а не каждая. Заголовок отделяет линия снизу. */
  .selected-table-card {
    max-height: none;
    height: auto;
    border: 1px solid var(--border);
    border-radius: 30px;
    background: var(--surface);
  }

  /* Шапка блока - один ряд в 48px (контракт волны 6, те же числа у «Моих
     сотрудников» и «Доступных мне»): имя экрана кеглем 18, счётчик и «Обновить» у
     правого края, переноса нет.

     Перенос и был «кучей пустого места»: три группы контролов (счётчик с «Историей»
     и два тумблера) в ряд не влезали, уезжали второй строкой и раздували шапку до
     97px при вьюпорте 390. Лишнее уходит в переполнение - лист «⋯», - а не во
     вторую строку.

     Боковой отступ слагаемыми, а не числом: рамка карточки + внутренний отступ
     строки. Это ровно та вертикаль, на которой стоит текст карточек под шапкой
     (тело списка своего бокового отступа больше не добавляет - см. `.items-body`
     ниже, лишний слой давал заявленный владельцем "лишний боковой отступ").

     `min-height` из базовых стилей (50px) сбрасываем - с ним высота 48 не сойдётся. */
  .card-header {
    flex-direction: row;
    flex-wrap: nowrap;
    align-items: center;
    gap: 8px;
    height: 48px;
    min-height: 0;
    padding: 0 calc(1px + 14px);
  }

  .card-title {
    font-size: 18px;
  }

  .card-title__tail {
    display: none;
  }

  .items-count__label {
    display: none;
  }

  .items-count__label-short {
    display: inline;
  }

  /* «История» переехала в лист «⋯» (TablesComponent) - в ряду для неё места нет, а
     открывают её редко. Тумблер увеличенного режима скрыт по той же причине, что и
     «Сетка»: оба про геометрию столбцов, а на телефоне строки идут карточками. */
  .history-btn,
  .enlarged-toggle {
    display: none;
  }

  /* Режим мог остаться включённым с десктопа (он помнится в localStorage): там он
     прячет столбец статуса прозрачностью, а в карточке это не узкий столбец, а
     пустая строка на месте бейджа. */
  .selected-table-card.enlarged .status-col {
    opacity: 1;
    pointer-events: auto;
  }

  /* flex-basis именно 0, а не auto: перенос строк во flex считается по
     ГИПОТЕТИЧЕСКИМ размерам элементов, до применения flex-shrink. При auto
     гипотетический размер заголовка равен его тексту (замер на 320: 230px), и
     230 + 12 gap + 36 кнопки не влезали в 246 доступных (320 - 40 padding
     страницы - 2 рамки карточки - 32 padding шапки) - «Обновить» уезжала на
     вторую строку, хотя ellipsis у заголовка есть. С basis 0 строка не ломается,
     заголовок дорастает остатком (198px) и ужимается многоточием. */
  .card-header__title {
    flex: 1 1 0;
    min-width: 0;
  }

  .card-header__refresh {
    order: 1;
    flex-shrink: 0;
  }

  /* Шапка занимает несколько строк и растёт по контенту - фиксированный оверлей
     .bulk-bar (50px) накрыл бы только верх, хвост торчал бы под ним. Возвращаем
     панель в поток (образец CompaniesManagement, #1194). */
  .bulk-bar {
    position: static;
    height: auto;
    padding: 12px 16px;
    flex-wrap: wrap;
  }

  /* Три кнопки операций + "Снять выбор" не помещаются в строку на узком экране -
     переносим, чтобы не утекали в горизонтальный скролл (#1097 S8/#1114). */
  .bulk-actions {
    flex-wrap: wrap;
    margin-left: 0;
    width: 100%;
  }

  /* Счётчик остаётся в ряду шапки (на него смотрит шаг обучения `ob-on-territory`),
     но своей строки больше не занимает: ширина по содержимому, к правому краю его
     подводит автополе, а «Обновить» идёт следом по order.

     Автополе только у первого из двух: два `margin-left: auto` подряд делят
     свободное место между собой и растаскивают группы по краям. */
  .card-header__settings,
  .card-header__actions {
    order: 0;
    width: auto;
    flex-shrink: 0;
    flex-wrap: nowrap;
    gap: 8px;
    margin-left: auto;
    justify-content: flex-end;
  }

  .card-header__actions ~ .card-header__settings {
    margin-left: 0;
  }

  /* Талон - не карточка со своим зазором (паттерн Центра), а строка настоящей
     таблицы: рамку и фон блока теперь даёт сам `.selected-table-card` (выше).
     Полный бордер+радиус части 1 responsive-tables.css у КАЖДОЙ строки поверх
     рамки контейнера и был «квадратом в квадрате». Строки идут вплотную (зазора
     нет), разделяет только горизонтальная черта; скругление контейнера получают
     исключительно верхний край первой строки и нижний край последней - середина
     остаётся прямоугольной. Радиус строки равен радиусу контейнера (30px) - строка
     стоит вплотную к рамке (см. `.items-body` ниже), поэтому кривые продолжают
     друг друга без излома. */
  .item-row + .item-row {
    margin-top: 0;
  }

  .selected-table-card .rt-pass {
    border-radius: 0 !important;
    border-left: none !important;
    border-right: none !important;
    border-top: none !important;
    background: transparent !important;
  }

  .selected-table-card .item-row:not(:last-child) .rt-pass {
    border-bottom: 2px solid var(--border) !important;
  }

  .selected-table-card .item-row:last-child .rt-pass {
    border-bottom: none !important;
  }

  .selected-table-card .item-row:first-child .rt-pass {
    border-top-left-radius: 30px !important;
    border-top-right-radius: 30px !important;
  }

  .selected-table-card .item-row:last-child .rt-pass {
    border-bottom-left-radius: 30px !important;
    border-bottom-right-radius: 30px !important;
  }

  /* Тело списка без бокового отступа - строка стоит вплотную к рамке карточки,
     её собственный `padding: 10px 14px` (часть 1 responsive-tables.css) уже даёт
     воздух вокруг текста. Добавленные волной 7 8px были лишним отступом (жалоба
     владельца) и вдобавок отрывали разделитель строк и линию отрыва талона
     (`.rt-pass::before`, её расчёт базиса рассчитан ровно на этот случай - строка
     вплотную к краю карточки) от рамки на те же 8px с каждой стороны - обе
     линии не доставали до краёв. `.card-header` above выравнивается по той же
     вертикали через `calc(1px + 14px)`. Асимметричный зазор десктопного
     скроллбара (padding-right 4 + margin-right 4 в базовых стилях) на мобилке не
     нужен - скролл тач, и margin-right снимаем. */
  .items-body {
    padding: 0;
    margin-right: 0;
  }

  /* Первая загрузка на мобилке: `.selected-table-card{height:auto}` выше снимает
     десктопный `max-height`, и без него спиннер `.loading-message` сжимается по
     контенту (~25px) - страница остаётся короче вьюпорта, палец сразу упирается
     в конец документа, и жест замирает до отпускания (замер: непрерывный драг
     сразу после захода на таблицу стоял на месте весь ход жеста, пока список не
     дозагрузится). Резерв высоты только на время спиннера - список после загрузки
     сам определяет высоту страницы, лишнего пустого блока не остаётся. Возвращён
     вместе с прокруткой документа (волна 14) - без внутренней панели вьюпорта
     обрыв жеста снова возможен. */
  .loading-message {
    min-height: calc(var(--app-vh, 1vh) * 45);
  }

  /* Значения в карточке не обрезаем многоточием - там больше горизонтального
     места, чем в узкой табличной колонке. */
  .selected-table-card .rt-row > [data-label]:not(.col--collapsed) {
    white-space: normal;
    overflow: visible;
    text-overflow: clip;
  }

  /* Приоритет-схлопнутые (col--collapsed) колонки в card-режиме прячем совсем -
     их поля доступны в панели "Подробнее". Иначе card-override overflow:visible
     перебивает overflow:hidden схлопывания (равная специфичность, правило ниже) и
     колонка становится пустой раздутой строкой карточки вместо исчезновения. */
  .selected-table-card .rt-row > .col--collapsed {
    display: none !important;
  }

  /* #1097 S9. Обёртку полосы заголовков убираем целиком, а не только её внутренний ряд:
     глобальный `rt-head-row` прячет `.header-row`, а `.items-header` остаётся в потоке
     со своим `border-bottom` и рисует лишнюю линию в 1px перед первой карточкой
     (замер: height 1 при вьюпорте 320 и 390). Ловушка описана в эталоне, §8.

     Селектор длиннее собственного `.items-header`, чтобы исход не зависел от порядка
     правил: базовое правило стоит выше по файлу, но при равной специфичности его хватило
     бы перенести ниже, чтобы линия вернулась. Закреплению полосы это не мешает - оно
     живёт в `@media (min-width: 768px)` и сюда не достаёт. */
  .selected-table-card .items-header {
    display: none;
  }

  /* #1097 S9, переработано в волне 5 по мокапу docs/mockups/mobile-ux.html (экран
     «Проходная»): строка машины на телефоне - отрывной талон. Сверху пара кнопок
     прохода, под ними линия отрыва, дальше номер крупно и данные, внизу статус и
     действия.

     Кнопки прохода до S9 стояли двумя отдельными строками, и слева от каждой висела
     дублирующая подпись - "Въезд" подписью и "Въезд" кнопкой в одной строке. Поэтому
     карточка переведена из колонки в строку с переносом, а перенос во флексе держит
     БАЗИС, а не ширина.

     Специфичность выше правил-источников и `!important` обязательны: те объявлены с
     `!important` сами, и более коротким селектором их не перебить. */
  .selected-table-card .item-data.rt-row {
    flex-direction: row !important;
    flex-wrap: wrap !important;
    column-gap: 8px;
    row-gap: 0;
  }

  /* Доли столбцов заданы через `flex: N 0 0`, то есть с базисом 0. В колонке базис
     управлял высотой и не мешал, а в строке он и есть ширина: `width: 100%` из
     responsive-tables.css при нулевом базисе не считается вовсе, и ячейки делят одну
     строку по табличным долям - кнопка прохода в своей 14-пиксельной ячейке при этом
     вылезает за неё и накрывает соседей. Базис задаём явно: своя строка каждой ячейке.

     Правило целит во ВСЕ дочерние ячейки, а не в `[data-label]`: колонка без подписи
     (действия, "Подробнее") иначе осталась бы со своей табличной долей и уехала бы в
     ряд к кнопкам. Из этого правила выходят только сами кнопки прохода - ниже. */
  .selected-table-card .rt-row > * {
    flex: 0 0 100% !important;
    width: 100% !important;
    min-width: 0 !important;
  }

  /* Значения влево. Геометрия строки поля и отсутствие пунктира между полями -
     общие для талона и живут в responsive-tables.css (часть 3). */
  .selected-table-card .rt-row > [data-label] {
    justify-content: flex-start !important;
    text-align: left !important;
    border-bottom: none !important;
  }

  /* Ячейки прохода делят верхнюю строку пополам - единственные, кто выходит из
     «своя строка каждому». Это шапка талона: то, ради чего экран открывают.

     Базис ровно половина, а не 0 с ростом: перенос строк во flex считается по
     базисам ДО распределения свободного места, поэтому при нулевом базисе в первую
     строку набиралась ещё и следующая ячейка (её базис 100% как раз укладывался в
     остаток), свободного места не оставалось, и обе кнопки схлопывались в 6px друг
     на друга. С половиной третья ячейка в строку не входит и уезжает вниз. */
  .selected-table-card .rt-row > .entry-col,
  .selected-table-card .rt-row > .exit-col {
    width: auto !important;
    flex: 0 0 calc(50% - 4px) !important;
    padding: 5px 0 !important;
    border-top: none !important;
  }

  .selected-table-card .rt-row > .entry-col .action-btn,
  .selected-table-card .rt-row > .exit-col .action-btn {
    width: 100%;
    min-width: 0;
  }

  /* Статус в карточке телефона не нужен: экран открывают ради проезда, а не ради
     состояния заявки, и своей строкой в подвале он только оттягивал место у кнопок
     «Подробнее»/«Удалить» - было `order:9999; flex:0 0 100%`, разворачивающие его
     отдельным рядом. Прячем целиком.

     Подвал талона: «Подробнее» и «Удалить» пополам, пилюлями в 28px. Порядок задаём
     обоим заведомо большими числами, а не правим один столбец: разметочные `order`
     служебных (9998 и 9999) соседствуют с порядком настраиваемых столбцов.

     Базис половины, а не «0 с ростом»: перенос во flex считается по базисам ДО
     распределения свободного места, и с нулевым базисом в строку набиралась бы ещё
     и следующая ячейка, а кнопки схлопывались бы друг на друга (уже случалось с
     кнопками прохода). Единственной кнопке (нет права на удаление либо прятать
     нечего) достаётся левая половина - к правому краю она не жмётся. */
  .selected-table-card .rt-pass > .status-col {
    display: none !important;
  }

  .selected-table-card .rt-pass > .expand-col {
    order: 10000 !important;
  }

  .selected-table-card .rt-pass > .actions-col {
    order: 10001 !important;
  }

  /* `overflow: visible` обязателен: базовый `.col { overflow: hidden }` обрезает
     невидимый ::before, которым кнопки подвала добирают зону нажатия до 44px, - палец
     мимо пилюли попадал бы в пустоту. */
  .selected-table-card .rt-pass > .expand-col,
  .selected-table-card .rt-pass > .actions-col {
    flex: 0 0 calc(50% - 4px) !important;
    width: auto !important;
    overflow: visible;
    padding: 10px 0 0;
  }

  .selected-table-card .rt-row > [data-label]::before {
    display: none !important;
  }

  /* Исключение из "убрать все подписи": значение, которое без подписи не отличить от
     соседнего такого же. Организация и компания идут двумя строками с однотипными
     названиями; номер заявки, место разгрузки, дата и время - голые значения, которые
     сами себя не называют. Номер Т/С, марка и бейдж статуса говорят за себя. */
  .selected-table-card .rt-row > .organization-col::before,
  .selected-table-card .rt-row > .company-col::before,
  .selected-table-card .rt-row > .application-col::before,
  .selected-table-card .rt-row > .place-col::before,
  .selected-table-card .rt-row > .date-col::before,
  .selected-table-card .rt-row > .time-col::before {
    display: block !important;
  }

  /* Кнопки прохода - главное действие экрана, но не 44px "огромные": замер на
     карточке 370px давал 158x44 - тач-таргет для двух кнопок в половину строки
     взят с большим запасом. Норма проекта для контролов такого калибра - 36px
     (эталон §18); кегль и вес приведены к соседней пилюле «Удалить» (12.5px/600). */
  .action-btn {
    min-width: 70px;
    height: 36px;
    font-size: 13px;
    font-weight: 600;
  }

  /* Шеврон в пилюле «Подробнее» показывает раскрытие поворотом - саму пилюлю при
     этом не вертим, её `transform: none` приходит из rt-pass. Правило своё, а не
     общее: «Подробнее» есть только здесь, у таблицы «по факту» такой кнопки нет. */
  .selected-table-card .rt-pass > .expand-col .expand-btn svg {
    transition: transform 0.2s ease;
  }

  .selected-table-card .rt-pass > .expand-col .expand-btn--open svg {
    transform: rotate(180deg);
  }
}

/* Полоса заголовков столбцов не уезжает при прокрутке страницы (#1097 S8 волна 4).
   Список прокручивается и внутри карточки (.items-body), но саму карточку на
   планшете видно не целиком - страница прокручивается вместе с ней, и статичная
   полоса уходила за верх экрана: столбцы оставались без подписей.

   Карточка и её содержимое режутся `clip`, а не `hidden`: `hidden` делает предка
   скроллпортом, и sticky внутри него замирает на месте (прилипать не к чему).
   `clip` обрезает ровно так же - скругление 30px цело, - но скроллпорта не
   создаёт, поэтому отсчёт идёт от прокрутки документа. Там же живут шапка
   приложения и шапки списков (эталон: все закреплённые полосы в одной системе
   отсчёта). Браузер без поддержки `clip` просто оставит прежний `hidden` и
   прежнее поведение.

   Фон обязателен и обязан быть непрозрачным - строки уходят ПОД полосу;
   --surface в обеих палитрах задан hex-ом, без альфы. z-index 3: выше оверлея
   обновления (2) и .items-container (position: relative, идёт следом в разметке),
   ниже панели групповых операций (6).

   На мобилке правило не действует - там шапка скрыта (rt-head-row), строки
   показываются карточками. */
@media (min-width: 768px) {
  /* min-height здесь не украшение: `hidden` заодно обнулял автоминимум flex-элемента
     (оба - карточка в колоночном .tables__content и .card-content в карточке), и
     без него потолок 575px проиграл бы содержимому - min всегда бьёт max. Задаём
     нулевой минимум явно, чтобы высота не зависела от того, как браузер трактует
     автоминимум при `clip`. */
  .selected-table-card,
  .card-content {
    overflow: clip;
    min-height: 0;
  }

  .items-header {
    position: sticky;
    top: 0;
    z-index: 3;
    background: var(--surface);
  }
}

/* Ровно на 768 (планшет в портрете) шапка приложения ещё закреплена - её
   медиазапрос max-width: 768px, высота = токен. Полоса заголовков встаёт под
   неё, иначе прилипает к верху экрана и прячется за шапкой (z-index 100). */
@media (min-width: 768px) and (max-width: 768px) {
  .items-header {
    top: var(--mobile-header-height);
  }
}

/* #345 Phase 1D: размер шрифта строк через CSS-переменную с дефолтом 14px.
   Применяется ТОЛЬКО к телу таблицы - заголовки сохраняют исходный размер. */
.selected-table-card .items-body .col {
  font-size: var(--table-font-size, 14px);
}

/* #345 Phase 1E: плотность строк управляет вертикальным padding в ячейках. */
.selected-table-card.density-compact .item-data,
.selected-table-card.density-compact .header-row {
  padding-top: 4px;
  padding-bottom: 4px;
}

.selected-table-card.density-spacious .item-data,
.selected-table-card.density-spacious .header-row {
  padding-top: 16px;
  padding-bottom: 16px;
}

/* На мобилке строки показываются карточками - сетка не применяется, тумблер
   там не нужен. */
@media (max-width: 767.98px) {
  .grid-toggle {
    display: none;
  }
}

/* Служебные столбцы не проходят через getColStyle, поэтому минимум ширины
   задаём здесь: без него режим «Сетка» (overflow: clip) обнулял их
   автоминимум и ширины столбцов прыгали при включении (#1307). */
.entry-col,
.exit-col { min-width: 86px; }
.actions-col { min-width: 44px; }
.expand-col { min-width: 44px; }

/* Режим "Сетка" (#1289): вертикальные линии между колонками. Горизонтальные
   линии и внешний контур уже дают border-bottom строк и рамка карточки.
   На мобилке строки превращаются в карточки (rt-row), поэтому блок живёт от 768px.

   Включение режима ничего не двигает: отступы, выравнивание и размеры ячеек
   остаются прежними при любых настройках размера шрифта и плотности строк.
   Линия - псевдоэлемент; ячейке разрешён вынос за свои границы (overflow: clip
   вместо hidden, обрезка содержимого и ellipsis при этом сохраняются), а по
   высоте строки линию подрезает сама строка. Так линия идёт от края до края
   независимо от того, насколько содержимое ячейки ниже строки. */
@media (min-width: 768px) {
  .selected-table-card.grid-mode .header-row,
  .selected-table-card.grid-mode .item-data {
    overflow: clip;
  }

  .selected-table-card.grid-mode .header-row > .col,
  .selected-table-card.grid-mode .item-data > .col {
    position: relative;
    overflow: clip;
    /* Заведомо больше любой строки - лишнее подрежет строка. */
    overflow-clip-margin: 200px;
  }

  .selected-table-card.grid-mode .header-row > .col::after,
  .selected-table-card.grid-mode .item-data > .col::after {
    content: '';
    position: absolute;
    top: -200px;
    right: 0;
    bottom: -200px;
    width: 1px;
    background: var(--border);
  }

  /* У схлопнутой колонки (увеличенный режим, приоритет) нулевая ширина - её
     линия легла бы поверх соседней. Последняя колонка упирается в рамку
     карточки, своя линия ей не нужна. */
  .selected-table-card.grid-mode .col--collapsed::after,
  .selected-table-card.grid-mode .header-row > .col:last-child::after,
  .selected-table-card.grid-mode .item-data > .col:last-child::after {
    display: none;
  }
}

/* #345 Phase 1F: портретный режим (узкий экран с orientation: portrait). */
.expand-col {
  flex: 2.5 0 0;
  display: flex;
  justify-content: center;
  align-items: center;
}

.expand-btn {
  width: 22px;
  height: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-muted);
  cursor: pointer;
  transition: transform 0.2s ease, color 0.15s ease, background 0.15s ease;
}

@media (hover: hover) {
  .expand-btn:hover {
    background: var(--surface-2);
    color: var(--accent-text);
  }
}

.expand-btn--open {
  transform: rotate(180deg);
  color: var(--accent-text);
  background: var(--accent-tint);
}

/* Раскрытие "Подробнее" - стиль карточек label/value как в VehicleDetailsModal.
   Auto-fit grid, каждый item - flex-column с label сверху (мелкий серый)
   и value снизу (крупнее, тёмный, weight 500), в белой карточке с border. */
.item-row__details {
  padding: 12px 16px 14px;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 10px;
  background: var(--surface-2);
  border-top: 1px dashed var(--border);
}

.detail-item {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 4px;
  min-width: 0;
  padding: 10px 14px;
  background: var(--surface);
  border-radius: 20px;
  border: 1px solid var(--border);
}

.detail-item__label {
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 400;
  letter-spacing: 0.3px;
  white-space: nowrap;
}

.detail-item__value {
  color: var(--text);
  font-size: 14px;
  font-weight: 500;
  word-break: break-word;
  min-width: 0;
}
</style>