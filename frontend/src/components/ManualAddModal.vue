<template>
  <BaseModal
    :show="show"
    :title="title"
    width="840px"
    radius="30px"
    content-testid="manual-add-modal"
    @close="close"
  >
    <div class="manual-modal">
      <div
        v-if="canAttach"
        class="manual-modal__mode"
        data-testid="manual-add-mode"
      >
        <FilterTabs
          :tabs="modeTabs"
          :model-value="bindMode"
          @update:model-value="bindMode = $event"
        />
      </div>

      <p class="manual-modal__hint">
        <template v-if="isBinding">
          Записи попадут в таблицу «{{ tableName }}» и будут привязаны к выбранной
          заявке (доступно только администратору).
        </template>
        <template v-else>
          Записи попадут в таблицу «{{ tableName }}» без заявки и будут помечены
          «Добавлено вручную».
        </template>
      </p>

      <div
        v-if="isBinding"
        class="manual-modal__bind"
        data-testid="manual-add-bind"
      >
        <div class="manual-modal__field">
          <label class="manual-modal__label">
            Заявка <span class="manual-modal__req">*</span>
          </label>
          <BaseDropdown
            :model-value="selectedApplicationId"
            :options="applicationOptions"
            label-key="label"
            value-key="id"
            :searchable="true"
            :disabled="loadingApplications"
            :placeholder="loadingApplications ? 'Загрузка заявок...' : 'Выберите заявку'"
            data-testid="manual-add-application"
            @update:model-value="onApplicationChange"
          />
          <p
            v-if="!loadingApplications && !applicationOptions.length"
            class="manual-modal__note"
          >
            Нет активных согласованных заявок для привязки.
          </p>
        </div>

        <div
          v-if="selectedApplicationId"
          class="manual-modal__target"
          role="radiogroup"
        >
          <label class="manual-modal__radio">
            <input
              v-model="attachTarget"
              type="radio"
              value="new"
              data-testid="manual-target-new"
            >
            <span>Новое вложение в заявке</span>
          </label>
          <label class="manual-modal__radio">
            <input
              v-model="attachTarget"
              type="radio"
              value="existing"
              :disabled="!attachmentOptions.length"
              data-testid="manual-target-existing"
            >
            <span>
              Существующее вложение
              <template v-if="!loadingAttachments && !attachmentOptions.length">
                (нет подходящих)
              </template>
            </span>
          </label>
        </div>

        <div
          v-if="selectedApplicationId && attachTarget === 'existing'"
          class="manual-modal__field"
        >
          <label class="manual-modal__label">
            Вложение заявки <span class="manual-modal__req">*</span>
          </label>
          <BaseDropdown
            :model-value="selectedAttachmentId"
            :options="attachmentOptions"
            label-key="label"
            value-key="id"
            :searchable="true"
            :disabled="loadingAttachments"
            :placeholder="loadingAttachments ? 'Загрузка вложений...' : 'Выберите вложение'"
            data-testid="manual-add-attachment"
            @update:model-value="selectedAttachmentId = $event"
          />
        </div>
      </div>

      <div class="manual-modal__grid">
        <div class="manual-modal__field">
          <label class="manual-modal__label">
            Организация <span class="manual-modal__req">*</span>
          </label>
          <BaseDropdown
            :model-value="selectedOrgId"
            :options="organizations"
            label-key="name"
            value-key="id"
            :searchable="true"
            placeholder="Выберите организацию"
            data-testid="manual-add-org"
            @update:model-value="onOrgChange"
          />
        </div>
        <div class="manual-modal__field">
          <label class="manual-modal__label">Компания</label>
          <BaseDropdown
            :model-value="selectedCompanyId"
            :options="companies"
            label-key="name"
            value-key="id"
            :searchable="true"
            placeholder="Без компании"
            data-testid="manual-add-company"
            @update:model-value="onCompanyChange"
          />
        </div>
      </div>

      <div class="manual-modal__dates">
        <DateRangeSection
          :is-one-day="dateData.isOneDay"
          :start-date="dateData.startDate"
          :end-date="dateData.endDate"
          :single-date="dateData.singleDate"
          :start-time="dateData.startTime"
          :end-time="dateData.endTime"
          :roof-access="dateData.roofAccess"
          :free-parking="dateData.freeParking"
          :field-config="dateFieldConfig"
          @update:is-one-day="dateData.isOneDay = $event"
          @update:start-date="dateData.startDate = $event"
          @update:end-date="dateData.endDate = $event"
          @update:single-date="dateData.singleDate = $event"
          @update:start-time="dateData.startTime = $event"
          @update:end-time="dateData.endTime = $event"
          @update:roof-access="dateData.roofAccess = $event"
          @update:free-parking="dateData.freeParking = $event"
        />
      </div>

      <div
        class="manual-modal__form-section"
        :class="{ 'manual-modal__form-section--locked': !selectedOrgId }"
      >
        <div
          v-if="!selectedOrgId"
          class="manual-modal__form-lock"
          data-testid="manual-form-lock"
        >
          Сначала выберите организацию, чтобы {{ isPeople ? 'добавить сотрудников' : 'добавить машины' }}
        </div>

        <VehicleForm
          v-if="!isPeople"
          ref="vehicleForm"
          :user-organization="selectedOrgName"
          :user-organization-id="selectedOrgId"
          :user-company="selectedCompanyName"
          :user-company-id="selectedCompanyId"
          :existing-vehicles="addedVehicles"
          :allow-existing-search="false"
          :disabled="!selectedOrgId"
          @vehicle-added="handleVehicleAdded"
          @vehicles-added="handleVehiclesAdded"
          @vehicle-updated="handleVehicleUpdated"
        />
        <EmployeeForm
          v-else
          ref="employeeForm"
          :user-organization="selectedOrgName"
          :user-organization-id="selectedOrgId"
          :user-company="selectedCompanyName"
          :user-company-id="selectedCompanyId"
          :existing-employees="addedEmployees"
          :allow-existing-search="false"
          :disabled="!selectedOrgId"
          @employee-added="handleEmployeeAdded"
          @employees-added="handleEmployeesAdded"
          @employee-updated="handleEmployeeUpdated"
        />
      </div>

      <div
        v-if="!isPeople && addedVehicles.length"
        class="manual-modal__added"
        data-testid="manual-add-list"
      >
        <div class="manual-modal__added-title">
          К добавлению: {{ addedVehicles.length }}
        </div>
        <ul class="manual-added-list">
          <li
            v-for="vehicle in addedVehicles"
            :key="vehicle.id"
            class="manual-added-item"
          >
            <span class="manual-added-name">
              {{ vehicle.plateNumber }} - {{ vehicle.mark || 'марка не указана' }}
            </span>
            <div class="manual-added-actions">
              <button
                type="button"
                class="manual-added-btn"
                @click="editVehicle(vehicle)"
              >
                Изменить
              </button>
              <button
                type="button"
                class="manual-added-btn manual-added-btn--danger"
                @click="removeVehicle(vehicle)"
              >
                Удалить
              </button>
            </div>
          </li>
        </ul>
      </div>

      <div
        v-if="isPeople && addedEmployees.length"
        class="manual-modal__added"
        data-testid="manual-add-list"
      >
        <div class="manual-modal__added-title">
          К добавлению: {{ addedEmployees.length }}
        </div>
        <ul class="manual-added-list">
          <li
            v-for="employee in addedEmployees"
            :key="employee.id"
            class="manual-added-item"
          >
            <span class="manual-added-name">
              {{ employeeFullName(employee) }}
            </span>
            <div class="manual-added-actions">
              <button
                type="button"
                class="manual-added-btn"
                @click="editEmployee(employee)"
              >
                Изменить
              </button>
              <button
                type="button"
                class="manual-added-btn manual-added-btn--danger"
                @click="removeEmployee(employee)"
              >
                Удалить
              </button>
            </div>
          </li>
        </ul>
      </div>
    </div>

    <template #actions>
      <button
        type="button"
        class="manual-btn manual-btn--ghost"
        @click="close"
      >
        Отмена
      </button>
      <span
        class="hint-anchor hint-anchor--right"
        :data-hint="submitHint"
      >
        <button
          type="button"
          class="manual-btn manual-btn--primary"
          :disabled="!canSubmit"
          data-testid="manual-add-submit"
          @click="submit"
        >
          {{ submitLabel }}
        </button>
      </span>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import FilterTabs from '@/components/ui/FilterTabs.vue';
import DateRangeSection from '@/components/CreateApplication/DateRangeSection.vue';
import VehicleForm from '@/components/CreateApplication/VehicleForm.vue';
import EmployeeForm from '@/components/CreateApplication/EmployeeForm.vue';
import { getOrganizations, getCompanies } from '@/api/organizations';
import { createManualCars } from '@/api/cars';
import { createManualEmployees } from '@/api/employees';
import { getAttachableApplications, getApplicationAttachments } from '@/api/applications';
import { attachToApplication } from '@/api/attachments';
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';

// Привязка (режим-2) возможна только к активной согласованной заявке - те же
// критерии, что валидирует бэк (loadActiveApprovedApp), иначе привязка спрячет
// запись из таблиц проходной. Фильтруем дропдаун заявок этими значениями.
const APP_CONFIRMATION_APPROVED = 'Согласовано';
const APP_STATUS_IN_WORK = 'В работе';

function defaultDateData() {
    return {
        isOneDay: false,
        startDate: '',
        endDate: '',
        singleDate: '',
        startTime: '',
        endTime: '',
        roofAccess: false,
        freeParking: false,
    };
}

// Крыша/парковка - атрибуты авто-вложения; DTO ManualEmployeeRequest их не несёт,
// поэтому в people-режиме скрываем тумблеры (fieldVisible вернёт false).
const PEOPLE_DATE_FIELD_CONFIG = {
    roof_access: { visible: false },
    free_parking: { visible: false },
};

export default {
    name: 'ManualAddModal',
    components: { BaseModal, BaseDropdown, FilterTabs, DateRangeSection, VehicleForm, EmployeeForm },
    props: {
        show: {
            type: Boolean,
            default: false,
        },
        // 'cars' -> встраивает VehicleForm + POST /cars/manual; 'people' -> EmployeeForm
        // + POST /employees/manual. Задаётся из TablesComponent по table_type.
        mode: {
            type: String,
            default: 'cars',
            validator: v => ['cars', 'people'].includes(v),
        },
        tableId: {
            type: Number,
            default: null,
        },
        tableName: {
            type: String,
            default: '',
        },
    },
    emits: ['close', 'added'],
    data() {
        return {
            organizations: [],
            companies: [],
            selectedOrgId: null,
            selectedCompanyId: null,
            dateData: defaultDateData(),
            addedVehicles: [],
            vehicleIdCounter: 1,
            addedEmployees: [],
            employeeIdCounter: 1,
            submitting: false,
            // --- режим-2 (привязка к заявке) ---
            bindMode: 'none',
            applications: [],
            selectedApplicationId: null,
            appAttachments: [],
            attachTarget: 'new',
            selectedAttachmentId: null,
            loadingApplications: false,
            loadingAttachments: false,
            attachmentsFetchSeq: 0,
        };
    },
    computed: {
        isPeople() {
            return this.mode === 'people';
        },
        // Зеркалит BE-гейт эндпоинта привязки (requireAdmin = page.admin): super
        // проходит авто, admin/normal - по праву. Без него переключатель не показываем.
        canAttach() {
            return usePermissionsStore().hasPermission('page.admin');
        },
        isBinding() {
            return this.canAttach && this.bindMode === 'application';
        },
        modeTabs() {
            return [
                { key: 'none', label: 'Без заявки' },
                { key: 'application', label: 'Привязать к заявке' },
            ];
        },
        applicationOptions() {
            // application_number уже содержит «№» у реальных заявок (DEMO-номера - без) -
            // показываем как есть, как в Центре, иначе выходит двойной «№ №».
            return this.applications
                .filter(a => a.confirmation === APP_CONFIRMATION_APPROVED && a.status === APP_STATUS_IN_WORK)
                .map(a => ({
                    id: a.id,
                    label: `${a.application_number}${a.organization_name ? ' - ' + a.organization_name : ''}`,
                }));
        },
        // Перевесить сущности можно только на вложение того же типа (cars->cars,
        // people->people) - это же требует бэк (target.AttachmentType == orphan).
        attachmentOptions() {
            const type = this.isPeople ? 'people' : 'cars';
            return this.appAttachments
                .filter(a => a.attachment_type === type)
                .map(a => ({ id: a.id, label: this.attachmentLabel(a) }));
        },
        submitLabel() {
            if (this.submitting) return 'Сохранение...';
            return this.isBinding ? 'Добавить и привязать' : 'Добавить в таблицу';
        },
        selectedApplicationNumber() {
            const app = this.applications.find(a => a.id === this.selectedApplicationId);
            return app ? app.application_number : '';
        },
        title() {
            return this.isPeople ? 'Добавить сотрудников вручную' : 'Добавить машины вручную';
        },
        dateFieldConfig() {
            return this.isPeople ? PEOPLE_DATE_FIELD_CONFIG : {};
        },
        addedCount() {
            return this.isPeople ? this.addedEmployees.length : this.addedVehicles.length;
        },
        selectedOrgName() {
            const org = this.organizations.find(o => o.id === this.selectedOrgId);
            return org ? org.name : '';
        },
        selectedCompanyName() {
            const company = this.companies.find(c => c.id === this.selectedCompanyId);
            return company ? company.name : '';
        },
        datesComplete() {
            const d = this.dateData;
            return d.isOneDay
                ? !!(d.singleDate && d.startTime && d.endTime)
                : !!(d.startDate && d.endDate && d.startTime && d.endTime);
        },
        /**
         * Подсказка на заблокированной кнопке отправки: чего не хватает.
         * Пока идёт сохранение подсказки нет - кнопка выключена по другой
         * причине и объяснять нечего.
         */
        submitHint() {
            if (this.canSubmit || this.submitting) return '';

            const missing = [];
            if (this.isBinding && !this.selectedApplicationId) missing.push('заявку');
            if (this.isBinding && this.attachTarget === 'existing' && !this.selectedAttachmentId) {
                missing.push('вложение заявки');
            }
            if (!this.selectedOrgId) missing.push('организацию');
            if (!this.datesComplete) missing.push('даты и время');

            const reasons = [];
            if (missing.length) reasons.push(`Заполните: ${missing.join(', ')}`);
            if (this.addedCount === 0) {
                reasons.push(this.isPeople
                    ? 'Добавьте хотя бы одного сотрудника'
                    : 'Добавьте хотя бы одну машину');
            }
            return reasons.join('. ');
        },
        canSubmit() {
            if (!this.selectedOrgId || this.addedCount === 0 || !this.datesComplete || this.submitting) {
                return false;
            }
            if (this.isBinding) {
                if (!this.selectedApplicationId) return false;
                if (this.attachTarget === 'existing' && !this.selectedAttachmentId) return false;
            }
            return true;
        },
    },
    watch: {
        // Оверлей/крестик/Escape/свайп и блокировку скролла фона теперь несёт
        // BaseModal (контракт окон - см. эталон §3.1) - здесь остаётся только
        // сброс и загрузка формы при открытии.
        show(open) {
            if (open) {
                this.resetState();
                this.loadDictionaries();
            }
        },
        bindMode(mode) {
            if (mode === 'application' && !this.applications.length) {
                this.loadApplications();
            }
        },
    },
    mounted() {
        if (this.show) {
            this.loadDictionaries();
        }
    },
    methods: {
        async loadDictionaries() {
            try {
                const [orgs, companies] = await Promise.all([getOrganizations(), getCompanies()]);
                this.organizations = Array.isArray(orgs) ? orgs : [];
                this.companies = Array.isArray(companies) ? companies : [];
            } catch {
                useDeletionsStore().notify({ bold: 'Не удалось загрузить справочники', type: 'error' });
            }
        },
        async loadApplications() {
            this.loadingApplications = true;
            try {
                // Эндпоинт привязки (super/admin, гейт page.admin) отдаёт ВСЕ активные
                // согласованные заявки без скоупа по автор/ответственный - иначе админ,
                // не участвующий в заявке, не увидел бы её. Сервер сам форсит
                // confirmation/status; applicationOptions дублирует фильтр как страховку.
                const list = await getAttachableApplications();
                this.applications = Array.isArray(list) ? list : [];
            } catch {
                useDeletionsStore().notify({ bold: 'Не удалось загрузить заявки', type: 'error' });
            } finally {
                this.loadingApplications = false;
            }
        },
        onApplicationChange(id) {
            this.selectedApplicationId = id;
            this.attachTarget = 'new';
            this.selectedAttachmentId = null;
            this.appAttachments = [];
            if (id) this.loadApplicationAttachments(id);
        },
        async loadApplicationAttachments(applicationId) {
            // Быстрое переключение заявки в дропдропе пускает несколько загрузок; seq-токен
            // гарантирует, что appAttachments запишет только ответ последнего выбора
            // (иначе устаревший ответ по прошлой заявке затрёт актуальные вложения).
            const seq = ++this.attachmentsFetchSeq;
            this.loadingAttachments = true;
            try {
                const list = await getApplicationAttachments(applicationId);
                if (seq !== this.attachmentsFetchSeq) return;
                this.appAttachments = Array.isArray(list) ? list : [];
            } catch {
                if (seq !== this.attachmentsFetchSeq) return;
                this.appAttachments = [];
                useDeletionsStore().notify({ bold: 'Не удалось загрузить вложения заявки', type: 'error' });
            } finally {
                if (seq === this.attachmentsFetchSeq) this.loadingAttachments = false;
            }
        },
        attachmentLabel(att) {
            const name = att.attachment_display_name || att.attachment_name || `Вложение №${att.id}`;
            const from = att.entry_date_from;
            const to = att.entry_date_to;
            if (from && to) return `${name} (${from} - ${to})`;
            return name;
        },
        onOrgChange(id) {
            this.selectedOrgId = id;
        },
        onCompanyChange(id) {
            this.selectedCompanyId = id;
        },
        // --- cars ---
        handleVehicleAdded(vehicle) {
            this.addedVehicles.push({ ...vehicle, id: this.vehicleIdCounter++, isExisting: false });
        },
        handleVehiclesAdded(vehicles) {
            vehicles.forEach(vehicle => {
                this.addedVehicles.push({ ...vehicle, id: this.vehicleIdCounter++, isExisting: false });
            });
        },
        handleVehicleUpdated(updated) {
            const index = this.addedVehicles.findIndex(v => v.id === updated.id);
            if (index !== -1) this.addedVehicles.splice(index, 1, updated);
        },
        editVehicle(vehicle) {
            this.$refs.vehicleForm?.editVehicle(vehicle);
        },
        removeVehicle(vehicle) {
            this.addedVehicles = this.addedVehicles.filter(v => v.id !== vehicle.id);
        },
        // --- people ---
        handleEmployeeAdded(employee) {
            this.addedEmployees.push({ ...employee, id: this.employeeIdCounter++, isExisting: false });
        },
        handleEmployeesAdded(employees) {
            employees.forEach(employee => {
                this.addedEmployees.push({ ...employee, id: this.employeeIdCounter++, isExisting: false });
            });
        },
        handleEmployeeUpdated(updated) {
            const index = this.addedEmployees.findIndex(e => e.id === updated.id);
            if (index !== -1) this.addedEmployees.splice(index, 1, updated);
        },
        editEmployee(employee) {
            this.$refs.employeeForm?.editEmployee(employee);
        },
        removeEmployee(employee) {
            this.addedEmployees = this.addedEmployees.filter(e => e.id !== employee.id);
        },
        employeeFullName(employee) {
            return [employee.lastName, employee.firstName, employee.middleName]
                .filter(Boolean)
                .join(' ')
                .trim() || 'ФИО не указано';
        },
        // --- shared ---
        formatDateForAPI(dateStr) {
            if (!dateStr) return null;
            const [day, month, year] = dateStr.split('.');
            return `${year}-${month}-${day}`;
        },
        buildDateFields() {
            const d = this.dateData;
            const dateFrom = d.isOneDay ? d.singleDate : d.startDate;
            const dateTo = d.isOneDay ? d.singleDate : d.endDate;
            return {
                entry_date_from: this.formatDateForAPI(dateFrom),
                entry_date_to: this.formatDateForAPI(dateTo),
                entry_time_from: d.startTime ? `${d.startTime}:00` : null,
                entry_time_to: d.endTime ? `${d.endTime}:00` : null,
            };
        },
        buildCarPayload() {
            const d = this.dateData;
            return {
                organization_id: this.selectedOrgId,
                company_id: this.selectedCompanyId || null,
                table_id: this.tableId,
                ...this.buildDateFields(),
                roof_access: !!d.roofAccess,
                free_parking: !!d.freeParking,
                vehicles: this.addedVehicles.map(v => ({
                    car_number: v.plateNumber,
                    car_brand: v.mark,
                    mark_id: v.markId || null,
                    mark_name: v.markName || v.mark || null,
                    unload_place: v.unloadingPlace,
                    unload_places: v.unloadPlaces || [],
                    target_tables: v.passage_tables || [],
                })),
            };
        },
        buildEmployeePayload() {
            return {
                organization_id: this.selectedOrgId,
                company_id: this.selectedCompanyId || null,
                table_id: this.tableId,
                ...this.buildDateFields(),
                employees: this.addedEmployees.map(e => ({
                    last_name: e.lastName,
                    first_name: e.firstName,
                    middle_name: e.middleName || null,
                    citizenship_id: e.citizenshipId || 0,
                    position: e.position || '',
                    passport_series_number: e.passportSeriesNumber || '',
                    patent_number: e.patentNumber || null,
                    other_permission: e.otherPermission || null,
                    target_tables: e.targetTables || [],
                })),
            };
        },
        async submit() {
            if (!this.selectedOrgId) {
                useDeletionsStore().notify({ bold: 'Выберите организацию', type: 'error' });
                return;
            }
            if (this.addedCount === 0) {
                useDeletionsStore().notify({
                    bold: this.isPeople ? 'Добавьте хотя бы одного сотрудника' : 'Добавьте хотя бы одну машину',
                    type: 'error',
                });
                return;
            }
            if (!this.datesComplete) {
                useDeletionsStore().notify({ bold: 'Заполните даты и время действия', type: 'error' });
                return;
            }
            if (this.isBinding) {
                if (!this.selectedApplicationId) {
                    useDeletionsStore().notify({ bold: 'Выберите заявку для привязки', type: 'error' });
                    return;
                }
                if (this.attachTarget === 'existing' && !this.selectedAttachmentId) {
                    useDeletionsStore().notify({ bold: 'Выберите вложение заявки', type: 'error' });
                    return;
                }
            }
            this.submitting = true;

            let resp;
            try {
                resp = this.isPeople
                    ? await createManualEmployees(this.buildEmployeePayload())
                    : await createManualCars(this.buildCarPayload());
            } catch (e) {
                useDeletionsStore().notify({
                    bold: e.message || (this.isPeople ? 'Не удалось добавить сотрудников' : 'Не удалось добавить машины'),
                    type: 'error',
                });
                this.submitting = false;
                return;
            }

            const count = this.isPeople
                ? (resp.employee_ids?.length || this.addedEmployees.length)
                : (resp.car_ids?.length || this.addedVehicles.length);
            const noun = this.isPeople ? this.pluralEmployees(count) : this.pluralCars(count);

            if (this.isBinding) {
                // Записи уже созданы ручными и лежат в таблице. Привязка - отдельный шаг;
                // её провал (даты машины вне окна вложения, гонка) НЕ теряет созданные
                // записи - честно сообщаем "добавлено, но не привязано", запись остаётся
                // ручной и видна в таблице.
                const target = this.attachTarget === 'existing'
                    ? { targetAttachmentId: this.selectedAttachmentId }
                    : { applicationId: this.selectedApplicationId };
                try {
                    await attachToApplication(resp.attachment_id, target);
                    useDeletionsStore().notify({
                        prefix: `Добавлено ${count} ${noun}, привязано к заявке `,
                        bold: this.selectedApplicationNumber,
                        type: 'success',
                    });
                } catch (e) {
                    useDeletionsStore().notify({
                        prefix: 'Добавлено вручную, но не привязано: ',
                        bold: e.message || 'ошибка привязки',
                        type: 'error',
                    });
                }
            } else {
                useDeletionsStore().notify({
                    prefix: 'Добавлено вручную: ',
                    bold: `${count} ${noun}`,
                    type: 'success',
                });
            }

            this.$emit('added', resp);
            this.$emit('close');
            this.submitting = false;
        },
        pluralCars(n) {
            const mod10 = n % 10;
            const mod100 = n % 100;
            if (mod10 === 1 && mod100 !== 11) return 'машина';
            if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return 'машины';
            return 'машин';
        },
        pluralEmployees(n) {
            const mod10 = n % 10;
            const mod100 = n % 100;
            if (mod10 === 1 && mod100 !== 11) return 'сотрудник';
            if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return 'сотрудника';
            return 'сотрудников';
        },
        resetState() {
            this.selectedOrgId = null;
            this.selectedCompanyId = null;
            this.dateData = defaultDateData();
            this.addedVehicles = [];
            this.vehicleIdCounter = 1;
            this.addedEmployees = [];
            this.employeeIdCounter = 1;
            this.submitting = false;
            this.bindMode = 'none';
            this.applications = [];
            this.selectedApplicationId = null;
            this.appAttachments = [];
            this.attachTarget = 'new';
            this.selectedAttachmentId = null;
            this.loadingApplications = false;
            this.loadingAttachments = false;
        },
        // Сброс формы - в watch(show) на открытие (resetState выше), не здесь: BaseModal
        // сам управляет анимацией закрытия и after-leave родителю не отдаёт, а очистка
        // ДО угасания оверлея была бы видна пользователю кадром пустой формы.
        close() {
            this.$emit('close');
        },
    },
};
</script>

<style scoped>
/* Оверлей/окно/анимация/крестик/Escape/свайп теперь несёт BaseModal (эталон §3.1).
   `.manual-modal` остаётся контентной обёрткой внутри слота BaseModal - боковой
   отступ повторяет прежний `.manual-modal__body`, а класс держим ради `:deep()`
   ниже: он сам стилизуется напрямую (тот же scope, что и у ManualAddModal), а вот
   классы VehicleForm/EmployeeForm (`.data__completion` и другие) - это разметка
   ДОЧЕРНИХ компонентов, до неё достаёт только `:deep()` от предка внутри ТОГО ЖЕ
   дерева scope - в отличие от классов самого BaseModal, `:deep()` в которые из
   родителя мёртв (эталон §3.1, радиус модалки поэтому задаётся пропом). */
.manual-modal {
    padding: 20px 28px;
}

/* VehicleForm/EmployeeForm в CreateApplication - левая колонка двухколоночной
   раскладки (width:450px + border-right под список справа). В модалке список
   свой, поэтому растягиваем на всю ширину и оформляем блок формы отдельной
   карточкой с рамкой (границы группируют «Добавление Т/С»). */
.manual-modal :deep(.data__completion) {
    width: 100%;
    box-sizing: border-box;
    border-right: none;
    border: 1px solid var(--border);
    border-radius: var(--radius-md, 15px);
    padding: 16px 18px;
}

/* Гриды чипов заточены под 425px-колонку заявки - снимаем кап, чтобы места
   разгрузки/проезда занимали всю ширину формы (шире чипы, длинные имена влезают). */
.manual-modal :deep(.unloading__grid),
.manual-modal :deep(.passage__grid) {
    max-width: 100%;
}

/* Номер/Марка: подпись слева, чекбокс «по факту» справа - по РАЗНЫМ краям инпута,
   но в пределах его ширины (не за границу). Инпут номера фиксирован 202px, а колонка
   тянулась на всю ширину (flex:1) -> «по факту» уезжал за правый край инпута. Сужаем
   колонку номера до ширины инпута, тогда шапка (space-between) прижимает чекбокс ровно
   к правому краю поля. Колонка марки тянется на остаток, её инпут = ширина колонки.
   Только для ДЕСКТОПНОЙ строки: на телефоне VehicleForm сам переводит
   `.completion__fields` в колонку (`flex-direction: column`, брейкпоинт 768px), и
   `flex-basis` из этих правил там читается уже как ВЫСОТА, а не ширина - номер
   растягивался в блок 202px, марка в 320px, и между полями возникал пустой провал. */
@media (min-width: 769px) {
    .manual-modal :deep(.completion__fields) {
        gap: 12px;
    }

    .manual-modal :deep(.completion__number) {
        flex: 0 0 202px;
        max-width: 202px;
    }

    /* Марка тянулась на весь остаток формы (слишком длинный инпут под дропдаун).
       Ограничиваем разумной шириной; ряд Номер+Марка выравнивается слева, «по факту»
       держится у правого края марки. */
    .manual-modal :deep(.completion__mark) {
        flex: 0 1 320px;
        max-width: 320px;
    }
}

/* Меню марки в заявке открывается СПРАВА от поля (left:100%) - там узкая колонка
   формы и справа есть место. В модалке поле марки широкое, его правый край у края
   модалки -> меню уезжало за модалку и обрезалось. Открываем меню ВНИЗ под полем,
   по ширине поля, внутри модалки. */
.manual-modal :deep(.mark__dropdown-menu) {
    top: calc(100% + 6px);
    left: 0;
    right: auto;
    margin-left: 0;
    width: 100%;
}

.manual-modal__hint {
    margin: 0 0 18px;
    font-size: 13px;
    color: var(--text-muted);
}

.manual-modal__mode {
    margin-bottom: 16px;
}

.manual-modal__bind {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 16px;
    margin-bottom: 18px;
    background: var(--accent-tint);
    border-radius: var(--radius-md, 15px);
}

.manual-modal__note {
    margin: 6px 0 0;
    font-size: 12px;
    color: var(--danger-text);
}

.manual-modal__target {
    display: flex;
    flex-wrap: wrap;
    gap: 18px;
}

.manual-modal__radio {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    color: var(--text);
    cursor: pointer;
}

.manual-modal__radio input[disabled] {
    cursor: not-allowed;
}

.manual-modal__radio input[disabled] + span {
    color: var(--text-muted);
}

.manual-modal__grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
    margin-bottom: 28px;
}

.manual-modal__label {
    display: block;
    margin-bottom: 6px;
    font-size: 13px;
    font-weight: 500;
    color: var(--text);
}

.manual-modal__req {
    color: var(--color-primary, var(--accent-text));
}

.manual-modal__dates {
    margin-bottom: 28px;
}

/* До выбора организации VehicleForm/EmployeeForm получают disabled -> нативный inert
   (поля не реагируют на клики). Без визуального признака это читается как «сломанный
   UI». Затемняем форму и показываем подсказку-пилюлю, что нужно сперва выбрать орг. */
.manual-modal__form-section {
    position: relative;
}

.manual-modal__form-section--locked :deep(.data__completion) {
    opacity: 0.45;
}

.manual-modal__form-lock {
    position: absolute;
    top: 12px;
    left: 50%;
    transform: translateX(-50%);
    z-index: 2;
    pointer-events: none;
    background: var(--color-primary, var(--accent));
    color: var(--accent-contrast);
    padding: 9px 18px;
    border-radius: 20px;
    /* 13px терялся на телефоне - подняли до кегля соседних заголовков-предупреждений
       формы (.manual-modal__added-title здесь же и .warning-title в VehicleForm). */
    font-size: 14px;
    font-weight: 500;
    text-align: center;
    /* Пилюля раньше сжималась по контенту (max-width как потолок) - текст жался
       узкой колонкой. Владелец просил ширину, не кегль: теперь пилюля занимает
       доступную ширину формы. */
    width: calc(100% - 32px);
    box-sizing: border-box;
    box-shadow: 0 2px 10px var(--shadow-drop);
}

.manual-modal__added {
    margin-top: 18px;
    padding-top: 16px;
    border-top: 1px solid var(--border);
}

.manual-modal__added-title {
    font-size: 14px;
    font-weight: 600;
    margin-bottom: 10px;
    color: var(--text);
}

.manual-added-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.manual-added-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 10px 14px;
    background: var(--accent-tint);
    border-radius: var(--radius-md, 15px);
}

.manual-added-name {
    font-size: 14px;
    color: var(--text);
}

.manual-added-actions {
    display: flex;
    gap: 8px;
    flex-shrink: 0;
}

.manual-added-btn {
    border: none;
    background: transparent;
    cursor: pointer;
    font-size: 13px;
    color: var(--color-primary, var(--accent-text));
}

.manual-added-btn--danger {
    color: var(--danger-text);
}

.manual-added-btn:hover {
    text-decoration: underline;
}

.manual-btn {
    border-radius: 20px;
    padding: 10px 24px;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    border: 1px solid transparent;
    transition: background 0.15s ease, color 0.15s ease;
}

.manual-btn--ghost {
    background: var(--surface);
    border-color: var(--border);
    color: var(--text);
}

.manual-btn--ghost:hover {
    background: var(--surface-2);
}

.manual-btn--primary {
    background: var(--color-primary, var(--accent));
    color: var(--accent-contrast);
}

.manual-btn--primary:hover:not(:disabled) {
    background: var(--color-primary-hover, var(--accent-hover));
}

.manual-btn--primary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

@media (max-width: 640px) {
    .manual-modal__grid {
        grid-template-columns: 1fr;
    }
}
</style>
