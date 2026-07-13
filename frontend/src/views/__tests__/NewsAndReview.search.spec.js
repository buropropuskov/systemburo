import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...a) => apiRequest(...a),
}));

vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

import NewsAndReview from '../NewsAndReview.vue';

const NEWS = [
  { id: 1, title: 'Регламент вывоза мусора', description: 'Новый порядок вывоза', created_at: '2026-01-01T00:00:00Z' },
  { id: 2, title: 'Плановые работы', description: 'Отключение электричества на час', created_at: '2026-01-02T00:00:00Z' },
];

const ANNOUNCEMENT = {
  id: 5,
  title: 'Смена пропускного режима',
  description: 'С понедельника новый порядок допуска',
  is_important: true,
  created_at: '2026-01-03T00:00:00Z',
};

function jsonResponse(body) {
  return { ok: true, json: async () => body };
}

function mockNewsApi({ news = NEWS, announcement = ANNOUNCEMENT } = {}) {
  apiRequest.mockImplementation((url) => {
    if (url === '/news') return Promise.resolve(jsonResponse(news));
    if (url === '/announcements/active') return Promise.resolve(jsonResponse(announcement));
    return Promise.resolve(jsonResponse({}));
  });
}

async function mountView(opts) {
  mockNewsApi(opts);
  const wrapper = shallowMount(NewsAndReview);
  await flushPromises();
  return wrapper;
}

describe('NewsAndReview - поиск по новостям и объявлениям (#1157)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
  });

  it('пустой запрос показывает все новости и активное объявление', async () => {
    const wrapper = await mountView();

    expect(wrapper.findAll('.news-item')).toHaveLength(2);
    expect(wrapper.find('[data-testid="ob-announcement"]').exists()).toBe(true);
  });

  it('находит новость по точному вхождению в заголовок', async () => {
    const wrapper = await mountView();

    wrapper.vm.searchQuery = 'Плановые';
    await flushPromises();

    const items = wrapper.findAll('.news-item');
    expect(items).toHaveLength(1);
    expect(items[0].text()).toContain('Плановые работы');
  });

  it('находит новость по варианту раскладки (EN-клавиши вместо RU)', async () => {
    const wrapper = await mountView();

    // "htukfvtyn" на физических клавишах EN-раскладки = "регламент" (RU).
    wrapper.vm.searchQuery = 'htukfvtyn';
    await flushPromises();

    const items = wrapper.findAll('.news-item');
    expect(items).toHaveLength(1);
    expect(items[0].text()).toContain('Регламент вывоза мусора');
  });

  it('запрос, не совпадающий с активным объявлением, скрывает его карточку', async () => {
    const wrapper = await mountView();

    wrapper.vm.searchQuery = 'Плановые';
    await flushPromises();

    expect(wrapper.find('[data-testid="ob-announcement"]').exists()).toBe(false);
  });

  it('запрос, совпадающий с объявлением, оставляет его видимым', async () => {
    const wrapper = await mountView();

    wrapper.vm.searchQuery = 'пропускного режима';
    await flushPromises();

    expect(wrapper.find('[data-testid="ob-announcement"]').exists()).toBe(true);
  });

  it('запрос без совпадений очищает список новостей и показывает заглушку поиска', async () => {
    const wrapper = await mountView();

    wrapper.vm.searchQuery = 'несуществующий текст запроса';
    await flushPromises();

    expect(wrapper.findAll('.news-item')).toHaveLength(0);
    expect(wrapper.find('.empty-state p').text()).toBe('Ничего не найдено по запросу');
  });
});
