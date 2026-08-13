import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { nextTick } from 'vue';

// Фабрика vi.mock поднимается над импортами — управляемое состояние в hoisted.
const { state, notifySpy } = vi.hoisted(() => ({
  notifySpy: vi.fn(),
  state: { summary: {}, journal: [], journalPages: null, journalTotal: null, journalCalls: [], trendRows: [], trendCalls: [], trendDeferred: [], journalDeferred: [], stuck: [], stuckCalls: 0, reject: false, deferred: [], streamHandler: null },
}));

vi.mock('@/api/statistics.js', () => ({
  getProcessingSummary: () => {
    if (state.reject) return Promise.reject(new Error('boom'));
    if (state.hang) return new Promise((resolve) => { state.deferred.push(resolve); });
    return Promise.resolve(state.summary);
  },
  // Бэк отдаёт страницу {items, meta:{total}} (#1251 P5b). Форма ответа - как в
  // api/statistics.js: items из envelope.data, total из envelope.meta.
  getProcessingJournal: (from, to, limit, offset = 0, filter = {}) => {
    state.journalCalls.push({ from, to, limit, offset, ...filter });
    const page = (items) => ({
      items,
      meta: {
        total: state.journalTotal != null ? state.journalTotal : items.length,
        page: Math.floor(offset / (limit || 50)) + 1,
        per_page: limit || 50,
      },
    });
    if (state.hang) return new Promise(() => {}); // на первой загрузке лента тоже висит
    if (state.holdJournal) return new Promise((resolve) => { state.journalDeferred.push((items) => resolve(page(items))); });
    // journalPages задаёт постраничную выдачу (ключ - offset), иначе одна страница.
    return Promise.resolve(page(state.journalPages ? (state.journalPages[offset] || []) : state.journal));
  },
  // Зависшие согласования (#1315 S4): снимок текущих зависших заявок, от периода
  // не зависит. На первой загрузке при state.hang висит (скелетон таблицы).
  getStuckApprovals: () => {
    state.stuckCalls += 1;
    if (state.hang) return new Promise(() => {});
    return Promise.resolve(state.stuck);
  },
  // Динамика по дням строится отдельным запросом к движку отчётов (S/P3).
  runReport: (req) => {
    state.trendCalls.push(req);
    if (state.hang) return new Promise((resolve) => { state.trendDeferred.push(resolve); });
    return Promise.resolve({ metric_rows: state.trendRows });
  },
}));

// Стор уведомлений мокаем модулем: Pinia в этой спеке не поднимается.
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify: notifySpy }),
}));

// eventStream.subscribe запоминаем handler, чтобы имитировать SSE-сигнал в тесте.
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: () => {},
    disconnect: () => {},
    subscribe: (scope, handler) => {
      state.streamHandler = handler;
      return () => { state.streamHandler = null; };
    },
  },
}));

import ProcessingAnalytics from '../ProcessingAnalytics.vue';
import AnalyticsAreaChart from '../AnalyticsAreaChart.vue';
import FilterTabs from '@/components/ui/FilterTabs.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import HintTooltip from '@/components/ui/HintTooltip.vue';

const mountTab = () => mount(ProcessingAnalytics, {
  props: { from: '2026-06-01', to: '2026-06-07' },
  global: { stubs: { AnalyticsAreaChart: true } },
});

// В шапке две кнопки «Обновить»: у журнала и у зависших согласований (#1315 S4).
// Различаем по title, чтобы тесты журнала не цепляли соседнюю.
const journalRefreshBtn = (wrapper) =>
  wrapper.findAllComponents(RefreshButton).find((b) => b.attributes('title')?.includes('журнал'));

const fullSummary = () => ({
  from: '2026-06-01',
  to: '2026-06-07',
  total_applications: 42,
  stages: [
    {
      key: 'approval_time',
      label: 'Время согласования',
      samples: 40,
      avg: 8100, // 2 ч 15 мин
      p90: 18000, // 5 ч
      prev_avg: 9000,
      trend: { delta_pct: -10, direction: 'down', sentiment: 'good' },
    },
    {
      key: 'acceptance_time',
      label: 'Время принятия в работу',
      samples: 30,
      avg: 3600, // 1 ч
      p90: 7200,
      prev_avg: 3000,
      trend: { delta_pct: 20, direction: 'up', sentiment: 'bad' },
    },
    {
      key: 'processing_time',
      label: 'Время обработки',
      samples: 0,
      avg: null, // этап никто не прошёл
      p90: null,
      prev_avg: null,
    },
    {
      // Бэк по-прежнему отдаёт этап, но вкладка его скрывает (#1251 polish, п.6).
      key: 'completion_time',
      label: 'Время до завершения',
      samples: 1,
      avg: 5154628,
      p90: 5154628,
      prev_avg: null,
    },
  ],
  quality: [
    {
      // Ветки отказа приходят по отдельности (#1251 polish, п.8).
      key: 'rejected_rate',
      label: 'Доля отказов принимающего',
      unit: '%',
      value: 15,
      prev_value: 20,
      trend: { delta_pct: -25, direction: 'down', sentiment: 'good' },
    },
    {
      key: 'not_approved_rate',
      label: 'Доля несогласованных',
      unit: '%',
      value: 5,
      prev_value: 4,
      trend: { delta_pct: 25, direction: 'up', sentiment: 'bad' },
    },
    {
      key: 'avg_forwards',
      label: 'Среднее число пересылок',
      unit: 'раз/заявку',
      value: 1.3,
      prev_value: 1.1,
      trend: { delta_pct: 18.2, direction: 'up', sentiment: 'bad' },
    },
  ],
  // Полные рейтинги по скорости (S6): бэк отдаёт отсортированными (быстрые сверху,
  // без данных — в конце). Фронт не пересортировывает.
  approvers: [
    { name: 'Иванов И.И.', avg_response_time: 14400, votes_count: 12 },
    { name: 'Петров П.П.', avg_response_time: null, votes_count: 3 },
  ],
  acceptors: [
    { name: 'Сидоров С.С.', avg_acceptance_time: 7200, accepts_count: 5 },
    { name: 'Кузнецов К.К.', avg_acceptance_time: 10800, accepts_count: 2 },
  ],
  by_organization: [
    {
      label: 'ООО Ромашка', avg_approval_time: 8100, avg_acceptance_time: null,
      avg_processing_time: 10800, applications_count: 20,
    },
  ],
  by_company: [
    {
      label: 'АО Компания', avg_approval_time: 60, avg_acceptance_time: 120,
      avg_processing_time: 180, applications_count: 7,
    },
  ],
});

beforeEach(() => {
  state.summary = {};
  state.journal = [];
  state.journalPages = null;
  state.journalTotal = null;
  state.journalCalls = [];
  state.trendRows = [];
  state.trendCalls = [];
  state.trendDeferred = [];
  state.journalDeferred = [];
  state.stuck = [];
  state.stuckCalls = 0;
  state.holdJournal = false;
  state.reject = false;
  state.hang = false;
  state.deferred = [];
  state.streamHandler = null;
  notifySpy.mockClear();
});

// application_number приходит от бэка УЖЕ с префиксом «№ » (COALESCE(app.application_number)),
// фронт его не дублирует.
const journalEntries = () => [
  { application_id: 5, application_number: '№ 20260710/001', actor_name: 'Иванов И.И.', role: 'acceptance', occurred_at: '2026-06-07T09:30:00Z', working_seconds: 3600 },
  { application_id: 5, application_number: '№ 20260710/001', actor_name: 'Петров П.П.', role: 'approval', occurred_at: '2026-06-07T08:00:00Z', working_seconds: null },
];

describe('ProcessingAnalytics — KPI этапов', () => {
  it('рисует длительность этапа человекочитаемо, а не сырые секунды', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const text = wrapper.text();
    expect(text).toContain('Время согласования');
    expect(text).toContain('2 ч 15 мин'); // 8100 с
    // p90 без жаргона: «9 из 10 — до X» вместо «p90: X» (#1251 polish, п.5).
    expect(text).toContain('9 из 10 — до 5 ч'); // 18000 с
    expect(text).not.toContain('p90');
    expect(text).not.toContain('8100');
  });

  it('подписывает крупное число как среднее — иначе непонятно, среднее это или максимум', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const approval = wrapper.findAll('.proc__tile').find((t) => t.text().includes('Время согласования'));
    expect(approval.find('.proc__tile-agg').text()).toBe('среднее');
  });

  it('«Время до завершения» с вкладки убрано (срок пропуска, а не работа бюро)', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    // Бэк этап отдаёт, но вкладка его не рисует и в график не кладёт.
    expect(wrapper.text()).not.toContain('Время до завершения');
    expect(wrapper.findAll('.proc__tile').length).toBe(3 + 3); // 3 видимых этапа + 3 метрики качества
  });

  it('этап без выборки показывает прочерк, а не «0 с»', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const tile = wrapper.findAll('.proc__tile').find((t) => t.text().includes('Время обработки'));
    expect(tile).toBeTruthy();
    expect(tile.find('.proc__tile-val').text()).toContain('—');
    expect(tile.text()).not.toContain('0 с');
  });

  it('дельту красит по тональности, а не по направлению (стрелка вниз может быть зелёной)', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    // Время согласования упало (direction=down) — это улучшение (sentiment=good) -> зелёный.
    const approval = wrapper.findAll('.proc__tile').find((t) => t.text().includes('Время согласования'));
    const delta = approval.find('.proc__delta');
    expect(delta.classes()).toContain('proc__delta--good');
    expect(delta.classes()).not.toContain('proc__delta--down');
    expect(delta.text()).toContain('-10%');

    // Принятие выросло (up) — ухудшение (bad) -> красный.
    const acceptance = wrapper.findAll('.proc__tile').find((t) => t.text().includes('Время принятия'));
    expect(acceptance.find('.proc__delta').classes()).toContain('proc__delta--bad');
  });
});

describe('ProcessingAnalytics — тултипы и основа времени (S5)', () => {
  it('подсказка этапа — teleport-компонент (не ::after, который резали контейнеры со скроллом)', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const approval = wrapper.findAll('.proc__tile').find((t) => t.text().includes('Время согласования'));
    const hint = approval.findComponent(HintTooltip);
    expect(hint.exists()).toBe(true);
    expect(hint.props('text')).toContain('рабочему времени бюро');
    // И объясняет, что за числа на плитке (среднее vs 9 из 10).
    expect(hint.props('text')).toContain('среднее');
    expect(hint.props('text')).toContain('9 заявок из 10');
  });

  it('помечает основу расчёта: у видимых этапов — рабочее время бюро', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const approval = wrapper.findAll('.proc__tile').find((t) => t.text().includes('Время согласования'));
    const workBadge = approval.find('.proc__basis');
    expect(workBadge.classes()).toContain('proc__basis--work');
    expect(workBadge.text()).toContain('раб. время');

    // Единственный календарный этап (время до завершения) с вкладки убран.
    expect(wrapper.find('.proc__basis--calendar').exists()).toBe(false);
  });

  it('у метрики качества тоже есть подсказка «что считается»', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const refusal = wrapper.findAll('.proc__tile').find((t) => t.text().includes('Доля отказов принимающего'));
    const hint = refusal.findComponent(HintTooltip);
    expect(hint.exists()).toBe(true);
    expect(hint.props('text')).toContain('отказ');
  });
});

describe('ProcessingAnalytics — качество', () => {
  it('долю отказов показывает процентом, число пересылок — с единицей', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const refusal = wrapper.findAll('.proc__tile').find((t) => t.text().includes('Доля отказов принимающего'));
    expect(refusal.find('.proc__tile-val').text()).toBe('15%');

    const notApproved = wrapper.findAll('.proc__tile').find((t) => t.text().includes('Доля несогласованных'));
    expect(notApproved.find('.proc__tile-val').text()).toBe('5%');

    const forwards = wrapper.findAll('.proc__tile').find((t) => t.text().includes('Среднее число пересылок'));
    expect(forwards.find('.proc__tile-val').text()).toBe('1,3'); // ru-RU десятичная запятая
    expect(forwards.text()).toContain('раз/заявку');
  });
});

describe('ProcessingAnalytics — динамика (P3)', () => {
  it('строит серию выбранного этапа; день с заявками, но без прошедшего этапа = разрыв, а не ноль', async () => {
    state.summary = fullSummary();
    // Реальная форма ответа движка: бины анкерятся объединением строк ВСЕХ
    // запрошенных метрик. День 02.06 заявки имел, но этап не прошла ни одна ->
    // ключа длительности в values нет (metricOmitsFakeZero), спутник есть.
    state.trendRows = [
      { label: '2026-06-01', values: { avg_processing_time: 8100, applications_count: 4 } },
      { label: '2026-06-02', values: { applications_count: 2 } },
      { label: '2026-06-03', values: { avg_processing_time: 3600, applications_count: 3 } },
    ];
    const wrapper = mountTab();
    await flushPromises();

    const chart = wrapper.findComponent(AnalyticsAreaChart);
    expect(chart.props('valueType')).toBe('duration');
    expect(chart.props('data')).toEqual([
      { timestamp: '2026-06-01', count: 8100 },
      { timestamp: '2026-06-02', count: null }, // разрыв, не 0
      { timestamp: '2026-06-03', count: 3600 },
    ]);
  });

  it('запрашивает спутника-анкер, явный лимит и шаг по длине окна', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const req = state.trendCalls[0];
    // Спутник нужен, чтобы день без прошедшего этапа вообще пришёл в ответе.
    expect(req.metrics).toEqual(['avg_processing_time', 'applications_count']);
    expect(req.dimension).toBe('period');
    // Окно теста - неделя, значит шаг дневной.
    expect(req.granularity).toBe('day');
    // Без явного лимита движок обрезал бы период до 100 первых бинов.
    expect(req.limit).toBeGreaterThanOrEqual(365);
    expect(wrapper.text()).toContain('по дням');
  });

  it('на длинном окне шаг укрупняется, чтобы период не обрезался лимитом', async () => {
    state.summary = fullSummary();
    const wrapper = mount(ProcessingAnalytics, {
      props: { from: '2025-01-01', to: '2026-07-19' }, // ~1.5 года
      global: { stubs: { AnalyticsAreaChart: true } },
    });
    await flushPromises();

    expect(state.trendCalls.at(-1).granularity).toBe('week');
    expect(wrapper.text()).toContain('по неделям');
  });

  it('переключение этапа перестраивает график (раньше вид был один и без выбора)', async () => {
    state.summary = fullSummary();
    state.trendRows = [{ label: '2026-06-01', values: { avg_approval_time: 100 } }];
    const wrapper = mountTab();
    await flushPromises();
    const before = state.trendCalls.length;

    await wrapper.findComponent(FilterTabs).vm.$emit('update:modelValue', 'approval_time');
    await flushPromises();

    expect(state.trendCalls.length).toBe(before + 1);
    expect(state.trendCalls.at(-1).metrics[0]).toBe('avg_approval_time');
    expect(wrapper.findComponent(AnalyticsAreaChart).props('seriesName')).toBe('Согласование');
  });
});

describe('ProcessingAnalytics — рейтинги (S6)', () => {
  it('согласующие — полный рейтинг по скорости с рангом; без ответов — прочерк времени, но реальная нагрузка', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    // Первая таблица — рейтинг согласующих; Иванов ранг 1 (порядок задаёт бэк).
    const approversTable = wrapper.findAll('.proc__table')[0];
    const rows = approversTable.findAll('tbody tr');
    expect(rows[0].find('.proc__rank').text()).toBe('1');
    expect(rows[0].text()).toContain('Иванов И.И.');
    expect(rows[0].findAll('.proc__num')[0].text()).toBe('4 ч'); // 14400 с

    // Петров без времени реакции — прочерк, нагрузка 3.
    const petrov = rows.find((r) => r.text().includes('Петров'));
    expect(petrov.findAll('.proc__num')[0].text()).toBe('—');
    expect(petrov.findAll('.proc__num')[1].text()).toBe('3');
  });

  it('принимающие — отдельная таблица рейтинга по скорости принятия', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    expect(wrapper.text()).toContain('Принимающие');
    // Вторая таблица — рейтинг принимающих.
    const acceptorsTable = wrapper.findAll('.proc__table')[1];
    const rows = acceptorsTable.findAll('tbody tr');
    expect(rows[0].find('.proc__rank').text()).toBe('1');
    expect(rows[0].text()).toContain('Сидоров С.С.');
    expect(rows[0].findAll('.proc__num')[0].text()).toBe('2 ч'); // 7200 с — время принятия
    expect(rows[0].findAll('.proc__num')[1].text()).toBe('5'); // принято

    expect(rows[1].text()).toContain('Кузнецов К.К.');
    expect(rows[1].findAll('.proc__num')[0].text()).toBe('3 ч'); // 10800 с
  });

  it('разбивка несёт все этапы, а не только общее время; прочерк вместо нуля', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const rows = wrapper.findAll('.proc__table')[2].findAll('tbody tr');
    const cells = rows[0].findAll('.proc__num').map((c) => c.text());
    expect(rows[0].text()).toContain('ООО Ромашка');
    expect(cells).toEqual(['2 ч 15 мин', '—', '3 ч', '20']); // согласование / принятие / обработка / заявок
  });

  it('переключает разрез организации <-> компании одной таблицей', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    // Ищем переключатель по составу вкладок, а не по позиции: FilterTabs на вкладке
    // несколько (этап динамики, разрез разбивки, роль журнала).
    const breakdownTabs = wrapper.findAllComponents(FilterTabs)
      .find((t) => t.props('tabs').some((tab) => tab.key === 'organization'));
    await breakdownTabs.vm.$emit('update:modelValue', 'company');
    await flushPromises();

    const table = wrapper.findAll('.proc__table')[2];
    expect(table.find('thead').text()).toContain('Компания');
    expect(table.findAll('tbody tr')[0].text()).toContain('АО Компания');
  });
});

describe('ProcessingAnalytics — зависшие согласования (S4)', () => {
  const stuckRows = () => ([
    { application_id: 11, application_number: '№ 20260701/002', approver_name: 'Иванов И.И.', waiting_days: 5, reminder_count: 2, last_reminder_at: '2026-06-06T09:00:00Z' },
    { application_id: 12, application_number: '№ 20260703/004', approver_name: 'Петров П.П.', waiting_days: 1, reminder_count: 0, last_reminder_at: null },
  ]);

  it('рисует строки зависших: согласующий, дни ожидания и число напоминаний', async () => {
    state.summary = fullSummary();
    state.stuck = stuckRows();
    const wrapper = mountTab();
    await flushPromises();

    expect(wrapper.text()).toContain('Зависшие согласования');
    // Таблица зависших идёт после рейтингов и разбивки (индекс 3 среди .proc__table).
    const table = wrapper.findAll('.proc__table')[3];
    const rows = table.findAll('tbody tr');
    expect(rows).toHaveLength(2);

    expect(rows[0].text()).toContain('20260701/002');
    expect(rows[0].text()).toContain('Иванов И.И.');
    expect(rows[0].findAll('.proc__num').map((c) => c.text())).toEqual(['5 дней', '2']);
    // Единственное число дней склоняется корректно.
    expect(rows[1].findAll('.proc__num').map((c) => c.text())).toEqual(['1 день', '0']);
  });

  it('пустой список — дружелюбное «Зависших согласований нет», не «Нет данных»', async () => {
    state.summary = fullSummary();
    state.stuck = [];
    const wrapper = mountTab();
    await flushPromises();

    const table = wrapper.findAll('.proc__table')[3];
    expect(table.text()).toContain('Зависших согласований нет');
  });

  it('снимок живой: SSE-сигнал перечитывает список зависших', async () => {
    state.summary = fullSummary();
    state.stuck = stuckRows();
    const wrapper = mountTab();
    await flushPromises();
    expect(wrapper.findAll('.proc__table')[3].findAll('tbody tr')).toHaveLength(2);

    // Согласующий проголосовал -> заявка ушла из зависших -> сигнал -> рефетч.
    state.stuck = [stuckRows()[0]];
    state.streamHandler();
    await flushPromises();
    expect(wrapper.findAll('.proc__table')[3].findAll('tbody tr')).toHaveLength(1);
  });
});

describe('ProcessingAnalytics — журнал (S7)', () => {
  it('рендерит события ленты: роль, актор, заявка, рабочая длительность', async () => {
    state.summary = fullSummary();
    state.journal = journalEntries();
    const wrapper = mountTab();
    await flushPromises();

    const rows = wrapper.findAll('.proc__journal-row');
    expect(rows).toHaveLength(2);

    // Первое событие — принятие (роль «Принятие»), актор, заявка, длительность 1 ч.
    expect(rows[0].find('.proc__journal-role').text()).toBe('Принятие');
    expect(rows[0].find('.proc__journal-role').classes()).toContain('proc__journal-role--acceptance');
    expect(rows[0].find('.proc__journal-actor').text()).toBe('Иванов И.И.');
    // Номер как есть от бэка, без дублирования префикса «№».
    expect(rows[0].find('.proc__journal-app').text()).toBe('№ 20260710/001');
    expect(rows[0].find('.proc__journal-dur').text()).toBe('1 ч'); // 3600 с

    // Время абсолютное: «5 дн назад» не давало понять, когда именно (#1251 polish, п.12).
    expect(rows[0].find('.proc__journal-when').text()).toMatch(/^\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}$/);

    // Второе — согласование без рабочей длительности (null -> пусто, не «—»).
    expect(rows[1].find('.proc__journal-role').text()).toBe('Согласование');
    expect(rows[1].find('.proc__journal-role').classes()).toContain('proc__journal-role--approval');
    expect(rows[1].find('.proc__journal-dur').text()).toBe('');
  });

  // #1251 P7: до этого среза отрицательный голос приезжал ролью 'approval' и лента
  // показывала его как «Согласование», а отказов и отзывов в ней не было вовсе.
  it('различает несогласование, отказ принимающего и отзыв инициатором', async () => {
    state.summary = fullSummary();
    state.journal = [
      { application_id: 7, application_number: '№ 20260710/007', actor_name: 'Согласуев С.С.', role: 'not_approved', occurred_at: '2026-06-07T09:00:00Z', working_seconds: 3600 },
      { application_id: 8, application_number: '№ 20260710/008', actor_name: 'Принимаев П.П.', role: 'rejection', occurred_at: '2026-06-07T08:30:00Z', working_seconds: 1800 },
      { application_id: 9, application_number: '№ 20260710/009', actor_name: 'Отправителев О.О.', role: 'withdrawal', occurred_at: '2026-06-07T08:00:00Z', working_seconds: null },
    ];
    const wrapper = mountTab();
    await flushPromises();

    const badges = wrapper.findAll('.proc__journal-row').map((r) => r.find('.proc__journal-role'));
    expect(badges.map((b) => b.text())).toEqual(['Несогласование', 'Отказ', 'Отзыв']);
    expect(badges[0].classes()).toContain('proc__journal-role--not_approved');
    expect(badges[1].classes()).toContain('proc__journal-role--rejection');
    expect(badges[2].classes()).toContain('proc__journal-role--withdrawal');

    // На отзыв инициатором рабочее время Бюро не тратится - ячейка пустая, не «0 с».
    expect(wrapper.findAll('.proc__journal-row')[2].find('.proc__journal-dur').text()).toBe('');
  });

  it('пустой журнал показывает заглушку, а не строки', async () => {
    state.summary = fullSummary();
    state.journal = [];
    const wrapper = mountTab();
    await flushPromises();

    expect(wrapper.findAll('.proc__journal-row')).toHaveLength(0);
    expect(wrapper.text()).toContain('Событий за период нет');
  });

  it('журнал виден даже когда за период не подали новых заявок (окно по времени события)', async () => {
    // total_applications=0 (нет подач), но в журнале есть события (согласовали/приняли
    // заявки, поданные раньше) — лента не должна скрываться заглушкой «не подавали».
    state.summary = { from: '2026-06-01', to: '2026-06-07', total_applications: 0, stages: [], quality: [], approvers: [], acceptors: [], by_organization: [], by_company: [] };
    state.journal = journalEntries();
    const wrapper = mountTab();
    await flushPromises();

    expect(wrapper.text()).toContain('заявок не подавали'); // заглушка бандла на месте
    // Но журнал всё равно отрисован со своими событиями.
    expect(wrapper.text()).toContain('Журнал');
    expect(wrapper.findAll('.proc__journal-row')).toHaveLength(2);
  });

  it('номер заявки копируется в буфер и подтверждается уведомлением', async () => {
    const writeText = vi.fn().mockResolvedValue();
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });

    state.summary = fullSummary();
    state.journal = journalEntries();
    const wrapper = mountTab();
    await flushPromises();

    await wrapper.findAll('.proc__journal-app--copy')[0].trigger('click');
    await flushPromises();

    expect(writeText).toHaveBeenCalledWith('№ 20260710/001');
    expect(notifySpy).toHaveBeenCalledWith(
      expect.objectContaining({ bold: '№ 20260710/001', type: 'success' }),
    );
  });

  it('кнопка обновления перечитывает ленту руками', async () => {
    state.summary = fullSummary();
    state.journal = journalEntries();
    const wrapper = mountTab();
    await flushPromises();
    expect(wrapper.findAll('.proc__journal-row')).toHaveLength(2);

    state.journal = [journalEntries()[0]];
    await journalRefreshBtn(wrapper).vm.$emit('refresh');
    await flushPromises();

    expect(wrapper.findAll('.proc__journal-row')).toHaveLength(1);
  });

  it('на время обновления кнопка показывает загрузку и отпускает её после ответа', async () => {
    state.summary = fullSummary();
    state.journal = journalEntries();
    const wrapper = mountTab();
    await flushPromises();

    const btn = journalRefreshBtn(wrapper);
    expect(btn.props('loading')).toBe(false);

    state.holdJournal = true;
    btn.vm.$emit('refresh');
    await flushPromises();
    expect(btn.props('loading')).toBe(true); // запрос висит

    state.journalDeferred[0](journalEntries());
    await flushPromises();
    expect(btn.props('loading')).toBe(false); // отпустили
  });

  it('событие без номера заявки показывает прочерк, а не кнопку копирования', async () => {
    state.summary = fullSummary();
    state.journal = [{ ...journalEntries()[0], application_number: '' }];
    const wrapper = mountTab();
    await flushPromises();

    const row = wrapper.findAll('.proc__journal-row')[0];
    expect(row.find('.proc__journal-app--copy').exists()).toBe(false);
    expect(row.find('.proc__journal-app').text()).toBe('—');
  });

  it('обновляет ленту по SSE-сигналу applications-center (real-time)', async () => {
    state.summary = fullSummary();
    state.journal = journalEntries();
    const wrapper = mountTab();
    await flushPromises();
    expect(wrapper.findAll('.proc__journal-row')).toHaveLength(2);

    // Пришло новое событие -> сигнал -> рефетч ленты.
    expect(typeof state.streamHandler).toBe('function');
    state.journal = [
      { application_id: 9, application_number: '№ 20260711/003', actor_name: 'Сидоров С.С.', role: 'acceptance', occurred_at: '2026-06-07T10:00:00Z', working_seconds: 1800 },
      ...journalEntries(),
    ];
    state.streamHandler();
    await flushPromises();

    const rows = wrapper.findAll('.proc__journal-row');
    expect(rows).toHaveLength(3);
    expect(rows[0].text()).toContain('20260711/003'); // свежее сверху
  });
});

describe('ProcessingAnalytics — страницы журнала (P5b)', () => {
  // Страницы по 50: ключ journalPages — offset, который должен уйти на бэк.
  const secondPage = () => [
    { application_id: 12, application_number: '№ 20260601/077', actor_name: 'Сидоров С.С.', role: 'approval', occurred_at: '2026-06-02T08:00:00Z', working_seconds: 600 },
  ];

  const mountWithPages = async () => {
    state.summary = fullSummary();
    state.journalPages = { 0: journalEntries(), 50: secondPage() };
    state.journalTotal = 51;
    const wrapper = mountTab();
    await flushPromises();
    return wrapper;
  };

  it('показывает общее число событий и номер страницы, первую грузит без смещения', async () => {
    const wrapper = await mountWithPages();

    expect(wrapper.find('.pager__total').text()).toBe('Всего: 51');
    expect(wrapper.find('.pager__page').text()).toBe('1 / 2');
    expect(state.journalCalls[0]).toMatchObject({ limit: 50, offset: 0 });
  });

  it('«Вперёд» листает на следующую страницу и запрашивает её со смещением', async () => {
    const wrapper = await mountWithPages();
    const [back, forward] = wrapper.findAll('.pager__btn');
    expect(back.attributes('disabled')).toBeDefined(); // на первой странице назад некуда

    await forward.trigger('click');
    await flushPromises();

    expect(state.journalCalls.at(-1)).toMatchObject({ offset: 50 });
    expect(wrapper.find('.pager__page').text()).toBe('2 / 2');
    const rows = wrapper.findAll('.proc__journal-row');
    expect(rows).toHaveLength(1);
    expect(rows[0].text()).toContain('20260601/077');
    // На последней странице вперёд некуда, назад — можно.
    expect(wrapper.findAll('.pager__btn')[1].attributes('disabled')).toBeDefined();
    expect(wrapper.findAll('.pager__btn')[0].attributes('disabled')).toBeUndefined();
  });

  it('автообновление держит текущую страницу, а смена периода возвращает на первую', async () => {
    const wrapper = await mountWithPages();
    await wrapper.findAll('.pager__btn')[1].trigger('click');
    await flushPromises();
    expect(wrapper.find('.pager__page').text()).toBe('2 / 2');

    // SSE-сигнал перечитывает ТУ ЖЕ страницу: читающего не выбрасывает наверх.
    state.streamHandler();
    await flushPromises();
    expect(state.journalCalls.at(-1)).toMatchObject({ offset: 50 });
    expect(wrapper.find('.pager__page').text()).toBe('2 / 2');

    // Новый период — новая лента с первой страницы.
    await wrapper.setProps({ from: '2026-05-01', to: '2026-05-31' });
    await flushPromises();
    expect(state.journalCalls.at(-1)).toMatchObject({ from: '2026-05-01', offset: 0 });
    expect(wrapper.find('.pager__page').text()).toBe('1 / 2');
  });

  it('если событий стало меньше, страница за хвостом схлопывается к последней', async () => {
    const wrapper = await mountWithPages();
    await wrapper.findAll('.pager__btn')[1].trigger('click');
    await flushPromises();
    expect(wrapper.find('.pager__page').text()).toBe('2 / 2');

    // Лента усохла до одной страницы — вторая больше не существует.
    state.journalPages = { 0: journalEntries(), 50: [] };
    state.journalTotal = 2;
    state.streamHandler();
    await flushPromises();

    expect(wrapper.find('.pager__page').text()).toBe('1 / 1');
    expect(wrapper.findAll('.proc__journal-row')).toHaveLength(2);
  });

  it('без событий пейджер не показывается', async () => {
    state.summary = fullSummary();
    state.journal = [];
    const wrapper = mountTab();
    await flushPromises();

    expect(wrapper.find('.proc__journal-pager').exists()).toBe(false);
    expect(wrapper.find('.proc__journal .proc__table-empty').text()).toContain('Событий за период нет');
  });
});

describe('ProcessingAnalytics — шапка ленты (P6)', () => {
  it('над лентой стоит строка заголовков колонок', async () => {
    state.summary = fullSummary();
    state.journal = journalEntries();
    const wrapper = mountTab();
    await flushPromises();

    const head = wrapper.find('.proc__journal-head');
    expect(head.exists()).toBe(true);
    expect(head.text()).toContain('Событие');
    expect(head.text()).toContain('Кто');
    expect(head.text()).toContain('Заявка');
    expect(head.text()).toContain('Когда');
    // Шапка НЕ должна попадать в выборку событий - иначе счётчики строк врут.
    expect(wrapper.findAll('.proc__journal-row')).toHaveLength(2);
  });

  it('заголовки ленты остаются и когда событий нет', async () => {
    state.summary = fullSummary();
    state.journal = [];
    const wrapper = mountTab();
    await flushPromises();

    expect(wrapper.find('.proc__journal-head').exists()).toBe(true);
    expect(wrapper.findAll('.proc__journal-row')).toHaveLength(0);
  });
});

describe('ProcessingAnalytics — фильтры журнала (P5c)', () => {
  const mountFiltered = async () => {
    state.summary = fullSummary();
    state.journalPages = { 0: journalEntries(), 50: [] };
    state.journalTotal = 51;
    const wrapper = mountTab();
    await flushPromises();
    return wrapper;
  };

  // Роли лежат вкладками FilterTabs: ключ '' — «Все», остальные уходят на бэк as-is.
  const roleTab = (wrapper, key) => wrapper.find(`[data-testid="filter-tab-${key}"]`);

  it('фильтр роли уходит на бэк и возвращает ленту на первую страницу', async () => {
    const wrapper = await mountFiltered();
    await wrapper.findAll('.pager__btn')[1].trigger('click');
    await flushPromises();
    expect(wrapper.find('.pager__page').text()).toBe('2 / 2');

    await roleTab(wrapper, 'approval').trigger('click');
    await flushPromises();

    expect(state.journalCalls.at(-1)).toMatchObject({ role: 'approval', offset: 0 });
    expect(wrapper.find('.pager__page').text()).toBe('1 / 2');

    // «Все» снимает фильтр, а не шлёт своё значение (бэк на неизвестную роль даёт 400).
    await roleTab(wrapper, '').trigger('click');
    await flushPromises();
    expect(state.journalCalls.at(-1).role).toBe('');
  });

  // Ключи вкладок = роли бэка (models.ProcessingJournalRoles): на чужое значение
  // эндпоинт отвечает 400, поэтому каждая новая вкладка обязана доехать as-is.
  it('вкладки покрывают все роли ленты и уходят на бэк своим ключом', async () => {
    const wrapper = await mountFiltered();

    for (const role of ['not_approved', 'rejection', 'withdrawal']) {
      const tab = roleTab(wrapper, role);
      expect(tab.exists()).toBe(true);
      await tab.trigger('click');
      await flushPromises();
      expect(state.journalCalls.at(-1)).toMatchObject({ role, offset: 0 });
    }
  });

  it('поиск уходит одним запросом после паузы в наборе, а не на каждый символ', async () => {
    vi.useFakeTimers();
    try {
      const wrapper = await mountFiltered();
      const callsBefore = state.journalCalls.length;
      const input = wrapper.find('.proc__journal-search input');

      await input.setValue('Куз');
      await input.setValue('Кузнецов');
      expect(state.journalCalls).toHaveLength(callsBefore); // до паузы бэк не дёргаем

      vi.advanceTimersByTime(300);
      await flushPromises();

      expect(state.journalCalls).toHaveLength(callsBefore + 1);
      expect(state.journalCalls.at(-1)).toMatchObject({ q: 'Кузнецов', offset: 0 });
    } finally {
      vi.useRealTimers();
    }
  });

  it('свой диапазон дат сужает ленту, не трогая период вкладки', async () => {
    const wrapper = await mountFiltered();
    const dateFilter = wrapper.findComponent({ name: 'DateFilter' });

    dateFilter.vm.$emit('update:date-range-start', new Date(2026, 5, 3));
    dateFilter.vm.$emit('update:date-range-end', new Date(2026, 5, 4));
    dateFilter.vm.$emit('apply');
    await flushPromises();

    // Локальные части даты, не toISOString: иначе 03.06 уехало бы на 02.06.
    expect(state.journalCalls.at(-1)).toMatchObject({ from: '2026-06-03', to: '2026-06-04' });
    // Сводка и график остались на периоде вкладки — их запросы к датам журнала не привязаны.
    expect(state.trendCalls.at(-1).filters[0]).toMatchObject({ from: '2026-06-01', to: '2026-06-07' });
  });

  it('«Сбросить» снимает все фильтры одним запросом', async () => {
    const wrapper = await mountFiltered();
    await roleTab(wrapper, 'acceptance').trigger('click');
    await flushPromises();
    await wrapper.find('.proc__journal-search input').setValue('Иванов');
    const dateFilter = wrapper.findComponent({ name: 'DateFilter' });
    dateFilter.vm.$emit('update:date-range-start', new Date(2026, 5, 3));
    dateFilter.vm.$emit('apply');
    await flushPromises();

    const reset = wrapper.find('.proc__journal-reset');
    expect(reset.attributes('disabled')).toBeUndefined();
    const callsBefore = state.journalCalls.length;
    await reset.trigger('click');
    await flushPromises();

    expect(state.journalCalls).toHaveLength(callsBefore + 1); // одно обращение на сброс
    expect(state.journalCalls.at(-1)).toMatchObject({
      role: '', q: '', from: '2026-06-01', to: '2026-06-07', offset: 0,
    });
    expect(wrapper.find('.proc__journal-reset').attributes('disabled')).toBeDefined();
  });

  it('смена периода вкладки снимает свой диапазон журнала', async () => {
    const wrapper = await mountFiltered();
    const dateFilter = wrapper.findComponent({ name: 'DateFilter' });
    dateFilter.vm.$emit('update:date-range-start', new Date(2026, 5, 3));
    dateFilter.vm.$emit('update:date-range-end', new Date(2026, 5, 4));
    dateFilter.vm.$emit('apply');
    await flushPromises();

    await wrapper.setProps({ from: '2026-05-01', to: '2026-05-31' });
    await flushPromises();

    expect(state.journalCalls.at(-1)).toMatchObject({ from: '2026-05-01', to: '2026-05-31' });
  });

  it('под фильтрами пустая лента объясняет, что дело в фильтрах', async () => {
    state.summary = fullSummary();
    state.journal = [];
    const wrapper = mountTab();
    await flushPromises();
    expect(wrapper.find('.proc__journal .proc__table-empty').text()).toContain('Событий за период нет');

    await roleTab(wrapper, 'approval').trigger('click');
    await flushPromises();

    expect(wrapper.find('.proc__journal .proc__table-empty').text()).toContain('По фильтрам ничего не нашлось');
  });
});

describe('ProcessingAnalytics — крайние состояния', () => {
  it('пустой период показывает заглушку, а не нулевые метрики', async () => {
    state.summary = { from: '2026-06-01', to: '2026-06-07', total_applications: 0, stages: [], quality: [], approvers: [], acceptors: [], by_organization: [], by_company: [] };
    const wrapper = mountTab();
    await flushPromises();

    expect(wrapper.text()).toContain('заявок не подавали');
    expect(wrapper.find('.proc__tiles').exists()).toBe(false);
  });

  it('на первой загрузке показывает скелетоны во всех секциях, а не «Нет данных»', async () => {
    state.hang = true;
    const wrapper = mountTab();
    await nextTick(); // onMounted -> loadSummary висит (loading, !ready)

    expect(wrapper.findAll('.proc__tile--skeleton').length).toBeGreaterThan(0);
    expect(wrapper.find('.proc__skeleton--chart').exists()).toBe(true);
    expect(wrapper.findAll('.proc__skeleton--table').length).toBe(5); // согласующие + принимающие + организации + журнал + зависшие
    // График/таблицы не должны флэшить пустоту до прихода ответа.
    expect(wrapper.text()).not.toContain('Нет данных');

    // Ответ пришёл -> скелетоны уходят, данные на месте.
    state.deferred[0](fullSummary());
    state.trendDeferred[0]({ metric_rows: [] }); // график ждёт свой запрос отдельно
    await flushPromises();
    expect(wrapper.find('.proc__skeleton--chart').exists()).toBe(false);
    expect(wrapper.text()).toContain('Время согласования');
  });

  it('ошибка загрузки показывает сообщение', async () => {
    state.reject = true;
    const wrapper = mountTab();
    await flushPromises();

    expect(wrapper.find('.proc__state--error').exists()).toBe(true);
    expect(wrapper.text()).toContain('boom');
  });

  it('устаревший ответ не затирает актуальный (seq-guard при смене периода)', async () => {
    state.hang = true;
    const wrapper = mountTab();
    await nextTick(); // onMounted -> loadSummary (seq 1, зависший)

    // Смена периода -> второй loadSummary (seq 2).
    await wrapper.setProps({ from: '2026-05-01', to: '2026-05-07' });
    await nextTick();
    expect(state.deferred).toHaveLength(2);

    // Актуальный seq 2 приходит первым, устаревший seq 1 — позже с другими данными.
    state.deferred[1](fullSummary());
    await flushPromises();
    state.deferred[0]({ total_applications: 999, stages: [], quality: [], approvers: [], acceptors: [], by_organization: [] });
    await flushPromises();

    // На экране — данные seq 2 (42 заявки), а не устаревшие seq 1 (999).
    expect(wrapper.text()).toContain('42 заявки за период');
    expect(wrapper.text()).not.toContain('999');
  });
});

describe('ProcessingAnalytics — журнал на телефоне (#1097 w8)', () => {
  const origMatchMedia = window.matchMedia;

  /** Тот же приём, что в StatisticsDashboardFeedLimit.spec.js. */
  function mockNarrowViewport(matches) {
    window.matchMedia = vi.fn().mockImplementation((query) => ({
      matches,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
  }

  afterEach(() => {
    window.matchMedia = origMatchMedia;
  });

  it('на десктопе остаются табы роли, дропдаун не рендерится', async () => {
    mockNarrowViewport(false);
    state.summary = fullSummary();
    state.journal = journalEntries();
    const wrapper = mountTab();
    await flushPromises();

    expect(wrapper.find('[data-testid="filter-tab-approval"]').exists()).toBe(true);
    expect(wrapper.findComponent({ name: 'BaseDropdown' }).exists()).toBe(false);
  });

  it('на телефоне табы роли уступают выпадающей кнопке, выбор уходит тем же фильтром на бэк', async () => {
    mockNarrowViewport(true);
    state.summary = fullSummary();
    state.journal = journalEntries();
    const wrapper = mountTab();
    await flushPromises();

    expect(wrapper.find('[data-testid="filter-tab-approval"]').exists()).toBe(false);
    const dropdown = wrapper.findComponent({ name: 'BaseDropdown' });
    expect(dropdown.exists()).toBe(true);

    const callsBefore = state.journalCalls.length;
    await dropdown.find('.base-dropdown__button').trigger('click');
    await nextTick();
    const target = dropdown.findAll('.base-dropdown__item').find((i) => i.text() === 'Принятия');
    expect(target).toBeTruthy();
    await target.trigger('click');
    await flushPromises();

    expect(state.journalCalls).toHaveLength(callsBefore + 1);
    expect(state.journalCalls.at(-1)).toMatchObject({ role: 'acceptance', offset: 0 });
  });

  it('на телефоне поиск свёрнут в иконку, раскрывается по тапу и уходит тем же дебаунсом', async () => {
    vi.useFakeTimers();
    try {
      mockNarrowViewport(true);
      state.summary = fullSummary();
      state.journal = journalEntries();
      const wrapper = mountTab();
      await flushPromises();

      expect(wrapper.find('.proc__journal-search-icon').exists()).toBe(true);
      expect(wrapper.find('.proc__journal-search-overlay').exists()).toBe(false);
      // Десктопный SearchComponent на телефоне не рендерится вовсе - его подменяет иконка.
      expect(wrapper.find('.proc__journal-search input').exists()).toBe(false);

      await wrapper.find('.proc__journal-search-icon').trigger('click');
      await flushPromises();
      expect(wrapper.find('.proc__journal-search-overlay').exists()).toBe(true);

      const input = wrapper.find('.proc__journal-search-input');
      const callsBefore = state.journalCalls.length;
      await input.setValue('Кузнецов');
      expect(state.journalCalls).toHaveLength(callsBefore); // до паузы бэк не дёргаем

      vi.advanceTimersByTime(300);
      await flushPromises();
      expect(state.journalCalls.at(-1)).toMatchObject({ q: 'Кузнецов', offset: 0 });

      // Крестик очищает поле и закрывает оверлей - тот же приём, что в Центре/кабинете.
      await wrapper.find('.proc__journal-search-clear').trigger('click');
      await flushPromises();
      expect(wrapper.find('.proc__journal-search-overlay').exists()).toBe(false);
      expect(state.journalCalls.at(-1)).toMatchObject({ q: '' });
    } finally {
      vi.useRealTimers();
    }
  });
});
