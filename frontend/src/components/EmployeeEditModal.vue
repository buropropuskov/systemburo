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
                  <button
                    class="add-button"
                    :disabled="!canSaveEmployee"
                    @click="saveEmployee"
                  >
                    {{ editingEmployee ? 'Сохранить' : 'Добавить' }}
                  </button>
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

                <!-- Загрузка файлов -->
                <div
                  v-if="isPatentRequired"
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
                      accept="image/*,.pdf,.doc,.docx"
                      class="file-input"
                      @change="handleFileUpload"
                    >
                    <button
                      class="upload-button"
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
                      <span class="file-name">{{ truncateText(file.name, 30) }}</span>
                      <button
                        class="remove-file-btn"
                        @click="removeFile(index)"
                      >
                        ×
                      </button>
                    </div>
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
            uploadedFiles: [],

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
                this.uploadedFiles = [];
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
            this.uploadedFiles = [];
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
            this.uploadedFiles = [];
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
                this.uploadedFiles = [];
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

        triggerFileInput() {
            this.$refs.fileInput.click();
        },

        handleFileUpload(event) {
            const files = Array.from(event.target.files);
            files.forEach(file => {
                this.uploadedFiles.push({
                    name: file.name,
                    file: file,
                    type: this.getFileType(file)
                });
            });
            event.target.value = '';
        },

        getFileType(file) {
            if (file.type.startsWith('image/')) return 'photo';
            if (file.name.toLowerCase().includes('patent')) return 'patent';
            if (file.type === 'application/pdf') return 'document';
            return 'other';
        },

        removeFile(index) {
            this.uploadedFiles.splice(index, 1);
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

                    if (this.uploadedFiles.length > 0 && savedEmployee.id) {
                        await this.uploadEmployeeFiles(savedEmployee.id);
                    }

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

        async uploadEmployeeFiles(employeeId) {
            try {
                const formData = new FormData();
                this.uploadedFiles.forEach(file => {
                    formData.append('files', file.file);
                    formData.append('file_types', file.type);
                });
                const response = await apiRequest(`/unique-employees/${employeeId}/files`, {
                    method: 'POST',
                    body: formData,
                    headers: {}
                });
                if (!response.ok) {
                    console.error('Ошибка при загрузке файлов');
                }
            } catch (error) {
                console.error('Ошибка при загрузке файлов:', error);
            }
        }
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
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    backdrop-filter: blur(0.1px);
    -webkit-backdrop-filter: blur(0.1px);
}

.modal-content {
    background: white;
    border-radius: 20px;
    padding: 0;
    width: 600px;
    max-width: 90vw;
    max-height: 90vh;
    overflow: hidden;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: 20px;
    border-bottom: 1px solid #e6e6e6;
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
    color: #333;
    font-size: 18px;
}

.modal-close {
    background: none;
    border: none;
    font-size: 24px;
    cursor: pointer;
    color: #a2a2a2;
    padding: 0;
    width: 30px;
    height: 30px;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-left: 10px;
}

.modal-close:hover {
    color: #333;
}

.modal-body {
    padding: 20px;
    max-height: 70vh;
    overflow-y: auto;
}

/* Стили формы */
.data__completion {
    padding: 0;
}

.input__label {
    font-size: 13px;
    color: #a2a2a2;
}

.required {
    color: #ff4444;
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
    color: #a2a2a2;
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

.dropdown__button:hover {
    border-color: #4F5BDF;
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
    color: #000;
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

.disabled-field {
    opacity: 0.5;
}

.disabled-field .name__input,
.disabled-field .permission__dropdown-button {
    background-color: #f5f5f5;
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
}

.permission__dropdown-item:hover {
    background-color: #f5f5f5;
}

.permission__dropdown-item:last-child {
    border-bottom: none;
}

.permission__item-text {
    font-size: 14px;
    color: #333;
}

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

.upload-button:hover {
    background: #3a45c0;
}

.uploaded-files {
    margin-top: 10px;
    display: flex;
    flex-direction: column;
    gap: 5px;
}

.uploaded-file {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 5px 10px;
    background: #f8f9fa;
    border-radius: 8px;
    border: 1px solid #e6e6e6;
}

.file-name {
    font-size: 12px;
    color: #333;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 85%;
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
}

.remove-file-btn:hover {
    background: #ffebee;
    border-radius: 50%;
}

.completion__binding {
    margin-top: 15px;
    padding-top: 15px;
    border-top: 1px solid #e6e6e6;
}

.binding-info {
    margin-top: 10px;
    margin-bottom: 10px;
}

.binding-note {
    font-size: 12px;
    color: #666;
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
    color: #000;
    font-weight: 400;
}

.red {
    color: #ff4444;
}

.add-button {
    background: #4F5BDF;
    color: white;
    border: none;
    border-radius: 15px;
    padding: 6px 12px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.add-button:hover:not(:disabled) {
    background: #3a45c0;
}

.add-button:disabled {
    background: #a2a2a2;
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
