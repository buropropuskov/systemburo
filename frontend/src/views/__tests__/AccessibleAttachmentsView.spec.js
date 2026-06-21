import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// FE-S3 (#706): вкладка "Доступные мне" - список доступных вложений + деталь.
// FE-S6 (#706): панель поиска и фильтров (тип/организация/компания) + URL-как-state.
// Проверяем рендер списка, пустое состояние, защиту детали от гонки (#632),
// формирование query-параметров фильтрами и сброс пагинации при смене фильтра.

vi.mock('@/api/applications', () => ({
  getAccessibleAttachments: vi.fn(),
  getAccessibleAttachmentDetail: vi.fn(),
}));

vi.mock('@/api/organizations', () => ({
  getOrganizations: vi.fn(),
  getCompanies: vi.fn(),
}));

const notify = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}));

// Роутер мокаем, чтобы тестировать URL-как-state без поднятия реального роутера:
// route.query - источник стартовых фильтров, router.replace - запись фильтров в URL.
const { routeState, replace } = vi.hoisted(() => ({
  routeState: { query: {} },
  replace: vi.fn(() => Promise.resolve()),
}));
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: routeState.query }),
  useRouter: () => ({ replace }),
}));

import AccessibleAttachmentsView from '@/views/AccessibleAttachmentsView.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import { getAccessibleAttachments, getAccessibleAttachmentDetail } from '@/api/applications';
import { getOrganizations, getCompanies } from '@/api/organizations';

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

function mountView(query = {}) {
  routeState.query = query;
  return mount(AccessibleAttachmentsView, { global: { stubs } });
}

let wrapper;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  routeState.query = {};
  getOrganizations.mockResolvedValue([]);
  getCompanies.mockResolvedValue([]);
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

  it('уведомляет и снимает выбор при ошибке загрузки детали', async () => {
    getAccessibleAttachments.mockResolvedValue({
      items: [makeItem(1)],
      meta: { total: 1, page: 1, per_page: 30 },
    });
    getAccessibleAttachmentDetail.mockRejectedValue(new Error('boom'));

    wrapper = mountView();
    await flushPromises();
    await wrapper.find('[data-testid="aa-card"]').trigger('click');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    expect(wrapper.find('[data-testid="aa-detail"]').exists()).toBe(false);
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

describe('AccessibleAttachmentsView (FE-S6) фильтры', () => {
  it('восстанавливает фильтры из query при загрузке (URL-как-state)', async () => {
    getAccessibleAttachments.mockResolvedValue({ items: [], meta: { total: 0 } });
    wrapper = mountView({ type: 'cars', search: 'абв', organization: '7' });
    await flushPromises();

    expect(getAccessibleAttachments).toHaveBeenCalledWith(
      expect.objectContaining({
        attachment_type: 'cars',
        search: 'абв',
        organization_id: 7,
        page: 1,
        per_page: 30,
      }),
    );
  });

  it('смена типа шлёт attachment_type, пишет URL и сбрасывает страницу на 1', async () => {
    getAccessibleAttachments.mockResolvedValue({
      items: [makeItem(1)],
      meta: { total: 60, page: 1, per_page: 30 },
    });
    wrapper = mountView();
    await flushPromises();

    // Подгружаем вторую страницу через "Показать ещё" - страница становится 2.
    await wrapper.find('[data-testid="aa-load-more"]').trigger('click');
    await flushPromises();
    getAccessibleAttachments.mockClear();

    const dropdowns = wrapper.findAllComponents(BaseDropdown);
    dropdowns[0].vm.$emit('update:modelValue', 'cars'); // тип вложения
    await flushPromises();

    expect(getAccessibleAttachments).toHaveBeenCalledWith(
      expect.objectContaining({ attachment_type: 'cars', page: 1 }),
    );
    expect(replace).toHaveBeenLastCalledWith({ query: { type: 'cars' } });
  });

  it('смена организации шлёт organization_id числом и пишет URL', async () => {
    getAccessibleAttachments.mockResolvedValue({ items: [], meta: { total: 0 } });
    getOrganizations.mockResolvedValue([{ id: 7, name: 'ООО Ромашка' }]);
    wrapper = mountView();
    await flushPromises();
    getAccessibleAttachments.mockClear();

    const dropdowns = wrapper.findAllComponents(BaseDropdown);
    dropdowns[1].vm.$emit('update:modelValue', 7); // организация
    await flushPromises();

    expect(getAccessibleAttachments).toHaveBeenCalledWith(
      expect.objectContaining({ organization_id: 7, page: 1 }),
    );
    expect(replace).toHaveBeenLastCalledWith({ query: { organization: '7' } });
  });

  it('поиск применяется после debounce и шлёт search', async () => {
    vi.useFakeTimers();
    try {
      getAccessibleAttachments.mockResolvedValue({ items: [], meta: { total: 0 } });
      wrapper = mountView();
      // advanceTimersByTimeAsync прокручивает микротаски между таймерами - так
      // под фейковыми таймерами дорешиваются промисы onMounted-запросов.
      await vi.advanceTimersByTimeAsync(0);
      getAccessibleAttachments.mockClear();

      await wrapper.find('[data-testid="aa-search"]').setValue('ромашка');
      // До истечения debounce (300мс) запроса нет.
      expect(getAccessibleAttachments).not.toHaveBeenCalled();

      await vi.advanceTimersByTimeAsync(300);

      expect(getAccessibleAttachments).toHaveBeenCalledWith(
        expect.objectContaining({ search: 'ромашка', page: 1 }),
      );
    } finally {
      vi.useRealTimers();
    }
  });

  it('сброс фильтров очищает query-параметры и URL', async () => {
    getAccessibleAttachments.mockResolvedValue({ items: [], meta: { total: 0 } });
    wrapper = mountView({ type: 'cars', search: 'абв' });
    await flushPromises();
    getAccessibleAttachments.mockClear();

    await wrapper.find('[data-testid="aa-filter-reset"]').trigger('click');
    await flushPromises();

    const lastArg = getAccessibleAttachments.mock.calls.at(-1)[0];
    expect(lastArg).not.toHaveProperty('attachment_type');
    expect(lastArg).not.toHaveProperty('search');
    expect(lastArg).toMatchObject({ page: 1, per_page: 30 });
    expect(replace).toHaveBeenLastCalledWith({ query: {} });
  });

  it('кнопка сброса всегда в DOM: disabled без фильтров, активна с фильтром', async () => {
    getAccessibleAttachments.mockResolvedValue({ items: [], meta: { total: 0 } });

    wrapper = mountView();
    await flushPromises();
    // Кнопка не исчезает (раньше была через v-if и двигала ряд при появлении),
    // а просто блокируется, пока нет активных фильтров.
    const reset = wrapper.find('[data-testid="aa-filter-reset"]');
    expect(reset.exists()).toBe(true);
    expect(reset.attributes('disabled')).toBeDefined();
    wrapper.unmount();

    wrapper = mountView({ type: 'cars' });
    await flushPromises();
    expect(
      wrapper.find('[data-testid="aa-filter-reset"]').attributes('disabled'),
    ).toBeUndefined();
  });
});
