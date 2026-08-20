import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { nextTick } from 'vue';

// Управляемое состояние моков: фабрика vi.mock поднимается над импортами,
// поэтому summary/deferred выносим в hoisted.
const { state } = vi.hoisted(() => ({
  state: { deferred: [], onlineDeferred: [], summary: {}, insights: {}, onlinePeaks: [], passages: { people: [], cars: [] }, insightsHang: false, summaryReject: false },
}));

vi.mock('@/api/statistics.js', () => ({
  getSummary: () => (state.summaryReject ? Promise.reject(new Error('net')) : Promise.resolve(state.summary)),
  getRecentPassages: () => Promise.resolve(state.passages),
  getTimeline: () => new Promise((resolve) => { state.deferred.push(resolve); }),
  // insightsHang оставляет промис вечно pending — для проверки состояния загрузки.
  getInsights: () => (state.insightsHang ? new Promise(() => {}) : Promise.resolve(state.insights)),
  getOnlinePeaks: () => Promise.resolve(state.onlinePeaks),
  // deferred: резолвим вручную в тестах, чтобы проверить гонку seq-токена.
  getOnlineUsers: () => new Promise((resolve) => { state.onlineDeferred.push(resolve); }),
}));

import StatisticsDashboard from '../StatisticsDashboard.vue';
import AnalyticsAreaChart from '../AnalyticsAreaChart.vue';
import AnalyticsBarChart from '../AnalyticsBarChart.vue';
import TrendSparkline from '../TrendSparkline.vue';
import OnlineUsersModal from '../OnlineUsersModal.vue';

const mountDashboard = () => mount(StatisticsDashboard, {
  props: { from: '2026-06-01', to: '2026-06-07' },
  global: {
    // PushAdoptionSummary (#974) стоит рядом с остальными тяжёлыми детьми -
    // как и они, стабится: своя загрузка (api/webPush) не замокана в этом
    // файле, тестируется отдельно в PushAdoptionSummary.spec.js.
    stubs: {
      AnalyticsAreaChart: true, AnalyticsBarChart: true, AnalyticsDonutChart: true,
      RefreshButton: true, OnlineUsersModal: true, PushAdoptionSummary: true,
    },
  },
});

const tileByText = (wrapper, label) =>
  wrapper.findAll('.dashboard__tile').find((t) => t.text().includes(label));

beforeEach(() => {
  state.deferred.length = 0;
  state.onlineDeferred.length = 0;
  state.summary = {};
  state.insights = {};
  state.onlinePeaks = [];
  state.passages = { people: [], cars: [] };
  state.insightsHang = false;
  state.summaryReject = false;
});

describe('StatisticsDashboard — гонка таймлайна', () => {
  it('при быстрой смене метрики рисует данные последнего запроса, медленный предыдущий игнорирует', async () => {
    const wrapper = mountDashboard();
    await nextTick(); // onMounted -> loadTimeline (seq 1, deferred)

    // Быстрое переключение метрики -> второй loadTimeline (seq 2).
    await wrapper.findAll('.dashboard__seg-btn')[1].trigger('click');
    await nextTick();

    expect(state.deferred).toHaveLength(2);

    // Последний запрос (seq 2) приходит ПЕРВЫМ, устаревший seq 1 — позже.
    state.deferred[1]([{ date: '2026-06-02', count: 222 }]);
    await flushPromises();
    state.deferred[0]([{ date: '2026-06-01', count: 111 }]);
    await flushPromises();

    const chart = wrapper.findComponent(AnalyticsAreaChart);
    const data = chart.props('data');
    expect(data).toHaveLength(1);
    expect(data[0].count).toBe(222); // не 111 от устаревшего ответа
  });
});

describe('StatisticsDashboard — плитки', () => {
  it('показывает переименованные метрики и убирает снятые плитки', async () => {
    state.summary = { total_applications: 5, processed: 2, active_users: 7 };
    const wrapper = mountDashboard();
    await flushPromises();

    const text = wrapper.text();
    expect(text).toContain('Получено заявок');
    expect(text).toContain('Пользователи');
    expect(text).not.toContain('Всего заявок');
    expect(text).not.toContain('В работе');
    expect(text).not.toContain('Сумма товаров');
    expect(text).not.toContain('Вложения (всего)');
  });

  it('раздел Вложения рендерит блок на каждый активный тип системы', async () => {
    state.summary = {
      by_attachment_type: [
        { name: 'Паспорт', count: 3 },
        { name: 'Виза', count: 0 },
      ],
    };
    const wrapper = mountDashboard();
    await flushPromises();

    const text = wrapper.text();
    expect(text).toContain('Вложения');
    expect(text).toContain('Паспорт');
    expect(text).toContain('Виза');
  });

  it('длинный список типов рендерит плитку на каждый тип', async () => {
    // Типы вложений заводит администратор: их бывает и десяток. Раскладку в две
    // колонки на таком списке стережёт замок в assets/__tests__.
    state.summary = {
      by_attachment_type: Array.from({ length: 10 }, (_, i) => ({ name: `Тип ${i + 1}`, count: i })),
    };
    const wrapper = mountDashboard();
    await flushPromises();

    expect(wrapper.findAll('.an-panel__tiles > .dashboard__tile')).toHaveLength(10);
  });
});

describe('StatisticsDashboard — инсайты карточек', () => {
  it('карточка с метрикой инсайта рендерит спарклайн и бейдж дельты; карточки без инсайта — нет', async () => {
    state.summary = { total_applications: 10, processed: 4, cars_entered: 8 };
    state.insights = {
      comparisons: [
        { metric: 'applications_count', current: 10, previous: 8, delta_pct: 25, direction: 'up' },
      ],
      trends: [
        { metric: 'applications_count', direction: 'up', series: [1, 2, 3, 4] },
      ],
    };
    const wrapper = mountDashboard();
    await flushPromises();

    // Только applications_count покрыт инсайтом -> ровно один футер с дельтой и спарклайном.
    expect(wrapper.text()).toContain('+25%');
    expect(wrapper.find('.dashboard__delta--up').exists()).toBe(true);
    expect(wrapper.findComponent(TrendSparkline).exists()).toBe(true);
    expect(wrapper.findAll('.dashboard__tile-insight')).toHaveLength(1);
  });

  it('без инсайтов карточки рендерятся без футера', async () => {
    state.summary = { total_applications: 10, cars_entered: 8 };
    state.insights = {};
    const wrapper = mountDashboard();
    await flushPromises();

    expect(wrapper.findAll('.dashboard__tile-insight')).toHaveLength(0);
  });
});

describe('StatisticsDashboard — разворот карточки', () => {
  const withInsights = () => {
    state.summary = { total_applications: 10, cars_entered: 8 };
    state.insights = {
      comparisons: [{ metric: 'applications_count', current: 10, previous: 8, delta_pct: 25, direction: 'up' }],
      trends: [{ metric: 'applications_count', direction: 'up', series: [1, 2, 3, 4] }],
      peak_hours: [{
        metric: 'applications_count', label: 'Заявки', peak_hour: 9, peak_value: 5,
        hourly: [{ hour: 8, value: 2 }, { hour: 9, value: 5 }],
      }],
    };
  };

  it('обогащённая карточка кликабельна, клик раскрывает тренд и пик, повторный — сворачивает', async () => {
    withInsights();
    const wrapper = mountDashboard();
    await flushPromises();

    const tile = tileByText(wrapper, 'Получено заявок');
    expect(tile.classes()).toContain('dashboard__tile--clickable');
    expect(wrapper.find('.dashboard__detail').exists()).toBe(false);

    await tile.trigger('click');
    await nextTick();

    expect(wrapper.find('.dashboard__detail').exists()).toBe(true);
    expect(wrapper.find('.dashboard__detail').text()).toContain('Получено заявок');
    // Тренд (area) и пик по часам (bar) — оба графика в детальной панели.
    const detail = wrapper.find('.dashboard__detail');
    expect(detail.findComponent(AnalyticsAreaChart).exists()).toBe(true);
    expect(detail.findComponent(AnalyticsBarChart).exists()).toBe(true);
    expect(tile.classes()).toContain('dashboard__tile--active');

    await tile.trigger('click');
    await nextTick();
    expect(wrapper.find('.dashboard__detail').exists()).toBe(false);
  });

  it('тренд детали строит датированный ряд из getTimeline (ось X — даты, не порядковые номера)', async () => {
    withInsights();
    const wrapper = mountDashboard();
    await flushPromises(); // onMounted -> loadTimeline (deferred[0])

    await tileByText(wrapper, 'Получено заявок').trigger('click');
    await nextTick(); // watch(expandedMetric) -> loadDetailTimeline -> getTimeline

    // Детальный тренд грузится отдельным getTimeline по дням; резолвим последний.
    state.deferred[state.deferred.length - 1]([
      { date: '2026-06-01', count: 3 },
      { date: '2026-06-02', count: 7 },
    ]);
    await flushPromises();

    const area = wrapper.find('.dashboard__detail').findComponent(AnalyticsAreaChart);
    // Точки с timestamp -> график сам строит подписи дд.мм; categories не навязываем.
    expect(area.props('data')).toEqual([
      { timestamp: '2026-06-01', count: 3 },
      { timestamp: '2026-06-02', count: 7 },
    ]);
    expect(area.props('categories') == null).toBe(true);
    // Разворот обёрнут в grid-collapse -> плавная высота без телепортации соседей.
    expect(wrapper.find('.dashboard__detail-collapse').exists()).toBe(true);
  });

  it('быстрое переключение карточек: тренд показывает данные последней, устаревший ответ игнорирует', async () => {
    state.summary = { total_applications: 10, cars_entered: 8 };
    state.insights = {
      comparisons: [
        { metric: 'applications_count', current: 10, previous: 8, delta_pct: 25, direction: 'up' },
        { metric: 'car_entries_count', current: 8, previous: 6, delta_pct: 33, direction: 'up' },
      ],
      trends: [
        { metric: 'applications_count', direction: 'up', series: [1, 2] },
        { metric: 'car_entries_count', direction: 'up', series: [3, 4] },
      ],
    };
    const wrapper = mountDashboard();
    await flushPromises(); // onMounted -> loadTimeline (deferred[0])

    await tileByText(wrapper, 'Получено заявок').trigger('click'); // detail A -> deferred[1]
    await nextTick();
    await tileByText(wrapper, 'Машин заехало').trigger('click'); // detail B -> deferred[2]
    await nextTick();

    // Последний запрос (B) резолвится ПЕРВЫМ, устаревший A — позже.
    const aResolve = state.deferred[1];
    const bResolve = state.deferred[2];
    bResolve([{ date: '2026-06-03', count: 99 }]);
    await flushPromises();
    aResolve([{ date: '2026-06-01', count: 11 }]);
    await flushPromises();

    const area = wrapper.find('.dashboard__detail').findComponent(AnalyticsAreaChart);
    expect(area.props('data')).toEqual([{ timestamp: '2026-06-03', count: 99 }]); // B, не A
  });

  it('смена периода сворачивает разворот', async () => {
    withInsights();
    const wrapper = mountDashboard();
    await flushPromises();

    await tileByText(wrapper, 'Получено заявок').trigger('click');
    await nextTick();
    expect(wrapper.find('.dashboard__detail').exists()).toBe(true);

    await wrapper.setProps({ from: '2026-05-01', to: '2026-05-07' });
    await nextTick();
    expect(wrapper.find('.dashboard__detail').exists()).toBe(false);
  });

  it('карточка без инсайтов не кликабельна и не раскрывается', async () => {
    state.summary = { processed: 4 };
    state.insights = {};
    const wrapper = mountDashboard();
    await flushPromises();

    const tile = tileByText(wrapper, 'Обработано');
    expect(tile).toBeTruthy();
    expect(tile.classes()).not.toContain('dashboard__tile--clickable');
    await tile.trigger('click');
    await nextTick();
    expect(wrapper.find('.dashboard__detail').exists()).toBe(false);
  });
});

describe('StatisticsDashboard — топы за период', () => {
  it('рендерит лидерборды мест и организаций из инсайтов', async () => {
    state.insights = {
      top_places: [{ metric: 'car_entries_count', label: 'Дебаркадер №1', value: 9 }],
      top_orgs: [{ metric: 'applications_count', label: 'ООО Ромашка', value: 20 }],
    };
    const wrapper = mountDashboard();
    await flushPromises();

    expect(wrapper.text()).toContain('Топ за период');
    expect(wrapper.text()).toContain('Места разгрузки');
    expect(wrapper.text()).toContain('Дебаркадер №1');
    expect(wrapper.text()).toContain('Организации');
    expect(wrapper.text()).toContain('ООО Ромашка');
  });

  it('без данных топов лидерборды показывают пустое состояние', async () => {
    state.insights = {};
    const wrapper = mountDashboard();
    await flushPromises();

    expect(wrapper.text()).toContain('Топ за период');
    // Оба TopList без items -> два плейсхолдера «Нет данных».
    expect(wrapper.findAll('.top__empty')).toHaveLength(2);
  });

  it('во время загрузки инсайтов секция топов показывает скелетоны вместо лидербордов', async () => {
    state.insightsHang = true; // getInsights не резолвится -> insightsLoading остаётся true
    const wrapper = mountDashboard();
    await nextTick(); // onMounted -> loadInsights выставил insightsLoading -> ре-рендер

    expect(wrapper.findAll('.dashboard__top-skeleton')).toHaveLength(2);
    expect(wrapper.find('.top').exists()).toBe(false);
  });
});

describe('StatisticsDashboard — плитка онлайна', () => {
  it('плитка "Пользователей онлайн" кликабельна и доступна с клавиатуры', async () => {
    const wrapper = mountDashboard();
    await flushPromises();

    const tile = tileByText(wrapper, 'Пользователей онлайн');
    expect(tile.classes()).toContain('dashboard__tile--clickable');
    expect(tile.attributes('role')).toBe('button');
    expect(tile.attributes('tabindex')).toBe('0');
    expect(tile.attributes('aria-haspopup')).toBe('dialog');
  });

  it('клик по плитке открывает модалку и прокидывает список из API', async () => {
    const wrapper = mountDashboard();
    await flushPromises();

    const modal = wrapper.findComponent(OnlineUsersModal);
    expect(modal.props('show')).toBe(false);

    await tileByText(wrapper, 'Пользователей онлайн').trigger('click');
    await nextTick();
    // Пока запрос в полёте — модалка открыта в состоянии загрузки.
    expect(modal.props('show')).toBe(true);
    expect(modal.props('loading')).toBe(true);

    state.onlineDeferred[0]([
      { id: 1, login: 'ivanov', full_name: 'Иванов И.', role: 'Руководитель', user_type: 'Арендатор', last_seen: '2026-06-20T12:00:00Z' },
    ]);
    await flushPromises();

    expect(modal.props('users')).toHaveLength(1);
    expect(modal.props('users')[0].login).toBe('ivanov');
    expect(modal.props('loading')).toBe(false);
  });

  it('повторное открытие: показывает список последнего запроса, устаревший игнорирует', async () => {
    const wrapper = mountDashboard();
    await flushPromises();
    const tile = tileByText(wrapper, 'Пользователей онлайн');

    await tile.trigger('click'); // seq 1
    await tile.trigger('click'); // seq 2
    await nextTick();
    expect(state.onlineDeferred).toHaveLength(2);

    // Последний запрос (seq 2) резолвится ПЕРВЫМ, устаревший seq 1 — позже.
    state.onlineDeferred[1]([{ id: 2, login: 'fresh', full_name: '', role: '', user_type: '', last_seen: '2026-06-20T12:00:00Z' }]);
    await flushPromises();
    state.onlineDeferred[0]([{ id: 1, login: 'stale', full_name: '', role: '', user_type: '', last_seen: '2026-06-20T11:00:00Z' }]);
    await flushPromises();

    const modal = wrapper.findComponent(OnlineUsersModal);
    expect(modal.props('users')).toHaveLength(1);
    expect(modal.props('users')[0].login).toBe('fresh'); // не stale от устаревшего ответа
  });
});

describe('StatisticsDashboard — секция Мониторинг', () => {
  it('occupancy-плитки (на территории) рендерятся в Мониторинге, а не в группе Данные', async () => {
    state.summary = { total_applications: 5, cars_on_territory: 17, people_on_territory: 42 };
    const wrapper = mountDashboard();
    await flushPromises();

    const monitoring = wrapper.find('.dashboard__group--monitoring');
    expect(monitoring.exists()).toBe(true);
    expect(monitoring.text()).toContain('Мониторинг');
    expect(monitoring.text()).toContain('в реальном времени');
    expect(monitoring.text()).toContain('Машин на территории');
    expect(monitoring.text()).toContain('Людей на территории');
    expect(monitoring.text()).toContain((17).toLocaleString('ru-RU'));
    expect(monitoring.text()).toContain((42).toLocaleString('ru-RU'));

    // Группа «Данные» (первая) больше не содержит снимок территории.
    const dataGroup = wrapper.findAll('.dashboard__group')[0];
    expect(dataGroup.text()).toContain('Данные');
    expect(dataGroup.text()).not.toContain('на территории');
  });

  it('пост (place) виден явным лейблом в строке прохода, при отсутствии — заглушка', async () => {
    state.passages = {
      people: [
        { subject: 'Иванов И.', organization: 'ООО Ромашка', place: 'КПП-1', created_at: '2026-06-20T10:00:00Z', action_type: 'entry' },
        { subject: 'Петров П.', organization: 'ООО Ромашка', place: '—', created_at: '2026-06-20T10:01:00Z', action_type: 'exit' },
        { subject: 'Сидоров С.', organization: 'ООО Ромашка', place: null, created_at: '2026-06-20T10:02:00Z', action_type: 'entry' },
        { subject: 'Кузнецов К.', organization: 'ООО Ромашка', place: '   ', created_at: '2026-06-20T10:03:00Z', action_type: 'exit' },
      ],
      cars: [
        { subject: 'А123АА', mark: 'BMW', organization: 'ООО Ромашка', place: '', created_at: '2026-06-20T10:05:00Z', action_type: 'entry' },
      ],
    };
    const wrapper = mountDashboard();
    await flushPromises();

    const posts = wrapper.findAll('.dashboard__feed-post');
    // Четыре прохода людей + один проезд машины = пять строк, у каждой явный пост.
    expect(posts).toHaveLength(5);
    expect(wrapper.text()).toContain('Место: КПП-1');
    // Плейсхолдер «—», null, пробелы и пустая строка -> заглушка «не указан».
    const empties = posts.filter((p) => p.text().includes('не указан'));
    expect(empties).toHaveLength(4);
    expect(empties[0].classes()).toContain('dashboard__feed-post--empty');
  });
});

describe('StatisticsDashboard — динамика онлайна', () => {
  it('рендерит area-график пиков онлайна за период', async () => {
    state.onlinePeaks = [
      { date: '2026-06-01', peak: 4 },
      { date: '2026-06-02', peak: 9 },
    ];
    const wrapper = mountDashboard();
    await flushPromises();

    expect(wrapper.text()).toContain('Динамика онлайна');
    const online = wrapper.findAllComponents(AnalyticsAreaChart)
      .find((c) => c.props('seriesName') === 'Пик онлайна');
    expect(online).toBeTruthy();
    expect(online.props('data')).toEqual([
      { timestamp: '2026-06-01', count: 4 },
      { timestamp: '2026-06-02', count: 9 },
    ]);
  });
});

describe('StatisticsDashboard — плавная перезагрузка без мигания', () => {
  it('первичная загрузка таймлайна показывает скелетон графика', async () => {
    const wrapper = mountDashboard();
    await flushPromises(); // summary/insights/онлайн готовы; таймлайн в полёте (deferred[0])

    expect(wrapper.find('.dashboard__chart-skeleton').exists()).toBe(true);
  });

  it('смена периода не возвращает скелетон графика — он остаётся смонтированным', async () => {
    const wrapper = mountDashboard();
    await flushPromises();
    state.deferred[0]([{ date: '2026-06-01', count: 5 }]);
    await flushPromises();

    expect(wrapper.find('.dashboard__chart-skeleton').exists()).toBe(false);
    expect(wrapper.findComponent(AnalyticsAreaChart).exists()).toBe(true);

    // Новый период -> таймлайн снова в полёте, но без мигания скелетоном.
    await wrapper.setProps({ from: '2026-05-01', to: '2026-05-07' });
    await nextTick();

    expect(state.deferred.length).toBeGreaterThanOrEqual(2);
    expect(wrapper.find('.dashboard__chart-skeleton').exists()).toBe(false);
    expect(wrapper.findComponent(AnalyticsAreaChart).exists()).toBe(true);
  });

  it('смена периода не возвращает скелетоны плиток — значения остаются на месте', async () => {
    state.summary = { total_applications: 5 };
    const wrapper = mountDashboard();
    await flushPromises();
    expect(wrapper.findAll('.dashboard__tile--skeleton')).toHaveLength(0);

    await wrapper.setProps({ from: '2026-05-01', to: '2026-05-07' });
    await nextTick();

    expect(wrapper.findAll('.dashboard__tile--skeleton')).toHaveLength(0);
    expect(tileByText(wrapper, 'Получено заявок')).toBeTruthy();
  });

  it('значение плитки рендерится через AnimatedNumber и отформатировано', async () => {
    state.summary = { total_applications: 1234 };
    const wrapper = mountDashboard();
    await flushPromises();

    const tile = tileByText(wrapper, 'Получено заявок');
    expect(tile.text()).toContain((1234).toLocaleString('ru-RU'));
  });

  it('ошибка первичной загрузки убирает скелетон (не зависает на нём навсегда)', async () => {
    state.summaryReject = true;
    const wrapper = mountDashboard();
    await flushPromises();

    // summaryReady встал в finally даже при reject -> скелетон не висит вечно.
    expect(wrapper.findAll('.dashboard__tile--skeleton')).toHaveLength(0);
    // Плитки группы «Данные» рендерятся с прочерками вместо чисел.
    expect(tileByText(wrapper, 'Получено заявок')).toBeTruthy();
  });
});
