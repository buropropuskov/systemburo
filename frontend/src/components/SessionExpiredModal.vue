<template>
  <div class="modal-overlay" @click.self="handleOverlayClick">
    <div class="session-expired-modal">
      <div class="modal-header">
        <h2 class="modal-title">Ваш сеанс скоро закончится</h2>
      </div>
      
      <div class="modal-body">
        <div class="time-remaining">{{ formattedTime }}</div>
        
        <div class="countdown">
          <div class="countdown-bar">
            <div class="countdown-progress" :style="progressStyle"></div>
          </div>
        </div>
        
        <p class="modal-message">
          Для продолжения работы необходимо продлить сеанс
        </p>
      </div>
      
      <div class="modal-footer">
        <button class="btn btn-secondary" @click="logout">
          Выйти
        </button>
        <button class="btn btn-primary" @click="extendSession">
          Продлить сеанс
        </button>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'SessionExpiredModal',
  props: {
    timeRemaining: {
      type: Number,
      required: true,
      default: 300
    },
    maxTime: {
      type: Number,
      default: 600
    }
  },
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
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 10000;
  backdrop-filter: blur(3px);
}

.session-expired-modal {
  background: white;
  border-radius: 50px;
  padding: 30px;
  max-width: 400px;
  width: 90%;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
}

.modal-header {
  margin-bottom: 20px;
  text-align: center;
}

.modal-title {
  color: #000;
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
  color: #000;
  padding: 10px;
  width: 100%;
  background: #f3f3f3;
  border-radius: 50px;
  border: 1px solid #e6e6e6;
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
  background-color: #e6e6e6;
  border-radius: 10px;
  overflow: hidden;
}

.countdown-progress {
  height: 100%;
  border-radius: 10px;
  transition: width 1s linear, background-color 0.3s ease;
}

.modal-message {
  color: #a2a2a2;
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
  background: #4F5BDF;
  color: white;
}

.btn-primary:hover {
  background: #3a45c8;
}

.btn-secondary {
  background: #f8f8f8;
  color: #333;
  border: 1px solid #e6e6e6;
}

.btn-secondary:hover {
  background: #e8e8e8;
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