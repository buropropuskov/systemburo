<template>
  <div id="app">
    <a href="#main-content" class="skip-link">Перейти к основному содержанию</a>
    <NavMenu
      v-if="showChrome"
      :is-buropropuskov="isBuropropuskov"
      @logout="logout"
    />
    <div class="content" id="main-content">
      <TheHeader class="theheader" v-if="showChrome"/>
      <router-view v-slot="{ Component }" class="content__container">
        <transition name="page-fade" mode="out-in">
          <component :is="Component" @login-success="handleSuccessfulLogin" :key="$route.path" />
        </transition>
      </router-view>
      <ScrollTopButton v-if="showChrome" />
    </div>
  </div>
</template>

<script>
import { apiRequest, tryRestoreSession } from '@/api/client'
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
     * аутентифицирован и находится не на странице логина. Иначе после
     * setTokens() в LoginComponent до router.push('/news')
     * (задержан на 1.5с из-за success-анимации) шапка и меню моргают
     * поверх формы логина.
     */
    showChrome() {
      return this.isAuthenticated && this.$route.path !== '/';
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
  async created() {
    // После F5 access_token в памяти потерян. Refresh cookie живёт на
    // стороне сервера - пытаемся восстановить сессию одним запросом.
    const authStore = useAuthStore()
    if (!authStore.token) {
      const restored = await tryRestoreSession()
      if (restored) {
        const permissionsStore = usePermissionsStore()
        permissionsStore.fetchPermissions()
      }
    }
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
  transition: top 0.2s ease;
  text-decoration: none;
}

.skip-link:focus {
  top: 0;
}

.page-fade-enter-active,
.page-fade-leave-active {
  transition: opacity 0.3s ease, transform 0.3s ease;
}

.page-fade-enter-from {
  opacity: 0;
  transform: translateY(10px);
}

.page-fade-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}
</style>
