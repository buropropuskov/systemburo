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

vi.mock('@/api/attachment-templates', () => ({
  previewBlank: vi.fn(),
}));

const notify = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}));

// onMounted поднимает real-time подписку (#840 V3) - без мока реальный eventStream
// тянет api/client и уходит в fetchTicket/reconnect в тестовой среде.
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

// Роутер мокаем, чтобы тестировать URL-как-state без поднятия реального роутера:
// route.query - источник стартовых фильтров, router.replace - запись фильтров в URL.
const { routeState, replace } = vi.hoisted(() => ({
  routeState: { query: {} },
  replace: vi.fn(() => Promise.resolve()),
}));
// createRouter/createWebHistory нужны из-за цепочки импортов: список элементов
// вложения ходит в api/client, а тот тянет router (#1393).
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: routeState.query }),
  useRouter: () => ({ replace }),
  createRouter: () => ({ beforeEach: vi.fn(), afterEach: vi.fn(), push: vi.fn(), replace: vi.fn() }),
  createWebHistory: () => ({}),
}));

import AccessibleAttachmentsView from '@/views/AccessibleAttachmentsView.vue';
import eventStream from '@/services/eventStream';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import { getAccessibleAttachments, getAccessibleAttachmentDetail } from '@/api/applications';
import { getOrganizations, getCompanies } from '@/api/organizations';
import { previewBlank } from '@/api/attachment-templates';

const stubs = {
  AdminPageShell: { template: '<div><slot /></div>' },
  RefreshButton: { template: '<button class="refresh-stub" @click="$emit(\'refresh\')" />' },
  ApplicationAttachmentDetail: {
    props: ['attachment', 'cars', 'employees', 'items'],
    template: '<div class="detail-stub">{{ attachment.attachment_id }}</div>',
  },
  // BaseModal рендерит слот, только когда show=true - так проверяем содержимое превью.
  BaseModal: { props: ['show'], template: '<div v-if="show" class="modal-stub"><slot /></div>' },
  XlsxViewer: { props: ['fileBuffer'], template: '<div class="xlsx-stub" />' },
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

  it('карточка: организация (+ компания) в шапке, название отдельной строкой', async () => {
    getAccessibleAttachments.mockResolvedValue({
      items: [
        makeItem(1, { organization_name: 'ООО Ромашка', company_name: 'ЗАО Компания' }),
        // org == company (заявка без отдельной организации) - дубль не показываем.
        makeItem(2, { organization_name: 'ООО Один', company_name: 'ООО Один' }),
      ],
      meta: { total: 2 },
    });
    wrapper = mountView();
    await flushPromises();

    const cards = wrapper.findAll('[data-testid="aa-card"]');
    expect(cards[0].find('.attachment-card__org').text()).toBe('ООО Ромашка / ЗАО Компания');
    expect(cards[0].find('.attachment-card__name').text()).toContain('Вложение 1');
    expect(cards[1].find('.attachment-card__org').text()).toBe('ООО Один');
  });

  it('срок действия - в отдельном бейдже шапки, не в серой мете', async () => {
    getAccessibleAttachments.mockResolvedValue({
      items: [makeItem(1)],
      meta: { total: 1 },
    });
    wrapper = mountView();
    await flushPromises();

    const card = wrapper.find('[data-testid="aa-card"]');
    const dateBadge = card.find('[data-testid="aa-card-date"]');
    expect(dateBadge.exists()).toBe(true);
    expect(dateBadge.text()).toBe('01.06.2026 - 02.06.2026');
    // Дата ушла из меты (там остаются номер и отправитель) - не дублируем.
    expect(card.find('.attachment-card__meta').text()).not.toContain('01.06.2026');
  });

  // application_number реальных заявок уже начинается с «№» (DEMO-номера - без),
  // поэтому свой знак давал «Заявка № № 20260808/027» и в мете, и в шапке детали.
  it('номер заявки печатается как есть, без второго «№»', async () => {
    const number = '№ 20260808/027';
    getAccessibleAttachments.mockResolvedValue({
      items: [makeItem(1, { application_number: number })],
      meta: { total: 1 },
    });
    getAccessibleAttachmentDetail.mockResolvedValue({
      attachment: { ...makeItem(1, { application_number: number }), application_id: 42 },
      cars: [],
    });
    wrapper = mountView();
    await flushPromises();

    const meta = wrapper.find('.attachment-card__meta').text();
    expect(meta).toContain(number);
    expect(meta).not.toContain('№ № ');

    await wrapper.find('[data-testid="aa-card"]').trigger('click');
    await flushPromises();

    const title = wrapper.find('.application-block__title').text();
    expect(title).toContain(`Заявка ${number}`);
    expect(title).not.toContain('№ № ');
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

  it('«К списку» снимает выбор и гасит устаревший ответ детали', async () => {
    getAccessibleAttachments.mockResolvedValue({
      items: [makeItem(1)],
      meta: { total: 1, page: 1, per_page: 30 },
    });
    const resolvers = new Map();
    getAccessibleAttachmentDetail.mockImplementation(
      (id) => new Promise((resolve) => resolvers.set(id, resolve)),
    );

    wrapper = mountView();
    await flushPromises();
    notify.mockClear();

    await wrapper.find('[data-testid="aa-card"]').trigger('click'); // выбрали (ответ ещё в пути)
    expect(wrapper.find('[data-testid="aa-detail"]').exists()).toBe(true);

    await wrapper.find('[data-testid="aa-detail-back"]').trigger('click'); // назад к списку
    expect(wrapper.find('[data-testid="aa-detail"]').exists()).toBe(false);

    // Поздний ответ по уже снятому выбору не переоткрывает деталь и не шлёт error-toast.
    resolvers.get(1)({ attachment: makeItem(1), cars: [] });
    await flushPromises();
    expect(wrapper.find('[data-testid="aa-detail"]').exists()).toBe(false);
    expect(notify).not.toHaveBeenCalled();
  });
});

describe('AccessibleAttachmentsView (S4) предпросмотр бланка', () => {
  function mountWithDetail(detailOver = {}) {
    getAccessibleAttachments.mockResolvedValue({
      items: [makeItem(1)],
      meta: { total: 1, page: 1, per_page: 30 },
    });
    getAccessibleAttachmentDetail.mockResolvedValue({
      attachment: { ...makeItem(1), application_id: 42, attachment_id: 1, ...detailOver },
      cars: [],
    });
    return mountView();
  }

  async function openDetail() {
    await flushPromises();
    await wrapper.find('[data-testid="aa-card"]').trigger('click');
    await flushPromises();
  }

  it('кнопка превью видна по has_blank, открывает модалку и шлёт (app_id, att_id)', async () => {
    wrapper = mountWithDetail({ has_blank: true });
    previewBlank.mockResolvedValue(new ArrayBuffer(8));
    await openDetail();

    const btn = wrapper.find('[data-testid="aa-preview-blank"]');
    expect(btn.exists()).toBe(true);

    await btn.trigger('click');
    await flushPromises();

    // Третьим аргументом идёт режим документов: охране без пары прав
    // (detail.documents и detail.documents.export) бланк открывается с прочерками.
    expect(previewBlank).toHaveBeenCalledWith(42, 1, { withDocuments: false });
    expect(wrapper.find('[data-testid="aa-preview-viewer"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="aa-preview-loading"]').exists()).toBe(false);
  });

  it('без has_blank кнопки превью нет', async () => {
    wrapper = mountWithDetail({ has_blank: false });
    await openDetail();
    expect(wrapper.find('[data-testid="aa-preview-blank"]').exists()).toBe(false);
  });

  it('показывает ошибку, если бланк не загрузился', async () => {
    wrapper = mountWithDetail({ has_blank: true });
    previewBlank.mockRejectedValue(new Error('boom'));
    await openDetail();

    await wrapper.find('[data-testid="aa-preview-blank"]').trigger('click');
    await flushPromises();

    expect(wrapper.find('[data-testid="aa-preview-error"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="aa-preview-viewer"]').exists()).toBe(false);
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

  it('тоггл "Завершённые" шлёт completed, пишет URL и восстанавливается из query', async () => {
    getAccessibleAttachments.mockResolvedValue({ items: [], meta: { total: 0 } });
    wrapper = mountView();
    await flushPromises();
    getAccessibleAttachments.mockClear();

    await wrapper.find('[data-testid="aa-filter-completed"]').trigger('click');
    await flushPromises();

    expect(getAccessibleAttachments).toHaveBeenCalledWith(
      expect.objectContaining({ completed: true, page: 1 }),
    );
    expect(replace).toHaveBeenLastCalledWith({ query: { completed: '1' } });

    // Повторный клик снимает фильтр - completed уходит из параметров и URL.
    getAccessibleAttachments.mockClear();
    await wrapper.find('[data-testid="aa-filter-completed"]').trigger('click');
    await flushPromises();
    expect(getAccessibleAttachments.mock.calls.at(-1)[0]).not.toHaveProperty('completed');
    expect(replace).toHaveBeenLastCalledWith({ query: {} });

    wrapper.unmount();
    // Диплинк ?completed=1 восстанавливает фильтр одним стартовым запросом.
    getAccessibleAttachments.mockClear();
    wrapper = mountView({ completed: '1' });
    await flushPromises();
    expect(getAccessibleAttachments).toHaveBeenCalledWith(
      expect.objectContaining({ completed: true }),
    );
  });

  it('тоггл "Ночь" шлёт night, пишет URL и восстанавливается из query', async () => {
    getAccessibleAttachments.mockResolvedValue({ items: [], meta: { total: 0 } });
    wrapper = mountView();
    await flushPromises();
    getAccessibleAttachments.mockClear();

    await wrapper.find('[data-testid="aa-filter-night"]').trigger('click');
    await flushPromises();

    expect(getAccessibleAttachments).toHaveBeenCalledWith(
      expect.objectContaining({ night: true, page: 1 }),
    );
    expect(replace).toHaveBeenLastCalledWith({ query: { night: '1' } });

    // Повторный клик снимает фильтр - night уходит из параметров и URL.
    getAccessibleAttachments.mockClear();
    await wrapper.find('[data-testid="aa-filter-night"]').trigger('click');
    await flushPromises();
    expect(getAccessibleAttachments.mock.calls.at(-1)[0]).not.toHaveProperty('night');
    expect(replace).toHaveBeenLastCalledWith({ query: {} });

    wrapper.unmount();
    // Диплинк ?night=1 восстанавливает фильтр одним стартовым запросом.
    getAccessibleAttachments.mockClear();
    wrapper = mountView({ night: '1' });
    await flushPromises();
    expect(getAccessibleAttachments).toHaveBeenCalledWith(
      expect.objectContaining({ night: true }),
    );
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

describe('AccessibleAttachmentsView - real-time available.new (#840 V3)', () => {
  it('подписывается на available, рефетчит по сигналу и отписывается при unmount', async () => {
    getAccessibleAttachments.mockResolvedValue({ items: [], meta: { total: 0 } });
    const unsub = vi.fn();
    eventStream.subscribe.mockReturnValue(unsub);

    wrapper = mountView();
    await flushPromises();

    expect(eventStream.connect).toHaveBeenCalledTimes(1);
    expect(eventStream.subscribe).toHaveBeenCalledWith('available', expect.any(Function));

    const cb = eventStream.subscribe.mock.calls.find((c) => c[0] === 'available')[1];
    getAccessibleAttachments.mockClear();
    cb();
    await flushPromises();
    expect(getAccessibleAttachments).toHaveBeenCalled();

    wrapper.unmount();
    wrapper = null;
    expect(unsub).toHaveBeenCalledTimes(1);
    expect(eventStream.disconnect).toHaveBeenCalledTimes(1);
  });
});
