import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import BlacklistTabBase from '../BlacklistTabBase.vue';

const items = [
  { id: 1, is_active: true, reason: 'причина1' },
  { id: 2, is_active: true, reason: 'причина2' },
  { id: 3, is_active: false, reason: 'архивная' },
];

function mountBase(overrides = {}) {
  return mount(BlacklistTabBase, {
    props: {
      apiList: vi.fn().mockResolvedValue(items),
      getPrimaryText: (i) => `Запись ${i.id}`,
      getDetailRows: (i) => [{ label: 'Причина', value: i.reason }],
      searchPlaceholder: 'Поиск...',
      emptyNoun: 'записей',
      ...overrides,
    },
    global: {
      stubs: { BaseDropdown: true, SearchComponent: true, RefreshButton: true, LoaderSpinner: true },
    },
  });
}

describe('BlacklistTabBase', () => {
  it('по умолчанию показывает только активные записи', async () => {
    const wrapper = mountBase();
    await flushPromises();
    expect(wrapper.findAll('.bl-row')).toHaveLength(2);
    expect(wrapper.text()).toContain('Запись 1');
    expect(wrapper.text()).not.toContain('Запись 3');
  });

  it('эмитит count = число активных', async () => {
    const wrapper = mountBase();
    await flushPromises();
    expect(wrapper.emitted('count')).toBeTruthy();
    expect(wrapper.emitted('count').at(-1)).toEqual([2]);
  });

  it('режим Архив показывает неактивные с бейджем (архив)', async () => {
    const wrapper = mountBase();
    await flushPromises();
    wrapper.vm.onArchiveModeChange('archive');
    await flushPromises();
    expect(wrapper.findAll('.bl-row')).toHaveLength(1);
    expect(wrapper.text()).toContain('Запись 3');
    expect(wrapper.find('.bl-inactive-badge').exists()).toBe(true);
  });

  it('поиск фильтрует по причине', async () => {
    const wrapper = mountBase();
    await flushPromises();
    wrapper.vm.searchQuery = 'причина2';
    await flushPromises();
    expect(wrapper.findAll('.bl-row')).toHaveLength(1);
    expect(wrapper.text()).toContain('Запись 2');
  });

  it('поиск матчит по варианту раскладки - EN-ввод находит кириллицу (#1157)', async () => {
    // "ghbxbyf2" на EN-раскладке физически совпадает с "причина2" на RU.
    const wrapper = mountBase();
    await flushPromises();
    wrapper.vm.searchQuery = 'ghbxbyf2';
    await flushPromises();
    expect(wrapper.findAll('.bl-row')).toHaveLength(1);
    expect(wrapper.text()).toContain('Запись 2');
  });

  it('пустой поисковый запрос снова показывает все активные записи (#1157)', async () => {
    const wrapper = mountBase();
    await flushPromises();
    wrapper.vm.searchQuery = 'причина2';
    await flushPromises();
    expect(wrapper.findAll('.bl-row')).toHaveLength(1);

    wrapper.vm.searchQuery = '';
    await flushPromises();
    expect(wrapper.findAll('.bl-row')).toHaveLength(2);
  });

  it('клик по строке открывает панель деталей', async () => {
    const wrapper = mountBase();
    await flushPromises();
    expect(wrapper.find('.bl-no-selection').exists()).toBe(true);
    await wrapper.findAll('.bl-row')[0].trigger('click');
    expect(wrapper.find('.bl-details').exists()).toBe(true);
    expect(wrapper.find('.bl-details-title').text()).toBe('Запись 1');
  });

  it('строка kind=reason рендерится отдельным callout, остальные - в def-list', async () => {
    const wrapper = mountBase({
      getDetailRows: (i) => [
        { label: 'Номер', value: `N${i.id}` },
        { label: 'Причина', value: i.reason, kind: 'reason' },
      ],
    });
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click');
    expect(wrapper.find('.bl-reason').exists()).toBe(true);
    expect(wrapper.find('.bl-reason-text').text()).toBe('причина1');
    const defRows = wrapper.findAll('.bl-def-row');
    expect(defRows).toHaveLength(1);
    expect(defRows[0].text()).toContain('Номер');
    expect(wrapper.find('.bl-def-list').text()).not.toContain('причина1');
  });

  it('статус-баннер: активная - is-active, архивная - is-archived', async () => {
    const wrapper = mountBase();
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click');
    expect(wrapper.find('.bl-status-banner.is-active').exists()).toBe(true);
    expect(wrapper.find('.bl-status-banner').text()).toContain('В чёрном списке');

    wrapper.vm.onArchiveModeChange('archive');
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click');
    expect(wrapper.find('.bl-status-banner.is-archived').exists()).toBe(true);
  });

  it('иконка сущности рендерится при entity-icon', async () => {
    // Проп несёт имя глифа реестра, а не путь к файлу: значок красится цветом текста.
    const wrapper = mountBase({ entityIcon: 'car' });
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click');
    expect(wrapper.find('.bl-details-icon').exists()).toBe(true);
    expect(wrapper.find('.bl-details-icon-img').element.tagName.toLowerCase()).toBe('svg');
    expect(wrapper.find('.bl-details-icon-img').html()).toContain('<circle');
  });

  it('кнопка "Создать запись" эмитит create', async () => {
    const wrapper = mountBase();
    await flushPromises();
    // rt-btn-compact оборачивает текст в .rt-btn-label рядом с иконкой .rt-btn-icon (#1097 S9)
    const btn = wrapper.findAll('button').find((b) => b.text().includes('Создать запись'));
    await btn.trigger('click');
    expect(wrapper.emitted('create')).toBeTruthy();
  });

  it('кнопка "Убрать из ЧС" эмитит archive с активной записью', async () => {
    const wrapper = mountBase();
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click');
    const btn = wrapper.findAll('button').find((b) => b.text() === 'Убрать из ЧС');
    await btn.trigger('click');
    expect(wrapper.emitted('archive')[0][0].id).toBe(1);
  });

  it('кнопка "Вернуть в ЧС" эмитит restore для архивной записи', async () => {
    const wrapper = mountBase();
    await flushPromises();
    wrapper.vm.onArchiveModeChange('archive');
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click');
    const btn = wrapper.findAll('button').find((b) => b.text() === 'Вернуть в ЧС');
    await btn.trigger('click');
    expect(wrapper.emitted('restore')[0][0].id).toBe(3);
  });

  it('кнопка "История" в шапке эмитит history-all (общий журнал)', async () => {
    const wrapper = mountBase();
    await flushPromises();
    const btn = wrapper.findAll('button').find((b) => b.text() === 'История');
    await btn.trigger('click');
    expect(wrapper.emitted('history-all')).toBeTruthy();
  });

  it('в панели деталей нет кнопки "История" (она глобальная в шапке)', async () => {
    const wrapper = mountBase();
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click');
    const inActions = wrapper.findAll('.bl-details-actions button').find((b) => b.text() === 'История');
    expect(inActions).toBeUndefined();
  });

  it('кнопка "Удалить навсегда" эмитит purge для архивной записи', async () => {
    const wrapper = mountBase();
    await flushPromises();
    wrapper.vm.onArchiveModeChange('archive');
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click');
    const btn = wrapper.findAll('button').find((b) => b.text() === 'Удалить навсегда');
    await btn.trigger('click');
    expect(wrapper.emitted('purge')[0][0].id).toBe(3);
  });

  it('у активной записи нет кнопки "Удалить навсегда"', async () => {
    const wrapper = mountBase();
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click');
    const btn = wrapper.findAll('button').find((b) => b.text() === 'Удалить навсегда');
    expect(btn).toBeUndefined();
  });

  it('кнопка "Редактировать" в строке статуса эмитит edit для активной записи', async () => {
    const wrapper = mountBase();
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click');
    const btn = wrapper.findAll('.bl-status-row button').find((b) => b.text() === 'Редактировать');
    expect(btn).toBeTruthy();
    await btn.trigger('click');
    expect(wrapper.emitted('edit')[0][0].id).toBe(1);
  });

  it('пустой список показывает empty-state', async () => {
    const wrapper = mountBase({ apiList: vi.fn().mockResolvedValue([]) });
    await flushPromises();
    expect(wrapper.find('.bl-empty').exists()).toBe(true);
  });

  it('без lookupCard кнопки "Открыть карточку" нет', async () => {
    const wrapper = mountBase();
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click');
    const btn = wrapper.findAll('button').find((b) => b.text() === 'Открыть карточку');
    expect(btn).toBeUndefined();
  });

  it('с lookupCard: кнопка disabled пока запись не найдена, потом активна и эмитит open-card', async () => {
    const entity = { id: 99, plateNumber: 'A1' };
    const lookupCard = vi.fn().mockResolvedValue(entity);
    const wrapper = mountBase({ lookupCard });
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click');
    await flushPromises();
    expect(lookupCard).toHaveBeenCalledWith(items[0]);
    const btn = wrapper.findAll('button').find((b) => b.text() === 'Открыть карточку');
    expect(btn).toBeTruthy();
    expect(btn.attributes('disabled')).toBeUndefined();
    await btn.trigger('click');
    expect(wrapper.emitted('open-card')[0]).toEqual([entity]);
  });

  it('с lookupCard: кнопка disabled, если записи в реестре нет (null)', async () => {
    const lookupCard = vi.fn().mockResolvedValue(null);
    const wrapper = mountBase({ lookupCard });
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click');
    await flushPromises();
    const btn = wrapper.findAll('button').find((b) => b.text() === 'Открыть карточку');
    expect(btn.attributes('disabled')).toBeDefined();
  });

  it('гонка: при быстром переключении строк применяется только последний лукап', async () => {
    // Первый лукап (item1) разрешается ПОЗЖЕ второго (item2) - не должен затереть результат.
    let resolveFirst;
    const lookupCard = vi.fn()
      .mockImplementationOnce(() => new Promise((r) => { resolveFirst = r; }))
      .mockResolvedValueOnce({ id: 2, plateNumber: 'B2' });
    const wrapper = mountBase({ lookupCard });
    await flushPromises();
    await wrapper.findAll('.bl-row')[0].trigger('click'); // item1 - висит
    await wrapper.findAll('.bl-row')[1].trigger('click'); // item2 - резолвится
    await flushPromises();
    // item2 разрешён -> cardEntity = item2, loading сброшен
    expect(wrapper.vm.cardEntity).toEqual({ id: 2, plateNumber: 'B2' });
    expect(wrapper.vm.cardLoading).toBe(false);
    // теперь поздно разрешаем item1 - он устарел, не должен затереть cardEntity/loading
    resolveFirst({ id: 1, plateNumber: 'B1' });
    await flushPromises();
    expect(wrapper.vm.cardEntity).toEqual({ id: 2, plateNumber: 'B2' });
    expect(wrapper.vm.cardLoading).toBe(false);
  });
});

/**
 * Переход из сквозного поиска: найденная запись чёрного списка должна раскрыться
 * сама. Вкладки «Машины» и «Люди» смонтированы одновременно, а идентификаторы у них
 * независимые - без указания вкладки в адресе открылась бы чужая запись с тем же id.
 */
describe('BlacklistTabBase - открытие записи по ссылке из сквозного поиска', () => {
  const ITEMS = [{ id: 5, is_active: true, reason: 'угон' }, { id: 9, is_active: true, reason: 'долг' }];

  function mountWithRoute(query, tabKey = 'vehicles', replace = vi.fn().mockResolvedValue(undefined)) {
    return mount(BlacklistTabBase, {
      props: {
        apiList: vi.fn().mockResolvedValue(ITEMS),
        getPrimaryText: (i) => `Запись ${i.id}`,
        getDetailRows: () => [],
        tabKey,
      },
      global: {
        stubs: { BaseDropdown: true, SearchComponent: true, RefreshButton: true, LoaderSpinner: true },
        mocks: { $route: { query }, $router: { replace } },
      },
    });
  }

  it('запись своей вкладки открывается сразу', async () => {
    const wrapper = mountWithRoute({ tab: 'vehicles', open: '5', q: 'угон' });
    await flushPromises();

    expect(wrapper.vm.selected?.id).toBe(5);
  });

  it('строка поиска из адреса подставляется в фильтр', async () => {
    const wrapper = mountWithRoute({ tab: 'vehicles', open: '5', q: 'угон' });
    await flushPromises();

    expect(wrapper.vm.searchQuery).toBe('угон');
  });

  it('чужая вкладка ничего не открывает - id у вкладок независимые', async () => {
    const wrapper = mountWithRoute({ tab: 'persons', open: '5' }, 'vehicles');
    await flushPromises();

    expect(wrapper.vm.selected).toBeNull();
  });

  it('после открытия open вычищается из адреса', async () => {
    const replace = vi.fn().mockResolvedValue(undefined);
    mountWithRoute({ tab: 'vehicles', open: '5', q: 'угон' }, 'vehicles', replace);
    await flushPromises();

    expect(replace).toHaveBeenCalledWith({ query: { tab: 'vehicles', q: 'угон' } });
  });

  it('без роутера (монтаж в тестах и в кабинете) вкладка работает как раньше', async () => {
    const bare = mount(BlacklistTabBase, {
      props: {
        apiList: vi.fn().mockResolvedValue(ITEMS),
        getPrimaryText: (i) => `Запись ${i.id}`,
        getDetailRows: () => [],
      },
      global: { stubs: { BaseDropdown: true, SearchComponent: true, RefreshButton: true, LoaderSpinner: true } },
    });
    await flushPromises();

    expect(bare.vm.selected).toBeNull();
    expect(bare.vm.searchQuery).toBe('');
  });
});
