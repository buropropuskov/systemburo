import { describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import CarAccessFlagRows from '../CreateApplication/CarAccessFlagRows.vue';
import CarsTable from '../CarsTable.vue';
import { carsTablePeriodDetails, carsRegistryPeriodDetails, applicationPeriodDetails } from '../entityPeriodParentMixins';
import { apiRequest } from '@/api/client';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
const flags = { roof_access: true, free_parking: true, individual_roof_access: true, individual_free_parking: false };

describe('car own and effective flags #2665', () => {
  it('renders compact read-only own and inherited effective values without controls', () => {
    const wrapper = mount(CarAccessFlagRows, { props: { flags } });
    expect(wrapper.find('[data-testid="car-access-roof_access"]').text()).toContain('Да — собственный признак');
    expect(wrapper.find('[data-testid="car-access-free_parking"]').text()).toContain('Да — из вложения');
    expect(wrapper.find('input, button, select').exists()).toBe(false);
    wrapper.unmount();
  });
  it('distinguishes an authoritative false value from missing metadata', async () => {
    const wrapper = mount(CarAccessFlagRows, { props: { flags: { ...flags, roof_access: false, individual_roof_access: false } } });
    expect(wrapper.find('[data-testid="car-access-roof_access"]').text()).toContain('Нет');
    await wrapper.setProps({ flags: {} });
    expect(wrapper.findAll('.detail-value').map(row => row.text())).toEqual(['Нет данных', 'Нет данных']);
    wrapper.unmount();
  });
  it('preserves four server flags through the actual active-table reader and open-card mapper', async () => {
    apiRequest.mockResolvedValueOnce({ ok: true, json: async () => [{ id: 17, status: 1, car_number: 'TEST', car_brand: 'Synthetic', ...flags }] });
    const ctx = { tableId: 4, itemsData: [], organizationsMap: {}, fetchOrganizations: vi.fn().mockResolvedValue(undefined) };
    await CarsTable.methods.fetchCarsData.call(ctx);
    expect(ctx.itemsData[0]).toMatchObject(flags);
    carsTablePeriodDetails.methods.openVehicleDetails.call(ctx, ctx.itemsData[0]);
    expect(ctx.selectedVehicle).toMatchObject(flags);
  });
  it('preserves flags from the same active registry row without using its registry ID as the car ID', () => {
    const ctx = { carUnloadPlacesMap: { 17: [3] } };
    carsRegistryPeriodDetails.methods.openCarDetails.call(ctx, { id: 999, active_car_id: 17, number: 'TEST', ...flags });
    expect(ctx.detailsCar).toMatchObject({ ...flags, id: 999, activeCarId: 17, unloadPlaces: [3] });
  });
  it('preserves attachment-car DTO flags without modifying parent attachment badges', () => {
    const ctx = { applicationData: { id: 7, roof_access: false, free_parking: false } };
    applicationPeriodDetails.methods.openVehicleModal.call(ctx, { id: 17, car_number: 'TEST', ...flags });
    expect(ctx.selectedVehicle).toMatchObject(flags);
    expect(ctx.applicationData).toEqual({ id: 7, roof_access: false, free_parking: false });
  });
});
