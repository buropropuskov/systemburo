import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// FE-S3 (#706): вкладка "Доступные мне" - список доступных вложений + деталь.
// Проверяем рендер списка, пустое состояние и защиту детали от гонки (#632):
// быстрые клики не дают медленному ответу предыдущего вложения затереть актуальное.

vi.mock('@/api/applications', () => ({
  getAccessibleAttachments: vi.fn(),
  getAccessibleAttachmentDetail: vi.fn(),
}));

const notify = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}));

import AccessibleAttachmentsView from '@/views/AccessibleAttachmentsView.vue';
import { getAccessibleAttachments, getAccessibleAttachmentDetail } from '@/api/applications';

const stubs = {
  AdminPageShell: { template: '<div><slot /></div>' },
  RefreshButton: { template: '<button class="refresh-stub" @click="$emit(\'refresh\')" />' },
  ApplicationAttachmentDetail: {
    props: ['attachment', 'cars', 'employees', 'items'],
    template: '<div class="detail-stub">{{ attachment.attachment_id }}</div>',
  },
};

function makeItem(id, over = {}) {
  return {
    attachment_id: id,
    attachment_type: 'cars',
    attachment_display_name: `Вложение ${id}`,
    application_number: `A-${id}`,
    organization_name: 'ООО Ромашка',
    sender_full_name: 'Иванов И.И.',
    entry_date_from: '2026-06-01',
    entry_date_to: '2026-06-02',
    places: 'Склад 1',
    status: 'Согласовано',
    ...over,
  };
}

function mountView() {
  return mount(AccessibleAttachmentsView, { global: { stubs } });
}

let wrapper;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
});

afterEach(() => {
  wrapper?.unmount();
});

describe('AccessibleAttachmentsView (FE-S3)', () => {
  it('рендерит список карточек вложений и счётчик', async () => {
    getAccessibleAttachments.mockResolvedValue({
      items: [makeItem(1), makeItem(2)],
      meta: { total: 2, page: 1, per_page: 30 },
    });
    wrapper = mountView();
    await flushPromises();

    const cards = wrapper.findAll('[data-testid="aa-card"]');
    expect(cards).toHaveLength(2);
    expect(wrapper.text()).toContain('Вложение 1');
    expect(wrapper.text()).toContain('A-1');
    expect(wrapper.find('.list-footer').text()).toContain('Всего: 2');
    expect(wrapper.find('[data-testid="aa-empty"]').exists()).toBe(false);
  });

  it('показывает пустое состояние без вложений', async () => {
    getAccessibleAttachments.mockResolvedValue({
      items: [],
      meta: { total: 0, page: 1, per_page: 30 },
    });
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.find('[data-testid="aa-empty"]').exists()).toBe(true);
    expect(wrapper.text()).toContain('Доступных вложений нет');
    expect(wrapper.findAll('[data-testid="aa-card"]')).toHaveLength(0);
  });

  it('уведомляет об ошибке загрузки списка', async () => {
    getAccessibleAttachments.mockRejectedValue(new Error('boom'));
    wrapper = mountView();
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    expect(wrapper.find('[data-testid="aa-empty"]').exists()).toBe(true);
  });

  it('не даёт устаревшему ответу детали затереть актуальный выбор (#632)', async () => {
    getAccessibleAttachments.mockResolvedValue({
      items: [makeItem(1), makeItem(2)],
      meta: { total: 2, page: 1, per_page: 30 },
    });
    const resolvers = new Map();
    getAccessibleAttachmentDetail.mockImplementation(
      (id) => new Promise((resolve) => resolvers.set(id, resolve)),
    );

    wrapper = mountView();
    await flushPromises();

    const cards = wrapper.findAll('[data-testid="aa-card"]');
    await cards[0].trigger('click'); // выбрали вложение 1 (медленный ответ)
    await cards[1].trigger('click'); // быстро переключились на вложение 2

    // Резолвим последний выбор первым, затем устаревший первый запрос.
    resolvers.get(2)({ attachment: makeItem(2), cars: [] });
    await flushPromises();
    resolvers.get(1)({ attachment: makeItem(1), cars: [] });
    await flushPromises();

    // Деталь показывает вложение 2 (актуальный выбор), а не затёртое первым ответом.
    expect(wrapper.find('.detail-stub').text()).toBe('2');
  });
});
