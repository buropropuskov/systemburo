<template>
  <teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="modal-overlay"
        @click.self="close"
      >
        <div class="modal-content announcement-modal">
          <div class="modal-body">
            <!-- Заголовок перед описанием -->
            
            
            <div class="modal-info">
              <time class="modal-date">{{ formatDate(announcement?.created_at) }}</time>
              <span
                class="modal-type"
                :class="{ important: announcement?.is_important }"
              >
                {{ announcement?.is_important ? 'Важное объявление' : 'Объявление' }}
              </span>
            </div>
            <h3 class="modal-title">
              {{ announcement?.title }}
            </h3>
            <p class="modal-description">
              {{ announcement?.description }}
            </p>
            <div
              v-if="announcement?.full_text"
              class="modal-full-text"
            >
              {{ announcement.full_text }}
            </div>
          </div>
          <div class="modal-footer">
            <button
              class="btn close-modal-btn"
              @click="close"
            >
              Закрыть
            </button>
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script>
export default {
  name: 'AnnouncementModal',
  props: {
    show: {
      type: Boolean,
      default: false
    },
    announcement: {
      type: Object,
      default: null
    }
  },
  emits: ['update:show', 'close'],
  methods: {
    formatDate(dateString) {
      if (!dateString) return '';
      const date = new Date(dateString);
      return date.toLocaleDateString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
      }).replace(',', '');
    },
    close() {
      this.$emit('update:show', false);
      this.$emit('close');
    }
  }
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
}

.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: all 0.4s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-from .modal-content,
.modal-fade-leave-to .modal-content {
  opacity: 0;
  transform: scale(0.9) translateY(-20px);
}

.modal-content {
  background: #fff;
  border-radius: 50px;
  width: 600px;
  max-width: 90vw;
  max-height: 80vh;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.announcement-modal .modal-content {
  width: 540px;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 30px 0;
  flex-shrink: 0;
}

.modal-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: #1a1a1a;
  flex: 1;
  padding-bottom: 15px;
}

.announcement-title {
  margin: 0 0 10px 0;
  font-size: 18px;
  font-weight: 600;
  color: #4F5BDF;
}

.modal-close {
  background: none;
  border: none;
  cursor: pointer;
  padding: 6px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s ease;
}

.modal-close:hover {
  background-color: #f5f5f5;
}

.modal-body {
  padding: 20px 40px;
  overflow-y: auto;
  flex: 1;
}

.modal-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px 0;
}

.modal-date {
  font-size: 12px;
  color: #a2a2a2;
}

.modal-type {
  font-size: 11px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 20px;
  background: #fff3cd;
  color: #856404;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.modal-type.important {
  background: #ffb3b3;
  color: #c62828;
}

.modal-description {
  font-size: 14px;
  line-height: 1.5;
  color: #333;
  margin: 0 0 20px 0;
}

.modal-full-text {
  font-size: 14px;
  line-height: 1.6;
  color: #666;
  padding-top: 16px;
  border-top: 1px solid #f0f0f0;
  margin-top: 8px;
}

.modal-footer {
  padding: 16px 30px 24px;
  border-top: 1px solid #f0f0f0;
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  flex-shrink: 0;
}

.btn {
  padding: 8px 24px;
  font-size: 13px;
  font-weight: 500;
  border-radius: 30px;
  cursor: pointer;
  border: 1px solid;
  transition: all 0.2s ease;
}

.close-modal-btn {
  background: #4F5BDF;
  color: white;
  border-color: #4F5BDF;
}

.close-modal-btn:hover {
  background: #3a45c0;
}

@media (max-width: 768px) {
  .modal-content {
    width: 95vw;
  }
  
  .modal-header {
    padding: 16px 20px 0;
  }
  
  .modal-body {
    padding: 16px 20px;
  }
  
  .modal-footer {
    padding: 12px 20px 20px;
  }
}
</style>