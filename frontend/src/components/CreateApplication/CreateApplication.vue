<template>
  <div class="create">
    <div class="create__header">
      <div class="create__header-top">
        <div class="create__header-left">
          <div class="create__title">
            <h3>Оформление и подача заявки</h3>
          </div>
          <ApplicationRecipientsRow
            :approvers="defaultApprovers"
            :readers="readers"
            @update:readers="readers = $event"
          />
        </div>
        <h4>{{ currentFormTitle }}</h4>
      </div>
    </div>

    <div class="create__container">
      <BlankSelector
        ref="blankSelector"
        data-testid="ob-app-selector"
        :attachments="attachments"
        :current-application-data="currentApplicationData"
        @attachment-selected="handleAttachmentSelected"
        @attachment-added="handleAttachmentAdded"
        @attachment-removed="handleAttachmentRemoved"
        @attachment-renamed="handleAttachmentRenamed"
      />

      <div
        v-if="selectedAttachment"
        class="create__form"
        data-testid="ob-app-form"
      >
        <!-- 1 ряд: Письмо сопроводительное -->
        <div class="form__header">
          <TextConstructor
            v-model="message"
            class="form__message-tc"
            :rows="3"
            placeholder="Введите сопроводительное письмо / сообщение"
          />
          <!-- Согласие и отправка — правая колонка шапки, рядом с полем сообщения -->
          <div class="form__submit-bar">
            <div class="consent-section">
              <div class="consent-checkbox">
                <input
                  id="consent"
                  v-model="consentGiven"
                  type="checkbox"
                  data-testid="create-app-consent-checkbox"
                  required
                >
                <label for="consent">
                  Даю <a
                    href="/data-processing"
                    target="_blank"
                    rel="noopener"
                    class="blue consent-link"
                    @click.stop
                  >согласие</a> на обработку, хранение, передачу
                  персональных данных, изложенных в заявке
                </label>
              </div>
              <div
                class="submit-button-container"
                :data-testid="'create-app-submit-wrapper'"
                @mouseenter="tooltipMouseEnter"
                @mouseleave="tooltipMouseLeave"
              >
                <button
                  class="send-all-btn"
                  data-testid="create-app-button-submit"
                  :disabled="!canSubmit"
                  @click="submitApplication"
                >
                  Отправить заявку
                </button>
                <div
                  v-if="showSubmitTooltip && !canSubmit && tooltipSections.length"
                  class="submit-tooltip"
                  @mouseenter="tooltipMouseEnter"
                  @mouseleave="tooltipMouseLeave"
                  @click="handleTooltipClick"
                >
                  <div class="tooltip-content">
                    <div
                      v-for="(section, idx) in tooltipSections"
                      :key="idx"
                      class="tooltip-section"
                    >
                      <div
                        v-if="section.type === 'global'"
                        class="tooltip-global"
                      >
                        <div class="tooltip-section-title">
                          Необходимо заполнить:
                        </div>
                        <ul>
                          <li
                            v-for="(msg, i) in section.messages"
                            :key="i"
                          >
                            {{ msg }}
                          </li>
                        </ul>
                      </div>
                      <div
                        v-else-if="section.type === 'attachment'"
                        class="tooltip-attachment"
                      >
                        <div class="tooltip-attachment-title">
                          <span
                            class="attachment-clickable"
                            :data-attachment-key="section.attachmentKey"
                            @click="handleTooltipAttachmentClick(section)"
                          >
                            Вложение "{{ section.attachmentName }}"
                          </span>
                        </div>
                        <ul>
                          <li
                            v-for="(msg, i) in section.messages"
                            :key="i"
                          >
                            {{ msg }}
                          </li>
                        </ul>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 2 ряд: Организация, Компания, Ответственное лицо -->
        <UserInfoRow
          data-testid="ob-app-orginfo"
          :organization="organization"
          :company="company"
          :responsible-person="responsiblePerson"
          :phone-number="phoneNumber"
          :errors="errors"
          @update:organization="organization = $event"
          @update:company="company = $event"
          @update:responsible-person="responsiblePerson = $event"
          @update:phone-number="phoneNumber = $event"
          @validate-field="validateField"
          @format-phone="handleFormatPhoneNumber"
          @clear-phone="handleClearPhoneFormat"
        />

        <CustomFieldsSection
          v-if="currentCustomFields.length"
          data-testid="ob-app-custom"
          :fields="currentCustomFields"
          :model-value="currentCustomFieldValues"
          @update:model-value="updateCustomFieldValues($event)"
        />

        <!-- 3 ряд: Заголовок, Дата действия, Время пребывания (теперь индивидуально для вложения) -->
        <div
          class="form__info-row"
          data-testid="ob-app-dates"
        >
          <DateRangeSection
            :is-one-day="currentAttachmentData.isOneDay"
            :start-date="currentAttachmentData.startDate"
            :end-date="currentAttachmentData.endDate"
            :single-date="currentAttachmentData.singleDate"
            :start-time="currentAttachmentData.startTime"
            :end-time="currentAttachmentData.endTime"
            :roof-access="currentAttachmentData.roofAccess"
            :free-parking="currentAttachmentData.freeParking"
            :errors="currentAttachmentErrors"
            :field-config="currentFieldConfig"
            @update:is-one-day="updateAttachmentData('isOneDay', $event)"
            @update:start-date="updateAttachmentData('startDate', $event)"
            @update:end-date="updateAttachmentData('endDate', $event)"
            @update:single-date="updateAttachmentData('singleDate', $event)"
            @update:start-time="updateAttachmentData('startTime', $event)"
            @update:end-time="updateAttachmentData('endTime', $event)"
            @update:roof-access="updateAttachmentData('roofAccess', $event)"
            @update:free-parking="updateAttachmentData('freeParking', $event)"
            @validate-field="validateAttachmentField"
            @validate-date-range="validateAttachmentDateRange"
            @validate-time-range="validateAttachmentTimeRange"
          />
        </div>

        <!-- 4 ряд: Динамические формы в зависимости от типа вложения -->
        <div
          class="form__data"
          data-testid="ob-app-formdata"
        >
          <!-- Для автомобилей -->
          <template v-if="selectedAttachment && selectedAttachment.attachment_type === 'cars'">
            <VehicleForm
              :key="vehicleFormKey"
              ref="vehicleForm"
              :field-config="currentFieldConfig"
              :user-organization="organization"
              :user-organization-id="organizationId"
              :user-company="company"
              :user-company-id="companyId"
              :existing-vehicles="vehicles"
              :application-unload-places="applicationUnloadPlaces"
              :entry-period="currentEntryPeriod"
              @notices-change="placeNotices = $event"
              @vehicle-added="handleVehicleAdded"
              @vehicles-added="handleVehiclesAdded"
              @vehicle-updated="handleVehicleUpdated"
              @edit-cancelled="handleVehicleEditCancelled"
              @update:unload-places="onApplicationUnloadPlacesChange"
            />
            <VehiclesList
              :vehicles="sortedVehicles"
              :sort-field="sortField"
              :sort-direction="sortDirection"
              :all-unloading-places="allUnloadingPlaces"
              :license-plate-formats="licensePlateFormats"
              :detail-info="detailViewInfo"
              @sort="sortBy"
              @edit-vehicle="editVehicle"
              @delete-vehicle="deleteVehicle"
            />
          </template>

          <!-- Для людей/сотрудников -->
          <template v-else-if="selectedAttachment && selectedAttachment.attachment_type === 'people'">
            <EmployeeForm
              :key="employeeFormKey"
              ref="employeeForm"
              :field-config="currentFieldConfig"
              :user-organization="organization"
              :user-organization-id="organizationId"
              :user-company="company"
              :user-company-id="companyId"
              :existing-employees="employees"
              :entry-period="currentEntryPeriod"
              @notices-change="placeNotices = $event"
              @employee-added="handleEmployeeAdded"
              @employees-added="handleEmployeesAdded"
              @employee-updated="handleEmployeeUpdated"
              @edit-cancelled="handleEmployeeEditCancelled"
            />
            <EmployeesList
              :employees="sortedEmployees"
              :sort-field="sortField"
              :sort-direction="sortDirection"
              :all-tables="allPassageTables"
              :detail-info="detailViewInfo"
              @sort="sortBy"
              @edit-employee="editEmployee"
              @delete-employee="deleteEmployee"
            />
          </template>

          <!-- Для ТМЦ -->
          <template v-else-if="selectedAttachment && selectedAttachment.attachment_type === 'items'">
            <ItemsForm
              :key="itemsFormKey"
              ref="itemsForm"
              :field-config="currentFieldConfig"
              :existing-items="items"
              :show-unload-places="showItemsUnloadPlaces"
              :all-unloading-places="allUnloadingPlaces"
              :selected-unload-places="applicationUnloadPlaces"
              @item-added="handleItemAdded"
              @items-added="handleItemsAdded"
              @item-updated="handleItemUpdated"
              @edit-cancelled="handleItemEditCancelled"
              @update:unload-places="onApplicationUnloadPlacesChange"
            />
            <ItemsList
              :items="sortedItems"
              :sort-field="sortField"
              :sort-direction="sortDirection"
              @sort="sortBy"
              @edit-item="editItem"
              @delete-item="deleteItem"
            />
          </template>
        </div>
      </div>
            
      <!-- Заглушка для формы, когда вложение не выбрано -->
      <div
        v-else
        class="form-placeholder"
      >
        <div class="placeholder-content">
          <p>Добавьте или выберите вложение для начала работы</p>
        </div>
      </div>
    </div>

    <!-- Универсальное модальное окно привязки -->
    <UniversalBindingModal
      :show="showBindingModal"
      :new-vehicles-to-bind="newVehiclesToBind"
      :new-employees-to-bind="newEmployeesToBind"
      :organization="organization"
      :company="company"
      :has-organization="hasOrganization"
      :has-company="hasCompany"
      @confirm-binding="confirmBinding"
      @skip-binding="skipBinding"
      @close="closeBindingModal"
    />

    <ApplicationSuccessModal
      :show="showSuccessModal"
      :application-number="createdApplicationNumber"
      :attachments-data="createdAttachmentsData"
      @close="onSuccessClose"
    />

    <!-- #952: дубль пришёл, а в форме уже есть данные - заменить/объединить/отмена. -->
    <DuplicateConflictModal
      :show="showDuplicateConflict"
      @replace="onDuplicateConflictReplace"
      @merge="onDuplicateConflictMerge"
      @cancel="onDuplicateConflictCancel"
    />

    <!-- #1183: единая плавающая панель предупреждений выбранных мест (режим/текст/окна). -->
    <SchedulePlaceWarningPanel :groups="placeNotices" />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { useDeletionsStore } from '@/stores/deletions'
import { formatPhoneNumberImmediately, formatPhoneNumber, clearPhoneFormat } from '@/composables/usePhoneFormat'
import BlankSelector from '../BlankSelector.vue';
import UserInfoRow from './UserInfoRow.vue';
import DateRangeSection from './DateRangeSection.vue';
import VehicleForm from './VehicleForm.vue';
import VehiclesList from './VehiclesList.vue';
import EmployeeForm from './EmployeeForm.vue';
import EmployeesList from './EmployeesList.vue';
import ItemsForm from './ItemsForm.vue';
import ItemsList from './ItemsList.vue';
import UniversalBindingModal from './UniversalBindingModal.vue';
import ApplicationSuccessModal from './ApplicationSuccessModal.vue';
import CustomFieldsSection from './CustomFieldsSection.vue';
import TextConstructor from '@/components/TextConstructor.vue';
import ApplicationRecipientsRow from './ApplicationRecipientsRow.vue';
import DuplicateConflictModal from './DuplicateConflictModal.vue';
import SchedulePlaceWarningPanel from './SchedulePlaceWarningPanel.vue';

export default {
    name: 'CreateApplication',
    components: {
        BlankSelector,
        SchedulePlaceWarningPanel,
        UserInfoRow,
        DateRangeSection,
        VehicleForm,
        VehiclesList,
        EmployeeForm,
        EmployeesList,
        ItemsForm,
        ItemsList,
        UniversalBindingModal,
        ApplicationSuccessModal,
        CustomFieldsSection,
        TextConstructor,
        ApplicationRecipientsRow,
        DuplicateConflictModal
    },
    data() {
        return {
            message: '',
            // Конфликт дублирования (#952): на странице уже есть черновик, а из ЛК пришёл
            // дубль - показываем модалку "Заменить / Объединить / Отмена".
            showDuplicateConflict: false,
            pendingDuplicate: null,
            organization: '',
            company: '',
            responsiblePerson: '',
            phoneNumber: '',
            rawPhoneNumber: '',
            consentGiven: false,
            applicationNumber: 1,

            allUnloadingPlaces: [],

            // Места разгрузки на уровне заявки (#706): единый выбор, синхронизируется
            // во все cars-вложения и уходит в items-вложения. Для items-без-машин это
            // единственный источник; для cars - дубль дедуп-union мест всех машин.
            applicationUnloadPlaces: [],

            organizationId: null,
            companyId: null,

            // Получатели заявки (#884): дефолтные согласующие орг/компании (показ,
            // удалить нельзя) и добавленные читатели (read-only доступ, уходят в payload.readers).
            defaultApprovers: [],
            readers: [],

            selectedAttachment: null,
            attachments: [],

            // #1183: предупреждения выбранных мест текущей формы для плавающей панели.
            placeNotices: [],

            licensePlateFormats: [],

            vehiclesByAttachment: {},
            employeesByAttachment: {},
            itemsByAttachment: {},
            
            attachmentDatesByAttachment: {},
            
            vehicleIdCounter: 1,
            employeeIdCounter: 1,
            itemIdCounter: 1,
            
            sortField: null,
            sortDirection: 'asc',
            
            errors: {
                organization: '',
                company: '',
                responsiblePerson: '',
                phone: ''
            },
            
            vehicleFormKey: 0,
            employeeFormKey: 0,
            itemsFormKey: 0,

            showBindingModal: false,
            newVehiclesToBind: [],
            newEmployeesToBind: [],
            hasOrganization: false,
            hasCompany: false,
            
            currentApplicationData: {},

            allPassageTables: [],

            showSuccessModal: false,
            createdApplicationNumber: '',
            createdAttachmentsData: [],

            showSubmitTooltip: false,
            tooltipTimer: null,

            customFieldsByAttachment: {},
            customFieldDefinitions: {},
            // Настройка полей по UniqueAttachment (#529): { [uaId]: { [fieldKey]: { visible, required, locked, requirable } } }.
            // Источник - GET /attachments/{id}/field-config (base). Раздаётся секциям формы пропсом field-config.
            fieldConfigByAttachment: {},
        }
    },
    computed: {
        currentFormTitle() {
            if (this.selectedAttachment) {
                return this.selectedAttachment.display_name;
            }
            return 'Добавьте новое вложение';
        },
        
        vehicles() {
            if (!this.selectedAttachment) return [];
            return this.vehiclesByAttachment[this.attachmentKey(this.selectedAttachment)] || [];
        },

        employees() {
            if (!this.selectedAttachment) return [];
            return this.employeesByAttachment[this.attachmentKey(this.selectedAttachment)] || [];
        },

        items() {
            if (!this.selectedAttachment) return [];
            return this.itemsByAttachment[this.attachmentKey(this.selectedAttachment)] || [];
        },

        hasCars() {
            return this.attachments.some(a => a.attachment_type === 'cars');
        },

        hasItems() {
            return this.attachments.some(a => a.attachment_type === 'items');
        },

        // Выбор мест разгрузки в форме ТМЦ показываем только когда машин нет:
        // при наличии машин единый выбор живёт в форме авто (#706).
        showItemsUnloadPlaces() {
            return !this.hasCars && this.hasItems;
        },

        // Для ТМЦ-без-машин место разгрузки обязательно: без него охранник не увидит вложение.
        itemsUnloadRequired() {
            return this.showItemsUnloadPlaces;
        },

        currentCustomFields() {
            if (!this.selectedAttachment) return [];
            const uaId = this.selectedAttachment.template_id || this.selectedAttachment.id;
            return this.customFieldDefinitions[uaId] || [];
        },

        currentCustomFieldValues() {
            if (!this.selectedAttachment) return {};
            const key = this.attachmentKey(this.selectedAttachment);
            return this.customFieldsByAttachment[key] || {};
        },

        currentFieldConfig() {
            if (!this.selectedAttachment) return {};
            const uaId = this.selectedAttachment.template_id || this.selectedAttachment.id;
            return this.fieldConfigByAttachment[uaId] || {};
        },

        currentAttachmentData() {
            if (!this.selectedAttachment) {
                return this.getDefaultDateData();
            }
            const data = this.attachmentDatesByAttachment[this.attachmentKey(this.selectedAttachment)];
            return data || this.getDefaultDateData();
        },

        // Орг/компания заявки и срок+время текущего вложения для карточек просмотра
        // в списках (сущности в форме этих полей не несут). Дата -> формат API
        // (YYYY-MM-DD), как ждут модалки; время остаётся HH:MM.
        detailViewInfo() {
            const d = this.currentAttachmentData;
            return {
                organization: this.organization,
                company: this.company,
                entryDateTo: this.formatDateForAPI(d.isOneDay ? d.singleDate : d.endDate),
                timeFrom: d.startTime || '',
                timeTo: d.endTime || ''
            };
        },

        // Срок текущего вложения в API-формате для авто-проверки расписания мест
        // (#1183 S5): формы сверяют его с time_slots выбранных мест. Даты -> YYYY-MM-DD,
        // время остаётся ЧЧ:ММ; пустые границы -> null (проверка их пропускает).
        currentEntryPeriod() {
            const d = this.currentAttachmentData;
            return {
                date_from: this.formatDateForAPI(d.isOneDay ? d.singleDate : d.startDate),
                date_to: this.formatDateForAPI(d.isOneDay ? d.singleDate : d.endDate),
                time_from: d.startTime || null,
                time_to: d.endTime || null
            };
        },

        currentAttachmentErrors() {
            if (!this.selectedAttachment) return {};
            const data = this.attachmentDatesByAttachment[this.attachmentKey(this.selectedAttachment)];
            return data?.errors || {};
        },
        
        submitValidation() {
            const reasons = [];

            if (this.attachments.length === 0) {
                reasons.push('Добавьте хотя бы одно вложение');
            }

            const missingFields = [];
            if (!this.organization?.trim() && !this.company?.trim()) missingFields.push('организация или компания');
            if (!this.responsiblePerson) missingFields.push('инициатор заявки');
            if (!this.phoneNumber) missingFields.push('телефон');
            if (!this.consentGiven) missingFields.push('согласие на обработку данных');
            if (missingFields.length > 0) {
                reasons.push(`Заполните поля: ${missingFields.join(', ')}`);
            }

            this.attachments.forEach(attachment => {
                const key = this.attachmentKey(attachment);
                const label = attachment.attachment_display_name || attachment.attachment_name || attachment.display_name || `вложение #${attachment.id}`;
                let hasAttachmentData;
                switch (attachment.attachment_type) {
                    case 'cars':
                        hasAttachmentData = (this.vehiclesByAttachment[key] || []).length > 0;
                        if (!hasAttachmentData) reasons.push(`"${label}": добавьте хотя бы одно авто`);
                        break;
                    case 'people':
                        hasAttachmentData = (this.employeesByAttachment[key] || []).length > 0;
                        if (!hasAttachmentData) reasons.push(`"${label}": добавьте хотя бы одного сотрудника`);
                        break;
                    case 'items':
                        hasAttachmentData = (this.itemsByAttachment[key] || []).length > 0;
                        if (!hasAttachmentData) reasons.push(`"${label}": добавьте хотя бы одну позицию`);
                        if (this.itemsUnloadRequired && this.applicationUnloadPlaces.length === 0) {
                            reasons.push(`"${label}": выберите место разгрузки`);
                        }
                        break;
                }

                const dateData = this.attachmentDatesByAttachment[key];
                if (!dateData) {
                    reasons.push(`"${label}": укажите даты действия`);
                    return;
                }
                if (!this.attachmentDatesComplete(dateData)) {
                    reasons.push(`"${label}": заполните даты и время действия`);
                }
                const emptyFields = this.emptyRequiredCustomFields(attachment, key);
                if (emptyFields.length > 0) {
                    reasons.push(`"${label}": заполните доп. поля: ${emptyFields.map(f => f.label).join(', ')}`);
                }
            });

            return reasons;
        },

        canSubmit() {
            return this.submitValidation.length === 0;
        },

        submitDisabledReason() {
            if (this.canSubmit) return '';
            return 'Для отправки заявки:\n- ' + this.submitValidation.join('\n- ');
        },

        tooltipSections() {
            const sections = [];

            const globalErrors = [];
            if (this.attachments.length === 0) globalErrors.push('Не добавлено ни одного вложения');
            if (!this.organization?.trim() && !this.company?.trim()) globalErrors.push('Не заполнена организация или компания');
            if (!this.responsiblePerson) globalErrors.push('Не указан инициатор заявки');
            if (!this.phoneNumber) globalErrors.push('Не указан номер телефона');
            if (!this.consentGiven) globalErrors.push('Не дано согласие на обработку данных');

            if (globalErrors.length) {
                sections.push({ type: 'global', messages: globalErrors });
            }

            for (const attachment of this.attachments) {
                const type = attachment.attachment_type;
                const key = this.attachmentKey(attachment);
                const displayName = attachment.display_name || attachment.attachment_display_name || attachment.attachment_name || `вложение #${attachment.id}`;
                const errors = [];

                let items = [];
                if (type === 'cars') items = this.vehiclesByAttachment[key] || [];
                else if (type === 'people') items = this.employeesByAttachment[key] || [];
                else if (type === 'items') items = this.itemsByAttachment[key] || [];

                if (type === 'cars' && items.length === 0) errors.push('Не добавлено ни одного автомобиля');
                else if (type === 'people' && items.length === 0) errors.push('Не добавлено ни одного сотрудника');
                else if (type === 'items' && items.length === 0) errors.push('Не добавлено ни одной позиции');

                if (type === 'items' && this.itemsUnloadRequired && this.applicationUnloadPlaces.length === 0) {
                    errors.push('Не выбрано место разгрузки');
                }

                const dateData = this.attachmentDatesByAttachment[key];
                if (dateData) {
                    let hasDates = false;
                    if (dateData.isOneDay) {
                        if (dateData.singleDate && dateData.startTime && dateData.endTime) hasDates = true;
                        else errors.push('Не указаны дата и время пребывания');
                    } else {
                        if (dateData.startDate && dateData.endDate && dateData.startTime && dateData.endTime) hasDates = true;
                        else errors.push('Не указан период действия (даты) или время пребывания');
                    }
                    if (hasDates && !dateData.isOneDay && dateData.startDate && dateData.endDate) {
                        const start = new Date(dateData.startDate.split('.').reverse().join('-'));
                        const end = new Date(dateData.endDate.split('.').reverse().join('-'));
                        if (start > end) errors.push('Дата окончания не может быть раньше даты начала');
                    }
                } else {
                    errors.push('Не заполнены даты действия и время пребывания');
                }

                const emptyCf = this.emptyRequiredCustomFields(attachment, key);
                if (emptyCf.length > 0) {
                    emptyCf.forEach(f => errors.push(`Не заполнено доп. поле "${f.label}"`));
                }

                if (errors.length) {
                    sections.push({
                        type: 'attachment',
                        attachmentKey: key,
                        attachmentType: type,
                        attachmentName: displayName,
                        messages: errors,
                    });
                }
            }

            return sections;
        },

        sortedVehicles() {
            if (!this.sortField || !this.vehicles.length) {
                return this.vehicles;
            }
            
            return [...this.vehicles].sort((a, b) => {
                let valueA, valueB;
                
                switch (this.sortField) {
                    case 'number':
                        return this.sortDirection === 'asc' ? a.id - b.id : b.id - a.id;
                    case 'plate':
                        valueA = a.plateNumber ? a.plateNumber.toLowerCase() : '';
                        valueB = b.plateNumber ? b.plateNumber.toLowerCase() : '';
                        break;
                    case 'mark':
                        valueA = a.mark ? a.mark.toLowerCase() : '';
                        valueB = b.mark ? b.mark.toLowerCase() : '';
                        break;
                    default:
                        return 0;
                }
                
                if (valueA < valueB) {
                    return this.sortDirection === 'asc' ? -1 : 1;
                }
                if (valueA > valueB) {
                    return this.sortDirection === 'asc' ? 1 : -1;
                }
                return 0;
            });
        },
        
        sortedEmployees() {
            if (!this.sortField || !this.employees.length) {
                return this.employees;
            }
            
            return [...this.employees].sort((a, b) => {
                let valueA, valueB;
                
                switch (this.sortField) {
                    case 'number':
                        return this.sortDirection === 'asc' ? a.id - b.id : b.id - a.id;
                    case 'name':
                        valueA = this.formatFullName(a).toLowerCase();
                        valueB = this.formatFullName(b).toLowerCase();
                        break;
                    case 'position':
                        valueA = a.position ? a.position.toLowerCase() : '';
                        valueB = b.position ? b.position.toLowerCase() : '';
                        break;
                    case 'tables':
                        valueA = a.passageTables ? a.passageTables.toLowerCase() : '';
                        valueB = b.passageTables ? b.passageTables.toLowerCase() : '';
                        break;
                    default:
                        return 0;
                }
                
                if (valueA < valueB) {
                    return this.sortDirection === 'asc' ? -1 : 1;
                }
                if (valueA > valueB) {
                    return this.sortDirection === 'asc' ? 1 : -1;
                }
                return 0;
            });
        },
        
        sortedItems() {
            if (!this.sortField || !this.items.length) {
                return this.items;
            }
            
            return [...this.items].sort((a, b) => {
                let valueA, valueB;
                
                switch (this.sortField) {
                    case 'number':
                        return this.sortDirection === 'asc' ? a.id - b.id : b.id - a.id;
                    case 'name':
                        valueA = a.itemName ? a.itemName.toLowerCase() : '';
                        valueB = b.itemName ? b.itemName.toLowerCase() : '';
                        break;
                    case 'quantity':
                        return this.sortDirection === 'asc' ? a.quantity - b.quantity : b.quantity - a.quantity;
                    default:
                        return 0;
                }
                
                if (valueA < valueB) {
                    return this.sortDirection === 'asc' ? -1 : 1;
                }
                if (valueA > valueB) {
                    return this.sortDirection === 'asc' ? 1 : -1;
                }
                return 0;
            });
        }
    },
    mounted() {
        this.restoreFromLocalStorage();
        this.checkPendingDuplicate();
        this.loadUserData();
        this.loadAllUnloadingPlaces();
        this.loadLicensePlateFormats();
        this.loadPassageTables();

        window.addEventListener('beforeunload', () => {
            this.saveCurrentAttachmentData();
            this.saveToLocalStorage();
        });
    },
    beforeUnmount() {
        window.removeEventListener('beforeunload', this.saveToLocalStorage);
    },
    methods: {

        async loadPassageTables() {
            try {
                const response = await apiRequest("/system-tables", {
                    method: "GET"});

                if (response.ok) {
                    const tables = await response.json();
                    console.log('Загруженные таблицы в CreateApplication:', tables);
                    this.allPassageTables = tables;
                } else {
                    console.error("Ошибка при загрузке системных таблиц");
                }
            } catch (error) {
                console.error("Ошибка при загрузке таблиц:", error);
            }
        },
        async checkVehiclesBeforeSubmit() {
  const activeVehicles = [];

  for (const attachment of this.attachments) {
    if (attachment.attachment_type !== 'cars') continue;

    const vehicles = this.vehiclesByAttachment[this.attachmentKey(attachment)] || [];
    
    for (const vehicle of vehicles) {
      try {
        const params = new URLSearchParams();
        params.append('car_number', vehicle.plateNumber);
        params.append('car_brand', vehicle.mark);
        if (this.organizationId) params.append('organization_id', this.organizationId);
        if (this.companyId) params.append('company_id', this.companyId);

        const response = await apiRequest(`/cars/check-active?${params.toString()}`, {});

        if (response.ok) {
          const data = await response.json();
          if (data.active) {
            activeVehicles.push({
              ...vehicle,
              activeInfo: data
            });
          }
        }
      } catch (error) {
        console.error('Ошибка при проверке авто:', error);
      }
    }
  }
  
  return activeVehicles;
},
        async loadLicensePlateFormats() {
            try {
                const response = await apiRequest("/license-plate-formats", {
                    method: "GET"});

                if (response.ok) {
                    this.licensePlateFormats = await response.json();
                } else {
                    console.error("Ошибка при загрузке форматов номеров");
                }
            } catch (error) {
                console.error("Ошибка при загрузке форматов номеров:", error);
            }
        },

        async loadAllUnloadingPlaces() {
            try {
                const response = await apiRequest("/unload-places", {
                    method: "GET"});

                if (response.ok) {
                    this.allUnloadingPlaces = await response.json();
                }
            } catch (error) {
                console.error("Ошибка при загрузке мест разгрузки:", error);
            }
        },

        // Единый обработчик изменения мест разгрузки заявки (#706): источник -
        // форма авто или форма ТМЦ. Пишем app-level выбор и раскатываем его во все
        // машины cars-вложений, чтобы дедуп-union на бэке совпал с выбором.
        onApplicationUnloadPlacesChange(placeIds) {
            this.applicationUnloadPlaces = Array.isArray(placeIds) ? [...placeIds] : [];
            this.syncUnloadPlacesToCars();
            this.saveToLocalStorage();
        },

        syncUnloadPlacesToCars() {
            const ids = this.applicationUnloadPlaces;
            const formatted = this.formatUnloadPlacesDisplay(ids);
            this.attachments.forEach(attachment => {
                if (attachment.attachment_type !== 'cars') return;
                const vehicles = this.vehiclesByAttachment[this.attachmentKey(attachment)];
                if (!vehicles) return;
                vehicles.forEach(vehicle => {
                    vehicle.unloadPlaces = [...ids];
                    vehicle.unloadingPlace = formatted;
                });
            });
        },

        formatUnloadPlacesDisplay(ids) {
            if (!ids || ids.length === 0) return '';
            const names = ids
                .map(id => {
                    const place = this.allUnloadingPlaces.find(p => p.id === id);
                    return place ? place.name : '';
                })
                .filter(Boolean);
            if (names.length === 0) return '';
            return names.length > 1 ? `${names[0]} и др.` : names[0];
        },
        
        getDefaultDateData() {
            return {
                isOneDay: false,
                startDate: '',
                endDate: '',
                singleDate: '',
                startTime: '',
                endTime: '',
                roofAccess: false,
                freeParking: false,
                errors: {}
            };
        },

        // Дата+время вложения заполнены полностью (одиночная дата либо диапазон).
        attachmentDatesComplete(dateData) {
            if (!dateData) return false;
            return dateData.isOneDay
                ? !!(dateData.singleDate && dateData.startTime && dateData.endTime)
                : !!(dateData.startDate && dateData.endDate && dateData.startTime && dateData.endTime);
        },

        // Незаполненные обязательные доп. поля вложения.
        emptyRequiredCustomFields(attachment, key) {
            const uaId = attachment.template_id || attachment.id;
            const fields = this.customFieldDefinitions[uaId] || [];
            const values = this.customFieldsByAttachment[key] || {};
            return fields.filter(f => f.is_required && (!values[f.id] || !values[f.id].trim()));
        },

        updateAttachmentData(field, value) {
            if (!this.selectedAttachment) return;
            
            const attachmentId = this.attachmentKey(this.selectedAttachment);
            
            if (!this.attachmentDatesByAttachment[attachmentId]) {
                this.attachmentDatesByAttachment[attachmentId] = this.getDefaultDateData();
            }
            
            this.attachmentDatesByAttachment[attachmentId][field] = value;
            
            this.saveToLocalStorage();
        },
        
        validateAttachmentField(field) {
            if (!this.selectedAttachment) return;
            
            const attachmentId = this.attachmentKey(this.selectedAttachment);
            const dateData = this.attachmentDatesByAttachment[attachmentId];
            if (!dateData) return;
            
            let timeRegex;
            
            if (!dateData.errors) {
                dateData.errors = {};
            }
            
            switch (field) {
                case 'startDate':
                    dateData.errors.startDate = dateData.isOneDay ? '' : (dateData.startDate ? '' : 'Укажите дату начала');
                    break;
                case 'endDate':
                    dateData.errors.endDate = dateData.isOneDay ? '' : (dateData.endDate ? '' : 'Укажите дату окончания');
                    break;
                case 'singleDate':
                    dateData.errors.singleDate = !dateData.isOneDay ? '' : (dateData.singleDate ? '' : 'Укажите дату');
                    break;
                case 'startTime':
                    timeRegex = /^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$/;
                    dateData.errors.startTime = dateData.startTime && timeRegex.test(dateData.startTime) ? '' : 'Введите время в формате ЧЧ:ММ';
                    break;
                case 'endTime':
                    timeRegex = /^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$/;
                    dateData.errors.endTime = dateData.endTime && timeRegex.test(dateData.endTime) ? '' : 'Введите время в формате ЧЧ:ММ';
                    break;
            }
            
            this.saveToLocalStorage();
        },
        
        validateAttachmentDateRange() {
            if (!this.selectedAttachment) return;
            
            const attachmentId = this.attachmentKey(this.selectedAttachment);
            const dateData = this.attachmentDatesByAttachment[attachmentId];
            if (!dateData || !dateData.errors) return;
            
            if (!dateData.isOneDay && dateData.startDate && dateData.endDate) {
                const start = new Date(dateData.startDate.split('.').reverse().join('-'));
                const end = new Date(dateData.endDate.split('.').reverse().join('-'));
                if (start > end) {
                    dateData.errors.endDate = 'Дата окончания не может быть раньше даты начала';
                } else {
                    dateData.errors.endDate = '';
                }
            }
            
            this.saveToLocalStorage();
        },
        
        validateAttachmentTimeRange() {
            if (!this.selectedAttachment) return;
            
            const attachmentId = this.attachmentKey(this.selectedAttachment);
            const dateData = this.attachmentDatesByAttachment[attachmentId];
            if (!dateData || !dateData.errors) return;
            
            // Время окончания раньше начала больше не ошибка: DateRangeSection авто-переносит
            // окончание на следующий день (получается валидный кросс-дневной диапазон).
            dateData.errors.endTime = '';

            this.saveToLocalStorage();
        },
        
        clearFormData() {
            if (this.attachments.length === 0 && this.selectedAttachment === null) {
                this.message = '';
                this.consentGiven = false;
                
                this.errors = {
                    organization: '',
                    company: '',
                    responsiblePerson: '',
                    phone: ''
                };
            }
        },
        
        async loadUserData() {
            const authStore = useAuthStore();
            if (!authStore.token) {
                console.error("Токен не найден");
                return;
            }

            try {
                const response = await apiRequest("/user-data", {
                    method: "GET"});

                if (response.ok) {
                    const userData = await response.json();
                    this.organization = userData.organization || '';
                    this.company = userData.company || '';
                    
                    this.organizationId = userData.organization_id || null;
                    this.companyId = userData.company_id || null;
                    
                    this.hasOrganization = !!this.organizationId;
                    this.hasCompany = !!this.companyId;

                    this.loadDefaultApprovers();

                    const lastName = userData.last_name || '';
                    const firstName = userData.first_name || '';
                    const middleName = userData.middle_name || '';
                    this.responsiblePerson = `${lastName} ${firstName} ${middleName}`.trim();
                    
                    this.phoneNumber = userData.phone || '';
                    if (this.phoneNumber) {
                        const { raw, formatted } = formatPhoneNumberImmediately(this.phoneNumber);
                        this.rawPhoneNumber = raw;
                        this.phoneNumber = formatted;
                    }
                    
                } else {
                    console.error("Ошибка загрузки данных пользователя");
                }
            } catch (error) {
                console.error("Ошибка:", error);
            }
        },

        /**
         * Дефолтные согласующие заявки (#884) - ответственные орг/компании с
         * required_approval. Показываются как неудаляемые чипы-получатели; бэк и так
         * добавляет их в ответственные при подаче, тут только отображение.
         */
        async loadDefaultApprovers() {
            const byId = new Map();
            const collect = async (url) => {
                try {
                    const r = await apiRequest(url, {});
                    if (!r.ok) return;
                    const users = await r.json();
                    (users || []).forEach(u => {
                        if (u.required_approval && !byId.has(u.id)) {
                            byId.set(u.id, { user_id: u.id, name: this.userDisplayName(u) });
                        }
                    });
                } catch (error) {
                    console.error('Ошибка загрузки согласующих:', error);
                }
            };
            if (this.organizationId) await collect(`/organizations/${this.organizationId}/users`);
            if (this.companyId) await collect(`/companies/${this.companyId}/users`);
            this.defaultApprovers = Array.from(byId.values());
        },

        userDisplayName(u) {
            const names = [u.last_name, u.first_name, u.middle_name].filter(Boolean);
            return names.length ? names.join(' ') : (u.username || '');
        },

        handleFormatPhoneNumber() {
            const { raw, formatted } = formatPhoneNumber(this.phoneNumber);
            this.rawPhoneNumber = raw;
            this.phoneNumber = formatted;
            this.validateField('phone');
        },

        handleClearPhoneFormat() {
            this.phoneNumber = clearPhoneFormat(this.rawPhoneNumber);
        },

        updateCustomFieldValues(values) {
            const key = this.attachmentKey(this.selectedAttachment);
            this.customFieldsByAttachment[key] = values;
        },

        async loadFieldConfig(uniqueAttachmentId) {
            if (this.fieldConfigByAttachment[uniqueAttachmentId]) return;
            try {
                const { getFieldConfig } = await import('@/api/attachment-templates');
                const data = await getFieldConfig(uniqueAttachmentId);
                const base = Array.isArray(data?.base) ? data.base : [];
                const map = {};
                base.forEach((f) => {
                    map[f.key] = {
                        visible: f.visible,
                        required: f.required,
                        locked: f.locked,
                        requirable: f.requirable,
                    };
                });
                this.fieldConfigByAttachment[uniqueAttachmentId] = map;
                this.customFieldDefinitions[uniqueAttachmentId] = Array.isArray(data?.custom) ? data.custom : [];
            } catch {
                // Конфиг недоступен (сеть/транзиентная ошибка) - деградируем к дефолту:
                // дефолт = все поля видимы, обязательность как сейчас (спека #529). Пустой
                // объект falsy -> следующий выбор вложения повторит запрос.
                this.fieldConfigByAttachment[uniqueAttachmentId] = {};
                this.customFieldDefinitions[uniqueAttachmentId] = [];
            }
        },

        handleAttachmentSelected(attachment) {
            // Смена вложения - гасим панель предупреждений; новая форма пришлёт свои
            // группы через @notices-change (иначе стейл-группы прошлого вложения
            // мелькнут до пересчёта). Это реальный путь переключения между вложениями.
            this.placeNotices = [];

            if (!attachment) {
                this.selectedAttachment = null;
                this.clearFormData();
                return;
            }

            this.selectedAttachment = attachment;
            this.restoreAttachmentData(attachment);

            const uaId = attachment.template_id || attachment.id;
            this.loadFieldConfig(uaId);
        },

        attachmentKey(attachment) {
            if (!attachment) return null;
            return attachment.local_id || attachment.id;
        },

        tooltipMouseEnter() {
            if (this.tooltipTimer) {
                clearTimeout(this.tooltipTimer);
                this.tooltipTimer = null;
            }
            this.showSubmitTooltip = true;
        },

        tooltipMouseLeave() {
            this.tooltipTimer = setTimeout(() => {
                this.showSubmitTooltip = false;
            }, 200);
        },

        handleTooltipClick(event) {
            const target = event.target.closest('.attachment-clickable');
            if (target) {
                const attachmentKey = target.dataset.attachmentKey;
                const attachment = this.attachments.find(a => String(this.attachmentKey(a)) === String(attachmentKey));
                if (attachment) {
                    this.handleAttachmentSelected(attachment);
                    this.showSubmitTooltip = false;
                }
            }
        },

        handleTooltipAttachmentClick(section) {
            if (!section.attachmentKey) return;
            const attachment = this.attachments.find(a => String(this.attachmentKey(a)) === String(section.attachmentKey));
            if (attachment) {
                this.handleAttachmentSelected(attachment);
                this.showSubmitTooltip = false;
            }
        },

        handleAttachmentAdded(attachment) {
            this.attachments.push(attachment);

            const key = this.attachmentKey(attachment);
            if (attachment.attachment_type === 'cars') {
                this.vehiclesByAttachment[key] = [];
            } else if (attachment.attachment_type === 'people') {
                this.employeesByAttachment[key] = [];
            } else if (attachment.attachment_type === 'items') {
                this.itemsByAttachment[key] = [];
            }

            this.attachmentDatesByAttachment[key] = this.getDefaultDateData();

            this.selectAttachment(attachment);

            const uaId = attachment.template_id || attachment.id;
            this.loadFieldConfig(uaId);

            this.saveToLocalStorage();
        },

        selectAttachment(attachment) {
            this.selectedAttachment = attachment;
        },

        handleAttachmentRenamed({ attachment, display_name }) {
            const key = this.attachmentKey(attachment);
            // attachments - источник истины для payload (сериализуется отсюда) и для чипов.
            const target = this.attachments.find(a => this.attachmentKey(a) === key);
            if (target) target.display_name = display_name;
            // selectedAttachment может быть копией (withCurrentInstruction) - обновляем и его,
            // чтобы заголовок формы (currentFormTitle) подхватил новое имя.
            if (this.selectedAttachment && this.attachmentKey(this.selectedAttachment) === key) {
                this.selectedAttachment.display_name = display_name;
            }
            this.saveToLocalStorage();
        },

        handleAttachmentRemoved(attachment) {
            const key = this.attachmentKey(attachment);
            this.attachments = this.attachments.filter(a => this.attachmentKey(a) !== key);

            if (attachment.attachment_type === 'cars') {
                delete this.vehiclesByAttachment[key];
            } else if (attachment.attachment_type === 'people') {
                delete this.employeesByAttachment[key];
            } else if (attachment.attachment_type === 'items') {
                delete this.itemsByAttachment[key];
            }

            delete this.attachmentDatesByAttachment[key];

            if (this.selectedAttachment && this.attachmentKey(this.selectedAttachment) === key) {
                this.selectedAttachment = null;
                // П.25: при удалении выбранного вложения открываем первое оставшееся (сверху).
                if (this.attachments.length > 0) {
                    this.handleAttachmentSelected(this.attachments[0]);
                }
            }

            this.clearFormData();

            this.saveToLocalStorage();
        },

        restoreAttachmentData(attachment) {
            if (this.selectedAttachment) {
                this.saveCurrentAttachmentData();
            }
            
            this.loadAttachmentData(attachment);
        },

        saveCurrentAttachmentData() {
            if (!this.selectedAttachment) return;

            const key = this.attachmentKey(this.selectedAttachment);
            switch (this.selectedAttachment.attachment_type) {
                case 'cars':
                    this.vehiclesByAttachment[key] = this.vehicles;
                    break;
                case 'people':
                    this.employeesByAttachment[key] = this.employees;
                    break;
                case 'items':
                    this.itemsByAttachment[key] = this.items;
                    break;
            }

            this.saveToLocalStorage();
        },

        loadAttachmentData(attachment) {
            if (!attachment) return;

            const key = this.attachmentKey(attachment);
            switch (attachment.attachment_type) {
                case 'cars':
                    if (!this.vehiclesByAttachment[key]) {
                        this.vehiclesByAttachment[key] = [];
                    }
                    break;
                case 'people':
                    if (!this.employeesByAttachment[key]) {
                        this.employeesByAttachment[key] = [];
                    }
                    break;
                case 'items':
                    if (!this.itemsByAttachment[key]) {
                        this.itemsByAttachment[key] = [];
                    }
                    break;
            }

            if (!this.attachmentDatesByAttachment[key]) {
                this.attachmentDatesByAttachment[key] = this.getDefaultDateData();
            }
        },

        handleVehicleAdded(newVehicle) {
            const vehicleWithId = {
                ...newVehicle,
                id: this.vehicleIdCounter++,
                isExisting: false,
                organization: this.organization,
                organizationId: this.organizationId,
                company: this.company,
                companyId: this.companyId
            };
            
            if (this.selectedAttachment) {
                if (!this.vehiclesByAttachment[this.attachmentKey(this.selectedAttachment)]) {
                    this.vehiclesByAttachment[this.attachmentKey(this.selectedAttachment)] = [];
                }
                this.vehiclesByAttachment[this.attachmentKey(this.selectedAttachment)].push(vehicleWithId);
                
                this.saveToLocalStorage();
            }
        },

        handleVehiclesAdded(vehicles) {
            vehicles.forEach(vehicle => {
                const vehicleWithId = {
                    ...vehicle,
                    id: this.vehicleIdCounter++,
                    isExisting: false,
                    organization: this.organization,
                    organizationId: this.organizationId,
                    company: this.company,
                    companyId: this.companyId
                };
                
                if (this.selectedAttachment) {
                    if (!this.vehiclesByAttachment[this.attachmentKey(this.selectedAttachment)]) {
                        this.vehiclesByAttachment[this.attachmentKey(this.selectedAttachment)] = [];
                    }
                    this.vehiclesByAttachment[this.attachmentKey(this.selectedAttachment)].push(vehicleWithId);
                }
            });
            
            this.saveToLocalStorage();
        },

        handleVehicleUpdated(updatedVehicle) {
            if (!this.selectedAttachment) return;
            
            const vehicles = this.vehiclesByAttachment[this.attachmentKey(this.selectedAttachment)];
            if (!vehicles) return;
            
            const index = vehicles.findIndex(v => v.id === updatedVehicle.id);
            if (index !== -1) {
                vehicles.splice(index, 1, updatedVehicle);
                this.saveToLocalStorage();
            }
        },

        handleVehicleEditCancelled() {
            this.vehicleFormKey += 1;
        },

        deleteVehicle(vehicleId) {
            if (!this.selectedAttachment) return;
            
            const vehicles = this.vehiclesByAttachment[this.attachmentKey(this.selectedAttachment)];
            if (!vehicles) return;
            
            const index = vehicles.findIndex(vehicle => vehicle.id === vehicleId);
            if (index !== -1) {
                vehicles.splice(index, 1);
                this.saveToLocalStorage();
            }
        },

        editVehicle(vehicle) {
            if (this.$refs.vehicleForm) {
                this.$refs.vehicleForm.editVehicle(vehicle);
            }
        },

        handleEmployeeAdded(newEmployee) {
            const employeeWithId = {
                ...newEmployee,
                id: this.employeeIdCounter++,
                isExisting: false
            };
            
            if (this.selectedAttachment) {
                if (!this.employeesByAttachment[this.attachmentKey(this.selectedAttachment)]) {
                    this.employeesByAttachment[this.attachmentKey(this.selectedAttachment)] = [];
                }
                this.employeesByAttachment[this.attachmentKey(this.selectedAttachment)].push(employeeWithId);
                
                this.saveToLocalStorage();
            }
        },

        handleEmployeesAdded(employees) {
            employees.forEach(employee => {
                const employeeWithId = {
                    ...employee,
                    id: this.employeeIdCounter++,
                    isExisting: false
                };
                
                if (this.selectedAttachment) {
                    if (!this.employeesByAttachment[this.attachmentKey(this.selectedAttachment)]) {
                        this.employeesByAttachment[this.attachmentKey(this.selectedAttachment)] = [];
                    }
                    this.employeesByAttachment[this.attachmentKey(this.selectedAttachment)].push(employeeWithId);
                }
            });
            
            this.saveToLocalStorage();
        },

        handleEmployeeUpdated(updatedEmployee) {
            if (!this.selectedAttachment) return;
            
            const employees = this.employeesByAttachment[this.attachmentKey(this.selectedAttachment)];
            if (!employees) return;
            
            const index = employees.findIndex(e => e.id === updatedEmployee.id);
            if (index !== -1) {
                employees.splice(index, 1, updatedEmployee);
                this.saveToLocalStorage();
            }
        },

        handleEmployeeEditCancelled() {
            this.employeeFormKey += 1;
        },

        deleteEmployee(employeeId) {
            if (!this.selectedAttachment) return;
            
            const employees = this.employeesByAttachment[this.attachmentKey(this.selectedAttachment)];
            if (!employees) return;
            
            const index = employees.findIndex(employee => employee.id === employeeId);
            if (index !== -1) {
                employees.splice(index, 1);
                this.saveToLocalStorage();
            }
        },

        editEmployee(employee) {
            if (this.$refs.employeeForm) {
                this.$refs.employeeForm.editEmployee(employee);
            }
        },

        formatFullName(employee) {
            const parts = [];
            if (employee.lastName) parts.push(employee.lastName);
            if (employee.firstName) parts.push(employee.firstName);
            if (employee.middleName) parts.push(employee.middleName);
            return parts.join(' ') || 'Не указано';
        },

        handleItemAdded(newItem) {
            const itemWithId = {
                ...newItem,
                id: this.itemIdCounter++
            };
            
            if (this.selectedAttachment) {
                if (!this.itemsByAttachment[this.attachmentKey(this.selectedAttachment)]) {
                    this.itemsByAttachment[this.attachmentKey(this.selectedAttachment)] = [];
                }
                this.itemsByAttachment[this.attachmentKey(this.selectedAttachment)].push(itemWithId);
                
                this.saveToLocalStorage();
            }
        },

        handleItemsAdded(items) {
            items.forEach(item => {
                const itemWithId = {
                    ...item,
                    id: this.itemIdCounter++
                };
                
                if (this.selectedAttachment) {
                    if (!this.itemsByAttachment[this.attachmentKey(this.selectedAttachment)]) {
                        this.itemsByAttachment[this.attachmentKey(this.selectedAttachment)] = [];
                    }
                    this.itemsByAttachment[this.attachmentKey(this.selectedAttachment)].push(itemWithId);
                }
            });
            
            this.saveToLocalStorage();
        },

        handleItemUpdated(updatedItem) {
            if (!this.selectedAttachment) return;
            
            const items = this.itemsByAttachment[this.attachmentKey(this.selectedAttachment)];
            if (!items) return;
            
            const index = items.findIndex(e => e.id === updatedItem.id);
            if (index !== -1) {
                items.splice(index, 1, updatedItem);
                this.saveToLocalStorage();
            }
        },

        handleItemEditCancelled() {
            this.itemsFormKey += 1;
        },

        deleteItem(itemId) {
            if (!this.selectedAttachment) return;
            
            const items = this.itemsByAttachment[this.attachmentKey(this.selectedAttachment)];
            if (!items) return;
            
            const index = items.findIndex(item => item.id === itemId);
            if (index !== -1) {
                items.splice(index, 1);
                this.saveToLocalStorage();
            }
        },

        editItem(item) {
            if (this.$refs.itemsForm) {
                this.$refs.itemsForm.editItem(item);
            }
        },

        sortBy(field) {
            if (this.sortField === field) {
                this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
            } else {
                this.sortField = field;
                this.sortDirection = 'asc';
            }
        },

        validateField(field) {
            let phoneRegex;

            switch (field) {
                case 'organization':
                case 'company': {
                    // Достаточно заполнить организацию ИЛИ компанию - не оба поля.
                    const orgOrCompany = this.organization?.trim() || this.company?.trim()
                        ? '' : 'Укажите организацию или компанию';
                    this.errors.organization = orgOrCompany;
                    this.errors.company = orgOrCompany;
                    break;
                }
                case 'responsiblePerson':
                    this.errors.responsiblePerson = this.responsiblePerson ? '' : 'Обязательное поле';
                    break;
                case 'phone':
                    phoneRegex = /^(\+7|8)?[\s-]?\(?[489][0-9]{2}\)?[\s-]?[0-9]{3}[\s-]?[0-9]{2}[\s-]?[0-9]{2}$/;
                    this.errors.phone = this.phoneNumber ? (phoneRegex.test(this.rawPhoneNumber) ? '' : 'Введите корректный номер') : 'Обязательное поле';
                    break;
            }
        },

        validateAllFields() {
            this.validateField('organization');
            this.validateField('company');
            this.validateField('responsiblePerson');
            this.validateField('phone');
            
            Object.keys(this.attachmentDatesByAttachment).forEach(attachmentId => {
                const dateData = this.attachmentDatesByAttachment[attachmentId];
                if (dateData) {
                    this.validateAttachmentDateRangeForAttachment(attachmentId);
                    this.validateAttachmentTimeRangeForAttachment(attachmentId);
                }
            });
        },
        
        validateAttachmentDateRangeForAttachment(attachmentId) {
            const dateData = this.attachmentDatesByAttachment[attachmentId];
            if (!dateData || !dateData.errors) return;
            
            if (!dateData.isOneDay && dateData.startDate && dateData.endDate) {
                const start = new Date(dateData.startDate.split('.').reverse().join('-'));
                const end = new Date(dateData.endDate.split('.').reverse().join('-'));
                if (start > end) {
                    dateData.errors.endDate = 'Дата окончания не может быть раньше даты начала';
                } else {
                    dateData.errors.endDate = '';
                }
            }
        },
        
        validateAttachmentTimeRangeForAttachment(attachmentId) {
            const dateData = this.attachmentDatesByAttachment[attachmentId];
            if (!dateData || !dateData.errors) return;
            
            // см. validateAttachmentTimeRange: время окончания раньше начала - не ошибка.
            dateData.errors.endTime = '';
        },

        async submitApplication() {
            this.validateAllFields();
            
            if (!this.canSubmit) {
                useDeletionsStore().notify({ prefix: 'Заполните все обязательные поля во всех вложениях', type: 'error' });
                return;
            }

            // Проверяем активные машины
  const activeVehicles = await this.checkVehiclesBeforeSubmit();
  
  if (activeVehicles.length > 0) {
    const vehicleList = activeVehicles.map(v => `${v.plateNumber} ${v.mark}`).join(', ');
    useDeletionsStore().notify({ prefix: 'Невозможно отправить заявку. Уже имеют активные заявки: ', bold: vehicleList, type: 'error' });
    return;
  }

            let hasDateErrors = false;
            let errorMessage = '';
            
            this.attachments.forEach(attachment => {
                const dateData = this.attachmentDatesByAttachment[this.attachmentKey(attachment)];
                if (dateData) {
                    if (!dateData.isOneDay && dateData.startDate && dateData.endDate) {
                        const start = new Date(dateData.startDate.split('.').reverse().join('-'));
                        const end = new Date(dateData.endDate.split('.').reverse().join('-'));
                        if (start > end) {
                            hasDateErrors = true;
                            errorMessage = `В вложении "${attachment.display_name}" дата окончания не может быть раньше даты начала`;
                        }
                    }

                }
            });

            if (hasDateErrors) {
                useDeletionsStore().notify({ prefix: errorMessage, type: 'error' });
                return;
            }

            await this.collectNewDataForBinding();

            const vehiclesForBinding = this.newVehiclesToBind.filter(vehicle => 
                vehicle.plateNumber !== 'По факту' && vehicle.mark !== 'По факту'
            );
            
            const employeesForBinding = this.newEmployeesToBind.filter(employee => 
                employee.passportSeriesNumber !== 'По факту' && employee.position !== 'По факту'
            );

            if (vehiclesForBinding.length > 0 || employeesForBinding.length > 0) {
                this.showBindingModal = true;
            } else {
                await this.sendCompleteApplication();
            }
        },

        async collectNewDataForBinding() {
            this.newVehiclesToBind = [];
            this.newEmployeesToBind = [];
            
            const existingVehicles = await this.loadExistingVehicles();
            const existingEmployees = await this.loadExistingEmployees();
            
            Object.keys(this.vehiclesByAttachment).forEach(attachmentId => {
                const vehicles = this.vehiclesByAttachment[attachmentId] || [];
                vehicles.forEach(vehicle => {
                    if (!vehicle.isExisting) {
                        const isByFact = vehicle.plateNumber === 'По факту' || vehicle.mark === 'По факту';
                        
                        if (!isByFact) {
                            const alreadyExists = existingVehicles.some(existingVehicle => 
                                existingVehicle.number === vehicle.plateNumber && 
                                existingVehicle.mark === vehicle.mark
                            );
                            
                            if (!alreadyExists) {
                                this.newVehiclesToBind.push({
                                    ...vehicle,
                                    attachmentId: attachmentId
                                });
                            }
                        }
                    }
                });
            });
            
            Object.keys(this.employeesByAttachment).forEach(attachmentId => {
                const employees = this.employeesByAttachment[attachmentId] || [];
                employees.forEach(employee => {
                    if (!employee.isExisting) {
                        const isByFact = employee.passportSeriesNumber === 'По факту' || 
                                       employee.position === 'По факту';
                        
                        if (!isByFact) {
                            const alreadyExists = existingEmployees.some(existingEmployee => 
                                existingEmployee.passport_series_number === employee.passportSeriesNumber
                            );
                            
                            if (!alreadyExists) {
                                this.newEmployeesToBind.push({
                                    ...employee,
                                    attachmentId: attachmentId
                                });
                            }
                        }
                    }
                });
            });
        },

        async loadExistingVehicles() {
            try {
                const response = await apiRequest("/unique-cars?filter_type=all", {
                    method: "GET"});

                if (response.ok) {
                    return await response.json();
                }
                return [];
            } catch (error) {
                console.error("Ошибка при загрузке существующих автомобилей:", error);
                return [];
            }
        },

        async loadExistingEmployees() {
            try {
                const response = await apiRequest("/unique-employees?filter_type=all", {
                    method: "GET"});

                if (response.ok) {
                    return await response.json();
                }
                return [];
            } catch (error) {
                console.error("Ошибка при загрузке существующих сотрудников:", error);
                return [];
            }
        },
        
        async confirmBinding(bindingData) {
            try {
                if (this.newVehiclesToBind.length > 0 && bindingData.vehicles.hasVehiclesForBinding) {
                    const vehiclesToBind = this.newVehiclesToBind.filter(vehicle => 
                        vehicle.plateNumber !== 'По факту' && vehicle.mark !== 'По факту'
                    );
                    
                    if (vehiclesToBind.length > 0) {
                        const vehiclePromises = vehiclesToBind.map(vehicle => {
                            const vehicleData = {
                                number: vehicle.plateNumber,
                                mark: vehicle.mark,
                                mark_id: vehicle.markId || null,
                                mark_name: vehicle.markName || vehicle.mark || null,
                                user_id: null,
                                organization_id: bindingData.vehicles.bindToOrganization ? this.organizationId : null,
                                company_id: bindingData.vehicles.bindToCompany ? this.companyId : null,
                                format_id: vehicle.formatId
                            };

                            return apiRequest("/unique-cars", {
                                method: "POST",
                                body: JSON.stringify(vehicleData)
                            });
                        });

                        await Promise.all(vehiclePromises);
                    }
                }

                if (this.newEmployeesToBind.length > 0 && bindingData.employees.hasEmployeesForBinding) {
                    const employeesToBind = this.newEmployeesToBind.filter(employee => 
                        employee.passportSeriesNumber !== 'По факту' && employee.position !== 'По факту'
                    );
                    
                    if (employeesToBind.length > 0) {
                        const employeePromises = employeesToBind.map(employee => {
                            const employeeData = {
                                last_name: employee.lastName,
                                first_name: employee.firstName,
                                middle_name: employee.middleName,
                                position: employee.position,
                                citizenship_id: employee.citizenshipId,
                                passport_series_number: employee.passportSeriesNumber,
                                patent_number: employee.patentNumber,
                                other_permission: employee.otherPermission,
                                user_id: null,
                                organization_id: bindingData.employees.bindToOrganization ? this.organizationId : null,
                                company_id: bindingData.employees.bindToCompany ? this.companyId : null
                            };

                            return apiRequest("/unique-employees", {
                                method: "POST",
                                body: JSON.stringify(employeeData)
                            });
                        });

                        await Promise.all(employeePromises);
                    }
                }

                this.closeBindingModal();
                await this.sendCompleteApplication();

            } catch (error) {
                console.error('Ошибка при привязке:', error);
                this.closeBindingModal();
            }
        },

        skipBinding() {
            this.closeBindingModal();
            this.sendCompleteApplication();
        },

        closeBindingModal() {
            this.showBindingModal = false;
            this.newVehiclesToBind = [];
            this.newEmployeesToBind = [];
        },

        onSuccessClose() {
            this.showSuccessModal = false;
            this.resetForm();
        },

        clearAllAttachments() {
            this.vehiclesByAttachment = {};
            this.employeesByAttachment = {};
            this.itemsByAttachment = {};
            this.attachmentDatesByAttachment = {};
            
            this.vehicleIdCounter = 1;
            this.employeeIdCounter = 1;
            this.itemIdCounter = 1;
            
            this.vehicleFormKey += 1;
            this.employeeFormKey += 1;
            this.itemsFormKey += 1;
            
            this.message = '';
            this.consentGiven = false;
            
            this.errors = {
                organization: '',
                company: '',
                responsiblePerson: '',
                phone: ''
            };
            
            if (this.$refs.blankSelector) {
                this.$refs.blankSelector.clearSelection();
            }
            this.selectedAttachment = null;
            this.attachments = [];
            this.readers = [];

            this.clearLocalStorageAfterSubmit();
        },

        async sendCompleteApplication() {
            if (this.attachments.length === 0) {
                useDeletionsStore().notify({ prefix: 'Добавьте вложения для отправки', type: 'error' });
                return;
            }

            const applicationData = {
                message: this.message || null,
                organization: this.organization,
                company: this.company || null,
                responsible_person: this.responsiblePerson,
                contact_phone: this.phoneNumber.replace(/\D/g, ''),
                data_approval: this.consentGiven,
                // Читатели-получатели (#884): только просмотр заявки после подачи.
                readers: this.readers.map(r => r.user_id),
                attachments: []
            };

            for (const attachment of this.attachments) {
                const key = this.attachmentKey(attachment);
                const dateData = this.attachmentDatesByAttachment[key] || this.getDefaultDateData();

                const attachmentData = {
                    attachment_type: attachment.attachment_type,
                    attachment_name: attachment.name,
                    attachment_display_name: attachment.display_name,
                    unique_attachment_id: attachment.template_id || attachment.id,
                    entry_date_from: this.formatDateForAPI(dateData.isOneDay ? dateData.singleDate : dateData.startDate),
                    entry_date_to: this.formatDateForAPI(dateData.isOneDay ? dateData.singleDate : dateData.endDate),
                    entry_time_from: dateData.startTime + ":00",
                    entry_time_to: dateData.endTime + ":00",
                    roof_access: dateData.roofAccess,
                    free_parking: dateData.freeParking,
                    // Место разгрузки на уровне вложения (#706): для items - единственный
                    // источник; для cars бэк берёт дедуп-union мест машин, поле дублирует.
                    unload_places: [...this.applicationUnloadPlaces],
                    data: {}
                };

                switch (attachment.attachment_type) {
                    case 'cars': {
                        const vehicles = this.vehiclesByAttachment[key] || [];
                        attachmentData.data.vehicles = vehicles.map(vehicle => ({
                            car_number: vehicle.plateNumber,
                            car_brand: vehicle.mark,
                            mark_id: vehicle.markId || null,
                            mark_name: vehicle.markName || vehicle.mark || null,
                            unload_place: vehicle.unloadingPlace,
                            unload_places: vehicle.unloadPlaces || [],
                            passage_tables: vehicle.passage_tables || []
                        }));
                        break;
                    }
                    case 'people': {
                        const employees = this.employeesByAttachment[key] || [];
                        attachmentData.data.employees = employees.map(employee => ({
                            last_name: employee.lastName,
                            first_name: employee.firstName,
                            middle_name: employee.middleName,
                            citizenship_id: employee.citizenshipId,
                            position: employee.position,
                            passport_series_number: employee.passportSeriesNumber,
                            patent_number: employee.patentNumber,
                            other_permission: employee.otherPermission,
                            target_tables: employee.targetTables || []
                        }));
                        break;
                    }
                    case 'items': {
                        const items = this.itemsByAttachment[key] || [];
                        attachmentData.data.items = items.map((item, index) => ({
                            name: item.itemName,
                            count: item.quantity,
                            order_index: index + 1
                        }));
                        break;
                    }
                }

                const customValues = this.customFieldsByAttachment[key] || {};
                if (Object.keys(customValues).length > 0) {
                    attachmentData.custom_values = Object.entries(customValues)
                        .filter(([, v]) => v)
                        .map(([fieldId, value]) => ({
                            custom_field_id: parseInt(fieldId),
                            value: value,
                        }));
                }

                applicationData.attachments.push(attachmentData);
            }

            try {
                const authStore = useAuthStore();
                if (!authStore.token) {
                    useDeletionsStore().notify({ prefix: 'Токен не найден', type: 'error' });
                    return;
                }

                const requiredUsers = [];
                
                if (this.organizationId) {
                    try {
                        const orgResponse = await apiRequest(`/organizations/${this.organizationId}/users`, {});
                        
                        if (orgResponse.ok) {
                            const orgUsers = await orgResponse.json();
                            orgUsers.forEach(user => {
                                if (user.required_approval) {
                                    requiredUsers.push({
                                        user_id: user.id,
                                        required_approval: true
                                    });
                                }
                            });
                        }
                    } catch (error) {
                        console.error("Ошибка при загрузке ответственных из организации:", error);
                    }
                }
                
                if (this.companyId) {
                    try {
                        const companyResponse = await apiRequest(`/companies/${this.companyId}/users`, {});
                        
                        if (companyResponse.ok) {
                            const companyUsers = await companyResponse.json();
                            companyUsers.forEach(user => {
                                const alreadyAdded = requiredUsers.some(u => u.user_id === user.id);
                                if (!alreadyAdded && user.required_approval) {
                                    requiredUsers.push({
                                        user_id: user.id,
                                        required_approval: true
                                    });
                                }
                            });
                        }
                    } catch (error) {
                        console.error("Ошибка при загрузке ответственных из компании:", error);
                    }
                }

                const finalRequestData = {
                    ...applicationData,
                    required_users: requiredUsers
                };

                const response = await apiRequest("/applications/submit-complete-application", {
                    method: "POST",
                    body: JSON.stringify(finalRequestData)
                });

                if (response.ok) {
                    const result = await response.json();
                    this.createdApplicationNumber = result.application_number;
                    this.createdAttachmentsData = this.attachments.map(att => {
                        const dateData = this.attachmentDatesByAttachment[this.attachmentKey(att)] || this.getDefaultDateData();
                        let period = '';
                        let time = '';

                        if (dateData.isOneDay) {
                            period = dateData.singleDate || '';
                        } else if (dateData.startDate || dateData.endDate) {
                            period = `${dateData.startDate || ''} - ${dateData.endDate || ''}`.trim();
                        }

                        if (dateData.startTime && dateData.endTime) {
                            time = `${dateData.startTime} - ${dateData.endTime}`;
                        }

                        return {
                            display_name: att.display_name || att.name,
                            period,
                            time
                        };
                    });
                    this.showSuccessModal = true;
                } else {
                    const errorText = await response.text();
                    console.error('Ошибка отправки заявки:', errorText);
                    useDeletionsStore().notify({ prefix: 'Ошибка отправки заявки: ', bold: errorText, type: 'error' });
                }
            } catch (error) {
                console.error('Ошибка отправки заявки:', error);
                useDeletionsStore().notify({ prefix: 'Произошла ошибка при отправке заявки: ', bold: error.message, type: 'error' });
            }
        },
        
        async loadRequiredResponsibles() {
            const requiredUsers = [];

            if (this.organizationId) {
                try {
                    const orgResponse = await apiRequest(`/organizations/${this.organizationId}/users`, {});
                    
                    if (orgResponse.ok) {
                        const orgUsers = await orgResponse.json();
                        orgUsers.forEach(user => {
                            if (user.required_approval) {
                                requiredUsers.push({
                                    user_id: user.id,
                                    required_approval: true
                                });
                            }
                        });
                    }
                } catch (error) {
                    console.error("Ошибка при загрузке ответственных из организации:", error);
                }
            }
            
            if (this.companyId) {
                try {
                    const companyResponse = await apiRequest(`/companies/${this.companyId}/users`, {});
                    
                    if (companyResponse.ok) {
                        const companyUsers = await companyResponse.json();
                        companyUsers.forEach(user => {
                            const alreadyAdded = requiredUsers.some(u => u.user_id === user.id);
                            if (!alreadyAdded && user.required_approval) {
                                requiredUsers.push({
                                    user_id: user.id,
                                    required_approval: true
                                });
                            }
                        });
                    }
                } catch (error) {
                    console.error("Ошибка при загрузке ответственных из компании:", error);
                }
            }
            
            return requiredUsers;
        },

        clearLocalStorageAfterSubmit() {
            localStorage.removeItem('draftApplicationState');
            localStorage.removeItem('draftApplication');
            
            const keysToRemove = [];
            for (let i = 0; i < localStorage.length; i++) {
                const key = localStorage.key(i);
                if (key.includes('attachment') || key.includes('draft') || key.includes('application')) {
                    keysToRemove.push(key);
                }
            }
            
            keysToRemove.forEach(key => {
                localStorage.removeItem(key);
            });
        },
        
        formatDateForAPI(dateStr) {
            if (!dateStr) return null;
            const [day, month, year] = dateStr.split('.');
            return `${year}-${month}-${day}`;
        },

        resetForm() {
            this.applicationNumber++;

            this.vehiclesByAttachment = {};
            this.employeesByAttachment = {};
            this.itemsByAttachment = {};
            this.attachmentDatesByAttachment = {};
            this.customFieldsByAttachment = {};
            this.customFieldDefinitions = {};
            this.fieldConfigByAttachment = {};

            this.vehicleIdCounter = 1;
            this.employeeIdCounter = 1;
            this.itemIdCounter = 1;

            this.vehicleFormKey += 1;
            this.employeeFormKey += 1;
            this.itemsFormKey += 1;

            this.organizationId = null;
            this.companyId = null;

            this.message = '';
            this.consentGiven = false;
            this.applicationUnloadPlaces = [];

            this.errors = {
                organization: '',
                company: '',
                responsiblePerson: '',
                phone: ''
            };

            this.loadUserData();

            if (this.$refs.blankSelector) {
                this.$refs.blankSelector.clearSelection();
            }
            this.selectedAttachment = null;
            this.attachments = [];
            this.readers = [];

            this.clearLocalStorageAfterSubmit();
        },

        saveToLocalStorage() {
            try {
                const hasAttachments = this.attachments.length > 0;

                const savedData = {
                    message: this.message,
                    organization: this.organization,
                    company: this.company,
                    responsiblePerson: this.responsiblePerson,
                    phoneNumber: this.phoneNumber,
                    rawPhoneNumber: this.rawPhoneNumber,
                    consentGiven: this.consentGiven,
                    readers: this.readers,

                    // attachments - источник истины для UI. Без них vehiclesByAttachment
                    // остаётся сиротским (ключи без самих attachment-записей) - BlankSelector
                    // не может отрендерить вложения. Демо-вложение онбординга НЕ
                    // персистим (иначе "прилипнет" при закрытии браузера посреди тура).
                    attachments: this.attachments.filter(a => !a.__onboardingDemo),

                    vehiclesByAttachment: hasAttachments ? this.vehiclesByAttachment : {},
                    employeesByAttachment: hasAttachments ? this.employeesByAttachment : {},
                    itemsByAttachment: hasAttachments ? this.itemsByAttachment : {},

                    applicationUnloadPlaces: hasAttachments ? this.applicationUnloadPlaces : [],

                    attachmentDatesByAttachment: hasAttachments ? this.attachmentDatesByAttachment : {},
                    customFieldsByAttachment: hasAttachments ? this.customFieldsByAttachment : {},

                    vehicleIdCounter: this.vehicleIdCounter,
                    employeeIdCounter: this.employeeIdCounter,
                    itemIdCounter: this.itemIdCounter,

                    savedAt: new Date().toISOString()
                };

                localStorage.setItem('draftApplicationState', JSON.stringify(savedData));
            } catch (error) {
                console.error('Ошибка сохранения состояния в localStorage:', error);
            }
        },

        restoreFromLocalStorage() {
            try {
                const savedData = localStorage.getItem('draftApplicationState');
                if (savedData) {
                    const parsedData = JSON.parse(savedData);
                    
                    this.message = parsedData.message || '';
                    this.organization = parsedData.organization || '';
                    this.company = parsedData.company || '';
                    this.responsiblePerson = parsedData.responsiblePerson || '';
                    this.phoneNumber = parsedData.phoneNumber || '';
                    this.rawPhoneNumber = parsedData.rawPhoneNumber || '';
                    this.consentGiven = parsedData.consentGiven || false;
                    this.readers = parsedData.readers || [];

                    this.attachments = parsedData.attachments || [];

                    this.vehiclesByAttachment = parsedData.vehiclesByAttachment || {};
                    this.employeesByAttachment = parsedData.employeesByAttachment || {};
                    this.itemsByAttachment = parsedData.itemsByAttachment || {};

                    this.applicationUnloadPlaces = parsedData.applicationUnloadPlaces || [];

                    this.attachmentDatesByAttachment = parsedData.attachmentDatesByAttachment || {};
                    this.customFieldsByAttachment = parsedData.customFieldsByAttachment || {};

                    this.vehicleIdCounter = parsedData.vehicleIdCounter || 1;
                    this.employeeIdCounter = parsedData.employeeIdCounter || 1;
                    this.itemIdCounter = parsedData.itemIdCounter || 1;

                    const hasAttachments = this.attachments.length > 0;

                    if (!hasAttachments) {
                        this.message = '';
                        this.consentGiven = false;
                    } else {
                        // Открываем первое вложение сверху, чтобы форма дубля не осталась пустой (#952).
                        this.selectedAttachment = this.attachments[0];
                    }
                }
            } catch (error) {
                console.error('Ошибка восстановления состояния из localStorage:', error);
            }
        },

        // #952: пришёл дубль из ЛК (pendingDuplicateState). Если на странице уже начат
        // черновик - спрашиваем "Заменить / Объединить / Отмена"; если пусто - берём сразу.
        checkPendingDuplicate() {
            const pendingRaw = localStorage.getItem('pendingDuplicateState');
            if (!pendingRaw) return;

            let pending;
            try {
                pending = JSON.parse(pendingRaw);
            } catch (e) {
                console.error('Битый pendingDuplicateState, пропускаю:', e);
                localStorage.removeItem('pendingDuplicateState');
                return;
            }

            if (this.hasExistingDraftData()) {
                this.pendingDuplicate = pending;
                this.showDuplicateConflict = true;
            } else {
                this.applyPendingDuplicate(pending, 'replace');
                localStorage.removeItem('pendingDuplicateState');
            }
        },

        // Есть ли на странице осмысленный черновик (вложения или текст сообщения).
        hasExistingDraftData() {
            const raw = localStorage.getItem('draftApplicationState');
            if (!raw) return false;
            try {
                const d = JSON.parse(raw);
                return (Array.isArray(d.attachments) && d.attachments.length > 0)
                    || (typeof d.message === 'string' && d.message.trim().length > 0);
            } catch (e) {
                console.error('Битый draftApplicationState:', e);
                return false;
            }
        },

        // Применяет дубль: replace - целиком заменяет черновик; merge - дописывает вложения
        // дубля к существующим (шапка/сообщение не трогаются). Затем перечитывает состояние.
        applyPendingDuplicate(pending, mode) {
            let finalDraft = pending;
            if (mode === 'merge') {
                const raw = localStorage.getItem('draftApplicationState');
                const existing = raw ? JSON.parse(raw) : {};
                finalDraft = this.mergeDrafts(existing, pending);
            }
            localStorage.setItem('draftApplicationState', JSON.stringify(finalDraft));
            this.restoreFromLocalStorage();
        },

        // Объединение: существующие данные и вложения остаются, вложения дубля добавляются.
        // local_id вложений уникальны (Date.now()+random), коллизий ключей нет.
        mergeDrafts(existing, dup) {
            return {
                ...existing,
                attachments: [...(existing.attachments || []), ...(dup.attachments || [])],
                vehiclesByAttachment: { ...(existing.vehiclesByAttachment || {}), ...(dup.vehiclesByAttachment || {}) },
                employeesByAttachment: { ...(existing.employeesByAttachment || {}), ...(dup.employeesByAttachment || {}) },
                itemsByAttachment: { ...(existing.itemsByAttachment || {}), ...(dup.itemsByAttachment || {}) },
                attachmentDatesByAttachment: { ...(existing.attachmentDatesByAttachment || {}), ...(dup.attachmentDatesByAttachment || {}) },
                customFieldsByAttachment: { ...(existing.customFieldsByAttachment || {}), ...(dup.customFieldsByAttachment || {}) },
                vehicleIdCounter: Math.max(existing.vehicleIdCounter || 1, dup.vehicleIdCounter || 1),
                employeeIdCounter: Math.max(existing.employeeIdCounter || 1, dup.employeeIdCounter || 1),
                itemIdCounter: Math.max(existing.itemIdCounter || 1, dup.itemIdCounter || 1),
            };
        },

        onDuplicateConflictReplace() {
            this.applyPendingDuplicate(this.pendingDuplicate, 'replace');
            this.finishDuplicateConflict();
        },

        onDuplicateConflictMerge() {
            this.applyPendingDuplicate(this.pendingDuplicate, 'merge');
            this.finishDuplicateConflict();
        },

        onDuplicateConflictCancel() {
            // Черновик уже восстановлен в mounted - просто закрываем и выкидываем дубль.
            this.finishDuplicateConflict();
        },

        finishDuplicateConflict() {
            localStorage.removeItem('pendingDuplicateState');
            this.pendingDuplicate = null;
            this.showDuplicateConflict = false;
        },

        async loadApplication(applicationId) {
            try {
                const response = await apiRequest(`/application/${applicationId}`, {});

                if (response.ok) {
                    const data = await response.json();
                    this.currentApplicationData = data;
                    
                    this.message = data.message || '';
                    this.organization = data.organization || '';
                    this.company = data.company || '';
                    this.responsiblePerson = data.responsible_person || '';
                    this.phoneNumber = data.contact_phone || '';
                    
                    if (data.attachments && data.attachments.length > 0) {
                        this.attachments = data.attachments;
                        data.attachments.forEach(attachment => {
                            this.attachmentDatesByAttachment[this.attachmentKey(attachment)] = {
                                isOneDay: data.entry_date_from === data.entry_date_to,
                                startDate: data.entry_date_from ? this.formatDateFromAPI(data.entry_date_from) : '',
                                endDate: data.entry_date_to ? this.formatDateFromAPI(data.entry_date_to) : '',
                                singleDate: data.entry_date_from === data.entry_date_to ? 
                                    (data.entry_date_from ? this.formatDateFromAPI(data.entry_date_from) : '') : '',
                                startTime: data.entry_time_from ? data.entry_time_from.substring(0, 5) : '',
                                endTime: data.entry_time_to ? data.entry_time_to.substring(0, 5) : '',
                                roofAccess: false,
                                freeParking: false,
                                errors: {}
                            };
                        });
                    }
                    
                    if (data.attachment_type === 'cars' && data.vehicles) {
                        this.vehicles = data.vehicles.map((vehicle, index) => ({
                            ...vehicle,
                            id: index + 1,
                            isExisting: true
                        }));
                    } else if (data.attachment_type === 'people' && data.employees) {
                        this.employees = data.employees.map((employee, index) => ({
                            ...employee,
                            id: index + 1,
                            isExisting: true
                        }));
                    } else if (data.attachment_type === 'items' && data.items) {
                        this.items = data.items.map((item, index) => ({
                            ...item,
                            id: index + 1
                        }));
                    }
                }
            } catch (error) {
                console.error('Ошибка загрузки заявки:', error);
            }
        },

        formatDateFromAPI(dateStr) {
            if (!dateStr) return '';
            const [year, month, day] = dateStr.split('-');
            return `${day}.${month}.${year}`;
        }
    }
}
</script>

<style scoped>
    .text-constructor-content :deep(*) {
        overflow-wrap: break-word;
    }

    .text-constructor-content :deep(img) {
        max-width: 100%;
        border-radius: 8px;
    }

    .text-constructor-content :deep(img:not([height])) {
        height: auto;
    }

    .text-constructor-content :deep(.text-align-left) { text-align: left; }
    .text-constructor-content :deep(.text-align-center) { text-align: center; }
    .text-constructor-content :deep(.text-align-right) { text-align: right; }

    .create {
        padding: var(--gutter, 20px);
    }

    .create__title {
        display: flex;
        display: flex;
        gap: 10px;
    }

    .create__header {
        display: flex;
        flex-direction: column;
        align-items: stretch;
        gap: 6px;
        padding-bottom: 15px;
    }

    .create__header-top {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 16px;
    }

    /* Заголовок + "Получатели:" в одной строке слева (#884). */
    .create__header-left {
        display: flex;
        align-items: center;
        gap: 20px;
        flex-wrap: wrap;
        min-width: 0;
    }

    .create__container {
        display: flex;
        gap: 15px;
    }

    .tables__instruction {
        width: fit-content;
        font-size: 14px;
        font-weight: 500;
        color: #4F5BDF;
        padding: 0 10px;
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 5px;
        border-radius: 50px;
        background: #FFF;
        border: 1px solid #e6e6e6;
        outline: none;
        cursor: pointer;
        height: 25px;
    }

    .tables__icon {
        width: 15px;
        height: 15px;
    }

    .tables__instruction:hover {
        background-color: #f2f2f2;
    }

    .create__form {
        width: 100%;
        height: fit-content;
        background-color: #FFF;
        border: 1px solid #e6e6e6;
        border-radius: 30px;
        box-shadow: 0 3px 10px rgba(0,0,0,0.05);
    }

    /* Шапка - две колонки: поле сообщения слева, согласие + отправка справа */
    .form__header {
        width: 100%;
        border-bottom: 1px solid #e6e6e6;
        padding: 15px;
        display: flex;
        align-items: flex-start;
        gap: 16px;
    }

    .create__form .form__message-tc {
        flex: 4 1 0;
        max-width: 1000px;
        min-width: 0;
        margin-bottom: 0;
        border-radius: 30px;
    }

    /* Инструменты редактора в форме заявки - компактнее */
    .create__form :deep(.toolbar-btn) {
        min-width: 28px;
        height: 28px;
        font-size: 13px;
        padding: 0 6px;
    }

    /* Согласие + кнопка отправки — выделены в блок справа от поля сообщения */
    .form__submit-bar {
        flex: 1 1 0;
        min-width: 190px;
        max-width: 270px;
        align-self: flex-start;
        display: flex;
        flex-direction: column;
        gap: 12px;
        padding: 14px;
        background: #fafafa;
        border: 1px solid #e6e6e6;
        border-radius: 15px;
    }

    /* согласие сверху, кнопка под ним; всё по левому краю */
    .consent-section {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        gap: 12px;
    }

    .consent-checkbox {
        display: flex;
        gap: 10px;
    }

    .consent-checkbox input[type="checkbox"] {
        width: 14px;
        height: 14px;
        cursor: pointer;
        flex-shrink: 0;
    }

    .consent-checkbox label {
        font-size: 12px;
        color: #333;
        cursor: pointer;
        line-height: 1.2;
    }

    .submit-button-container {
        position: relative;
        display: block;
        width: 100%;
        text-align: left;
    }

    .submit-tooltip {
        position: absolute;
        right: 100%;
        margin-right: 8px;
        top: 0;
        background: #333;
        color: white;
        padding: 12px;
        border-radius: 8px;
        font-size: 12px;
        width: 380px;
        max-height: 400px;
        overflow-y: auto;
        z-index: 1000;
        pointer-events: auto;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
        white-space: normal;
    }

    .submit-tooltip::before {
        content: '';
        position: absolute;
        top: 8px;
        left: 100%;
        border: 5px solid transparent;
        border-left-color: #333;
    }

    .tooltip-content {
        font-family: inherit;
    }

    .tooltip-section {
        margin-bottom: 12px;
    }

    .tooltip-section:last-child {
        margin-bottom: 0;
    }

    .tooltip-section-title {
        font-weight: 600;
        margin-bottom: 6px;
        color: #fff;
    }

    .tooltip-attachment-title {
        font-weight: 600;
        margin-bottom: 6px;
        color: #d4d4d4;
    }

    .attachment-clickable {
        cursor: pointer;
        text-decoration: underline;
        transition: opacity 0.2s;
    }

    .attachment-clickable:hover {
        opacity: 0.8;
    }

    .tooltip-section ul {
        margin: 0;
        padding-left: 20px;
    }

    .tooltip-section li {
        margin-bottom: 4px;
        line-height: 1.4;
    }

    .send-all-btn {
        background: #4F5BDF;
        color: white;
        border: none;
        border-radius: 15px;
        padding: 8px 15px;
        font-size: 12px;
        cursor: pointer;
        transition: background-color 0.2s;
        width: auto;
        display: inline-block;
        flex-shrink: 0;
        height: fit-content;
    }

    .send-all-btn:hover:not(:disabled) {
        background: #3a45c0;
    }

    .send-all-btn:disabled {
        background: #a2a2a2;
        cursor: not-allowed;
        opacity: 0.6;
    }

    .form__info-row {
        padding: 15px;
        display: flex;
        gap: 50px;
        border-bottom: 1px solid #e6e6e6;
        /* Поднимаем ряд дат над form__data: иначе выпадающее меню "Быстрый выбор"
           (открывается вниз) уходит под серый гейт-оверлей и кнопки формы ниже. */
        position: relative;
        z-index: 1;
    }

    h4 {
        font-size: 22px;
        font-weight: 800;
        color: #4F5BDF;
        border: 1px solid #e6e6e6;
        border-radius: 50px;
        padding: 5px 20px;
        width: fit-content;
    }

    .form__data {
        display: flex;
        position: relative;
    }

    /* Форма ввода (data__completion, 450px) + список (data__list, flex:1) стоят рядом
       и на планшете начинают давить друг друга - стекаем в колонку заранее (lg), не
       дожидаясь мобильного. */
    @media (max-width: 1024px) {
        .form__data {
            flex-direction: column;
        }
    }

    .blue {
        color: #4F5BDF;
    }

    .consent-link {
        text-decoration: underline;
        cursor: pointer;
    }

    .consent-link:hover {
        color: #3d49c7;
    }

    .form-placeholder {
        width: 100%;
        height: fit-content;
        min-height: 490px;
        background-color: #FFF;
        border: 1px solid #e6e6e6;
        border-radius: 30px;
        box-shadow: 0 3px 10px rgba(0,0,0,0.05);
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .placeholder-content {
        text-align: center;
        color: #a2a2a2;
    }

    .placeholder-content p {
        font-size: 16px;
        margin: 0;
        padding: 20px;
    }

    /* Mobile: stacked layout, forms в 1 колонку */
    @media (max-width: 768px) {
        .create__header {
            flex-direction: column;
            align-items: flex-start;
            gap: 10px;
            padding-bottom: 12px;
        }

        .create__title {
            font-size: 18px;
        }

        /* Заголовок страницы и пилюля с типом бланка рядом не помещаются: на 320
           ряд распирало до 331-352px и браузер ужимал всю страницу. */
        .create__header-top {
            flex-direction: column;
            align-items: flex-start;
            gap: 8px;
            /* min-width:auto у flex-элемента = ширина по содержимому: строка
               получателей распирала шапку шире страницы. */
            width: 100%;
            min-width: 0;
        }

        /* В колонке align-items:flex-start даёт детям ширину по содержимому -
           строка получателей тогда распирает страницу вместо того, чтобы ужаться. */
        .create__header-left {
            gap: 10px;
            width: 100%;
        }

        h4 {
            font-size: 15px;
            padding: 4px 14px;
            max-width: 100%;
        }

        .create__container {
            flex-direction: column;
            gap: 12px;
        }

        .form__header {
            height: auto;
            padding: 12px;
            flex-direction: column;
        }

        .create__form .form__message-tc {
            flex-basis: auto;
            max-width: none;
            width: 100%;
        }

        .form__submit-bar {
            min-width: 0;
            max-width: none;
            width: 100%;
            align-self: stretch;
        }

        .consent-section {
            width: 100%;
        }

        .consent-checkbox input[type="checkbox"] {
            width: 20px;
            height: 20px;
        }

        .consent-checkbox label {
            font-size: 13px;
            line-height: 1.35;
        }

        .send-all-btn {
            width: 100%;
            min-height: 44px;
            font-size: 14px;
        }

        /* Причины блокировки: сбоку окно в 380px не помещается - ставим его в поток
           под кнопкой, стрелка смотрит вверх. */
        .submit-tooltip {
            position: static;
            width: 100%;
            max-width: 100%;
            margin: 8px 0 0;
            max-height: min(50dvh, 320px);
        }

        .submit-tooltip::before {
            top: -10px;
            left: 16px;
            border-color: transparent;
            border-bottom-color: #333;
        }

        .form__textarea {
            width: 100%;
        }

        .form__info-row {
            padding: 12px;
            gap: 16px;
        }

        /* До выбора вложения заглушка занимала пол-экрана - на телефоне это
           пустая карточка между селектором и низом страницы. */
        .form-placeholder {
            min-height: 0;
            border-radius: var(--radius-lg);
        }

        .placeholder-content p {
            font-size: 14px;
            padding: 14px;
        }
    }

    @media (max-width: 480px) {
        .create__title {
            font-size: 16px;
        }

        .tables__instruction {
            font-size: 12px;
            padding: 0 8px;
        }
    }
</style>