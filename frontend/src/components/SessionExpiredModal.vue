<template>
  <Teleport to="body">
    <div
      class="modal-overlay"
      @click.self="handleOverlayClick"
    >
      <div class="session-expired-modal">
        <div class="modal-header">
          <h2 class="modal-title">
            Ваш сеанс скоро закончится
          </h2>
        </div>
      
        <div class="modal-body">
          <div class="time-remaining">
            {{ formattedTime }}
          </div>
        
          <div class="countdown">
            <div class="countdown-bar">
              <div
                class="countdown-progress"
                :style="progressStyle"
              />
            </div>
          </div>
        
          <p class="modal-message">
            Для продолжения работы необходимо продлить сеанс
          </p>
        </div>
      
        <div class="modal-footer">
          <button
            class="btn btn-secondary"
            @click="logout"
          >
            Выйти
          </button>
          <button
            class="btn btn-primary"
            @click="extendSession"
          >
            Продлить сеанс
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script>
export default {
  name: 'SessionExpiredModal',
  props: {
    timeRemaining: {
      type: Number,
      required: true
    },
    maxTime: {
      type: Number,
      default: 600
    }
  },
  emits: ['extend-session', 'logout'],
  computed: {
    formattedTime() {
      const minutes = Math.floor(this.timeRemaining / 60);
      const seconds = this.timeRemaining % 60;
      return `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
    },
    progressStyle() {
      const progress = Math.max(0, Math.min(100, (this.timeRemaining / this.maxTime) * 100));
      const percent = progress / 100;
      const red = Math.floor(255 * (1 - percent));
      const green = Math.floor(255 * percent);
      const color = `rgb(${red}, ${green}, 0)`;

      return {
        width: `${progress}%`,
        backgroundColor: color,
        backgroundImage: `linear-gradient(90deg, ${color}, ${color})`
      };
    }
  },
  methods: {
    extendSession() {
      this.$emit('extend-session');
    },
    
    logout() {
      this.$emit('logout');
    },
    
    handleOverlayClick() {
      // Блокируем закрытие модального окна по клику на оверлей
    }
  }
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background-color: var(--overlay);
  display: flex;
  justify-content: center;
  align-items: center;
  /* Истечение сессии - блокирующий takeover, должен лежать поверх любых открытых стопок
     модалок (деталь/карточка/override/история), иначе re-auth прячется за ними (#481). */
  z-index: 25000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

.session-expired-modal {
  background: var(--surface);
  border-radius: 50px;
  padding: 30px;
  max-width: 400px;
  width: 90%;
  box-shadow: 0 10px 30px var(--shadow-drop);
}

.modal-header {
  margin-bottom: 20px;
  text-align: center;
}

.modal-title {
  color: var(--text);
  font-size: 1.0em;
  font-weight: 400;
  margin: 0;
}

.modal-body {
  margin-bottom: 25px;
  text-align: center;

}

.time-remaining {
  text-align: center;
  font-size: 70px;
  font-weight: bold;
  color: var(--text);
  padding: 10px;
  width: 100%;
  background: var(--surface-2);
  border-radius: 50px;
  border: 1px solid var(--border);
  line-height: 1;
  font-variant-numeric: tabular-nums;

}

.countdown {
  margin: 20px auto;
  max-width: 150px;
}

.countdown-bar {
  width: 100%;
  height: 4px;
  background-color: var(--border);
  border-radius: 10px;
  overflow: hidden;
}

.countdown-progress {
  height: 100%;
  border-radius: 10px;
  transition: width 1s linear, background-color 0.3s ease;
}

.modal-message {
  color: var(--text-muted);
  font-size: 0.9em;
  margin: 15px 0 0 0;
  line-height: 1.4;
}

.modal-footer {
  display: flex;
  gap: 12px;
  justify-content: center;
}

.btn {
  padding: 12px 24px;
  border: none;
  border-radius: 50px;
  font-size: 0.9em;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s ease;
  min-width: 120px;
}

.btn-primary {
  background: var(--accent);
  color: var(--accent-contrast);
}

.btn-primary:hover {
  background: var(--accent-hover);
}

.btn-secondary {
  background: var(--surface-2);
  color: var(--text);
  border: 1px solid var(--border);
}

.btn-secondary:hover {
  background: var(--row-hover);
}

@media (max-width: 480px) {
  .session-expired-modal {
    padding: 25px 20px;
  }
  
  .time-remaining {
    font-size: 48px;
  }
  
  .modal-footer {
    flex-direction: column;
  }
  
  .btn {
    width: 100%;
  }
}
</style>