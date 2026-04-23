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
app.use(router)
app.directive('permission', vPermission)

// Bootstrap: пробуем восстановить сессию из HttpOnly refresh cookie
// ДО первой navigation. Без этого router guard срабатывает раньше
// чем access token попадает в Pinia, и юзер на F5 выкидывается на /.
// Silent fail OK - если cookie мёртв, guard просто redirect'ит на /.
await tryRestoreSession()
await router.isReady()
app.mount('#app')
