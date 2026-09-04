<template>
  <div
    class="selected-table-card"
    :class="[
      { 'enlarged': enlarged, 'grid-mode': grid, 'is-portrait': isCompact, 'config-not-ready': !configReady },
      `density-${rowDensity}`,
    ]"
    :style="{ '--table-font-size': tableFontSize + 'px' }"
    data-testid="people-table"
  >
    <div class="card-header">
      <div class="card-header__title">
        <!-- Хвост «по заявке» на телефоне скрыт: имя экрана кеглем 18 и так не
             помещается рядом со счётчиком и «Обновить», а различает таблицы первая
             половина - соседний блок называется «Люди по факту». -->
        <h3 class="card-title">
          <span class="blue">Люди</span><span class="card-title__tail"> по заявке</span>
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
          <span class="items-count__text"><span
            class="items-count__label"
          >Людей зашло:</span><span
            class="items-count__label-short"
          >На территории:</span> <AnimatedCounter :value="peopleOnTerritory" /></span>
          <button
            v-if="can(`table.${tableName}.history`)"
            class="history-btn"
            @click="openEmployeesHistory"
          >История</button>
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
      data-testid="people-bulk-bar"
    >
      <span class="bulk-count">Выбрано: {{ selectedCount }}</span>
      <div class="bulk-actions">
        <!-- Экспорт выбранных (#1194 S6) - клиентский ExcelJS по уже загруженным
             строкам, read-only. Гейт - то же право, что у полного экспорта
             (table.<name>.export), НЕ page.admin: доступно любому, кто видит таблицу. -->
        <button
          v-if="can(`table.${tableName}.export`)"
          class="lk-button lk-button--secondary lk-button--sm"
          data-testid="people-bulk-export"
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
            data-testid="people-bulk-move"
            @click="openBulkModal('move')"
          >
            Перенести
          </button>
          <button
            class="lk-button lk-button--secondary lk-button--sm"
            data-testid="people-bulk-add"
            @click="openBulkModal('add')"
          >
            Добавить в таблицу
          </button>
          <button
            class="lk-button lk-button--danger lk-button--sm"
            data-testid="people-bulk-remove"
            @click="openBulkRemoveConfirm"
          >
            Убрать
          </button>
        </template>
        <button
          class="lk-button lk-button--ghost lk-button--sm bulk-clear"
          data-testid="people-bulk-clear"
          @click="clearSelection"
        >
          Снять выбор
        </button>
      </div>
    </div>

    <div class="card-content rt-table">
      <div class="items-header">
        <div class="header-row rt-head-row">
          <div
            class="col entry-col"
            style="order: 0;"
          >
            Вход
          </div>
          <div
            class="col exit-col"
            style="order: 1;"
          >
            Выход
          </div>
          <div
            v-if="isFieldInDom('last_name')"
            class="col last-name-col"
            :class="fieldColClass('last_name')"
            :style="getColStyle('last_name', true)"
            @click="sortBy('last_name')"
          >
            <p :class="{ 'active-sort': sortField === 'last_name' }">
              Фамилия
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'last_name', 'desc': sortField === 'last_name' && sortDirection === 'desc' }"
            />
          </div>
          <div
            v-if="isFieldInDom('first_name')"
            class="col first-name-col"
            :class="fieldColClass('first_name')"
            :style="getColStyle('first_name', true)"
            @click="sortBy('first_name')"
          >
            <p :class="{ 'active-sort': sortField === 'first_name' }">
              Имя
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'first_name', 'desc': sortField === 'first_name' && sortDirection === 'desc' }"
            />
          </div>
          <div
            v-if="isFieldInDom('middle_name')"
            class="col middle-name-col"
            :class="fieldColClass('middle_name')"
            :style="getColStyle('middle_name', true)"
            @click="sortBy('middle_name')"
          >
            <p :class="{ 'active-sort': sortField === 'middle_name' }">
              Отчество
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'middle_name', 'desc': sortField === 'middle_name' && sortDirection === 'desc' }"
            />
          </div>
          <div
            v-if="isFieldInDom('position')"
            class="col position-col"
            :class="fieldColClass('position')"
            :style="getColStyle('position', true)"
            @click="sortBy('position')"
          >
            <p :class="{ 'active-sort': sortField === 'position' }">
              Должность
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'position', 'desc': sortField === 'position' && sortDirection === 'desc' }"
            />
          </div>
          <div
            v-if="isFieldInDom('citizenship_name')"
            class="col citizenship-col"
            :class="fieldColClass('citizenship_name')"
            :style="getColStyle('citizenship_name', true)"
            @click="sortBy('citizenship_name')"
          >
            <p :class="{ 'active-sort': sortField === 'citizenship_name' }">
              Гражданство
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'citizenship_name', 'desc': sortField === 'citizenship_name' && sortDirection === 'desc' }"
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
            v-if="isFieldInDom('pass_time')"
            class="col time-col"
            :class="fieldColClass('pass_time')"
            :style="getColStyle('pass_time', true)"
            @click="sortBy('pass_time')"
          >
            <p :class="{ 'active-sort': sortField === 'pass_time' }">
              Время прохода
            </p>
            <AppIcon
              name="sort"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'pass_time', 'desc': sortField === 'pass_time' && sortDirection === 'desc' }"
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
          <!-- Пустой spacer-заголовок над chevron-кнопкой "Подробнее" в строке. -->
          <div
            v-if="hiddenInPortraitFields().length"
            class="col expand-col"
            style="order: 9997;"
          />
          <div
            class="col status-col"
            style="order: 9998;"
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
          <div
            class="col actions-col"
            style="order: 9999;"
          />
        </div>
      </div>
      
      <div class="items-container">
        <div
          v-if="isLoading"
          class="loading-message"
        >
          <LoaderSpinner label="Загрузка сотрудников…" />
        </div>

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
              :style="{ animationDelay: `${index * 0.05}s` }"
              @click="preview ? null : onRowClick($event, item)"
              @mousedown="preview ? null : onRowMouseDown($event, item)"
              @mouseenter="preview ? null : dragOver(item.id)"
            >
              <!-- rt-pass: строка собирается талоном на мобилке (responsive-tables.css,
                   часть 3). Талон определяется наличием кнопок прохода, а не тем, машина
                   в строке или человек: сверху «Вход»/«Выход», ниже линия отрыва, под ней
                   ФИО одной строкой и данные, в подвале статус и действия. Разнобой с
                   таблицей машин на том же экране владелец засчитывает как дефект. -->
              <div class="item-data rt-row rt-pass">
                <div
                  class="col entry-col"
                  style="order: 0;"
                  data-label="Вход"
                  @click.stop
                >
                  <button
                    class="action-btn entry-btn"
                    :class="{ 'active': item.entry_checked }"
                    :disabled="preview || item.entry_checked"
                    @click="preview ? null : handleEntryExit(item, 'entry')"
                  >
                    Вход
                  </button>
                </div>
                <div
                  class="col exit-col"
                  style="order: 1;"
                  data-label="Выход"
                  @click.stop
                >
                  <button
                    class="action-btn exit-btn"
                    :class="{ 'active': item.exit_checked }"
                    :disabled="preview || !item.entry_checked || item.exit_checked"
                    @click="preview ? null : handleEntryExit(item, 'exit')"
                  >
                    Выход
                  </button>
                </div>
                <div
                  v-if="isFieldInDom('last_name')"
                  class="col last-name-col"
                  :class="fieldColClass('last_name')"
                  :style="getColStyle('last_name')"
                  data-label="Фамилия"
                >
                  {{ item.last_name }}
                </div>
                <div
                  v-if="isFieldInDom('first_name')"
                  class="col first-name-col"
                  :class="fieldColClass('first_name')"
                  :style="getColStyle('first_name')"
                  data-label="Имя"
                >
                  {{ item.first_name }}
                </div>
                <div
                  v-if="isFieldInDom('middle_name')"
                  class="col middle-name-col"
                  :class="fieldColClass('middle_name')"
                  :style="getColStyle('middle_name')"
                  data-label="Отчество"
                >
                  {{ item.middle_name || '-' }}
                </div>
                <div
                  v-if="isFieldInDom('position')"
                  class="col position-col"
                  :class="fieldColClass('position')"
                  :style="getColStyle('position')"
                  data-label="Должность"
                >
                  {{ item.position || '-' }}
                </div>
                <div
                  v-if="isFieldInDom('citizenship_name')"
                  class="col citizenship-col"
                  :class="fieldColClass('citizenship_name')"
                  :style="getColStyle('citizenship_name')"
                  data-label="Гражданство"
                >
                  {{ item.citizenshipName || '-' }}
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
                  v-if="isFieldInDom('valid_until')"
                  class="col date-col"
                  :class="fieldColClass('valid_until')"
                  :style="getColStyle('valid_until')"
                  data-label="Действует до"
                >
                  {{ formatDate(item.entry_date_to) }}
                </div>
                <div
                  v-if="isFieldInDom('pass_time')"
                  class="col time-col"
                  :class="fieldColClass('pass_time')"
                  :style="getColStyle('pass_time')"
                  data-label="Время прохода"
                >
                  {{ formatPassTime(item.pass_time) }}
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
                  class="col status-col"
                  style="order: 9998;"
                  data-label="Статус"
                >
                  <StatusBadge :status="item.status" />
                </div>
                <div
                  v-if="hiddenInPortraitFields().length"
                  class="col expand-col"
                  style="order: 9997;"
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
                    <!-- Подпись видна только в карточке: в таблице на месте кнопки
                         узкий служебный столбец, туда влезает лишь значок. -->
                    <span class="rt-pass__act-label">{{ expandedRows[item.id] ? 'Скрыть' : 'Подробнее' }}</span>
                  </button>
                </div>
                <div
                  v-if="can('entity.employees.delete')"
                  class="col actions-col"
                  style="order: 9999;"
                  @click.stop
                >
                  <!-- Сотрудник привязан к нескольким таблицам (#1194 S5) -
                       выбор между "убрать только отсюда" и глобальной
                       деактивацией. Единственная привязка ИЛИ не-админ - как
                       раньше (unbind-table гейтится page.admin на бэке). -->
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
                  <span class="detail-item__value">{{ portraitFieldValue(item, name) }}</span>
                </div>
              </div>
            </div>
          </transition-group>
        </div>

        <div
          v-else-if="!isLoading"
          class="no-data-message"
        >
          {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Нет активных сотрудников' }}
        </div>
      </div>
    </div>

    <!-- Модальное окно деталей сотрудника -->
    <EmployeeDetailsModal
      v-if="!preview && showDetailsModal"
      :show="showDetailsModal"
      :employee="selectedEmployee"
      :all-tables="allTables"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      :source="'peopletable'"
      @close="closeDetailsModal"
      @open-application="openApplicationDetail"
    />

    <EmployeesTableHistoryModal
      v-if="!preview && showEmployeesHistory"
      :table-id="currentTableId"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      @close="showEmployeesHistory = false"
    />

    <!-- Групповые операции "Перенести"/"Добавить в таблицу" (#1194) -->
    <TableBulkTargetModal
      v-if="!preview"
      :show="bulkModalVisible"
      :mode="bulkModalMode"
      entity-type="people"
      :exclude-table-id="currentTableId"
      :selected-count="selectedCount"
      :submitting="bulkSubmitting"
      @close="closeBulkModal"
      @apply="applyBulkTableOp"
    />

    <!-- Групповое "Убрать" (#1194 S5): снимает привязку к текущей таблице у
         выделенных строк; последняя привязка -> BE деактивирует сотрудника сам. -->
    <ConfirmationModal
      v-if="!preview"
      :show="bulkRemoveConfirmVisible"
      title="Убрать из таблицы"
      :message="`Убрать выбранных сотрудников (${selectedCount}) из этой таблицы? Если это последняя таблица сотрудника, он будет деактивирован.`"
      confirm-text="Убрать"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
      @confirm="confirmBulkRemove"
      @cancel="cancelBulkRemove"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import { idFilterSet } from '@/utils/idFilter';
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';
import { useOrientation } from '@/composables/useOrientation';
import { useRowSelection } from '@/composables/useRowSelection';
import eventStream from '@/services/eventStream';
import RefreshButton from './RefreshButton.vue';
import EmployeeDetailsModal from './CreateApplication/EmployeeDetailsModal.vue';
import EmployeesTableHistoryModal from './CreateApplication/EmployeesTableHistoryModal.vue';
import TableBulkTargetModal from './TableBulkTargetModal.vue';
import TableRowRemoveMenu from './TableRowRemoveMenu.vue';
import ConfirmationModal from './ConfirmationModal.vue';
import StatusBadge from './ui/StatusBadge.vue';
import SwitchToggle from './ui/SwitchToggle.vue';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import AnimatedCounter from './ui/AnimatedCounter.vue';
import ExcelJS from 'exceljs';
import { bulkMoveEmployeesTable, bulkAddEmployeesTable, bulkUnbindEmployeesTable } from '@/api/employees';
import { pickOverflowFields, columnMinWidth, measureRowAvailableWidth, SERVICE_COLUMNS_WIDTH } from '@/utils/tableColumnFit';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import AppIcon from '@/components/icons/AppIcon.vue';
import { formatMoscowDateTime } from '@/utils/serverTime';

const ENLARGED_KEY_PREFIX = 'enlarged-mode:people:';

/**
 * Состав карточки на телефоне: что видно сразу, остальное - в «Подробнее».
 *
 * Набор назван владельцем дословно - ФИО одной строкой, организация, срок действия.
 * Статус в него добавлен потому, что живёт не строкой поля, а бейджем в подвале.
 * Должность, гражданство, компания, номер заявки и время прохода уходят под кнопку.
 *
 * Подбор столбцов по ширине (#1307) карточке не подходит: поля стоят своими
 * строками и за ширину не конкурируют, а мерить он пытается скрытую строку
 * заголовков. Скатывался он всегда в одно и то же - оставить `keepAtLeast` столбца
 * из десяти, то есть фамилию и имя, - поэтому в карточке не было ни отчества, ни
 * организации, ни срока.
 */
const MOBILE_CARD_FIELDS = [
  'last_name',
  'first_name',
  'middle_name',
  'organization',
  'valid_until',
  'status',
];

export default {
  name: 'PeopleTable',
  components: {
    RefreshButton,
    EmployeeDetailsModal,
    EmployeesTableHistoryModal,
    TableBulkTargetModal,
    TableRowRemoveMenu,
    ConfirmationModal,
    StatusBadge,
    SwitchToggle,
    LoaderSpinner,
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
    tableName: {
      type: String,
      default: ''
    },
    searchQuery: {
      type: String,
      default: ''
    },
    // Мультивыбор (#1398): пустой массив - фильтр выключен. Дефолт обязателен -
    // preview-монтирования (вкладка «Колонки», версии таблицы) пропсы не передают.
    // Места разгрузки здесь нет намеренно: у сотрудников такой связи не существует,
    // фильтр применим только к машинам.
    selectedOrganizationIds: {
      type: Array,
      default: () => []
    },
    selectedCompanyIds: {
      type: Array,
      default: () => []
    },
    dateRangeStart: {
      type: Date,
      default: null
    },
    dateRangeEnd: {
      type: Date,
      default: null
    },
    selectedDate: {
      type: Date,
      default: null
    },
    currentUserId: {
      type: Number,
      default: null
    },
    currentUserName: {
      type: String,
      default: ''
    },
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
      currentTableId: null,
      organizationsMap: {},
      allTables: [],
      showDetailsModal: false,
      selectedEmployee: null,
      // Столбцы, не поместившиеся по ширине (#1307): скрываются от наименее
      // важных, при равном приоритете - правые. Значения остаются в «Подробнее».
      overflowFields: [],
      showEmployeesHistory: false,
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
      // false до первой загрузки конфига - класс config-not-ready на корне
      // подавляет transitions, чтобы шапка/столбцы не "ездили" при init.
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
            item.last_name,
            item.first_name,
            item.middle_name || '',
            item.organization_name,
            this.formatDate(item.entry_date_to),
            item.pass_time || '',
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
            case 'last_name':
            case 'first_name':
            case 'middle_name':
            case 'organization':
            case 'status':
            case 'pass_places':
              valueA = (a[this.sortField] || '').toString().toLowerCase();
              valueB = (b[this.sortField] || '').toString().toLowerCase();
              break;
            case 'citizenship':
              valueA = (a.citizenshipName || '').toString().toLowerCase();
              valueB = (b.citizenshipName || '').toString().toLowerCase();
              break;
            case 'entry_date_to':
              valueA = a.entry_date_to ? new Date(a.entry_date_to) : new Date(0);
              valueB = b.entry_date_to ? new Date(b.entry_date_to) : new Date(0);
              break;
            case 'pass_time':
              valueA = this.extractPassTime(a.pass_time);
              valueB = this.extractPassTime(b.pass_time);
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
      }

      return filtered;
    },

    peopleOnTerritory() {
      return this.itemsData.filter(item => item.entry_checked && !item.exit_checked).length;
    },

    hasActiveFilters() {
      return !!(
        this.searchQuery ||
        idFilterSet(this.selectedOrganizationIds) ||
        idFilterSet(this.selectedCompanyIds) ||
        this.selectedDate ||
        (this.dateRangeStart && this.dateRangeEnd)
      );
    }
  },
  watch: {
    tableName: {
      handler() {
        if (this.preview) return;
        this.stopPolling();
        this.startPolling();
        this.loadEnlargedFromStorage();
      },
      immediate: true
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
    },
    // Real-time (#840): tableId для PeopleTable известен только после первого fetchPeopleData
    // (резолвится по tableName), поэтому подписываемся/пересобираем по watch, а не в mounted.
    currentTableId(newVal) {
      if (this.preview) return;
      this.subscribeTableScope(newVal);
    }
  },
  mounted() {
    if (this.preview) return;
    this.$nextTick(() => this.bindWidthWatcher());
    this.startPolling();
    this.loadEnlargedFromStorage();
    // Real-time (#840): по сигналу продюсера tables.refresh тихо перезагружаем строки
    // вместо ожидания поллинга. Сама подписка на scope - в watch currentTableId.
    eventStream.connect();
    this.eventStreamStatusOff = eventStream.onStatus((status) => {
      this.sseConnected = status === 'connected';
    });

    // Drag-select (#1227 P4): mouseup может произойти вне строки (курсор ушёл
    // за пределы таблицы) - слушаем на window, иначе drag "залипнет".
    this.onGlobalMouseUp = () => this.endDrag();
    window.addEventListener('mouseup', this.onGlobalMouseUp);
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
    // seq-токен против гонки конкурентных вызовов (поллинг + SSE-сигнал, #632/#840):
    // устаревший (медленно резолвнутый) ответ не должен затирать более свежие данные.
    async _loadData(silent = false) {
      const seq = ++this.refreshSeq;
      if (!silent && this.isLoading) return;
      if (!silent) this.isLoading = true;
      try {
        await this.fetchAllTables();
        await this.fetchOrganizations();
        await this.fetchPeopleData(seq, silent);
        await this.fetchEmployeesStatus(seq);
        return true;
      } catch (error) {
        console.error('Ошибка при загрузке людей:', error);
        return false;
      } finally {
        if (!silent) this.isLoading = false;
      }
    },

    async loadData() {
      this.refreshing = true;
      try {
        const ok = await this._loadData(true);
        // Тихий сбой оставляет прежние строки (не чистим таблицу), но на ЯВНОЕ
        // обновление пользователь должен получить сигнал, что данные не свежие.
        if (!ok) {
          useDeletionsStore().notify({ prefix: 'Не удалось обновить таблицу: ', bold: 'показаны последние данные', type: 'error' });
        }
      } finally {
        this.refreshing = false;
      }
    },

    async silentRefresh() {
      await this._loadData(true);
    },

    async fetchAllTables() {
      try {
        const response = await apiRequest("/system-tables", { method: "GET" });
        if (response.ok) {
          this.allTables = await response.json();
        }
      } catch (error) {
        console.error("Ошибка при загрузке таблиц:", error);
      }
    },

    async fetchOrganizations() {
      try {
        const response = await apiRequest("/organizations", { method: "GET" });
        if (response.ok) {
          const data = await response.json();
          this.organizationsMap = {};
          data.forEach(org => { this.organizationsMap[org.id] = org.name; });
        }
      } catch (error) {
        console.error("Ошибка при загрузке организаций:", error);
      }
    },

    async fetchPeopleData(seq, silent = false) {
      try {
        if (!this.tableName) return;

        const tableRes = await apiRequest(`/system-tables/name/${this.tableName}`, { method: "GET" });
        if (!tableRes.ok) return;
        const responseData = await tableRes.json();
        const table = responseData.table;
        this.currentTableId = table.id;

        // Подтягиваем конфиг видимости, порядка, ширины и приоритета столбцов
        // из того же ответа. Стиль таблицы (font_size/row_density) - из table.
        const nextVisibility = {};
        const nextOrders = {};
        const nextWidths = {};
        const nextPriorities = {};
        const nextEnlVis = {};
        const nextEnlW = {};
        const nextEnlWeight = {};
        (responseData.fields || []).forEach(f => {
          nextVisibility[f.field_name] = f.is_visible !== false;
          if (typeof f.display_order === 'number') {
            nextOrders[f.field_name] = f.display_order;
          }
          if (typeof f.width === 'number' && f.width > 0) {
            nextWidths[f.field_name] = f.width;
          }
          if (typeof f.priority === 'number' && f.priority > 0) {
            nextPriorities[f.field_name] = f.priority;
          }
          nextEnlVis[f.field_name] = f.enlarged_is_visible !== false;
          if (typeof f.enlarged_width === 'number' && f.enlarged_width > 0) {
            nextEnlW[f.field_name] = f.enlarged_width;
          }
          if (typeof f.enlarged_font_weight === 'number' && f.enlarged_font_weight > 0) {
            nextEnlWeight[f.field_name] = f.enlarged_font_weight;
          }
        });
        this.fieldsVisibility = nextVisibility;
        this.fieldOrders = nextOrders;
        this.fieldWidths = nextWidths;
        this.fieldPriorities = nextPriorities;
        this.fieldsEnlargedVisibility = nextEnlVis;
        this.fieldsEnlargedWidth = nextEnlW;
        this.fieldsEnlargedWeight = nextEnlWeight;
        const fs = Number(table.font_size);
        if (fs >= 10 && fs <= 24) this.tableFontSize = fs;
        const dens = table.row_density;
        if (['compact', 'normal', 'spacious'].includes(dens)) this.rowDensity = dens;
        this.markConfigReady();

        const employeesRes = await apiRequest(`/employees/active-for-table/${table.id}`, { method: "GET" });
        if (!employeesRes.ok) return;
        const employees = await employeesRes.json();

        const nameToIdMap = {};
        Object.keys(this.organizationsMap).forEach(id => {
          nameToIdMap[this.organizationsMap[id]] = id;
        });

        // Территориальное состояние уже отрисованных строк - страховка на случай,
        // если строка пришла без territory_status: без неё каждая перезагрузка
        // (real-time сигнал, поллинг) обнуляла бы отметки входа и счётчик зашедших
        // проваливался бы в 0 до ответа /employees/history/current-status.
        const prevTerritory = new Map(
          this.itemsData.map(item => [item.id, {
            entry_checked: item.entry_checked,
            exit_checked: item.exit_checked,
            territory_status: item.territory_status,
          }])
        );

        const newItems = employees.map(emp => {
          const orgName = emp.organization || '';
          const orgId = nameToIdMap[orgName] || emp.organization_id;
          // Статус берём из самой строки (та же колонка employees.territory_status,
          // что читает current-status), а при его отсутствии - из предыдущего
          // состояния строки. Так отметки входа/выхода и счётчик не мигают.
          const prev = prevTerritory.get(emp.id);
          const territoryStatus = emp.territory_status ?? prev?.territory_status ?? 0;
          return {
            id: emp.id,
            last_name: emp.last_name || '',
            first_name: emp.first_name || '',
            middle_name: emp.middle_name || '',
            organization_id: orgId,
            organization_name: orgName || 'Не указана',
            entry_date_to: emp.entry_date_to || '',
            pass_time: emp.pass_time || '',
          pass_places: emp.pass_places || '',
            status: 'Активен',
            applicationId: emp.application_id,
            applicationNumber: emp.application_number,
            target_tables: emp.target_tables || [],
            passport_series_number: emp.passport_series_number,
            patent_number: emp.patent_number,
            other_permission: emp.other_permission,
            citizenshipName: emp.citizenship_name,
            position: emp.position,
            company: emp.company,
            company_id: emp.company_id,
            entry_checked: territoryStatus === 1,
            exit_checked: territoryStatus === 2,
            territory_status: territoryStatus,
            // Число таблиц «Проход», к которым привязан сотрудник (#1194 S5) -
            // >1 включает per-row подменю «Убрать из этой/из всех».
            target_tables_count: emp.target_tables_count || 0
          };
        });
        if (seq !== undefined && seq !== this.refreshSeq) return; // устарел - новее уже в работе/загружен
        this.itemsData = newItems;
      } catch (error) {
        console.error("Ошибка при загрузке сотрудников:", error);
        // Тихое обновление (real-time сигнал, поллинг) при сбое сети оставляет
        // последние известные строки: очистка стирала бы таблицу и счётчик под
        // пользователем на ровном месте (тот же класс, что обнуление счётчика #1021).
        if (!silent && (seq === undefined || seq === this.refreshSeq)) this.itemsData = [];
      }
    },

    async fetchEmployeesStatus(seq) {
      try {
        const response = await apiRequest("/employees/history/current-status", { method: "GET" });
        if (response.ok) {
          const statuses = await response.json();
          if (seq !== undefined && seq !== this.refreshSeq) return; // устарел
          const statusMap = {};
          statuses.forEach(status => { statusMap[status.employee_id] = status; });
          this.itemsData.forEach(item => {
            const status = statusMap[item.id];
            if (status) {
              item.territory_status = status.territory_status;
              item.entry_checked = status.territory_status === 1;
              item.exit_checked = status.territory_status === 2;
            }
          });
        }
      } catch (error) {
        console.error("Ошибка при загрузке статусов территории:", error);
      }
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

    formatPassTime(passTime) {
      if (!passTime) return '-';
      const [timeFrom, timeTo] = passTime.split('-');
      const formatTime = (timeStr) => {
        if (!timeStr) return '';
        const parts = timeStr.trim().split(':');
        if (parts.length >= 2) return `${parts[0]}:${parts[1]}`;
        return timeStr;
      };
      const formattedFrom = formatTime(timeFrom);
      const formattedTo = formatTime(timeTo);
      if (!formattedTo) return formattedFrom;
      if (!formattedFrom) return formattedTo;
      return `${formattedFrom} - ${formattedTo}`;
    },

    extractPassTime(passTime) {
      if (!passTime || passTime === '-') return 0;
      const startTime = passTime.split('-')[0];
      const parts = startTime.split(':');
      if (parts.length >= 2) {
        const hours = parseInt(parts[0]) || 0;
        const minutes = parseInt(parts[1]) || 0;
        return hours * 60 + minutes;
      }
      return 0;
    },

    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'desc';
      }
    },

    async handleEntryExit(item, type) {
      if (!this.currentUserId || !this.currentTableId) return;
      const territory_status = type === 'entry' ? 1 : 2;
      try {
        const response = await apiRequest(`/employees/${item.id}/territory-status`, {
          method: "PUT",
          body: JSON.stringify({
            territory_status,
            user_id: this.currentUserId,
            table_id: this.currentTableId
          })
        });
        if (response.ok) {
          item.entry_checked = type === 'entry';
          item.exit_checked = type === 'exit';
        } else {
          console.error('Ошибка при обновлении статуса');
        }
      } catch (error) {
        console.error('Ошибка сети:', error);
      }
    },

    removeItemWithNotification(item) {
      if (this.isLoading) return;
      if (this.pendingDeleteIds.includes(item.id)) return;
      const empId = item.id;
      const tableId = this.currentTableId;
      const userId = this.currentUserId;
      const fullName = [item.last_name, item.first_name, item.middle_name].filter(Boolean).join(' ') || String(item.last_name || '');
      this.pendingDeleteIds.push(empId);
      useDeletionsStore().enqueue({
        prefix: 'Сотрудник ',
        bold: fullName,
        suffix: ' удалён',
        onConfirm: () => this.commitDelete(empId, tableId, userId),
        onUndo: () => this.unhidePending(empId),
      });
    },

    unhidePending(empId) {
      this.pendingDeleteIds = this.pendingDeleteIds.filter(id => id !== empId);
    },

    async commitDelete(empId, tableId, userId) {
      try {
        const response = await apiRequest(`/employees/${empId}/deactivate`, {
          method: "PUT",
          body: JSON.stringify({ status: 0, user_id: userId, table_id: tableId })
        });
        if (!response.ok) {
          console.error("Ошибка при удалении");
          this.unhidePending(empId);
          return;
        }
        await this._loadData(true);
        this.unhidePending(empId);
      } catch (error) {
        console.error("Ошибка сети при удалении:", error);
        this.unhidePending(empId);
      }
    },

    // Убрать ТОЛЬКО из текущей таблицы (#1194 S5) - альтернатива глобальной
    // деактивации, доступная per-row через TableRowRemoveMenu, когда сотрудник
    // привязан к нескольким таблицам. Тот же enqueue/undo UX, что и обычное
    // удаление, коммит идёт через bulkUnbindEmployeesTable ([id], tableId).
    removeFromCurrentTableWithNotification(item) {
      if (this.isLoading) return;
      if (this.pendingDeleteIds.includes(item.id)) return;
      const empId = item.id;
      const tableId = this.currentTableId;
      const fullName = [item.last_name, item.first_name, item.middle_name].filter(Boolean).join(' ') || String(item.last_name || '');
      this.pendingDeleteIds.push(empId);
      useDeletionsStore().enqueue({
        prefix: 'Сотрудник ',
        bold: fullName,
        suffix: ' убран из таблицы',
        onConfirm: () => this.commitUnbindFromCurrentTable(empId, tableId),
        onUndo: () => this.unhidePending(empId),
      });
    },

    async commitUnbindFromCurrentTable(empId, tableId) {
      try {
        const result = await bulkUnbindEmployeesTable([empId], tableId);
        if (!result || typeof result.success_count !== 'number' || result.error_count > 0) {
          const message = result?.errors?.[0]?.error || result?.message || 'ошибка сервера';
          useDeletionsStore().notify({ prefix: 'Не удалось убрать сотрудника: ', bold: message, type: 'error' });
        }
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось убрать сотрудника: ', bold: 'ошибка сети', type: 'error' });
      } finally {
        await this._loadData(true);
        this.unhidePending(empId);
      }
    },

    // Ctrl/Shift-клик по строке (#1194) - групповое выделение вместо открытия
    // детали; обычный клик поведение не меняет (handleRowClick вернёт false).
    onRowClick(event, item) {
      const orderedIds = this.displayItems.map(i => i.id);
      if (this.handleRowClick(event, item.id, orderedIds)) return;
      this.openEmployeeDetails(item);
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
          ? await bulkMoveEmployeesTable(ids, this.currentTableId, toTableIds)
          : await bulkAddEmployeesTable(ids, toTableIds);
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
      const label = mode === 'move' ? 'Перенесено сотрудников: ' : 'Добавлено сотрудников: ';
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

    // Групповое "Убрать" (#1194 S5): снимает привязку выделенных сотрудников к
    // ТЕКУЩЕЙ таблице (bulkUnbindEmployeesTable). Последняя привязка -> BE сам
    // деактивирует сотрудника (status=0) - фронту достаточно показать результат.
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
        result = await bulkUnbindEmployeesTable(ids, this.currentTableId);
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
        useDeletionsStore().notify({ prefix: 'Убрано сотрудников: ', bold: String(result.success_count) });
      }
      this.clearSelection();
      this._loadData(true);
      return true;
    },

    openEmployeeDetails(item) {
      this.selectedEmployee = {
        id: item.id,
        last_name: item.last_name,
        first_name: item.first_name,
        middle_name: item.middle_name,
        position: item.position,
        citizenshipName: item.citizenshipName,
        passport_series_number: item.passport_series_number,
        patent_number: item.patent_number,
        other_permission: item.other_permission,
        organization: item.organization_name,
        organizationId: item.organization_id,
        company: item.company,
        companyId: item.company_id,
        entry_date_to: item.entry_date_to,
        pass_time: item.pass_time,
        target_tables: item.target_tables || [],
        territory_status: item.territory_status,
        applicationId: item.applicationId
      };
      this.showDetailsModal = true;
    },

    closeDetailsModal() {
      this.showDetailsModal = false;
      this.selectedEmployee = null;
    },

    // Сотрудник добавлен вручную без заявки (#1049): application_id === null (BE отдаёт
    // NULL для вложения-сироты). Строгий null - у обычных строк applicationId - число.
    isManualItem(item) {
      return item.applicationId === null;
    },

    openApplicationDetail(applicationId) {
      // Убрано закрытие модалки сотрудника
      this.$emit('open-application', applicationId);
    },

    openEmployeesHistory() {
      this.showEmployeesHistory = true;
    },

    startPolling() {
      if (this.pollingInterval) return;
      this.silentRefresh();
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
      return `${ENLARGED_KEY_PREFIX}${this.tableName || 'default'}`;
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
     */
    isFieldInDom() {
      return true;
    },

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
        + (this.can('entity.employees.delete') ? SERVICE_COLUMNS_WIDTH.actions : 0)
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
     * Стиль конфигурируемой ячейки:
     * - order: 10 + display_order (между фиксированными entry/exit/status/actions).
     * - flex-grow: width из конфига (если задан) - переопределяет дефолт из CSS.
     *
     * В enlarged НЕ задаём inline flex-grow ни одному столбцу - пусть CSS
     * пропорционально распределяет освободившееся от status пространство
     * по всем оставшимся столбцам.
     */
    getColStyle(fieldName, isHeader = false) {
      const order = this.fieldOrders[fieldName];
      const width = this.fieldWidths[fieldName];
      const style = {};
      if (order !== undefined) style.order = 10 + order;
      if (this.enlarged) {
        const ew = this.fieldsEnlargedWidth[fieldName];
        if (typeof ew === 'number' && ew > 0) style.flexGrow = ew;
        // enlarged_font_weight - применяется ТОЛЬКО к данным, не к заголовку.
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

    markConfigReady() {
      // 2 rAF + 100ms - гарантия что layout с итоговыми ширинами применился
      // и transition не сработает на стартовом расхождении.
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

    portraitFieldLabel(name) {
      const LABELS = {
        last_name: 'Фамилия',
        first_name: 'Имя',
        middle_name: 'Отчество',
        position: 'Должность',
        citizenship_name: 'Гражданство',
        organization: 'Организация',
        company: 'Компания',
        valid_until: 'Действует до',
        pass_time: 'Время прохода',
        application_id: 'Номер заявки',
      };
      return LABELS[name] || name;
    },

    portraitFieldValue(item, name) {
      switch (name) {
        case 'last_name': return item.last_name || '-';
        case 'first_name': return item.first_name || '-';
        case 'middle_name': return item.middle_name || '-';
        case 'position': return item.position || '-';
        case 'citizenship_name': return item.citizenship_name || '-';
        case 'organization': return item.organization_name || '-';
        case 'company': return item.company || '-';
        case 'valid_until': return this.formatDate(item.entry_date_to);
        case 'pass_time': return item.pass_time || '-';
        case 'application_id': return this.isManualItem(item) ? 'Добавлено вручную' : (item.applicationNumber || '-');
        default: return '-';
      }
    },

    async exportToExcel() {
      await this.buildPeopleExcel(this.displayItems, 'Lyudi');
    },

    // Экспорт только выделенных строк (#1194 S6) - reuse форматирования полного
    // экспорта (buildPeopleExcel), фильтр по selectedIds (useRowSelection).
    async exportSelectedToExcel() {
      const rows = this.displayItems.filter(item => this.selectedIds.includes(item.id));
      if (!rows.length) return;
      await this.buildPeopleExcel(rows, 'Lyudi_vybrannye');
      useDeletionsStore().notify({ prefix: 'Выгружено строк: ', bold: String(rows.length) });
    },

    // Общий билдер книги people-таблицы: набор строк и префикс имени файла -
    // единственное, что различается между полным экспортом и экспортом выбранных.
    async buildPeopleExcel(rows, filenamePrefix) {
      if (!rows.length) return;

      const workbook = new ExcelJS.Workbook();
      const worksheet = workbook.addWorksheet('Lyudi');

      const headers = [
        'Въезд', 'Выезд', 'Фамилия', 'Имя', 'Отчество', 'Должность',
        'Организация', 'Компания', 'Дата до', 'Время прохода', '№ заявки', 'Статус',
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
          item.last_name || '-',
          item.first_name || '-',
          item.middle_name || '-',
          item.position || '-',
          item.organization_name || '-',
          item.company || '-',
          this.formatDate(item.entry_date_to),
          item.pass_time || '-',
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
      // Штамп выгрузки московский и по серверным часам (#2298): файл уходит
      // наружу, время в нём должно совпадать с временем отметок в таблице.
      const dateStr = formatMoscowDateTime();
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
        { width: 10 }, { width: 10 }, { width: 22 }, { width: 18 }, { width: 18 }, { width: 22 },
        { width: 35 }, { width: 25 }, { width: 14 }, { width: 16 }, { width: 16 }, { width: 20 },
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
  padding-right: 8px;
}

/* Веса колонок (flex-grow). Браузер делит доступную ширину пропорционально весам
   видимых колонок. При скрытии любых через v-if остальные автоматически расширяются. */
.entry-col { flex: 6 0 0; }
.exit-col { flex: 6 0 0; }
.last-name-col { flex: 14 0 0; }
.first-name-col { flex: 9 0 0; }
.middle-name-col { flex: 11 0 0; }
.position-col { flex: 9 0 0; }
.citizenship-col { flex: 10 0 0; }
.organization-col { flex: 16 0 0; }
.company-col { flex: 11 0 0; }
.date-col { flex: 11 0 0; }
.time-col { flex: 13 0 0; }
.application-col { flex: 11 0 0; }
/* Статус - служебный столбец, он не проходит через getColStyle, поэтому минимум
   ширины (под бейдж StatusBadge) задаём здесь (#1307). */
.status-col { flex: 8 0 0; min-width: 150px; }
.actions-col { flex: 2 0 0; padding-right: 0; }

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

/* Overlay-лоадер при refresh - сохраняет высоту таблицы. */
.refresh-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
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
  padding: 2px;
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
  /* Значок мельче 16px: общая обводка 1.7 садится в волосок, здесь плотнее. */
  stroke-width: 2.2;
  color: var(--text);
  width: 14px;
  height: 14px;
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

/* В enlarged status-col схлопывается; освободившиеся 8 grow распределяются
   автоматически по ВСЕМ оставшимся столбцам пропорционально базовым весам -
   inline flex-grow для них не задаётся (getColStyle пропускает). */
.selected-table-card.enlarged .status-col {
  flex-grow: 0;
  opacity: 0;
  pointer-events: none;
}

.selected-table-card.enlarged .item-data {
  min-height: 36px;
}

@media (max-width: 767.98px) {
  /* Убирать рамку панели целиком владелец не просил - без неё "куда пропала
     таблица? почему нету границ таблицы?" (талон читается разрозненными строками,
     а не таблицей). Радиус тот же, что на десктопе (30px) и что у таблицы «по
     факту» - владелец забраковал 15px волны 7 отдельно ("таблицам больше
     скругление нужно дать, как и было"). Против «квадрата в квадрате» отвечают
     уже сами строки ниже: скругление получают только верхний край первой и
     нижний последней, а не каждая. Заголовок отделяет линия снизу. */
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

  /* Счётчик остаётся в ряду шапки, но своей строки больше не занимает: ширина по
     содержимому, к правому краю его подводит автополе, а «Обновить» идёт следом по
     order.

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

  /* #1097 S9. Карточка по образцу заявки (ApplicationAttachmentDetail.vue): подписи
     полей убраны, значения выровнены влево, разделитель рисуется сверху.

     Кнопки прохода при этом стояли двумя отдельными строками, и слева от каждой висела
     дублирующая подпись - "Вход" подписью и "Вход" кнопкой в одной строке. Поэтому
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

  /* Разделитель полей рисуем сверху у ячеек 2..N, а не снизу: последней в строке идёт
     колонка действий без data-label, глобальное `[data-label]:last-child` до неё не
     достаёт, и пунктир висел бы оторванной чертой над нижним краем карточки. */
  .selected-table-card .rt-row > [data-label] {
    justify-content: flex-start !important;
    text-align: left !important;
    border-bottom: none !important;
  }

  /* ФИО одной строкой: три ячейки имени идут по содержимому и читаются как одно
     значение - это заголовок карточки, ровно как номер в талоне машины. Раньше
     каждая занимала свою строку, и карточка открывалась столбцом «Иванов / Иван /
     Иванович», причём отчество в неё вообще не попадало.

     Базис auto с разрешённым сжатием, а не `0 0 auto`: ФИО длиннее строки тогда
     переносится на вторую, а не выдавливает карточку. Зазор ряда (8px) между
     словами имени вдвое шире пробела - гасим отрицательным полем у первых двух. */
  .selected-table-card .rt-row > .last-name-col,
  .selected-table-card .rt-row > .first-name-col,
  .selected-table-card .rt-row > .middle-name-col {
    flex: 0 1 auto !important;
    width: auto !important;
    padding-top: 8px !important;
    /* `!important` не перестраховка: размер строки задан ниже по файлу правилом
       `.selected-table-card .items-body .col { font-size: var(--table-font-size) }` -
       специфичность та же (0,3,0), а объявлено оно позже медиазапроса и выигрывает
       тай-брейк. Тем же лечится крупный номер в талоне машины. */
    font-size: 16px !important;
    font-weight: 700;
  }

  .selected-table-card .rt-row > .last-name-col,
  .selected-table-card .rt-row > .first-name-col {
    margin-right: -4px;
  }

  /* Ячейки прохода делят верхнюю строку пополам - единственные, кто выходит из
     «своя строка каждому». Пунктир им не нужен: между кнопками одного ряда он лёг бы
     вертикальной чертой посреди строки, а поле под ними свой верхний пунктир
     сохраняет - он и отделяет ряд действий от данных. */
  .selected-table-card .rt-row > .entry-col,
  .selected-table-card .rt-row > .exit-col {
    width: auto !important;
    /* Базис ровно половина, а не 0 с ростом: перенос во flex считается по базисам ДО
       распределения свободного места, и при нулевом базисе в первую строку набирается
       ещё и следующая ячейка - кнопки схлопываются друг на друга. На проходной это
       уже случилось, здесь держим тот же явный базис. */
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

     Подвал талона: «Подробнее» и «Удалить» пополам, пилюлями в 28px (раскладка и
     числа те же, что у таблицы машин: обе таблицы открывают один и тот же экран
     поста). Порядок задаём обоим заведомо большими числами, а не правим один
     столбец: разметочные `order` (9998-9999) соседствуют с порядком настраиваемых.

     Базис половины, а не «0 с ростом»: перенос во flex считается по базисам ДО
     распределения свободного места, и с нулевым базисом кнопки схлопывались бы друг
     на друга. Единственной кнопке достаётся левая половина - к правому краю карточки
     она не жмётся.

     `overflow: visible` обязателен: базовый `.col { overflow: hidden }` обрезает
     невидимый ::before, которым пилюля добирает зону нажатия до 44px. */
  .selected-table-card .rt-pass > .status-col {
    display: none !important;
  }

  .selected-table-card .rt-pass > .expand-col {
    order: 10000 !important;
  }

  .selected-table-card .rt-pass > .actions-col {
    order: 10001 !important;
  }

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
     названиями; должность, гражданство, номер заявки, дата и время прохода - голые
     значения, которые сами себя не называют. Фамилия, имя и отчество стоят подряд
     вверху карточки и читаются как ФИО, бейдж статуса говорит за себя. */
  .selected-table-card .rt-row > .position-col::before,
  .selected-table-card .rt-row > .citizenship-col::before,
  .selected-table-card .rt-row > .organization-col::before,
  .selected-table-card .rt-row > .company-col::before,
  .selected-table-card .rt-row > .application-col::before,
  .selected-table-card .rt-row > .date-col::before,
  .selected-table-card .rt-row > .time-col::before {
    display: block !important;
  }

  /* Кнопки прохода - главное действие экрана, но не 44px "огромные": замер на
     карточке 370px давал 158x44 - тач-таргет для двух кнопок в половину строки
     взят с большим запасом. Норма проекта для контролов такого калибра - 36px
     (эталон §18). */
  .action-btn {
    min-width: 70px;
    height: 36px;
    font-size: 13px;
  }

  /* Шеврон в пилюле «Подробнее» показывает раскрытие поворотом - саму пилюлю при
     этом не вертим, её `transform: none` приходит из rt-pass. */
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

/* #345 Phase 1D: размер шрифта строк через CSS-переменную. */
.selected-table-card .items-body .col {
  font-size: var(--table-font-size, 14px);
}

/* #345 Phase 1E: плотность строк - вертикальный padding ячейки. */
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

/* #345 Phase 1F: портретный режим. */
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

/* Раскрытие "Подробнее" - стиль карточек label/value как в EmployeeDetailsModal.
   Auto-fit grid, каждый item - flex-column с label сверху и value снизу. */
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
