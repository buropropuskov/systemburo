<template>
  <header class="header" ref="header">
    <div class="header__title">
      <template v-if="loading">
        <SkeletonLine width="200px" height="20px" />
        <SkeletonLine width="150px" height="14px" />
      </template>
      <template v-else>
        <h3>Добрый день, {{ displayName }}!</h3>
        <p class="header__subtitle">Мы рады, что вы здесь!</p>
      </template>
    </div>

    <div class="header__info">
      <button class="feedback-btn" data-testid="header-button-feedback" @click="openFeedbackModal">
        Сообщить о проблеме
      </button>
      <button class="broadcast">
        Важное объявление
      </button>
      <p class="time">
        {{ currentDateTime }}
      </p>
      <div class="language-selector">
        <p class="current-language">RU</p>
      </div>
      <div class="user__notifications">
        <img src="@/assets/icons/messages.png" class="notifications__icon" alt="Сообщения" />
      </div>
      <div class="appl-btn__container">
        <button class="appl-btn" data-testid="header-button-submit-app" @click="navigateToSubmit" :class="{ 'appl-btn--fixed': isHeaderHidden }">
          Подать заявку
        </button>
      </div>
    </div>

    <!-- Используем отдельный компонент модального окна -->
    <FeedbackModal
      v-model:show="showFeedbackModal"
      @submitted="handleFeedbackSubmitted"
    />
  </header>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import FeedbackModal from '@/components/FeedbackModal.vue';
import { SkeletonLine } from '@/components/ui';

export default {
  name: 'TheHeader',
  components: {
    FeedbackModal,
    SkeletonLine,
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
      showFeedbackModal: false
    };
  },
  computed: {
    displayName() {
      return this.userFirstName || this.userLastName || '';
    }
  },
  watch: {
    '$route'() {
      this.fetchUserData();
    }
  },
  methods: {
    openFeedbackModal() {
      this.showFeedbackModal = true;
    },
    handleFeedbackSubmitted(message) {
      console.log('Обратная связь отправлена:', message);
      // Если мы на странице обратной связи, можно обновить список
      if (this.$route.path === '/feedback') {
        this.$emit('refresh-feedback');
      }
    },
    navigateToSubmit() {
      this.$router.push('/submit-form');
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
  mounted() {
    this.fetchUserData();
    this.startDateTimeTimer();
    this.$nextTick(() => {
      this.initIntersectionObserver();
    });
  },
  beforeUnmount() {
    if (this.timer) {
      clearInterval(this.timer);
    }
    
    if (this.observer) {
      this.observer.disconnect();
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
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #e6e6e6;
  padding: 0 20px;
  position: relative;
  z-index: 100;
}

.header__title {
  display:flex;
  flex-direction: column;
  gap: 0px;
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
  color: #6E4A3A;
  border: 1px solid #e6e6e6;
  outline: none;
  border-radius: 50px;
  font-weight: 500;
  background: linear-gradient(to right, rgba(255,255,240,1), rgba(255,246,217,1));
  cursor: pointer;
  white-space: nowrap;
}

.broadcast:hover {
  background: linear-gradient(to right, rgba(255,250,220,1), rgba(255,240,200,1));
}

.time {
  font-size: 16px;
  color: #a2a2a2;
  min-width: 160px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.current-language {
  font-size: 16px;
  font-weight: 500;
  color: #a2a2a2;
  cursor: pointer;
}

.current-language:hover {
  color: #333
}

.user__notifications {
  width: fit-content;
  height: 35px;
  border-radius: 50px;
  padding: 0 15px;
  display: flex;
  gap:20px;
  align-items: center;
  justify-content: center;
  border: 1px solid #e6e6e6;
  box-shadow: 0 2px 2px rgba(0,0,0,0.05);
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
  height: 30px;
  border-radius: 50px;
  background-color: #f2f2f2;
}

.appl-btn {
  position: fixed;
  height: 30px;
  width: fit-content;
  padding: 0 20px;
  font-size: 15px;
  color: #000;
  background-color: #fff;
  border: 1px solid #4F5BDF;
  outline: none;
  cursor: pointer;
  font-weight: 400;
  border-radius: 15px;
  transition: .2s;
  box-shadow: 0 2px 2px rgba(0,0,0,0.05);
}

.appl-btn:hover {
  background-color: #e6e6e6;
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
  animation: slide-down 0.3s ease;
}

@keyframes slide-down {
  from {
    transform: translateY(-20px);
    opacity: 0;
  }
  to {
    transform: translateY(0);
    opacity: 1;
  }
}

/* Адаптивность */
@media (max-width: 768px) {
  .header__info {
    gap: 10px;
  }
  
  .feedback-btn {
    padding: 0 10px;
    font-size: 13px;
  }
  
  .broadcast {
    padding: 0 12px;
    font-size: 13px;
  }
  
  .time {
    min-width: 140px;
    font-size: 14px;
  }
  
  .appl-btn--fixed {
    right: 10px;
    top: 10px;
  }
}

@media (max-width: 576px) {
  .header {
    padding: 0 10px;
  }
  
  .header__title h3 {
    font-size: 14px;
  }
  
  .header__subtitle {
    font-size: 11px;
  }
  
  .user__notifications {
    padding: 0 10px;
    gap: 10px;
  }
  
  .feedback-btn {
    font-size: 12px;
    padding: 0 8px;
  }
}
</style>