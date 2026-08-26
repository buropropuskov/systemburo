import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { flushPromises } from '@vue/test-utils';

// Приватность и честность выгрузки журнала (#2125): файл забирается отдельным
// клиентом (поток байтов, а не JSON-конверт), обрезка проговаривается словами, а
// отказ чтения перестаёт выглядеть пустым отбором.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  apiRequestRaw: vi.fn(),
}));

vi.mock('@/api/requestLogs', () => ({
  downloadRequestLogs: vi.fn(),
}));

const notify = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}));

import { apiRequest, apiRequestRaw } from '@/api/client';
import { downloadRequestLogs } from '@/api/requestLogs';
import { logsPage, mountView, resetApiMocks, unmountAll } from './helpers/requestsView';

afterEach(() => {
  unmountAll();
});

beforeEach(() => {
  resetApiMocks();
  downloadRequestLogs.mockResolvedValue({ rows: 5, total: 5, truncated: false });
});

async function clickExport(wrapper) {
  await wrapper.get('[data-testid="journal-export"]').trigger('click');
  await flushPromises();
}

describe('RequestsView, выгрузка журнала', () => {
  it('шлёт выбранный порядок и отбор тем же набором, что список', async () => {
    const { wrapper } = await mountView({ sort: 'duration', order: 'asc', status: 'errors' });
    await flushPromises();

    await clickExport(wrapper);

    expect(downloadRequestLogs).toHaveBeenCalledTimes(1);
    expect(downloadRequestLogs.mock.calls[0][0]).toMatchObject({
      sort: 'duration', order: 'asc', status_min: 400,
    });
  });

  it('обрезанная выгрузка говорит, сколько записей осталось за бортом', async () => {
    downloadRequestLogs.mockResolvedValue({ rows: 10000, total: 24513, truncated: true });
    const { wrapper } = await mountView();
    await flushPromises();

    await clickExport(wrapper);

    const warn = notify.mock.calls.map(([arg]) => arg).find(arg => arg.type === 'warning');
    expect(warn, 'обрезка обязана быть замечена').toBeTruthy();
    expect(warn.bold).toContain('10000');
    expect(warn.bold).toContain('24513');
  });

  it('полная выгрузка не пугает предупреждением', async () => {
    const { wrapper } = await mountView();
    await flushPromises();

    await clickExport(wrapper);

    const types = notify.mock.calls.map(([arg]) => arg.type);
    expect(types).toContain('success');
    expect(types).not.toContain('warning');
  });

  it('отказ выгрузки объясняется на экране, а не только тостом', async () => {
    const denied = new Error('403');
    denied.status = 403;
    downloadRequestLogs.mockRejectedValue(denied);
    const { wrapper } = await mountView();
    await flushPromises();

    await clickExport(wrapper);

    expect(wrapper.text()).toContain('нет прав на раздел');
  });
});

describe('RequestsView, отказ чтения журнала', () => {
  it('403 на списке показывает причину вместо пустой таблицы', async () => {
    apiRequestRaw.mockResolvedValue({ ok: false, status: 403, json: () => Promise.resolve({}) });
    const { wrapper } = await mountView();
    await flushPromises();

    expect(wrapper.text()).toContain('нет прав на раздел');
    expect(wrapper.text()).not.toContain('Записей по такому отбору нет');
  });

  it('сбой раздела шапки замечен один раз, а не на каждом опросе', async () => {
    apiRequest.mockResolvedValue({ ok: false, status: 403, json: () => Promise.resolve({}) });
    vi.useFakeTimers();
    try {
      await mountView();
      await flushPromises();

      const before = notify.mock.calls.filter(([arg]) => arg.type === 'error').length;
      expect(before, 'об отказе раздела сказали').toBeGreaterThan(0);

      // Показатели и график опрашиваются каждые полминуты - отказ повторится.
      vi.advanceTimersByTime(30000);
      await flushPromises();

      const after = notify.mock.calls.filter(([arg]) => arg.type === 'error').length;
      expect(after, 'повторный опрос не плодит одинаковые тосты').toBe(before);
    } finally {
      vi.useRealTimers();
    }
  });

  it('пустой отбор остаётся пустым отбором, а не ошибкой', async () => {
    const { wrapper } = await mountView();
    await flushPromises();

    expect(wrapper.text()).toContain('Записей по такому отбору нет');
  });

  it('успешный ответ снимает прежнее сообщение об отказе', async () => {
    apiRequestRaw.mockResolvedValueOnce({ ok: false, status: 500, json: () => Promise.resolve({}) });
    const { wrapper } = await mountView();
    await flushPromises();
    expect(wrapper.text()).toContain('сервер ответил ошибкой 500');

    apiRequestRaw.mockResolvedValue(logsPage([{ id: 1, method: 'GET', url: '/api/x', response_status: 200 }]));
    await wrapper.get('.refresh-stub').trigger('click');
    await flushPromises();

    expect(wrapper.text()).not.toContain('сервер ответил ошибкой 500');
  });
});
