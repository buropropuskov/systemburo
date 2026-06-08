import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

// Все 4 модалки грузят историю через apiRequest(path).ok -> json().
// Контракт закрытия (visible/requestClose/onAfterLeave/Escape/overlay) от истории
// не зависит - мок отдаёт пустую историю, чтобы mounted не падал.
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })),
}));
// exceljs тяжёлый и для теста закрытия не нужен.
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import CarHistoryModal from '../CarHistoryModal.vue';
import CarsTableHistoryModal from '../CarsTableHistoryModal.vue';
import EmployeeHistoryModal from '../CreateApplication/EmployeeHistoryModal.vue';
import EmployeesTableHistoryModal from '../CreateApplication/EmployeesTableHistoryModal.vue';

// Срез D-2 (#406): открытие/закрытие 4 истории-модалок переведено на self-contained
// паттерн visible+<transition @after-leave>. Тест фиксирует контракт закрытия, единый
// для всех (эталон - CitizenshipHistoryModal из D-1).
const MODALS = [
  { name: 'CarHistoryModal', component: CarHistoryModal, props: { carId: 1, carNumber: 'A123AA' } },
  { name: 'CarsTableHistoryModal', component: CarsTableHistoryModal, props: { cars: [] } },
  { name: 'EmployeeHistoryModal', component: EmployeeHistoryModal, props: { lastName: 'Иванов', firstName: 'Иван' } },
  { name: 'EmployeesTableHistoryModal', component: EmployeesTableHistoryModal, props: { tableId: 1 } },
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

describe('История-модалки: анимация закрытия (D-паттерн)', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
  });

  MODALS.forEach(({ name, component, props }) => {
    describe(name, () => {
      it('visible=true по mounted, overlay отрендерен', async () => {
        const wrapper = await mountModal(component, props);
        expect(wrapper.vm.visible).toBe(true);
        expect(wrapper.find('.modal-overlay').exists()).toBe(true);
        wrapper.unmount();
      });

      it('requestClose прячет overlay (visible=false), но НЕ эмитит close сразу', async () => {
        const wrapper = await mountModal(component, props);
        wrapper.vm.requestClose();
        expect(wrapper.vm.visible).toBe(false);
        // close эмитится только ПОСЛЕ leave-перехода (@after-leave), не моментально.
        expect(wrapper.emitted('close')).toBeFalsy();
        wrapper.unmount();
      });

      it('close эмитится по завершении leave-перехода (onAfterLeave)', async () => {
        const wrapper = await mountModal(component, props);
        wrapper.vm.onAfterLeave();
        expect(wrapper.emitted('close')).toHaveLength(1);
        wrapper.unmount();
      });

      it('Escape запускает закрытие через requestClose (visible=false)', async () => {
        const wrapper = await mountModal(component, props);
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
        expect(wrapper.vm.visible).toBe(false);
        wrapper.unmount();
      });

      it('клик по overlay (mousedown+mouseup) закрывает через useOverlayClose', async () => {
        const wrapper = await mountModal(component, props);
        const overlay = wrapper.find('.modal-overlay');
        await overlay.trigger('mousedown');
        await overlay.trigger('mouseup');
        expect(wrapper.vm.visible).toBe(false);
        wrapper.unmount();
      });
    });
  });
});
