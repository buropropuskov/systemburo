import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { vPermission } from './directives/permission'
import bus from './eventBus'
import './assets/tokens.css'

const app = createApp(App)
app.config.globalProperties.$bus = bus
app.use(createPinia())
app.use(router)
app.directive('permission', vPermission)
app.mount('#app')
