import { createRouter, createWebHistory } from 'vue-router';
// import SubmitForm from './components/SubmitForm.vue';
import LoginComponent from './components/LoginComponent.vue';
import TablesComponent from './components/TablesComponent.vue';
import AccountComponent from './components/AccountComponent.vue';
import CreateApplication from './components/CreateApplication.vue';

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
    path: '/personal-cabinet', 
    name: 'Account', 
    component: AccountComponent,
    meta: { requiresAuth: true } 
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes
});

router.beforeEach((to, from, next) => {
  const isAuthenticated = !!localStorage.getItem('token');
  console.log('Auth check - isAuthenticated:', isAuthenticated);
  
  let userType = null;
  const token = localStorage.getItem('token');
  
  if (token) {
    try {
      const payload = JSON.parse(atob(token.split('.')[1]));
      console.log('Token payload:', payload);
      userType = payload.type_id;
    } catch (e) {
      console.error("Token decode error:", e);
    }
  }

  const isBuroPropuskov = userType === 6;
  console.log('User type:', userType, 'isBuroPropuskov:', isBuroPropuskov);

  if (to.meta.requiresAuth && !isAuthenticated) {
    console.log('Redirect to login: requires auth');
    next('/');
  } else if (to.meta.requiresBuro && !isBuroPropuskov) {
    console.log('Redirect to cabinet: requires buro');
    next('/personal-cabinet');
  } else if (to.path === '/' && isAuthenticated) {
    console.log('Redirect from login to cabinet');
    next('/personal-cabinet');
  } else {
    next();
  }
});

export default router;