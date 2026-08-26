import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// Админ-вкладка "Обратная связь" в раскладке эталона (master-detail): список слева,
// деталь + инлайн-ответ справа. Проверяем рендер/счётчики, авто-выбор, авто-отметку
// прочтения при открытии, общий флажок, ответ с сохранением комментария, возврат в
// работу, фильтр по статусу и обработку ошибок.

vi.mock('@/api/feedback', () => ({
  getAllFeedback: vi.fn(),
  updateFeedbackStatus: vi.fn(),
  markFeedbackAsRead: vi.fn(),
  setFeedbackFlag: vi.fn(),
}));

const notify = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}));

const busEmit = vi.fn();

import FeedbackPage from '@/views/FeedbackPage.vue';
import FilterTabs from '@/components/ui/FilterTabs.vue';
import { getAllFeedback, updateFeedbackStatus, markFeedbackAsRead, setFeedbackFlag } from '@/api/feedback';

const stubs = {
  AdminPageShell: { template: '<div><slot /></div>' },
  RefreshButton: { template: '<button class="refresh-stub" @click="$emit(\'refresh\')" />' },
  SearchComponent: {
    props: ['modelValue', 'title'],
    emits: ['update:modelValue'],
    template: '<input class="search-stub" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
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
  return mount(FeedbackPage, {
    global: {
      stubs,
      config: { globalProperties: { $bus: { emit: busEmit, on: vi.fn(), off: vi.fn() } } },
    },
  });
}

// Строка списка по имени автора (авто-отметка прочтения переупорядочивает список,
// поэтому индекс строки нестабилен - ищем по содержимому).
function rowByAuthor(name) {
  return wrapper.findAll('[data-testid="fb-row"]').find((r) => r.text().includes(name));
}

let wrapper;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  getAllFeedback.mockResolvedValue([]);
  updateFeedbackStatus.mockResolvedValue({});
  markFeedbackAsRead.mockResolvedValue({});
  setFeedbackFlag.mockResolvedValue({});
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

    // При входе ничего не открывается автоматически -> ни одно обращение не
    // прочитано, "Новые" = 2 (#2, #3). Статусные вкладки чтением не затронуты.
    expect(wrapper.findComponent(FilterTabs).props('tabs')).toEqual([
      { key: 'all', label: 'Все', count: 3 },
      { key: 'new', label: 'Новые', count: 2 },
      { key: 'open', label: 'В работе', count: 2 },
      { key: 'resolved', label: 'Решено', count: 1 },
    ]);
  });

  it('при входе на вкладку не открывает обращение автоматически', async () => {
    getAllFeedback.mockResolvedValue([fb(5), fb(9)]);
    wrapper = mountPage();
    await flushPromises();

    // Список отрисован, но деталь не показана - открытие только по клику.
    expect(wrapper.findAll('[data-testid="fb-row"]')).toHaveLength(2);
    expect(wrapper.find('[data-testid="fb-detail"]').exists()).toBe(false);
    expect(wrapper.find('.no-selection').exists()).toBe(true);
    expect(markFeedbackAsRead).not.toHaveBeenCalled();
  });

  it('клик по строке открывает её деталь', async () => {
    getAllFeedback.mockResolvedValue([fb(5), fb(9)]);
    wrapper = mountPage();
    await flushPromises();

    await rowByAuthor('Пользователь 5').trigger('click');
    expect(wrapper.find('.detail-title').text()).toContain('Обращение #5');
  });

  // Класс unread несёт подсветку строки (жёлтый фон + полоса слева, как в Центре).
  it('непрочитанное обращение помечено классом unread, прочтение его снимает', async () => {
    getAllFeedback.mockResolvedValue([resolved(1), fb(2)]);
    wrapper = mountPage();
    await flushPromises();

    expect(rowByAuthor('Пользователь 2').classes()).toContain('unread');
    expect(rowByAuthor('Пользователь 1').classes()).not.toContain('unread');

    await rowByAuthor('Пользователь 2').trigger('click');
    await flushPromises();
    expect(rowByAuthor('Пользователь 2').classes()).not.toContain('unread');
  });

  it('ответ: шлёт status+comment, уведомляет и показывает карточку ответа', async () => {
    getAllFeedback.mockResolvedValue([fb(7)]);
    wrapper = mountPage();
    await flushPromises();

    await rowByAuthor('Пользователь 7').trigger('click');
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

    await rowByAuthor('Пользователь 7').trigger('click');
    await wrapper.find('[data-testid="fb-resolve"]').trigger('click');
    await flushPromises();

    expect(updateFeedbackStatus).toHaveBeenCalledWith(7, { status: 'Решено' });
  });

  it('возврат в работу: шлёт "Не решено" и очищает ответ', async () => {
    getAllFeedback.mockResolvedValue([resolved(7)]);
    wrapper = mountPage();
    await flushPromises();

    await rowByAuthor('Пользователь 7').trigger('click');
    expect(wrapper.find('[data-testid="fb-answer"]').exists()).toBe(true);
    await wrapper.find('[data-testid="fb-reopen"]').trigger('click');
    await flushPromises();

    expect(updateFeedbackStatus).toHaveBeenCalledWith(7, { status: 'Не решено' });
    expect(wrapper.find('[data-testid="fb-answer"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="fb-reply"]').exists()).toBe(true);
  });

  it('клик по обращению авто-отмечает его прочитанным и гасит бейдж через $bus', async () => {
    getAllFeedback.mockResolvedValue([fb(4)]);
    wrapper = mountPage();
    await flushPromises();

    // До клика ничего не открыто и не прочитано.
    expect(markFeedbackAsRead).not.toHaveBeenCalled();

    await rowByAuthor('Пользователь 4').trigger('click');
    await flushPromises();

    // Клик открыл #4 -> персональная отметка прочтения (без второго аргумента).
    expect(markFeedbackAsRead).toHaveBeenCalledWith(4);
    expect(busEmit).toHaveBeenCalledWith('feedback-read', 4);
    // Ручной кнопки "Отметить прочитанным" нет.
    expect(wrapper.find('[data-testid="fb-read"]').exists()).toBe(false);
  });

  it('уже прочитанное обращение не отмечается повторно при открытии', async () => {
    getAllFeedback.mockResolvedValue([resolved(8)]);
    wrapper = mountPage();
    await flushPromises();

    await rowByAuthor('Пользователь 8').trigger('click');
    await flushPromises();

    expect(markFeedbackAsRead).not.toHaveBeenCalled();
  });

  it('флажок: клик переключает общий флажок обращения', async () => {
    getAllFeedback.mockResolvedValue([fb(4)]);
    wrapper = mountPage();
    await flushPromises();

    const flag = wrapper.find('[data-testid="fb-flag"]');
    expect(flag.exists()).toBe(true);
    expect(flag.classes()).not.toContain('is-flagged');

    await flag.trigger('click');
    await flushPromises();

    expect(setFeedbackFlag).toHaveBeenCalledWith(4, true);
    expect(wrapper.find('[data-testid="fb-flag"]').classes()).toContain('is-flagged');
  });

  it('флажок: клик не открывает деталь и не отмечает прочтение', async () => {
    getAllFeedback.mockResolvedValue([fb(4), fb(6)]);
    wrapper = mountPage();
    await flushPromises();

    // Клик по флажку строки #4 не должен её открыть или прочитать.
    await rowByAuthor('Пользователь 4').find('[data-testid="fb-flag"]').trigger('click');
    await flushPromises();

    expect(setFeedbackFlag).toHaveBeenCalledWith(4, true);
    expect(markFeedbackAsRead).not.toHaveBeenCalled();
    // Ничего не открылось - деталь по-прежнему не показана.
    expect(wrapper.find('[data-testid="fb-detail"]').exists()).toBe(false);
  });

  it('поиск находит по вводу в EN-раскладке (общий util вместо вариантов из SearchComponent)', async () => {
    getAllFeedback.mockResolvedValue([
      fb(1, { user_name: 'Иванов', message: 'Пропала карта' }),
      fb(2, { user_name: 'Петров', message: 'Не работает вход' }),
    ]);
    wrapper = mountPage();
    await flushPromises();

    // "bdfyjd" на физических клавишах = "иванов".
    await wrapper.find('.search-stub').setValue('bdfyjd');
    await flushPromises();

    const rows = wrapper.findAll('[data-testid="fb-row"]');
    expect(rows).toHaveLength(1);
    expect(rows[0].text()).toContain('Иванов');
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
