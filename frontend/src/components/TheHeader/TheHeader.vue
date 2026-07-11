<template>
  <header
    ref="header"
    class="header"
  >
    <button
      class="header__burger"
      aria-label="Открыть меню"
      type="button"
      @click="toggleMobileNav"
    >
      <span />
      <span />
      <span />
    </button>

    <div
      class="header__title"
      :class="{ 'header__title--shifted': uiStore.sidebarHidden }"
    >
      <template v-if="loading">
        <SkeletonLine
          width="200px"
          height="20px"
        />
        <SkeletonLine
          width="150px"
          height="14px"
        />
      </template>
      <template v-else>
        <h3>{{ greeting }}</h3>
        <p class="header__subtitle">
          Мы рады, что вы здесь!
        </p>
      </template>
    </div>

    <div class="header__info">
      <div class="header__overflow-wrap">
        <button
          class="header__overflow-toggle"
          type="button"
          aria-label="Ещё"
          aria-haspopup="true"
          :aria-expanded="showHeaderOverflow"
          @click.stop="showHeaderOverflow = !showHeaderOverflow"
        >
          <span aria-hidden="true">⋯</span>
        </button>
        <div
          class="header__overflow"
          :class="{ 'header__overflow--open': showHeaderOverflow }"
        >
          <button
            v-if="can('header.report_problem')"
            class="feedback-btn"
            data-testid="header-button-feedback"
            @click="openFeedbackModal"
          >
            Сообщить о проблеме
          </button>
          <button
            v-if="activeAnnouncement"
            class="broadcast"
            :class="{ 'broadcast--important': activeAnnouncement.is_important }"
            data-testid="ob-header-broadcast"
            @click="showAnnouncement = true"
          >
            {{ activeAnnouncement.is_important ? 'Важное объявление' : 'Объявление' }}
          </button>
          <p
            class="time"
            data-testid="ob-header-time"
          >
            {{ currentDateTime }}
          </p>
          <div
            class="user__notifications"
            :class="{ 'user__notifications--active': showNotifications }"
            data-testid="ob-header-notifications"
            @click.stop="showNotifications = !showNotifications"
          >
            <img
              src="@/assets/icons/notifications.png"
              class="notifications__icon"
              alt="Уведомления"
            >
            <span
              v-if="unreadCount > 0"
              class="notifications__badge"
            >{{ unreadCount }}</span>
            <UserNotifications
              :show="showNotifications"
              @update:unread-count="unreadCount = $event"
              @close="showNotifications = false"
            />
          </div>
        </div>
      </div>
      <div
        v-if="can('header.create_application')"
        class="appl-btn__container"
      >
        <button
          class="appl-btn"
          data-testid="header-button-submit-app"
          @click="navigateToSubmit"
        >
          <span
            class="appl-btn__icon"
            aria-hidden="true"
          >+</span>
          <span class="appl-btn__label">Подать заявку</span>
        </button>
      </div>
    </div>

    <!-- Используем отдельный компонент модального окна -->
    <FeedbackModal
      v-model:show="showFeedbackModal"
      @submitted="handleFeedbackSubmitted"
    />
    <AnnouncementModal
      :show="showAnnouncement"
      :announcement="activeAnnouncement"
      @close="showAnnouncement = false"
    />
  </header>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'
import { usePermissionsStore } from '@/stores/permissions'
import FeedbackModal from '@/components/FeedbackModal.vue';
import AnnouncementModal from '@/components/AnnouncementModal.vue';
import UserNotifications from '@/components/UserNotifications.vue';
import { SkeletonLine } from '@/components/ui';

export default {
  name: 'TheHeader',
  components: {
    FeedbackModal,
    AnnouncementModal,
    UserNotifications,
    SkeletonLine,
  },
  emits: ['refresh-feedback'],
  setup() {
    // uiStore нужен для сдвига приветствия, когда рельс скрыт (плавающая кнопка
    // возврата перекрывала бы заголовок). permissionsStore - гейт кнопок шапки:
    // «Сообщить о проблеме» (header.report_problem) и «Подать заявку»
    // (header.create_application, отдельно от nav «Новая заявка» = page.new_application).
    const uiStore = useUiStore();
    const permissionsStore = usePermissionsStore();
    return { uiStore, permissionsStore };
  },
  data() {
    return {
      loading: true,
      userFirstName: '',
      userLastName: '',
      currentDateTime: '',
      timer: null,
      isHeaderHidden: false,
      observer: null,
      showFeedbackModal: false,
      showAnnouncement: false,
      activeAnnouncement: null,
      showNotifications: false,
      showHeaderOverflow: false,
      unreadCount: 0,
      currentHour: new Date().getHours(),
    };
  },
  computed: {
    displayName() {
      return this.userFirstName || this.userLastName || '';
    },
    greetingPrefix() {
      const h = this.currentHour;
      if (h >= 6 && h < 12) return 'Доброе утро';
      if (h >= 12 && h < 18) return 'Добрый день';
      if (h >= 18 && h < 23) return 'Добрый вечер';
      return 'Доброй ночи';
    },
    greeting() {
      const name = this.displayName;
      return name ? `${this.greetingPrefix}, ${name}!` : `${this.greetingPrefix}!`;
    },
  },
  watch: {
    '$route'() {
      this.fetchUserData();
    }
  },
  mounted() {
    this.fetchUserData();
    this.fetchActiveAnnouncement();
    this.startDateTimeTimer();
    this.$nextTick(() => {
      this.initIntersectionObserver();
    });
    this._onDocumentClick = () => {
      if (this.showNotifications) {
        this.showNotifications = false;
      }
      if (this.showHeaderOverflow) {
        this.showHeaderOverflow = false;
      }
    };
    document.addEventListener('click', this._onDocumentClick);
  },
  beforeUnmount() {
    if (this.timer) {
      clearInterval(this.timer);
    }

    if (this.observer) {
      this.observer.disconnect();
    }
    document.removeEventListener('click', this._onDocumentClick);
  },
  methods: {
    // Гейтинг по правам (#187 Фаза 2). super -> всегда true, admin -> всё кроме
    // denied, обычный -> по эффективному гранту. Реактивно: читает стор прав.
    can(key) {
      return this.permissionsStore.hasPermission(key);
    },
    async fetchActiveAnnouncement() {
      try {
        const response = await apiRequest('/announcements/active', { method: 'GET' });
        if (response.ok) {
          this.activeAnnouncement = await response.json();
        }
      } catch (error) {
        console.error('Ошибка при загрузке объявления:', error);
      }
    },
    openFeedbackModal() {
      this.showFeedbackModal = true;
    },
    toggleMobileNav() {
      this.$bus.emit('mobile-nav-toggle');
    },
    handleFeedbackSubmitted(message) {
      console.log('Обратная связь отправлена:', message);
      // Если мы на странице обратной связи, можно обновить список
      if (this.$route.path === '/feedback') {
        this.$emit('refresh-feedback');
      }
    },
    navigateToSubmit() {
      this.$router.push('/new-application');
    },
    async fetchUserData() {
      try {
        const authStore = useAuthStore();
        if (!authStore.token) {
          console.log("Пользователь не авторизован");
          return;
        }

        const response = await apiRequest("/users/me", {
          method: "GET",
        });

        if (response.ok) {
          const userData = await response.json();
          this.userFirstName = userData.first_name || '';
          this.userLastName = userData.last_name || '';
        } else {
          console.error("Ошибка при загрузке данных пользователя");
        }
      } catch (error) {
        console.error("Ошибка сети при загрузке данных пользователя:", error);
      } finally {
        this.loading = false;
      }
    },
    updateDateTime() {
      const now = new Date();
      const day = String(now.getDate()).padStart(2, '0');
      const month = String(now.getMonth() + 1).padStart(2, '0');
      const year = now.getFullYear();
      const hours = String(now.getHours()).padStart(2, '0');
      const minutes = String(now.getMinutes()).padStart(2, '0');
      const seconds = String(now.getSeconds()).padStart(2, '0');
      this.currentDateTime = `${day}.${month}.${year} ${hours}:${minutes}:${seconds}`;
      const h = now.getHours();
      if (h !== this.currentHour) {
        this.currentHour = h;
      }
    },
    startDateTimeTimer() {
      this.updateDateTime();
      this.timer = setInterval(() => {
        this.updateDateTime();
      }, 1000);
    },
    initIntersectionObserver() {
      this.observer = new IntersectionObserver(
        (entries) => {
          entries.forEach(entry => {
            this.isHeaderHidden = !entry.isIntersecting;
          });
        },
        {
          threshold: 0,
          rootMargin: '0px'
        }
      );

      if (this.$refs.header) {
        this.observer.observe(this.$refs.header);
      }
    }
  },
}
</script>

<style scoped>
h3 {
  font-size: 16px;
}

.header {
  width: 100%;
  min-height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #e6e6e6;
  padding: 8px 20px;
  position: relative;
  z-index: 100;
  flex-wrap: wrap;
  gap: 12px;
}

.header__title {
  display:flex;
  flex-direction: column;
  gap: 0px;
  min-width: 0;
  flex-shrink: 1;
  transition: margin-left 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

/* Рельс скрыт: уводим приветствие вправо, чтобы не налезала плавающая кнопка
   возврата меню (#510). */
.header__title--shifted {
  margin-left: 45px;
}

.header__subtitle {
  font-size: 12px;
  color: #a2a2a2;
  font-weight: 500;
}

.header__info {
  display: flex;
  align-items: center;
  gap: 15px;
  position: relative;
  flex-wrap: wrap;
  justify-content: flex-end;
}

/* На десктопе overflow-обёртка прозрачна для layout (display:contents) - её
   дети рендерятся как обычные элементы .header__info. На мобилке (<768)
   становится реальным блоком с выпадающим меню "⋯" (feedback/broadcast/time/bell). */
.header__overflow-wrap {
  display: contents;
}

.header__overflow {
  display: contents;
}

.header__overflow-toggle {
  display: none;
}

.feedback-btn {
  height: 35px;
  font-size: 14px;
  color: #6E4A3A;
  border: none;
  outline: none;
  font-weight: 500;
  background: transparent;
  cursor: pointer;
  white-space: nowrap;
  padding: 0 15px;
  text-decoration: underline;
  text-decoration-color: transparent;
  transition: text-decoration-color 0.2s ease;
  text-underline-position: under;
}

.feedback-btn:hover {
  text-decoration-color: #6E4A3A;
}

.broadcast {
  width: fit-content;
  padding: 0 15px;
  height: 35px;
  font-size: 14px;
  color: #856404;
  border: 1px solid #fff3cd;
  outline: none;
  border-radius: 50px;
  font-weight: 500;
  background: #fff3cd;
  cursor: pointer;
  white-space: nowrap;
  transition: filter 0.2s ease;
}

.broadcast:hover {
  filter: brightness(0.96);
}

.broadcast--important {
  color: #c62828;
  background: #ffb3b3;
  border-color: #ffb3b3;
}

.broadcast--important:hover {
  filter: brightness(0.96);
}

.time {
  font-size: 16px;
  color: #a2a2a2;
  min-width: 160px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.user__notifications {
  width: 35px;
  height: 35px;
  border-radius: 50%;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #e6e6e6;
  box-shadow: 0 2px 2px rgba(0,0,0,0.05);
  position: relative;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.user__notifications--active .notifications__icon,
.user__notifications--active .notifications__icon:hover {
  filter: grayscale(100%) brightness(0.6);
}

.notifications__badge {
  position: absolute;
  top: -4px;
  right: -4px;
  background-color: #f14c4c;
  color: #fff;
  font-size: 11px;
  font-weight: 600;
  min-width: 18px;
  height: 18px;
  border-radius: 9px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 4px;
  line-height: 1;
}

.notifications__icon {
  width: 20px;
  height: 20px;
  cursor: pointer;
}

.notifications__icon:hover {
  filter: contrast(0.01);
}

.appl-btn__container {
  width: 155px;
  height: 26px;
  border-radius: 50px;
  background-color: #f2f2f2;
}

.appl-btn {
  position: relative;
  height: 26px;
  width: fit-content;
  padding: 0 16px;
  font-size: 13px;
  color: #000;
  background-color: #fff;
  border: 1px solid #4F5BDF;
  outline: none;
  cursor: pointer;
  font-weight: 400;
  border-radius: 13px;
  transition: .2s;
  box-shadow: 0 2px 2px rgba(0,0,0,0.05);
}

.appl-btn:hover {
  background-color: #e6e6e6;
}

/* Иконка "+" - видна только на узком экране (кнопка схлопывается в иконку,
   не переносится на новую строку - директива юзера). */
.appl-btn__icon {
  display: none;
}

.appl-btn--fixed {
  position: fixed;
  z-index: 1000;
}

/* Стили для фиксированной кнопки при скрытии шапки */
.appl-btn--fixed {
  position: fixed;
  top: 20px;
  right: 20px;
  z-index: 999;
  animation: slide-down 250ms cubic-bezier(0.4, 0, 0.2, 1);
}

@keyframes slide-down {
  from {
    transform: translateY(-12px);
    opacity: 0;
  }
  to {
    transform: translateY(0);
    opacity: 1;
  }
}

/* Burger-кнопка - только на мобильном */
.header__burger {
  display: none;
  width: 44px;
  height: 44px;
  padding: 0;
  border: none;
  background: transparent;
  cursor: pointer;
  flex-direction: column;
  gap: 4px;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: 8px;
  transition: background 0.2s;
}

.header__burger:hover {
  background: #f0f0f5;
}

.header__burger span {
  display: block;
  width: 22px;
  height: 2px;
  background: #333;
  border-radius: 1px;
}

/* Адаптивность */
@media (max-width: 768px) {
  .header {
    padding: 0 12px;
    gap: 8px;
  }

  .header__burger {
    display: flex;
  }

  .header__info {
    gap: 10px;
  }

  /* Приветствие + подзаголовок "Мы рады..." скрыты на мобилке (директива юзера) */
  .header__title {
    display: none;
  }

  .header__overflow-wrap {
    display: block;
    position: relative;
  }

  .header__overflow-toggle {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    border-radius: 50%;
    border: 1px solid #e6e6e6;
    background: #fff;
    cursor: pointer;
    font-size: 20px;
    line-height: 1;
    color: #555;
    padding: 0;
    flex-shrink: 0;
  }

  .header__overflow-toggle:hover {
    background: #f5f5f5;
  }

  /* Вторичные иконки (feedback/broadcast/time/bell) сворачиваются в overflow-меню */
  .header__overflow {
    display: none;
  }

  .header__overflow.header__overflow--open {
    display: flex;
    flex-direction: column;
    align-items: stretch;
    position: absolute;
    top: calc(100% + 8px);
    right: 0;
    min-width: 220px;
    background: #fff;
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.14);
    padding: 8px;
    gap: 4px;
    z-index: 300;
  }

  .header__overflow--open .feedback-btn,
  .header__overflow--open .broadcast,
  .header__overflow--open .time {
    width: 100%;
    min-height: 44px;
    display: flex;
    align-items: center;
    text-align: left;
    white-space: normal;
  }

  .header__overflow--open .broadcast {
    border-radius: 10px;
    justify-content: center;
  }

  .header__overflow--open .user__notifications {
    width: 44px;
    height: 44px;
    align-self: flex-start;
  }

  /* Кнопка "Подать заявку" остаётся в строке заголовка, схлопывается в иконку "+" */
  .appl-btn__container {
    width: 44px;
    height: 44px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .appl-btn {
    width: 44px;
    height: 44px;
    padding: 0;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 20px;
    font-weight: 600;
  }

  .appl-btn__icon {
    display: inline-flex;
  }

  .appl-btn__label {
    display: none;
  }

  .appl-btn--fixed {
    right: 10px;
    top: 10px;
  }
}

@media (max-width: 480px) {
  .header {
    padding: 0 10px;
  }
}
</style>