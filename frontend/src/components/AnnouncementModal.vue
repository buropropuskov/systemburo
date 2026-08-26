<template>
  <teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="modal-overlay"
        @click.self="close"
      >
        <div
          class="modal-content announcement-modal"
          :class="{ 'is-dragging': sheetDragging }"
          :style="sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : null"
          @touchstart="onSheetTouchStart"
          @touchmove="onSheetTouchMove"
          @touchend="onSheetTouchEnd"
        >
          <div
            class="sheet-handle"
            aria-hidden="true"
          />
          <div
            ref="sheetScroll"
            class="modal-body"
          >
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
              class="modal-full-text announcement-body-html"
              v-html="sanitizeHtml(announcement.full_text)"
            />
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
import { ref } from 'vue'
import { sanitizeHtml } from '@/utils/sanitize.js'
import { useSwipeDismiss } from '@/composables/useSwipeDismiss'

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
  setup(props, { emit }) {
    const sheetScroll = ref(null);
    const close = () => {
      emit('update:show', false);
      emit('close');
    };
    const swipe = useSwipeDismiss(close, {
      getScrollTop: () => sheetScroll.value?.scrollTop ?? 0,
      handleSelector: '.sheet-handle',
    });
    return {
      sheetScroll,
      close,
      sheetOffset: swipe.offset,
      sheetDragging: swipe.isDragging,
      onSheetTouchStart: swipe.onTouchStart,
      onSheetTouchMove: swipe.onTouchMove,
      onSheetTouchEnd: swipe.onTouchEnd,
    };
  },
  methods: {
    sanitizeHtml,
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
    }
  },
  mounted() {
    this.escHandler = (e) => {
      if (e.key === 'Escape' && this.show) this.close();
    };
    document.addEventListener('keydown', this.escHandler);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.escHandler);
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
  background-color: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
  /* backdrop-filter НЕ используем: даже blur(0.1px) форсит compositing-слой и
     репэйнты, роняющие кадры при слайде листа на 120Hz (#1097 R3-4, как BaseModal). */
}

/* Открытие 1:1 как у остальных окон обзора («Сообщить о проблеме», «Режимы работы»,
   «Руководство», карточка новости - modal-fade в NewsAndReview): фейдится ВЕСЬ оверлей
   по opacity, лист дополнительно приходит из scale(0.9). Отдельный фейд одной подложки
   (background-color) давал другое ощущение открытия, чем у соседних окон. */
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
  background: var(--surface);
  border-radius: 50px;
  width: 600px;
  max-width: 90vw;
  max-height: calc(var(--app-vh, 1vh) * 80);
  box-shadow: 0 20px 60px var(--shadow-drop);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.announcement-modal .modal-content {
  width: 540px;
}

/* Ползунок bottom-sheet - виден только на мобилке (тянуть за него для закрытия). */
.sheet-handle {
  display: none;
  width: 40px;
  height: 4px;
  border-radius: 2px;
  background: var(--border);
  margin: 10px auto 2px;
  flex-shrink: 0;
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
  color: var(--text);
  flex: 1;
  padding-bottom: 26px;
}

.announcement-title {
  margin: 0 0 10px 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--accent-text);
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
  background-color: var(--surface-2);
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
  color: var(--text-muted);
}

.modal-type {
  font-size: 11px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 20px;
  background: var(--warning-bg);
  border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
  color: var(--warning-text);
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.modal-type.important {
  background: var(--danger-bg);
  color: var(--danger-text);
}

.modal-description {
  font-size: 14px;
  line-height: 1.5;
  color: var(--text);
  margin: 0 0 20px 0;
}

.modal-full-text {
  font-size: 14px;
  line-height: 1.6;
  color: var(--text-muted);
  padding-top: 16px;
  border-top: 1px solid var(--border);
  margin-top: 8px;
}

.modal-footer {
  padding: 16px 30px 24px;
  border-top: 1px solid var(--border);
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
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

.close-modal-btn:hover {
  background: var(--accent-hover);
}

@media (max-width: 768px) {
  /* Прилипание к низу/ширина/радиус bottom-sheet задаёт глобальный паттерн App.vue
     (.modal-overlay>.modal-content, !important). Здесь добавляем только выезд снизу
     вверх, свайп-закрытие и ползунок. transition для снап-назад после свайпа. */
  .modal-content {
    transition: transform 0.3s ease;
    will-change: transform;
  }

  /* Во время свайпа лист следует за пальцем 1:1 (без сглаживания). */
  .modal-content.is-dragging {
    transition: none;
  }

  .sheet-handle {
    display: block;
  }

  /* Слайд снизу вверх на открытие/закрытие вместо scale (десктоп-анимации). */
  .modal-fade-enter-from .modal-content,
  .modal-fade-leave-to .modal-content {
    transform: translateY(100%);
    opacity: 1;
  }

  .modal-header {
    padding: 16px 20px 0;
  }

  .modal-body {
    padding: 8px 20px 16px;
  }

  .modal-footer {
    padding: 12px 20px 20px;
  }
}

.announcement-body-html {
  line-height: 1.6;
}
.announcement-body-html :deep(*) { overflow-wrap: break-word; }
.announcement-body-html :deep(h1),
.announcement-body-html :deep(h2),
.announcement-body-html :deep(h3) { font-weight: 600; margin: 0.75em 0 0.4em; }
.announcement-body-html :deep(p) { margin: 0.5em 0; }
.announcement-body-html :deep(ul),
.announcement-body-html :deep(ol) { padding-left: 1.5em; margin: 0.5em 0; }
.announcement-body-html :deep(img) { max-width: 100%; border-radius: 8px; }
.announcement-body-html :deep(img:not([height])) { height: auto; }
.announcement-body-html :deep(.constructor-image.img-align-left) { float: left; margin: 0 14px 10px 0; }
.announcement-body-html :deep(.constructor-image.img-align-right) { float: right; margin: 0 0 10px 14px; }
.announcement-body-html :deep(.constructor-image.img-align-center) { display: block; margin: 10px auto; float: none; }
.announcement-body-html::after { content: ''; display: block; clear: both; }
.announcement-body-html :deep(.text-align-left) { text-align: left; }
.announcement-body-html :deep(.text-align-center) { text-align: center; }
.announcement-body-html :deep(.text-align-right) { text-align: right; }
</style>