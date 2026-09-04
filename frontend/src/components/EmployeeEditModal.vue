<template>
  <BaseModal
    :show="visible"
    :title="editingEmployee ? 'Редактирование' : 'Добавление сотрудника'"
    width="600px"
    radius="30px"
    @close="$emit('close')"
  >
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
              <svg
                class="button__arrow"
                :class="{ 'button__arrow--open': isCitizenshipDropdownOpen }"
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
              placeholder="Введите серию и номер паспорта"
              maxlength="100"
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
                <svg
                  class="permission__button-arrow"
                  :class="{ 'permission__button-arrow--open': isPermissionDropdownOpen }"
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

      <!-- Согласие субъекта на обработку персональных данных (152-ФЗ). У записи, где
           согласие уже получено, показываем дату - подтверждать второй раз нечего. -->
      <div class="completion__consent">
        <label class="input__label">Согласие на обработку персональных данных</label>
        <p
          v-if="consentAlreadyGranted"
          class="consent-granted"
          data-testid="employee-consent-granted"
        >
          Получено {{ formatConsentDate(editingEmployee.pd_consent_at) }}
        </p>
        <label
          v-else
          class="consent-option"
        >
          <input
            v-model="pdConsent"
            type="checkbox"
            data-testid="employee-registry-pd-consent"
          >
          <span>
            Работник дал <a
              href="/data-processing"
              target="_blank"
              rel="noopener"
              class="blue"
              @click.stop
            >согласие</a> на обработку своих персональных данных<span class="required">*</span>
          </span>
        </label>
      </div>

      <!-- Привязка чужой записи: администратор её не переносит на себя, поэтому
           вместо переключателей «привязать к моей организации» показываем, за кем
           запись закреплена. -->
      <div
        v-if="foreignRecord"
        class="completion__binding"
      >
        <label class="input__label">Привязка</label>
        <div class="binding-info">
          <p
            class="binding-note"
            data-testid="employee-foreign-binding-note"
          >
            Запись закреплена за
            <strong v-if="editingEmployee && editingEmployee.user_name">пользователем «{{ editingEmployee.user_name }}»</strong>
            <strong v-else>другим пользователем</strong>
            <template v-if="editingEmployee && editingEmployee.organization_name">
              , организация «{{ editingEmployee.organization_name }}»
            </template>
            <template v-if="editingEmployee && editingEmployee.company_name">
              , компания «{{ editingEmployee.company_name }}»
            </template>.
            Правка данных привязку не меняет.
          </p>
        </div>
      </div>

      <!-- Привязка -->
      <div
        v-else
        class="completion__binding"
      >
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
  </BaseModal>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useDeletionsStore } from '@/stores/deletions'
import BaseModal from '@/components/ui/BaseModal.vue'
import { formatMomentDate } from '@/utils/datetime';

export default {
    components: {
        BaseModal
    },
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
        },
        // Правим запись, которая не относится ни к пользователю, ни к его организации
        // или компании - так бывает только у администратора. Принадлежность считает
        // вью (employeeBelongsToUser), чтобы правило жило в одном месте.
        foreignRecord: {
            type: Boolean,
            default: false
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

            // Отметка о согласии субъекта. Для новой записи обязательна (сервер без неё
            // не создаёт запись), у существующей заполняется только если согласия ещё нет.
            pdConsent: false,

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
            if (!this.consentAlreadyGranted && !this.pdConsent) {
                return false;
            }
            return true;
        },

        // Согласие у записи уже зафиксировано - повторно его не спрашиваем и снять
        // галочкой не даём: отметка живёт в базе с датой и автором.
        consentAlreadyGranted() {
            return !!(this.editingEmployee && this.editingEmployee.pd_consent_at);
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
            if (!this.consentAlreadyGranted && !this.pdConsent) {
                reasons.push('Отметьте согласие работника на обработку персональных данных');
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
                // У записи без отметки согласие подтверждают заново: галочку начинаем
                // снятой, иначе правка «поставила» бы согласие сама собой.
                this.pdConsent = false;
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
            this.pdConsent = false;
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
            // Отметка о согласии снимается вместе с данными: подтверждают конкретного
            // человека, а не всех, кого заведут дальше. Оставшись стоять, она превращала
            // осознанное подтверждение в состояние формы - следующего работника можно было
            // добавить, не глядя на неё (форма подачи, EmployeeForm, снимает её так же).
            this.pdConsent = false;
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

            // У чужой записи привязку карточка не показывает и не отправляет, поэтому и
            // сравнивать нечего: организация администратора не совпала бы с организацией
            // записи, и «изменения» находились бы всегда.
            if (this.foreignRecord) {
                return false;
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

        // Дата получения согласия: человеку нужен день, не отметка времени.
        formatConsentDate(value) {
            if (!value) return '';
            const date = new Date(value);
            if (Number.isNaN(date.getTime())) return '';
            return formatMomentDate(date);
        },

        truncateText(text, maxLength) {
            if (!text) return '';
            if (text.length <= maxLength) return text;
            return text.substring(0, maxLength) + '...';
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
                    pd_consent: this.pdConsent || this.consentAlreadyGranted
                };
                if (this.foreignRecord) {
                    // Администратор правит запись чужой организации: привязку переносим
                    // как есть. Прежде поля брались из ownership-info правящего, то есть
                    // сотрудник контрагента переехал бы к бюро вместе с исправлением ФИО.
                    // user_id не отправляем совсем - сервер сохранит прежнего владельца.
                    employeeData.organization_id = this.editingEmployee?.organization_id ?? null;
                    employeeData.company_id = this.editingEmployee?.company_id ?? null;
                } else {
                    employeeData.user_id = this.ownershipInfo.user_id;
                    employeeData.organization_id = this.bindToOrganization ? this.ownershipInfo.organization_id : null;
                    employeeData.company_id = this.bindToCompany ? this.ownershipInfo.company_id : null;
                }

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
                    // Текст сервера показываем как есть: он различает три случая -
                    // запись уже у вас, у кого-то в организации или в компании. Прежде
                    // два последних подменялись на «уже привязан к вашему аккаунту», и
                    // человек шёл искать сотрудника в «Мои сотрудники», где его нет (#2021).
                    const errorMessage = errorData.message || 'Ошибка при сохранении сотрудника';
                    useDeletionsStore().notify({ bold: errorMessage, type: 'error' });
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
/* Модальное окно теперь на BaseModal (шапка/крестик/overlay/Escape/bottom-sheet -
   его контракт). base-modal__body у BaseModal идёт БЕЗ padding (отступы несёт
   содержимое) - без них поля упирались в края окна и на телефоне читались еле-еле
   ("отступов нет по бокам"). Значение - как у соседних окон на BaseModal
   (ChangePasswordModal/AttachmentMappingCopyModal): 20px по бокам вровень с
   заголовком шапки. */
.data__completion {
    padding: 14px 20px 18px;
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
    height: 6px;
    flex-shrink: 0;
    color: var(--text-muted);
    transition: transform 0.2s;
}

.button__arrow--open {
    transform: rotate(180deg);
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

/*
 * Заголовки полей - только подпись над контролом. Раньше они стояли в одной
 * группе селекторов с .name__input (список кончался запятой, а следом шла
 * пустая строка), поэтому получали высоту 40px, рамку и скругление: над каждым
 * полем рисовался пустой прямоугольник, и форма выглядела задвоенной.
 */
.completion__last-name-header,
.completion__first-name-header,
.completion__middle-name-header,
.completion__position-header,
.completion__passport-header,
.completion__patent-header,
.completion__permission-header {
    margin-bottom: 5px;
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
    height: 6px;
    flex-shrink: 0;
    color: var(--text-muted);
    transition: transform 0.2s;
}

.permission__button-arrow--open {
    transform: rotate(180deg);
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

.completion__consent {
    margin-top: 14px;
}

.consent-option {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    font-size: 11px;
    line-height: 1.3;
    cursor: pointer;
    margin-top: 6px;
}

.consent-option input[type="checkbox"] {
    margin-top: 2px;
    flex-shrink: 0;
}

.consent-granted {
    margin: 6px 0 0;
    font-size: 11px;
    color: var(--text-muted);
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
    .completion__name-row {
        flex-direction: column;
    }

    /* Подписи чекбоксов привязки («Привязать к организации/компании») на телефоне
       было еле видно - +2px к шрифту и увеличенный чекбокс (12px -> 18px, тот же приём,
       что у выбора машин/сотрудников в ExistingCarsModal/ExistingEmployeesModal: видимый
       квадрат остаётся некрупным, а тач-таргет строки дотягивает до нормы проекта 36px
       через min-height, а не раздутый чекбокс). */
    .binding-option {
        min-height: 36px;
        font-size: 14px;
    }

    .binding-option input[type="checkbox"] {
        width: 18px;
        height: 18px;
        flex-shrink: 0;
    }

    .user-binding-text {
        font-size: 12px;
    }
}
</style>
