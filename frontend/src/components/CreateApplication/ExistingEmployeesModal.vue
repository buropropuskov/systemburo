<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="modal-overlay"
      @click="$emit('close')"
    >
      <div
        class="modal-content"
        @click.stop
      >
        <div class="modal-header">
          <h3>Выбор существующих сотрудников</h3>
          <div class="header-right">
            <SearchComponent
              v-model="searchQuery"
              title="Поиск сотрудников..."
              @update:model-value="handleSearch"
            />
          </div>
          <button
            class="modal-close"
            @click="$emit('close')"
          >
            ×
          </button>
        </div>

        <div class="filter-section">
          <div class="filter-tabs">
            <button
              class="filter-tab"
              :class="{ 'filter-tab--active': currentFilter === 'all' }"
              @click="switchFilter('all')"
            >
              Все сотрудники
            </button>
            <button
              v-if="userOrganizationId"
              class="filter-tab"
              :class="{ 'filter-tab--active': currentFilter === 'organization' }"
              @click="switchFilter('organization')"
            >
              Организация
            </button>
            <button
              class="filter-tab"
              :class="{ 'filter-tab--active': currentFilter === 'user' }"
              @click="switchFilter('user')"
            >
              Мои
            </button>
          </div>
          <div
            v-if="tempSelectedEmployees.length > 0"
            class="selected-counter"
          >
            Выбрано: <span class="selected-count">{{ tempSelectedEmployees.length }}</span>
          </div>
        </div>

        <div class="employees-table-container">
          <div class="employees-table">
            <div class="table-header">
              <div class="header-cell select-cell" />
              <div class="header-cell number-cell">
                №
              </div>
              <div class="header-cell name-col">
                ФИО
              </div>
              <div class="header-cell position-col">
                Должность
              </div>
              <div class="header-cell citizenship-col">
                Гражданство
              </div>
              <div class="header-cell status-cell">
                Статус
              </div>
            </div>

            <div class="table-body">
              <div
                v-for="employee in displayedEmployees"
                :key="employee.id"
                class="table-row"
                :class="{
                  'table-row--disabled': isEmployeeDisabled(employee),
                  'table-row--selected': isEmployeeSelected(employee)
                }"
                @click="handleRowClick(employee)"
              >
                <div
                  class="table-cell select-cell"
                  @click.stop
                >
                  <input
                    type="checkbox"
                    :checked="isEmployeeSelected(employee)"
                    :disabled="isEmployeeDisabled(employee)"
                    @change="toggleEmployeeSelection(employee)"
                  >
                </div>
                <div class="table-cell number-cell">
                  {{ employee.id }}
                </div>
                <div class="table-cell name-col">
                  {{ formatFullName(employee) }}
                </div>
                <div class="table-cell position-col">
                  {{ employee.position || 'Не указана' }}
                </div>
                <div class="table-cell citizenship-col">
                  {{ employee.citizenship_name || 'Не указано' }}
                </div>
                <div class="table-cell status-cell">
                  <span
                    class="status-badge"
                    :class="{
                      'status-active': employee.status,
                      'status-inactive': !employee.status
                    }"
                  >
                    {{ employee.status ? 'Активен' : 'Неактивен' }}
                  </span>
                </div>
              </div>

              <div
                v-if="loadingEmployees"
                class="loading-state"
              >
                <LoaderSpinner label="Загрузка сотрудников…" />
              </div>
              <div
                v-else-if="displayedEmployees.length === 0"
                class="empty-state"
              >
                {{ searchQuery ? 'Ничего не найдено' : 'Нет доступных сотрудников' }}
              </div>
            </div>
          </div>
        </div>

        <div class="modal-actions">
          <button
            class="btn btn-secondary"
            @click="$emit('close')"
          >
            Отмена
          </button>
          <button
            class="btn btn-primary"
            :disabled="tempSelectedEmployees.length === 0"
            @click="confirmSelection"
          >
            {{ tempSelectedEmployees.length > 0 ? `Добавить (${tempSelectedEmployees.length})` : 'Добавить' }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script>
import { apiRequest } from '@/api/client'
import SearchComponent from '@/components/SearchComponent.vue'
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue'

export default {
    name: 'ExistingEmployeesModal',
    components: {
        SearchComponent,
        LoaderSpinner
    },
    props: {
        visible: {
            type: Boolean,
            default: false
        },
        alreadyAddedEmployees: {
            type: Array,
            default: () => []
        },
        userOrganizationId: {
            type: Number,
            default: null
        },
        initialSelectedEmployees: {
            type: Array,
            default: () => []
        }
    },
    emits: ['employees-selected', 'close'],
    data() {
        return {
            filteredEmployees: [],
            displayedEmployees: [],
            tempSelectedEmployees: [],
            currentFilter: 'all',
            loadingEmployees: false,
            searchQuery: ''
        }
    },
    watch: {
        visible(newVal) {
            if (newVal) {
                this.tempSelectedEmployees = [...this.initialSelectedEmployees]
                this.currentFilter = 'all'
                this.searchQuery = ''
                this.loadEmployeesByFilter('all')
            } else {
                this.tempSelectedEmployees = []
                this.searchQuery = ''
            }
        }
    },
    methods: {
        async loadEmployeesByFilter(filterType) {
            this.loadingEmployees = true
            this.filteredEmployees = []
            this.displayedEmployees = []

            try {
                const response = await apiRequest(`/unique-employees?filter_type=${filterType}`, {
                    method: 'GET'
                })

                if (response.ok) {
                    this.filteredEmployees = await response.json()
                    this.applySearch()
                } else {
                    console.error('Ошибка при загрузке сотрудников по фильтру:', filterType)
                }
            } catch (error) {
                console.error('Ошибка при загрузке сотрудников:', error)
            } finally {
                this.loadingEmployees = false
            }
        },

        handleSearch() {
            this.applySearch()
        },

        applySearch() {
            if (!this.searchQuery.trim()) {
                this.displayedEmployees = [...this.filteredEmployees]
                return
            }

            const searchTerm = this.searchQuery.toLowerCase().trim()
            this.displayedEmployees = this.filteredEmployees.filter(employee => {
                const fullName = this.formatFullName(employee).toLowerCase()
                return fullName.includes(searchTerm) ||
                    (employee.position && employee.position.toLowerCase().includes(searchTerm)) ||
                    (employee.passport_series_number && employee.passport_series_number.toLowerCase().includes(searchTerm)) ||
                    (employee.citizenship_name && employee.citizenship_name.toLowerCase().includes(searchTerm)) ||
                    (employee.id && employee.id.toString().includes(searchTerm))
            })
        },

        formatFullName(employee) {
            const parts = []
            if (employee.last_name) parts.push(employee.last_name)
            if (employee.first_name) parts.push(employee.first_name)
            if (employee.middle_name) parts.push(employee.middle_name)
            return parts.join(' ') || 'Не указано'
        },

        switchFilter(filter) {
            this.currentFilter = filter
            this.loadEmployeesByFilter(filter)
        },

        handleRowClick(employee) {
            if (!this.isEmployeeDisabled(employee)) {
                this.toggleEmployeeSelection(employee)
            }
        },

        toggleEmployeeSelection(employee) {
            if (this.isEmployeeDisabled(employee)) return

            const index = this.tempSelectedEmployees.findIndex(sel => sel.id === employee.id)
            if (index > -1) {
                this.tempSelectedEmployees.splice(index, 1)
            } else {
                this.tempSelectedEmployees.push(employee)
            }
        },

        isEmployeeSelected(employee) {
            return this.tempSelectedEmployees.some(sel => sel.id === employee.id)
        },

        isEmployeeDisabled(employee) {
            return this.alreadyAddedEmployees.some(emp =>
                (emp.isExisting && emp.existingEmployeeId === employee.id) ||
                (!emp.isExisting && emp.passportSeriesNumber === employee.passport_series_number)
            )
        },

        confirmSelection() {
            this.$emit('employees-selected', [...this.tempSelectedEmployees])
        }
    }
}
</script>

<style scoped>
.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 20px;
}

.modal-content {
    background: white;
    border-radius: 30px;
    width: 100%;
    max-width: 900px;
    max-height: 85vh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15);
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px;
    border-bottom: 1px solid #e6e6e6;
    background: white;
    flex-shrink: 0;
    gap: 20px;
}

.modal-header h3 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: #333;
    flex-shrink: 0;
}

.header-right {
    display: flex;
    align-items: center;
    gap: 20px;
    flex: 1;
    justify-content: flex-end;
}

.modal-close {
    background: none;
    border: none;
    font-size: 24px;
    cursor: pointer;
    color: #a2a2a2;
    padding: 0;
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    transition: all 0.2s;
    flex-shrink: 0;
}

.modal-close:hover {
    background: #f5f5f5;
    color: #333;
}

.filter-section {
    padding: 12px 20px;
    border-bottom: 1px solid #f0f0f0;
    background: #fafafa;
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-shrink: 0;
    gap: 16px;
}

.filter-tabs {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
}

.filter-tab {
    padding: 6px 12px;
    border: 1px solid #e6e6e6;
    background: white;
    border-radius: 16px;
    cursor: pointer;
    font-size: 12px;
    font-weight: 500;
    color: #666;
    transition: all 0.2s;
    outline: none;
    min-height: 32px;
    line-height: 1;
}

.filter-tab:hover:not(.filter-tab--active) {
    border-color: #4F5BDF;
    color: #4F5BDF;
}

.filter-tab--active {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
    pointer-events: none;
}

.selected-counter {
    font-size: 12px;
    color: #666;
    display: flex;
    align-items: center;
    gap: 4px;
    white-space: nowrap;
}

.selected-count {
    font-weight: 600;
    color: #4F5BDF;
}

.employees-table-container {
    flex: 1;
    overflow: hidden;
    min-height: 240px;
    max-height: 240px;
    display: flex;
    flex-direction: column;
}

.employees-table {
    height: 100%;
    display: flex;
    flex-direction: column;
    flex: 1;
}

.table-header {
    display: flex;
    background: #f8f8f8;
    border-bottom: 1px solid #e6e6e6;
    padding: 0 20px;
    height: 40px;
    min-height: 40px;
    font-size: 14px;
    font-weight: 500;
    color: #a2a2a2;
    flex-shrink: 0;
    align-items: center;
}

.header-cell {
    padding: 0 8px;
    display: flex;
    align-items: center;
}

.select-cell {
    width: 40px;
    flex-shrink: 0;
    justify-content: center;
}

.number-cell {
    width: 30px;
    flex-shrink: 0;
}

.name-col {
    flex: 2;
    min-width: 250px;
}

.position-col {
    flex: 2;
    min-width: 140px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.citizenship-col {
    flex: 1;
    min-width: 200px;
}

.status-cell {
    width: 100px;
    flex-shrink: 0;
    justify-content: center;
}

.table-body {
    flex: 1;
    overflow-y: auto;
    max-height: 200px;
    min-height: 200px;
    height: 200px;
}

.table-row {
    display: flex;
    align-items: center;
    padding: 0 20px;
    border-bottom: 1px solid #f5f5f5;
    cursor: pointer;
    transition: background-color 0.2s;
    height: 40px;
    min-height: 40px;
}

.table-row:hover:not(.table-row--disabled) {
    background-color: #fafafa;
}

.table-row--selected {
    background-color: #f0f9ff;
}

.table-row--selected:hover {
    background-color: #e0f2fe;
}

.table-row--disabled {
    background-color: #f9f9f9;
    opacity: 0.5;
    cursor: not-allowed;
}

.table-cell {
    padding: 0 8px;
    font-size: 14px;
    color: #000;
    display: flex;
    align-items: center;
}

.table-row--disabled .table-cell {
    color: #999;
}

.table-cell input[type="checkbox"] {
    width: 16px;
    height: 16px;
    cursor: pointer;
    accent-color: #4F5BDF;
    margin: 0;
}

.table-row--disabled input[type="checkbox"] {
    cursor: not-allowed;
    opacity: 0.6;
}

.status-badge {
    padding: 4px 10px;
    border-radius: 12px;
    font-size: 11px;
    font-weight: 500;
    display: inline-block;
    min-width: 70px;
    text-align: center;
}

.status-active {
    background-color: #f0f9ff;
    color: #0369a1;
    border: 1px solid #bae6fd;
}

.status-inactive {
    background-color: #fef2f2;
    color: #991b1b;
    border: 1px solid #fecaca;
}

.table-row--disabled .status-badge {
    opacity: 0.7;
}

.loading-state,
.empty-state {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    min-height: 200px;
    color: #999;
    font-size: 14px;
    text-align: center;
}

.loading-state {
    flex-direction: column;
    gap: 12px;
}

.spinner {
    width: 24px;
    height: 24px;
    border: 2px solid #f3f3f3;
    border-top: 2px solid #4F5BDF;
    border-radius: 50%;
    animation: spin 1s linear infinite;
}

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}

.empty-state {
    font-style: italic;
    color: #a2a2a2;
}

.modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    padding: 16px 20px;
    border-top: 1px solid #e6e6e6;
    background: white;
    flex-shrink: 0;
}

.btn {
    padding: 8px 20px;
    border-radius: 8px;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s;
    border: none;
    outline: none;
    min-height: 36px;
    min-width: 100px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.btn-secondary {
    background: white;
    color: #333;
    border: 1px solid #e6e6e6;
}

.btn-secondary:hover:not(:disabled) {
    background: #f5f5f5;
    border-color: #d9d9d9;
}

.btn-primary {
    background: #4F5BDF;
    color: white;
}

.btn-primary:hover:not(:disabled) {
    background: #3a45c0;
}

.btn-primary:disabled {
    background: #a2a2a2;
    cursor: not-allowed;
    opacity: 0.6;
}

.table-body::-webkit-scrollbar {
    width: 6px;
}

.table-body::-webkit-scrollbar-track {
    background: #f1f1f1;
}

.table-body::-webkit-scrollbar-thumb {
    background: #c1c1c1;
    border-radius: 3px;
}

.table-body::-webkit-scrollbar-thumb:hover {
    background: #a8a8a8;
}

@media (max-width: 768px) {
    .modal-content {
        max-height: 90vh;
        max-width: 95vw;
    }

    .modal-header {
        padding: 12px 16px;
        flex-direction: column;
        gap: 12px;
        align-items: stretch;
    }

    .header-right {
        justify-content: center;
    }

    .filter-section {
        padding: 12px 16px;
        flex-direction: column;
        gap: 12px;
        align-items: stretch;
    }

    .filter-tabs {
        justify-content: center;
    }

    .table-header,
    .table-row {
        padding: 0 16px;
    }

    .modal-actions {
        padding: 12px 16px;
    }

    .btn {
        min-width: 80px;
        padding: 8px 16px;
    }
}
</style>
