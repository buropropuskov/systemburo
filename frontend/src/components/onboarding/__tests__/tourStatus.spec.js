import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createTourStatus } from '../tourStatus';
import { getOnboardingStatus } from '@/api/onboarding';
vi.mock('@/api/onboarding', () => ({ getOnboardingStatus: vi.fn() }));
function deferred() { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
beforeEach(() => vi.resetAllMocks());
describe('status identity lifetime', () => {
  it.each(['resolve', 'reject'])('ignores an old %s and does not release a new pending load after reset', async settlement => {
    const old = deferred(), fresh = deferred();
    getOnboardingStatus.mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise);
    const state = createTourStatus(); const pending = state.load();
    state.reset(); const next = state.load();
    old[settlement](settlement === 'resolve' ? { completed: { user: 1 }, finished: ['user'] } : new Error('old'));
    await pending;
    expect(state.loaded.value).toBe(false); expect(state.completedByTour.value).toEqual({});
    const same = state.load(); expect(getOnboardingStatus).toHaveBeenCalledTimes(2);
    fresh.resolve({ completed: { guard: 2 }, finished: ['guard'] }); await Promise.all([next, same]);
    expect(state.completedByTour.value).toEqual({ guard: 2 }); expect(state.finishedTours.value).toEqual(['guard']);
  });
});
