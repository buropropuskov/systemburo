import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

/**
 * Страница обращений сама выбирает первое в списке. Переход из сквозного поиска
 * должен пересилить этот выбор: человек шёл к конкретному обращению, а не к верхнему.
 */

const replace = vi.fn().mockResolvedValue(undefined);
let query = {};
vi.mock('vue-router', () => ({
  useRoute: () => ({ query }),
  useRouter: () => ({ replace }),
}));

vi.mock('@/api/feedback', () => ({
  getAllFeedback: vi.fn(),
  updateFeedbackStatus: vi.fn(),
  markFeedbackAsRead: vi.fn(),
  setFeedbackFlag: vi.fn(),
}));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify: vi.fn() }) }));

import FeedbackPage from '@/views/FeedbackPage.vue';
import { getAllFeedback, markFeedbackAsRead } from '@/api/feedback';

const stubs = {
  AdminPageShell: { template: '<div><slot /></div>' },
  RefreshButton: { template: '<button class="refresh-stub" />' },
  SearchComponent: {
    props: ['modelValue', 'title'],
    emits: ['update:modelValue'],
    template: '<input class="search-stub" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
  SkeletonTransition: { props: ['loading'], template: '<div><slot /></div>' },
  SkeletonCard: { template: '<div class="skeleton-stub" />' },
};

function fb(id) {
  return {
    id, user_id: id, user_name: `Пользователь ${id}`,
    message: `Сообщение обращения ${id} для теста системы`,
    status: 'Не решено', is_read: true,
    created_at: '2026-06-20T10:00:00Z', updated_at: '2026-06-20T10:00:00Z',
    resolution_comment: null, resolved_at: null,
  };
}

function mountPage() {
  return mount(FeedbackPage, {
    global: { stubs, config: { globalProperties: { $bus: { emit: vi.fn(), on: vi.fn(), off: vi.fn() } } } },
  });
}

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  markFeedbackAsRead.mockResolvedValue({});
  getAllFeedback.mockResolvedValue([fb(1), fb(2), fb(3)]);
  query = {};
});

describe('FeedbackPage — переход из сквозного поиска', () => {
  it('раскрывается найденное обращение, а не первое в списке', async () => {
    query = { open: '3' };
    const wrapper = mountPage();
    await flushPromises();

    expect(wrapper.text()).toContain('Обращение #3');
  });

  it('open вычищается из адреса', async () => {
    query = { open: '3' };
    mountPage();
    await flushPromises();

    expect(replace).toHaveBeenCalledWith({ query: {} });
  });

  it('строка поиска из адреса попадает в фильтр', async () => {
    query = { q: 'обращения 2', open: '2' };
    const wrapper = mountPage();
    await flushPromises();

    expect(wrapper.find('.search-stub').element.value).toBe('обращения 2');
  });

  it('без параметра ничего не открывается - страница ждёт выбора, как раньше', async () => {
    const wrapper = mountPage();
    await flushPromises();

    expect(wrapper.find('[data-testid="fb-detail"]').exists()).toBe(false);
    expect(wrapper.text()).toContain('Выберите обращение слева');
  });
});
