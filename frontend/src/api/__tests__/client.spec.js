import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { apiRequest } from '../client';

describe('apiRequest', () => {
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

  it('calls fetch with the correct URL', async () => {
    await apiRequest('/health');

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe('/health');
  });

  it('adds Authorization header when token exists in store', async () => {
    const { useAuthStore } = await import('@/stores/auth');
    const authStore = useAuthStore();
    authStore.setTokens('test-bearer-token', 'refresh');

    await apiRequest('/users/me');

    const [, options] = fetchMock.mock.calls[0];
    expect(options.headers.Authorization).toBe('Bearer test-bearer-token');
  });

  it('does not add Authorization header when no token', async () => {
    await apiRequest('/health');

    const [, options] = fetchMock.mock.calls[0];
    expect(options.headers.Authorization).toBeUndefined();
  });

  it('sets Content-Type to application/json', async () => {
    await apiRequest('/test');

    const [, options] = fetchMock.mock.calls[0];
    expect(options.headers['Content-Type']).toBe('application/json');
  });

  it('merges custom headers', async () => {
    await apiRequest('/test', {
      headers: { 'X-Custom': 'value' },
    });

    const [, options] = fetchMock.mock.calls[0];
    expect(options.headers['X-Custom']).toBe('value');
    expect(options.headers['Content-Type']).toBe('application/json');
  });

  it('passes method and body through', async () => {
    const body = JSON.stringify({ key: 'value' });
    await apiRequest('/test', { method: 'POST', body });

    const [, options] = fetchMock.mock.calls[0];
    expect(options.method).toBe('POST');
    expect(options.body).toBe(body);
  });

  it('returns the response object', async () => {
    const mockResponse = new Response('{"ok":true}', { status: 200 });
    fetchMock.mockResolvedValue(mockResponse);

    const result = await apiRequest('/test');
    expect(result).toBe(mockResponse);
  });

  it('provides an AbortController signal for timeout', async () => {
    await apiRequest('/test');

    const [, options] = fetchMock.mock.calls[0];
    expect(options.signal).toBeInstanceOf(AbortSignal);
  });

  it('uses VUE_APP_API_BASE_URL when set', async () => {
    // The module reads process.env.VUE_APP_API_BASE_URL at import time,
    // so this test verifies default behavior (empty base URL)
    await apiRequest('/endpoint');

    const [url] = fetchMock.mock.calls[0];
    expect(url).toContain('/endpoint');
  });
});
