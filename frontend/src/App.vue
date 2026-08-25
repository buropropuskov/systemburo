<template>
  <div id="app">
    <NavMenu
      v-if="showChrome"
      :is-buropropuskov="isBuropropuskov"
      @logout="logout"
    />
    <div
      id="main-content"
      class="content"
    >
      <TheHeader
        v-if="showChrome"
        class="theheader"
      />
      <router-view
        v-if="!consentBlocking && !passwordChangeBlocking"
        v-slot="{ Component }"
      >
        <transition
          name="page-fade"
          mode="out-in"
        >
          <component
            :is="Component"
            :key="$route.path"
            class="content__container"
            @login-success="handleSuccessfulLogin"
          />
        </transition>
      </router-view>
      <ScrollTopButton v-if="showChrome" />
    </div>
    <ConfirmDialog />
    <DirtyConfirmModal />
    <DeleteNotifications />
    <OnboardingTour v-if="isAuthenticated && !consentBlocking && !passwordChangeBlocking" />
    <GlobalSearchPanel
      v-if="isAuthenticated"
      :show="searchOpen"
      :query="searchQuery"
      @update:query="searchQuery = $event"
      @close="closeGlobalSearch"
    />
    <ImpersonationBanner v-if="isAuthenticated" />
    <BanOverlay @logout="logout" />
    <PDConsentOverlay
      :active="consentBlocking"
      @logout="logout"
    />
    <ChangePasswordModal
      :show="passwordChangeBlocking"
      mandatory
      @changed="onPasswordChanged"
      @logout="logout"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { usePermissionsStore } from '@/stores/permissions'
import { useOnboardingStore } from '@/stores/onboarding'
import { usePDConsentStore } from '@/stores/pdConsent'
import { usePasswordChangeStore } from '@/stores/passwordChange'
import { useThemeStore } from '@/stores/theme'
import { isShellHiddenPath } from '@/utils/shellPaths'
import eventStream from '@/services/eventStream'
import { unsubscribeWebPush } from '@/api/webPush'
import { getCurrentSubscription, unsubscribeLocal } from '@/utils/webPushSubscription'
import NavMenu from './components/NavMenu.vue';
import TheHeader from './components/TheHeader/TheHeader.vue';
import ScrollTopButton from './components/ScrollTopButton.vue';
import ConfirmDialog from './components/ConfirmDialog.vue';
import DirtyConfirmModal from './components/DirtyConfirmModal.vue';
import DeleteNotifications from './components/DeleteNotifications.vue';
import GlobalSearchPanel from './components/GlobalSearchPanel.vue';
import OnboardingTour from './components/onboarding/OnboardingTour.vue';
import ImpersonationBanner from './components/ImpersonationBanner.vue';
import BanOverlay from './components/BanOverlay.vue';
import PDConsentOverlay from './components/PDConsentOverlay.vue';
import ChangePasswordModal from './components/ChangePasswordModal.vue';

export default {
  name: "App",
  components: {
    NavMenu,
    TheHeader,
    ScrollTopButton,
    ConfirmDialog,
    DirtyConfirmModal,
    DeleteNotifications,
    GlobalSearchPanel,
    OnboardingTour,
    ImpersonationBanner,
    BanOverlay,
    PDConsentOverlay,
    ChangePasswordModal,
  },
  data() {
    return {
      // Функция отписки от real-time сигнала бана (scope user:<id>); null пока не подписаны.
      banEventOff: null,
      // userId, на который сейчас стоит подписка бана; держит её синхронной с токеном.
      banSubUserId: null,
      // Панель поиска: признак открытия и строка запроса. Живут в корне -- панель
      // открывают кнопкой из шапки, а показывается она поверх всей страницы.
      searchOpen: false,
      searchQuery: '',
      // Панель поиска раскрыл онбординг-тур (reveal), а не пользователь -
      // только такую тур и закроет за собой.
      searchOpenedByTour: false,
    }
  },
  computed: {
    isAuthenticated() {
      const authStore = useAuthStore()
      return authStore.isAuthenticated
    },
    isBuropropuskov() {
      const authStore = useAuthStore()
      return authStore.isSuperAdmin
    },
    /**
     * Показывать шапку, навигацию и связанный chrome только когда юзер
     * аутентифицирован и находится не на странице логина/ошибки. Иначе после
     * setTokens() в LoginComponent до router.push('/news')
     * (задержан на 1.5с из-за success-анимации) шапка и меню моргают
     * поверх формы логина. На /500 chrome тоже скрываем - это full-page view.
     * Список путей общий с гейтом согласия - utils/shellPaths.
     */
    showChrome() {
      return this.isAuthenticated
        && !isShellHiddenPath(this.$route.path)
        && !this.consentBlocking
        && !this.passwordChangeBlocking;
    },
    /**
     * Согласие на обработку ПД ещё не дано, а без него доступа нет (#1567):
     * шапка, навигация, страница и онбординг-тур не монтируются, поверх стоит
     * PDConsentOverlay.
     *
     * `resolved` обязателен - без ответа гейта окно на догадке не показываем.
     * Супер-администратор исключён и здесь, и на сервере: с битой настройкой
     * согласия систему всё равно надо чинить через интерфейс. Забаненному
     * показываем блокировку, а не согласие - принять его он всё равно не сможет,
     * серверный гейт бана стоит раньше.
     */
    consentBlocking() {
      const consent = usePDConsentStore()
      return this.isAuthenticated
        && !this.isBuropropuskov
        && !this.isBanned
        && consent.resolved
        && consent.required
        && !isShellHiddenPath(this.$route.path);
    },
    /**
     * Система обязала задать свой пароль вместо присланного письмом (#1911):
     * шапка, навигация, страница и тур не монтируются, поверх стоит несъёмное окно
     * смены пароля. Флаг поднимается маркером отказа из api/client.js - серверный
     * гейт всё равно отвечает 403 на всё, кроме смены пароля, и без окна человек
     * видел бы пустой экран.
     *
     * Забаненному показываем блокировку, а не смену пароля: менять пароль ему
     * незачем, и серверная проверка блокировки стоит раньше гейта.
     */
    passwordChangeBlocking() {
      return this.isAuthenticated
        && !this.isBanned
        && usePasswordChangeStore().required
        && !isShellHiddenPath(this.$route.path);
    },
    isBanned() {
      return usePermissionsStore().banned
    },
    /** Сигнал раскрытия свёрнутого узла от онбординг-тура - см. watch ниже. */
    onboardingReveal() {
      return useOnboardingStore().revealOpen
    },
    /**
     * ID залогиненного пользователя - scope адресного real-time сигнала бана;
     * null без токена. Реактивен к token (userPayload = decodeToken(token)):
     * логин, logout и истечение сессии в client.js (сброс токена в обход
     * logout()) двигают watch и переключают подписку.
     */
    banScopeUserId() {
      const authStore = useAuthStore()
      return authStore.token ? (authStore.userPayload?.user_id || null) : null
    }
  },
  watch: {
    // Держим подписку бана синхронной с залогиненным юзером: смена/сброс токена
    // (логин/logout/истечение сессии/смена юзера в той же вкладке) -> пере-подписка.
    banScopeUserId() {
      this.reconcileBanSubscription()
    },
    /**
     * Реактивная блокировка: как только permissions-стор узнаёт о бане
     * (на навигации/загрузке), уводим юзера в ЛК - единственную доступную
     * забаненному страницу. BanOverlay поверх неё блокирует взаимодействие.
     * router-guard делает то же на навигации; здесь - для случая, когда бан
     * прилетел без смены маршрута.
     */
    isBanned(banned) {
      if (banned && this.isAuthenticated && this.$route.name !== 'Account') {
        this.$router.push('/personal-cabinet').catch(() => {})
      }
    },
    /**
     * Онбординг просит показать панель сквозного поиска (reveal.open). Закрываем
     * только то, что открыли сами: панель, открытую пользователем, тур не трогает.
     */
    onboardingReveal(target) {
      if (target === 'search-panel') {
        // Флаг ставим и когда панель уже открыта: на шаге про поиск ею
        // распоряжается тур, кто бы её ни открыл. Иначе панель, открытая
        // человеком по просьбе предыдущего шага, оставалась висеть поверх
        // следующих шагов.
        this.searchOpenedByTour = true
        if (!this.searchOpen) this.openGlobalSearch()
      } else if (this.searchOpenedByTour) {
        this.searchOpenedByTour = false
        this.closeGlobalSearch()
      }
    }
  },
  created() {
    // tryRestoreSession вызывается в main.js ДО mount - auth уже hydrated.
    // Здесь остаётся только загрузить permissions и userTypeCode если юзер залогинен.
    const authStore = useAuthStore()
    if (authStore.token) {
      const permissionsStore = usePermissionsStore()
      permissionsStore.fetchPermissions()
      authStore.loadUserTypeCode()
      usePDConsentStore().refresh()
      // Тему профиля здесь НЕ запрашиваем: её тянет main.js до mount, иначе
      // интерфейс успевал отрисоваться в чужой/светлой теме (#1415).
    }
    // Подписка на бан для восстановленной при старте сессии; дальше её ведёт
    // watch(banScopeUserId) на смену токена.
    this.reconcileBanSubscription()
    window.addEventListener('keydown', this.onGlobalSearchHotkey)
    this.$bus?.on?.('global-search:open', this.openGlobalSearch)
  },
  beforeUnmount() {
    this.teardownBanSubscription()
    window.removeEventListener('keydown', this.onGlobalSearchHotkey)
    this.$bus?.off?.('global-search:open', this.openGlobalSearch)
  },
  methods: {
    openGlobalSearch() {
      if (this.isAuthenticated) this.searchOpen = true
    },
    /** Закрытие чистит запрос: следующий поиск начинается с чистого листа. */
    closeGlobalSearch() {
      this.searchOpen = false
      this.searchQuery = ''
      this.searchOpenedByTour = false
    },
    /**
     * Ctrl+K (Cmd+K на маке) -- общепринятое сочетание для поиска по приложению.
     * Открывает и закрывает панель поиска; курсор ставится в её поле.
     * preventDefault обязателен: в Firefox сочетание уводит фокус в адресную строку.
     */
    onGlobalSearchHotkey(e) {
      if (e.key !== 'k' && e.key !== 'K' && e.key !== 'л' && e.key !== 'Л') return
      if (!e.ctrlKey && !e.metaKey) return
      if (!this.isAuthenticated) return
      e.preventDefault()
      this.searchOpen = !this.searchOpen
    },
    /**
     * Приводит подписку на адресный real-time сигнал бана (scope user:<id>, #840)
     * в соответствие с текущим залогиненным юзером. Прилетевший
     * user.banned/user.unbanned -> fetchPermissions(true): force ОБЯЗАТЕЛЕН -
     * без него свежие (<30с) права коротят вызов в no-op и бан не всплывёт;
     * fetchPermissions ставит и флаг banned (баннер ЧС + watch уводит в ЛК), и
     * набор прав (пустой при бане -> все can() false, UI блокируется) - без
     * ожидания навигации/опроса. Идемпотентна: тот же userId -> no-op.
     */
    reconcileBanSubscription() {
      const userId = this.banScopeUserId
      if (userId === this.banSubUserId) return
      this.teardownBanSubscription()
      if (!userId) return
      eventStream.connect()
      this.banEventOff = eventStream.subscribe(`user:${userId}`, () => {
        usePermissionsStore().fetchPermissions(true)
      })
      this.banSubUserId = userId
    },
    teardownBanSubscription() {
      if (this.banEventOff) {
        this.banEventOff()
        this.banEventOff = null
        eventStream.disconnect()
      }
      this.banSubUserId = null
    },
    /**
     * Пароль задан: окно само чистит сессию и уводит на вход (смена отзывает все
     * продления). Снимаем требование, иначе оно всплыло бы поверх формы входа.
     */
    onPasswordChanged() {
      usePasswordChangeStore().reset()
    },

    handleSuccessfulLogin(tokenData) {
      const authStore = useAuthStore()
      authStore.setTokens(tokenData.token)
      const permissionsStore = usePermissionsStore()
      permissionsStore.fetchPermissions()
      authStore.loadUserTypeCode()
      useThemeStore().syncFromServer()
      // force: в этой вкладке до логина уже мог отвечать гейт другого юзера
      // (или гостя) - без force свежий resolved закоротил бы запрос в no-op.
      usePDConsentStore().refresh(true)
      // Подписку на бан для нового токена ставит watch(banScopeUserId).
    },

    async logout() {
      const authStore = useAuthStore()
      // Явный выход снимает push-подписку этого устройства (#974): иначе на
      // общем компьютере следующий вошедший увидит в системных уведомлениях
      // заголовки чужих заявок. Протухание токена (не через эту кнопку) push
      // намеренно НЕ трогает - там подписка обязана пережить сессию.
      try {
        const subscription = await getCurrentSubscription()
        if (subscription) {
          await unsubscribeWebPush(subscription.endpoint)
          await unsubscribeLocal(subscription)
        }
      } catch {
        // best-effort: не блокируем выход из-за push - подписка просто
        // останется висеть до следующей ручной отписки в настройках.
      }
      try {
        if (authStore.token) {
          await apiRequest("/logout", { method: "POST", body: '{}' });
        }
      } catch {
        // сеть упала - всё равно чистим клиент
      } finally {
        // clearTokens обнуляет banScopeUserId -> watch снимет подписку на бан.
        authStore.clearTokens()
        const permissionsStore = usePermissionsStore()
        permissionsStore.clearPermissions()
        // Снять онбординг-тур, если он был активен - иначе overlay driver.js
        // останется висеть поверх страницы логина.
        useOnboardingStore().reset()
        // Иначе окно согласия предыдущего юзера осталось бы висеть поверх формы
        // входа, а следующий увидел бы чужую редакцию текста.
        usePDConsentStore().reset()
        // Иначе требование сменить пароль вышедшего осталось бы висеть окном
        // поверх формы входа и досталось бы следующему на этом устройстве.
        usePasswordChangeStore().reset()
        if (this.$route.path !== '/') {
          this.$router.push("/");
        }
      }
    },
  },
};
</script>

<style>
* {
    font-family: 'Montserrat', sans-serif;
    padding: 0;
    margin: 0;
    box-sizing: border-box;
    scroll-behavior: smooth;
    /* Убираем серую вспышку при тапе на iOS - наши hover/active стили
     * уже дают visual feedback. */
    -webkit-tap-highlight-color: transparent;
}

/* Отключаем pull-to-refresh на мобильных - наша логика обновления
 * через явные кнопки "Обновить", не через свайп. */
html, body {
    overscroll-behavior-y: contain;
}

::-webkit-scrollbar {
  width: 0;
}

/*
 * Контент на desktop заходит на 25px под рельс NavMenu. Переменную --nav-ml
 * выставляет NavMenu по персистентному состоянию (свёрнут 25 / пин 120 / hide 0);
 * hover-разворот оверлеит контент и margin не меняет. На мобильном (<768px)
 * NavMenu - burger-drawer, margin не нужен.
 */
@media (min-width: 768px) {
  body.auth-active #app {
    margin-left: var(--nav-ml, 25px);
    transition: margin-left 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  }
}

body:not(.auth-active) #app {
  margin-left: 0;
}

/*
 * Потолок ширины контента на авторизованных страницах (chrome показан).
 * Основную работу на больших мониторах делает масштаб под эталон 1440
 * (utils/viewportScale.js); этот кап - страховка для ультрашироких/мультимонитора,
 * где масштаб упёрся в MAX_ZOOM и контент иначе растянулся бы на всю ширину.
 * Login/ошибки идут без auth-active и остаются full-bleed (их фон на всю ширину).
 */
/* Контент заполняет всю доступную ширину без центрирования (по требованию:
 * никаких боковых полей и центровки). Масштаб под большие экраны делает
 * utils/viewportScale.js; ограничивать ширину контейнером не нужно. */
body.auth-active .content__container {
  width: 100%;
}

/* На мобилке шапка (TheHeader) - position:sticky;top:0, т.е. ОСТАЁТСЯ В ПОТОКЕ и сама
   занимает свою высоту: резервировать её padding'ом НЕ нужно (иначе двойной отступ).
   Раньше шапка была fixed и высота компенсировалась здесь, но fixed рассинхронизировался
   со второй закреплённой шапкой Центра (sticky) при сворачивании адресной строки -
   обе прыгали. Подробности - в мобильном @media TheHeader.vue. */

/* Блокировка body-scroll пока mobile drawer открыт */
body.nav-drawer-open {
  overflow: hidden;
}

/* Включает анимацию transition между length и intrinsic-значениями (auto / max-content)
   для width/height/max-width/min-width. Без этого схлопывание столбца таблицы
   через max-width: 0 -> auto обратно не анимируется и даёт визуальный скачок
   при выключении "Увеличенного режима". Chromium 129+, Firefox 132+, Safari 18+. */
:root {
  interpolate-size: allow-keywords;
}

.blue {
  color: var(--accent-text);
}

.red {
  color: var(--danger-text);
}

.page-fade-enter-active {
  transition: opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}
.page-fade-leave-active {
  transition: opacity 0.15s cubic-bezier(0.4, 0, 0.2, 1);
}

/*
 * Mobile bottom-sheet паттерн для всех модалок с классами
 * .modal-overlay > .modal-content. На <768px модалка прилипает к низу
 * экрана, ширина 100%, высота - по контенту (короткие confirmations
 * не тянутся на весь экран). Длинные модалки получают internal scroll
 * до 90dvh. !important нужен потому что большинство использует scoped.
 */
@media (max-width: 768px) {
  .modal-overlay {
    padding: 0 !important;
    align-items: flex-end !important;
    /* Нативный dvh: композитор прибивает высоту к видимой области без reflow-лага,
       JS-синхронизация вьюпорта не нужна (#1097 R5-S2/S3). */
    top: 0 !important;
    height: 100dvh !important;
    bottom: auto !important;
  }

  .modal-content {
    width: 100vw !important;
    max-width: 100vw !important;
    min-width: 100vw !important;
    max-height: 90dvh !important;
    border-radius: 16px 16px 0 0 !important;
    margin: 0 !important;
    overflow-y: auto !important;
    /* Плавный выезд снизу-вверх для всех модалок с базовым паттерном .modal-content.
       Специфичность (0,1,0) - модалки с собственной scoped-анимацией сохраняют свою.
       fill-mode: backwards (НЕ both) - иначе финальный translateY(0) анимации залипает
       и перебивает inline-transform свайпа (лист не тянется за пальцем). backwards даёт
       кадр 'from' до старта (без мигания) и отпускает transform после - свайп работает. */
    animation: app-sheet-up 0.34s cubic-bezier(0.32, 0.72, 0, 1) backwards;
  }

  /* Для inputs и textarea внутри модалок - font-size 16px предотвращает
   * авто-зум на iOS при focus'е. */
  .modal-content input[type="text"],
  .modal-content input[type="email"],
  .modal-content input[type="password"],
  .modal-content input[type="tel"],
  .modal-content input[type="number"],
  .modal-content input[type="date"],
  .modal-content textarea,
  .modal-content select {
    font-size: 16px !important;
  }
}

@keyframes app-sheet-up {
  from {
    transform: translateY(100%);
  }
  to {
    transform: translateY(0);
  }
}
.page-fade-enter-from,
.page-fade-leave-to {
  opacity: 0;
}
</style>
