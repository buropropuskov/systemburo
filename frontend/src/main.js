import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { vPermissionScope } from './directives/permission-scope'
import bus from './eventBus'
import { tryRestoreSession } from '@/api/client'
import { useMaintenanceStore } from '@/stores/maintenance'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { installBeforeUnloadGuard } from '@/utils/dirtyTracker'
import { initViewportScale } from '@/utils/viewportScale'
import { attachPushNavigationListener } from '@/utils/pushNavigation'
import './assets/tokens.css'
import './assets/forms.css'
import './assets/hints.css'
import './assets/onboarding.css'
import './assets/responsive-tables.css'
import './assets/analytics-panels.css'

const app = createApp(App)
app.config.globalProperties.$bus = bus
app.use(createPinia())
app.directive('permission-scope', vPermissionScope)

// Bootstrap: восстанавливаем сессию из HttpOnly refresh cookie ДО app.use(router).
// Vue Router 4.x триггерит initial navigation синхронно при install - если router
// подключить раньше, guard успеет прочитать пустой authStore.token и редиректнуть
// на /, а наш await tryRestoreSession() применится уже ПОСЛЕ редиректа.
// Silent fail OK: если cookie мёртв, guard штатно отправит на /.
await tryRestoreSession()
// Загружаем maintenance-статус до mount - чтобы router.beforeEach guard мог
// сразу решать, пускать ли юзера на любую страницу или отправлять на /maintenance.
// Тему профиля тянем ТУТ ЖЕ и параллельно (#1415): bootstrap-скрипт index.html
// ставит только тему из localStorage, а её на новом устройстве нет - и юзер
// пару секунд видел светлую, пока не приедет профиль. В одной пачке с
// maintenance запрос ничего не удлиняет, зато интерфейс монтируется уже в
// выбранной теме. Гость (нет токена) профиль не запрашивает - там 401.
const boot = [useMaintenanceStore().fetchStatus()]
if (useAuthStore().token) boot.push(useThemeStore().syncFromServer())
await Promise.all(boot)

app.use(router)
// Клик по push-уведомлению у открытой вкладки обрабатывается тут, а не
// перезагрузкой страницы service worker'ом (#974).
attachPushNavigationListener(router)
await router.isReady()
installBeforeUnloadGuard()
app.mount('#app')
// Масштаб UI под эталон 1440 на больших мониторах - после mount, чтобы корень
// уже существовал и первый zoom применился к готовому дереву.
initViewportScale()
