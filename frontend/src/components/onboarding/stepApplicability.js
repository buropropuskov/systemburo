import { adminRoutes } from '@/routes/admin';

// Learning follows the actual entry points, without changing route/API access.
const registryPages = { '/carsview': 'page.cars', '/employeesview': 'page.employees' };
const registryActions = { 'cars-add': 'cars', 'employees-add': 'employees' };
const applicationActions = {
  'apr-detail-vote': 'action.approve.application',
  'apr-detail-forward': 'action.forward.application',
  'acc-detail-forward': 'action.forward.application',
};

/** Filter the learning set before navigation or DOM preparation begins. */
export function isStepApplicable(step, context) {
  const can = context.can || (() => false);
  const canCreate = can('page.new_application') || can('header.create_application');
  const hasRegistryPage = can('page.cars') || can('page.employees');
  if (registryPages[step.route] && !can(registryPages[step.route])) return false;
  if (step.route === '/new-application' && !canCreate) return false;
  if ((step.route === '/center' || ['apr-nav-center', 'acc-nav-center'].includes(step.id))
    && !can('page.center')) return false;
  if (step.route === '/accessible-attachments' && !context.canViewAccessibleAttachments) return false;
  if (['sec-nav-tables', 'sec-nav-tables-open'].includes(step.id)
    && context.factRouteLoaded === true && !context.factTableRoute) return false;
  if (['sec-aa-card', 'sec-aa-detail', 'sec-aa-elements', 'sec-aa-preview', 'sec-aa-blank'].includes(step.id)
    && context.accessibleContext?.hasItems === false) return false;
  if (['sec-fact-report', 'sec-fact-report-window'].includes(step.id)) {
    const name = step.route?.startsWith('/table/') ? decodeURIComponent(step.route.slice(7)) : null;
    if (!name || !can(`table.${name}.report`)) return false;
  }
  if (['nav-group-data', 'cabinet-outro'].includes(step.id) && !hasRegistryPage) return false;

  const registry = registryActions[step.id];
  if (registry && (!can(`entity.${registry}.read`) || !can(`entity.${registry}.write`))) return false;

  // This tour starts in the admin menu. A leaf still needs its own route gate.
  if (step.id?.startsWith('admin-') && !can('page.admin')) return false;
  const routePermission = adminRoutes.find(route => route.path === step.route)?.meta?.permission;
  if (typeof routePermission === 'string' && !can(routePermission)) return false;

  const required = applicationActions[step.id] || step.requires;
  if (!required) return true;
  if (['action.approve.application', 'action.forward.application'].includes(required)) {
    return Boolean(context.allowsApplicationAction?.(required));
  }
  return can(required);
}
