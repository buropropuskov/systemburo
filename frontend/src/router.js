import { createRouter, createWebHistory } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { useMaintenanceStore } from '@/stores/maintenance';
import LoginComponent from './components/LoginComponent.vue';
import TablesComponent from './components/TablesComponent.vue';
import AccountComponent from './components/AccountComponent.vue';
import CreateApplication from './components/CreateApplication/CreateApplication.vue';
import CarsView from './views/CarsView.vue';
import ApplicationsCenter from './views/ApplicationsCenter.vue';
import TableConstructor from './components/TableConstructor.vue';
import NumberFormat from './components/NumberFormat.vue';
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
    name: 'NumberFormat',
    component: NumberFormat,
    meta: { requiresAuth: true, requiresBuro: true }
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
    meta: {requiresAuth: true, requiresBuro: true}
  },
  {
    path: '/admin/requests',
    name: 'RequestsView',
    component: RequestsView,
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/admin/settings',
    name: 'AdminSettings',
    component: () => import('./views/AdminSettings.vue'),
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/admin/users',
    name: 'AdminUsers',
    component: () => import('./views/AdminUsers.vue'),
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/admin/permission-groups',
    name: 'AdminPermissionGroups',
    component: () => import('./views/admin/AdminPermissionGroups.vue'),
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/admin/roles',
    name: 'AdminRoles',
    component: () => import('./views/admin/AdminRoles.vue'),
    meta: { requiresAuth: true, requiresBuro: true }
  },
  {
    path: '/admin/access-denials',
    name: 'AccessDenialsLog',
    component: () => import('./views/admin/AccessDenialsLog.vue'),
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
    meta: { requiresAuth: true, requiresBuro: true }
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
router.beforeEach((to, from, next) => {
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
  }
  else if (to.meta.requiresBuro && !isBuroPropuskov) {
    next('/personal-cabinet');
  }
  else if (to.path === '/' && isAuthenticated) {
    next('/news');
  }
  else {
    next();
  }
});

export default router;