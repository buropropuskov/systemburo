<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="visible"
        class="modal-overlay"
        :style="{ zIndex }"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div
          class="modal-content"
          :class="{ 'is-dragging': sheetDragging }"
          :style="sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : null"
          @mousedown.stop
          @click.stop
          @touchstart="onSheetTouchStart"
          @touchmove="onSheetTouchMove"
          @touchend="onSheetTouchEnd"
        >
          <div
            class="sheet-handle"
            aria-hidden="true"
          />
          <!-- Заголовок модалки -->
          <div class="modal-header">
            <h3>Выбор существующих автомобилей</h3>
            <div class="header-right">
              <SearchComponent
                v-model="searchQuery"
                title="Поиск автомобилей..."
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

          <!-- Фильтры -->
          <div class="filter-section">
            <div class="filter-tabs">
              <button
                class="filter-tab"
                :class="{ 'filter-tab--active': currentFilter === 'all' }"
                @click="switchFilter('all')"
              >
                Все машины
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
                v-if="userCompanyId"
                class="filter-tab"
                :class="{ 'filter-tab--active': currentFilter === 'company' }"
                @click="switchFilter('company')"
              >
                Компания
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
              :class="{ 'is-empty': !tempSelectedCars.length }"
            >
              Выбрано: <span class="selected-count">{{ tempSelectedCars.length }}</span>
            </div>
          </div>

          <!-- Список машин -->
          <div class="cars-table-container">
            <div class="cars-table rt-table">
              <!-- Заголовки таблицы -->
              <div class="table-header rt-head-row">
                <div class="header-cell select-cell" />
                <div class="header-cell number-cell">
                  №
                </div>
                <div class="header-cell plate-cell">
                  Номер
                </div>
                <div class="header-cell mark-cell">
                  Марка
                </div>
                <div class="header-cell status-cell">
                  Статус
                </div>
              </div>

              <!-- Тело таблицы -->
              <div
                ref="listBody"
                class="table-body"
              >
                <div
                  v-for="car in displayedCars"
                  :key="car.id"
                  class="table-row rt-row"
                  :class="{
                    'table-row--disabled': isCarDisabled(car),
                    'table-row--blacklisted': isCarBlacklisted(car),
                    'table-row--selected': isCarSelected(car)
                  }"
                  :title="carRowTitle(car)"
                  @click="handleRowClick(car)"
                >
                  <div
                    class="table-cell select-cell"
                    @click.stop
                  >
                    <input
                      type="checkbox"
                      :checked="isCarSelected(car)"
                      :disabled="isCarDisabled(car)"
                      @change="toggleCarSelection(car)"
                    >
                  </div>
                  <div class="table-cell number-cell">
                    {{ car.id }}
                  </div>
                  <div class="table-cell plate-cell">
                    {{ car.number }}
                  </div>
                  <div class="table-cell mark-cell">
                    {{ car.mark }}
                  </div>
                  <div class="table-cell status-cell">
                    <span
                      v-if="isCarBlacklisted(car)"
                      class="status-badge status-blacklisted"
                      title="В чёрном списке"
                    >
                      В ЧС
                    </span>
                    <span
                      v-else
                      class="status-badge"
                      :class="{
                        'status-active': car.status,
                        'status-inactive': !car.status
                      }"
                    >
                      {{ car.status ? 'Активна' : 'Неактивна' }}
                    </span>
                  </div>
                </div>

                <!-- Состояния загрузки/пусто -->
                <div
                  v-if="loadingCars"
                  class="loading-state"
                >
                  <LoaderSpinner label="Загрузка машин…" />
                </div>
                <div
                  v-else-if="displayedCars.length === 0"
                  class="empty-state"
                >
                  {{ searchQuery ? 'Ничего не найдено' : 'Нет доступных автомобилей' }}
                </div>
              </div>
            </div>
          </div>

          <!-- Кнопки действий -->
          <div class="modal-actions">
            <button
              class="lk-button lk-button--ghost"
              @click="$emit('close')"
            >
              Отмена
            </button>
            <button
              class="lk-button lk-button--primary"
              :disabled="tempSelectedCars.length === 0"
              @click="confirmSelection"
            >
              {{ tempSelectedCars.length > 0 ? `Выбрать (${tempSelectedCars.length})` : 'Выбрать' }}
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
import { ref } from 'vue'
import { useOverlayClose } from '@/composables/useOverlayClose'
import { useSwipeDismiss } from '@/composables/useSwipeDismiss'
import { isSameVehicle, vehicleFromCatalog } from '@/utils/applicationDuplicates'

export default {
    name: 'ExistingCarsModal',
    components: {
        SearchComponent,
        LoaderSpinner
    },
    props: {
        visible: {
            type: Boolean,
            default: false
        },
        alreadyAddedVehicles: {
            type: Array,
            default: () => []
        },
        userOrganizationId: {
            type: Number,
            default: null
        },
        userCompanyId: {
            type: Number,
            default: null
        },
        initialSelectedCars: {
            type: Array,
            default: () => []
        },
        // Слой оверлея. Дефолт 1000 - подача заявки, где поверх страницы больше ничего нет.
        // Из окна, открытого над деталью заявки (лестница 10002+), поднимать пропом, иначе
        // модалка уходит ЗА родителя - inline-стиль бьёт scoped-правило.
        zIndex: {
            type: Number,
            default: 1000
        }
    },
    emits: ['cars-selected', 'close'],
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
            filteredCars: [],
            displayedCars: [],
            tempSelectedCars: [],
            currentFilter: 'all',
            loadingCars: false,
            searchQuery: ''
        }
    },
    watch: {
        visible(newVal) {
            if (newVal) {
                setBodyScrollLock(this, true);
                this.tempSelectedCars = [...this.initialSelectedCars]
                this.currentFilter = 'all'
                this.searchQuery = ''
                // Флаг ЧС приходит в самих строках реестра (is_blacklisted) - сервер
                // считает его сам, список ЧС в браузер больше не выгружается.
                this.loadCarsByFilter('all')
            } else {
                releaseBodyScrollLock(this);
                this.tempSelectedCars = []
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

        async loadCarsByFilter(filterType) {
            this.loadingCars = true
            this.filteredCars = []
            this.displayedCars = []

            try {
                const response = await apiRequest(`/unique-cars?filter_type=${filterType}`, {
                    method: 'GET'
                })

                if (response.ok) {
                    this.filteredCars = this.sortByPlateDigits(await response.json())
                    this.applySearch()
                } else {
                    console.error('Ошибка при загрузке машин по фильтру:', filterType)
                }
            } catch (error) {
                console.error('Ошибка при загрузке машин:', error)
            } finally {
                this.loadingCars = false
            }
        },

        handleSearch() {
            this.applySearch()
        },

        /**
         * Список идёт по первой ЦИФРОВОЙ группе номера (А 123 ВС - по 123): буквенный
         * префикс - серия, искать машину глазами удобнее по числу. Номера без цифр
         * («По факту») - в конец, по алфавиту.
         */
        sortByPlateDigits(cars) {
            const firstDigits = (number) => {
                const m = String(number || '').match(/\d+/)
                return m ? parseInt(m[0], 10) : Number.POSITIVE_INFINITY
            }
            return [...cars].sort((a, b) => {
                const diff = firstDigits(a.number) - firstDigits(b.number)
                if (diff !== 0) return diff
                return String(a.number || '').localeCompare(String(b.number || ''), 'ru')
            })
        },

        applySearch() {
            if (!this.searchQuery.trim()) {
                this.displayedCars = [...this.filteredCars]
                return
            }

            const searchTerm = this.searchQuery.trim().toUpperCase().replace(/\s+/g, '')

            this.displayedCars = this.filteredCars.filter(car => {
                if (!car.number) return false

                const normalizedCarNumber = car.number.toUpperCase().replace(/\s+/g, '')

                if (normalizedCarNumber.includes(searchTerm)) {
                    return true
                }

                const numberParts = car.number.toUpperCase().split(/\s+/)
                const concatenatedParts = numberParts.join('')

                if (concatenatedParts.includes(searchTerm)) {
                    return true
                }

                const variations = [
                    car.number.toUpperCase(),
                    car.number.toUpperCase().replace(/\s+/g, ''),
                    numberParts.join(' '),
                    numberParts.slice(0, 2).join(''),
                    numberParts.slice(2).join(''),
                    numberParts[0] + numberParts[1],
                    numberParts[2] + numberParts[3],
                    numberParts[0] + numberParts[1] + numberParts[2].slice(0, 1),
                    numberParts[0] + numberParts[1] + numberParts[2].slice(0, 1) + numberParts[3].slice(0, 1)
                ]

                for (const variation of variations) {
                    if (variation.includes(searchTerm)) {
                        return true
                    }
                }

                if (car.mark && car.mark.toLowerCase().includes(this.searchQuery.toLowerCase())) {
                    return true
                }

                if (car.id && car.id.toString().includes(this.searchQuery)) {
                    return true
                }

                return false
            })
        },

        switchFilter(filter) {
            this.currentFilter = filter
            this.loadCarsByFilter(filter)
        },

        handleRowClick(car) {
            if (!this.isCarDisabled(car)) {
                this.toggleCarSelection(car)
            }
        },

        toggleCarSelection(car) {
            if (this.isCarDisabled(car)) return

            const index = this.tempSelectedCars.findIndex(selectedCar => selectedCar.id === car.id)
            if (index > -1) {
                this.tempSelectedCars.splice(index, 1)
            } else {
                this.tempSelectedCars.push(car)
            }
        },

        isCarSelected(car) {
            return this.tempSelectedCars.some(selectedCar => selectedCar.id === car.id)
        },

        isCarBlacklisted(car) {
            return car.is_blacklisted === true
        },

        carRowTitle(car) {
            if (this.isCarBlacklisted(car)) return 'Машина в чёрном списке - выбрать нельзя'
            if (this.isCarAlreadyAdded(car)) return 'Уже добавлена в список заявки'
            return ''
        },

        isCarAlreadyAdded(car) {
            const candidate = vehicleFromCatalog(car)
            return this.alreadyAddedVehicles.some(vehicle => isSameVehicle(vehicle, candidate))
        },

        isCarDisabled(car) {
            return this.isCarBlacklisted(car) || this.isCarAlreadyAdded(car)
        },

        confirmSelection() {
            this.$emit('cars-selected', [...this.tempSelectedCars])
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
    background: var(--overlay);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 20px;
    backdrop-filter: blur(0.1px);
    -webkit-backdrop-filter: blur(0.1px);
}

.modal-content {
    background: var(--surface);
    border-radius: 30px;
    width: 100%;
    max-width: 700px;
    max-height: calc(var(--app-vh, 1vh) * 85);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-shadow: 0 8px 32px var(--shadow-drop);
}

.sheet-handle {
    display: none;
    width: 40px;
    height: 4px;
    border-radius: 2px;
    background: var(--border);
    margin: 10px auto 2px;
    flex-shrink: 0;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border);
    background: var(--surface);
    flex-shrink: 0;
    gap: 20px;
}

.modal-header h3 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: var(--text);
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
    color: var(--text-muted);
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
    background: var(--surface-2);
    color: var(--text);
}

.filter-section {
    padding: 12px 20px;
    border-bottom: 1px solid var(--border);
    background: var(--surface-2);
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
    border: 1px solid var(--border);
    background: var(--surface);
    border-radius: 16px;
    cursor: pointer;
    font-size: 12px;
    font-weight: 500;
    color: var(--text-muted);
    transition: all 0.2s;
    outline: none;
    min-height: 32px;
    line-height: 1;
}

.filter-tab:hover:not(.filter-tab--active) {
    border-color: var(--accent);
    color: var(--accent-text);
}

.filter-tab--active {
    background: var(--accent);
    color: var(--accent-contrast);
    border-color: var(--accent);
    pointer-events: none;
}

/* Место под счётчик держим всегда: его появление сдвигало список под пальцем. */
.selected-counter.is-empty {
    visibility: hidden;
}

.selected-counter {
    font-size: 12px;
    color: var(--text-muted);
    display: flex;
    align-items: center;
    gap: 4px;
    white-space: nowrap;
}

.selected-count {
    font-weight: 600;
    color: var(--accent-text);
}

.cars-table-container {
    flex: 1;
    overflow: hidden;
    min-height: 240px;
    max-height: 240px;
    display: flex;
    flex-direction: column;
}

.cars-table {
    height: 100%;
    display: flex;
    flex-direction: column;
    flex: 1;
}

.table-header {
    display: flex;
    background: var(--surface-2);
    border-bottom: 1px solid var(--border);
    border-top: 1px solid var(--border);
    padding: 0 20px;
    height: 40px;
    min-height: 40px;
    font-size: 14px;
    font-weight: 500;
    color: var(--text-muted);
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
    width: 60px;
    flex-shrink: 0;
}

.plate-cell {
    flex: 2;
    min-width: 150px;
}

.mark-cell {
    flex: 2;
    min-width: 120px;
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
    border-bottom: 1px solid var(--surface-2);
    cursor: pointer;
    transition: background-color 0.2s;
    height: 40px;
    min-height: 40px;
}

.table-row:hover:not(.table-row--disabled) {
    background-color: var(--surface-2);
}

.table-row--selected {
    background-color: var(--success-bg);
}

.table-row--selected:hover {
    background-color: var(--accent-tint);
}

.table-row--disabled {
    background-color: var(--surface-2);
    opacity: 0.5;
    cursor: not-allowed;
}

.table-cell {
    padding: 0 8px;
    font-size: 14px;
    color: var(--text);
    display: flex;
    align-items: center;
}

.table-row--disabled .table-cell {
    color: var(--text-muted);
}

.table-cell input[type="checkbox"] {
    width: 16px;
    height: 16px;
    cursor: pointer;
    accent-color: var(--accent-text);
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
    background-color: var(--success-bg);
    color: var(--accent-text);
    border: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
}

.status-inactive {
    background-color: var(--danger-bg);
    color: var(--danger-text);
    border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.status-blacklisted {
    background-color: var(--danger-bg);
    color: var(--danger-text);
    border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
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
    color: var(--text-muted);
    font-size: 14px;
    text-align: center;
}

.loading-state {
    flex-direction: column;
    gap: 12px;
}

.empty-state {
    font-style: italic;
    color: var(--text-muted);
}

.modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    padding: 16px 20px;
    border-top: 1px solid var(--border);
    background: var(--surface);
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
    background: var(--surface-2);
}

.table-body::-webkit-scrollbar-thumb {
    background: var(--border);
    border-radius: 3px;
}

.table-body::-webkit-scrollbar-thumb:hover {
    background: var(--text-muted);
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

    /* Фильтры - карусель по паттерну строки типов вложений: пилюли нормального
       размера, уход под край с обеих сторон вместо жёсткой обрезки у кромки. */
    .filter-section {
        display: block;
        padding: 10px 0;
    }

    .filter-tabs {
        flex-wrap: nowrap;
        gap: 8px;
        overflow-x: auto;
        overflow-y: hidden;
        scroll-snap-type: x proximity;
        /* Иначе снап тянет первую пилюлю к кромке мимо padding - строка выглядит
           прокрученной и обрезанной слева уже при открытии. */
        scroll-padding: 0 16px;
        -webkit-overflow-scrolling: touch;
        overscroll-behavior-x: contain;
        scrollbar-width: none;
        justify-content: flex-start;
        padding: 0 16px 2px;
    }

    .filter-tabs::-webkit-scrollbar {
        display: none;
    }

    /* Все четыре пилюли встают в один ряд без прокрутки - «Мои» обрезало краем. */
    .filter-tab {
        flex: 0 0 auto;
        min-height: 36px;
        padding: 8px 14px;
        font-size: 13px;
        scroll-snap-align: start;
    }

    /* Счётчик выбранных занимает своё место всегда - иначе его появление
       сдвигало список под пальцем. */
    .selected-counter {
        min-height: 16px;
        margin: 4px 16px 0;
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

    .table-header,
    .table-row {
        padding: 0 16px;
    }

    .modal-actions {
        padding: 12px 16px;
    }
}

@media (max-width: 767.98px) {
    /* Карточки вместо таблицы: минимум 502px ширины колонок на 390 просто резал
       данные - скролла у контейнера не было. Классы rt-* из responsive-tables.css,
       подписи полей не выводим (решение по эпику - карточки без лейблов). */
    .cars-table-container {
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

    .plate-cell {
        font-weight: 600;
        font-size: 15px;
    }

    .mark-cell {
        flex-basis: 100%;
        color: var(--text-muted);
        font-size: 13px;
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
