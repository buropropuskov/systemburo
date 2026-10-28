import { beforeEach, describe, expect, it, vi } from 'vitest';
import { reactive, ref } from 'vue';
import { createContextReadiness } from '../contextReadiness';
import { pickAutostartTour } from '../tours';

function deferred() { let resolve; const promise = new Promise(done => { resolve = done; }); return { promise, resolve }; }
let auth, permissions, data, readiness, generation;
function role(isApprover = false, isReviewer = false) {
  data.approvalRole.value = { isApprover, isReviewer }; data.approvalRoleLoaded.value = true;
}
function context() {
  return { isAuthenticated: true, isSecurity: readiness.knownType() === 'security',
    can: permissions.hasPermission, approvalRole: data.approvalRole.value };
}
beforeEach(() => {
  generation = 0;
  auth = reactive({ userId: 101, userTypeCode: 'user' });
  const grants = new Set(['page.center']);
  permissions = { loaded: true, hasPermission: key => grants.has(key),
    grant: key => grants.add(key), fetchPermissions: vi.fn(async () => {}) };
  data = { approvalRole: ref({ isApprover: false, isReviewer: false }), approvalRoleLoaded: ref(false),
    factRouteLoaded: ref(false), ownApplicationLoaded: ref(false), userTypeCode: ref('user'), getGeneration: () => generation,
    ensureUserType: vi.fn(async () => false), ensureApprovalRole: vi.fn(async () => false),
    ensureFactRoute: vi.fn(async () => false), ensureOwnApplication: vi.fn(async () => false) };
  readiness = createContextReadiness({ auth, permissions, data, getContext: context });
});
describe('per-tour readiness', () => {
  it('does not select the user fallback when the relevant role request fails; retries the real role', async () => {
    expect((await readiness.ensureSelectionContext()).status).toBe('blocked');
    data.ensureApprovalRole.mockImplementation(async () => { role(true); return true; });
    expect((await readiness.ensureSelectionContext()).status).toBe('ready');
    expect(pickAutostartTour(context(), () => false).key).toBe('accept');
  });
  it.each(['guard', 'admin'])('does not await irrelevant role/own requests for %s', async key => {
    if (key === 'guard') { data.userTypeCode.value = 'security'; data.factRouteLoaded.value = true; }
    else permissions.grant('page.admin');
    expect((await readiness.ensureSelectionContext()).status).toBe('ready');
    expect((await readiness.prepareTour(key)).status).toBe('ready');
    expect(data.ensureApprovalRole).not.toHaveBeenCalled(); expect(data.ensureOwnApplication).not.toHaveBeenCalled();
  });
  it('does not turn an unknown type into an ordinary user', async () => {
    data.userTypeCode.value = null; permissions.hasPermission = () => false;
    expect(await readiness.ensureSelectionContext()).toMatchObject({ status: 'blocked', missing: ['type'] });
  });
  it('manual user preparation waits only for its own application context', async () => {
    data.ensureOwnApplication.mockImplementation(async () => { data.ownApplicationLoaded.value = true; });
    expect((await readiness.prepareTour('user')).status).toBe('ready');
    expect(data.ensureApprovalRole).not.toHaveBeenCalled(); expect(data.ensureFactRoute).not.toHaveBeenCalled();
  });
  it('manual accept does not wait for own/fact endpoints', async () => {
    data.ensureApprovalRole.mockImplementation(async () => { role(true); });
    expect((await readiness.prepareTour('accept')).status).toBe('ready');
    expect(data.ensureOwnApplication).not.toHaveBeenCalled(); expect(data.ensureFactRoute).not.toHaveBeenCalled();
  });
  it('does not prepare a known unavailable page', async () => {
    permissions.hasPermission = () => false;
    expect((await readiness.prepareTour('accept')).status).toBe('unavailable');
    expect(data.ensureApprovalRole).not.toHaveBeenCalled();
  });
  it('keeps own-data failure blocked rather than using a demo', async () => {
    expect(await readiness.prepareTour('user')).toMatchObject({ status: 'blocked', missing: ['own'] });
    expect(readiness.status('user').status).toBe('blocked');
  });
  it('rejects an old completion after switching identity or reset', async () => {
    const held = deferred(); data.ensureOwnApplication.mockReturnValue(held.promise);
    const pending = readiness.prepareTour('user'); await Promise.resolve();
    generation++; auth.userId = 202; held.resolve(true);
    expect((await pending).status).toBe('stale');
  });
  it('known negative approval roles permit the genuine user fallback', async () => {
    role(); expect((await readiness.ensureSelectionContext()).status).toBe('ready');
    expect(pickAutostartTour(context(), () => false).key).toBe('user');
  });
});
