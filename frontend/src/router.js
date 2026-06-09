import { createRouter, createWebHistory } from 'vue-router';
import { confirmIfAnyDirty } from '@/utils/dirtyTracker';
import { useAuthStore } from '@/stores/auth';
import { useMaintenanceStore } from '@/stores/maintenance';
import LoginComponent from './components/LoginComponent.vue';
import TablesComponent from './components/TablesComponent.vue';
import AccountComponent from './components/AccountComponent.vue';
import CreateApplication from './components/CreateApplication/CreateApplication.vue';
import CarsView from './views/CarsView.vue';
import ApplicationsCenter from './views/ApplicationsCenter.vue';
import TableConstructor from './components/TableConstructor.vue';
import EmployeeView from './views/EmployeeView.vue';
import NewsAndReview from './views/NewsAndReview.vue';
import FeedbackPage from './views/FeedbackPage.vue';
import RequestsView from './views/RequestsView.vue';

const routes = [
  { 
    path: '/', 
    name: 'LoginComponent', 
    component: LoginComponent,
    meta: { requiresAuth: false } 
  },
  {
    path: '/new-application',
    name: 'NewApplication',
    component: CreateApplication,
    meta: { requiresAuth: true }
  },
  {
    path: '/submit-form',
    redirect: '/new-application'
  },
  {
    path: '/table',
    redirect: '/personal-cabinet'
  },
  {
    path: '/table/:tableName',
    name: 'DynamicTable',
    component: TablesComponent,
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/table/:tableName/trash',
    name: 'TableTrash',
    component: () => import('@/views/TrashView.vue'),
    meta: { requiresAuth: true, requiresBuro: true }
  },
  { 
    path: '/personal-cabinet', 
    name: 'Account', 
    component: AccountComponent,
    meta: { requiresAuth: true } 
  },
  {
    path: '/carsview',
    name: 'CarsView',
    component: CarsView,
    meta: { requiresAuth: true }
  },
  {
    path: '/center',
    name: 'ApplicationsCenter',
    component: ApplicationsCenter,
    meta: { requiresAuth: true }
  },
  {
    path: '/table-constructor',
    name: 'TableConstructor',
    component: TableConstructor,
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/number-format',
    redirect: '/admin/number-formats'
  },
  {
    path: '/employeesview',
    name: 'EmployeeView',
    component: EmployeeView,
    meta: { requiresAuth: true }
  },
  {
    path: '/news',
    name: 'NewsAndReview',
    component: NewsAndReview,
    meta: {requiresAuth: true}
  },
  {
    path: '/admin/feedback',
    name: 'FeedbackPage',
    component: FeedbackPage,
    meta: { requiresAuth: true, requiresBuro: true, permission: 'page.admin.feedback' }
  },
  {
    path: '/admin/requests',
    name: 'RequestsView',
    component: RequestsView,
    meta: { requiresAuth: true, requiresBuro: true, permission: 'page.admin' }
  },
  {
    path: '/admin/settings',
    name: 'AdminSettings',
    component: () => import('./views/AdminSettings.vue'),
    meta: { requiresAuth: true, requiresBuro: true, permission: 'page.admin' }
  },
  {
    path: '/admin/users',
    name: 'AdminUsers',
    component: () => import('./views/AdminUsers.vue'),
    meta: { requiresAuth: true, requiresBuro: true, permission: 'page.admin.users' }
  },
  {
    path: '/admin/blacklist',
    name: 'AdminBlacklist',
    component: () => import('./views/admin/BlacklistView.vue'),
    meta: { requiresAuth: true, requiresBuro: true, permission: 'page.admin.blacklist' }
  },
  {
    path: '/admin/permission-groups',
    name: 'AdminPermissionGroups',
    component: () => import('./views/admin/AdminPermissionGroups.vue'),
    meta: { requiresAuth: true, requiresBuro: true, permission: 'permission.audit.manage' }
  },
  {
    path: '/admin/roles',
    name: 'AdminRoles',
    component: () => import('./views/admin/AdminRoles.vue'),
    meta: { requiresAuth: true, requiresBuro: true, permission: 'permission.audit.manage' }
  },
  {
    path: '/admin/access-denials',
    name: 'AccessDenialsLog',
    component: () => import('./views/admin/AccessDenialsLog.vue'),
    meta: { requiresAuth: true, requiresBuro: true, permission: 'permission.audit.read' }
  },
  // requiresBuro сохраняет текущий доступ супер-админа без правок бэкенда прав.
  {
    path: '/admin/organizations',
    name: 'AdminOrganizations',
    component: () => import('./views/admin/OrganizationsView.vue'),
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/admin/companies',
    name: 'AdminCompanies',
    component: () => import('./views/admin/CompaniesView.vue'),
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/admin/unload-places',
    name: 'AdminUnloadPlaces',
    component: () => import('./views/admin/UnloadPlacesView.vue'),
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/admin/number-formats',
    name: 'AdminNumberFormats',
    component: () => import('./views/admin/NumberFormatsView.vue'),
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/admin/citizenship',
    name: 'AdminCitizenship',
    component: () => import('./views/admin/CitizenshipView.vue'),
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/admin/marks',
    name: 'AdminMarks',
    component: () => import('./views/admin/MarksView.vue'),
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/admin/attachment-types',
    name: 'AdminAttachmentTypes',
    component: () => import('./views/admin/AttachmentTypesView.vue'),
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/admin/user-types',
    name: 'AdminUserTypes',
    component: () => import('./views/admin/UserTypesView.vue'),
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/admin/approvers',
    name: 'AdminApprovers',
    component: () => import('./views/admin/ApproversView.vue'),
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/500',
    name: 'Error500',
    component: () => import('./views/Error500.vue'),
    meta: { requiresAuth: false }
  },
  {
    path: '/maintenance',
    name: 'Maintenance',
    component: () => import('./views/Maintenance.vue'),
    meta: { requiresAuth: false }
  },
  {
    path: '/admin/system-control',
    name: 'SystemControl',
    component: () => import('./views/admin/SystemControl.vue'),
    meta: { requiresAuth: true, requiresBuro: true, permission: 'page.admin.system_control' }
  },
  {
    path: '/403',
    name: 'Forbidden',
    component: () => import('./views/Forbidden.vue'),
    meta: { requiresAuth: true }
  }
];

const router = createRouter({
  history: createWebHistory(),
  routes
});

// Guard синхронный - tryRestoreSession вызывается в main.js ДО mount,
// поэтому к моменту первой navigation auth store уже hydrated (token
// в памяти если refresh cookie жив). На F5 guard сразу видит реальное
// состояние без async гонок.
router.beforeEach(async (to, from, next) => {
  // Защита от потери несохранённых изменений: если на странице есть форма
  // с pending-правками - спросить подтверждение перед navigation. Применяется
  // ко всем переходам, ВКЛЮЧАЯ программные next() ниже - кроме первого
  // монтирования (from.name === undefined).
  if (from.name !== undefined && !(await confirmIfAnyDirty())) {
    next(false);
    return;
  }

  const authStore = useAuthStore();
  const maintenanceStore = useMaintenanceStore();
  const isAuthenticated = authStore.isAuthenticated;
  const isBuroPropuskov = authStore.isSuperAdmin;

  // Maintenance: если режим включён и юзер не супер-админ - на /maintenance.
  // Сам /maintenance + /500 доступны всегда чтобы не зациклить.
  if (
    maintenanceStore.enabled
    && !isBuroPropuskov
    && to.name !== 'Maintenance'
    && to.name !== 'Error500'
  ) {
    next({ name: 'Maintenance' });
    return;
  }

  if (to.meta.requiresAuth && !isAuthenticated) {
    next('/');
    return;
  }
  if (to.meta.requiresBuro && !isBuroPropuskov) {
    next('/personal-cabinet');
    return;
  }
  if (to.path === '/' && isAuthenticated) {
    next('/news');
    return;
  }

  // Permission polling (#187e): refresh permissions при stale-cache на любой
  // протектед-странице. Бан/изменение прав администратором заметится в
  // течение 30s максимум.
  if (isAuthenticated && (to.meta.requiresAuth || to.meta.permission)) {
    const { usePermissionsStore } = await import('@/stores/permissions');
    const store = usePermissionsStore();
    if (store.isStale) await store.fetchPermissions();
    if (to.meta.permission && !store.hasPermission(to.meta.permission)) {
      next({ name: 'Forbidden', query: { permission: to.meta.permission, from: to.fullPath } });
      return;
    }
  }

  next();
});

export default router;