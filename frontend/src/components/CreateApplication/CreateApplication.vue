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
        <h4 class="create__blank-title create__blank-title--header">
          {{ currentFormTitle }}
        </h4>
      </div>
    </div>

    <div class="create__container">
      <BlankSelector
        ref="blankSelector"
        data-testid="ob-app-selector"
        :attachments="attachments"
        :current-application-data="currentApplicationData"
        :active-attachment="selectedAttachment"
        @attachment-selected="handleAttachmentSelected"
        @attachment-added="handleAttachmentAdded"
        @attachment-removed="handleAttachmentRemoved"
        @attachment-renamed="handleAttachmentRenamed"
      />

      <!-- На телефоне заголовок вложения стоит между селектором и формой: в шапке
           он висел рядом с названием страницы и терялся. При скролле формы
           заголовок закрепляется под шапкой приложения (sticky), чтобы было видно,
           какое вложение заполняешь. -->
      <div
        ref="blankSentinel"
        class="create__blank-sentinel"
        aria-hidden="true"
      />
      <div
        class="create__blank-sticky"
        :class="{ 'create__blank-sticky--stuck': blankStuck }"
      >
        <h4 class="create__blank-title create__blank-title--inline">
          {{ currentFormTitle }}
        </h4>
      </div>

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
          >
            <template #attachments>
              <ApplicationFilesUpload
                ref="filesUpload"
                v-model="applicationFileIds"
              />
            </template>
          </TextConstructor>
          <!-- Согласие и отправка — правая колонка шапки, рядом с полем сообщения -->
          <div class="form__submit-bar">
            <div
              class="consent-section"
              data-testid="ob-app-consent"
            >
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
                    data-testid="create-app-consent-link"
                    @click.stop="onConsentClick"
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
                  :disabled="!canSubmit || isSubmitting"
                  @click="submitApplication"
                >
                  <span
                    v-if="isSubmitting"
                    class="send-all-btn__spinner"
                    aria-hidden="true"
                  />
                  {{ isSubmitting ? 'Отправляем...' : 'Отправить заявку' }}
                </button>
                <!-- Честный статус долгой подачи (эпик blank-import, срез E2E3): большой
                     импортированный список отправляется дольше обычного, кнопка одна
                     "погасла" читалась бы как зависшая форма. -->
                <div
                  v-if="isSubmitting"
                  class="submit-progress-hint"
                  data-testid="create-app-submit-progress"
                  role="status"
                  aria-live="polite"
                >
                  Заявка отправляется - при большом числе строк это может занять до пары
                  минут. Не закрывайте страницу.
                </div>
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
          :can-override-directory="canOverrideDirectory"
          @update:organization="organization = $event"
          @update:company="company = $event"
          @update:responsible-person="responsiblePerson = $event"
          @update:phone-number="phoneNumber = $event"
          @select-organization="applyOrganizationChoice($event)"
          @select-company="applyCompanyChoice($event)"
          @validate-field="validateField"
          @format-phone="handleFormatPhoneNumber"
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
        <!-- data-attachment-type читает онбординг: форма одна на все бланки, и по
             одному лишь её появлению не понять, успела ли она перерисоваться под
             выбранный бланк. -->
        <div
          class="form__data"
          data-testid="ob-app-formdata"
          :data-attachment-type="selectedAttachment && selectedAttachment.attachment_type"
        >
          <!-- Режим импорта (blank-import-ux, U4): панель встаёт НА МЕСТО формы ручного
               ввода, список рядом остаётся - вход и выход из режима живут в его шапке.
               На время правки строки панель прячется, а НЕ размонтируется: разбор файла,
               выбор мест и правки проблемных строк живут в ней, а повторное монтирование
               заново отправило бы принятые строки в список. -->
          <BlankImportPanel
            v-if="importMode"
            v-show="!importPausedForEdit"
            :attachment-type="selectedAttachment ? selectedAttachment.attachment_type : 'people'"
            :result="importResult"
            :uploading="importUploading"
            :downloading="downloadingBlankTemplate"
            :all-passage-tables="allPassageTables"
            :all-unloading-places="allUnloadingPlaces"
            :field-config="currentFieldConfig"
            :pending-count="pendingImportCount"
            @file="uploadImportFile"
            @download-blank="downloadBlankTemplate"
            @stage="stageImportRows"
            @import="handleImportRows"
            @reset="resetImportResult"
            @close="closeImportMode"
          />

          <!-- Для автомобилей -->
          <template v-if="selectedAttachment && selectedAttachment.attachment_type === 'cars'">
            <VehicleForm
              v-if="!importMode || importPausedForEdit"
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
              :can-import="canImportList"
              :import-active="importMode"
              @sort="sortBy"
              @edit-vehicle="editVehicle"
              @delete-vehicle="deleteVehicle"
              @toggle-import="toggleImportMode"
              @clear-list="clearList('cars')"
            />
          </template>

          <!-- Для людей/сотрудников -->
          <template v-else-if="selectedAttachment && selectedAttachment.attachment_type === 'people'">
            <EmployeeForm
              v-if="!importMode || importPausedForEdit"
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
              :can-import="canImportList"
              :import-active="importMode"
              @sort="sortBy"
              @edit-employee="editEmployee"
              @delete-employee="deleteEmployee"
              @toggle-import="toggleImportMode"
              @clear-list="clearList('people')"
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
      :processing="isBindingActionInProgress"
      @confirm-binding="confirmBinding"
      @skip-binding="skipBinding"
      @close="cancelBindingModal"
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

    <!-- #1380: на телефоне согласие открывается модалкой с содержимым, не новой вкладкой. -->
    <DataProcessingModal
      :show="showConsentModal"
      @close="showConsentModal = false"
    />
  </div>
</template>

<script>
import { apiRequest, createExtendedTimeoutSignal } from '@/api/client'
import { mapWithConcurrency } from '@/utils/mapWithConcurrency'
import { useAuthStore } from '@/stores/auth'
import { usePermissionsStore } from '@/stores/permissions'
import { downloadBlankTemplate as fetchBlankTemplate, uploadImportList } from '@/api/blankImport'
import { saveBlobAs } from '@/api/attachment-templates'
import { toAttachmentContent } from '@/utils/applicationEntityPayload';
import { useDeletionsStore } from '@/stores/deletions'
import { formatRussianPhone, isValidRussianPhone } from '@/composables/useRussianPhoneMask'
import BlankSelector from '../BlankSelector.vue';
import ApplicationFilesUpload from './ApplicationFilesUpload.vue';
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
import DataProcessingModal from '@/components/DataProcessingModal.vue';
import BlankImportPanel from './BlankImportPanel.vue';
import {
    findFirstDuplicate,
    findDuplicateEmployee,
    findDuplicateVehicle,
    isSameEmployee,
    isSameVehicle,
    employeeLabel,
    vehicleLabel,
} from '@/utils/applicationDuplicates';
import { notifyApiError } from '@/utils/apiError';
import { hasByFactVehicle, BY_FACT_ONE_DAY_HINT } from '@/utils/byFactVehicle';

// Параллелизм привязки новых ТС/сотрудников при подаче: держим веер узким, чтобы
// крупная заявка не выстрелила сотнями одновременных POST и не упёрлась в лимит.
const BIND_CONCURRENCY = 6;

// Право на импорт списка из заполненного бланка (эпик blank-import, срез C1C2).
// Кнопка скачивания ПУСТОГО бланка (B2) гейтится тем же правом: скачал -
// значит сможет и загрузить обратно, иначе получится класс "видно, но 403".
const ACTION_IMPORT_LIST_PERMISSION = 'action.import.list';

// Подача заявки с массовым импортом (до 2000 строк) не укладывается в дефолтный
// таймаут apiRequest (10с) - даём этому единственному запросу отдельный, куда
// более щедрый бюджет (эпик blank-import, срез E2E3).
const SUBMIT_TIMEOUT_MS = 120000;

// Предварительная строка (эпик blank-import-ux, срез U5): разобрана из бланка и уже
// видна в списке, но в подачу не уходит, пока в сводке импорта не нажата «Добавить».
// Флаг живёт на самой строке, поэтому в черновик localStorage он попадает вместе с ней
// и переживает перезагрузку без отдельного хранилища.
const isPendingRow = (row) => !!row && row.isPending === true;

/** Строки, которые реально уходят в заявку: без предварительных. */
const submittedRows = (rows) => (rows || []).filter((row) => !isPendingRow(row));

// Кросс-браузерное распознавание переполнения квоты localStorage: разные
// движки шлют разный name/code (см. MDN QuotaExceededError).
function isQuotaExceededError(error) {
    return error instanceof DOMException && (
        error.name === 'QuotaExceededError'
        || error.name === 'NS_ERROR_DOM_QUOTA_REACHED'
        || error.code === 22
        || error.code === 1014
    );
}

export default {
    name: 'CreateApplication',
    components: {
        ApplicationFilesUpload,
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
        DuplicateConflictModal,
        DataProcessingModal,
        BlankImportPanel
    },
    data() {
        return {
            message: '',
            // Файлы, приложенные к заявке (#1721). Наверх приходят id уже
            // загруженных черновиков: подача привязывает их к заявке.
            applicationFileIds: [],
            // Прилип ли заголовок вложения к верху (для подложки и границы при скролле).
            blankStuck: false,
            // Конфликт дублирования (#952): на странице уже есть черновик, а из ЛК пришёл
            // дубль - показываем модалку "Заменить / Объединить / Отмена".
            showDuplicateConflict: false,
            pendingDuplicate: null,
            organization: '',
            company: '',
            responsiblePerson: '',
            phoneNumber: '',
            consentGiven: false,
            // На телефоне ссылка "согласие" открывает эту модалку вместо новой вкладки
            // с /data-processing (там <embed> PDF на мобилке не рендерится).
            showConsentModal: false,
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

            // Скачивание пустого бланка для массового ввода (эпик blank-import, B2):
            // блокирует повторный клик, пока летит запрос.
            downloadingBlankTemplate: false,

            // Режим импорта из бланка (blank-import-ux, U4): панель вместо формы
            // ручного ввода. Живёт на вложение - переключение вложения его гасит.
            importMode: false,
            importUploading: false,
            // {rows, summary} последнего разбора - показывается панелью вместо дропзона.
            importResult: null,
            // Правка строки списка при открытом импорте: панель уступает место форме и
            // возвращается, когда правка завершена. Режим при этом не выключается -
            // разбор файла и выбор мест переживают правку.
            importPausedForEdit: false,

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
            
            currentApplicationData: {},

            allPassageTables: [],

            showSuccessModal: false,
            createdApplicationNumber: '',
            createdAttachmentsData: [],

            // Блокирует повторное нажатие "Отправить заявку", пока летит подача
            // (эпик blank-import, срез E2E3): без гарда двойной клик на большом
            // списке успевает уйти вторым POST раньше, чем отработает первый.
            isSubmitting: false,
            // Уточнение isSubmitting: true строго ПОДМНОЖЕСТВО isSubmitting=true, с момента
            // клика "Привязать и отправить"/"Отправить без привязки" внутри модалки привязки
            // и до завершения confirmBinding/skipBinding (mapWithConcurrency-привязка + сам
            // submit). Пока модалка только ОТКРЫТА и юзер решает - false, крестик/оверлей/
            // Escape/свайп закрывают её штатно. Как только процесс стартовал - true, и
            // UniversalBindingModal (проп processing) сама блокирует и свои кнопки, и закрытие -
            // иначе повторный клик внутри модалки или "отмена" посреди работающего confirmBinding
            // оставляют его фоновый прогон невидимым и ведут ко второму параллельному submit.
            isBindingActionInProgress: false,

            showSubmitTooltip: false,
            tooltipTimer: null,

            customFieldsByAttachment: {},
            customFieldDefinitions: {},
            // Настройка полей по UniqueAttachment (#529): { [uaId]: { [fieldKey]: { visible, required, locked, requirable } } }.
            // Источник - GET /attachments/{id}/field-config (base). Раздаётся секциям формы пропсом field-config.
            fieldConfigByAttachment: {},
            // Номер последнего запуска restoreFromLocalStorage (защита от позднего ответа).
            restoreSeq: 0,
            // Номер последнего выбора вложения: пока грузится конфиг, пользователь
            // мог кликнуть другое - поздний ответ не должен открыть чужую форму.
            attachmentSelectSeq: 0,
        }
    },
    computed: {
        // Право application.organization.override (#1437): открывает ручной ввод
        // организации и компании и подсказки по справочнику. Без него поля показывают
        // запись из профиля, а сервер всё равно привяжет заявку к ней.
        canOverrideDirectory() {
            return usePermissionsStore().hasPermission('application.organization.override');
        },

        // Кнопка входа в импорт в шапке списка (U4): видна только когда у пользователя
        // есть право на загрузку списка (C1C2) и выбрано вложение со списком
        // участников - у ТМЦ (items) списочной части в бланке нет.
        canImportList() {
            return usePermissionsStore().hasPermission(ACTION_IMPORT_LIST_PERMISSION)
                && !!this.selectedAttachment
                && ['cars', 'people'].includes(this.selectedAttachment.attachment_type);
        },

        // Привязка машин и сотрудников к организации возможна, только когда организация
        // заявки есть в справочнике: у введённой руками новой записи id появится лишь
        // после подачи, привязывать не к чему.
        hasOrganization() {
            return !!this.organizationId;
        },

        hasCompany() {
            return !!this.companyId;
        },

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

        // Предварительные строки текущего вложения (U5). Сводка импорта считает свои
        // счётчики по ЭТОМУ числу, а не по summary разбора: удалил строку из списка -
        // сводка обязана пересчитаться, иначе кнопка обещает добавить то, чего нет.
        pendingImportCount() {
            if (!this.selectedAttachment) return 0;
            const key = this.attachmentKey(this.selectedAttachment);
            return this.rowsForAttachment(this.selectedAttachment.attachment_type, key)
                .filter(isPendingRow).length;
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
            if (!data?.isOneDay && hasByFactVehicle(this.vehicles)) return { ...(data?.errors || {}), endDate: BY_FACT_ONE_DAY_HINT };
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
                // Предварительные строки в подачу не идут, поэтому и в счёт «есть данные»
                // не идут тоже: вложение из одних серых строк для сервера пустое.
                const rows = this.rowsForAttachment(attachment.attachment_type, key);
                const ready = submittedRows(rows);
                const pending = rows.length - ready.length;
                const isEmpty = ready.length === 0 && pending === 0;

                switch (attachment.attachment_type) {
                    case 'cars':
                        if (isEmpty) reasons.push(`"${label}": добавьте хотя бы одно авто`);
                        break;
                    case 'people':
                        if (isEmpty) reasons.push(`"${label}": добавьте хотя бы одного сотрудника`);
                        break;
                    case 'items':
                        if (isEmpty) reasons.push(`"${label}": добавьте хотя бы одну позицию`);
                        if (this.itemsUnloadRequired && this.applicationUnloadPlaces.length === 0) {
                            reasons.push(`"${label}": выберите место разгрузки`);
                        }
                        break;
                }

                // Не «предупредим и отправим»: незамеченные серые строки ушли бы в никуда
                // вместе с черновиком, который подача чистит. Поэтому подача ждёт решения -
                // добавить их или удалить.
                if (pending > 0) {
                    reasons.push(`"${label}": строки из бланка ещё предварительные (${pending}) - нажмите «Добавить» в сводке импорта или удалите их`);
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

                const allRows = this.rowsForAttachment(type, key);
                const items = submittedRows(allRows);
                const pending = allRows.length - items.length;

                if (type === 'cars' && items.length === 0 && pending === 0) errors.push('Не добавлено ни одного автомобиля');
                else if (type === 'people' && items.length === 0 && pending === 0) errors.push('Не добавлено ни одного сотрудника');
                else if (type === 'items' && items.length === 0) errors.push('Не добавлено ни одной позиции');

                if (pending > 0) {
                    errors.push(`Строки из бланка ещё предварительные (${pending})`);
                }

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
    async mounted() {
        // Профиль подтягиваем ДО восстановления черновика (#1457). loadUserData
        // безусловно перезаписывает организацию, компанию, ФИО и телефон, а раньше он
        // стоял после восстановления и приходил вторым: заявитель, набравший чужую
        // организацию по праву application.organization.override (#1437), после
        // перезагрузки страницы молча получал обратно свою. Порядок «профиль, затем
        // черновик» оставляет за черновиком последнее слово.
        //
        // Сбой профиля не должен стоить введённого: ошибку глотаем и всё равно
        // восстанавливаем черновик, иначе форма откроется пустой.
        try {
            await this.loadUserData();
        } catch (error) {
            console.error('Не удалось загрузить данные профиля:', error);
        }
        this.restoreFromLocalStorage();
        this.checkPendingDuplicate();
        this.loadAllUnloadingPlaces();
        this.loadLicensePlateFormats();
        this.loadPassageTables();

        window.addEventListener('beforeunload', () => {
            this.saveCurrentAttachmentData();
            this.saveToLocalStorage();
        });

        this.initBlankStickyObserver();
    },
    beforeUnmount() {
        window.removeEventListener('beforeunload', this.saveToLocalStorage);
        if (this._blankObserver) this._blankObserver.disconnect();
    },
    methods: {
        /**
         * Заголовок вложения «прилипает» под шапкой при скролле формы. Sentinel
         * перед ним уходит за верхнюю линию (шапка 55px) - значит заголовок
         * закреплён, включаем подложку и границу (blankStuck).
         */
        initBlankStickyObserver() {
            if (typeof IntersectionObserver === 'undefined') return;
            const sentinel = this.$refs.blankSentinel;
            if (!sentinel) return;
            this._blankObserver = new IntersectionObserver(
                ([entry]) => { this.blankStuck = !entry.isIntersecting; },
                { rootMargin: '-56px 0px 0px 0px', threshold: 0 },
            );
            this._blankObserver.observe(sentinel);
        },

        // На телефоне открываем согласие модалкой (в ней PDF рендерит pdf.js), на десктопе -
        // отдаём нативной ссылке href="/data-processing" (там <embed> работает). Порог 768px
        // совпадает с sheet-брейкпоинтом BaseModal, чтобы модалка выезжала снизу листом.
        onConsentClick(e) {
            if (typeof window !== 'undefined' && typeof window.matchMedia === 'function'
                && window.matchMedia('(max-width: 768px)').matches) {
                e.preventDefault();
                this.showConsentModal = true;
            }
        },

        async loadPassageTables() {
            try {
                const response = await apiRequest("/system-tables", {
                    method: "GET"});

                if (response.ok) {
                    const tables = await response.json();
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

    // Проверяем то, что уйдёт в заявку: предварительная строка активной машиной
    // никого не блокирует, а запрос на неё - лишний круг по сети на каждую строку.
    const vehicles = submittedRows(this.vehiclesByAttachment[this.attachmentKey(attachment)]);

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

        /**
         * Формирует строку конфликта для одной машины: номер, марка и заявка,
         * которая уже держит её активной (номер заявки + срок действия). Данные
         * приходят из GET /cars/check-active (activeInfo). Без номера заявки
         * (старый ответ бэка) откатываемся на «номер марка».
         * @param {{plateNumber: string, mark: string, activeInfo?: {application_number?: string, entry_date_to?: string, entry_time_to?: string}}} vehicle
         * @returns {string}
         */
        formatActiveVehicleConflict(vehicle) {
            const base = `${vehicle.plateNumber} ${vehicle.mark}`;
            const info = vehicle.activeInfo || {};
            if (!info.application_number) return base;
            let until = '';
            if (info.entry_date_to) {
                const time = info.entry_time_to ? ` ${info.entry_time_to.slice(0, 5)}` : '';
                until = `, до ${this.formatDateFromAPI(info.entry_date_to)}${time}`;
            }
            return `${base} (в заявке ${info.application_number}${until})`;
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
                    
                    this.loadDefaultApprovers();

                    const lastName = userData.last_name || '';
                    const firstName = userData.first_name || '';
                    const middleName = userData.middle_name || '';
                    this.responsiblePerson = `${lastName} ${firstName} ${middleName}`.trim();
                    
                    this.phoneNumber = formatRussianPhone(userData.phone || '');

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

            // Принимающие: заявка уходит им, и в получателях они должны быть видны
            // всегда - даже когда согласующих у организации нет. Имя приходит готовым:
            // маска, если администратор её задал, иначе ФИО.
            try {
                const r = await apiRequest('/application-approvers/recipients', {});
                if (r.ok) {
                    const recipients = await r.json();
                    (recipients || []).forEach(u => {
                        if (u.user_id && !byId.has(u.user_id)) {
                            byId.set(u.user_id, { user_id: u.user_id, name: u.name });
                        }
                    });
                }
            } catch (error) {
                console.error('Ошибка загрузки принимающих:', error);
            }

            this.defaultApprovers = Array.from(byId.values());
        },

        userDisplayName(u) {
            const names = [u.last_name, u.first_name, u.middle_name].filter(Boolean);
            return names.length ? names.join(' ') : (u.username || '');
        },

        handleFormatPhoneNumber() {
            this.phoneNumber = formatRussianPhone(this.phoneNumber);
            this.validateField('phone');
        },

        updateCustomFieldValues(values) {
            const key = this.attachmentKey(this.selectedAttachment);
            this.customFieldsByAttachment[key] = values;
        },

        async loadFieldConfig(uniqueAttachmentId) {
            // Черновики без id вложения (старый формат localStorage) - без запроса
            // по адресу /attachments/undefined/field-config.
            if (!uniqueAttachmentId) return;
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

        // Скачивание пустого бланка для массового заполнения списка (эпик blank-import,
        // B1 отдаёт файл, B2 - эта кнопка). Наличие list-маппингов не запрашивается
        // отдельным эндпоинтом заранее - эндпоинт сам возвращает 404 с готовым русским
        // текстом ("Шаблон бланка не настроен" / "В бланке не размечен список
        // участников"), его и показываем как есть.
        async downloadBlankTemplate() {
            if (!this.selectedAttachment || this.downloadingBlankTemplate) return;
            const uaId = this.selectedAttachment.template_id || this.selectedAttachment.id;
            this.downloadingBlankTemplate = true;
            try {
                const { blob, filename } = await fetchBlankTemplate(uaId);
                saveBlobAs(blob, filename);
            } catch (error) {
                useDeletionsStore().notify({ prefix: error.message || 'Не удалось скачать бланк для заполнения', type: 'error' });
            } finally {
                this.downloadingBlankTemplate = false;
            }
        },

        // Вход и выход из режима импорта (U4). Разобранный файл режим не переживает:
        // вернуться к ручному вводу и снова открыть импорт - это новая загрузка, а не
        // продолжение старой сводки.
        toggleImportMode() {
            if (this.importMode) {
                this.closeImportMode();
                return;
            }
            this.importMode = true;
        },

        /**
         * Выход из режима импорта. Предварительные строки живут ровно столько, сколько
         * открыт разбор: закрыли панель, не нажав «Добавить», - их уносит вместе с
         * разбором (решение владельца). Иначе серые строки остались бы в списке без
         * сводки, то есть без способа ни принять их, ни убрать штатно.
         *
         * Правка строки (pauseImportForEdit) закрытием НЕ считается - там панель прячется,
         * а не закрывается, и сюда не заходит.
         */
        closeImportMode() {
            const dropped = this.dropPendingRows();
            this.importMode = false;
            this.importPausedForEdit = false;
            this.importResult = null;

            if (dropped > 0) {
                useDeletionsStore().notify({
                    bold: `Разбор бланка закрыт: убрано строк ${dropped}`,
                    suffix: ' - они не были добавлены в заявку',
                });
            }
        },

        /**
         * Убирает предварительные строки вложения. По умолчанию - текущего: при смене
         * вложения метод зовётся ДО подмены selectedAttachment, то есть чистит то, из
         * которого уходят.
         *
         * @param {Object} [attachment] вложение, если нужно не текущее
         * @returns {number} сколько строк убрано
         */
        dropPendingRows(attachment) {
            const target = attachment || this.selectedAttachment;
            if (!target) return 0;

            const rows = this.rowsForAttachment(target.attachment_type, this.attachmentKey(target));
            let dropped = 0;
            // Идём с конца: splice по ходу сдвигает индексы впереди стоящих строк.
            for (let i = rows.length - 1; i >= 0; i -= 1) {
                if (!isPendingRow(rows[i])) continue;
                rows.splice(i, 1);
                dropped += 1;
            }

            if (dropped > 0) this.saveToLocalStorage();
            return dropped;
        },

        resetImportResult() {
            this.importResult = null;
        },

        /**
         * Список рядом с панелью импорта остаётся кликабельным, а правка строки живёт в
         * форме ручного ввода, которую занимает панель. Панель уступает ей место, но
         * остаётся смонтированной: разбор файла, выбранные места и правки проблемных
         * строк живут в ней, и выход из режима стоил бы человеку всей загрузки.
         *
         * @param {Function} openEditor открывает нужную форму на правке строки
         */
        pauseImportForEdit(openEditor) {
            // Режим выключен или форма уже показана - открывать нечего, зовём сразу.
            if (!this.importMode || this.importPausedForEdit) {
                openEditor();
                return;
            }
            this.importPausedForEdit = true;
            this.$nextTick(openEditor);
        },

        /**
         * Правка завершена (сохранена или отменена) - сводка возвращается на место формы.
         */
        resumeImportAfterEdit() {
            if (this.importPausedForEdit) this.importPausedForEdit = false;
        },

        // Загрузка заполненного бланка (эпик blank-import, D1D2). Файл уходит сразу по
        // выбору/сбросу - интерфейс рассчитан на неопытного пользователя, отдельного шага
        // "подтвердить загрузку" нет. 200/207 оба показывают сводку (D2); строки с
        // ошибками разбирает сама сводка, здесь их не фильтруем.
        async uploadImportFile(file) {
            if (!this.selectedAttachment || this.importUploading) return;
            const uaId = this.selectedAttachment.template_id || this.selectedAttachment.id;
            this.importUploading = true;
            try {
                this.importResult = await uploadImportList(uaId, file);
            } catch (error) {
                useDeletionsStore().notify({
                    prefix: 'Не удалось загрузить список: ',
                    bold: error.message || 'ошибка сервера',
                    type: 'error',
                });
            } finally {
                this.importUploading = false;
            }
        },

        /**
         * Разобранные бланком строки встают в список СРАЗУ, предварительными (U5): человек
         * видит, что именно прочитано, и правит это в привычном списке, а не в таблице
         * разбора. Дедуп живёт здесь, а не на «Добавить»: строка входит в список в этот
         * момент, и дубль, показанный серым, пришлось бы гасить уже после того, как человек
         * его увидел и посчитал добавленным.
         */
        stageImportRows({ attachmentType, rows }) {
            // Сводка ждёт справочник гражданств, прежде чем отдать строки, и за это время
            // человек успевает выйти из режима или переключить вложение. Панель к тому
            // моменту уже размонтирована, но её эмит доходит - без гарда строки легли бы
            // в чужое вложение.
            if (!this.importMode || !this.selectedAttachment) return;
            if (this.selectedAttachment.attachment_type !== attachmentType) return;

            const staged = this.addImportedRows(attachmentType, rows, true);
            if (staged > 0) {
                useDeletionsStore().notify({
                    bold: `Строк из бланка: ${staged}`,
                    suffix: ' - показаны в списке предварительно, выберите места и нажмите «Добавить»',
                });
            }
        },

        /**
         * «Добавить» в сводке: предварительные строки становятся обычными и получают
         * выбранные места, а вручную исправленные проблемные строки приходят этим же
         * событием и заводятся сразу обычными.
         *
         * @param {{attachmentType: string, rows: Array<Object>, places?: Object}} payload
         */
        handleImportRows({ attachmentType, rows, places }) {
            // Тот же гард, что у stageImportRows: сейчас эмит синхронный и разойтись
            // нечему, но стоит появиться ожиданию перед ним - строки лягут в чужое
            // вложение. Защита стоит дешевле, чем разбор такого случая потом.
            if (!this.selectedAttachment) return;
            if (this.selectedAttachment.attachment_type !== attachmentType) return;

            const fixedRows = rows || [];
            if (!fixedRows.length && this.pendingImportCount === 0) return;

            const added = this.addImportedRows(attachmentType, fixedRows, false);
            const activated = this.activatePendingRows(attachmentType, places);

            if (added + activated > 0) {
                useDeletionsStore().notify({
                    bold: `Добавлено строк: ${added + activated}`,
                    suffix: ' из импортированного бланка',
                });
            }

            // Строки в списке - режим отработал, возвращаем форму ручного ввода.
            this.closeImportMode();
        },

        /**
         * Снимает предварительность со строк текущего вложения и раскатывает по ним
         * выбранные в сводке места (в файле их нет и не будет - решение владельца).
         *
         * @param {string} attachmentType cars | people
         * @param {Object} places патч полей мест, общий для всех строк пачки
         * @returns {number} сколько строк стало обычными
         */
        activatePendingRows(attachmentType, places) {
            if (!this.selectedAttachment) return 0;

            const rows = this.rowsForAttachment(attachmentType, this.attachmentKey(this.selectedAttachment));
            let activated = 0;
            rows.forEach((row, index) => {
                if (!isPendingRow(row)) return;
                const activeRow = { ...row, ...(places || {}) };
                delete activeRow.isPending;
                rows.splice(index, 1, activeRow);
                activated += 1;
            });

            if (activated > 0) this.saveToLocalStorage();
            return activated;
        },

        // Строки уходят в список заявки ТЕМ ЖЕ путём, что ручное массовое добавление
        // (существующие handleEmployeesAdded/handleVehiclesAdded) - без своей копии логики
        // создания строк (см. addExistingEmployees/addExistingCars).
        // Дубли ВНУТРИ файла бэкенд уже отсеивает при разборе (attachment_import_validate.go,
        // строка попадает в rows только один раз) - здесь только пересечение с тем, что уже
        // добавлено в текущее вложение заявки, теми же findDuplicateEmployee/findDuplicateVehicle,
        // что и ручной ввод. Строку-в-строку внутри самой пачки (правка проблемных строк вручную
        // может свести две разные строки к одному человеку/машине) та же накопительная проверка
        // гасит побочным эффектом - как и addExistingEmployees/addExistingCars для каталога.
        addImportedRows(attachmentType, rows, pending) {
            if (!rows || !rows.length) return 0;
            const isPeople = attachmentType === 'people';
            const findDuplicate = isPeople ? findDuplicateEmployee : findDuplicateVehicle;
            const label = isPeople ? employeeLabel : vehicleLabel;

            // employeeLabel фолбэком показывает паспорт, если ФИО пусто (так он и задуман
            // для ручного ввода). В импорте это означало бы паспорт в тексте уведомления,
            // поэтому здесь безымянная строка называется обезличенно.
            const safeLabel = (row) => {
                const text = label(row);
                if (!isPeople) return text;
                const hasName = [row.lastName, row.firstName, row.middleName]
                    .some((part) => (part || '').trim());
                return hasName ? text : 'Строка без ФИО';
            };

            const list = [...(isPeople ? this.employees : this.vehicles)];
            const toAdd = [];
            const skipped = [];
            rows.forEach((row) => {
                if (findDuplicate(list, row)) {
                    skipped.push(safeLabel(row));
                    return;
                }
                list.push(row);
                toAdd.push(row);
            });

            if (skipped.length > 0) {
                // Файл может нести до 2000 строк - в тост попадают первые несколько имён,
                // а не вся пачка целиком (SystemTableColumnsTab.vue - тот же приём "и ещё N").
                const shown = skipped.slice(0, 5).join(', ');
                const more = skipped.length > 5 ? ` и ещё ${skipped.length - 5}` : '';
                useDeletionsStore().notify({
                    prefix: `${shown}${more} `,
                    bold: skipped.length > 1
                        ? 'уже в списке - пропущены при импорте'
                        : (isPeople ? 'уже в списке - пропущен при импорте' : 'уже в списке - пропущена при импорте'),
                    type: 'error',
                });
            }

            if (toAdd.length === 0) return 0;

            const marked = pending ? toAdd.map((row) => ({ ...row, isPending: true })) : toAdd;
            if (isPeople) {
                this.handleEmployeesAdded(marked);
            } else {
                this.handleVehiclesAdded(marked);
            }
            return marked.length;
        },

        async handleAttachmentSelected(attachment) {
            // Смена вложения - гасим панель предупреждений; новая форма пришлёт свои
            // группы через @notices-change (иначе стейл-группы прошлого вложения
            // мелькнут до пересчёта). Это реальный путь переключения между вложениями.
            this.placeNotices = [];
            // Режим импорта (D1D2, U4) - у другого вложения свой шаблон/право, открытая
            // панель и разобранный файл иначе переживают переключение.
            this.closeImportMode();
            this.preserveFormHeight();

            if (!attachment) {
                this.selectedAttachment = null;
                this.clearFormData();
                return;
            }

            const uaId = attachment.template_id || attachment.id;
            // Конфиг ждём ДО показа формы: без него fieldVisible деградирует к
            // «видимы все» и тумблеры «Дополнительно» моргают полным списком шаблона.
            const seq = ++this.attachmentSelectSeq;
            await this.loadFieldConfig(uaId);
            if (seq !== this.attachmentSelectSeq) return;

            this.selectedAttachment = attachment;
            this.restoreAttachmentData(attachment);
        },

        attachmentKey(attachment) {
            if (!attachment) return null;
            return attachment.local_id || attachment.id;
        },

        /**
         * «Редактировать» в списке заполняет форму, которая стоит ВЫШЕ списка: на
         * телефоне без прокрутки кажется, что кнопка не сработала. Ведём к началу
         * формы; на десктопе форма и список рядом - скролл только мешал бы.
         */
        scrollToEntityForm() {
            if (typeof window === 'undefined' || typeof window.matchMedia !== 'function'
                || !window.matchMedia('(max-width: 768px)').matches) return;
            this.$nextTick(() => {
                const el = this.$el && this.$el.querySelector('.form__data');
                if (!el) return;
                const top = el.getBoundingClientRect().top + window.scrollY - 64;
                window.scrollTo({ top: Math.max(top, 0), behavior: 'smooth' });
            });
        },

        /**
         * Пока форма пересобирается под другое вложение, документ на миг схлопывается
         * на её высоту - мобильный браузер клампит прокрутку, и страницу выбрасывает
         * к началу. Держим высоту карточки, пока новая форма не встала.
         */
        preserveFormHeight() {
            if (typeof window === 'undefined' || typeof window.matchMedia !== 'function'
                || !window.matchMedia('(max-width: 768px)').matches) return;
            const el = this.$el && this.$el.querySelector('.create__form');
            if (!el) return;
            // Токен поколения: при быстром переключении вложений хвост раннего
            // вызова не должен снять подпорку, поставленную поздним.
            const seq = (this.preserveHeightSeq = (this.preserveHeightSeq || 0) + 1);
            el.style.minHeight = `${el.offsetHeight}px`;
            this.$nextTick(() => {
                window.requestAnimationFrame(() => {
                    window.requestAnimationFrame(() => {
                        if (seq !== this.preserveHeightSeq) return;
                        el.style.minHeight = '';
                    });
                });
            });
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

        async handleAttachmentAdded(attachment) {
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

            // Черновик пишем до ожидания конфига: вложение уже в списке, и его
            // сохранность не должна зависеть от того, дождались ли мы выбора.
            this.saveToLocalStorage();

            const uaId = attachment.template_id || attachment.id;
            const seq = ++this.attachmentSelectSeq;
            await this.loadFieldConfig(uaId);
            if (seq !== this.attachmentSelectSeq) return;

            this.selectAttachment(attachment);
        },

        selectAttachment(attachment) {
            // Добавление нового вложения идёт сюда, минуя handleAttachmentSelected, и без
            // этого панель импорта переживала смену: у вложения другого типа кнопки
            // «Импорт» нет вовсе, так что закрыть её штатно было нечем, а сводка
            // показывала числа от прошлого вложения.
            if (this.importMode && this.attachmentKey(this.selectedAttachment) !== this.attachmentKey(attachment)) {
                this.closeImportMode();
            }
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
                vehicles.splice(index, 1, this.keepPendingFlag(vehicles[index], updatedVehicle));
                this.saveToLocalStorage();
            }
            this.resumeImportAfterEdit();
        },

        handleVehicleEditCancelled() {
            this.resumeImportAfterEdit();
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
            this.pauseImportForEdit(() => {
                if (!this.$refs.vehicleForm) return;
                this.$refs.vehicleForm.editVehicle(vehicle);
                this.scrollToEntityForm();
            });
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
                employees.splice(index, 1, this.keepPendingFlag(employees[index], updatedEmployee));
                this.saveToLocalStorage();
            }
            this.resumeImportAfterEdit();
        },

        /**
         * Формы отдают свой объект строки и про предварительность ничего не знают -
         * без переноса флага правка серой строки молча делала бы её обычной.
         *
         * @param {Object} previous строка до правки
         * @param {Object} updated строка, собранная формой
         * @returns {Object}
         */
        keepPendingFlag(previous, updated) {
            return isPendingRow(previous) ? { ...updated, isPending: true } : updated;
        },

        handleEmployeeEditCancelled() {
            this.resumeImportAfterEdit();
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

        /**
         * «Очистить» в шапке списка (blank-import-ux, U6): убирает ВСЕ строки текущего
         * вложения - и заведённые руками, и предварительные из бланка. Подтверждение
         * спрашивает сам список, здесь только удаление: отмены на странице нет.
         * Сводка импорта считает готовые к добавлению по pendingImportCount, поэтому
         * после очистки она сама показывает ноль и не обещает добавить пустоту.
         *
         * @param {string} attachmentType cars | people
         */
        clearList(attachmentType) {
            if (!this.selectedAttachment) return;
            if (this.selectedAttachment.attachment_type !== attachmentType) return;

            const key = this.attachmentKey(this.selectedAttachment);
            const rows = this.rowsForAttachment(attachmentType, key);
            const removed = rows.length;
            if (removed === 0) return;

            // splice, а не новый массив: на этот же экземпляр смотрят restoreAttachmentData
            // и черновик, подмена ссылки оставила бы их со старыми строками.
            rows.splice(0);
            this.saveToLocalStorage();

            useDeletionsStore().notify({
                bold: `Убрано строк: ${removed}`,
                suffix: attachmentType === 'people'
                    ? ' из списка сотрудников'
                    : ' из списка транспортных средств',
            });
        },

        editEmployee(employee) {
            this.pauseImportForEdit(() => {
                if (!this.$refs.employeeForm) return;
                this.$refs.employeeForm.editEmployee(employee);
                this.scrollToEntityForm();
            });
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
                this.scrollToEntityForm();
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

        // Выбор подсказки связывает поле с записью справочника, ручная правка связь рвёт
        // (компонент шлёт null). Без id подача уходит наименованием, и сервер сам решает:
        // найдёт по ключу дедупликации - привяжет, не найдёт - заведёт запись на проверке.
        applyOrganizationChoice(choice) {
            this.organizationId = choice ? choice.id : null;
        },

        applyCompanyChoice(choice) {
            this.companyId = choice ? choice.id : null;
        },

        /**
         * @param {string} field - Имя проверяемого поля
         * @param {{live?: boolean}} [options] - live: проверка по ходу ввода, когда
         *   недобранный номер ещё не повод показывать ошибку (жалоба userbugs-0728).
         */
        validateField(field, options = {}) {
            const live = options?.live === true;

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
                case 'phone': {
                    if (!this.phoneNumber) {
                        this.errors.phone = live ? '' : 'Обязательное поле';
                        break;
                    }
                    const complete = String(this.phoneNumber).replace(/\D/g, '').length >= 11;
                    this.errors.phone = (live && !complete) || isValidRussianPhone(this.phoneNumber)
                        ? ''
                        : 'Введите корректный номер';
                    break;
                }
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
            // Гард от повторного клика: пока летит подача (проверки + сам POST,
            // включая время внутри модалки привязки и после неё), второй клик
            // по кнопке - no-op, а не второй параллельный прогон.
            if (this.isSubmitting) return;
            this.isSubmitting = true;
            // Модалка привязки доводит подачу до sendCompleteApplication сама
            // (confirmBinding/skipBinding) и сама снимает isSubmitting по
            // завершении - здесь снимаем только если до неё дело не дошло.
            let handedOffToBindingModal = false;

            try {
                this.validateAllFields();

                if (!this.canSubmit) {
                    useDeletionsStore().notify({ prefix: 'Заполните все обязательные поля во всех вложениях', type: 'error' });
                    return;
                }

                // Формы гасят дубли при добавлении, но черновик из localStorage мог накопить их
                // раньше - такую заявку на бэк не пускаем.
                const duplicate = this.findDuplicateEntry();
                if (duplicate) {
                    useDeletionsStore().notify({
                        prefix: `Во вложении "${duplicate.attachmentName}" повторяется `,
                        bold: duplicate.label,
                        suffix: ' - удалите лишнюю строку',
                        type: 'error',
                    });
                    return;
                }

                // Проверяем активные машины
                const activeVehicles = await this.checkVehiclesBeforeSubmit();

                if (activeVehicles.length > 0) {
                    const vehicleList = activeVehicles.map(v => this.formatActiveVehicleConflict(v)).join('; ');
                    useDeletionsStore().notify({ prefix: 'Невозможно отправить заявку. Уже в активных заявках: ', bold: vehicleList, type: 'error' });
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
                    // Модалка привязки перекрывает форму оверлеем, а confirmBinding/
                    // skipBinding доводят подачу до sendCompleteApplication и сами
                    // снимают isSubmitting по завершении - кнопка остаётся
                    // заблокированной весь путь, а не только пока модалка открыта.
                    handedOffToBindingModal = true;
                    this.showBindingModal = true;
                } else {
                    await this.sendCompleteApplication();
                }
            } finally {
                if (!handedOffToBindingModal) {
                    this.isSubmitting = false;
                }
            }
        },

        /** Первый повтор человека/машины среди вложений: { attachmentName, label } или null. */
        findDuplicateEntry() {
            for (const attachment of this.attachments) {
                const key = this.attachmentKey(attachment);

                // Гард стережёт ТЕЛО подачи, поэтому смотрит на те же строки, что в него
                // уйдут - предварительные в нём не участвуют.
                if (attachment.attachment_type === 'people') {
                    const employee = findFirstDuplicate(submittedRows(this.employeesByAttachment[key]), isSameEmployee);
                    if (employee) {
                        return { attachmentName: attachment.display_name, label: employeeLabel(employee) };
                    }
                }

                if (attachment.attachment_type === 'cars') {
                    const vehicle = findFirstDuplicate(submittedRows(this.vehiclesByAttachment[key]), isSameVehicle);
                    if (vehicle) {
                        return { attachmentName: attachment.display_name, label: vehicleLabel(vehicle) };
                    }
                }
            }

            return null;
        },

        async collectNewDataForBinding() {
            this.newVehiclesToBind = [];
            this.newEmployeesToBind = [];
            
            const existingVehicles = await this.loadExistingVehicles();
            const existingEmployees = await this.loadExistingEmployees();
            
            // Привязка заводит записи в справочнике под то, что уйдёт в заявку -
            // предварительные строки туда не входят и справочник не трогают.
            Object.keys(this.vehiclesByAttachment).forEach(attachmentId => {
                const vehicles = submittedRows(this.vehiclesByAttachment[attachmentId]);
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
                const employees = submittedRows(this.employeesByAttachment[attachmentId]);
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
            // Гард от повторного клика внутри самой модалки: пока привязка уже летит
            // (mapWithConcurrency по сотням строк массового импорта - окно широкое),
            // второй emit того же confirm-binding - no-op, не второй набор запросов.
            if (this.isBindingActionInProgress) return;
            this.isBindingActionInProgress = true;

            try {
                if (this.newVehiclesToBind.length > 0 && bindingData.vehicles.hasVehiclesForBinding) {
                    const vehiclesToBind = this.newVehiclesToBind.filter(vehicle => 
                        vehicle.plateNumber !== 'По факту' && vehicle.mark !== 'По факту'
                    );
                    
                    if (vehiclesToBind.length > 0) {
                        await mapWithConcurrency(vehiclesToBind, BIND_CONCURRENCY, (vehicle) => {
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
                    }
                }

                if (this.newEmployeesToBind.length > 0 && bindingData.employees.hasEmployeesForBinding) {
                    // Запись реестра хранит персональные данные, поэтому создаётся только
                    // с подтверждённым согласием субъекта (гейт в uniqueEmployeeService.Create).
                    // Отсеиваем неотмеченных здесь, чтобы не получать 400 на привязке: сама
                    // заявка при этом уходит, просто человек не попадает в справочник.
                    const employeesToBind = this.newEmployeesToBind.filter(employee =>
                        employee.passportSeriesNumber !== 'По факту' && employee.position !== 'По факту'
                        && employee.pdConsent === true
                    );
                    
                    if (employeesToBind.length > 0) {
                        await mapWithConcurrency(employeesToBind, BIND_CONCURRENCY, (employee) => {
                            const employeeData = {
                                last_name: employee.lastName,
                                first_name: employee.firstName,
                                middle_name: employee.middleName,
                                position: employee.position,
                                citizenship_id: employee.citizenshipId,
                                passport_series_number: employee.passportSeriesNumber,
                                patent_number: employee.patentNumber,
                                other_permission: employee.otherPermission,
                                pd_consent: true,
                                user_id: null,
                                organization_id: bindingData.employees.bindToOrganization ? this.organizationId : null,
                                company_id: bindingData.employees.bindToCompany ? this.companyId : null
                            };

                            return apiRequest("/unique-employees", {
                                method: "POST",
                                body: JSON.stringify(employeeData)
                            });
                        });
                    }
                }

                this.closeBindingModal();
                await this.sendCompleteApplication();

            } catch (error) {
                console.error('Ошибка при привязке:', error);
                this.closeBindingModal();
            } finally {
                this.isBindingActionInProgress = false;
                this.isSubmitting = false;
            }
        },

        async skipBinding() {
            // Тот же гард, что и в confirmBinding - "Отправить без привязки" тоже
            // не должен запускать sendCompleteApplication дважды на двойной клик.
            if (this.isBindingActionInProgress) return;
            this.isBindingActionInProgress = true;

            this.closeBindingModal();
            try {
                await this.sendCompleteApplication();
            } finally {
                this.isBindingActionInProgress = false;
                this.isSubmitting = false;
            }
        },

        closeBindingModal() {
            this.showBindingModal = false;
            this.newVehiclesToBind = [];
            this.newEmployeesToBind = [];
        },

        /** Пользователь закрыл модалку сам (крестик/оверлей/Escape/свайп), не подтвердив
         * и не пропустив привязку - продолжения не будет, снимаем isSubmitting здесь,
         * иначе кнопка "Отправить заявку" зависнет в состоянии "Отправляем..." навсегда.
         * Пока confirmBinding/skipBinding уже работают (isBindingActionInProgress) -
         * модалка сама блокирует эти пути (проп processing), это - страховка на случай
         * программного вызова: закрыть/отменить фоновый прогон отсюда нельзя, иначе он
         * останется невидимым и всё равно дойдёт до sendCompleteApplication. */
        cancelBindingModal() {
            if (this.isBindingActionInProgress) return;
            this.closeBindingModal();
            this.isSubmitting = false;
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

        /**
         * Строки вложения по его типу: формы хранят их в трёх раздельных словарях.
         *
         * @param {string} attachmentType cars | people | items
         * @param {string|number} key ключ вложения (local_id либо id)
         * @returns {Array<Object>}
         */
        rowsForAttachment(attachmentType, key) {
            switch (attachmentType) {
                case 'cars': return this.vehiclesByAttachment[key] || [];
                case 'people': return this.employeesByAttachment[key] || [];
                case 'items': return this.itemsByAttachment[key] || [];
                default: return [];
            }
        },
        async sendCompleteApplication() {
            if (this.attachments.length === 0) {
                useDeletionsStore().notify({ prefix: 'Добавьте вложения для отправки', type: 'error' });
                return;
            }

            const applicationData = {
                message: this.message || null,
                file_ids: this.applicationFileIds,
                // Контракт подачи #1437: id, когда поле связано с записью справочника
                // (профиль или выбранная подсказка), наименование - когда введено руками.
                // Заполнять оба не нужно: при заданном id наименование не смотрится.
                organization_id: this.organizationId || null,
                organization_name: this.organizationId ? null : (this.organization || null),
                company_id: this.companyId || null,
                company_name: this.companyId ? null : (this.company || null),
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

                // Предварительные строки (U5) в тело подачи не попадают: они живут в
                // черновике до нажатия «Добавить» в сводке импорта.
                Object.assign(
                    attachmentData.data,
                    toAttachmentContent(
                        attachment.attachment_type,
                        submittedRows(this.rowsForAttachment(attachment.attachment_type, key)),
                    )
                );

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
                    body: JSON.stringify(finalRequestData),
                    // Массовый импорт даёт до 2000 строк - дефолтных 10с apiRequest не хватает.
                    signal: createExtendedTimeoutSignal(SUBMIT_TIMEOUT_MS),
                });

                if (response.ok) {
                    const result = await response.json();
                    // Файлы уже привязаны к заявке - список в форме больше не нужен.
                    this.applicationFileIds = [];
                    this.$refs.filesUpload?.reset();
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
                    notifyApiError('Ошибка отправки заявки: ', await response.text());
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

        /**
         * Учётная запись, которой принадлежит черновик. Берётся из маркера доступа:
         * данные профиля в форму приезжают асинхронно, а сохранение может случиться
         * раньше них.
         * @returns {number|null}
         */
        draftOwnerId() {
            return useAuthStore().userPayload?.user_id ?? null;
        },

        /**
         * Черновик текущего работника либо null. Чужой (и старый, без владельца)
         * стирается: браузер на проходной и в бюро один на смену, и после смены
         * учётной записи форма подставляла организацию и компанию предыдущего - сервер
         * такую заявку отбивал отказом «подать от другой организации», а человек видел
         * в своей форме чужое место работы.
         * @returns {object|null}
         */
        readOwnDraft() {
            const raw = localStorage.getItem('draftApplicationState');
            if (!raw) return null;
            try {
                const draft = JSON.parse(raw);
                const me = this.draftOwnerId();
                // Пока маркер не разобран, судить о владельце не по чему - отдаём как есть,
                // иначе черновик пропадал бы на ровном месте при задержке восстановления сеанса.
                if (me === null || draft?.ownerId === me) return draft;
                localStorage.removeItem('draftApplicationState');
                return null;
            } catch (e) {
                console.error('Битый draftApplicationState:', e);
                localStorage.removeItem('draftApplicationState');
                return null;
            }
        },

        saveToLocalStorage() {
            try {
                const hasAttachments = this.attachments.length > 0;

                const savedData = {
                    ownerId: this.draftOwnerId(),
                    message: this.message,
                    organization: this.organization,
                    company: this.company,
                    // id идут в черновик рядом с наименованиями, чтобы пара не разъехалась
                    // при восстановлении: текст чужой организации с id профиля отправил бы
                    // заявку не от той записи.
                    organizationId: this.organizationId,
                    companyId: this.companyId,
                    responsiblePerson: this.responsiblePerson,
                    phoneNumber: this.phoneNumber,
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
                // Импорт бланком на 2000 строк с паспортами/патентами - сотни килобайт
                // черновика, квота браузера (обычно 5-10 МБ на источник) может кончиться.
                // Форма при этом не должна молчать или падать: данные всё ещё в памяти
                // Vue и уйдут на сервер по кнопке "Отправить", теряется только автосохранение.
                if (isQuotaExceededError(error)) {
                    useDeletionsStore().notify({
                        prefix: 'Не удалось сохранить черновик заявки: в хранилище браузера закончилось место. ',
                        suffix: 'Данные пока не потеряны - продолжайте заполнение и отправьте заявку, не закрывая вкладку.',
                        type: 'error',
                    });
                }
            }
        },

        async restoreFromLocalStorage() {
            // Восстановление ждёт конфиг полей, а «Заменить/Объединить» в конфликте
            // дублей запускает его повторно: без токена поздний ответ первого прогона
            // открыл бы вложение прежнего черновика поверх выбранного пользователем.
            const seq = ++this.restoreSeq;
            try {
                const parsedData = this.readOwnDraft();
                if (parsedData) {

                    this.message = parsedData.message || '';
                    // Шапку заявителя берём из черновика только если она там есть. Пустое
                    // значение не затираем: организация, компания, ФИО и телефон приходят
                    // из записи работника и уже подставлены, а поля нередактируемы -
                    // затёртое вручную не вернуть. Черновики, сохранённые до этой правки,
                    // несут пустую шапку и без проверки обнуляли форму при каждом открытии.
                    if (parsedData.organization) this.organization = parsedData.organization;
                    if (parsedData.company) this.company = parsedData.company;
                    // Черновики, сохранённые до #1437, id не несут: у них поле было
                    // нередактируемым, поэтому наименование отвечает записи профиля.
                    if ('organizationId' in parsedData) this.organizationId = parsedData.organizationId || null;
                    if ('companyId' in parsedData) this.companyId = parsedData.companyId || null;
                    if (parsedData.responsiblePerson) this.responsiblePerson = parsedData.responsiblePerson;
                    if (parsedData.phoneNumber) this.phoneNumber = formatRussianPhone(parsedData.phoneNumber);
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
                        const first = this.attachments[0];
                        // Конфиг полей ждём ДО показа формы: без него fieldVisible деградирует
                        // к «видимы все» и в «Дополнительно» встают лишние тумблеры шаблона.
                        const selectSeq = ++this.attachmentSelectSeq;
                        await this.loadFieldConfig(first.template_id || first.id);
                        if (seq !== this.restoreSeq) return;
                        // Список вложений кликабелен, пока грузится конфиг: явный выбор
                        // пользователя за это время важнее нашего «открыть первое».
                        if (selectSeq !== this.attachmentSelectSeq) return;
                        this.selectedAttachment = first;
                        this.restorePendingImport();
                    }
                }
            } catch (error) {
                console.error('Ошибка восстановления состояния из localStorage:', error);
            }
        },

        /**
         * Черновик перезагрузку переживает, разбор бланка - нет: в localStorage лежат
         * только строки. Серая строка без сводки бесполезна (принять её нечем и убрать
         * штатно тоже), поэтому после восстановления сводка открывается по ним сама.
         * Где открыть её невозможно - строки убираем, чтобы список не врал.
         */
        restorePendingImport() {
            // Сводка открывается только у выбранного вложения; у остальных серые строки
            // остались бы без хозяина (черновики прежних версий могли их накопить).
            const currentKey = this.attachmentKey(this.selectedAttachment);
            this.attachments.forEach((attachment) => {
                if (this.attachmentKey(attachment) === currentKey) return;
                this.dropPendingRows(attachment);
            });

            if (this.pendingImportCount === 0) return;
            if (this.canImportList) {
                this.importMode = true;
                return;
            }

            const dropped = this.dropPendingRows();
            if (dropped > 0) {
                useDeletionsStore().notify({
                    bold: `Строки из бланка убраны: ${dropped}`,
                    suffix: ' - массовый ввод для этого вложения недоступен',
                    type: 'error',
                });
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
            // Дубль, отложенный другой учётной записью, не подхватываем: браузер общий.
            const me = this.draftOwnerId();
            if (me !== null && pending?.ownerId !== me) {
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
            const d = this.readOwnDraft();
            if (!d) return false;
            return (Array.isArray(d.attachments) && d.attachments.length > 0)
                || (typeof d.message === 'string' && d.message.trim().length > 0);
        },

        // Применяет дубль: replace - целиком заменяет черновик; merge - дописывает вложения
        // дубля к существующим (шапка/сообщение не трогаются). Затем перечитывает состояние.
        applyPendingDuplicate(pending, mode) {
            let finalDraft = pending;
            if (mode === 'merge') {
                finalDraft = this.mergeDrafts(this.readOwnDraft() || {}, pending);
            }
            // Шапку заявителя дубль не несёт: организация, компания, ФИО и телефон берутся
            // из записи работника и в форме уже заполнены. Без переноса восстановление
            // затрёт их пустыми значениями, а поля заблокированы - вернуть нечем.
            finalDraft = {
                organization: this.organization,
                company: this.company,
                organizationId: this.organizationId,
                companyId: this.companyId,
                responsiblePerson: this.responsiblePerson,
                phoneNumber: this.phoneNumber,
                ...finalDraft,
                ownerId: this.draftOwnerId(),
            };
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
                    this.phoneNumber = formatRussianPhone(data.contact_phone || '');
                    
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
        color: var(--accent-text);
        padding: 0 10px;
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 5px;
        border-radius: 50px;
        background: var(--surface);
        border: 1px solid var(--border);
        outline: none;
        cursor: pointer;
        height: 25px;
    }

    .tables__icon {
        width: 15px;
        height: 15px;
    }

    .tables__instruction:hover {
        background-color: var(--surface-2);
    }

    .create__form {
        width: 100%;
        /* Форма - flex-элемент .create__container рядом с панелью бланков. Без
           min-width:0 у неё дефолтный min-width:auto = ширина по min-content
           содержимого (после поля организации #1451 min-content вырос до ~1162px),
           поэтому на узком слоте (1440 -> ~1120px) форма не сжимается и уезжает
           за правый край, давая горизонтальный скролл страницы (ломает fixed-меню).
           min-width:0 позволяет ужаться до доступной ширины; содержимое (message-tc,
           info-row) само сжимается/переносится. */
        min-width: 0;
        height: fit-content;
        background-color: var(--surface);
        border: 1px solid var(--border);
        border-radius: 30px;
        box-shadow: 0 3px 10px var(--shadow-drop);
    }

    /* Шапка - две колонки: поле сообщения слева, согласие + отправка справа */
    .form__header {
        width: 100%;
        border-bottom: 1px solid var(--border);
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
        background: var(--surface-2);
        border: 1px solid var(--border);
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
        color: var(--text);
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
        background: var(--hint-bg);
        color: var(--hint-text);
        padding: 12px;
        border-radius: 8px;
        font-size: 12px;
        width: 380px;
        max-height: 400px;
        overflow-y: auto;
        z-index: 1000;
        pointer-events: auto;
        box-shadow: 0 2px 8px var(--shadow-drop);
        white-space: normal;
    }

    .submit-tooltip::before {
        content: '';
        position: absolute;
        top: 8px;
        left: 100%;
        border: 5px solid transparent;
        border-left-color: var(--hint-bg);
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
        color: var(--accent-contrast);
    }

    .tooltip-attachment-title {
        font-weight: 600;
        margin-bottom: 6px;
        color: var(--border);
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
        background: var(--accent);
        color: var(--accent-contrast);
        border: none;
        border-radius: 15px;
        padding: 8px 15px;
        font-size: 12px;
        cursor: pointer;
        transition: background-color 0.2s;
        width: auto;
        display: inline-flex;
        align-items: center;
        gap: 6px;
        flex-shrink: 0;
        height: fit-content;
    }

    .send-all-btn:hover:not(:disabled) {
        background: var(--accent-hover);
    }

    .send-all-btn:disabled {
        background: var(--text-muted);
        cursor: not-allowed;
        opacity: 0.6;
    }

    /* Крутилка на кнопке подачи - вместе со сменой текста объясняет, ПОЧЕМУ кнопка
       погасла: голый disabled без визуальной причины читается как мёртвая форма. */
    .send-all-btn__spinner {
        width: 11px;
        height: 11px;
        border-radius: 50%;
        border: 2px solid color-mix(in srgb, var(--accent-contrast) 35%, transparent);
        border-top-color: var(--accent-contrast);
        animation: send-all-btn-spin 0.7s linear infinite;
        flex-shrink: 0;
    }

    @keyframes send-all-btn-spin {
        to { transform: rotate(360deg); }
    }

    /* Как в LoaderSpinner.vue: не гасим крутилку совсем (текст "Отправляем..." сам по
       себе не объясняет, что форма ещё жива), а замедляем - тот же приём, что там. */
    @media (prefers-reduced-motion: reduce) {
        .send-all-btn__spinner {
            animation-duration: 2.4s;
        }
    }

    .submit-progress-hint {
        margin-top: 8px;
        max-width: 320px;
        font-size: 12px;
        line-height: 1.4;
        color: var(--text-muted);
        animation: submit-progress-hint-in 0.2s ease-out;
    }

    @keyframes submit-progress-hint-in {
        from {
            opacity: 0;
            transform: translateY(-4px);
        }
        to {
            opacity: 1;
            transform: translateY(0);
        }
    }

    .form__info-row {
        padding: 15px;
        display: flex;
        gap: 50px;
        border-bottom: 1px solid var(--border);
        /* Поднимаем ряд дат над form__data: иначе выпадающее меню "Быстрый выбор"
           (открывается вниз) уходит под серый гейт-оверлей и кнопки формы ниже. */
        position: relative;
        z-index: 1;
    }

    .create__blank-title--inline {
        display: none;
    }

    /* На десктопе inline-заголовок скрыт (он в шапке) - обёртка не должна занимать
       место; на мобилке @media переопределит на sticky-контейнер. */
    .create__blank-sticky {
        display: contents;
    }

    /* Маркер для IntersectionObserver: ловит момент закрепления заголовка. */
    .create__blank-sentinel {
        height: 0;
    }

    h4 {
        font-size: 22px;
        font-weight: 800;
        color: var(--accent-text);
        border: 1px solid var(--border);
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
        color: var(--accent-text);
    }

    .consent-link {
        text-decoration: underline;
        cursor: pointer;
    }

    .consent-link:hover {
        color: var(--accent-hover);
    }

    .form-placeholder {
        width: 100%;
        height: fit-content;
        min-height: 490px;
        background-color: var(--surface);
        border: 1px solid var(--border);
        border-radius: 30px;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .placeholder-content {
        text-align: center;
        color: var(--text-muted);
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

        .create__blank-title--header {
            display: none;
        }

        /* Обёртка заголовка закрепляется под шапкой приложения при скролле формы:
           в потоке подложки нет, а закрепившись (blankStuck) - полупрозрачная
           поверхность темы с размытием и нижняя граница, чтобы отделиться от контента. */
        .create__blank-sticky {
            position: sticky;
            top: var(--mobile-header-height, 55px);
            z-index: 5;
            display: flex;
            justify-content: center;
            /* Full-bleed подложка гасит padding .create (= --gutter). margin И padding
               ОБА из --gutter, иначе на брейкпоинте где gutter меняется (768->12,
               480->10) хардкод -12px вылезает за экран на 2px -> горизонтальное
               переполнение -> Chrome ужимает страницу -> шапки уезжают на ~5px (#1282). */
            margin: 0 calc(-1 * var(--gutter, 12px));
            padding: 8px var(--gutter, 12px);
            background: transparent;
            border-bottom: 1px solid transparent;
            transition: background-color 0.2s ease, border-color 0.2s ease;
        }

        .create__blank-sticky--stuck {
            /* Поверхность темы, а не белый литерал: в тёмной теме закреплённая
               плашка светилась белым поверх тёмной формы. */
            background: color-mix(in srgb, var(--surface) 82%, transparent);
            backdrop-filter: blur(6px);
            -webkit-backdrop-filter: blur(6px);
            border-bottom-color: var(--border);
        }

        .create__blank-title--inline {
            display: block;
            align-self: center;
            margin: 0;
            text-align: center;
            font-size: 22px;
            padding: 8px 24px;
            /* Сама пилюля - плотный белый: при закреплении она выделяется на
               полупрозрачной подложке обёртки, а не сливается/просвечивает. */
            background: var(--surface);
        }

        .create__container {
            flex-direction: column;
            gap: 12px;
        }

        .form__header {
            height: auto;
            padding: 0 0 12px;
            flex-direction: column;
        }

        /* Сообщение во всю ширину карточки и вплотную к её верху: прямые углы
           блока торчали в скруглённом верхе карточки. Радиус на 1px меньше
           карточного - рамки совпадают. */
        .create__form .form__message-tc {
            width: 100%;
            border-left: none;
            border-right: none;
            border-top: none;
            border-radius: 29px 29px 0 0;
        }

        /* Панель форматирования - как в мобильном Outlook: одна компактная
           прокручиваемая строка иконок с разделителями групп вместо трёх рядов
           кнопок с рамками. */
        /* Свой фон и скругление под радиус карточки: прямоугольные крайние кнопки
           у скруглённых углов выглядели неровно. */
        .create__form :deep(.editor-toolbar) {
            flex-wrap: nowrap;
            overflow-x: auto;
            gap: 2px;
            padding: 6px 18px;
            background: var(--surface-2);
            border-radius: 29px 29px 0 0;
            scrollbar-width: none;
        }

        .create__form :deep(.editor-toolbar)::-webkit-scrollbar {
            display: none;
        }

        .create__form :deep(.toolbar-group) {
            flex: 0 0 auto;
            gap: 2px;
            padding-right: 8px;
            margin-right: 6px;
            border-right: 1px solid var(--border);
        }

        .create__form :deep(.toolbar-group:last-child) {
            border-right: none;
            margin-right: 0;
            padding-right: 0;
        }

        .create__form :deep(.toolbar-btn) {
            min-width: 34px;
            height: 34px;
            padding: 0 6px;
            border: none;
            border-radius: 8px;
            background: transparent;
        }

        /* Цвет глифа вместе с фоном: базовый .active красит букву белым под сплошную
           заливку - на бледной подложке она исчезала. */
        .create__form :deep(.toolbar-btn.active) {
            background: var(--color-primary-tint);
            color: var(--accent-text);
        }

        /* Часть мобильных браузеров держит :hover после тапа до следующего касания -
           глушим его на тулбаре принудительно, не полагаясь на media (hover). */
        .create__form :deep(.toolbar-btn:hover:not(.active)) {
            background: transparent;
            border-color: transparent;
        }

        .create__form :deep(.toolbar-btn:focus) {
            outline: none;
        }

        /* Полоса согласия и отправки возвращает себе боковые отступы, которые
           сняли с form__header ради full-bleed сообщения. */
        .form__header .form__submit-bar {
            width: calc(100% - 24px);
            margin: 0 12px;
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

        /* Причины блокировки: поверх контента НАД кнопкой - в потоке подсказка
           двигала форму, а её стрелка отрывалась от блока. */
        .submit-tooltip {
            position: absolute;
            right: auto;
            top: auto;
            bottom: calc(100% + 10px);
            left: 0;
            width: 100%;
            max-width: 100%;
            margin: 0;
            max-height: min(50dvh, 320px);
            z-index: 1100;
        }

        .submit-tooltip::before {
            top: 100%;
            left: 24px;
            border-color: transparent;
            border-top-color: var(--text);
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
