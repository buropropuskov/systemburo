import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { SRC_ROOT, userFlowImports, userFlowModules } from './helpers/userFlowModules';

describe('user flow graph preserves deferred route boundaries', () => {
  const source = `
    import Shared from './Shared.vue';
    const eager = import('./Eager.js');
    export const routes = [
      { path: '/closed', component: () => import('./Closed.vue'), meta: { requiresAuth: true, permission: 'page.admin' } },
      { path: '/special', component: () => import('./Special.vue'), meta: { requiresAuth: true, requiresSuperAdmin: true } },
      { path: '/dynamic', component: () => import('./Dynamic.vue'), meta: { requiresAuth: true, permission: to => to.params.key } },
      { path: '/open', component: () => import('./Open.vue'), meta: { requiresAuth: true } },
    ];
    function ordinary() { return import('./Closed.vue'); }
  `;
  it('omits only guarded lazy component factories, retaining static and ordinary dynamic imports', () => {
    expect(userFlowImports(source, { routeDefinitions: true })).toEqual([
      './Shared.vue', './Eager.js', './Open.vue', './Closed.vue',
    ]);
  });
  it('follows every import in ordinary modules even if they contain route-like objects', () => {
    expect(userFlowImports(source)).toContain('./Special.vue');
    expect(userFlowImports(source)).toContain('./Dynamic.vue');
  });
  it('continues scanning route metadata and ungated application screens in the actual graph', () => {
    const modules = userFlowModules().map(file => path.relative(SRC_ROOT, file).split(path.sep).join('/'));
    expect(modules).toContain('routes/admin.js');
    expect(modules).toContain('components/CreateApplication/CreateApplication.vue');
    expect(modules).toContain('components/onboarding/stepApplicability.js');
    expect(modules).not.toContain('views/RequestsView.vue');
  });
});
