<template>
  <div class="data__list">
    <div class="header-with-badge">
      <h4>Список сотрудников</h4>
      <span class="employees-badge">{{ employees.length }}</span>
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
        <button
          type="button"
          class="lk-button lk-button--secondary lk-button--sm import-entry__btn"
          data-testid="employees-import-btn"
          :aria-pressed="importActive ? 'true' : 'false'"
          @click="$emit('toggle-import')"
        >
          {{ importActive ? 'Закрыть импорт' : 'Импорт' }}
        </button>
      </div>
    </div>

    <!-- Импорт бланком (blank-import) может занести до 2000 строк - показываем
         поиск и постраничную навигацию только когда список реально большой,
         чтобы обычная ручная подача из нескольких человек выглядела как раньше. -->
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
          <img 
            src="@/assets/icons/sort.png" 
            class="sort-icon" 
            :class="{ 
              'desc': sortField === 'number' && sortDirection === 'desc'
            }" 
          >
        </div>
        <div
          class="header-col lastName-col"
          @click="$emit('sort', 'lastName')"
        >
          <p :class="{ 'active-sort': sortField === 'lastName' }">
            Фамилия
          </p>
          <img 
            src="@/assets/icons/sort.png" 
            class="sort-icon" 
            :class="{ 
              'desc': sortField === 'lastName' && sortDirection === 'desc'
            }" 
          >
        </div>
        <div
          class="header-col firstName-col"
          @click="$emit('sort', 'firstName')"
        >
          <p :class="{ 'active-sort': sortField === 'firstName' }">
            Имя
          </p>
          <img 
            src="@/assets/icons/sort.png" 
            class="sort-icon" 
            :class="{ 
              'desc': sortField === 'firstName' && sortDirection === 'desc'
            }" 
          >
        </div>
        <div
          class="header-col middleName-col"
          @click="$emit('sort', 'middleName')"
        >
          <p :class="{ 'active-sort': sortField === 'middleName' }">
            Отчество
          </p>
          <img 
            src="@/assets/icons/sort.png" 
            class="sort-icon" 
            :class="{ 
              'desc': sortField === 'middleName' && sortDirection === 'desc'
            }" 
          >
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
        >
          <div class="table-col number-col">
            {{ row.number }}
          </div>
          <div class="table-col lastName-col">
            {{ row.item.lastName || 'Не указано' }}
          </div>
          <div class="table-col firstName-col">
            {{ row.item.firstName || 'Не указано' }}
          </div>
          <div class="table-col middleName-col">
            {{ row.item.middleName || 'Не указано' }}
          </div>
          <div class="table-col actions-col">
            <button
              class="details-btn"
              title="Детали"
              @click="showEmployeeDetails(row.item)"
            >
              <DetailsIcon class="details-icon" />
            </button>
            <button
              class="edit-btn"
              title="Редактировать"
              @click="$emit('edit-employee', row.item)"
            >
              <img
                src="@/assets/icons/edit.png"
                alt="Редактировать"
                class="edit-icon"
              >
            </button>
            <button
              class="delete-btn"
              @click="$emit('delete-employee', row.item.id)"
            >
              <img
                src="@/assets/icons/trashcan.png"
                alt="Удалить"
                class="delete-icon"
              >
            </button>
          </div>
        </div>
        <div
          v-if="employees.length === 0"
          class="no-employees"
        >
          Нет добавленных сотрудников
        </div>
        <div
          v-else-if="filteredEmployees.length === 0"
          class="no-employees"
        >
          Ничего не найдено по запросу «{{ searchQuery }}»
        </div>
      </div>
    </div>

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
import Badge from '@/components/ui/Badge.vue';
import DetailsIcon from '@/components/ui/DetailsIcon.vue';
import Pager from '@/components/ui/Pager.vue';
import { useListSearchPagination } from '@/composables/useListSearchPagination';

export default {
    name: 'EmployeesList',
    components: { EmployeeDetailsModal, Badge, DetailsIcon, Pager },
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
    emits: ['sort', 'edit-employee', 'delete-employee', 'toggle-import'],
    setup(props) {
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
            selectedEmployee: null
        };
    },
    methods: {
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

/* Вход в импорт прижат к правому краю шапки списка; бейдж слева от кнопки. */
.import-entry {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-left: auto;
}

@media (max-width: 768px) {
    .import-entry__btn {
        min-height: 44px;
        padding: 4px 14px;
    }
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

.list-toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
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

.no-employees {
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
        /* Резерв под три кнопки действий, приколотые справа. */
        padding: 10px 136px 10px 12px !important;
        font-size: 14px;
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

    .actions-col {
        position: absolute;
        top: 50%;
        right: 8px;
        transform: translateY(-50%);
        width: auto !important;
        gap: 2px;
    }

    .details-btn,
    .edit-btn,
    .delete-btn {
        width: 40px;
        height: 40px;
    }

    .details-icon,
    .edit-icon,
    .delete-icon {
        width: 20px;
        height: 20px;
        opacity: 0.75;
    }
}
</style>