import { describe, expect, it, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import CarsTable from '../CarsTable.vue';
import PeopleTable from '../PeopleTable.vue';
import CarsView from '../../views/CarsView.vue';
import EmployeeView from '../../views/EmployeeView.vue';
import ApplicationDetail from '../ApplicationDetail/ApplicationDetail.vue';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

const effective = {
  entry_date_from: '2035-01-01', entry_date_to: '2035-01-03',
  entry_time_from: '08:00:00', entry_time_to: '22:00:00', bounded: true, source: 'individual',
};
const result = { entity_id: 17, period_mode: 'individual', effective_period: effective };
const parents = [
  { name: 'cars table', component: CarsTable, key: 'selectedVehicle', show: 'showVehicleDetails', rows: 'itemsData', reload: 'loadData', open: 'openVehicleDetails', selected: { id: 17 }, fresh: { id: 17 } },
  { name: 'people table', component: PeopleTable, key: 'selectedEmployee', show: 'showDetailsModal', rows: 'itemsData', reload: 'loadData', open: 'openEmployeeDetails', selected: { id: 17 }, fresh: { id: 17 }, employee: true },
  { name: 'cars registry', component: CarsView, key: 'detailsCar', show: 'showDetailsViewModal', rows: 'carsData', reload: 'fetchCars', open: 'openCarDetails', selected: { id: 999, activeCarId: 17 }, fresh: { id: 999, active_car_id: 17 } },
  { name: 'people registry', component: EmployeeView, key: 'detailsEmployee', show: 'showDetailsModal', rows: 'employeesData', reload: 'fetchEmployees', open: 'openEmployeeDetails', selected: { id: 999, activeEmployeeId: 17 }, fresh: { id: 999, active_employee_id: 17 }, employee: true },
];

function context(parent, reload = null) {
  const ctx = {
    [parent.key]: { ...parent.selected }, [parent.show]: true,
    [parent.rows]: [{ ...parent.fresh }], [parent.open]: vi.fn(),
    tableId: 8, currentTableId: 8,
  };
  ctx[parent.reload] = reload || vi.fn(async () => { ctx[parent.rows] = [parent.fresh]; });
  return ctx;
}

describe('Entity period parent integration #2665', () => {
  for (const parent of parents.slice(0, 2)) {
    it(`${parent.name} closes stale manual card and refreshes after exact-row attachment`, async () => {
      const ctx = context(parent);
      const kind = parent.employee ? 'employee' : 'car';
      const methods = parent.component.mixins[0].methods;
      await methods.onManualEntityAttached.call(ctx, { entity_id: 999, entity_kind: kind });
      expect(ctx[parent.show]).toBe(true);
      expect(ctx[parent.reload]).not.toHaveBeenCalled();
      await methods.onManualEntityAttached.call(ctx, { entity_id: 17, entity_kind: kind === 'car' ? 'employee' : 'car' });
      expect(ctx[parent.reload]).not.toHaveBeenCalled();
      await methods.onManualEntityAttached.call(ctx, { entity_id: 17, entity_kind: kind, application_id: 5, destination_attachment_id: 23 });
      expect(ctx[parent.show]).toBe(false);
      expect(ctx[parent.reload]).toHaveBeenCalledTimes(1);
      expect(ctx[parent.open]).not.toHaveBeenCalled();
    });
  }
  for (const parent of parents) {
    it(`${parent.name} retains confirmed period and mode when a failed or superseded GET leaves the old row`, async () => {
      const ctx = context(parent, vi.fn().mockResolvedValue(undefined));
      ctx.carUnloadPlacesMap = {};
      const methods = parent.component.mixins[0].methods;
      const old = { ...parent.fresh, entry_date_to: '2035-01-02', active_entry_date_to: '2035-01-02' };
      ctx[parent.rows] = [old];
      ctx[parent.open] = methods[parent.open].bind(ctx);
      ctx[parent.open](old); // Exercise the actual mapper, not a spy.
      await methods.onEntityPeriodChanged.call(ctx, result);
      expect(ctx[parent.key].entry_date_to).toBe(effective.entry_date_to);
      expect(ctx[parent.key].entry_date_from).toBe(effective.entry_date_from);
      expect(ctx[parent.key].entry_time_from).toBe(effective.entry_time_from);
      expect(ctx[parent.key].entry_time_to).toBe(effective.entry_time_to);
      expect(ctx[parent.key].period_mode).toBe('individual');
      expect(ctx[parent.show]).toBe(true);
      expect(ctx[parent.rows][0]).toBe(old);
    });

    it(`${parent.name} respects an accepted newer GET period using the actual mapper`, async () => {
      const ctx = context(parent);
      ctx.carUnloadPlacesMap = {};
      const methods = parent.component.mixins[0].methods;
      const old = { ...parent.fresh, entry_date_to: '2035-01-02', active_entry_date_to: '2035-01-02' };
      const newer = { ...parent.fresh, entry_date_to: '2035-01-05', active_entry_date_to: '2035-01-05' };
      ctx[parent.rows] = [old];
      ctx[parent.open] = methods[parent.open].bind(ctx);
      ctx[parent.open](old);
      ctx[parent.reload] = vi.fn(async () => { ctx[parent.rows] = [newer]; });
      await methods.onEntityPeriodChanged.call(ctx, result);
      expect(ctx[parent.key].entry_date_to).toBe('2035-01-05');
      expect(ctx[parent.show]).toBe(true);
      expect(ctx[parent.rows][0]).toBe(newer);
    });

    it(`${parent.name} keeps the card open, applies saved period, then uses existing fresh-row mapping`, async () => {
      const ctx = context(parent);
      await parent.component.mixins[0].methods.onEntityPeriodChanged.call(ctx, result);
      expect(ctx[parent.show]).toBe(true);
      expect(ctx[parent.key].entry_date_to).toBe(effective.entry_date_to);
      if (parent.employee) expect(ctx[parent.key].pass_time).toBe('08:00 - 22:00');
      expect(ctx[parent.reload]).toHaveBeenCalledOnce();
      expect(ctx[parent.open]).toHaveBeenCalledWith(parent.fresh);
    });

    it(`${parent.name} never reopens a card closed during the reload`, async () => {
      let finish;
      const ctx = context(parent, vi.fn(() => new Promise(resolve => { finish = resolve; })));
      const pending = parent.component.mixins[0].methods.onEntityPeriodChanged.call(ctx, result);
      expect(ctx[parent.key].entry_date_to).toBe(effective.entry_date_to);
      ctx[parent.show] = false;
      ctx[parent.key] = null;
      finish();
      await pending;
      expect(ctx[parent.open]).not.toHaveBeenCalled();
    });

    it(`${parent.name} ignores another entity save instead of treating registry ID as row ID`, async () => {
      const ctx = context(parent);
      await parent.component.mixins[0].methods.onEntityPeriodChanged.call(ctx, { ...result, entity_id: 999 });
      expect(ctx[parent.reload]).not.toHaveBeenCalled();
      expect(ctx[parent.open]).not.toHaveBeenCalled();
    });

    it(`${parent.name} keeps confirmed values when the reloaded filtered list no longer includes the row`, async () => {
      const ctx = context(parent, vi.fn().mockResolvedValue(undefined));
      ctx[parent.rows] = [];
      await parent.component.mixins[0].methods.onEntityPeriodChanged.call(ctx, result);
      expect(ctx[parent.key].entry_date_to).toBe(effective.entry_date_to);
      expect(ctx[parent.show]).toBe(true);
      expect(ctx[parent.open]).not.toHaveBeenCalled();
    });
  }

  it.each(['vehicle', 'employee'])('application %s retains PUT on swallowed failure but honors a newly accepted GET', async kind => {
    const vehicle = kind === 'vehicle';
    const key = vehicle ? 'selectedVehicle' : 'selectedEmployee';
    const rowsKey = vehicle ? 'attachmentCars' : 'attachmentEmployees';
    const open = vehicle ? 'openVehicleModal' : 'openEmployeeModal';
    const methods = ApplicationDetail.mixins[0].methods;
    const old = { id: 17, entry_date_to: '2035-01-02' };
    const ctx = {
      applicationData: { id: 5 }, attachmentCars: [], attachmentEmployees: [],
      $refs: { historyComponent: { loadHistory: vi.fn() } }, $emit: vi.fn(),
      loadApplicationDetails: vi.fn().mockResolvedValue(undefined),
    };
    ctx[rowsKey] = [old];
    ctx[open] = methods[open].bind(ctx);
    ctx[open](old);
    await methods.onEntityPeriodChanged.call(ctx, kind, result);
    expect(ctx[key].entry_date_to).toBe(effective.entry_date_to);
    expect(ctx[key].period_mode).toBe('individual');
    expect(ctx[key].entry_time_to).toBe(effective.entry_time_to);
    const newer = { ...old, entry_date_to: '2035-01-05' };
    ctx.loadApplicationDetails = vi.fn(async () => { ctx[rowsKey] = [newer]; });
    await methods.onEntityPeriodChanged.call(ctx, kind, result);
    expect(ctx[key].entry_date_to).toBe('2035-01-05');
  });

  it.each(['vehicle', 'employee'])('application %s refreshes selected attachment and history without closing its card', async kind => {
    const vehicle = kind === 'vehicle';
    const key = vehicle ? 'selectedVehicle' : 'selectedEmployee';
    const fresh = { id: 17, entry_date_to: '2035-01-04' };
    const ctx = {
      [key]: { id: 17 }, showVehicleModal: vehicle, showEmployeeModal: !vehicle,
      applicationData: { id: 5 }, attachmentCars: vehicle ? [{ ...fresh }] : [], attachmentEmployees: vehicle ? [] : [{ ...fresh }],
      loadApplicationDetails: vi.fn(async () => {
        if (vehicle) ctx.attachmentCars = [fresh];
        else ctx.attachmentEmployees = [fresh];
      }),
      $refs: { historyComponent: { loadHistory: vi.fn() } }, $emit: vi.fn(),
      openVehicleModal: vi.fn(), openEmployeeModal: vi.fn(),
    };
    await ApplicationDetail.mixins[0].methods.onEntityPeriodChanged.call(ctx, kind, result);
    expect(ctx.loadApplicationDetails).toHaveBeenCalledWith(ctx.applicationData, { preserveSelection: true });
    expect(ctx.$refs.historyComponent.loadHistory).toHaveBeenCalledOnce();
    expect(ctx.$emit).toHaveBeenCalledWith('application-changed', ctx.applicationData);
    expect(ctx[vehicle ? 'openVehicleModal' : 'openEmployeeModal']).toHaveBeenCalledWith(fresh);
    expect(ctx[vehicle ? 'showVehicleModal' : 'showEmployeeModal']).toBe(true);
  });

  it('does not publish application refresh for another application opened while saving', async () => {
    const ctx = {
      selectedVehicle: { id: 17 }, applicationData: { id: 5 },
      attachmentCars: [], attachmentEmployees: [],
      loadApplicationDetails: vi.fn(async () => { ctx.applicationData = { id: 6 }; }),
      $refs: { historyComponent: { loadHistory: vi.fn() } }, $emit: vi.fn(),
    };
    await ApplicationDetail.mixins[0].methods.onEntityPeriodChanged.call(ctx, 'vehicle', result);
    expect(ctx.$emit).not.toHaveBeenCalled();
    expect(ctx.$refs.historyComponent.loadHistory).not.toHaveBeenCalled();
  });

  it('table callers pass their actual server table context and all parents subscribe to the event', () => {
    const read = path => readFileSync(new URL(path, import.meta.url), 'utf8');
    expect(CarsTable.mixins[0].computed.entityDetailsProps.call({ tableId: 8 }).periodTableId).toBe(8);
    expect(PeopleTable.mixins[0].computed.entityDetailsProps.call({ currentTableId: 9 }).periodTableId).toBe(9);
    for (const path of ['../CarsTable.vue', '../PeopleTable.vue', '../../views/CarsView.vue', '../../views/EmployeeView.vue']) {
      expect(read(path)).toContain('@period-changed="onEntityPeriodChanged"');
    }
    expect(read('../ApplicationDetail/ApplicationDetail.vue')).toContain('@period-changed="onEntityPeriodChanged(\'vehicle\', $event)"');
    expect(read('../ApplicationDetail/ApplicationDetail.vue')).toContain('@period-changed="onEntityPeriodChanged(\'employee\', $event)"');
  });

  it('table mappers preserve existing aliases and source-specific fields', () => {
    const car = { id: 17, car_number: 'TEST', car_brand: 'Brand', unload_place_ids: [4], applicationId: 5, entry_checked: true, exit_checked: false };
    const employee = { id: 18, last_name: 'Test', citizenshipName: 'Test country', passport_series_number: 'TEST-DOCUMENT', target_tables: [{ id: 8 }], territory_status: 1, applicationId: 5 };
    const cars = {};
    const people = {};
    CarsTable.mixins[0].methods.openVehicleDetails.call(cars, car);
    PeopleTable.mixins[0].methods.openEmployeeDetails.call(people, employee);
    expect(cars.selectedVehicle).toMatchObject({ ...car, plateNumber: 'TEST', mark: 'Brand', unloadPlaces: [4], isExisting: true });
    expect(people.selectedEmployee).toMatchObject(employee);
    expect(cars.showVehicleDetails).toBe(true);
    expect(people.showDetailsModal).toBe(true);
  });

  it('registry mappers retain distinct registry/active IDs and active application labels', () => {
    const ctx = { carUnloadPlacesMap: { 17: [4] } };
    CarsView.mixins[0].methods.openCarDetails.call(ctx, {
      id: 999, number: 'TEST', mark: 'Brand', active_car_id: 17, active_application_id: 5,
      active_app_org_name: 'Active organization', organization_name: 'Registry organization',
      active_entry_date_to: effective.entry_date_to, active_entry_time_from: effective.entry_time_from, active_entry_time_to: effective.entry_time_to,
    });
    expect(ctx.detailsCar).toMatchObject({ id: 999, activeCarId: 17, applicationId: 5, organization: 'Active organization', unloadPlaces: [4], entry_date_to: effective.entry_date_to });
    EmployeeView.mixins[0].methods.openEmployeeDetails.call(ctx, {
      id: 888, active_employee_id: 18, active_application_id: 6, last_name: 'Test',
      active_app_company_name: 'Active company', company_name: 'Registry company',
      active_entry_date_to: effective.entry_date_to, active_pass_time: '08:00:00 - 22:00:00',
    });
    expect(ctx.detailsEmployee).toMatchObject({ id: 888, activeEmployeeId: 18, applicationId: 6, company: 'Active company', pass_time: '08:00:00 - 22:00:00' });
  });

  it('application mappers retain row IDs, unload-place objects and parent application context', () => {
    const ctx = { applicationData: { id: 5 } };
    const methods = ApplicationDetail.mixins[0].methods;
    methods.openVehicleModal.call(ctx, { id: 17, car_number: 'TEST', unload_places: [{ id: 4 }], target_tables: [{ id: 8, source: 'manual' }], blacklist_similar: { active: true } });
    methods.openEmployeeModal.call(ctx, { id: 18, last_name: 'Test', citizenship_name: 'Test country', pass_time: '08:00:00 - 22:00:00' });
    expect(ctx.selectedVehicle).toMatchObject({ id: 17, applicationId: 5, unloadPlaces: [4], target_tables: [{ id: 8, source: 'manual' }], blacklist_similar: { active: true } });
    expect(ctx.selectedEmployee).toMatchObject({ id: 18, applicationId: 5, citizenshipName: 'Test country', pass_time: '08:00:00 - 22:00:00' });
  });

  it('card prop groups preserve source, permission and shared-data contracts', () => {
    const ctx = { showVehicleModal: true, showEmployeeModal: true, selectedVehicle: { id: 17 }, selectedEmployee: { id: 18 },
      allTables: [{ id: 8 }], allUnloadingPlaces: [{ id: 4 }], licensePlateFormats: [],
      currentUserId: 2, currentUserName: 'Test actor', canOverrideBlacklist: true, canManageBlacklistOverride: false };
    const computed = ApplicationDetail.mixins[0].computed;
    expect(computed.vehicleDetailsProps.call(ctx)).toEqual({ show: true, vehicle: ctx.selectedVehicle,
      allTables: ctx.allTables, allUnloadingPlaces: ctx.allUnloadingPlaces, licensePlateFormats: [],
      currentUserId: 2, currentUserName: 'Test actor', showCarFeatures: true, source: 'application', canOverride: true, canCancelOverride: false });
    expect(computed.employeeDetailsProps.call(ctx)).toEqual({ show: true, employee: ctx.selectedEmployee,
      allTables: ctx.allTables, currentUserId: 2, currentUserName: 'Test actor', source: 'application', canOverride: true, canCancelOverride: false });
  });
});
