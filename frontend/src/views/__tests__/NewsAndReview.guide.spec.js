import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...a) => apiRequest(...a),
}));

const listGuideSections = vi.fn();
vi.mock('@/api/guide', () => ({
  listGuideSections: (...a) => listGuideSections(...a),
}));

// NewsAndReview подписывается на eventStream (#840 news.refresh) в mounted -
// без мока реальный модуль дёргает apiRequest('/events/ticket') и планирует
// reconnect реальным таймером, засоряя тесты, не связанные с real-time.
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

import NewsAndReview from '../NewsAndReview.vue';

function jsonResponse(body) {
  return { ok: true, json: async () => body };
}

function mockNewsApi() {
  apiRequest.mockImplementation((url) => {
    if (url === '/news') return Promise.resolve(jsonResponse([]));
    if (url === '/announcements/active') return Promise.resolve(jsonResponse(null));
    return Promise.resolve(jsonResponse({}));
  });
}

async function mountView() {
  const wrapper = shallowMount(NewsAndReview);
  await flushPromises();
  return wrapper;
}

describe('NewsAndReview — разделы руководства из API', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    listGuideSections.mockReset();
    mockNewsApi();
  });

  it('грузит разделы из GET /guide/sections при первом открытии', async () => {
    const sections = [
      { role: 'user', title: 'Пользователь', lead: 'l', items: [], file: null },
      { role: 'admin', title: 'Администратор', lead: 'l', items: [], file: null },
    ];
    listGuideSections.mockResolvedValue(sections);
    const wrapper = await mountView();

    // До открытия модалки разделы не запрашиваются.
    expect(wrapper.vm.guideSections).toEqual([]);
    expect(listGuideSections).not.toHaveBeenCalled();

    wrapper.vm.openGuide();
    await flushPromises();

    expect(listGuideSections).toHaveBeenCalledTimes(1);
    expect(wrapper.vm.guideSections).toEqual(sections);
    expect(wrapper.vm.showGuide).toBe(true);
  });

  it('не перезапрашивает разделы при повторном открытии', async () => {
    listGuideSections.mockResolvedValue([{ role: 'user', title: 'x', lead: '', items: [], file: null }]);
    const wrapper = await mountView();

    wrapper.vm.openGuide();
    await flushPromises();
    wrapper.vm.closeGuide();
    wrapper.vm.openGuide();
    await flushPromises();

    expect(listGuideSections).toHaveBeenCalledTimes(1);
  });

  it('пустой ответ оставляет guideSections пустым (модалка покажет заглушку)', async () => {
    listGuideSections.mockResolvedValue([]);
    const wrapper = await mountView();

    wrapper.vm.openGuide();
    await flushPromises();

    expect(wrapper.vm.guideSections).toEqual([]);
  });

  it('ошибка загрузки не роняет компонент и сбрасывает loading', async () => {
    listGuideSections.mockRejectedValue(new Error('boom'));
    const wrapper = await mountView();

    wrapper.vm.openGuide();
    await flushPromises();

    expect(wrapper.vm.guideLoading).toBe(false);
    expect(wrapper.vm.guideSections).toEqual([]);
  });
});
