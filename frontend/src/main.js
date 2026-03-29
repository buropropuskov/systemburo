import { createApp } from 'vue';
import App from './App.vue';
import router from './router';
import bus from './eventBus';

const app = createApp(App);
app.config.globalProperties.$bus = bus; // для доступа в компонентах
app.use(router);
app.mount('#app');