import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import VehiclesList from '../VehiclesList.vue';

// blank-import E1: список должен пережить импорт бланком (до 2000 машин) без рендера
// всего массива v-for'ом ни в десктопной колоночной раскладке, ни в мобильных карточках
// (обе ветки читают один и тот же pagedVehicles, но десктоп дублирует v-for на 4 колонки -
// самая тяжёлая по числу узлов раскладка). mount (не shallowMount) - пагинация теперь
// живёт внутри реального Pager, клик по data-testid у стаба ничего бы не сделал.

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

// В Pager есть только Назад/Вперёд (без прыжка в начало/конец) - .pager__btn[1].
function clickNext(w) {
  return w.findAll('.pager__btn')[1].trigger('click');
}

describe('VehiclesList - десктоп (колоночная раскладка)', () => {
  it('2000 машин: в DOM попадает страница, а не весь массив', () => {
    mockMatchMedia(false);
    const vehicles = makeVehicles(2000);
    const w = mount(VehiclesList, { props: { vehicles } });

    // vehicles-row стоит на колонке "№" - один элемент на строку.
    const rows = w.findAll('[data-testid="vehicles-row"]');
    expect(rows.length).toBe(50);

    const totalNodes = w.element.querySelectorAll('*').length;
    expect(totalNodes).toBeLessThan(vehicles.length);
  });

  it('переход на следующую страницу показывает следующий блок машин', async () => {
    mockMatchMedia(false);
    const vehicles = makeVehicles(2000);
    const w = mount(VehiclesList, { props: { vehicles } });

    await clickNext(w);

    const rows = w.findAll('[data-testid="vehicles-row"]');
    expect(rows.length).toBe(50);
    expect(rows[rows.length - 1].text()).toBe('100');
    expect(w.get('.pager__page').text()).toBe('Стр. 2 / 40');
  });

  it('удаление машины эмитит delete-vehicle с корректным id', async () => {
    mockMatchMedia(false);
    const vehicles = makeVehicles(2000);
    const w = mount(VehiclesList, { props: { vehicles } });

    await w.findAll('.delete-btn')[0].trigger('click');

    expect(w.emitted('delete-vehicle')).toEqual([[1]]);
  });

  // Баг, который чинили в этом срезе: DOM колоночный, "строка" - это набор ячеек
  // одного индекса в разных столбцах, курсор синхронизирует их через hoveredIndex.
  // Клик по пейджеру не даёт mouseleave, поэтому без явного сброса подсветка
  // оставалась на строке с тем же ЛОКАЛЬНЫМ индексом на новой странице - то есть
  // на другой машине.
  it('переход на другую страницу сбрасывает подсветку наведения по индексу строки', async () => {
    mockMatchMedia(false);
    const vehicles = makeVehicles(2000);
    const w = mount(VehiclesList, { props: { vehicles } });

    const plateCells = () => w.findAll('.vcol--plate .vcol__cell');
    await plateCells()[2].trigger('mouseenter');
    expect(plateCells()[2].classes()).toContain('vcol__cell--hover');

    await clickNext(w);

    expect(plateCells()[2].classes()).not.toContain('vcol__cell--hover');
  });
});

describe('VehiclesList - мобилка (карточки)', () => {
  it('2000 машин: в DOM попадает страница, а не весь массив', () => {
    mockMatchMedia(true);
    const vehicles = makeVehicles(2000);
    const w = mount(VehiclesList, { props: { vehicles } });

    const rows = w.findAll('[data-testid="vehicles-row"]');
    expect(rows.length).toBe(50);
  });

  it('поиск фильтрует карточки по номеру/марке', async () => {
    mockMatchMedia(true);
    const vehicles = makeVehicles(2000);
    const w = mount(VehiclesList, { props: { vehicles } });

    await w.get('[data-testid="vehicles-search"]').setValue('Марка2000');

    const filtered = w.findAll('[data-testid="vehicles-row"]');
    expect(filtered.length).toBe(1);
    expect(filtered[0].text()).toContain('2000');
  });

  it('удаление машины на большом списке эмитит delete-vehicle с корректным id', async () => {
    mockMatchMedia(true);
    const vehicles = makeVehicles(2000);
    const w = mount(VehiclesList, { props: { vehicles } });

    await w.findAll('.delete-btn')[0].trigger('click');

    expect(w.emitted('delete-vehicle')).toEqual([[1]]);
  });

  it('пустой список сохраняет исходное пустое состояние', () => {
    mockMatchMedia(true);
    const w = mount(VehiclesList, { props: { vehicles: [] } });

    expect(w.find('[data-testid="vehicles-empty"]').text()).toContain('Нет добавленных транспортных средств');
    expect(w.find('[data-testid="vehicles-search"]').exists()).toBe(false);
  });

  // Тот же регресс, что и у EmployeesList (общий composable): активный поиск сузил
  // видимую часть, а список ЦЕЛИКОМ упал ниже порога тулбара - тулбар прячется,
  // и вместе с ним обязан сброситься поиск, иначе фильтр молча остаётся действующим.
  it('список уменьшился ниже порога тулбара во время активного поиска - сбрасывает поиск, показывает все карточки', async () => {
    mockMatchMedia(true);
    const vehicles = makeVehicles(2000);
    const w = mount(VehiclesList, { props: { vehicles } });

    await w.get('[data-testid="vehicles-search"]').setValue('Марка2000');
    expect(w.findAll('[data-testid="vehicles-row"]').length).toBe(1);

    await w.setProps({ vehicles: makeVehicles(10) });

    expect(w.find('[data-testid="vehicles-search"]').exists()).toBe(false);
    expect(w.findAll('[data-testid="vehicles-row"]').length).toBe(10);
  });
});
