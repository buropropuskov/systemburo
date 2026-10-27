import { beforeEach, describe, expect, it, vi } from 'vitest';

const api = vi.hoisted(() => ({ fact: vi.fn(), approval: vi.fn(), own: vi.fn() }));
vi.mock('@/api/onboarding', () => ({ getSecurityFactRoute: api.fact }));
vi.mock('@/api/approvers', () => ({ getMyApprovalRole: api.approval }));
vi.mock('@/api/applications', () => ({ getUserApplicationsPaginated: api.own }));

function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

let createGatingData;
const actor = { userId: 101 };
beforeEach(async () => {
  vi.resetModules();
  Object.values(api).forEach(mock => mock.mockReset());
  actor.userId = 101;
  vi.doMock('@/stores/auth', () => ({ useAuthStore: () => actor }));
  ({ createGatingData } = await import('../gatingData'));
});

const cases = [
  { name: 'fact route', mock: api.fact, ensure: 'ensureFactRoute', read: state => state.factTableRoute.value, old: '/table/old', fresh: '/table/new', empty: null, expected: '/table/new', expectedEmpty: null },
  { name: 'approval role', mock: api.approval, ensure: 'ensureApprovalRole', read: state => state.approvalRole.value, old: { is_approver: true }, fresh: { is_reviewer: true }, empty: {}, expected: { isApprover: false, isReviewer: true }, expectedEmpty: { isApprover: false, isReviewer: false } },
  { name: 'own application', mock: api.own, ensure: 'ensureOwnApplication', read: state => state.hasOwnApplication.value, old: { items: [], meta: { total: 0 } }, fresh: { items: [{ id: 22 }], meta: { total: 1 } }, empty: { items: [], meta: { total: 0 } }, expected: true, expectedEmpty: false },
];

describe.each(cases)('$name context', scenario => {
  it('all concurrent callers wait for the result and successful data is cached', async () => {
    const request = deferred();
    scenario.mock.mockReturnValue(request.promise);
    const state = createGatingData();
    const first = state[scenario.ensure]();
    let secondSettled = false;
    const second = state[scenario.ensure]().then(() => { secondSettled = true; });
    // Both callers have returned a promise; neither may report readiness yet.
    await Promise.resolve(); await Promise.resolve();
    expect(secondSettled).toBe(false);
    request.resolve(scenario.fresh);
    await Promise.all([first, second]);
    expect(scenario.read(state)).toEqual(scenario.expected);
    await state[scenario.ensure]();
    expect(scenario.mock).toHaveBeenCalledTimes(1);
  });

  it('caches a successful empty result without repeating the request', async () => {
    scenario.mock.mockResolvedValue(scenario.empty);
    const state = createGatingData();
    await state[scenario.ensure](); await state[scenario.ensure]();
    expect(scenario.read(state)).toEqual(scenario.expectedEmpty);
    expect(scenario.mock).toHaveBeenCalledTimes(1);
  });

  it('retries after a rejected request instead of permanently caching emptiness', async () => {
    scenario.mock.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(scenario.fresh);
    const state = createGatingData();
    await state[scenario.ensure]().catch(() => {});
    await state[scenario.ensure]();
    expect(scenario.mock).toHaveBeenCalledTimes(2);
    expect(scenario.read(state)).toEqual(scenario.expected);
  });

  it.each(['resolve', 'reject'])('an old %s after reset cannot replace the new result', async settlement => {
    const old = deferred();
    scenario.mock.mockReturnValueOnce(old.promise).mockResolvedValueOnce(scenario.fresh);
    const state = createGatingData();
    const first = state[scenario.ensure]().catch(() => {});
    await vi.waitFor(() => expect(scenario.mock).toHaveBeenCalledTimes(1));
    state.reset(); actor.userId = 202;
    await state[scenario.ensure]();
    old[settlement](settlement === 'resolve' ? scenario.old : new Error('old failure'));
    await first;
    expect(scenario.read(state)).toEqual(scenario.expected);
    await state[scenario.ensure]();
    expect(scenario.mock).toHaveBeenCalledTimes(2);
  });

  it('old completion cannot release or mark a newer pending request ready', async () => {
    const old = deferred(), fresh = deferred();
    scenario.mock.mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise);
    const state = createGatingData();
    const first = state[scenario.ensure]().catch(() => {});
    await vi.waitFor(() => expect(scenario.mock).toHaveBeenCalledTimes(1));
    state.reset(); actor.userId = 202;
    const second = state[scenario.ensure]();
    await vi.waitFor(() => expect(scenario.mock).toHaveBeenCalledTimes(2));
    old.reject(new Error('stale'));
    await first;
    let thirdSettled = false;
    const third = state[scenario.ensure]().then(() => { thirdSettled = true; });
    await Promise.resolve(); await Promise.resolve();
    expect(thirdSettled).toBe(false);
    expect(scenario.mock).toHaveBeenCalledTimes(2);
    fresh.resolve(scenario.fresh);
    await Promise.all([second, third]);
    expect(scenario.read(state)).toEqual(scenario.expected);
  });
});

describe('own application identity boundary', () => {
  it('uses the current sender filter and only asks for one record', async () => {
    api.own.mockResolvedValue({ items: [], meta: { total: 0 } });
    await createGatingData().ensureOwnApplication();
    expect(api.own).toHaveBeenCalledWith({ page: 1, per_page: 1, sender_user_id: 101 });
  });

  it('reset while the auth import is pending prevents its old API request', async () => {
    const entered = deferred(), release = deferred();
    vi.doMock('@/stores/auth', async () => {
      entered.resolve();
      await release.promise;
      return { useAuthStore: () => actor };
    });
    const state = createGatingData();
    const pending = state.ensureOwnApplication();
    await entered.promise;
    state.reset(); actor.userId = 202;
    release.resolve();
    await pending;
    expect(api.own).not.toHaveBeenCalled();
    api.own.mockResolvedValue({ items: [], meta: { total: 0 } });
    await state.ensureOwnApplication();
    expect(api.own).toHaveBeenCalledWith({ page: 1, per_page: 1, sender_user_id: 202 });
  });
});
