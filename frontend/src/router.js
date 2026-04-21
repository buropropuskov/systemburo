import { createRouter, createWebHistory } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
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
    name: 'TablesComponent', 
    component: TablesComponent,
    meta: { requiresAuth: true, requiresBuro: true } 
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
  }
];

const router = createRouter({
  history: createWebHistory(),
  routes
});

router.beforeEach((to, from, next) => {
  const authStore = useAuthStore();
  const isAuthenticated = authStore.isAuthenticated;
  const isBuroPropuskov = authStore.isAdmin;

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