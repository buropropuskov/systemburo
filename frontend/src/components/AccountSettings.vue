<template>
  <div
    class="account__settings"
    :class="{ 'is-sticky': isSticky }"
  >
    <div class="settings__container">
      <img
        src="@/assets/icons/settings.png"
        class="settings__icon"
      >
      <h2 class="settings__title">
        Управление и настройка системы
      </h2>
    </div>
    <ul class="settings__navigation">
      <li
        v-for="item in items"
        :key="item.id"
        class="navigation__link"
      >
        <a
          :href="`#${item.id}`"
          class="link"
          :class="{ active: activeId === item.id }"
          @click="onNavClick(item.id, $event)"
        >{{ item.label }}</a>
      </li>
    </ul>
  </div>
</template>

<script>
const SECTIONS = [
  { id: 'users', label: 'Пользователи' },
  { id: 'organizations', label: 'Организации' },
  { id: 'companies', label: 'Компании' },
  { id: 'unload_place', label: 'Разгрузка' },
  { id: 'number', label: 'Авто-номера' },
  { id: 'citizenships', label: 'Гражданства' },
  { id: 'tables', label: 'Таблицы' },
  { id: 'user_types', label: 'Типы аккаунта' },
  { id: 'attachments', label: 'Бланки' },
  { id: 'approvers', label: 'Принимающие' }
];

export default {
  name: 'AccountSettings',
  data() {
    return {
      items: SECTIONS,
      activeId: '',
      isSticky: false,
      observer: null,
      stickySentinel: null
    };
  },
  mounted() {
    this.setupSectionObserver();
    this.setupStickyObserver();
  },
  beforeUnmount() {
    if (this.observer) this.observer.disconnect();
    if (this.stickyObs) this.stickyObs.disconnect();
    if (this.stickySentinel?.parentNode) this.stickySentinel.parentNode.removeChild(this.stickySentinel);
  },
  methods: {
    setupSectionObserver() {
      const targets = this.items
        .map(i => document.getElementById(i.id))
        .filter(Boolean);
      if (!targets.length || typeof IntersectionObserver === 'undefined') return;

      this.observer = new IntersectionObserver(entries => {
        const visible = entries
          .filter(e => e.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)[0];
        if (visible) this.activeId = visible.target.id;
      }, { rootMargin: '-30% 0px -50% 0px', threshold: 0 });

      targets.forEach(t => this.observer.observe(t));
    },
    setupStickyObserver() {
      if (typeof IntersectionObserver === 'undefined') return;
      this.stickySentinel = document.createElement('div');
      this.stickySentinel.style.height = '1px';
      this.$el.parentNode?.insertBefore(this.stickySentinel, this.$el);
      this.stickyObs = new IntersectionObserver(([entry]) => {
        this.isSticky = !entry.isIntersecting;
      });
      this.stickyObs.observe(this.stickySentinel);
    },
    onNavClick(id, event) {
      const target = document.getElementById(id);
      if (!target) return;
      event.preventDefault();
      target.scrollIntoView({ behavior: 'smooth', block: 'start' });
      this.activeId = id;
    }
  }
};
</script>

<style scoped>
.account__settings {
  display: flex;
  width: 100%;
  gap: 0;
  position: sticky;
  top: 0;
  z-index: 20;
  background: #fff;
  transition: box-shadow 0.2s ease, padding 0.2s ease;
}

.account__settings.is-sticky {
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  padding: 6px 0;
}

.settings__container {
  min-width: 360px;
  height: 50px;
  background-color: #4F5BDF;
  border-radius: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.settings__icon {
  width: 22px;
  height: 22px;
  animation: settings_rotate 8s linear infinite;
}

.settings__title {
  font-size: 16px;
  color: #FFF;
}

.settings__navigation {
  width: 100%;
  height: auto;
  border-top: 2px solid #4F5BDF;
  margin-top: auto;
  display: flex;
  list-style: none;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  gap: 8px 14px;
}

.link {
  font-size: 12px;
  font-weight: 600;
  color: #939CFF;
  transition: color 0.2s ease, background 0.2s ease;
  text-decoration: none;
  padding: 4px 10px;
  border-radius: 999px;
  white-space: nowrap;
}

.link:hover {
  color: #4F5BDF;
  background: rgba(79, 91, 223, 0.08);
}

.link.active {
  color: #fff;
  background: #4F5BDF;
}

@keyframes settings_rotate {
  0%   { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

@media (max-width: 1200px) {
  .settings__container { min-width: 300px; }
}

@media (max-width: 768px) {
  .account__settings {
    flex-direction: column;
    gap: 10px;
  }
  .settings__container {
    min-width: auto;
    width: 100%;
  }
  .settings__navigation {
    border-top: none;
    padding: 6px 4px;
    gap: 6px;
  }
}

@media (max-width: 480px) {
  .settings__title { font-size: 16px; }
  .link { font-size: 11px; padding: 3px 8px; }
}
</style>
