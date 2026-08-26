import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...a) => apiRequest(...a),
}));

const notifyMock = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify: notifyMock }),
}));

vi.mock('@/stores/ui', () => ({
  useUiStore: () => ({ confirm: vi.fn().mockResolvedValue(true) }),
}));

import NewsManagement from '../NewsManagement.vue';

const NEWS = [
  { id: 1, title: 'Регламент вывоза мусора', description: 'Новый порядок вывоза', is_active: true, created_at: '2026-01-01T00:00:00Z' },
  { id: 2, title: 'Плановые работы', description: 'Отключение электричества на час', is_active: true, created_at: '2026-01-02T00:00:00Z' },
];

const ANNOUNCEMENTS = [
  { id: 10, title: 'Смена пропускного режима', description: 'С понедельника новый порядок допуска', is_important: true, is_active: true, created_at: '2026-01-03T00:00:00Z' },
  { id: 11, title: 'Технический перерыв', description: 'Сервис недоступен ночью', is_important: false, is_active: false, created_at: '2026-01-04T00:00:00Z' },
];

function jsonResponse(body) {
  return { ok: true, json: async () => body };
}

function mockApi() {
  apiRequest.mockImplementation((url) => {
    if (url === '/news/all') return Promise.resolve(jsonResponse(NEWS));
    if (url === '/announcements/all') return Promise.resolve(jsonResponse(ANNOUNCEMENTS));
    return Promise.resolve(jsonResponse({}));
  });
}

async function mountView() {
  mockApi();
  const wrapper = shallowMount(NewsManagement);
  await flushPromises();
  return wrapper;
}

describe('NewsManagement - поиск по новостям и объявлениям (#1157)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    notifyMock.mockReset();
  });

  it('пустой запрос показывает все новости на вкладке "Новости"', async () => {
    const wrapper = await mountView();

    expect(wrapper.findAll('.manage-item')).toHaveLength(2);
    expect(wrapper.find('.items-footer').text()).toBe('Всего: 2');
  });

  it('находит новость по точному вхождению в заголовок', async () => {
    const wrapper = await mountView();

    wrapper.vm.searchQuery = 'Плановые';
    await flushPromises();

    const items = wrapper.findAll('.manage-item');
    expect(items).toHaveLength(1);
    expect(items[0].text()).toContain('Плановые работы');
  });

  it('находит новость по варианту раскладки (EN-клавиши вместо RU)', async () => {
    const wrapper = await mountView();

    // "htukfvtyn" на физических клавишах EN-раскладки = "регламент" (RU).
    wrapper.vm.searchQuery = 'htukfvtyn';
    await flushPromises();

    const items = wrapper.findAll('.manage-item');
    expect(items).toHaveLength(1);
    expect(items[0].text()).toContain('Регламент вывоза мусора');
  });

  it('запрос без совпадений показывает заглушку "Ничего не найдено"', async () => {
    const wrapper = await mountView();

    wrapper.vm.searchQuery = 'несуществующий текст запроса';
    await flushPromises();

    expect(wrapper.findAll('.manage-item')).toHaveLength(0);
    expect(wrapper.find('.empty-state').text()).toBe('Ничего не найдено по запросу');
  });

  it('на вкладке "Объявления" ищет по заголовку/описанию объявлений', async () => {
    const wrapper = await mountView();

    wrapper.vm.switchTab('announcements');
    await flushPromises();
    expect(wrapper.findAll('.manage-item')).toHaveLength(2);

    wrapper.vm.searchQuery = 'пропускного режима';
    await flushPromises();

    const items = wrapper.findAll('.manage-item');
    expect(items).toHaveLength(1);
    expect(items[0].text()).toContain('Смена пропускного режима');
  });

  it('пустой запрос на вкладке "Объявления" снова показывает все', async () => {
    const wrapper = await mountView();

    wrapper.vm.switchTab('announcements');
    wrapper.vm.searchQuery = 'пропускного режима';
    await flushPromises();
    expect(wrapper.findAll('.manage-item')).toHaveLength(1);

    wrapper.vm.searchQuery = '';
    await flushPromises();

    expect(wrapper.findAll('.manage-item')).toHaveLength(2);
    expect(wrapper.find('.items-footer').text()).toBe('Всего: 2');
  });

  it('переключение вкладки сбрасывает поисковый запрос', async () => {
    const wrapper = await mountView();

    wrapper.vm.searchQuery = 'Плановые';
    await flushPromises();
    expect(wrapper.findAll('.manage-item')).toHaveLength(1);

    wrapper.vm.switchTab('announcements');
    await flushPromises();

    // Запрос сброшен - вторая вкладка не наследует фильтр прошлой.
    expect(wrapper.vm.searchQuery).toBe('');
    expect(wrapper.findAll('.manage-item')).toHaveLength(2);
  });

  it('запрос, отфильтровавший выбранный элемент, сбрасывает деталь-панель', async () => {
    const wrapper = await mountView();

    wrapper.vm.selectItem(NEWS[1]); // "Плановые работы"
    await flushPromises();
    expect(wrapper.vm.selectedItem).not.toBeNull();

    // Запрос совпадает только с другой новостью - выбранная уходит из списка.
    wrapper.vm.searchQuery = 'Регламент';
    await flushPromises();

    expect(wrapper.vm.selectedItem).toBeNull();
  });

  it('запрос, оставляющий выбранный элемент в списке, деталь-панель не гасит', async () => {
    const wrapper = await mountView();

    wrapper.vm.selectItem(NEWS[1]); // "Плановые работы"
    await flushPromises();

    wrapper.vm.searchQuery = 'Плановые';
    await flushPromises();

    expect(wrapper.vm.selectedItem).toEqual(NEWS[1]);
  });
});
