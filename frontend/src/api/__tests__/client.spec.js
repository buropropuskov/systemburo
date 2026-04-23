import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';

const routerPush = vi.fn();
vi.mock('@/router', () => ({
  default: {
    push: (...args) => routerPush(...args),
    currentRoute: { value: { path: '/personal-cabinet' } },
  },
}));

import { apiRequest, apiRequestRaw } from '../client';

function okJson(body, init = {}) {
  return new Response(JSON.stringify(body), {
    status: init.status ?? 200,
    headers: { 'Content-Type': 'application/json' },
  });
}
function errJson(body, status) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
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
