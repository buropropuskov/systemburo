<template>
  <div
    class="fact-table-card"
    :class="[
      { 'grid-mode': grid, 'config-not-ready': !configReady },
      `density-${rowDensity}`,
    ]"
    :style="{ '--table-font-size': tableFontSize + 'px' }"
    data-testid="fact-table"
  >
    <div class="card-header">
      <div class="card-header__title">
        <h3 class="card-title">
          {{ tableType === 'cars' ? 'Автомобили' : 'Люди' }} <span class="highlight-text">по факту</span>
        </h3>
      </div>
      <div class="card-header__settings">
        <RefreshButton
          :loading="refreshing"
          @refresh="handleManualRefresh"
        />
      </div>
    </div>
    
    <div class="card-content rt-table">
      <!-- Заголовок таблицы -->
      <div class="fact-header">
        <div class="header-row rt-head-row">
          <!-- Служебные въезд/выезд - всегда первые (только cars) -->
          <div
            v-if="tableType === 'cars'"
            class="col entry-col"
            style="order: 0;"
          >
            Въезд
          </div>
          <div
            v-if="tableType === 'cars'"
            class="col exit-col"
            style="order: 1;"
          >
            Выезд
          </div>
          <!-- Конфигурируемые столбцы CarsTable-каталога -->
          <div
            v-if="tableType === 'cars' && isFieldVisible('car_number')"
            class="col number-col"
            :style="getColStyle('car_number')"
          >
            <p>Номер Т/С</p>
          </div>
          <div
            v-if="tableType === 'cars' && isFieldVisible('car_brand')"
            class="col brand-col"
            :style="getColStyle('car_brand')"
            @click="sortBy('car_brand')"
          >
            <p :class="{ 'active-sort': sortField === 'car_brand' }">Марка</p>
          </div>
          <div
            v-if="isFieldVisible('organization')"
            class="col organization-col"
            :style="getColStyle('organization')"
            @click="sortBy('organization')"
          >
            <p :class="{ 'active-sort': sortField === 'organization' }">Организация</p>
          </div>
          <div
            v-if="tableType === 'cars' && isFieldVisible('company')"
            class="col company-col"
            :style="getColStyle('company')"
          >
            <p>Компания</p>
          </div>
          <div
            v-if="isFieldVisible('application_id')"
            class="col application-col"
            :style="getColStyle('application_id')"
          >
            <p>Номер заявки</p>
          </div>
          <div
            v-if="tableType === 'cars' && isFieldVisible('unload_place')"
            class="col place-col"
            :style="getColStyle('unload_place')"
          >
            <p>Место разгрузки</p>
          </div>
          <div
            v-if="isFieldVisible('valid_until')"
            class="col date-col"
            :style="getColStyle('valid_until')"
            @click="sortBy('entry_date_to')"
          >
            <p :class="{ 'active-sort': sortField === 'entry_date_to' }">Действует до</p>
          </div>
          <div
            v-if="isFieldVisible(tableType === 'cars' ? 'time_range' : 'pass_time')"
            class="col time-col"
            :style="getColStyle(tableType === 'cars' ? 'time_range' : 'pass_time')"
            @click="sortBy('entry_time')"
          >
            <p :class="{ 'active-sort': sortField === 'entry_time' }">
              {{ tableType === 'cars' ? 'Время' : 'Время прохода' }}
            </p>
          </div>
          <div
            v-if="tableType === 'cars' && isFieldVisible('status')"
            class="col status-col"
            :style="getColStyle('status')"
          >
            <p>Статус</p>
          </div>
          <!-- People-only поля -->
          <div
            v-if="tableType === 'people' && isFieldVisible('last_name')"
            class="col last-name-col"
            :style="getColStyle('last_name')"
          >
            <p>Фамилия</p>
          </div>
          <div
            v-if="tableType === 'people' && isFieldVisible('first_name')"
            class="col first-name-col"
            :style="getColStyle('first_name')"
          >
            <p>Имя</p>
          </div>
          <div
            v-if="tableType === 'people' && isFieldVisible('middle_name')"
            class="col middle-name-col"
            :style="getColStyle('middle_name')"
          >
            <p>Отчество</p>
          </div>
          <div
            v-if="tableType === 'people' && isFieldVisible('position')"
            class="col position-col"
            :style="getColStyle('position')"
          >
            <p>Должность</p>
          </div>
          <div
            v-if="tableType === 'people' && isFieldVisible('citizenship_name')"
            class="col citizenship-col"
            :style="getColStyle('citizenship_name')"
          >
            <p>Гражданство</p>
          </div>
          <!-- Служебная actions всегда последняя -->
          <div
            class="col actions-col"
            style="order: 9999;"
          >
            <!-- Пустой заголовок для действий -->
          </div>
        </div>
      </div>
      
      <!-- Тело таблицы -->
      <div class="fact-container">
        <!-- Спиннер ТОЛЬКО на первой загрузке (isLoading, свой внутренний флаг - волна 8).
             Раньше гейтился пропом `loading` от родителя, а тот отражал ЕГО собственный
             рефреш - дёргался на клик ЛЮБОЙ из трёх кнопок «Обновить» на экране. «По
             факту» почти всегда пуста, поэтому чужой клик подменял "Заявок... нет"
             спиннером и обратно - блок прыгал 94 -> 150 -> 94px без единой смены данных.
             Тихие обновления (поллинг, SSE, свой клик «Обновить» - handleManualRefresh)
             держат isLoading в покое: подмена списка спиннером на них схлопывает высоту
             документа и на телефоне выбрасывает страницу в начало. -->
        <!-- refreshing (иконка «Обновить» крутится) НЕ гейтит этот блок - только isLoading:
             тихое обновление обязано остаться тихим, крутящаяся иконка - её собственный,
             более мелкий сигнал. -->
        <div
          v-if="isLoading && !filteredData.length"
          class="loading-message"
        >
          <LoaderSpinner :label="tableType === 'cars' ? 'Загрузка машин...' : 'Загрузка...'" />
        </div>
        <div
          v-else-if="filteredData.length > 0"
          class="fact-body"
        >
          <transition-group
            name="fade-list"
            tag="div"
          >
            <div
              v-for="(item, index) in sortedData"
              :key="item.id"
              class="fact-item"
              data-testid="ob-fact-row"
              :style="{ animationDelay: `${index * 0.1}s` }"
              @click="openItemDetails(item)"
            >
              <!-- rt-pass: строка собирается талоном на мобилке (responsive-tables.css,
                   часть 3) - той же, что у таблицы «по заявке» под ней: обе стоят на
                   одном экране, и разнобой между ними виден целиком. Только для машин:
                   у людей нет ни номера, ни кнопок прохода, талону не из чего собраться. -->
              <div
                class="fact-row rt-row"
                :class="{ 'rt-pass': tableType === 'cars' }"
              >
                <!-- Служебные въезд/выезд - всегда первые (только cars) -->
                <div
                  v-if="tableType === 'cars'"
                  class="col entry-col"
                  style="order: 0;"
                  data-label="Въезд"
                  @click.stop
                >
                  <button
                    class="action-btn entry-btn"
                    :class="{ 'active': item.entry_checked }"
                    :disabled="item.entry_checked"
                    data-testid="ob-fact-entry"
                    @click="handleEntryExit(item, 'entry')"
                  >
                    Въезд
                  </button>
                </div>
                <div
                  v-if="tableType === 'cars'"
                  class="col exit-col"
                  style="order: 1;"
                  data-label="Выезд"
                  @click.stop
                >
                  <button
                    class="action-btn exit-btn"
                    :class="{ 'active': item.exit_checked }"
                    :disabled="!item.entry_checked || item.exit_checked"
                    data-testid="ob-fact-exit"
                    @click="handleEntryExit(item, 'exit')"
                  >
                    Выезд
                  </button>
                </div>
                <!-- Конфигурируемые столбцы -->
                <div
                  v-if="tableType === 'cars' && isFieldVisible('car_number')"
                  class="col number-col rt-pass__plate"
                  :style="getColStyle('car_number')"
                  data-label="Номер Т/С"
                >
                  {{ item.car_number || '-' }}
                </div>
                <div
                  v-if="tableType === 'cars' && isFieldVisible('car_brand')"
                  class="col brand-col rt-pass__mark"
                  :style="getColStyle('car_brand')"
                  data-label="Марка"
                >
                  {{ item.car_brand || '-' }}
                </div>
                <div
                  v-if="isFieldVisible('organization')"
                  class="col organization-col"
                  :style="getColStyle('organization')"
                  data-label="Организация"
                >
                  {{ item.organization_name || '-' }}
                </div>
                <div
                  v-if="tableType === 'cars' && isFieldVisible('company')"
                  class="col company-col"
                  :style="getColStyle('company')"
                  data-label="Компания"
                >
                  {{ item.company || '-' }}
                </div>
                <div
                  v-if="isFieldVisible('application_id')"
                  class="col application-col"
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
                  v-if="tableType === 'cars' && isFieldVisible('unload_place')"
                  class="col place-col"
                  :style="getColStyle('unload_place')"
                  data-label="Место разгрузки"
                >
                  {{ formatUnloadPlaces ? formatUnloadPlaces(item) : '-' }}
                </div>
                <div
                  v-if="isFieldVisible('valid_until')"
                  class="col date-col"
                  :style="getColStyle('valid_until')"
                  data-label="Действует до"
                >
                  {{ formatDate(item.entry_date_to) }}
                </div>
                <div
                  v-if="isFieldVisible(tableType === 'cars' ? 'time_range' : 'pass_time')"
                  class="col time-col"
                  :style="getColStyle(tableType === 'cars' ? 'time_range' : 'pass_time')"
                  :data-label="tableType === 'cars' ? 'Время' : 'Время прохода'"
                >
                  {{ tableType === 'cars'
                    ? formatTimeRange(item.entry_time_from, item.entry_time_to)
                    : formatPassTime(item.pass_time)
                  }}
                </div>
                <div
                  v-if="tableType === 'cars' && isFieldVisible('status')"
                  class="col status-col"
                  :style="getColStyle('status')"
                  data-label="Статус"
                >
                  <StatusBadge :status="item.status" />
                </div>
                <!-- People-only поля -->
                <div
                  v-if="tableType === 'people' && isFieldVisible('last_name')"
                  class="col last-name-col"
                  :style="getColStyle('last_name')"
                  data-label="Фамилия"
                >
                  {{ item.last_name || '-' }}
                </div>
                <div
                  v-if="tableType === 'people' && isFieldVisible('first_name')"
                  class="col first-name-col"
                  :style="getColStyle('first_name')"
                  data-label="Имя"
                >
                  {{ item.first_name || '-' }}
                </div>
                <div
                  v-if="tableType === 'people' && isFieldVisible('middle_name')"
                  class="col middle-name-col"
                  :style="getColStyle('middle_name')"
                  data-label="Отчество"
                >
                  {{ item.middle_name || '-' }}
                </div>
                <div
                  v-if="tableType === 'people' && isFieldVisible('position')"
                  class="col position-col"
                  :style="getColStyle('position')"
                  data-label="Должность"
                >
                  {{ item.position || '-' }}
                </div>
                <div
                  v-if="tableType === 'people' && isFieldVisible('citizenship_name')"
                  class="col citizenship-col"
                  :style="getColStyle('citizenship_name')"
                  data-label="Гражданство"
                >
                  {{ item.citizenshipName || item.citizenship_name || '-' }}
                </div>
                <!-- Удалить - всегда в конце -->
                <div
                  class="col actions-col"
                  style="order: 9999;"
                  @click.stop
                >
                  <button
                    class="delete-btn rt-pass__act rt-pass__act--danger"
                    title="Удалить"
                    @click="deleteItem(item)"
                  >
                    <AppIcon
                      name="trashcan"
                      class="delete-icon rt-pass__act-icon"
                    />
                    <span class="rt-pass__act-label">Удалить</span>
                  </button>
                </div>
              </div>
            </div>
          </transition-group>
        </div>
        <p
          v-else
          class="no-data-message"
        >
          {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : `Заявок ${tableType === 'cars' ? 'на машины' : 'на людей'} по факту нет` }}
        </p>
      </div>
    </div>

    <!-- Модальное окно с деталями автомобиля (для машин) -->
    <VehicleDetailsModal
      v-if="tableType === 'cars'"
      :show="showDetailsModal"
      :vehicle="selectedVehicle"
      :all-unloading-places="allUnloadingPlaces"
      :license-plate-formats="licensePlateFormats"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      :show-car-features="true"
      :source="'facttable'"
      @close="closeDetailsModal"
      @open-application="$emit('open-application', $event)"
    />

    <!-- Модалка ввода данных при пропуске "по факту" (#1132) -->
    <FactPassModal
      :show="showPassModal"
      :formats="licensePlateFormats"
      :loading="passLoading"
      :error="passError"
      @close="closePassModal"
      @confirm="onPassConfirm"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useUiStore } from '@/stores/ui';
import eventStream from '@/services/eventStream';
import RefreshButton from './RefreshButton.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import StatusBadge from '@/components/ui/StatusBadge.vue';
import VehicleDetailsModal from './CreateApplication/VehicleDetailsModal.vue';
import FactPassModal from './FactPassModal.vue';
import ExcelJS from 'exceljs';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import { idFilterSet } from '@/utils/idFilter';
import { pickOverflowFields, columnMinWidth, measureRowAvailableWidth, SERVICE_COLUMNS_WIDTH } from '@/utils/tableColumnFit';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import AppIcon from '@/components/icons/AppIcon.vue';
import { formatMoscowDateTime } from '@/utils/serverTime';

export default {
  name: 'FactTable',
  components: {
    AppIcon,
    RefreshButton,
    LoaderSpinner,
    StatusBadge,
    VehicleDetailsModal,
    FactPassModal
  },
  props: {
    tableType: { type: String, default: 'cars', validator: (v) => ['cars', 'people'].includes(v) },
    tableId: { type: Number, default: null },
    tableData: { type: Object, default: null },
    searchQuery: { type: String, default: '' },
    // Мультивыбор (#1398): пустой массив - фильтр выключен.
    selectedOrganizationIds: { type: Array, default: () => [] },
    selectedCompanyIds: { type: Array, default: () => [] },
    selectedUnloadingPlaceIds: { type: Array, default: () => [] },
    dateRangeStart: { type: Date, default: null },
    dateRangeEnd: { type: Date, default: null },
    selectedDate: { type: Date, default: null },
    currentUserId: { type: Number, default: null },
    currentUserName: { type: String, default: '' },
    // Режим "Сетка" (#1289): границы ячеек. Управляется одним тумблером страницы.
    grid: { type: Boolean, default: false }
  },
  emits: ['refresh-data', 'open-application'],
  setup() {
    // Порог тот же, что у card-правил responsive-tables.css: брейкпоинт компонента
    // обязан совпадать с брейкпоинтом инфраструктуры, которой он пользуется.
    const { isNarrow } = useNarrowScreen(767.98);
    return { isNarrow };
  },
  data() {
    return {
      // Свой флаг первой загрузки (волна 8) - `loading` проп родителя отражает ЕГО
      // собственный рефреш (по клику любой из трёх кнопок «Обновить» на экране), а
      // не факт того, что у ЭТОЙ таблицы ещё нет данных для первого показа.
      isLoading: false,
      // Иконка «Обновить» крутится, пока ручное обновление в полёте (эталон CarsTable/
      // PeopleTable) - отдельно от isLoading, который держит только первый показ.
      refreshing: false,
      sortField: null,
      sortDirection: 'desc',
      factData: [],
      organizationsMap: {},
      factCarUnloadPlacesMap: {},
      allUnloadingPlaces: [],
      licensePlateFormats: [],
      showDetailsModal: false,
      selectedVehicle: null,
      // Пропуск "по факту" (#1132): модалка ввода формата/номера/марки при въезде.
      showPassModal: false,
      passItem: null,
      passLoading: false,
      passError: '',
      pollingInterval: null,
      // Real-time (#840): подписка на tables:<tableId>, статус SSE-соединения и seq-токен
      // против гонки конкурентных silentRefresh (таймер + SSE-сигнал, урок #632/#840).
      eventStreamOff: null,
      eventStreamStatusOff: null,
      sseConnected: false,
      refreshSeq: 0,
      fieldsVisibility: {},
      fieldOrders: {},
      fieldWidths: {},
      tableFontSize: 14,
      rowDensity: 'normal',
      // Столбцы, не поместившиеся по ширине (#1307): скрываются от наименее
      // важных, при равном приоритете - правые. Значения видны в карточке строки.
      overflowFields: [],
      configReady: false,
    };
  },
  computed: {
    filteredData() {
      let filtered = [...this.factData];
      const variants = buildSearchVariants(this.searchQuery);
      if (variants.length) {
        filtered = filtered.filter(item => {
          const haystack = [
            item.organization_name,
            this.tableType === 'cars' ? item.car_brand : '',
            this.tableType === 'cars' ? (item.company || '') : '',
            item.status,
            this.tableType === 'cars'
              ? this.formatTimeRange(item.entry_time_from, item.entry_time_to)
              : this.formatPassTime(item.pass_time),
            this.formatDate(item.entry_date_to),
          ].join(' ');
          return matchesSearch(haystack, variants);
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
      const unloadPlaceIds = this.tableType === 'cars' ? idFilterSet(this.selectedUnloadingPlaceIds) : null;
      if (unloadPlaceIds) {
        filtered = filtered.filter(item => {
          const unloadPlaces = this.factCarUnloadPlacesMap[item.id] || [];
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
      return filtered;
    },

    sortedData() {
      const data = [...this.filteredData];
      if (!this.sortField) return data;
      return data.sort((a, b) => {
        let valueA, valueB;
        switch (this.sortField) {
          case 'organization':
          case 'status':
          case 'car_brand':
          case 'company':
            valueA = (a[this.sortField] || '').toString().toLowerCase();
            valueB = (b[this.sortField] || '').toString().toLowerCase();
            break;
          case 'unload_place':
            // Сортировка по месту разгрузки закомментирована вместе с колонкой
            // valueA = this.formatUnloadPlaces(a).toLowerCase();
            // valueB = this.formatUnloadPlaces(b).toLowerCase();
            // break;
            return 0;
          case 'entry_date_to':
            valueA = a.entry_date_to ? new Date(a.entry_date_to) : new Date(0);
            valueB = b.entry_date_to ? new Date(b.entry_date_to) : new Date(0);
            break;
          case 'entry_time':
            if (this.tableType === 'cars') {
              valueA = this.extractStartTime(a.entry_time_from);
              valueB = this.extractStartTime(b.entry_time_from);
            } else {
              valueA = this.extractPassTime(a.pass_time);
              valueB = this.extractPassTime(b.pass_time);
            }
            break;
          default: return 0;
        }
        if (valueA < valueB) return this.sortDirection === 'asc' ? -1 : 1;
        if (valueA > valueB) return this.sortDirection === 'asc' ? 1 : -1;
        return 0;
      });
    },

    hasActiveFilters() {
      return !!(
        this.searchQuery ||
        idFilterSet(this.selectedOrganizationIds) ||
        idFilterSet(this.selectedCompanyIds) ||
        (this.tableType === 'cars' && idFilterSet(this.selectedUnloadingPlaceIds)) ||
        this.selectedDate ||
        (this.dateRangeStart && this.dateRangeEnd)
      );
    }
  },
  watch: {
    tableType: {
      handler() {
        this.stopPolling();
        this.startPolling();
      },
      immediate: true
    },
    // Поворот телефона и переход через брейкпоинт меняют правила подбора столбцов:
    // ResizeObserver сам по себе сюда не доедет, ширина карточки при повороте может
    // и не измениться (планшет 768 <-> 767).
    isNarrow() {
      this.recalcOverflowFields();
    },
    // Применяем фактовые настройки таблицы (#345 PR-B).
    tableData: {
      immediate: true,
      deep: true,
      handler(newVal) {
        if (!newVal) return;
        const tbl = newVal.table || {};
        const factFields = newVal.fact_fields || [];
        const nextVis = {};
        const nextOrd = {};
        const nextW = {};
        factFields.forEach(f => {
          nextVis[f.field_name] = f.is_visible !== false;
          if (typeof f.display_order === 'number') nextOrd[f.field_name] = f.display_order;
          if (typeof f.width === 'number' && f.width > 0) nextW[f.field_name] = f.width;
        });
        this.fieldsVisibility = nextVis;
        this.fieldOrders = nextOrd;
        this.fieldWidths = nextW;
        this.$nextTick(() => this.recalcOverflowFields());
        const fs = Number(tbl.font_size_fact);
        if (fs >= 10 && fs <= 24) this.tableFontSize = fs;
        const dens = tbl.row_density_fact;
        if (['compact', 'normal', 'spacious'].includes(dens)) this.rowDensity = dens;
        this.markConfigReady();
      },
    },
    // Real-time (#840): подписка на scope конкретной таблицы, пересобирается при смене tableId.
    tableId: {
      immediate: true,
      handler(newVal) {
        this.subscribeTableScope(newVal);
      }
    },
  },
  mounted() {
    this.$nextTick(() => this.bindWidthWatcher());
    this.startPolling();

    // Real-time (#840): по сигналу продюсера tables.refresh тихо перезагружаем строки
    // вместо ожидания поллинга. Сама подписка на scope - в watch tableId (уже immediate).
    eventStream.connect();
    this.eventStreamStatusOff = eventStream.onStatus((status) => {
      this.sseConnected = status === 'connected';
    });
  },
  beforeUnmount() {
    this.unbindWidthWatcher();
    this.stopPolling();
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
    /**
     * Поля, видимые по настройкам таблицы, в порядке отображения слева направо.
     */
    configuredFields() {
      const source = this.fieldsVisibility;
      return Object.keys(source)
        .filter(name => source[name] !== false)
        .sort((a, b) => (this.fieldOrders[a] ?? 0) - (this.fieldOrders[b] ?? 0));
    },

    /**
     * Пересчитывает, какие столбцы не помещаются в текущую ширину таблицы.
     */
    recalcOverflowFields() {
      // На телефоне строка идёт карточкой: у каждого поля своя строка, за ширину
      // они не конкурируют, и прятать нечего. Подбор здесь не просто бесполезен, а
      // вреден: он мерит скрытую строку заголовков, получает ноль, берёт ширину
      // карточки (368 на 390) против 260 служебных и оставляет ровно `keepAtLeast`
      // столбца из девяти - а «Подробнее» у таблицы «по факту» нет, и остальные
      // значения не видны нигде.
      if (this.isNarrow) {
        this.overflowFields = [];
        return;
      }
      const host = this.$el && this.$el.querySelector('.card-content');
      if (!host) return;
      const reserved = SERVICE_COLUMNS_WIDTH.passage
        + SERVICE_COLUMNS_WIDTH.actions;
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

    isFieldVisible(fieldName) {
      const v = this.fieldsVisibility[fieldName];
      if (v === false) return false;
      // Не поместившиеся по ширине (#1307). Панели «Подробнее» здесь нет, поэтому
      // набор пуст везде, где строка идёт карточкой, - см. recalcOverflowFields.
      return !this.overflowFields.includes(fieldName);
    },
    // Запись добавлена вручную без заявки (#1049): application_id === null.
    isManualItem(item) {
      return item.applicationId === null;
    },

    getColStyle(fieldName) {
      const order = this.fieldOrders[fieldName];
      const width = this.fieldWidths[fieldName];
      const style = {};
      if (order !== undefined) style.order = 10 + order;
      if (width !== undefined && width > 0) style.flexGrow = width;
      // Ниже этого порога столбец не сжимается - значения переставали читаться
      // (#1307). Не поместившиеся столбцы к этому моменту уже скрыты.
      style.minWidth = columnMinWidth(fieldName) + 'px';
      return Object.keys(style).length ? style : null;
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
    // silent=false (только первый вызов, см. isLoading в data()) показывает полноэкранный
    // спиннер - на тихих обновлениях (поллинг, SSE, чужая кнопка «Обновить») он не нужен.
    async _loadData(silent = false) {
      const seq = ++this.refreshSeq;
      if (!silent) this.isLoading = true;
      try {
        await this.fetchUnloadingPlaces();
        await this.fetchLicensePlateFormats();
        await this.fetchOrganizations();
        // Только машины: строка "по факту" рождается тумблером в форме транспорта,
        // у сотрудников такого тумблера нет, поэтому и блока "по факту" у таблиц людей
        // не бывает - см. showFactTable в TablesComponent (#2019).
        await this.fetchCarsData(seq);
        await this.fetchFactCarUnloadPlaces(seq);
        await this.fetchCarHistoryStatus(seq);
      } catch (error) {
        console.error(`Ошибка при загрузке данных по факту (${this.tableType}):`, error);
      } finally {
        if (!silent) this.isLoading = false;
      }
    },

    async loadData() {
      await this._loadData(false);
    },

    async silentRefresh() {
      await this._loadData(true);
    },

    // Клик по «Обновить» этой карточки раньше только гонял метаданные таблицы через
    // родителя ($emit('refresh-data')) и вовсе не перезагружал СВОИ строки - тихо
    // ждал следующего поллинга/SSE. Теперь зовёт и то, и другое: родитель обновляет
    // конфиг колонок, а silentRefresh - сам список (тихо, без спиннера - иначе клик
    // по этой кнопке мигал бы спиннером ровно так же, как раньше мигал от ЧУЖИХ).
    async handleManualRefresh() {
      this.$emit('refresh-data');
      this.refreshing = true;
      try {
        await this.silentRefresh();
      } finally {
        this.refreshing = false;
      }
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

    getOrganizationName(organizationId) {
      if (!organizationId) return 'Не указана';
      return this.organizationsMap[organizationId] || `Организация ID: ${organizationId}`;
    },

    async fetchCarsData(seq) {
      if (!this.tableId) return;
      try {
        const response = await apiRequest(`/cars/fact-for-table/${this.tableId}`, {});
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
        const factCars = await response.json();
        const nameToIdMap = {};
        Object.keys(this.organizationsMap).forEach(id => {
          nameToIdMap[this.organizationsMap[id]] = id;
        });
        const newData = factCars.map(car => {
          const orgName = car.organization || '';
          const orgId = nameToIdMap[orgName] || car.organization_id;
          return {
            id: car.id,
            car_number: car.car_number || 'по факту',
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
            entry_checked: false,
            exit_checked: false,
            applicationId: car.application_id,
            plateNumber: car.car_number,
            mark: car.car_brand,
            formatId: null,
            unloadPlaces: car.unload_place_ids || []
          };
        });
        if (seq !== undefined && seq !== this.refreshSeq) return; // устарел - новее уже в работе/загружен
        this.factData = newData;
      } catch (error) {
        console.error("Ошибка при загрузке данных по факту:", error);
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
          this.factData.forEach(item => {
            const status = statusMap[item.id];
            if (status) {
              item.entry_checked = status.territory_status === 1;
              item.exit_checked = status.territory_status === 2;
            }
          });
        }
      } catch (error) {
        console.error("Ошибка при загрузке статусов въезда/выезда:", error);
      }
    },

    async fetchFactCarUnloadPlaces(seq) {
      try {
        const response = await apiRequest("/cars/fact-unload-places", {});
        if (response.ok) {
          const carUnloadPlaces = await response.json();
          // seq-guard (#632/#840): устаревший ответ не должен перезаписывать карту
          // мест разгрузки и мутировать unload_place_ids уже отрисованных строк.
          if (seq !== undefined && seq !== this.refreshSeq) return;
          this.factCarUnloadPlacesMap = {};
          carUnloadPlaces.forEach(cup => {
            if (!this.factCarUnloadPlacesMap[cup.car_id]) this.factCarUnloadPlacesMap[cup.car_id] = [];
            this.factCarUnloadPlacesMap[cup.car_id].push({
              id: cup.unload_place_id,
              name: cup.unload_place_name || `Место #${cup.unload_place_id}`
            });
            const car = this.factData.find(c => c.id === cup.car_id);
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
        console.error("Ошибка при загрузке связей факт-машин с местами разгрузки:", error);
        if (seq === undefined || seq === this.refreshSeq) this.factCarUnloadPlacesMap = {};
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
      const [year, month, day] = dateString.split('-');
      const date = new Date(year, month - 1, day);
      return date.toLocaleDateString('ru-RU');
    },

    formatTimeRange(timeFrom, timeTo) {
      if (!timeFrom && !timeTo) return '-';
      const formatTime = (timeStr) => {
        if (!timeStr) return '';
        if (timeStr.includes(':') && timeStr.split(':').length === 3) return timeStr.substring(0, 5);
        return timeStr;
      };
      const formattedTimeFrom = formatTime(timeFrom);
      const formattedTimeTo = formatTime(timeTo);
      if (!formattedTimeTo) return formattedTimeFrom;
      if (!formattedTimeFrom) return formattedTimeTo;
      return `${formattedTimeFrom} - ${formattedTimeTo}`;
    },

    formatPassTime(passTime) { return passTime || '-'; },
    
    async handleEntryExit(item, type) {
      if (!this.currentUserId) return;
      // Въезд "по факту" (#1132): сначала модалка ввода данных, статус ставим только
      // после успешного сохранения (onPassConfirm). Выезд - как прежде, сразу.
      if (type === 'entry') {
        this.passItem = item;
        this.passError = '';
        this.showPassModal = true;
        return;
      }
      await this.applyTerritoryStatus(item, 2, null);
    },

    async onPassConfirm(pass) {
      if (!this.passItem) return;
      // Пока охранник вводил данные, строка могла уйти из фактовой таблицы (чужой
      // выезд/деактивация через SSE-рефреш) или уже получить въезд - не шлём пропуск
      // по устаревшей записи (замечание ревью: PR расширяет окно гонки).
      const current = this.factData.find(i => i.id === this.passItem.id);
      if (!current || current.entry_checked) {
        this.passError = 'Запись изменилась, обновите таблицу и попробуйте снова.';
        return;
      }
      this.passLoading = true;
      this.passError = '';
      const ok = await this.applyTerritoryStatus(current, 1, pass);
      this.passLoading = false;
      if (ok) {
        this.closePassModal();
      } else {
        this.passError = 'Не удалось сохранить пропуск. Попробуйте ещё раз.';
      }
    },

    closePassModal() {
      this.showPassModal = false;
      this.passItem = null;
      this.passError = '';
    },

    // Отправляет смену территориального статуса и при успехе флипает флаги строки.
    // pass != null (только при въезде) добавляет данные пропуска "по факту" (#1132).
    async applyTerritoryStatus(item, territory_status, pass) {
      try {
        const body = { territory_status, user_id: this.currentUserId, table_id: this.tableId };
        if (pass) body.pass = pass;
        const response = await apiRequest(`/cars/${item.id}/territory-status`, {
          method: "PUT",
          body: JSON.stringify(body)
        });
        if (!response.ok) {
          const errorText = await response.text();
          console.error('Ошибка при обновлении статуса:', errorText);
          return false;
        }
        const index = this.factData.findIndex(i => i.id === item.id);
        if (index !== -1) {
          const updatedItem = { ...this.factData[index] };
          updatedItem.entry_checked = territory_status === 1;
          updatedItem.exit_checked = territory_status === 2;
          this.factData.splice(index, 1, updatedItem);
        }
        return true;
      } catch (error) {
        console.error('Ошибка сети:', error);
        return false;
      }
    },

    async deleteItem(item) {
      const ok = await useUiStore().confirm({
        title: 'Удаление записи',
        message: 'Удалить запись?',
        confirmText: 'Удалить',
        danger: true,
      });
      if (!ok) return;
      try {
        if (this.tableType === 'cars') {
          const response = await apiRequest(`/cars/${item.id}/deactivate`, {
            method: "PUT",
            body: JSON.stringify({ status: 0, user_id: this.currentUserId })
          });
          if (response.ok) this.factData = this.factData.filter(i => i.id !== item.id);
        } else {
          this.factData = this.factData.filter(i => i.id !== item.id);
        }
      } catch (error) {
        console.error("Ошибка при удалении:", error);
      }
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
      const timeWithoutSeconds = timeString.split(':').slice(0, 2).join(':');
      const [hours, minutes] = timeWithoutSeconds.split(':').map(Number);
      return hours * 60 + minutes;
    },

    extractPassTime(passTime) {
      if (!passTime || passTime === '-') return 0;
      const startTime = passTime.split('-')[0];
      const [hours, minutes] = startTime.split(':').map(Number);
      return hours * 60 + minutes;
    },

    openItemDetails(item) {
      if (this.tableType !== 'cars') return;
      this.selectedVehicle = {
        id: item.id,
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
        exit_checked: item.exit_checked,
        car_number: item.car_number,
        car_brand: item.car_brand
      };
      this.showDetailsModal = true;
    },

    closeDetailsModal() {
      this.showDetailsModal = false;
      this.selectedVehicle = null;
    },

    startPolling() {
      if (this.pollingInterval) return;
      // Только самый первый вызов - не тихий: показывать нечего, спиннер уместен, если
      // сеть медленная. Дальше (таймер, SSE, ручное «Обновить») - без него, см. isLoading.
      this.loadData();
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

    async exportToExcel() {
      const rows = this.filteredData;
      if (!rows.length) return;

      const isCars = this.tableType === 'cars';
      const workbook = new ExcelJS.Workbook();
      const sheetName = isCars ? 'Fakt_Avtomobili' : 'Fakt_Lyudi';
      const worksheet = workbook.addWorksheet(sheetName);

      const headers = isCars
        ? ['Въезд', 'Выезд', 'Номер Т/С', 'Марка', 'Организация', 'Компания', 'Дата до', 'Время', 'Статус']
        : ['Въезд', 'Выезд', 'Фамилия', 'Имя', 'Отчество', 'Должность', 'Гражданство', 'Организация', 'Дата до', 'Время прохода', 'Статус'];

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
        const rowData = isCars
          ? [
              item.entry_checked ? 'Да' : 'Нет',
              item.exit_checked ? 'Да' : 'Нет',
              item.car_number || '-',
              item.car_brand || '-',
              item.organization_name || '-',
              item.company || '-',
              this.formatDate(item.entry_date_to),
              this.formatTimeRange(item.entry_time_from, item.entry_time_to),
              item.status || '-',
            ]
          : [
              item.entry_checked ? 'Да' : 'Нет',
              item.exit_checked ? 'Да' : 'Нет',
              item.last_name || '-',
              item.first_name || '-',
              item.middle_name || '-',
              item.position || '-',
              item.citizenshipName || item.citizenship_name || '-',
              item.organization_name || '-',
              this.formatDate(item.entry_date_to),
              this.formatPassTime(item.pass_time),
              item.status || '-',
            ];

        const row = worksheet.addRow(rowData);
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

      worksheet.columns = isCars
        ? [{ width: 10 }, { width: 10 }, { width: 18 }, { width: 22 }, { width: 35 }, { width: 25 }, { width: 14 }, { width: 18 }, { width: 20 }]
        : [{ width: 10 }, { width: 10 }, { width: 22 }, { width: 18 }, { width: 18 }, { width: 22 }, { width: 20 }, { width: 35 }, { width: 14 }, { width: 16 }, { width: 20 }];

      const buffer = await workbook.xlsx.writeBuffer();
      const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.download = `${sheetName}_${dateStr.replace(/[.:,\s]/g, '-')}.xlsx`;
      a.href = url;
      a.click();
      window.URL.revokeObjectURL(url);
    },
  }
};
</script>

<style scoped>
.fact-table-card {
  background-color: var(--surface);
  border-radius: 30px;
  border: 1px solid var(--border);
  overflow: hidden;
  width: 100%;
  min-height: 222px;
  max-height: 222px;
}

.card-header {
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0px 20px;
  height: 40px;
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

.card-title {
  margin: 0;
  color: var(--text);
  font-weight: 600;
  font-size: 1.1em;
}

.highlight-text {
  color: var(--accent-text);
}

.card-content {
  padding: 0;
  height: calc(100% - 40px);
  display: flex;
  flex-direction: column;
}

/* fact-header повторяет геометрию fact-body (padding-right + margin-right 4px),
   чтобы доступная ширина колонок совпала и заголовки выровнялись с данными. */
.fact-header {
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
  padding-right: 4px;
  margin-right: 4px;
}

/* header-row повторяет геометрию fact-row: padding 10/16 + flex + gap 4. */
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

/* Веса колонок (flex-grow) - как в CarsTable. Inline-style из getColStyle
   перебивает базовые веса при наличии пользовательской ширины в конфиге. */
.entry-col { flex: 6.5 0 0; }
.exit-col { flex: 8 0 0; }
.number-col { flex: 10 0 0; }
.brand-col { flex: 10 0 0; }
.organization-col { flex: 18 0 0; }
.company-col { flex: 12 0 0; }
.application-col { flex: 12 0 0; }

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
.date-col { flex: 12 0 0; }
.time-col { flex: 10 0 0; }
.status-col { flex: 7 0 0; }
.last-name-col { flex: 14 0 0; }
.first-name-col { flex: 9 0 0; }
.middle-name-col { flex: 11 0 0; }
.position-col { flex: 11 0 0; }
.citizenship-col { flex: 10 0 0; }
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

/* Подпись столбца сжимается с многоточием: раньше длинный заголовок распирал
   ячейку и наезжал на соседний столбец в режиме «Сетка» (#1307). */
.header-row .col > p {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin: 0;
}

.header-row .col:hover {
  color: var(--text);
}

.active-sort {
  color: var(--text) !important;
  font-weight: 500 !important;
}

.fact-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.fact-body {
  overflow-y: auto;
  flex-grow: 1;
  padding-right: 4px;
  margin-right: 4px;
  scroll-behavior: smooth;
}

.fact-item {
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
  .fact-item:hover {
    background-color: var(--surface-2);
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

.fact-row {
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

.fact-body::-webkit-scrollbar {
  width: 6px;
}

.fact-body::-webkit-scrollbar-track {
  background: transparent;
  margin: 2px 0;
  border-radius: 3px;
}

.fact-body::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, var(--accent) 22%, var(--surface));
  border-radius: 3px;
  border: 1px solid transparent;
  background-clip: content-box;
  transition: all 0.3s ease;
}

.fact-body::-webkit-scrollbar-thumb:hover {
  background: color-mix(in srgb, var(--accent) 22%, var(--surface));
  border: 1px solid transparent;
  background-clip: content-box;
  transform: scale(1.1);
}

.fact-body {
  scrollbar-width: thin;
  scrollbar-color: color-mix(in srgb, var(--accent) 22%, var(--surface)) transparent;
  scroll-behavior: smooth;
  overscroll-behavior: contain;
}

.fade-list-enter-active,
.fade-list-leave-active {
  transition: all 0.5s ease;
}

.fade-list-enter-from,
.fade-list-leave-to {
  opacity: 0;
  transform: translateY(10px);
}

.fade-list-move {
  transition: transform 0.5s ease;
}

@media (max-width: 767.98px) {
  /* Высота - по содержимому в обе стороны. `min-height: 222px` из базовых стилей
     держит на десктопе ряд с карточкой-подсказкой; на телефоне подсказка стоит
     отдельным блоком, а резерв высоты остаётся резервом под список, которого может
     не быть вовсе: пустая таблица «по факту» занимала 222px + шапку 68 при том, что
     показывала одну строку «Заявок на машины по факту нет».

     Рамка и фон блока - на месте: без них таблица «по факту» (почти всегда пустая)
     схлопывалась в голый абзац текста под заголовком - "куда она делась?". Радиус
     тот же, что на десктопе (30px) - владелец забраковал 15px волны 7 отдельно
     ("скруглить как и по факту", "таблицам больше скругление нужно дать, как и
     было"); значение общее с CarsTable/PeopleTable ниже. Заголовок отделяет линия
     снизу. */
  .fact-table-card {
    width: 100%;
    height: auto;
    min-height: 0;
    max-height: none;
    border: 1px solid var(--border);
    border-radius: 30px;
    background: var(--surface);
  }

  /* Своя первая загрузка (см. комментарий у `isLoading` в шаблоне выше) схлопывает
     страницу до высоты спиннера - без резерва документ короче вьюпорта, и палец,
     потянувший список машин сразу после открытия таблицы, упирается в конец
     документа на первых же миллиметрах жеста: браузер решает, скроллить страницу
     дальше или нет, один раз на touchstart, и до конца жеста уже не пересматривает
     решение - требуется отпустить и начать заново (замер: непрерывный драг во
     время загрузки стоял на месте весь жест). Резерв только на время спиннера -
     `.no-data-message` выше держит `flex-grow: 0` намеренно (не резервировать
     место под постоянно пустую таблицу), к спиннеру это не относится - он живёт
     секунду-две и после загрузки уступает место либо списку, либо той же надписи.
     Возвращён вместе с прокруткой документа (волна 14) - без внутренней панели
     вьюпорта обрыв жеста снова возможен. */
  .loading-message {
    min-height: calc(var(--app-vh, 1vh) * 45);
  }

  /* Шапка блока - один ряд в 48px (контракт волны 6, те же числа у соседних
     экранов): имя блока кеглем 18, «Обновить» у правого края, переноса нет. В
     настройках шапки здесь только сама кнопка, поэтому выносить её отдельным
     элементом, как в CarsTable/PeopleTable, не нужно.

     Боковой отступ слагаемыми, а не числом: рамка карточки + внутренний отступ
     строки - заголовок стоит над текстом карточек (тело списка своего бокового
     отступа больше не добавляет, см. `.fact-body` ниже). Прежние 16px по кругу
     давали шапку в 68px над пустой таблицей. */
  .card-header {
    flex-direction: row;
    flex-wrap: nowrap;
    align-items: center;
    gap: 8px;
    height: 48px;
    padding: 0 calc(1px + 14px);
  }

  .card-title {
    font-size: 18px;
  }

  .card-header__settings {
    margin-left: auto;
  }

  /* Строка «Заявок по факту нет» - подпись под шапкой, а не пустой экран: центровка
     по вертикали вместе с flex-grow растягивала её на всю высоту карточки. Боковой
     отступ добирает до вертикали заголовка: 8px тело списка уже дало. */
  .no-data-message {
    flex-grow: 0;
    justify-content: flex-start;
    padding: 14px calc(1px + 14px);
    font-size: 13px;
    text-align: left;
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

  .card-title {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .card-header__settings {
    flex-shrink: 0;
  }

  /* Талон - не карточка со своим зазором (паттерн Центра), а строка настоящей
     таблицы: рамку и фон блока теперь даёт сам `.fact-table-card` (выше). Полный
     бордер+радиус части 1 responsive-tables.css у КАЖДОЙ строки поверх рамки
     контейнера и был «квадратом в квадрате». Строки идут вплотную (зазора нет),
     разделяет только горизонтальная черта; скругление контейнера получают
     исключительно верхний край первой строки и нижний край последней - середина
     остаётся прямоугольной. Радиус строки равен радиусу контейнера (30px) - строка
     стоит вплотную к рамке (см. `.fact-body` ниже), поэтому кривые продолжают
     друг друга без излома. */
  .fact-item + .fact-item {
    margin-top: 0;
  }

  .fact-table-card .rt-pass {
    border-radius: 0 !important;
    border-left: none !important;
    border-right: none !important;
    border-top: none !important;
    background: transparent !important;
  }

  .fact-table-card .fact-item:not(:last-child) .rt-pass {
    border-bottom: 2px solid var(--border) !important;
  }

  .fact-table-card .fact-item:last-child .rt-pass {
    border-bottom: none !important;
  }

  .fact-table-card .fact-item:first-child .rt-pass {
    border-top-left-radius: 30px !important;
    border-top-right-radius: 30px !important;
  }

  .fact-table-card .fact-item:last-child .rt-pass {
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
  .fact-body {
    padding: 0;
    margin-right: 0;
  }

  /* Значения в карточке не обрезаем многоточием - там больше горизонтального
     места, чем в узкой табличной колонке. */
  .fact-table-card .rt-row > [data-label] {
    white-space: normal;
    overflow: visible;
    text-overflow: clip;
  }

  /* #1097 S9. Обёртку полосы заголовков убираем целиком, а не только её внутренний ряд:
     глобальный `rt-head-row` прячет `.header-row`, а `.fact-header` остаётся в потоке
     со своим `border-bottom` и рисует лишнюю линию в 1px перед первой карточкой
     (замер: height 1 при вьюпорте 320 и 390). Ловушка описана в эталоне, §8.

     Селектор длиннее собственного `.fact-header`, чтобы исход не зависел от порядка
     правил: базовое правило стоит выше по файлу, но при равной специфичности его хватило
     бы перенести ниже, чтобы линия вернулась. Закреплению полосы это не мешает - оно
     живёт в `@media (min-width: 768px)` и сюда не достаёт. */
  .fact-table-card .fact-header {
    display: none;
  }

  /* #1097 S9. Карточка по образцу заявки (ApplicationAttachmentDetail.vue): подписи
     полей убраны, значения выровнены влево, разделитель рисуется сверху.

     Кнопки прохода при этом стояли двумя отдельными строками, и слева от каждой висела
     дублирующая подпись - "Въезд" подписью и "Въезд" кнопкой в одной строке. Поэтому
     карточка переведена из колонки в строку с переносом, а перенос во флексе держит
     БАЗИС, а не ширина. В таблице людей кнопок прохода нет - там просто нечему делить
     строку, и карточка остаётся прежним стеком полей.

     Специфичность выше правил-источников и `!important` обязательны: те объявлены с
     `!important` сами, и более коротким селектором их не перебить. */
  .fact-table-card .fact-row.rt-row {
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

     Правило целит во ВСЕ дочерние ячейки, а не в `[data-label]`: колонка действий
     подписи не несёт и иначе осталась бы со своей табличной долей, уехав в ряд к
     кнопкам. Из этого правила выходят только сами кнопки прохода - ниже. */
  .fact-table-card .rt-row > * {
    flex: 0 0 100% !important;
    width: 100% !important;
    min-width: 0 !important;
  }

  /* Разделитель полей рисуем сверху у ячеек 2..N, а не снизу: последней в строке идёт
     колонка действий без data-label, глобальное `[data-label]:last-child` до неё не
     достаёт, и пунктир висел бы оторванной чертой над нижним краем карточки. */
  .fact-table-card .rt-row > [data-label] {
    justify-content: flex-start !important;
    text-align: left !important;
    border-bottom: none !important;
  }

  /* Только в режиме people: у машин строка собирается талоном, где единственная
     горизонтальная линия - линия отрыва, и пунктиры её глушат. */
  .fact-table-card .rt-row:not(.rt-pass) > [data-label] ~ [data-label] {
    border-top: 1px dashed color-mix(in srgb, var(--border) 60%, var(--surface));
  }

  /* Ячейки прохода делят верхнюю строку пополам - единственные, кто выходит из
     «своя строка каждому». Это шапка талона: то, ради чего экран открывают.

     Базис ровно половина, а не 0 с ростом: перенос строк во flex считается по
     базисам ДО распределения свободного места, поэтому при нулевом базисе в первую
     строку набиралась ещё и следующая ячейка, свободного места не оставалось, и обе
     кнопки схлопывались друг на друга. */
  .fact-table-card .rt-row > .entry-col,
  .fact-table-card .rt-row > .exit-col {
    width: auto !important;
    flex: 0 0 calc(50% - 4px) !important;
    padding: 5px 0 !important;
    border-top: none !important;
  }

  .fact-table-card .rt-row > .entry-col .action-btn,
  .fact-table-card .rt-row > .exit-col .action-btn {
    width: 100%;
    min-width: 0;
  }

  .fact-table-card .rt-row > [data-label]::before {
    display: none !important;
  }

  /* Исключение из "убрать все подписи": значение, которое без подписи не отличить от
     соседнего такого же. Организация и компания идут двумя строками с однотипными
     названиями; должность, гражданство, номер заявки, место разгрузки, дата и время -
     голые значения, которые сами себя не называют. Номер Т/С, марка и бейдж статуса
     говорят за себя, фамилия с именем и отчеством стоят подряд и читаются как ФИО. */
  .fact-table-card .rt-row > .organization-col::before,
  .fact-table-card .rt-row > .company-col::before,
  .fact-table-card .rt-row > .application-col::before,
  .fact-table-card .rt-row > .place-col::before,
  .fact-table-card .rt-row > .position-col::before,
  .fact-table-card .rt-row > .citizenship-col::before,
  .fact-table-card .rt-row > .date-col::before,
  .fact-table-card .rt-row > .time-col::before {
    display: block !important;
  }

  /* Статус в карточке телефона не нужен: экран открывают ради проезда, а не ради
     состояния заявки, и своей строкой в подвале он только оттягивал место у кнопки
     «Удалить». Прячем целиком, а не разворачиваем строкой - было `order:9999;
     flex:0 0 100%`. */
  .fact-table-card .rt-pass > .status-col {
    display: none !important;
  }

  /* Подвал талона: под линией отрыва сразу «Удалить», без статуса над ней. Порядок
     заведомо большим числом - разметочный `order` колонки действий (9999) иначе
     соседствует с порядком настраиваемых столбцов.

     `overflow: visible` обязателен: базовый `.col { overflow: hidden }` обрезает
     невидимый ::before, которым кнопка добирает зону нажатия до 44px. */
  .fact-table-card .rt-pass > .actions-col {
    order: 10001 !important;
    flex: 0 0 calc(50% - 4px) !important;
    width: auto !important;
    overflow: visible;
    padding: 10px 0 0;
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

  /* Режим people: талона нет, кнопка удаления остаётся тач-таргетом 44px. У машин
     её перебивает пилюля подвала из rt-pass. */
  .delete-btn {
    width: 44px;
    height: 44px;
  }
}

.loading-message {
  text-align: center;
  color: var(--text-muted);
  padding: 40px 20px;
  font-size: 14px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
}

/* Полоса заголовков столбцов не уезжает при прокрутке страницы (#1097 S8 волна 4).
   Список прокручивается и внутри карточки (.fact-body), но саму карточку на
   планшете видно не целиком - страница прокручивается вместе с ней, и статичная
   полоса уходила за верх экрана: столбцы оставались без подписей.

   Карточка режется `clip`, а не `hidden`: `hidden` делает предка скроллпортом,
   и sticky внутри него замирает на месте (прилипать не к чему). `clip` обрезает
   ровно так же - скругление 30px цело, - но скроллпорта не создаёт, поэтому
   отсчёт идёт от прокрутки документа. Там же живут шапка приложения и шапки
   списков (эталон: все закреплённые полосы в одной системе отсчёта). Браузер без
   поддержки `clip` просто оставит прежний `hidden` и прежнее поведение.

   Фон обязателен и обязан быть непрозрачным - строки уходят ПОД полосу;
   --surface в обеих палитрах задан hex-ом, без альфы. z-index 3 - выше
   .fact-container, который идёт следом в разметке.

   На мобилке правило не действует - там шапка скрыта (rt-head-row), строки
   показываются карточками. */
@media (min-width: 768px) {
  /* min-width здесь не украшение: карточка - flex-элемент строки .fact-section
     (рядом карточка-подсказка), и `hidden` заодно обнулял её автоминимум по
     главной оси. Без него широкая таблица распирала бы секцию. Задаём нулевой
     минимум явно, чтобы ширина не зависела от того, как браузер трактует
     автоминимум при `clip`. */
  .fact-table-card {
    overflow: clip;
    min-width: 0;
  }

  .fact-header {
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
  .fact-header {
    top: var(--mobile-header-height);
  }
}

/* #345 PR-B: размер шрифта строк через CSS-переменную (только тело). */
.fact-table-card .fact-body .col {
  font-size: var(--table-font-size, 14px);
}

/* Плотность строк через padding row. */
.fact-table-card.density-compact .fact-row,
.fact-table-card.density-compact .header-row {
  padding-top: 4px;
  padding-bottom: 4px;
}

.fact-table-card.density-spacious .fact-row,
.fact-table-card.density-spacious .header-row {
  padding-top: 16px;
  padding-bottom: 16px;
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
  .fact-table-card.grid-mode .header-row,
  .fact-table-card.grid-mode .fact-row {
    overflow: clip;
  }

  .fact-table-card.grid-mode .header-row > .col,
  .fact-table-card.grid-mode .fact-row > .col {
    position: relative;
    overflow: clip;
    /* Заведомо больше любой строки - лишнее подрежет строка. */
    overflow-clip-margin: 200px;
  }

  .fact-table-card.grid-mode .header-row > .col::after,
  .fact-table-card.grid-mode .fact-row > .col::after {
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
  .fact-table-card.grid-mode .col--collapsed::after,
  .fact-table-card.grid-mode .header-row > .col:last-child::after,
  .fact-table-card.grid-mode .fact-row > .col:last-child::after {
    display: none;
  }
}

/* Пока конфиг не загружен - запрещаем transitions на всех потомках, чтобы
   шапка/строки не "ездили" между дефолтами и сохранёнными значениями. */
.fact-table-card.config-not-ready,
.fact-table-card.config-not-ready * {
  transition: none !important;
}
</style>
