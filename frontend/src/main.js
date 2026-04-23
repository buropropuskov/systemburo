import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { vPermission } from './directives/permission'
import bus from './eventBus'
import { tryRestoreSession } from '@/api/client'
import './assets/tokens.css'

const app = createApp(App)
app.config.globalProperties.$bus = bus
app.use(createPinia())
app.directive('permission', vPermission)

// Bootstrap: восстанавливаем сессию из HttpOnly refresh cookie ДО app.use(router).
// Vue Router 4.x триггерит initial navigation синхронно при install - если router
// подключить раньше, guard успеет прочитать пустой authStore.token и редиректнуть
// на /, а наш await tryRestoreSession() применится уже ПОСЛЕ редиректа.
// Silent fail OK: если cookie мёртв, guard штатно отправит на /.
await tryRestoreSession()

app.use(router)
await router.isReady()
app.mount('#app')
