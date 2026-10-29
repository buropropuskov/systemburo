import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
vi.mock('@/router', () => ({ default: { currentRoute: { value: { path: '/center' } }, push: vi.fn() } }));
import { apiRequest } from '@/api/client';
import { setReadInterceptor, interceptRead } from '@/api/readInterceptor';
import { createCenterDemoResponder, DEMO_ACTION_MESSAGE } from '../centerDemoResponder';
import { buildDemoCenterScenario } from '../demoCenterScenarios';
const snapshot = () => buildDemoCenterScenario({ scene: 'accept-ready', actorId: 42, now: Date.parse('2026-10-28T12:00:00Z') });
describe('center fictional transport stays local', () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.stubGlobal('fetch', vi.fn()); setReadInterceptor(createCenterDemoResponder(snapshot())); });
  afterEach(() => { setReadInterceptor(null); vi.unstubAllGlobals(); });
  it('has no silent no-actor default', () => { expect(() => createCenterDemoResponder()).toThrow(); });
  it.each(['/applications/999000002/approve', '/applications/999000002/take-to-work', '/attachments/999000102/unknown', '/cars/999000203'])('refuses owned action %s without fetch or success', async path => {
    const response = await apiRequest(path, { method: 'POST' });
    expect(response.status).toBe(409); expect(response.ok).toBe(false);
    expect(await response.json()).toEqual({ message: DEMO_ACTION_MESSAGE }); expect(fetch).not.toHaveBeenCalled();
  });
  it('allows only exact read bookkeeping, not arbitrary writes', async () => {
    const read = await apiRequest('/applications/999000002/read', { method: 'POST' });
    expect(read.ok).toBe(true);expect(await read.json()).toBe(null);expect(fetch).not.toHaveBeenCalled();
  });
  it.each(['/applications/999000002/files/999000501', '/applications/999000002/unknown', '/attachments/999000102/download'])('refuses owned binary/unknown GET %s', async path => {
    const response = await apiRequest(path); expect(response.status).toBe(409);
    expect(await response.json()).toEqual({ message: DEMO_ACTION_MESSAGE });expect(fetch).not.toHaveBeenCalled();
  });
  it('keeps all unrelated memberships, permissions and real IDs untouched', () => {
    for (const path of ['/application-approvers/me', '/application-approvers', '/permissions', '/applications/12/read', '/applications/files', '/cars/7', '/applications/9990000020/details']) expect(interceptRead(path)).toBe(null);
  });
  it('returns independent data and proper list/detail envelopes', async () => {
    const first = await (await apiRequest('/applications')).json(); first[0].status = 'changed';
    const detail = await (await apiRequest('/applications/999000002/details')).json();
    expect(detail.status).toBe('Непрочитано'); expect(detail.responsible_users).toHaveLength(2);
  });
  it('late JSON is cancelled on stop or scene/actor replacement', async () => {
    const response = await apiRequest('/applications/999000002/details');setReadInterceptor(null);
    await expect(response.json()).rejects.toMatchObject({ name: 'AbortError' });
  });
  it('does not revive an old response when the same responder is installed in a new lifetime',async()=>{
    const respond=createCenterDemoResponder(snapshot());setReadInterceptor(respond);
    const response=interceptRead('/applications/999000002/details');setReadInterceptor(null);setReadInterceptor(respond);
    await expect(response.json()).rejects.toMatchObject({name:'AbortError'});
  });
});
