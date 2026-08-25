import { describe, it, expect, vi, afterEach } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import VehiclesList from '../VehiclesList.vue';
import EmployeesList from '../EmployeesList.vue';
import Badge from '@/components/ui/Badge.vue';

// Срез U4: вход в массовый ввод живёт в шапке списка (кнопка + предупреждающий бейдж
// рядом), а не парой кнопок над формой. Право считает родитель и отдаёт пропом.

const VEHICLE = { id: 1, plateNumber: 'А001АА777', mark: 'Volvo' };
const EMPLOYEE = { id: 1, lastName: 'Иванов', firstName: 'Иван', middleName: 'Иванович' };

function mountVehicles(props = {}) {
  return shallowMount(VehiclesList, {
    props: { vehicles: [VEHICLE], ...props },
  });
}

function mountEmployees(props = {}) {
  return shallowMount(EmployeesList, {
    props: { employees: [EMPLOYEE], ...props },
  });
}

describe.each([
  ['список машин', mountVehicles, 'vehicles-import-btn'],
  ['список сотрудников', mountEmployees, 'employees-import-btn'],
])('%s - вход в импорт из шапки (U4)', (_name, mountList, testid) => {
  it('без права кнопки импорта в шапке нет', () => {
    const wrapper = mountList();
    expect(wrapper.find(`[data-testid="${testid}"]`).exists()).toBe(false);
  });

  it('с правом рядом с кнопкой стоит предупреждающий бейдж Experimental', () => {
    const wrapper = mountList({ canImport: true });

    const entry = wrapper.find('.import-entry');
    expect(entry.exists()).toBe(true);
    expect(entry.find(`[data-testid="${testid}"]`).exists()).toBe(true);

    // Бейдж - общий ui/Badge в предупреждающем варианте, и он идёт ПЕРЕД кнопкой.
    const badge = wrapper.findComponent(Badge);
    expect(badge.props()).toMatchObject({ label: 'Experimental', variant: 'warning' });

    const children = Array.from(entry.element.children);
    const badgeIndex = children.findIndex((el) => el.contains(badge.element));
    const buttonIndex = children.findIndex((el) => el.matches(`[data-testid="${testid}"]`));
    expect(badgeIndex).toBeGreaterThanOrEqual(0);
    expect(badgeIndex).toBeLessThan(buttonIndex);
  });

  it('клик просит родителя переключить режим', async () => {
    const wrapper = mountList({ canImport: true });

    await wrapper.find(`[data-testid="${testid}"]`).trigger('click');

    expect(wrapper.emitted('toggle-import')).toHaveLength(1);
  });

  it('в открытом режиме кнопка предлагает выход и отмечена нажатой', () => {
    const off = mountList({ canImport: true });
    expect(off.find(`[data-testid="${testid}"]`).text()).toBe('Импорт');
    expect(off.find(`[data-testid="${testid}"]`).attributes('aria-pressed')).toBe('false');

    const on = mountList({ canImport: true, importActive: true });
    expect(on.find(`[data-testid="${testid}"]`).text()).toBe('Закрыть импорт');
    expect(on.find(`[data-testid="${testid}"]`).attributes('aria-pressed')).toBe('true');
  });

  // На телефоне подпись выхода короче, иначе шапка блока не собирается в строку
  // (заголовок + счётчик + бейдж + «Закрыть импорт» = 338px при 314 на 360).
  it('на телефоне подпись выхода короткая, а доступное имя остаётся полным', async () => {
    window.matchMedia = vi.fn().mockImplementation((query) => ({
      matches: true,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));

    const on = mountList({ canImport: true, importActive: true });
    await on.vm.$nextTick();

    const button = on.find(`[data-testid="${testid}"]`);
    expect(button.text()).toBe('Закрыть');
    // WCAG 2.5.3: видимая подпись - префикс доступного имени, голосовой командой
    // «закрыть» кнопка по-прежнему находится.
    expect(button.attributes('aria-label')).toBe('Закрыть импорт');
  });

  afterEach(() => {
    delete window.matchMedia;
  });
});
