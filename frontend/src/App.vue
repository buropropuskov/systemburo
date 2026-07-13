<template>
  <div id="app">
    <a
      href="#main-content"
      class="skip-link"
    >Перейти к основному содержанию</a>
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
      <router-view v-slot="{ Component }">
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
    <OnboardingTour v-if="isAuthenticated" />
    <BanOverlay @logout="logout" />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { usePermissionsStore } from '@/stores/permissions'
import { useOnboardingStore } from '@/stores/onboarding'
import eventStream from '@/services/eventStream'
import NavMenu from './components/NavMenu.vue';
import TheHeader from './components/TheHeader/TheHeader.vue';
import ScrollTopButton from './components/ScrollTopButton.vue';
import ConfirmDialog from './components/ConfirmDialog.vue';
import DirtyConfirmModal from './components/DirtyConfirmModal.vue';
import DeleteNotifications from './components/DeleteNotifications.vue';
import OnboardingTour from './components/onboarding/OnboardingTour.vue';
import BanOverlay from './components/BanOverlay.vue';

export default {
  name: "App",
  components: {
    NavMenu,
    TheHeader,
    ScrollTopButton,
    ConfirmDialog,
    DirtyConfirmModal,
    DeleteNotifications,
    OnboardingTour,
    BanOverlay,
  },
  data() {
    return {
      // Функция отписки от real-time сигнала бана (scope user:<id>); null пока не подписаны.
      banEventOff: null,
      // userId, на который сейчас стоит подписка бана; держит её синхронной с токеном.
      banSubUserId: null,
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
     */
    showChrome() {
      const hidePaths = ['/', '/500', '/maintenance']
      return this.isAuthenticated && !hidePaths.includes(this.$route.path);
    },
    isBanned() {
      return usePermissionsStore().banned
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
    }
    // Подписка на бан для восстановленной при старте сессии; дальше её ведёт
    // watch(banScopeUserId) на смену токена.
    this.reconcileBanSubscription()
  },
  beforeUnmount() {
    this.teardownBanSubscription()
  },
  methods: {
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
    handleSuccessfulLogin(tokenData) {
      const authStore = useAuthStore()
      authStore.setTokens(tokenData.token)
      const permissionsStore = usePermissionsStore()
      permissionsStore.fetchPermissions()
      authStore.loadUserTypeCode()
      // Подписку на бан для нового токена ставит watch(banScopeUserId).
    },

    async logout() {
      const authStore = useAuthStore()
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
        if (this.$route.path !== '/') {
          this.$router.push("/");
        }
      }
    },
  },
};
</script>

<style>
.skip-link {
  position: absolute;
  top: -40px;
  left: 0;
  background: var(--color-primary);
  color: white;
  padding: 8px 16px;
  z-index: 10000;
  transition: top 0.2s;
}
.skip-link:focus {
  top: 0;
}

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
  color: #4F5BDF;
}

.red {
  color: rgb(241, 76, 76);
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
  }

  .modal-content {
    width: 100vw !important;
    max-width: 100vw !important;
    min-width: 100vw !important;
    max-height: 90dvh !important;
    border-radius: 16px 16px 0 0 !important;
    margin: 0 !important;
    overflow-y: auto !important;
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
.page-fade-enter-from,
.page-fade-leave-to {
  opacity: 0;
}
</style>
