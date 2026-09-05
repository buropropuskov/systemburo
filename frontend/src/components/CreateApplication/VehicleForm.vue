<template>
  <div
    class="data__completion"
    :class="{ 'data__completion--locked': disabled }"
    :inert="disabled ? '' : undefined"
  >
    <div class="completion__header">
      <h3>Добавление Т/С</h3>
      <button
        v-if="allowExistingSearch"
        class="completion__button"
        data-testid="ob-form-existing"
        @click="openExistingCarsModal"
      >
        {{ isNarrow ? 'Добавить сущ.' : 'Добавить существующую(-ие)' }}
      </button>
    </div>

    <!-- Отображение количества выбранных существующих машин -->
    <div
      v-if="selectedExistingCars.length > 0"
      class="existing-cars-info"
    >
      <div class="existing-cars-header">
        <span class="existing-cars-count">Машин добавлено: {{ selectedExistingCars.length }}</span>
        <div class="existing-cars-actions">
          <button
            class="view-cars-btn"
            @click="openExistingCarsModal"
          >
            Просмотреть
          </button>
          <div class="add-existing-wrap">
            <button
              class="add-existing-btn"
              :disabled="!canAddExistingCars"
              @click="addExistingCars"
              @mouseenter="showExistingTooltip = true"
              @mouseleave="showExistingTooltip = false"
            >
              Добавить
            </button>
            <div
              v-if="showExistingTooltip && !canAddExistingCars"
              class="tooltip"
            >
              <div class="tooltip-content">
                {{ existingCarsTooltip }}
              </div>
            </div>
          </div>
        </div>
      </div>
      <!-- Список добавленных машин: номер - марка -->
      <div class="existing-cars-list">
        <div
          v-for="car in displayedExistingCars"
          :key="car.id || car.number"
          class="existing-car-item"
        >
          <span>{{ car.number }} - {{ car.mark || 'не указана' }}</span>
          <button
            class="existing-car-remove"
            type="button"
            title="Убрать машину"
            @click="removeExistingCar(car)"
          >
            ×
          </button>
        </div>
        <button
          v-if="selectedExistingCars.length > 5 && !showAllExistingCars"
          class="show-all-btn"
          @click="showAllExistingCars = true"
        >
          Показать все ({{ selectedExistingCars.length }})
        </button>
      </div>
    </div>

    <!-- Форма для добавления новой машины -->
    <div
      v-else
      class="completion__body"
    >
      <div class="completion__format">
        <div class="format__header">
          <div
            class="format-actions"
            @click="revealBlockedHint($event)"
          >
            <button
              v-if="editingVehicle"
              class="cancel-edit-btn"
              @click="cancelEdit"
            >
              Отменить
            </button>
            <button 
              class="add-button" 
              :disabled="!canAddVehicle"
              @click="addVehicle"
              @mouseenter="showTooltip = true"
              @mouseleave="showTooltip = false"
            >
              {{ editingVehicle ? 'Применить' : 'Добавить' }}
            </button>
            <!-- Подсказка для кнопки -->
            <div
              v-if="showTooltip && !canAddVehicle"
              class="tooltip"
            >
              <div class="tooltip-content">
                {{ getTooltipMessage }}
              </div>
            </div>
          </div>
          <!-- В DOM лейбл после кнопок: на мобилке шапка разворачивается в поток,
               и лейбл встаёт прямо над своим дропдауном, а не над липкой строкой.
               На десктопе order возвращает его влево. -->
          <label class="format__label">Формат номеров</label>
        </div>
        <div class="format__dropdown">
          <button 
            class="dropdown__button" 
            :disabled="editingVehicle && editingVehicle.isExisting"
            @click="toggleFormatDropdown"
          >
            <div class="button__content">
              <span class="button__text">{{ selectedFormatText }}</span>
              <AppIcon
                name="arrow"
                class="button__arrow"
                :class="{ 'button__arrow--open': isFormatDropdownOpen }"
              />
            </div>
          </button>
          <transition name="dropdown">
            <div
              v-if="isFormatDropdownOpen"
              class="dropdown__menu"
            >
              <div 
                v-for="format in availableFormats" 
                :key="format.format.id"
                class="dropdown__item" 
                @click="selectFormat(format)"
              >
                <span class="item__text">{{ format.format.name }}</span>
              </div>
            </div>
          </transition>
        </div>
      </div>
      <div class="completion__fields">
        <div
          v-if="fieldVisible('number')"
          class="completion__number"
        >
          <div class="completion__number-header">
            <label class="input__label">Номер Т/C <span
              v-if="fieldRequired('number')"
              class="required"
            >*</span></label>
            <label class="number-fact fact-toggle">
              <input
                v-model="isNumberByFact"
                class="fact-checkbox"
                type="checkbox"
                :disabled="editingVehicle && editingVehicle.isExisting"
                @change="handleNumberByFactChange"
              >
              <span
                class="fact-switch"
                aria-hidden="true"
              />
              <span class="fact-text">по факту</span>
            </label>
          </div>
          <!-- Поле "по факту" -->
          <div
            v-if="isNumberByFact"
            class="number__field number__field--fact"
          >
            <input
              class="number__input number__input--fact"
              value="По факту"
              readonly
            >
          </div>

          <!-- Динамический формат из базы данных -->
          <div
            v-else-if="selectedFormat"
            class="number__field"
          >
            <input
              v-for="(cell, index) in selectedFormat.cells"
              :key="index"
              ref="numberCells"
              v-model="numberParts[index]"
              class="number__input"
              :placeholder="getPlaceholder(cell)"
              :maxlength="cell.max_length"
              :style="{ width: getInputWidth(cell) }"
              :disabled="editingVehicle && editingVehicle.isExisting"
              @input="validatePart(index, $event, cell)"
              @blur="formatPart(index, cell)"
            >
          </div>
          <div
            v-else
            class="no-format-message"
          >
            Выберите формат номера
          </div>

          <!-- Блок предупреждения об активной заявке для номера -->
        </div>

        <div
          v-if="fieldVisible('mark')"
          class="completion__mark"
        >
          <div class="completion__mark-header">
            <label class="input__label">Марка Т/С <span
              v-if="fieldRequired('mark')"
              class="required"
            >*</span></label>
            <label class="mark-fact fact-toggle">
              <input
                v-model="isMarkByFact"
                class="fact-checkbox"
                type="checkbox"
                @change="handleMarkByFactChange"
              >
              <span
                class="fact-switch"
                aria-hidden="true"
              />
              <span class="fact-text">по факту</span>
            </label>
          </div>
          <div
            v-if="isMarkByFact"
            class="mark__field mark__field--fact"
          >
            <input
              class="mark__input mark__input--fact"
              value="По факту"
              readonly
            >
          </div>
          <div
            v-else
            class="mark__field"
          >
            <div class="mark__dropdown">
              <button
                class="mark__dropdown-button"
                @click="toggleMarkDropdown"
              >
                <div class="mark__button-content">
                  <span
                    class="mark__button-text"
                    :title="selectedMark || ''"
                  >{{ selectedMark || 'Выберите марку' }}</span>
                  <svg
                    class="mark__button-arrow"
                    :class="{ 'mark__button-arrow--open': isMarkDropdownOpen }"
                    viewBox="0 0 10 6"
                    fill="none"
                    xmlns="http://www.w3.org/2000/svg"
                  >
                    <path
                      d="M1 1L5 5L9 1"
                      stroke="currentColor"
                      stroke-width="1.5"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    />
                  </svg>
                </div>
              </button>
              <transition name="dropdown">
                <div
                  v-if="isMarkDropdownOpen"
                  class="mark__dropdown-menu"
                >
                  <div class="mark__search">
                    <input
                      v-model="markSearch"
                      class="mark__search-input"
                      placeholder="Поиск марки..."
                      @input="filterMarks"
                    >
                  </div>
                  <div class="mark__dropdown-list">
                    <div
                      v-for="mark in filteredMarks"
                      :key="mark.id"
                      class="mark__dropdown-item"
                      @click="selectMark(mark)"
                    >
                      <span class="mark__item-text">{{ mark.name }}</span>
                    </div>
                    <div
                      v-if="!filteredMarks.length"
                      class="mark__dropdown-empty"
                    >
                      Марки не найдены
                    </div>
                  </div>
                </div>
              </transition>
            </div>
          </div>
        </div>
      </div>
      <div
        v-if="activeCarInfo && !isNumberByFact"
        class="active-warning"
      >
        <div class="warning-text">
          <p class="warning-title">
            На это авто уже есть активная заявка!
          </p>
          <p class="warning-details">
            Действует до: {{ formatDate(activeCarInfo.entry_date_to) }} {{ formatTime(activeCarInfo.entry_time_to) }}<br>
            Заявка {{ activeCarInfo.application_number }}<br>
            Организация: {{ activeCarInfo.organization_name || 'Не указана' }}<br>
            Компания: {{ activeCarInfo.company_name || 'Не указана' }}
          </p>
        </div>
      </div>
      <div
        v-if="blacklistInfo"
        class="blacklist-warning"
        data-testid="vehicle-blacklist-warning"
      >
        <div class="warning-text">
          <p class="warning-title">
            Машина в чёрном списке
          </p>
          <p class="warning-details">
            Причина: {{ blacklistInfo.reason || 'не указана' }}<br>
            Добавить эту машину в заявку нельзя.
          </p>
        </div>
      </div>
    </div>

    <!-- Места разгрузки -->
    <div
      v-if="fieldVisible('unloading_places')"
      class="completion__unloading"
      data-testid="ob-form-places"
    >
      <label class="input__label">Места разгрузки (выбор) <span
        v-if="fieldRequired('unloading_places')"
        class="required"
      >*</span></label>
      <div
        v-if="!loadingUnloadingPlaces && allUnloadingPlaces.length > 0"
        class="unloading__grid"
      >
        <div 
          v-for="place in allUnloadingPlaces" 
          :key="place.id"
          class="unloading__item"
          :class="{
            'unloading__item--active': selectedUnloadingPlaces.includes(place.id) && place.status === 'active',
            'unloading__item--inactive': place.status !== 'active'
          }"
          @click="toggleUnloadingPlace(place, $event)"
          @mouseenter="showInactiveTooltip(place, $event)"
          @mouseleave="hideInactiveTooltip"
        >
          {{ place.name }}
        </div>
      </div>
      <div
        v-else-if="loadingUnloadingPlaces"
        class="loading-message"
      >
        Загрузка мест разгрузки...
      </div>
      <div
        v-else
        class="no-places-message"
      >
        Нет доступных мест разгрузки
      </div>
      <div
        v-if="errors.unloadingPlaces"
        class="error-message"
      >
        {{ errors.unloadingPlaces }}
      </div>
    </div>

    <!-- Проезд -->
    <div
      v-if="fieldVisible('passage_tables')"
      class="completion__passage"
    >
      <label class="input__label">Проезд <span
        v-if="fieldRequired('passage_tables')"
        class="required"
      >*</span></label>
      <TargetTablesGrid
        v-if="!loadingPassageTables && filteredPassageTables.length > 0"
        v-model="selectedPassageTables"
        :tables="filteredPassageTables"
        :attached-ids="attachedTablesIds"
      />
      <div
        v-else-if="loadingPassageTables"
        class="loading-message"
      >
        Загрузка мест проезда...
      </div>
      <div
        v-else
        class="no-tables-message"
      >
        Нет доступных мест проезда
      </div>
      <div
        v-if="errors.passageTables"
        class="error-message"
      >
        {{ errors.passageTables }}
      </div>
    </div>

    <!-- Предупреждения выбранных мест (#1183): режим работы против срока + текст/окна -
         рендерятся единой плавающей панелью в CreateApplication (@notices-change). -->

    <!-- Tooltip для неактивных мест -->
    <div
      v-if="inactiveTooltip.visible"
      class="inactive-tooltip"
      :style="{ top: inactiveTooltip.y + 'px', left: inactiveTooltip.x + 'px' }"
    >
      <div class="inactive-tooltip-content">
        {{ inactiveTooltip.text }}
      </div>
    </div>

    <!-- Модальное окно выбора существующих машин -->
    <ExistingCarsModal
      :visible="showExistingCarsModal"
      :already-added-vehicles="existingVehicles"
      :user-organization-id="userOrganizationId"
      :user-company-id="userCompanyId"
      :initial-selected-cars="selectedExistingCars"
      :z-index="existingModalZIndex"
      @cars-selected="onExistingCarsSelected"
      @close="closeExistingCarsModal"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { getViewportZoom } from '@/utils/viewportScale'
import { checkVehicleBlacklist } from '@/api/blacklist'
import { useAuthStore } from '@/stores/auth'
import { useDeletionsStore } from '@/stores/deletions'
import { useFormValidation } from '@/composables/useFormValidation'
import { useNarrowScreen } from '@/composables/useNarrowScreen'
import { validatePartValue, formatPartValue, initializeNumberParts, matchNumberToFormat } from '@/composables/useNumberFormat'
import { useFieldConfig } from '@/composables/useFieldConfig'
import { collectActiveWarnings } from '@/utils/warningWindows'
import { buildScheduleReport } from '@/utils/scheduleCheck'
import { findDuplicateVehicle, vehicleLabel } from '@/utils/applicationDuplicates'
import { getCurrentInstance } from 'vue'
import ExistingCarsModal from '@/components/CreateApplication/ExistingCarsModal.vue'
import { resetVehicleFormState } from './entryFormReset'
import TargetTablesGrid from '@/components/CreateApplication/TargetTablesGrid.vue'
import AppIcon from '@/components/icons/AppIcon.vue'
import { BY_FACT_ALREADY_ADDED, hasByFactVehicle } from '@/utils/byFactVehicle';

export default {
    name: 'VehicleForm',
    components: {
        AppIcon,
        ExistingCarsModal,
        TargetTablesGrid
    },
    props: {
        userOrganization: {
            type: String,
            default: ''
        },
        userOrganizationId: {
            type: Number,
            default: null
        },
        userCompany: {
            type: String,
            default: ''
        },
        userCompanyId: {
            type: Number,
            default: null
        },
        existingVehicles: {
            type: Array,
            default: () => []
        },
        // Места разгрузки на уровне заявки (#706): приоритетный источник начального
        // выбора. Пусто - падаем на автоподстановку по организации/компании.
        applicationUnloadPlaces: {
            type: Array,
            default: () => []
        },
        // Настройка полей шаблона (#529): { [fieldKey]: { visible, required, locked, requirable } }.
        // Раздаётся из CreateApplication; потребление (скрытие/обязательность полей машин) - срез H-7.
        fieldConfig: {
            type: Object,
            default: () => ({})
        },
        // Гейт п.36: форма недоступна, пока не заполнены обязательные поля вложения.
        disabled: {
            type: Boolean,
            default: false
        },
        // Ручное добавление (#1049): DTO ManualVehicle не имеет existing_car_id, поэтому
        // выбор "существующей" машины создал бы дубликат - прячем поиск в этом контексте.
        allowExistingSearch: {
            type: Boolean,
            default: true
        },
        // Слой окна «Добавить существующую(-ие)». Дефолт 1000 - подача заявки; форма,
        // встроенная в окно поверх детали заявки, поднимает его (#1685).
        existingModalZIndex: {
            type: Number,
            default: 1000
        },
        // Срок заявки текущего вложения (#1183 S5): { date_from, date_to, time_from,
        // time_to } в API-формате (YYYY-MM-DD + ЧЧ:ММ). Против него сверяется расписание
        // (time_slots) выбранных мест - предупреждаем, если место закрыто на границе срока.
        entryPeriod: {
            type: Object,
            default: null
        }
    },
    emits: ['edit-cancelled', 'vehicle-added', 'vehicle-updated', 'vehicles-added', 'update:unload-places', 'notices-change'],
    setup(props) {
        const instance = getCurrentInstance()
        // Геттер сохраняет реактивность пропса fieldConfig (#529).
        const { fieldVisible, fieldRequired } = useFieldConfig(() => props.fieldConfig)

        const { isValid, tooltipMessage, showTooltip } = useFormValidation(() => {
            const vm = instance.proxy
            const hasInactiveSelected = vm.selectedUnloadingPlaces.some(placeId => {
                const place = vm.allUnloadingPlaces.find(p => p.id === placeId)
                return place && place.status !== 'active'
            })

            if (vm.selectedExistingCars.length > 0) {
                const existingRules = [
                    { check: !fieldVisible('unloading_places') || vm.selectedUnloadingPlaces.length > 0, message: 'хотя бы одно место разгрузки' }
                ]
                if (fieldVisible('passage_tables') && fieldRequired('passage_tables')) {
                    existingRules.push({ check: vm.selectedPassageTables.length > 0, message: 'выберите хотя бы одно место проезда' })
                }
                return existingRules
            }

            return [
                { check: !vm.isNumberByFact || !hasByFactVehicle(vm.existingVehicles, vm.editingVehicle), message: BY_FACT_ALREADY_ADDED },
                { check: !vm.activeCarInfo || vm.isNumberByFact || !fieldVisible('number'), message: 'На этот автомобиль уже есть активная заявка' },
                { check: !vm.blacklistInfo || !fieldVisible('number'), message: 'Машина в чёрном списке' },
                { check: (vm.isNumberByFact && vm.isMarkByFact) || !hasInactiveSelected || !fieldVisible('unloading_places'), message: 'Невозможно выбрать неактивные места разгрузки' },
                { check: !fieldVisible('number') || !fieldRequired('number') || vm.isNumberByFact || !!vm.selectedFormat, message: 'формат номера' },
                {
                    check: !fieldVisible('number') || !fieldRequired('number') || vm.isNumberByFact || (
                        !!vm.selectedFormat &&
                        vm.numberParts.length > 0 &&
                        vm.numberParts.every((part, i) => {
                            const cell = vm.selectedFormat.cells[i]
                            return part && part.length >= cell.min_length && part.length <= cell.max_length
                        })
                    ),
                    message: 'номер Т/С'
                },
                { check: !fieldVisible('mark') || !fieldRequired('mark') || vm.isMarkByFact || !!vm.selectedMark, message: 'марка Т/С' },
                { check: !fieldVisible('unloading_places') || !fieldRequired('unloading_places') || vm.selectedUnloadingPlaces.length > 0, message: 'хотя бы одно место разгрузки' },
                { check: !fieldVisible('passage_tables') || !fieldRequired('passage_tables') || vm.selectedPassageTables.length > 0, message: 'выберите хотя бы одно место проезда' }
            ]
        })

        // Причина блокировки кнопки живёт на hover - на телефоне его нет,
        // поэтому там показываем её сразу под кнопкой.
        const { isNarrow } = useNarrowScreen()
        return { canAddVehicle: isValid, getTooltipMessage: tooltipMessage, showTooltip, fieldVisible, fieldRequired, isNarrow }
    },
    data() {
        return {
            numberParts: [],
            isNumberByFact: false,
            availableFormats: [],
            // Промис загрузки справочника форматов - см. mounted и applyEditedVehicleNumber.
            formatsReady: null,
            selectedFormat: null,
            isFormatDropdownOpen: false,
            isMarkByFact: false,
            selectedMark: '',
            selectedMarkId: null,
            isMarkDropdownOpen: false,
            markSearch: '',
            marks: [],
            filteredMarks: [],
            allUnloadingPlaces: [],
            attachedUnloadingPlaces: [],
            selectedUnloadingPlaces: [],
            loadingUnloadingPlaces: false,
            allPassageTables: [],
            attachedPassageTables: [],
            selectedPassageTables: [],
            loadingPassageTables: false,
            errors: { unloadingPlaces: '', passageTables: '' },
            allowedCyrillicLetters: ['А', 'В', 'Е', 'К', 'М', 'Н', 'О', 'Р', 'С', 'Т', 'У', 'Х'],
            allowedLatinLetters: ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z'],
            showExistingCarsModal: false,
            selectedExistingCars: [],
            showExistingTooltip: false,
            showAllExistingCars: false,
            editingVehicle: null,
            inactiveTooltip: {
                visible: false,
                text: '',
                x: 0,
                y: 0
            },
            // Новые поля для проверки активных заявок
            activeCarInfo: null,
            checkingTimeout: null,
            // Проверка ЧС (#443): null или { is_blacklisted, reason }
            blacklistInfo: null,
            blacklistTimeout: null,
            // Опорный момент для предупреждений окон (#1183 S4): тикает раз в минуту,
            // чтобы баннер релевантных окон не залипал по времени.
            warningNow: new Date(),
            warningTimer: null,
            // Дебаунс эмита предупреждений наверх (#1183 polish): быстрая смена мест/
            // времени коалесцируется, панель не дёргается.
            noticesTimer: null
        }
    },
    computed: {
        selectedFormatText() {
            return this.selectedFormat ? this.selectedFormat.format.name : 'Выберите формат';
        },
        canAddExistingCars() {
            // Проверяем, что среди выбранных мест нет неактивных
            const hasInactiveSelected = this.selectedUnloadingPlaces.some(placeId => {
                const place = this.allUnloadingPlaces.find(p => p.id === placeId);
                return place && place.status !== 'active';
            });
            const placesOk = !this.fieldVisible('unloading_places') || (this.selectedUnloadingPlaces.length > 0 && !hasInactiveSelected);
            // "Проезд" обязателен так же, как для новой машины (см. useFormValidation) -
            // без выбранной таблицы проезда кнопка "Добавить" для существующих не активна.
            const passageOk = !this.fieldVisible('passage_tables') || !this.fieldRequired('passage_tables') || this.selectedPassageTables.length > 0;
            return this.selectedExistingCars.length > 0 && placesOk && passageOk;
        },
        existingCarsTooltip() {
            if (this.canAddExistingCars) return '';
            const hasInactiveSelected = this.selectedUnloadingPlaces.some(placeId => {
                const place = this.allUnloadingPlaces.find(p => p.id === placeId);
                return place && place.status !== 'active';
            });
            const missing = [];
            if (this.fieldVisible('unloading_places') && this.selectedUnloadingPlaces.length === 0) {
                missing.push('хотя бы одно место разгрузки');
            }
            if (hasInactiveSelected) {
                missing.push('убрать неактивное место разгрузки');
            }
            if (this.fieldVisible('passage_tables') && this.fieldRequired('passage_tables') && this.selectedPassageTables.length === 0) {
                missing.push('хотя бы одно место проезда');
            }
            if (missing.length === 0) return 'Заполните обязательные поля';
            if (missing.length === 1) return `Заполните поле: ${missing[0]}`;
            return `Заполните поля:\n${missing.map(f => `• ${f}`).join('\n')}`;
        },
        displayedExistingCars() {
            if (this.showAllExistingCars) return this.selectedExistingCars;
            return this.selectedExistingCars.slice(0, 5);
        },
        attachedTablesIds() {
            return this.attachedPassageTables.map(table => table.id);
        },
        filteredPassageTables() {
            return this.allPassageTables.filter(item => {
                const table = item.table || item;
                return table && table.table_type === 'cars';
            }).map(item => {
                if (item.table) {
                    return item;
                } else {
                    return { table: item };
                }
            });
        },
        // Предупреждения выбранных мест разгрузки и таблиц проезда (#1183): группа на
        // место со свободным текстом (S1), активными сейчас окнами (S4) и отчётом
        // "режим работы против окна пребывания срока" (S5). Окна зависят от warningNow
        // (тикает раз в минуту), расписание - от entryPeriod; всё реактивно, панель
        // в CreateApplication обновляется на лету при смене мест/времени.
        noticeGroups() {
            const at = this.warningNow;
            const groups = [];

            this.selectedUnloadingPlaces.forEach(placeId => {
                const place = this.allUnloadingPlaces.find(p => p.id === placeId);
                if (!place) return;
                const { free, windows } = collectActiveWarnings(place, at);
                const schedule = buildScheduleReport(place.time_slots, this.entryPeriod);
                if (free || windows.length || (schedule && schedule.anyClosed)) {
                    groups.push({ id: `place-${place.id}`, name: place.name, free, windows, schedule });
                }
            });

            this.selectedPassageTables.forEach(tableId => {
                const item = this.allPassageTables.find(t => t.table && t.table.id === tableId);
                if (!item) return;
                const { free, windows } = collectActiveWarnings(
                    { warning: item.table.warning, warning_windows: item.warning_windows },
                    at
                );
                const schedule = buildScheduleReport(item.time_slots, this.entryPeriod);
                if (free || windows.length || (schedule && schedule.anyClosed)) {
                    groups.push({ id: `table-${item.table.id}`, name: item.table.display_name || item.table.name, free, windows, schedule });
                }
            });

            return groups;
        }
    },
    watch: {
        // Следим за изменениями частей номера для проверки активности
        numberParts: {
            deep: true,
            handler() {
                this.checkVehicleActive();
                this.checkBlacklist();
            }
        },
        // Авто-выделение: если organizationId пришёл позже mounted() (async loadUserData),
        // а места ещё не выбраны - подставляем по организации.
        userOrganizationId(newVal, oldVal) {
            if (newVal && !oldVal &&
                this.selectedUnloadingPlaces.length === 0 &&
                this.applicationUnloadPlaces.length === 0) {
                this.autoSelectPlaces();
            }
        },
        userCompanyId(newVal, oldVal) {
            if (newVal && !oldVal &&
                !this.userOrganizationId &&
                this.selectedUnloadingPlaces.length === 0 &&
                this.applicationUnloadPlaces.length === 0) {
                this.autoSelectPlaces();
            }
        },
        // Предупреждения наверх в единую панель, дебаунс - гасит дёрганье при
        // быстрой смене мест/времени.
        noticeGroups: {
            handler(groups) {
                if (this.noticesTimer) clearTimeout(this.noticesTimer);
                this.noticesTimer = setTimeout(() => {
                    this.$emit('notices-change', groups);
                }, 150);
            },
            deep: true,
            immediate: true
        }
    },
    async mounted() {
        // Промис держим отдельно: правка строки может прийти раньше, чем справочник
        // форматов доедет, и тогда подбор формата по номеру ложно не находит ничего.
        this.formatsReady = this.loadLicensePlateFormats();
        await Promise.all([
            this.formatsReady,
            this.loadUnloadingPlaces(),
            this.loadMarks(),
            this.loadPassageTables()
        ]);

        document.addEventListener('click', this.handleDocumentClick);
        this.warningTimer = setInterval(() => { this.warningNow = new Date(); }, 60000);
    },
    beforeUnmount() {
        if (this.hintTimer) clearTimeout(this.hintTimer);
        if (this.inactiveTooltipTimer) clearTimeout(this.inactiveTooltipTimer);
        document.removeEventListener('click', this.handleDocumentClick);
        if (this.checkingTimeout) {
            clearTimeout(this.checkingTimeout);
        }
        if (this.blacklistTimeout) {
            clearTimeout(this.blacklistTimeout);
        }
        if (this.warningTimer) {
            clearInterval(this.warningTimer);
        }
        if (this.noticesTimer) {
            clearTimeout(this.noticesTimer);
        }
        // Форма уходит (смена типа вложения) - гасим панель, чтобы не висели стейл-группы.
        this.$emit('notices-change', []);
    },
    methods: {
        /**
         * Причина блокировки на телефоне показывается по тапу на зону кнопки
         * (сама кнопка disabled и события не даёт - на мобилке она прозрачна для
         * тапа через pointer-events) и гаснет сама.
         */
        revealBlockedHint(event) {
            // Тап по «Отменить» в режиме редактирования - не повод объяснять,
            // почему заблокировано добавление.
            if (event && event.target.closest('.cancel-edit-btn')) return;
            if (!this.isNarrow || this.canAddVehicle) return;
            this.showTooltip = true;
            if (this.hintTimer) clearTimeout(this.hintTimer);
            this.hintTimer = setTimeout(() => { this.showTooltip = false; }, 3000);
        },

        // Закрывает дропдауны формата/марки при клике вне них. Именованный метод (не
        // анонимная стрелка в mounted) - иначе removeEventListener не снимет слушатель
        // и он копится при частом откр/закр формы в модалке ручного добавления.
        handleDocumentClick(e) {
            if (!e.target.closest('.format__dropdown')) {
                this.isFormatDropdownOpen = false;
            }

            if (!e.target.closest('.mark__dropdown')) {
                this.isMarkDropdownOpen = false;
            }
        },

        // Новый метод для проверки активной заявки
        async checkVehicleActive() {
            // Отменяем предыдущий таймаут
            if (this.checkingTimeout) {
                clearTimeout(this.checkingTimeout);
            }

            // Если не заполнены обязательные поля или выбрано "по факту", не проверяем
            if (this.isNumberByFact || !this.selectedFormat || !this.numberParts.every(part => part)) {
                this.activeCarInfo = null;
                return;
            }

            // Собираем номер из частей
            const plateNumber = this.numberParts.join(' ');

            // Ждем небольшую паузу, чтобы не дёргать сервер на каждый символ
            this.checkingTimeout = setTimeout(async () => {
                try {
                    const params = new URLSearchParams();
                    params.append('car_number', plateNumber);
                    params.append('car_brand', this.selectedMark || '');
                    if (this.userOrganizationId) params.append('organization_id', this.userOrganizationId);
                    if (this.userCompanyId) params.append('company_id', this.userCompanyId);

                    const response = await apiRequest(`/cars/check-active?${params.toString()}`, {});

                    if (response.ok) {
                        const data = await response.json();
                        if (data.active) {
                            this.activeCarInfo = data;
                        } else {
                            this.activeCarInfo = null;
                        }
                    }
                } catch (error) {
                    console.error('Ошибка при проверке активности авто:', error);
                    this.activeCarInfo = null;
                }
            }, 500);
        },

        // Проверка машины в чёрном списке (#443): номер + mark_id, как и серверный гард.
        // "По факту" не проверяем - нет конкретного номера/марки для совпадения.
        checkBlacklist() {
            if (this.blacklistTimeout) {
                clearTimeout(this.blacklistTimeout);
            }

            if (this.isNumberByFact || this.isMarkByFact || !this.selectedMarkId ||
                !this.selectedFormat || !this.numberParts.every(part => part)) {
                this.blacklistInfo = null;
                return;
            }

            const plateNumber = this.numberParts.join(' ');
            const markId = this.selectedMarkId;

            this.blacklistTimeout = setTimeout(async () => {
                try {
                    const res = await checkVehicleBlacklist({ car_number: plateNumber, mark_id: markId });
                    this.blacklistInfo = res && res.is_blacklisted ? res : null;
                } catch (error) {
                    // Тихо: фоновая проверка не блокирует ввод; серверный гард - бэкстоп.
                    console.error('Ошибка при проверке ЧС машины:', error);
                    this.blacklistInfo = null;
                }
            }, 500);
        },

        async loadLicensePlateFormats() {
            try {
                const response = await apiRequest("/license-plate-formats", {
                    method: "GET"});

                if (response.ok) {
                    this.availableFormats = await response.json();
                    const defaultFormat = this.availableFormats.find(f => f.format.is_default);
                    this.selectedFormat = defaultFormat || this.availableFormats[0];
                    this.initializeNumberParts();
                } else {
                    console.error("Ошибка при загрузке форматов номеров");
                }
            } catch (error) {
                console.error("Ошибка при загрузке форматов номеров:", error);
            }
        },

        // Авто-выделение мест по организации / компании (без сброса allUnloadingPlaces).
        // Вызывается из watch, когда userOrganizationId приходит позже mounted().
        async autoSelectPlaces() {
            try {
                if (this.userOrganizationId) {
                    const orgRes = await apiRequest(`/organizations/${this.userOrganizationId}/unload-places`, { method: 'GET' });
                    if (orgRes.ok) {
                        const places = await orgRes.json();
                        if (Array.isArray(places) && places.length > 0) {
                            this.attachedUnloadingPlaces = places;
                            this.selectedUnloadingPlaces = this.activeAttachedIds(places);
                            if (this.selectedUnloadingPlaces.length > 0) {
                                useDeletionsStore().notify({ prefix: 'Место разгрузки выбрано автоматически для вашей', bold: ' организации' });
                                this.$emit('update:unload-places', [...this.selectedUnloadingPlaces]);
                            }
                            return;
                        }
                    }
                }

                if (this.userCompanyId) {
                    const compRes = await apiRequest(`/companies/${this.userCompanyId}/unload-places`, { method: 'GET' });
                    if (compRes.ok) {
                        const places = await compRes.json();
                        if (Array.isArray(places) && places.length > 0) {
                            this.attachedUnloadingPlaces = places;
                            this.selectedUnloadingPlaces = this.activeAttachedIds(places);
                            if (this.selectedUnloadingPlaces.length > 0) {
                                useDeletionsStore().notify({ prefix: 'Место разгрузки выбрано автоматически для вашей', bold: ' компании' });
                                this.$emit('update:unload-places', [...this.selectedUnloadingPlaces]);
                            }
                        }
                    }
                }
            } catch (err) {
                console.error('Ошибка при авто-выделении мест разгрузки:', err);
            }
        },

        // Из привязанных к организации/компании мест берём id тех, что реально активны.
        // org/company-эндпоинт отдаёт места без поля status (SELECT только id/name/description),
        // поэтому статус сверяем с авторитетным allUnloadingPlaces (по нему же рендерится грид) -
        // иначе фильтр `place.status === 'active'` по ответу без status давал пусто и автовыбор не
        // срабатывал (в отличие от людей, где нормализация дефолтит status в 'active').
        activeAttachedIds(attachedPlaces) {
            const ids = attachedPlaces.map(place => place.id);
            // Общий список ещё не подгружен (гонка позднего organizationId) - берём все
            // привязанные: бэк уже вернул только is_active, статус уточнится при рендере.
            if (this.allUnloadingPlaces.length === 0) return ids;
            return ids.filter(id => this.allUnloadingPlaces.some(place => place.id === id && place.status === 'active'));
        },

        async loadUnloadingPlaces() {
            this.loadingUnloadingPlaces = true;
            this.allUnloadingPlaces = [];
            this.attachedUnloadingPlaces = [];
            this.selectedUnloadingPlaces = [];
            
            try {
                const authStore = useAuthStore();
                if (!authStore.token) {
                    console.error("Токен не найден");
                    return;
                }

                const allPlacesResponse = await apiRequest("/unload-places", {
                    method: "GET"});

                if (allPlacesResponse.ok) {
                    this.allUnloadingPlaces = await allPlacesResponse.json();
                }

                if (this.applicationUnloadPlaces.length > 0) {
                    // Приоритет - единый выбор мест заявки (#706): предзаполняем новое
                    // cars-вложение уже сделанным выбором, минуя автоподстановку.
                    this.selectedUnloadingPlaces = [...this.applicationUnloadPlaces];
                } else {
                    if (this.userOrganizationId) {
                        const orgPlacesResponse = await apiRequest(`/organizations/${this.userOrganizationId}/unload-places`, {
                            method: "GET"});

                        if (orgPlacesResponse.ok) {
                            this.attachedUnloadingPlaces = await orgPlacesResponse.json();
                            this.selectedUnloadingPlaces = this.activeAttachedIds(this.attachedUnloadingPlaces);
                            if (this.selectedUnloadingPlaces.length > 0) {
                                useDeletionsStore().notify({ prefix: 'Место разгрузки выбрано автоматически для вашей', bold: ' организации' });
                            }
                        }
                    }

                    if (this.attachedUnloadingPlaces.length === 0 && this.userCompanyId) {
                        const companyPlacesResponse = await apiRequest(`/companies/${this.userCompanyId}/unload-places`, {
                            method: "GET"});

                        if (companyPlacesResponse.ok) {
                            this.attachedUnloadingPlaces = await companyPlacesResponse.json();
                            this.selectedUnloadingPlaces = this.activeAttachedIds(this.attachedUnloadingPlaces);
                            if (this.selectedUnloadingPlaces.length > 0) {
                                useDeletionsStore().notify({ prefix: 'Место разгрузки выбрано автоматически для вашей', bold: ' компании' });
                            }
                        }
                    }

                    // Поднимаем автоподставленный выбор на уровень заявки, чтобы он
                    // пережил удаление последнего cars-вложения и ушёл в items (#706).
                    if (this.selectedUnloadingPlaces.length > 0) {
                        this.$emit('update:unload-places', [...this.selectedUnloadingPlaces]);
                    }
                }

                this.validateUnloadingPlaces();

            } catch (error) {
                console.error("Ошибка при загрузке мест разгрузки:", error);
                this.allUnloadingPlaces = [];
                this.attachedUnloadingPlaces = [];
            } finally {
                this.loadingUnloadingPlaces = false;
            }
        },

        async loadPassageTables() {
            this.loadingPassageTables = true;
            this.allPassageTables = [];
            this.attachedPassageTables = [];
            this.selectedPassageTables = [];

            try {
                const authStore = useAuthStore();
                if (!authStore.token) {
                    console.error("Токен не найден");
                    return;
                }

                const allTablesResponse = await apiRequest("/system-tables", {
                    method: "GET"});

                if (allTablesResponse.ok) {
                    const tables = await allTablesResponse.json();
                    this.allPassageTables = tables.map(table => {
                        if (table.table) {
                            return table;
                        } else {
                            return {
                                table: {
                                    id: table.id,
                                    name: table.name,
                                    display_name: table.display_name,
                                    table_type: table.table_type,
                                    status: table.status || 'active',
                                    status_comment: table.status_comment,
                                    location_description: table.location_description,
                                    map_link: table.map_link
                                },
                                time_slots: table.time_slots || [],
                                photos: table.photos || [],
                                current_status: table.current_status || 'closed'
                            };
                        }
                    });
                } else {
                    console.error("Ошибка при загрузке системных таблиц");
                }

                if (this.userOrganizationId) {
                    const orgTablesResponse = await apiRequest(`/organizations/${this.userOrganizationId}/tables`, {
                        method: "GET"});

                    if (orgTablesResponse.ok) {
                        const orgTables = await orgTablesResponse.json();
                        this.attachedPassageTables = orgTables.map(table => {
                            if (table.table) {
                                return table;
                            } else {
                                return {
                                    table: {
                                        id: table.id,
                                        name: table.name,
                                        display_name: table.display_name,
                                        table_type: table.table_type,
                                        status: table.status || 'active',
                                        status_comment: table.status_comment,
                                        location_description: table.location_description,
                                        map_link: table.map_link
                                    },
                                    time_slots: table.time_slots || [],
                                    photos: table.photos || [],
                                    current_status: table.current_status || 'closed'
                                };
                            }
                        });
                    }
                }

                if (this.attachedPassageTables.length === 0 && this.userCompanyId) {
                    const companyTablesResponse = await apiRequest(`/companies/${this.userCompanyId}/tables`, {
                        method: "GET"});

                    if (companyTablesResponse.ok) {
                        const companyTables = await companyTablesResponse.json();
                        this.attachedPassageTables = companyTables.map(table => {
                            if (table.table) {
                                return table;
                            } else {
                                return {
                                    table: {
                                        id: table.id,
                                        name: table.name,
                                        display_name: table.display_name,
                                        table_type: table.table_type,
                                        status: table.status || 'active',
                                        status_comment: table.status_comment,
                                        location_description: table.location_description,
                                        map_link: table.map_link
                                    },
                                    time_slots: table.time_slots || [],
                                    photos: table.photos || [],
                                    current_status: table.current_status || 'closed'
                                };
                            }
                        });
                    }
                }

                this.validatePassageTables();

            } catch (error) {
                console.error("Ошибка при загрузке мест проезда:", error);
                this.allPassageTables = [];
                this.attachedPassageTables = [];
            } finally {
                this.loadingPassageTables = false;
            }
        },

        showInactiveTooltip(place, event) {
            if (place.status !== 'active') {
                const tooltipText = place.status_comment 
                    ? `Недоступно: ${place.status_comment}`
                    : 'Недоступно';
                
                this.inactiveTooltip.text = tooltipText;
                this.inactiveTooltip.visible = true;
                
                // Позиционируем тултип
                this.$nextTick(() => {
                    // position:fixed тултип внутри зазумленного <html>: rect device-px ->
                    // делим на zoom (inline left/top = layout-px). Отступ -10 не делим.
                    const z = getViewportZoom();
                    const rect = event.target.getBoundingClientRect();
                    this.inactiveTooltip.x = (rect.left + rect.width / 2) / z;
                    this.inactiveTooltip.y = rect.top / z - 10;
                });
            }
        },

        hideInactiveTooltip() {
            if (this.inactiveTooltipTimer) {
                clearTimeout(this.inactiveTooltipTimer);
                this.inactiveTooltipTimer = null;
            }
            this.inactiveTooltip.visible = false;
        },

        initializeNumberParts() {
            this.numberParts = initializeNumberParts(this.selectedFormat);
        },

        getPlaceholder(cell) {
            if (cell.cell_type === 'numbers') {
                return '0'.repeat(cell.max_length);
            } else {
                return 'A'.repeat(cell.max_length);
            }
        },

        getInputWidth(cell) {
            const baseWidth = 25;
            const minWidth = 50;
            const width = Math.max(minWidth, cell.max_length * baseWidth);
            return `${width}px`;
        },

        validatePart(index, event, cell) {
            const value = validatePartValue(event.target.value, cell);
            this.numberParts[index] = value;
            event.target.value = value;

            // Проверяем активность после изменения номера
            this.checkVehicleActive();

            // Клетка заполнена до предела - курсор сам прыгает в следующую,
            // чтобы номер набирался без ручного перескока между клетками.
            this.advanceCellFocus(index, value, cell);
        },

        /**
         * Переводит фокус в следующую клетку номера, когда текущая заполнена
         * полностью (только при вводе вперёд, не при стирании/правке короче).
         */
        advanceCellFocus(index, value, cell) {
            if (!value || value.length < cell.max_length) return;
            const cells = this.selectedFormat && this.selectedFormat.cells;
            if (!cells || index >= cells.length - 1) return;
            this.$nextTick(() => {
                const inputs = this.$refs.numberCells;
                const next = Array.isArray(inputs) ? inputs[index + 1] : null;
                if (next && !next.disabled) next.focus();
            });
        },

        formatPart(index, cell) {
            if (this.numberParts[index]) {
                const formatted = formatPartValue(this.numberParts[index], cell);
                if (formatted !== this.numberParts[index]) {
                    this.numberParts[index] = formatted;
                }
            }
        },

        handleNumberByFactChange() {
            if (this.isNumberByFact) {
                this.numberParts = [];
                this.activeCarInfo = null; // Сбрасываем информацию об активной заявке
            } else {
                this.initializeNumberParts();
            }
        },
        
        handleMarkByFactChange() {
            if (this.isMarkByFact) this.selectedMark = '';
        },
        
        toggleUnloadingPlace(place, event) {
            if (place.status !== 'active') {
                // На телефоне hover не наступает, и причина недоступности была недостижима:
                // показываем её по тапу и гасим сама через пару секунд.
                this.showInactiveTooltip(place, event);
                if (this.inactiveTooltipTimer) clearTimeout(this.inactiveTooltipTimer);
                this.inactiveTooltipTimer = setTimeout(() => this.hideInactiveTooltip(), 2500);
                return;
            }
            
            const index = this.selectedUnloadingPlaces.indexOf(place.id);
            if (index > -1) {
                this.selectedUnloadingPlaces.splice(index, 1);
            } else {
                this.selectedUnloadingPlaces.push(place.id);
            }
            // Единый выбор на заявку (#706): синхронизируем во все cars-вложения и items.
            this.$emit('update:unload-places', [...this.selectedUnloadingPlaces]);
        },
        
        validateUnloadingPlaces() {
            this.errors.unloadingPlaces = this.selectedUnloadingPlaces.length === 0 ? '' : '';
        },

        validatePassageTables() {
            this.errors.passageTables = this.selectedPassageTables.length === 0 ? '' : '';
        },

        formatUnloadingPlaces() {
            if (this.selectedUnloadingPlaces.length === 0) return '';
            
            const placeNames = this.selectedUnloadingPlaces.map(placeId => {
                const place = this.allUnloadingPlaces.find(p => p.id === placeId);
                return place ? place.name : '';
            }).filter(name => name);
            
            if (placeNames.length > 1) {
                return placeNames[0] + ' и др.';
            }
            
            return placeNames[0] || '';
        },

        formatDate(dateString) {
            if (!dateString) return '';
            const [year, month, day] = dateString.split('-');
            return `${day}.${month}.${year}`;
        },

        formatTime(timeString) {
            if (!timeString) return '';
            return timeString.substring(0, 5);
        },
        
        addVehicle() {
            if (!this.canAddVehicle) {
                return;
            }

            // Проверка активной заявки
            if (this.activeCarInfo && !this.isNumberByFact) {
                useDeletionsStore().notify({ prefix: 'Невозможно добавить автомобиль: ', bold: 'на него уже есть активная заявка', type: 'error' });
                return;
            }
            
            if (this.selectedExistingCars.length > 0) {
                this.addExistingCars();
                return;
            }
            
            const plateNumber = this.isNumberByFact ? 'По факту' : this.numberParts.join(' ');
            
            const mark = this.isMarkByFact ? 'По факту' : this.selectedMark;
            const markId = this.isMarkByFact ? null : (this.selectedMarkId || null);
            const markName = this.isMarkByFact ? null : (this.selectedMark || null);

            const newVehicle = {
                plateNumber: plateNumber,
                mark: mark,
                markId: markId,
                markName: markName,
                unloadingPlace: this.formatUnloadingPlaces(),
                unloadPlaces: [...this.selectedUnloadingPlaces],
                passage_tables: [...this.selectedPassageTables],
                formatId: this.selectedFormat ? this.selectedFormat.format.id : null,
                isExisting: false
            };

            const duplicate = findDuplicateVehicle(
                this.existingVehicles,
                newVehicle,
                this.editingVehicle ? this.editingVehicle.id : null,
            );
            if (duplicate) {
                useDeletionsStore().notify({
                    prefix: `${vehicleLabel(duplicate)} `,
                    bold: 'уже добавлена в список',
                    type: 'error',
                });
                return;
            }

            if (this.editingVehicle) {
                newVehicle.id = this.editingVehicle.id;
                this.$emit('vehicle-updated', newVehicle);
                this.cancelEdit();
            } else {
                this.$emit('vehicle-added', newVehicle);
                this.clearVehicleFormPartial();
            }
        },

        clearVehicleFormPartial() {
            this.initializeNumberParts();
            this.selectedMark = '';
            this.selectedMarkId = null;
            this.isNumberByFact = false;
            this.isMarkByFact = false;
            this.activeCarInfo = null;
            this.blacklistInfo = null;
        },

        clearVehicleForm() {
            this.initializeNumberParts();
            this.selectedMark = '';
            this.selectedMarkId = null;
            this.selectedUnloadingPlaces = [];
            this.selectedPassageTables = [];
            this.isNumberByFact = false;
            this.isMarkByFact = false;
            this.errors.unloadingPlaces = '';
            this.errors.passageTables = '';
            this.selectedExistingCars = [];
            this.editingVehicle = null;
            this.activeCarInfo = null;
            this.blacklistInfo = null;
        },

        openExistingCarsModal() {
            this.showExistingCarsModal = true;
        },

        closeExistingCarsModal() {
            this.showExistingCarsModal = false;
        },

        onExistingCarsSelected(cars) {
            this.selectedExistingCars = cars;
            this.showAllExistingCars = false;
            this.showExistingCarsModal = false;
            this.clearVehicleFormPartial();
        },

        // Отмена добавления конкретной существующей машины из блока "Машин добавлено".
        removeExistingCar(car) {
            const key = car.id || car.number;
            this.selectedExistingCars = this.selectedExistingCars.filter(c => (c.id || c.number) !== key);
            if (this.selectedExistingCars.length <= 5) this.showAllExistingCars = false;
        },

        addExistingCars() {
            if (this.selectedExistingCars.length === 0) {
                useDeletionsStore().notify({ bold: 'Выберите машины для добавления', type: 'error' });
                return;
            }

            if (this.selectedUnloadingPlaces.length === 0) {
                useDeletionsStore().notify({ bold: 'Выберите места разгрузки', type: 'error' });
                return;
            }

            if (this.fieldVisible('passage_tables') && this.fieldRequired('passage_tables') && this.selectedPassageTables.length === 0) {
                useDeletionsStore().notify({ bold: 'Выберите места проезда', type: 'error' });
                return;
            }

            const vehicles = this.selectedExistingCars.map(car => ({
                plateNumber: car.number,
                mark: car.mark,
                markId: car.mark_id || null,
                markName: car.mark_name || car.mark || null,
                unloadingPlace: this.formatUnloadingPlaces(),
                unloadPlaces: [...this.selectedUnloadingPlaces],
                passage_tables: [...this.selectedPassageTables],
                formatId: car.format_id,
                isExisting: true,
                existingCarId: car.id
            }));

            // Модалка выбора уже гасит добавленные строки, но выбор мог устареть, а в каталоге
            // встречаются записи с одним номером - отсеиваем здесь ещё раз.
            const list = [...this.existingVehicles];
            const toAdd = [];
            const skipped = [];
            vehicles.forEach(vehicle => {
                if (findDuplicateVehicle(list, vehicle)) {
                    skipped.push(vehicleLabel(vehicle));
                    return;
                }
                list.push(vehicle);
                toAdd.push(vehicle);
            });

            if (skipped.length > 0) {
                useDeletionsStore().notify({
                    prefix: `${skipped.join(', ')} `,
                    bold: skipped.length > 1 ? 'уже в списке - пропущены' : 'уже добавлена в список',
                    type: 'error',
                });
            }

            if (toAdd.length === 0) {
                this.clearExistingCarsSelection();
                return;
            }

            this.$emit('vehicles-added', toAdd);
            this.clearExistingCarsSelection();
        },

        clearExistingCarsSelection() {
            this.selectedExistingCars = [];
        },

        // Раскладывает номер редактируемой строки по ячейкам формата (U3). Строка несёт
        // formatId в двух случаях: добавлена вручную (numberParts.join(' ') - части через
        // пробел) или пришла из импорта бланка с явно выбранным форматом (доводка владельца,
        // BlankImportResult.buildVehicleFromRow) - там строка сырая, без пробелов по границам
        // ячеек. matchNumberToFormat разбирает оба вида одинаково (сам убирает пробелы и
        // раскладывает по cells формата), поэтому раскладка идёт через него всегда, а не
        // прямым split(' '). formatId не пришёл или номер в него не лёг - формат подбирается
        // по самой строке среди ВСЕХ активных форматов. Не подошёл ни один - явно сообщаем об
        // этом, а не оставляем пустые ячейки под чужим форматом молча.
        async applyEditedVehicleNumber(vehicle) {
            if (!this.availableFormats.length && this.formatsReady) {
                await this.formatsReady;
            }
            const knownFormat = vehicle.formatId
                ? this.availableFormats.find(f => f.format.id === vehicle.formatId)
                : null;

            if (knownFormat) {
                const matched = matchNumberToFormat(vehicle.plateNumber, [knownFormat]);
                if (matched) {
                    this.selectedFormat = knownFormat;
                    this.numberParts = matched.parts;
                    return;
                }
            }

            const guessed = matchNumberToFormat(vehicle.plateNumber, this.availableFormats);
            if (guessed) {
                this.selectedFormat = guessed.format;
                this.numberParts = guessed.parts;
                return;
            }

            this.selectedFormat = null;
            this.numberParts = [];
            useDeletionsStore().notify({
                prefix: `Номер "${vehicle.plateNumber}" не подошёл ни под один формат. `,
                bold: 'Выберите формат и введите номер вручную',
                type: 'error',
            });
        },

        async editVehicle(vehicle) {
            this.editingVehicle = vehicle;
            this.selectedExistingCars = [];
            this.activeCarInfo = null; // Сбрасываем информацию об активной заявке
            
            const restoreMarkSelection = () => {
                if (vehicle.mark === 'По факту') {
                    this.isMarkByFact = true;
                    this.selectedMark = '';
                    this.selectedMarkId = null;
                } else {
                    this.isMarkByFact = false;
                    this.selectedMark = vehicle.mark || '';
                    if (vehicle.markId) {
                        this.selectedMarkId = vehicle.markId;
                    } else {
                        const match = this.marks.find(m => m.name === vehicle.mark);
                        this.selectedMarkId = match ? match.id : null;
                    }
                }
            };

            if (vehicle.isExisting) {
                restoreMarkSelection();
                this.isNumberByFact = vehicle.plateNumber === 'По факту';
                this.selectedUnloadingPlaces = vehicle.unloadPlaces || [];
                this.selectedPassageTables = vehicle.passage_tables || [];

                if (vehicle.formatId) {
                    const format = this.availableFormats.find(f => f.format.id === vehicle.formatId);
                    if (format) {
                        this.selectedFormat = format;
                    }
                }
            } else {
                if (vehicle.plateNumber === 'По факту') {
                    this.isNumberByFact = true;
                } else {
                    this.isNumberByFact = false;
                    await this.applyEditedVehicleNumber(vehicle);
                }

                restoreMarkSelection();

                this.selectedUnloadingPlaces = vehicle.unloadPlaces || [];
                this.selectedPassageTables = vehicle.passage_tables || [];
            }

            // Перепроверяем ЧС для редактируемой машины (для "по факту" watcher
            // numberParts не сработает - сбросит/обновит баннер явно).
            this.checkBlacklist();
        },

        cancelEdit() {
            this.$emit('edit-cancelled');
            resetVehicleFormState(this);
        },
        
        toggleMarkDropdown() {
            this.isMarkDropdownOpen = !this.isMarkDropdownOpen;
            if (this.isMarkDropdownOpen) {
                this.filterMarks();
            }
        },
        
        async loadMarks() {
            try {
                const { listMarks } = await import('@/api/marks');
                const res = await listMarks();
                const arr = Array.isArray(res) ? res : (res?.marks || []);
                this.marks = arr
                    .filter(m => m.is_active !== false)
                    .map(m => ({ id: m.id, name: m.name }));
                this.filteredMarks = this.marks;
            } catch (err) {
                console.error('Не удалось загрузить справочник марок', err);
                this.marks = [];
                this.filteredMarks = [];
            }
        },

        filterMarks() {
            if (!this.markSearch) {
                this.filteredMarks = this.marks;
            } else {
                const searchTerm = this.markSearch.toLowerCase();
                this.filteredMarks = this.marks.filter(mark =>
                    mark.name.toLowerCase().includes(searchTerm)
                );
            }
        },

        selectMark(mark) {
            this.selectedMark = mark.name;
            this.selectedMarkId = mark.id;
            this.isMarkDropdownOpen = false;
            this.markSearch = '';

            this.checkVehicleActive();
            this.checkBlacklist();
        },
        
        toggleFormatDropdown() {
            this.isFormatDropdownOpen = !this.isFormatDropdownOpen;
        },
        
        selectFormat(format) {
            this.selectedFormat = format;
            this.initializeNumberParts();
            this.isFormatDropdownOpen = false;

            // Проверяем активность после смены формата
            this.checkVehicleActive();
        }
    }
}
</script>

<style scoped>
.input__label {
    font-size: 13px;
    color: var(--text-muted);
}

.required {
    color: var(--danger-text);
}

.data__completion {
    padding: 15px;
    width: 450px;
    border-right: 1px solid var(--border);
}

.data__completion--locked {
    position: relative;
}

.completion__format {
    display: flex;
    flex-direction: column;
    gap: 10px;
    position: relative;
    padding-bottom: 15px;
}

.format__header {
    display: flex;
    justify-content: space-between;
    align-items: end;
}

.format__label {
    order: -1;
    font-size: 13px;
    color: var(--text-muted);
}

.format-actions {
    display: flex;
    gap: 10px;
    align-items: center;
    position: relative;
}

.cancel-edit-btn {
    background: var(--surface-2);
    color: var(--text);
    border: 1px solid var(--border);
    border-radius: 15px;
    padding: 8px 15px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.cancel-edit-btn:hover {
    background: var(--row-hover);
}

.add-button {
    background: var(--accent);
    color: var(--accent-contrast);
    /* Рамка держит место всегда: без неё соседняя «Отменить» с рамкой выше
       на 2px и строка кнопок прыгает при входе в режим редактирования. */
    border: 1px solid transparent;
    border-radius: 15px;
    padding: 8px 15px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
    margin-top: 0;
    position: relative;
}

.add-button:hover:not(:disabled) {
    background: var(--accent-hover);
}

.add-button:disabled {
    background: var(--text-muted);
    cursor: not-allowed;
    opacity: 0.6;
}

.tooltip {
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: 5px;
    z-index: 1000;
}

.tooltip-content {
    background: var(--hint-bg);
    color: var(--hint-text);
    padding: 10px 12px;
    border-radius: 8px;
    font-size: 12px;
    max-width: 420px;
    min-width: 420px;
    box-shadow: 0 2px 8px var(--shadow-drop);
}

.tooltip-content::before {
    content: '';
    position: absolute;
    bottom: 100%;
    right: 40px;
    border: 5px solid transparent;
    border-bottom-color: var(--hint-bg);
}

.inactive-tooltip {
    position: fixed;
    transform: translateX(-50%) translateY(-100%);
    z-index: 10000;
    pointer-events: none;
}

.inactive-tooltip-content {
    background: var(--hint-bg);
    color: var(--hint-text);
    padding: 8px 12px;
    border-radius: 8px;
    font-size: 12px;
    max-width: 300px;
    box-shadow: 0 2px 8px var(--shadow-drop);
}

.inactive-tooltip-content::before {
    content: '';
    position: absolute;
    top: 100%;
    left: 50%;
    transform: translateX(-50%);
    border: 5px solid transparent;
    border-top-color: var(--hint-bg);
}

.active-warning {
    width: 100%;
    margin-top: 10px;
    padding: 12px;
    background: var(--warning-bg);
    border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
    border-radius: 10px;
    display: flex;
    gap: 12px;
    align-items: flex-start;
}

.warning-icon {
    font-size: 20px;
    flex-shrink: 0;
}

.warning-text {
    flex: 1;
}

.warning-title {
    font-weight: 600;
    color: var(--warning-text);
    margin: 0 0 5px 0;
    font-size: 14px;
}

.warning-details {
    color: var(--warning-text);
    margin: 0;
    font-size: 12px;
    line-height: 1.5;
}

.blacklist-warning {
    width: 100%;
    margin-top: 10px;
    padding: 12px;
    background: var(--danger-bg);
    border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
    border-radius: 10px;
    display: flex;
    gap: 12px;
    align-items: flex-start;
}

.blacklist-warning .warning-title,
.blacklist-warning .warning-details {
    color: var(--danger-text);
}

.format__dropdown {
    position: relative;
}

.dropdown__button {
    width: 100%;
    height: 30px;
    border: 1px solid var(--border);
    background-color: var(--surface);
    border-radius: 50px;
    outline: none;
    cursor: pointer;
    padding: 0 15px;
    transition: border-color 0.2s;
}

.dropdown__button:hover:not(:disabled) {
    border-color: var(--accent);
}

.dropdown__button:disabled {
    background-color: var(--surface-2);
    cursor: not-allowed;
    opacity: 0.6;
}

.button__content {
    display: flex;
    align-items: center;
    width: 100%;
    height: 100%;
    justify-content: space-between;
}

.completion__header {
    padding-bottom: 15px;
    display: flex;
    justify-content: space-between;
}

.button__text {
    font-size: 14px;
    color: var(--text);
    font-weight: 500;
    display: block;
}

.button__arrow {
    width: 10px;
    height: 10px;
    transition: transform 0.2s;
    transform: rotate(90deg);
    flex-shrink: 0;
}

.button__arrow--open {
    transform: rotate(-90deg);
}

.dropdown__menu {
    position: absolute;
    top: 100%;
    left: 0;
    width: 100%;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    margin-top: 5px;
    box-shadow: 0 3px 10px var(--shadow-drop);
    z-index: 1000;
    max-height: 300px;
    overflow-y: auto;
}

.dropdown__item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 15px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.dropdown__item:hover {
    background-color: var(--surface-2);
}

.dropdown__item:first-child {
    border-radius: 10px 10px 0 0;
}

.dropdown__item:last-child {
    border-radius: 0 0 10px 10px;
}

.item__text {
    font-size: 13px;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.completion__fields {
    display: flex;
    gap: 20px;
    align-items: flex-start;
    margin-bottom: 15px;
}

.completion__number,
.completion__mark {
    flex: 1;
}

.completion__number-header,
.completion__mark-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 5px;
}

.number-fact,
.mark-fact {
    display: flex;
    align-items: center;
    gap: 5px;
}

/* Тумблер вместо чекбокса 12px: попасть в него пальцем было нельзя, а подпись
   не была кликабельной. Нативный input прячем, но оставляем в потоке для фокуса
   с клавиатуры; вся пара «переключатель + подпись» - один label. */
.fact-toggle {
    cursor: pointer;
    user-select: none;
}

.fact-checkbox {
    position: absolute;
    width: 1px;
    height: 1px;
    opacity: 0;
    pointer-events: none;
}

.fact-switch {
    position: relative;
    display: inline-block;
    width: 34px;
    height: 20px;
    border-radius: var(--radius-pill);
    background: var(--border);
    transition: background-color 0.2s ease;
    flex-shrink: 0;
}

.fact-switch::after {
    content: '';
    position: absolute;
    top: 2px;
    left: 2px;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: var(--surface);
    transition: transform 0.2s ease;
}

.fact-checkbox:checked + .fact-switch {
    background: var(--color-primary);
}

.fact-checkbox:checked + .fact-switch::after {
    transform: translateX(14px);
}

.fact-checkbox:focus-visible + .fact-switch {
    box-shadow: var(--shadow-focus);
}

.fact-checkbox:disabled + .fact-switch {
    opacity: 0.6;
}

/* Курсор на всём label, а не только на переключателе: label и есть кликабельная зона. */
.fact-toggle:has(.fact-checkbox:disabled) {
    cursor: not-allowed;
}

.fact-text {
    font-size: 13px;
}

.number__field {
    max-width: 202px;
    min-width: 202px;
    height: 40px;
    display: flex;
    border: 1px solid var(--border);
    border-radius: 15px;
    overflow: hidden;
    background: var(--surface);
}

.number__field--fact {
    display: block;
}

.number__input {
    border: none;
    height: 100%;
    outline: none;
    text-align: center;
    font-size: 14px;
    background: transparent;
    flex: 1;
    min-width: 0;
}

.number__input:disabled {
    background-color: var(--surface-2);
    cursor: not-allowed;
}

.number__input--fact {
    width: 100%;
    text-align: left;
    padding: 0 15px;
    color: var(--text-muted);
}

.number__input:not(:last-child) {
    border-right: 1px solid var(--border);
}

.number__input:first-child {
    border-radius: 15px 0 0 15px;
}

.number__input:last-child {
    border-radius: 0 15px 15px 0;
}

.number__input::placeholder {
    color: var(--text-muted);
    font-size: 12px;
}

.number__input:focus {
    background-color: var(--surface-2);
}

.no-format-message {
    font-size: 12px;
    color: var(--text-muted);
    text-align: center;
    padding: 10px;
    background: var(--surface-2);
    border-radius: 10px;
    border: 1px solid var(--border);
}

.mark__field {
    width: 100%;
    height: 40px;
    position: relative;
}

.mark__field--fact {
    border: 1px solid var(--border);
    border-radius: 15px;
    overflow: hidden;
}

.mark__dropdown {
    width: 100%;
    height: 100%;
}

.mark__dropdown-button {
    width: 100%;
    height: 100%;
    border: 1px solid var(--border);
    background-color: var(--surface);
    border-radius: 15px;
    outline: none;
    cursor: pointer;
    /* Правый паддинг больше левого - без запаса длинная марка обрезалась
       эллипсисом впритык к стрелке, и они визуально слипались. */
    padding: 0 20px 0 15px;
    transition: border-color 0.2s;
}

.mark__dropdown-button:hover {
    border-color: var(--accent);
}

.mark__button-content {
    display: flex;
    align-items: center;
    width: 100%;
    height: 100%;
    justify-content: space-between;
    /* Тот же зазор, что у BaseDropdown.vue: без него текст и стрелка - соседние
       flex-элементы без гарантированного расстояния - сходятся вплотную. */
    gap: 10px;
}

.mark__button-text {
    font-size: 14px;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 150px;
    /* Без min-width: 0 flex-элемент с white-space: nowrap отказывается сжиматься
       ниже собственной content-ширины (дефолтный min-width: auto у flex-детей) -
       ellipsis объявлен, но не срабатывает, и длинная марка вылезает за поле. */
    min-width: 0;
    display: block;
}

/* Стрелка - тот же inline SVG-шеврон, что у BaseDropdown.vue (см. другие дропдауны
   проекта): растровый arrow.png 10x10 на Retina-экранах масштабируется блоками
   пикселей и выглядит зазубренным, SVG чёткий на любом DPI. Поворот -90/90deg (не
   0/180, как у BaseDropdown) - это боковое меню (dropdown__menu открывается вправо
   от кнопки), не выпадающее вниз. Знак важен: шеврон нарисован остриём вниз, и
   поворот по часовой уводит его влево, от меню. */
.mark__button-arrow {
    width: 10px;
    height: 10px;
    color: var(--text-muted);
    transition: transform 0.2s;
    transform: rotate(-90deg);
    flex-shrink: 0;
}

.mark__button-arrow--open {
    transform: rotate(90deg);
}

.mark__dropdown-menu {
    position: absolute;
    top: 0;
    left: 100%;
    width: 220px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    margin-left: 5px;
    box-shadow: 0 3px 10px var(--shadow-drop);
    z-index: 1000;
    max-height: 220px;
    overflow: hidden;
}

.mark__search {
    padding: 10px;
    border-bottom: 1px solid var(--border);
}

.mark__search-input {
    width: 100%;
    border: 1px solid var(--border);
    border-radius: 15px;
    padding: 5px 10px;
    outline: none;
    font-size: 14px;
}

.mark__dropdown-list {
    max-height: 144px;
    overflow-y: auto;
}

.mark__dropdown-item {
    padding: 8px 15px;
    cursor: pointer;
    transition: background-color 0.2s;
    border-bottom: 1px solid var(--surface-2);
}

.mark__dropdown-item:hover {
    background-color: var(--surface-2);
}

.mark__dropdown-item:last-child {
    border-bottom: none;
}

.mark__dropdown-empty {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 56px;
    padding: 16px 15px;
    color: var(--text-muted);
    font-size: 14px;
    text-align: center;
}

.mark__item-text {
    font-size: 14px;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.mark__input {
    width: 100%;
    height: 100%;
    border: none;
    outline: none;
    background: transparent;
    padding: 0 15px;
    font-size: 14px;
    color: var(--text-muted);
}

.mark__input--fact::placeholder {
    color: var(--text-muted);
    font-size: 12px;
}

.completion__unloading {
    margin-top: 15px;
}

.unloading-info {
    margin-bottom: 10px;
}

.unloading-source {
    font-size: 12px;
    color: var(--accent-text);
    font-weight: 500;
    background: var(--accent-tint);
    padding: 5px 10px;
    border-radius: 8px;
    display: inline-block;
}

.attached-count {
    font-size: 11px;
    color: var(--text-muted);
    font-weight: normal;
}

.attached-badge {
    position: absolute;
    top: 2px;
    right: 2px;
    background: var(--accent);
    color: var(--accent-contrast);
    border-radius: 50%;
    width: 16px;
    height: 16px;
    font-size: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.unloading__grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
    row-gap: 5px;
    max-width: 425px;
    margin-top: 5px;
    position: relative;
}

.unloading__item {
    height: 30px;
    background: var(--surface-2);
    color: var(--text-muted);
    border-radius: 50px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s;
    padding: 0 10px;
    text-align: center;
    border: 1px solid transparent;
    position: relative;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.unloading__item:hover:not(.unloading__item--active):not(.unloading__item--inactive) {
    background: var(--row-hover);
}

.unloading__item--active {
    background: var(--accent);
    color: var(--accent-contrast);
    border-color: var(--accent);
}

.unloading__item--inactive {
    background: var(--danger-bg);
    color: var(--danger-text);
    border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
    cursor: not-allowed;
    opacity: 0.7;
}

.error-message {
    font-size: 11px;
    color: var(--danger-text);
    margin-top: 5px;
}

.loading-message {
    font-size: 12px;
    color: var(--text-muted);
    text-align: center;
    padding: 20px;
}

.no-places-message {
    font-size: 12px;
    color: var(--danger-text);
    text-align: center;
    padding: 20px;
    background: var(--danger-bg);
    border-radius: 8px;
    margin-top: 10px;
}

.completion__passage {
    margin-top: 15px;
}

.no-tables-message {
    font-size: 12px;
    color: var(--danger-text);
    text-align: center;
    padding: 20px;
    background: var(--danger-bg);
    border-radius: 8px;
    margin-top: 10px;
}

.completion__button {
    width: fit-content;
    height: 25px;
    padding: 0 15px;
    border-radius: 50px;
    background: var(--surface);
    border: 1px solid var(--border);
    outline: none;
    font-size: 11px;
    color: var(--accent-text);
    font-weight: 600;
    cursor: pointer;
}

.completion__button:hover {
    background-color: var(--surface-2);
}

.existing-cars-info {
    margin-bottom: 15px;
    padding: 10px;
    background: var(--surface-2);
    border-radius: 15px;
    border: 1px solid var(--border);
}

.existing-cars-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.existing-cars-count {
    font-size: 14px;
    font-weight: 500;
    color: var(--text);
}

.existing-cars-actions {
    display: flex;
    gap: 10px;
}

.view-cars-btn {
    background: var(--surface);
    color: var(--accent-text);
    border: 1px solid var(--accent);
    border-radius: 15px;
    padding: 5px 10px;
    font-size: 11px;
    cursor: pointer;
}

.view-cars-btn:hover {
    background: var(--accent-tint);
}

.add-existing-btn {
    background: var(--accent);
    color: var(--accent-contrast);
    border: none;
    border-radius: 15px;
    padding: 5px 10px;
    font-size: 11px;
    cursor: pointer;
}

.add-existing-btn:hover:not(:disabled) {
    background: var(--accent-hover);
}

.add-existing-btn:disabled {
    background: var(--text-muted);
    cursor: not-allowed;
    opacity: 0.6;
}

.existing-cars-list {
    margin-top: 8px;
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.existing-car-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    font-size: 12px;
    color: var(--text);
    padding: 3px 6px;
    background: var(--surface);
    border-radius: 8px;
    border: 1px solid var(--border);
}

.existing-car-remove {
    flex-shrink: 0;
    width: 18px;
    height: 18px;
    line-height: 1;
    border: none;
    border-radius: 50%;
    background: var(--surface-2);
    color: var(--text-muted);
    font-size: 14px;
    cursor: pointer;
    transition: background 0.15s, color 0.15s;
}

.existing-car-remove:hover {
    background: var(--danger-bg);
    color: var(--danger-text);
}

.add-existing-wrap {
    position: relative;
    /* inline-flex плотно оборачивает кнопку - она остаётся в исходной позиции
       и виде, не смещается; tooltip позиционируется относительно неё. */
    display: inline-flex;
}

.show-all-btn {
    margin-top: 2px;
    background: none;
    border: none;
    color: var(--accent-text);
    font-size: 11px;
    cursor: pointer;
    padding: 2px 6px;
    text-align: left;
}

.show-all-btn:hover {
    text-decoration: underline;
}

.dropdown-enter-active,
.dropdown-leave-active {
    transition: all 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
    opacity: 0;
    transform: translateY(-10px);
}

/* Форма (450px) + список машин рядом не влезают на планшете - стекаем в колонку
   (form__data в CreateApplication.vue делает то же на этом же брейкпоинте). */
@media (max-width: 1024px) {
    .data__completion {
        width: 100%;
        border-right: none;
        border-bottom: 1px solid var(--border);
    }
}

@media (max-width: 768px) {
    /* Подсказка поверх НАД кнопкой: в потоке она двигала форму. Контейнер
       кнопок - её positioned-родитель. */
    .tooltip {
        position: absolute;
        top: auto;
        bottom: calc(100% + 10px);
        right: 0;
        left: auto;
        margin: 0;
        width: min(320px, calc(100vw - 44px));
        z-index: 1100;
    }

    .tooltip-content::before {
        display: none;
    }

    /* Тап по заблокированной кнопке уходит контейнеру и показывает причину. */
    .add-button:disabled {
        pointer-events: none;
    }

    /* Заголовок блока и кнопка «Добавить сущ.» - строго одна строка. nowrap, а не
       перенос: флекс решает про перенос по НАТУРАЛЬНОЙ ширине элемента, до
       flex-shrink, поэтому сжимаемый заголовок кнопку в строке не удерживал -
       «Добавление Т/С» (165px при 18.72px по умолчанию) плюс кнопка 124px требовали
       299px при 268 доступных на 320, и кнопка падала под заголовок.
       Кегль заголовка задан явно (по умолчанию h3 = 18.72px): 15px совпадает с
       подписью соседнего списка, на узких телефонах 14px - как у неё же. */
    .completion__header {
        align-items: center;
        flex-wrap: nowrap;
        gap: 8px;
    }

    .completion__header h3 {
        min-width: 0;
        font-size: 15px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .completion__button {
        flex-shrink: 0;
        min-height: 36px;
    }

    /* Лейбл и кнопка добавления - на одной линии. */

    /* Кнопка «Добавить» - внизу формы, куда пользователь приходит, заполнив поля.
       contents у двух обёрток выводит строку кнопок в прямые дети формы, флекс с
       order отправляет её в конец; лейбл «Формат номеров» остаётся над дропдауном. */
    /* Места разгрузки и проезд - сиблинги этой обёртки, поэтому флекс с order
       живёт на всей форме, а обёртка разворачивается. */
    .data__completion {
        display: flex;
        flex-direction: column;
    }

    .completion__body {
        display: contents;
    }

    .completion__format,
    .format__header {
        display: contents;
    }

    .format-actions {
        order: 999;
        justify-content: stretch;
        margin-top: 4px;
        padding: 4px 0;
    }

    .format-actions .add-button {
        flex: 1;
        min-height: 44px;
    }

    .format-actions .cancel-edit-btn {
        min-height: 44px;
    }

    /* Компенсация потерянных отступов contents-обёрток. */
    /* order:-1 нужен только десктопной строке шапки: во флексе всей формы он
       утаскивал лейбл в самое начало. */
    .format__label {
        order: 0;
        display: block;
        margin-bottom: 6px;
    }

    .format__dropdown {
        margin-bottom: 15px;
    }

    .tooltip-content {
        max-width: 100%;
        white-space: pre-line;
    }

    .tooltip-content {
        min-width: 0;
        max-width: calc(100vw - 40px);
    }

    /* Дропдаун марки раскрывался ВПРАВО от поля (left:100%) - на узком уезжает
       за край экрана. На мобильном раскрываем вниз, как у EmployeeForm. */
    .mark__dropdown-menu {
        left: 0;
        top: 100%;
        width: 100%;
        margin-left: 0;
        margin-top: 5px;
    }

    /* Поля и гриды перестраивались только с 480 - на 481-768 форма ещё жила
       десктопной сеткой. */
    .completion__fields {
        flex-direction: column;
    }

    .unloading__grid {
        grid-template-columns: repeat(2, 1fr);
        max-width: 100%;
    }

    /* Поле госномера было фиксировано на 202px - ячейки под палец не попадали.
       В колонке flex:1 ширину не задаёт, поэтому растягиваем сами блоки. */
    .completion__number,
    .completion__mark {
        width: 100%;
    }

    /* Ячейки растянулись по ширине, а высота и шрифт остались десктопными -
       поле выглядело приплюснутым. */
    .number__field {
        min-width: 0;
        max-width: none;
        width: 100%;
        height: 52px;
    }

    .number__input {
        font-size: 18px;
        font-weight: 500;
    }

    .number__input::placeholder {
        font-size: 15px;
    }

    .mark__dropdown,
    .mark__dropdown-button,
    .mark__field {
        width: 100%;
    }

    .dropdown__button {
        height: 40px;
    }

    .add-button {
        min-height: 40px;
        padding: 8px 18px;
        font-size: 13px;
    }

    .completion__button {
        height: 36px;
        font-size: 12px;
    }

    /* Названия мест не влезали в 164px и обрезались многоточием - пускаем в
       две строки вместо ellipsis. */
    .unloading__item {
        height: auto;
        min-height: 36px;
        padding: 6px 10px;
        white-space: normal;
        overflow: visible;
        text-overflow: clip;
        line-height: 1.25;
    }
}

/* Узкие телефоны: на 320 ряду шапки формы остаётся 268px. Кегль как у подписи
   соседнего списка (14px) и более плотные поля кнопки дают запас - заголовок
   читается целиком, без многоточия. */
@media (max-width: 480px) {
    .completion__header h3 {
        font-size: 14px;
    }

    .completion__button {
        padding: 0 12px;
    }
}
</style>
