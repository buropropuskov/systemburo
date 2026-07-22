<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="visible"
        class="modal-overlay"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div
          class="modal-content"
          @mousedown.stop
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
              aria-label="Закрыть"
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
              class="selected-counter"
              :class="{ 'is-empty': !tempSelectedEmployees.length }"
            >
              Выбрано: <span class="selected-count">{{ tempSelectedEmployees.length }}</span>
            </div>
          </div>

          <div class="employees-table-container">
            <div class="employees-table rt-table">
              <div class="table-header rt-head-row">
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

              <div
                ref="listBody"
                class="table-body"
              >
                <div
                  v-for="employee in displayedEmployees"
                  :key="employee.id"
                  class="table-row rt-row"
                  :class="{
                    'table-row--disabled': isEmployeeDisabled(employee),
                    'table-row--blacklisted': isEmployeeBlacklisted(employee),
                    'table-row--selected': isEmployeeSelected(employee)
                  }"
                  :title="employeeRowTitle(employee)"
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
                      v-if="isEmployeeBlacklisted(employee)"
                      class="status-badge status-blacklisted"
                      title="В чёрном списке"
                    >
                      В ЧС
                    </span>
                    <span
                      v-else
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
              class="lk-button lk-button--ghost"
              @click="$emit('close')"
            >
              Отмена
            </button>
            <button
              class="lk-button lk-button--primary"
              :disabled="tempSelectedEmployees.length === 0"
              @click="confirmSelection"
            >
              {{ tempSelectedEmployees.length > 0 ? `Добавить (${tempSelectedEmployees.length})` : 'Добавить' }}
            </button>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { apiRequest } from '@/api/client'
import SearchComponent from '@/components/SearchComponent.vue'
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue'
import { listPersonBlacklist } from '@/api/blacklist'
import { ref } from 'vue'
import { useOverlayClose } from '@/composables/useOverlayClose'
import { useSwipeDismiss } from '@/composables/useSwipeDismiss'

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
    setup(props, { emit }) {
        const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => emit('close'))
        // Свайп вниз закрывает лист: тянем за ползунок или за окно, когда список вверху.
        const listBody = ref(null)
        const swipe = useSwipeDismiss(() => emit('close'), {
            handleSelector: '.sheet-handle',
            getScrollTop: () => listBody.value?.scrollTop ?? 0,
        })
        return {
            onOverlayMousedown,
            onOverlayMouseup,
            listBody,
            sheetOffset: swipe.offset,
            sheetDragging: swipe.isDragging,
            onSheetTouchStart: swipe.onTouchStart,
            onSheetTouchMove: swipe.onTouchMove,
            onSheetTouchEnd: swipe.onTouchEnd,
        }
    },
    data() {
        return {
            filteredEmployees: [],
            displayedEmployees: [],
            tempSelectedEmployees: [],
            currentFilter: 'all',
            loadingEmployees: false,
            searchQuery: '',
            blacklistKeys: new Set()
        }
    },
    watch: {
        visible(newVal) {
            if (newVal) {
                setBodyScrollLock(this, true);
                this.tempSelectedEmployees = [...this.initialSelectedEmployees]
                this.currentFilter = 'all'
                this.searchQuery = ''
                // Грузим ЧС до списка, чтобы строки сразу рисовались с бейджем "В ЧС".
                this.loadBlacklist().then(() => this.loadEmployeesByFilter('all'))
            } else {
                releaseBodyScrollLock(this);
                this.tempSelectedEmployees = []
                this.searchQuery = ''
            }
        }
    },
    mounted() {
        document.addEventListener('keydown', this.handleKeydown)
    },
    beforeUnmount() {
        document.removeEventListener('keydown', this.handleKeydown)
        releaseBodyScrollLock(this);
    },
    methods: {
        handleKeydown(e) {
            if (!this.visible) return
            if (e.key === 'Escape') this.$emit('close')
        },

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

        async loadBlacklist() {
            try {
                const items = await listPersonBlacklist()
                const list = Array.isArray(items) ? items : []
                this.blacklistKeys = new Set(
                    list.map(e => this.blacklistKey(e.last_name, e.first_name, e.middle_name))
                )
            } catch (error) {
                // Не блокируем модалку при сбое ЧС - серверный гард всё равно отклонит подачу.
                console.error('Не удалось загрузить чёрный список людей:', error)
                this.blacklistKeys = new Set()
            }
        },

        // Ключ зеркалит серверный Check: LOWER(TRIM) по ФИО, пустое отчество матчит пустое.
        blacklistKey(last, first, middle) {
            const norm = (v) => (v || '').trim().toLowerCase()
            return `${norm(last)}|${norm(first)}|${norm(middle)}`
        },

        isEmployeeBlacklisted(employee) {
            return this.blacklistKeys.has(
                this.blacklistKey(employee.last_name, employee.first_name, employee.middle_name)
            )
        },

        employeeRowTitle(employee) {
            if (this.isEmployeeBlacklisted(employee)) return 'Человек в чёрном списке - выбрать нельзя'
            return ''
        },

        isEmployeeDisabled(employee) {
            if (this.isEmployeeBlacklisted(employee)) return true
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
    max-height: 85dvh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15);
}

.sheet-handle {
    display: none;
    width: 40px;
    height: 4px;
    border-radius: 2px;
    background: #d5d5d5;
    margin: 10px auto 2px;
    flex-shrink: 0;
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

/* Место под счётчик держим всегда: его появление сдвигало список под пальцем. */
.selected-counter.is-empty {
    visibility: hidden;
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

.status-blacklisted {
    background-color: #fee2e2;
    color: #991b1b;
    border: 1px solid #fecaca;
    font-weight: 700;
}

.table-row--blacklisted .status-badge.status-blacklisted {
    opacity: 1;
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

/* Анимация открытия/закрытия */
.modal-fade-enter-active {
    transition: opacity 0.2s ease-out;
}

.modal-fade-leave-active {
    transition: opacity 0.2s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
    opacity: 0;
}

.modal-fade-enter-active .modal-content {
    animation: modal-slide-up 0.2s ease-out;
}

.modal-fade-leave-active .modal-content {
    animation: modal-slide-down 0.2s ease;
}

@keyframes modal-slide-up {
    from {
        transform: translateY(20px);
        opacity: 0;
    }
    to {
        transform: translateY(0);
        opacity: 1;
    }
}

@keyframes modal-slide-down {
    from {
        transform: translateY(0);
        opacity: 1;
    }
    to {
        transform: translateY(20px);
        opacity: 0;
    }
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
    .modal-overlay {
        padding: 0;
        align-items: flex-end;
    }

    .modal-content {
        max-height: 90dvh;
        max-width: 100vw;
        border-radius: 16px 16px 0 0;
    }

    .modal-header {
        padding: 12px 16px;
    }

    /* Лист фиксированной высоты: пока список грузился, окно росло и «подпрыгивало». */
    .modal-content {
        height: 90dvh;
        /* Глобальный .modal-content из App.vue запускает свою выездку снизу, и
           вместе с локальной анимацией лист дёргался: замер кадров показывал
           ход 844 -> 96 -> 356 -> 84. Оставляем одну - локальную. */
        animation: none;
    }

    /* Своя анимация (сдвиг на 20px) спорила с выездом листа снизу - на мобилке
       оставляем только слайд. */
    .modal-fade-enter-active .modal-content {
        animation: modal-sheet-up 0.3s cubic-bezier(0.32, 0.72, 0, 1);
    }

    .modal-fade-leave-active .modal-content {
        animation: none;
        transition: transform 0.25s cubic-bezier(0.32, 0.72, 0, 1);
    }

    .modal-fade-leave-to .modal-content {
        transform: translateY(100%);
    }

    /* Поиск во всю ширину: прижатый вправо, он выглядел обрезанным. */
    .header-right {
        justify-content: stretch;
        gap: 8px;
    }

    .header-right :deep(.search-component),
    .header-right > * {
        width: 100%;
    }

    /* Фильтры - одна прокручиваемая строка вместо переноса «3 + 1 по центру». */
    .filter-section {
        display: block;
        padding: 10px 16px;
    }

    .filter-tabs {
        flex-wrap: nowrap;
        overflow-x: auto;
        scrollbar-width: none;
        justify-content: flex-start;
        padding-bottom: 2px;
    }

    .filter-tabs::-webkit-scrollbar {
        display: none;
    }

    .filter-tab {
        flex: 0 0 auto;
        min-height: 34px;
    }

    /* Счётчик выбранных занимает своё место всегда - иначе его появление
       сдвигало список под пальцем. */
    .selected-counter {
        min-height: 16px;
        margin-top: 4px;
        justify-content: flex-start;
    }

    /* Шапка в колонку ставила крестик третьей строкой слева - возвращаем его в
       правый верхний угол: заголовок и крестик в строке, поиск под ними. */
    .modal-header {
        display: grid;
        grid-template-columns: 1fr auto;
        gap: 8px 12px;
        align-items: center;
    }

    .modal-header h3 {
        grid-column: 1;
        grid-row: 1;
        font-size: 16px;
    }

    .modal-close {
        grid-column: 2;
        grid-row: 1;
        width: 36px;
        height: 36px;
    }

    .header-right {
        grid-column: 1 / -1;
        grid-row: 2;
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
}

@media (max-width: 767.98px) {
    /* Карточки вместо таблицы: минимум 760px ширины колонок на 390 просто резал
       данные - скролла у контейнера не было. Классы rt-* из responsive-tables.css,
       подписи полей не выводим (решение по эпику - карточки без лейблов). */
    .employees-table-container {
        min-height: 0;
        max-height: none;
        flex: 1;
        overflow: visible;
    }

    /* Высота списка была прибита к 200px внутри листа на 90dvh - под ним оставалась
       пустота, а данные скроллились в окошке. */
    .table-body {
        height: auto;
        min-height: 0;
        max-height: none;
        flex: 1;
        overflow-y: auto;
        padding: 8px 12px 12px;
    }

    .table-row.rt-row {
        position: relative;
        flex-direction: row !important;
        flex-wrap: wrap;
        align-items: center;
        gap: 2px 8px;
        height: auto !important;
        min-height: 56px;
        padding: 10px 100px 10px 46px !important;
        font-size: 14px;
    }

    .table-cell {
        width: auto !important;
        min-width: 0 !important;
        flex: 0 1 auto;
        padding: 0;
    }

    .select-cell {
        position: absolute;
        left: 12px;
        top: 50%;
        transform: translateY(-50%);
    }

    /* Внутренний идентификатор записи на карточке не нужен. */
    .number-cell {
        display: none;
    }

    .name-col {
        flex-basis: 100%;
        font-weight: 600;
        font-size: 15px;
    }

    /* Должность и гражданство - одной строкой под ФИО; при нехватке места
       режется гражданство, а не должность. */
    .position-col {
        flex: 0 0 auto;
        color: #666;
        font-size: 13px;
    }

    /* flex-basis 0 - иначе wrap уносит гражданство на свою строку вместо того,
       чтобы ужать его многоточием. */
    .citizenship-col {
        flex: 1 1 0;
        min-width: 0;
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
        color: #666;
        font-size: 13px;
    }

    .citizenship-col::before {
        content: '· ';
    }

    .status-cell {
        position: absolute;
        right: 12px;
        top: 50%;
        transform: translateY(-50%);
    }

    /* Чекбокс под палец: сам 24px, зона клика расширена псевдоэлементом. */
    /* 24px выглядели крупно рядом с текстом строки - зону клика держит ::before. */
    .table-cell input[type="checkbox"] {
        position: relative;
        width: 19px;
        height: 19px;
    }

    .table-cell input[type="checkbox"]::before {
        content: '';
        position: absolute;
        inset: -8px;
    }
}

@media (max-width: 768px) {
    .sheet-handle {
        display: block;
    }

    .modal-content {
        transition: transform 0.3s ease;
    }

    .modal-content.is-dragging {
        transition: none;
    }
}

@keyframes modal-sheet-up {
    from { transform: translateY(100%); }
    to { transform: translateY(0); }
}
</style>
