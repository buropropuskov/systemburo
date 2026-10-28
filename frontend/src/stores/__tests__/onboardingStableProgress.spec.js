import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { nextTick } from 'vue';
import { useOnboardingStore } from '../onboarding';
import { usePermissionsStore } from '../permissions';
import { useAuthStore } from '../auth';
import { markOnboardingComplete } from '@/api/onboarding';

vi.mock('@/api/onboarding', () => ({ getOnboardingStatus: vi.fn(), markOnboardingComplete: vi.fn(), getSecurityPostContext: vi.fn() }));
vi.mock('@/api/approvers', () => ({ getMyApprovalRole: vi.fn() }));
vi.mock('@/api/applications', () => ({ getUserApplicationsPaginated: vi.fn(async () => ({ items: [], meta: { total: 0 } })) }));
vi.mock('@/components/onboarding/demoBackend', () => ({ syncDemoBackend: vi.fn() }));

const rights = ['page.cars', 'page.employees', 'page.new_application', 'header.create_application',
  'entity.cars.read', 'entity.cars.write', 'entity.employees.read', 'entity.employees.write'];
function effective(keys = rights) { return Object.fromEntries(keys.map(key => [key, { value: 'allow' }])); }
function fixture() {
  const permissions = usePermissionsStore(); permissions.effective = effective(); permissions.loaded = true;
  const store = useOnboardingStore(); store.ownApplicationLoaded = true;
  expect(store.start({ tour: 'user', manual: true })).toBe(true);
  return { store, permissions };
}
beforeEach(() => { localStorage.clear(); setActivePinia(createPinia()); vi.clearAllMocks(); });

describe('active learning set reconciliation', () => {
  it('refuses to start when permissions are still unknown', () => {
    const store = useOnboardingStore();
    expect(store.accessReady).toBe(false);
    expect(store.start({ tour: 'user' })).toBe(false);
    expect(store.isActive).toBe(false);
    expect(store.activeTourKey).toBe(null);
    expect(markOnboardingComplete).not.toHaveBeenCalled();
  });
  it('does not equate a resolved failed permissions fetch with known access', async () => {
    const permissions = usePermissionsStore();
    permissions.fetchPermissions = vi.fn(async () => {});
    useAuthStore().loadUserTypeCode = vi.fn(async () => {});
    const store = useOnboardingStore();
    await expect(store.ensureGatingContext()).resolves.toBe(false);
    expect(permissions.fetchPermissions).toHaveBeenCalledOnce();
    expect(store.accessReady).toBe(false);
    expect(store.start({ tour: 'user' })).toBe(false);
    expect(store.stepsRevision).toBe(0);
    expect(markOnboardingComplete).not.toHaveBeenCalled();
  });
  it('keeps the current semantic step when an earlier chapter disappears', async () => {
    const { store, permissions } = fixture();
    store.setIndex(store.steps.findIndex(step => step.id === 'employees-table'));
    const previous = store.currentIndex;
    store.markSkipped(store.steps.findIndex(step => step.id === 'employees-add'));
    permissions.effective = effective(rights.filter(key => key !== 'page.cars'));
    await nextTick();
    expect(store.currentStepId).toBe('employees-table');
    expect(store.currentStep.id).toBe('employees-table');
    expect(store.currentIndex).toBeLessThan(previous);
    expect(store.skippedIndexes.map(index => store.steps[index].id)).toEqual(['employees-add']);
    expect(store.stepsRevision).toBe(1);
    expect(markOnboardingComplete).not.toHaveBeenCalled();
  });
  it('rebases an unavailable current step to the next remaining semantic step', async () => {
    const { store, permissions } = fixture();
    store.setIndex(store.steps.findIndex(step => step.id === 'cars-table'));
    const priorIds = store.steps.map(step => step.id);
    const priorIndex = store.currentIndex;
    permissions.effective = effective(rights.filter(key => key !== 'page.cars'));
    await nextTick();
    const expectedId = priorIds.slice(priorIndex + 1).find(id => store.steps.some(step => step.id === id));
    expect(store.currentStepId).toBe(expectedId);
    expect(store.currentStep.id).toBe(expectedId);
    expect(store.isActive).toBe(true);
    expect(markOnboardingComplete).not.toHaveBeenCalled();
  });
  it('does not rebase on an intermediate mix of permission fields within one refresh', async () => {
    const { store, permissions } = fixture();
    store.setIndex(store.steps.findIndex(step => step.id === 'cars-table'));
    permissions.effective = effective(rights.filter(key => key !== 'page.cars'));
    permissions.effective = effective();
    await nextTick();
    expect(store.currentStepId).toBe('cars-table');
    expect(store.currentStep.id).toBe('cars-table');
    expect(store.stepsRevision).toBe(0);
  });
  it('freezes the known snapshot while access is unknown, then rechecks when it recovers', async () => {
    const { store, permissions } = fixture();
    store.setIndex(store.steps.findIndex(step => step.id === 'cars-table'));
    const ids = store.steps.map(step => step.id);
    permissions.loaded = false;
    permissions.effective = {};
    await nextTick();
    expect(store.accessReady).toBe(false);
    expect(store.steps.map(step => step.id)).toEqual(ids);
    expect(store.currentStepId).toBe('cars-table');
    expect(store.stepsRevision).toBe(1);
    permissions.effective = effective(); permissions.loaded = true;
    await nextTick();
    expect(store.currentStep.id).toBe('cars-table');
    expect(store.stepsRevision).toBe(2);
    expect(markOnboardingComplete).not.toHaveBeenCalled();
  });
  it('preserves completed/finished information and semantic resume during ordinary stop/start', () => {
    const { store } = fixture();
    store.completedByTour = { admin: 3 }; store.finishedTours = ['admin'];
    store.setIndex(store.steps.findIndex(step => step.id === 'employees-table'));
    store.stop();
    expect(store.start({ tour: 'user', manual: true })).toBe(true);
    expect(store.currentStepId).toBe('employees-table');
    expect(store.completedByTour).toEqual({ admin: 3 });
    expect(store.finishedTours).toEqual(['admin']);
    expect(markOnboardingComplete).not.toHaveBeenCalled();
  });
});
