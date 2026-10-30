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

import eventStream from '@/services/eventStream';
import NewsAndReview from '../NewsAndReview.vue';
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';

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

function mountView() {
  return shallowMount(NewsAndReview);
}

describe('NewsAndReview — блокировка фона читалки', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    mockNewsApi();
  });

  it('блокирует документ только пока открыта новость', async () => {
    const wrapper = mountView();
    try {
      await flushPromises();
      wrapper.vm.openNewsModal({ id: 1, title: 'Новость', created_at: '2026-10-01T09:00:00Z' });
      await wrapper.vm.$nextTick();
      expect(document.body.style.overflow).toBe('hidden');
      expect(document.documentElement.style.overflow).toBe('hidden');
      wrapper.vm.closeNewsDetailsModal();
      await wrapper.vm.$nextTick();
      expect(document.body.style.overflow).toBe('');
      expect(document.documentElement.style.overflow).toBe('');
    } finally {
      wrapper.unmount();
    }
  });

  it('при уходе со страницы сохраняет блокировку другого окна', async () => {
    const other = {};
    const wrapper = mountView();
    try {
      await flushPromises();
      wrapper.vm.openNewsModal({ id: 1, title: 'Новость', created_at: '2026-10-01T09:00:00Z' });
      await wrapper.vm.$nextTick();
      setBodyScrollLock(other, true);
      wrapper.unmount();
      expect(document.body.style.overflow).toBe('hidden');
      expect(document.documentElement.style.overflow).toBe('hidden');
    } finally {
      releaseBodyScrollLock(other);
      if (wrapper.exists()) wrapper.unmount();
    }
    expect(document.documentElement.style.overflow).toBe('');
  });
});

describe('NewsAndReview - real-time доставка ленты через SSE (#840 news.refresh)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    mockNewsApi();
    eventStream.connect.mockClear();
    eventStream.disconnect.mockClear();
    eventStream.subscribe.mockClear();
    eventStream.subscribe.mockImplementation(() => vi.fn());
  });

  it('при mount подписывается на scope news и подключается', async () => {
    const wrapper = mountView();
    await flushPromises();

    expect(eventStream.connect).toHaveBeenCalledTimes(1);
    expect(eventStream.subscribe).toHaveBeenCalledWith('news', expect.any(Function));

    wrapper.unmount();
  });

  it('колбэк subscribe перезапрашивает новости и активное объявление', async () => {
    const wrapper = mountView();
    await flushPromises();

    const scopeCb = eventStream.subscribe.mock.calls.find((c) => c[0] === 'news')[1];
    apiRequest.mockClear();
    mockNewsApi();
    scopeCb();
    await flushPromises();

    expect(apiRequest).toHaveBeenCalledWith('/news');
    expect(apiRequest).toHaveBeenCalledWith('/announcements/active');

    wrapper.unmount();
  });

  it('при unmount отписывается и отключает eventStream', async () => {
    const unsubScope = vi.fn();
    eventStream.subscribe.mockImplementation(() => unsubScope);

    const wrapper = mountView();
    await flushPromises();
    wrapper.unmount();

    expect(unsubScope).toHaveBeenCalledTimes(1);
    expect(eventStream.disconnect).toHaveBeenCalledTimes(1);
  });
});
