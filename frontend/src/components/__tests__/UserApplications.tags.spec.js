import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

/**
 * Теги заявки в личном кабинете живут по тем же правилам, что в Центре (#2319).
 *
 * Раньше они были свёрстаны в самом кабинете и всегда шли полным текстом: на узкой
 * строке переносились и раздвигали её. Свёртку («N похожи на ЧС» -> число -> иконка,
 * хвост в «+N») умеет только общий ApplicationTags, поэтому проверяем, что кабинет
 * рисует теги ИМ, а не своей разметкой, и что ширина колонки до него доходит - без
 * неё компонент считает, что места вдоволь, и свёртка не включается никогда.
 */

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })),
}));

const APPLICATION = {
  id: 42,
  application_number: '№ 20260905/001',
  status: 'В обработке',
  confirmation: 'Согласование',
  sending_datetime: new Date(Date.now() - 6 * 86400000).toISOString(),
  blacklist_flags_count: 1,
  has_roof_access: true,
  has_free_parking: true,
  sender_is_important: true,
};

vi.mock('@/api/applications', () => ({
  getUserApplicationsPaginated: vi.fn(),
  getApplicationById: vi.fn(() => Promise.resolve({ message: 'Не найдена' })),
  getUserStatusUpdatesCount: vi.fn(() => Promise.resolve({ status_updates: 0 })),
}));

import UserApplications from '../UserApplications.vue';
import ApplicationTags from '../ApplicationTags.vue';
import { getUserApplicationsPaginated } from '@/api/applications';
import { useAuthStore } from '@/stores/auth';

async function mountCabinet() {
  setActivePinia(createPinia());
  // Без токена и владельца список не грузится вовсе: запрос без scope бэк понял бы
  // как «отдай весь кабинет» и его намеренно не отправляют (#2218).
  useAuthStore().token = 'test-token';
  getUserApplicationsPaginated.mockResolvedValue({
    items: [APPLICATION],
    meta: { total: 1, page: 1, per_page: 30 },
  });
  const wrapper = mount(UserApplications, {
    props: { userId: 1 },
    global: { mocks: { $route: { query: {} }, $router: { replace: vi.fn(() => Promise.resolve()), push: vi.fn() } } },
  });
  await flushPromises();
  return wrapper;
}

describe('UserApplications — теги строки', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('рисует теги общим компонентом, а не собственной разметкой', async () => {
    const wrapper = await mountCabinet();

    const tags = wrapper.findComponent(ApplicationTags);
    expect(tags.exists(), 'теги кабинета должны идти через ApplicationTags - иначе они не сворачиваются').toBe(true);
    expect(tags.props('application').id).toBe(APPLICATION.id);

    wrapper.unmount();
  });

  it('ширина колонки доходит до компонента', async () => {
    const wrapper = await mountCabinet();

    // В jsdom ResizeObserver не срабатывает, поэтому ставим ширину напрямую: важно,
    // что она СВЯЗАНА с пропом. Без этой связи компонент всегда считает, что места
    // вдоволь, и теги остаются полным текстом - тот самый дефект, что чиним.
    await wrapper.setData({ tagsColumnWidth: 130 });
    expect(wrapper.findComponent(ApplicationTags).props('availableWidth')).toBe(130);

    wrapper.unmount();
  });

  it('«Важный» в кабинете скрыт: отправитель там - сам читающий', async () => {
    const wrapper = await mountCabinet();

    const tags = wrapper.findComponent(ApplicationTags);
    expect(tags.props('exclude')).toContain('important');
    expect(tags.text()).not.toContain('Важный');
    expect(tags.text(), 'прочие теги остаются на месте').toContain('Крыша');

    wrapper.unmount();
  });
});
