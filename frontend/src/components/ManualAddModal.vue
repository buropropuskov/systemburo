<template>
  <Teleport to="body">
    <transition name="manual-modal-fade">
      <div
        v-if="show"
        class="manual-modal-overlay"
        data-testid="manual-add-modal"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div
          class="manual-modal"
          @mousedown.stop
        >
          <div class="manual-modal__header">
            <h3>{{ title }}</h3>
            <button
              type="button"
              class="manual-modal__close"
              aria-label="Закрыть"
              @click="close"
            >
              &times;
            </button>
          </div>

          <div class="manual-modal__body">
            <p class="manual-modal__hint">
              Записи попадут в таблицу «{{ tableName }}» без заявки и будут помечены
              «Добавлено вручную».
            </p>

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

            <VehicleForm
              ref="vehicleForm"
              :user-organization="selectedOrgName"
              :user-organization-id="selectedOrgId"
              :user-company="selectedCompanyName"
              :user-company-id="selectedCompanyId"
              :existing-vehicles="addedVehicles"
              :disabled="!selectedOrgId"
              @vehicle-added="handleVehicleAdded"
              @vehicles-added="handleVehiclesAdded"
              @vehicle-updated="handleVehicleUpdated"
            />

            <div
              v-if="addedVehicles.length"
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
          </div>

          <div class="manual-modal__footer">
            <button
              type="button"
              class="manual-btn manual-btn--ghost"
              @click="close"
            >
              Отмена
            </button>
            <button
              type="button"
              class="manual-btn manual-btn--primary"
              :disabled="!canSubmit"
              data-testid="manual-add-submit"
              @click="submit"
            >
              {{ submitting ? 'Сохранение...' : 'Добавить в таблицу' }}
            </button>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import DateRangeSection from '@/components/CreateApplication/DateRangeSection.vue';
import VehicleForm from '@/components/CreateApplication/VehicleForm.vue';
import { getOrganizations, getCompanies } from '@/api/organizations';
import { createManualCars } from '@/api/cars';
import { useDeletionsStore } from '@/stores/deletions';
import { useOverlayClose } from '@/composables/useOverlayClose';

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

export default {
    name: 'ManualAddModal',
    components: { BaseDropdown, DateRangeSection, VehicleForm },
    props: {
        show: {
            type: Boolean,
            default: false,
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
    setup() {
        const closeRef = { fn: null };
        const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => closeRef.fn && closeRef.fn());
        return { onOverlayMousedown, onOverlayMouseup, closeRef };
    },
    data() {
        return {
            organizations: [],
            companies: [],
            selectedOrgId: null,
            selectedCompanyId: null,
            dateData: defaultDateData(),
            addedVehicles: [],
            vehicleIdCounter: 1,
            submitting: false,
        };
    },
    computed: {
        title() {
            return 'Добавить машины вручную';
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
        canSubmit() {
            return !!this.selectedOrgId && this.addedVehicles.length > 0 && this.datesComplete && !this.submitting;
        },
    },
    watch: {
        show(open) {
            document.body.style.overflow = open ? 'hidden' : '';
            if (open) {
                this.resetState();
                this.loadDictionaries();
            }
        },
    },
    mounted() {
        this.closeRef.fn = this.close;
        document.addEventListener('keydown', this.onKeydown);
        if (this.show) {
            document.body.style.overflow = 'hidden';
            this.loadDictionaries();
        }
    },
    beforeUnmount() {
        document.removeEventListener('keydown', this.onKeydown);
        document.body.style.overflow = '';
    },
    methods: {
        onKeydown(e) {
            if (e.key === 'Escape' && this.show) this.close();
        },
        async loadDictionaries() {
            try {
                const [orgs, companies] = await Promise.all([getOrganizations(), getCompanies()]);
                this.organizations = Array.isArray(orgs) ? orgs : [];
                this.companies = Array.isArray(companies) ? companies : [];
            } catch {
                useDeletionsStore().notify({ bold: 'Не удалось загрузить справочники', type: 'error' });
            }
        },
        onOrgChange(id) {
            this.selectedOrgId = id;
        },
        onCompanyChange(id) {
            this.selectedCompanyId = id;
        },
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
        formatDateForAPI(dateStr) {
            if (!dateStr) return null;
            const [day, month, year] = dateStr.split('.');
            return `${year}-${month}-${day}`;
        },
        buildPayload() {
            const d = this.dateData;
            const dateFrom = d.isOneDay ? d.singleDate : d.startDate;
            const dateTo = d.isOneDay ? d.singleDate : d.endDate;
            return {
                organization_id: this.selectedOrgId,
                company_id: this.selectedCompanyId || null,
                table_id: this.tableId,
                entry_date_from: this.formatDateForAPI(dateFrom),
                entry_date_to: this.formatDateForAPI(dateTo),
                entry_time_from: d.startTime ? `${d.startTime}:00` : null,
                entry_time_to: d.endTime ? `${d.endTime}:00` : null,
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
        async submit() {
            if (!this.selectedOrgId) {
                useDeletionsStore().notify({ bold: 'Выберите организацию', type: 'error' });
                return;
            }
            if (!this.addedVehicles.length) {
                useDeletionsStore().notify({ bold: 'Добавьте хотя бы одну машину', type: 'error' });
                return;
            }
            if (!this.datesComplete) {
                useDeletionsStore().notify({ bold: 'Заполните даты и время действия', type: 'error' });
                return;
            }
            this.submitting = true;
            try {
                const resp = await createManualCars(this.buildPayload());
                const count = resp.car_ids?.length || this.addedVehicles.length;
                useDeletionsStore().notify({
                    prefix: 'Добавлено вручную: ',
                    bold: `${count} ${this.pluralCars(count)}`,
                    type: 'success',
                });
                this.$emit('added', resp);
                this.resetState();
                this.$emit('close');
            } catch (e) {
                useDeletionsStore().notify({ bold: e.message || 'Не удалось добавить машины', type: 'error' });
            } finally {
                this.submitting = false;
            }
        },
        pluralCars(n) {
            const mod10 = n % 10;
            const mod100 = n % 100;
            if (mod10 === 1 && mod100 !== 11) return 'машина';
            if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return 'машины';
            return 'машин';
        },
        resetState() {
            this.selectedOrgId = null;
            this.selectedCompanyId = null;
            this.dateData = defaultDateData();
            this.addedVehicles = [];
            this.vehicleIdCounter = 1;
            this.submitting = false;
        },
        close() {
            this.resetState();
            this.$emit('close');
        },
    },
};
</script>

<style scoped>
.manual-modal-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 20px;
}

.manual-modal {
    background: #fff;
    border-radius: 30px;
    width: 760px;
    max-width: 95%;
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
}

.manual-modal__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 24px 28px 16px;
    border-bottom: 1px solid #eee;
}

.manual-modal__header h3 {
    margin: 0;
    font-size: 20px;
    font-weight: 700;
    color: #1a1a1a;
}

.manual-modal__close {
    border: none;
    background: transparent;
    font-size: 28px;
    line-height: 1;
    cursor: pointer;
    color: #888;
    padding: 0 4px;
}

.manual-modal__close:hover {
    color: #1a1a1a;
}

.manual-modal__body {
    padding: 20px 28px;
    overflow-y: auto;
    flex: 1;
}

.manual-modal__hint {
    margin: 0 0 18px;
    font-size: 13px;
    color: #666;
}

.manual-modal__grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
    margin-bottom: 18px;
}

.manual-modal__label {
    display: block;
    margin-bottom: 6px;
    font-size: 13px;
    font-weight: 500;
    color: #333;
}

.manual-modal__req {
    color: var(--color-primary, #4f5bdf);
}

.manual-modal__dates {
    margin-bottom: 8px;
}

.manual-modal__added {
    margin-top: 18px;
    padding-top: 16px;
    border-top: 1px solid #eee;
}

.manual-modal__added-title {
    font-size: 14px;
    font-weight: 600;
    margin-bottom: 10px;
    color: #1a1a1a;
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
    background: #f7f7f9;
    border-radius: var(--radius-md, 15px);
}

.manual-added-name {
    font-size: 14px;
    color: #1a1a1a;
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
    color: var(--color-primary, #4f5bdf);
}

.manual-added-btn--danger {
    color: #d14343;
}

.manual-added-btn:hover {
    text-decoration: underline;
}

.manual-modal__footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    padding: 16px 28px 24px;
    border-top: 1px solid #eee;
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
    background: #fff;
    border-color: #e0e0e0;
    color: #333;
}

.manual-btn--ghost:hover {
    background: #f2f2f2;
}

.manual-btn--primary {
    background: var(--color-primary, #4f5bdf);
    color: #fff;
}

.manual-btn--primary:hover:not(:disabled) {
    background: var(--color-primary-hover, #3d49c7);
}

.manual-btn--primary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.manual-modal-fade-enter-active,
.manual-modal-fade-leave-active {
    transition: opacity 0.2s ease;
}

.manual-modal-fade-enter-from,
.manual-modal-fade-leave-to {
    opacity: 0;
}

.manual-modal-fade-enter-active .manual-modal,
.manual-modal-fade-leave-active .manual-modal {
    transition: transform 0.2s ease-out;
}

.manual-modal-fade-enter-from .manual-modal,
.manual-modal-fade-leave-to .manual-modal {
    transform: translateY(20px);
}

@media (max-width: 640px) {
    .manual-modal__grid {
        grid-template-columns: 1fr;
    }
}
</style>
