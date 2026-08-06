<template>
  <Teleport to="body">
    <Transition name="modal-fade">
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
            <div class="modal-header__top">
              <h3>{{ editingEmployee ? 'Редактирование' : 'Добавление сотрудника' }}</h3>
            </div>
            <button
              class="modal-close"
              @click="$emit('close')"
            >
              ×
            </button>
          </div>
          <div class="modal-body">
            <div class="data__completion">
              <div class="completion__citizenship">
                <div class="citizenship__header">
                  <label class="citizenship__label">Гражданство <span class="required">*</span></label>
                  <span
                    class="hint-anchor hint-anchor--below hint-anchor--right"
                    :data-hint="saveEmployeeHint"
                  >
                    <button
                      class="add-button"
                      :disabled="!canSaveEmployee"
                      @click="saveEmployee"
                    >
                      {{ editingEmployee ? 'Сохранить' : 'Добавить' }}
                    </button>
                  </span>
                </div>
                <div class="citizenship__dropdown">
                  <button
                    class="dropdown__button"
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
                        v-for="citizenship in citizenships"
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
                <!-- Первая строка: Фамилия и Имя -->
                <div class="completion__name-row">
                  <div class="completion__last-name">
                    <div class="completion__last-name-header">
                      <label class="input__label">Фамилия <span class="required">*</span></label>
                    </div>
                    <input
                      v-model="lastName"
                      class="name__input"
                      placeholder="Введите фамилию"
                    >
                  </div>
                  <div class="completion__first-name">
                    <div class="completion__first-name-header">
                      <label class="input__label">Имя <span class="required">*</span></label>
                    </div>
                    <input
                      v-model="firstName"
                      class="name__input"
                      placeholder="Введите имя"
                    >
                  </div>
                </div>

                <!-- Вторая строка: Отчество и Должность -->
                <div class="completion__name-row">
                  <div class="completion__middle-name">
                    <div class="completion__middle-name-header">
                      <label class="input__label">Отчество</label>
                    </div>
                    <input
                      v-model="middleName"
                      class="name__input"
                      placeholder="Введите отчество"
                    >
                  </div>
                  <div class="completion__position">
                    <div class="completion__position-header">
                      <label class="input__label">Должность <span class="required">*</span></label>
                    </div>
                    <input
                      v-model="position"
                      class="name__input"
                      placeholder="Введите должность"
                    >
                  </div>
                </div>

                <!-- Третья строка: Паспорт и Номер патента -->
                <div class="completion__name-row">
                  <div class="completion__passport">
                    <div class="completion__passport-header">
                      <label class="input__label">Серия и номер паспорта <span class="required">*</span></label>
                    </div>
                    <input
                      v-model="passportSeriesNumber"
                      class="name__input"
                      placeholder="XXXX XXXXXX"
                      @input="formatPassport"
                    >
                  </div>
                  <div
                    class="completion__patent"
                    :class="{ 'disabled-field': !isPatentRequired }"
                  >
                    <div class="completion__patent-header">
                      <label class="input__label">Номер патента</label>
                    </div>
                    <input
                      v-model="patentNumber"
                      class="name__input"
                      placeholder="Введите номер патента"
                      :disabled="!isPatentRequired || selectedPermission !== 'Не выбрано'"
                      @input="handlePatentInput"
                    >
                  </div>
                </div>

                <!-- Четвертая строка: Иное разрешение -->
                <div
                  class="completion__permission"
                  :class="{ 'disabled-field': !isPatentRequired }"
                >
                  <div class="completion__permission-header">
                    <label class="input__label">Иное разрешение на работы</label>
                  </div>
                  <div class="permission__dropdown">
                    <button
                      class="permission__dropdown-button"
                      :disabled="!isPatentRequired || patentNumber.trim() !== ''"
                      @click="togglePermissionDropdown"
                    >
                      <div class="permission__button-content">
                        <span class="permission__button-text">{{ selectedPermission || 'Не выбрано' }}</span>
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
              </div>

              <!-- Привязка -->
              <div class="completion__binding">
                <label class="input__label">Привязка</label>
                <div class="binding-info">
                  <p class="binding-note">
                    <strong>Добавляемый сотрудник автоматически привязывается к аккаунту пользователя.</strong>
                    Сотрудника можно привязать к организации или компании, для использования <strong>другими сотрудниками</strong>:
                  </p>
                </div>
                <div class="binding-options">
                  <label
                    v-if="ownershipInfo && ownershipInfo.has_organization"
                    class="binding-option"
                  >
                    <input
                      v-model="bindToOrganization"
                      type="checkbox"
                    >
                    <span>Привязать к организации<template v-if="ownershipInfo.organization_name"> «{{ ownershipInfo.organization_name }}»</template></span>
                  </label>
                  <label
                    v-if="ownershipInfo && ownershipInfo.has_company"
                    class="binding-option"
                  >
                    <input
                      v-model="bindToCompany"
                      type="checkbox"
                    >
                    <span>Привязать к компании<template v-if="ownershipInfo.company_name"> «{{ ownershipInfo.company_name }}»</template></span>
                  </label>
                  <div class="user-binding">
                    <span class="user-binding-text"><strong class="red">Внимание!</strong> При привязке сотрудника к организации или компании, он будет доступен для отображения и использования для всех сотрудников, привязанных к организации/компании. </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useDeletionsStore } from '@/stores/deletions'

export default {
    props: {
        visible: {
            type: Boolean,
            required: true
        },
        editingEmployee: {
            type: Object,
            default: null
        },
        citizenships: {
            type: Array,
            required: true
        },
        ownershipInfo: {
            type: Object,
            default: null
        }
    },
    emits: ['saved', 'close'],
    data() {
        return {
            // Гражданство
            selectedCitizenship: null,
            isCitizenshipDropdownOpen: false,

            // Основные поля
            lastName: '',
            firstName: '',
            middleName: '',
            position: '',
            passportSeriesNumber: '',
            patentNumber: '',

            // Разрешения
            selectedPermission: 'Не выбрано',
            isPermissionDropdownOpen: false,
            availablePermissions: [
                'Не выбрано',
                'Разрешение на работу временного проживания',
                'Разрешение на работу вида на жительство',
                'Свидетельство участника Госпрограммы',
                'Разрешение на работу для высококвалифицированных специалистов',
                'Иное разрешение'
            ],

            // Файлы

            // Привязка
            bindToOrganization: false,
            bindToCompany: false,

            // Для проверки изменений при редактировании
            originalEmployeeData: null
        };
    },
    computed: {
        selectedCitizenshipText() {
            return this.selectedCitizenship ? this.selectedCitizenship.name : 'Выберите гражданство';
        },

        isPatentRequired() {
            return this.selectedCitizenship ? this.selectedCitizenship.patent_required : false;
        },

        canSaveEmployee() {
            if (!this.lastName.trim() || !this.firstName.trim()) {
                return false;
            }
            if (!this.selectedCitizenship) {
                return false;
            }
            if (!this.passportSeriesNumber.trim()) {
                return false;
            }
            if (!this.position.trim()) {
                return false;
            }
            if (this.isPatentRequired) {
                if (!this.patentNumber.trim() && (this.selectedPermission === 'Не выбрано' || !this.selectedPermission)) {
                    return false;
                }
            }
            return true;
        },

        /**
         * Подсказка на заблокированной кнопке сохранения: чего не хватает.
         * Пустая строка - форма заполнена (селектор подсказки пустое
         * значение не берёт).
         */
        saveEmployeeHint() {
            if (this.canSaveEmployee) return '';

            const missing = [];
            if (!this.selectedCitizenship) missing.push('гражданство');
            if (!this.lastName.trim()) missing.push('фамилию');
            if (!this.firstName.trim()) missing.push('имя');
            if (!this.position.trim()) missing.push('должность');
            if (!this.passportSeriesNumber.trim()) missing.push('серию и номер паспорта');

            const reasons = [];
            if (missing.length) reasons.push(`Заполните: ${missing.join(', ')}`);
            if (this.isPatentRequired
                && !this.patentNumber.trim()
                && (this.selectedPermission === 'Не выбрано' || !this.selectedPermission)) {
                reasons.push('Для этого гражданства нужен номер патента или иное разрешение на работы');
            }
            return reasons.join('. ');
        }
    },
    watch: {
        visible(newVal) {
            if (newVal) {
                this.initForm();
            }
        },

        isPatentRequired(newVal) {
            if (!newVal) {
                this.patentNumber = '';
                this.selectedPermission = 'Не выбрано';
            }
        },

        patentNumber(newVal) {
            if (newVal.trim() !== '') {
                this.selectedPermission = 'Не выбрано';
            }
        },

        selectedPermission(newVal) {
            if (newVal !== 'Не выбрано' && newVal !== '') {
                this.patentNumber = '';
            }
        }
    },
    mounted() {
        document.addEventListener('click', (e) => {
            if (!e.target.closest('.citizenship__dropdown')) {
                this.isCitizenshipDropdownOpen = false;
            }
            if (!e.target.closest('.permission__dropdown')) {
                this.isPermissionDropdownOpen = false;
            }
        });

        if (this.visible) {
            this.initForm();
        }
    },
    methods: {
        initForm() {
            if (this.editingEmployee) {
                this.originalEmployeeData = {
                    last_name: this.editingEmployee.last_name,
                    first_name: this.editingEmployee.first_name,
                    middle_name: this.editingEmployee.middle_name,
                    position: this.editingEmployee.position,
                    citizenship_id: this.editingEmployee.citizenship_id,
                    passport_series_number: this.editingEmployee.passport_series_number,
                    patent_number: this.editingEmployee.patent_number,
                    other_permission: this.editingEmployee.other_permission,
                    organization_id: this.editingEmployee.organization_id,
                    company_id: this.editingEmployee.company_id
                };

                this.lastName = this.editingEmployee.last_name || '';
                this.firstName = this.editingEmployee.first_name || '';
                this.middleName = this.editingEmployee.middle_name || '';
                this.position = this.editingEmployee.position || '';
                this.passportSeriesNumber = this.editingEmployee.passport_series_number || '';
                this.patentNumber = this.editingEmployee.patent_number || '';
                this.selectedPermission = this.editingEmployee.other_permission || 'Не выбрано';

                if (this.editingEmployee.citizenship_id) {
                    const found = this.citizenships.find(c => c.id === this.editingEmployee.citizenship_id);
                    if (found) {
                        this.selectedCitizenship = found;
                    }
                }

                this.bindToOrganization = !!this.editingEmployee.organization_id;
                this.bindToCompany = !!this.editingEmployee.company_id;
            } else {
                this.resetForm();
            }
        },

        resetForm() {
            this.selectedCitizenship = this.citizenships.find(c => c.is_default) || this.citizenships[0] || null;
            this.lastName = '';
            this.firstName = '';
            this.middleName = '';
            this.position = '';
            this.passportSeriesNumber = '';
            this.patentNumber = '';
            this.selectedPermission = 'Не выбрано';
            this.bindToOrganization = false;
            this.bindToCompany = false;
            this.originalEmployeeData = null;
        },

        clearFormFields() {
            this.lastName = '';
            this.firstName = '';
            this.middleName = '';
            this.position = '';
            this.passportSeriesNumber = '';
            this.patentNumber = '';
            this.selectedPermission = 'Не выбрано';
            if (!this.editingEmployee) {
                this.bindToOrganization = false;
                this.bindToCompany = false;
            }
        },

        hasChanges() {
            if (!this.editingEmployee) {
                return true;
            }

            if (this.lastName !== this.originalEmployeeData.last_name ||
                this.firstName !== this.originalEmployeeData.first_name ||
                this.middleName !== this.originalEmployeeData.middle_name ||
                this.position !== this.originalEmployeeData.position ||
                this.passportSeriesNumber !== this.originalEmployeeData.passport_series_number ||
                this.patentNumber !== this.originalEmployeeData.patent_number ||
                this.selectedPermission !== this.originalEmployeeData.other_permission) {
                return true;
            }

            if (this.selectedCitizenship.id !== this.originalEmployeeData.citizenship_id) {
                return true;
            }

            const currentOrgId = this.bindToOrganization ? this.ownershipInfo.organization_id : null;
            if (currentOrgId !== this.originalEmployeeData.organization_id) {
                return true;
            }

            const currentCompanyId = this.bindToCompany ? this.ownershipInfo.company_id : null;
            if (currentCompanyId !== this.originalEmployeeData.company_id) {
                return true;
            }

            return false;
        },

        truncateText(text, maxLength) {
            if (!text) return '';
            if (text.length <= maxLength) return text;
            return text.substring(0, maxLength) + '...';
        },

        formatPassport(event) {
            let value = event.target.value.replace(/\D/g, '');
            if (value.length > 10) {
                value = value.slice(0, 10);
            }
            if (value.length > 4) {
                value = value.slice(0, 4) + ' ' + value.slice(4);
            }
            this.passportSeriesNumber = value;
            event.target.value = value;
        },

        handlePatentInput() {
            if (this.patentNumber.trim() !== '') {
                this.selectedPermission = 'Не выбрано';
            }
        },

        toggleCitizenshipDropdown() {
            this.isCitizenshipDropdownOpen = !this.isCitizenshipDropdownOpen;
        },

        selectCitizenship(citizenship) {
            this.selectedCitizenship = citizenship;
            this.isCitizenshipDropdownOpen = false;
            if (!citizenship.patent_required) {
                this.patentNumber = '';
                this.selectedPermission = 'Не выбрано';
            }
        },

        togglePermissionDropdown() {
            this.isPermissionDropdownOpen = !this.isPermissionDropdownOpen;
        },

        selectPermission(permission) {
            this.selectedPermission = permission;
            this.isPermissionDropdownOpen = false;
            if (permission !== 'Не выбрано') {
                this.patentNumber = '';
            }
        },

        async saveEmployee() {
            if (!this.canSaveEmployee) {
                useDeletionsStore().notify({ bold: 'Заполните обязательные поля', type: 'error' });
                return;
            }

            if (this.editingEmployee && !this.hasChanges()) {
                useDeletionsStore().notify({ bold: 'Изменений не обнаружено', type: 'info' });
                return;
            }

            try {
                const employeeData = {
                    last_name: this.lastName.trim(),
                    first_name: this.firstName.trim(),
                    middle_name: this.middleName.trim(),
                    position: this.position.trim(),
                    citizenship_id: this.selectedCitizenship.id,
                    passport_series_number: this.passportSeriesNumber.trim(),
                    patent_number: this.isPatentRequired && this.patentNumber.trim() ? this.patentNumber.trim() : null,
                    other_permission: this.isPatentRequired && this.selectedPermission !== 'Не выбрано' ? this.selectedPermission : null,
                    user_id: this.ownershipInfo.user_id,
                    organization_id: this.bindToOrganization ? this.ownershipInfo.organization_id : null,
                    company_id: this.bindToCompany ? this.ownershipInfo.company_id : null
                };

                let response;
                if (this.editingEmployee) {
                    response = await apiRequest(`/unique-employees/${this.editingEmployee.id}`, {
                        method: 'PUT',
                        body: JSON.stringify(employeeData)
                    });
                } else {
                    response = await apiRequest('/unique-employees', {
                        method: 'POST',
                        body: JSON.stringify(employeeData)
                    });
                }

                if (response.ok) {
                    const savedEmployee = await response.json();
                    const action = this.editingEmployee ? 'обновлён' : 'добавлен';
                    useDeletionsStore().notify({ prefix: 'Сотрудник ', bold: `${this.lastName} ${this.firstName}`.trim() || 'запись', suffix: ` ${action}`, type: 'success' });

                    if (!this.editingEmployee) {
                        this.clearFormFields();
                    } else {
                        this.originalEmployeeData = {
                            last_name: this.lastName,
                            first_name: this.firstName,
                            middle_name: this.middleName,
                            position: this.position,
                            citizenship_id: this.selectedCitizenship.id,
                            passport_series_number: this.passportSeriesNumber,
                            patent_number: employeeData.patent_number,
                            other_permission: employeeData.other_permission,
                            organization_id: employeeData.organization_id,
                            company_id: employeeData.company_id
                        };
                    }

                    this.$emit('saved', savedEmployee);
                } else {
                    const errorData = await response.json();
                    const errorMessage = errorData.message || 'Ошибка при сохранении сотрудника';

                    if (errorMessage.includes('уже существует') || errorMessage.includes('already exists')) {
                        useDeletionsStore().notify({ bold: 'Сотрудник уже привязан к вашему аккаунту', type: 'error' });
                    } else {
                        useDeletionsStore().notify({ bold: errorMessage, type: 'error' });
                    }
                }
            } catch (error) {
                console.error('Ошибка при сохранении сотрудника:', error);
                useDeletionsStore().notify({ prefix: 'Не удалось сохранить ', bold: 'сотрудника', type: 'error' });
            }
        },
    }
}
</script>

<style scoped>
/* Модальное окно */
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
    backdrop-filter: blur(0.1px);
    -webkit-backdrop-filter: blur(0.1px);
}

.modal-content {
    background: var(--surface);
    border-radius: 20px;
    padding: 0;
    width: 600px;
    max-width: 90vw;
    max-height: calc(var(--app-vh, 1vh) * 90);
    overflow: hidden;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: 20px;
    border-bottom: 1px solid var(--border);
}

.modal-header__top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex: 1;
    height: 25px;
}

.modal-header h3 {
    margin: 0;
    color: var(--text);
    font-size: 18px;
}

.modal-close {
    background: none;
    border: none;
    font-size: 24px;
    cursor: pointer;
    color: var(--text-muted);
    padding: 0;
    width: 30px;
    height: 30px;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-left: 10px;
}

.modal-close:hover {
    color: var(--text);
}

.modal-body {
    padding: 20px;
    max-height: calc(var(--app-vh, 1vh) * 70);
    overflow-y: auto;
}

/* Стили формы */
.data__completion {
    padding: 0;
}

.input__label {
    font-size: 13px;
    color: var(--text-muted);
}

.required {
    color: var(--danger-text);
}

.completion__citizenship {
    display: flex;
    flex-direction: column;
    gap: 5px;
    position: relative;
    padding-bottom: 10px;
}

.citizenship__header {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.citizenship__label {
    font-size: 13px;
    color: var(--text-muted);
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

.dropdown__button:hover {
    border-color: var(--accent);
}

.button__content {
    display: flex;
    align-items: center;
    width: 100%;
    height: 100%;
    justify-content: space-between;
}

.button__text {
    font-size: 14px;
    color: var(--text);
    font-weight: 500;
}

.button__arrow {
    width: 10px;
    height: 10px;
    transition: transform 0.2s;
    transform: rotate(90deg);
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
}

.patent-required-badge {
    background: var(--danger-bg);
    color: var(--danger-text);
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

.disabled-field {
    opacity: 0.5;
}

.disabled-field .name__input,
.disabled-field .permission__dropdown-button {
    background-color: var(--surface-2);
    cursor: not-allowed;
}

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
    border: 1px solid var(--border);
    background-color: var(--surface);
    border-radius: 15px;
    outline: none;
    cursor: pointer;
    padding: 0 15px;
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
    height: 100%;
    justify-content: space-between;
}

.permission__button-text {
    font-size: 14px;
    color: var(--text);
}

.permission__button-arrow {
    width: 10px;
    height: 10px;
    transition: transform 0.2s;
    transform: rotate(90deg);
}

.permission__button-arrow--open {
    transform: rotate(-90deg);
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
    max-height: 220px;
    overflow: hidden;
}

.permission__dropdown-item {
    padding: 8px 15px;
    cursor: pointer;
    transition: background-color 0.2s;
    border-bottom: 1px solid var(--surface-2);
}

.permission__dropdown-item:hover {
    background-color: var(--surface-2);
}

.permission__dropdown-item:last-child {
    border-bottom: none;
}

.permission__item-text {
    font-size: 14px;
    color: var(--text);
}

.completion__binding {
    margin-top: 15px;
    padding-top: 15px;
    border-top: 1px solid var(--border);
}

.binding-info {
    margin-top: 10px;
    margin-bottom: 10px;
}

.binding-note {
    font-size: 12px;
    color: var(--text-muted);
    line-height: 1.4;
    margin: 0 0 10px 0;
}

.binding-options {
    display: flex;
    flex-direction: column;
    gap: 10px;
}

.binding-option {
    display: flex;
    align-items: center;
    gap: 10px;
    cursor: pointer;
    font-size: 12px;
}

.binding-option input[type="checkbox"] {
    width: 12px;
    height: 12px;
    cursor: pointer;
}

.user-binding {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 5px 0;
}

.user-binding-text {
    font-size: 10px;
    color: var(--text);
    font-weight: 400;
}

.red {
    color: var(--danger-text);
}

.add-button {
    background: var(--accent);
    color: var(--accent-contrast);
    border: none;
    border-radius: 15px;
    padding: 6px 12px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.add-button:hover:not(:disabled) {
    background: var(--accent-hover);
}

.add-button:disabled {
    background: var(--text-muted);
    cursor: not-allowed;
    opacity: 0.6;
}

/* Анимации для dropdown */
.dropdown-enter-active,
.dropdown-leave-active {
    transition: all 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
    opacity: 0;
    transform: translateY(-10px);
}

@media (max-width: 768px) {
    .modal-content {
        width: 95vw;
        margin: 10px;
    }

    .completion__name-row {
        flex-direction: column;
    }
}

/* Анимация открытия/закрытия */
.modal-fade-enter-active {
    transition: opacity 0.18s ease;
}
.modal-fade-leave-active {
    transition: opacity 0.18s ease;
}
.modal-fade-enter-from,
.modal-fade-leave-to {
    opacity: 0;
}
.modal-fade-enter-active .modal-content {
    animation: modal-scale-in 0.18s ease;
}
@keyframes modal-scale-in {
    from { transform: scale(0.96); opacity: 0; }
    to { transform: scale(1); opacity: 1; }
}
</style>
