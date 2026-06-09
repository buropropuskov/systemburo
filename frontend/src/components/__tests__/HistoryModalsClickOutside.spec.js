import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

// Все модалки грузят историю по-разному: 6 через apiRequest(@/api/client),
// LPF через @/api/licenseFormats, Mark через @/api/marks. Контракт закрытия
// дропдаунов-фильтров от данных не зависит - моки отдают пустую историю.
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })),
}));
vi.mock('@/api/licenseFormats', () => ({
  getLicenseFormatHistory: vi.fn(() => Promise.resolve([])),
}));
vi.mock('@/api/marks', () => ({
  getMarkHistory: vi.fn(() => Promise.resolve([])),
}));
// exceljs тяжёлый и для теста закрытия дропдауна не нужен.
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import CarHistoryModal from '../CarHistoryModal.vue';
import CarsTableHistoryModal from '../CarsTableHistoryModal.vue';
import LicensePlateFormatHistoryModal from '../LicensePlateFormatHistoryModal.vue';
import MarkHistoryModal from '../MarkHistoryModal.vue';
import SystemTableHistoryModal from '../SystemTableHistoryModal.vue';
import UnloadPlaceHistoryModal from '../UnloadPlaces/UnloadPlaceHistoryModal.vue';
import EmployeeHistoryModal from '../CreateApplication/EmployeeHistoryModal.vue';
import EmployeesTableHistoryModal from '../CreateApplication/EmployeesTableHistoryModal.vue';

// Срез D-clickoutside (#406): handleClickOutside во всех history-модалках переведён
// с this.$el.querySelector('.custom-select') на this.$refs.<select>. При <Teleport>
// $el - якорный комментарий без querySelector, поэтому дропдаун фильтра не закрывался
// бы по клику снаружи (баг найден как Y-1 в #484 на CitizenshipHistoryModal).
const MODALS = [
  { name: 'CarHistoryModal', component: CarHistoryModal, props: { carId: 1, carNumber: 'A123AA' }, refs: ['userSelect'] },
  { name: 'CarsTableHistoryModal', component: CarsTableHistoryModal, props: { cars: [] }, refs: ['userSelect', 'carSelect'] },
  { name: 'LicensePlateFormatHistoryModal', component: LicensePlateFormatHistoryModal, props: { format: { id: 7, name: 'Российские номера' } }, refs: ['userSelect'] },
  { name: 'MarkHistoryModal', component: MarkHistoryModal, props: { mark: { id: 3, name: 'BMW' } }, refs: ['userSelect'] },
  { name: 'SystemTableHistoryModal', component: SystemTableHistoryModal, props: { table: { id: 1 } }, refs: ['userSelect'] },
  { name: 'UnloadPlaceHistoryModal', component: UnloadPlaceHistoryModal, props: { unloadPlace: { id: 1 } }, refs: ['userSelect'] },
  { name: 'EmployeeHistoryModal', component: EmployeeHistoryModal, props: { lastName: 'Иванов', firstName: 'Иван' }, refs: ['userSelect', 'placeSelect'] },
  { name: 'EmployeesTableHistoryModal', component: EmployeesTableHistoryModal, props: { tableId: 1 }, refs: ['userSelect', 'employeeSelect'] },
];

async function mountModal(component, props) {
  const wrapper = mount(component, {
    props,
    global: { stubs: { teleport: true } },
    attachTo: document.body,
  });
  await flushPromises();
  return wrapper;
}

describe('История-модалки: закрытие дропдаунов-фильтров по клику снаружи (refs при Teleport)', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
  });

  MODALS.forEach(({ name, component, props, refs }) => {
    describe(name, () => {
      it('фильтр-селекты подключены через ref (а не this.$el.querySelector)', async () => {
        const wrapper = await mountModal(component, props);
        // Регресс-защита на сам факт наличия ref-атрибута: с teleport-stub реальный
        // баг ($el = якорный комментарий) не воспроизводится, но если ref случайно
        // удалят при рефакторинге - $refs.<select> станет undefined и тест упадёт.
        refs.forEach((r) => expect(wrapper.vm.$refs[r]).toBeTruthy());
        wrapper.unmount();
      });

      it('дропдаун закрывается по клику снаружи', async () => {
        const wrapper = await mountModal(component, props);

        await wrapper.find('.custom-select').trigger('click');
        expect(wrapper.find('.select-dropdown').exists()).toBe(true);

        document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        await wrapper.vm.$nextTick();
        expect(wrapper.find('.select-dropdown').exists()).toBe(false);

        wrapper.unmount();
      });
    });
  });
});
