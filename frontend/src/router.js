import { createRouter, createWebHistory } from 'vue-router';
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
    path: '/submit-form', 
    name: 'SubmitForm', 
    component: CreateApplication,
    meta: { requiresAuth: true } 
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
    meta: {requiresAuth: true, requiresBuro: true}
  },

];

const router = createRouter({
  history: createWebHistory(),
  routes
});

// Функция для проверки валидности токена
const isTokenValid = (token) => {
  if (!token) return false;
  
  try {
    const payload = JSON.parse(atob(token.split('.')[1]));
    const currentTime = Math.floor(Date.now() / 1000);
    const isValid = payload.exp > currentTime;
    
    // const timeUntilExpiry = payload.exp - currentTime;
    
    /* console.log('🔐 Token validation:', {
      timeUntilExpiry: timeUntilExpiry + ' seconds',
      isValid: isValid ? '✅ Valid' : '❌ Expired'
    }); */
    
    return isValid;
  } catch (e) {
    /* console.error("❌ Token validation error:", e); */
    return false;
  }
};

// Функция для получения типа пользователя
const getUserType = () => {
  const token = localStorage.getItem('token');
  if (!token) return null;
  
  try {
    const payload = JSON.parse(atob(token.split('.')[1]));
    return payload.type_id;
  } catch (e) {
    /* console.error("❌ Token decode error:", e); */
    return null;
  }
};

router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token');
  const refreshToken = localStorage.getItem('refreshToken');
  
  // Проверяем наличие ОБОИХ токенов и валидность access token
  const isAuthenticated = token && refreshToken && isTokenValid(token);
  
  const userType = getUserType();
  const isBuroPropuskov = userType === 6;

  /* console.log('🛡️ Router auth check:', {
    route: to.path,
    hasToken: token ? '✅' : '❌',
    hasRefreshToken: refreshToken ? '✅' : '❌',
    isTokenValid: isTokenValid(token) ? '✅' : '❌',
    isAuthenticated: isAuthenticated ? '✅' : '❌'
  }); */

  if (to.meta.requiresAuth && !isAuthenticated) {
    console.log('🚫 Redirect to login: requires auth');
    
    // Очищаем невалидные токены
    localStorage.removeItem('token');
    localStorage.removeItem('refreshToken');
    
    next('/');
  } 
  else if (to.meta.requiresBuro && !isBuroPropuskov) {
    console.log('🔒 Redirect to cabinet: requires buro');
    next('/personal-cabinet');
  } 
  else if (to.path === '/' && isAuthenticated) {
    console.log('🔄 Redirect from login to news');
    next('/news');
}
  else {
    next();
  }
});

export default router;