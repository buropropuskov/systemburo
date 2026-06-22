<template>
  <Teleport to="body">
    <transition name="confirm-fade">
      <div
        v-if="state"
        class="confirm-overlay"
        data-testid="confirm-overlay"
        @click.self="cancel"
      >
        <div
          class="confirm-dialog"
          role="dialog"
          aria-modal="true"
        >
          <div class="confirm-dialog__header">
            <h3 class="confirm-dialog__title">
              {{ state.title }}
            </h3>
            <button
              class="confirm-dialog__close"
              aria-label="Закрыть"
              @click="cancel"
            >
              ×
            </button>
          </div>
          <div class="confirm-dialog__body">
            {{ state.message }}
          </div>
          <div class="confirm-dialog__footer">
            <button
              class="confirm-dialog__btn confirm-dialog__btn--cancel"
              data-testid="confirm-cancel"
              @click="cancel"
            >
              {{ state.cancelText }}
            </button>
            <button
              class="confirm-dialog__btn"
              :class="state.danger ? 'confirm-dialog__btn--danger' : 'confirm-dialog__btn--primary'"
              data-testid="confirm-ok"
              @click="confirm"
            >
              {{ state.confirmText }}
            </button>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import { computed } from 'vue';
import { useUiStore } from '@/stores/ui';
import { useEscapeClose } from '@/composables/useEscapeClose';

export default {
  name: 'ConfirmDialog',
  setup() {
    const ui = useUiStore();
    const state = computed(() => ui.confirmState);

    function confirm() {
      ui.resolveConfirm(true);
    }
    function cancel() {
      ui.resolveConfirm(false);
    }

    useEscapeClose(cancel, () => !!state.value);

    return { state, confirm, cancel };
  },
};
</script>

<style scoped>
.confirm-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  /* Глобальный блокирующий confirm должен лежать ПОВЕРХ любых стопок модалок, из которых
     его зовут (деталь заявки 10002, карточка 10003, override 10005, история 12000). Ниже
     тоста (29000), чтобы уведомления оставались видны. Раньше было 1100 - терялся за
     карточкой авто/сотрудника, открытой из заявки (#481). */
  z-index: 20000;
  padding: 20px;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

.confirm-dialog {
  width: 100%;
  max-width: 400px;
  background: #FFFFFF;
  border-radius: 30px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
  overflow: hidden;
  font-family: 'Montserrat', sans-serif;
}

.confirm-dialog__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid #E6E6E6;
}

.confirm-dialog__title {
  margin: 0;
  font-size: 1.1em;
  font-weight: 600;
  color: #000;
}

.confirm-dialog__close {
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

.confirm-dialog__close:hover {
  color: #333;
}

.confirm-dialog__body {
  padding: 20px;
  color: #333;
  font-size: 14px;
  line-height: 1.5;
}

.confirm-dialog__footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 20px;
  border-top: 1px solid #E6E6E6;
}

.confirm-dialog__btn {
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

.confirm-dialog__btn--cancel {
  background: #f8f9fa;
  color: #666;
  border-color: #e6e6e6;
}

.confirm-dialog__btn--cancel:hover {
  background: #e9ecef;
}

.confirm-dialog__btn--primary {
  background: #4F5BDF;
  color: #FFFFFF;
}

.confirm-dialog__btn--primary:hover {
  background: #3a45b2;
}

.confirm-dialog__btn--danger {
  background: #FF6668;
  color: #FFFFFF;
}

.confirm-dialog__btn--danger:hover {
  background: #e54e50;
}

.confirm-fade-enter-active,
.confirm-fade-leave-active {
  transition: opacity 0.2s ease;
}

.confirm-fade-enter-from,
.confirm-fade-leave-to {
  opacity: 0;
}
</style>
