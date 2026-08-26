<template>
  <div class="data__list">
    <div class="header-with-badge">
      <h4>
        <span class="list-title__full">Список сотрудников</span>
        <span class="list-title__short">Сотрудники</span>
      </h4>
      <span class="employees-badge">{{ employees.length }}</span>
      <!-- Действия шапки: вход в импорт (когда доступен) и очистка списка (когда есть,
           что чистить) - обе живут в шапке справа, а не в отдельной полосе тулбара под
           ней: иначе при коротком списке (нет поиска/пейджера) тулбар оставался пустой
           строкой под кнопку и раздувал пробел между шапкой и таблицей.
           На телефоне очистка уезжает в подвал блока (см. .list-foot ниже): счётчик и
           действие в одной строке с заголовком переносились. -->
      <div
        v-if="canImport || (!isNarrow && employees.length)"
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
            data-testid="employees-import-btn"
            :aria-pressed="importActive ? 'true' : 'false'"
            :aria-label="importActive ? 'Закрыть импорт' : 'Импорт'"
            @click="$emit('toggle-import')"
          >
            {{ importActive ? (isNarrow ? 'Закрыть' : 'Закрыть импорт') : 'Импорт' }}
          </button>
        </div>
        <button
          v-if="!isNarrow && employees.length"
          type="button"
          class="lk-button lk-button--danger lk-button--sm list-toolbar__clear"
          data-testid="employees-clear-btn"
          @click="showClearConfirm = true"
        >
          Очистить
        </button>
      </div>
    </div>

    <!-- Поиск с пейджером - только когда список длинный (импорт бланком может занести
         до 2000 строк, обычной ручной подаче они не нужны). Короткий список этой
         строки не занимает вовсе. -->
    <div
      v-if="showToolbar"
      class="list-toolbar"
    >
      <input
        v-model="searchQuery"
        type="text"
        class="lk-input list-search"
        placeholder="Поиск по ФИО"
        data-testid="employees-search"
      >
      <Pager
        v-if="totalPages > 1"
        class="list-pager"
        :page="currentPage"
        :total-pages="totalPages"
        :total="filteredEmployees.length"
        page-prefix="Стр. "
        @update:page="goToPage"
      />
    </div>

    <div class="employees-table rt-table">
      <div class="table-header rt-head-row">
        <div
          class="header-col number-col"
          @click="$emit('sort', 'number')"
        >
          <p :class="{ 'active-sort': sortField === 'number' }">
            №
          </p>
          <AppIcon
            name="sort"
            class="sort-icon"
            :class="{
              'desc': sortField === 'number' && sortDirection === 'desc'
            }"
          />
        </div>
        <div
          class="header-col lastName-col"
          @click="$emit('sort', 'lastName')"
        >
          <p :class="{ 'active-sort': sortField === 'lastName' }">
            Фамилия
          </p>
          <AppIcon
            name="sort"
            class="sort-icon"
            :class="{
              'desc': sortField === 'lastName' && sortDirection === 'desc'
            }"
          />
        </div>
        <div
          class="header-col firstName-col"
          @click="$emit('sort', 'firstName')"
        >
          <p :class="{ 'active-sort': sortField === 'firstName' }">
            Имя
          </p>
          <AppIcon
            name="sort"
            class="sort-icon"
            :class="{
              'desc': sortField === 'firstName' && sortDirection === 'desc'
            }"
          />
        </div>
        <div
          class="header-col middleName-col"
          @click="$emit('sort', 'middleName')"
        >
          <p :class="{ 'active-sort': sortField === 'middleName' }">
            Отчество
          </p>
          <AppIcon
            name="sort"
            class="sort-icon"
            :class="{
              'desc': sortField === 'middleName' && sortDirection === 'desc'
            }"
          />
        </div>
        <div class="header-col actions-col">
          Действия
        </div>
      </div>
      <div class="table-body">
        <div
          v-for="row in pagedEmployees"
          :key="row.item.id"
          class="table-row rt-row"
          data-testid="employees-row"
          :class="{ 'is-pending': row.item.isPending }"
        >
          <div class="table-col number-col">
            {{ row.number }}
          </div>
          <div class="table-col lastName-col">
            <span class="cell-value">{{ row.item.lastName || 'Не указано' }}</span>
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
          <div class="table-col firstName-col">
            {{ row.item.firstName || 'Не указано' }}
          </div>
          <div class="table-col middleName-col">
            {{ row.item.middleName || 'Не указано' }}
          </div>
          <!-- Разметка одна на обе раскладки: на десктопе действия остаются иконками
               в колонке (порядок DOM не трогаем), на телефоне показываются подписи,
               иконки прячутся, ряд уходит подвалом карточки, а «Детали» переставляется
               в конец через order - см. @media ниже. -->
          <div class="table-col actions-col">
            <button
              class="details-btn"
              title="Детали"
              @click="showEmployeeDetails(row.item)"
            >
              <DetailsIcon class="details-icon" />
              <span class="act-label">Детали</span>
            </button>
            <button
              class="edit-btn"
              title="Редактировать"
              @click="$emit('edit-employee', row.item)"
            >
              <AppIcon
                name="edit"
                class="edit-icon"
              />
              <span class="act-label">Изменить</span>
            </button>
            <button
              class="delete-btn"
              title="Удалить"
              @click="$emit('delete-employee', row.item.id)"
            >
              <AppIcon
                name="trashcan"
                class="delete-icon"
              />
              <span class="act-label">Удалить</span>
            </button>
          </div>
        </div>
        <!-- Пустое состояние - строка внутри тела таблицы под шапкой колонок: причина
             пустоты разная, место одно. -->
        <div
          v-if="emptyMessage"
          class="table-empty"
          data-testid="employees-empty"
        >
          {{ emptyMessage }}
        </div>
      </div>
    </div>

    <!-- Итог блока отдельной строкой (только телефон): счётчик слева, очистка справа.
         В строке заголовка та же пара переносилась вместе с подписью списка. -->
    <div
      v-if="isNarrow && employees.length"
      class="list-foot"
    >
      <span
        class="list-foot__total"
        data-testid="employees-total"
      >Всего {{ totalLabel }}</span>
      <button
        type="button"
        class="lk-button lk-button--danger lk-button--sm list-mini-btn list-foot__clear"
        data-testid="employees-clear-btn"
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

    <!-- Модальное окно деталей сотрудника -->
    <EmployeeDetailsModal
      :show="showDetailsModal"
      :employee="selectedEmployee"
      :all-tables="allTables"
      source="employeeslist"
      :readonly="true"
      @close="closeDetailsModal"
    />
  </div>
</template>

<script>
import EmployeeDetailsModal from './EmployeeDetailsModal.vue';
import ConfirmationModal from '@/components/ConfirmationModal.vue';
import Badge from '@/components/ui/Badge.vue';
import DetailsIcon from '@/components/ui/DetailsIcon.vue';
import Pager from '@/components/ui/Pager.vue';
import { useListSearchPagination } from '@/composables/useListSearchPagination';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import { entityCountLabel } from '@/utils/entityCount';
import AppIcon from '@/components/icons/AppIcon.vue';

export default {
    name: 'EmployeesList',
    components: { EmployeeDetailsModal, ConfirmationModal, Badge, DetailsIcon, Pager, AppIcon },
    props: {
        employees: {
            type: Array,
            required: true
        },
        sortField: { type: String, default: null },
        sortDirection: { type: String, default: null },
        allTables: {
            type: Array,
            default: () => []
        },
        // Орг/компания и дата+время текущего вложения - сущность в форме их не несёт,
        // подмешиваем при открытии карточки просмотра (организация/компания/срок/время).
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
    emits: ['sort', 'edit-employee', 'delete-employee', 'toggle-import', 'clear-list'],
    // 767.98 - тот же порог, что у карточного @media: очистка списка и подписи действий
    // живут в разных местах раскладки, одним CSS этого не выразить.
    setup(props) {
        const { isNarrow } = useNarrowScreen(767.98);

        // Поиск+постраничный показ - см. useListSearchPagination (blank-import E1: до
        // 2000 строк, рендерить всё v-for'ом не годится).
        const {
            searchQuery,
            currentPage,
            showToolbar,
            filteredItems: filteredEmployees,
            totalPages,
            pagedItems: pagedEmployees,
            goToPage
        } = useListSearchPagination(
            () => props.employees,
            (employee, q) => `${employee.lastName || ''} ${employee.firstName || ''} ${employee.middleName || ''}`.toLowerCase().includes(q)
        );

        return {
            isNarrow,
            searchQuery,
            currentPage,
            showToolbar,
            filteredEmployees,
            totalPages,
            pagedEmployees,
            goToPage
        };
    },
    data() {
        return {
            showDetailsModal: false,
            selectedEmployee: null,
            showClearConfirm: false
        };
    },
    computed: {
        // Считаем по ВСЕМУ списку, а не по видимой странице: поиск и пейджер режут показ,
        // а чистится вложение целиком - иначе число в вопросе обещало бы меньше, чем уйдёт.
        clearConfirmMessage() {
            const pending = this.employees.filter(employee => employee.isPending).length;
            const fromBlank = pending > 0 ? `, из них предварительных из бланка: ${pending}` : '';
            return `Будет убрано строк: ${this.employees.length}${fromBlank}. Отменить это действие нельзя.`;
        },

        // Итог блока в подвале: «Всего 2 сотрудника» - со склонением по числу.
        totalLabel() {
            return entityCountLabel(this.employees.length, 'employees');
        },

        // Пустая таблица объясняет причину пустоты: список не заполняли вовсе или поиск
        // ничего не нашёл. Предварительные строки из бланка тоже считаются заполнением.
        emptyMessage() {
            if (this.employees.length === 0) return 'Нет добавленных сотрудников';
            if (this.filteredEmployees.length === 0) return `Ничего не найдено по запросу «${this.searchQuery}»`;
            return '';
        }
    },
    methods: {
        confirmClear() {
            this.showClearConfirm = false;
            this.$emit('clear-list');
        },

        showEmployeeDetails(employee) {
            // EmployeeForm кладёт в employeesByAttachment объекты в camelCase
            // (lastName, firstName, citizenshipName, targetTables, ...), а
            // EmployeeDetailsModal читает snake_case (last_name, ..., target_tables).
            // Трансформируем перед передачей — иначе ФИО, места прохода, паспорт
            // не отображаются в модалке details (баг из issue #116).
            const info = this.detailInfo || {};
            const passTime = info.timeFrom && info.timeTo ? `${info.timeFrom} - ${info.timeTo}` : '';
            this.selectedEmployee = {
                id: employee.id,
                last_name: employee.lastName,
                first_name: employee.firstName,
                middle_name: employee.middleName,
                position: employee.position,
                citizenshipName: employee.citizenshipName,
                passport_series_number: employee.passportSeriesNumber,
                patent_number: employee.patentNumber,
                other_permission: employee.otherPermission,
                target_tables: employee.targetTables || [],
                // entry_date_to/pass_time/organization/company хранятся на уровне
                // вложения/заявки, не на сущности формы - берём из detailInfo.
                entry_date_to: employee.entryDateTo || info.entryDateTo,
                pass_time: employee.passTime || passTime,
                organization: employee.organization || info.organization,
                company: employee.company || info.company,
                applicationId: employee.applicationId
            };
            this.showDetailsModal = true;
        },
        closeDetailsModal() {
            this.showDetailsModal = false;
            this.selectedEmployee = null;
        }
    }
};
</script>

<style scoped>
.data__list {
    padding: 12px;
    flex: 1;
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

/* Подпись действия видна только на телефоне - на десктопе кнопка остаётся иконкой. */
.act-label {
    display: none;
}

/* Подвал блока (только телефон): «Всего N сотрудника» слева, очистка справа. */
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

.employees-badge {
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

.employees-table {
    width: 100%;
    border: 1px solid var(--border);
    border-radius: 12px;
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

.header-col:hover .sort-icon,
.header-col.active-sort .sort-icon {
    opacity: 0.8;
}

.sort-icon {
    color: var(--text-muted);
    width: 10px;
    height: 10px;
    transition: all 0.2s ease;
    opacity: 0.4;
    transform: rotate(0deg);
}

.sort-icon.desc {
    transform: rotate(180deg);
    opacity: 0.8;
}

/* Высота и прокрутка как в списке машин: карточка занимает не меньше 180px и растёт
   под содержимое, переполнение уходит во внутреннюю прокрутку. */
.table-body {
    flex: 1;
    min-height: 180px;
    overflow-y: auto;
    background: var(--surface);
    border-bottom-left-radius: 20px;
    border-bottom-right-radius: 20px;
    scrollbar-width: thin;
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

/* Предварительная строка (blank-import-ux, U5): разобрана из бланка, но в заявку ещё не
   добавлена - детали, правка и удаление работают как у обычной. Кроме бейджа «В очереди»
   строка заметно серее обычной: одного приглушённого текста владельцу было мало.
   Метка слева - inset-тень, а не border: он сдвинул бы содержимое строки. */
.table-row.is-pending {
    color: var(--text-muted);
    background: color-mix(in srgb, var(--text-muted) 12%, var(--surface));
    box-shadow: inset 3px 0 0 var(--accent);
}

/* Правило серой подложки идёт после hover-правила той же специфичности, поэтому
   отклик на курсор возвращаем явно - иначе строка из бланка перестаёт реагировать. */
.table-row.is-pending:hover {
    background: color-mix(in srgb, var(--text-muted) 20%, var(--surface));
}

/* Ячейка фамилии несёт значение и бейдж: фамилия сжимается многоточием, бейдж
   остаётся целым - он и есть статус строки. Шапку не трогаем - у неё свой gap. */
.table-col.lastName-col {
    display: flex;
    align-items: center;
    gap: 6px;
}

.cell-value {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.pending-badge {
    flex: 0 0 auto;
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

.lastName-col {
    width: 22%;
}

.firstName-col {
    width: 22%;
}

.middleName-col {
    width: 22%;
}

.actions-col {
    width: 24%;
    text-align: center;
    display: flex;
    justify-content: center;
    gap: 4px;
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

/* Пустое состояние живёт внутри тела таблицы, вместо карточек/строк. */
.table-empty {
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

/* Scrollbar styling */
.table-body::-webkit-scrollbar {
    width: 4px;
}

.table-body::-webkit-scrollbar-track {
    background: var(--surface-2);
}

.table-body::-webkit-scrollbar-thumb {
    background: var(--border);
    border-radius: 2px;
}

.table-body::-webkit-scrollbar-thumb:hover {
    background: var(--text-muted);
}

/* Мобилка: строки становятся карточками (rt-* из responsive-tables.css). Подписи
   полей не выводим - решение по эпику: карточки без лейблов, как в Центре; ФИО
   читается само. Брейкпоинт 767.98 - как у инфраструктуры. */
@media (max-width: 767.98px) {
    /* Шапка списка перестаёт разваливаться на стопку. Замер на 320 (ряд шапки - 274px):
       полная подпись занимала 176px, бейдж режима 95px, и пара «Импорт»+«Очистить»
       требовала 280px - действия уезжали на свою строку, а там ломались ещё раз. После
       сжатия группа действий укладывается в 249px и остаётся одной строкой.

       nowrap, а не wrap: перенос флекс считает по НАТУРАЛЬНОЙ ширине элемента, до
       применения flex-shrink, поэтому многоточие у заголовка перенос не отменяло -
       группа действий всё равно уезжала во вторую строку («Сотрудники (0)» + бейдж +
       «Закрыть импорт» требовали 338px при 314 на 360). Теперь единственный
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

    .employees-badge {
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

    .employees-table {
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
        gap: 2px 6px;
        min-height: 56px;
        /* Резерва справа больше нет: действия уехали в подвал карточки, и ФИО занимает
           всю ширину (было 136px под три иконки поперёк строки). */
        padding: 10px 12px !important;
        font-size: 14px;
    }

    /* Серую подложку строки из бланка возвращаем: карточный фон приходит из
       инфраструктуры с !important и иначе её съедает. */
    .table-row.rt-row.is-pending {
        background: color-mix(in srgb, var(--text-muted) 12%, var(--surface)) !important;
    }

    .table-col {
        width: auto !important;
        padding: 0;
    }

    .number-col {
        color: var(--text-muted);
        font-size: 12px;
    }

    /* 14px, а не 15: при 15 типовое ФИО не влезало в строку карточки и
       переносилось на вторую. */
    .lastName-col,
    .firstName-col,
    .middleName-col {
        font-weight: 600;
        font-size: 14px;
    }

    /* Подвал карточки: действия бейджами под ФИО, а не поперёк строки. */
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
        order: 1;
        border-color: var(--accent);
        color: var(--accent-text);
    }

    .delete-btn {
        order: 2;
        border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
        color: var(--danger-text);
    }

    /* «Детали» - вторичное действие: без рамки и прижата к правому краю подвала.
       Порядок DOM оставлен десктопным (иконка деталей там идёт первой), поэтому
       переставляем через order, а не перестановкой разметки. */
    .details-btn {
        order: 3;
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

    /* Подпись вместо иконки: текст в кнопке читается без догадок. */
    .act-label {
        display: inline;
    }

    .details-icon,
    .edit-icon,
    .delete-icon {
        display: none;
    }
}

/* Узкие телефоны: подпись и бейдж плотнее. Зазоры группы действий не ужимаем - они
   держат зоны нажатия мини-кнопок раздельными (см. .list-mini-btn). */
@media (max-width: 480px) {
    .header-with-badge {
        gap: 4px;
    }

    /* Подпись 14px, а не 15: на 320 ряд с «Сотрудники» + счётчик + бейдж + «Импорт»
       требовал 279px при доступных 274 и переносил действия на вторую строку. */
    h4 {
        font-size: 14px;
    }

    .import-entry .hint-anchor :deep(.badge--sm) {
        padding: 0 6px;
    }
}
</style>