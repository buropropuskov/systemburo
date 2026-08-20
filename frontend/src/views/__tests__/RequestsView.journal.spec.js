import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { flushPromises } from '@vue/test-utils';

// Живой журнал раздела мониторинга (#2125): порядок строк задаёт сервер, отбор
// живёт в адресной строке, а медленный ответ прошлого фильтра не затирает
// свежий. Проверки идут через действия пользователя и наблюдаемые следствия
// (адрес запроса, адресная строка, разметка), а не через внутренние поля:
// журнал живёт в своём компоненте и внутренностей наружу не отдаёт.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  apiRequestRaw: vi.fn(),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify: vi.fn() }),
}));

import { apiRequest, apiRequestRaw } from '@/api/client';
import { JOURNAL_REFRESH_MS } from '@/utils/requestLogsLive';
import {
  chip, currentQuery, filterDropdown, journalCalls, lastJournalCall, logsPage,
  mountView, pickOption, resetApiMocks, setTabHidden, unmountAll,
} from './helpers/requestsView';

const LOG = { id: 7, url: '/api/applications', method: 'GET', response_status: 200, duration_us: 900 };

afterEach(() => {
  unmountAll();
});

beforeEach(() => {
  resetApiMocks();
});

describe('RequestsView, порядок строк', () => {
  it('клик по заголовку перезапрашивает список, повторный клик разворачивает порядок', async () => {
    const { wrapper } = await mountView();
    await flushPromises();
    apiRequestRaw.mockClear();

    const duration = wrapper.findAll('.header-col').find(col => col.text().includes('Отклик'));
    expect(duration, 'колонка «Отклик» есть в шапке таблицы').toBeTruthy();

    await duration.trigger('click');
    await flushPromises();
    expect(lastJournalCall()).toContain('sort=duration');
    expect(lastJournalCall()).toContain('order=desc');

    await duration.trigger('click');
    await flushPromises();
    expect(lastJournalCall()).toContain('order=asc');
  });

  it('клик по другой колонке сбрасывает страницу на первую', async () => {
    const { wrapper } = await mountView({ page: '3' });
    await flushPromises();
    expect(journalCalls()[0]).toContain('page=3');

    const method = wrapper.findAll('.header-col').find(col => col.text().includes('Метод'));
    await method.trigger('click');
    await flushPromises();

    const last = lastJournalCall();
    expect(last, 'сортировка показывает первую страницу нового порядка').toContain('page=1');
    expect(last).toContain('sort=method');
  });
});

describe('RequestsView, отбор в адресной строке', () => {
  it('открывает журнал в том виде, в каком записан адрес', async () => {
    const { wrapper } = await mountView({
      method: 'post',
      status: '500',
      from: '2026-08-01',
      to: '2026-08-19',
      sort: 'status',
      order: 'asc',
      per_page: '50',
    });
    await flushPromises();

    const url = journalCalls()[0];
    expect(url).toContain('method=POST');
    expect(url).toContain('status=500');
    expect(url).toContain('from_date=2026-08-01');
    expect(url).toContain('to_date=2026-08-19');
    expect(url).toContain('sort=status');
    expect(url).toContain('order=asc');
    expect(url).toContain('per_page=50');

    const status = wrapper.findAll('.header-col').find(col => col.text().includes('Статус'));
    expect(status.find('.active-sort').exists(), 'выбранный порядок подсвечен в шапке').toBe(true);
  });

  it('чужое поле сортировки из адреса не применяется', async () => {
    await mountView({ sort: 'response_body', order: 'asc' });
    await flushPromises();

    expect(journalCalls()[0]).toContain('sort=created_at');
    expect(journalCalls()[0]).toContain('order=desc');
  });

  it('смена отбора складывается обратно в адрес, значения по умолчанию не пишутся', async () => {
    const { wrapper, router } = await mountView();
    await flushPromises();

    await pickOption(filterDropdown(wrapper, 0), 'DELETE');
    expect(currentQuery(router)).toEqual({ method: 'DELETE' });

    await wrapper.get('[data-testid="journal-clear"]').trigger('click');
    await flushPromises();
    expect(currentQuery(router), 'сброшенный отбор не оставляет мусора в ссылке').toEqual({});
  });

  it('сброс фильтров не трогает порядок строк', async () => {
    const { wrapper, router } = await mountView({ sort: 'duration', order: 'asc', method: 'GET' });
    await flushPromises();

    await wrapper.get('[data-testid="journal-clear"]').trigger('click');
    await flushPromises();

    const query = currentQuery(router);
    expect(query.method, 'фильтры сброшены').toBeUndefined();
    expect(query.sort, 'выбранный порядок остаётся: его задаёт заголовок, а не панель фильтров').toBe('duration');
    expect(query.order).toBe('asc');
    expect(lastJournalCall()).toContain('sort=duration');
  });
});

describe('RequestsView, гонка запросов', () => {
  it('ответ прошлого отбора не затирает свежий', async () => {
    const { wrapper } = await mountView();
    await flushPromises();

    let releaseStale;
    const stale = new Promise(resolve => { releaseStale = resolve; });
    apiRequestRaw.mockImplementationOnce(() => stale);
    apiRequestRaw.mockImplementationOnce(() => Promise.resolve(
      logsPage([{ id: 2, url: '/api/fresh', method: 'GET', response_status: 200, duration_us: 900 }])
    ));

    // Два отбора подряд: первый ответ задержан, второй приходит сразу.
    await pickOption(filterDropdown(wrapper, 0), 'GET');
    await pickOption(filterDropdown(wrapper, 0), 'POST');

    releaseStale(logsPage([{ id: 1, url: '/api/stale', method: 'GET', response_status: 200, duration_us: 100 }]));
    await flushPromises();

    const rows = wrapper.findAll('.table-row');
    expect(rows.map(r => r.text()), 'на экране остались строки последнего отбора').toEqual(
      expect.arrayContaining([expect.stringContaining('/api/fresh')])
    );
    expect(rows.some(r => r.text().includes('/api/stale')), 'запоздавший ответ на экран не попал').toBe(false);
    expect(wrapper.find('.loading-overlay').exists(), 'запоздавший ответ не зажигает загрузку заново').toBe(false);
  });
});

describe('RequestsView, живая лента', () => {
  it('список перечитывает себя сам, пока журнал открыт на первой странице', async () => {
    vi.useFakeTimers();
    try {
      await mountView();
      await flushPromises();
      apiRequestRaw.mockClear();

      vi.advanceTimersByTime(JOURNAL_REFRESH_MS);
      await flushPromises();

      expect(apiRequestRaw, 'лента сходила за свежими записями сама').toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it('открытое окно запроса держит ленту, закрытие возвращает обновление', async () => {
    apiRequestRaw.mockResolvedValue(logsPage([LOG]));
    vi.useFakeTimers();
    try {
      const { wrapper } = await mountView();
      await flushPromises();

      await wrapper.get('.table-row').trigger('click');
      apiRequestRaw.mockClear();
      vi.advanceTimersByTime(JOURNAL_REFRESH_MS * 2);
      await flushPromises();
      expect(apiRequestRaw, 'из-под открытого окна строка уехать не должна').not.toHaveBeenCalled();
      expect(wrapper.find('.toggle-stub').text()).toContain('открыто окно запроса');

      await wrapper.get('[data-testid="modal-button-close"]').trigger('click');
      vi.advanceTimersByTime(JOURNAL_REFRESH_MS);
      await flushPromises();
      expect(apiRequestRaw).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it('в фоновой вкладке опросы стоят, при возврате лента догоняет пропущенное', async () => {
    vi.useFakeTimers();
    try {
      await mountView();
      await flushPromises();

      setTabHidden(true);
      // Видимость доезжает до вкладки пропом, то есть на следующем тике.
      await flushPromises();
      apiRequestRaw.mockClear();
      apiRequest.mockClear();
      vi.advanceTimersByTime(JOURNAL_REFRESH_MS * 3);
      await flushPromises();
      expect(apiRequestRaw, 'фоновая вкладка не жжёт запросы').not.toHaveBeenCalled();
      expect(apiRequest, 'счётчики ленты в фоне тоже молчат').not.toHaveBeenCalled();

      setTabHidden(false);
      await flushPromises();
      expect(apiRequestRaw, 'вернувшись, человек видит свежий список, а не устаревший').toHaveBeenCalledTimes(1);
    } finally {
      setTabHidden(false);
      vi.useRealTimers();
    }
  });

  it('выключенная лента не обновляется даже на открытом журнале', async () => {
    vi.useFakeTimers();
    try {
      const { wrapper } = await mountView();
      await flushPromises();

      await wrapper.get('.toggle-stub').trigger('click');
      apiRequestRaw.mockClear();
      vi.advanceTimersByTime(JOURNAL_REFRESH_MS * 2);
      await flushPromises();

      expect(apiRequestRaw).not.toHaveBeenCalled();
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('RequestsView, быстрый отбор', () => {
  it('«только ошибки» уходит на сервер границей кода и остаётся в адресе', async () => {
    const { wrapper, router } = await mountView();
    await flushPromises();
    apiRequestRaw.mockClear();

    await chip(wrapper, 'Только ошибки').trigger('click');
    await flushPromises();

    expect(lastJournalCall()).toContain('status_min=400');
    expect(currentQuery(router)).toEqual({ status: 'errors' });
  });

  it('«медленнее 1 с» ставит порог, повторное нажатие его снимает', async () => {
    const { wrapper } = await mountView();
    await flushPromises();

    await chip(wrapper, 'Медленнее 1 с').trigger('click');
    await flushPromises();
    expect(lastJournalCall()).toContain('min_duration_ms=1000');

    await chip(wrapper, 'Медленнее 1 с').trigger('click');
    await flushPromises();
    expect(lastJournalCall()).not.toContain('min_duration_ms');
  });

  it('«последний час» шлёт момент, а выбранный день его снимает', async () => {
    const { wrapper, router } = await mountView();
    await flushPromises();

    await chip(wrapper, 'Последний час').trigger('click');
    await flushPromises();

    const since = currentQuery(router).since;
    expect(since, 'граница периода это момент, а не сутки').toContain('T');
    expect(lastJournalCall()).toContain(`from_date=${encodeURIComponent(since)}`);

    const calendar = wrapper.findAllComponents({ name: 'DateFilter' })[0];
    calendar.vm.$emit('update:dateRangeStart', new Date(2026, 7, 1));
    calendar.vm.$emit('apply');
    await flushPromises();

    expect(currentQuery(router).since, 'введённый день отменяет отбор за час').toBeUndefined();
    expect(lastJournalCall()).toContain('from_date=2026-08-01');
  });
});
