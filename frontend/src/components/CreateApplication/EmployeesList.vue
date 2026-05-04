<template>
  <div class="data__list">
    <div class="header-with-badge">
      <h4>Список сотрудников</h4>
      <span class="employees-badge">{{ employees.length }}</span>
    </div>
    <div class="employees-table">
      <div class="table-header">
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
          v-for="(employee, index) in employees" 
          :key="employee.id"
          class="table-row"
        >
          <div class="table-col number-col">
            {{ index + 1 }}
          </div>
          <div class="table-col lastName-col">
            {{ employee.lastName || 'Не указано' }}
          </div>
          <div class="table-col firstName-col">
            {{ employee.firstName || 'Не указано' }}
          </div>
          <div class="table-col middleName-col">
            {{ employee.middleName || 'Не указано' }}
          </div>
          <div class="table-col actions-col">
            <button
              class="details-btn"
              title="Детали"
              @click="showEmployeeDetails(employee)"
            >
              <DetailsIcon class="details-icon" />
            </button>
            <button 
              class="edit-btn"
              title="Редактировать"
              @click="$emit('edit-employee', employee)"
            >
              <img 
                src="@/assets/icons/edit.png" 
                alt="Редактировать" 
                class="edit-icon"
              >
            </button>
            <button 
              class="delete-btn"
              @click="$emit('delete-employee', employee.id)"
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
      </div>
    </div>

    <!-- Модальное окно деталей сотрудника -->
    <EmployeeDetailsModal
      :show="showDetailsModal"
      :employee="selectedEmployee"
      :all-tables="allTables"
      source="employeeslist"
      @close="closeDetailsModal"
    />
  </div>
</template>

<script>
import EmployeeDetailsModal from './EmployeeDetailsModal.vue';
import DetailsIcon from '@/components/ui/DetailsIcon.vue';

export default {
    name: 'EmployeesList',
    components: { EmployeeDetailsModal, DetailsIcon },
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
        }
    },
    emits: ['sort', 'edit-employee', 'delete-employee'],
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
                entry_date_to: employee.entryDateTo,
                pass_time: employee.passTime,
                organization: employee.organization,
                company: employee.company,
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
    gap: 8px;
    padding-bottom: 12px;
}

.employees-badge {
    background: #1976d2;
    color: white;
    padding: 2px 6px;
    border-radius: 10px;
    font-size: 11px;
    font-weight: 600;
    min-width: 18px;
    text-align: center;
    line-height: 1.2;
}

.employees-table {
    width: 100%;
    border: 1px solid #e0e0e0;
    border-radius: 12px;
    overflow: hidden;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.table-header {
    display: flex;
    background: #f5f5f5;
    border-bottom: 1px solid #e0e0e0;
    padding: 10px 12px;
    font-weight: 500;
    color: #666;
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
    color: #333;
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
    background: #fff;
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.table-body::-webkit-scrollbar {
    display: none;
}

.table-row {
    display: flex;
    padding: 8px 12px;
    border-bottom: 1px solid #f5f5f5;
    align-items: center;
    font-size: 13px;
    transition: background-color 0.2s ease;
}

.table-row:last-child {
    border-bottom: none;
}

.table-row:hover {
    background: #f8f9fa;
}

.table-row.has-active {
    background-color: #fff3cd;
}

.table-row.has-active:hover {
    background-color: #ffe69b;
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
    background: #e3f2fd;
}

.edit-btn:hover {
    background: #e8f5e8;
}

.delete-btn:hover {
    background: #ffebee;
}

.details-icon, .edit-icon, .delete-icon {
    width: 14px;
    height: 14px;
    opacity: 0.6;
    transition: opacity 0.2s ease, color 0.2s ease;
}

.details-btn .details-icon {
    color: #1976d2;
}

.details-btn:hover .details-icon {
    opacity: 1;
    color: var(--color-primary, #4F5BDF);
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
    color: #666;
    font-size: 13px;
    font-style: italic;
}

h4 {
    font-size: 16px;
    color: #333;
    font-weight: 600;
    margin: 0;
}

/* Scrollbar styling */
.table-body::-webkit-scrollbar {
    width: 4px;
}

.table-body::-webkit-scrollbar-track {
    background: #f1f1f1;
}

.table-body::-webkit-scrollbar-thumb {
    background: #c1c1c1;
    border-radius: 2px;
}

.table-body::-webkit-scrollbar-thumb:hover {
    background: #a8a8a8;
}

@media (max-width: 768px) {
    .table-row {
        flex-wrap: wrap;
    }
    
    .table-col {
        width: 50% !important;
        margin-bottom: 4px;
    }
    
    .actions-col {
        width: 100%;
        justify-content: flex-end;
    }
}
</style>