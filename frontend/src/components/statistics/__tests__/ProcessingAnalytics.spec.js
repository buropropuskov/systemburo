import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { nextTick } from 'vue';

// Фабрика vi.mock поднимается над импортами — управляемое состояние в hoisted.
const { state } = vi.hoisted(() => ({
  state: { summary: {}, journal: [], trendRows: [], trendCalls: [], trendDeferred: [], reject: false, deferred: [], streamHandler: null },
}));

vi.mock('@/api/statistics.js', () => ({
  getProcessingSummary: () => {
    if (state.reject) return Promise.reject(new Error('boom'));
    if (state.hang) return new Promise((resolve) => { state.deferred.push(resolve); });
    return Promise.resolve(state.summary);
  },
  getProcessingJournal: () => {
    if (state.hang) return new Promise(() => {}); // на первой загрузке лента тоже висит
    return Promise.resolve(state.journal);
  },
  // Динамика по дням строится отдельным запросом к движку отчётов (S/P3).
  runReport: (req) => {
    state.trendCalls.push(req);
    if (state.hang) return new Promise((resolve) => { state.trendDeferred.push(resolve); });
    return Promise.resolve({ metric_rows: state.trendRows });
  },
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
import HintTooltip from '@/components/ui/HintTooltip.vue';

const mountTab = () => mount(ProcessingAnalytics, {
  props: { from: '2026-06-01', to: '2026-06-07' },
  global: { stubs: { AnalyticsAreaChart: true } },
});

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
    { label: 'ООО Ромашка', avg_processing_time: 10800, applications_count: 20 },
  ],
});

beforeEach(() => {
  state.summary = {};
  state.journal = [];
  state.trendRows = [];
  state.trendCalls = [];
  state.trendDeferred = [];
  state.reject = false;
  state.hang = false;
  state.deferred = [];
  state.streamHandler = null;
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

  it('разбивку по организациям показывает временем обработки и числом заявок', async () => {
    state.summary = fullSummary();
    const wrapper = mountTab();
    await flushPromises();

    const text = wrapper.text();
    expect(text).toContain('ООО Ромашка');
    expect(text).toContain('3 ч'); // 10800 с
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

    // Второе — согласование без рабочей длительности (null -> пусто, не «—»).
    expect(rows[1].find('.proc__journal-role').text()).toBe('Согласование');
    expect(rows[1].find('.proc__journal-role').classes()).toContain('proc__journal-role--approval');
    expect(rows[1].find('.proc__journal-dur').text()).toBe('');
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
    state.summary = { from: '2026-06-01', to: '2026-06-07', total_applications: 0, stages: [], quality: [], approvers: [], acceptors: [], by_organization: [] };
    state.journal = journalEntries();
    const wrapper = mountTab();
    await flushPromises();

    expect(wrapper.text()).toContain('заявок не подавали'); // заглушка бандла на месте
    // Но журнал всё равно отрисован со своими событиями.
    expect(wrapper.text()).toContain('Журнал');
    expect(wrapper.findAll('.proc__journal-row')).toHaveLength(2);
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

describe('ProcessingAnalytics — крайние состояния', () => {
  it('пустой период показывает заглушку, а не нулевые метрики', async () => {
    state.summary = { from: '2026-06-01', to: '2026-06-07', total_applications: 0, stages: [], quality: [], approvers: [], acceptors: [], by_organization: [] };
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
    expect(wrapper.findAll('.proc__skeleton--table').length).toBe(4); // согласующие + принимающие + организации + журнал
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
