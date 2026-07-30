import { mount } from '@vue/test-utils';
import { describe, it, expect } from 'vitest';
import BaseDropdown from '../BaseDropdown.vue';

// Поиск в BaseDropdown идёт через общий util searchVariants (#1157) - тот же, что был
// в прежних фильтрах-компонентах до переезда фильтров на этот дропдаун.
// Кейсы перенесены оттуда: плоское `includes` теряло забытую раскладку и транслит,
// а техническое описание (раздел 7.1) обещает заказчику именно такое поведение.

const OPTIONS = [
  { id: 1, name: 'Ромашка' },
  { id: 2, name: 'Восток' },
];

const PLACES = [
  { id: 10, name: 'Склад 1', description: 'Северные ворота' },
  { id: 11, name: 'Склад 2', description: 'Южные ворота' },
];

function mountSearchable(props = {}) {
  return mount(BaseDropdown, {
    props: { options: OPTIONS, searchable: true, ...props },
  });
}

const ids = (w) => w.vm.filteredOptions.map((o) => o.id);

describe('BaseDropdown - поиск через общий util searchVariants (#1157)', () => {
  it('без запроса показывает все опции', () => {
    expect(ids(mountSearchable())).toEqual([1, 2]);
  });

  it('EN-ввод в забытой раскладке находит кириллицу', async () => {
    const w = mountSearchable();
    // "hjvfirf" на EN-раскладке физически совпадает с "ромашка" на RU.
    w.vm.searchQuery = 'hjvfirf';
    await w.vm.$nextTick();

    expect(ids(w)).toEqual([1]);
  });

  it('фонетический транслит находит кириллицу', async () => {
    const w = mountSearchable();
    w.vm.searchQuery = 'vostok';
    await w.vm.$nextTick();

    expect(ids(w)).toEqual([2]);
  });

  it('обычное вхождение подстроки продолжает работать', async () => {
    const w = mountSearchable();
    w.vm.searchQuery = 'ромаш';
    await w.vm.$nextTick();

    expect(ids(w)).toEqual([1]);
  });

  it('запрос из одних пробелов не отсеивает опции', async () => {
    const w = mountSearchable();
    w.vm.searchQuery = '   ';
    await w.vm.$nextTick();

    expect(ids(w)).toEqual([1, 2]);
  });

  it('очистка запроса возвращает все опции', async () => {
    const w = mountSearchable();
    w.vm.searchQuery = 'hjvfirf';
    await w.vm.$nextTick();
    expect(ids(w)).toHaveLength(1);

    w.vm.searchQuery = '';
    await w.vm.$nextTick();
    expect(ids(w)).toEqual([1, 2]);
  });

  it('без searchable запрос игнорируется', async () => {
    const w = mountSearchable({ searchable: false });
    w.vm.searchQuery = 'hjvfirf';
    await w.vm.$nextTick();

    expect(ids(w)).toEqual([1, 2]);
  });
});

describe('BaseDropdown - searchKeys', () => {
  it('по умолчанию ищет только по labelKey', async () => {
    const w = mountSearchable({ options: PLACES });
    w.vm.searchQuery = 'северные';
    await w.vm.$nextTick();

    expect(ids(w)).toEqual([]);
  });

  it('searchKeys добавляет поля к haystack', async () => {
    const w = mountSearchable({ options: PLACES, searchKeys: ['name', 'description'] });
    w.vm.searchQuery = 'северные';
    await w.vm.$nextTick();

    expect(ids(w)).toEqual([10]);
  });

  it('searchKeys не ломает поиск по подписи', async () => {
    const w = mountSearchable({ options: PLACES, searchKeys: ['name', 'description'] });
    w.vm.searchQuery = 'склад 2';
    await w.vm.$nextTick();

    expect(ids(w)).toEqual([11]);
  });

  it('пустое поле в searchKeys не склеивает соседей в ложное совпадение', async () => {
    const w = mountSearchable({
      options: [{ id: 20, name: 'Склад', description: null }],
      searchKeys: ['name', 'description'],
    });
    w.vm.searchQuery = 'склад';
    await w.vm.$nextTick();

    expect(ids(w)).toEqual([20]);
  });
});
