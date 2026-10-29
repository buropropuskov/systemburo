import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { nextTick, watch } from 'vue';
import { useOnboardingStore } from '../onboarding';
import { useAuthStore } from '../auth';
import { usePermissionsStore } from '../permissions';
import { getSecurityPostContext, getOnboardingStatus, markOnboardingComplete } from '@/api/onboarding';
import { getMyApprovalRole } from '@/api/approvers';
import { getUserApplicationsPaginated, getAccessibleAttachments } from '@/api/applications';
import { getMe } from '@/api/auth';
import { syncDemoBackend } from '@/components/onboarding/demoBackend';
import { createAutostart } from '@/components/onboarding/tourAutostart';
vi.mock('@/api/onboarding', () => ({ getOnboardingStatus: vi.fn(), markOnboardingComplete: vi.fn(), getSecurityPostContext: vi.fn() }));
vi.mock('@/api/approvers', () => ({ getMyApprovalRole: vi.fn() }));
vi.mock('@/api/applications', () => ({ getAccessibleAttachments: vi.fn(async () => ({ items: [{ id: 1 }], meta: { total: 1 } })),  getUserApplicationsPaginated: vi.fn() }));
vi.mock('@/api/auth', async original => ({ ...await original(), getMe: vi.fn() }));
vi.mock('@/components/onboarding/demoBackend', () => ({ syncDemoBackend: vi.fn() }));
function deferred() { let resolve; const promise = new Promise(done => { resolve = done; }); return { promise, resolve }; }
let store, auth, permissions;
function allow(...keys) { permissions.effective = Object.fromEntries(keys.map(key => [key, { value: 'allow' }])); }
beforeEach(() => {
  localStorage.clear(); setActivePinia(createPinia()); vi.resetAllMocks();
  auth = useAuthStore(); auth.setTokens(`${btoa('{}')}.${btoa(JSON.stringify({ username: 'fixture', user_id: 101, exp: 9999999999 }))}.signature`);
  auth.userTypeCode = 'user'; permissions = usePermissionsStore(); permissions.loaded = true;
  getMyApprovalRole.mockResolvedValue({ is_approver: false, is_reviewer: false });
  getUserApplicationsPaginated.mockResolvedValue({ items: [], meta: { total: 0 } });
  getSecurityPostContext.mockResolvedValue(null); getMe.mockResolvedValue({ user_type_code: 'user' });
  getOnboardingStatus.mockResolvedValue({ completed: {}, finished: [] });
  markOnboardingComplete.mockResolvedValue({ message: 'ok' });
  store = useOnboardingStore();
});
describe('real launch resource boundaries', () => {
  it('excludes known empty attachment actions and hidden table links before the first guard step, keeping Fact', async () => {
    allow('page.available');
    getAccessibleAttachments.mockResolvedValueOnce({ items: [], meta: { total: 0 } });
    expect((await store.prepareTour('guard')).status).toBe('ready');
    expect(store.start({ tour: 'guard', manual: true })).toBe(true);
    const ids = store.steps.map(step => step.id);
    expect(ids).not.toContain('sec-nav-tables');
    expect(ids).not.toContain('sec-nav-tables-open');
    expect(ids).not.toContain('sec-aa-card');
    expect(ids).not.toContain('sec-aa-blank');
    expect(ids).toContain('sec-aa-intro');
    expect(ids).toContain('sec-aa-filters');
    expect(ids).toContain('sec-fact-intro');
    expect(store.steps.find(step => step.id === 'sec-fact-intro').optional).not.toBe(true);
    expect(getAccessibleAttachments).toHaveBeenCalledWith({ page: 1, per_page: 1 });
  });

  it('does not turn a failed available-list read into a shortened successful course', async () => {
    allow('page.available');
    getAccessibleAttachments.mockRejectedValueOnce(new Error('offline'));
    expect((await store.prepareTour('guard')).missing).toContain('available');
    expect(store.start({ tour: 'guard', manual: true })).toBe(false);
    expect((await store.prepareTour('guard')).status).toBe('ready');
    expect(store.start({ tour: 'guard', manual: true })).toBe(true);
    expect(store.steps.some(step => step.id === 'sec-aa-card')).toBe(true);
  });

  it('refreshes attachment availability on a later launch without adding requests to step navigation', async () => {
    allow('page.available');
    getAccessibleAttachments.mockResolvedValueOnce({ items: [] });
    await store.prepareTour('guard'); store.start({ tour: 'guard', manual: true });
    expect(store.steps.some(step => step.id === 'sec-aa-card')).toBe(false);
    store.stop();
    getAccessibleAttachments.mockResolvedValueOnce({ items: [{ id: 7 }] });
    await store.prepareTour('guard'); store.start({ tour: 'guard', manual: true });
    expect(store.steps.some(step => step.id === 'sec-aa-card')).toBe(true);
    store.setIndex(1); store.setIndex(2);
    expect(getAccessibleAttachments).toHaveBeenCalledTimes(2);
  });
  it('keeps the current actor context across token rotation', async () => {
    await store.prepareTour('user'); await store.loadStatus();
    expect(store.start({ tour: 'user', manual: true })).toBe(true);
    auth.setTokens(`${btoa('{}')}.${btoa(JSON.stringify({ username: 'fixture', user_id: 101, exp: 9999999998 }))}.rotated`);
    expect(store.isActive).toBe(true);
    expect(store.ownApplicationLoaded).toBe(true);
    expect(store.statusLoaded).toBe(true);
    expect((await store.prepareTour('user')).status).toBe('ready');
    expect(getUserApplicationsPaginated).toHaveBeenCalledOnce();
  });

  it('invalidates cached resources and progress when the actor changes without explicit reset', async () => {
    allow('page.center', 'page.available');
    await store.ensureApprovalRole();
    await store.prepareTour('user');
    await store.prepareTour('guard');
    await store.loadStatus();
    expect(store.start({ tour: 'user', manual: true })).toBe(true);
    const teardown = watch(() => store.isActive, active => { if (!active) store.markCompleted(); }, { flush: 'sync' });
    auth.setTokens(`${btoa('{}')}.${btoa(JSON.stringify({ username: 'next', user_id: 202, exp: 9999999999 }))}.signature`);
    expect(store.isActive).toBe(false);
    expect(store.activeTourKey).toBeNull();
    expect(store.approvalRoleLoaded).toBe(false);
    expect(store.factRouteLoaded).toBe(false);
    expect(store.ownApplicationLoaded).toBe(false);
    expect(store.statusLoaded).toBe(false);
    expect(markOnboardingComplete).not.toHaveBeenCalled();
    expect((await store.prepareTour('user')).status).toBe('ready');
    expect(getUserApplicationsPaginated).toHaveBeenCalledTimes(2);
    teardown();
  });

  it('ignores old resource and status responses after actor switch without explicit reset', async () => {
    const role = deferred(), own = deferred(), post = deferred(), status = deferred();
    getMyApprovalRole.mockReturnValueOnce(role.promise);
    getUserApplicationsPaginated.mockReturnValueOnce(own.promise);
    getSecurityPostContext.mockReturnValueOnce(post.promise);
    getOnboardingStatus.mockReturnValueOnce(status.promise);
    const pending = [store.ensureApprovalRole(), store.ensureOwnApplication(), store.ensureFactRoute(), store.loadStatus()];
    await vi.waitFor(() => expect(getOnboardingStatus).toHaveBeenCalledOnce());
    auth.setTokens(`${btoa('{}')}.${btoa(JSON.stringify({ username: 'next', user_id: 202, exp: 9999999999 }))}.signature`);
    role.resolve({ is_approver: true, is_reviewer: true });
    own.resolve({ items: [{ id: 1 }], meta: { total: 1 } });
    post.resolve({ route: '/table/previous', tableType: 'cars', showFactTable: true });
    status.resolve({ completed: { user: 99 }, finished: ['user'] });
    await Promise.all(pending);
    expect(store.approvalRoleLoaded).toBe(false);
    expect(store.factRouteLoaded).toBe(false);
    expect(store.ownApplicationLoaded).toBe(false);
    expect(store.statusLoaded).toBe(false);
    expect(store.hasCompleted('user')).toBe(false);
    expect(store.securityPostContext).toBeNull();
  });

  it('does not launch a user fallback after an unknown approval role, then retries the right profile', async () => {
    allow('page.center'); store.statusLoaded = true;
    getMyApprovalRole.mockRejectedValueOnce(new Error('network'));
    await createAutostart(store, () => '/news')();
    expect(store.isActive).toBe(false); expect(syncDemoBackend).not.toHaveBeenCalledWith(true, false, 'user');
    getMyApprovalRole.mockResolvedValue({ is_approver: true, is_reviewer: false });
    await createAutostart(store, () => '/news')();
    expect(store.activeTourKey).toBe('accept'); expect(getUserApplicationsPaginated).not.toHaveBeenCalled();
  });
  it('does not enable the own demo on error; retries a confirmed empty result', async () => {
    getUserApplicationsPaginated.mockRejectedValueOnce(new Error('network'));
    expect((await store.prepareTour('user')).status).toBe('blocked');
    expect(store.start({ tour: 'user' })).toBe(false); expect(syncDemoBackend).not.toHaveBeenCalled();
    expect((await store.prepareTour('user')).status).toBe('ready');
    expect(store.start({ tour: 'user' })).toBe(true); expect(syncDemoBackend).toHaveBeenCalledWith(true, false, 'user');
  });
  it('revalidates a cached post after table policy changes but not unrelated grants', async () => {
    auth.userTypeCode = 'security';
    getMe.mockResolvedValue({ user_type_code: 'security' });
    getSecurityPostContext.mockResolvedValue({ route: '/table/post', tableType: 'cars', showFactTable: false });
    expect((await store.prepareTour('guard')).status).toBe('ready'); expect(store.factRouteLoaded).toBe(true);
    store.start({ tour: 'guard' });
    permissions.effective['header.report_problem'] = { value: 'allow' };
    await nextTick(); const total = store.totalSteps;
    expect(store.factRouteLoaded).toBe(true); expect(getSecurityPostContext).toHaveBeenCalledTimes(1);
    permissions.effective['table.post.view'] = { value: 'deny' }; await nextTick();
    expect(store.factRouteLoaded).toBe(false); expect(store.accessReady).toBe(false);
    expect(store.totalSteps).toBe(total);
    getSecurityPostContext.mockResolvedValue(null);
    expect((await store.prepareTour('guard')).status).toBe('ready');
    expect(store.factRouteLoaded).toBe(true); expect(store.securityPostContext).toBe(null);
    expect(getSecurityPostContext).toHaveBeenCalledTimes(2);
  });
  it('rejects the old type result after reset and never writes the auth store', async () => {
    auth.userTypeCode = null; const held = deferred(); getMe.mockReturnValueOnce(held.promise);
    const pending = store.ensureSelectionContext(); await vi.waitFor(() => expect(getMe).toHaveBeenCalledOnce());
    store.reset(); held.resolve({ user_type_code: 'security' });
    expect((await pending).status).toBe('stale'); expect(auth.userTypeCode).toBe(null);
    expect(store.tourContext.isSecurity).toBe(false);
  });

  it('does not trust an unscoped auth type left by the previous actor', async () => {
    auth.setTokens(`${btoa('{}')}.${btoa(JSON.stringify({ username: 'next', user_id: 202, exp: 9999999999 }))}.signature`);
    auth.userTypeCode = 'security'; // A late reply owned by the previous identity.
    getMe.mockResolvedValue({ user_type_code: 'user' });
    expect((await store.ensureSelectionContext()).status).toBe('ready');
    expect(getMe).toHaveBeenCalledOnce();
    expect(store.tourContext.isSecurity).toBe(false);
    expect(store.tourContext.canViewAccessibleAttachments).toBe(false);
    expect((await store.prepareTour('guard')).status).toBe('unavailable');
    expect(getSecurityPostContext).not.toHaveBeenCalled();
  });

  it('resolves the current guard type even when auth still contains an ordinary type', async () => {
    auth.userTypeCode = 'user';
    getMe.mockResolvedValue({ user_type_code: 'security' });
    expect((await store.ensureSelectionContext()).status).toBe('ready');
    expect(store.tourContext.isSecurity).toBe(true);
    expect((await store.prepareTour('guard')).status).toBe('ready');
    expect(store.tourContext.canViewAccessibleAttachments).toBe(true);
    expect(auth.userTypeCode).toBe('user');
  });
});
