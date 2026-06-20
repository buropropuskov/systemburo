<template>
  <div
    class="data__completion"
    :class="{ 'data__completion--locked': disabled }"
    :inert="disabled ? '' : undefined"
  >
    <div class="completion__header">
      <h3>Новый сотрудник</h3>
      <button
        class="completion__button"
        @click="openExistingEmployeesModal"
      >
        Добавить существующего(-их)
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

    <div v-else>
      <div
        v-if="fieldVisible('citizenship')"
        class="completion__citizenship"
      >
        <div class="citizenship__header">
          <label class="citizenship__label">Гражданство <span
            v-if="fieldRequired('citizenship')"
            class="required"
          >*</span></label>
          <div class="citizenship-actions">
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
        </div>
        <div class="citizenship__dropdown">
          <button 
            class="dropdown__button" 
            :disabled="editingEmployee && editingEmployee.isExisting"
            @click="toggleCitizenshipDropdown"
          >
            <div class="button__content">
              <span class="button__text">{{ selectedCitizenshipText }}</span>
              <img
                src="@/assets/icons/arrow.png"
                class="button__arrow"
                :class="{ 'button__arrow--open': isCitizenshipDropdownOpen }"
              >
            </div>
          </button>
          <transition name="dropdown">
            <div
              v-if="isCitizenshipDropdownOpen"
              class="dropdown__menu"
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
          <div class="permission__dropdown">
            <button
              class="permission__dropdown-button"
              :disabled="!effectivePatentRequired || permissionFieldDisabled || (editingEmployee && editingEmployee.isExisting)"
              @click="togglePermissionDropdown"
            >
              <div class="permission__button-content">
                <span class="permission__button-text">{{ selectedPermission || (effectivePatentRequired ? 'Выберите разрешение' : 'Не требуется') }}</span>
                <img
                  src="@/assets/icons/arrow.png"
                  class="permission__button-arrow"
                  :class="{ 'permission__button-arrow--open': isPermissionDropdownOpen }"
                >
              </div>
            </button>
            <transition name="dropdown">
              <div
                v-if="isPermissionDropdownOpen"
                class="permission__dropdown-menu"
              >
                <div
                  v-for="permission in availablePermissions"
                  :key="permission"
                  class="permission__dropdown-item"
                  @click="selectPermission(permission)"
                >
                  <span class="permission__item-text">{{ permission }}</span>
                </div>
              </div>
            </transition>
          </div>
        </div>

        <div
          v-if="fieldVisible('patent') && effectivePatentRequired"
          class="completion__files"
        >
          <div class="completion__files-header">
            <label class="input__label">Фото, скан документа(-ов), подтверждающее иное разрешение на работы</label>
          </div>
          <div class="files__upload">
            <input 
              ref="fileInput" 
              type="file"
              multiple
              accept="image/*,.pdf,.doc,.docx,.xlsx,.xls"
              class="file-input"
              :disabled="editingEmployee && editingEmployee.isExisting"
              @change="handleFileUpload"
            >
            <button
              class="upload-button"
              :disabled="editingEmployee && editingEmployee.isExisting"
              @click="triggerFileInput"
            >
              Загрузить
            </button>
          </div>
          <div
            v-if="uploadedFiles.length > 0"
            class="uploaded-files"
          >
            <div
              v-for="(file, index) in uploadedFiles"
              :key="index"
              class="uploaded-file"
            >
              <div class="file-preview">
                <img
                  v-if="file.type === 'image'"
                  :src="file.preview"
                  class="file-preview-image"
                >
                <img
                  v-else
                  :src="getFileIcon(file.extension)"
                  class="file-icon"
                >
              </div>
              <span class="file-name">{{ file.name }}</span>
              <button
                class="remove-file-btn"
                :disabled="editingEmployee && editingEmployee.isExisting"
                @click="removeFile(index)"
              >
                ×
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div
      v-if="fieldVisible('target_tables')"
      class="completion__passage"
    >
      <label class="input__label">Места прохода (целевые таблицы) <span
        v-if="fieldRequired('target_tables')"
        class="required"
      >*</span></label>
      <div
        v-if="!loadingPassageTables && filteredPassageTables.length > 0"
        class="passage__grid"
      >
        <div 
          v-for="table in filteredPassageTables" 
          :key="table.table.id"
          class="passage__item"
          :class="{ 
            'passage__item--active': selectedPassageTables.includes(table.table.id) && table.table.status === 'active',
            'passage__item--attached': attachedTablesIds.includes(table.table.id),
            'passage__item--inactive': table.table.status !== 'active'
          }"
          @click="togglePassageTable(table)"
          @mouseenter="showTableTooltip(table, $event)"
          @mouseleave="hideTableTooltip"
        >
          {{ table.table.display_name }}
        </div>
      </div>
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

    <div
      v-if="tableTooltip.visible" 
      class="inactive-tooltip"
      :style="{ top: tableTooltip.y + 'px', left: tableTooltip.x + 'px' }"
    >
      <div class="inactive-tooltip-content">
        {{ tableTooltip.text }}
      </div>
    </div>

    <ExistingEmployeesModal
      :visible="showExistingEmployeesModal"
      :already-added-employees="existingEmployees"
      :user-organization-id="userOrganizationId"
      :initial-selected-employees="selectedExistingEmployees"
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
import { useFormValidation } from '@/composables/useFormValidation'
import { useFieldConfig } from '@/composables/useFieldConfig'
import { getCurrentInstance } from 'vue'

export default {
    name: 'EmployeeForm',
    components: {
        ExistingEmployeesModal
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
        }
    },
    emits: ['edit-cancelled', 'employee-added', 'employee-updated', 'employees-added'],
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

            return rules
        })

        return { canAddEmployee: isValid, getTooltipMessage: tooltipMessage, showTooltip, fieldVisible, fieldRequired }
    },
    data() {
        return {
            lastName: '',
            firstName: '',
            middleName: '',
            position: '',
            passportSeriesNumber: '',
            patentNumber: '',
            
            availableCitizenships: [],
            selectedCitizenship: null,
            isCitizenshipDropdownOpen: false,
            
            selectedPermission: '',
            isPermissionDropdownOpen: false,
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
            
            uploadedFiles: [],
            
            allPassageTables: [],
            attachedPassageTables: [],
            selectedPassageTables: [],
            loadingPassageTables: false,
            autoPlacesNotified: sessionStorage.getItem('autoPlacesNotified') === 'true',
            
            errors: {
                passageTables: ''
            },
            
            showExistingEmployeesModal: false,
            selectedExistingEmployees: [],

            editingEmployee: null,

            tableTooltip: {
                visible: false,
                text: '',
                x: 0,
                y: 0
            },
            // Проверка ЧС (#443): null или { is_blacklisted, reason }
            blacklistInfo: null,
            blacklistTimeout: null
        }
    },
    computed: {
        selectedCitizenshipText() {
            return this.selectedCitizenship ? this.selectedCitizenship.name : 'Выберите гражданство';
        },
        isPatentRequired() {
            return this.selectedCitizenship ? this.selectedCitizenship.patent_required : false;
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
        }
    },
    watch: {
        lastName() { this.checkBlacklist(); },
        firstName() { this.checkBlacklist(); },
        middleName() { this.checkBlacklist(); }
    },
    async mounted() {
        await Promise.all([
            this.loadCitizenships(),
            this.loadPassageTables()
        ]);

        document.addEventListener('click', (e) => {
            if (!e.target.closest('.citizenship__dropdown')) {
                this.isCitizenshipDropdownOpen = false;
            }
            
            if (!e.target.closest('.permission__dropdown')) {
                this.isPermissionDropdownOpen = false;
            }
        });
    },
    beforeUnmount() {
        if (this.blacklistTimeout) {
            clearTimeout(this.blacklistTimeout);
        }
    },
    methods: {
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
            console.log('RAW DATA from /system-tables:', JSON.stringify(tables, null, 2));
            
            if (tables && tables.length > 0) {
    console.log('First table structure:', tables[0]);
    console.log('Has table field:', 'table' in tables[0]);
    console.log('Has id field directly:', 'id' in tables[0]);
}
            
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
            
            console.log('NORMALIZED allPassageTables:', this.allPassageTables);
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
                console.log('Organization tables:', orgTables);
                
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
                
                const activeAttachedTables = this.attachedPassageTables.filter(table => table.table.status === 'active');
                this.selectedPassageTables = activeAttachedTables.map(table => table.table.id);
                if (this.selectedPassageTables.length > 0 && !this.autoPlacesNotified) {
                    this.autoPlacesNotified = true;
                    sessionStorage.setItem('autoPlacesNotified', 'true');
                    useDeletionsStore().notify({ prefix: 'Места прохода выбраны автоматически для вашей', bold: ' организации' });
                }
            }
        }

        // Если нет привязанных таблиц организации, пробуем компанию
        if (this.attachedPassageTables.length === 0 && this.userCompanyId) {
            const companyTablesResponse = await apiRequest(`/companies/${this.userCompanyId}/tables`, {
                method: "GET"});

            if (companyTablesResponse.ok) {
                const companyTables = await companyTablesResponse.json();
                console.log('Company tables:', companyTables);
                
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
                
                const activeAttachedTables = this.attachedPassageTables.filter(table => table.table.status === 'active');
                this.selectedPassageTables = activeAttachedTables.map(table => table.table.id);
                if (this.selectedPassageTables.length > 0 && !this.autoPlacesNotified) {
                    this.autoPlacesNotified = true;
                    sessionStorage.setItem('autoPlacesNotified', 'true');
                    useDeletionsStore().notify({ prefix: 'Места прохода выбраны автоматически для вашей', bold: ' компании' });
                }
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

        getTableTooltip(table) {
            if (table.table.status !== 'active') {
                if (table.table.status_comment) {
                    return `Недоступно: ${table.table.status_comment}`;
                }
                return 'Недоступно';
            }
            return '';
        },

        showTableTooltip(table, event) {
            if (table.table.status !== 'active') {
                const tooltipText = table.table.status_comment 
                    ? `Недоступно: ${table.table.status_comment}`
                    : 'Недоступно';
                
                this.tableTooltip.text = tooltipText;
                this.tableTooltip.visible = true;
                
                this.$nextTick(() => {
                    const rect = event.target.getBoundingClientRect();
                    this.tableTooltip.x = rect.left + rect.width / 2;
                    this.tableTooltip.y = rect.top - 10;
                });
            }
        },

        hideTableTooltip() {
            this.tableTooltip.visible = false;
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

        togglePassageTable(table) {
            if (table.table.status !== 'active') {
                return;
            }
            
            const index = this.selectedPassageTables.indexOf(table.table.id);
            if (index > -1) {
                this.selectedPassageTables.splice(index, 1);
            } else {
                this.selectedPassageTables.push(table.table.id);
            }
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
                isExisting: false
            };
            
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
            this.lastName = '';
            this.firstName = '';
            this.middleName = '';
            this.position = '';
            this.passportSeriesNumber = '';
            this.patentNumber = '';
            this.selectedPermission = '';
            this.uploadedFiles = [];
        },
        
        clearEmployeeForm() {
            this.lastName = '';
            this.firstName = '';
            this.middleName = '';
            this.position = '';
            this.passportSeriesNumber = '';
            this.patentNumber = '';
            this.selectedPermission = '';
            this.selectedPassageTables = [];
            this.uploadedFiles = [];
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
                isExisting: true,
                existingEmployeeId: employee.id
            }));
            
            this.$emit('employees-added', employees);
            this.clearExistingEmployeesSelection();
        },

        clearExistingEmployeesSelection() {
            this.selectedExistingEmployees = [];
        },

        editEmployee(employee) {
            this.editingEmployee = employee;
            this.selectedExistingEmployees = [];
            
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
            this.editingEmployee = null;
            this.clearEmployeeForm();
        },
        
        toggleCitizenshipDropdown() {
            this.isCitizenshipDropdownOpen = !this.isCitizenshipDropdownOpen;
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
        
        selectPermission(permission) {
            this.selectedPermission = permission;
            this.isPermissionDropdownOpen = false;
            this.patentNumber = '';
        },

        handlePatentInput() {
            if (this.patentNumber.trim() !== '') {
                this.selectedPermission = '';
            }
        },

        triggerFileInput() {
            this.$refs.fileInput.click();
        },

        handleFileUpload(event) {
            const files = Array.from(event.target.files);
            files.forEach(file => {
                const fileExtension = file.name.split('.').pop().toLowerCase();
                const fileType = file.type.startsWith('image/') ? 'image' : 'document';
                const fileData = {
                    name: file.name,
                    file: file,
                    type: fileType,
                    extension: fileExtension
                };

                if (fileType === 'image') {
                    const reader = new FileReader();
                    reader.onload = (e) => {
                        fileData.preview = e.target.result;
                        this.uploadedFiles.push(fileData);
                    };
                    reader.readAsDataURL(file);
                } else {
                    this.uploadedFiles.push(fileData);
                }
            });
            
            event.target.value = '';
        },

        getFileIcon(extension) {
            const icons = import.meta.glob('@/assets/icons/*.png', { eager: true, import: 'default' });
            const iconMap = {
                'pdf': icons['/src/assets/icons/pdf.png'],
                'doc': icons['/src/assets/icons/doc.png'],
                'docx': icons['/src/assets/icons/doc.png'],
                'xlsx': icons['/src/assets/icons/xlsx.png'],
                'xls': icons['/src/assets/icons/xlsx.png'],
            };
            return iconMap[extension] || icons['/src/assets/icons/document.png'];
        },

        removeFile(index) {
            this.uploadedFiles.splice(index, 1);
        }
    }
}
</script>
<style scoped>
.input__label {
    font-size: 13px;
    color: #a2a2a2;
}

.required {
    color: #ff4444;
}

.data__completion {
    padding: 15px;
    width: 450px;
    border-right: 1px solid #e6e6e6;
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
    font-size: 13px;
    color: #a2a2a2;
}

.citizenship-actions {
    display: flex;
    gap: 10px;
    align-items: center;
    position: relative;
}

.cancel-edit-btn {
    background: #f8f8f8;
    color: #333;
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    padding: 8px 15px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.cancel-edit-btn:hover {
    background: #e8e8e8;
}

.add-button {
    background: #4F5BDF;
    color: white;
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
    background: #3a45c0;
}

.add-button:disabled {
    background: #a2a2a2;
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
    background: #333;
    color: white;
    padding: 10px 12px;
    border-radius: 8px;
    font-size: 12px;
    max-width: 420px;
    min-width: 420px;
    box-shadow: 0 2px 8px rgba(0,0,0,0.2);
}

.tooltip-content::before {
    content: '';
    position: absolute;
    bottom: 100%;
    right: 40px;
    border: 5px solid transparent;
    border-bottom-color: #333;
}

.citizenship__dropdown {
    position: relative;
}

.dropdown__button {
    width: 100%;
    height: 30px;
    border: 1px solid #e6e6e6;
    background-color: #FFF;
    border-radius: 50px;
    outline: none;
    cursor: pointer;
    padding: 0 15px;
    transition: border-color 0.2s;
}

.dropdown__button:hover:not(:disabled) {
    border-color: #4F5BDF;
}

.dropdown__button:disabled {
    background-color: #f5f5f5;
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
    color: #000;
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 380px;
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
    background: #FFF;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    margin-top: 5px;
    box-shadow: 0 3px 10px rgba(0,0,0,0.1);
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
    background-color: #f5f5f5;
}

.dropdown__item:first-child {
    border-radius: 10px 10px 0 0;
}

.dropdown__item:last-child {
    border-radius: 0 0 10px 10px;
}

.item__text {
    font-size: 13px;
    color: #333;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.patent-required-badge {
    background: #ffebee;
    color: #c62828;
    padding: 2px 6px;
    border-radius: 8px;
    font-size: 10px;
    font-weight: 500;
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
.completion__permission-header,
.completion__files-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 5px;
}

.name__input {
    width: 100%;
    height: 40px;
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    padding: 0 15px;
    outline: none;
    font-size: 14px;
    background: #FFF;
}

.name__input:focus {
    border-color: #4F5BDF;
}

.name__input:disabled {
    background-color: #f5f5f5;
    cursor: not-allowed;
}

/* Disabled field styles */
.disabled-field {
    opacity: 0.5;
}

.disabled-field .name__input,
.disabled-field .permission__dropdown-button {
    background-color: #f5f5f5;
    cursor: not-allowed;
}

/* Permission dropdown styles */
.completion__permission {
    width: 100%;
}

.permission__dropdown {
    width: 100%;
    height: 40px;
    position: relative;
}

.permission__dropdown-button {
    width: 100%;
    height: 100%;
    border: 1px solid #e6e6e6;
    background-color: #FFF;
    border-radius: 15px;
    outline: none;
    cursor: pointer;
    padding: 0 15px;
    transition: border-color 0.2s;
}

.permission__dropdown-button:hover:not(:disabled) {
    border-color: #4F5BDF;
}

.permission__dropdown-button:disabled {
    background-color: #f5f5f5;
    cursor: not-allowed;
    opacity: 0.6;
}

.permission__button-content {
    display: flex;
    align-items: center;
    width: 100%;
    height: 100%;
    justify-content: space-between;
}

.permission__button-text {
    font-size: 14px;
    color: #000;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 150px;
    display: block;
}

.permission__button-arrow {
    width: 10px;
    height: 10px;
    transition: transform 0.2s;
    transform: rotate(90deg);
    flex-shrink: 0;
}

.permission__button-arrow--open {
    transform: rotate(-90deg);
}

.permission__dropdown-menu {
    position: absolute;
    top: 100%;
    left: 0;
    width: 100%;
    background: #FFF;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    margin-top: 5px;
    box-shadow: 0 3px 10px rgba(0,0,0,0.1);
    z-index: 1000;
    max-height: 220px;
    overflow: hidden;
}

.permission__dropdown-item {
    padding: 8px 15px;
    cursor: pointer;
    transition: background-color 0.2s;
    border-bottom: 1px solid #f5f5f5;
    font-size: 12px;
}

.permission__dropdown-item:hover {
    background-color: #f5f5f5;
}

.permission__dropdown-item:last-child {
    border-bottom: none;
}

.permission__item-text {
    font-size: 12px;
    color: #333;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

/* File upload styles */
.completion__files {
    margin-top: 10px;
}

.files__upload {
    display: flex;
    gap: 10px;
    align-items: center;
}

.file-input {
    display: none;
}

.upload-button {
    background: #4F5BDF;
    color: white;
    border: none;
    border-radius: 15px;
    padding: 8px 15px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.upload-button:hover:not(:disabled) {
    background: #3a45c0;
}

.upload-button:disabled {
    background: #a2a2a2;
    cursor: not-allowed;
    opacity: 0.6;
}

.uploaded-files {
    margin-top: 10px;
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.uploaded-file {
    display: flex;
    align-items: center;
    padding: 8px 10px;
    background: #f8f9fa;
    border-radius: 8px;
    border: 1px solid #e6e6e6;
    gap: 10px;
}

.file-preview {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
}

.file-preview-image {
    width: 100%;
    height: 100%;
    object-fit: cover;
    border-radius: 4px;
}

.file-icon {
    width: 20px;
    height: 20px;
}

.file-name {
    font-size: 12px;
    color: #333;
    flex: 1;
}

.remove-file-btn {
    background: none;
    border: none;
    color: #ff4444;
    cursor: pointer;
    font-size: 16px;
    padding: 0;
    width: 20px;
    height: 20px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
}

.remove-file-btn:hover:not(:disabled) {
    background: #ffebee;
    border-radius: 50%;
}

.remove-file-btn:disabled {
    cursor: not-allowed;
    opacity: 0.6;
}

/* Passage tables styles */
.completion__passage {
    margin-top: 15px;
}

.passage__grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
    row-gap: 5px;
    max-width: 425px;
    margin-top: 5px;
}

.passage__item {
    height: 30px;
    background: #F2F2F2;
    color: #a2a2a2;
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

.passage__item:hover:not(.passage__item--active):not(.passage__item--inactive) {
    background: #e8e8e8;
}

.passage__item--active {
    background: #4F5BDF;
    color: #fff;
    border-color: #4F5BDF;
}

.passage__item--inactive {
    background: #ffe6e6;
    color: #ff6b6b;
    border-color: #ffcccc;
    cursor: not-allowed;
    opacity: 0.7;
}

.passage__item--attached {
    border-left: 3px solid #4F5BDF;
}

.error-message {
    font-size: 11px;
    color: #ff4444;
    margin-top: 5px;
}

.loading-message {
    font-size: 12px;
    color: #a2a2a2;
    text-align: center;
    padding: 20px;
}

.no-tables-message {
    font-size: 12px;
    color: #ff6b6b;
    text-align: center;
    padding: 20px;
    background: #fff5f5;
    border-radius: 8px;
    margin-top: 10px;
}

.completion__button {
    width: fit-content;
    height: 25px;
    padding: 0 15px;
    border-radius: 50px;
    background: #FFF;
    border: 1px solid #e6e6e6;
    outline: none;
    font-size: 11px;
    color: #4F5BDF;
    font-weight: 600;
    cursor: pointer;
}

.completion__button:hover {
    background-color: #f5f5f5;
}

/* Стили для существующих сотрудников */
.existing-employees-info {
    margin-bottom: 15px;
    padding: 10px;
    background: #f8f9fa;
    border-radius: 10px;
    border: 1px solid #e6e6e6;
}

.existing-employees-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.existing-employees-count {
    font-size: 14px;
    font-weight: 500;
    color: #333;
}

.existing-employees-actions {
    display: flex;
    gap: 10px;
}

.view-employees-btn {
    background: white;
    color: #4F5BDF;
    border: 1px solid #4F5BDF;
    border-radius: 15px;
    padding: 5px 10px;
    font-size: 11px;
    cursor: pointer;
}

.view-employees-btn:hover {
    background: #f0f2ff;
}

.add-existing-btn {
    background: #4F5BDF;
    color: white;
    border: none;
    border-radius: 15px;
    padding: 5px 10px;
    font-size: 11px;
    cursor: pointer;
}

.add-existing-btn:hover:not(:disabled) {
    background: #3a45c0;
}

.add-existing-btn:disabled {
    background: #a2a2a2;
    cursor: not-allowed;
    opacity: 0.6;
}

/* Tooltip для неактивных таблиц */
.inactive-tooltip {
    position: fixed;
    transform: translateX(-50%) translateY(-100%);
    z-index: 10000;
    pointer-events: none;
}

.inactive-tooltip-content {
    background: #333;
    color: white;
    padding: 8px 12px;
    border-radius: 8px;
    font-size: 12px;
    max-width: 300px;

    box-shadow: 0 2px 8px rgba(0,0,0,0.2);
}

.inactive-tooltip-content::before {
    content: '';
    position: absolute;
    top: 100%;
    left: 50%;
    transform: translateX(-50%);
    border: 5px solid transparent;
    border-top-color: #333;
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
    background: #fff5f5;
    border: 1px solid #f5c2c7;
    border-radius: 10px;
}

.blacklist-warning .warning-title {
    font-weight: 600;
    color: #b02a37;
    margin: 0 0 5px 0;
    font-size: 14px;
}

.blacklist-warning .warning-details {
    color: #b02a37;
    margin: 0;
    font-size: 12px;
    line-height: 1.5;
}
</style>