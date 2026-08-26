<template>
  <div
    class="data__completion"
    :class="{ 'data__completion--locked': disabled }"
    :inert="disabled ? '' : undefined"
  >
    <div class="completion__header">
      <h3>Новый сотрудник</h3>
      <button
        v-if="allowExistingSearch"
        class="completion__button"
        data-testid="ob-form-existing"
        @click="openExistingEmployeesModal"
      >
        {{ isNarrow ? 'Добавить сущ.' : 'Добавить существующего(-их)' }}
      </button>
    </div>

    <div
      v-if="selectedExistingEmployees.length > 0"
      class="existing-employees-info"
    >
      <div class="existing-employees-header">
        <span class="existing-employees-count">Сотрудников добавлено: {{ selectedExistingEmployees.length }}</span>
        <div class="existing-employees-actions">
          <button
            class="view-employees-btn"
            @click="openExistingEmployeesModal"
          >
            Просмотреть
          </button>
          <button
            class="add-existing-btn"
            :disabled="!canAddExistingEmployees"
            @click="addExistingEmployees"
          >
            Добавить
          </button>
        </div>
      </div>
    </div>

    <div
      v-else
      class="completion__body"
    >
      <div
        v-if="fieldVisible('citizenship')"
        class="completion__citizenship"
      >
        <div class="citizenship__header">
          <div
            class="citizenship-actions"
            @click="revealBlockedHint($event)"
          >
            <button
              v-if="editingEmployee"
              class="cancel-edit-btn"
              @click="cancelEdit"
            >
              Отменить
            </button>
            <button 
              class="add-button" 
              :disabled="!canAddEmployee"
              @click="addEmployee"
              @mouseenter="showTooltip = true"
              @mouseleave="showTooltip = false"
            >
              {{ editingEmployee ? 'Применить' : 'Добавить' }}
            </button>
            <div
              v-if="showTooltip && !canAddEmployee"
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
          <label class="citizenship__label">Гражданство <span
            v-if="fieldRequired('citizenship')"
            class="required"
          >*</span></label>
        </div>
        <div class="citizenship__dropdown">
          <button 
            ref="citizenshipButton"
            class="dropdown__button" 
            :disabled="editingEmployee && editingEmployee.isExisting"
            @click="toggleCitizenshipDropdown"
          >
            <div class="button__content">
              <span class="button__text">{{ selectedCitizenshipText }}</span>
              <AppIcon
                name="arrow"
                class="button__arrow"
                :class="{ 'button__arrow--up': citizenshipArrowUp }"
              />
            </div>
          </button>
          <transition name="dropdown">
            <div
              v-if="isCitizenshipDropdownOpen"
              class="dropdown__menu"
              :style="citizenshipMenuStyle"
            >
              <div 
                v-for="citizenship in availableCitizenships" 
                :key="citizenship.id"
                class="dropdown__item" 
                @click="selectCitizenship(citizenship)"
              >
                <span class="item__text">{{ citizenship.name }}</span>
                <span
                  v-if="citizenship.patent_required"
                  class="patent-required-badge"
                >Требуется патент</span>
              </div>
            </div>
          </transition>
        </div>
      </div>
            
      <div class="completion__fields">
        <div
          v-if="fieldVisible('last_name') || fieldVisible('first_name')"
          class="completion__name-row"
        >
          <div
            v-if="fieldVisible('last_name')"
            class="completion__last-name"
          >
            <div class="completion__last-name-header">
              <label class="input__label">Фамилия <span
                v-if="fieldRequired('last_name')"
                class="required"
              >*</span></label>
            </div>
            <input
              v-model="lastName"
              class="name__input"
              placeholder="Введите фамилию"
              :disabled="editingEmployee && editingEmployee.isExisting"
              @blur="formatNameField('lastName')"
            >
          </div>
          <div
            v-if="fieldVisible('first_name')"
            class="completion__first-name"
          >
            <div class="completion__first-name-header">
              <label class="input__label">Имя <span
                v-if="fieldRequired('first_name')"
                class="required"
              >*</span></label>
            </div>
            <input
              v-model="firstName"
              class="name__input"
              placeholder="Введите имя"
              :disabled="editingEmployee && editingEmployee.isExisting"
              @blur="formatNameField('firstName')"
            >
          </div>
        </div>

        <div
          v-if="fieldVisible('middle_name') || fieldVisible('position')"
          class="completion__name-row"
        >
          <div
            v-if="fieldVisible('middle_name')"
            class="completion__middle-name"
          >
            <div class="completion__middle-name-header">
              <label class="input__label">Отчество <span
                v-if="fieldRequired('middle_name')"
                class="required"
              >*</span></label>
            </div>
            <input
              v-model="middleName"
              class="name__input"
              placeholder="Введите отчество"
              :disabled="editingEmployee && editingEmployee.isExisting"
              @blur="formatNameField('middleName')"
            >
          </div>
          <div
            v-if="fieldVisible('position')"
            class="completion__position"
          >
            <div class="completion__position-header">
              <label class="input__label">Должность <span
                v-if="fieldRequired('position')"
                class="required"
              >*</span></label>
            </div>
            <input
              v-model="position"
              class="name__input"
              placeholder="Введите должность"
              :disabled="editingEmployee && editingEmployee.isExisting"
              @blur="formatNameField('position')"
            >
          </div>
        </div>

        <div
          v-if="blacklistInfo"
          class="blacklist-warning"
          data-testid="person-blacklist-warning"
        >
          <p class="warning-title">
            Человек в чёрном списке
          </p>
          <p class="warning-details">
            Причина: {{ blacklistInfo.reason || 'не указана' }}<br>
            Добавить этого человека в заявку нельзя.
          </p>
        </div>

        <div
          v-if="fieldVisible('passport') || fieldVisible('patent')"
          class="completion__name-row"
        >
          <div
            v-if="fieldVisible('passport')"
            class="completion__passport"
          >
            <div class="completion__passport-header">
              <label class="input__label">Паспортные данные <span
                v-if="fieldRequired('passport')"
                class="required"
              >*</span></label>
            </div>
            <input
              v-model="passportSeriesNumber"
              class="name__input"
              placeholder="Введите паспортные данные"
              :disabled="editingEmployee && editingEmployee.isExisting"
            >
          </div>
          <div
            v-if="fieldVisible('patent')"
            class="completion__patent"
            :class="{ 'disabled-field': !effectivePatentRequired }"
          >
            <div class="completion__patent-header">
              <label class="input__label">Номер патента <span
                v-if="fieldRequired('patent') && effectivePatentRequired"
                class="required"
              >*</span></label>
            </div>
            <input
              v-model="patentNumber"
              class="name__input"
              :placeholder="effectivePatentRequired ? 'Номер патента' : 'Не требуется'"
              :disabled="!effectivePatentRequired || patentFieldDisabled || (editingEmployee && editingEmployee.isExisting)"
              @input="handlePatentInput"
            >
          </div>
        </div>

        <div
          v-if="fieldVisible('work_permission')"
          class="completion__permission"
          :class="{ 'disabled-field': !effectivePatentRequired }"
        >
          <div class="completion__permission-header">
            <label class="input__label">Иное разрешение на работы</label>
          </div>
          <div
            ref="permissionDropdown"
            class="permission__dropdown"
          >
            <button
              type="button"
              class="permission__dropdown-button"
              :class="{ 'permission__dropdown-button--placeholder': !selectedPermission }"
              :disabled="!effectivePatentRequired || permissionFieldDisabled || (editingEmployee && editingEmployee.isExisting)"
              :title="selectedPermission || null"
              @click="togglePermissionDropdown"
            >
              <div class="permission__button-content">
                <span class="permission__button-text">{{ selectedPermission || (effectivePatentRequired ? 'Не выбрано' : 'Не требуется') }}</span>
                <AppIcon
                  name="arrow"
                  class="permission__button-arrow"
                  :class="{ 'permission__button-arrow--up': permissionArrowUp }"
                />
              </div>
            </button>
            <transition name="dropdown">
              <div
                v-if="isPermissionDropdownOpen"
                class="permission__dropdown-menu"
                :style="permissionMenuStyle"
              >
                <div class="permission__dropdown-search">
                  <input
                    ref="permissionSearch"
                    v-model="permissionQuery"
                    type="text"
                    class="permission__search-input"
                    placeholder="Поиск по списку"
                    @click.stop
                    @keydown.esc.prevent="isPermissionDropdownOpen = false"
                    @keydown.enter.prevent="selectOnlyFoundPermission"
                  >
                </div>
                <div class="permission__dropdown-list">
                  <!-- "Не выбрано" остаётся в списке и при поиске: это единственный
                       способ снять выбор, прятать его за очисткой запроса незачем. -->
                  <div
                    class="permission__dropdown-item"
                    :class="{ 'permission__dropdown-item--selected': !selectedPermission }"
                    @click="selectPermission('')"
                  >
                    <span class="permission__item-text permission__item-text--empty">Не выбрано</span>
                  </div>
                  <div
                    v-for="permission in filteredPermissions"
                    :key="permission"
                    class="permission__dropdown-item"
                    :class="{ 'permission__dropdown-item--selected': permission === selectedPermission }"
                    @click="selectPermission(permission)"
                  >
                    <span class="permission__item-text">{{ permission }}</span>
                  </div>
                  <div
                    v-if="filteredPermissions.length === 0"
                    class="permission__dropdown-empty"
                  >
                    Ничего не найдено
                  </div>
                </div>
              </div>
            </transition>
          </div>
        </div>
      </div>
    </div>

    <div
      v-if="fieldVisible('target_tables')"
      class="completion__passage"
      data-testid="ob-form-places"
    >
      <label class="input__label">Места прохода (целевые таблицы) <span
        v-if="fieldRequired('target_tables')"
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
        Загрузка мест прохода...
      </div>
      <div
        v-else
        class="no-tables-message"
      >
        Нет доступных мест прохода
      </div>
      <div
        v-if="errors.passageTables"
        class="error-message"
      >
        {{ errors.passageTables }}
      </div>
    </div>

    <!-- Согласие субъекта на обработку его персональных данных (152-ФЗ). Показывается
         и требуется по настройке полей вложения; у сотрудника, выбранного из реестра,
         согласие уже получено при заведении записи - там отметка не спрашивается. -->
    <div
      v-if="fieldVisible('pd_consent') && selectedExistingEmployees.length === 0"
      class="completion__consent"
    >
      <label class="consent-option">
        <input
          v-model="pdConsent"
          type="checkbox"
          data-testid="employee-pd-consent"
        >
        <span>
          Работник дал <a
            href="/data-processing"
            target="_blank"
            rel="noopener"
            class="blue"
            @click.stop
          >согласие</a> на обработку своих персональных данных<span
            v-if="fieldRequired('pd_consent')"
            class="required"
          >*</span>
        </span>
      </label>
    </div>

    <!-- Предупреждения выбранных таблиц прохода (#1183): единая плавающая панель
         рендерится в CreateApplication (@notices-change). -->

    <ExistingEmployeesModal
      :visible="showExistingEmployeesModal"
      :already-added-employees="existingEmployees"
      :user-organization-id="userOrganizationId"
      :initial-selected-employees="selectedExistingEmployees"
      :z-index="existingModalZIndex"
      @employees-selected="onEmployeesSelected"
      @close="closeExistingEmployeesModal"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { checkPersonBlacklist } from '@/api/blacklist'
import { useAuthStore } from '@/stores/auth'
import { useDeletionsStore } from '@/stores/deletions'
import ExistingEmployeesModal from '@/components/CreateApplication/ExistingEmployeesModal.vue'
import TargetTablesGrid from '@/components/CreateApplication/TargetTablesGrid.vue'
import { useFormValidation } from '@/composables/useFormValidation'
import { useNarrowScreen } from '@/composables/useNarrowScreen'
import { useFieldConfig } from '@/composables/useFieldConfig'
import { resetEmployeeFormState } from './entryFormReset'
import { collectActiveWarnings } from '@/utils/warningWindows'
import { buildScheduleReport } from '@/utils/scheduleCheck'
import { findDuplicateEmployee, employeeLabel } from '@/utils/applicationDuplicates'
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants'
import { getCurrentInstance } from 'vue'
import { getViewportZoom } from '@/utils/viewportScale'
import AppIcon from '@/components/icons/AppIcon.vue'

export default {
    name: 'EmployeeForm',
    components: {
        AppIcon,
        ExistingEmployeesModal,
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
        existingEmployees: {
            type: Array,
            default: () => []
        },
        // Настройка полей шаблона (#529): { [fieldKey]: { visible, required, locked, requirable } }.
        // Раздаётся из CreateApplication; потребление (скрытие/обязательность полей людей) - срез H-6.
        fieldConfig: {
            type: Object,
            default: () => ({})
        },
        // Гейт п.36: форма недоступна, пока не заполнены обязательные поля вложения.
        disabled: {
            type: Boolean,
            default: false
        },
        // Ручное добавление (#1049): DTO ManualEmployee не имеет existing_employee_id,
        // поэтому выбор "существующего" сотрудника создал бы дубликат - прячем поиск
        // в этом контексте (зеркало VehicleForm.allowExistingSearch).
        allowExistingSearch: {
            type: Boolean,
            default: true
        },
        // Слой окна «Добавить существующего(-их)». Дефолт 1000 - подача заявки; форма,
        // встроенная в окно поверх детали заявки, поднимает его (#1685).
        existingModalZIndex: {
            type: Number,
            default: 1000
        },
        // Срок заявки текущего вложения (#1183 S5): { date_from, date_to, time_from,
        // time_to } в API-формате. Против него сверяется расписание (time_slots) таблиц
        // прохода - предупреждаем, если проход закрыт на границе срока (зеркало VehicleForm).
        entryPeriod: {
            type: Object,
            default: null
        }
    },
    emits: ['edit-cancelled', 'employee-added', 'employee-updated', 'employees-added', 'notices-change'],
    setup(props) {
        const instance = getCurrentInstance()
        const { fieldVisible, fieldRequired } = useFieldConfig(() => props.fieldConfig)

        const { isValid, tooltipMessage, showTooltip } = useFormValidation(() => {
            const vm = instance.proxy

            if (vm.selectedExistingEmployees.length > 0) {
                const existingRules = []
                if (fieldVisible('target_tables') && fieldRequired('target_tables')) {
                    existingRules.push({ check: vm.selectedPassageTables.length > 0, message: 'выберите хотя бы одно место прохода' })
                }
                return existingRules
            }

            // Поле требует заполнения только когда видимо И помечено обязательным.
            // Видимое необязательное (required=false) не должно блокировать submit.
            const rules = []
            if (fieldVisible('last_name') && fieldRequired('last_name')) {
                rules.push({ check: !!vm.lastName.trim(), message: 'фамилию' })
            }
            if (fieldVisible('first_name') && fieldRequired('first_name')) {
                rules.push({ check: !!vm.firstName.trim(), message: 'имя' })
            }
            rules.push({ check: !vm.blacklistInfo, message: 'Человек в чёрном списке' })
            if (fieldVisible('position') && fieldRequired('position')) {
                rules.push({ check: !!vm.position.trim(), message: 'должность' })
            }
            if (fieldVisible('citizenship') && fieldRequired('citizenship')) {
                rules.push({ check: !!vm.selectedCitizenship, message: 'гражданство' })
            }
            if (fieldVisible('passport') && fieldRequired('passport')) {
                rules.push({ check: !!vm.passportSeriesNumber.trim(), message: 'паспортные данные' })
            }
            if (fieldVisible('patent')) {
                rules.push({
                    check: !vm.effectivePatentRequired || !!(vm.patentNumber.trim() || vm.selectedPermission),
                    message: 'номер патента или иное разрешение'
                })
            }
            if (fieldVisible('target_tables') && fieldRequired('target_tables')) {
                rules.push({ check: vm.selectedPassageTables.length > 0, message: 'выберите хотя бы одно место прохода' })
            }
            if (fieldVisible('pd_consent') && fieldRequired('pd_consent')) {
                rules.push({ check: vm.pdConsent, message: 'отметьте согласие работника на обработку персональных данных' })
            }

            return rules
        })

        // Причина блокировки кнопки живёт на hover - на телефоне его нет,
        // поэтому там показываем её сразу под кнопкой.
        const { isNarrow } = useNarrowScreen()
        return { canAddEmployee: isValid, getTooltipMessage: tooltipMessage, showTooltip, fieldVisible, fieldRequired, isNarrow }
    },
    data() {
        return {
            lastName: '',
            firstName: '',
            middleName: '',
            // Отметка о согласии субъекта. Ставится на каждого человека отдельно и
            // сбрасывается после добавления: подтверждают конкретного работника, а не
            // «вообще всех», кого заведут дальше.
            pdConsent: false,
            position: '',
            passportSeriesNumber: '',
            patentNumber: '',
            
            availableCitizenships: [],
            selectedCitizenship: null,
            isCitizenshipDropdownOpen: false,
            citizenshipMenuStyle: null,
            citizenshipMenuUp: false,

            selectedPermission: '',
            isPermissionDropdownOpen: false,
            permissionMenuStyle: null,
            permissionMenuUp: false,
            permissionQuery: '',
            // rAF-хендл пересчёта стороны раскрытия; null - пересчёт не запланирован.
            dropDirectionFrame: null,
            availablePermissions: [
                'Иностранцы с видом на жительство (ВНЖ) или разрешением на временное проживание (РВП)',
                'Беженцы или получившие временное убежище в России',
                'Участники Госпрограммы переселения соотечественников в РФ и члены их семей',
                'Люди с временным удостоверением личности лица без гражданства, выданным в России',
                'Студенты, которые работают в образовательных организациях или хозяйственных обществах и партнёрствах, созданных этими организациями',
                'Студенты, обучающиеся очно в образовательных организациях',
                'Работники посольств и консульств',
                'Аккредитованные журналисты',
                'Специалисты аккредитованных ИТ‑компаний',
                'Специалисты иностранных компаний, которых пригласили для монтажных работ или гарантийно‑сервисного обслуживания оборудования',
                'Сотрудники представительств иностранных организаций',
                'Медики, педагоги, учёные, которые работают на территории международного медицинского кластера',
                'Педагоги и учёные, которых пригласили на работу в образовательные или научные организации',
                'Педагоги и учёные, прибывшие с деловой или гуманитарной целью в образовательные или научные организации, кроме духовных',
                'Творческие работники, учёные и педагоги, прибывшие с гостевым или деловым визитом — до 30 календарных дней',
                'Творческие работники, учёные и педагоги, прибывшие по приглашению госучреждений культуры и искусства для участия в мероприятиях — до 30 календарных дней'
            ],
            
            
            allPassageTables: [],
            attachedPassageTables: [],
            selectedPassageTables: [],
            loadingPassageTables: false,

            errors: {
                passageTables: ''
            },
            
            showExistingEmployeesModal: false,
            selectedExistingEmployees: [],

            editingEmployee: null,

            // Проверка ЧС (#443): null или { is_blacklisted, reason }
            blacklistInfo: null,
            blacklistTimeout: null,
            // Опорный момент для предупреждений окон (#1183 S4): тикает раз в минуту,
            // чтобы баннер релевантных окон не залипал по времени.
            warningNow: new Date(),
            warningTimer: null,
            // Дебаунс эмита предупреждений наверх (#1183 polish).
            noticesTimer: null
        }
    },
    computed: {
        selectedCitizenshipText() {
            return this.selectedCitizenship ? this.selectedCitizenship.name : 'Выберите гражданство';
        },
        isPatentRequired() {
            return this.selectedCitizenship ? this.selectedCitizenship.patent_required : false;
        },
        // Стрелка показывает, куда поедет меню: закрытая - в сторону раскрытия,
        // открытая - в сторону сворачивания. Отсюда исключающее ИЛИ.
        citizenshipArrowUp() {
            return this.citizenshipMenuUp !== this.isCitizenshipDropdownOpen;
        },
        permissionArrowUp() {
            return this.permissionMenuUp !== this.isPermissionDropdownOpen;
        },
        // Список из 16 длинных формулировок читать целиком дорого, поэтому поиск.
        // Варианты запроса общие с остальными клиентскими фильтрами: терпят
        // неверную раскладку и транслит ([[searchVariants]]).
        filteredPermissions() {
            const variants = buildSearchVariants(this.permissionQuery);
            if (variants.length === 0) return this.availablePermissions;
            return this.availablePermissions.filter((permission) => matchesSearch(permission, variants));
        },
        // patent трёхзначен: нет строки конфига -> условная логика по гражданству;
        // required=true -> всегда обязателен; required=false -> снова условная.
        // fieldRequired() тут не годится: при отсутствии строки он даёт true (дефолт),
        // что форсировало бы обязательность вместо условной логики - читаем конфиг напрямую.
        effectivePatentRequired() {
            const cfg = this.fieldConfig && this.fieldConfig['patent']
            if (cfg) {
                return cfg.required ? true : this.isPatentRequired
            }
            return this.isPatentRequired
        },
        patentFieldDisabled() {
            return this.selectedPermission !== '';
        },
        permissionFieldDisabled() {
            return this.patentNumber.trim() !== '';
        },
        attachedTablesIds() {
            return this.attachedPassageTables.map(table => table.id);
        },
        canAddExistingEmployees() {
            return this.selectedExistingEmployees.length > 0 && this.selectedPassageTables.length > 0;
        },
        filteredPassageTables() {
            return this.allPassageTables.filter(item => {
                const table = item.table || item;
                return table && table.table_type === 'people';
            }).map(item => {
                if (item.table) {
                    return item;
                } else {
                    return { table: item };
                }
            });
        },
        // Предупреждения выбранных таблиц прохода (#1183): группа на таблицу со
        // свободным текстом (S1), активными окнами (S4) и отчётом "режим работы против
        // окна пребывания срока" (S5). Реактивно к warningNow/entryPeriod; единая
        // панель в CreateApplication обновляется на лету.
        noticeGroups() {
            const at = this.warningNow;
            const groups = [];
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
        // Дропдаун гражданства закрывают из трёх мест (тоггл, выбор, клик вне),
        // поэтому позиция живёт на одном вотчере, а не в каждом из них.
        // Скролл и ресайз слушает onDropViewportChange - он работает и с закрытым
        // меню, потому что стрелка на кнопке заранее показывает сторону раскрытия.
        isCitizenshipDropdownOpen(open) {
            this.citizenshipMenuStyle = open ? this.buildCitizenshipMenuStyle() : null;
        },
        // То же для разрешений: закрывается из тоггла, выбора и клика вне.
        // Стиль считаем после рендера меню - высота подгоняется по его пунктам.
        isPermissionDropdownOpen(open) {
            if (open) {
                this.$nextTick(() => {
                    this.permissionMenuStyle = this.buildPermissionMenuStyle();
                    // На телефоне фокус поднимает клавиатуру и съедает пол-экрана
                    // вместе с только что раскрытым списком - там ставит курсор сам юзер.
                    if (!this.isNarrow && this.$refs.permissionSearch) {
                        this.$refs.permissionSearch.focus();
                    }
                });
            } else {
                this.permissionMenuStyle = null;
                this.permissionQuery = '';
            }
        },
        // Список укоротился - меню должно ужаться следом, иначе под ним висит пустота.
        permissionQuery() {
            if (!this.isPermissionDropdownOpen) return;
            this.$nextTick(() => {
                this.permissionMenuStyle = this.buildPermissionMenuStyle();
            });
        },
        // Блок разрешения появляется вместе с требованием патента - к этому моменту
        // кнопки ещё не было, и сторону раскрытия никто не считал.
        effectivePatentRequired(required) {
            if (required) this.$nextTick(this.updateDropDirections);
        },
        lastName() { this.checkBlacklist(); },
        firstName() { this.checkBlacklist(); },
        middleName() { this.checkBlacklist(); },
        // Предупреждения наверх в единую панель, дебаунс - гасит дёрганье при быстрой
        // смене таблиц/времени.
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
        // До загрузки справочников: слушатель не зависит от данных, а до его
        // навешивания стрелки не знают сторону раскрытия.
        window.addEventListener('scroll', this.onDropViewportChange, true);
        window.addEventListener('resize', this.onDropViewportChange);

        await Promise.all([
            this.loadCitizenships(),
            this.loadPassageTables()
        ]);

        document.addEventListener('click', this.handleDocumentClick);
        this.warningTimer = setInterval(() => { this.warningNow = new Date(); }, 60000);

        this.updateDropDirections();
    },
    beforeUnmount() {
        if (this.hintTimer) clearTimeout(this.hintTimer);
        this.stopDropDirectionWatch();
        document.removeEventListener('click', this.handleDocumentClick);
        if (this.blacklistTimeout) {
            clearTimeout(this.blacklistTimeout);
        }
        if (this.warningTimer) {
            clearInterval(this.warningTimer);
        }
        if (this.noticesTimer) {
            clearTimeout(this.noticesTimer);
        }
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
            if (!this.isNarrow || this.canAddEmployee) return;
            this.showTooltip = true;
            if (this.hintTimer) clearTimeout(this.hintTimer);
            this.hintTimer = setTimeout(() => { this.showTooltip = false; }, 3000);
        },

        // Закрывает открытые дропдауны при клике вне них. Именованный метод (не
        // анонимная стрелка в mounted) - иначе removeEventListener не снимет слушатель
        // и он копится при частом откр/закр формы в модалке ручного добавления.
        handleDocumentClick(e) {
            if (!e.target.closest('.citizenship__dropdown')) {
                this.isCitizenshipDropdownOpen = false;
            }

            if (!e.target.closest('.permission__dropdown')) {
                this.isPermissionDropdownOpen = false;
            }
        },

        async loadCitizenships() {
            try {
                const response = await apiRequest("/citizenships", {
                    method: "GET"});

                if (response.ok) {
                    this.availableCitizenships = await response.json();
                    const defaultCitizenship = this.availableCitizenships.find(c => c.is_default);
                    this.selectedCitizenship = defaultCitizenship || this.availableCitizenships[0];
                } else {
                    console.error("Ошибка при загрузке гражданств");
                }
            } catch (error) {
                console.error("Ошибка при загрузке гражданств:", error);
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

        // Загружаем все доступные системные таблицы
        const allTablesResponse = await apiRequest("/system-tables", {
            method: "GET"});

        if (allTablesResponse.ok) {
            const tables = await allTablesResponse.json();

            // Нормализуем данные
            this.allPassageTables = tables.map(table => {
                // Если данные уже в правильном формате
                if (table.table) {
                    return table;
                }
                // Если данные в плоском формате
                else {
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
            const errorText = await allTablesResponse.text();
            console.error("Ошибка при загрузке системных таблиц:", errorText);
        }

        // Загружаем привязанные таблицы организации
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

        // Если нет привязанных таблиц организации, пробуем компанию
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
        console.error("Ошибка при загрузке мест прохода:", error);
        this.allPassageTables = [];
        this.attachedPassageTables = [];
    } finally {
        this.loadingPassageTables = false;
    }
},

        formatNameField(fieldName) {
            if (this[fieldName]) {
                this[fieldName] = this.formatName(this[fieldName]);
            }
        },

        formatName(name) {
            if (!name) return '';
            return name.toLowerCase()
                .split(' ')
                .map(word => word.charAt(0).toUpperCase() + word.slice(1))
                .join(' ');
        },

        // Проверка человека в чёрном списке (#443): строгое совпадение ФИО,
        // как и серверный гард. Отчество опционально.
        checkBlacklist() {
            if (this.blacklistTimeout) {
                clearTimeout(this.blacklistTimeout);
            }

            const last = this.lastName.trim();
            const first = this.firstName.trim();
            if (!last || !first) {
                this.blacklistInfo = null;
                return;
            }
            const middle = this.middleName.trim();

            this.blacklistTimeout = setTimeout(async () => {
                try {
                    const res = await checkPersonBlacklist({ last_name: last, first_name: first, middle_name: middle });
                    this.blacklistInfo = res && res.is_blacklisted ? res : null;
                } catch (error) {
                    // Тихо: фоновая проверка не блокирует ввод; серверный гард - бэкстоп.
                    console.error('Ошибка при проверке ЧС человека:', error);
                    this.blacklistInfo = null;
                }
            }, 500);
        },

        validatePassageTables() {
            this.errors.passageTables = this.selectedPassageTables.length === 0 ? '' : '';
        },
        
        formatPassageTables() {
            if (this.selectedPassageTables.length === 0) return '';
            
            const tableNames = this.selectedPassageTables.map(tableId => {
                const table = this.allPassageTables.find(t => t.table.id === tableId);
                return table ? table.table.display_name : '';
            }).filter(name => name);
            
            if (tableNames.length > 1) {
                return tableNames[0] + ' и др.';
            }
            
            return tableNames[0] || '';
        },
        
        addEmployee() {
            if (!this.canAddEmployee) {
                return;
            }
            
            if (this.selectedExistingEmployees.length > 0) {
                this.addExistingEmployees();
                return;
            }
            
            const newEmployee = {
                lastName: this.lastName.trim(),
                firstName: this.firstName.trim(),
                middleName: this.middleName.trim(),
                position: this.position.trim(),
                citizenshipId: this.selectedCitizenship.id,
                citizenshipName: this.selectedCitizenship.name,
                passportSeriesNumber: this.passportSeriesNumber.trim(),
                patentNumber: this.effectivePatentRequired ? this.patentNumber.trim() : null,
                otherPermission: this.effectivePatentRequired ? this.selectedPermission : null,
                passageTables: this.formatPassageTables(),
                targetTables: [...this.selectedPassageTables],
                pdConsent: this.pdConsent,
                isExisting: false
            };

            const duplicate = findDuplicateEmployee(
                this.existingEmployees,
                newEmployee,
                this.editingEmployee ? this.editingEmployee.id : null,
            );
            if (duplicate) {
                useDeletionsStore().notify({
                    prefix: `${employeeLabel(duplicate)} `,
                    bold: 'уже добавлен в список',
                    type: 'error',
                });
                return;
            }

            if (this.editingEmployee) {
                newEmployee.id = this.editingEmployee.id;
                this.$emit('employee-updated', newEmployee);
                this.cancelEdit();
            } else {
                this.$emit('employee-added', newEmployee);
                this.clearEmployeeFormPartial();
            }
        },

        clearEmployeeFormPartial() {
            this.pdConsent = false;
            this.lastName = '';
            this.firstName = '';
            this.middleName = '';
            this.position = '';
            this.passportSeriesNumber = '';
            this.patentNumber = '';
            this.selectedPermission = '';
        },
        
        clearEmployeeForm() {
            this.pdConsent = false;
            this.lastName = '';
            this.firstName = '';
            this.middleName = '';
            this.position = '';
            this.passportSeriesNumber = '';
            this.patentNumber = '';
            this.selectedPermission = '';
            this.selectedPassageTables = [];
            this.errors.passageTables = '';
            this.selectedExistingEmployees = [];
            this.editingEmployee = null;
        },

        openExistingEmployeesModal() {
            this.showExistingEmployeesModal = true;
        },

        closeExistingEmployeesModal() {
            this.showExistingEmployeesModal = false;
        },

        onEmployeesSelected(employees) {
            this.selectedExistingEmployees = employees;
            this.closeExistingEmployeesModal();
            this.clearEmployeeFormPartial();
        },

        addExistingEmployees() {
            if (this.selectedExistingEmployees.length === 0) {
                useDeletionsStore().notify({ bold: 'Выберите сотрудников для добавления', type: 'error' });
                return;
            }

            if (this.selectedPassageTables.length === 0) {
                useDeletionsStore().notify({ bold: 'Выберите места прохода', type: 'error' });
                return;
            }

            const employees = this.selectedExistingEmployees.map(employee => ({
                lastName: employee.last_name,
                firstName: employee.first_name,
                middleName: employee.middle_name,
                position: employee.position,
                citizenshipId: employee.citizenship_id,
                citizenshipName: employee.citizenship_name,
                passportSeriesNumber: employee.passport_series_number,
                patentNumber: employee.patent_number,
                otherPermission: employee.other_permission,
                passageTables: this.formatPassageTables(),
                targetTables: [...this.selectedPassageTables],
                // Сотрудник из реестра: согласие получено при заведении записи, поэтому
                // отметка едет с ним и повторно её не спрашиваем. У записей, заведённых
                // до появления поля, pd_consent_at пустой - тогда согласие подтверждают
                // заново в карточке реестра.
                pdConsent: !!employee.pd_consent_at,
                isExisting: true,
                existingEmployeeId: employee.id
            }));

            // Модалка выбора уже гасит добавленные строки, но выбор мог устареть, а среди
            // записей каталога встречаются двойники по паспорту - отсеиваем здесь ещё раз.
            const list = [...this.existingEmployees];
            const toAdd = [];
            const skipped = [];
            employees.forEach(employee => {
                if (findDuplicateEmployee(list, employee)) {
                    skipped.push(employeeLabel(employee));
                    return;
                }
                list.push(employee);
                toAdd.push(employee);
            });

            if (skipped.length > 0) {
                useDeletionsStore().notify({
                    prefix: `${skipped.join(', ')} `,
                    bold: skipped.length > 1 ? 'уже в списке - пропущены' : 'уже добавлен в список',
                    type: 'error',
                });
            }

            if (toAdd.length === 0) {
                this.clearExistingEmployeesSelection();
                return;
            }

            this.$emit('employees-added', toAdd);
            this.clearExistingEmployeesSelection();
        },

        clearExistingEmployeesSelection() {
            this.selectedExistingEmployees = [];
        },

        editEmployee(employee) {
            this.editingEmployee = employee;
            this.selectedExistingEmployees = [];
            // Отметку согласия возвращаем в форму: правка фамилии не должна её терять.
            this.pdConsent = employee.pdConsent === true;
            
            if (employee.isExisting) {
                this.lastName = employee.lastName;
                this.firstName = employee.firstName;
                this.middleName = employee.middleName;
                this.position = employee.position;
                this.passportSeriesNumber = employee.passportSeriesNumber;
                this.patentNumber = employee.patentNumber;
                this.selectedPermission = employee.otherPermission;
                this.selectedPassageTables = employee.targetTables || [];
                
                if (employee.citizenshipId) {
                    const citizenship = this.availableCitizenships.find(c => c.id === employee.citizenshipId);
                    if (citizenship) {
                        this.selectedCitizenship = citizenship;
                    }
                }
            } else {
                this.lastName = employee.lastName;
                this.firstName = employee.firstName;
                this.middleName = employee.middleName;
                this.position = employee.position;
                this.passportSeriesNumber = employee.passportSeriesNumber;
                this.patentNumber = employee.patentNumber;
                this.selectedPermission = employee.otherPermission;
                this.selectedPassageTables = employee.targetTables || [];
                
                if (employee.citizenshipId) {
                    const citizenship = this.availableCitizenships.find(c => c.id === employee.citizenshipId);
                    if (citizenship) {
                        this.selectedCitizenship = citizenship;
                    }
                }
            }
        },

        cancelEdit() {
            this.$emit('edit-cancelled');
            resetEmployeeFormState(this);
        },
        
        toggleCitizenshipDropdown() {
            this.isCitizenshipDropdownOpen = !this.isCitizenshipDropdownOpen;
        },

        /**
         * Список гражданств длинный: снизу его обрезает край экрана, поэтому
         * высоту ограничиваем доступным местом, а если снизу тесно - раскрываем
         * вверх. Формула зеркалит BaseDropdown.updateMenuPosition: rect отдаёт
         * device-px под корневым zoom, innerHeight - незумленную высоту, поэтому
         * к layout-px приводятся ОБА (при zoom=1 деление ничего не меняет).
         */
        buildCitizenshipMenuStyle() {
            const btn = this.$refs.citizenshipButton;
            if (!btn || typeof window === 'undefined') return null;
            const space = this.measureDropSpace(btn);
            if (!space) return null;
            const flipUp = space.flipUp;
            this.citizenshipMenuUp = flipUp;
            const maxHeight = Math.max(160, Math.min(300, (flipUp ? space.above : space.below) - 16));
            if (!flipUp) {
                return { maxHeight: maxHeight + 'px' };
            }
            // top:'auto' обязателен - иначе базовое .dropdown__menu{top:100%}
            // остаётся в силе и конфликтует с bottom.
            return {
                top: 'auto',
                bottom: '100%',
                marginTop: '0',
                marginBottom: '5px',
                maxHeight: maxHeight + 'px'
            };
        },

        selectCitizenship(citizenship) {
            this.selectedCitizenship = citizenship;
            this.isCitizenshipDropdownOpen = false;
            if (!citizenship.patent_required) {
                this.patentNumber = '';
                this.selectedPermission = '';
            }
        },
        
        togglePermissionDropdown() {
            this.isPermissionDropdownOpen = !this.isPermissionDropdownOpen;
        },
        
        /**
         * Формула та же, что в buildCitizenshipMenuStyle, с двумя добавками.
         * Высота режется по границе пункта: формулировки разрешений занимают
         * две-три строки, и обрезанная посередине строка читается как баг вёрстки.
         * Границы берутся по прокручиваемому предку: форму встраивают и в тело
         * модалки ручного добавления, где меню обрезает её край, а не край экрана.
         */
        buildPermissionMenuStyle() {
            const dropdown = this.$refs.permissionDropdown;
            if (!dropdown || typeof window === 'undefined') return null;
            const space = this.measureDropSpace(dropdown);
            if (!space) return null;
            const flipUp = space.flipUp;
            this.permissionMenuUp = flipUp;
            const available = Math.max(160, Math.min(300, (flipUp ? space.above : space.below) - 16));
            const maxHeight = this.fitPermissionMenuToItems(available);
            if (!flipUp) {
                return { maxHeight: maxHeight + 'px' };
            }
            // top:'auto' обязателен - иначе базовое .permission__dropdown-menu{top:100%}
            // остаётся в силе и конфликтует с bottom.
            return {
                top: 'auto',
                bottom: '100%',
                marginTop: '0',
                marginBottom: '5px',
                maxHeight: maxHeight + 'px'
            };
        },

        /** Наибольшая высота меню, при которой последний видимый пункт помещается целиком. */
        fitPermissionMenuToItems(limit) {
            const menu = this.$refs.permissionDropdown.querySelector('.permission__dropdown-menu');
            if (!menu) return limit;
            // offsetHeight, а не getBoundingClientRect: во время анимации открытия rect врёт.
            const borders = 2;
            // Строка поиска не прокручивается вместе со списком, её высота - фикс. расход.
            const search = menu.querySelector('.permission__dropdown-search');
            const searchHeight = search ? search.offsetHeight : 0;
            const listLimit = limit - searchHeight - borders;

            // Строка "Ничего не найдено" занимает место наравне с пунктами, иначе
            // список окажется на её высоту короче и заведёт полосу прокрутки.
            let fitted = 0;
            for (const row of menu.querySelectorAll('.permission__dropdown-item, .permission__dropdown-empty')) {
                if (fitted + row.offsetHeight > listLimit) break;
                fitted += row.offsetHeight;
            }
            if (fitted === 0) return limit;

            // offsetHeight округляет дробную высоту вниз, и списку не хватает
            // полупикселя - появляется полоса прокрутки на ровном месте.
            const subpixelSlack = 1;
            return Math.min(limit, searchHeight + fitted + borders + subpixelSlack);
        },

        /**
         * Свободное место над и под якорем в layout-px и сторона раскрытия.
         * rect отдаёт device-px под корневым zoom, innerHeight - незумленную высоту,
         * поэтому к layout-px приводятся ОБА (при zoom=1 деление ничего не меняет).
         */
        measureDropSpace(anchor) {
            if (!anchor || typeof window === 'undefined') return null;
            const zoom = getViewportZoom() || 1;
            const rect = anchor.getBoundingClientRect();
            const bounds = this.dropBounds(anchor);
            const below = (bounds.bottom - rect.bottom) / zoom;
            const above = (rect.top - bounds.top) / zoom;
            return { below, above, flipUp: below < 200 && above > below };
        },

        /** Границы, за которые меню выходить нельзя: экран и прокручиваемые предки. */
        dropBounds(anchor) {
            let top = 0;
            let bottom = window.innerHeight;

            for (let el = anchor.parentElement; el && el !== document.body; el = el.parentElement) {
                const overflowY = getComputedStyle(el).overflowY;
                if (overflowY === 'auto' || overflowY === 'scroll' || overflowY === 'hidden') {
                    const rect = el.getBoundingClientRect();
                    top = Math.max(top, rect.top);
                    bottom = Math.min(bottom, rect.bottom);
                }
            }

            return { top, bottom };
        },

        /**
         * Стрелка на закрытой кнопке показывает, куда меню раскроется, поэтому
         * сторону надо знать и до открытия: скролл страницы меняет её на ходу.
         * Через requestAnimationFrame - обход предков дорог для каждого события скролла.
         */
        onDropViewportChange() {
            if (this.dropDirectionFrame) return;
            this.dropDirectionFrame = requestAnimationFrame(() => {
                this.dropDirectionFrame = null;
                this.updateDropDirections();
                if (this.isCitizenshipDropdownOpen) {
                    this.citizenshipMenuStyle = this.buildCitizenshipMenuStyle();
                }
                if (this.isPermissionDropdownOpen) {
                    this.permissionMenuStyle = this.buildPermissionMenuStyle();
                }
            });
        },

        updateDropDirections() {
            const citizenship = this.measureDropSpace(this.$refs.citizenshipButton);
            if (citizenship) this.citizenshipMenuUp = citizenship.flipUp;

            const permission = this.measureDropSpace(this.$refs.permissionDropdown);
            if (permission) this.permissionMenuUp = permission.flipUp;
        },

        stopDropDirectionWatch() {
            window.removeEventListener('scroll', this.onDropViewportChange, true);
            window.removeEventListener('resize', this.onDropViewportChange);
            if (this.dropDirectionFrame) {
                cancelAnimationFrame(this.dropDirectionFrame);
                this.dropDirectionFrame = null;
            }
        },

        selectPermission(permission) {
            this.selectedPermission = permission;
            this.isPermissionDropdownOpen = false;
            if (permission) {
                this.patentNumber = '';
            }
        },

        /** Enter в поиске выбирает вариант, только когда он остался один - иначе гадание. */
        selectOnlyFoundPermission() {
            if (this.filteredPermissions.length === 1) {
                this.selectPermission(this.filteredPermissions[0]);
            }
        },

        handlePatentInput() {
            if (this.patentNumber.trim() !== '') {
                this.selectedPermission = '';
            }
        },

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

.completion__citizenship {
    display: flex;
    flex-direction: column;
    gap: 10px;
    position: relative;
    padding-bottom: 15px;
}

.citizenship__header {
    display: flex;
    justify-content: space-between;
    align-items: end;
}

.citizenship__label {
    order: -1;
    font-size: 13px;
    color: var(--text-muted);
}

.citizenship-actions {
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
    border: none;
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

/* Tooltip styles */
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

.citizenship__dropdown {
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
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 380px;
    display: block;
}

/* Иконка нарисована остриём вправо, отсюда базовые 90deg (вниз). Вверх - это 270deg,
   а не -90deg: поворот идёт дальше по кругу, а не отматывается назад через исходное
   положение иконки, где она на миг встаёт боком. */
.button__arrow {
    width: 10px;
    height: 10px;
    transition: transform 0.2s ease-out;
    transform: rotate(90deg);
    flex-shrink: 0;
}

.button__arrow--up {
    transform: rotate(270deg);
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
    justify-content: space-between;
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

.patent-required-badge {
    background: var(--danger-bg);
    color: var(--danger-text);
    padding: 2px 6px;
    border-radius: 8px;
    font-size: 10px;
    font-weight: 500;
}

.completion__consent {
    margin-top: 10px;
}

.consent-option {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    font-size: 11px;
    line-height: 1.3;
    cursor: pointer;
}

.consent-option input[type="checkbox"] {
    margin-top: 2px;
    flex-shrink: 0;
}

.completion__fields {
    display: flex;
    flex-direction: column;
    gap: 15px;
    margin-bottom: 15px;
}

.completion__name-row {
    display: flex;
    gap: 20px;
}

.completion__last-name,
.completion__first-name,
.completion__middle-name,
.completion__position,
.completion__passport,
.completion__patent {
    flex: 1;
}

.completion__last-name-header,
.completion__first-name-header,
.completion__middle-name-header,
.completion__position-header,
.completion__passport-header,
.completion__patent-header,
.completion__permission-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 5px;
}

.name__input {
    width: 100%;
    height: 40px;
    border: 1px solid var(--border);
    border-radius: 15px;
    padding: 0 15px;
    outline: none;
    font-size: 14px;
    background: var(--surface);
}

.name__input:focus {
    border-color: var(--accent);
}

.name__input:disabled {
    background-color: var(--surface-2);
    cursor: not-allowed;
}

/* Disabled field styles */
.disabled-field {
    opacity: 0.5;
}

.disabled-field .name__input,
.disabled-field .permission__dropdown-button {
    background-color: var(--surface-2);
    cursor: not-allowed;
}

/* Permission dropdown styles */
.completion__permission {
    width: 100%;
}

.permission__dropdown {
    width: 100%;
    position: relative;
}

.permission__dropdown-button {
    width: 100%;
    min-height: 40px;
    border: 1px solid var(--border);
    background-color: var(--surface);
    border-radius: 15px;
    outline: none;
    cursor: pointer;
    padding: 8px 15px;
    text-align: left;
    transition: border-color 0.2s;
}

.permission__dropdown-button:hover:not(:disabled) {
    border-color: var(--accent);
}

.permission__dropdown-button:disabled {
    background-color: var(--surface-2);
    cursor: not-allowed;
    opacity: 0.6;
}

.permission__button-content {
    display: flex;
    align-items: center;
    width: 100%;
    justify-content: space-between;
    gap: 10px;
}

/* Формулировка разрешения длинная: даём ей всю ширину поля и две строки,
   дальше многоточие (полный текст - в title кнопки и в самом меню). */
.permission__button-text {
    flex: 1;
    min-width: 0;
    font-size: 14px;
    color: var(--text);
    line-height: 1.3;
    text-align: left;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
}

.permission__dropdown-button--placeholder .permission__button-text {
    color: var(--text-muted);
}

.permission__button-arrow {
    width: 10px;
    height: 10px;
    transition: transform 0.2s ease-out;
    transform: rotate(90deg);
    flex-shrink: 0;
}

.permission__button-arrow--up {
    transform: rotate(270deg);
}

.permission__dropdown-menu {
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
    /* Рабочий потолок ставит buildPermissionMenuStyle() по месту на экране,
       здесь - запас на случай, если меню открыли до расчёта. */
    max-height: 300px;
    /* Колонка со скроллом на списке, а не на меню: строка поиска остаётся на месте,
       а inline max-height ужимает список, а не режет его через overflow. */
    display: flex;
    flex-direction: column;
    overflow: hidden;
}

.permission__dropdown-search {
    padding: 8px 12px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
}

.permission__search-input {
    width: 100%;
    padding: 6px 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface);
    color: var(--text);
    font-size: 13px;
    outline: none;
    transition: border-color 0.2s;
    box-sizing: border-box;
}

.permission__search-input:focus {
    border-color: var(--accent);
}

.permission__dropdown-list {
    overflow-y: auto;
    overscroll-behavior: contain;
    flex: 1 1 auto;
    min-height: 0;
}

.permission__dropdown-empty {
    padding: 12px 15px;
    font-size: 13px;
    color: var(--text-muted);
    text-align: center;
}

.permission__dropdown-item {
    padding: 10px 15px;
    cursor: pointer;
    transition: background-color 0.2s;
    border-bottom: 1px solid var(--surface-2);
}

.permission__dropdown-item:hover {
    background-color: var(--surface-2);
}

.permission__dropdown-item--selected {
    background-color: var(--accent-tint);
}

.permission__dropdown-item--selected .permission__item-text {
    color: var(--accent-text);
}

.permission__dropdown-item:last-child {
    border-bottom: none;
}

.permission__item-text {
    display: block;
    font-size: 13px;
    line-height: 1.35;
    color: var(--text);
    overflow-wrap: break-word;
}

.permission__item-text--empty {
    color: var(--text-muted);
}

/* File upload styles */

/* Passage tables styles */
.completion__passage {
    margin-top: 15px;
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

/* Стили для существующих сотрудников */
.existing-employees-info {
    margin-bottom: 15px;
    padding: 10px;
    background: var(--surface-2);
    border-radius: 10px;
    border: 1px solid var(--border);
}

.existing-employees-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.existing-employees-count {
    font-size: 14px;
    font-weight: 500;
    color: var(--text);
}

.existing-employees-actions {
    display: flex;
    gap: 10px;
}

.view-employees-btn {
    background: var(--surface);
    color: var(--accent-text);
    border: 1px solid var(--accent);
    border-radius: 15px;
    padding: 5px 10px;
    font-size: 11px;
    cursor: pointer;
}

.view-employees-btn:hover {
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

.dropdown-enter-active,
.dropdown-leave-active {
    transition: all 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
    opacity: 0;
    transform: translateY(-10px);
}

.blacklist-warning {
    width: 100%;
    margin-top: 12px;
    padding: 12px;
    background: var(--danger-bg);
    border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
    border-radius: 10px;
}

.blacklist-warning .warning-title {
    font-weight: 600;
    color: var(--danger-text);
    margin: 0 0 5px 0;
    font-size: 14px;
}

.blacklist-warning .warning-details {
    color: var(--danger-text);
    margin: 0;
    font-size: 12px;
    line-height: 1.5;
}

/* Форма (450px) + список сотрудников рядом не влезают на планшете - стекаем в
   колонку (form__data в CreateApplication.vue делает то же на этом же брейкпоинте). */
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

    /* Кнопка «Добавить» - внизу формы, куда пользователь приходит, заполнив поля.
       contents у двух обёрток выводит строку кнопок в прямые дети формы, флекс с
       order отправляет её в конец; лейбл «Гражданство» остаётся над дропдауном. */
    /* Места разгрузки и проезд - сиблинги этой обёртки, поэтому флекс с order
       живёт на всей форме, а обёртка разворачивается. */
    .data__completion {
        display: flex;
        flex-direction: column;
    }

    .completion__body {
        display: contents;
    }

    .completion__citizenship,
    .citizenship__header {
        display: contents;
    }

    .citizenship-actions {
        order: 999;
        justify-content: stretch;
        margin-top: 4px;
        padding: 4px 0;
    }

    .citizenship-actions .add-button {
        flex: 1;
        min-height: 44px;
    }

    .citizenship-actions .cancel-edit-btn {
        min-height: 44px;
    }

    /* Компенсация потерянных отступов contents-обёрток. */
    /* order:-1 нужен только десктопной строке шапки: во флексе всей формы он
       утаскивал лейбл в самое начало. */
    .citizenship__label {
        order: 0;
        display: block;
        margin-bottom: 6px;
    }

    .citizenship__dropdown {
        margin-bottom: 15px;
    }

    /* Заголовок блока и кнопка «Добавить сущ.» - строго одна строка. nowrap, а не
       перенос: флекс решает про перенос по НАТУРАЛЬНОЙ ширине элемента, до
       flex-shrink, поэтому сжимаемый заголовок кнопку в строке не удерживал -
       «Новый сотрудник» (182px при 18.72px по умолчанию) плюс кнопка 124px требовали
       316px при 308 доступных на 360, и кнопка падала под заголовок.
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

    .tooltip-content {
        max-width: 100%;
        white-space: pre-line;
    }

    .tooltip-content {
        min-width: 0;
        max-width: calc(100vw - 40px);
    }

    /* Ряды полей вставали в колонку только с 480 - на 481-768 ФИО и должность
       ещё делили строку пополам. */
    .completion__name-row {
        flex-direction: column;
        gap: 15px;
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
}

/* Узкие телефоны: на 320 ряду шапки формы остаётся 268px, а «Новый сотрудник» 15px
   вместе с кнопкой требуют 278. Кегль как у подписи соседнего списка (14px) и более
   плотные поля кнопки дают 262 - заголовок читается целиком, без многоточия. */
@media (max-width: 480px) {
    .completion__header h3 {
        font-size: 14px;
    }

    .completion__button {
        padding: 0 12px;
    }
}
</style>
