<template>
  <header class="header" ref="header">
    <div class="header__title">
      <h3>Добрый день, {{ userFirstName }}!</h3>
      <p class="header__subtitle">Мы рады, что Вы здесь!</p>
    </div>

    <div class="header__info">
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
        <img src="@/assets/icons/notifications.png" class="notifications__icon" alt="Уведомления" />
         <img src="@/assets/icons/messages.png" class="notifications__icon" alt="Уведомления" />
      </div>
      <button class="appl-btn" @click="navigateToSubmit" :class="{ 'appl-btn--fixed': isHeaderHidden }">
        + Новая заявка
      </button>
    </div>
  </header>
</template>

<script>
export default {
  name: 'TheHeader',
  data() {
    return {
      userFirstName: '',
      currentDateTime: '',
      timer: null,
      isHeaderHidden: false,
      observer: null
    };
  },
  methods: {
    navigateToSubmit() {
      this.$router.push('/submit-form');
    },
    async fetchUserData() {
      try {
        const token = localStorage.getItem("token");
        if (!token) {
          console.log("Пользователь не авторизован");
          return;
        }

        const response = await fetch("http://localhost:8080/users/me", {
          method: "GET",
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });

        if (response.ok) {
          const userData = await response.json();
          this.userFirstName = userData.first_name || '';
        } else {
          console.error("Ошибка при загрузке данных пользователя");
        }
      } catch (error) {
        console.error("Ошибка сети при загрузке данных пользователя:", error);
      }
    },
    updateDateTime() {
      const now = new Date();
      
      // Форматирование даты: день.месяц.год
      const day = String(now.getDate()).padStart(2, '0');
      const month = String(now.getMonth() + 1).padStart(2, '0');
      const year = now.getFullYear();
      
      // Форматирование времени: часы:минуты:секунды
      const hours = String(now.getHours()).padStart(2, '0');
      const minutes = String(now.getMinutes()).padStart(2, '0');
      const seconds = String(now.getSeconds()).padStart(2, '0');
      
      this.currentDateTime = `${day}.${month}.${year} ${hours}:${minutes}:${seconds}`;
    },
    startDateTimeTimer() {
      // Обновляем время сразу при запуске
      this.updateDateTime();
      
      // Запускаем таймер для обновления каждую секунду
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
    // Очищаем таймер при уничтожении компонента
    if (this.timer) {
      clearInterval(this.timer);
    }
    
    // Отключаем observer
    if (this.observer) {
      this.observer.disconnect();
    }
  },
  watch: {
    // Обновляем данные при изменении маршрута (на случай смены пользователя)
    '$route'() {
      this.fetchUserData();
    }
  }
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
  gap: 25px;
  position: relative;
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

.appl-btn {
  height: 40px;
  width: fit-content;
  padding: 0 20px;
  font-size: 16px;
  color: #FFF;
  background-color: #4F5BDF;
  border: none;
  outline: none;
  cursor: pointer;
  font-weight: 500;
  border-radius: 10px;
  transition: .2s;
}

.appl-btn:hover {
  background-color: #7580fc;
}

.appl-btn--fixed {
  position: fixed;
  top: 10px;
  right: 20px;
  z-index: 1000;
  animation: slideIn 0.3s ease;
  border: 2px solid #e6e6e6;
}

@keyframes slideIn {
  from {
    transform: translateY(-20px);
    opacity: 0;
  }
  to {
    transform: translateY(0);
    opacity: 1;
  }
}

@media (max-width: 768px) {
  .appl-btn--fixed {
    right: 10px;
    top: 10px;
  }
}
</style>