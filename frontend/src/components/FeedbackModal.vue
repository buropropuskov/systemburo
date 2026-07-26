<template>
  <teleport to="body">
    <!-- Оверлей смонтирован пока show=true; при закрытии сперва проигрывается slide-down
         листа (showContent=false, оверлей на месте), а update:show=false эмитится только
         в @after-leave (onSheetLeft) - иначе inner-лист снимался вместе с оверлеем и slide
         не проигрывался. Затемнение подложки - классом is-visible (по showContent),
         transition background-color, чтобы фейд подложки шёл СИНХРОННО со слайдом и НЕ
         каскадил opacity на лист. -->
    <div
      v-if="show"
      class="modal-overlay"
      :class="{ 'is-visible': showContent }"
      @mousedown="onOverlayMousedown"
      @mouseup="onOverlayMouseup"
    >
      <transition
        name="modal"
        @after-leave="onSheetLeft"
      >
        <div
          v-if="showContent"
          class="modal"
          :class="{ 'is-dragging': sheetDragging }"
          :style="sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : null"
          @mousedown.stop
          @touchstart="onSheetTouchStart"
          @touchmove="onSheetTouchMove"
          @touchend="onSheetTouchEnd"
        >
          <div
            class="sheet-handle"
            aria-hidden="true"
          />
          <div class="modal__header">
            <h3 class="modal__title">
              Сообщить о проблеме
            </h3>
            <button
              class="modal__close"
              aria-label="Закрыть"
              @click="handleCloseClick"
            >
              <span class="close-icon">&times;</span>
            </button>
          </div>
          <div
            ref="modalScroll"
            class="modal__body"
          >
            <div class="modal__content">
              <label
                for="feedback-textarea"
                class="textarea-label"
              >
                Ниже вы можете дать обратную связь по работе системы. Расскажите о вашей проблеме, что не работает, с чем вам нужна помощь. Вы можете оставить предложение по улучшению работы системы.
              </label>
              <div class="textarea-wrapper">
                <textarea 
                  id="feedback-textarea" 
                  ref="textareaRef"
                  v-model="message"
                  placeholder="Например: не работает кнопка отправки формы на странице..."
                  class="feedback-textarea"
                  :class="{ 'feedback-textarea--error': hasError }"
                  rows="6"
                  :disabled="isSubmitting"
                  @keydown.enter.prevent="handleEnterKey"
                  @input="handleInput"
                />
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
              <div
                v-if="error"
                class="error-message"
                role="alert"
              >
                <span class="error-icon">⚠</span>
                {{ error }}
              </div>
            </div>
          </div>
          <div class="modal__footer">
            <button 
              class="modal-btn modal-btn--cancel" 
              :disabled="isSubmitting"
              @click="handleCancelClick"
            >
              Отмена
            </button>
            <button 
              class="modal-btn modal-btn--submit" 
              :disabled="isSubmitDisabled" 
              :class="{ 
                'modal-btn--disabled': isSubmitDisabled,
                'modal-btn--loading': isSubmitting 
              }"
              @click="submitFeedback"
            >
              <span
                v-if="isSubmitting"
                class="submit-spinner"
              />
              <span
                v-else
                class="submit-text"
              >
                {{ submitButtonText }}
              </span>
            </button>
          </div>
        </div>
      </transition>
    </div>
  </teleport>
</template>

<script>
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { ref, getCurrentInstance } from 'vue'
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { useDeletionsStore } from '@/stores/deletions'
import { useSwipeDismiss } from '@/composables/useSwipeDismiss'
export default {
  name: 'FeedbackModal',

  props: {
    show: {
      type: Boolean,
      required: true
    },
    autoFocus: {
      // Десктоп фокусирует textarea сразу (default true). Мобильный вызов из drawer'а
      // передаёт false: на bottom-sheet автофокус поднимает клавиатуру поверх листа.
      type: Boolean,
      default: true
    },
    preserveTextOnClose: {
      type: Boolean,
      default: true
    }
  },
  
  emits: ['close', 'submitted', 'update:show'],

  setup() {
    const inst = getCurrentInstance();
    const modalScroll = ref(null);
    // Свайп-вниз закрывает так же, как крестик/overlay - с сохранением текста.
    const swipe = useSwipeDismiss(() => inst.proxy.handleCloseClick(), {
      getScrollTop: () => modalScroll.value?.scrollTop ?? 0,
      handleSelector: '.sheet-handle',
    });
    return {
      modalScroll,
      sheetOffset: swipe.offset,
      sheetDragging: swipe.isDragging,
      onSheetTouchStart: swipe.onTouchStart,
      onSheetTouchMove: swipe.onTouchMove,
      onSheetTouchEnd: swipe.onTouchEnd,
    };
  },

  data() {
    return {
      message: '',
      isSubmitting: false,
      error: '',
      showContent: false,
      minLength: 10,
      maxLength: 1000,
      warningThreshold: 800,
      escListener: null,
      savedMessage: '',
      shouldSaveOnClose: true,
      overlayMousedown: false
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
        this.shouldSaveOnClose = true;
        this.setupEscListener();
        // showContent на nextTick: overlay (v-if=show) монтируется первым, и только затем
        // внутренняя <transition name="modal"> играет РЕАЛЬНЫЙ enter (слайд снизу). Если
        // ставить showContent синхронно с show, модалка появляется внутри только что
        // вставленного родителя - Vue считает это "appear" и без appear-пропа НЕ анимирует
        // (окно просто возникало, не выезжало снизу, #1097 R4-4).
        this.$nextTick(() => {
          this.showContent = true;
          this.$nextTick(() => {
            if (this.autoFocus) {
              this.$refs.textareaRef?.focus();
            }
          });
        });
      } else {
        // Штатное закрытие идёт через closeModal->slide->onSheetLeft (тот эмитит
        // update:show=false). Эта ветка - safety на форс-закрытие родителем напрямую
        // (show=false в обход round-trip): слайда не будет (оверлей v-if=show снимется
        // разом), но обязательно снимаем scroll-lock, иначе body.overflow залипнет.
        this.showContent = false;
        this.removeEscListener();
        releaseBodyScrollLock(this);
      }
    },
    
    message() {
      this.error = '';
    }
  },
  
  created() {
    this.message = this.savedMessage;
  },
  
  mounted() {
    
    this.$watch(
      () => this.show,
      (newVal) => {
        if (newVal) {
          setBodyScrollLock(this, true);
        }
      },
      { immediate: true }
    );
  },
  
  beforeUnmount() {
    this.removeEscListener();
    releaseBodyScrollLock(this);
  },
  
  methods: {
    resetErrors() {
      this.error = '';
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
      // Инициируем закрытие: гасим лист (showContent=false -> slide-down + фейд подложки
      // через is-visible). update:show=false эмитим только когда лист доиграл leave
      // (onSheetLeft), иначе оверлей (v-if=show) снимал бы лист ДО слайда. Гвард от
      // повторного вызова во время анимации.
      if (!this.showContent) return;
      this.showContent = false;
    },

    onSheetLeft() {
      // Лист доехал вниз и размонтирован: размонтируем оверлей (родитель через v-model),
      // сообщаем о закрытии, снимаем scroll-lock.
      releaseBodyScrollLock(this);
      this.$emit('update:show', false);
      this.$emit('close');
    },
    
    onOverlayMousedown(e) {
      this.overlayMousedown = e.target === e.currentTarget;
    },

    onOverlayMouseup(e) {
      const started = this.overlayMousedown;
      this.overlayMousedown = false;
      if (!started || e.target !== e.currentTarget) return;
      if (this.isSubmitting) return;
      this.saveMessage();
      this.closeModal();
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
          const data = await response.json();
          const feedbackId = data.id;

          useDeletionsStore().notify({ prefix: 'Обращение ', bold: `#${feedbackId}`, suffix: ' отправлено', type: 'success' });

          this.shouldSaveOnClose = false;

          setTimeout(() => {
            this.clearSavedMessage();
            this.closeModal();
            this.$emit('submitted', trimmedMessage);
          }, 5000);

        } else {
          let errorMessage = "Ошибка при отправке сообщения";
          try {
            const errorData = await response.json();
            errorMessage = errorData.message || errorMessage;
          } catch (e) {
            console.error("Ошибка парсинга ответа:", e);
          }
          
          this.error = errorMessage;
          useDeletionsStore().notify({ bold: errorMessage, type: 'error' });
          this.isSubmitting = false;
        }
      } catch (error) {
        console.error("Ошибка при отправке обратной связи:", error);
        const errorMsg = "Ошибка сети. Пожалуйста, проверьте подключение к интернету и попробуйте позже.";
        this.error = errorMsg;
        useDeletionsStore().notify({ prefix: 'Не удалось отправить: ', bold: 'ошибка сети', type: 'error' });
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
  }
};
</script>

<style scoped>
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

/* Для мобильных устройств: лист - bottom-sheet, ТОЛЬКО выезд снизу без фейда.
   Транзишн на transform (не all), opacity держим 1 во всех фазах (перебиваем базовое
   opacity:0 у enter-from/leave-to) - иначе лист гаснет вместе со слайдом. */
@media (max-width: 768px) {
  .modal-enter-active,
  .modal-leave-active {
    transition: transform 0.3s ease-out;
  }

  .modal-enter-from {
    transform: translateY(100%);
    opacity: 1;
  }

  .modal-enter-to {
    transform: translateY(0);
    opacity: 1;
  }

  .modal-leave-from {
    transform: translateY(0);
    opacity: 1;
  }

  .modal-leave-to {
    transform: translateY(100%);
    opacity: 1;
  }
}

.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  /* Затемнение через класс is-visible (по showContent), transition background-color -
     фейд подложки идёт синхронно со слайдом листа и НЕ каскадит opacity на лист
     (opacity на оверлее гасила бы и лист -> slide не виден). */
  background-color: var(--overlay);
  transition: background-color 0.3s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  padding: 20px;
  /* backdrop-filter НЕ используем: даже blur(0.1px) форсит compositing-слой и
     репэйнты, роняющие кадры при слайде листа на 120Hz (#1097 R3-4, как BaseModal). */
}

.modal-overlay.is-visible {
  background-color: var(--overlay);
}

.modal {
  background: var(--surface);
  border-radius: 25px;
  width: 100%;
  max-width: 500px;
  box-shadow: 0 20px 60px var(--shadow-drop);
  overflow: hidden;
  position: relative;
  max-height: calc(var(--app-vh, 1vh) * 90);
  display: flex;
  flex-direction: column;
}

/* Ползунок bottom-sheet - виден только на мобилке (тянуть для закрытия). */
.sheet-handle {
  display: none;
  width: 40px;
  height: 4px;
  border-radius: 2px;
  background: var(--border);
  margin: 10px auto 0;
  flex-shrink: 0;
}

.modal__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  flex-shrink: 0;
}

.modal__title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--text);
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
  color: var(--text-muted);
}

.modal__close:hover:not(:disabled) {
  background-color: var(--surface-2);
  color: var(--text);
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
  color: var(--text-muted);
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
  border: 1px solid var(--border);
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
  background: var(--surface);
  color: var(--text);
  display: block;
  padding-bottom: 30px; /* Добавляем отступ снизу для счетчика */
}

.feedback-textarea:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.1);
}

.feedback-textarea:hover:not(:disabled) {
  border-color: var(--border);
}

.feedback-textarea:disabled {
  background-color: var(--surface-2);
  cursor: not-allowed;
  opacity: 0.7;
}

.feedback-textarea::placeholder {
  color: var(--text-muted);
}

.feedback-textarea--error {
  border-color: var(--danger);
  background-color: var(--danger-bg);
}

.feedback-textarea--error:focus {
  border-color: var(--danger);
  box-shadow: 0 0 0 3px rgba(220, 53, 69, 0.1);
}

.textarea-counter-wrapper {
  position: absolute;
  bottom: 8px;
  right: 12px;
  font-size: 12px;
  color: var(--text-muted);
  background-color: color-mix(in srgb, var(--surface) 80%, transparent);
  padding: 2px 6px;
  border-radius: 8px;
  pointer-events: none;
  font-variant-numeric: tabular-nums;
  font-weight: 500;
  transition: all 0.2s ease;
  backdrop-filter: blur(2px);
}

.textarea-counter-wrapper--warning {
  color: var(--warning-text);
  background-color: color-mix(in srgb, var(--warning) 60%, var(--surface));
}

.textarea-counter-wrapper--error {
  color: var(--danger-text);
  background-color: rgba(255, 245, 245, 0.9);
  font-weight: 600;
}

.error-message {
  color: var(--danger-text);
  font-size: 14px;
  margin-top: 12px;
  padding: 10px 12px;
  background-color: var(--danger-bg);
  border-radius: 6px;
  border-left: 3px solid var(--danger);
  animation: error-shake 0.4s ease;
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.error-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.modal__footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 15px 20px;
  border-top: 1px solid var(--border);
  background: var(--surface);
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
  background-color: var(--surface-2);
  color: var(--text-muted);
  border: 1px solid var(--border);
}

.modal-btn--cancel:hover:not(:disabled) {
  background: var(--row-hover);
  color: var(--text);
}

.modal-btn--cancel:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.modal-btn--submit {
  background-color: var(--accent);
  color: var(--accent-contrast);
  border: 1px solid var(--accent);
  width: 240px;
}

.modal-btn--submit:hover:not(.modal-btn--disabled):not(:disabled) {
  background: var(--accent-hover);
  border-color: var(--accent);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(79, 91, 223, 0.3);
}

.modal-btn--submit:active:not(.modal-btn--disabled):not(:disabled) {
  transform: translateY(0);
  box-shadow: 0 2px 6px rgba(79, 91, 223, 0.2);
}

.modal-btn--disabled {
  background-color: var(--accent) !important;
  border-color: var(--accent) !important;
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
  border-top-color: var(--surface);
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

@keyframes spinner-rotate {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}

@media (max-width: 768px) {
  .modal {
    max-width: 100%;
    border-radius: 16px 16px 0 0;
    margin-top: auto;
    margin-bottom: 0;
    /* Выше (директива юзера): лист занимает заметную часть экрана, а не жмётся. */
    min-height: 62dvh;
    max-height: 92dvh;
    transition: transform 0.3s ease;
    will-change: transform;
  }

  /* Во время свайпа лист следует за пальцем 1:1. */
  .modal.is-dragging {
    transition: none;
  }

  .sheet-handle {
    display: block;
  }

  .modal-overlay {
    align-items: flex-end;
    padding: 0;
    /* backdrop-filter НЕ используем на мобилке: blur над оверлеем роняет кадры при
       слайде листа на 120Hz (#1097 R3-4). */
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
    padding: 12px 16px;
    flex-direction: column-reverse;
    gap: 8px;
  }

  .modal-btn {
    width: 100%;
    padding: 11px;
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