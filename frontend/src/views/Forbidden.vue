<template>
  <main class="forbidden">
    <div class="forbidden__card">
      <h1 class="forbidden__code">
        403
      </h1>
      <h2 class="forbidden__title">
        Нет доступа
      </h2>
      <p class="forbidden__message">
        У вас нет прав на просмотр запрашиваемой страницы. Если вы считаете это ошибкой, обратитесь к администратору.
      </p>
      <div class="forbidden__actions">
        <button
          class="lk-button lk-button--primary"
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
export default {
  name: 'ForbiddenPage',
  methods: {
    goHome() {
      this.$router.push('/news');
    },
    goBack() {
      if (window.history.length > 1) this.$router.back();
      else this.$router.push('/news');
    },
  },
};
</script>

<style scoped>
.forbidden {
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

.forbidden__card {
  background: var(--surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
  padding: 32px 36px;
  max-width: 480px;
  width: 100%;
  text-align: center;
}

.forbidden__code {
  margin: 0;
  font-size: 64px;
  font-weight: 700;
  color: var(--accent-text);
  line-height: 1;
}

.forbidden__title {
  margin: 12px 0 8px 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text);
}

.forbidden__message {
  margin: 0 0 8px 0;
  font-size: 14px;
  line-height: 1.5;
  color: var(--color-text-muted);
}

.forbidden__actions {
  display: flex;
  gap: 10px;
  justify-content: center;
  margin-top: 20px;
}
</style>
