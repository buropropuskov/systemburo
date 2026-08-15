<template>
  <Teleport to="body">
    <transition name="dup-conflict-fade">
      <div
        v-if="show"
        class="dup-conflict-overlay"
        data-testid="duplicate-conflict-overlay"
        @click.self="$emit('cancel')"
      >
        <div
          class="dup-conflict-dialog"
          :class="{ 'is-dragging': sheetDragging }"
          :style="sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : null"
          role="dialog"
          aria-modal="true"
          @touchstart="onSheetTouchStart"
          @touchmove="onSheetTouchMove"
          @touchend="onSheetTouchEnd"
        >
          <div
            class="sheet-handle"
            aria-hidden="true"
          />
          <div class="dup-conflict-dialog__header">
            <h3 class="dup-conflict-dialog__title">
              В форме уже есть данные
            </h3>
            <button
              class="dup-conflict-dialog__close"
              aria-label="Закрыть"
              @click="$emit('cancel')"
            >
              &times;
            </button>
          </div>
          <div class="dup-conflict-dialog__body">
            В «Оформлении и подаче заявки» уже есть заполненные данные. Что сделать с дублируемой заявкой?
          </div>
          <div class="dup-conflict-dialog__footer">
            <button
              type="button"
              class="dup-conflict-dialog__btn dup-conflict-dialog__btn--cancel"
              data-hint="Оставит текущие данные формы как есть, дубль не применится"
              @click="$emit('cancel')"
            >
              Отмена
            </button>
            <button
              type="button"
              class="dup-conflict-dialog__btn dup-conflict-dialog__btn--primary"
              data-hint="Добавит вложения дублируемой заявки к текущим, уже заполненное не меняется"
              @click="$emit('merge')"
            >
              Объединить
            </button>
            <button
              type="button"
              class="dup-conflict-dialog__btn dup-conflict-dialog__btn--danger"
              data-hint="Удалит текущие данные формы и подставит дублируемую заявку целиком"
              @click="$emit('replace')"
            >
              Заменить
            </button>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup>
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { watch, onBeforeUnmount } from 'vue';
import { useEscapeClose } from '@/composables/useEscapeClose';
import { useSwipeDismiss } from '@/composables/useSwipeDismiss';

const props = defineProps({
  show: { type: Boolean, default: false },
});
const emit = defineEmits(['replace', 'merge', 'cancel']);

// Escape закрывает как "Отмена" (гейтим по show, т.к. модалка присутствует в DOM всегда).
useEscapeClose(() => emit('cancel'), () => props.show, 20000);

// Свайп вниз = отмена, как крестик и клик по затемнению.
const swipe = useSwipeDismiss(() => emit('cancel'), { handleSelector: '.sheet-handle' });
const sheetOffset = swipe.offset;
const sheetDragging = swipe.isDragging;
const onSheetTouchStart = swipe.onTouchStart;
const onSheetTouchMove = swipe.onTouchMove;
const onSheetTouchEnd = swipe.onTouchEnd;

// Владелец блокировки - токен инстанса: в script setup `this` недоступен.
const scrollLockOwner = {};
watch(() => props.show, (visible) => {
  setBodyScrollLock(scrollLockOwner, visible);
});
onBeforeUnmount(() => { releaseBodyScrollLock(scrollLockOwner); });
</script>

<style scoped>
.dup-conflict-overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 20000;
  padding: 20px;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

.dup-conflict-dialog {
  width: 100%;
  max-width: 440px;
  background: var(--surface);
  border-radius: 30px;
  box-shadow: 0 10px 30px var(--shadow-drop);
  /* без overflow:hidden - иначе всплывающая подсказка над кнопкой обрезается по краю диалога */
  font-family: 'Montserrat', sans-serif;
}

.sheet-handle {
  display: none;
  width: 40px;
  height: 4px;
  border-radius: 2px;
  background: var(--border);
  margin: 10px auto 2px;
  flex-shrink: 0;
}

.dup-conflict-dialog__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
}

.dup-conflict-dialog__title {
  margin: 0;
  font-size: 1.1em;
  font-weight: 600;
  color: var(--text);
}

.dup-conflict-dialog__close {
  background: none;
  border: none;
  font-size: 18px;
  cursor: pointer;
  color: var(--text-muted);
  padding: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.dup-conflict-dialog__close:hover {
  color: var(--text);
}

.dup-conflict-dialog__body {
  padding: 20px;
  color: var(--text);
  font-size: 14px;
  line-height: 1.5;
}

.dup-conflict-dialog__footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 20px;
  border-top: 1px solid var(--border);
  flex-wrap: wrap;
}

.dup-conflict-dialog__btn {
  position: relative;
  padding: 9px 20px;
  border-radius: var(--radius-pill, 50px);
  cursor: pointer;
  font-size: 0.85em;
  font-weight: 500;
  transition: all 0.2s ease;
  border: 1px solid transparent;
  min-width: 90px;
  font-family: inherit;
}

/* Всплывающая подсказка над кнопкой (единый системный стиль #333, как .tag-hint). */
.dup-conflict-dialog__btn[data-hint]::after {
  content: attr(data-hint);
  position: absolute;
  bottom: calc(100% + 9px);
  left: 50%;
  transform: translateX(-50%);
  width: max-content;
  max-width: 220px;
  background: var(--hint-bg);
  color: var(--hint-text);
  padding: 6px 10px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 400;
  line-height: 1.35;
  text-align: center;
  white-space: normal;
  z-index: 10;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.15s;
  box-shadow: 0 2px 8px var(--shadow-drop);
}

.dup-conflict-dialog__btn[data-hint]::before {
  content: '';
  position: absolute;
  bottom: calc(100% + 4px);
  left: 50%;
  transform: translateX(-50%);
  border: 5px solid transparent;
  border-top-color: var(--hint-bg);
  z-index: 11;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.15s;
}

.dup-conflict-dialog__btn[data-hint]:hover::after,
.dup-conflict-dialog__btn[data-hint]:hover::before {
  opacity: 1;
}

.dup-conflict-dialog__btn--cancel {
  background: var(--surface-2);
  color: var(--text-muted);
  border-color: var(--border);
}

.dup-conflict-dialog__btn--cancel:hover {
  background: var(--accent-tint);
}

.dup-conflict-dialog__btn--primary {
  background: var(--accent);
  color: var(--accent-contrast);
}

.dup-conflict-dialog__btn--primary:hover {
  background: var(--accent-hover);
}

.dup-conflict-dialog__btn--danger {
  background: var(--danger);
  color: var(--fill-text);
}

.dup-conflict-dialog__btn--danger:hover {
  background: color-mix(in srgb, var(--danger) 85%, var(--text));
}

.dup-conflict-fade-enter-active,
.dup-conflict-fade-leave-active {
  transition: opacity 0.2s ease;
}

.dup-conflict-fade-enter-from,
.dup-conflict-fade-leave-to {
  opacity: 0;
}

@media (max-width: 768px) {
  /* Окно жило вне общего контракта: карточка 440px с радиусом 30px по центру.
     Приводим к листу снизу, как у остальных окон страницы. */
  .dup-conflict-overlay {
    padding: 0;
    align-items: flex-end;
  }

  .dup-conflict-dialog {
    max-width: 100%;
    max-height: 90dvh;
    overflow-y: auto;
    border-radius: 16px 16px 0 0;
    transition: transform 0.3s ease;
    animation: dup-conflict-up 0.3s cubic-bezier(0.32, 0.72, 0, 1) backwards;
  }

  .dup-conflict-dialog.is-dragging {
    transition: none;
  }

  .sheet-handle {
    display: block;
  }

  .dup-conflict-dialog__close {
    width: 36px;
    height: 36px;
  }


  /* Подсказка к варианту живёт на hover - на телефоне его нет, а тап по кнопке
     сразу выполняет действие. Показываем пояснение прямо в кнопке строкой ниже. */
  .dup-conflict-dialog__footer {
    flex-direction: column;
    align-items: stretch;
  }

  .dup-conflict-dialog__btn {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
    min-height: 56px;
    padding: 10px 14px;
    text-align: left;
  }

  .dup-conflict-dialog__btn[data-hint]::after {
    position: static;
    transform: none;
    opacity: 1;
    max-width: 100%;
    padding: 0;
    background: none;
    box-shadow: none;
    color: inherit;
    font-size: 11px;
    line-height: 1.3;
    text-align: left;
    opacity: 0.75;
  }

  .dup-conflict-dialog__btn[data-hint]::before {
    display: none;
  }
}

@keyframes dup-conflict-up {
  from { transform: translateY(100%); }
  to { transform: translateY(0); }
}
</style>
