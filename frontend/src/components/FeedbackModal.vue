<template>
  <!-- Модальное окно обратной связи -->
  <teleport to="body">
    <transition name="modal-overlay" @after-leave="handleAfterLeave">
      <div 
        v-if="show" 
        class="modal-overlay" 
        @click.self="handleOverlayClick"
      >
        <transition name="modal" @after-leave="emitClose">
          <div 
            v-if="showContent" 
            class="modal"
          >
            <div class="modal__header">
              <h3 class="modal__title">Сообщить о проблеме</h3>
              <button class="modal__close" @click="handleCloseClick" aria-label="Закрыть">
                <span class="close-icon">&times;</span>
              </button>
            </div>
            <div class="modal__body">
              <div class="modal__content">
                <label for="feedback-textarea" class="textarea-label">
                    Ниже вы можете дать обратную связь по работе системы. Расскажите о вашей проблеме, что не работает, с чем вам нужна помощь. Вы можете оставить предложение по улучшению работы системы.
                </label>
                <div class="textarea-wrapper">
                  <textarea 
                    v-model="message" 
                    id="feedback-textarea"
                    placeholder="Например: не работает кнопка отправки формы на странице..."
                    class="feedback-textarea"
                    :class="{ 'feedback-textarea--error': hasError }"
                    rows="6"
                    ref="textareaRef"
                    @keydown.enter.prevent="handleEnterKey"
                    @input="handleInput"
                    :disabled="isSubmitting"
                  ></textarea>
                  <div 
                    class="textarea-counter-wrapper"
                    :class="{ 
                      'textarea-counter-wrapper--error': isOverLimit,
                      'textarea-counter-wrapper--warning': isNearLimit 
                    }"
                  >
                    {{ message.length }}/{{ maxLength }}
                  </div>
                </div>
                <div v-if="error" class="error-message" role="alert">
                  <span class="error-icon">⚠</span>
                  {{ error }}
                </div>
                <div v-if="success" class="success-message" role="alert">
                  <span class="success-icon">✓</span>
                  {{ success }}
                </div>
              </div>
            </div>
            <div class="modal__footer">
              <button 
                class="modal-btn modal-btn--cancel" 
                @click="handleCancelClick"
                :disabled="isSubmitting"
              >
                Отмена
              </button>
              <button 
                class="modal-btn modal-btn--submit" 
                @click="submitFeedback" 
                :disabled="isSubmitDisabled"
                :class="{ 
                  'modal-btn--disabled': isSubmitDisabled,
                  'modal-btn--loading': isSubmitting 
                }"
              >
                <span v-if="isSubmitting" class="submit-spinner"></span>
                <span v-else class="submit-text">
                  {{ submitButtonText }}
                </span>
              </button>
            </div>
          </div>
        </transition>
      </div>
    </transition>
  </teleport>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
export default {
  name: 'FeedbackModal',
  
  props: {
    show: {
      type: Boolean,
      required: true
    },
    autoFocus: {
      type: Boolean,
      default: true
    },
    // Сохранять ли текст при закрытии
    preserveTextOnClose: {
      type: Boolean,
      default: true
    }
  },
  
  emits: ['close', 'submitted', 'update:show'],
  
  data() {
    return {
      message: '',
      isSubmitting: false,
      error: '',
      success: '',
      showContent: false,
      minLength: 10,
      maxLength: 1000,
      warningThreshold: 800,
      escListener: null,
      // Сохраняем текст при закрытии
      savedMessage: '',
      // Флаг, что нужно сохранить текст при следующем закрытии
      shouldSaveOnClose: true
    };
  },
  
  computed: {
    hasError() {
      return this.error && this.message.length > 0;
    },
    
    isOverLimit() {
      return this.message.length > this.maxLength;
    },
    
    isNearLimit() {
      return this.message.length >= this.warningThreshold && this.message.length <= this.maxLength;
    },
    
    isTooShort() {
      return this.message.trim().length > 0 && this.message.trim().length < this.minLength;
    },
    
    isSubmitDisabled() {
      return this.isSubmitting || 
             !this.message.trim() || 
             this.isOverLimit || 
             this.isTooShort;
    },
    
    submitButtonText() {
      if (this.isOverLimit) return 'Слишком длинный текст';
      if (this.isTooShort) return `Минимум ${this.minLength} символов`;
      if (!this.message.trim()) return 'Введите сообщение';
      return 'Отправить';
    }
  },
  
  watch: {
    show(newVal) {
      if (newVal) {
        this.resetErrors();
        this.showContent = true;
        this.shouldSaveOnClose = true;
        this.setupEscListener();
        this.$nextTick(() => {
          if (this.autoFocus) {
            this.$refs.textareaRef?.focus();
          }
        });
      } else {
        this.showContent = false;
        this.removeEscListener();
      }
    },
    
    message() {
      this.error = '';
      this.success = '';
    }
  },
  
  created() {
    // При создании компонента восстанавливаем сохраненное сообщение
    this.message = this.savedMessage;
  },
  
  methods: {
    resetErrors() {
      this.error = '';
      this.success = '';
      this.isSubmitting = false;
    },
    
    saveMessage() {
      if (this.shouldSaveOnClose && this.preserveTextOnClose) {
        this.savedMessage = this.message;
      }
    },
    
    clearSavedMessage() {
      this.savedMessage = '';
      this.message = '';
    },
    
    handleCloseClick() {
      this.saveMessage();
      this.closeModal();
    },
    
    handleCancelClick() {
      this.saveMessage();
      this.closeModal();
    },
    
    closeModal() {
      this.$emit('update:show', false);
    },
    
    emitClose() {
      this.$emit('close');
    },
    
    handleAfterLeave() {
      // Восстанавливаем прокрутку после завершения анимации
      document.body.style.overflow = '';
    },
    
    handleOverlayClick() {
      if (!this.isSubmitting) {
        this.saveMessage();
        this.closeModal();
      }
    },
    
    handleEnterKey(event) {
      if (event.shiftKey) {
        // Shift+Enter - новая строка
        this.message += '\n';
      } else if (!this.isSubmitDisabled) {
        // Enter - отправить
        this.submitFeedback();
      }
    },
    
    handleInput() {
      if (this.message.length > this.maxLength) {
        this.message = this.message.substring(0, this.maxLength);
      }
    },
    
    async submitFeedback() {
      if (this.isSubmitDisabled) return;
      
      const trimmedMessage = this.message.trim();
      
      if (trimmedMessage.length < this.minLength) {
        this.error = `Сообщение должно содержать минимум ${this.minLength} символов`;
        return;
      }
      
      this.isSubmitting = true;
      this.error = '';
      this.success = '';
      
      try {
        const authStore = useAuthStore();
        if (!authStore.token) {
          this.error = "Вы не авторизованы. Пожалуйста, войдите в систему.";
          this.isSubmitting = false;
          return;
        }
        
        const response = await apiRequest("/feedback", {
          method: "POST",
          body: JSON.stringify({
            message: trimmedMessage,
            timestamp: new Date().toISOString(),
            userAgent: navigator.userAgent
          }),
        });
        
        if (response.ok) {
          this.success = "Сообщение отправлено успешно! Мы рассмотрим его в ближайшее время.";
          this.shouldSaveOnClose = false; // Не сохраняем текст после успешной отправки
          
          // Автоматически закрываем модалку через 2 секунды после успешной отправки
          setTimeout(() => {
            this.clearSavedMessage();
            this.closeModal();
            this.$emit('submitted', trimmedMessage);
          }, 2000);
          
        } else {
          let errorMessage = "Ошибка при отправке сообщения";
          try {
            const errorData = await response.json();
            errorMessage = errorData.message || errorMessage;
          } catch (e) {
            console.error("Ошибка парсинга ответа:", e);
          }
          
          this.error = errorMessage;
          this.isSubmitting = false;
        }
      } catch (error) {
        console.error("Ошибка при отправке обратной связи:", error);
        this.error = "Ошибка сети. Пожалуйста, проверьте подключение к интернету и попробуйте позже.";
        this.isSubmitting = false;
      }
    },
    
    setupEscListener() {
      this.escListener = (event) => {
        if (event.key === 'Escape' && this.show && !this.isSubmitting) {
          this.saveMessage();
          this.closeModal();
        }
      };
      document.addEventListener('keydown', this.escListener);
    },
    
    removeEscListener() {
      if (this.escListener) {
        document.removeEventListener('keydown', this.escListener);
        this.escListener = null;
      }
    }
  },
  
  mounted() {
    
    this.$watch(
      () => this.show,
      (newVal) => {
        if (newVal) {
          document.body.style.overflow = 'hidden';
        }
      },
      { immediate: true }
    );
  },
  
  beforeUnmount() {
    this.removeEscListener();
    // Восстанавливаем прокрутку при размонтировании
    document.body.style.overflow = '';
  }
};
</script>

<style scoped>
/* Анимации оверлея */
.modal-overlay-enter-active,
.modal-overlay-leave-active {
  transition: opacity 0.3s ease;
}

.modal-overlay-enter-from,
.modal-overlay-leave-to {
  opacity: 0;
}

/* Анимации модального окна */
.modal-enter-active,
.modal-leave-active {
  transition: all 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
  transform: translateY(30px) scale(0.95);
}

.modal-enter-to,
.modal-leave-from {
  opacity: 1;
  transform: translateY(0) scale(1);
}

/* Для мобильных устройств */
@media (max-width: 768px) {
  .modal-enter-active,
  .modal-leave-active {
    transition: all 0.3s ease-out;
  }
  
  .modal-enter-from {
    transform: translateY(100%);
  }
  
  .modal-enter-to {
    transform: translateY(0);
  }
  
  .modal-leave-from {
    transform: translateY(0);
  }
  
  .modal-leave-to {
    transform: translateY(100%);
  }
}

.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.75);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  padding: 20px;
  backdrop-filter: blur(4px);
}

.modal {
  background: #ffffff;
  border-radius: 25px;
  width: 100%;
  max-width: 500px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  overflow: hidden;
  position: relative;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
}

.modal__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px 20px;
  border-bottom: 1px solid #f0f0f0;
  background: #fff;
  flex-shrink: 0;
}

.modal__title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: #1a1a1a;
  line-height: 1.3;
}

.modal__close {
  background: none;
  border: none;
  cursor: pointer;
  padding: 8px;
  margin: -8px;
  border-radius: 8px;
  transition: all 0.15s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  color: #666;
}

.modal__close:hover:not(:disabled) {
  background-color: #f5f5f5;
  color: #333;
}

.modal__close:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.close-icon {
  font-size: 28px;
  line-height: 1;
  font-weight: 300;
}

.modal__body {
  flex: 1;
  overflow-y: auto;
  padding: 0;
}

.modal__content {
  padding: 15px 20px;
}

.textarea-label {
  display: block;
  font-size: 13px;
  color: #666;
  padding-bottom: 20px;
  font-weight: 500;
}

.textarea-wrapper {
  position: relative;
  width: 100%;
}

.feedback-textarea {
  width: 100%;
  padding: 16px;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  font-size: 15px;
  font-family: inherit;
  line-height: 1.5;
  resize: vertical;
  min-height: 140px;
  max-height: 300px;
  transition: all 0.2s ease;
  box-sizing: border-box;
  outline: none;
  background: #fff;
  color: #1a1a1a;
  display: block;
  padding-bottom: 30px; /* Добавляем отступ снизу для счетчика */
}

.feedback-textarea:focus {
  border-color: #4F5BDF;
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.1);
}

.feedback-textarea:hover:not(:disabled) {
  border-color: #b0b0b0;
}

.feedback-textarea:disabled {
  background-color: #f9f9f9;
  cursor: not-allowed;
  opacity: 0.7;
}

.feedback-textarea::placeholder {
  color: #999;
}

.feedback-textarea--error {
  border-color: #dc3545;
  background-color: #fffafa;
}

.feedback-textarea--error:focus {
  border-color: #dc3545;
  box-shadow: 0 0 0 3px rgba(220, 53, 69, 0.1);
}

.textarea-counter-wrapper {
  position: absolute;
  bottom: 8px;
  right: 12px;
  font-size: 12px;
  color: #999;
  background-color: rgba(255, 255, 255, 0.8);
  padding: 2px 6px;
  border-radius: 8px;
  pointer-events: none;
  font-variant-numeric: tabular-nums;
  font-weight: 500;
  transition: all 0.2s ease;
  backdrop-filter: blur(2px);
}

.textarea-counter-wrapper--warning {
  color: #ff9800;
  background-color: rgba(255, 248, 225, 0.9);
}

.textarea-counter-wrapper--error {
  color: #dc3545;
  background-color: rgba(255, 245, 245, 0.9);
  font-weight: 600;
}

.error-message {
  color: #dc3545;
  font-size: 14px;
  margin-top: 12px;
  padding: 10px 12px;
  background-color: #fff5f5;
  border-radius: 6px;
  border-left: 3px solid #dc3545;
  animation: error-shake 0.4s ease;
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.error-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.success-message {
  color: #28a745;
  font-size: 14px;
  margin-top: 12px;
  padding: 10px 12px;
  background-color: #f8fff9;
  border-radius: 6px;
  border-left: 3px solid #28a745;
  animation: success-appear 0.4s ease;
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.success-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.modal__footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 15px 20px;
  border-top: 1px solid #f0f0f0;
  background: #fff;
  flex-shrink: 0;
}

.modal-btn {
  padding: 12px 28px;
  border: none;
  border-radius: 15px;
  font-size: 15px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  min-width: 100px;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  outline: none;
  font-family: inherit;
}

.modal-btn--cancel {
  background-color: #f5f5f5;
  color: #666;
  border: 1px solid #e0e0e0;
}

.modal-btn--cancel:hover:not(:disabled) {
  background-color: #e8e8e8;
  color: #333;
}

.modal-btn--cancel:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.modal-btn--submit {
  background-color: #4F5BDF;
  color: white;
  border: 1px solid #4F5BDF;
  width: 240px;
}

.modal-btn--submit:hover:not(.modal-btn--disabled):not(:disabled) {
  background-color: #3a45c5;
  border-color: #3a45c5;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(79, 91, 223, 0.3);
}

.modal-btn--submit:active:not(.modal-btn--disabled):not(:disabled) {
  transform: translateY(0);
  box-shadow: 0 2px 6px rgba(79, 91, 223, 0.2);
}

.modal-btn--disabled {
  background-color: #a0a5e8 !important;
  border-color: #a0a5e8 !important;
  color: rgba(255, 255, 255, 0.7) !important;
  cursor: not-allowed !important;
  transform: none !important;
  box-shadow: none !important;
}

.modal-btn--loading {
  color: transparent !important;
}

.submit-spinner {
  display: inline-block;
  width: 18px;
  height: 18px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-radius: 50%;
  border-top-color: white;
  animation: spinner-rotate 0.8s linear infinite;
  position: absolute;
}

.submit-text {
  display: inline-block;
}

@keyframes error-shake {
  0%, 100% {
    transform: translateX(0);
  }
  25% {
    transform: translateX(-4px);
  }
  75% {
    transform: translateX(4px);
  }
}

@keyframes success-appear {
  from {
    opacity: 0;
    transform: translateY(-10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes spinner-rotate {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}

/* Адаптивность для мобильных устройств */
@media (max-width: 768px) {
  .modal {
    max-width: 100%;
    border-radius: 16px 16px 0 0;
    margin-top: auto;
    margin-bottom: 0;
  }
  
  .modal-overlay {
    align-items: flex-end;
    padding: 0;
    backdrop-filter: blur(2px);
  }
  
  .modal__header {
    padding: 20px;
  }
  
  .modal__title {
    font-size: 18px;
  }
  
  .modal__content {
    padding: 20px;
  }
  
  .modal__footer {
    padding: 20px;
    flex-direction: column-reverse;
    gap: 10px;
  }
  
  .modal-btn {
    width: 100%;
    padding: 14px;
  }
  
  .modal-btn--submit {
    width: 100%;
  }
  
  .feedback-textarea {
    min-height: 120px;
    padding-bottom: 28px;
  }
  
  .textarea-counter-wrapper {
    bottom: 6px;
    right: 10px;
    font-size: 11px;
  }
}

@media (max-width: 576px) {
  .modal__header {
    padding: 16px;
  }
  
  .modal__content {
    padding: 16px;
  }
  
  .modal__footer {
    padding: 16px;
  }
  
  .modal__title {
    font-size: 16px;
  }
  
  .textarea-label {
    font-size: 13px;
  }
  
  .feedback-textarea {
    padding: 12px;
    padding-bottom: 26px;
    font-size: 14px;
  }
  
  .textarea-counter-wrapper {
    bottom: 5px;
    right: 8px;
    padding: 1px 5px;
  }
}
</style>