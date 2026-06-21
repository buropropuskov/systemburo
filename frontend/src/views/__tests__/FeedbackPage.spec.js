import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// Админ-вкладка "Обратная связь" в раскладке эталона (master-detail): список слева,
// деталь + инлайн-ответ справа. Проверяем рендер/счётчики, авто-выбор, ответ с
// сохранением комментария, возврат в работу, фильтр по статусу и обработку ошибок.

vi.mock('@/api/feedback', () => ({
  getAllFeedback: vi.fn(),
  updateFeedbackStatus: vi.fn(),
  markFeedbackAsRead: vi.fn(),
}));

const notify = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}));

import FeedbackPage from '@/views/FeedbackPage.vue';
import FilterTabs from '@/components/ui/FilterTabs.vue';
import { getAllFeedback, updateFeedbackStatus, markFeedbackAsRead } from '@/api/feedback';

const stubs = {
  AdminPageShell: { template: '<div><slot /></div>' },
  RefreshButton: { template: '<button class="refresh-stub" @click="$emit(\'refresh\')" />' },
  SearchComponent: { props: ['modelValue', 'title'], template: '<input class="search-stub" />' },
  SkeletonTransition: { props: ['loading'], template: '<div><slot /></div>' },
  SkeletonCard: { template: '<div class="skeleton-stub" />' },
};

function fb(id, over = {}) {
  return {
    id,
    user_id: id,
    user_name: `Пользователь ${id}`,
    message: `Сообщение обращения ${id} для теста системы`,
    status: 'Не решено',
    is_read: false,
    created_at: '2026-06-20T10:00:00Z',
    updated_at: '2026-06-20T10:00:00Z',
    resolution_comment: null,
    resolved_at: null,
    ...over,
  };
}

const resolved = (id, comment = 'Ответ оператора') => fb(id, {
  status: 'Решено',
  is_read: true,
  resolution_comment: comment,
  resolved_at: '2026-06-20T11:00:00Z',
});

function mountPage() {
  return mount(FeedbackPage, { global: { stubs } });
}

let wrapper;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  getAllFeedback.mockResolvedValue([]);
  updateFeedbackStatus.mockResolvedValue({});
  markFeedbackAsRead.mockResolvedValue({});
});

afterEach(() => {
  wrapper?.unmount();
});

describe('FeedbackPage', () => {
  it('рендерит список, счётчик "Всего" и счётчики вкладок', async () => {
    getAllFeedback.mockResolvedValue([resolved(1), fb(2), fb(3)]);
    wrapper = mountPage();
    await flushPromises();

    expect(wrapper.findAll('[data-testid="fb-row"]')).toHaveLength(3);
    expect(wrapper.find('.list-footer').text()).toContain('Всего: 3');

    expect(wrapper.findComponent(FilterTabs).props('tabs')).toEqual([
      { key: 'all', label: 'Все', count: 3 },
      { key: 'new', label: 'Новые', count: 2 },
      { key: 'open', label: 'В работе', count: 2 },
      { key: 'resolved', label: 'Решено', count: 1 },
    ]);
  });

  it('авто-выбирает первое обращение и показывает деталь', async () => {
    getAllFeedback.mockResolvedValue([fb(5), fb(9)]);
    wrapper = mountPage();
    await flushPromises();

    // сортировка по id desc -> первым идёт #9
    expect(wrapper.find('[data-testid="fb-detail"]').exists()).toBe(true);
    expect(wrapper.find('.detail-title').text()).toContain('Обращение #9');
  });

  it('клик по строке открывает её деталь', async () => {
    getAllFeedback.mockResolvedValue([fb(5), fb(9)]);
    wrapper = mountPage();
    await flushPromises();

    await wrapper.findAll('[data-testid="fb-row"]')[1].trigger('click');
    expect(wrapper.find('.detail-title').text()).toContain('Обращение #5');
  });

  it('ответ: шлёт status+comment, уведомляет и показывает карточку ответа', async () => {
    getAllFeedback.mockResolvedValue([fb(7)]);
    wrapper = mountPage();
    await flushPromises();

    await wrapper.find('[data-testid="fb-reply"]').setValue('Готово, проверьте раздел');
    await wrapper.find('[data-testid="fb-resolve"]').trigger('click');
    await flushPromises();

    expect(updateFeedbackStatus).toHaveBeenCalledWith(7, {
      status: 'Решено',
      comment: 'Готово, проверьте раздел',
    });
    expect(notify).toHaveBeenCalledWith(
      expect.objectContaining({ suffix: expect.stringContaining('решённым') }),
    );
    expect(wrapper.find('[data-testid="fb-answer"]').text()).toContain('Готово, проверьте раздел');
    expect(wrapper.find('[data-testid="fb-reply"]').exists()).toBe(false);
  });

  it('ответ без текста не отправляет поле comment', async () => {
    getAllFeedback.mockResolvedValue([fb(7)]);
    wrapper = mountPage();
    await flushPromises();

    await wrapper.find('[data-testid="fb-resolve"]').trigger('click');
    await flushPromises();

    expect(updateFeedbackStatus).toHaveBeenCalledWith(7, { status: 'Решено' });
  });

  it('возврат в работу: шлёт "Не решено" и очищает ответ', async () => {
    getAllFeedback.mockResolvedValue([resolved(7)]);
    wrapper = mountPage();
    await flushPromises();

    expect(wrapper.find('[data-testid="fb-answer"]').exists()).toBe(true);
    await wrapper.find('[data-testid="fb-reopen"]').trigger('click');
    await flushPromises();

    expect(updateFeedbackStatus).toHaveBeenCalledWith(7, { status: 'Не решено' });
    expect(wrapper.find('[data-testid="fb-answer"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="fb-reply"]').exists()).toBe(true);
  });

  it('"Отметить прочитанным" шлёт is_read и убирает кнопку', async () => {
    getAllFeedback.mockResolvedValue([fb(4)]);
    wrapper = mountPage();
    await flushPromises();

    expect(wrapper.find('[data-testid="fb-read"]').exists()).toBe(true);
    await wrapper.find('[data-testid="fb-read"]').trigger('click');
    await flushPromises();

    expect(markFeedbackAsRead).toHaveBeenCalledWith(4, true);
    expect(wrapper.find('[data-testid="fb-read"]').exists()).toBe(false);
  });

  it('фильтр "Решено" оставляет только решённые обращения', async () => {
    getAllFeedback.mockResolvedValue([resolved(1), fb(2)]);
    wrapper = mountPage();
    await flushPromises();

    wrapper.findComponent(FilterTabs).vm.$emit('update:modelValue', 'resolved');
    await flushPromises();

    const rows = wrapper.findAll('[data-testid="fb-row"]');
    expect(rows).toHaveLength(1);
    expect(rows[0].text()).toContain('Пользователь 1');
  });

  it('пустой список показывает подсказку, деталь скрыта', async () => {
    getAllFeedback.mockResolvedValue([]);
    wrapper = mountPage();
    await flushPromises();

    expect(wrapper.find('[data-testid="fb-empty"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="fb-detail"]').exists()).toBe(false);
  });

  it('ошибка загрузки уведомляет пользователя', async () => {
    getAllFeedback.mockRejectedValue(new Error('boom'));
    wrapper = mountPage();
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
  });
});
