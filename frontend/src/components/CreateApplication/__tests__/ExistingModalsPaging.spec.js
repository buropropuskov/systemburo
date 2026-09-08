import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import ExistingCarsModal from '../ExistingCarsModal.vue';
import ExistingEmployeesModal from '../ExistingEmployeesModal.vue';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })) }));

const stubs = { SearchComponent: true, LoaderSpinner: true };

/**
 * Окно выбора листается по 10 строк (#2399): в реестре у одного заявителя набирается
 * под сотню записей, и сплошной список в модалке читать нечем.
 *
 * Shift-диапазон при этом считается по СТРАНИЦЕ - по тому, что человек видит. Взять
 * диапазон через границу страницы нельзя намеренно: якорь уходит с экрана, и что
 * именно попадёт в выбор, предсказать невозможно.
 */
const людей = (n) => Array.from({ length: n }, (_, i) => ({
  id: i + 1, last_name: `Фамилия${i + 1}`, first_name: 'Имя', middle_name: 'Отчество',
}));
const машин = (n) => Array.from({ length: n }, (_, i) => ({
  id: i + 1, number: `А${String(i + 1).padStart(3, '0')}АА777`, mark: 'Volvo',
}));

describe.each([
  ['сотрудники', ExistingEmployeesModal, людей, 'displayedEmployees', 'pagedEmployees', 'tempSelectedEmployees'],
  ['машины', ExistingCarsModal, машин, 'displayedCars', 'pagedCars', 'tempSelectedCars'],
])('%s - страницы по 10 (#2399)', (_имя, Компонент, набор, пок, стр, темп) => {
  const открыть = async (n) => {
    const w = mount(Компонент, { props: { visible: false }, global: { stubs } });
    w.vm[пок] = набор(n);
    await w.vm.$nextTick();
    return w;
  };

  it('на странице не больше десяти строк', async () => {
    const w = await открыть(70);
    expect(w.vm[стр]).toHaveLength(10);
    expect(w.vm.totalPages).toBe(7);
    w.unmount();
  });

  it('короткий список умещается на одной странице', async () => {
    const w = await открыть(4);
    expect(w.vm[стр]).toHaveLength(4);
    expect(w.vm.totalPages, 'пагинация не должна появляться из-за четырёх строк').toBe(1);
    w.unmount();
  });

  it('переход на страницу отдаёт следующую десятку', async () => {
    const w = await открыть(70);
    w.vm.goToPage(3);
    await w.vm.$nextTick();
    expect(w.vm[стр].map((i) => i.id)).toEqual([21, 22, 23, 24, 25, 26, 27, 28, 29, 30]);
    w.unmount();
  });

  it('за пределы списка страница не уходит', async () => {
    const w = await открыть(25);
    w.vm.goToPage(99);
    expect(w.vm.currentPage).toBe(3);
    w.vm.goToPage(0);
    expect(w.vm.currentPage).toBe(1);
    w.unmount();
  });

  it('выбор переживает переход по страницам', async () => {
    // Иначе набрать людей с разных страниц было бы нечем - а ради этого всё и затевалось.
    const w = await открыть(70);
    w.vm.handleRowClick(w.vm[стр][0], {});
    w.vm.goToPage(2);
    await w.vm.$nextTick();
    w.vm.handleRowClick(w.vm[стр][0], {});
    expect(w.vm[темп].map((i) => i.id)).toEqual([1, 11]);
    w.unmount();
  });

  it('Shift берёт диапазон в пределах страницы', async () => {
    const w = await открыть(70);
    w.vm.goToPage(2);
    await w.vm.$nextTick();
    w.vm.handleRowClick(w.vm[стр][1], {});
    w.vm.handleRowClick(w.vm[стр][4], { shiftKey: true });
    expect(w.vm[темп].map((i) => i.id)).toEqual([12, 13, 14, 15]);
    w.unmount();
  });

  it('новая выдача возвращает на первую страницу', async () => {
    // Поиск, начатый на третьей странице, иначе показывает пустоту при непустом
    // результате: страниц стало меньше, а номер остался прежним.
    const w = await открыть(70);
    w.vm.goToPage(5);
    w.vm[пок] = набор(70);
    w.vm.applySearch();
    expect(w.vm.currentPage).toBe(1);
    w.unmount();
  });
});
