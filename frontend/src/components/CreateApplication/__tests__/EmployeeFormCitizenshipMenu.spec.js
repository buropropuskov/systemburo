import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// Меню гражданства - рукописный дропдаун с absolute-позиционированием. Регресс-замок
// по образцу BaseDropdownPosition.spec.js: rect отдаёт device-px под корневым zoom,
// innerHeight - незумленную высоту, поэтому к layout-px приводятся ОБА, иначе на
// мониторах шире 1440 свободное место снизу считается завышенным и меню уезжает
// под кромку вместо раскрытия вверх.

const zoom = { value: 1 };
vi.mock('@/utils/viewportScale', () => ({
  getViewportZoom: () => zoom.value,
}));
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/blacklist', () => ({
  checkPersonBlacklist: vi.fn().mockResolvedValue(null),
}));
vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn().mockReturnValue({ token: 'test-token' }),
}));
vi.mock('@/components/CreateApplication/ExistingEmployeesModal.vue', () => ({
  default: { name: 'ExistingEmployeesModal', template: '<div />' },
}));

import EmployeeForm from '../EmployeeForm.vue';

function mountForm(buttonRect) {
  const w = mount(EmployeeForm);
  w.vm.$refs.citizenshipButton.getBoundingClientRect = () => buttonRect;
  return w;
}

describe('EmployeeForm: положение меню гражданства', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    zoom.value = 1;
    Object.defineProperty(window, 'innerHeight', { value: 800, configurable: true });
  });

  it('места снизу хватает - меню вниз, высота ограничена свободным местом', () => {
    const w = mountForm({ top: 100, bottom: 140 });
    const style = w.vm.buildCitizenshipMenuStyle();
    expect(style.bottom).toBeUndefined();
    // 800 - 140 - 16 = 644, кап 300
    expect(style.maxHeight).toBe('300px');
  });

  it('снизу меньше 200px - меню раскрывается вверх и сбрасывает top', () => {
    const w = mountForm({ top: 700, bottom: 740 });
    const style = w.vm.buildCitizenshipMenuStyle();
    expect(style.bottom).toBe('100%');
    expect(style.top).toBe('auto');
    // сверху 700, кап 300
    expect(style.maxHeight).toBe('300px');
  });

  it('zoom=1.6: innerHeight делится на zoom так же, как rect - иначе флип не срабатывает', () => {
    zoom.value = 1.6;
    // В layout-px кнопка стоит на 700..740 при вьюпорте 800 - снизу 60px, нужен флип.
    const w = mountForm({ top: 700 * 1.6, bottom: 740 * 1.6 });
    Object.defineProperty(window, 'innerHeight', { value: 800 * 1.6, configurable: true });
    const style = w.vm.buildCitizenshipMenuStyle();
    expect(style.bottom).toBe('100%');
    expect(style.top).toBe('auto');
  });

  // Пересчёт висит всё время жизни формы, а не только пока меню открыто: стрелка на
  // закрытой кнопке показывает сторону раскрытия, а скролл эту сторону меняет.
  it('форма слушает скролл и ресайз, размонтирование снимает слушатели', async () => {
    const add = vi.spyOn(window, 'addEventListener');
    const remove = vi.spyOn(window, 'removeEventListener');
    const w = mountForm({ top: 100, bottom: 140 });
    await w.vm.$nextTick();

    expect(add).toHaveBeenCalledWith('scroll', w.vm.onDropViewportChange, true);
    expect(add).toHaveBeenCalledWith('resize', w.vm.onDropViewportChange);

    w.vm.isCitizenshipDropdownOpen = true;
    await w.vm.$nextTick();
    expect(w.vm.citizenshipMenuStyle).not.toBe(null);

    w.vm.isCitizenshipDropdownOpen = false;
    await w.vm.$nextTick();
    expect(w.vm.citizenshipMenuStyle).toBe(null);

    const handler = w.vm.onDropViewportChange;
    w.unmount();
    expect(remove).toHaveBeenCalledWith('scroll', handler, true);
    expect(remove).toHaveBeenCalledWith('resize', handler);

    add.mockRestore();
    remove.mockRestore();
  });
});
