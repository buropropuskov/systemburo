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
      <router-view
        v-slot="{ Component }"
        class="content__container"
      >
        <transition
          name="page-fade"
          mode="out-in"
        >
          <component
            :is="Component"
            :key="$route.path"
            @login-success="handleSuccessfulLogin"
          />
        </transition>
      </router-view>
      <ScrollTopButton v-if="showChrome" />
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { usePermissionsStore } from '@/stores/permissions'
import NavMenu from './components/NavMenu.vue';
import TheHeader from './components/TheHeader/TheHeader.vue';
import ScrollTopButton from './components/ScrollTopButton.vue';

export default {
  name: "App",
  components: {
    NavMenu,
    TheHeader,
    ScrollTopButton
  },
  computed: {
    isAuthenticated() {
      const authStore = useAuthStore()
      return authStore.isAuthenticated
    },
    isBuropropuskov() {
      const authStore = useAuthStore()
      return authStore.isAdmin
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
    }
  },
  created() {
    // tryRestoreSession вызывается в main.js ДО mount - auth уже hydrated.
    // Здесь остаётся только загрузить permissions если юзер залогинен.
    const authStore = useAuthStore()
    if (authStore.token) {
      const permissionsStore = usePermissionsStore()
      permissionsStore.fetchPermissions()
    }
  },
  methods: {
    handleSuccessfulLogin(tokenData) {
      const authStore = useAuthStore()
      authStore.setTokens(tokenData.token)
      const permissionsStore = usePermissionsStore()
      permissionsStore.fetchPermissions()
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
        authStore.clearTokens()
        const permissionsStore = usePermissionsStore()
        permissionsStore.clearPermissions()
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
}

::-webkit-scrollbar {
  width: 0;
}

/*
 * margin-left: 25px компенсирует ширину NavMenu на desktop.
 * На мобильном (<768px) NavMenu скрывается/становится burger-menu,
 * поэтому margin не нужен - даёт только горизонтальный overflow.
 */
@media (min-width: 768px) {
  body.auth-active #app {
    margin-left: 25px;
  }
}

body:not(.auth-active) #app {
  margin-left: 0;
}

/* Блокировка body-scroll пока mobile drawer открыт */
body.nav-drawer-open {
  overflow: hidden;
}

.form-input-sm {
  border-radius: 10px !important;
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
