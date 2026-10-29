import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { nextTick } from 'vue';
import { useOnboardingStore } from '../onboarding';
import { usePermissionsStore } from '../permissions';
import { useAuthStore } from '../auth';
import { isStepApplicable } from '@/components/onboarding/stepApplicability';
import { onboardingSteps } from '@/components/onboarding/onboardingSteps';
import { approveOnboardingSteps } from '@/components/onboarding/approveOnboardingSteps';
import { acceptCardSteps } from '@/components/onboarding/acceptCardSteps';
import { adminOnboardingSteps } from '@/components/onboarding/adminOnboardingSteps';
import { tourChapters } from '@/components/onboarding/stepsFlow';
import { getMe } from '@/api/auth';

vi.mock('@/api/onboarding', () => ({ getOnboardingStatus: vi.fn(), markOnboardingComplete: vi.fn(), getSecurityPostContext: vi.fn() }));
vi.mock('@/api/approvers', () => ({ getMyApprovalRole: vi.fn() }));
vi.mock('@/api/applications', () => ({ getUserApplicationsPaginated: vi.fn(), getAccessibleAttachments: vi.fn(async () => ({ items: [{ id: 1 }] })) }));
vi.mock('@/api/auth', async original => ({ ...await original(), getMe: vi.fn() }));
vi.mock('@/components/onboarding/demoBackend', () => ({ syncDemoBackend: vi.fn() }));

beforeEach(() => {
  localStorage.clear(); setActivePinia(createPinia());
  useAuthStore().setTokens(`${btoa('{}')}.${btoa(JSON.stringify({ username: 'fixture', user_id: 42, exp: 9999999999 }))}.signature`);
  const permissions = usePermissionsStore(); permissions.loaded = true;
  const store = useOnboardingStore(); store.ownApplicationLoaded = true; store.factRouteLoaded = true;
  store.approvalRoleLoaded = true; store.approvalRole = { isApprover: true, isReviewer: true };
  useAuthStore().userTypeCode = 'user';
  getMe.mockResolvedValue({ user_type_code: 'user' });
});
function allow(...keys) {
  const permissions = usePermissionsStore();
  permissions.effective = Object.fromEntries(keys.map(key => [key, { value: 'allow' }]));
  useOnboardingStore().accessibleContext = { hasItems: true };
  useOnboardingStore().accessibleContextLoaded = true;
}
function deny(key) {
  const permissions = usePermissionsStore(); permissions.denied.add(key);
  permissions.effective[key] = { value: 'deny' };
}
function userTour() { const store = useOnboardingStore(); store.start({ tour: 'user' }); return store; }

describe('learning follows available entry points', () => {
  it('excludes both report steps before DOM preparation when only other table actions are allowed', () => {
    allow('page.available', 'table.kpp_4.export');
    const store = useOnboardingStore();
    store.securityPostContext = { route: '/table/kpp_4', tableType: 'cars', showFactTable: true };
    store.start({ tour: 'guard' });
    expect(store.steps.map(step => step.id)).not.toContain('sec-fact-report');
    expect(store.steps.map(step => step.id)).not.toContain('sec-fact-report-window');
    expect(store.steps.map(step => step.id)).toContain('sec-fact-intro');
  });
  it('rebases a current report immediately after its permission is revoked', async () => {
    allow('page.available', 'table.kpp_4.report');
    const store = useOnboardingStore();
    store.securityPostContext = { route: '/table/kpp_4', tableType: 'cars', showFactTable: true };
    store.start({ tour: 'guard' });
    store.setIndex(store.steps.findIndex(step => step.id === 'sec-fact-report-window'));
    const previousRevision = store.stepsRevision;
    deny('table.kpp_4.report'); await nextTick();
    expect(store.steps.map(step => step.id)).not.toContain('sec-fact-report');
    expect(store.steps.map(step => step.id)).not.toContain('sec-fact-report-window');
    expect(store.currentStepId).not.toBe('sec-fact-report-window');
    expect(store.stepsRevision).toBeGreaterThan(previousRevision);
    expect(store.totalSteps).toBe(store.steps.length);
  });
  it.each([['page.cars', '/carsview'], ['page.employees', '/employeesview']])('removes the complete %s chapter before DOM work', (permission, route) => {
    allow('page.cars', 'page.employees', 'header.create_application',
      'entity.cars.read', 'entity.cars.write', 'entity.employees.read', 'entity.employees.write'); deny(permission);
    const store = userTour();
    expect(store.steps.some(step => step.route === route)).toBe(false);
    expect(tourChapters(store.steps).some(chapter => chapter.route === route)).toBe(false);
    expect(store.steps.map(step => step.id)).toContain(route === '/carsview' ? 'employees-filters' : 'cars-filters');
    expect(store.totalSteps).toBe(store.steps.length);
  });
  it('keeps application record selection when both registry pages are hidden', () => {
    allow('header.create_application'); deny('page.cars'); deny('page.employees');
    const ids = userTour().steps.map(step => step.id);
    expect(ids).toContain('createapp-car-existing'); expect(ids).toContain('createapp-people-existing');
    expect(ids).not.toContain('cars-table'); expect(ids).not.toContain('employees-table');
    expect(ids).not.toContain('nav-group-data'); expect(ids).not.toContain('cabinet-outro');
  });
  it.each(['page.new_application', 'header.create_application'])('keeps form learning through the allowed %s path', permission => {
    allow(permission); deny(permission === 'page.new_application' ? 'header.create_application' : 'page.new_application');
    const store = userTour();
    expect(store.steps.some(step => step.route === '/new-application')).toBe(true);
    expect(store.steps.some(step => step.id === 'header-submit')).toBe(permission === 'header.create_application');
  });
  it('removes every form step when neither entry point is available', () => {
    deny('page.new_application'); deny('header.create_application');
    const store = userTour();
    expect(store.steps.some(step => step.route === '/new-application')).toBe(false);
    expect(store.steps.some(step => step.id === 'header-submit')).toBe(false);
  });
  it.each([['cars', '/carsview', 'cars-add', 'cars-table'], ['employees', '/employeesview', 'employees-add', 'employees-table']])('keeps %s reading without teaching unavailable creation', (entity, route, add, table) => {
    allow(`page.${entity}`, `entity.${entity}.read`); deny(`entity.${entity}.write`);
    const steps = userTour().steps;
    expect(steps.some(step => step.route === route)).toBe(true);
    expect(steps.map(step => step.id)).toContain(table); expect(steps.map(step => step.id)).not.toContain(add);
  });
  it('keeps the built-in security accessible section despite additive page denial', async () => {
    useAuthStore().userTypeCode = 'security'; deny('page.available');
    getMe.mockResolvedValue({ user_type_code: 'security' });
    const store = useOnboardingStore(); await store.ensureSelectionContext(); await store.prepareTour('guard'); store.start({ tour: 'guard' });
    expect(store.steps.some(step => step.route === '/accessible-attachments')).toBe(true);
  });
  it('does not teach the accessible section to a normal user without its entry point', () => {
    const store = useOnboardingStore(); store.start({ tour: 'guard' });
    expect(store.steps.some(step => step.route === '/accessible-attachments')).toBe(false);
  });
});

describe('application learning uses the same contextual action policy', () => {
  it.each(['apr-detail-forward', 'acc-detail-forward', 'apr-detail-vote'])('retains %s without additive allow but removes explicit denial', id => {
    allow('page.center');
    const store = useOnboardingStore();
    const step = [...approveOnboardingSteps, ...acceptCardSteps].find(step => step.id === id);
    expect(isStepApplicable(step, store.tourContext)).toBe(true);
    deny(id === 'apr-detail-vote' ? 'action.approve.application' : 'action.forward.application');
    expect(isStepApplicable(step, store.tourContext)).toBe(false);
  });
  it('does not wait for center actions when the center itself is unavailable', () => {
    deny('page.center');
    const store = useOnboardingStore(); store.start({ tour: 'approve' });
    expect(store.steps.some(step => step.route === '/center')).toBe(false);
  });
  it.each([['approve', 'apr-nav-center'], ['accept', 'acc-nav-center']])('removes the hidden center entry from %s before DOM preparation', (tour, id) => {
    deny('page.center');
    const store = useOnboardingStore(); store.start({ tour });
    expect(store.steps.map(step => step.id)).not.toContain(id);
    expect(store.steps.some(step => step.route === '/center')).toBe(false);
  });
  it.each([['approve', 'apr-nav-center'], ['accept', 'acc-nav-center']])('rebases %s when its current center entry is revoked', async (tour, id) => {
    allow('page.center');
    const store = useOnboardingStore(); store.start({ tour });
    store.setIndex(store.steps.findIndex(step => step.id === id));
    const previousRevision = store.stepsRevision;
    deny('page.center'); await nextTick();
    expect(store.steps.map(step => step.id)).not.toContain(id);
    expect(store.steps.some(step => step.route === '/center')).toBe(false);
    expect(store.currentStepId).not.toBe(id);
    expect(store.currentStep.id).toBe(store.currentStepId);
    expect(store.stepsRevision).toBe(previousRevision + 1);
  });
  it('does not mistake an unresolved action policy for permission to act', () => {
    allow('page.center'); usePermissionsStore().loaded = false;
    const context = useOnboardingStore().tourContext;
    expect(isStepApplicable(approveOnboardingSteps.find(step => step.id === 'apr-detail-vote'), context)).toBe(false);
  });
});

describe('restricted admin learning', () => {
  it('keeps available settings and neutral bookends while excluding denied pages and analytics', () => {
    const permissions = usePermissionsStore(); permissions.mode = 'admin';
    deny('page.admin.users'); deny('page.admin.settings'); deny('page.statistics');
    const store = useOnboardingStore(); store.start({ tour: 'admin' });
    const ids = store.steps.map(step => step.id);
    expect(ids).not.toContain('admin-users-create'); expect(ids).not.toContain('admin-users-ban');
    expect(ids).not.toContain('admin-users-reset-onboarding'); expect(ids).not.toContain('admin-settings');
    expect(ids).not.toContain('admin-analytics');
    expect(ids).toContain('admin-groups'); expect(ids).toContain('admin-more'); expect(ids).toContain('admin-finish');
  });
  it('checks the actual admin route gate even when metadata only names an action', () => {
    allow('page.admin', 'permission.audit.manage'); deny('page.admin.users');
    const store = useOnboardingStore();
    const step = { id: 'admin-user-example', route: '/admin/users', requires: 'permission.audit.manage' };
    expect(isStepApplicable(step, store.tourContext)).toBe(false);
  });
  it('does not derive a statistics gate from the data-processing route name', () => {
    allow('page.admin'); deny('page.statistics');
    const context = useOnboardingStore().tourContext;
    expect(isStepApplicable(adminOnboardingSteps.find(step => step.id === 'admin-more'), context)).toBe(true);
  });
  it('preserves the configuration order and definition objects for eligible steps', () => {
    usePermissionsStore().mode = 'super';
    const store = userTour(); expect(store.steps).toEqual(onboardingSteps);
  });
});
