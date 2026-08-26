<template>
  <teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="base-modal-overlay"
        :data-theme="theme || null"
        :style="{ zIndex }"
        @mousedown="handleOverlayMousedown"
        @mouseup="handleOverlayMouseup"
      >
        <div
          ref="modal"
          class="base-modal"
          :class="[contentClass, { 'base-modal--sheet': sheetSwipe, 'is-dragging': sheetDragging, 'is-closing': sheetClosing }]"
          :style="{ maxWidth: width, '--base-modal-radius': radius || null, ...(sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : {}) }"
          role="dialog"
          aria-modal="true"
          :data-testid="contentTestid || null"
          :aria-label="title"
          @click.stop
          @mousedown.stop
          @touchstart="onSheetTouchStart"
          @touchmove="onSheetTouchMove"
          @touchend="onSheetTouchEnd"
        >
          <div
            v-if="sheetSwipe"
            class="sheet-handle"
            aria-hidden="true"
          />
          <div
            v-if="title || $slots.header || closable"
            class="base-modal__header"
          >
            <slot name="header">
              <h3 class="base-modal__title">
                {{ title }}
              </h3>
            </slot>
            <button
              v-if="closable"
              class="base-modal__close"
              data-testid="modal-button-close"
              aria-label="Закрыть"
              @click="$emit('close')"
            >
              &times;
            </button>
          </div>
          <div
            ref="body"
            class="base-modal__body"
          >
            <slot />
          </div>
          <div
            v-if="$slots.actions"
            class="base-modal__actions"
          >
            <slot name="actions" />
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script>
import { ref } from 'vue';
import { useSwipeDismiss } from '@/composables/useSwipeDismiss';
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { setModalOpen, releaseModal, isTopModal, isEscapeHandled, markEscapeHandled } from '@/utils/modalStack';

export default {
  name: 'BaseModal',
  props: {
    show: {
      type: Boolean,
      required: true,
    },
    title: {
      type: String,
      default: '',
    },
    width: {
      type: String,
      default: '500px',
    },
    closable: {
      type: Boolean,
      default: true,
    },
    closeOnOverlay: {
      type: Boolean,
      default: true,
    },
    // Якорь для онбординга: окно телепортируется в body, и достучаться до него
    // из родителя нечем - имя передаётся явно.
    contentTestid: {
      type: String,
      default: '',
    },
    contentClass: {
      type: String,
      default: '',
    },
    // z-index оверлея. Дефолт 1000 (базовый слой модалок). Поднимать, когда модалка
    // открывается ПОВЕРХ другой модалки с высоким z-index (напр. из карточки Т/С).
    zIndex: {
      type: Number,
      default: 1000,
    },
    // Радиус скругления окна. Пусто -> дефолт var(--radius-md). Задаётся окном, т.к.
    // content-class телепортируется в body и scoped :deep из родителя до него не
    // достаёт - пробрасываем значение CSS-переменной инлайном.
    radius: {
      type: String,
      default: '',
    },
    // Тема окна: пусто -> следует за выбранной темой системы. Значение из
    // utils/theme.js ставит «островок» на слой окна (tokens.css оформляет любой
    // элемент с data-theme, не только <html>). Нужно окнам с экранов, которые
    // сами живут вне тем: вход - светлый остров, но окно телепортируется в body
    // и без этого пропа оставалось тёмным поверх светлого экрана.
    theme: {
      type: String,
      default: '',
    },
    // Bottom-sheet со свайпом-вниз на мобилке (#1097). По умолчанию ВКЛючён - все
    // модалки на BaseModal выезжают снизу с ползунком/слайдом/свайп-закрытием
    // (единое мобильное поведение). Отключить точечно: :sheet-swipe="false".
    sheetSwipe: {
      type: Boolean,
      default: true,
    },
  },
  emits: ['close'],
  setup(props, { emit }) {
    // modal - template-ref окна, body - тело. Скроллер зависит от вьюпорта: на
    // мобилке (<=768) скроллится body (окно overflow:hidden), на десктоп/планшет
    // (>768) - само окно (body без overflow). Берём max: у неактивного скроллера
    // scrollTop == 0, у активного - реальный, - иначе свайп внутри прокрученного
    // контента ошибочно трактуется как закрытие (getScrollTop всегда 0).
    const modal = ref(null);
    const body = ref(null);
    const swipe = useSwipeDismiss(() => emit('close'), {
      getScrollTop: () => Math.max(body.value?.scrollTop ?? 0, modal.value?.scrollTop ?? 0),
      handleSelector: '.sheet-handle',
    });
    // Свайп активен только при sheetSwipe - иначе жест на любой модалке не трогаем.
    const guard = (fn) => (e) => {
      if (props.sheetSwipe) fn(e);
    };
    return {
      modal,
      body,
      sheetOffset: swipe.offset,
      sheetDragging: swipe.isDragging,
      sheetClosing: swipe.closing,
      resetSwipe: swipe.reset,
      onSheetTouchStart: guard(swipe.onTouchStart),
      onSheetTouchMove: guard(swipe.onTouchMove),
      onSheetTouchEnd: guard(swipe.onTouchEnd),
    };
  },
  data() {
    return {
      overlayMousedown: false,
    };
  },
  watch: {
    show: {
      immediate: true,
      handler(val) {
        // Через общий замок: окна живут стопкой, и прямое присвоение снимало блокировку,
        // поставленную окном-родителем (закрыли вложенное - фон поехал под открытым).
        setBodyScrollLock(this, val);
        // Та же стопка отвечает на вопрос «кто сверху» для Escape.
        setModalOpen(this, val, Number(this.zIndex) || 0);
        // Переоткрытие: сбросить застрявший после свайп-закрытия offset/closing (лист снизу).
        if (val) this.resetSwipe();
      },
    },
  },
  mounted() {
    document.addEventListener('keydown', this.handleKeydown);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.handleKeydown);
    releaseBodyScrollLock(this);
    releaseModal(this);
  },
  methods: {
    handleOverlayMousedown(e) {
      this.overlayMousedown = e.target === e.currentTarget;
    },
    handleOverlayMouseup(e) {
      const startedOnOverlay = this.overlayMousedown;
      this.overlayMousedown = false;
      if (!startedOnOverlay) return;
      if (e.target !== e.currentTarget) return;
      if (this.closeOnOverlay) {
        this.$emit('close');
      }
    },
    handleKeydown(e) {
      if (!this.show) return;
      if (e.key === 'Escape' && this.closable) {
        // Одно нажатие - один закрытый слой. Стопка отвечает, кто сейчас сверху, а
        // пометка на событии страхует от порядка слушателей: слой, ответивший первым,
        // забирает нажатие себе, даже если со стопки он снимется только следующим тиком.
        if (isEscapeHandled(e)) return;
        if (!isTopModal(this)) return;
        markEscapeHandled(e);
        this.$emit('close');
      }
      if (e.key === 'Tab') {
        this.trapFocus(e);
      }
    },
    trapFocus(e) {
      const container = this.modal;
      if (!container) return;
      const focusable = container.querySelectorAll(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      if (focusable.length === 0) return;

      const first = focusable[0];
      const last = focusable[focusable.length - 1];

      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    },
  },
};
</script>

<style scoped>
.base-modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  /* z-index задаётся через prop zIndex (:style), дефолт 1000 - базовый слой модалок.
     backdrop-filter НЕ используем: даже blur(0.1px) форсит compositing-слой и
     репэйнты, роняющие кадры при слайде листа на 120Hz (#1097 p2). */
}

.base-modal {
  background: var(--surface);
  border-radius: var(--base-modal-radius, var(--radius-md));
  box-shadow: 0 8px 30px var(--shadow-drop);
  /* Не голый 92vh: на >1440 корень зумлен (viewportScale.js), vh считается от
     НЕзумленной высоты и завышает кап в zoom раз - на 2500px окно вылезало за
     экран (#1359). --app-vh = innerHeight/zoom/100, нормирован на zoom; на <=1440
     (zoom=1) он равен vh, поведение не меняется. Эталон - UserAccessModal (P10). */
  max-height: calc(var(--app-vh, 1vh) * 92);
  overflow-y: auto;
  width: 100%;
  margin: 0 20px;
}

/* Ползунок bottom-sheet - виден только на мобилке (тянуть для закрытия), см. sheetSwipe. */
.sheet-handle {
  display: none;
  width: 40px;
  height: 4px;
  border-radius: 2px;
  background: var(--border);
  margin: 10px auto 0;
  flex-shrink: 0;
}

.base-modal__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
}

.base-modal__title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--text);
}

.base-modal__close {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  line-height: 1;
  color: var(--text-muted);
  background: none;
  border: none;
  cursor: pointer;
  border-radius: 50%;
  transition: all 0.2s;
  flex-shrink: 0;
}

.base-modal__close:hover {
  color: var(--text);
  background: var(--surface-2);
}

.base-modal__body {
  padding: 0;
}

.base-modal__actions {
  padding: 15px 20px;
  border-top: 1px solid var(--color-border);
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

/* Transition */
.modal-fade-enter-active {
  transition: opacity 0.3s ease;
}
.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .base-modal {
  animation: modal-scale-in 0.3s ease;
}
.modal-fade-leave-active .base-modal {
  animation: modal-scale-out 0.2s ease;
}

@keyframes modal-scale-in {
  from {
    transform: scale(0.95);
    opacity: 0;
  }
  to {
    transform: scale(1);
    opacity: 1;
  }
}

@keyframes modal-scale-out {
  from {
    transform: scale(1);
    opacity: 1;
  }
  to {
    transform: scale(0.95);
    opacity: 0;
  }
}

/* Слайд bottom-sheet (мобилка): въезд снизу вверх и обратно. */
@keyframes base-sheet-up {
  from {
    transform: translateY(100%);
  }
  to {
    transform: translateY(0);
  }
}

@keyframes base-sheet-down {
  from {
    transform: translateY(0);
  }
  to {
    transform: translateY(100%);
  }
}

/* Bottom-sheet на мобильном */
@media (max-width: 768px) {
  .base-modal-overlay {
    padding: 0;
    align-items: flex-end;
    /* Нативный dvh: композитор держит высоту у видимой области без reflow-лага. */
    top: 0;
    height: 100dvh;
    bottom: auto;
  }

  .base-modal {
    width: 100vw !important;
    max-width: 100vw !important;
    min-width: 100vw !important;
    max-height: 90dvh;
    border-radius: 16px 16px 0 0;
    margin: 0;
    /* Тело скроллится, ползунок/шапка/actions зафиксированы (общий баг 4). */
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .base-modal .sheet-handle,
  .base-modal__header,
  .base-modal__actions {
    flex-shrink: 0;
  }

  .base-modal__body {
    flex: 1 1 auto;
    min-height: 0;
    overflow-y: auto;
    -webkit-overflow-scrolling: touch;
    overscroll-behavior: contain;
  }

  /* Компактная шапка на мобилке: близко посаженный заголовок + меньший ползунок и
     тач-таргет крестика (40px хватает, лист ещё закрывается свайпом/оверлеем) - высокая
     шапка (69px из-за 44px крестика + padding 12) съедала место под контент (#1097 R4-6/8). */
  .base-modal .sheet-handle {
    margin: 6px auto 0;
  }

  .base-modal__header {
    padding: 6px 16px;
  }

  .base-modal__title {
    font-size: 16px;
  }

  /* Sheet-вариант: выезжает снизу, тянется за пальцем 1:1 во время свайпа. */
  .base-modal--sheet {
    transition: transform 0.34s cubic-bezier(0.32, 0.72, 0, 1);
    will-change: transform;
  }

  .base-modal--sheet.is-dragging {
    transition: none;
  }

  .base-modal--sheet .sheet-handle {
    display: block;
  }

  .modal-fade-enter-active .base-modal--sheet {
    animation: base-sheet-up 0.34s cubic-bezier(0.32, 0.72, 0, 1);
  }

  .modal-fade-leave-active .base-modal--sheet {
    animation: base-sheet-down 0.24s cubic-bezier(0.32, 0.72, 0, 1);
  }

  /* Закрытие свайпом: лист уже уехал вниз по offset (transition transform), поэтому
     @keyframes base-sheet-down (0 -> 100%) дал бы ВТОРОЙ слайд с рывком наверх. Гасим -
     остаётся только inline transform (лист за кромкой) + fade оверлея (#1097 R3-1). */
  .modal-fade-leave-active .base-modal--sheet.is-closing {
    animation: none;
  }

  .base-modal__close {
    min-width: 40px;
    min-height: 40px;
  }

  .base-modal__body input[type="text"],
  .base-modal__body input[type="email"],
  .base-modal__body input[type="password"],
  .base-modal__body input[type="tel"],
  .base-modal__body input[type="number"],
  .base-modal__body textarea,
  .base-modal__body select {
    font-size: 16px;
  }
}
</style>
