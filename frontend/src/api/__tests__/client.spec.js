import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';

const notifyMock = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify: notifyMock }),
}));

const routerPush = vi.fn();
vi.mock('@/router', () => ({
  default: {
    push: (...args) => routerPush(...args),
    currentRoute: { value: { path: '/personal-cabinet' } },
  },
}));

import { apiRequest, apiRequestRaw, createExtendedTimeoutSignal, _resetDedup403, SILENT_403_PREFIXES } from '../client';
import { usePDConsentStore } from '@/stores/pdConsent';
import { usePasswordChangeStore } from '@/stores/passwordChange';

function okJson(body, init = {}) {
  return new Response(JSON.stringify(body), {
    status: init.status ?? 200,
    headers: { 'Content-Type': 'application/json' },
  });
}
function errJson(body, status) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}
function err429(retryAfterSec) {
  const headers = { 'Content-Type': 'application/json' };
  if (retryAfterSec != null) headers['Retry-After'] = String(retryAfterSec);
  return new Response(JSON.stringify({ success: false, error: 'too many' }), { status: 429, headers });
}

describe('apiRequest basics', () => {
  let fetchMock;

  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    routerPush.mockReset();
    fetchMock = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('calls fetch with the correct URL (with /api prefix)', async () => {
    await apiRequest('/health');
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe('/api/health');
  });

  it('adds Authorization header when token exists in store', async () => {
    const { useAuthStore } = await import('@/stores/auth');
    useAuthStore().setTokens('test-bearer-token');

    await apiRequest('/users/me');

    expect(fetchMock.mock.calls[0][1].headers.Authorization).toBe('Bearer test-bearer-token');
  });

  it('does not add Authorization header when no token', async () => {
    await apiRequest('/health');
    expect(fetchMock.mock.calls[0][1].headers.Authorization).toBeUndefined();
  });

  it('sets Content-Type to application/json', async () => {
    await apiRequest('/test');
    expect(fetchMock.mock.calls[0][1].headers['Content-Type']).toBe('application/json');
  });

  it('merges custom headers', async () => {
    await apiRequest('/test', { headers: { 'X-Custom': 'value' } });
    const opts = fetchMock.mock.calls[0][1];
    expect(opts.headers['X-Custom']).toBe('value');
    expect(opts.headers['Content-Type']).toBe('application/json');
  });

  it('passes method and body through', async () => {
    const body = JSON.stringify({ key: 'value' });
    await apiRequest('/test', { method: 'POST', body });
    expect(fetchMock.mock.calls[0][1].method).toBe('POST');
    expect(fetchMock.mock.calls[0][1].body).toBe(body);
  });

  it('returns the response object', async () => {
    const mockResponse = new Response('{"ok":true}', { status: 200 });
    fetchMock.mockResolvedValue(mockResponse);
    const result = await apiRequest('/test');
    expect(result).toBe(mockResponse);
  });

  it('provides an AbortController signal for timeout', async () => {
    await apiRequest('/test');
    expect(fetchMock.mock.calls[0][1].signal).toBeInstanceOf(AbortSignal);
  });

  it('unwraps the envelope via .json()', async () => {
    fetchMock.mockResolvedValue(okJson({ success: true, data: { id: 1 } }));
    const resp = await apiRequest('/users/me');
    expect(await resp.json()).toEqual({ id: 1 });
  });

  it('translates success:false envelope into {message}', async () => {
    fetchMock.mockResolvedValue(okJson({ success: false, error: 'boom' }, { status: 200 }));
    const resp = await apiRequest('/test');
    expect(await resp.json()).toEqual({ message: 'boom' });
  });
});

// Подача заявки с массовым импортом (эпик blank-import, срез E2E3) не укладывается
// в дефолтные 10с - у неё свой AbortSignal через createExtendedTimeoutSignal.
describe('свой таймаут поверх дефолтных 10 секунд', () => {
  let fetchMock;

  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    fetchMock = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('createExtendedTimeoutSignal возвращает AbortSignal', () => {
    expect(createExtendedTimeoutSignal(120000)).toBeInstanceOf(AbortSignal);
  });

  it('переданный options.signal уходит в fetch как есть, а не подменяется дефолтным контроллером', async () => {
    const ownSignal = createExtendedTimeoutSignal(120000);
    await apiRequest('/applications/submit-complete-application', {
      method: 'POST',
      body: '{}',
      signal: ownSignal,
    });

    expect(fetchMock.mock.calls[0][1].signal).toBe(ownSignal);
  });

  it('без options.signal запрос по-прежнему получает свой (дефолтный) AbortSignal', async () => {
    await apiRequest('/test');
    const usedSignal = fetchMock.mock.calls[0][1].signal;
    expect(usedSignal).toBeInstanceOf(AbortSignal);

    const ownSignal = createExtendedTimeoutSignal(120000);
    await apiRequest('/test2', { signal: ownSignal });
    expect(fetchMock.mock.calls[1][1].signal).not.toBe(usedSignal);
    expect(fetchMock.mock.calls[1][1].signal).toBe(ownSignal);
  });
});

describe('401 auto-refresh', () => {
  let fetchMock;

  beforeEach(async () => {
    setActivePinia(createPinia());
    localStorage.clear();
    routerPush.mockReset();
    const { useAuthStore } = await import('@/stores/auth');
    useAuthStore().setTokens('old-access');
    fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('on 401 refreshes once and retries with new token', async () => {
    fetchMock
      .mockResolvedValueOnce(errJson({ success: false, error: 'Missing' }, 401))
      .mockResolvedValueOnce(okJson({ success: true, data: { token: 'new-access', refreshToken: 'new-refresh' } }))
      .mockResolvedValueOnce(okJson({ success: true, data: { id: 42 } }));

    const resp = await apiRequest('/users/me');

    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(fetchMock.mock.calls[0][0]).toBe('/api/users/me');
    expect(fetchMock.mock.calls[1][0]).toBe('/api/refresh-token');
    expect(fetchMock.mock.calls[2][0]).toBe('/api/users/me');
    expect(fetchMock.mock.calls[2][1].headers.Authorization).toBe('Bearer new-access');
    expect(await resp.json()).toEqual({ id: 42 });

    const { useAuthStore } = await import('@/stores/auth');
    expect(useAuthStore().token).toBe('new-access');
  });

  it('on refresh failure clears tokens and redirects to /', async () => {
    fetchMock
      .mockResolvedValueOnce(errJson({ success: false, error: 'Missing' }, 401))
      .mockResolvedValueOnce(errJson({ success: false, error: 'Invalid refresh token' }, 401));

    await apiRequest('/users/me');

    const { useAuthStore } = await import('@/stores/auth');
    expect(useAuthStore().token).toBeNull();
    expect(routerPush).toHaveBeenCalledWith('/');
  });

  it('does not try to refresh when /login itself returns 401', async () => {
    fetchMock.mockResolvedValueOnce(errJson({ success: false, error: 'bad creds' }, 401));

    const resp = await apiRequest('/login', { method: 'POST', body: '{}' });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(resp.status).toBe(401);
  });

  it('does not try to refresh when /refresh-token itself returns 401', async () => {
    fetchMock.mockResolvedValueOnce(errJson({ success: false, error: 'bad' }, 401));

    const resp = await apiRequestRaw('/refresh-token', {
      method: 'POST',
      body: JSON.stringify({ refresh_token: 'x' }),
    });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(resp.status).toBe(401);
  });

  it('deduplicates parallel 401s into one refresh call', async () => {
    const unauthorized = () => errJson({ success: false, error: 'Missing' }, 401);
    const ok = (data) => okJson({ success: true, data });

    let refreshCalls = 0;
    fetchMock.mockImplementation(async (url) => {
      if (url === '/api/refresh-token') {
        refreshCalls++;
        await new Promise((r) => setTimeout(r, 10));
        return ok({ token: 'new-access', refreshToken: 'new-refresh' });
      }
      // First-time protected calls return 401; retries succeed (detected via header).
      const callCount = fetchMock.mock.calls.filter((c) => c[0] === url).length;
      return callCount === 1 ? unauthorized() : ok({ ok: true, path: url });
    });

    const [r1, r2, r3] = await Promise.all([
      apiRequest('/users/me'),
      apiRequest('/permissions/my'),
      apiRequest('/applications/unread-count'),
    ]);

    expect(refreshCalls).toBe(1);
    expect(r1.status).toBe(200);
    expect(r2.status).toBe(200);
    expect(r3.status).toBe(200);

    const { useAuthStore } = await import('@/stores/auth');
    expect(useAuthStore().token).toBe('new-access');
  });

  it('apiRequestRaw exposes raw envelope without unwrap', async () => {
    fetchMock.mockResolvedValueOnce(okJson({ success: true, data: { x: 1 }, meta: { total: 7 } }));

    const resp = await apiRequestRaw('/items');
    expect(await resp.json()).toEqual({ success: true, data: { x: 1 }, meta: { total: 7 } });
  });
});

// #2016: кратковременная недоступность базы отвечает /refresh-token пятисоткой, а не
// 401. Раньше performRefresh кидал ошибку на ЛЮБОЙ !response.ok, и catch в baseRequest
// стирал токены и уводил на форму входа - фоновое продление сессии разлогинивало
// человека посреди работы. Теперь 500 ретраится, и только настоящий 401 (токен
// отклонён) рвёт сессию.
describe('500 при обновлении сессии (#2016)', () => {
  let fetchMock;

  beforeEach(async () => {
    setActivePinia(createPinia());
    localStorage.clear();
    routerPush.mockReset();
    const { useAuthStore } = await import('@/stores/auth');
    useAuthStore().setTokens('old-access');
    fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('после исчерпания ретраев не стирает токены и не редиректит на /', async () => {
    vi.useFakeTimers();
    fetchMock
      .mockResolvedValueOnce(errJson({ success: false, error: 'Missing' }, 401)) // защищённый запрос
      .mockResolvedValueOnce(errJson({ success: false, error: 'db down' }, 500)) // refresh, попытка 1
      .mockResolvedValueOnce(errJson({ success: false, error: 'db down' }, 500)) // refresh, попытка 2
      .mockResolvedValueOnce(errJson({ success: false, error: 'db down' }, 500)); // refresh, попытка 3

    const p = apiRequest('/users/me');
    await vi.advanceTimersByTimeAsync(3000);
    const resp = await p;

    expect(fetchMock).toHaveBeenCalledTimes(4);
    expect(fetchMock.mock.calls[1][0]).toBe('/api/refresh-token');
    expect(fetchMock.mock.calls[2][0]).toBe('/api/refresh-token');
    expect(fetchMock.mock.calls[3][0]).toBe('/api/refresh-token');
    // Исходный 401 уходит вызывающему коду как есть - именно он покажет ошибку,
    // а не полный выход из системы.
    expect(resp.status).toBe(401);

    const { useAuthStore } = await import('@/stores/auth');
    expect(useAuthStore().token).toBe('old-access');
    expect(routerPush).not.toHaveBeenCalled();
  });

  it('если база оживает между ретраями, продлевает сессию без разлогина', async () => {
    vi.useFakeTimers();
    fetchMock
      .mockResolvedValueOnce(errJson({ success: false, error: 'Missing' }, 401)) // защищённый запрос
      .mockResolvedValueOnce(errJson({ success: false, error: 'db down' }, 500)) // refresh, попытка 1
      .mockResolvedValueOnce(okJson({ success: true, data: { token: 'new-access', refreshToken: 'new-refresh' } })) // refresh, попытка 2
      .mockResolvedValueOnce(okJson({ success: true, data: { id: 42 } })); // повтор защищённого запроса

    const p = apiRequest('/users/me');
    await vi.advanceTimersByTimeAsync(3000);
    const resp = await p;

    expect(fetchMock).toHaveBeenCalledTimes(4);
    expect(await resp.json()).toEqual({ id: 42 });

    const { useAuthStore } = await import('@/stores/auth');
    expect(useAuthStore().token).toBe('new-access');
    expect(routerPush).not.toHaveBeenCalled();
  });

  it('настоящий 401 от /refresh-token по-прежнему рвёт сессию без ожидания ретраев', async () => {
    fetchMock
      .mockResolvedValueOnce(errJson({ success: false, error: 'Missing' }, 401))
      .mockResolvedValueOnce(errJson({ success: false, error: 'Invalid refresh token' }, 401));

    await apiRequest('/users/me');

    // Токен точно недействителен - ретраить нечего, второго обращения к
    // /refresh-token быть не должно.
    expect(fetchMock).toHaveBeenCalledTimes(2);

    const { useAuthStore } = await import('@/stores/auth');
    expect(useAuthStore().token).toBeNull();
    expect(routerPush).toHaveBeenCalledWith('/');
  });
});

describe('403 handling', () => {
  let fetchMock;

  beforeEach(async () => {
    setActivePinia(createPinia());
    localStorage.clear();
    routerPush.mockReset();
    notifyMock.mockReset();
    _resetDedup403();
    fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('показывает notify с типом error при 403 на обычном пути', async () => {
    fetchMock.mockResolvedValueOnce(errJson({ banned: false }, 403));

    await apiRequest('/applications/1/confirm-pass', { method: 'POST' });

    expect(notifyMock).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'error', prefix: 'Недостаточно прав для этого действия.' })
    );
  });

  it('НЕ показывает тост 403 для заблокированной учётки (banned:true) -- плашка блокировки уже всё объясняет', async () => {
    fetchMock.mockResolvedValueOnce(errJson({ banned: true }, 403));

    await apiRequest('/applications', { method: 'POST' });

    expect(notifyMock).not.toHaveBeenCalled();
  });

  // #1567: гейт согласия отбивает protected-запросы 403 с маркером consent_required.
  // Клиент обязан поднять флаг стора и промолчать - иначе пользователь получает
  // стену тостов «Недостаточно прав» вместо окна согласия.
  it('маркер consent_required в теле поднимает флаг согласия и не тостит', async () => {
    fetchMock.mockResolvedValueOnce(errJson({ success: false, consent_required: true }, 403));

    await apiRequest('/applications', { method: 'POST' });

    expect(notifyMock).not.toHaveBeenCalled();
    expect(usePDConsentStore().required).toBe(true);
  });

  it('маркер согласия в заголовке распознаётся так же, как в теле', async () => {
    fetchMock.mockResolvedValueOnce(new Response('', {
      status: 403,
      headers: { 'Content-Type': 'application/json', 'X-PD-Consent-Required': '1' },
    }));

    await apiRequest('/notifications');

    expect(notifyMock).not.toHaveBeenCalled();
    expect(usePDConsentStore().required).toBe(true);
  });

  // Опознаём по маркеру ОТВЕТА, а не по флагу стора: устаревший флаг заглушил бы
  // настоящие отказы в правах.
  it('обычный 403 без маркера флаг согласия не трогает и тостит', async () => {
    fetchMock.mockResolvedValueOnce(errJson({ banned: false }, 403));

    await apiRequest('/applications/1/confirm-pass', { method: 'POST' });

    expect(usePDConsentStore().required).toBe(false);
    expect(notifyMock).toHaveBeenCalled();
  });

  // #1911: гейт обязательной смены пароля отбивает запросы 403 с кодом
  // PASSWORD_CHANGE_REQUIRED. Клиент поднимает флаг и молчит - иначе вместо окна
  // смены человек получает стену «Недостаточно прав» и не понимает, что от него ждут.
  it('код PASSWORD_CHANGE_REQUIRED в теле поднимает флаг смены пароля и не тостит', async () => {
    fetchMock.mockResolvedValueOnce(errJson({ success: false, code: 'PASSWORD_CHANGE_REQUIRED' }, 403));

    await apiRequest('/applications', { method: 'POST' });

    expect(notifyMock).not.toHaveBeenCalled();
    expect(usePasswordChangeStore().required).toBe(true);
  });

  it('маркер смены пароля в заголовке распознаётся так же, как в теле', async () => {
    fetchMock.mockResolvedValueOnce(new Response('', {
      status: 403,
      headers: { 'Content-Type': 'application/json', 'X-Password-Change-Required': '1' },
    }));

    await apiRequest('/notifications');

    expect(notifyMock).not.toHaveBeenCalled();
    expect(usePasswordChangeStore().required).toBe(true);
  });

  it('обычный 403 без маркера флаг смены пароля не трогает', async () => {
    fetchMock.mockResolvedValueOnce(errJson({ banned: false }, 403));

    await apiRequest('/applications/1/confirm-pass', { method: 'POST' });

    expect(usePasswordChangeStore().required).toBe(false);
    expect(notifyMock).toHaveBeenCalled();
  });

  // Гейт согласия стоит на сервере раньше, и его маркер не должен путаться с этим:
  // иначе окно согласия подменилось бы окном смены пароля.
  it('маркер согласия не поднимает флаг смены пароля', async () => {
    fetchMock.mockResolvedValueOnce(errJson({ success: false, consent_required: true }, 403));

    await apiRequest('/applications', { method: 'POST' });

    expect(usePasswordChangeStore().required).toBe(false);
  });

  it('не вызывает notify для билета real-time потока (/events/ticket)', async () => {
    fetchMock.mockResolvedValueOnce(errJson({ banned: false }, 403));

    await apiRequest('/events/ticket', { method: 'POST' });

    await Promise.resolve();
    expect(notifyMock).not.toHaveBeenCalled();
  });

  it('не вызывает notify при silent403:true', async () => {
    fetchMock.mockResolvedValueOnce(errJson({ banned: false }, 403));

    await apiRequest('/applications/1/action', { method: 'POST', silent403: true });

    await Promise.resolve();
    expect(notifyMock).not.toHaveBeenCalled();
  });

  it('не вызывает notify для тихих путей (/permissions/my)', async () => {
    fetchMock.mockResolvedValueOnce(errJson({ banned: false }, 403));

    await apiRequest('/permissions/my');

    await Promise.resolve();
    expect(notifyMock).not.toHaveBeenCalled();
  });

  it('не вызывает notify для тихих путей (/users/me)', async () => {
    fetchMock.mockResolvedValueOnce(errJson({ banned: false }, 403));

    await apiRequest('/users/me');

    await Promise.resolve();
    expect(notifyMock).not.toHaveBeenCalled();
  });

  it('не вызывает notify для тихих путей (/unique-cars/lookup)', async () => {
    fetchMock.mockResolvedValueOnce(errJson({ banned: false }, 403));

    await apiRequest('/unique-cars/lookup?q=test');

    await Promise.resolve();
    expect(notifyMock).not.toHaveBeenCalled();
  });

  // Список тихих путей пополняется по мере того, как фоновые запросы упираются в
  // право (подсказки справочников, билет потока, сквозной поиск). Ходим по нему
  // целиком: перечисленные руками пути покрывают лишь часть, и новый префикс
  // приезжает без проверки.
  it.each(SILENT_403_PREFIXES)('не вызывает notify для тихого пути %s', async (prefix) => {
    fetchMock.mockResolvedValueOnce(errJson({ banned: false }, 403));

    await apiRequest(prefix);

    await Promise.resolve();
    expect(notifyMock).not.toHaveBeenCalled();
  });

  it('дедуплицирует: два параллельных 403 с одним текстом = один notify', async () => {
    fetchMock.mockResolvedValue(errJson({ banned: false }, 403));

    await Promise.all([
      apiRequest('/applications/1/confirm-pass', { method: 'POST' }),
      apiRequest('/applications/1/confirm-pass', { method: 'POST' }),
    ]);

    expect(notifyMock).toHaveBeenCalledTimes(1);
  });

  it('возвращает response со статусом 403 (вызывающий код может обработать)', async () => {
    fetchMock.mockResolvedValueOnce(errJson({ banned: false }, 403));

    const resp = await apiRequest('/some-action', { method: 'POST' });

    expect(resp.status).toBe(403);
  });
});

describe('429 handling', () => {
  let fetchMock;

  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
    routerPush.mockReset();
    notifyMock.mockReset();
    _resetDedup403();
    fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('на 429 ждёт Retry-After и повторяет запрос; при успехе повтора тоста нет', async () => {
    vi.useFakeTimers();
    fetchMock
      .mockResolvedValueOnce(err429(1))
      .mockResolvedValueOnce(okJson({ success: true, data: { ok: 1 } }));

    const p = apiRequest('/cars/1/territory-status', { method: 'PUT', body: '{}' });
    await vi.advanceTimersByTimeAsync(1500);
    const resp = await p;

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(resp.status).toBe(200);
    expect(notifyMock).not.toHaveBeenCalled();
  });

  it('ограничивает ожидание потолком: Retry-After 60с не ждём дольше 6с на попытку', async () => {
    vi.useFakeTimers();
    fetchMock
      .mockResolvedValueOnce(err429(60))
      .mockResolvedValueOnce(okJson({ success: true, data: { ok: 1 } }));

    const p = apiRequest('/x', { method: 'PUT', body: '{}' });
    await vi.advanceTimersByTimeAsync(6000);
    const resp = await p;

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(resp.status).toBe(200);
  });

  it('после исчерпания попыток показывает notify с типом warning и возвращает 429', async () => {
    vi.useFakeTimers();
    fetchMock.mockResolvedValue(err429(2));

    const p = apiRequest('/x', { method: 'PUT', body: '{}' });
    await vi.advanceTimersByTimeAsync(10000);
    const resp = await p;

    // 3 попытки: первичная + 2 ретрая (MAX_429_RETRIES).
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(resp.status).toBe(429);
    expect(notifyMock).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'warning', prefix: expect.stringContaining('Слишком много запросов') })
    );
  });

  it('silent: 429 не ретраится и не тостится', async () => {
    fetchMock.mockResolvedValue(err429(2));

    const resp = await apiRequest('/notifications', { silent: true });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(resp.status).toBe(429);
    expect(notifyMock).not.toHaveBeenCalled();
  });

  it('/login 429 не перехватывается глобально (у логина свой таймер)', async () => {
    fetchMock.mockResolvedValueOnce(err429(60));

    const resp = await apiRequest('/login', { method: 'POST', body: '{}' });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(resp.status).toBe(429);
    expect(notifyMock).not.toHaveBeenCalled();
  });
});
