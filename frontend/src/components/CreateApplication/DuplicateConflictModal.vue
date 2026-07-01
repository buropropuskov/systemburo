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
          role="dialog"
          aria-modal="true"
        >
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
import { useEscapeClose } from '@/composables/useEscapeClose';

const props = defineProps({
  show: { type: Boolean, default: false },
});
const emit = defineEmits(['replace', 'merge', 'cancel']);

// Escape закрывает как "Отмена" (гейтим по show, т.к. модалка присутствует в DOM всегда).
useEscapeClose(() => emit('cancel'), () => props.show);
</script>

<style scoped>
.dup-conflict-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
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
  background: #ffffff;
  border-radius: 30px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
  /* без overflow:hidden - иначе всплывающая подсказка над кнопкой обрезается по краю диалога */
  font-family: 'Montserrat', sans-serif;
}

.dup-conflict-dialog__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid #e6e6e6;
}

.dup-conflict-dialog__title {
  margin: 0;
  font-size: 1.1em;
  font-weight: 600;
  color: #000;
}

.dup-conflict-dialog__close {
  background: none;
  border: none;
  font-size: 18px;
  cursor: pointer;
  color: #999;
  padding: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.dup-conflict-dialog__close:hover {
  color: #333;
}

.dup-conflict-dialog__body {
  padding: 20px;
  color: #333;
  font-size: 14px;
  line-height: 1.5;
}

.dup-conflict-dialog__footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 20px;
  border-top: 1px solid #e6e6e6;
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
  background: #333;
  color: #fff;
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
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.dup-conflict-dialog__btn[data-hint]::before {
  content: '';
  position: absolute;
  bottom: calc(100% + 4px);
  left: 50%;
  transform: translateX(-50%);
  border: 5px solid transparent;
  border-top-color: #333;
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
  background: #f8f9fa;
  color: #666;
  border-color: #e6e6e6;
}

.dup-conflict-dialog__btn--cancel:hover {
  background: #e9ecef;
}

.dup-conflict-dialog__btn--primary {
  background: #4f5bdf;
  color: #ffffff;
}

.dup-conflict-dialog__btn--primary:hover {
  background: #3a45b2;
}

.dup-conflict-dialog__btn--danger {
  background: #ff6668;
  color: #ffffff;
}

.dup-conflict-dialog__btn--danger:hover {
  background: #e54e50;
}

.dup-conflict-fade-enter-active,
.dup-conflict-fade-leave-active {
  transition: opacity 0.2s ease;
}

.dup-conflict-fade-enter-from,
.dup-conflict-fade-leave-to {
  opacity: 0;
}
</style>
