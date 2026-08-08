import { describe, it, expect, vi } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import VehiclesList from '../VehiclesList.vue';

// blank-import E1: список должен пережить импорт бланком (до 2000 машин) без рендера
// всего массива v-for'ом ни в десктопной колоночной раскладке, ни в мобильных карточках
// (обе ветки читают один и тот же pagedVehicles, но десктоп дублирует v-for на 4 колонки -
// самая тяжёлая по числу узлов раскладка).

// jsdom не реализует matchMedia - без мока useNarrowScreen выходит по гарду и isNarrow
// навсегда false (тот же паттерн, что в SchedulePlaceWarningPanel.spec.js).
function mockMatchMedia(matches) {
  window.matchMedia = vi.fn().mockImplementation((query) => ({
    matches,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

function makeVehicles(n) {
  return Array.from({ length: n }, (_, i) => ({
    id: i + 1,
    plateNumber: `А${String(i + 1).padStart(3, '0')}АА777`,
    mark: `Марка${i + 1}`,
  }));
}

describe('VehiclesList - десктоп (колоночная раскладка)', () => {
  it('2000 машин: в DOM попадает страница, а не весь массив', () => {
    mockMatchMedia(false);
    const vehicles = makeVehicles(2000);
    const w = shallowMount(VehiclesList, { props: { vehicles } });

    // vehicles-row стоит на колонке "№" - один элемент на строку.
    const rows = w.findAll('[data-testid="vehicles-row"]');
    expect(rows.length).toBe(50);

    const totalNodes = w.element.querySelectorAll('*').length;
    expect(totalNodes).toBeLessThan(vehicles.length);
  });

  it('переход в конец списка показывает последнюю машину', async () => {
    mockMatchMedia(false);
    const vehicles = makeVehicles(2000);
    const w = shallowMount(VehiclesList, { props: { vehicles } });

    await w.get('[data-testid="vehicles-last-page"]').trigger('click');

    const rows = w.findAll('[data-testid="vehicles-row"]');
    expect(rows.length).toBe(50);
    expect(rows[rows.length - 1].text()).toBe('2000');
  });

  it('удаление машины эмитит delete-vehicle с корректным id', async () => {
    mockMatchMedia(false);
    const vehicles = makeVehicles(2000);
    const w = shallowMount(VehiclesList, { props: { vehicles } });

    await w.findAll('.delete-btn')[0].trigger('click');

    expect(w.emitted('delete-vehicle')).toEqual([[1]]);
  });
});

describe('VehiclesList - мобилка (карточки)', () => {
  it('2000 машин: в DOM попадает страница, а не весь массив', () => {
    mockMatchMedia(true);
    const vehicles = makeVehicles(2000);
    const w = shallowMount(VehiclesList, { props: { vehicles } });

    const rows = w.findAll('[data-testid="vehicles-row"]');
    expect(rows.length).toBe(50);
  });

  it('поиск фильтрует карточки по номеру/марке', async () => {
    mockMatchMedia(true);
    const vehicles = makeVehicles(2000);
    const w = shallowMount(VehiclesList, { props: { vehicles } });

    await w.get('[data-testid="vehicles-search"]').setValue('Марка2000');

    const filtered = w.findAll('[data-testid="vehicles-row"]');
    expect(filtered.length).toBe(1);
    expect(filtered[0].text()).toContain('2000');
  });

  it('удаление машины на большом списке эмитит delete-vehicle с корректным id', async () => {
    mockMatchMedia(true);
    const vehicles = makeVehicles(2000);
    const w = shallowMount(VehiclesList, { props: { vehicles } });

    await w.findAll('.delete-btn')[0].trigger('click');

    expect(w.emitted('delete-vehicle')).toEqual([[1]]);
  });

  it('пустой список сохраняет исходное пустое состояние', () => {
    mockMatchMedia(true);
    const w = shallowMount(VehiclesList, { props: { vehicles: [] } });

    expect(w.find('.no-vehicles').text()).toContain('Нет добавленных транспортных средств');
    expect(w.find('[data-testid="vehicles-search"]').exists()).toBe(false);
  });
});
