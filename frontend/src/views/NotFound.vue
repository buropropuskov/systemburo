<template>
  <main
    class="not-found"
    data-testid="not-found-page"
  >
    <div class="not-found__card">
      <h1 class="not-found__code">
        404
      </h1>
      <h2 class="not-found__title">
        Страница не найдена
      </h2>
      <p class="not-found__message">
        Такой страницы нет: адрес набран с ошибкой или раздел переехал. Проверьте ссылку или вернитесь к работе через меню.
      </p>
      <p
        v-if="requestedPath"
        class="not-found__path"
      >
        {{ requestedPath }}
      </p>
      <div class="not-found__actions">
        <button
          class="lk-button lk-button--primary"
          data-testid="not-found-home"
          @click="goHome"
        >
          На главную
        </button>
        <button
          class="lk-button lk-button--ghost"
          @click="goBack"
        >
          Назад
        </button>
      </div>
    </div>
  </main>
</template>

<script>
import { useAuthStore } from '@/stores/auth';

export default {
  name: 'NotFoundPage',
  computed: {
    /**
     * Адрес, по которому юзер сюда попал. Для явного /404 (на него уводит код,
     * не нашедший сущность) показывать нечего - там путь ничего не объясняет.
     */
    requestedPath() {
      return this.$route.path === '/404' ? '' : this.$route.path;
    },
  },
  methods: {
    home() {
      return useAuthStore().isAuthenticated ? '/news' : '/';
    },
    goHome() {
      this.$router.push(this.home());
    },
    goBack() {
      if (window.history.length > 1) this.$router.back();
      else this.$router.push(this.home());
    },
  },
};
</script>

<style scoped>
.not-found {
  /* zoom-safe (#1097): vh под корневым zoom раздувается -> низ за экраном, см. Error500. */
  min-height: calc(var(--app-vh, 1vh) * 100);
  /* B.3 (#1097): svh стабилизирует высоту на мобилке, min() держит zoom-корректность, см. Error500. */
  min-height: min(calc(var(--app-vh, 1vh) * 100), 100svh);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  /* Фон страницы, не подложка внутри карточки: --color-bg это алиас --surface-2. */
  background: var(--bg);
}

.not-found__card {
  background: var(--surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
  padding: 32px 36px;
  max-width: 480px;
  width: 100%;
  text-align: center;
}

.not-found__code {
  margin: 0;
  font-size: 64px;
  font-weight: 700;
  color: var(--accent-text);
  line-height: 1;
}

.not-found__title {
  margin: 12px 0 8px 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text);
}

.not-found__message {
  margin: 0 0 8px 0;
  font-size: 14px;
  line-height: 1.5;
  color: var(--color-text-muted);
}

.not-found__path {
  margin: 0;
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-size: 13px;
  color: var(--color-text-muted);
  overflow-wrap: anywhere;
}

.not-found__actions {
  display: flex;
  gap: 10px;
  justify-content: center;
  margin-top: 20px;
}
</style>
