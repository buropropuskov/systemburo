<template>
  <div class="data__list">
    <div class="header-with-badge">
      <h4>
        <span class="list-title__full">Список транспортных средств</span>
        <span class="list-title__short">Транспорт</span>
      </h4>
      <span class="vehicles-badge">{{ vehicles.length }}</span>
      <!-- Действия шапки: вход в импорт (когда доступен) и очистка списка (когда есть,
           что чистить) - обе живут в шапке справа, а не в отдельной полосе тулбара под
           ней: иначе при коротком списке (нет поиска/пейджера) тулбар оставался пустой
           строкой под кнопку и раздувал пробел между шапкой и таблицей.
           На телефоне очистка уезжает в подвал блока (см. .list-foot ниже): счётчик и
           действие в одной строке с заголовком переносились. -->
      <div
        v-if="canImport || (!isNarrow && vehicles.length)"
        class="header-actions"
      >
        <div
          v-if="canImport"
          class="import-entry"
        >
          <span
            class="hint-anchor"
            data-hint="Массовый ввод из бланка в опытной эксплуатации: проверяйте, что попало в список"
          >
            <Badge
              variant="warning"
              size="sm"
              label="Experimental"
            />
          </span>
          <!-- На телефоне подпись выхода короче: полная («Закрыть импорт») вместе с
               заголовком, счётчиком и бейджем режима не оставляла шапке ни одной
               свободной точки на 360. Доступное имя остаётся полным - видимый текст
               его префикс, требование «label in name» соблюдено. -->
          <button
            type="button"
            class="lk-button lk-button--secondary lk-button--sm list-mini-btn import-entry__btn"
            data-testid="vehicles-import-btn"
            :aria-pressed="importActive ? 'true' : 'false'"
            :aria-label="importActive ? 'Закрыть импорт' : 'Импорт'"
            @click="$emit('toggle-import')"
          >
            {{ importActive ? (isNarrow ? 'Закрыть' : 'Закрыть импорт') : 'Импорт' }}
          </button>
        </div>
        <button
          v-if="!isNarrow && vehicles.length"
          type="button"
          class="lk-button lk-button--danger lk-button--sm list-toolbar__clear"
          data-testid="vehicles-clear-btn"
          @click="showClearConfirm = true"
        >
          Очистить
        </button>
      </div>
    </div>

    <!-- Поиск с пейджером - только когда список длинный (импорт бланком может завести
         до 2000 машин, обычной ручной подаче они не нужны). Короткий список этой
         строки не занимает вовсе. -->
    <div
      v-if="showToolbar"
      class="list-toolbar"
    >
      <input
        v-model="searchQuery"
        type="text"
        class="lk-input list-search"
        placeholder="Поиск по номеру или марке"
        data-testid="vehicles-search"
      >
      <Pager
        v-if="totalPages > 1"
        class="list-pager"
        :page="currentPage"
        :total-pages="totalPages"
        :total="filteredVehicles.length"
        page-prefix="Стр. "
        @update:page="goToPage"
      />
    </div>

    <!-- Десктоп/планшет: колоночная раскладка. В DOM ячейки идут по столбцам, поэтому
         выделение мышью сверху-вниз захватывает один столбец и копируется как список
         значений (Ctrl+C). Мобилка (карточки) - строковый блок ниже. -->
    <div
      v-if="!isNarrow"
      class="vehicles-cols"
      @mouseleave="hoveredIndex = null"
    >
      <div class="vcol vcol--index">
        <div
          class="vcol__head"
          @click="$emit('sort', 'number')"
        >
          <p :class="{ 'active-sort': sortField === 'number' }">
            №
          </p>
          <AppIcon
            name="sort"
            class="sort-icon"
            :class="{ 'desc': sortField === 'number' && sortDirection === 'desc' }"
          />
        </div>
        <div
          v-for="(row, index) in pagedVehicles"
          :key="row.item.id"
          class="vcol__cell vcol__cell--muted"
          data-testid="vehicles-row"
          :class="rowState(row.item, index)"
          @mouseenter="hoveredIndex = index"
        >
          {{ row.number }}
        </div>
      </div>

      <div class="vcol vcol--plate">
        <div
          class="vcol__head"
          @click="$emit('sort', 'plate')"
        >
          <p :class="{ 'active-sort': sortField === 'plate' }">
            Номер
          </p>
          <AppIcon
            name="sort"
            class="sort-icon"
            :class="{ 'desc': sortField === 'plate' && sortDirection === 'desc' }"
          />
        </div>
        <div
          v-for="(row, index) in pagedVehicles"
          :key="row.item.id"
          class="vcol__cell vcol__cell--text vcol__cell--plate"
          :class="rowState(row.item, index)"
          @mouseenter="hoveredIndex = index"
        >
          <span class="vcol__value">{{ row.item.plateNumber || 'Не указано' }}</span>
          <!-- Строка из бланка ещё не в заявке: приглушённого цвета мало, статус
               называем словами (blank-import-ux, доводка U5). -->
          <Badge
            v-if="row.item.isPending"
            class="pending-badge"
            variant="info"
            size="sm"
            label="В очереди"
          />
        </div>
      </div>

      <div class="vcol vcol--mark">
        <div
          class="vcol__head"
          @click="$emit('sort', 'mark')"
        >
          <p :class="{ 'active-sort': sortField === 'mark' }">
            Марка
          </p>
          <AppIcon
            name="sort"
            class="sort-icon"
            :class="{ 'desc': sortField === 'mark' && sortDirection === 'desc' }"
          />
        </div>
        <div
          v-for="(row, index) in pagedVehicles"
          :key="row.item.id"
          class="vcol__cell vcol__cell--text"
          :class="rowState(row.item, index)"
          @mouseenter="hoveredIndex = index"
        >
          {{ row.item.mark || 'Не указано' }}
        </div>
      </div>

      <div class="vcol vcol--actions">
        <div class="vcol__head vcol__head--actions">
          Действия
        </div>
        <div
          v-for="(row, index) in pagedVehicles"
          :key="row.item.id"
          class="vcol__cell vcol__cell--actions"
          :class="rowState(row.item, index)"
          @mouseenter="hoveredIndex = index"
        >
          <button
            class="details-btn"
            title="Детали"
            @click="showVehicleDetails(row.item)"
          >
            <DetailsIcon class="details-icon" />
          </button>
          <button
            class="edit-btn"
            title="Редактировать"
            @click="$emit('edit-vehicle', row.item)"
          >
            <AppIcon
              name="edit"
              class="edit-icon"
            />
          </button>
          <button
            class="delete-btn"
            title="Удалить"
            @click="$emit('delete-vehicle', row.item.id)"
          >
            <AppIcon
              name="trashcan"
              class="delete-icon"
            />
          </button>
        </div>
      </div>

      <!-- Пустое состояние - строка внутри самой таблицы под шапкой колонок, а не
           отдельный блок под ней: причина пустоты разная, место одно. -->
      <div
        v-if="emptyMessage"
        class="table-empty"
        data-testid="vehicles-empty"
      >
        {{ emptyMessage }}
      </div>
    </div>

    <!-- Мобилка: строки становятся карточками (rt-* из responsive-tables.css). -->
    <div
      v-else
      class="vehicles-table rt-table"
    >
      <div class="table-header rt-head-row">
        <div
          class="header-col number-col"
          @click="$emit('sort', 'number')"
        >
          <p :class="{ 'active-sort': sortField === 'number' }">
            №
          </p>
        </div>
        <div
          class="header-col plate-col"
          @click="$emit('sort', 'plate')"
        >
          <p :class="{ 'active-sort': sortField === 'plate' }">
            Номер
          </p>
        </div>
        <div
          class="header-col mark-col"
          @click="$emit('sort', 'mark')"
        >
          <p :class="{ 'active-sort': sortField === 'mark' }">
            Марка
          </p>
        </div>
        <div class="header-col actions-col">
          Действия
        </div>
      </div>
      <div class="table-body">
        <div
          v-for="row in pagedVehicles"
          :key="row.item.id"
          class="table-row rt-row"
          data-testid="vehicles-row"
          :class="{ 'has-active': row.item.activeInfo, 'is-pending': row.item.isPending }"
        >
          <div class="table-col number-col">
            {{ row.number }}
          </div>
          <div class="table-col plate-col">
            <div class="cell-with-icon">
              <span class="cell-value">{{ row.item.plateNumber || 'Не указано' }}</span>
              <Badge
                v-if="row.item.isPending"
                class="pending-badge"
                variant="info"
                size="sm"
                label="В очереди"
              />
            </div>
          </div>
          <div class="table-col mark-col">
            <div class="cell-with-icon">
              {{ row.item.mark || 'Не указано' }}
            </div>
          </div>
          <!-- Вторая строка карточки: где машина разгружается и в какие часы. Время
               общее на вложение (detailInfo), место - своё у каждой строки. Нет ни
               того, ни другого - строки нет вовсе, пустых подписей не рисуем. -->
          <div
            v-if="row.item.unloadingPlace || passTime"
            class="table-col meta-col"
            data-testid="vehicles-row-meta"
          >
            <span class="meta-col__place">{{ row.item.unloadingPlace || 'Место не выбрано' }}</span>
            <span
              v-if="passTime"
              class="meta-col__time"
            >{{ passTime }}</span>
          </div>
          <!-- Действия - подвалом карточки бейджами с подписями: иконками поперёк
               строки они съедали больше половины ширины у номера и марки. Иконок здесь
               нет вовсе: ветка рендерится только на телефоне, а скрытый через CSS
               <img> браузер всё равно загружает. -->
          <div class="table-col actions-col">
            <button
              class="edit-btn"
              @click="$emit('edit-vehicle', row.item)"
            >
              Изменить
            </button>
            <button
              class="delete-btn"
              @click="$emit('delete-vehicle', row.item.id)"
            >
              Удалить
            </button>
            <button
              class="details-btn"
              @click="showVehicleDetails(row.item)"
            >
              Детали
            </button>
          </div>
        </div>

        <div
          v-if="emptyMessage"
          class="table-empty"
          data-testid="vehicles-empty"
        >
          {{ emptyMessage }}
        </div>
      </div>
    </div>

    <!-- Итог блока отдельной строкой (только телефон): счётчик слева, очистка справа.
         В строке заголовка та же пара переносилась вместе с подписью списка. -->
    <div
      v-if="isNarrow && vehicles.length"
      class="list-foot"
    >
      <span
        class="list-foot__total"
        data-testid="vehicles-total"
      >Всего {{ totalLabel }}</span>
      <button
        type="button"
        class="lk-button lk-button--danger lk-button--sm list-mini-btn list-foot__clear"
        data-testid="vehicles-clear-btn"
        @click="showClearConfirm = true"
      >
        Очистить
      </button>
    </div>

    <!-- Очистка списка необратима (отмены на странице нет), поэтому идёт только через
         подтверждение; само удаление делает родитель по clear-list. -->
    <ConfirmationModal
      :show="showClearConfirm"
      title="Очистить список"
      :message="clearConfirmMessage"
      confirm-text="Очистить"
      cancel-text="Отмена"
      :confirm-button-style="{ background: 'var(--danger)', borderColor: 'var(--danger)', color: 'var(--fill-text)' }"
      @confirm="confirmClear"
      @cancel="showClearConfirm = false"
    />

    <!-- Модальное окно деталей транспортного средства -->
    <VehicleDetailsModal
      :show="showDetailsModal"
      :vehicle="selectedVehicle"
      :all-unloading-places="allUnloadingPlaces"
      :license-plate-formats="licensePlateFormats"
      :show-car-features="false"
      :readonly="true"
      :active-info="selectedVehicle?.activeInfo"
      @close="closeDetailsModal"
    />
  </div>
</template>

<script>
import VehicleDetailsModal from './VehicleDetailsModal.vue';
import ConfirmationModal from '@/components/ConfirmationModal.vue';
import Badge from '@/components/ui/Badge.vue';
import DetailsIcon from '@/components/ui/DetailsIcon.vue';
import Pager from '@/components/ui/Pager.vue';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import { useListSearchPagination } from '@/composables/useListSearchPagination';
import { entityCountLabel } from '@/utils/entityCount';
import AppIcon from '@/components/icons/AppIcon.vue';

export default {
    name: 'VehiclesList',
    components: {
        VehicleDetailsModal,
        ConfirmationModal,
        Badge,
        DetailsIcon,
        Pager,
        AppIcon,
    },
    props: {
        vehicles: {
            type: Array,
            required: true
        },
        sortField: { type: String, default: null },
        sortDirection: { type: String, default: null },
        allUnloadingPlaces: {
            type: Array,
            default: () => []
        },
        licensePlateFormats: {
            type: Array,
            default: () => []
        },
        showStatus: {
            type: Boolean,
            default: true
        },
        // Дата+время текущего вложения - на сущности машины их нет, подмешиваем
        // при открытии карточки просмотра (срок действия и время пребывания).
        detailInfo: {
            type: Object,
            default: () => ({})
        },
        // Вход в массовый ввод из бланка (blank-import-ux, U4): гейт права
        // action.import.list считает родитель, здесь только показ кнопки.
        canImport: {
            type: Boolean,
            default: false
        },
        importActive: {
            type: Boolean,
            default: false
        }
    },
    emits: ['sort', 'edit-vehicle', 'delete-vehicle', 'toggle-import', 'clear-list'],
    // 767.98 - тот же порог, что у карточного @media: ниже него рендерим карточки,
    // выше - колоночную раскладку с выделением по столбцам.
    setup(props) {
        const { isNarrow } = useNarrowScreen(767.98);

        // Поиск+постраничный показ - см. useListSearchPagination (blank-import E1: до
        // 2000 машин, рендерить всё v-for'ом - да ещё продублированным на 4 колонки в
        // десктопной раскладке - не годится).
        const {
            searchQuery,
            currentPage,
            showToolbar,
            filteredItems: filteredVehicles,
            totalPages,
            pagedItems: pagedVehicles,
            goToPage
        } = useListSearchPagination(
            () => props.vehicles,
            (vehicle, q) => `${vehicle.plateNumber || ''} ${vehicle.mark || ''}`.toLowerCase().includes(q)
        );

        return {
            isNarrow,
            searchQuery,
            currentPage,
            showToolbar,
            filteredVehicles,
            totalPages,
            pagedVehicles,
            goToPage
        };
    },
    data() {
        return {
            showDetailsModal: false,
            selectedVehicle: null,
            showClearConfirm: false,
            // Индекс строки под курсором: подсвечиваем ячейки того же индекса во всех
            // столбцах (в колоночном DOM «строки» как элемента нет - синхроним по index).
            hoveredIndex: null
        }
    },
    computed: {
        // Считаем по ВСЕМУ списку, а не по видимой странице: поиск и пейджер режут показ,
        // а чистится вложение целиком - иначе число в вопросе обещало бы меньше, чем уйдёт.
        clearConfirmMessage() {
            const pending = this.vehicles.filter(vehicle => vehicle.isPending).length;
            const fromBlank = pending > 0 ? `, из них предварительных из бланка: ${pending}` : '';
            return `Будет убрано строк: ${this.vehicles.length}${fromBlank}. Отменить это действие нельзя.`;
        },

        // Итог блока в подвале: «Всего 2 машины» - со склонением по числу.
        totalLabel() {
            return entityCountLabel(this.vehicles.length, 'vehicles');
        },

        // Часы пребывания берём с вложения (на самой машине их нет) - вторая строка
        // карточки на телефоне. Половинчатый интервал не показываем.
        passTime() {
            const info = this.detailInfo || {};
            return info.timeFrom && info.timeTo ? `${info.timeFrom}—${info.timeTo}` : '';
        },

        // Пустая таблица объясняет причину пустоты: список не заполняли вовсе или поиск
        // ничего не нашёл. Предварительные строки из бланка тоже считаются заполнением.
        emptyMessage() {
            if (this.vehicles.length === 0) return 'Нет добавленных транспортных средств';
            if (this.filteredVehicles.length === 0) return `Ничего не найдено по запросу «${this.searchQuery}»`;
            return '';
        }
    },
    watch: {
        currentPage() {
            // Клик по пейджеру не даёт mouseleave - без сброса подсветка осталась бы
            // на строке с тем же ЛОКАЛЬНЫМ индексом, то есть на другой машине.
            this.hoveredIndex = null;
        }
    },
    methods: {
        // Классы состояния ячейки: жёлтая подсветка уже активной машины и hover-строка
        // по общему индексу (колонки - отдельные элементы, CSS :hover строку не даёт).
        rowState(vehicle, index) {
            return {
                'vcol__cell--active': !!vehicle.activeInfo,
                'vcol__cell--hover': this.hoveredIndex === index,
                // Строка из бланка, ещё не добавленная в заявку (blank-import-ux, U5).
                'vcol__cell--pending': !!vehicle.isPending
            };
        },

        confirmClear() {
            this.showClearConfirm = false;
            this.$emit('clear-list');
        },

        showVehicleDetails(vehicle) {
            const info = this.detailInfo || {};
            this.selectedVehicle = {
                ...vehicle,
                organization: vehicle.organization || info.organization,
                company: vehicle.company || info.company,
                entry_date_to: vehicle.entry_date_to || info.entryDateTo,
                entry_time_from: vehicle.entry_time_from || info.timeFrom,
                entry_time_to: vehicle.entry_time_to || info.timeTo
            };
            this.showDetailsModal = true;
        },

        closeDetailsModal() {
            this.showDetailsModal = false;
            this.selectedVehicle = null;
        }
    }
}
</script>

<style scoped>
.data__list {
    padding: 12px;
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
}

.header-with-badge {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    padding-bottom: 12px;
}

/* Действия списка прижаты к правому краю шапки: вход в импорт, за ним очистка.
   Перенос обязателен: у .lk-button white-space: nowrap, и на узком экране пара
   «Закрыть импорт» + «Очистить» иначе выехала бы за край шапки. */
.header-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    flex-wrap: wrap;
    gap: 8px;
    margin-left: auto;
}

.import-entry {
    display: flex;
    align-items: center;
    gap: 8px;
}

/* Короткая подпись списка для телефона - отдельный класс, а не модификатор общего:
   видимость не должна зависеть от порядка правил при правке media-блока. */
.list-title__short {
    display: none;
}

/* Подвал блока (только телефон): «Всего N машины» слева, очистка справа. */
.list-foot {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 8px;
    padding: 10px 12px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-md, 15px);
    font-size: 13px;
    color: var(--text-muted);
}

.list-foot__clear {
    margin-left: auto;
}

.vehicles-badge {
    background: var(--accent);
    color: var(--accent-contrast);
    padding: 2px 6px;
    border-radius: 10px;
    font-size: 11px;
    font-weight: 600;
    min-width: 18px;
    text-align: center;
    line-height: 1.2;
}

/* Только поиск+пейджер - рисуется исключительно у длинных списков (showToolbar),
   иначе строки в раскладке нет вовсе (см. header-actions выше для очистки). */
.list-toolbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    padding-bottom: 10px;
}

/* Границы/фон/фокус/тёмная тема - на .lk-input, здесь только раскладка в тулбаре. */
.list-search {
    flex: 1;
    min-width: 160px;
    height: 32px;
    padding: 6px 12px;
    font-size: 13px;
}

.list-pager {
    flex-shrink: 0;
    color: var(--text-muted);
}

.list-toolbar__clear {
    flex-shrink: 0;
}

/* Колоночная раскладка (десктоп/планшет): каждый столбец - отдельный flex-column,
   ячейки одного индекса выровнены по фиксированной высоте. Растягиваем на высоту
   соседней формы (data__list stretch по form__data) - список показывает максимум
   строк, а не фиксированные ~180px; переполнение уходит во внутренний скролл. */
.vehicles-cols {
    display: flex;
    /* Перенос нужен пустому состоянию: его строка встаёт под столбцами на всю ширину.
       Столбцы делят ровно 100%, поэтому на данных перенос не срабатывает. */
    flex-wrap: wrap;
    flex: 1;
    min-height: 180px;
    overflow-y: auto;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    box-shadow: 0 1px 3px var(--shadow-drop);
    scrollbar-width: thin;
}

.vcol {
    display: flex;
    flex-direction: column;
    min-width: 0;
}

.vcol--index {
    width: 10%;
    flex-shrink: 0;
    user-select: none;
    text-align: center;
}

.vcol--plate {
    width: 32%;
    /* Столбцы с данными выделяемы для копирования; служебные (№, действия) - нет. */
    user-select: text;
}

.vcol--mark {
    width: 30%;
    user-select: text;
}

.vcol--actions {
    width: 28%;
    flex-shrink: 0;
    user-select: none;
}

.vcol__head {
    position: sticky;
    top: 0;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: 4px;
    height: 40px;
    padding: 0 12px;
    box-sizing: border-box;
    background: var(--surface-2);
    border-bottom: 1px solid var(--border);
    font-weight: 500;
    color: var(--text-muted);
    font-size: 13px;
    cursor: pointer;
    user-select: none;
    transition: color 0.2s ease;
}

.vcol--index .vcol__head {
    justify-content: center;
}

.vcol__head--actions {
    cursor: default;
    justify-content: center;
}

.vcol__head p {
    margin: 0;
}

.vcol__head:hover:not(.vcol__head--actions) p,
.vcol__head p.active-sort {
    color: var(--text);
}

.sort-icon {
    color: var(--text-muted);
    width: 10px;
    height: 10px;
    transition: all 0.2s ease;
    opacity: 0.4;
    transform: rotate(0deg);
}

.vcol__head:hover .sort-icon,
.vcol__head .active-sort ~ .sort-icon {
    opacity: 0.8;
}

.sort-icon.desc {
    transform: rotate(180deg);
    opacity: 0.8;
}

.vcol__cell {
    height: 38px;
    padding: 0 12px;
    box-sizing: border-box;
    line-height: 38px;
    font-size: 13px;
    border-bottom: 1px solid var(--surface-2);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    transition: background-color 0.2s ease;
}

.vcol__cell:last-child {
    border-bottom: none;
}

.vcol__cell--muted {
    color: var(--text-muted);
    text-align: center;
}

.vcol__cell--actions {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    line-height: normal;
    overflow: visible;
}

.vcol__cell--hover {
    background: var(--surface-2);
}

.vcol__cell--active {
    background: var(--warning-bg);
    border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
}

.vcol__cell--active.vcol__cell--hover {
    background: var(--warning);
}

/* Предварительная строка (blank-import-ux, U5): разобрана из бланка, но в заявку ещё не
   добавлена - действия (детали, правка, удаление) работают как у обычной. Кроме бейджа
   «В очереди» строка заметно серее обычной: одного приглушённого текста владельцу было
   мало. Метка слева - inset-тень, а не border: он сдвинул бы текст ячейки. */
.vcol__cell--pending {
    color: var(--text-muted);
    background: color-mix(in srgb, var(--text-muted) 12%, var(--surface));
}

.vcol--index .vcol__cell--pending {
    box-shadow: inset 3px 0 0 var(--accent);
}

/* Правило серой подложки идёт после hover-правила той же специфичности, поэтому
   отклик на курсор возвращаем явно - иначе строка из бланка перестаёт реагировать. */
.vcol__cell--pending.vcol__cell--hover {
    background: color-mix(in srgb, var(--text-muted) 20%, var(--surface));
}

/* Ячейка номера несёт значение и бейдж: номер сжимается многоточием, бейдж остаётся
   целым - он и есть статус строки. */
.vcol__cell--plate {
    display: flex;
    align-items: center;
    gap: 6px;
}

.vcol__value {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
}

.pending-badge {
    flex: 0 0 auto;
}

/* --- Строковая раскладка (мобильные карточки) --- */
.vehicles-table {
    width: 100%;
    border: 1px solid var(--border);
    border-radius:20px;
    overflow: hidden;
    box-shadow: 0 1px 3px var(--shadow-drop);
}

.table-header {
    display: flex;
    background: var(--surface-2);
    border-bottom: 1px solid var(--border);
    padding: 10px 12px;
    font-weight: 500;
    color: var(--text-muted);
    font-size: 13px;
}

.header-col {
    display: flex;
    align-items: center;
    gap: 4px;
    cursor: pointer;
    transition: all 0.2s ease;
    user-select: none;
}

.header-col:hover,
.header-col.active-sort {
    color: var(--text);
}

.table-body {
    max-height: 180px;
    overflow-y: auto;
    background: var(--surface);
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.table-body::-webkit-scrollbar {
    display: none;
}

.table-row {
    display: flex;
    padding: 8px 12px;
    border-bottom: 1px solid var(--surface-2);
    align-items: center;
    font-size: 13px;
    transition: background-color 0.2s ease;
}

.table-row:last-child {
    border-bottom: none;
}

.table-row:hover {
    background: var(--surface-2);
}

/* Карточная раскладка: та же серая подложка и метка, что и в колонках выше. */
.table-row.is-pending {
    color: var(--text-muted);
    background: color-mix(in srgb, var(--text-muted) 12%, var(--surface));
    box-shadow: inset 3px 0 0 var(--accent);
}

.table-row.is-pending:hover {
    background: color-mix(in srgb, var(--text-muted) 20%, var(--surface));
}

.table-row.has-active {
    background-color: var(--warning-bg);
    border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
}

.table-row.has-active:hover {
    background-color: var(--warning);
}

.header-col, .table-col {
    padding: 0 4px;
}

.number-col {
    width: 10%;
    text-align: center;
}

.plate-col {
    width: 25%;
}

.mark-col {
    width: 25%;
}

.actions-col {
    width: 20%;
    text-align: center;
    display: flex;
    justify-content: center;
    gap: 4px;
}

.cell-with-icon {
    display: flex;
    align-items: center;
    gap: 6px;
}

.details-btn, .edit-btn, .delete-btn {
    background: none;
    border: none;
    cursor: pointer;
    padding: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
    transition: all 0.2s ease;
}

.details-btn:hover {
    background: var(--info-bg);
}

.edit-btn:hover {
    background: var(--success-bg);
}

.delete-btn:hover {
    background: var(--danger-bg);
}

.details-icon, .edit-icon, .delete-icon {
    color: var(--text);
    width: 18px;
    height: 18px;
    opacity: 0.6;
    transition: opacity 0.2s ease, color 0.2s ease;
}

.details-btn .details-icon {
    color: var(--text);
}

.details-btn:hover .details-icon {
    opacity: 1;
    color: var(--color-primary, var(--accent-text));
}

.edit-btn:hover .edit-icon {
    opacity: 0.9;
}

.delete-btn:hover .delete-icon {
    opacity: 0.9;
}

/* Пустое состояние живёт внутри таблицы: в колоночной раскладке - строкой на всю
   ширину под шапкой колонок (для этого .vehicles-cols переносит), в карточной - в теле
   вместо карточек. */
.table-empty {
    flex: 1 0 100%;
    text-align: center;
    padding: 16px;
    color: var(--text-muted);
    font-size: 13px;
    font-style: italic;
}

h4 {
    font-size: 16px;
    color: var(--text);
    font-weight: 600;
    margin: 0;
}

/* Мобилка: строки таблицы становятся карточками (rt-* из responsive-tables.css).
   Подписи полей не выводим - решение по эпику: карточки без лейблов, как в Центре;
   номер и марка читаются сами по себе. Брейкпоинт 767.98 - как у инфраструктуры. */
@media (max-width: 767.98px) {
    /* Шапка списка перестаёт разваливаться на стопку. Замер на 320 (ряд шапки - 274px):
       одна подпись «Список транспортных средств» занимала 261px и выталкивала действия
       на свою строку, где они ломались ещё раз - выходило четыре ряда. После сжатия
       подпись 85px, а группа действий укладывается в 249px одной строкой.

       nowrap, а не wrap: перенос флекс считает по НАТУРАЛЬНОЙ ширине элемента, до
       применения flex-shrink, поэтому многоточие у заголовка перенос не отменяло -
       группа действий всё равно уезжала во вторую строку («Транспорт (0)» + бейдж +
       «Закрыть импорт» требовали 326px при 314 на 360). Теперь единственный
       сжимаемый элемент ряда - заголовок, остальные идут в натуральную ширину. */
    .header-with-badge {
        flex-wrap: nowrap;
        gap: 6px;
    }

    h4 {
        min-width: 0;
        font-size: 15px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .list-title__full {
        display: none;
    }

    .list-title__short {
        display: inline;
    }

    .vehicles-badge {
        flex: 0 0 auto;
    }

    /* 8px, а не 6: зоны нажатия мини-кнопок расширены на 4px в стороны, при меньшем
       зазоре они наложились бы друг на друга. Места хватает - очистка ушла в подвал. */
    .header-actions,
    .import-entry {
        gap: 8px;
    }

    /* Группа действий несжимаема: у .lk-button white-space: nowrap, и сжатие не
       укоротило бы подпись, а вытолкнуло её за границы кнопки. */
    .header-actions {
        flex-shrink: 0;
        flex-wrap: nowrap;
    }

    /* Бейдж режима ровно той же высоты, что и кнопка рядом (22px): «как бейдж
       Experimental» - это про совпадение, а собственная высота бейджа на телефоне
       была 16px. Специфичность (0,3,0) взята выше scoped-правила самого Badge
       (0,2,0) намеренно: при равной побеждает чанк, загруженный позже, а его
       порядок на проде не совпадает с dev (#1097 S9a). */
    .import-entry .hint-anchor :deep(.badge--sm) {
        height: 22px;
        padding: 0 8px;
        font-size: 10px;
    }

    /* Кнопка шапки блока ровно по высоте бейджа «Experimental» рядом - 22px, как в
       мокапе (.mini-btn). Прежние 44px делали из строки заголовка панель инструментов.
       Палец при этом не мимо: невидимый ::before растягивает зону нажатия до 44px
       (22 + 11 сверху и снизу), горизонтальный запас 4px меньше половины зазора 8px -
       зоны соседних кнопок не перекрываются. */
    .list-mini-btn {
        position: relative;
        height: 22px;
        min-height: 0;
        padding: 0 9px;
        font-size: 11.5px;
        font-weight: 700;
        line-height: 1;
    }

    .list-mini-btn::before {
        content: '';
        position: absolute;
        inset: -11px -4px;
    }

    .vehicles-table {
        border: none;
        border-radius: 0;
        box-shadow: none;
        /* Только по Y: по X инфраструктура держит свой overflow-x: hidden. */
        overflow-y: visible;
    }

    /* Тач-таргет 44px (WCAG 2.5.5, эталон адаптивности #1097). */
    .list-search {
        height: 44px;
        font-size: 16px;
    }

    .list-pager :deep(.lk-button) {
        min-height: 44px;
    }

    /* Список больше не скроллится внутри 180px - страница скроллит сама. */
    .table-body {
        max-height: none;
        overflow-y: visible;
        background: transparent;
    }

    .table-row.rt-row {
        position: relative;
        flex-direction: row !important;
        flex-wrap: wrap;
        align-items: center;
        gap: 2px 8px;
        min-height: 56px;
        /* Резерва справа больше нет: действия уехали в подвал карточки, и данные
           занимают всю ширину (было 136px под три иконки поперёк строки). */
        padding: 10px 12px !important;
        font-size: 14px;
    }

    /* Серую подложку строки из бланка возвращаем по той же причине, что и подсветку
       ниже: карточный фон приходит из инфраструктуры с !important. */
    .table-row.rt-row.is-pending {
        background: color-mix(in srgb, var(--text-muted) 12%, var(--surface)) !important;
    }

    /* Подсветку уже заведённой машины возвращаем: карточный фон приходит
       из инфраструктуры с !important и иначе её съедает. */
    .table-row.rt-row.has-active {
        background: var(--warning-bg) !important;
        border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface)) !important;
    }

    .table-col {
        width: auto !important;
        padding: 0;
    }

    .number-col {
        color: var(--text-muted);
        font-size: 12px;
    }

    /* Номер жирным слева, марка серым справа - одной строкой (мокап). Обе ужимаются
       многоточием, поэтому длинная марка не выталкивает номер. */
    .plate-col {
        flex: 1 1 auto;
        min-width: 0;
        font-weight: 600;
        font-size: 15px;
    }

    .plate-col .cell-with-icon {
        min-width: 0;
    }

    .plate-col .cell-value {
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .mark-col {
        flex: 0 1 auto;
        min-width: 0;
        margin-left: auto;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        color: var(--text-muted);
        font-size: 13px;
    }

    /* Вторая строка: место разгрузки слева, часы пребывания справа. */
    .meta-col {
        display: flex;
        align-items: baseline;
        gap: 8px;
        flex-basis: 100%;
        color: var(--text-muted);
        font-size: 13px;
    }

    .meta-col__place {
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .meta-col__time {
        margin-left: auto;
        flex-shrink: 0;
    }

    /* Подвал карточки: действия бейджами под данными, а не поперёк строки. */
    .actions-col {
        position: static;
        transform: none;
        flex-basis: 100%;
        width: auto !important;
        justify-content: flex-start;
        gap: 6px;
        margin-top: 8px;
        padding-top: 8px;
        border-top: 1px solid color-mix(in srgb, var(--border) 60%, var(--surface));
    }

    /* Высота 28px как у бейджа, зона нажатия 44px невидимым ::before (мокап .act):
       кнопка перестаёт претендовать на половину карточки, но мимо неё не попадёшь. */
    .details-btn,
    .edit-btn,
    .delete-btn {
        position: relative;
        width: auto;
        height: 28px;
        padding: 0 10px;
        border: 1px solid var(--border);
        border-radius: var(--radius-pill, 999px);
        background: var(--surface);
        font-size: 12.5px;
        font-weight: 600;
        line-height: 1;
        white-space: nowrap;
    }

    .details-btn::before,
    .edit-btn::before,
    .delete-btn::before {
        content: '';
        position: absolute;
        inset: -8px -2px;
    }

    .edit-btn {
        border-color: var(--accent);
        color: var(--accent-text);
    }

    .delete-btn {
        border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
        color: var(--danger-text);
    }

    /* «Детали» - вторичное действие: без рамки и прижата к правому краю подвала. */
    .details-btn {
        margin-left: auto;
        border-color: transparent;
        color: var(--text-muted);
    }

    /* Подложка кнопок из десктопной раскладки (зелёная/красная заливка на весь
       квадрат) на бейджах читается как залитая кнопка - гасим. */
    .details-btn:hover,
    .edit-btn:hover,
    .delete-btn:hover {
        background: var(--surface-2);
    }
}

/* Узкие телефоны: подпись и бейдж плотнее. Зазоры группы действий не ужимаем - они
   держат зоны нажатия мини-кнопок раздельными (см. .list-mini-btn). */
@media (max-width: 480px) {
    .header-with-badge {
        gap: 4px;
    }

    /* Подпись 14px, а не 15: тот же кегль, что у соседнего списка сотрудников, где на
       320 пяти пикселей не хватало, чтобы действия остались в строке заголовка. */
    h4 {
        font-size: 14px;
    }

    .import-entry .hint-anchor :deep(.badge--sm) {
        padding: 0 6px;
    }
}
</style>
