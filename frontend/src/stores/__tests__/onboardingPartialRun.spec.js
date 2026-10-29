import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { nextTick } from 'vue';
import { useOnboardingStore } from '../onboarding';
import { useAuthStore } from '../auth';
import { usePermissionsStore } from '../permissions';
import { markOnboardingComplete } from '@/api/onboarding';

vi.mock('@/api/onboarding', () => ({ getOnboardingStatus: vi.fn(), markOnboardingComplete: vi.fn(async () => ({})), getSecurityPostContext: vi.fn() }));
vi.mock('@/api/approvers', () => ({ getMyApprovalRole: vi.fn() }));
vi.mock('@/api/applications', () => ({ getAccessibleAttachments: vi.fn(async () => ({ items: [{ id: 1 }], meta: { total: 1 } })),  getUserApplicationsPaginated: vi.fn(async () => ({ items: [], meta: { total: 0 } })) }));
vi.mock('@/components/onboarding/demoBackend', () => ({ syncDemoBackend: vi.fn() }));
let store, permissions;
beforeEach(async () => {
  localStorage.clear(); vi.clearAllMocks(); setActivePinia(createPinia());
  useAuthStore().setTokens(`${btoa('{}')}.${btoa(JSON.stringify({ username: 'fixture', user_id: 101, exp: 9999999999 }))}.signature`);
  permissions = usePermissionsStore(); permissions.loaded = true;
  store = useOnboardingStore(); await store.prepareTour('user');
});

describe('starting from a selected section', () => {
  it('rejects unknown or unavailable IDs without starting or changing the current tour', () => {
    expect(store.start({ tour: 'user', manual: true, fromStepId: 'cars-add' })).toBe(false);
    expect(store.start({ tour: 'user', manual: true, fromStepId: 'missing' })).toBe(false);
    expect(store.isActive).toBe(false); expect(store.activeTourKey).toBeNull();
  });
  it('allows section selection only for an explicit manual launch', () => {
    const id = store.stepsForTour('user')[1].id;
    expect(store.start({ tour: 'user', fromStepId: id })).toBe(false);
    expect(store.start({ tour: 'user', manual: true, fromStepId: id })).toBe(true);
    expect(store.currentStepId).toBe(id); expect(store.partialRun).toBe(true);
  });
  it('does not mark the full tour as finished after a partial run', () => {
    const id = store.stepsForTour('user')[1].id;
    store.start({ tour: 'user', manual: true, fromStepId: id });
    store.setIndex(store.totalSteps - 1); store.markCompleted(true); store.stop(true);
    expect(markOnboardingComplete).not.toHaveBeenCalled();
    expect(store.hasCompleted('user')).toBe(false); expect(store.hasFinished('user')).toBe(false);
  });
  it('preserves an existing full-run resume position through a partial run and access refresh', async () => {
    store.start({ tour: 'user', manual: true }); store.setIndex(2); store.stop();
    const saved = localStorage.getItem('ob:progress:101:user');
    expect(saved).not.toBeNull();
    const id = store.stepsForTour('user')[1].id;
    store.start({ tour: 'user', manual: true, fromStepId: id });
    store.setIndex(store.totalSteps - 1);
    permissions.effective['page.employees'] = { value: 'allow' }; await nextTick();
    store.markCompleted(true); store.stop(true);
    expect(localStorage.getItem('ob:progress:101:user')).toBe(saved);
    store.start({ tour: 'user', manual: true });
    expect(store.partialRun).toBe(false); expect(store.currentIndex).toBe(2);
  });
  it('keeps existing completed and finished records intact', () => {
    store.completedByTour = { user: 1 }; store.finishedTours = ['user'];
    store.start({ tour: 'user', manual: true, fromStepId: store.stepsForTour('user')[1].id });
    store.markCompleted(true); store.stop(true);
    expect(store.completedByTour).toEqual({ user: 1 }); expect(store.finishedTours).toEqual(['user']);
    expect(markOnboardingComplete).not.toHaveBeenCalled();
  });
  it('does not overwrite full resume when a section is closed before its final step', () => {
    store.start({ tour: 'user', manual: true }); store.setIndex(2); store.stop();
    const saved = localStorage.getItem('ob:progress:101:user');
    store.start({ tour: 'user', manual: true, fromStepId: store.stepsForTour('user')[1].id });
    store.setIndex(1); store.stop(false);
    expect(localStorage.getItem('ob:progress:101:user')).toBe(saved);
    expect(markOnboardingComplete).not.toHaveBeenCalled();
  });
  it('treats starting at the actual first step as a full run', () => {
    const id = store.stepsForTour('user')[0].id;
    store.start({ tour: 'user', manual: true, restart: true, fromStepId: id });
    expect(store.partialRun).toBe(false); expect(store.currentIndex).toBe(0);
    store.markCompleted(true);
    expect(markOnboardingComplete).toHaveBeenCalledOnce(); expect(store.hasFinished('user')).toBe(true);
  });
});
